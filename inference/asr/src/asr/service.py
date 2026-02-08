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
        translator=None,
    ):
        self.translator = translator
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
        target_language: str | None = None,
        translate_timeout_ms: int = 250,
    ) -> dict:
        """Transcribe PCM s16le audio bytes and optionally translate.

        Args:
            audio_bytes: Raw PCM s16le mono audio.
            sample_rate: Sample rate (must be 16000).
            language: Optional language hint (BCP-47).
            task: "transcribe" or "translate" (to English via Whisper).
            target_language: If set, translate text to this language via NLLB.
            translate_timeout_ms: Timeout for NLLB translation.

        Returns:
            dict with "text", "language", "segments", "inference_duration_ms",
            "translated_text", "target_language", "translate_duration_ms".
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

        result = {
            "text": " ".join(text_parts),
            "language": info.language,
            "segments": segments,
            "inference_duration_ms": inference_ms,
            "translated_text": "",
            "target_language": "",
            "translate_duration_ms": 0,
        }

        # Run NLLB translation if requested and translator is available.
        if target_language and self.translator and result["text"]:
            tr = self.translator.translate(
                text=result["text"],
                source_lang=result["language"],
                target_lang=target_language,
                timeout_ms=translate_timeout_ms,
            )
            result["translated_text"] = tr["translated_text"]
            result["target_language"] = target_language
            result["translate_duration_ms"] = tr["duration_ms"]

        return result
