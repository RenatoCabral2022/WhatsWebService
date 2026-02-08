"""gRPC server entry point for the TTS service."""

import logging
from concurrent import futures

import grpc

from tts.config import TTSConfig
from tts.service import TTSService
from tts.grpc_servicer import TtsServicer
from whats.v1 import tts_pb2_grpc

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def serve():
    """Start the TTS gRPC server."""
    cfg = TTSConfig()

    # Load all voice models at startup (not in request path)
    tts_service = TTSService(
        models_dir=cfg.models_dir,
        device=cfg.device,
    )

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=cfg.num_workers))
    tts_pb2_grpc.add_TtsServiceServicer_to_server(TtsServicer(tts_service), server)

    server.add_insecure_port(f"[::]:{cfg.grpc_port}")
    logger.info(
        "TTS server starting on port %d (models_dir=%s, device=%s)",
        cfg.grpc_port,
        cfg.models_dir,
        cfg.device,
    )
    server.start()
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
