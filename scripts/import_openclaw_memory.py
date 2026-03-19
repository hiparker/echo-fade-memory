#!/usr/bin/env python3
"""Import local OpenClaw memory sources into echo-fade-memory HTTP API.

Supported sources:
- Markdown memory files (MEMORY.md and workspace daily logs)
- SQLite databases with a `memories` table
- LanceDB tables (best effort via SDK, with JSON fallback)
"""

from __future__ import annotations

import argparse
import json
import re
import sqlite3
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


DEFAULT_BASE_URL = "http://127.0.0.1:8080"
DEFAULT_MEMORY_MD = Path("~/.openclaw/workspace/MEMORY.md").expanduser()
DEFAULT_DAILY_DIR = Path("~/.openclaw/workspace/memory").expanduser()
DEFAULT_OPENCLAW_MEMORY_DIR = Path("~/.openclaw/memory").expanduser()


@dataclass
class MemoryItem:
    content: str
    summary: str
    memory_type: str
    importance: float
    source_ref: dict


def _clean_text(text: str) -> str:
    text = re.sub(r"\*\*(.*?)\*\*", r"\1", text)
    text = re.sub(r"`([^`]*)`", r"\1", text)
    text = re.sub(r"\[(.*?)\]\((.*?)\)", r"\1 (\2)", text)
    return re.sub(r"\s+", " ", text).strip()


def _summary(text: str, max_len: int = 120) -> str:
    text = _clean_text(text)
    if len(text) <= max_len:
        return text
    return text[: max_len - 3].rstrip() + "..."


def parse_markdown_file(path: Path, memory_type: str, importance: float) -> list[MemoryItem]:
    if not path.exists():
        return []
    raw = path.read_text(encoding="utf-8", errors="ignore")
    lines = raw.splitlines()
    headings: list[str] = []
    out: list[MemoryItem] = []

    bullet_re = re.compile(r"^\s*-\s+(.*)$")
    checkbox_re = re.compile(r"^\s*-\s+\[[xX ]\]\s+(.*)$")
    ordered_re = re.compile(r"^\s*\d+\.\s+(.*)$")

    for line in lines:
        if line.startswith("#"):
            title = _clean_text(line.lstrip("#").strip())
            if title:
                level = len(line) - len(line.lstrip("#"))
                while len(headings) >= level:
                    headings.pop()
                headings.append(title)
            continue

        payload = None
        for regex in (checkbox_re, bullet_re, ordered_re):
            m = regex.match(line)
            if m:
                payload = _clean_text(m.group(1))
                break
        if not payload:
            continue

        context = " > ".join(headings[-3:]) if headings else path.stem
        content = f"{context}: {payload}" if context else payload
        if len(content) < 6:
            continue
        out.append(
            MemoryItem(
                content=content,
                summary=_summary(payload),
                memory_type=memory_type,
                importance=importance,
                source_ref={
                    "kind": "file",
                    "ref": str(path),
                    "title": path.name,
                    "snippet": payload[:200],
                },
            )
        )
    return out


def parse_sqlite_file(path: Path) -> list[MemoryItem]:
    out: list[MemoryItem] = []
    if not path.exists():
        return out
    try:
        conn = sqlite3.connect(str(path))
    except sqlite3.Error:
        return out

    try:
        cur = conn.cursor()
        cur.execute("SELECT name FROM sqlite_master WHERE type='table'")
        tables = {row[0] for row in cur.fetchall()}
        if "memories" not in tables:
            return out

        columns = set()
        cur.execute("PRAGMA table_info(memories)")
        for row in cur.fetchall():
            columns.add(row[1])
        if "content" not in columns:
            return out

        select_cols = ["content"]
        if "summary" in columns:
            select_cols.append("summary")
        if "memory_type" in columns:
            select_cols.append("memory_type")
        if "importance" in columns:
            select_cols.append("importance")

        cur.execute(f"SELECT {', '.join(select_cols)} FROM memories")
        for row in cur.fetchall():
            idx = 0
            content = _clean_text(str(row[idx] or ""))
            idx += 1
            if not content:
                continue
            summary = _summary(content)
            mtype = "long_term"
            importance = 0.7
            if "summary" in columns:
                summary = _summary(str(row[idx] or "")) or _summary(content)
                idx += 1
            if "memory_type" in columns:
                mtype = _clean_text(str(row[idx] or "")) or "long_term"
                idx += 1
            if "importance" in columns:
                try:
                    importance = float(row[idx])
                except (TypeError, ValueError):
                    importance = 0.7

            out.append(
                MemoryItem(
                    content=content,
                    summary=summary,
                    memory_type=mtype,
                    importance=max(0.05, min(1.0, importance)),
                    source_ref={
                        "kind": "sqlite",
                        "ref": str(path),
                        "title": path.name,
                        "snippet": content[:200],
                    },
                )
            )
    finally:
        conn.close()
    return out


