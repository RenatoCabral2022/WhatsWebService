"""Configuration for the TTS service."""

import os


class TTSConfig:
    """TTS service configuration loaded from environment variables."""

    grpc_port: int = int(os.getenv("GRPC_PORT", "50052"))
    model_name: str = os.getenv("TTS_MODEL", "default")
    device: str = os.getenv("TTS_DEVICE", "cpu")
    num_workers: int = int(os.getenv("NUM_WORKERS", "2"))
    # Output format: PCM s16le, 16kHz, mono (canonical format)
    sample_rate: int = 16000
    channels: int = 1
    sample_width: int = 2  # bytes per sample (s16le)
