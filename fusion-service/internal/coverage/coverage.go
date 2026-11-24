package coverage

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	DefaultMinObservations     = 2
	DefaultMaxStalenessSeconds = 600.0 // 10 minutes
	MetersPerDegreeLat         = 111320.0
)

// Cell represents one spatial tile in the mission coverage grid.
type Cell struct {
	ID               string     `json:"cell_id"`
	MinLat           float64    `json:"-"`
	MaxLat           float64    `json:"-"`
	MinLon           float64    `json:"-"`
	MaxLon           float64    `json:"-"`
	ObservationCount int        `json:"observation_count"`
	LastObservedAt   *time.Time `json:"last_observed_at"`
	BestConfidence   float64    `json:"best_confidence"`
	Searched         bool       `json:"searched"`
}

// Summary provides high-level metrics for the mission area coverage.
type Summary struct {
	TotalCells      int     `json:"total_cells"`
	SearchedCells   int     `json:"searched_cells"`
	CoveragePercent float64 `json:"coverage_percent"`
	MeanConfidence  float64 `json:"mean_confidence"`
}

// Grid maintains the 2D coverage status over the operational area.
type Grid struct {
	mu                  sync.RWMutex
	MinLat              float64
	MaxLat              float64
	MinLon              float64
	MaxLon              float64
	CellSizeMeters      float64
	Rows                int
	Cols                int
	Cells               [][]*Cell
	MinObservations     int
	MaxStalenessSeconds float64
}

// NewGrid initializes a coverage grid for the given bounding box.
func NewGrid(minLat, minLon, maxLat, maxLon, cellSizeMeters float64) *Grid {
	if cellSizeMeters <= 0 {
		cellSizeMeters = 20.0
	}

	latSpanMeters := (maxLat - minLat) * MetersPerDegreeLat
	midLat := (minLat + maxLat) / 2.0
	lonSpanMeters := (maxLon - minLon) * (MetersPerDegreeLat * math.Cos(midLat*math.Pi/180.0))

	rows := int(math.Ceil(latSpanMeters / cellSizeMeters))
	cols := int(math.Ceil(lonSpanMeters / cellSizeMeters))
	if rows <= 0 {
		rows = 1
	}
	if cols <= 0 {
		cols = 1
	}
	// Limit grid dimensions to prevent excessive memory/JSON payload
	if rows > 80 {
		rows = 80
	}
	if cols > 80 {
		cols = 80
	}

	dLat := (maxLat - minLat) / float64(rows)
	dLon := (maxLon - minLon) / float64(cols)

	cells := make([][]*Cell, rows)
	for r := 0; r < rows; r++ {
		cells[r] = make([]*Cell, cols)
		for c := 0; c < cols; c++ {
			cMinLat := minLat + float64(r)*dLat
			cMaxLat := cMinLat + dLat
			cMinLon := minLon + float64(c)*dLon
			cMaxLon := cMinLon + dLon

			cells[r][c] = &Cell{
				ID:               fmt.Sprintf("cell_%d_%d", r, c),
				MinLat:           cMinLat,
				MaxLat:           cMaxLat,
				MinLon:           cMinLon,
				MaxLon:           cMaxLon,
				ObservationCount: 0,
				LastObservedAt:   nil,
				BestConfidence:   0.0,
				Searched:         false,
			}
		}
	}

	return &Grid{
		MinLat:              minLat,
		MaxLat:              maxLat,
		MinLon:              minLon,
		MaxLon:              maxLon,
		CellSizeMeters:      cellSizeMeters,
		Rows:                rows,
		Cols:                cols,
		Cells:               cells,
		MinObservations:     DefaultMinObservations,
		MaxStalenessSeconds: DefaultMaxStalenessSeconds,
	}
}

