FROM python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN pip install --no-cache-dir uv

# Download piper voice models at build time (cached in Docker layer)
RUN mkdir -p /app/models && \
    # English (US)
    wget -q "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/en/en_US/lessac/medium/en_US-lessac-medium.onnx" \
      -O /app/models/en_US-lessac-medium.onnx && \
    wget -q "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json" \
      -O /app/models/en_US-lessac-medium.onnx.json && \
    # Portuguese (Brazil)
    wget -q "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/pt/pt_BR/faber/medium/pt_BR-faber-medium.onnx" \
      -O /app/models/pt_BR-faber-medium.onnx && \
    wget -q "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/pt/pt_BR/faber/medium/pt_BR-faber-medium.onnx.json" \
      -O /app/models/pt_BR-faber-medium.onnx.json && \
    # Spanish (Mexico)
    wget -q "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_MX/ald/medium/es_MX-ald-medium.onnx" \
      -O /app/models/es_MX-ald-medium.onnx && \
    wget -q "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_MX/ald/medium/es_MX-ald-medium.onnx.json" \
      -O /app/models/es_MX-ald-medium.onnx.json && \
    # Fix phoneme_type format in voice configs (some models use "PhonemeType.ESPEAK" instead of "espeak")
    sed -i 's/"PhonemeType.ESPEAK"/"espeak"/g' /app/models/*.onnx.json

# Copy gen/python for proto stubs
COPY gen/python/ /app/gen/python/

# Copy TTS service source
COPY inference/tts/pyproject.toml ./
COPY inference/tts/src/ ./src/
COPY inference/tts/tests/ ./tests/
RUN uv pip install --system .

ENV PYTHONPATH=/app/gen/python
ENV TTS_MODELS_DIR=/app/models

EXPOSE 50052
CMD ["python", "-m", "tts.server"]
