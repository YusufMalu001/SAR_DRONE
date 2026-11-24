SHELL := /bin/bash
.PHONY: up down sitl demo test lint evaluate fetch-assets test-go test-ts test-e2e demo-capture test-contracts test-py test-sim test-mission lint-go lint-ts test-chaos evaluate-smoke

SCENARIO ?= flood
MODE ?= replay

up:
	docker compose up -d --build

down:
	docker compose down

sitl:
	docker compose up -d sitl

demo:
	python3 -m simulator.runner --scenario scenarios/$(SCENARIO).yaml --mode $(MODE) --verify-fusion

demo-capture:
	cd dashboard && npx playwright test e2e/capture_demo.spec.ts

test: test-go test-ts test-e2e test-contracts test-py test-sim test-mission

test-go:
	export PATH="/home/yusuf/.local/bin:/home/yusuf/.local/go/bin:$$PATH" && cd fusion-service && go test -v ./...
	export PATH="/home/yusuf/.local/bin:/home/yusuf/.local/go/bin:$$PATH" && cd shared/generated/go && go test -v ./...

test-ts:
	cd dashboard && npm run test:unit
	python3 scripts/check_no_hardcoded_numbers.py

test-e2e:
	cd dashboard && npx playwright test e2e/dashboard.spec.ts

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

test-sim:
	pytest -v simulator/

test-mission:
	PYTHONPATH=mission-service pytest -v mission-service/tests

test-chaos:
	python3 scripts/test_chaos.py

lint: lint-go lint-ts

lint-go:
	export PATH="/home/yusuf/.local/bin:/home/yusuf/.local/go/bin:$$PATH" && cd fusion-service && go vet ./...

lint-ts:
	cd dashboard && npm run lint

evaluate:
	python3 scripts/evaluate_system.py

evaluate-smoke:
	python3 scripts/evaluate_smoke.py

fetch-assets:
	./scripts/fetch-assets.sh
