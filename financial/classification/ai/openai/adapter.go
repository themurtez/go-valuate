// Package openai is an OpenAI-backed implementation of
// financial/classification/ai.Classifier, using the official
// github.com/openai/openai-go/v2 SDK (Apache-2.0 licensed).
//
// Every OpenAI-specific type (openai.Client, chat-completion params, the
// SDK's own Error type) is confined to this package — nothing outside
// financial/classification/ai/openai ever imports openai-go, so the rest of
// this repository compiles and every OTHER package's tests run with zero
// network access and zero credentials. Only THIS package's own opt-in,
// credential-gated integration test (see openai_integration_test.go)
// touches the network, and it is skipped entirely unless an API key is
// present in the environment.
//
// The caller always supplies configuration explicitly (see Config/New) —
// this package never initializes a client at package scope and never reads
// an environment variable except inside the one explicit convenience
// constructor documented for that (NewFromEnv).
package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	openaisdk "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification/ai"
)

// AdapterVersion identifies this adapter package's own request-building/
// response-parsing logic, independent of ai.RequestSchemaVersion (the
// wire-shape contract) and ai.OrchestrationVersion (when/why a request is
// sent at all) — see the repository README's versioning-strategy section.
// Bump only when this file's prompt construction or response parsing
// changes in a way that could make a historical ai.Provenance not
// reproduce identically under new code.
const AdapterVersion = "1.0.0"

// DefaultModel is used when Config.Model is empty. Chosen for cost/latency
// suitability for a short, closed-set classification task, not for general
// reasoning quality — a caller with different cost/quality priorities sets
// Config.Model explicitly.
const DefaultModel = "gpt-4o-mini"

// providerName is the fixed ai.Response.Provider value this adapter
// reports.
const providerName = "openai"

// Config bundles everything needed to construct a Classifier. The caller
// always supplies it explicitly — see Package's doc comment.
type Config struct {
	// APIKey is the OpenAI API key. Required unless HTTPClient/ClientOptions
	// already configures authentication another way (e.g. an
	// organization-managed proxy). Never logged — see Classifier.Classify's
	// doc comment.
	APIKey string
	// Model is the model identifier (e.g. "gpt-4o-mini"). Defaults to
	// DefaultModel when empty.
	Model string
	// BaseURL overrides the SDK's default API base URL, for a proxy or
	// Azure-OpenAI-compatible endpoint. Optional.
	BaseURL string
	// HTTPClient overrides the SDK's default HTTP client (e.g. to inject a
	// custom transport, proxy, or test double). Optional.
	HTTPClient *http.Client
	// ClientOptions is passed straight through to openai-go's NewClient for
	// any configuration this Config does not otherwise expose. Optional.
	ClientOptions []option.RequestOption
	// MaxBatchCharacters bounds the estimated size (marshaled-JSON byte
	// length, not an exact token count) of the rows this adapter packs into
	// one underlying batch chat-completion request — see
	// DefaultMaxBatchCharacters and splitBatchByCharacterBudget. Defaults to
	// DefaultMaxBatchCharacters when zero. This is independent of
	// ai.Policy.MaxBatchSize (a row-COUNT cap enforced one layer up, in
	// ai.ClassifyBatchWithFallback, before ClassifyBatch is ever called) —
	// the two guards compose rather than replace one another, so a caller
	// with unusually long labels/context rows is still protected even at a
	// small Policy.MaxBatchSize.
	MaxBatchCharacters int
}

// Classifier is an ai.Classifier (and ai.BatchClassifier — see batch.go) backed
// by the OpenAI chat-completions API using Structured Outputs (a strict JSON
// Schema response format — see buildResponseFormat/buildBatchResponseFormat)
// so every response is guaranteed to parse into ai.Response's shape without
// free-text scraping.
//
// A Classifier value is safe for concurrent use by multiple goroutines:
// openai-go's Client is documented as safe for concurrent use, and
// Classifier holds no other mutable state.
type Classifier struct {
	client             openaisdk.Client
	model              string
	maxBatchCharacters int
}

// New constructs a Classifier from an explicit Config. Does not perform any
// network call itself (the underlying SDK client is lazy) — no credentials
// are validated until the first Classify call.
func New(cfg Config) (*Classifier, error) {
	if cfg.APIKey == "" && len(cfg.ClientOptions) == 0 {
		return nil, errors.New("openai: Config.APIKey is required (or supply authentication via Config.ClientOptions)")
	}

	opts := make([]option.RequestOption, 0, 4+len(cfg.ClientOptions))
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if cfg.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(cfg.HTTPClient))
	}
	opts = append(opts, cfg.ClientOptions...)

	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	return &Classifier{
		client:             openaisdk.NewClient(opts...),
		model:              model,
		maxBatchCharacters: cfg.MaxBatchCharacters,
	}, nil
}

