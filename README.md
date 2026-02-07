# Realtime Audio Enunciate Platform

## What This Is

This project is a **real-time audio processing platform** that lets users:

- stream live microphone audio,
- rewind the last few seconds,
- transcribe what was said,
- optionally translate it,
- and hear it spoken back clearly.

Think of it as:

> **A real-time audio DVR powered by AI.**

---

## Key Features

- 🎙️ Live microphone streaming
- ⏪ Snapshot the last N seconds of audio
- 🧠 Speech-to-text (ASR)
- 🌍 Optional translation
- 🔊 Text-to-speech playback
- ⚡ Low-latency, streaming-first design

---

## Supported Clients

- Web (browser, WebRTC)
- iOS / macOS (WebRTC native)
- Android (WebRTC or gRPC streaming)

All clients use the **same backend architecture**.

---

## High-Level Architecture

- **Control Plane (Go, REST)**
  - Authentication
  - Session lifecycle
  - WebRTC negotiation

- **Media Plane (Go, WebRTC)**
  - Audio ingest & playback
  - Ring buffers
  - Real-time commands

- **Inference Plane (Python, gRPC)**
  - ASR (Whisper)
  - TTS
  - Long-lived, warm workers

Detailed diagrams live in `docs/architecture.md`.

---

## Why WebRTC

WebRTC is used because it:
- is designed for real-time audio,
- works natively in browsers,
- supports low-latency streaming,
- handles jitter and network variability well.

This is not a file-upload API.

---

## Repository Layout

```
control-plane/        Go REST API
webrtc-gateway/       Go WebRTC gateway
inference/
  ├── asr/            Python ASR service
  └── tts/            Python TTS service
shared/
  ├── protos/         gRPC contracts
  └── schemas/        Data channel JSON schemas
clients/              Reference clients
docs/                 Architecture & latency docs
```

---

## Documentation

- `CLAUDE.md` – Core project context and constraints
- `CLAUDE_CODE_PLANNING.md` – Strict planning rules for Claude Code
- `docs/architecture.md` – System architecture
- `docs/latency.md` – Latency budgets and measurement

---

## Status

This project is under active development.
Expect rapid iteration and evolving models, but **stable contracts**.

---

## Guiding Principle

> Optimize for time-to-first-audio and clarity over complexity.
