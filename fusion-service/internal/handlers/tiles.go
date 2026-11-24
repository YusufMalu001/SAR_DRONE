package handlers

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ServeTile returns an http.HandlerFunc that serves pre-cached map tiles from tileDir,
// or generates a valid offline raster tile dynamically if not present in cache.
func ServeTile(tileDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Expected path format: /tiles/{z}/{x}/{y}.png
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 4 {
			http.Error(w, "invalid tile path format: /tiles/{z}/{x}/{y}.png", http.StatusBadRequest)
			return
		}

		zStr := parts[len(parts)-3]
		xStr := parts[len(parts)-2]
		yRaw := parts[len(parts)-1]
		yStr := strings.TrimSuffix(yRaw, ".png")

		z, errZ := strconv.Atoi(zStr)
		x, errX := strconv.Atoi(xStr)
		y, errY := strconv.Atoi(yStr)

		if errZ != nil || errX != nil || errY != nil {
			http.Error(w, "invalid tile coordinates", http.StatusBadRequest)
			return
		}

		tilePath := filepath.Join(tileDir, zStr, xStr, fmt.Sprintf("%d.png", y))
		if data, err := os.ReadFile(tilePath); err == nil && len(data) > 0 {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}

		// Fallback: Generate offline tile on the fly
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusOK)
		_ = generateFallbackTile(w, z, x, y)
	}
}

func generateFallbackTile(w http.ResponseWriter, z, x, y int) error {
	const size = 256
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Background color: muted map gray/blue
	bgColor := color.RGBA{R: 236, G: 239, B: 243, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// Border and crosshairs
	gridColor := color.RGBA{R: 210, G: 215, B: 222, A: 255}
	for i := 0; i < size; i++ {
		img.Set(i, 0, gridColor)
		img.Set(i, size-1, gridColor)
		img.Set(0, i, gridColor)
		img.Set(size-1, i, gridColor)

		img.Set(i, size/2, gridColor)
		img.Set(size/2, i, gridColor)
	}

	return png.Encode(w, img)
}
