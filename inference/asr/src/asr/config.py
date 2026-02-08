"""Configuration for the ASR service."""

import os


class ASRConfig:
    """ASR service configuration loaded from environment variables."""

    grpc_port: int = int(os.getenv("GRPC_PORT", "50051"))
    model_size: str = os.getenv("WHISPER_MODEL_SIZE", "base")
    device: str = os.getenv("WHISPER_DEVICE", "cpu")  # "cpu" or "cuda"
    compute_type: str = os.getenv("WHISPER_COMPUTE_TYPE", "int8")
    num_workers: int = int(os.getenv("NUM_WORKERS", "2"))

    # Translation (NLLB)
    translation_enabled: bool = os.getenv("TRANSLATION_ENABLED", "true").lower() == "true"
    nllb_model_path: str = os.getenv(
        "NLLB_MODEL_PATH", "/app/models/nllb-200-distilled-600M-ct2-int8"
    )
    nllb_device: str = os.getenv("NLLB_DEVICE", "cpu")
    nllb_compute_type: str = os.getenv("NLLB_COMPUTE_TYPE", "int8")
    translate_timeout_ms: int = int(os.getenv("TRANSLATE_TIMEOUT_MS", "250"))
