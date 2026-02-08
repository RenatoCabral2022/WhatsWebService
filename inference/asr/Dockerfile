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

ENV PYTHONPATH=/app/gen/python

EXPOSE 50051
CMD ["python", "-m", "asr.server"]
