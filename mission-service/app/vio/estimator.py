"""Visual-Inertial Dead-Reckoning (VIO) Estimator for GPS-denied drone navigation.

Fuses:
1. Downward optical-flow velocity scaled by barometric altitude (OpenCV Lucas-Kanade / Farneback).
2. IMU 3-axis accelerometer and gyro yaw rate.
3. Barometric altitude.
Via an Extended Kalman Filter (EKF) to maintain continuous position estimation during GPS outages.

Note: This is Visual-Inertial Odometry (dead-reckoning), not full SLAM (loop closure / relocalization).
"""

import math
from typing import Dict, List, Optional, Tuple
import numpy as np


class VisualInertialEKF:
    """7-State Extended Kalman Filter for Visual-Inertial Dead-Reckoning:

    State vector: [p_x, p_y, p_z, v_x, v_y, v_z, yaw]
    Coordinate frame: Local Cartesian (North-East-Down meters, yaw radians)
    """

    def __init__(
        self,
        focal_length_px: float = 800.0,
        process_noise_accel: float = 0.20,
        process_noise_gyro: float = 0.05,
        meas_noise_flow: float = 0.25,
        meas_noise_baro: float = 0.15,
        meas_noise_gps: float = 0.80,
    ):
        self.focal_length_px = focal_length_px
        self.dt_default = 0.05

        # State: [px, py, pz, vx, vy, vz, yaw]
        self.x = np.zeros(7, dtype=np.float64)

        # Covariance matrix P (7x7)
        self.P = np.diag([1.0, 1.0, 1.0, 0.5, 0.5, 0.5, 0.05])

        # Process noise Q
        self.q_accel = process_noise_accel
        self.q_gyro = process_noise_gyro

        # Measurement noise
        self.r_flow = meas_noise_flow ** 2
        self.r_baro = meas_noise_baro ** 2
        self.r_gps = meas_noise_gps ** 2

        self.last_predict_time: Optional[float] = None
        self.gps_available = True

    def reset(self, init_pos=(0.0, 0.0, -50.0), init_yaw=0.0):
        self.x = np.zeros(7, dtype=np.float64)
        self.x[0] = init_pos[0]
        self.x[1] = init_pos[1]
        self.x[2] = init_pos[2]  # Negative for altitude above ground in NED
        self.x[6] = init_yaw
        self.P = np.diag([1.0, 1.0, 1.0, 0.5, 0.5, 0.5, 0.05])

    def predict(self, ax_body: float, ay_body: float, az_body: float, yaw_rate: float, dt: float):
        """Predict step driven by IMU body accelerations and yaw rate."""
        yaw = self.x[6]

        # Rotate horizontal acceleration from body FRD to world NED
        cos_y = math.cos(yaw)
        sin_y = math.sin(yaw)

        ax_world = cos_y * ax_body - sin_y * ay_body
        ay_world = sin_y * ax_body + cos_y * ay_body
        az_world = az_body  # Assuming near-nadir attitude

        # State transition
        # p += v * dt + 0.5 * a * dt^2
        self.x[0] += self.x[3] * dt + 0.5 * ax_world * dt * dt
        self.x[1] += self.x[4] * dt + 0.5 * ay_world * dt * dt
        self.x[2] += self.x[5] * dt + 0.5 * az_world * dt * dt

        # v += a * dt
        self.x[3] += ax_world * dt
        self.x[4] += ay_world * dt
        self.x[5] += az_world * dt

        # yaw += yaw_rate * dt
        self.x[6] = (self.x[6] + yaw_rate * dt + math.pi) % (2.0 * math.pi) - math.pi

        # State transition Jacobian F (7x7)
        F = np.eye(7)
        F[0, 3] = dt
        F[1, 4] = dt
        F[2, 5] = dt

        # Derivative of a_world w.r.t yaw
        d_ax_dyaw = -sin_y * ax_body - cos_y * ay_body
        d_ay_dyaw = cos_y * ax_body - sin_y * ay_body
        F[0, 6] = 0.5 * d_ax_dyaw * dt * dt
        F[1, 6] = 0.5 * d_ay_dyaw * dt * dt
        F[3, 6] = d_ax_dyaw * dt
        F[4, 6] = d_ay_dyaw * dt

        # Process noise covariance Q
        Q = np.zeros((7, 7))
        q_pos = 0.25 * (self.q_accel ** 2) * (dt ** 4)
        q_vel = (self.q_accel ** 2) * (dt ** 2)
        q_yaw = (self.q_gyro ** 2) * (dt ** 2)

        Q[0, 0] = q_pos
        Q[1, 1] = q_pos
        Q[2, 2] = q_pos
        Q[3, 3] = q_vel
        Q[4, 4] = q_vel
        Q[5, 5] = q_vel
        Q[6, 6] = q_yaw

        self.P = F @ self.P @ F.T + Q

    def update_optical_flow(self, flow_dx_px: float, flow_dy_px: float, dt: float, altitude_agl_m: float):
        """Update filter using optical flow pixel velocity scaled by altitude."""
        if dt <= 1e-4 or altitude_agl_m <= 1.0:
            return

        # Optical flow metric velocity in body frame (m/s)
        # v = (flow_px / dt) * (altitude / focal_length)
        v_flow_x_body = (flow_dx_px / dt) * (altitude_agl_m / self.focal_length_px)
        v_flow_y_body = (flow_dy_px / dt) * (altitude_agl_m / self.focal_length_px)

        # Rotate to world frame
        yaw = self.x[6]
        cos_y = math.cos(yaw)
        sin_y = math.sin(yaw)
        v_flow_x_world = cos_y * v_flow_x_body - sin_y * v_flow_y_body
        v_flow_y_world = sin_y * v_flow_x_body + cos_y * v_flow_y_body

        z = np.array([v_flow_x_world, v_flow_y_world])

        # Measurement matrix H (2x7) observing [vx, vy]
        H = np.zeros((2, 7))
        H[0, 3] = 1.0
        H[1, 4] = 1.0

        R = np.eye(2) * self.r_flow
        y = z - H @ self.x
        S = H @ self.P @ H.T + R
        K = self.P @ H.T @ np.linalg.inv(S)

        self.x = self.x + K @ y
        self.P = (np.eye(7) - K @ H) @ self.P

    def update_barometer(self, altitude_m: float):
        """Update filter with barometric altitude (-pz in NED)."""
        z = -altitude_m
        H = np.zeros((1, 7))
        H[0, 2] = 1.0

        R = np.array([[self.r_baro]])
        y = np.array([z]) - H @ self.x
        S = H @ self.P @ H.T + R
        K = self.P @ H.T @ np.linalg.inv(S)

        self.x = self.x + (K @ y).flatten()
        self.P = (np.eye(7) - K @ H) @ self.P

    def update_gps(self, pos_x: float, pos_y: float):
        """Update filter when GPS is available to anchor position."""
        z = np.array([pos_x, pos_y])
        H = np.zeros((2, 7))
        H[0, 0] = 1.0
        H[1, 1] = 1.0

        R = np.eye(2) * self.r_gps
        y = z - H @ self.x
        S = H @ self.P @ H.T + R
        K = self.P @ H.T @ np.linalg.inv(S)

        self.x = self.x + K @ y
        self.P = (np.eye(7) - K @ H) @ self.P

    def update_heading(self, heading_rad: float, variance: float = 0.001):
        """Update filter with 3-axis compass/magnetometer heading observation."""
        diff = (heading_rad - self.x[6] + math.pi) % (2.0 * math.pi) - math.pi
        k_yaw = self.P[6, 6] / (self.P[6, 6] + variance)
        self.x[6] = (self.x[6] + k_yaw * diff + math.pi) % (2.0 * math.pi) - math.pi
        self.P[6, 6] *= (1.0 - k_yaw)

    @property
    def position(self) -> Tuple[float, float, float]:
        """Return estimated position (x, y, -z) where z is altitude above ground."""
        return float(self.x[0]), float(self.x[1]), float(-self.x[2])

    @property
    def velocity(self) -> Tuple[float, float, float]:
        return float(self.x[3]), float(self.x[4]), float(self.x[5])

    @property
    def yaw(self) -> float:
        return float(self.x[6])


