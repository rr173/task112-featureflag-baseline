package evalreport

// SelectRecent returns at most limit observations in evaluation time order.
// The copy protects the caller from accidental reordering of its store-backed
// slice when a report is assembled with a second sort.
func SelectRecent(audits []Audit, limit int) []Audit {
	if limit <= 0 || limit >= len(audits) {
		return append([]Audit(nil), audits...)
	}
	ordered := SortAudits(audits)
	recent := ordered[len(ordered)-limit:]
	out := make([]Audit, len(recent))
	for i := range recent {
		out[len(recent)-1-i] = recent[i]
	}
	return out
}