def parse_lancedb_dir(path: Path) -> list[MemoryItem]:
    """Best-effort parser for LanceDB tables, with JSON sidecar fallback."""
    out: list[MemoryItem] = []
    if not path.exists():
        return out

    # Preferred path: use LanceDB SDK to read actual table rows.
    # Table path usually looks like: <db_uri>/<table_name>.lance
    try:
        import lancedb  # type: ignore

        db = lancedb.connect(str(path.parent))
        table = db.open_table(path.stem)
        arrow_tbl = table.to_arrow()
        records = arrow_tbl.to_pylist()
        for row in records:
            if not isinstance(row, dict):
                continue
            content = ""
            for key in ("content", "text", "memory", "raw", "value"):
                value = row.get(key)
                if isinstance(value, str) and value.strip():
                    content = _clean_text(value)
                    break
            if not content:
                continue
            out.append(
                MemoryItem(
                    content=content,
                    summary=_summary(str(row.get("summary") or content)),
                    memory_type=_clean_text(str(row.get("memory_type") or "long_term")),
                    importance=0.65,
                    source_ref={
                        "kind": "lancedb",
                        "ref": str(path),
                        "title": path.name,
                        "snippet": content[:200],
                    },
                )
            )
        if out:
            return out
    except Exception:
        # Fallback below for environments without lancedb SDK or partial table files.
        pass

    # Fallback path: parse any JSON sidecar-like files.
    for json_file in path.rglob("*.json"):
        try:
            payload = json.loads(json_file.read_text(encoding="utf-8", errors="ignore"))
        except Exception:
            continue

        rows: Iterable[dict] = []
        if isinstance(payload, list):
            rows = [x for x in payload if isinstance(x, dict)]
        elif isinstance(payload, dict):
            for k in ("rows", "data", "records", "items"):
                if isinstance(payload.get(k), list):
                    rows = [x for x in payload[k] if isinstance(x, dict)]
                    break
            else:
                rows = [payload]

        for row in rows:
            content = ""
            for key in ("content", "text", "memory", "raw", "value"):
                value = row.get(key)
                if isinstance(value, str) and value.strip():
                    content = _clean_text(value)
                    break
            if not content:
                continue

            out.append(
                MemoryItem(
                    content=content,
                    summary=_summary(str(row.get("summary") or content)),
                    memory_type=_clean_text(str(row.get("memory_type") or "long_term")),
                    importance=0.65,
                    source_ref={
                        "kind": "lancedb",
                        "ref": str(path),
                        "title": path.name,
                        "snippet": content[:200],
                    },
                )
            )
    return out


def post_memory(base_url: str, item: MemoryItem, timeout: float = 8.0) -> tuple[bool, str]:
    payload = {
        "content": item.content,
        "summary": item.summary,
        "memory_type": item.memory_type,
        "importance": item.importance,
        "source_refs": [item.source_ref],
    }
    req = urllib.request.Request(
        f"{base_url.rstrip('/')}/v1/memories",
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            data = resp.read().decode("utf-8", errors="ignore")
            if 200 <= resp.status < 300:
                try:
                    rid = json.loads(data).get("id", "")
                    return True, rid
                except Exception:
                    return True, ""
            return False, f"status={resp.status}"
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="ignore")
        return False, f"http {e.code}: {body[:200]}"
    except Exception as e:
        return False, str(e)


def dedupe(items: list[MemoryItem]) -> list[MemoryItem]:
    seen: set[str] = set()
    out: list[MemoryItem] = []
    for item in items:
        key = item.content.strip().lower()
        if not key or key in seen:
            continue
        seen.add(key)
        out.append(item)
    return out


