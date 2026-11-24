// Package geo provides full 3D camera ray-casting geo-projection,
// coordinate transformations, and spatial geometry utilities for drone SAR operations.
package geo

import (
	"errors"
	"math"
)

const (
	earthRadiusMeters  = 6371000.0
	metersPerDegreeLat = 111320.0
)

var (
	ErrRayAboveHorizon = errors.New("camera ray points at or above horizon; does not intersect ground")
	ErrDroneAtGround   = errors.New("drone altitude above terrain is zero or negative")
)

// Point is a WGS84 lat/lon pair.
type Point struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// CameraModel defines the full intrinsic and extrinsic mounting parameters.
type CameraModel struct {
	HFOVDeg       float64 // Horizontal field of view in degrees
	VFOVDeg       float64 // Vertical field of view in degrees
	MountPitchDeg float64 // Camera tilt from nadir (0 = straight down, >0 = tilted forward)
	MountRollDeg  float64 // Camera mounting roll offset
	MountYawDeg   float64 // Camera mounting yaw offset
}

// DronePose defines the position and attitude of the drone at frame capture.
type DronePose struct {
	Lat           float64 // WGS84 latitude in degrees
	Lon           float64 // WGS84 longitude in degrees
	Alt           float64 // Altitude in meters
	TerrainOffset float64 // Ground elevation in meters (AGL = Alt - TerrainOffset)
	HeadingDeg    float64 // Yaw/Heading in degrees clockwise from North [0, 360)
	PitchDeg      float64 // Aircraft pitch in degrees (nose up > 0)
	RollDeg       float64 // Aircraft roll in degrees (right wing down > 0)
}

// BBox is a normalized [0,1] axis-aligned bounding box.
type BBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// GroundContactPoint calculates the normalized (u, v) image coordinate representing
// the ground contact location of a detected object.
// For upright 3D entities (e.g. "person"), the ground contact point is the bottom center.
// For planar ground hazards ("flood", "fire", "debris", "structural_damage"), it uses the center.
func GroundContactPoint(bbox BBox, class string) (u, v float64) {
	u = bbox.X + bbox.W*0.5
	if class == "person" {
		v = bbox.Y + bbox.H
	} else {
		v = bbox.Y + bbox.H*0.5
	}
	// Clamp to [0, 1]
	if u < 0 {
		u = 0
	} else if u > 1 {
		u = 1
	}
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}
	return u, v
}

// ProjectRay casts an image point (u, v in [0, 1]) onto the ground plane using the full 3D camera model,
// mount geometry, and drone attitude.
func ProjectRay(u, v float64, cam CameraModel, pose DronePose) (Point, error) {
	hAGL := pose.Alt - pose.TerrainOffset
	if hAGL <= 0 {
		return Point{Lat: pose.Lat, Lon: pose.Lon}, ErrDroneAtGround
	}

	// 1. Ray in camera optical coordinates
	// Optical axis = +Z, Right = +X, Down = +Y
	tanHalfH := math.Tan(deg2rad(cam.HFOVDeg * 0.5))
	tanHalfV := math.Tan(deg2rad(cam.VFOVDeg * 0.5))

	xCam := 2.0 * (u - 0.5) * tanHalfH
	yCam := 2.0 * (v - 0.5) * tanHalfV

	// 2. Map camera optical frame into body mount frame:
	// In nadir mount: image up (-yCam) is forward (+X), image right (+xCam) is right (+Y), optical axis is down (+Z).
	vMount := [3]float64{-yCam, xCam, 1.0}

	// 3. Apply Mount Pitch/Roll/Yaw rotations:
	// Mounting pitch tilts optical axis forward toward aircraft nose
	mountRot := eulerToMatrix(cam.MountRollDeg, cam.MountPitchDeg, cam.MountYawDeg)
	vBody := matVecMul(mountRot, vMount)

	// 4. Apply Drone Attitude (Aircraft FRD -> World NED):
	// Standard aerospace sequence: Yaw (Z), Pitch (Y), Roll (X)
	bodyToNed := eulerToMatrix(pose.RollDeg, pose.PitchDeg, pose.HeadingDeg)
	vNed := matVecMul(bodyToNed, vBody)

	dN, dE, dD := vNed[0], vNed[1], vNed[2]

	// 5. Intersect with flat ground plane at Z_ned = 0
	// Ray: P(t) = (0, 0, -hAGL) + t * (dN, dE, dD)
	// Ground at Z = 0 => t * dD = hAGL => t = hAGL / dD
	if dD <= 1e-5 {
		return Point{Lat: pose.Lat, Lon: pose.Lon}, ErrRayAboveHorizon
	}

	t := hAGL / dD
	deltaN := t * dN
	deltaE := t * dE

	// 6. Convert local NED (meters) to WGS84 lat/lon
	metersPerDegLon := metersPerDegreeLat * math.Cos(deg2rad(pose.Lat))
	lat := pose.Lat + (deltaN / metersPerDegreeLat)
	lon := pose.Lon + (deltaE / metersPerDegLon)

	return Point{Lat: lat, Lon: lon}, nil
}

