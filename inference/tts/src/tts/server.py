"""gRPC server entry point for the TTS service."""

import logging
from concurrent import futures

import grpc

from tts.config import TTSConfig

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def serve():
    """Start the TTS gRPC server."""
    cfg = TTSConfig()
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=cfg.num_workers))
    # TODO: register TtsServiceServicer after proto generation
    server.add_insecure_port(f"[::]:{cfg.grpc_port}")
    logger.info(
        "TTS server starting on port %d (model=%s, device=%s)",
        cfg.grpc_port,
        cfg.model_name,
        cfg.device,
    )
    server.start()
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
