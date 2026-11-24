package handlers

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServeTile_ServesCachedTileOffline(t *testing.T) {
	tmpDir := t.TempDir()
	tilePath := filepath.Join(tmpDir, "14", "11742", "7369.png")
	if err := os.MkdirAll(filepath.Dir(tilePath), 0755); err != nil {
		t.Fatalf("failed to create tile dir: %v", err)
	}

	// Create a synthetic PNG
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}
	if err := os.WriteFile(tilePath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write test tile: %v", err)
	}

	handler := ServeTile(tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/tiles/14/11742/7369.png", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png, got %s", ct)
	}
	if !bytes.Equal(rr.Body.Bytes(), buf.Bytes()) {
		t.Fatal("served body does not match cached tile bytes")
	}
}

func TestServeTile_FallbackWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	handler := ServeTile(tmpDir)

	// Non-cached coordinate
	req := httptest.NewRequest(http.MethodGet, "/tiles/16/9999/8888.png", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for fallback tile, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png for fallback, got %s", ct)
	}

	// Verify that the body is a valid PNG image
	decoded, err := png.Decode(rr.Body)
	if err != nil {
		t.Fatalf("fallback body is not a valid PNG: %v", err)
	}
	if decoded.Bounds().Dx() != 256 || decoded.Bounds().Dy() != 256 {
		t.Fatalf("expected 256x256 fallback tile, got %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}
