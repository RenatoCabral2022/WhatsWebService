"""TTS service implementation using piper-tts."""

import logging
import time
from collections.abc import Iterator
from pathlib import Path

import numpy as np

from tts.config import DEFAULT_VOICE, VOICE_MAP

logger = logging.getLogger(__name__)

# Chunk size in bytes for streaming TTS output.
# At 16kHz s16le mono: 1600 samples = 100ms = 3200 bytes.
CHUNK_SIZE_BYTES = 3200


class TTSService:
    """Text-to-speech service wrapping piper-tts with multi-voice support.

    All voice models are loaded ONCE at construction time.
    Synthesis streams PCM chunks as an iterator (for gRPC server streaming).
    """

    def __init__(self, models_dir: str, device: str = "cpu"):
        from piper import PiperVoice
        from piper.config import SynthesisConfig

        self.SynthesisConfig = SynthesisConfig
        self.voices = {}  # model_name → PiperVoice
        self.native_rates = {}  # model_name → int

        models_path = Path(models_dir)
        for onnx_file in sorted(models_path.glob("*.onnx")):
            name = onnx_file.stem  # e.g. "en_US-lessac-medium"
            logger.info("Loading piper voice: %s", name)
            start = time.monotonic()
            voice = PiperVoice.load(str(onnx_file))
            native_rate = voice.config.sample_rate
            self.voices[name] = voice
            self.native_rates[name] = native_rate
            logger.info(
                "  loaded in %.2fs (native_rate=%d)", time.monotonic() - start, native_rate
            )

        if not self.voices:
            raise RuntimeError(f"No .onnx voice models found in {models_dir}")

        logger.info("Loaded %d voice(s): %s", len(self.voices), list(self.voices.keys()))

    def _resolve_voice(self, voice: str, language: str) -> str:
        """Resolve a voice name from explicit voice param or language mapping."""
        # Explicit voice name that exists → use it.
        if voice != "default" and voice in self.voices:
            return voice

        # Language → voice mapping.
        mapped = VOICE_MAP.get(language, DEFAULT_VOICE)
        if mapped in self.voices:
            return mapped

        # Fall back to default voice.
        if DEFAULT_VOICE in self.voices:
            return DEFAULT_VOICE

        # Last resort: first loaded voice.
        return next(iter(self.voices))

    def synthesize(
        self, text: str, voice: str = "default", speed: float = 1.0, language: str = "en"
    ) -> Iterator[bytes]:
        """Synthesize text to PCM s16le 16kHz mono audio chunks.

        Args:
            text: Text to synthesize.
            voice: Explicit voice model name, or "default" for auto-select.
            speed: Speech speed multiplier (1.0 = normal).
            language: BCP-47 language code for voice auto-selection.

        Yields:
            bytes: PCM audio chunks of CHUNK_SIZE_BYTES each.
        """
        if not text.strip():
            return

        voice_name = self._resolve_voice(voice, language)
        piper_voice = self.voices[voice_name]
        native_rate = self.native_rates[voice_name]

        logger.info("Synthesizing with voice=%s (language=%s)", voice_name, language)

        # length_scale < 1 = faster, > 1 = slower (inverse of speed)
        length_scale = 1.0 / speed if speed > 0 else 1.0
        syn_config = self.SynthesisConfig(length_scale=length_scale)

        residual = b""

        for audio_chunk in piper_voice.synthesize(text, syn_config=syn_config):
            # audio_chunk.audio_int16_bytes is raw int16 PCM at native_rate
            pcm = np.frombuffer(audio_chunk.audio_int16_bytes, dtype=np.int16)

            # Resample to 16kHz if needed
            if native_rate != 16000:
                n_out = int(len(pcm) * 16000 / native_rate)
                if n_out == 0:
                    continue
                x_old = np.arange(len(pcm))
                x_new = np.linspace(0, len(pcm) - 1, n_out)
                pcm_resampled = np.interp(x_new, x_old, pcm.astype(np.float64))
                pcm = np.clip(pcm_resampled, -32768, 32767).astype(np.int16)

            raw_16k = residual + pcm.tobytes()
            residual = b""

            offset = 0
            while offset + CHUNK_SIZE_BYTES <= len(raw_16k):
                yield raw_16k[offset : offset + CHUNK_SIZE_BYTES]
                offset += CHUNK_SIZE_BYTES

            if offset < len(raw_16k):
                residual = raw_16k[offset:]

        # Yield final residual padded with silence
        if residual:
            yield residual + b"\x00" * (CHUNK_SIZE_BYTES - len(residual))
