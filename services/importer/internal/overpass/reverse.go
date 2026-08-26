package overpass

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const nominatimEndpoint = "https://nominatim.openstreetmap.org/reverse"

// Nominatim's usage policy caps clients at roughly one request per second
// and requires an identifying User-Agent. Both are conditions of use on a
// free shared service, not suggestions — ignoring them gets the whole
// project blocked.
const nominatimMinInterval = 1100 * time.Millisecond

// Geocoder resolves coordinates to a city name, rate-limited to respect
// Nominatim's policy. Not safe for concurrent use: the throttle is a plain
// timestamp, and parallel callers would defeat the point of it anyway.
type Geocoder struct {
	HTTPClient *http.Client
	lastCall   time.Time
}

func NewGeocoder() *Geocoder {
	return &Geocoder{HTTPClient: &http.Client{Timeout: 30 * time.Second}}
}

type reverseResponse struct {
	Address map[string]string `json:"address"`
}

// City returns the best available place name for a coordinate, or "" when
// Nominatim can't name one. Callers must handle the empty case rather than
// substituting a guess.
func (g *Geocoder) City(ctx context.Context, lat, lng float64) (string, error) {
	if wait := nominatimMinInterval - time.Since(g.lastCall); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	g.lastCall = time.Now()

	params := url.Values{
		"lat":            {fmt.Sprintf("%f", lat)},
		"lon":            {fmt.Sprintf("%f", lng)},
		"format":         {"json"},
		"zoom":           {"10"}, // city level
		"addressdetails": {"1"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		nominatimEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("nominatim request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nominatim returned %d: %s", resp.StatusCode, truncate(body, 200))
	}

	var parsed reverseResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse nominatim response: %w", err)
	}

	// Nominatim names the same level differently depending on how the place
	// is administered, so fall through in decreasing specificity rather than
	// assuming one key exists.
	for _, key := range []string{"city", "town", "village", "municipality", "county"} {
		if v := strings.TrimSpace(parsed.Address[key]); v != "" {
			return v, nil
		}
	}
	return "", nil
}
