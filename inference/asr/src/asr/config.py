"""Configuration for the ASR service."""

import os


class ASRConfig:
    """ASR service configuration loaded from environment variables."""

    grpc_port: int = int(os.getenv("GRPC_PORT", "50051"))
    model_size: str = os.getenv("WHISPER_MODEL_SIZE", "base")
    device: str = os.getenv("WHISPER_DEVICE", "cpu")  # "cpu" or "cuda"
    compute_type: str = os.getenv("WHISPER_COMPUTE_TYPE", "int8")
    num_workers: int = int(os.getenv("NUM_WORKERS", "2"))
