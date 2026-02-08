"""Configuration for the TTS service."""

import os


# Language → piper voice model name mapping.
VOICE_MAP = {
    "en": "en_US-lessac-medium",
    "pt-BR": "pt_BR-faber-medium",
    "pt": "pt_BR-faber-medium",
    "es": "es_MX-ald-medium",
}
DEFAULT_VOICE = "en_US-lessac-medium"


class TTSConfig:
    """TTS service configuration loaded from environment variables."""

    grpc_port: int = int(os.getenv("GRPC_PORT", "50052"))
    models_dir: str = os.getenv("TTS_MODELS_DIR", "/app/models")
    device: str = os.getenv("TTS_DEVICE", "cpu")
    num_workers: int = int(os.getenv("NUM_WORKERS", "2"))
    # Output format: PCM s16le, 16kHz, mono (canonical format)
    sample_rate: int = 16000
    channels: int = 1
    sample_width: int = 2  # bytes per sample (s16le)
