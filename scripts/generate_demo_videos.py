#!/usr/bin/env python3
"""Generates deterministic synthetic demo footage for SAR drone simulation scenarios.

Creates both visible RGB and thermal-infrared style video clips:
  1. flood_demo.mp4 & flood_demo_thermal.mp4 (flood water, submerged roof, stranded survivors)
  2. fire_demo.mp4 & fire_demo_thermal.mp4 (wildfire front, smoke plume, person near hazard)
  3. collapse_demo.mp4 & collapse_demo_thermal.mp4 (collapsed rubble, debris, power lines, victim)
  4. demo_flight.mp4 (standard demo flight footage for backward compatibility)

The videos are fully deterministic given a fixed seed, producing exact frame dimensions and
visual elements matching the scenario ground-truth bounding boxes.
"""

import argparse
import math
from pathlib import Path
import cv2
import numpy as np


def create_thermal_colormap(gray_frame: np.ndarray) -> np.ndarray:
    """Applies a high-contrast thermal palette (Inferno / Ironbow style)."""
    return cv2.applyColorMap(gray_frame, cv2.COLORMAP_INFERNO)


def generate_flood_video(rgb_path: Path, thermal_path: Path, num_frames: int = 60, width: int = 640, height: int = 480, fps: float = 5.0):
    fourcc = cv2.VideoWriter_fourcc(*"mp4v")
    rgb_writer = cv2.VideoWriter(str(rgb_path), fourcc, fps, (width, height))
    thermal_writer = cv2.VideoWriter(str(thermal_path), fourcc, fps, (width, height))

    rng = np.random.default_rng(seed=101)

    for i in range(num_frames):
        # Base terrain: murky flood water (brownish-blue)
        frame_rgb = np.zeros((height, width, 3), dtype=np.uint8)
        frame_rgb[:, :] = (120, 100, 60)  # BGR murky water

        # Ripple waves
        for y in range(0, height, 15):
            wave_offset = int(5 * math.sin((i * 0.2) + (y * 0.05)))
            cv2.line(frame_rgb, (0, y + wave_offset), (width, y + wave_offset), (140, 115, 75), 1)

        # Thermal base: water is cold (~15-20°C -> low intensity)
        frame_thermal_gray = np.full((height, width), 40, dtype=np.uint8)

        # Submerged rooftop / dry island (appearance across frames 5 to 55)
        if 5 <= i <= 55:
            roof_pts = np.array([[180, 180], [320, 150], [450, 180], [420, 320], [150, 320]], np.int32)
            cv2.fillPoly(frame_rgb, [roof_pts], (60, 70, 130))  # red-tile rooftop
            cv2.polylines(frame_rgb, [roof_pts], True, (40, 50, 90), 2)
            cv2.fillPoly(frame_thermal_gray, [roof_pts], 90)     # warmer roof surface

        # Stranded person 1 on rooftop (appearance frames 10 to 45)
        # Bounding box roughly {x: 0.45, y: 0.40, w: 0.08, h: 0.12} -> around px (288, 192)
        if 10 <= i <= 45:
            px, py = int(width * 0.47), int(height * 0.44)
            # Body: head and waving arms in RGB
            cv2.circle(frame_rgb, (px, py - 10), 6, (180, 200, 240), -1)  # head
            cv2.rectangle(frame_rgb, (px - 6, py - 4), (px + 6, py + 14), (200, 50, 50), -1)  # red shirt
            # Waving arm animation
            arm_y = py - 4 + int(6 * math.sin(i * 0.8))
            cv2.line(frame_rgb, (px + 6, py), (px + 16, arm_y), (180, 200, 240), 2)
            cv2.line(frame_rgb, (px - 6, py), (px - 16, arm_y), (180, 200, 240), 2)

            # High thermal signature (body heat ~37°C)
            cv2.circle(frame_thermal_gray, (px, py - 10), 7, 240, -1)
            cv2.rectangle(frame_thermal_gray, (px - 7, py - 4), (px + 7, py + 14), 220, -1)

        # Stranded person 2 on rooftop edge (appearance frames 18 to 48)
        if 18 <= i <= 48:
            px2, py2 = int(width * 0.55), int(height * 0.50)
            cv2.circle(frame_rgb, (px2, py2 - 8), 5, (170, 190, 230), -1)
            cv2.rectangle(frame_rgb, (px2 - 5, py2 - 3), (px2 + 5, py2 + 12), (50, 150, 50), -1) # green shirt
            cv2.circle(frame_thermal_gray, (px2, py2 - 8), 6, 235, -1)
            cv2.rectangle(frame_thermal_gray, (px2 - 6, py2 - 3), (px2 + 6, py2 + 12), 215, -1)

        # Sensor noise
        noise = rng.integers(-4, 5, size=(height, width, 3), dtype=np.int16)
        frame_rgb = np.clip(frame_rgb.astype(np.int16) + noise, 0, 255).astype(np.uint8)

        frame_thermal = create_thermal_colormap(frame_thermal_gray)

        rgb_writer.write(frame_rgb)
        thermal_writer.write(frame_thermal)

    rgb_writer.release()
    thermal_writer.release()


