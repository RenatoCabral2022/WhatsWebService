"""Tests for the TTS service."""

from tts.service import CHUNK_SIZE_BYTES, TTSService


def test_synthesize_yields_chunks():
    svc = TTSService()
    chunks = list(svc.synthesize("Hello, world."))
    assert len(chunks) >= 1
    for chunk in chunks:
        assert isinstance(chunk, bytes)
        assert len(chunk) == CHUNK_SIZE_BYTES
