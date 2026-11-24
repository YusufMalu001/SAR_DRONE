package telemetry

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestInterpolate_ExactAndMidpoint(t *testing.T) {
	buf := NewBuffer(10*time.Second, 100)
	base := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

	s1 := Sample{
		Lat:     20.0000,
		Lon:     78.0000,
		Alt:     50.0,
		Heading: 350.0,
		Pitch:   -5.0,
		Roll:    2.0,
		At:      base,
	}
	s2 := Sample{
		Lat:     20.0010,
		Lon:     78.0020,
		Alt:     60.0,
		Heading: 10.0, // Crossing 360/0 degree boundary (+20 degrees change)
		Pitch:   5.0,
		Roll:    -2.0,
		At:      base.Add(1 * time.Second),
	}

	buf.Add(s1)
	buf.Add(s2)

	// 1. Exact match s1
	exact1, err := buf.Interpolate(base, 2*time.Second, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("expected no error at exact s1, got %v", err)
	}
	if exact1.Lat != 20.0000 || exact1.Alt != 50.0 || exact1.Heading != 350.0 {
		t.Errorf("unexpected exact1: %+v", exact1)
	}

	// 2. Exact match s2
	exact2, err := buf.Interpolate(base.Add(1*time.Second), 2*time.Second, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("expected no error at exact s2, got %v", err)
	}
	if exact2.Lat != 20.0010 || exact2.Alt != 60.0 || exact2.Heading != 10.0 {
		t.Errorf("unexpected exact2: %+v", exact2)
	}

	// 3. Midpoint (t = +500ms)
	mid, err := buf.Interpolate(base.Add(500*time.Millisecond), 2*time.Second, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("expected no error at midpoint, got %v", err)
	}
	if math.Abs(mid.Lat-20.0005) > 1e-6 {
		t.Errorf("expected lat 20.0005, got %f", mid.Lat)
	}
	if math.Abs(mid.Lon-78.0010) > 1e-6 {
		t.Errorf("expected lon 78.0010, got %f", mid.Lon)
	}
	if math.Abs(mid.Alt-55.0) > 1e-6 {
		t.Errorf("expected alt 55.0, got %f", mid.Alt)
	}
	if math.Abs(mid.Pitch-0.0) > 1e-6 {
		t.Errorf("expected pitch 0.0, got %f", mid.Pitch)
	}
	if math.Abs(mid.Roll-0.0) > 1e-6 {
		t.Errorf("expected roll 0.0, got %f", mid.Roll)
	}
	// Heading should wrap across 360/0: halfway from 350 to 10 is 0.0!
	if math.Abs(mid.Heading-0.0) > 1e-6 && math.Abs(mid.Heading-360.0) > 1e-6 {
		t.Errorf("expected heading 0.0 deg across 360/0 boundary, got %f", mid.Heading)
	}
}

func TestInterpolate_GapsAndClockSkew(t *testing.T) {
	buf := NewBuffer(10*time.Second, 100)
	base := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

	// Empty buffer error
	_, err := buf.Interpolate(base, 1*time.Second, 100*time.Millisecond)
	if !errors.Is(err, ErrBufferEmpty) {
		t.Errorf("expected ErrBufferEmpty, got %v", err)
	}

	buf.Add(Sample{Lat: 10, Lon: 10, Alt: 20, Heading: 0, At: base})
	buf.Add(Sample{Lat: 10, Lon: 10, Alt: 20, Heading: 0, At: base.Add(5 * time.Second)}) // 5s gap!

	// 1. Gap test: gap between samples is 5s, max allowed is 1s
	_, err = buf.Interpolate(base.Add(2*time.Second), 1*time.Second, 100*time.Millisecond)
	if !errors.Is(err, ErrTelemetryGap) {
		t.Errorf("expected ErrTelemetryGap, got %v", err)
	}

	// 2. Clock skew test: timestamp earlier than oldest by 300ms (> 100ms tolerance)
	_, err = buf.Interpolate(base.Add(-300*time.Millisecond), 1*time.Second, 100*time.Millisecond)
	if !errors.Is(err, ErrClockSkew) {
		t.Errorf("expected ErrClockSkew for early timestamp, got %v", err)
	}

	// 3. Clock skew test: timestamp within 50ms of oldest (< 100ms tolerance)
	skewOk, err := buf.Interpolate(base.Add(-50*time.Millisecond), 1*time.Second, 100*time.Millisecond)
	if err != nil {
		t.Errorf("expected success for skew within tolerance, got %v", err)
	}
	if skewOk.Lat != 10 {
		t.Errorf("expected sample lat 10, got %f", skewOk.Lat)
	}

	// 4. Clock skew test: timestamp later than newest by 500ms (> 100ms tolerance)
	_, err = buf.Interpolate(base.Add(5500*time.Millisecond), 1*time.Second, 100*time.Millisecond)
	if !errors.Is(err, ErrClockSkew) {
		t.Errorf("expected ErrClockSkew for late timestamp, got %v", err)
	}
}
