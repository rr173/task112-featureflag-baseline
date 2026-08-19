package evalreport

type FlagSummary struct {
	Flag    string
	Total   int
	Enabled int
	Rate    float64
}

func SummarizeFlag(flag string, audits []Audit) FlagSummary {
	items := ByFlag(audits, flag)
	result := FlagSummary{Flag: flag, Total: len(items)}
	for _, item := range items {
		if item.Enabled {
			result.Enabled++
		}
	}
	if result.Total > 0 {
		result.Rate = float64(result.Enabled) / float64(result.Total)
	}
	return result
}
