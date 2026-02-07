# CLAUDE.md

## Project Overview

This repository contains a **low-latency real-time audio processing platform**.

The core product continuously captures live audio from a client, maintains a rolling buffer of the **last N seconds**, and on demand:
1. snapshots recent audio,
2. runs **speech-to-text (ASR)**,
3. optionally **translates** the text,
4. generates **clear spoken audio (TTS)**,
5. streams the spoken audio back to the client in real time.

The system is designed to be:
- **low latency (interactive)**
- **streaming-first**
- **cross-platform (Web, iOS, macOS, Android)**
- **AI-runtime agnostic**

This is not a batch transcription API. It is a **real-time media system**.

---

## Chosen Architecture

**Option 1 – Fast to ship, great performance**

- Control Plane: Go (REST)
- Media Gateway: Go (WebRTC)
- Inference: Python (gRPC, long-lived workers)
- Internal Protocol: gRPC
- Client Media Protocol: WebRTC

---

## Core Mental Model

> “A real-time audio DVR with an AI-powered ‘rewind and enunciate’ button.”

All design decisions should preserve this mental model.

---

## Non‑Negotiable Constraints

- Streaming everywhere (no full buffering)
- No model loading in request paths
- Canonical audio format: PCM s16le, 16kHz, mono
- Measure p95 and p99 latency, not averages
- Explicit contracts (OpenAPI, JSON Schema, Protobuf)

---

## Repository Structure (Expected)

/control-plane        → Go REST APIs  
/webrtc-gateway       → Go WebRTC server (audio + ring buffers)  
/inference/asr        → Python ASR gRPC service  
/inference/tts        → Python TTS gRPC service  
/shared/protos        → gRPC protobufs  
/shared/schemas       → JSON schemas (data channel)  
/clients              → Reference clients  
/docs                 → Architecture & latency docs  

---

## Claude Code Instructions

When generating or modifying code:
- Preserve streaming semantics
- Respect service boundaries
- Avoid adding latency-heavy abstractions
- Do not change API contracts unless explicitly requested
- Prefer clarity and debuggability over cleverness
- When I ask you to plan or design, you must read and follow CLAUDE_CODE_PLANNING.md before responding.
---

## Success Criteria

- Realtime audio roundtrip works end-to-end
- “Enunciate last N seconds” is reliable
- Predictable latency under load
- Same backend works for browser + native apps
