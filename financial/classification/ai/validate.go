package ai

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/financial"
)

// ValidateResponse checks resp against the Request that produced it,
// enforcing the closed-set contract (section 2 of the task brief): resp.Code
// must be CodeUnknown or a member of req.AllowedCodes, never anything else,
// and every entry in resp.Alternatives is held to the same rule. It also
// rejects a structurally malformed response (no Code at all, a
// RawConfidence outside [0, 1] or non-finite).
//
// Returns nil when resp is fully valid. A non-nil Issue always has
// Severity == SeverityError: an invalid AI response is never partially
// trusted — see ClassifyWithFallback, which downgrades the row to UNKNOWN
// whenever this returns non-nil.
func ValidateResponse(req Request, resp Response) *Issue {
	if resp.Code == "" {
		return &Issue{Code: IssueEmptyResponse, Severity: SeverityError,
			Message: "provider returned a response with no code set"}
	}

	allowed := allowedCodeSet(req.AllowedCodes)

	if resp.Code != CodeUnknown && !allowed[resp.Code] {
		return &Issue{Code: IssueInvalidCode, Severity: SeverityError,
			Message: fmt.Sprintf("model proposed code %q, which is not in the allowed closed set for this request", resp.Code)}
	}

	for _, alt := range resp.Alternatives {
		if alt != CodeUnknown && !allowed[alt] {
			return &Issue{Code: IssueInvalidCode, Severity: SeverityError,
				Message: fmt.Sprintf("model proposed alternative code %q, which is not in the allowed closed set for this request", alt)}
		}
	}

	if resp.RawConfidence != nil {
		c := *resp.RawConfidence
		if math.IsNaN(c) || math.IsInf(c, 0) {
			return &Issue{Code: IssueInvalidResponse, Severity: SeverityError,
				Message: fmt.Sprintf("provider confidence %v is not finite", c)}
		}
		if c < 0 || c > 1 {
			return &Issue{Code: IssueInvalidResponse, Severity: SeverityError,
				Message: fmt.Sprintf("provider confidence %v is outside [0, 1]", c)}
		}
	}

	return nil
}

func allowedCodeSet(codes []AllowedCode) map[financial.Code]bool {
	set := make(map[financial.Code]bool, len(codes))
	for _, c := range codes {
		set[c.Code] = true
	}
	return set
}
