package httpapi

import (
	"errors"
	"strings"
)

var errBadBBox = errors.New("bbox must be 'west,south,east,north'")

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
