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

    @staticmethod
    def _detect_language(text: str) -> str:
        """Detect language from text using Google's langdetect library.

        Maps detected language to our supported NLLB codes (en, pt, es).
        Defaults to 'en' when uncertain or on error.
        """
        from langdetect import detect

        try:
            lang = detect(text)
            if lang.startswith("pt"):
                return "pt"
            if lang.startswith("es"):
                return "es"
            return "en"
        except Exception:
            return "en"

    def transcribe(
        self,
        audio_bytes: bytes,
        sample_rate: int = 16000,
        language: str | None = None,
        task: str = "transcribe",
        target_language: str | None = None,
        translate_timeout_ms: int = 250,
        source_text: str | None = None,
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

        # ── Text-only translation mode (Spotify lyrics) ──────────
        # When audio is empty but source_text is provided,
        # skip Whisper and go straight to NLLB translation.
        if len(audio_bytes) == 0 and source_text:
            # Detect source language if set to "auto" (NLLB needs a real code)
            detected_lang = language if language and language != "auto" else self._detect_language(source_text)
            logger.info("Text-only translation: source_text_len=%d, source_lang=%s (detected=%s), target_lang=%s",
                        len(source_text), language or "auto", detected_lang, target_language or "(none)")
            result = {
                "text": source_text,
                "language": detected_lang,
                "segments": [],
                "inference_duration_ms": 0,
                "translated_text": "",
                "target_language": "",
                "translate_duration_ms": 0,
            }
            # Run NLLB translation if requested
            if target_language and self.translator and result["language"] != target_language:
                tr = self.translator.translate(
                    text=source_text,
                    source_lang=result["language"],
                    target_lang=target_language,
                    timeout_ms=translate_timeout_ms,
                )
                result["translated_text"] = tr["translated_text"]
                result["target_language"] = target_language
                result["translate_duration_ms"] = tr["duration_ms"]
                logger.info("Text-only translation result: translated_len=%d, translate_ms=%d",
                            len(result["translated_text"]), result["translate_duration_ms"])
            return result

        if len(audio_bytes) == 0:
            return {"text": "", "language": language or "en", "segments": [], "inference_duration_ms": 0}

        # Convert PCM s16le bytes to float32 numpy array (faster-whisper expects float32 in [-1, 1])
        pcm_int16 = np.frombuffer(audio_bytes, dtype=np.int16)
        audio_float = pcm_int16.astype(np.float32) / 32768.0

        start = time.monotonic()
        segments_iter, info = self.model.transcribe(
            audio_float,
            language=language if language else None,
            task=task,
            beam_size=5,
            vad_filter=True,
            vad_parameters=dict(
                threshold=0.2,               # aggressive: keep quieter vocals mixed with music
                min_silence_duration_ms=600,  # longer silence needed to split segments
                speech_pad_ms=400,            # extra padding around detected speech
            ),
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

        # Fallback: if VAD stripped everything, retry WITHOUT VAD.
        # This handles music where vocals are mixed too quietly for Silero VAD.
        if not text_parts:
            logger.info("VAD returned no speech, retrying without VAD filter")

            # Try Demucs vocal separation first (if enabled)
            from asr import vocal_separator

            if vocal_separator.is_enabled():
                logger.info("Attempting Demucs vocal separation before retry")
                try:
                    vocals_only = vocal_separator.separate_vocals(audio_float, sample_rate)
                    # Re-run Whisper on isolated vocals (with VAD, since vocals should be clean)
                    segments_iter2, info = self.model.transcribe(
                        vocals_only,
                        language=language if language else None,
                        task=task,
                        beam_size=5,
                        vad_filter=True,
                        vad_parameters=dict(
                            threshold=0.3,
                            min_silence_duration_ms=500,
                            speech_pad_ms=300,
                        ),
                    )
                    for seg in segments_iter2:
                        segments.append({
                            "text": seg.text.strip(),
                            "start": seg.start,
                            "end": seg.end,
                            "confidence": getattr(seg, "avg_log_prob", 0.0),
                        })
                        text_parts.append(seg.text.strip())
                except Exception as e:
                    logger.warning("Demucs separation failed: %s, falling back to no-VAD", e)

            # If Demucs didn't help (or isn't enabled), try raw no-VAD as last resort
            if not text_parts:
                segments_iter3, info = self.model.transcribe(
                    audio_float,
                    language=language if language else None,
                    task=task,
                    beam_size=5,
                    vad_filter=False,
                )
                for seg in segments_iter3:
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

        # Run NLLB translation if requested, translator is available,
        # text exists, and source != target language.
        if target_language and self.translator and result["text"] and result["language"] != target_language:
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
