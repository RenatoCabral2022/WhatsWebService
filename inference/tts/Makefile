.PHONY: install test run lint

install:
	uv pip install -e ".[dev]"

test:
	python -m pytest tests/ -v

run:
	python -m tts.server

lint:
	ruff check src/ tests/
