// Package openai is an OpenAI-backed implementation of
// financial/adjustments/ai.Suggester, using the official
// github.com/openai/openai-go/v2 SDK (Apache-2.0 licensed).
//
// Every OpenAI-specific type (openai.Client, chat-completion params, the
// SDK's own Error type) is confined to this package — nothing outside
// financial/adjustments/ai/openai ever imports openai-go, so the rest of
// this repository compiles and every OTHER package's tests run with zero
// network access and zero credentials. Only THIS package's own opt-in,
// credential-gated integration test touches the network, and it is skipped
// entirely unless an API key is present in the environment.
//
// This adapter is deliberately independent of
// financial/classification/ai/openai: the two packages solve different
// problems (closed-set label classification for one row at a time vs.
// source-bound adjustment suggestions for a bounded batch of rows) with
// different wire contracts, so forcing them to share a Config/Classifier-
// shaped abstraction would leak one domain's assumptions into the other —
// see the top-level financial/adjustments/ai package doc comment. Request-
// building/response-parsing patterns (Structured Outputs, Store: false,
// explicit Config, no global client, character-budgeted batching) are
// reused where they genuinely apply, matching this repository's established
// conventions.
package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	openaisdk "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"

	"github.com/themurtez/go-valuate/financial/adjustments/ai"
)

// AdapterVersion identifies this adapter package's own request-building/
// response-parsing logic, independent of ai.RequestSchemaVersion (the
// wire-shape contract) and ai.OrchestrationVersion (when/why a request is
// sent at all).
const AdapterVersion = "1.0.0"

// DefaultModel is used when Config.Model is empty.
const DefaultModel = "gpt-4o-mini"

// providerName is the fixed ai.Response.Provider value this adapter
// reports.
const providerName = "openai"

// Config bundles everything needed to construct a Suggester. The caller
// always supplies it explicitly — this package never initializes a client
// at package scope.
type Config struct {
	// APIKey is the OpenAI API key. Required unless HTTPClient/ClientOptions
	// already configures authentication another way. Never logged.
	APIKey string
	// Model is the model identifier. Defaults to DefaultModel when empty.
	Model string
	// BaseURL overrides the SDK's default API base URL.
	BaseURL string
	// HTTPClient overrides the SDK's default HTTP client.
	HTTPClient *http.Client
	// ClientOptions is passed straight through to openai-go's NewClient.
	ClientOptions []option.RequestOption
}

// Suggester is an ai.Suggester backed by the OpenAI chat-completions API
// using Structured Outputs (a strict JSON Schema response format) so every
// response is guaranteed to parse into ai.Response's shape without
// free-text scraping.
//
// A Suggester value is safe for concurrent use by multiple goroutines:
// openai-go's Client is documented as safe for concurrent use, and Suggester
// holds no other mutable state.
type Suggester struct {
	client openaisdk.Client
	model  string
}

// New constructs a Suggester from an explicit Config. Does not perform any
// network call itself.
func New(cfg Config) (*Suggester, error) {
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

	return &Suggester{client: openaisdk.NewClient(opts...), model: model}, nil
}

// NewFromEnv is the one explicit convenience constructor that reads
// configuration from the environment (OPENAI_API_KEY, and optionally
// OPENAI_MODEL/OPENAI_BASE_URL) — domain logic itself never does this; only
// this named, opt-in entry point does. Returns an error (never a
// partially-usable Suggester) if OPENAI_API_KEY is unset.
func NewFromEnv() (*Suggester, error) {
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

// Suggest implements ai.Suggester. It sends req as a single structured chat
// completion request (see buildChatParams) covering the whole bounded
// candidate batch, and parses the model's structured-output JSON into an
// ai.Response. It never logs req's contents, the API key, or the raw
// provider response by default.
//
// Respects ctx cancellation/deadlines directly; does not retry internally.
func (s *Suggester) Suggest(ctx context.Context, req ai.Request) (ai.Response, error) {
	params := buildChatParams(s.model, req)

	completion, err := s.client.Chat.Completions.New(ctx, params)
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

// wrapError classifies an openai-go SDK error into ai's optional
// TimeoutError/RateLimitError interfaces where possible.
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
	_ ai.Suggester      = (*Suggester)(nil)
)
