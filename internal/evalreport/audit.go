package evalreport

import "sort"

type Audit struct {
	Flag      string
	Identity  string
	Enabled   bool
	Variant   string
	Reason    string
	Evaluated int64
}

func SortAudits(audits []Audit) []Audit {
	out := append([]Audit(nil), audits...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Evaluated != out[j].Evaluated {
			return out[i].Evaluated < out[j].Evaluated
		}
		if out[i].Flag != out[j].Flag {
			return out[i].Flag < out[j].Flag
		}
		return out[i].Identity < out[j].Identity
	})
	return out
}

// EnabledRate returns the fraction of audits whose evaluation actually
// enabled the flag for the target. A closed result such as a failed
// prerequisite (which falls back to the default variant) is not counted,
// so the rate reflects the actual enabled outcome rather than the flag
// merely being active.
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

func ByFlag(audits []Audit, flag string) []Audit {
	out := make([]Audit, 0)
	for _, audit := range audits {
		if audit.Flag == flag {
			out = append(out, audit)
		}
	}
	return SortAudits(out)
}
