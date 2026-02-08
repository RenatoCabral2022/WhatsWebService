# Latency Budget

## Target: p95 < 3s for enunciate (5s lookback, short utterance)

## Measured Latency (CPU, Phase 3+4)

| Stage | p50 | p95 | Notes |
|-------|-----|-----|-------|
| Snapshot | <0.1ms | <0.1ms | Ring buffer memory copy |
| ASR (Whisper base, CPU, int8) | ~700ms | ~800ms | 5s audio, faster-whisper |
| TTS first chunk (Piper, CPU) | ~50ms | ~125ms | en_US-lessac-medium |
| TTS total playout | ~1.5s | ~4.5s | Proportional to text length |
| **Total E2E** | **~2s** | **~5.5s** | Short utterance |

## Methodology

- Measured via structured logs (Phase 3+4 development runs)
- Prometheus histograms available at `:9092/metrics` (Phase 5)
- CPU-only, single-session, Docker Compose on WSL2
- Whisper base model, int8 quantization
- Piper en_US-lessac-medium ONNX model

## Prometheus Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `whats_gateway_action_duration_ms` | Histogram | `stage`: total, snapshot, asr, tts_first_chunk |
| `whats_gateway_active_sessions` | Gauge | — |
| `whats_gateway_active_actions` | Gauge | — |
| `whats_gateway_actions_total` | Counter | `outcome`: success, cancelled, asr_error, tts_error, rate_limited, timeout |

## Known Bottlenecks

1. **ASR** is the dominant cost (~700ms for 5s of audio on CPU)
2. **TTS playout** is real-time (proportional to output text length)
3. **Snapshot** is negligible (<0.1ms)

## Optimization Opportunities

- GPU inference: 5-10x faster ASR, 3-5x faster TTS
- Whisper small/tiny model: faster but lower accuracy
- Streaming ASR: reduce time-to-first-token
- Shorter lookback: 3s instead of 5s reduces ASR input proportionally
