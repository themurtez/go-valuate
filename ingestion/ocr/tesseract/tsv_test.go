package tesseract

import "testing"

const sampleTSV = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
	"1\t1\t0\t0\t0\t0\t0\t0\t612\t792\t-1\t\n" +
	"2\t1\t1\t0\t0\t0\t72\t100\t400\t20\t-1\t\n" +
	"3\t1\t1\t1\t0\t0\t72\t100\t400\t20\t-1\t\n" +
	"4\t1\t1\t1\t1\t0\t72\t100\t400\t20\t-1\t\n" +
	"5\t1\t1\t1\t1\t1\t72\t100\t60\t20\t95.5\tRevenue\n" +
	"5\t1\t1\t1\t1\t2\t300\t100\t80\t20\t88.2\t850,000\n"

func TestParseTSV_ExtractsWordLevelRowsOnly(t *testing.T) {
	words, err := parseTSV([]byte(sampleTSV), 0)
	if err != nil {
		t.Fatalf("parseTSV: %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("got %d words, want 2 (only level-5 rows)", len(words))
	}
	if words[0].Text != "Revenue" || words[1].Text != "850,000" {
		t.Errorf("words = %+v", words)
	}
}

func TestParseTSV_PopulatesPositionAndConfidence(t *testing.T) {
	words, err := parseTSV([]byte(sampleTSV), 3)
	if err != nil {
		t.Fatalf("parseTSV: %v", err)
	}
	w := words[0]
	if w.X != 72 || w.Y != 100 || w.Width != 60 || w.Height != 20 {
		t.Errorf("position = %+v, want X=72 Y=100 W=60 H=20", w)
	}
	if w.Confidence != 95.5 {
		t.Errorf("Confidence = %v, want 95.5", w.Confidence)
	}
	if w.PageIndex != 3 {
		t.Errorf("PageIndex = %d, want 3 (caller-supplied)", w.PageIndex)
	}
	if w.BlockNum != 1 || w.ParNum != 1 || w.LineNum != 1 {
		t.Errorf("grouping ids = block:%d par:%d line:%d, want 1/1/1", w.BlockNum, w.ParNum, w.LineNum)
	}
}

func TestParseTSV_SkipsMalformedRows(t *testing.T) {
	data := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"5\t1\t1\t1\t1\t1\tnot-enough-columns\n" +
		"5\t1\t1\t1\t1\t1\t72\t100\t60\t20\t95.5\tGood\n"
	words, err := parseTSV([]byte(data), 0)
	if err != nil {
		t.Fatalf("parseTSV: %v", err)
	}
	if len(words) != 1 || words[0].Text != "Good" {
		t.Errorf("words = %+v, want exactly one 'Good' word (malformed row skipped)", words)
	}
}

func TestParseTSV_SkipsEmptyTextRows(t *testing.T) {
	data := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"5\t1\t1\t1\t1\t1\t72\t100\t60\t20\t-1\t\n"
	words, err := parseTSV([]byte(data), 0)
	if err != nil {
		t.Fatalf("parseTSV: %v", err)
	}
	if len(words) != 0 {
		t.Errorf("words = %+v, want none (empty-text placeholder row)", words)
	}
}

func TestParseTSV_EmptyInput(t *testing.T) {
	words, err := parseTSV([]byte(""), 0)
	if err != nil {
		t.Fatalf("parseTSV: %v", err)
	}
	if len(words) != 0 {
		t.Errorf("words = %+v, want none", words)
	}
}
