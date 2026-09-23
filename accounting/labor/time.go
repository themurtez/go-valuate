package labor

import "time"

// mustParseDate parses a "YYYY-MM-DD" string produced by this package's
// own time.Format calls; it never receives untrusted caller input (every
// call site formats a time.Time via the same layout first), so a parse
// error here would indicate a bug in this package, not bad caller data —
// it returns the zero time.Time in that defensive case rather than
// panicking.
func mustParseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}
	}
	return t
}
