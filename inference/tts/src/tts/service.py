"""TTS service implementation."""

import logging
from collections.abc import Iterator

logger = logging.getLogger(__name__)

# Chunk size in bytes for streaming TTS output.
# At 16kHz s16le mono: 1600 samples = 100ms = 3200 bytes.
CHUNK_SIZE_BYTES = 3200


class TTSService:
    """Text-to-speech service.

    The model is loaded ONCE at construction time.
    Synthesis streams PCM chunks as an iterator (for gRPC server streaming).
    """

    def __init__(self, model_name: str = "default", device: str = "cpu"):
        logger.info("Loading TTS model: name=%s, device=%s", model_name, device)
        # TODO: load actual TTS model
        self.model = None  # placeholder
        logger.info("TTS model loaded")

    def synthesize(
        self, text: str, voice: str = "default", speed: float = 1.0
    ) -> Iterator[bytes]:
        """Synthesize text to PCM s16le 16kHz mono audio chunks.

        Yields:
            bytes: PCM audio chunks of CHUNK_SIZE_BYTES each.
        """
        # TODO: implement actual synthesis; yield chunks as they are produced
        yield b"\x00" * CHUNK_SIZE_BYTES  # silence placeholder
