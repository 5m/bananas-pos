package input

import "strings"

func SplitLabels(raw string) []string {
	var labels []string
	searchFrom := 0

	for {
		start := strings.Index(raw[searchFrom:], "^XA")
		if start < 0 {
			return labels
		}
		start += searchFrom

		end := strings.Index(raw[start:], "^XZ")
		if end < 0 {
			return labels
		}
		end += start + len("^XZ")

		labels = append(labels, raw[start:end])
		searchFrom = end
	}
}
