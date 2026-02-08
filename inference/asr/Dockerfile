FROM python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN pip install --no-cache-dir uv

# Copy gen/python for proto stubs
COPY gen/python/ /app/gen/python/

# Copy ASR service source
COPY inference/asr/pyproject.toml ./
COPY inference/asr/src/ ./src/
COPY inference/asr/tests/ ./tests/
RUN uv pip install --system .

# Download NLLB translation model at build time (cached in Docker layer)
# Note: huggingface_hub is kept installed because faster_whisper needs it at runtime.
RUN pip install --no-cache-dir huggingface_hub && \
    python -c "from huggingface_hub import snapshot_download; snapshot_download('JustFrederik/nllb-200-distilled-600M-ct2-int8', local_dir='/app/models/nllb-200-distilled-600M-ct2-int8')"

ENV PYTHONPATH=/app/gen/python
ENV NLLB_MODEL_PATH=/app/models/nllb-200-distilled-600M-ct2-int8

EXPOSE 50051
CMD ["python", "-m", "asr.server"]
