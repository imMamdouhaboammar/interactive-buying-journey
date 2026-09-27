#!/usr/bin/env python3
"""Document Lifecycle Validator

Validates YAML frontmatter, uniqueness of doc IDs, archive locations,
warning banners, cross-document links, index synchronization, and visibility
contracts across a repository.
"""

import argparse
import os
import re
import sys
from datetime import date, datetime
from pathlib import Path

ARCHIVE_BANNER_SNIPPET = "> **ARCHIVED, STALE: kept for the record only. Do not implement from this file. Current source:"
SUPERSEDED_BANNER_SNIPPET = "> **SUPERSEDED: kept for the record only. Do not implement from this file. Current source:"


def parse_frontmatter(file_path: Path):
    try:
        content = file_path.read_text(encoding="utf-8")
    except Exception as e:
        return None, f"Could not read file: {e}", ""

    if not content.startswith("---"):
        return None, "File does not start with YAML frontmatter delimiters '---'", content

    parts = content.split("---", 2)
    if len(parts) < 3:
        return None, "Malformed frontmatter: closing '---' delimiter not found", content

    fm_raw = parts[1]
    body = parts[2]
    meta = {}
    for line in fm_raw.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if ":" in line:
            k, v = line.split(":", 1)
            key = k.strip()
            val = v.split("#")[0].strip()
            if (val.startswith('"') and val.endswith('"')) or (val.startswith("'") and val.endswith("'")):
                val = val[1:-1]
            if val.lower() == "null":
                val = None
            meta[key] = val

    return meta, None, body


def extract_markdown_links(content: str):
    # Matches [text](link)
    pattern = r'\[([^\]]+)\]\(([^)]+)\)'
    return re.findall(pattern, content)


