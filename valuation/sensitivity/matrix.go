package sensitivity

// EarningsScenario is one named earnings figure to analyze in a Matrix —
// e.g. "Base Case" at the maintainable earnings figure, "Downside" at a
// caller-reduced figure, "Upside" at a caller-increased one. This package
// does not generate scenarios; the caller supplies whatever earnings
// figures its own adjustment/forecast process produced.
type EarningsScenario struct {
	// Label identifies this scenario (e.g. "Base Case", "-10% Downside").
	Label string `json:"label"`
	// Earnings is this scenario's earnings figure.
	Earnings float64 `json:"earnings"`
}

// MatrixCell is a single (earnings scenario, multiple) combination's
// result within a Matrix.
type MatrixCell struct {
	// EarningsLabel echoes the EarningsScenario.Label this cell's row
	// belongs to.
	EarningsLabel string `json:"earnings_label"`
	// Earnings echoes the EarningsScenario.Earnings this cell's row was
	// computed from.
	Earnings float64 `json:"earnings"`
	// Multiple is this cell's column multiple.
	Multiple float64 `json:"multiple"`
	// Value is Earnings * Multiple. Meaningful only when Valid is true.
	Value float64 `json:"value"`
	// Valid is false if Multiple was non-finite or <= 0.
	Valid bool `json:"valid"`
	// Reason explains why Valid is false, when it is.
	Reason string `json:"reason,omitempty"`
}

// Matrix is a 2D earnings-scenario x multiple-scenario sensitivity grid:
// one row per EarningsScenario, one column per multiple, every cell
// Earnings * Multiple.
type Matrix struct {
	// EarningsScenarios echoes the input scenarios, in row order.
	EarningsScenarios []EarningsScenario `json:"earnings_scenarios"`
	// Multiples echoes the input multiples, in column order.
	Multiples []float64 `json:"multiples"`
	// Rows holds one row per EarningsScenario, each containing one
	// MatrixCell per Multiples entry, in the same (row, column) order as
	// EarningsScenarios x Multiples — so Rows[i][j] is always
	// EarningsScenarios[i] x Multiples[j], letting a caller render this
	// directly as a table without re-deriving indices.
	Rows [][]MatrixCell `json:"rows"`
}

// EarningsMultipleMatrix computes a full earnings-scenario x multiple grid:
// for every scenario in scenarios and every multiple in multiples, a
// MatrixCell of scenario.Earnings * multiple, with the same per-cell
// invalid-multiple handling as MultipleSensitivity (a non-finite or
// non-positive multiple invalidates only the cells in its column, not the
// whole Matrix).
func EarningsMultipleMatrix(scenarios []EarningsScenario, multiples []float64) Matrix {
	rows := make([][]MatrixCell, 0, len(scenarios))
	for _, sc := range scenarios {
		row := make([]MatrixCell, 0, len(multiples))
		for _, m := range multiples {
			cell := MatrixCell{EarningsLabel: sc.Label, Earnings: sc.Earnings, Multiple: m}
			switch {
			case !isFinite(m):
				cell.Reason = "multiple is not a finite number"
			case m <= 0:
				cell.Reason = "multiple must be greater than zero"
			default:
				cell.Valid = true
				cell.Value = sc.Earnings * m
			}
			row = append(row, cell)
		}
		rows = append(rows, row)
	}
	return Matrix{EarningsScenarios: scenarios, Multiples: multiples, Rows: rows}
}
