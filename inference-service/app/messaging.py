import json

import redis

from app.config import settings

_client: redis.Redis | None = None


def get_redis() -> redis.Redis:
    global _client
    if _client is None:
        _client = redis.from_url(settings.redis_url, decode_responses=True)
    return _client


def redis_ping() -> bool:
    try:
        return get_redis().ping()
    except redis.RedisError:
        return False


def xadd_json(client: redis.Redis, stream: str, payload: dict) -> str:
    """Publish `payload` as a Redis stream entry under the "data" field.

    Streams (not pub/sub) are used so fusion-service can consume with a
    consumer group and replay/catch up after a restart.
    """
    return client.xadd(
        stream,
        {"data": json.dumps(payload)},
        maxlen=settings.stream_maxlen,
        approximate=True,
    )
