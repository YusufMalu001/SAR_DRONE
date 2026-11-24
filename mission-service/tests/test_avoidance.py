import numpy as np
import pytest
from app.avoidance.potential_field import ObstacleAvoidanceAPF, simulate_navigation_with_avoidance


def test_obstacle_avoidance_single_obstacle():
    apf = ObstacleAvoidanceAPF(d_influence_m=12.0, d_collision_m=2.0, max_speed_mps=5.0)

    start = (0.0, 0.0)
    goal = (50.0, 0.0)
    # Obstacle placed directly in the center of the path at (25.0, 0.0) with radius 3.0m
    obstacles = [(25.0, 0.0, 3.0)]

    reached, collided, traj, min_dist = simulate_navigation_with_avoidance(start, goal, obstacles, apf)

    assert reached is True, "Drone should reach the goal"
    assert collided is False, f"Drone collided with obstacle! Min distance: {min_dist:.2f}m <= {apf.d_collision_m}m"
    assert min_dist > apf.d_collision_m


def test_obstacle_avoidance_randomized_seeded_maps():
    """Verify that path with obstacles reaches goal without collision across N randomized seeded maps."""
    apf = ObstacleAvoidanceAPF(d_influence_m=12.0, d_collision_m=2.0, max_speed_mps=5.0)

    num_test_maps = 20
    start = (0.0, 0.0)
    goal = (60.0, 0.0)

    for seed in range(num_test_maps):
        np.random.seed(seed + 100)

        # Place 3-5 random circular obstacles in corridor between X=10 and X=50
        num_obs = np.random.randint(3, 6)
        obstacles = []
        for _ in range(num_obs):
            ox = float(np.random.uniform(12.0, 48.0))
            oy = float(np.random.uniform(-8.0, 8.0))
            r = float(np.random.uniform(1.5, 3.0))
            obstacles.append((ox, oy, r))

        reached, collided, traj, min_dist = simulate_navigation_with_avoidance(
            start, goal, obstacles, apf, max_steps=1200
        )

        assert collided is False, f"Collision occurred in map seed {seed}! Min dist: {min_dist:.2f}m"
        assert reached is True, f"Failed to reach goal in map seed {seed}"
