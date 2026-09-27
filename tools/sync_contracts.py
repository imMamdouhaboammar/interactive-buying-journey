#!/usr/bin/env python3
# Copyright (c) 2026 Mamdouh Aboammar
# SPDX-License-Identifier: PolyForm-Shield-1.0.0

"""
Contract Synchronization Utility for IBJ.

Unidirectionally synchronizes contract schemas and examples from the private
specification repository to the public code repository under strict isolation
rules, clean working tree validation, and SHA-256 provenance registration.
"""

import argparse
import hashlib
import os
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


def calculate_sha256(filepath: Path) -> str:
    hasher = hashlib.sha256()
    with open(filepath, "rb") as f:
        while chunk := f.read(65536):
            hasher.update(chunk)
    return hasher.hexdigest()


def check_git_clean(repo_path: Path) -> bool:
    res = subprocess.run(
        ["git", "status", "--porcelain"],
        cwd=repo_path,
        capture_output=True,
        text=True,
    )
    return len(res.stdout.strip()) == 0


def verify_commit_exists(repo_path: Path, commit_sha: str) -> bool:
    res = subprocess.run(
        ["git", "cat-file", "-e", f"{commit_sha}^{{commit}}"],
        cwd=repo_path,
        capture_output=True,
    )
    return res.returncode == 0


def sync_contracts(spec_dir: Path, code_dir: Path, commit_sha: str) -> bool:
    spec_dir = spec_dir.resolve()
    code_dir = code_dir.resolve()

    if not spec_dir.exists():
        print(f"❌ Error: Spec directory does not exist: {spec_dir}", file=sys.stderr)
        return False

    # 1. Clean tree verification
    if not check_git_clean(spec_dir):
        print(f"❌ Error: Spec repository working tree at {spec_dir} is dirty.", file=sys.stderr)
        print("   All changes in spec/ must be committed before synchronizing contracts.", file=sys.stderr)
        return False

    # 2. Commit hash verification
    if not verify_commit_exists(spec_dir, commit_sha):
        print(f"❌ Error: Commit {commit_sha} does not exist in spec repository.", file=sys.stderr)
        return False

    spec_contracts = spec_dir / "contracts"
    if not spec_contracts.exists():
        print(f"❌ Error: No contracts folder found at {spec_contracts}", file=sys.stderr)
        return False

    dest_contracts = code_dir / "contracts"
    dest_schemas = dest_contracts / "schemas"
    dest_examples = dest_contracts / "examples"

    dest_schemas.mkdir(parents=True, exist_ok=True)
    dest_examples.mkdir(parents=True, exist_ok=True)

    synced_schemas = []
    synced_examples = []

    # 3. Strict Whitelist Enforcement: only contracts/schemas/*.json and contracts/examples/*.json
    # First, scan spec contracts folder to reject any disallowed files
    for root, _, files in os.walk(spec_contracts):
        rel_root = Path(root).relative_to(spec_contracts)
        for f in files:
            file_rel = rel_root / f
            file_str = file_rel.as_posix()
            is_schema = file_str.startswith("schemas/") and f.endswith(".json")
            is_example = file_str.startswith("examples/") and f.endswith(".json")
            is_meta = f in ["README.md", "SCHEMA-EVOLUTION.md", "PROVENANCE.md", "openapi.yaml"]

            if not (is_schema or is_example or is_meta):
                print(f"❌ Error: Forbidden/non-contract file in spec contracts: contracts/{file_str}", file=sys.stderr)
                return False

            if is_schema:
                src_file = Path(root) / f
                dst_file = dest_contracts / file_rel
                dst_file.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(src_file, dst_file)
                sha = calculate_sha256(dst_file)
                synced_schemas.append((f, sha))

            elif is_example:
                src_file = Path(root) / f
                dst_file = dest_contracts / file_rel
                dst_file.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(src_file, dst_file)
                sha = calculate_sha256(dst_file)
                synced_examples.append((f, sha))

    # 4. Rewrite contracts/PROVENANCE.md
    provenance_path = dest_contracts / "PROVENANCE.md"
    now_iso = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")

    provenance_lines = [
        "# Contracts Provenance",
        "",
        "The JSON Schemas and example payloads in this directory are vendored directly",
        "from the private IBJ specification repository under owner decision D-27.",
        "",
        "## Upstream Provenance",
        "",
        "- **Source Repository:** `imMamdouhaboammar/interactive-buying-journey-spec` (private)",
        f"- **Source Commit:** `{commit_sha}`",
        f"- **Sync Timestamp:** `{now_iso}`",
        "- **Vendor Policy:** Only `contracts/schemas/*.json` and `contracts/examples/*.json` are mirrored into this repository. No internal spec prose, research notes, private decision registers, or business plans are included.",
        "",
        "## Cryptographic Checksums (SHA-256)",
        "",
        "### Schemas (`contracts/schemas/`)",
        "",
        "| File | SHA-256 Checksum |",
        "| --- | --- |",
    ]

    for fname, sha in sorted(synced_schemas):
        provenance_lines.append(f"| `{fname}` | `{sha}` |")

    provenance_lines.extend([
        "",
        "### Examples (`contracts/examples/`)",
        "",
        "| File | SHA-256 Checksum |",
        "| --- | --- |",
    ])

    for fname, sha in sorted(synced_examples):
        provenance_lines.append(f"| `{fname}` | `{sha}` |")

    provenance_lines.extend([
        "",
        "## Integrity Verification",
        "",
        "CI runs `python tools/validate_contracts.py --check-provenance`, which re-calculates the SHA-256 hash of each file on disk and fails if any hash diverges from this file.",
        "",
    ])

    provenance_path.write_text("\n".join(provenance_lines), encoding="utf-8")
    print(f"✅ Synchronized {len(synced_schemas)} schemas and {len(synced_examples)} examples.")
    print(f"✅ Updated {provenance_path}")

    # 5. Run tools/validate_contracts.py --check-provenance
    validate_tool = code_dir / "tools" / "validate_contracts.py"
    if validate_tool.exists():
        print(f"--> Running validation: {validate_tool} --check-provenance")
        res = subprocess.run([sys.executable, str(validate_tool), "--check-provenance"], cwd=code_dir)
        if res.returncode != 0:
            print("❌ Error: validate_contracts.py failed provenance check after sync.", file=sys.stderr)
            return False

    return True


def main():
    parser = argparse.ArgumentParser(description="Synchronize contracts with provenance verification.")
    parser.add_argument("--spec", help="Path to specification repository")
    parser.add_argument("--code", default=".", help="Path to code repository (default: current directory)")
    parser.add_argument("--commit", help="Commit SHA from spec repository to pin")

    args = parser.parse_args()

    if not args.spec or not args.commit:
        parser.error("--spec and --commit are required for synchronization.")

    code_path = Path(args.code)
    success = sync_contracts(Path(args.spec), code_path, args.commit)
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
