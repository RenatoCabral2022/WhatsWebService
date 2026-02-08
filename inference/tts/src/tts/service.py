"""TTS service implementation using piper-tts."""

import logging
import time
from collections.abc import Iterator

import numpy as np

logger = logging.getLogger(__name__)

# Chunk size in bytes for streaming TTS output.
# At 16kHz s16le mono: 1600 samples = 100ms = 3200 bytes.
CHUNK_SIZE_BYTES = 3200


class TTSService:
    """Text-to-speech service wrapping piper-tts.

    The model is loaded ONCE at construction time.
    Synthesis streams PCM chunks as an iterator (for gRPC server streaming).
    """

    def __init__(self, model_path: str, device: str = "cpu"):
        logger.info("Loading piper TTS model: path=%s", model_path)
        start = time.monotonic()
        from piper import PiperVoice
        from piper.config import SynthesisConfig

        self.voice = PiperVoice.load(model_path)
        self.native_rate = self.voice.config.sample_rate
        self.SynthesisConfig = SynthesisConfig
        logger.info(
            "Piper model loaded in %.2fs (native_rate=%d)",
            time.monotonic() - start,
            self.native_rate,
        )

    def synthesize(
        self, text: str, voice: str = "default", speed: float = 1.0
    ) -> Iterator[bytes]:
        """Synthesize text to PCM s16le 16kHz mono audio chunks.

        Yields:
            bytes: PCM audio chunks of CHUNK_SIZE_BYTES each.
        """
        if not text.strip():
            return

        # length_scale < 1 = faster, > 1 = slower (inverse of speed)
        length_scale = 1.0 / speed if speed > 0 else 1.0
        syn_config = self.SynthesisConfig(length_scale=length_scale)

        residual = b""

        for audio_chunk in self.voice.synthesize(text, syn_config=syn_config):
            # audio_chunk.audio_int16_bytes is raw int16 PCM at native_rate
            pcm = np.frombuffer(audio_chunk.audio_int16_bytes, dtype=np.int16)

            # Resample to 16kHz if needed
            if self.native_rate != 16000:
                n_out = int(len(pcm) * 16000 / self.native_rate)
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
