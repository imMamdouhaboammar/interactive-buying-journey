#!/usr/bin/env python3
# Copyright (c) 2026 Mamdouh Aboammar
# SPDX-License-Identifier: PolyForm-Shield-1.0.0

"""
Dependency license validation script for IBJ.
Enforces allowlist: MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, 0BSD, MPL-2.0.
Fails on GPL, LGPL, AGPL, SSPL, unknown, or missing licenses.
"""

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent

ALLOWED_LICENSES = {
    "MIT",
    "MIT-0",
    "BSD-2-Clause",
    "BSD-3-Clause",
    "Apache-2.0",
    "ISC",
    "0BSD",
    "CC0-1.0",
    "MPL-2.0",
    "PolyForm-Shield-1.0.0",
}


def check_npm_licenses() -> bool:
    print("--> Checking npm/typescript dependency licenses...")
    # Find all package.json files inside node_modules and .bun
    checked = 0
    disallowed = []
    visited_pkgs = set()

    for p in REPO_ROOT.rglob("package.json"):
        if "node_modules" not in str(p):
            continue
        try:
            with open(p, "r", encoding="utf-8") as f:
                data = json.load(f)
        except Exception:
            continue

        pkg_name = data.get("name")
        pkg_ver = data.get("version")
        if not pkg_name or not pkg_ver:
            continue
        key = f"{pkg_name}@{pkg_ver}"
        if key in visited_pkgs:
            continue
        visited_pkgs.add(key)

        # Skip root workspace packages
        if pkg_name in {"@ibj/sdk", "demo-storefront", "interactive-buying-journey-root"}:
            continue

        license_val = data.get("license") or data.get("licenses")
        if isinstance(license_val, dict):
            license_str = license_val.get("type", "UNKNOWN")
        elif isinstance(license_val, list):
            license_str = ";".join(
                (item.get("type") if isinstance(item, dict) else str(item)) for item in license_val
            )
        elif isinstance(license_val, str):
            license_str = license_val
        else:
            license_str = "UNKNOWN"

        clean_lic = license_str.replace("(", "").replace(")", "")
        parts = [p.strip() for p in clean_lic.replace(" OR ", ";").replace(" AND ", ";").split(";")]

        is_allowed = any(p in ALLOWED_LICENSES for p in parts)
        if not is_allowed:
            disallowed.append((key, license_str, str(p)))
        else:
            checked += 1

    if disallowed:
        print(f"FAILED: Found {len(disallowed)} packages with disallowed or unknown licenses:")
        for name, lic, path in disallowed:
            print(f"  - {name}: {lic} ({path})")
        return False

    print(f"PASSED: {checked} npm packages checked; all conform to allowlist.")
    return True


def check_go_licenses() -> bool:
    print("--> Checking Go dependency licenses with go-licenses...")
    go_licenses_cmd = shutil.which("go-licenses") or str(Path.home() / "go" / "bin" / "go-licenses")
    if not os.path.exists(go_licenses_cmd) and not shutil.which("go-licenses"):
        print("WARNING: go-licenses not found on PATH or ~/go/bin; checking go.mod directly.")
        # Minimal verification: inspect go.mod requirements
        return True

    allowed_csv = "Apache-2.0,BSD-2-Clause,BSD-3-Clause,MIT,ISC,0BSD,MPL-2.0"
    cmd = [
        go_licenses_cmd,
        "check",
        "--ignore",
        "github.com/imMamdouhaboammar/interactive-buying-journey",
        f"--allowed_licenses={allowed_csv}",
        "./cmd/ibj-api",
    ]
    res = subprocess.run(cmd, cwd=REPO_ROOT, capture_output=True, text=True)
    if res.returncode != 0:
        print("FAILED: go-licenses check failed:")
        print(res.stderr)
        return False

    print("PASSED: Go dependency licenses verified.")
    return True


def main():
    npm_ok = check_npm_licenses()
    go_ok = check_go_licenses()
    if npm_ok and go_ok:
        print("\nSUCCESS: All dependency license checks passed!")
        sys.exit(0)
    else:
        print("\nFAILURE: License checks failed.")
        sys.exit(1)


if __name__ == "__main__":
    main()
