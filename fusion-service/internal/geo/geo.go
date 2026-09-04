// Package geo converts a detection's bounding box into an approximate
// ground position using a simple pinhole camera model, and provides a
// haversine distance helper for proximity checks.
package geo

import "math"

const (
	earthRadiusMeters  = 6371000.0
	metersPerDegreeLat = 111320.0
)

// Point is a WGS84 lat/lon pair.
type Point struct {
	Lat float64
	Lon float64
}

// ProjectDetection converts a detection's normalized bbox center
// (bboxCenterX, bboxCenterY, both in [0,1]) into an approximate ground
// lat/lon.
//
// Model assumptions (documented since this is an approximation, not a
// calibrated camera projection):
//   - The camera is mounted looking straight down (nadir).
//   - The image's "up" direction (smaller Y) points toward the drone's
//     direction of travel (heading); "right" (larger X) points 90 degrees
//     clockwise from heading.
//   - hfovDeg/vfovDeg are the camera's full horizontal/vertical field of
//     view, so a detection at the edge of the frame is offset by half the
//     FOV from the boresight.
//   - The ground is locally flat at the drone's altitude above it (i.e.
//     altitudeMeters is height-above-ground, not above sea level).
func ProjectDetection(bboxCenterX, bboxCenterY, hfovDeg, vfovDeg, altitudeMeters, headingDeg, droneLat, droneLon float64) Point {
	if altitudeMeters <= 0 {
		return Point{Lat: droneLat, Lon: droneLon}
	}

	angleX := (bboxCenterX - 0.5) * hfovDeg
	angleY := (bboxCenterY - 0.5) * vfovDeg

	// Ground offsets in the drone's body frame, in meters.
	right := altitudeMeters * math.Tan(deg2rad(angleX))
	forward := altitudeMeters * math.Tan(deg2rad(-angleY))

	// Rotate from body frame (forward, right) into world frame (north, east).
	headingRad := deg2rad(headingDeg)
	north := forward*math.Cos(headingRad) - right*math.Sin(headingRad)
	east := forward*math.Sin(headingRad) + right*math.Cos(headingRad)

	deltaLat := north / metersPerDegreeLat
	deltaLon := east / (metersPerDegreeLat * math.Cos(deg2rad(droneLat)))

	return Point{Lat: droneLat + deltaLat, Lon: droneLon + deltaLon}
}

// HaversineMeters returns the great-circle distance between two points.
func HaversineMeters(a, b Point) float64 {
	lat1, lon1 := deg2rad(a.Lat), deg2rad(a.Lon)
	lat2, lon2 := deg2rad(b.Lat), deg2rad(b.Lon)
	dLat := lat2 - lat1
	dLon := lon2 - lon1

	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusMeters * c
}

// PointToSegmentMeters returns the shortest distance from p to the line
// segment ab, in meters. Projects all three points into a local flat-earth
// (ENU) plane centered on a — accurate for the short segments a drone
// flight path is made of, consistent with the flat-earth approximation
// ProjectDetection already uses.
func PointToSegmentMeters(p, a, b Point) float64 {
	ax, ay := 0.0, 0.0 // a is the local origin
	bx, by := toLocalMeters(a, b)
	px, py := toLocalMeters(a, p)

	return distancePointToSegment(px, py, ax, ay, bx, by)
}

// toLocalMeters projects p into a local flat-earth (x=east, y=north) plane
// in meters, relative to origin.
func toLocalMeters(origin, p Point) (x, y float64) {
	metersPerDegreeLon := metersPerDegreeLat * math.Cos(deg2rad(origin.Lat))
	x = (p.Lon - origin.Lon) * metersPerDegreeLon
	y = (p.Lat - origin.Lat) * metersPerDegreeLat
	return x, y
}

func distancePointToSegment(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	lengthSq := dx*dx + dy*dy
	if lengthSq == 0 {
		return math.Hypot(px-ax, py-ay)
	}

	// t is how far along ab the closest point to p falls; clamp to the
	// segment itself rather than the infinite line.
	t := ((px-ax)*dx + (py-ay)*dy) / lengthSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	closestX, closestY := ax+t*dx, ay+t*dy
	return math.Hypot(px-closestX, py-closestY)
}

func deg2rad(d float64) float64 {
	return d * math.Pi / 180
}
