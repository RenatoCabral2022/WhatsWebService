# architecture.md

## Overview

This system is a **real-time audio processing platform** built around WebRTC and streaming AI inference.

It is divided into **three logical planes**:
1. Control Plane (REST)
2. Media Plane (WebRTC)
3. Inference Plane (gRPC)

---

## Control Plane (Go)

Responsibilities:
- Authentication & authorization
- Session creation & teardown
- WebRTC SDP negotiation
- Rate limiting & quotas

Characteristics:
- Stateless
- Horizontally scalable
- REST + JSON
- No audio data flows through this plane

---

## Media Plane – WebRTC Gateway (Go)

Responsibilities:
- Accept WebRTC connections
- Receive microphone audio
- Decode & normalize audio
- Maintain per-session ring buffers
- Execute commands (e.g. enunciate)
- Stream synthesized audio back

Key Components:
- WebRTC PeerConnection (Pion)
- Audio decoder & resampler
- Ring buffer (fixed-size PCM)
- Data channel command/event router

This is the **latency-critical core** of the system.

---

## Inference Plane (Python)

Services:
- ASR Service (Whisper)
- TTS Service

Characteristics:
- gRPC only (no public access)
- Long-lived processes
- Models loaded once at startup
- Scale independently from gateway

---

## Data Flow (Realtime)

1. Client creates session (REST)
2. Client connects via WebRTC
3. Audio frames stream to gateway
4. Gateway writes to ring buffer
5. Client sends command.enunciate
6. Gateway snapshots last N seconds
7. ASR → optional translate → TTS
8. Audio streamed back to client

---

## Why WebRTC

- Designed for real-time audio
- Built-in jitter buffering
- NAT traversal
- Works in browsers and native apps
- Lower latency than WebSockets for audio

---

## Failure Domains

- Control plane failure ≠ media failure
- Inference overload ≠ WebRTC disconnect
- Session isolation is critical

---

## Design Principles

- Streaming > batching
- Explicit contracts
- Observable by default
- Optimize for time-to-first-audio
