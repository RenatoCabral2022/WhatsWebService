# CLAUDE_CODE_PLANNING.md

## Purpose

This file exists to guide **Claude Code in planning mode**.

It is intentionally **directive, strict, and opinionated**.
If there is a conflict between this file and human preferences, **ask before proceeding**.

Claude should treat this file as a **source-of-truth for planning and task decomposition**.

---

## Planning Mode Rules

When entering planning mode, Claude MUST:

1. **Restate the goal in one sentence**
2. **Identify which plane(s) are affected**
   - Control Plane (REST, Go)
   - Media Plane (WebRTC, Go)
   - Inference Plane (Python, gRPC)
3. **List explicit assumptions**
4. **Produce a step-by-step execution plan**
5. **Call out latency-sensitive sections**
6. **Identify contract changes explicitly**
7. **Stop and wait for confirmation before coding**

Claude must NOT:
- Start coding immediately
- Change API schemas silently
- Collapse multiple architectural layers into one
- Introduce new infrastructure without justification

---

## Architectural Invariants (Do Not Break)

These rules are absolute unless explicitly overridden.

### Media & Streaming
- Audio is streamed, never batch-buffered
- Ring buffer semantics must be preserved
- Snapshot = copy last N seconds only
- No file-based audio pipelines in realtime paths

### AI Inference
- Models are loaded once at service startup
- Inference services are stateless per request
- Gateway never performs ML inference itself

### Audio Format
- Canonical internal format:
  - PCM s16le
  - 16 kHz
  - mono

Conversions happen once, as early as possible.

---

## Latency Rules

Claude must assume:
- p95 latency matters more than mean
- Time-to-first-audio is the primary KPI
- Any additional buffering must be justified

If a plan adds >50ms to p95 latency, Claude must:
- Call it out explicitly
- Offer an alternative

---

## Contracts & Boundaries

Claude must respect the following boundaries:

- REST = control only (no audio)
- WebRTC = all realtime media
- gRPC = internal service-to-service only
- JSON Schemas + Protobufs = contracts

If a change touches:
- OpenAPI
- JSON Schema
- Protobuf

Claude must:
1. Explain why
2. Show the diff conceptually
3. Ask for approval

---

## Planning Output Format

Claude planning responses should follow this structure:

1. **Goal**
2. **Affected Components**
3. **Assumptions**
4. **Step-by-Step Plan**
5. **Latency Impact**
6. **Risks & Mitigations**
7. **Open Questions**

No code until approval is given.

---

## Mental Check

Before finalizing a plan, Claude should ask:

> “Does this preserve the idea of a real-time audio DVR with a rewind-and-enunciate button?”

If not, the plan is wrong.