def generate_fire_video(rgb_path: Path, thermal_path: Path, num_frames: int = 60, width: int = 640, height: int = 480, fps: float = 5.0):
    fourcc = cv2.VideoWriter_fourcc(*"mp4v")
    rgb_writer = cv2.VideoWriter(str(rgb_path), fourcc, fps, (width, height))
    thermal_writer = cv2.VideoWriter(str(thermal_path), fourcc, fps, (width, height))

    rng = np.random.default_rng(seed=202)

    for i in range(num_frames):
        # Base terrain: dry forest / brush (olive greenish-brown)
        frame_rgb = np.zeros((height, width, 3), dtype=np.uint8)
        frame_rgb[:, :] = (40, 80, 60)

        # Thermal base: ambient vegetation ~25°C
        frame_thermal_gray = np.full((height, width), 60, dtype=np.uint8)

        # Active Fire front in upper-center (frames 8 to 55)
        # Bounding box roughly {x: 0.30, y: 0.15, w: 0.45, h: 0.40}
        if 8 <= i <= 55:
            fire_cx, fire_cy = int(width * 0.48), int(height * 0.30)
            # Flame flickering
            for _ in range(12):
                flame_x = int(fire_cx + rng.integers(-80, 80))
                flame_y = int(fire_cy + rng.integers(-40, 40))
                flame_r = int(rng.integers(15, 35))
                color = (0, int(rng.integers(140, 220)), int(rng.integers(220, 255))) # yellow-orange flame BGR
                cv2.circle(frame_rgb, (flame_x, flame_y), flame_r, color, -1)
                # Intense thermal hotspot (>300°C -> saturated white)
                cv2.circle(frame_thermal_gray, (flame_x, flame_y), int(flame_r * 1.2), 255, -1)

            # Billowing smoke plume drifting up/right
            for s in range(8):
                smoke_x = fire_cx + int(s * 15) + int(10 * math.sin(i * 0.3 + s))
                smoke_y = fire_cy - int(s * 14)
                if smoke_y > 10:
                    cv2.circle(frame_rgb, (smoke_x, smoke_y), 25 + s * 4, (140, 140, 140), -1)

        # Person fleeing fire at lower right (appearance frames 15 to 45)
        # Bounding box roughly {x: 0.65, y: 0.60, w: 0.08, h: 0.14}
        if 15 <= i <= 45:
            px, py = int(width * 0.68), int(height * 0.65)
            # Person moving slightly
            px_curr = px + int((i - 15) * 1.2)
            cv2.circle(frame_rgb, (px_curr, py - 12), 6, (170, 195, 235), -1)
            cv2.rectangle(frame_rgb, (px_curr - 6, py - 6), (px_curr + 6, py + 16), (220, 100, 30), -1) # blue/orange clothing
            # High thermal contrast against ambient grass
            cv2.circle(frame_thermal_gray, (px_curr, py - 12), 7, 245, -1)
            cv2.rectangle(frame_thermal_gray, (px_curr - 7, py - 6), (px_curr + 7, py + 16), 225, -1)

        # Sensor noise
        noise = rng.integers(-4, 5, size=(height, width, 3), dtype=np.int16)
        frame_rgb = np.clip(frame_rgb.astype(np.int16) + noise, 0, 255).astype(np.uint8)

        frame_thermal = create_thermal_colormap(frame_thermal_gray)

        rgb_writer.write(frame_rgb)
        thermal_writer.write(frame_thermal)

    rgb_writer.release()
    thermal_writer.release()


