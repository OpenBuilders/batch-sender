import asyncio
import logging
from app.ton_wallet_manager import TonWalletManager
from app.rabbit_consumer import RabbitConsumer

class TonService:
    def __init__(self):
        self.wallet_manager = TonWalletManager()
        self.rabbit_consumer = RabbitConsumer(self.wallet_manager)

    async def start(self):
        logging.info("Starting TON Service...")
        # Запускаем RabbitMQ слушателя в отдельной задаче
        consumer_task = asyncio.create_task(self.rabbit_consumer.start())
        try:
            # Удерживаем сервис активным бесконечным ожиданием
            await asyncio.Future()
        except asyncio.CancelledError:
            logging.info("TON Service is shutting down.")
        finally:
            consumer_task.cancel()
            try:
                await consumer_task
            except asyncio.CancelledError:
                logging.info("RabbitConsumer task was cancelled.")
