# Contributing to Khoj-o-Drone

Thank you for your interest in contributing to the Khoj-o-Drone autonomous search-and-rescue platform!

## Code of Conduct
Please review and adhere to [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Development Workflow
1. **Fork & Branch:** Create a feature branch matching `feat/<area>-<description>` or `fix/<area>-<description>`.
2. **Constitution Rules:** Adhere strictly to the non-negotiable rules in `CLAUDE.md`:
   - **Rule 1 (TRUTHFULNESS):** Never fabricate or mock metrics, telemetry values, or results.
   - **Rule 2 (NO PLACEHOLDERS):** No `TODO` stubs or fake data in production code paths.
   - **Rule 5 (MEASURED):** Performance and accuracy claims must come solely from `make evaluate` (`docs/RESULTS.md`).
   - **Rule 6 (CONTRACTS FIRST):** Shared schemas live in `shared/` and are validated in Python, Go, and TypeScript.
3. **Testing:**
   - Ensure all unit, contract, integration, and e2e tests pass:
     ```bash
     make test
     make lint
     make evaluate-smoke
     ```
4. **Commits:** Write clear, conventional commit messages (`feat: ...`, `fix: ...`, `docs: ...`).
