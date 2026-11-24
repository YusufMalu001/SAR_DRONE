import pytest
from app.vio.estimator import VisualInertialEKF, evaluate_gps_outage_drift


def test_vio_ekf_prediction_and_measurement_updates():
    ekf = VisualInertialEKF()
    ekf.reset(init_pos=(0.0, 0.0, -50.0), init_yaw=0.0)

    # Predict with forward acceleration 1.0 m/s^2 for 1 second
    dt = 0.1
    for _ in range(10):
        ekf.predict(ax_body=1.0, ay_body=0.0, az_body=0.0, yaw_rate=0.0, dt=dt)

    pos = ekf.position
    vel = ekf.velocity
    assert pos[0] > 0.3, "X position should increase under forward acceleration"
    assert vel[0] > 0.8, "X velocity should increase"

    # Optical flow measurement update (observing 100 pixels/sec flow)
    ekf.update_optical_flow(flow_dx_px=100.0, flow_dy_px=0.0, dt=1.0, altitude_agl_m=50.0)

    # Barometer update
    ekf.update_barometer(52.0)
    assert abs(ekf.position[2] - 52.0) < 2.0


def test_vio_gps_outage_drift_rate():
    """Empirically measure VIO dead-reckoning drift over 200m outage window."""
    metrics = evaluate_gps_outage_drift(travel_distance_m=200.0, speed_mps=5.0, seed=42)

    assert metrics["travel_distance_m"] == 200.0
    assert metrics["final_drift_m"] >= 0.0

    # Drift rate should be under 5.0m per 100m travelled for a realistic VIO model
    assert metrics["drift_per_100m"] < 5.0, f"Drift rate too high: {metrics['drift_per_100m']}m/100m"
    assert metrics["drift_per_100m"] > 0.0, "Drift rate must not be zero (must reflect real measurement error)"
