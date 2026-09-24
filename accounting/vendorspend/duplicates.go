package vendorspend

import (
	"fmt"
	"math"
	"sort"
)

// DuplicateSignature is the normalized factual tuple two SpendRecords are
// compared on for possible-economic-duplicate detection — task section
// 22. Deliberately factual fields only (SupplierID/Date/Amount/
// ProductID/ReferenceID/Category), never a fraud inference.
type DuplicateSignature struct {
	SupplierID  string  `json:"supplier_id"`
	Date        string  `json:"date"`
	Amount      float64 `json:"amount"`
	ProductID   string  `json:"product_id,omitempty"`
	ReferenceID string  `json:"reference_id,omitempty"`
	Category    string  `json:"category,omitempty"`
}

// DuplicateLikeGroup is one set of SpendRecords whose normalized
// signatures matched within Policy.DuplicateWindowDays — task section 22.
// No fraud language anywhere in this package (see neutral_language_test.go);
// this only reports "these records look alike," never "this is
// fraudulent" or "this is an error."
type DuplicateLikeGroup struct {
	SupplierID string   `json:"supplier_id"`
	SpendIDs   []string `json:"spend_ids"`
	Amount     float64  `json:"amount"`
	// FirstDate/LastDate are the earliest/latest Date among SpendIDs.
	FirstDate string `json:"first_date"`
	LastDate  string `json:"last_date"`
}

// dupBucketKey buckets records into coarse (SupplierID, rounded Amount,
// day-window index) cells so only records already sharing a bucket are
// ever compared — the "bounded indexed approach rather than O(N^2)" task
// section 22 requires. windowDays defines the bucket width in days;
// candidates in adjacent buckets are also compared (see
// computeDuplicateLikeGroups) so a pair straddling a bucket boundary is
// never missed.
type dupBucketKey struct {
	supplierID string
	amountCent int64
	bucket     int64
}

// roundToCents converts a dollar amount to an integer cent count,
// rounding to the nearest cent (half away from zero via math.Round) —
// correct for both positive amounts and the negative amounts a CREDIT/
// REFUND/REVERSAL record's Amount can legitimately carry (see
// SpendRecord.Amount's doc comment). The naive `int64(v*100 + 0.5)`
// round-half-up formula this replaced was wrong for negative values:
// int64(-100.0*100+0.5) truncates -9999.5 toward zero to -9999, one cent
// off from the correct -10000.
func roundToCents(v float64) int64 {
	return int64(math.Round(v * 100))
}

// computeDuplicateLikeGroups detects possible economic duplicates among
// records (already deduplicated by SpendID — a duplicate SpendID is a
// separate, structural IssueDuplicateSpend, not this detection). Runs in
// O(N log N): records are bucketed by (SupplierID, rounded Amount, date
// window), and only records in the same or an adjacent bucket are ever
// compared pairwise, bounding the comparison work per record regardless
// of total dataset size (see benchmark_test.go's scaling verification).
func computeDuplicateLikeGroups(records []SpendRecord, windowDays int) []DuplicateLikeGroup {
	if windowDays <= 0 {
		windowDays = 3
	}
	windowSeconds := int64(windowDays) * 86400

	buckets := map[dupBucketKey][]SpendRecord{}

	for _, r := range records {
		if r.Date.IsZero() {
			continue
		}
		k := dupBucketKey{
			supplierID: r.SupplierID,
			amountCent: roundToCents(r.Amount),
			bucket:     r.Date.Unix() / windowSeconds,
		}
		buckets[k] = append(buckets[k], r)
	}

	// Union-find over SpendIDs so transitively-matching records (A~B,
	// B~C) end up in one group even if A and C are not themselves within
	// windowDays of each other.
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for _, r := range records {
		parent[r.SpendID] = r.SpendID
	}

	compareAndUnion := func(a, b SpendRecord) {
		if a.SpendID == b.SpendID {
			return
		}
		if !signaturesMatch(a, b) {
			return
		}
		diff := a.Date.Unix() - b.Date.Unix()
		if diff < 0 {
			diff = -diff
		}
		if diff > windowSeconds {
			return
		}
		union(a.SpendID, b.SpendID)
	}

	// Sorted bucket keys for deterministic adjacent-bucket lookup.
	keys := make([]dupBucketKey, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].supplierID != keys[j].supplierID {
			return keys[i].supplierID < keys[j].supplierID
		}
		if keys[i].amountCent != keys[j].amountCent {
			return keys[i].amountCent < keys[j].amountCent
		}
		return keys[i].bucket < keys[j].bucket
	})

	for _, k := range keys {
		items := buckets[k]
		// Within the same bucket: compare every pair (bucket size is
		// naturally small — same supplier, same rounded amount, same
		// narrow date window).
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				compareAndUnion(items[i], items[j])
			}
		}
		// Adjacent bucket (bucket+1): catches pairs whose dates straddle
		// a bucket boundary but are still within windowDays.
		adjKey := dupBucketKey{supplierID: k.supplierID, amountCent: k.amountCent, bucket: k.bucket + 1}
		if adjItems, ok := buckets[adjKey]; ok {
			for _, a := range items {
				for _, b := range adjItems {
					compareAndUnion(a, b)
				}
			}
		}
	}

	// Assemble groups from union-find roots with 2+ members.
	groupMembers := map[string][]SpendRecord{}
	for _, r := range records {
		root := find(r.SpendID)
		groupMembers[root] = append(groupMembers[root], r)
	}

	var groups []DuplicateLikeGroup
	for _, members := range groupMembers {
		if len(members) < 2 {
			continue
		}
		sort.SliceStable(members, func(i, j int) bool {
			if !members[i].Date.Equal(members[j].Date) {
				return members[i].Date.Before(members[j].Date)
			}
			return members[i].SpendID < members[j].SpendID
		})
		ids := make([]string, 0, len(members))
		for _, m := range members {
			ids = append(ids, m.SpendID)
		}
		sort.Strings(ids)
		groups = append(groups, DuplicateLikeGroup{
			SupplierID: members[0].SupplierID,
			SpendIDs:   ids,
			Amount:     members[0].Amount,
			FirstDate:  members[0].Date.Format("2006-01-02"),
			LastDate:   members[len(members)-1].Date.Format("2006-01-02"),
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].FirstDate != groups[j].FirstDate {
			return groups[i].FirstDate < groups[j].FirstDate
		}
		if groups[i].SupplierID != groups[j].SupplierID {
			return groups[i].SupplierID < groups[j].SupplierID
		}
		return groups[i].SpendIDs[0] < groups[j].SpendIDs[0]
	})
	return groups
}

// signaturesMatch reports whether a and b share the same normalized
// duplicate signature on every field that is non-empty on both sides.
// SupplierID and Amount (already bucketed) are always required to match;
// ProductID/ReferenceID/Category only constrain the match when at least
// one side supplies a non-empty value for that field, so records that
// simply lack optional detail are not excluded from matching purely on
// that basis.
func signaturesMatch(a, b SpendRecord) bool {
	if a.SupplierID != b.SupplierID {
		return false
	}
	if fmt.Sprintf("%.2f", a.Amount) != fmt.Sprintf("%.2f", b.Amount) {
		return false
	}
	if a.ProductID != "" && b.ProductID != "" && a.ProductID != b.ProductID {
		return false
	}
	if a.ReferenceID != "" && b.ReferenceID != "" && a.ReferenceID != b.ReferenceID {
		return false
	}
	if a.Category != "" && b.Category != "" && a.Category != b.Category {
		return false
	}
	return true
}
