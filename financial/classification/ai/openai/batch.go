package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	openaisdk "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification/ai"
)

// isNonFinite reports whether f is NaN or +/-Inf — a value ai.ValidateResponse
// would reject regardless, checked here too so a single malformed row's
// confidence is caught and isolated (see toAIResponse) before ever reaching
// that shared validation, consistent with this adapter's other structural
// sanity checks on the raw wire item.
func isNonFinite(f float64) bool {
	return math.IsNaN(f) || math.IsInf(f, 0)
}

// DefaultMaxBatchCharacters is Config.MaxBatchCharacters' default when zero
// — a deterministic, conservative estimate/limit on the user-message payload
// size for one provider call (section 10 of the task brief), not an exact
// token count. Chosen generously enough that a typical ai.DefaultMaxBatchSize
// (20) batch of ordinary line-item labels never gets needlessly split, while
// still guarding against an unbounded prompt if a caller configures a much
// larger Policy.MaxBatchSize or supplies unusually long labels/context.
const DefaultMaxBatchCharacters = 24000

// batchRow is the wire shape of one entry in a batch request's "rows" array
// — req's own fields (section 7's privacy boundary applies identically to
// each row) plus a deterministic id this adapter assigns so the model's
// per-row result can be mapped back to req's original slice position.
// Deliberately excludes AllowedCodes/DeterministicResult's full shape from
// being repeated in the doc comment — see batchRowFromRequest.
type batchRow struct {
	ID                  string                  `json:"id"`
	RawLabel            string                  `json:"raw_label"`
	NormalizedLabel     string                  `json:"normalized_label"`
	ParentLabel         string                  `json:"parent_label,omitempty"`
	StatementType       string                  `json:"statement_type"`
	RowKind             string                  `json:"row_kind,omitempty"`
	IndustryContext     string                  `json:"industry_context,omitempty"`
	ContextRows         []ai.ContextRow         `json:"context_rows,omitempty"`
	DeterministicResult ai.DeterministicSummary `json:"deterministic_result"`
}

// batchRowFromRequest converts req into its wire row, tagged with id. Never
// includes req.AllowedCodes on the row itself — the closed set is sent once
// at the batch level (batchRequestPayload.AllowedCodes) rather than repeated
// per row, keeping the request compact; buildBatchChatParams computes that
// shared set as the union of every row's own AllowedCodes, and
// parseBatchResponse still validates each row's result against that row's
// OWN AllowedCodes (never the wider union), so this is purely a wire-size
// optimization with no relaxation of the closed-set guarantee.
func batchRowFromRequest(id string, req ai.Request) batchRow {
	return batchRow{
		ID:                  id,
		RawLabel:            req.RawLabel,
		NormalizedLabel:     req.NormalizedLabel,
		ParentLabel:         req.ParentLabel,
		StatementType:       req.StatementType,
		RowKind:             req.RowKind,
		IndustryContext:     req.IndustryContext,
		ContextRows:         req.ContextRows,
		DeterministicResult: req.DeterministicResult,
	}
}

// batchRequestPayload is the deterministic JSON payload sent as the user
// message for a batch call — see promptBatchUserPayload.
type batchRequestPayload struct {
	Rows         []batchRow       `json:"rows"`
	AllowedCodes []ai.AllowedCode `json:"allowed_codes"`
}

// batchResultItem mirrors the JSON Schema handed to the model for one
// result entry (section 2's conceptual response shape).
type batchResultItem struct {
	ID           string   `json:"id"`
	Code         string   `json:"code"`
	Alternatives []string `json:"alternatives"`
	Reason       string   `json:"reason"`
	Confidence   *float64 `json:"confidence"`
}

// batchResponseEnvelope is this adapter's own private decoding target for
// the model's structured batch output — kept separate from
// batchResultItem/ai.Response for the same reason structuredResponse is kept
// separate in adapter.go: a future change to ai.Response's JSON tags must
// never silently change what shape this adapter demands from the model.
type batchResponseEnvelope struct {
	Results []batchResultItem `json:"results"`
}

// unionAllowedCodes merges every row's AllowedCodes into one de-duplicated
// slice (stable order: first-seen, by iterating rows and each row's own
// AllowedCodes in order) — the set shown to the model at the batch level.
// Deduplication and the deterministic input order this depends on (rows are
// always processed in their original slice order) mean this never depends on
// map iteration order for its OUTPUT order, even though a map is used
// internally to detect duplicates.
func unionAllowedCodes(reqs []ai.Request) []ai.AllowedCode {
	seen := make(map[financial.Code]bool)
	var out []ai.AllowedCode
	for _, req := range reqs {
		for _, c := range req.AllowedCodes {
			if seen[c.Code] {
				continue
			}
			seen[c.Code] = true
			out = append(out, c)
		}
	}
	return out
}

