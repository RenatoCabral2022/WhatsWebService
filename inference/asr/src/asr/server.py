"""gRPC server entry point for the ASR service."""

import logging
from concurrent import futures

import grpc

from asr.config import ASRConfig

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def serve():
    """Start the ASR gRPC server."""
    cfg = ASRConfig()
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=cfg.num_workers))
    # TODO: register AsrServiceServicer after proto generation
    server.add_insecure_port(f"[::]:{cfg.grpc_port}")
    logger.info(
        "ASR server starting on port %d (model=%s, device=%s)",
        cfg.grpc_port,
        cfg.model_size,
        cfg.device,
    )
    server.start()
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
