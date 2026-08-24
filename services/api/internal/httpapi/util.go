package httpapi

import (
	"errors"
	"strings"

	"flockwatch/api/internal/models"
)

var errBadBBox = errors.New("bbox must be 'west,south,east,north'")

func joinEvidenceTypes() string {
	names := make([]string, len(models.ValidEvidenceTypes))
	for i, e := range models.ValidEvidenceTypes {
		names[i] = string(e)
	}
	return strings.Join(names, ", ")
}

func joinDeploymentStatuses() string {
	names := make([]string, len(models.ValidDeploymentStatuses))
	for i, s := range models.ValidDeploymentStatuses {
		names[i] = string(s)
	}
	return strings.Join(names, ", ")
}

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