// buildBatchChatParams deterministically constructs the chat-completion
// request for a whole chunk of reqs, tagging each row with a positional id
// ("row-0", "row-1", ...) derived purely from its index in reqs — this id
// never leaves this adapter; ai.Request/ai.Response carry no row-identity
// field, so parseBatchResponse uses it only to map a result back to reqs'
// own slice position before translating into the plain []ai.Response this
// adapter's ClassifyBatch returns.
func buildBatchChatParams(model string, reqs []ai.Request) openaisdk.ChatCompletionNewParams {
	rows := make([]batchRow, len(reqs))
	for i, req := range reqs {
		rows[i] = batchRowFromRequest(batchRowID(i), req)
	}
	payload := batchRequestPayload{Rows: rows, AllowedCodes: unionAllowedCodes(reqs)}

	data, _ := json.Marshal(payload)
	var user []byte
	user = append(user, "Classify each of these line items:\n"...)
	user = append(user, data...)

	return openaisdk.ChatCompletionNewParams{
		Model: shared.ChatModel(model),
		Messages: []openaisdk.ChatCompletionMessageParamUnion{
			openaisdk.SystemMessage(batchSystemInstruction),
			openaisdk.UserMessage(string(user)),
		},
		Temperature:    openaisdk.Opt(0.0),
		Store:          openaisdk.Opt(false),
		ResponseFormat: buildBatchResponseFormat(),
	}
}

// batchRowID returns the deterministic, purely-positional row identifier
// this adapter assigns for index i within one batch chat request.
func batchRowID(i int) string { return fmt.Sprintf("row-%d", i) }

// batchSystemInstruction extends systemInstruction (adapter.go) for the
// batch case: same closed-set/no-figures rules, plus the explicit
// one-result-per-row/no-invented-ids contract batch mode additionally
// requires.
const batchSystemInstruction = `You are a financial statement line-item classifier.

You will be given SEVERAL line items from financial statements (income statements, balance sheets, or cash flow statements) that a deterministic rule-based classifier could not confidently map to canonical account codes.

Your task: for EACH row in "rows", choose exactly ONE canonical code from the "allowed_codes" list that best matches that row's label and context.

Rules:
- Return exactly one result per input row, using that row's own "id" value unchanged. Never omit a row, never invent a new id, never duplicate an id.
- For each row, you MUST choose a code from "allowed_codes", or the literal string "UNKNOWN" if you are not reasonably confident any of them fit that row.
- NEVER invent a code that is not in "allowed_codes".
- NEVER calculate, estimate, or infer any financial amount, total, or valuation figure. You are choosing a category label only, for each row independently.
- Base each row's answer only on that row's own label, parent section, statement type, and any nearby context rows given to you — do not assume information that was not provided, and do not let one row's content influence another row's classification.
- Respond ONLY with the structured JSON object requested. Do not include any other text.`

// buildBatchResponseFormat returns the strict JSON Schema Structured
// Outputs configuration matching batchResponseEnvelope's shape exactly: a
// "results" array of {id, code, alternatives, reason, confidence} objects.
// "strict: true" makes the API itself guarantee schema adherence for every
// item in the array, not just the envelope.
func buildBatchResponseFormat() openaisdk.ChatCompletionNewParamsResponseFormatUnion {
	resultItemSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "The id of the row this result answers, copied verbatim from the request.",
			},
			"code": map[string]any{
				"type":        "string",
				"description": "The chosen canonical code from allowed_codes for this row, or the literal string UNKNOWN.",
			},
			"alternatives": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Other candidate codes considered for this row, strongest first. Must also be from allowed_codes.",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "A short explanation for this row's choice.",
			},
			"confidence": map[string]any{
				"type":        []string{"number", "null"},
				"description": "A self-reported confidence in [0, 1] for this row, or null if not applicable.",
			},
		},
		"required":             []string{"id", "code", "alternatives", "reason", "confidence"},
		"additionalProperties": false,
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"results": map[string]any{
				"type":        "array",
				"items":       resultItemSchema,
				"description": "Exactly one result per input row, in any order, each id matching a request row's id exactly once.",
			},
		},
		"required":             []string{"results"},
		"additionalProperties": false,
	}

	return openaisdk.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
			JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "line_item_batch_classification",
				Strict: openaisdk.Opt(true),
				Schema: schema,
			},
		},
	}
}

