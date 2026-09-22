// Package ai defines a provider-neutral boundary for optional AI-assisted
// classification fallback, plus the deterministic orchestration that decides
// WHEN to cross that boundary at all.
//
// Nothing here knows about OpenAI, Anthropic, or any other provider's own
// types — see Classifier. A concrete provider lives in its own adapter
// subpackage (e.g. financial/classification/ai/openai) and depends on this
// package, never the other way around, so financial/classification and every
// downstream package (review, valuation, ...) can depend on this package's
// Request/Response/Policy shapes without ever importing a provider SDK or
// needing network access/credentials to build or test.
//
// AI is strictly a FALLBACK, never a replacement, for
// financial/classification's deterministic pipeline (explicit mapping ->
// alias -> context rule -> phrase rule -> UNKNOWN). See ClassifyWithFallback
// for exactly when and how it is consulted, and the repository README's "AI
// fallback classification" section for the full architecture and the
// product rules this package enforces (closed-set codes only, mandatory
// human review, structural-row safety, privacy-minimal requests).
package ai

import (
	"context"

	"github.com/themurtez/go-valuate/financial"
)

// RequestSchemaVersion identifies the exact shape of Request this package
// sends to a Classifier and the exact shape of Response it expects back.
// Echoed on Provenance.RequestSchemaVersion so a persisted AI suggestion can
// always be traced back to the request/response contract that produced it —
// see the repository README's versioning-strategy section for the general
// rule this follows (bump only when the shape or its semantics change in a
// way that could make a historical suggestion not reproduce identically).
const RequestSchemaVersion = "1.0.0"

// AllowedCode is one canonical taxonomy code the model is permitted to
// choose, plus enough descriptive metadata for it to choose correctly.
// Built from financial.CodeMeta (see BuildAllowedCodes) rather than handing
// the provider raw financial.Code values with no explanation — a bare enum
// string gives a language model far weaker signal than a short label plus
// category.
type AllowedCode struct {
	// Code is the canonical taxonomy code (financial.IsValidCode(Code) is
	// always true for every entry this package constructs).
	Code financial.Code `json:"code"`
	// Label is the human-readable display label (financial.CodeMeta.Label).
	Label string `json:"label"`
	// Category groups the code by broad statement section
	// (financial.CodeMeta.Category), given as a plain string so this
	// package's own JSON contract never depends on financial's internal
	// enum representation changing shape.
	Category string `json:"category"`
	// StatementType is the statement this code belongs to
	// (financial.CodeMeta.StatementType), as a plain string for the same
	// reason as Category.
	StatementType string `json:"statement_type"`
}

// BuildAllowedCodes converts financial.AllCodes() into the closed set a
// Classifier is told it may choose from. Callers needing a narrower set
// (e.g. only income-statement codes for an income-statement row) may filter
// financial.AllCodes() themselves and build a Request.AllowedCodes slice by
// hand instead of using this helper — Request.AllowedCodes is deliberately
// an explicit field the orchestrator populates, not something a Classifier
// implementation derives on its own, so the closed set is always visible in
// the request payload itself (see ClassifyWithFallback).
func BuildAllowedCodes(metas []financial.CodeMeta) []AllowedCode {
	out := make([]AllowedCode, len(metas))
	for i, m := range metas {
		out[i] = AllowedCode{
			Code:          m.Code,
			Label:         m.Label,
			Category:      string(m.Category),
			StatementType: string(m.StatementType),
		}
	}
	return out
}

// ContextRow is minimal, non-identifying context about a row NEAR the row
// being classified (e.g. the row immediately above/below in the source
// statement), supplied only when it plausibly helps the model disambiguate
// (e.g. "Field Labor" appearing directly under a "Cost of Sales" heading).
// See the repository README's privacy-boundary section for exactly what is
// and is not included here: no amounts, no per-cell values, no source
// document identifiers — label and statement-section context only.
type ContextRow struct {
	// Label is the nearby row's label, exactly as it appeared.
	Label string `json:"label"`
	// ParentLabel is the nearby row's enclosing section label, if any.
	ParentLabel string `json:"parent_label,omitempty"`
}

// Request is the minimal, deterministic, privacy-conscious payload sent to
// a Classifier for ONE row. It deliberately excludes anything not needed to
// classify the row's ACCOUNT TYPE: no per-period amounts, no customer/
// account/user identity, no source document metadata, no unrelated rows
// beyond a caller-bounded ContextRows window — see the README's privacy
// section for the full "exactly what is sent" list this type is the single
// source of truth for.
type Request struct {
	// RawLabel is the row's label exactly as it appeared in the source
	// document.
	RawLabel string `json:"raw_label"`
	// NormalizedLabel is classification.NormalizeLabel(RawLabel).Comparable,
	// included so the model sees the same normalized form the deterministic
	// pipeline already matched against and failed to resolve.
	NormalizedLabel string `json:"normalized_label"`
	// ParentLabel is the row's enclosing section label, if any.
	ParentLabel string `json:"parent_label,omitempty"`
	// StatementType is the row's statement (income statement, balance
	// sheet, cash flow), as a plain string.
	StatementType string `json:"statement_type"`
	// RowKind is the row's upstream structural read
	// (financial.RowKind: ""/heading/subtotal/total), as a plain string.
	// ClassifyWithFallback does not normally send a Request at all for a
	// non-empty RowKind (see its own doc comment) — this field exists so a
	// caller-forced diagnostic request (Policy.AllowStructuralRows) still
	// tells the model the row is structural, since the model's answer is
	// discarded for normalization purposes regardless (see
	// StructuralRowResponse's doc comment).
	RowKind string `json:"row_kind,omitempty"`
	// IndustryContext is free-form business-context text the CALLER
	// explicitly supplies (e.g. "residential HVAC contractor") — this
	// package never infers or looks up industry information itself, and
	// this field is empty unless a caller sets it.
	IndustryContext string `json:"industry_context,omitempty"`
	// AllowedCodes is the CLOSED SET of canonical codes the model may
	// choose from, plus "UNKNOWN" implicitly always allowed (see
	// Response.Code's doc comment). See BuildAllowedCodes.
	AllowedCodes []AllowedCode `json:"allowed_codes"`
	// ContextRows is an optional, caller-bounded window of nearby rows for
	// disambiguation (see ContextRow). Empty unless the caller's Policy
	// requests it and supplies rows; never populated automatically from
	// unrelated parts of a larger document.
	ContextRows []ContextRow `json:"context_rows,omitempty"`
	// DeterministicResult summarizes what the deterministic pipeline
	// already concluded before falling back to AI (e.g. "UNKNOWN" or a
	// low-confidence phrase-rule match), so the model has that context too
	// and a reviewer can later compare the two (see Disagreement).
	DeterministicResult DeterministicSummary `json:"deterministic_result"`
}

