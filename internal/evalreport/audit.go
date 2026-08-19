package evalreport

import "sort"

type Audit struct {
	Flag      string
	Identity  string
	Enabled   bool
	Variant   string
	Reason    string
	Evaluated int64
	// Seq is a monotonically increasing insertion identifier (the store rowid)
	// used as a deterministic tiebreaker when several observations share a
	// millisecond timestamp. It is not part of the report payload.
	Seq int64 `json:"-"`
}

// SortAudits returns a copy of audits ordered newest-to-oldest. Observations
// are compared first by evaluation time (descending); same-millisecond ties
// break by insertion sequence (descending) so the most recently recorded
// observation always wins, independent of the input order the caller supplied.
func SortAudits(audits []Audit) []Audit {
	out := append([]Audit(nil), audits...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Evaluated != out[j].Evaluated {
			return out[i].Evaluated > out[j].Evaluated
		}
		return out[i].Seq > out[j].Seq
	})
	return out
}

func EnabledRate(audits []Audit) float64 {
	if len(audits) == 0 {
		return 0
	}
	enabled := 0
	for _, audit := range audits {
		if audit.Enabled {
			enabled++
		}
	}
	return float64(enabled) / float64(len(audits))
}

// ByFlag returns the observations for a single flag, newest-to-oldest.
func ByFlag(audits []Audit, flag string) []Audit {
	out := make([]Audit, 0)
	for _, audit := range audits {
		if audit.Flag == flag {
			out = append(out, audit)
		}
	}
	return SortAudits(out)
}
