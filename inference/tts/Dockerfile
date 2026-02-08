FROM python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN pip install --no-cache-dir uv

# Download piper model at build time (cached in Docker layer)
RUN mkdir -p /app/models && \
    wget -q "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/en/en_US/lessac/medium/en_US-lessac-medium.onnx" \
      -O /app/models/en_US-lessac-medium.onnx && \
    wget -q "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json" \
      -O /app/models/en_US-lessac-medium.onnx.json

# Copy gen/python for proto stubs
COPY gen/python/ /app/gen/python/

# Copy TTS service source
COPY inference/tts/pyproject.toml ./
COPY inference/tts/src/ ./src/
COPY inference/tts/tests/ ./tests/
RUN uv pip install --system .

ENV PYTHONPATH=/app/gen/python
ENV TTS_MODEL_PATH=/app/models/en_US-lessac-medium.onnx

EXPOSE 50052
CMD ["python", "-m", "tts.server"]
