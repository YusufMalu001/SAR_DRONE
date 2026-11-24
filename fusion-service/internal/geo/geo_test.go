package geo

import (
	"math"
	"testing"
)

func TestProjectRay_HandComputedGeometries(t *testing.T) {
	cam := CameraModel{
		HFOVDeg: 90.0, // tan(45) = 1.0
		VFOVDeg: 60.0, // tan(30) = 1/sqrt(3) ~= 0.57735
	}
	droneLat := 20.0000
	droneLon := 78.0000
	alt := 50.0

	// 1. Nadir boresight (center of image 0.5, 0.5, zero attitude)
	poseNadir := DronePose{
		Lat:        droneLat,
		Lon:        droneLon,
		Alt:        alt,
		HeadingDeg: 0.0,
		PitchDeg:   0.0,
		RollDeg:    0.0,
	}
	ptCenter, err := ProjectRay(0.5, 0.5, cam, poseNadir)
	if err != nil {
		t.Fatalf("nadir center error: %v", err)
	}
	distCenter := HaversineMeters(ptCenter, Point{Lat: droneLat, Lon: droneLon})
	if distCenter > 0.01 {
		t.Errorf("expected nadir center ground distance ~= 0, got %f meters", distCenter)
	}

	// 2. Top-center of image (0.5, 0.0): pointing straight forward (North)
	ptTop, err := ProjectRay(0.5, 0.0, cam, poseNadir)
	if err != nil {
		t.Fatalf("top center error: %v", err)
	}
	expectedNorthMeters := alt * math.Tan(deg2rad(30.0)) // ~28.8675 meters
	distNorth := (ptTop.Lat - droneLat) * metersPerDegreeLat
	if math.Abs(distNorth-expectedNorthMeters) > 0.05 {
		t.Errorf("expected forward distance %f m, got %f m", expectedNorthMeters, distNorth)
	}

	// 3. Right-center of image (1.0, 0.5): pointing straight right (East)
	ptRight, err := ProjectRay(1.0, 0.5, cam, poseNadir)
	if err != nil {
		t.Fatalf("right center error: %v", err)
	}
	expectedEastMeters := alt * math.Tan(deg2rad(45.0)) // 50.0 meters
	metersPerDegLon := metersPerDegreeLat * math.Cos(deg2rad(droneLat))
	distEast := (ptRight.Lon - droneLon) * metersPerDegLon
	if math.Abs(distEast-expectedEastMeters) > 0.05 {
		t.Errorf("expected right distance %f m, got %f m", expectedEastMeters, distEast)
	}

	// 4. Rotated Heading = 90 degrees (East)
	// Top-center (forward) should now project East!
	poseEast := poseNadir
	poseEast.HeadingDeg = 90.0
	ptRotated, err := ProjectRay(0.5, 0.0, cam, poseEast)
	if err != nil {
		t.Fatalf("rotated heading error: %v", err)
	}
	distRotEast := (ptRotated.Lon - droneLon) * metersPerDegLon
	distRotNorth := (ptRotated.Lat - droneLat) * metersPerDegreeLat
	if math.Abs(distRotEast-expectedNorthMeters) > 0.05 {
		t.Errorf("expected forward along East heading to be %f m, got %f m", expectedNorthMeters, distRotEast)
	}
	if math.Abs(distRotNorth) > 0.05 {
		t.Errorf("expected North displacement to be ~0, got %f m", distRotNorth)
	}

	// 5. Oblique mounting: MountPitch = 45 degrees
	// Center ray tilts 45 deg forward => ground distance = alt * tan(45) = 50.0m North
	camOblique := cam
	camOblique.MountPitchDeg = 45.0
	ptOblique, err := ProjectRay(0.5, 0.5, camOblique, poseNadir)
	if err != nil {
		t.Fatalf("oblique error: %v", err)
	}
	distOblique := (ptOblique.Lat - droneLat) * metersPerDegreeLat
	if math.Abs(distOblique-50.0) > 0.05 {
		t.Errorf("expected oblique forward distance 50.0m, got %f m", distOblique)
	}
}

func TestProjectRay_PropertyInvariants(t *testing.T) {
	cam := CameraModel{HFOVDeg: 80.0, VFOVDeg: 50.0}
	basePose := DronePose{
		Lat:        25.0000,
		Lon:        85.0000,
		Alt:        40.0,
		HeadingDeg: 45.0,
		PitchDeg:   -2.0,
		RollDeg:    3.0,
	}

	// Invariant 1: Linear altitude scaling for any ray offset
	u, v := 0.7, 0.3
	pt1, err := ProjectRay(u, v, cam, basePose)
	if err != nil {
		t.Fatal(err)
	}
	dist1 := HaversineMeters(Point{Lat: basePose.Lat, Lon: basePose.Lon}, pt1)

	poseDoubleAlt := basePose
	poseDoubleAlt.Alt = basePose.Alt * 2.0
	pt2, err := ProjectRay(u, v, cam, poseDoubleAlt)
	if err != nil {
		t.Fatal(err)
	}
	dist2 := HaversineMeters(Point{Lat: basePose.Lat, Lon: basePose.Lon}, pt2)

	if math.Abs(dist2-dist1*2.0)/dist2 > 0.01 {
		t.Errorf("altitude scaling invariant violated: dist1=%f, dist2=%f (expected ~2x)", dist1, dist2)
	}

	// Invariant 2: Heading rotation invariance (magnitude of ground displacement is invariant to heading)
	poseRot1 := basePose
	poseRot1.HeadingDeg = 0.0
	ptH1, _ := ProjectRay(u, v, cam, poseRot1)
	mag1 := HaversineMeters(Point{Lat: basePose.Lat, Lon: basePose.Lon}, ptH1)

	poseRot2 := basePose
	poseRot2.HeadingDeg = 137.0
	ptH2, _ := ProjectRay(u, v, cam, poseRot2)
	mag2 := HaversineMeters(Point{Lat: basePose.Lat, Lon: basePose.Lon}, ptH2)

	if math.Abs(mag1-mag2) > 0.05 {
		t.Errorf("heading rotation invariance violated: mag1=%f, mag2=%f", mag1, mag2)
	}

	// Invariant 3: Translation invariance
	deltaLat := 0.5
	deltaLon := -0.3
	poseShifted := basePose
	poseShifted.Lat += deltaLat
	poseShifted.Lon += deltaLon
	ptShifted, _ := ProjectRay(u, v, cam, poseShifted)

	if math.Abs((ptShifted.Lat-pt1.Lat)-deltaLat) > 1e-6 || math.Abs((ptShifted.Lon-pt1.Lon)-deltaLon) > 1e-6 {
		t.Errorf("translation invariance violated")
	}
}

func TestGroundContactPoint(t *testing.T) {
	bbox := BBox{X: 0.2, Y: 0.3, W: 0.1, H: 0.2}

	// Person uses bottom-center
	uP, vP := GroundContactPoint(bbox, "person")
	if math.Abs(uP-0.25) > 1e-6 || math.Abs(vP-0.50) > 1e-6 {
		t.Errorf("expected person contact (0.25, 0.50), got (%f, %f)", uP, vP)
	}

	// Hazard uses center
	uH, vH := GroundContactPoint(bbox, "flood")
	if math.Abs(uH-0.25) > 1e-6 || math.Abs(vH-0.40) > 1e-6 {
		t.Errorf("expected hazard contact (0.25, 0.40), got (%f, %f)", uH, vH)
	}
}
