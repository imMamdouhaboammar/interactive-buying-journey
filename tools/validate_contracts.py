#!/usr/bin/env python3
# Copyright (c) 2026 Mamdouh Aboammar
# SPDX-License-Identifier: PolyForm-Shield-1.0.0

"""
Offline validation tool for IBJ JSON contracts, schemas, and provenance.
Validates all canonical examples, negative fixtures, and cryptographic checksums.
"""

import copy
import hashlib
import json
import os
import re
import sys
from pathlib import Path

try:
    import jsonschema
    from jsonschema import Draft202012Validator
except ImportError:
    print("Error: jsonschema package is required. Install via pip install jsonschema")
    sys.exit(1)


REPO_ROOT = Path(__file__).resolve().parent.parent
SCHEMAS_DIR = REPO_ROOT / "contracts" / "schemas"
EXAMPLES_DIR = REPO_ROOT / "contracts" / "examples"
PROVENANCE_FILE = REPO_ROOT / "contracts" / "PROVENANCE.md"

SCHEMA_MAPPING = {
    "catalog-batch.json": "catalog-batch.schema.json",
    "compose-request.json": "compose-request.schema.json",
    "decision-envelope.json": "decision-envelope.schema.json",
    "enrichment-compatibility.json": "enrichment-record.schema.json",
    "enrichment-record.json": "enrichment-record.schema.json",
    "enrichment-review.json": "enrichment-review.schema.json",
    "event-envelope.json": "event-envelope.schema.json",
    "experience-plan.json": "experience-plan.schema.json",
}


def load_json(filepath: Path) -> dict:
    with open(filepath, "r", encoding="utf-8") as f:
        return json.load(f)


def check_provenance_hashes() -> bool:
    print("--> Checking provenance checksums in contracts/PROVENANCE.md...")
    if not PROVENANCE_FILE.exists():
        print(f"FAILED: Provenance file missing: {PROVENANCE_FILE}")
        return False

    with open(PROVENANCE_FILE, "r", encoding="utf-8") as f:
        content = f.read()

    # Match rows like `| `filename.json` | `sha256` |`
    pattern = re.compile(r"\|\s*`([^`]+)`\s*\|\s*`([a-f0-9]{64})`\s*\|")
    matches = pattern.findall(content)
    if not matches:
        print("FAILED: No file checksum entries found in PROVENANCE.md")
        return False

    all_matched = True
    checked_files = 0
    for filename, expected_hash in matches:
        # Check in schemas or examples
        candidate_paths = [
            SCHEMAS_DIR / filename,
            EXAMPLES_DIR / filename,
        ]
        target_path = next((p for p in candidate_paths if p.exists()), None)
        if not target_path:
            print(f"FAILED: Recorded file does not exist on disk: {filename}")
            all_matched = False
            continue

        with open(target_path, "rb") as f:
            actual_hash = hashlib.sha256(f.read()).hexdigest()

        if actual_hash != expected_hash:
            print(f"FAILED: Hash mismatch for {filename}!")
            print(f"  Expected: {expected_hash}")
            print(f"  Actual:   {actual_hash}")
            all_matched = False
        else:
            checked_files += 1

    print(f"PASSED: {checked_files} files verified against recorded SHA-256 hashes.")
    return all_matched


def validate_canonical_examples() -> bool:
    print("--> Validating canonical examples against schemas...")
    all_valid = True
    for example_name, schema_name in SCHEMA_MAPPING.items():
        example_path = EXAMPLES_DIR / example_name
        schema_path = SCHEMAS_DIR / schema_name

        if not example_path.exists():
            print(f"FAILED: Example not found: {example_path}")
            all_valid = False
            continue
        if not schema_path.exists():
            print(f"FAILED: Schema not found: {schema_path}")
            all_valid = False
            continue

        example_data = load_json(example_path)
        schema_data = load_json(schema_path)

        validator = Draft202012Validator(schema_data)
        errors = list(validator.iter_errors(example_data))
        if errors:
            print(f"FAILED: {example_name} failed schema {schema_name}:")
            for err in errors:
                print(f"  - {err.message} at path {'/'.join(str(p) for p in err.path)}")
            all_valid = False
        else:
            print(f"  VALID: {example_name} matches {schema_name}")

    return all_valid


def validate_negative_fixtures() -> bool:
    print("--> Validating negative test fixtures (must fail schema validation)...")
    plan_schema = load_json(SCHEMAS_DIR / "experience-plan.schema.json")
    plan_validator = Draft202012Validator(plan_schema)

    enrichment_schema = load_json(SCHEMAS_DIR / "enrichment-record.schema.json")
    enrichment_validator = Draft202012Validator(enrichment_schema)

    valid_plan = load_json(EXAMPLES_DIR / "experience-plan.json")
    valid_enrichment = load_json(EXAMPLES_DIR / "enrichment-record.json")

    # 1. Negative fixture: empty reason_codes
    neg_empty_reasons = copy.deepcopy(valid_plan)
    neg_empty_reasons["sections"][0]["reason_codes"] = []
    if plan_validator.is_valid(neg_empty_reasons):
        print("FAILED: Empty reason_codes unexpectedly passed validation!")
        return False
    print("  REJECTED as expected: empty reason_codes in experience-plan")

    # 2. Negative fixture: unknown section kind
    neg_unknown_kind = copy.deepcopy(valid_plan)
    neg_unknown_kind["sections"][0]["kind"] = "arbitrary-banner"
    if plan_validator.is_valid(neg_unknown_kind):
        print("FAILED: Unknown section kind unexpectedly passed validation!")
        return False
    print("  REJECTED as expected: unknown section kind in experience-plan")

    # 3. Negative fixture: facet-panel carrying items
    neg_facet_items = copy.deepcopy(valid_plan)
    facet_section = {
        "section_id": "sec_facets_01",
        "kind": "facet-panel",
        "slot_id": "collection_facets",
        "priority": 10,
        "items": [{"variant_id": "lap_001"}],  # MUST be empty for facet-panel!
        "reason_codes": ["facet_variance"],
        "config": {
            "facet_keys": ["brand", "weight_g"],
            "preserve_applied_facets": True,
        },
    }
    neg_facet_items["sections"] = [facet_section]
    if plan_validator.is_valid(neg_facet_items):
        print("FAILED: facet-panel carrying items unexpectedly passed validation!")
        return False
    print("  REJECTED as expected: facet-panel carrying items in experience-plan")

    # 4. Negative fixture: enrichment status: approved with a pending review
    neg_enrichment_unreviewed = copy.deepcopy(valid_enrichment)
    neg_enrichment_unreviewed["status"] = "approved"
    neg_enrichment_unreviewed["review"] = {
        "decision": "pending",
        "reviewer_id": None,
        "reviewed_at": None,
        "note": "Awaiting merchant approval",
    }
    if enrichment_validator.is_valid(neg_enrichment_unreviewed):
        print("FAILED: Approved enrichment with pending review unexpectedly passed validation!")
        return False
    print("  REJECTED as expected: approved enrichment with pending review")

    return True


def main():
    success = True
    if not check_provenance_hashes():
        success = False
    if not validate_canonical_examples():
        success = False
    if not validate_negative_fixtures():
        success = False

    if success:
        print("\nSUCCESS: All contract schemas, examples, and provenance checks passed!")
        sys.exit(0)
    else:
        print("\nFAILURE: Contract validation checks failed.")
        sys.exit(1)


if __name__ == "__main__":
    main()