// splitBatchByCharacterBudget partitions reqs into ordered, contiguous
// chunks so that no chunk's estimated request-message size exceeds
// maxChars, without depending on map iteration and without ever reordering
// reqs (section 3/10's ordering and size-guard requirements). A single row
// whose own estimate already exceeds maxChars is never dropped — it forms
// its own one-row chunk rather than being silently sent unbounded or
// discarded, since a hard per-row failure here would contradict section 7's
// "one malformed row must not silently contaminate another" isolation goal
// (that row still gets a real provider attempt; only the BATCHING is
// affected, not eligibility).
func splitBatchByCharacterBudget(reqs []ai.Request, maxChars int) [][]ai.Request {
	if maxChars <= 0 {
		maxChars = DefaultMaxBatchCharacters
	}
	if len(reqs) == 0 {
		return nil
	}

	var chunks [][]ai.Request
	start := 0
	chunkChars := 0
	for i, req := range reqs {
		rowChars := estimateRequestChars(req)
		if i > start && chunkChars+rowChars > maxChars {
			chunks = append(chunks, reqs[start:i])
			start = i
			chunkChars = 0
		}
		chunkChars += rowChars
	}
	chunks = append(chunks, reqs[start:])
	return chunks
}

// estimateRequestChars returns a deterministic, cheap-to-compute size
// estimate (marshaled JSON byte length) for one Request's contribution to a
// batch prompt — not an exact token count (section 10 explicitly does not
// require one), just a stable, monotonic proxy good enough to bound prompt
// size.
func estimateRequestChars(req ai.Request) int {
	data, _ := json.Marshal(req)
	return len(data)
}

// ClassifyBatch implements ai.BatchClassifier. It classifies every entry in
// reqs, always returning same-length results/errs slices with
// results[i]/errs[i] corresponding to reqs[i] — see ai.BatchClassifier's own
// doc comment for the exact guarantee this method must uphold regardless of
// how many underlying provider calls it makes.
//
// reqs is first split deterministically by estimateRequestChars/
// Config.MaxBatchCharacters (never by map iteration) so no single provider
// call's prompt grows unbounded; ai.ClassifyBatchWithFallback has already
// bounded len(reqs) to at most Policy.MaxBatchSize before calling this
// method (see ai/batch.go's classifyMany), so this is an ADDITIONAL,
// character-based guard layered under that row-count guard, not a
// replacement for it.
//
// A provider-call failure for one character-bounded sub-chunk (timeout,
// rate limit, transport error, malformed JSON envelope) is isolated to
// exactly the rows in that sub-chunk — every other sub-chunk is still
// attempted. Respects ctx cancellation between sub-chunks and within each
// underlying call.
func (c *Classifier) ClassifyBatch(ctx context.Context, reqs []ai.Request) ([]ai.Response, []error) {
	results := make([]ai.Response, len(reqs))
	errs := make([]error, len(reqs))
	if len(reqs) == 0 {
		return results, errs
	}

	offset := 0
	for _, chunk := range splitBatchByCharacterBudget(reqs, c.maxBatchCharacters) {
		chunkResults, chunkErrs := c.classifyOneChatBatch(ctx, chunk)
		copy(results[offset:offset+len(chunk)], chunkResults)
		copy(errs[offset:offset+len(chunk)], chunkErrs)
		offset += len(chunk)
	}
	return results, errs
}

// classifyOneChatBatch issues exactly one provider chat-completion call for
// chunk and parses its structured batch response. On any failure that
// prevents the envelope itself from being trusted (provider error, no
// choices, empty content, invalid JSON) every row in chunk gets the same
// wrapped error — a whole-envelope failure cannot be attributed to one row
// more than another. Once a valid envelope IS obtained, per-row validation
// isolates each row independently (see parseBatchResponse).
func (c *Classifier) classifyOneChatBatch(ctx context.Context, chunk []ai.Request) ([]ai.Response, []error) {
	results := make([]ai.Response, len(chunk))
	errs := make([]error, len(chunk))

	params := buildBatchChatParams(c.model, chunk)
	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		wrapped := wrapError(err)
		for i := range chunk {
			errs[i] = wrapped
		}
		return results, errs
	}
	if len(completion.Choices) == 0 {
		envErr := errors.New("openai: batch response contained no choices")
		for i := range chunk {
			errs[i] = envErr
		}
		return results, errs
	}

	content := completion.Choices[0].Message.Content
	if content == "" {
		envErr := errors.New("openai: batch response message had no content")
		for i := range chunk {
			errs[i] = envErr
		}
		return results, errs
	}

	var envelope batchResponseEnvelope
	if jsonErr := json.Unmarshal([]byte(content), &envelope); jsonErr != nil {
		envErr := fmt.Errorf("openai: could not parse batch structured output as JSON: %w", jsonErr)
		for i := range chunk {
			errs[i] = envErr
		}
		return results, errs
	}

	meta := providerMeta{provider: providerName, model: completion.Model, adapterVersion: AdapterVersion}
	return parseBatchResponse(chunk, envelope, meta)
}

