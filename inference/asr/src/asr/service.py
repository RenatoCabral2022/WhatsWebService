"""ASR service implementation using faster-whisper."""

import logging
import time

import numpy as np

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
        from faster_whisper import WhisperModel

        self.model = WhisperModel(model_size, device=device, compute_type=compute_type)
        logger.info("Model loaded in %.2fs", time.monotonic() - start)

    def transcribe(
        self,
        audio_bytes: bytes,
        sample_rate: int = 16000,
        language: str | None = None,
        task: str = "transcribe",
    ) -> dict:
        """Transcribe PCM s16le audio bytes.

        Args:
            audio_bytes: Raw PCM s16le mono audio.
            sample_rate: Sample rate (must be 16000).
            language: Optional language hint (BCP-47).
            task: "transcribe" or "translate" (to English).

        Returns:
            dict with "text", "language", "segments", "inference_duration_ms".
        """
        assert sample_rate == 16000, "Only 16kHz is supported"

        # Convert PCM s16le bytes to float32 numpy array (faster-whisper expects float32 in [-1, 1])
        pcm_int16 = np.frombuffer(audio_bytes, dtype=np.int16)
        audio_float = pcm_int16.astype(np.float32) / 32768.0

        if len(audio_float) == 0:
            return {"text": "", "language": language or "en", "segments": [], "inference_duration_ms": 0}

        start = time.monotonic()
        segments_iter, info = self.model.transcribe(
            audio_float,
            language=language if language else None,
            task=task,
            beam_size=5,
            vad_filter=True,
        )

        segments = []
        text_parts = []
        for seg in segments_iter:
            segments.append({
                "text": seg.text.strip(),
                "start": seg.start,
                "end": seg.end,
                "confidence": getattr(seg, "avg_log_prob", 0.0),
            })
            text_parts.append(seg.text.strip())

        inference_ms = int((time.monotonic() - start) * 1000)

        return {
            "text": " ".join(text_parts),
            "language": info.language,
            "segments": segments,
            "inference_duration_ms": inference_ms,
        }
