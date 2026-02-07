# latency.md

## Latency Philosophy

Latency is a **product feature**, not an implementation detail.

We optimize for:
- Time-to-first-audio
- Predictable p95 / p99 latency
- Graceful degradation under load

---

## Canonical Latency Path

t0  Command received  
t1  Ring buffer snapshot complete  
t2  ASR request sent  
t3  ASR response received  
t4  TTS request sent  
t5  First TTS audio chunk received  
t6  Audio frame sent to client  

End-to-end latency = t6 - t0

---

## Target Latency Budget (p95)

| Stage                    | Target |
|--------------------------|--------|
| Snapshot ring buffer     | < 5 ms |
| ASR inference            | < 250 ms |
| Translation (optional)   | < 40 ms |
| TTS (first audio chunk)  | < 200 ms |
| WebRTC egress            | < 50 ms |
| **Total**                | **< 600 ms** |

---

## Measurement Strategy

- Timestamp every stage
- Emit `metrics.latency` events
- Track p50, p95, p99
- Correlate by sessionId + actionId

---

## Common Latency Killers

- Loading models per request
- Buffering full audio clips
- Excessive copying of PCM buffers
- Unbounded queues
- Cold autoscaling of inference workers

---

## Optimization Techniques

- Pre-warmed worker pools
- Pooled byte buffers
- Canonical audio format everywhere
- Micro-batching (carefully)
- Backpressure and admission control

---

## Degradation Strategy

When overloaded:
- Reject new sessions early
- Limit concurrent enunciate actions
- Reduce model size or quality
- Prefer fast failure over slow responses

---

## Rule of Thumb

> If latency is surprising, instrumentation is missing.
