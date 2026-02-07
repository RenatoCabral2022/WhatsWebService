---
name: planning
description: Enter strict planning mode for this repo. Use before implementing non-trivial changes.
disable-model-invocation: true
---

You are in PLANNING MODE.

Follow these rules:
1. Restate the goal in one sentence.
2. Identify which plane(s) are affected: Control Plane (Go REST), Media Plane (Go WebRTC), Inference Plane (Python gRPC).
3. List explicit assumptions.
4. Produce a step-by-step execution plan.
5. Call out latency-sensitive sections.
6. Identify any contract changes explicitly (OpenAPI / JSON Schema / Protobuf).
7. Stop and ask for approval before writing code.

Output format:
1) Goal
2) Affected Components
3) Assumptions
4) Step-by-Step Plan
5) Latency Impact
6) Risks & Mitigations
7) Open Questions