// RecordTelemetry updates cells overlapping with camera sensor ground footprint.
func (g *Grid) RecordTelemetry(lat, lon, alt, hfovDeg, vfovDeg float64, t time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Sensor altitude threshold check (10m - 120m AGL)
	if alt < 10.0 || alt > 150.0 {
		return
	}

	tanHalfH := math.Tan(hfovDeg * 0.5 * math.Pi / 180.0)
	tanHalfV := math.Tan(vfovDeg * 0.5 * math.Pi / 180.0)

	halfWidthMeters := alt * tanHalfH
	halfHeightMeters := alt * tanHalfV

	metersPerDegLon := MetersPerDegreeLat * math.Cos(lat*math.Pi/180.0)
	dLat := halfHeightMeters / MetersPerDegreeLat
	dLon := halfWidthMeters / metersPerDegLon

	fpMinLat := lat - dLat
	fpMaxLat := lat + dLat
	fpMinLon := lon - dLon
	fpMaxLon := lon + dLon

	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			cell := g.Cells[r][c]
			// Check bounding box intersection
			if !(cell.MaxLat < fpMinLat || cell.MinLat > fpMaxLat || cell.MaxLon < fpMinLon || cell.MinLon > fpMaxLon) {
				cell.ObservationCount++
				obsTime := t
				cell.LastObservedAt = &obsTime

				// Evaluate searched criteria: count >= minObservations and fresh
				if cell.ObservationCount >= g.MinObservations {
					cell.Searched = true
				}
			}
		}
	}
}

// RecordDetection records survivor/hazard confidence in the cell containing the detection.
func (g *Grid) RecordDetection(lat, lon, confidence float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			cell := g.Cells[r][c]
			if lat >= cell.MinLat && lat <= cell.MaxLat && lon >= cell.MinLon && lon <= cell.MaxLon {
				if confidence > cell.BestConfidence {
					cell.BestConfidence = confidence
				}
				return
			}
		}
	}
}

// EvaluateStaleness re-checks cells against staleness threshold at time now.
func (g *Grid) EvaluateStaleness(now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			cell := g.Cells[r][c]
			if cell.LastObservedAt == nil {
				cell.Searched = false
				continue
			}
			elapsed := now.Sub(*cell.LastObservedAt).Seconds()
			if elapsed > g.MaxStalenessSeconds {
				cell.Searched = false // Became stale
			} else if cell.ObservationCount >= g.MinObservations {
				cell.Searched = true
			}
		}
	}
}

// Summary returns current search statistics.
func (g *Grid) Summary(now time.Time) Summary {
	g.mu.RLock()
	defer g.mu.RUnlock()

	total := g.Rows * g.Cols
	searched := 0
	confSum := 0.0

	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			cell := g.Cells[r][c]
			isSearched := cell.Searched
			if cell.LastObservedAt != nil && now.Sub(*cell.LastObservedAt).Seconds() > g.MaxStalenessSeconds {
				isSearched = false
			}
			if isSearched {
				searched++
			}
			confSum += cell.BestConfidence
		}
	}

	coveragePct := 0.0
	if total > 0 {
		coveragePct = (float64(searched) / float64(total)) * 100.0
	}
	meanConf := 0.0
	if total > 0 {
		meanConf = confSum / float64(total)
	}

	return Summary{
		TotalCells:      total,
		SearchedCells:   searched,
		CoveragePercent: math.Round(coveragePct*100) / 100,
		MeanConfidence:  math.Round(meanConf*1000) / 1000,
	}
}

// ToGeoJSON outputs the coverage map as a GeoJSON FeatureCollection.
func (g *Grid) ToGeoJSON(now time.Time) ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	features := make([]map[string]interface{}, 0, g.Rows*g.Cols)

	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			cell := g.Cells[r][c]

			var staleness *float64
			var lastObsStr *string
			isSearched := cell.Searched

			if cell.LastObservedAt != nil {
				s := now.Sub(*cell.LastObservedAt).Seconds()
				if s < 0 {
					s = 0
				}
				staleness = &s
				formatted := cell.LastObservedAt.UTC().Format(time.RFC3339)
				lastObsStr = &formatted

				if s > g.MaxStalenessSeconds {
					isSearched = false
				}
			}

			// Polygon coordinates in [lon, lat] closed ring
			coords := [][][2]float64{
				{
					{cell.MinLon, cell.MinLat},
					{cell.MaxLon, cell.MinLat},
					{cell.MaxLon, cell.MaxLat},
					{cell.MinLon, cell.MaxLat},
					{cell.MinLon, cell.MinLat},
				},
			}

			feat := map[string]interface{}{
				"type": "Feature",
				"geometry": map[string]interface{}{
					"type":        "Polygon",
					"coordinates": coords,
				},
				"properties": map[string]interface{}{
					"cell_id":           cell.ID,
					"observation_count": cell.ObservationCount,
					"last_observed_at":  lastObsStr,
					"best_confidence":   cell.BestConfidence,
					"searched":          isSearched,
					"staleness_seconds": staleness,
				},
			}
			features = append(features, feat)
		}
	}

	collection := map[string]interface{}{
		"type":     "FeatureCollection",
		"features": features,
	}

	return json.Marshal(collection)
}
