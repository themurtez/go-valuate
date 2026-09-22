package csv

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/ingestion"
)

// FuzzParse proves csv.Parse never panics on arbitrary byte input — this
// is the package a real, untrusted user-uploaded file first reaches (see
// docs/INTEGRATION.md's Security / privacy section: "uploaded documents
// are untrusted input") — and that a fatal parse failure always comes
// back as a structured *ingestion.Error with a non-empty Code, never a
// crash or an ambiguous nil/nil-nil return.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		"Label,2025\nRevenue,100\n",
		"Label,2025,2024\nRevenue,\"100,000\",90000\n",
		"\xEF\xBB\xBFLabel,2025\nRevenue,100\n", // UTF-8 BOM, as raw bytes
		"Label;2025;2024\nRevenue;100;90\n",
		"Label\t2025\t2024\nRevenue\t100\t90\n",
		"\"Unterminated quote\nRevenue,100\n",
		",,,,\n,,,,\n",
		"Label,2025\n\n\n\nRevenue,100\n",
		"Label,2025\r\nRevenue,100\r\n",
		strings.Repeat("a,", 500) + "\n",
		strings.Repeat("Label,100\n", 200),
		"\x00\x01\x02,2025\nRevenue,100\n",
		"Label,2025\nRevenue,(1,234.56)\n",
		"Label,2025\nRevenue,N/A\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, doc string) {
		result, ingestErr := Parse(strings.NewReader(doc), ingestion.Options{})

		if ingestErr != nil {
			if result != nil {
				t.Fatalf("Parse returned both a non-nil Result and a non-nil error for input %q", doc)
			}
			if ingestErr.Code == "" {
				t.Fatalf("Parse returned a fatal error with an empty Code for input %q: %+v", doc, ingestErr)
			}
			return
		}
		if result == nil {
			t.Fatalf("Parse returned a nil Result and a nil error for input %q — an ambiguous outcome", doc)
		}
		// Every warning must carry a stable, non-empty Code — never a
		// caller-unmatchable freeform-only warning.
		for _, w := range result.Warnings {
			if w.Code == "" {
				t.Fatalf("Parse(%q) produced a Warning with an empty Code: %+v", doc, w)
			}
		}
	})
}
