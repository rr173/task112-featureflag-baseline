package evalreport

// SelectRecent returns at most limit observations ordered newest-to-oldest.
// It sorts defensively so the selection is correct regardless of how the
// caller ordered the input, and the copy protects the caller from accidental
// reordering of its store-backed slice.
func SelectRecent(audits []Audit, limit int) []Audit {
	if limit <= 0 {
		return []Audit{}
	}
	ordered := SortAudits(audits)
	if limit >= len(ordered) {
		return ordered
	}
	return append([]Audit(nil), ordered[:limit]...)
}
