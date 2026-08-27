package httpapi

import (
	"errors"
	"net/url"
)

// cameraFilters is the query surface shared by /cameras and
// /cameras/clusters. The two endpoints must agree exactly on what a filter
// means — if they drift, the count on a cluster bubble stops matching the
// points revealed by zooming into it, which is the kind of discrepancy that
// quietly destroys trust in the data.
type cameraFilters struct {
	hasBBox                  bool
	west, south, east, north float64

	source       string
	status       string
	manufacturer string
	// manufacturerUnknown selects rows where OSM recorded no manufacturer,
	// which can't be expressed as an equality match.
	manufacturerUnknown bool
}

func parseCameraFilters(q url.Values) (cameraFilters, error) {
	var f cameraFilters

	if b := q.Get("bbox"); b != "" {
		var err error
		f.west, f.south, f.east, f.north, err = parseBBox(b)
		if err != nil {
			return f, errors.New("invalid bbox, expected west,south,east,north")
		}
		f.hasBBox = true
	}

	// Empty means "no filter" for each, so an unqualified call returns
	// everything.
	f.source = q.Get("source")
	if f.source != "" && f.source != "osm_import" && f.source != "user_submission" {
		return f, errors.New("invalid source, must be osm_import or user_submission")
	}

	f.status = q.Get("status")
	if f.status != "" && f.status != "confirmed" && f.status != "under_review" && f.status != "removed" {
		return f, errors.New("invalid status, must be confirmed, under_review, or removed")
	}

	f.manufacturer = q.Get("manufacturer")
	if f.manufacturer == "unknown" {
		f.manufacturerUnknown = true
		f.manufacturer = ""
	}

	return f, nil
}
