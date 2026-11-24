import logging
import time
import uuid

from fastapi import FastAPI, Request
from starlette.middleware.base import BaseHTTPMiddleware

from app.config import settings
from app.logging_config import setup_json_logging
from app.messaging import redis_ping

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


@app.get("/health")
@app.get("/healthz")
def health():
    return {
        "status": "ok",
        "service": settings.service_name,
        "redis": "up" if redis_ping() else "down",
    }


@app.get("/")
def root():
    return {"service": settings.service_name, "status": "running"}
