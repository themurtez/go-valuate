package openai

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openaisdk "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"

	"github.com/themurtez/go-valuate/financial/classification/ai"
)

// osGetenv is the one place this package reads the environment (only from
// NewFromEnv, never from Classify or buildChatParams) — kept as a tiny
// indirection purely so the "domain logic never reads env vars" rule (see
// the package doc comment) is enforced by construction: no other file in
// this package imports "os".
func osGetenv(key string) string { return os.Getenv(key) }

// buildChatParams deterministically constructs the chat-completion request
// for req: a fixed system instruction plus a single JSON user message
// containing exactly req's own fields (see promptUserPayload) — never any
// broader application state, per section 7's explicit "do not send
// arbitrary application state" instruction. Temperature is pinned to 0 for
// the most reproducible behavior this task affords (classification is a
// closed-set choice, not creative generation), and Store is left false so
// OpenAI does not retain this request for model-distillation/eval products
// (see the package doc comment's "do not persist prompts/responses"
// requirement — this is this adapter's own request never asking the
// provider to retain it; it does not control the provider's general data
// retention policy, which is a matter for the caller's own account
// settings/DPA).
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

// systemInstruction is the fixed task definition sent on every request —
// see section 7: task definition, explicit instruction to return one of the
// allowed codes or UNKNOWN, and an explicit prohibition on inventing
// financial figures or valuation output, since this call's only job is a
// closed-set classification choice.
const systemInstruction = `You are a financial statement line-item classifier.

You will be given ONE line item from a financial statement (an income statement, balance sheet, or cash flow statement) that a deterministic rule-based classifier could not confidently map to a canonical account code.

Your task: choose exactly ONE canonical code from the "allowed_codes" list in the user message that best matches this line item, based on its label and context.

Rules:
- You MUST choose a code from "allowed_codes", or the literal string "UNKNOWN" if you are not reasonably confident any of them fit.
- NEVER invent a code that is not in "allowed_codes".
- NEVER calculate, estimate, or infer any financial amount, total, or valuation figure. You are choosing a category label only.
- Base your answer only on the label, parent section, statement type, and any nearby context rows given to you — do not assume information that was not provided.
- Respond ONLY with the structured JSON object requested. Do not include any other text.`

// promptUserPayload builds the deterministic JSON payload sent as the user
// message: exactly req's own fields, marshaled with sorted map keys (Go's
// default, and req has no maps besides simple string/slice fields) so the
// same Request always produces byte-identical prompt text — useful for
// reproducibility/debugging even though the model itself is not
// deterministic. Marshaling failure is not possible for this fixed,
// entirely-string/slice/float shape, so this deliberately does not return
// an error (mirrors classification.NormalizeLabel-style small deterministic
// helpers elsewhere in this repository that have no failure mode worth a
// signature change).
func promptUserPayload(req ai.Request) string {
	data, _ := json.Marshal(req)
	var b strings.Builder
	b.WriteString("Classify this line item:\n")
	b.Write(data)
	return b.String()
}

// buildResponseFormat returns the strict JSON Schema Structured Outputs
// configuration matching structuredResponse's shape exactly (section 8):
// code (string, required), alternatives (array of strings), reason
// (string), confidence (number or null). "strict: true" makes the API
// itself guarantee schema adherence rather than this adapter needing to
// tolerate free-text drift.
func buildResponseFormat() openaisdk.ChatCompletionNewParamsResponseFormatUnion {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "The chosen canonical code from allowed_codes, or the literal string UNKNOWN.",
			},
			"alternatives": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Other candidate codes considered, strongest first. Must also be from allowed_codes.",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "A short explanation for the choice.",
			},
			"confidence": map[string]any{
				"type":        []string{"number", "null"},
				"description": "A self-reported confidence in [0, 1], or null if not applicable.",
			},
		},
		"required":             []string{"code", "alternatives", "reason", "confidence"},
		"additionalProperties": false,
	}

	return openaisdk.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
			JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   "line_item_classification",
				Strict: openaisdk.Opt(true),
				Schema: schema,
			},
		},
	}
}

// describeRequestForDebug is an optional, never-automatically-invoked
// helper a caller MAY use for local debugging to print exactly what would
// be sent to the provider, without actually calling it — deliberately not
// wired into Classify itself (see the package doc comment: "do not log ...
// full financial inputs by default").
func describeRequestForDebug(req ai.Request) string {
	return fmt.Sprintf("system=%q user=%q", systemInstruction, promptUserPayload(req))
}
