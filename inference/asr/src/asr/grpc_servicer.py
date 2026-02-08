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
            language=request.language_hint or None,
            task=request.task or "transcribe",
            target_language=request.target_language or None,
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

        return asr_pb2.TranscribeResponse(
            text=result["text"],
            language=result["language"],
            segments=segments,
            inference_duration_ms=result["inference_duration_ms"],
            translated_text=result.get("translated_text", ""),
            target_language=result.get("target_language", ""),
            translate_duration_ms=result.get("translate_duration_ms", 0),
        )