// ProjectDetection is the convenience function mapping a detection's bounding box to ground lat/lon.
func ProjectDetection(bboxCenterX, bboxCenterY, hfovDeg, vfovDeg, altitudeMeters, headingDeg, droneLat, droneLon float64) Point {
	cam := CameraModel{
		HFOVDeg: hfovDeg,
		VFOVDeg: vfovDeg,
	}
	pose := DronePose{
		Lat:        droneLat,
		Lon:        droneLon,
		Alt:        altitudeMeters,
		HeadingDeg: headingDeg,
	}
	pt, err := ProjectRay(bboxCenterX, bboxCenterY, cam, pose)
	if err != nil {
		return Point{Lat: droneLat, Lon: droneLon}
	}
	return pt
}

// eulerToMatrix computes the 3x3 rotation matrix for aerospace Euler angles:
// R = Rz(yaw) * Ry(pitch) * Rx(roll)
func eulerToMatrix(rollDeg, pitchDeg, yawDeg float64) [3][3]float64 {
	r := deg2rad(rollDeg)
	p := deg2rad(pitchDeg)
	y := deg2rad(yawDeg)

	cr, sr := math.Cos(r), math.Sin(r)
	cp, sp := math.Cos(p), math.Sin(p)
	cy, sy := math.Cos(y), math.Sin(y)

	return [3][3]float64{
		{cy*cp, cy*sp*sr - sy*cr, cy*sp*cr + sy*sr},
		{sy*cp, sy*sp*sr + cy*cr, sy*sp*cr - cy*sr},
		{-sp, cp * sr, cp * cr},
	}
}

func matVecMul(m [3][3]float64, v [3]float64) [3]float64 {
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
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

// PointToSegmentMeters returns shortest distance from point p to line segment ab in meters.
func PointToSegmentMeters(p, a, b Point) float64 {
	metersPerDegLon := metersPerDegreeLat * math.Cos(deg2rad(a.Lat))
	ax, ay := 0.0, 0.0
	bx := (b.Lon - a.Lon) * metersPerDegLon
	by := (b.Lat - a.Lat) * metersPerDegreeLat
	px := (p.Lon - a.Lon) * metersPerDegLon
	py := (p.Lat - a.Lat) * metersPerDegreeLat

	dx, dy := bx-ax, by-ay
	lengthSq := dx*dx + dy*dy
	if lengthSq == 0 {
		return math.Hypot(px-ax, py-ay)
	}

	t := ((px-ax)*dx + (py-ay)*dy) / lengthSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	projX := ax + t*dx
	projY := ay + t*dy
	return math.Hypot(px-projX, py-projY)
}

func deg2rad(d float64) float64 {
	return d * (math.Pi / 180.0)
}

func rad2deg(r float64) float64 {
	return r * (180.0 / math.Pi)
}
