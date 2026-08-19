package evalreport

import "sort"

type Segment struct {
	Name      string
	Attribute string
	Values    map[string]bool
}

func (segment Segment) Includes(attrs map[string]string) bool {
	if segment.Attribute == "" {
		return false
	}
	value, ok := attrs[segment.Attribute]
	if !ok {
		return false
	}
	return segment.Values[value]
}

func MatchingSegments(segments []Segment, attrs map[string]string) []string {
	names := make([]string, 0)
	for _, segment := range segments {
		if segment.Includes(attrs) {
			names = append(names, segment.Name)
		}
	}
	sort.Strings(names)
	return names
}

func SegmentIndex(segments []Segment) map[string]Segment {
	index := make(map[string]Segment, len(segments))
	for _, segment := range segments {
		values := make(map[string]bool, len(segment.Values))
		for value, enabled := range segment.Values {
			values[value] = enabled
		}
		segment.Values = values
		index[segment.Name] = segment
	}
	return index
}
