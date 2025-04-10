"""
Module to start the FastAPI server for health checks and metrics.

This server imports health and metrics modules and exposes them via FastAPI routes.

Functions:
    start_server: Starts the FastAPI server.
"""

import asyncio
import uvicorn
from fastapi import FastAPI

from app.health import health_check
from app.metrics import metrics
from app.metrics import update_rabbitmq_queue_size

# Erstelle FastAPI App
app = FastAPI()

def setup_routes() -> None:
    """Registers FastAPI routes with the provided service instance."""
    @app.get("/health")
    async def health_check_route() -> dict[str, str | dict[str, str]]:
        """Runs the health check with the provided GiftService."""
        return await health_check()

    @app.get("/metrics")
    async def metrics_route():
        """Fetches Prometheus metrics."""
        return await metrics()

async def start_server() -> None:
    """Starts the FastAPI server for health checks & metrics."""

    setup_routes()  # Register routes with the passed-in service

    # Start RabbitMQ monitoring in background
    asyncio.create_task(update_rabbitmq_queue_size())

    # Start FastAPI Server (Uvicorn)
    config = uvicorn.Config(app, host="0.0.0.0", port=8000, log_level="info")
    server = uvicorn.Server(config)
    await server.serve()

if __name__ == "__main__":
    loop = asyncio.get_event_loop()
    loop.run_until_complete(start_server())  # Kein asyncio.run() mehr!
