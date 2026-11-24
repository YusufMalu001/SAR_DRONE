package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/geo"
	"sar-drone-prototype/fusion-service/internal/route"
)

// PlanSafeRoute handles GET /mission/routes/safe and POST /mission/routes/plan
func PlanSafeRoute(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var startLat, startLon, destLat, destLon float64
		var err error

		if r.Method == http.MethodPost {
			var body struct {
				StartLat float64 `json:"start_lat"`
				StartLon float64 `json:"start_lon"`
				DestLat  float64 `json:"dest_lat"`
				DestLon  float64 `json:"dest_lon"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			startLat, startLon, destLat, destLon = body.StartLat, body.StartLon, body.DestLat, body.DestLon
		} else {
			q := r.URL.Query()
			startLat, err = strconv.ParseFloat(q.Get("start_lat"), 64)
			if err != nil {
				http.Error(w, "invalid start_lat", http.StatusBadRequest)
				return
			}
			startLon, err = strconv.ParseFloat(q.Get("start_lon"), 64)
			if err != nil {
				http.Error(w, "invalid start_lon", http.StatusBadRequest)
				return
			}
			destLat, err = strconv.ParseFloat(q.Get("dest_lat"), 64)
			if err != nil {
				http.Error(w, "invalid dest_lat", http.StatusBadRequest)
				return
			}
			destLon, err = strconv.ParseFloat(q.Get("dest_lon"), 64)
			if err != nil {
				http.Error(w, "invalid dest_lon", http.StatusBadRequest)
				return
			}
		}

		hazards, err := loadHazardZones(r.Context(), pool)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load hazard zones: %v", err), http.StatusInternalServerError)
			return
		}

		startPt := geo.Point{Lat: startLat, Lon: startLon}
		destPt := geo.Point{Lat: destLat, Lon: destLon}

		routeRes := route.PlanSafeRoute(startPt, destPt, hazards, "route_custom")

		collection := map[string]interface{}{
			"type":     "FeatureCollection",
			"features": []interface{}{routeRes.ToGeoJSONFeature()},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(collection)
	}
}

// SurvivorSafeRoutes handles GET /mission/routes/survivors
func SurvivorSafeRoutes(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		stagingLat := 20.0000
		stagingLon := 78.0000

		if sLat := q.Get("staging_lat"); sLat != "" {
			if v, err := strconv.ParseFloat(sLat, 64); err == nil {
				stagingLat = v
			}
		}
		if sLon := q.Get("staging_lon"); sLon != "" {
			if v, err := strconv.ParseFloat(sLon, 64); err == nil {
				stagingLon = v
			}
		}

		hazards, err := loadHazardZones(r.Context(), pool)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load hazard zones: %v", err), http.StatusInternalServerError)
			return
		}

		survivors, err := db.ListActiveSurvivors(r.Context(), pool)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list survivors: %v", err), http.StatusInternalServerError)
			return
		}

		stagingPt := geo.Point{Lat: stagingLat, Lon: stagingLon}
		features := make([]interface{}, 0, len(survivors))

		for _, s := range survivors {
			destPt := geo.Point{Lat: s.Lat, Lon: s.Lon}
			routeID := fmt.Sprintf("route_to_%s", s.ID)
			routeRes := route.PlanSafeRoute(stagingPt, destPt, hazards, routeID)
			features = append(features, routeRes.ToGeoJSONFeature())
		}

		collection := map[string]interface{}{
			"type":     "FeatureCollection",
			"features": features,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(collection)
	}
}

func loadHazardZones(ctx context.Context, pool *pgxpool.Pool) ([]route.HazardZone, error) {
	rows, err := db.ListActiveHazardPolygons(ctx, pool)
	if err != nil {
		return nil, err
	}

	var zones []route.HazardZone
	for _, row := range rows {
		var geom struct {
			Type        string        `json:"type"`
			Coordinates [][][2]float64 `json:"coordinates"`
		}
		if err := json.Unmarshal([]byte(row.GeoJSON), &geom); err != nil {
			continue
		}

		if geom.Type == "Polygon" && len(geom.Coordinates) > 0 {
			pts := make([]geo.Point, len(geom.Coordinates[0]))
			for i, c := range geom.Coordinates[0] {
				pts[i] = geo.Point{Lon: c[0], Lat: c[1]}
			}
			zones = append(zones, route.HazardZone{
				ID:           row.ID,
				Class:        row.Class,
				Severity:     row.Severity,
				BufferMeters: row.BufferMeters,
				Polygon:      pts,
			})
		}
	}
	return zones, nil
}
