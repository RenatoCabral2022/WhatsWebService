"""Translation service using CTranslate2 + NLLB-200-distilled-600M."""

import logging
import time
from pathlib import Path

import ctranslate2
import sentencepiece as spm

logger = logging.getLogger(__name__)

# BCP-47 → NLLB flores-200 language codes.
LANG_MAP = {
    "en": "eng_Latn",
    "pt": "por_Latn",
    "pt-BR": "por_Latn",
    "es": "spa_Latn",
}


class Translator:
    """Translates text using CTranslate2 + NLLB-200-distilled-600M.

    Model is loaded ONCE at construction time (startup), never in request path.
    """

    def __init__(
        self,
        model_path: str,
        device: str = "cpu",
        compute_type: str = "int8",
    ):
        logger.info("Loading NLLB translation model: path=%s device=%s", model_path, device)
        start = time.monotonic()

        # Pin threads to match container CPU limits (avoids contention when
        # host has more CPUs than the container is allowed).
        self.model = ctranslate2.Translator(
            model_path,
            device=device,
            compute_type=compute_type,
            inter_threads=1,
            intra_threads=2,
        )

        # SentencePiece tokenizer lives alongside the CT2 model.
        sp_path = Path(model_path) / "sentencepiece.bpe.model"
        if not sp_path.exists():
            # Some CT2 conversions place it as source.spm or similar.
            candidates = list(Path(model_path).glob("*.model"))
            if candidates:
                sp_path = candidates[0]
            else:
                raise FileNotFoundError(
                    f"No SentencePiece model found in {model_path}"
                )

        self.sp = spm.SentencePieceProcessor()
        self.sp.Load(str(sp_path))

        elapsed = time.monotonic() - start
        logger.info("NLLB model loaded in %.2fs", elapsed)

        # Warm up: run a dummy translation to trigger CTranslate2 JIT compilation
        # at startup rather than on the first real request.
        self._warmup()

    def _warmup(self):
        """Run dummy translations to trigger CTranslate2 JIT compilation.

        Covers all common translation directions so the first real
        request in any direction is fast.
        """
        logger.info("Warming up NLLB translator...")
        start = time.monotonic()
        pairs = [
            ("eng_Latn", "por_Latn"),
            ("eng_Latn", "spa_Latn"),
            ("por_Latn", "eng_Latn"),
            ("por_Latn", "spa_Latn"),
            ("spa_Latn", "eng_Latn"),
            ("spa_Latn", "por_Latn"),
        ]
        warmup_text = "Hello world"
        for src_code, tgt_code in pairs:
            tokens = [src_code] + self.sp.Encode(warmup_text, out_type=str) + ["</s>"]
            self.model.translate_batch(
                [tokens],
                target_prefix=[[tgt_code]],
                beam_size=1,
                max_decoding_length=32,
            )
        elapsed = time.monotonic() - start
        logger.info("NLLB warmup complete in %.2fs (%d pairs)", elapsed, len(pairs))

    def translate(
        self,
        text: str,
        source_lang: str,
        target_lang: str,
        timeout_ms: int = 250,
    ) -> dict:
        """Translate text from source to target language.

        Returns dict with:
            translated_text: The translated string (or original on fallback).
            source_lang: BCP-47 source language used.
            target_lang: BCP-47 target language requested.
            duration_ms: Wall-clock time for translation.
            fallback_used: True if original text returned due to error/timeout.
        """
        # Same language → skip translation.
        src_nllb = LANG_MAP.get(source_lang)
        tgt_nllb = LANG_MAP.get(target_lang)

        if src_nllb and tgt_nllb and src_nllb == tgt_nllb:
            return {
                "translated_text": text,
                "source_lang": source_lang,
                "target_lang": target_lang,
                "duration_ms": 0,
                "fallback_used": False,
            }

        # Unknown language codes → fallback.
        if not src_nllb:
            logger.warning("Unknown source language '%s', skipping translation", source_lang)
            return self._fallback(text, source_lang, target_lang, 0)
        if not tgt_nllb:
            logger.warning("Unknown target language '%s', skipping translation", target_lang)
            return self._fallback(text, source_lang, target_lang, 0)

        start = time.monotonic()
        try:
            # Tokenize with SentencePiece.
            tokens = self.sp.Encode(text, out_type=str)

            # NLLB CTranslate2 format: [src_lang] + tokens + [</s>]
            tokens = [src_nllb] + tokens + ["</s>"]

            # Translate with CTranslate2.
            results = self.model.translate_batch(
                [tokens],
                target_prefix=[[tgt_nllb]],
                beam_size=1,
                max_decoding_length=256,
                repetition_penalty=1.2,
                no_repeat_ngram_size=3,
            )

            # Detokenize: skip the target language prefix token.
            output_tokens = results[0].hypotheses[0]
            if output_tokens and output_tokens[0] == tgt_nllb:
                output_tokens = output_tokens[1:]

            translated = self.sp.Decode(output_tokens)
            duration_ms = int((time.monotonic() - start) * 1000)

            if duration_ms > timeout_ms:
                logger.warning(
                    "Translation took %dms (timeout=%dms), using result anyway",
                    duration_ms,
                    timeout_ms,
                )

            return {
                "translated_text": translated,
                "source_lang": source_lang,
                "target_lang": target_lang,
                "duration_ms": duration_ms,
                "fallback_used": False,
            }

        except Exception:
            duration_ms = int((time.monotonic() - start) * 1000)
            logger.exception("Translation failed after %dms", duration_ms)
            return self._fallback(text, source_lang, target_lang, duration_ms)

    @staticmethod
    def _fallback(text: str, source_lang: str, target_lang: str, duration_ms: int) -> dict:
        return {
            "translated_text": text,
            "source_lang": source_lang,
            "target_lang": target_lang,
            "duration_ms": duration_ms,
            "fallback_used": True,
        }
