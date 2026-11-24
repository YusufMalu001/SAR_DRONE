#!/usr/bin/env python3
"""Pre-caches map raster tiles for SAR mission scenarios for offline operations.

Respects OpenStreetMap Tile Usage Policy (User-Agent header, low concurrency, max zoom 17).
If internet is unavailable, synthesizes valid raster tiles locally so offline demos always function.
"""

import argparse
import io
import math
import os
import sys
import time
import urllib.request
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

TILE_DIR = Path(__file__).resolve().parents[1] / "data" / "tiles"


def lat_lon_to_tile(lat: float, lon: float, zoom: int):
    lat_rad = math.radians(lat)
    n = 2.0 ** zoom
    x = int((lon + 180.0) / 360.0 * n)
    y = int((1.0 - math.asinh(math.tan(lat_rad)) / math.pi) / 2.0 * n)
    return x, y


def generate_fallback_tile(z: int, x: int, y: int) -> bytes:
    img = Image.new("RGB", (256, 256), color=(235, 238, 242))
    draw = ImageDraw.Draw(img)

    # Outer border & cross grid
    draw.rectangle([0, 0, 255, 255], outline=(200, 205, 215), width=1)
    draw.line([0, 128, 256, 128], fill=(220, 225, 230), width=1)
    draw.line([128, 0, 128, 256], fill=(220, 225, 230), width=1)

    text = f"SAR Offline Tile\nz={z} x={x} y={y}"
    draw.text((15, 15), text, fill=(120, 130, 145))

    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()


def cache_tiles_for_bbox(min_lat: float, min_lon: float, max_lat: float, max_lon: float, zooms=(14, 15, 16, 17)):
    os.makedirs(TILE_DIR, exist_ok=True)
    headers = {"User-Agent": "Khoj-o-Drone-SAR/1.0 (Smart India Hackathon SIH26177 Team NIRIKSH)"}

    total_cached = 0
    total_generated = 0

    for z in zooms:
        x_min, y_max = lat_lon_to_tile(min_lat, min_lon, z)
        x_max, y_min = lat_lon_to_tile(max_lat, max_lon, z)

        x_start, x_end = min(x_min, x_max), max(x_min, x_max)
        y_start, y_end = min(y_min, y_max), max(y_min, y_max)

        for x in range(x_start, x_end + 1):
            for y in range(y_start, y_end + 1):
                out_path = TILE_DIR / str(z) / str(x) / f"{y}.png"
                if out_path.exists() and out_path.stat().st_size > 0:
                    continue

                out_path.parent.mkdir(parents=True, exist_ok=True)
                url = f"https://tile.openstreetmap.org/{z}/{x}/{y}.png"

                data = None
                try:
                    req = urllib.request.Request(url, headers=headers)
                    with urllib.request.urlopen(req, timeout=3.0) as resp:
                        if resp.status == 200:
                            data = resp.read()
                            total_cached += 1
                            time.sleep(0.05)  # rate limit courtesy
                except Exception:
                    # Offline or rate-limited fallback
                    data = generate_fallback_tile(z, x, y)
                    total_generated += 1

                if data:
                    out_path.write_bytes(data)

    print(f"Caching complete for bbox: {total_cached} downloaded from OSM, {total_generated} generated offline.")


def main():
    parser = argparse.ArgumentParser(description="Cache raster tiles for SAR mission")
    parser.add_argument("--scenario", choices=["flood", "fire", "collapse", "all"], default="all")
    args = parser.parse_args()

    scenarios = {
        "flood": (20.000, 78.000, 20.006, 78.006),
        "fire": (18.520, 73.850, 18.526, 73.856),
        "collapse": (28.610, 77.200, 28.616, 77.206),
    }

    targets = scenarios.keys() if args.scenario == "all" else [args.scenario]
    for s in targets:
        bbox = scenarios[s]
        print(f"Caching tiles for scenario: {s} (bbox: {bbox})...")
        cache_tiles_for_bbox(*bbox)


if __name__ == "__main__":
    main()
