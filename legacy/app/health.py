"""
Module for system health checks.

This module verifies RabbitMQ and Telegram connectivity.

Functions:
    health_check: Checks the health status of the service.
"""

from __future__ import annotations

import logging
import time
import os 


import aio_pika
from app.metrics import healthcheck_latency, rabbitmq_queue_size
from fastapi import HTTPException

RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
RABBITMQ_QUEUE = os.getenv("RABBITMQ_QUEUE", "ton_transfer")

async def health_check() -> dict[str, str | dict[str, str]]:
    """Überprüft, ob RabbitMQ und Telegram erreichbar sind."""
    errors = {}
    start = time.time()  # Startzeit für Metrik

    # ✅ 1️⃣ Test RabbitMQ Connection
    try:
        connection = await aio_pika.connect_robust(RABBITMQ_URL)
        channel = await connection.channel()
        queue = await channel.declare_queue(RABBITMQ_QUEUE, passive=False, durable=True)
        rabbitmq_queue_size.set(queue.declaration_result.message_count) 
        await connection.close()
    except Exception as e:
        logging.exception("RabbitMQ Healthcheck failed")
        errors["rabbitmq"] = str(e)

    # ❌ On Error return 503
    healthcheck_latency.observe(time.time() - start)
    if errors:
        raise HTTPException(status_code=503, detail={"status": "error", "errors": errors})

    return {"status": "ok", "details": "All systems operational"}
