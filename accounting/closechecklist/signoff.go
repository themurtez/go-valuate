package closechecklist

import (
	"sort"
	"time"
)

// SignOffRole is the fixed set of sign-off roles this package recognizes
// for ReviewPolicy purposes. A caller may still record a SignOff with an
// arbitrary Role string (opaque, like ActorRef) — this list only
// enumerates the roles ReviewPolicy's fixed rules understand.
type SignOffRole string

const (
	SignOffPreparer   SignOffRole = "PREPARER"
	SignOffReviewer   SignOffRole = "REVIEWER"
	SignOffApprover   SignOffRole = "APPROVER"
	SignOffController SignOffRole = "CONTROLLER"
)

// SignOff is one recorded sign-off/review action on a task. ActorRef is
// opaque — this package performs no user/identity management.
type SignOff struct {
	SignOffID string      `json:"sign_off_id"`
	Role      SignOffRole `json:"role"`
	ActorRef  string      `json:"actor_ref"`
	SignedAt  time.Time   `json:"signed_at"`
	Note      string      `json:"note,omitempty"`
}

// ReviewPolicyType is the fixed set of review/sign-off requirement kinds
// a task may declare.
type ReviewPolicyType string

const (
	ReviewNone                ReviewPolicyType = "NONE"
	ReviewPreparerOnly        ReviewPolicyType = "PREPARER_ONLY"
	ReviewPreparerAndReviewer ReviewPolicyType = "PREPARER_AND_REVIEWER"
	ReviewSpecificRoles       ReviewPolicyType = "SPECIFIC_ROLES"
)

func isRecognizedReviewPolicyType(t ReviewPolicyType) bool {
	switch t {
	case "", ReviewNone, ReviewPreparerOnly, ReviewPreparerAndReviewer, ReviewSpecificRoles:
		return true
	default:
		return false
	}
}

// ReviewPolicy declares one task's sign-off requirement. The zero value
// (empty Type) is treated as NONE. RequireDistinctActors, combined with
// PREPARER_AND_REVIEWER or SPECIFIC_ROLES, requires that no single
// ActorRef fill two of the required roles (see SIGNOFF_ROLE_CONFLICT,
// section 9).
type ReviewPolicy struct {
	Type                  ReviewPolicyType `json:"type,omitempty"`
	RequiredRoles         []SignOffRole    `json:"required_roles,omitempty"`
	RequireDistinctActors bool             `json:"require_distinct_actors,omitempty"`
}

// requiredRoles returns the concrete role set p requires, given its Type.
func (p ReviewPolicy) requiredRoles() []SignOffRole {
	switch p.Type {
	case ReviewPreparerOnly:
		return []SignOffRole{SignOffPreparer}
	case ReviewPreparerAndReviewer:
		return []SignOffRole{SignOffPreparer, SignOffReviewer}
	case ReviewSpecificRoles:
		return p.RequiredRoles
	default:
		return nil
	}
}

// reviewEvaluation is the result of evaluating one task's ReviewPolicy
// against its recorded SignOffs.
type reviewEvaluation struct {
	satisfied     bool
	missingRoles  []SignOffRole
	roleConflict  bool
	conflictActor string
	conflictRoles []SignOffRole
}

// evaluateReview checks p against signOffs. Role-conflict detection
// (RequireDistinctActors) runs independently of satisfaction: a
// satisfied review (every required role has a distinct sign-off) can
// still, in principle, never conflict by construction, but this function
// keeps the checks structurally separate for clarity and so a caller
// with RequireDistinctActors=false never sees a spurious conflict.
func evaluateReview(p ReviewPolicy, signOffs []SignOff) reviewEvaluation {
	required := p.requiredRoles()
	if len(required) == 0 {
		return reviewEvaluation{satisfied: true}
	}

	byRole := make(map[SignOffRole][]SignOff)
	for _, s := range signOffs {
		byRole[s.Role] = append(byRole[s.Role], s)
	}

	var missing []SignOffRole
	for _, r := range required {
		if len(byRole[r]) == 0 {
			missing = append(missing, r)
		}
	}

	eval := reviewEvaluation{satisfied: len(missing) == 0, missingRoles: missing}

	if p.RequireDistinctActors && len(missing) == 0 {
		actorRoles := make(map[string][]SignOffRole)
		for _, r := range required {
			for _, s := range byRole[r] {
				actorRoles[s.ActorRef] = append(actorRoles[s.ActorRef], r)
			}
		}
		actors := make([]string, 0, len(actorRoles))
		for actor := range actorRoles {
			actors = append(actors, actor)
		}
		sort.Strings(actors)
		for _, actor := range actors {
			distinctRoles := dedupRoles(actorRoles[actor])
			if len(distinctRoles) > 1 {
				eval.roleConflict = true
				eval.conflictActor = actor
				eval.conflictRoles = distinctRoles
				eval.satisfied = false
				break
			}
		}
	}
	return eval
}

func dedupRoles(roles []SignOffRole) []SignOffRole {
	seen := make(map[SignOffRole]bool, len(roles))
	var out []SignOffRole
	for _, r := range roles {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}
