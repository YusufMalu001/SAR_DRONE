// Package report generates situational reports (JSON and printable HTML)
// for emergency response teams with deterministic, template-based rescue
// recommendations and embedded static vector maps.
package report

import (
	"context"
	"fmt"
	"html"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/connectivity"
	"sar-drone-prototype/fusion-service/internal/coverage"
	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/entity"
)

type TimeSpan struct {
	Start           time.Time `json:"start"`
	End             time.Time `json:"end"`
	DurationSeconds float64   `json:"duration_seconds"`
}

type SurvivorReport struct {
	ID            string    `json:"id"`
	Class         string    `json:"class"`
	Lat           float64   `json:"lat"`
	Lon           float64   `json:"lon"`
	Confidence    float64   `json:"confidence"`
	Modalities    []string  `json:"modalities"`
	Observations  int       `json:"observations"`
	PriorityScore float64   `json:"priority_score"`
	Status        string    `json:"status"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	NearbyHazard  string    `json:"nearby_hazard,omitempty"`
	HazardDistM   float64   `json:"hazard_distance_meters,omitempty"`
}

type HazardZoneReport struct {
	ID        string    `json:"id"`
	Class     string    `json:"class"`
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	RadiusM   float64   `json:"radius_meters"`
	Severity  string    `json:"severity"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

type RescueRecommendation struct {
	Rank               int     `json:"rank"`
	SurvivorID         string  `json:"survivor_id"`
	PriorityScore      float64 `json:"priority_score"`
	Lat                float64 `json:"lat"`
	Lon                float64 `json:"lon"`
	Rationale          string  `json:"rationale"`
	RouteObstruction   string  `json:"route_obstruction"`
	EstimatedApproachM float64 `json:"estimated_approach_meters"`
}

type UnresolvedArea struct {
	CellID         string  `json:"cell_id"`
	MinLat         float64 `json:"min_lat"`
	MaxLat         float64 `json:"max_lat"`
	MinLon         float64 `json:"min_lon"`
	MaxLon         float64 `json:"max_lon"`
	BestConfidence float64 `json:"best_confidence"`
	Reason         string  `json:"reason"`
}

type DegradedInterval struct {
	Reason    string    `json:"reason"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	DurationS float64   `json:"duration_seconds"`
}

type SystemHealthReport struct {
	CurrentMode       string             `json:"current_mode"`
	ConnectivityGaps  int                `json:"connectivity_gaps"`
	DegradedIntervals []DegradedInterval `json:"degraded_intervals"`
}

type SitRep struct {
	MissionID          string                 `json:"mission_id"`
	GeneratedAt        time.Time              `json:"generated_at"`
	TimeSpan           TimeSpan               `json:"time_span"`
	AreaCoveredPercent float64                `json:"area_covered_percent"`
	TotalSurvivors     int                    `json:"total_survivors"`
	TotalHazards       int                    `json:"total_hazards"`
	Survivors          []SurvivorReport       `json:"survivors"`
	HazardZones        []HazardZoneReport     `json:"hazard_zones"`
	Recommendations    []RescueRecommendation `json:"recommendations"`
	UnresolvedAreas    []UnresolvedArea       `json:"unresolved_areas"`
	SystemHealth       SystemHealthReport     `json:"system_health"`
	StaticMapSVG       string                 `json:"static_map_svg,omitempty"`
}

// GenerateSitRep queries current mission state across clusterer/DB and builds a complete SitRep.
func GenerateSitRep(
	ctx context.Context,
	pool *pgxpool.Pool,
	clusterer *entity.Clusterer,
	covGrid *coverage.Grid,
	monitor *connectivity.Monitor,
	missionID string,
	obstructionMeters float64,
) (*SitRep, error) {
	now := time.Now().UTC()
	if missionID == "" {
		missionID = "mission-sar-alpha"
	}

	// 1. Gather survivors and hazards
	var survivorRows []entity.Entity
	var hazardRows []entity.Entity
	if clusterer != nil {
		for _, e := range clusterer.ListEntities("", "") {
			if e.Type == entity.TypeSurvivor {
				survivorRows = append(survivorRows, e)
			} else if e.Type == entity.TypeHazard {
				hazardRows = append(hazardRows, e)
			}
		}
	}

	// Fallback/enrich from database if available
	if pool != nil {
		if dbSurvivors, err := db.ListEntities(ctx, pool, "survivor", ""); err == nil && len(dbSurvivors) > len(survivorRows) {
			survivorRows = nil
			for _, r := range dbSurvivors {
				survivorRows = append(survivorRows, entity.Entity{
					ID:                r.ID,
					Type:              entity.TypeSurvivor,
					Class:             r.Class,
					Lat:               r.Lat,
					Lon:               r.Lon,
					UncertaintyRadius: r.UncertaintyRadius,
					TrackIDs:          r.TrackIDs,
					ObservationCount:  r.ObservationCount,
					Confidence:        r.Confidence,
					Modalities:        r.Modalities,
					PriorityScore:     r.PriorityScore,
					Status:            entity.EntityStatus(r.Status),
					FirstSeen:         r.FirstSeen,
					LastSeen:          r.LastSeen,
				})
			}
		}
		if dbHazards, err := db.ListEntities(ctx, pool, "hazard", ""); err == nil && len(dbHazards) > len(hazardRows) {
			hazardRows = nil
			for _, r := range dbHazards {
				hazardRows = append(hazardRows, entity.Entity{
					ID:                r.ID,
					Type:              entity.TypeHazard,
					Class:             r.Class,
					Lat:               r.Lat,
					Lon:               r.Lon,
					UncertaintyRadius: r.UncertaintyRadius,
					TrackIDs:          r.TrackIDs,
					ObservationCount:  r.ObservationCount,
					Confidence:        r.Confidence,
					Modalities:        r.Modalities,
					PriorityScore:     r.PriorityScore,
					Status:            entity.EntityStatus(r.Status),
					FirstSeen:         r.FirstSeen,
					LastSeen:          r.LastSeen,
				})
			}
		}
	}

	// 2. Build Hazard Zone reports
	var hazardReports []HazardZoneReport
	for _, h := range hazardRows {
		sev := "high"
		if h.PriorityScore >= 0.9 {
			sev = "critical"
		} else if h.PriorityScore < 0.6 {
			sev = "medium"
		}
		rad := h.UncertaintyRadius
		if rad < 20.0 {
			rad = 25.0
		}
		hazardReports = append(hazardReports, HazardZoneReport{
			ID:        h.ID,
			Class:     h.Class,
			Lat:       h.Lat,
			Lon:       h.Lon,
			RadiusM:   rad,
			Severity:  sev,
			FirstSeen: h.FirstSeen,
			LastSeen:  h.LastSeen,
		})
	}

	// 3. Build Survivor reports & associate hazards
	var survivorReports []SurvivorReport
	var tMin, tMax time.Time

	for _, s := range survivorRows {
		if tMin.IsZero() || s.FirstSeen.Before(tMin) {
			tMin = s.FirstSeen
		}
		if s.LastSeen.After(tMax) {
			tMax = s.LastSeen
		}

		closestHazard := ""
		minHazardDist := math.MaxFloat64
		for _, h := range hazardReports {
			d := haversineMeters(s.Lat, s.Lon, h.Lat, h.Lon)
			if d < minHazardDist {
				minHazardDist = d
				closestHazard = h.Class
			}
		}

		sr := SurvivorReport{
			ID:            s.ID,
			Class:         s.Class,
			Lat:           s.Lat,
			Lon:           s.Lon,
			Confidence:    s.Confidence,
			Modalities:    s.Modalities,
			Observations:  s.ObservationCount,
			PriorityScore: s.PriorityScore,
			Status:        string(s.Status),
			FirstSeen:     s.FirstSeen,
			LastSeen:      s.LastSeen,
		}
		if minHazardDist <= 150.0 {
			sr.NearbyHazard = closestHazard
			sr.HazardDistM = minHazardDist
		}
		survivorReports = append(survivorReports, sr)
	}

	if tMin.IsZero() {
		tMin = now.Add(-10 * time.Minute)
		tMax = now
	}

	// 4. Area Coverage %
	covPercent := 0.0
	var unresolved []UnresolvedArea
	if covGrid != nil {
		summary := covGrid.Summary(now)
		covPercent = summary.CoveragePercent

		// Check for low-confidence or unsearched cells
		for r := 0; r < covGrid.Rows; r++ {
			for c := 0; c < covGrid.Cols; c++ {
				cell := covGrid.Cells[r][c]
				if cell == nil {
					continue
				}
				if !cell.Searched {
					unresolved = append(unresolved, UnresolvedArea{
						CellID:         cell.ID,
						MinLat:         cell.MinLat,
						MaxLat:         cell.MaxLat,
						MinLon:         cell.MinLon,
						MaxLon:         cell.MaxLon,
						BestConfidence: cell.BestConfidence,
						Reason:         "unsearched",
					})
				} else if cell.BestConfidence > 0 && cell.BestConfidence < 0.5 {
					unresolved = append(unresolved, UnresolvedArea{
						CellID:         cell.ID,
						MinLat:         cell.MinLat,
						MaxLat:         cell.MaxLat,
						MinLon:         cell.MinLon,
						MaxLon:         cell.MaxLon,
						BestConfidence: cell.BestConfidence,
						Reason:         "low confidence observation",
					})
				}
			}
		}
	}

	// 5. Generate prioritized rescue recommendations
	// Sort survivors descending by PriorityScore
	sortedSurvivors := make([]SurvivorReport, len(survivorReports))
	copy(sortedSurvivors, survivorReports)
	for i := 0; i < len(sortedSurvivors)-1; i++ {
		for j := i + 1; j < len(sortedSurvivors); j++ {
			if sortedSurvivors[j].PriorityScore > sortedSurvivors[i].PriorityScore {
				sortedSurvivors[i], sortedSurvivors[j] = sortedSurvivors[j], sortedSurvivors[i]
			}
		}
	}

	var recommendations []RescueRecommendation
	for rank, s := range sortedSurvivors {
		if s.Status == "rescued" || s.Status == "cleared" {
			continue
		}

		routeStatus := "clear"
		if s.NearbyHazard != "" && s.HazardDistM < obstructionMeters {
			routeStatus = fmt.Sprintf("caution: within %.0fm of %s zone", s.HazardDistM, s.NearbyHazard)
		}

		rationale := formatRecommendationRationale(s)
		recommendations = append(recommendations, RescueRecommendation{
			Rank:               rank + 1,
			SurvivorID:         s.ID,
			PriorityScore:      s.PriorityScore,
			Lat:                s.Lat,
			Lon:                s.Lon,
			Rationale:          rationale,
			RouteObstruction:   routeStatus,
			EstimatedApproachM: s.HazardDistM,
		})
	}

	// 6. System health & mode
	mode := "online"
	gaps := 0
	if monitor != nil && monitor.IsOffline() {
		mode = "offline"
		gaps = 1
	}

	// 7. Generate Static Vector Map (SVG)
	mapSVG := renderStaticMapSVG(survivorReports, hazardReports)

	return &SitRep{
		MissionID:   missionID,
		GeneratedAt: now,
		TimeSpan: TimeSpan{
			Start:           tMin,
			End:             tMax,
			DurationSeconds: tMax.Sub(tMin).Seconds(),
		},
		AreaCoveredPercent: covPercent,
		TotalSurvivors:     len(survivorReports),
		TotalHazards:       len(hazardReports),
		Survivors:          survivorReports,
		HazardZones:        hazardReports,
		Recommendations:    recommendations,
		UnresolvedAreas:    unresolved,
		SystemHealth: SystemHealthReport{
			CurrentMode:      mode,
			ConnectivityGaps: gaps,
		},
		StaticMapSVG: mapSVG,
	}, nil
}

// formatRecommendationRationale builds machine-generated rationales from strictly verified template parameters.
func formatRecommendationRationale(s SurvivorReport) string {
	modDesc := "RGB detection"
	hasRGB := false
	hasThermal := false
	for _, m := range s.Modalities {
		if m == "rgb" {
			hasRGB = true
		}
		if m == "thermal" {
			hasThermal = true
		}
	}
	if hasRGB && hasThermal {
		modDesc = "confirmed by RGB+thermal cross-modal fusion"
	} else if hasThermal {
		modDesc = "thermal infrared signature"
	}

	hazardDesc := "isolated from immediate hazard zones"
	if s.NearbyHazard != "" && s.HazardDistM > 0 {
		hazardDesc = fmt.Sprintf("%.0f m from %s zone", s.HazardDistM, s.NearbyHazard)
	}

	return fmt.Sprintf("%s %s; %s; confidence %.0f%% over %d observations; priority score %.2f",
		s.Class, modDesc, hazardDesc, s.Confidence*100, s.Observations, s.PriorityScore)
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000.0
	phi1 := lat1 * math.Pi / 180.0
	phi2 := lat2 * math.Pi / 180.0
	dphi := (lat2 - lat1) * math.Pi / 180.0
	dlam := (lon2 - lon1) * math.Pi / 180.0

	a := math.Sin(dphi/2.0)*math.Sin(dphi/2.0) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dlam/2.0)*math.Sin(dlam/2.0)
	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))
	return R * c
}

// renderStaticMapSVG renders an inline SVG map showing survivors, hazard perimeters, and coordinates.
func renderStaticMapSVG(survivors []SurvivorReport, hazards []HazardZoneReport) string {
	width := 640.0
	height := 360.0

	// Compute bounding box
	minLat, maxLat := 90.0, -90.0
	minLon, maxLon := 180.0, -180.0

	for _, s := range survivors {
		if s.Lat < minLat {
			minLat = s.Lat
		}
		if s.Lat > maxLat {
			maxLat = s.Lat
		}
		if s.Lon < minLon {
			minLon = s.Lon
		}
		if s.Lon > maxLon {
			maxLon = s.Lon
		}
	}
	for _, h := range hazards {
		if h.Lat < minLat {
			minLat = h.Lat
		}
		if h.Lat > maxLat {
			maxLat = h.Lat
		}
		if h.Lon < minLon {
			minLon = h.Lon
		}
		if h.Lon > maxLon {
			maxLon = h.Lon
		}
	}

	if minLat > maxLat {
		minLat, maxLat = 18.50, 18.52
		minLon, maxLon = 73.80, 73.82
	}

	// Add 20% padding
	dLat := (maxLat - minLat) * 0.2
	if dLat < 0.002 {
		dLat = 0.002
	}
	dLon := (maxLon - minLon) * 0.2
	if dLon < 0.002 {
		dLon = 0.002
	}
	minLat -= dLat
	maxLat += dLat
	minLon -= dLon
	maxLon += dLon

	project := func(lat, lon float64) (float64, float64) {
		x := (lon - minLon) / (maxLon - minLon) * (width - 80) + 40
		y := (maxLat - lat) / (maxLat - minLat) * (height - 80) + 40
		return x, y
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg viewBox="0 0 %.0f %.0f" width="100%%" height="360" xmlns="http://www.w3.org/2000/svg" style="background:#0f172a; border-radius:8px; font-family:sans-serif;">`, width, height))

	// Grid background
	sb.WriteString(`<defs><pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse"><path d="M 40 0 L 0 0 0 40" fill="none" stroke="#1e293b" stroke-width="1"/></pattern></defs>`)
	sb.WriteString(fmt.Sprintf(`<rect width="%.0f" height="%.0f" fill="url(#grid)" />`, width, height))

	// Hazard zones
	for _, h := range hazards {
		hx, hy := project(h.Lat, h.Lon)
		r := 35.0
		fill := "rgba(239, 68, 68, 0.25)"
		stroke := "#ef4444"
		if h.Class == "flood" {
			fill = "rgba(59, 130, 246, 0.25)"
			stroke = "#3b82f6"
		}
		sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s" stroke="%s" stroke-width="2" stroke-dasharray="4,2"/>`, hx, hy, r, fill, stroke))
		sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" fill="%s" font-size="11" font-weight="bold" text-anchor="middle">%s ZONE</text>`, hx, hy+4, stroke, strings.ToUpper(h.Class)))
	}

	// Survivors
	for i, s := range survivors {
		sx, sy := project(s.Lat, s.Lon)
		color := "#22c55e"
		if s.PriorityScore >= 0.8 {
			color = "#f97316"
		}
		if s.PriorityScore >= 0.9 {
			color = "#ef4444"
		}
		sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="8" fill="%s" stroke="#ffffff" stroke-width="2"/>`, sx, sy, color))
		sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" fill="#ffffff" font-size="10" font-weight="bold" text-anchor="middle" dy=".3em">%d</text>`, sx, sy, i+1))
		sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" fill="#e2e8f0" font-size="11" font-weight="bold">%s (%.2f)</text>`, sx+12, sy+4, s.ID, s.PriorityScore))
	}

	// Legend
	sb.WriteString(`<rect x="15" y="15" width="160" height="60" rx="4" fill="rgba(15, 23, 42, 0.8)" stroke="#334155"/>`)
	sb.WriteString(`<circle cx="30" cy="30" r="5" fill="#ef4444"/>`)
	sb.WriteString(`<text x="42" y="34" fill="#94a3b8" font-size="11">High Priority Survivor</text>`)
	sb.WriteString(`<circle cx="30" cy="50" r="5" fill="#22c55e"/>`)
	sb.WriteString(`<text x="42" y="54" fill="#94a3b8" font-size="11">Standard Survivor</text>`)

	sb.WriteString(`</svg>`)
	return sb.String()
}

