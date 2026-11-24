#!/usr/bin/env python3
"""
CI Check: Ensure there are no hardcoded fake metric numbers in dashboard UI components.
Inspects TSX files for suspicious patterns like hardcoded latency, throughput, accuracy,
or fake status metrics in presentation code.
"""

import sys
import re
from pathlib import Path

DASHBOARD_SRC = Path(__file__).resolve().parent.parent / "dashboard" / "src"

SUSPICIOUS_PATTERNS = [
    # Hardcoded fake metric strings like "98.5% accuracy" or "latency: 42ms" outside tests
    re.compile(r'(?:latency|throughput|accuracy|mAP)\s*[:=]\s*["\']?[0-9]+', re.IGNORECASE),
]

def main() -> int:
    failed = False
    for tsx_file in DASHBOARD_SRC.rglob("*.tsx"):
        if "__tests__" in str(tsx_file) or "e2e" in str(tsx_file):
            continue
        content = tsx_file.read_text(encoding="utf-8")
        for line_num, line in enumerate(content.splitlines(), start=1):
            for pattern in SUSPICIOUS_PATTERNS:
                if pattern.search(line):
                    print(f"FAILED: Hardcoded fake metric number found in {tsx_file.relative_to(DASHBOARD_SRC.parent)}:{line_num}")
                    print(f"  Line: {line.strip()}")
                    failed = True

    if failed:
        print("\nCI Check FAILED: Hardcoded fake numbers detected in UI components.")
        return 1

    print("CI Check PASSED: Zero hardcoded fake metric numbers found in dashboard UI.")
    return 0

if __name__ == "__main__":
    sys.exit(main())
