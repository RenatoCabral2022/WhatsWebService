"""gRPC servicer adapter for the ASR service."""

import logging
import time

from whats.v1 import asr_pb2, asr_pb2_grpc

logger = logging.getLogger(__name__)


class AsrServicer(asr_pb2_grpc.AsrServiceServicer):
    """Bridges the generated gRPC interface to the ASRService."""

    def __init__(self, asr_service):
        self.asr = asr_service

    def Transcribe(self, request, context):
        # Detect text-only translation mode:
        # When audio is empty and language_hint starts with "source_text:",
        # extract the source text and language for NLLB-only translation.
        source_text = None
        language_hint = request.language_hint or None
        if len(request.audio) == 0 and request.language_hint and request.language_hint.startswith("source_text:"):
            # Format: "source_text:<lang>:<text>"
            parts = request.language_hint.split(":", 2)
            if len(parts) == 3:
                language_hint = parts[1] or None
                source_text = parts[2]
                logger.info(
                    "Text-only translation request: session=%s action=%s source_lang=%s text_len=%d target_lang=%s",
                    request.session_id,
                    request.action_id,
                    language_hint or "auto",
                    len(source_text),
                    request.target_language or "(none)",
                )

        if source_text is None:
            logger.info(
                "Transcribe request: session=%s action=%s audio_len=%d task=%s target_lang=%s",
                request.session_id,
                request.action_id,
                len(request.audio),
                request.task or "transcribe",
                request.target_language or "(none)",
            )

        sample_rate = request.format.sample_rate if request.format else 16000
        result = self.asr.transcribe(
            audio_bytes=request.audio,
            sample_rate=sample_rate,
            language=language_hint,
            task=request.task or "transcribe",
            target_language=request.target_language or None,
            source_text=source_text,
        )

        segments = [
            asr_pb2.Segment(
                text=s["text"],
                start_time=s["start"],
                end_time=s["end"],
                confidence=s.get("confidence", 0.0),
            )
            for s in result["segments"]
        ]

        logger.info(
            "Transcribe result: text_len=%d language=%s segments=%d inference_ms=%d"
            " translated=%s translate_ms=%d",
            len(result["text"]),
            result["language"],
            len(segments),
            result["inference_duration_ms"],
            bool(result.get("translated_text")),
            result.get("translate_duration_ms", 0),
        )
        if result.get("translated_text"):
            logger.info(
                "Translation: '%s' -> '%s'",
                result["text"][:200],
                result["translated_text"][:200],
            )

        return asr_pb2.TranscribeResponse(
            text=result["text"],
            language=result["language"],
            segments=segments,
            inference_duration_ms=result["inference_duration_ms"],
            translated_text=result.get("translated_text", ""),
            target_language=result.get("target_language", ""),
            translate_duration_ms=result.get("translate_duration_ms", 0),
        )
