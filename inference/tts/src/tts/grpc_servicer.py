"""gRPC servicer adapter for the TTS service."""

import logging

from whats.v1 import common_pb2, tts_pb2, tts_pb2_grpc

logger = logging.getLogger(__name__)


class TtsServicer(tts_pb2_grpc.TtsServiceServicer):
    """Bridges the generated gRPC interface to the TTSService."""

    def __init__(self, tts_service):
        self.tts = tts_service

    def Synthesize(self, request, context):
        logger.info(
            "Synthesize request: session=%s action=%s text_len=%d voice=%s language=%s",
            request.session_id,
            request.action_id,
            len(request.text),
            request.voice or "default",
            request.language or "en",
        )

        seq = 0
        for pcm_chunk in self.tts.synthesize(
            text=request.text,
            voice=request.voice or "default",
            speed=request.speed if request.speed > 0 else 1.0,
            language=request.language or "en",
        ):
            yield tts_pb2.SynthesizeResponse(
                chunk=common_pb2.AudioChunk(
                    data=pcm_chunk,
                    sequence=seq,
                    duration_ms=100,  # 3200 bytes = 100ms at 16kHz
                    is_final=False,
                ),
            )
            seq += 1

        # Send final marker
        yield tts_pb2.SynthesizeResponse(
            chunk=common_pb2.AudioChunk(
                data=b"",
                sequence=seq,
                is_final=True,
            ),
        )

        logger.info("Synthesize complete: session=%s chunks=%d", request.session_id, seq)
