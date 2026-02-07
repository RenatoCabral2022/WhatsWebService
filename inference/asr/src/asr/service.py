"""ASR service implementation using faster-whisper."""

import logging
import time

logger = logging.getLogger(__name__)


class ASRService:
    """Speech-to-text service wrapping faster-whisper.

    The model is loaded ONCE at construction time (startup),
    never in the request path.
    """

    def __init__(
        self,
        model_size: str = "base",
        device: str = "cpu",
        compute_type: str = "int8",
    ):
        logger.info(
            "Loading whisper model: size=%s, device=%s, compute=%s",
            model_size,
            device,
            compute_type,
        )
        start = time.monotonic()
        # TODO: from faster_whisper import WhisperModel
        # self.model = WhisperModel(model_size, device=device, compute_type=compute_type)
        self.model = None  # placeholder
        logger.info("Model loaded in %.2fs", time.monotonic() - start)

    def transcribe(
        self,
        audio_bytes: bytes,
        sample_rate: int = 16000,
        language: str | None = None,
    ) -> dict:
        """Transcribe PCM s16le audio bytes.

        Args:
            audio_bytes: Raw PCM s16le mono audio.
            sample_rate: Sample rate (must be 16000).
            language: Optional language hint.

        Returns:
            dict with "text", "language", "segments".
        """
        assert sample_rate == 16000, "Only 16kHz is supported"
        # TODO: implement actual transcription
        return {"text": "", "language": language or "en", "segments": []}