def generate_collapse_video(rgb_path: Path, thermal_path: Path, num_frames: int = 60, width: int = 640, height: int = 480, fps: float = 5.0):
    fourcc = cv2.VideoWriter_fourcc(*"mp4v")
    rgb_writer = cv2.VideoWriter(str(rgb_path), fourcc, fps, (width, height))
    thermal_writer = cv2.VideoWriter(str(thermal_path), fourcc, fps, (width, height))

    rng = np.random.default_rng(seed=303)

    for i in range(num_frames):
        # Base terrain: fractured urban concrete / asphalt (gray)
        frame_rgb = np.zeros((height, width, 3), dtype=np.uint8)
        frame_rgb[:, :] = (90, 95, 100)

        # Thermal base: ambient concrete ~22°C
        frame_thermal_gray = np.full((height, width), 65, dtype=np.uint8)

        # Collapsed building / structural rubble (frames 5 to 55)
        # Bounding box roughly {x: 0.15, y: 0.15, w: 0.50, h: 0.50}
        if 5 <= i <= 55:
            # Concrete fracture slabs
            rubble_pts = np.array([[120, 120], [380, 100], [420, 340], [160, 360]], np.int32)
            cv2.fillPoly(frame_rgb, [rubble_pts], (125, 130, 135))
            cv2.polylines(frame_rgb, [rubble_pts], True, (60, 65, 70), 3)

            # Debris blocks & rebar
            for b in range(10):
                bx = 150 + (b * 24) % 220
                by = 150 + (b * 19) % 180
                cv2.rectangle(frame_rgb, (bx, by), (bx + 20, by + 15), (75, 80, 85), -1)
                cv2.line(frame_rgb, (bx, by), (bx + 30, by - 10), (40, 40, 45), 2) # exposed rebar

        # Downed power line pole and exposed high-voltage cables across scene
        cv2.line(frame_rgb, (50, 420), (580, 380), (30, 35, 40), 2)
        cv2.line(frame_rgb, (60, 430), (590, 395), (25, 30, 35), 2)
        # Spark / arcing on damaged power line at frame 25-35
        if 25 <= i <= 35:
            spark_x, spark_y = 320, 400
            cv2.circle(frame_rgb, (spark_x, spark_y), 8, (255, 255, 200), -1)
            cv2.circle(frame_thermal_gray, (spark_x, spark_y), 12, 255, -1)

        # Trapped person / survivor partly under rubble (appearance frames 18 to 45)
        # Bounding box roughly {x: 0.40, y: 0.45, w: 0.08, h: 0.12}
        if 18 <= i <= 45:
            px, py = int(width * 0.44), int(height * 0.48)
            cv2.circle(frame_rgb, (px, py - 8), 6, (175, 190, 230), -1)
            cv2.rectangle(frame_rgb, (px - 5, py - 2), (px + 6, py + 12), (30, 120, 210), -1) # high-visibility yellow vest
            # Thermal hotspot from body heat
            cv2.circle(frame_thermal_gray, (px, py - 8), 7, 240, -1)
            cv2.rectangle(frame_thermal_gray, (px - 6, py - 2), (px + 7, py + 12), 220, -1)

        # Sensor noise
        noise = rng.integers(-4, 5, size=(height, width, 3), dtype=np.int16)
        frame_rgb = np.clip(frame_rgb.astype(np.int16) + noise, 0, 255).astype(np.uint8)

        frame_thermal = create_thermal_colormap(frame_thermal_gray)

        rgb_writer.write(frame_rgb)
        thermal_writer.write(frame_thermal)

    rgb_writer.release()
    thermal_writer.release()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--videos-dir", type=Path, default=Path("data/videos"))
    parser.add_argument("--sample-footage-dir", type=Path, default=Path("data/sample_footage"))
    parser.add_argument("--num-frames", type=int, default=60)
    args = parser.parse_args()

    args.videos_dir.mkdir(parents=True, exist_ok=True)
    args.sample_footage_dir.mkdir(parents=True, exist_ok=True)

    print("Generating flood scenario videos...")
    generate_flood_video(
        args.videos_dir / "flood_demo.mp4",
        args.videos_dir / "flood_demo_thermal.mp4",
        num_frames=args.num_frames,
    )

    print("Generating fire scenario videos...")
    generate_fire_video(
        args.videos_dir / "fire_demo.mp4",
        args.videos_dir / "fire_demo_thermal.mp4",
        num_frames=args.num_frames,
    )

    print("Generating collapse scenario videos...")
    generate_collapse_video(
        args.videos_dir / "collapse_demo.mp4",
        args.videos_dir / "collapse_demo_thermal.mp4",
        num_frames=args.num_frames,
    )

    # For backward compatibility with seed_demo.sh
    demo_flight = args.sample_footage_dir / "demo_flight.mp4"
    print("Generating demo_flight.mp4 in sample_footage...")
    generate_flood_video(
        demo_flight,
        args.sample_footage_dir / "demo_flight_thermal.mp4",
        num_frames=args.num_frames,
    )

    print("All demo video assets generated successfully.")


if __name__ == "__main__":
    main()
