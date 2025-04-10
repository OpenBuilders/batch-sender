import os
import logging
import json
import aio_pika
import asyncio
from dotenv import load_dotenv
from tonutils.client import TonapiClient
from tonutils.wallet import HighloadWalletV3
from tonsdk.utils import to_nano
from tonutils.wallet.data import TransferData
from tonsdk.boc import Cell

load_dotenv()

RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
RABBITMQ_QUEUE = os.getenv("RABBITMQ_QUEUE", "main-service")

class TonWalletManager:
    def __init__(self):
        self.api_key = os.getenv("TON_API_KEY")
        self.wallet_seed = os.getenv("TON_WALLET_SEED")

        if not self.api_key or not self.wallet_seed:
            raise ValueError("TON_API_KEY and TON_WALLET_SEED must be set in .env")

        self.client = TonapiClient(api_key=self.api_key, is_testnet=False)
        self.tx_lock = asyncio.Lock()
        logging.info("Initialized TonWalletManager with API key and wallet seed")

    async def send_payment_status(self, transaction_id: str, status: str, tx_hash: str):
        try:
            connection = await aio_pika.connect_robust(RABBITMQ_URL)
            channel = await connection.channel()
            message = aio_pika.Message(
                body=json.dumps({
                    "pattern": "payment-status",
                    "data": {
                        "transaction_id": transaction_id,
                        "tx_hash": tx_hash,
                        "status": status
                    }
                }, ensure_ascii=False).encode(),
                content_type="application/json",
                delivery_mode=aio_pika.DeliveryMode.PERSISTENT
            )
            await channel.default_exchange.publish(message, routing_key="main-service")
            await connection.close()
            logging.info(f"Sent payment status for {transaction_id} ({status})")
        except Exception as e:
            logging.error("Failed to send payment status to RabbitMQ: %s", str(e))

    async def batch_withdraw_ton(self, payload: dict):
        transaction_id = payload.get("transaction_id")
        transactions = payload.get("data", [])

        logging.info("Preparing to send batch transaction for %s with %s entries", transaction_id, len(transactions))

        async with self.tx_lock:  # 🔐 Критический участок
            try:
                if not transaction_id:
                    logging.error("Transaction ID is missing in the payload")
                    await self.send_payment_status(transaction_id, "error", None)
                    return "error"

                wallet, _, _, _ = HighloadWalletV3.from_mnemonic(self.client, self.wallet_seed)
                logging.info("Loaded HighloadWalletV3 from mnemonic")

                transfer_data_list = []
                for tx in transactions:
                    recipient = tx.get("wallet")
                    amount = tx.get("amount")
                    comment = tx.get("comment")

                    if not comment:
                        logging.info("Comment is missing for transaction %s", tx)
                        comment = "Payment"

                    if not recipient or not amount or not comment:
                        continue

                    transfer_data_list.append(
                        TransferData(
                            destination=recipient,
                            amount=amount,
                            body=comment
                        )
                    )

                if not transfer_data_list:
                    logging.error("No valid transactions to send. Aborting.")
                    await self.send_payment_status(transaction_id, "error", None)
                    return "error"

                tx_hash = await wallet.batch_transfer(data_list=transfer_data_list)
                logging.info("Batch transaction sent: %s", tx_hash)

                await self.send_payment_status(transaction_id, "success", tx_hash)
                return tx_hash

            except Exception as e:
                logging.error("Error during batch transaction: %s", str(e))
                await self.send_payment_status(transaction_id, "error", None)
                return "error"
