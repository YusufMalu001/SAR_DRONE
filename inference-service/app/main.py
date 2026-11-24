import logging
import os
import time
import uuid
from pathlib import Path
from typing import Dict

from fastapi import FastAPI, Request, Response
from starlette.middleware.base import BaseHTTPMiddleware

from app.config import settings
from app.logging_config import setup_json_logging
from app.messaging import redis_ping
from app.pipeline.metrics import metrics

setup_json_logging()
logger = logging.getLogger("inference.api")

app = FastAPI(title=settings.service_name)


class RequestLoggingMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next):
        request_id = request.headers.get("x-request-id") or str(uuid.uuid4())
        start_time = time.time()
        response = await call_next(request)
        duration_ms = round((time.time() - start_time) * 1000, 2)
        response.headers["x-request-id"] = request_id
        logger.info(
            f"{request.method} {request.url.path} {response.status_code} ({duration_ms}ms)",
            extra={"request_id": request_id},
        )
        return response


app.add_middleware(RequestLoggingMiddleware)


def check_models_health() -> Dict[str, str]:
    """Inspects on-disk model weight files and updates model status metrics."""
    models_to_check = {
        "person": settings.person_model_path,
        "thermal": settings.thermal_model_path,
        "hazard": settings.hazard_model_path,
    }
    status = {}
    for name, path_str in models_to_check.items():
        p = Path(path_str)
        if p.is_file() and p.stat().st_size > 0:
            status[name] = "ok"
            metrics.model_status_gauge.set(1.0, {"model": name})
        else:
            status[name] = "missing"
            metrics.model_status_gauge.set(0.0, {"model": name})
    return status


@app.get("/health")
@app.get("/healthz")
def health():
    model_status = check_models_health()
    all_models_ok = all(v == "ok" for v in model_status.values())
    redis_ok = redis_ping()

    overall_status = "ok" if (all_models_ok and redis_ok) else "degraded"

    return {
        "status": overall_status,
        "service": settings.service_name,
        "redis": "up" if redis_ok else "down",
        "models": model_status,
        "provider": settings.execution_provider,
    }


@app.get("/metrics")
def get_metrics():
    """Prometheus-compatible plain text metrics exposition."""
    # Ensure current model health status is reflected in gauges
    check_models_health()
    return Response(
        content=metrics.render_prometheus(),
        media_type="text/plain; version=0.0.4; charset=utf-8",
    )


@app.get("/")
def root():
    return {"service": settings.service_name, "status": "running"}
