"""gRPC server entry point for the ASR service."""

import logging
from concurrent import futures

import grpc

from asr.config import ASRConfig
from asr.service import ASRService
from asr.grpc_servicer import AsrServicer
from whats.v1 import asr_pb2_grpc

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def serve():
    """Start the ASR gRPC server."""
    cfg = ASRConfig()

    # Load translator at startup if enabled (not in request path)
    translator = None
    if cfg.translation_enabled:
        from asr.translator import Translator

        translator = Translator(
            model_path=cfg.nllb_model_path,
            device=cfg.nllb_device,
            compute_type=cfg.nllb_compute_type,
        )
    else:
        logger.info("Translation disabled (TRANSLATION_ENABLED=false)")

    # Load ASR model at startup (not in request path)
    asr_service = ASRService(
        model_size=cfg.model_size,
        device=cfg.device,
        compute_type=cfg.compute_type,
        translator=translator,
    )

    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=cfg.num_workers),
        options=[
            ("grpc.max_receive_message_length", 10 * 1024 * 1024),  # 10MB
        ],
    )
    asr_pb2_grpc.add_AsrServiceServicer_to_server(AsrServicer(asr_service), server)

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