// RenderHTML formats the SitRep into printable HTML suitable for incident commanders.
func RenderHTML(r *SitRep) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>SAR Situational Report — ` + html.EscapeString(r.MissionID) + `</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; margin: 0; padding: 24px; color: #1e293b; background: #f8fafc; }
  .report-container { max-width: 900px; margin: 0 auto; background: #ffffff; padding: 32px; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  h1 { margin-top: 0; color: #0f172a; font-size: 26px; border-bottom: 2px solid #e2e8f0; padding-bottom: 12px; }
  h2 { color: #1e293b; font-size: 18px; margin-top: 28px; border-bottom: 1px solid #cbd5e1; padding-bottom: 6px; }
  .meta-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 24px; }
  .metric-card { background: #f1f5f9; padding: 14px; border-radius: 6px; }
  .metric-label { font-size: 12px; text-transform: uppercase; color: #64748b; font-weight: 600; }
  .metric-val { font-size: 20px; font-weight: 700; color: #0f172a; margin-top: 4px; }
  table { width: 100%; border-collapse: collapse; margin-top: 12px; font-size: 14px; }
  th, td { text-align: left; padding: 10px 12px; border-bottom: 1px solid #e2e8f0; }
  th { background: #f8fafc; color: #475569; font-weight: 600; }
  .badge { display: inline-block; padding: 3px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; text-transform: uppercase; }
  .badge-critical { background: #fee2e2; color: #dc2626; }
  .badge-high { background: #ffedd5; color: #ea580c; }
  .badge-medium { background: #fef9c3; color: #ca8a04; }
  .badge-clear { background: #dcfce7; color: #16a34a; }
  .rec-card { border-left: 4px solid #ea580c; background: #fff7ed; padding: 12px 16px; margin-bottom: 12px; border-radius: 0 6px 6px 0; }
  .rec-rank { font-weight: 700; color: #c2410c; margin-right: 8px; }
  .map-wrapper { margin-top: 20px; border-radius: 8px; overflow: hidden; }
  @media print {
    body { padding: 0; background: #ffffff; }
    .report-container { box-shadow: none; padding: 0; }
    .rec-card { break-inside: avoid; }
  }
</style>
</head>
<body>
<div class="report-container">
  <h1>SAR SITUATIONAL REPORT: ` + html.EscapeString(strings.ToUpper(r.MissionID)) + `</h1>
  
  <div class="meta-grid">
    <div class="metric-card">
      <div class="metric-label">Area Coverage</div>
      <div class="metric-val">` + fmt.Sprintf("%.1f%%", r.AreaCoveredPercent) + `</div>
    </div>
    <div class="metric-card">
      <div class="metric-label">Survivors Located</div>
      <div class="metric-val">` + fmt.Sprintf("%d", r.TotalSurvivors) + `</div>
    </div>
    <div class="metric-card">
      <div class="metric-label">Identified Hazards</div>
      <div class="metric-val">` + fmt.Sprintf("%d", r.TotalHazards) + `</div>
    </div>
    <div class="metric-card">
      <div class="metric-label">System Mode</div>
      <div class="metric-val">` + html.EscapeString(strings.ToUpper(r.SystemHealth.CurrentMode)) + `</div>
    </div>
  </div>

  <h2>Tactical Vector Overview</h2>
  <div class="map-wrapper">
    ` + r.StaticMapSVG + `
  </div>

  <h2>Prioritized Rescue Recommendations</h2>
`)

	if len(r.Recommendations) == 0 {
		sb.WriteString(`<p>No active rescue actions required at this time.</p>`)
	} else {
		for _, rec := range r.Recommendations {
			sb.WriteString(fmt.Sprintf(`
  <div class="rec-card">
    <span class="rec-rank">PRIORITY #%d</span> <strong>Survivor %s</strong> (Score: %.2f)
    <div style="margin-top: 6px; font-size: 13px; color: #334155;">%s</div>
    <div style="margin-top: 4px; font-size: 12px; color: #64748b;">Route Status: <strong>%s</strong> | Position: %.5f, %.5f</div>
  </div>`,
				rec.Rank, html.EscapeString(rec.SurvivorID), rec.PriorityScore,
				html.EscapeString(rec.Rationale), html.EscapeString(rec.RouteObstruction),
				rec.Lat, rec.Lon,
			))
		}
	}

	sb.WriteString(`
  <h2>Survivor Manifest</h2>
  <table>
    <thead>
      <tr>
        <th>Survivor ID</th>
        <th>Position</th>
        <th>Confidence</th>
        <th>Modalities</th>
        <th>Observations</th>
        <th>Score</th>
        <th>Status</th>
      </tr>
    </thead>
    <tbody>
`)

	for _, s := range r.Survivors {
		sb.WriteString(fmt.Sprintf(`
      <tr>
        <td><strong>%s</strong></td>
        <td>%.5f, %.5f</td>
        <td>%.1f%%</td>
        <td>%s</td>
        <td>%d</td>
        <td>%.2f</td>
        <td><span class="badge badge-clear">%s</span></td>
      </tr>`,
			html.EscapeString(s.ID), s.Lat, s.Lon, s.Confidence*100,
			html.EscapeString(strings.Join(s.Modalities, ", ")),
			s.Observations, s.PriorityScore, html.EscapeString(s.Status),
		))
	}

	sb.WriteString(`
    </tbody>
  </table>

  <h2>Hazard Zones</h2>
  <table>
    <thead>
      <tr>
        <th>Zone ID</th>
        <th>Hazard Type</th>
        <th>Position</th>
        <th>Buffer Radius</th>
        <th>Severity</th>
      </tr>
    </thead>
    <tbody>
`)

	for _, h := range r.HazardZones {
		sb.WriteString(fmt.Sprintf(`
      <tr>
        <td>%s</td>
        <td><strong>%s</strong></td>
        <td>%.5f, %.5f</td>
        <td>%.0fm</td>
        <td><span class="badge badge-%s">%s</span></td>
      </tr>`,
			html.EscapeString(h.ID), html.EscapeString(strings.ToUpper(h.Class)),
			h.Lat, h.Lon, h.RadiusM,
			html.EscapeString(h.Severity), html.EscapeString(h.Severity),
		))
	}

	sb.WriteString(`
    </tbody>
  </table>
  
  <p style="margin-top: 36px; font-size: 11px; color: #94a3b8; text-align: center;">
    Generated automatically by SAR_DRONE Fusion Engine on ` + r.GeneratedAt.Format(time.RFC1123) + `
  </p>
</div>
</body>
</html>`)

	return sb.String()
}
