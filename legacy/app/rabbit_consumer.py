import json
import logging
import os
import aio_pika
from app.metrics import batch_withdraws_inc, failed_batch_withdraws_inc

RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
RABBITMQ_QUEUE = os.getenv("RABBITMQ_QUEUE", "ton_transfer")

class RabbitConsumer:
    def __init__(self, wallet_manager):
        self.wallet_manager = wallet_manager

    async def start(self):
        connection = await aio_pika.connect_robust(RABBITMQ_URL)
        channel = await connection.channel()
        queue = await channel.declare_queue(RABBITMQ_QUEUE, durable=True, passive=False)
        logging.info("Connected to RabbitMQ. Listening for TON transfer requests...")

        async with queue.iterator() as queue_iter:
            async for message in queue_iter:
                async with message.process():
                    try:
                        msg = json.loads(message.body)
                        pattern = msg.get("pattern")
                        data = msg.get("data", {})

                        if pattern == "send-ton":
                            logging.info("Received transactions: %s", data)
                            await self.wallet_manager.batch_withdraw_ton(data)
                            batch_withdraws_inc()
                    except Exception as e:
                        failed_batch_withdraws_inc()
                        logging.error("Error processing message: %s", e)