def validate_repository(repo_root: Path, expected_visibility: str = None, check_index: bool = True) -> tuple[list[str], list[str]]:
    repo_root = repo_root.resolve()
    errors = []
    warnings = []

    doc_ids: dict[str, str] = {}
    exclude_dirs = {".git", ".github", "node_modules", "_backups", "_quarantine"}
    exempt_files = {"LICENSE.md", "README-WORKSPACE.md", "QUARANTINE-README.md", "contracts/PROVENANCE.md", "PROVENANCE.md"}

    markdown_files: list[Path] = []
    for root, dirs, files in os.walk(repo_root):
        dirs[:] = [d for d in dirs if d not in exclude_dirs]
        for f in files:
            if f.endswith(".md") or f.endswith(".markdown"):
                if f in exempt_files:
                    continue
                markdown_files.append(Path(root) / f)

    today = date.today().isoformat()

    for file_path in markdown_files:
        rel_posix = file_path.relative_to(repo_root).as_posix()
        if rel_posix == "DOCS-INDEX.md":
            continue

        meta, err, body = parse_frontmatter(file_path)
        if err:
            errors.append(f"[{rel_posix}] Frontmatter error: {err}")
            continue

        # Check required fields
        required_keys = ["doc_id", "title", "lifecycle", "status", "visibility", "owner", "review_by"]
        for rk in required_keys:
            if rk not in meta or meta[rk] is None or meta[rk] == "":
                errors.append(f"[{rel_posix}] Missing required frontmatter field: '{rk}'")

        doc_id = meta.get("doc_id")
        if doc_id:
            if doc_id in doc_ids:
                errors.append(f"[{rel_posix}] Duplicate doc_id '{doc_id}' already defined in '{doc_ids[doc_id]}'")
            else:
                doc_ids[doc_id] = rel_posix

        # Check visibility
        vis = meta.get("visibility")
        if expected_visibility and vis and vis != expected_visibility:
            errors.append(f"[{rel_posix}] Visibility mismatch: expected '{expected_visibility}', got '{vis}'")

        # Check lifecycle enum
        lifecycle = meta.get("lifecycle")
        if lifecycle not in ["durable", "transient"]:
            errors.append(f"[{rel_posix}] Invalid lifecycle '{lifecycle}': must be 'durable' or 'transient'")

        # Check status enum
        status = meta.get("status")
        if status not in ["active", "superseded", "archived"]:
            errors.append(f"[{rel_posix}] Invalid status '{status}': must be 'active', 'superseded', or 'archived'")

        # Status: archived checks
        is_in_archive_dir = rel_posix.startswith("archive/")
        if status == "archived":
            if not is_in_archive_dir:
                errors.append(f"[{rel_posix}] Document with status 'archived' must reside in archive/ folder")
            if ARCHIVE_BANNER_SNIPPET not in body:
                errors.append(f"[{rel_posix}] Archived document missing standard archive warning banner directly below frontmatter")
            if not meta.get("archived_on"):
                errors.append(f"[{rel_posix}] Archived document missing 'archived_on' date")
            if not meta.get("archive_reason"):
                errors.append(f"[{rel_posix}] Archived document missing 'archive_reason'")
        elif is_in_archive_dir:
            errors.append(f"[{rel_posix}] Document inside archive/ folder must have status 'archived' (got '{status}')")

        # Status: superseded checks
        if status == "superseded":
            superseded_by = meta.get("superseded_by")
            if not superseded_by:
                errors.append(f"[{rel_posix}] Superseded document missing 'superseded_by' path")
            else:
                target_path = repo_root / superseded_by
                if not target_path.exists():
                    errors.append(f"[{rel_posix}] 'superseded_by' target does not exist: {superseded_by}")
            if SUPERSEDED_BANNER_SNIPPET not in body:
                errors.append(f"[{rel_posix}] Superseded document missing standard superseded warning banner")

        # Check review_by date
        review_by = meta.get("review_by")
        if review_by and review_by < today and status == "active":
            warnings.append(f"[{rel_posix}] Review date '{review_by}' is in the past for active document")

        # Check inbound links into archive from active docs
        if status != "archived":
            # Check if links target archive/
            links = extract_markdown_links(body)
            # Find history sections
            history_blocks = re.findall(r'##\s+(?:History|Changelog)[\s\S]*?(?=\n##|\Z)', body, re.IGNORECASE)
            allowed_archive_links = []
            for hb in history_blocks:
                allowed_archive_links.extend([l[1] for l in extract_markdown_links(hb)])

            for text, link in links:
                clean_link = link.split("#")[0].strip()
                if "archive/" in clean_link:
                    if link not in allowed_archive_links:
                        errors.append(f"[{rel_posix}] Active document illegally links to archive file '{link}' outside a '## History' section")

    # Check index freshness if requested
    if check_index:
        index_file = repo_root / "DOCS-INDEX.md"
        if not index_file.exists():
            errors.append("DOCS-INDEX.md does not exist at repository root.")
        else:
            from generate_docs_index import generate_index
            fresh_content = generate_index(repo_root, None)
            existing_content = index_file.read_text(encoding="utf-8")
            if fresh_content.strip() != existing_content.strip():
                errors.append("DOCS-INDEX.md is stale or differs from disk state. Run 'python3 scripts/generate_docs_index.py' to regenerate.")

    return errors, warnings


def main():
    parser = argparse.ArgumentParser(description="Validate repository document lifecycles and schemas.")
    parser.add_argument("--repo", default=".", help="Repository root path")
    parser.add_argument("--expected-visibility", choices=["public", "private"], help="Enforce expected visibility")
    parser.add_argument("--no-index-check", action="store_true", help="Skip DOCS-INDEX.md freshness check")

    args = parser.parse_args()
    repo_path = Path(args.repo)

    errors, warnings = validate_repository(
        repo_path,
        expected_visibility=args.expected_visibility,
        check_index=not args.no_index_check
    )

    if warnings:
        print("\n⚠️  Warnings:")
        for w in warnings:
            print(f"  {w}")

    if errors:
        print(f"\n❌ Validation Failed ({len(errors)} error(s)):", file=sys.stderr)
        for e in errors:
            print(f"  {e}", file=sys.stderr)
        sys.exit(1)

    print(f"\n✅ All document lifecycles and schemas verified cleanly ({repo_path}).")
    sys.exit(0)


if __name__ == "__main__":
    main()