def discover_sqlite_candidates(root_dirs: list[Path]) -> list[Path]:
    candidates: list[Path] = []
    for root in root_dirs:
        if not root.exists():
            continue
        for pattern in ("*.db", "*.sqlite", "*.sqlite3"):
            candidates.extend(root.rglob(pattern))
        # Also detect extension-less SQLite files by magic header.
        for file_path in root.rglob("*"):
            if not file_path.is_file():
                continue
            try:
                with file_path.open("rb") as fh:
                    header = fh.read(16)
                if header == b"SQLite format 3\x00":
                    candidates.append(file_path)
            except Exception:
                continue
    unique = []
    seen = set()
    for path in candidates:
        p = str(path.resolve())
        if p not in seen:
            seen.add(p)
            unique.append(path)
    return unique


def check_health(base_url: str) -> None:
    req = urllib.request.Request(f"{base_url.rstrip('/')}/v1/healthz", method="GET")
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            if resp.status != 200:
                raise RuntimeError(f"healthz status={resp.status}")
    except Exception as e:
        raise RuntimeError(f"service health check failed: {e}") from e


def main() -> int:
    parser = argparse.ArgumentParser(description="Import OpenClaw memory into local echo-fade-memory HTTP service.")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL, help="memory service base url")
    parser.add_argument("--memory-md", type=Path, default=DEFAULT_MEMORY_MD, help="path to MEMORY.md")
    parser.add_argument("--daily-dir", type=Path, default=DEFAULT_DAILY_DIR, help="path to workspace daily memory dir")
    parser.add_argument("--openclaw-memory-dir", type=Path, default=DEFAULT_OPENCLAW_MEMORY_DIR, help="path to openclaw memory dir")
    parser.add_argument("--sqlite", type=Path, action="append", default=[], help="extra sqlite file path (repeatable)")
    parser.add_argument("--dry-run", action="store_true", help="parse only, no POST")
    args = parser.parse_args()

    memory_md = args.memory_md.expanduser()
    daily_dir = args.daily_dir.expanduser()
    openclaw_memory_dir = args.openclaw_memory_dir.expanduser()
    sqlite_paths = [p.expanduser() for p in args.sqlite]

    items: list[MemoryItem] = []
    stats = {
        "markdown_main": 0,
        "markdown_daily": 0,
        "sqlite": 0,
        "lancedb": 0,
    }

    md_items = parse_markdown_file(memory_md, memory_type="long_term", importance=0.95)
    items.extend(md_items)
    stats["markdown_main"] = len(md_items)

    if daily_dir.exists():
        for path in sorted(daily_dir.glob("*.md")):
            if path.name.lower() == "readme.md":
                continue
            day_items = parse_markdown_file(path, memory_type="project", importance=0.70)
            items.extend(day_items)
            stats["markdown_daily"] += len(day_items)

    scan_roots = [openclaw_memory_dir, Path("~/.echo-fade-memory").expanduser()]
    for sqlite_file in discover_sqlite_candidates(scan_roots):
        sqlite_paths.append(sqlite_file)

    unique_sqlite = []
    seen_sqlite = set()
    for p in sqlite_paths:
        key = str(p.resolve()) if p.exists() else str(p)
        if key not in seen_sqlite:
            seen_sqlite.add(key)
            unique_sqlite.append(p)

    for sqlite_path in unique_sqlite:
        sqlite_items = parse_sqlite_file(sqlite_path)
        items.extend(sqlite_items)
        stats["sqlite"] += len(sqlite_items)

    lance_dirs = list(openclaw_memory_dir.rglob("*.lance"))
    for lance_dir in lance_dirs:
        lance_items = parse_lancedb_dir(lance_dir)
        items.extend(lance_items)
        stats["lancedb"] += len(lance_items)

    items = dedupe(items)
    print(
        json.dumps(
            {
                "sources": stats,
                "parsed_total_after_dedupe": len(items),
                "dry_run": args.dry_run,
            },
            ensure_ascii=False,
            indent=2,
        )
    )

    if args.dry_run:
        return 0

    check_health(args.base_url)

    ok = 0
    fail = 0
    failures: list[str] = []
    for item in items:
        success, msg = post_memory(args.base_url, item)
        if success:
            ok += 1
        else:
            fail += 1
            failures.append(msg)

    result = {
        "imported_ok": ok,
        "failed": fail,
        "sample_failures": failures[:5],
        "base_url": args.base_url,
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if fail == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
