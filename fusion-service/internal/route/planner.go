package route

import (
	"container/heap"
	"math"

	"sar-drone-prototype/fusion-service/internal/geo"
)

// HazardZone represents a buffered hazard zone with polygon boundary and metadata.
type HazardZone struct {
	ID           string
	Class        string
	Severity     string
	BufferMeters float64
	Polygon      []geo.Point // Closed ring of WGS84 points
}

// RouteResult contains the computed safe or least-risk route.
type RouteResult struct {
	RouteID            string       `json:"route_id"`
	Safe               bool         `json:"safe"`
	Reason             *string      `json:"reason,omitempty"`
	LengthMeters       float64      `json:"length_meters"`
	RiskScore          float64      `json:"risk_score"`
	AvoidedHazards     []string     `json:"avoided_hazards"`
	IntersectedHazards []string     `json:"intersected_hazards"`
	Start              [2]float64   `json:"start"`       // [lon, lat]
	Destination        [2]float64   `json:"destination"` // [lon, lat]
	Waypoints          []geo.Point  `json:"waypoints"`
}

// GeoJSONFeature transforms RouteResult into a standard GeoJSON Feature.
func (r *RouteResult) ToGeoJSONFeature() map[string]interface{} {
	coords := make([][2]float64, len(r.Waypoints))
	for i, wp := range r.Waypoints {
		coords[i] = [2]float64{wp.Lon, wp.Lat}
	}

	props := map[string]interface{}{
		"route_id":            r.RouteID,
		"safe":                r.Safe,
		"length_meters":       r.LengthMeters,
		"risk_score":          r.RiskScore,
		"avoided_hazards":     r.AvoidedHazards,
		"intersected_hazards": r.IntersectedHazards,
		"start":               r.Start,
		"destination":         r.Destination,
	}
	if r.Reason != nil {
		props["reason"] = *r.Reason
	} else {
		props["reason"] = nil
	}

	return map[string]interface{}{
		"type": "Feature",
		"geometry": map[string]interface{}{
			"type":        "LineString",
			"coordinates": coords,
		},
		"properties": props,
	}
}

// PointInPolygon checks whether pt is inside poly using the ray-casting algorithm.
func PointInPolygon(pt geo.Point, poly []geo.Point) bool {
	if len(poly) < 3 {
		return false
	}
	inside := false
	j := len(poly) - 1
	for i := 0; i < len(poly); i++ {
		xi, yi := poly[i].Lon, poly[i].Lat
		xj, yj := poly[j].Lon, poly[j].Lat

		intersect := ((yi > pt.Lat) != (yj > pt.Lat)) &&
			(pt.Lon < (xj-xi)*(pt.Lat-yi)/(yj-yi)+xi)
		if intersect {
			inside = !inside
		}
		j = i
	}
	return inside
}

// DistanceToPolygonMeters calculates the shortest distance from pt to polygon edges.
func DistanceToPolygonMeters(pt geo.Point, poly []geo.Point) float64 {
	if len(poly) < 2 {
		return 0
	}
	if PointInPolygon(pt, poly) {
		return 0.0
	}
	minDist := math.MaxFloat64
	for i := 0; i < len(poly)-1; i++ {
		d := geo.PointToSegmentMeters(pt, poly[i], poly[i+1])
		if d < minDist {
			minDist = d
		}
	}
	return minDist
}

// LineIntersectsPolygon checks if a line segment between a and b intersects any polygon edge.
func LineIntersectsPolygon(a, b geo.Point, poly []geo.Point) bool {
	if PointInPolygon(a, poly) || PointInPolygon(b, poly) {
		return true
	}
	// Check edge crossings
	for i := 0; i < len(poly)-1; i++ {
		p1, p2 := poly[i], poly[i+1]
		if segmentsIntersect(a, b, p1, p2) {
			return true
		}
	}
	return false
}

func segmentsIntersect(a, b, c, d geo.Point) bool {
	ccw := func(p1, p2, p3 geo.Point) bool {
		return (p3.Lat-p1.Lat)*(p2.Lon-p1.Lon) > (p2.Lat-p1.Lat)*(p3.Lon-p1.Lon)
	}
	return (ccw(a, c, d) != ccw(b, c, d)) && (ccw(a, b, c) != ccw(a, b, d))
}

// A* Grid structures
type astarNode struct {
	r, c        int
	gCost       float64
	fCost       float64
	parent      *astarNode
	index       int // for heap
	inHazard    bool
	hazardIDs   []string
}

