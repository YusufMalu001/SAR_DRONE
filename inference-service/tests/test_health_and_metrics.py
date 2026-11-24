"""Unit tests verifying /health degradation on missing models and /metrics Prometheus exposition."""

from fastapi.testclient import TestClient
import pytest

from app.config import settings
from app.main import app
from app.pipeline.metrics import metrics


@pytest.fixture
def client():
    return TestClient(app)


def test_health_reports_degraded_when_model_missing(client, monkeypatch, tmp_path):
    # Set model paths to non-existent files
    monkeypatch.setattr(settings, "person_model_path", str(tmp_path / "missing_person.onnx"))
    monkeypatch.setattr(settings, "thermal_model_path", str(tmp_path / "missing_thermal.onnx"))
    monkeypatch.setattr(settings, "hazard_model_path", str(tmp_path / "missing_hazard.onnx"))
    monkeypatch.setattr("app.main.redis_ping", lambda: True)

    resp = client.get("/health")
    assert resp.status_code == 200
    data = resp.json()

    assert data["status"] == "degraded"
    assert data["models"]["person"] == "missing"
    assert data["models"]["thermal"] == "missing"
    assert data["models"]["hazard"] == "missing"


def test_health_reports_ok_when_models_present(client, monkeypatch, tmp_path):
    # Create fake model files with non-zero size
    p_person = tmp_path / "person.onnx"
    p_thermal = tmp_path / "thermal.onnx"
    p_hazard = tmp_path / "hazard.onnx"

    for p in (p_person, p_thermal, p_hazard):
        p.write_bytes(b"dummy ONNX weights")

    monkeypatch.setattr(settings, "person_model_path", str(p_person))
    monkeypatch.setattr(settings, "thermal_model_path", str(p_thermal))
    monkeypatch.setattr(settings, "hazard_model_path", str(p_hazard))
    monkeypatch.setattr("app.main.redis_ping", lambda: True)

    resp = client.get("/health")
    assert resp.status_code == 200
    data = resp.json()

    assert data["status"] == "ok"
    assert data["models"]["person"] == "ok"
    assert data["models"]["thermal"] == "ok"
    assert data["models"]["hazard"] == "ok"


def test_metrics_exposes_real_latency_histograms(client):
    # Record some sample observations
    metrics.stage_latency.observe({"stage": "decode"}, 0.005)
    metrics.stage_latency.observe({"stage": "inference"}, 0.045)
    metrics.record_frame_processed()
    metrics.record_frame_dropped()

    resp = client.get("/metrics")
    assert resp.status_code == 200
    assert "text/plain" in resp.headers["content-type"]
    text = resp.text

    assert "inference_stage_latency_seconds_bucket" in text
    assert 'stage="inference"' in text
    assert 'stage="decode"' in text
    assert "inference_frames_processed_total" in text
    assert "inference_frames_dropped_total" in text
    assert "inference_fps" in text
    assert "inference_model_status" in text
