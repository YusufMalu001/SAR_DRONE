from fastapi import FastAPI

from app.config import settings
from app.messaging import redis_ping

app = FastAPI(title=settings.service_name)


@app.get("/health")
def health():
    return {
        "status": "ok",
        "service": settings.service_name,
        "redis": "up" if redis_ping() else "down",
    }


@app.get("/")
def root():
    return {"service": settings.service_name, "status": "running"}
