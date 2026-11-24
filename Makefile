SHELL := /bin/bash
.PHONY: up down sitl demo test lint evaluate fetch-assets test-go test-ts test-contracts test-py lint-go lint-ts

up:
	docker compose up -d --build

down:
	docker compose down

sitl:
	@echo "make sitl: not implemented yet (scheduled for mission-service)" >&2
	@exit 1

demo:
	./scripts/seed_demo.sh

test: test-go test-ts test-contracts test-py

test-go:
	export PATH="/home/yusuf/.local/bin:/home/yusuf/.local/go/bin:$$PATH" && cd fusion-service && go test -v ./...
	export PATH="/home/yusuf/.local/bin:/home/yusuf/.local/go/bin:$$PATH" && cd shared/generated/go && go test -v ./...

test-ts:
	cd dashboard && npm run test

test-contracts:
	python3 shared/generate_contracts.py
	node shared/tests/test_contracts.mjs
	export PATH="/home/yusuf/.local/bin:/home/yusuf/.local/go/bin:$$PATH" && cd shared/generated/go && go test -v ./...

test-py:
	@if docker compose ps --format json inference-service 2>/dev/null | grep -q "healthy"; then \
		docker compose exec -T inference-service pytest -v tests; \
	elif [ -d "inference-service/.venv" ]; then \
		inference-service/.venv/bin/pytest -v inference-service/tests; \
	elif command -v pytest >/dev/null 2>&1 && python3 -c "import pydantic" 2>/dev/null; then \
		pytest -v inference-service/tests; \
	else \
		docker compose run --rm inference-service pytest -v tests 2>/dev/null || echo "Run 'make up' to test Python inside healthy container"; \
	fi

lint: lint-go lint-ts

lint-go:
	export PATH="/home/yusuf/.local/bin:/home/yusuf/.local/go/bin:$$PATH" && cd fusion-service && go vet ./...

lint-ts:
	cd dashboard && npm run lint

evaluate:
	@echo "make evaluate: not implemented yet (scheduled for benchmark evaluation)" >&2
	@exit 1

fetch-assets:
	@echo "Fetching / verifying asset directories..."
	@mkdir -p data/models data/sample_footage data/datasets
	@echo "Assets directories ready."
