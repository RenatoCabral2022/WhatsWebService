# Data Channel Protocol

## Overview

All real-time commands and events between the client and the WebRTC Gateway
are exchanged over a WebRTC DataChannel named `commands`.

Messages are JSON objects following the envelope format.

---

## Envelope Format

Every message MUST have:

| Field       | Type    | Required | Description                          |
|-------------|---------|----------|--------------------------------------|
| `type`      | string  | yes      | Message type identifier              |
| `sessionId` | string  | yes      | UUID of the owning session           |
| `timestamp` | integer | yes      | Unix timestamp in milliseconds       |
| `actionId`  | string  | commands | UUID unique to this action           |
| `payload`   | object  | yes      | Type-specific payload                |

---

## Client -> Server Messages

### `command.enunciate`

Triggers the "rewind and enunciate" flow:

1. Gateway snapshots last N seconds from ring buffer
2. Sends snapshot to ASR service (gRPC)
3. Optionally translates (Whisper built-in)
4. Sends text to TTS service (gRPC, server-streaming)
5. Streams synthesized audio back to client via WebRTC

**Payload:**

| Field             | Type    | Required | Description                        |
|-------------------|---------|----------|------------------------------------|
| `lookbackSeconds` | integer | yes      | Seconds to rewind (1-30)           |
| `targetLanguage`  | string  | no       | BCP-47 code for translation        |
| `ttsOptions`      | object  | no       | `{ voice: string, speed: number }` |

### `command.update`

Updates parameters for an in-progress action or session defaults.

**Payload:**

| Field            | Type   | Required | Description                        |
|------------------|--------|----------|------------------------------------|
| `targetLanguage` | string | no       | Update target language             |
| `ttsOptions`     | object | no       | `{ voice: string, speed: number }` |

---

## Server -> Client Messages

### `asr.partial`

Partial transcription result (sent during ASR if supported).

**Payload:** `{ text: string, language?: string }`

### `asr.final`

Final transcription result.

**Payload:**

| Field         | Type     | Description                          |
|---------------|----------|--------------------------------------|
| `text`        | string   | Full transcription                   |
| `language`    | string   | Detected language (BCP-47)           |
| `segments`    | array    | Time-aligned segments                |
| `inferenceMs` | integer  | ASR inference duration               |

### `tts.started`

Notification that TTS audio will begin streaming on the media track.

**Payload:** `{ voice?: string, estimatedDurationMs?: integer }`

### `metrics.latency`

Per-action latency breakdown, sent after an action completes.

**Payload:**

| Field             | Type   | Description                     |
|-------------------|--------|---------------------------------|
| `snapshotMs`      | number | Ring buffer snapshot duration    |
| `asrMs`           | number | ASR inference duration           |
| `translateMs`     | number | Translation duration (0 if none)|
| `ttsFirstChunkMs` | number | Time to first TTS audio chunk   |
| `totalMs`         | number | Total end-to-end latency        |

### `error`

Error notification. May be action-scoped (has `actionId`) or session-scoped.

**Payload:**

| Field     | Type   | Description                          |
|-----------|--------|--------------------------------------|
| `code`    | string | Machine-readable error code          |
| `message` | string | Human-readable description           |
| `details` | any    | Optional additional context          |

**Error codes:** `INVALID_COMMAND`, `SESSION_NOT_FOUND`, `ASR_FAILED`, `TTS_FAILED`, `BUFFER_EMPTY`, `RATE_LIMITED`, `INTERNAL_ERROR`

---

## Identity Model

- `sessionId` is assigned by the control plane at session creation
- `actionId` is assigned by the **client** for each `command.enunciate`
- All events related to an action carry the same `actionId`
- Metrics are always scoped to `sessionId` + `actionId`

---

## Schema Files

JSON Schema definitions are located in `shared/schemas/`.
