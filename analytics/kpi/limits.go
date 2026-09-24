package kpi

// Complexity limits protect against pathological Definitions causing
// accidental stack or memory explosions — task section 29. These are
// generous, not "tiny arbitrary limits" (task section 29's explicit
// instruction): a realistic hand-authored or even generator-composed
// formula is nowhere near these ceilings; they exist purely as a circuit
// breaker.
const (
	// MaxExpressionDepth is the maximum nesting depth of one Expression
	// tree (a leaf has depth 1).
	MaxExpressionDepth = 64
	// MaxExpressionNodes is the maximum total node count (every Expression
	// in the tree, leaves and internal nodes) of one Expression tree.
	MaxExpressionNodes = 2000
	// MaxDependencyDepth is the maximum length of any KPI-to-KPI
	// dependency chain (A -> B -> C -> ...).
	MaxDependencyDepth = 200
)

// expressionShape is the depth/node-count of one Expression tree, computed
// once during validation.
type expressionShape struct {
	depth int
	nodes int
}

// measureExpression walks e and reports its depth/node count. It does not
// itself enforce MaxExpressionDepth/MaxExpressionNodes — validateExpression
// does, using this measurement — but it still bounds its own recursion by
// refusing to descend past MaxExpressionDepth+1 levels, so a maliciously
// deep tree cannot blow this function's own call stack before validation
// gets a chance to report IssueExpressionTooComplex.
func measureExpression(e Expression) expressionShape {
	return measureExpressionAt(e, 1)
}

func measureExpressionAt(e Expression, depth int) expressionShape {
	if depth > MaxExpressionDepth+1 {
		// Stop descending; the caller will see a depth exceeding the
		// limit and report IssueExpressionTooComplex without this
		// function recursing any further.
		return expressionShape{depth: depth, nodes: 1}
	}
	shape := expressionShape{depth: depth, nodes: 1}
	for _, arg := range e.Args {
		childShape := measureExpressionAt(arg, depth+1)
		if childShape.depth > shape.depth {
			shape.depth = childShape.depth
		}
		shape.nodes += childShape.nodes
		if shape.nodes > MaxExpressionNodes*4 {
			// Extra headroom beyond the real limit so one adversarial
			// tree cannot force unbounded accumulation here either;
			// validateExpression still reports the precise issue.
			return shape
		}
	}
	return shape
}
