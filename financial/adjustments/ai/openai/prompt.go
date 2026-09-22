package openai

import (
	"encoding/json"
	"os"
	"strings"

	openaisdk "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/adjustments/ai"
)

// osGetenv is the one place this package reads the environment (only from
// NewFromEnv, never from Suggest or buildChatParams) — kept as a tiny
// indirection so "domain logic never reads env vars" is enforced by
// construction: no other file in this package imports "os".
func osGetenv(key string) string { return os.Getenv(key) }

// buildChatParams deterministically constructs the chat-completion request
// for req: a fixed system instruction plus a single JSON user message
// containing exactly req's own fields — never any broader application
// state. Temperature is pinned to 0, and Store is left false so OpenAI does
// not retain this request for model-distillation/eval products.
func buildChatParams(model string, req ai.Request) openaisdk.ChatCompletionNewParams {
	return openaisdk.ChatCompletionNewParams{
		Model: shared.ChatModel(model),
		Messages: []openaisdk.ChatCompletionMessageParamUnion{
			openaisdk.SystemMessage(systemInstruction),
			openaisdk.UserMessage(promptUserPayload(req)),
		},
		Temperature:    openaisdk.Opt(0.0),
		Store:          openaisdk.Opt(false),
		ResponseFormat: buildResponseFormat(),
	}
}

// systemInstruction is the fixed task definition sent on every request. It
// states the closed-set adjustment-type contract, the hard "never invent an
// amount" rule (every suggestion's amount must be copied verbatim from the
// candidate row it targets), and the REQUIRES_USER_INPUT escape hatch for
// owner-compensation/related-party-rent normalization, where this system
// forbids the model from inventing a replacement benchmark value.
const systemInstruction = `You are a financial-statement normalization assistant helping identify candidate add-back/adjustment SUGGESTIONS for a business valuation. You do NOT perform valuation, you do NOT calculate multiples or DCF, and you never make a final decision — every suggestion you produce will be reviewed and explicitly accepted or rejected by a human before it has any effect.

You will be given a bounded list of "candidates": already-confirmed financial statement line items, each with a row id, period, label, canonical classification code (if known), and its EXACT reported amount.

Your task: for each candidate that plausibly warrants a normalization adjustment, propose AT MOST ONE suggestion per (row, adjustment type) pair.

HARD RULES — violating any of these makes your entire response useless and will be discarded:
1. You MUST choose adjustment_type from the "allowed_types" list. Never invent a type not in that list.
2. The "amount" field of every suggestion MUST be copied EXACTLY from that candidate row's own "amount" field in the request. Never estimate, round, scale, or invent an amount. If you cannot point to a candidate row with that exact amount, do not produce a suggestion at all.
3. Never invent a replacement salary, market rent, or any other benchmark figure. If a suggestion is about owner compensation appearing potentially above/below a market rate, or related-party rent appearing above/below fair-market rent, set "requires_user_input": true and explain what benchmark a human needs to supply — do NOT propose what that benchmark should be.
4. "direction" must be "INCREASE_EARNINGS" if applying the adjustment would raise normalized earnings (e.g. adding back a personal/non-recurring expense), or "DECREASE_EARNINGS" if it would lower normalized earnings (e.g. removing non-operating income or an unusual gain).
5. Do not propose a suggestion for a row that is not in "candidates". Do not propose more than one suggestion for the same (row id, period, adjustment type) combination.
6. Only suggest an adjustment when the label/classification/context plausibly supports it. If nothing in the candidates warrants a suggestion, return an empty "suggestions" array — this is a normal, good outcome, not a failure.
7. Respond ONLY with the structured JSON object requested. Do not include any other text.`

// promptUserPayload builds the deterministic JSON payload sent as the user
// message: exactly req's own fields, marshaled with Go's default (sorted)
// map key order so the same Request always produces byte-identical prompt
// text.
func promptUserPayload(req ai.Request) string {
	data, _ := json.Marshal(req)
	var b strings.Builder
	b.WriteString("Evaluate these candidate line items for possible normalization adjustments:\n")
	b.Write(data)
	return b.String()
}

// structuredResponse mirrors the JSON Schema handed to the model (see
// buildResponseFormat) — kept as this adapter's own private decoding target
// rather than unmarshaling straight into ai.Response, so a future change to
// ai.Response's own JSON tags can never silently change what shape this
// adapter demands from the model.
type structuredResponse struct {
	Suggestions []structuredSuggestion `json:"suggestions"`
}

type structuredSuggestion struct {
	SourceRowID       string   `json:"source_row_id"`
	Period            string   `json:"period"`
	AdjustmentType    string   `json:"adjustment_type"`
	Amount            float64  `json:"amount"`
	Direction         string   `json:"direction"`
	Reason            string   `json:"reason"`
	Confidence        *float64 `json:"confidence"`
	RequiresUserInput bool     `json:"requires_user_input"`
}

func (s structuredResponse) toAIResponse() ai.Response {
	suggestions := make([]ai.Suggestion, len(s.Suggestions))
	for i, item := range s.Suggestions {
		suggestions[i] = ai.Suggestion{
			SourceRowID:       item.SourceRowID,
			Period:            financial.Period(item.Period),
			AdjustmentType:    adjustments.Type(item.AdjustmentType),
			Amount:            item.Amount,
			Direction:         ai.Direction(item.Direction),
			Reason:            item.Reason,
			Confidence:        item.Confidence,
			RequiresUserInput: item.RequiresUserInput,
		}
	}
	return ai.Response{Suggestions: suggestions}
}

// buildResponseFormat returns the strict JSON Schema Structured Outputs
// configuration matching structuredResponse's shape exactly. "strict: true"
// makes the API itself guarantee schema adherence rather than this adapter
// needing to tolerate free-text drift.
func buildResponseFormat() openaisdk.ChatCompletionNewParamsResponseFormatUnion {
	suggestionSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source_row_id": map[string]any{
				"type":        "string",
				"description": "The row_id of the candidate this suggestion targets, copied verbatim from the request.",
			},
			"period": map[string]any{
				"type":        "string",
				"description": "The period of the candidate this suggestion targets, copied verbatim from the request.",
			},
			"adjustment_type": map[string]any{
				"type":        "string",
				"description": "The chosen adjustment type from allowed_types.",
			},
			"amount": map[string]any{
				"type":        "number",
				"description": "MUST exactly equal the referenced candidate row's own amount field. Never invented, estimated, or rounded.",
			},
			"direction": map[string]any{
				"type":        "string",
				"enum":        []string{"INCREASE_EARNINGS", "DECREASE_EARNINGS"},
				"description": "Whether applying this adjustment would increase or decrease normalized earnings.",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "A short explanation for the suggestion.",
			},
			"confidence": map[string]any{
				"type":        []string{"number", "null"},
				"description": "A self-reported confidence in [0, 1], or null if not applicable.",
			},
			"requires_user_input": map[string]any{
				"type":        "boolean",
				"description": "True when this suggestion needs a human-supplied benchmark value (e.g. replacement salary, market rent) that you must not invent.",
			},
		},
		"required":             []string{"source_row_id", "period", "adjustment_type", "amount", "direction", "reason", "confidence", "requires_user_input"},
		"additionalProperties": false,
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"suggestions": map[string]any{
				"type":        "array",
				"items":       suggestionSchema,
				"description": "Zero or more proposed adjustment suggestions. Empty is a valid, normal response.",
			},
		},
		"required":             []string{"suggestions"},
		"additionalProperties": false,
	}

	return openaisdk.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
			JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "adjustment_suggestions",
				Strict: openaisdk.Opt(true),
				Schema: schema,
			},
		},
	}
}
