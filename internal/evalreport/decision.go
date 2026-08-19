package evalreport

import "sort"

type Rule struct {
	Name       string
	Attribute  string
	Expected   string
	Value      string
	Percentage int
}

func (r Rule) Matches(attrs map[string]string) bool {
	value := attrs[r.Attribute]
	if r.Expected != "" && value != r.Expected {
		return false
	}
	return true
}

type Decision struct {
	Flag    string
	Enabled bool
	Rule    string
	Reason  string
}

func Evaluate(flag string, enabled bool, rules []Rule, attrs map[string]string) Decision {
	if !enabled {
		return Decision{Flag: flag, Reason: "disabled"}
	}
	ordered := append([]Rule(nil), rules...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Percentage != ordered[j].Percentage {
			return ordered[i].Percentage > ordered[j].Percentage
		}
		return ordered[i].Name < ordered[j].Name
	})
	for _, rule := range ordered {
		if rule.Matches(attrs) {
			return Decision{Flag: flag, Enabled: true, Rule: rule.Name, Reason: "matched"}
		}
	}
	return Decision{Flag: flag, Reason: "no-rule"}
}

func Explain(decision Decision) string {
	if decision.Enabled {
		return decision.Flag + ":enabled:" + decision.Rule
	}
	return decision.Flag + ":disabled:" + decision.Reason
}