type priorityQueue []*astarNode

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].fCost < pq[j].fCost }
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*astarNode)
	item.index = n
	*pq = append(*pq, item)
}
func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// PlanSafeRoute plans an optimal route from start to dest avoiding buffered hazard zones.
// If a completely safe path exists, it returns safe=true.
// If no safe path exists, it falls back to least-risk mode and returns safe=false with risk score and intersected hazards.
func PlanSafeRoute(start, dest geo.Point, hazards []HazardZone, routeID string) RouteResult {
	result := RouteResult{
		RouteID:     routeID,
		Start:       [2]float64{start.Lon, start.Lat},
		Destination: [2]float64{dest.Lon, dest.Lat},
	}

	// 1. Determine bounding box for grid
	minLat := math.Min(start.Lat, dest.Lat)
	maxLat := math.Max(start.Lat, dest.Lat)
	minLon := math.Min(start.Lon, dest.Lon)
	maxLon := math.Max(start.Lon, dest.Lon)

	for _, hz := range hazards {
		for _, pt := range hz.Polygon {
			minLat = math.Min(minLat, pt.Lat)
			maxLat = math.Max(maxLat, pt.Lat)
			minLon = math.Min(minLon, pt.Lon)
			maxLon = math.Max(maxLon, pt.Lon)
		}
	}

	// Add margin (approx 50m)
	marginLat := 0.0005
	marginLon := 0.0005
	minLat -= marginLat
	maxLat += marginLat
	minLon -= marginLon
	maxLon += marginLon

	// Discrete grid dimensions (capped for predictable low latency)
	numRows := 60
	numCols := 60
	deltaLat := (maxLat - minLat) / float64(numRows)
	deltaLon := (maxLon - minLon) / float64(numCols)

	coordToCell := func(p geo.Point) (int, int) {
		r := int((p.Lat - minLat) / deltaLat)
		c := int((p.Lon - minLon) / deltaLon)
		if r < 0 {
			r = 0
		} else if r >= numRows {
			r = numRows - 1
		}
		if c < 0 {
			c = 0
		} else if c >= numCols {
			c = numCols - 1
		}
		return r, c
	}

	cellToCoord := func(r, c int) geo.Point {
		lat := minLat + (float64(r)+0.5)*deltaLat
		lon := minLon + (float64(c)+0.5)*deltaLon
		return geo.Point{Lat: lat, Lon: lon}
	}

	startR, startC := coordToCell(start)
	goalR, goalC := coordToCell(dest)

	// Pre-evaluate cell hazard occupancy
	type cellMeta struct {
		isBlocked bool
		hazards   []string
		penalty   float64
	}
	grid := make([][]cellMeta, numRows)
	for r := 0; r < numRows; r++ {
		grid[r] = make([]cellMeta, numCols)
		for c := 0; c < numCols; c++ {
			pt := cellToCoord(r, c)
			for _, hz := range hazards {
				if PointInPolygon(pt, hz.Polygon) {
					grid[r][c].isBlocked = true
					grid[r][c].hazards = append(grid[r][c].hazards, hz.ID)
					sevMult := 1.0
					if hz.Severity == "critical" {
						sevMult = 5.0
					} else if hz.Severity == "high" {
						sevMult = 3.0
					}
					grid[r][c].penalty += 1000.0 * sevMult
				}
			}
		}
	}

	// Helper to run A* search
	runAStar := func(strict bool) ([]*astarNode, bool) {
		if strict && (grid[startR][startC].isBlocked || grid[goalR][goalC].isBlocked) {
			return nil, false
		}

		pq := make(priorityQueue, 0)
		heap.Init(&pq)

		costSoFar := make(map[[2]int]float64)
		startNode := &astarNode{
			r:         startR,
			c:         startC,
			gCost:     0,
			fCost:     math.Hypot(float64(goalR-startR), float64(goalC-startC)),
			hazardIDs: grid[startR][startC].hazards,
		}
		heap.Push(&pq, startNode)
		costSoFar[[2]int{startR, startC}] = 0

		var goalNode *astarNode
		directions := [][2]int{
			{-1, 0}, {1, 0}, {0, -1}, {0, 1},
			{-1, -1}, {-1, 1}, {1, -1}, {1, 1},
		}

		for pq.Len() > 0 {
			curr := heap.Pop(&pq).(*astarNode)

			if curr.r == goalR && curr.c == goalC {
				goalNode = curr
				break
			}

			for _, d := range directions {
				nr, nc := curr.r+d[0], curr.c+d[1]
				if nr < 0 || nr >= numRows || nc < 0 || nc >= numCols {
					continue
				}

				if strict && grid[nr][nc].isBlocked {
					continue
				}

				stepDist := 1.0
				if d[0] != 0 && d[1] != 0 {
					stepDist = 1.41421356
				}
				stepCost := stepDist
				if !strict && grid[nr][nc].isBlocked {
					stepCost += grid[nr][nc].penalty
				}

				newG := curr.gCost + stepCost
				key := [2]int{nr, nc}

				if prevG, exists := costSoFar[key]; !exists || newG < prevG {
					costSoFar[key] = newG
					h := math.Hypot(float64(goalR-nr), float64(goalC-nc))
					nextHaz := curr.hazardIDs
					if len(grid[nr][nc].hazards) > 0 {
						merged := append([]string{}, curr.hazardIDs...)
						for _, hzID := range grid[nr][nc].hazards {
							found := false
							for _, id := range merged {
								if id == hzID {
									found = true
									break
								}
							}
							if !found {
								merged = append(merged, hzID)
							}
						}
						nextHaz = merged
					}

					next := &astarNode{
						r:         nr,
						c:         nc,
						gCost:     newG,
						fCost:     newG + h,
						parent:    curr,
						inHazard:  grid[nr][nc].isBlocked,
						hazardIDs: nextHaz,
					}
					heap.Push(&pq, next)
				}
			}
		}

		if goalNode == nil {
			return nil, false
		}

		// Backtrack path
		path := []*astarNode{}
		for cur := goalNode; cur != nil; cur = cur.parent {
			path = append([]*astarNode{cur}, path...)
		}
		return path, true
	}

	// Try Strict Search first
	pathNodes, foundSafe := runAStar(true)

	if foundSafe {
		result.Safe = true
		rawWps := make([]geo.Point, len(pathNodes))
		for i, n := range pathNodes {
			rawWps[i] = cellToCoord(n.r, n.c)
		}
		// Clamp endpoints to exact start and destination
		rawWps[0] = start
		rawWps[len(rawWps)-1] = dest

		// Shortcut / smooth waypoints
		result.Waypoints = smoothPath(rawWps, hazards)
		result.LengthMeters = computePathLength(result.Waypoints)
		result.RiskScore = 0.0

		// Identify avoided hazards (hazards within 2x buffer of path)
		avoided := []string{}
		for _, hz := range hazards {
			for _, wp := range result.Waypoints {
				if DistanceToPolygonMeters(wp, hz.Polygon) <= hz.BufferMeters*2.0 {
					avoided = append(avoided, hz.ID)
					break
				}
			}
		}
		result.AvoidedHazards = avoided
		result.IntersectedHazards = []string{}
		return result
	}

	// No safe path found -> Run least-risk penalty mode
	pathNodes, foundRisk := runAStar(false)
	result.Safe = false
	reason := "no safe route exists: destination or passage fully obstructed by hazard zones"
	result.Reason = &reason

	if foundRisk {
		rawWps := make([]geo.Point, len(pathNodes))
		intersectedMap := make(map[string]bool)
		totalRisk := 0.0
		for i, n := range pathNodes {
			rawWps[i] = cellToCoord(n.r, n.c)
			if n.inHazard {
				for _, hID := range grid[n.r][n.c].hazards {
					intersectedMap[hID] = true
				}
				totalRisk += grid[n.r][n.c].penalty
			}
		}
		rawWps[0] = start
		rawWps[len(rawWps)-1] = dest

		result.Waypoints = rawWps
		result.LengthMeters = computePathLength(rawWps)
		result.RiskScore = totalRisk

		for hID := range intersectedMap {
			result.IntersectedHazards = append(result.IntersectedHazards, hID)
		}
	} else {
		// Even penalty mode couldn't bridge (e.g. out of grid)
		result.Waypoints = []geo.Point{start, dest}
		result.LengthMeters = geo.HaversineMeters(start, dest)
		result.RiskScore = 99999.0
	}

	return result
}

// smoothPath performs line-of-sight shortcutting across waypoints.
func smoothPath(wps []geo.Point, hazards []HazardZone) []geo.Point {
	if len(wps) <= 2 {
		return wps
	}
	smoothed := []geo.Point{wps[0]}
	current := 0

	for current < len(wps)-1 {
		next := len(wps) - 1
		for next > current+1 {
			// Check if line from wps[current] to wps[next] intersects any hazard
			intersects := false
			for _, hz := range hazards {
				if LineIntersectsPolygon(wps[current], wps[next], hz.Polygon) {
					intersects = true
					break
				}
			}
			if !intersects {
				break
			}
			next--
		}
		smoothed = append(smoothed, wps[next])
		current = next
	}
	return smoothed
}

func computePathLength(wps []geo.Point) float64 {
	total := 0.0
	for i := 0; i < len(wps)-1; i++ {
		total += geo.HaversineMeters(wps[i], wps[i+1])
	}
	return total
}
