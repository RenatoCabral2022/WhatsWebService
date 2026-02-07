"""Tests for the ASR service."""

from asr.service import ASRService


def test_transcribe_returns_expected_shape():
    svc = ASRService()
    result = svc.transcribe(b"\x00" * 32000)  # 1 second of silence
    assert "text" in result
    assert "language" in result
    assert "segments" in result


def test_transcribe_with_language_hint():
    svc = ASRService()
    result = svc.transcribe(b"\x00" * 32000, language="pt")
    assert result["language"] == "pt"
