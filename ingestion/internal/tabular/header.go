package tabular

// DetectedColumn is one column recognized as a reporting-period column
// during header detection.
type DetectedColumn struct {
	ColumnIndex int
	Period      ParsedPeriod
}

// HeaderDetection is the outcome of scanning a grid for the row that
// contains period headers and the columns that represent periods.
type HeaderDetection struct {
	// RowIndex is the 0-based row chosen as the header row. -1 if none
	// could be identified.
	RowIndex int
	// Columns lists every column recognized as a reporting period, in
	// column order. Always excludes the label column (see
	// DetectLabelColumn).
	Columns []DetectedColumn
	// CandidateRows lists every row index that looked like a plausible
	// header row (had at least one cell that parsed as a period with
	// Confidence > 0), for MULTIPLE_POSSIBLE_HEADER_ROWS reporting. Always
	// includes RowIndex when RowIndex >= 0.
	CandidateRows []int
}

// scanLimit bounds how many leading rows are scanned for a header row,
// since financial statements conventionally put periods within the first
// handful of rows (title rows, possibly a blank separator, then the
// header). Scanning the whole grid would risk matching an ordinary data
// row whose label happens to look like a period (e.g. a "2024 Bonus
// Accrual" line item).
const headerScanLimit = 15

// DetectHeaderRow scans the first headerScanLimit rows of g (excluding
// labelCol) for the row with the most cells that parse as periods via
// ParsePeriodLabel with Confidence > 0. Ties are broken by preferring the
// later row (financial statements more often have a title row before the
// real header than the reverse), then by lowest row index for full
// determinism if a later tiebreak is ever needed.
func DetectHeaderRow(g Grid, labelCol int) HeaderDetection {
	limit := g.RowCount()
	if limit > headerScanLimit {
		limit = headerScanLimit
	}

	type rowScore struct {
		rowIndex int
		periods  []DetectedColumn
	}
	var scored []rowScore

	for r := 0; r < limit; r++ {
		if g.IsRowBlank(r) {
			continue
		}
		var cols []DetectedColumn
		for c := 0; c < g.ColCount(r); c++ {
			if c == labelCol {
				continue
			}
			text := g.Cell(r, c)
			if text == "" {
				continue
			}
			p := ParsePeriodLabel(text)
			if p.Confidence > 0 {
				cols = append(cols, DetectedColumn{ColumnIndex: c, Period: p})
			}
		}
		if len(cols) >= 1 {
			scored = append(scored, rowScore{rowIndex: r, periods: cols})
		}
	}

	if len(scored) == 0 {
		return HeaderDetection{RowIndex: -1}
	}

	best := scored[0]
	for _, s := range scored[1:] {
		if len(s.periods) >= len(best.periods) {
			best = s
		}
	}

	candidates := make([]int, 0, len(scored))
	for _, s := range scored {
		candidates = append(candidates, s.rowIndex)
	}

	return HeaderDetection{
		RowIndex:      best.rowIndex,
		Columns:       best.periods,
		CandidateRows: candidates,
	}
}

// DetectLabelColumn identifies which column most likely holds line-item
// labels: the column, among the first few, with the highest count of
// non-empty, non-purely-numeric cells across the grid's data rows. Ties
// prefer the lowest column index (label columns are conventionally
// leftmost).
func DetectLabelColumn(g Grid) int {
	maxCols := g.MaxColCount()
	scanCols := maxCols
	const labelColScanLimit = 6
	if scanCols > labelColScanLimit {
		scanCols = labelColScanLimit
	}
	if scanCols == 0 {
		return 0
	}

	rows := g.RowCount()
	bestCol := 0
	bestScore := -1
	for c := 0; c < scanCols; c++ {
		score := 0
		for r := 0; r < rows; r++ {
			text := g.TrimmedCell(r, c)
			if text == "" {
				continue
			}
			num := ParseNumeric(text, DashAsBlank)
			if num.Parsed || num.IsDash {
				continue
			}
			score++
		}
		if score > bestScore {
			bestScore = score
			bestCol = c
		}
	}
	return bestCol
}
