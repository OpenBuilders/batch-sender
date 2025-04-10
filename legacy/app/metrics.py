"""
Module for exposing Prometheus metrics.

This module provides system metrics like uptime, response times, and
application-specific metrics like message counts.

Functions:
    metrics: Exposes Prometheus metrics in a Prometheus-compatible format.
    sucessfull_batch_withdraws: Total number of sucessfull batch withdraws
"""

import asyncio
import logging
import time

import aio_pika
from aio_pika.exceptions import AMQPConnectionError, ChannelClosed
import os
from fastapi import Response
from prometheus_client import CONTENT_TYPE_LATEST, Counter, Gauge, Histogram, generate_latest

RABBITMQ_QUEUE = os.getenv("RABBITMQ_QUEUE", "ton_transfer")
RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")

# ⏱ System-Metriken
start_time = time.time()
uptime = Gauge("bot_uptime_seconds", "Bot Uptime in seconds")
healthcheck_latency = Histogram("healthcheck_response_time_seconds", "Response time of healthcheck endpoint")
# 📦 RabbitMQ Metriken
rabbitmq_queue_size = Gauge("rabbitmq_queue_length", "Number of messages in the RabbitMQ queue")

# ✉️ Nutzungsstatistiken
sucessfull_batch_withdraws = Counter("sucessfull_batch_withdraws", "Total number of sucessfull batch withdraws")
failed_batch_withdraws = Counter("failed_batch_withdraws", "Total number of failed batch withdraws")
def batch_withdraws_inc() -> None:
    """Erhöht den Zähler für gesendete Geschenke."""
    sucessfull_batch_withdraws.inc()

def failed_batch_withdraws_inc() -> None:
    """Erhöht den Zähler für gesendete Geschenke."""
    failed_batch_withdraws.inc()

async def update_rabbitmq_queue_size() -> None:
    """Aktualisiert periodisch die RabbitMQ-Queue-Länge für Prometheus."""
    while True:
        try:
            connection = await aio_pika.connect_robust(RABBITMQ_URL)
            channel = await connection.channel()
            queue = await channel.declare_queue(RABBITMQ_URL, durable=True, passive=False)
            rabbitmq_queue_size.set(queue.declaration_result.message_count)
            await connection.close()
        except (AMQPConnectionError, ChannelClosed):
            logging.exception("Failed to update RabbitMQ queue size")
            rabbitmq_queue_size.set(-1)  # If RabbitMQ is down , set to -1
        await asyncio.sleep(60)  # Update each minute


# 📊 Metrics Endpoint for Pormetheus
async def metrics() -> Response:
    """Prometheus Metriken-Endpoint."""
    uptime.set(time.time() - start_time)  # ⏱ Uptime aktualisieren
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)