// NewFromEnv is the one explicit convenience constructor that reads
// configuration from the environment (OPENAI_API_KEY, and optionally
// OPENAI_MODEL/OPENAI_BASE_URL) — see the package doc comment: domain logic
// itself never does this; only this named, opt-in entry point does. Returns
// an error (never a partially-usable Classifier) if OPENAI_API_KEY is
// unset.
func NewFromEnv() (*Classifier, error) {
	apiKey := osGetenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("openai: OPENAI_API_KEY is not set")
	}
	return New(Config{
		APIKey:  apiKey,
		Model:   osGetenv("OPENAI_MODEL"),
		BaseURL: osGetenv("OPENAI_BASE_URL"),
	})
}

// Classify implements ai.Classifier. It sends req as a structured chat
// completion request (see buildChatParams) and parses the model's
// structured-output JSON into an ai.Response. It never logs req's contents,
// the API key, or the raw provider response (see the package doc comment's
// privacy section) — any diagnostic detail returned is limited to what
// Classify's own error value carries, which the caller decides whether/how
// to log.
//
// Respects ctx cancellation/deadlines directly (passed straight to the SDK
// call); does not retry internally — see Config's doc comment: retries, if
// wanted, are the caller's explicit choice (e.g. via
// option.WithMaxRetries in Config.ClientOptions), never hidden inside this
// method.
func (c *Classifier) Classify(ctx context.Context, req ai.Request) (ai.Response, error) {
	params := buildChatParams(c.model, req)

	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return ai.Response{}, wrapError(err)
	}
	if len(completion.Choices) == 0 {
		return ai.Response{}, errors.New("openai: response contained no choices")
	}

	content := completion.Choices[0].Message.Content
	if content == "" {
		return ai.Response{}, errors.New("openai: response message had no content")
	}

	var parsed structuredResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return ai.Response{}, fmt.Errorf("openai: could not parse structured output as JSON: %w", err)
	}

	resp := parsed.toAIResponse()
	resp.Provider = providerName
	resp.Model = completion.Model
	resp.AdapterVersion = AdapterVersion
	return resp, nil
}

// structuredResponse mirrors the JSON Schema handed to the model (see
// buildResponseFormat) — kept as this adapter's own private decoding
// target rather than unmarshaling straight into ai.Response, so a future
// change to ai.Response's own JSON tags can never silently change what
// shape this adapter demands from the model.
type structuredResponse struct {
	Code         string   `json:"code"`
	Alternatives []string `json:"alternatives"`
	Reason       string   `json:"reason"`
	Confidence   *float64 `json:"confidence"`
}

func (s structuredResponse) toAIResponse() ai.Response {
	alts := make([]financial.Code, 0, len(s.Alternatives))
	for _, a := range s.Alternatives {
		alts = append(alts, financial.Code(a))
	}
	return ai.Response{
		Code:          financial.Code(s.Code),
		Alternatives:  alts,
		Reason:        s.Reason,
		RawConfidence: s.Confidence,
	}
}

// wrapError classifies an openai-go SDK error into ai's optional
// TimeoutError/RateLimitError interfaces where possible, so
// ai.ClassifyWithFallback can report the specific IssueTimeout/
// IssueRateLimited codes instead of the generic IssueProviderError — see
// ai.classifyProviderError. The original SDK error is always preserved via
// errors.Unwrap/errors.Is, never discarded.
func wrapError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	var apiErr *openaisdk.Error
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests {
			return &rateLimitedError{inner: apiErr}
		}
		if apiErr.StatusCode == http.StatusRequestTimeout || apiErr.StatusCode == http.StatusGatewayTimeout {
			return &timeoutError{inner: apiErr}
		}
	}
	return err
}

type rateLimitedError struct{ inner error }

func (e *rateLimitedError) Error() string     { return e.inner.Error() }
func (e *rateLimitedError) Unwrap() error     { return e.inner }
func (e *rateLimitedError) RateLimited() bool { return true }

type timeoutError struct{ inner error }

func (e *timeoutError) Error() string { return e.inner.Error() }
func (e *timeoutError) Unwrap() error { return e.inner }
func (e *timeoutError) Timeout() bool { return true }

var (
	_ ai.RateLimitError = (*rateLimitedError)(nil)
	_ ai.TimeoutError   = (*timeoutError)(nil)
	_ ai.Classifier     = (*Classifier)(nil)
)
