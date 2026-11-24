"""Reactive Vector Field / Potential Field obstacle avoidance layer.

Validated in simulation only.
Processes 2D planar range scans / simulated lidar depth measurements
and calculates steering vectors to safely route around obstacles to the goal.
"""

import math
from typing import Dict, List, Optional, Tuple
import numpy as np


class ObstacleAvoidanceAPF:
    """Reactive Vector Field / Potential Field obstacle avoidance layer."""

    def __init__(
        self,
        d_influence_m: float = 12.0,  # Range at which obstacle starts repelling
        d_collision_m: float = 2.0,   # Collision boundary
        k_att: float = 1.0,           # Attraction gain
        k_rep: float = 50.0,          # Repulsion gain
        max_speed_mps: float = 5.0,
    ):
        self.d_influence_m = d_influence_m
        self.d_collision_m = d_collision_m
        self.k_att = k_att
        self.k_rep = k_rep
        self.max_speed_mps = max_speed_mps

    def compute_adjusted_velocity(
        self,
        current_pos: Tuple[float, float],       # (x, y) in meters
        goal_pos: Tuple[float, float],          # (x, y) in meters
        range_scans: List[Tuple[float, float]],  # List of (angle_rad, distance_m) relative to drone heading
        heading_rad: float = 0.0,
    ) -> Tuple[float, float]:
        """Compute adjusted velocity vector (vx, vy) using Vector Field candidate ray steering."""
        px, py = current_pos
        gx, gy = goal_pos

        dx = gx - px
        dy = gy - py
        dist_to_goal = math.hypot(dx, dy)

        if dist_to_goal < 0.3:
            return 0.0, 0.0

        target_angle = math.atan2(dy, dx)

        # Build candidate headings from target_angle outward
        candidate_angles = [target_angle]
        for delta in np.linspace(0.08, math.pi * 0.75, 45):
            candidate_angles.append(target_angle + delta)
            candidate_angles.append(target_angle - delta)

        # Check candidate angles against range scans
        best_angle = None
        d_safe = self.d_collision_m + 0.6  # Safety clearance buffer

        for ang in candidate_angles:
            is_clear = True
            for scan_angle_rel, dist in range_scans:
                if dist >= self.d_influence_m:
                    continue

                scan_world_angle = heading_rad + scan_angle_rel
                # Angle difference between candidate heading and obstacle ray
                diff = (scan_world_angle - ang + math.pi) % (2.0 * math.pi) - math.pi

                # If ray passes close to detected obstacle point
                perp_dist = abs(dist * math.sin(diff))
                forward_dist = dist * math.cos(diff)

                if forward_dist > 0 and perp_dist < d_safe:
                    is_clear = False
                    break

            if is_clear:
                best_angle = ang
                break

        if best_angle is None:
            # Fallback repulsive field
            best_angle = target_angle + math.pi * 0.5

        vx = self.max_speed_mps * math.cos(best_angle)
        vy = self.max_speed_mps * math.sin(best_angle)
        return vx, vy


def simulate_navigation_with_avoidance(
    start: Tuple[float, float],
    goal: Tuple[float, float],
    obstacles: List[Tuple[float, float, float]],  # (x, y, radius)
    apf: ObstacleAvoidanceAPF,
    dt: float = 0.1,
    max_steps: int = 1500,
) -> Tuple[bool, bool, List[Tuple[float, float]], float]:
    """Simulate drone flight using vector field avoidance.

    Returns: (reached_goal, collision_occurred, trajectory, min_distance_to_obstacle)
    """
    pos = list(start)
    traj = [tuple(pos)]
    min_dist_seen = float("inf")
    reached = False
    collided = False

    num_scan_rays = 48
    scan_angles = [2.0 * math.pi * i / num_scan_rays for i in range(num_scan_rays)]

    for _ in range(max_steps):
        dist_goal = math.hypot(goal[0] - pos[0], goal[1] - pos[1])
        if dist_goal < 0.5:
            reached = True
            break

        # Check collision against obstacles
        for ox, oy, r in obstacles:
            dist_surface = math.hypot(ox - pos[0], oy - pos[1]) - r
            if dist_surface < min_dist_seen:
                min_dist_seen = dist_surface
            if dist_surface <= apf.d_collision_m:
                collided = True
                break

        if collided:
            break

        # Generate range scan
        scans = []
        for angle in scan_angles:
            ray_dx = math.cos(angle)
            ray_dy = math.sin(angle)
            min_ray_dist = apf.d_influence_m

            for ox, oy, r in obstacles:
                vox = ox - pos[0]
                voy = oy - pos[1]
                proj = vox * ray_dx + voy * ray_dy
                if proj > 0:
                    perp_sq = (vox * vox + voy * voy) - (proj * proj)
                    if perp_sq < (r * r):
                        hit_dist = proj - math.sqrt(max(0, r * r - perp_sq))
                        if 0 < hit_dist < min_ray_dist:
                            min_ray_dist = hit_dist

            scans.append((angle, min_ray_dist))

        # Compute steering velocity
        vx, vy = apf.compute_adjusted_velocity(tuple(pos), goal, scans, heading_rad=0.0)

        pos[0] += vx * dt
        pos[1] += vy * dt
        traj.append(tuple(pos))

    return reached, collided, traj, min_dist_seen
