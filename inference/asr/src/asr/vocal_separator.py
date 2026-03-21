"""Vocal separation using Demucs (htdemucs).

Isolates vocals from music so Whisper ASR only sees clean vocals.
Used as a fallback when VAD strips all audio (vocals mixed too quietly).
"""

import logging
import os
import time

import numpy as np

logger = logging.getLogger(__name__)

# Lazy-loaded globals
_separator = None
_device = None


def _get_separator():
    """Lazy-load the Demucs model (heavy import, ~1GB memory)."""
    global _separator, _device

    if _separator is not None:
        return _separator

    import torch
    from demucs.pretrained import get_model
    from demucs.apply import BagOfModels

    _device = torch.device("cuda" if torch.cuda.is_available() else "cpu")

    logger.info("Loading Demucs htdemucs model on %s...", _device)
    start = time.monotonic()

    model = get_model("htdemucs")
    if isinstance(model, BagOfModels):
        # BagOfModels wraps multiple models; use as-is
        pass
    model.to(_device)
    model.eval()

    _separator = model
    logger.info("Demucs model loaded in %.2fs", time.monotonic() - start)
    return _separator


def is_enabled() -> bool:
    """Check if vocal separation is enabled via environment variable."""
    return os.getenv("VOCAL_SEPARATION_ENABLED", "true").lower() == "true"


def separate_vocals(audio_float: np.ndarray, sample_rate: int = 16000) -> np.ndarray:
    """Separate vocals from audio using Demucs.

    Args:
        audio_float: Mono float32 audio in [-1, 1], shape (num_samples,)
        sample_rate: Input sample rate (will be resampled to 44100 for Demucs)

    Returns:
        Mono float32 vocals-only audio at the original sample_rate
    """
    import torch
    import torchaudio

    model = _get_separator()

    start = time.monotonic()

    # Demucs expects stereo 44100Hz
    # Convert mono to stereo
    audio_tensor = torch.from_numpy(audio_float).float().unsqueeze(0)  # (1, samples)
    stereo = audio_tensor.repeat(2, 1)  # (2, samples) — duplicate mono to stereo

    # Resample to 44100 if needed
    if sample_rate != 44100:
        resampler = torchaudio.transforms.Resample(sample_rate, 44100)
        stereo = resampler(stereo)

    # Add batch dimension: (1, 2, samples)
    stereo = stereo.unsqueeze(0).to(_device)

    # Run separation
    with torch.no_grad():
        sources = model(stereo)
        # sources shape: (1, num_sources, 2, samples)
        # Source order for htdemucs: drums, bass, other, vocals
        vocals = sources[0, 3]  # (2, samples) — vocals stem

    # Convert back to mono
    vocals_mono = vocals.mean(dim=0)  # (samples,)

    # Resample back to original rate if needed
    if sample_rate != 44100:
        resampler_back = torchaudio.transforms.Resample(44100, sample_rate)
        vocals_mono = resampler_back(vocals_mono.unsqueeze(0)).squeeze(0)

    result = vocals_mono.cpu().numpy()

    duration_ms = int((time.monotonic() - start) * 1000)
    logger.info(
        "Vocal separation complete: input_samples=%d output_samples=%d duration_ms=%d",
        len(audio_float),
        len(result),
        duration_ms,
    )

    return result
