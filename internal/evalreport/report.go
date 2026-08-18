package evalreport

// Summary describes the distribution of persisted evaluations. It is kept
// separate from the HTTP layer so the same report can be used by operators
// and by restart/recovery smoke checks.
type Summary struct {
	Total       int            `json:"total"`
	EnabledRate float64        `json:"enabled_rate"`
	ByReason    map[string]int `json:"by_reason"`
	Identities  int            `json:"identities"`
}

// BuildSummary aggregates persisted evaluation observations without mutating
// the input order used by the audit endpoint.
func BuildSummary(audits []Audit) Summary {
	identities := make(map[string]struct{}, len(audits))
	byReason := make(map[string]int)
	for _, audit := range audits {
		identities[audit.Identity] = struct{}{}
		byReason[audit.Rule]++
	}
	return Summary{
		Total:       len(audits),
		EnabledRate: EnabledRate(audits),
		ByReason:    byReason,
		Identities:  len(identities),
	}
}
