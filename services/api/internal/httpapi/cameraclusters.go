package httpapi

import (
	"math"
	"net/http"
	"strconv"
)

// Why this endpoint exists.
//
// /cameras returns individual points capped at a fixed limit. That is fine
// when the viewport holds a few hundred cameras and actively misleading when
// it holds 136,000: the map drew an arbitrary slice of the country — ordered
// by import recency, so not even spatially spread — and the national view
// showed a scattering of dots that looked like coverage was sparse. You had
// to zoom to street level before the cap stopped binding and the map
// started telling the truth.
//
// Sending every point instead is not an option (tens of MB of JSON, and a
// browser asked to draw 136k markers). So the aggregation happens where the
// data is: Postgres snaps points to a zoom-appropriate grid and returns one
// row per occupied cell with a true count. The national view becomes ~50
// rows describing all 136k cameras rather than 1,000 describing 0.7% of
// them.
const (
	// Grid cells are sized to render at roughly this many screen pixels.
	//
	// Bubbles are ~40px and sit on the mean position of their cameras rather
	// than the cell's centre, so neighbouring bubbles can end up much closer
	// together than the cell spacing implies. At 64px that produced visible
	// overlap in dense regions — Los Angeles and south Florida each had
	// three bubbles piled on each other. 88px spaces them out while keeping
	// roughly state-level granularity across a national view; going much
	// wider collapses neighbouring metros into a single meaningless blob.
	clusterCellPixels = 88
	// Web Mercator tiles are 256px, which is what relates zoom to degrees.
	tilePixels = 256
	// A viewport is at most a few hundred cells (see above), so this only
	// ever trips on a malformed zoom. It bounds the response either way.
	maxClusterCells = 4000
)

type cameraCluster struct {
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Count int     `json:"count"`
}

type cameraClustersResponse struct {
	// Total is the real number of cameras matching the filters in this
	// viewport — not the number of cells, and not a capped figure. The UI
	// says "N cameras in view", and that claim should be true.
	Total    int             `json:"total"`
	CellSize float64         `json:"cell_size_deg"`
	Clusters []cameraCluster `json:"clusters"`
}

// clusterCellDegrees converts a map zoom level into a grid cell size in
// degrees of longitude, so cells stay a constant size on screen as the user
// zooms rather than fragmenting or merging.
func clusterCellDegrees(zoom float64) float64 {
	// Degrees per pixel at this zoom, times the target cell size.
	deg := clusterCellPixels * 360 / (tilePixels * math.Pow(2, zoom))
	// A floor matters: ST_SnapToGrid with a zero or denormal cell size
	// either errors or collapses every point onto one coordinate.
	return math.Max(deg, 1e-7)
}

// handleCameraClusters serves GET /cameras/clusters.
func (s *Server) handleCameraClusters(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f, err := parseCameraFilters(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Zoom drives cell size only. It is clamped rather than rejected: a
	// nonsense value should still produce a usable map, not an error page.
	zoom := 4.0
	if v, err := strconv.ParseFloat(q.Get("zoom"), 64); err == nil {
		zoom = math.Min(math.Max(v, 0), 22)
	}
	cell := clusterCellDegrees(zoom)

	// avg(x)/avg(y) rather than the grid cell's centre: a bubble should sit
	// where its cameras actually are. On a cell straddling a coastline or a
	// state line, the grid centre can land somewhere with no cameras at all.
	rows, err := s.db.Query(r.Context(), `
		WITH filtered AS (
			SELECT location::geometry AS g
			FROM camera_sightings
			WHERE (
				NOT $1 OR location::geometry && ST_MakeEnvelope($2, $3, $4, $5, 4326)
			)
			  AND ($6 = '' OR source = $6)
			  AND ($7 = '' OR status = $7)
			  AND ($8 = '' OR manufacturer = $8)
			  AND (NOT $9 OR manufacturer IS NULL)
		)
		SELECT avg(ST_Y(g)), avg(ST_X(g)), count(*)
		FROM filtered
		GROUP BY ST_SnapToGrid(g, $10)
		ORDER BY count(*) DESC
		LIMIT $11
	`, f.hasBBox, f.west, f.south, f.east, f.north,
		f.source, f.status, f.manufacturer, f.manufacturerUnknown,
		cell, maxClusterCells)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load camera clusters", err)
		return
	}
	defer rows.Close()

	out := cameraClustersResponse{CellSize: cell, Clusters: []cameraCluster{}}
	for rows.Next() {
		var c cameraCluster
		if err := rows.Scan(&c.Lat, &c.Lng, &c.Count); err != nil {
			serverError(w, r, http.StatusInternalServerError, "could not load camera clusters", err)
			return
		}
		out.Clusters = append(out.Clusters, c)
		out.Total += c.Count
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load camera clusters", err)
		return
	}

	writeJSON(w, http.StatusOK, out)
}