def evaluate_gps_outage_drift(
    travel_distance_m: float = 200.0,
    speed_mps: float = 5.0,
    optical_flow_noise_std: float = 0.05,
    imu_accel_bias: float = 0.02,
    imu_gyro_bias: float = 0.005,
    seed: int = 42,
) -> Dict[str, float]:
    """Empirically measure VIO dead-reckoning drift over a GPS outage window.

    Simulates forward flight with downward optical flow, IMU biases, and baro.
    Returns:
    - distance_travelled_m
    - final_drift_m
    - drift_per_100m
    """
    np.random.seed(seed)
    ekf = VisualInertialEKF()
    ekf.reset(init_pos=(0.0, 0.0, -50.0), init_yaw=0.0)

    dt = 0.05
    total_time = travel_distance_m / speed_mps
    num_steps = int(total_time / dt)

    true_pos = np.array([0.0, 0.0, -50.0])
    true_vel = np.array([speed_mps, 0.0, 0.0])

    for _ in range(num_steps):
        # True kinematics
        true_pos += true_vel * dt

        # Simulated IMU with bias + noise
        ax_meas = 0.0 + imu_accel_bias + np.random.normal(0, 0.05)
        ay_meas = 0.0 + np.random.normal(0, 0.05)
        az_meas = 0.0 + np.random.normal(0, 0.05)
        gyro_meas = 0.0 + imu_gyro_bias + np.random.normal(0, 0.005)

        ekf.predict(ax_meas, ay_meas, az_meas, gyro_meas, dt)

        # Simulated Optical Flow: flow_dx = (v_true_x / altitude) * focal_length * dt
        ideal_flow_dx = (true_vel[0] / 50.0) * ekf.focal_length_px * dt
        meas_flow_dx = ideal_flow_dx * (1.0 + np.random.normal(0, optical_flow_noise_std))
        meas_flow_dy = np.random.normal(0, 0.5)

        ekf.update_optical_flow(meas_flow_dx, meas_flow_dy, dt, altitude_agl_m=50.0)
        ekf.update_barometer(50.0 + np.random.normal(0, 0.1))
        ekf.update_heading(0.0 + np.random.normal(0, 0.01))

    est_x, est_y, est_alt = ekf.position
    drift_x = est_x - true_pos[0]
    drift_y = est_y - true_pos[1]
    final_drift_m = math.hypot(drift_x, drift_y)

    drift_per_100m = (final_drift_m / travel_distance_m) * 100.0

    return {
        "travel_distance_m": travel_distance_m,
        "flight_time_s": total_time,
        "final_drift_m": round(final_drift_m, 2),
        "drift_per_100m": round(drift_per_100m, 2),
    }
