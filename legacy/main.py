import asyncio
import logging
from app.ton_service import TonService
from app.server import start_server

logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")

async def main():
    service = TonService()
    await asyncio.gather(
        service.start(),
        start_server(),
    )

# if __name__ == "__main__":
#     asyncio.run(main())
if __name__ == "__main__":
    loop = asyncio.get_event_loop()
    loop.create_task(main())  # starts main() as background-task
    loop.run_forever()  # keeps eventloop running