// DeterministicSummary is the minimal record of what
// financial/classification.Classify concluded for a row before
// ClassifyWithFallback decided to consult AI — carried on both Request (so
// the model has it as context) and Provenance (so a reviewer can compare
// the two results later without re-running Classify). See Disagreement.
type DeterministicSummary struct {
	// Code is the deterministic pipeline's proposed code, empty for
	// SourceUnknown.
	Code financial.Code `json:"code,omitempty"`
	// Source is classification.Result.Source, copied verbatim as a plain
	// string so this package never imports classification's Source enum
	// into its own public API (mirroring review's identical reasoning for
	// its own upstream-derived payload types).
	Source string `json:"source"`
	// Confidence is classification.Result.Confidence, copied verbatim.
	Confidence float64 `json:"confidence"`
}

// Response is what a Classifier returns for one Request. Every field is
// validated by ClassifyWithFallback before being trusted — see
// ValidateResponse.
type Response struct {
	// Code is the model's proposed canonical code, or the literal string
	// "UNKNOWN" (see CodeUnknown) if the model could not confidently choose
	// one from AllowedCodes. Any other value not present in the Request's
	// AllowedCodes is rejected (see ValidateResponse) — the model may never
	// invent a code outside the closed set.
	Code financial.Code `json:"code"`
	// Alternatives lists other codes the model considered, strongest first.
	// Every entry must also be a member of the Request's AllowedCodes (or
	// CodeUnknown) — see ValidateResponse.
	Alternatives []financial.Code `json:"alternatives,omitempty"`
	// Reason is the model's short natural-language explanation for its
	// choice. Never parsed by this package's own logic — display only,
	// exactly like review.ReviewItem.Reason.
	Reason string `json:"reason,omitempty"`
	// RawConfidence is a provider-supplied heuristic strength in [0, 1], if
	// the provider returns one. This is NEVER a calibrated statistical
	// probability — see Provenance.ModelConfidence's doc comment, which
	// carries this value forward under a name that makes that explicit at
	// every call site downstream.
	RawConfidence *float64 `json:"raw_confidence,omitempty"`
	// Provider identifies which adapter produced this Response (e.g.
	// "openai"). Set by the Classifier implementation, copied onto
	// Provenance.Provider — see BuildAllowedCodes' sibling doc comments for
	// why this package keeps provider identity as plain metadata rather
	// than an enum (a new adapter never requires a change here).
	Provider string `json:"provider,omitempty"`
	// Model identifies the specific model used (e.g. "gpt-4o-mini"), when
	// the adapter supplies one.
	Model string `json:"model,omitempty"`
	// AdapterVersion identifies the provider adapter package's own version
	// — see Provenance.AdapterVersion.
	AdapterVersion string `json:"adapter_version,omitempty"`
}

// CodeUnknown is the literal Response.Code value a Classifier returns when
// it cannot confidently propose any canonical code — the AI-fallback
// equivalent of classification.SourceUnknown. Distinct from the empty
// string so a Response with an accidentally-unset Code is caught by
// ValidateResponse as malformed rather than silently treated as "unknown."
const CodeUnknown financial.Code = "UNKNOWN"

// Classifier is the provider-neutral AI classification boundary. Exactly
// one Request in, exactly one Response (or an error) out — no batching
// contract at this level; ClassifyWithFallback owns sequencing/limits for a
// whole []financial.RawLineItem (see its own doc comment), so a Classifier
// implementation only ever needs to handle one row at a time and stays
// simple to implement/fake/test.
//
// Implementations must not depend on anything outside ctx/req to produce a
// Response — no hidden global client state, no reading files/env vars
// inside Classify itself (a convenience constructor MAY read an env var to
// build a Classifier value, but Classify itself must not) — so a Classifier
// is safe to construct once and reuse concurrently across goroutines if the
// underlying provider client supports that (this package places no
// requirement either way; see each adapter's own concurrency doc comment).
type Classifier interface {
	// Classify proposes a classification for req, or returns a non-nil
	// error if the provider call itself failed (timeout, rate limit,
	// transport error, ...) — as opposed to the provider successfully
	// responding with an answer this package then rejects as invalid (see
	// ValidateResponse), which is NOT an error return from Classify itself.
	// Classify must respect ctx cancellation/deadlines.
	Classify(ctx context.Context, req Request) (Response, error)
}