// providerMeta bundles the per-call provider/model/adapter metadata every
// successfully-parsed row's ai.Response is stamped with, exactly like
// Classify's single-row path does inline.
type providerMeta struct {
	provider       string
	model          string
	adapterVersion string
}

// parseBatchResponse maps envelope's results back onto chunk's original
// positions by id, enforcing section 2's per-row contract:
//
//   - every result id must match exactly one row in chunk (batchRowID(i) for
//     that row's index i in chunk)
//   - no duplicate result ids
//   - no unknown result ids (an id not present in chunk)
//   - every row in chunk must receive exactly one result (missing ids are a
//     per-row error, not a whole-batch failure)
//
// A malformed individual result (duplicate id, unknown id target, or a code
// that fails ai.ValidateResponse against THAT row's own AllowedCodes) never
// affects any other row: it becomes that one row's error (so
// ai.resolveOutcome downstream leaves it on its deterministic/UNKNOWN
// result with a structured Issue), while every other valid row's Response
// is still returned normally. This is the concrete mechanism behind section
// 7's "one malformed row must not silently contaminate another."
func parseBatchResponse(chunk []ai.Request, envelope batchResponseEnvelope, meta providerMeta) ([]ai.Response, []error) {
	results := make([]ai.Response, len(chunk))
	errs := make([]error, len(chunk))

	idToIndex := make(map[string]int, len(chunk))
	for i := range chunk {
		idToIndex[batchRowID(i)] = i
	}

	seen := make(map[string]bool, len(envelope.Results))
	for _, item := range envelope.Results {
		idx, ok := idToIndex[item.ID]
		if !ok {
			// Section 2: "Reject ... unknown result IDs." There is no chunk
			// row this result could attach to, so it is discarded outright
			// (not attributable to any specific row's errs slot) rather than
			// guessed at.
			continue
		}
		if seen[item.ID] {
			// Section 2: "Reject duplicate result IDs." The row keeps
			// whatever the FIRST occurrence produced (or its
			// missing-result error, if the first occurrence was itself
			// invalid) — a later duplicate can only make an already-decided
			// row worse, never better, so it is ignored rather than
			// overwriting a valid result.
			continue
		}
		seen[item.ID] = true

		resp, respErr := item.toAIResponse(meta)
		if respErr != nil {
			errs[idx] = respErr
			continue
		}
		results[idx] = resp
	}

	for i := range chunk {
		id := batchRowID(i)
		if !seen[id] {
			// Section 2: "Reject ... missing required result IDs." Isolated
			// to this row only — every other row in the chunk that DID get a
			// valid result is unaffected.
			errs[i] = fmt.Errorf("openai: batch response did not include a result for row id %q", id)
		}
	}

	return results, errs
}

// toAIResponse converts one wire batchResultItem into an ai.Response, or a
// non-nil error if the item itself is structurally malformed in a way this
// adapter can catch before ever handing it to ai.ValidateResponse (an empty
// code, or a non-finite confidence — mirroring adapter.go's single-row
// parsing strictness so batch mode never accepts something single-row mode
// would have rejected). Provider/model/adapter metadata is stamped uniformly
// via meta; per-row closed-set validation against that row's own
// AllowedCodes still happens one layer up, in
// ai.ClassifyBatchWithFallback/ValidateResponse, exactly as it does for the
// single-row path — this method never duplicates that check, only the
// structural sanity a malformed id/JSON item could violate.
func (item batchResultItem) toAIResponse(meta providerMeta) (ai.Response, error) {
	if item.Code == "" {
		return ai.Response{}, fmt.Errorf("openai: batch result for row id %q had no code set", item.ID)
	}
	if item.Confidence != nil {
		if isNonFinite(*item.Confidence) {
			return ai.Response{}, fmt.Errorf("openai: batch result for row id %q had a non-finite confidence value", item.ID)
		}
	}

	alts := make([]financial.Code, 0, len(item.Alternatives))
	for _, a := range item.Alternatives {
		alts = append(alts, financial.Code(a))
	}

	return ai.Response{
		Code:           financial.Code(item.Code),
		Alternatives:   alts,
		Reason:         item.Reason,
		RawConfidence:  item.Confidence,
		Provider:       meta.provider,
		Model:          meta.model,
		AdapterVersion: meta.adapterVersion,
	}, nil
}

var _ ai.BatchClassifier = (*Classifier)(nil)
