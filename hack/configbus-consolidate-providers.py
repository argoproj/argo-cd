#!/usr/bin/env python3
"""Consolidate split configbus provider implementations into one file each.

Merges util/configbus/{legacy,crd,hybrid}_provider_*.go into the matching
*_provider.go (deduping types/methods already present in the core), then
deletes the split files. Idempotent when already consolidated.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

# Prefer cwd (stack walks run from the repo root); fall back to script location.
REPO = Path.cwd()
if not (REPO / "util" / "configbus").is_dir():
	REPO = Path(__file__).resolve().parents[1]
CFG = REPO / "util" / "configbus"
DOC = REPO / "docs" / "developer-guide" / "architecture" / "configbus.md"

LEGACY_SECTION_TITLES = {
	"shared": "Shared",
	"applicationset": "ApplicationSet",
	"commitserver": "Commit server",
	"controller": "Controller",
	"env": "Env",
	"notifications": "Notifications",
	"reposerver": "Repo server",
	"server": "Server",
	"settings": "SettingsManager",
}

LEGACY_ORDER = [
	"shared",
	"applicationset",
	"commitserver",
	"controller",
	"env",
	"notifications",
	"reposerver",
	"server",
	"settings",
]

DECL_START = re.compile(
	r"^(?P<kind>type|func|var|const)\s+",
)


def strip_package_and_imports(text: str) -> tuple[list[str], str]:
	"""Return (imports, body_without_package_or_imports)."""
	lines = text.splitlines()
	i = 0
	while i < len(lines) and not lines[i].startswith("package "):
		i += 1
	i += 1
	while i < len(lines) and lines[i].strip() == "":
		i += 1
	imports: list[str] = []
	if i < len(lines) and lines[i].startswith("import "):
		if lines[i].startswith("import ("):
			i += 1
			while i < len(lines) and lines[i].strip() != ")":
				imp = lines[i].strip()
				if imp:
					imports.append(imp)
				i += 1
			i += 1
		else:
			imports.append(lines[i][len("import ") :].strip())
			i += 1
	while i < len(lines) and lines[i].strip() == "":
		i += 1
	return imports, "\n".join(lines[i:]).rstrip() + "\n"


def format_imports(imports: list[str]) -> str:
	ordered: list[str] = []
	seen: set[str] = set()
	for imp in imports:
		if imp and imp not in seen:
			seen.add(imp)
			ordered.append(imp)
	std, k8s, gh = [], [], []
	for imp in ordered:
		if imp.startswith('"github.com'):
			gh.append(imp)
		elif imp.startswith('"k8s.io'):
			k8s.append(imp)
		else:
			std.append(imp)
	parts = ["import ("]
	for group in (std, k8s, gh):
		if not group:
			continue
		if len(parts) > 1:
			parts.append("")
		for imp in group:
			parts.append(f"\t{imp}")
	parts.append(")")
	return "\n".join(parts) + "\n"


def decl_key(block: str) -> str | None:
	"""Stable key for a top-level Go decl, or None for comment-only preamble."""
	# Skip leading blank/comment lines for the signature
	sig_lines = []
	for line in block.splitlines():
		if not sig_lines and (line.strip() == "" or line.strip().startswith("//")):
			continue
		sig_lines.append(line)
		if len(sig_lines) >= 3:
			break
	if not sig_lines:
		return None
	sig = " ".join(s.strip() for s in sig_lines)
	# func (recv T) Name
	m = re.match(r"func\s+\(([^)]+)\)\s+([A-Za-z0-9_]+)", sig)
	if m:
		recv = m.group(1).split()[-1].lstrip("*")
		return f"method:{recv}.{m.group(2)}"
	m = re.match(r"func\s+([A-Za-z0-9_]+)", sig)
	if m:
		return f"func:{m.group(1)}"
	m = re.match(r"type\s+([A-Za-z0-9_]+)", sig)
	if m:
		return f"type:{m.group(1)}"
	m = re.match(r"var\s+\(\s*$", sig)
	if m:
		# var ( ... ) block — use first identifier inside
		inner = re.search(r"\n\s*([A-Za-z0-9_]+)\s+", block)
		if inner:
			return f"var:{inner.group(1)}"
		return f"varblock:{hash(block)}"
	m = re.match(r"var\s+([A-Za-z0-9_]+)", sig)
	if m:
		return f"var:{m.group(1)}"
	m = re.match(r"const\s+([A-Za-z0-9_]+)", sig)
	if m:
		return f"const:{m.group(1)}"
	# Ensure / comment docs attached to next decl handled by grouping
	return f"other:{hash(sig)}"


def split_decls(body: str) -> list[str]:
	"""Split body into top-level declaration blocks (comments attach upward)."""
	lines = body.splitlines()
	blocks: list[str] = []
	buf: list[str] = []
	brace = 0
	in_decl = False

	def flush() -> None:
		nonlocal buf
		text = "\n".join(buf).rstrip()
		if text.strip():
			blocks.append(text + "\n")
		buf = []

	i = 0
	while i < len(lines):
		line = lines[i]
		stripped = line.strip()

		# Start of a new top-level decl when brace==0 and line matches
		if brace == 0 and not in_decl:
			# Accumulate comments/blanks into buf until a decl starts
			if DECL_START.match(stripped) or stripped.startswith("var _"):
				in_decl = True
				buf.append(line)
				brace += line.count("{") - line.count("}")
				# single-line decl with no braces
				if brace == 0 and not stripped.endswith("("):
					# could be `var _ Provider = ...` or single-line func — flush
					# multi-line signatures end with `{` or continue
					if "{" in line or stripped.startswith("var _") or (
						stripped.startswith("func ") and "{" in line
					):
						flush()
						in_decl = False
					elif stripped.startswith("type ") and not stripped.endswith("{") and "{" not in line:
						# type Alias T
						flush()
						in_decl = False
				elif brace == 0 and stripped.endswith("("):
					# var (  or const (  — keep reading until matching )
					pass
				i += 1
				continue
			else:
				buf.append(line)
				i += 1
				continue

		if in_decl:
			buf.append(line)
			# Track braces; also var ( ) groups without braces for types inside
			brace += line.count("{") - line.count("}")
			# Handle import-like paren groups for var/const (
			if brace <= 0:
				# For `var (` blocks, brace stays 0; detect closing `)`
				joined = "\n".join(buf)
				if re.search(r"^(type|func|var|const)\b", next((l.strip() for l in buf if l.strip() and not l.strip().startswith("//")), ""), re.M):
					first = next((l.strip() for l in buf if l.strip() and not l.strip().startswith("//")), "")
					if first.startswith("var (") or first.startswith("const ("):
						if stripped == ")":
							flush()
							in_decl = False
							brace = 0
					elif brace <= 0:
						flush()
						in_decl = False
						brace = 0
			i += 1
			continue

		i += 1

	if buf:
		flush()
	return blocks


def split_decls_simple(body: str) -> list[str]:
	"""Brace/paren-aware split of top-level Go declarations."""
	lines = body.splitlines()
	blocks: list[str] = []
	buf: list[str] = []
	depth_brace = 0
	depth_paren = 0
	started = False

	def is_start(stripped: str) -> bool:
		return bool(DECL_START.match(stripped)) or stripped.startswith("var _")

	for line in lines:
		stripped = line.strip()
		if not started:
			if is_start(stripped):
				started = True
				buf.append(line)
				depth_brace += line.count("{") - line.count("}")
				depth_paren += line.count("(") - line.count(")")
				# Finish immediately if fully balanced and not opening a multi-line group awkwardly
				if depth_brace == 0 and depth_paren == 0:
					blocks.append("\n".join(buf).rstrip() + "\n")
					buf = []
					started = False
			else:
				# leading comments belong to next decl
				buf.append(line)
			continue

		buf.append(line)
		depth_brace += line.count("{") - line.count("}")
		depth_paren += line.count("(") - line.count(")")
		if depth_brace <= 0 and depth_paren <= 0:
			blocks.append("\n".join(buf).rstrip() + "\n")
			buf = []
			started = False
			depth_brace = 0
			depth_paren = 0

	if buf:
		blocks.append("\n".join(buf).rstrip() + "\n")
	return blocks


def satellites(prefix: str) -> list[Path]:
	return sorted(CFG.glob(f"{prefix}_provider_*.go"))


def consolidate_prefix(prefix: str) -> bool:
	core = CFG / f"{prefix}_provider.go"
	sats = satellites(prefix)
	if not core.exists() or not sats:
		return False

	all_imports: list[str] = []
	core_imports, core_body = strip_package_and_imports(core.read_text())
	all_imports.extend(core_imports)

	core_blocks = split_decls_simple(core_body)
	seen: set[str] = set()
	kept: list[str] = []
	for block in core_blocks:
		key = decl_key(block)
		if key is None:
			kept.append(block)
			continue
		if key in seen:
			continue
		seen.add(key)
		kept.append(block)

	# Order satellites
	if prefix == "legacy":
		by_suffix = {p.name[len(f"{prefix}_provider_") : -3]: p for p in sats}
		ordered = [by_suffix[s] for s in LEGACY_ORDER if s in by_suffix]
		ordered += [by_suffix[s] for s in sorted(by_suffix) if s not in LEGACY_ORDER]
	else:
		ordered = sats

	new_sections: list[tuple[str, list[str]]] = []
	for p in ordered:
		suffix = p.name[len(f"{prefix}_provider_") : -3]
		imps, body = strip_package_and_imports(p.read_text())
		all_imports.extend(imps)
		added: list[str] = []
		for block in split_decls_simple(body):
			key = decl_key(block)
			if key is None:
				continue
			if key in seen:
				continue
			seen.add(key)
			added.append(block)
		if added:
			title = LEGACY_SECTION_TITLES.get(suffix, suffix) if prefix == "legacy" else suffix
			new_sections.append((title, added))

	# If nothing new from satellites, still delete them (pure duplicates).
	out = "package configbus\n\n"
	out += format_imports(all_imports)
	out += "\n"
	for block in kept:
		out += block.rstrip() + "\n\n"
	for title, blocks in new_sections:
		out += "// ---------------------------------------------------------------------------\n"
		out += f"// {title}\n"
		out += "// ---------------------------------------------------------------------------\n\n"
		for block in blocks:
			out += block.rstrip() + "\n\n"

	core.write_text(out.rstrip() + "\n")
	for p in sats:
		p.unlink()
	return True


def update_docs() -> bool:
	if not DOC.exists():
		return False
	text = DOC.read_text()
	orig = text
	reps = [
		(
			"| `LegacyProvider` | `util/configbus/legacy_provider*.go` |",
			"| `LegacyProvider` | `util/configbus/legacy_provider.go` |",
		),
		(
			"| `LegacyValues` / `*Legacy` | `util/configbus/legacy_provider*.go` |",
			"| `LegacyValues` / `*Legacy` | `util/configbus/legacy_provider.go` |",
		),
		(
			"| `CRDProvider` | `util/configbus/crd_provider*.go` |",
			"| `CRDProvider` | `util/configbus/crd_provider.go` |",
		),
		(
			"| `HybridProvider` | `util/configbus/hybrid_provider*.go` |",
			"| `HybridProvider` | `util/configbus/hybrid_provider.go` |",
		),
		(
			"├── legacy_provider*.go           # LegacyProvider + per-component Legacy getters\n"
			"├── crd_provider*.go              # CRDProvider + per-component CRD getters\n"
			"├── hybrid_provider*.go           # HybridProvider + per-component Hybrid getters\n",
			"├── legacy_provider.go            # LegacyProvider + all component Legacy adapters/getters\n"
			"├── crd_provider.go               # CRDProvider + all CRD getters\n"
			"├── hybrid_provider.go            # HybridProvider + all Hybrid getters\n",
		),
		(
			"├── legacy_provider.go            # LegacyProvider + all component Legacy adapters/getters\n"
			"├── crd_provider*.go              # CRDProvider + per-component CRD getters\n"
			"├── hybrid_provider*.go           # HybridProvider + per-component Hybrid getters\n",
			"├── legacy_provider.go            # LegacyProvider + all component Legacy adapters/getters\n"
			"├── crd_provider.go               # CRDProvider + all CRD getters\n"
			"├── hybrid_provider.go            # HybridProvider + all Hybrid getters\n",
		),
	]
	for old, new in reps:
		text = text.replace(old, new)
	if text == orig:
		return False
	DOC.write_text(text)
	return True


def gofmt() -> None:
	paths = [p for p in (CFG / f"{x}_provider.go" for x in ("legacy", "crd", "hybrid")) if p.exists()]
	if not paths:
		return
	import os, shutil
	gofmt = shutil.which("gofmt") or "/usr/local/go/bin/gofmt"
	if not gofmt:
		root = os.environ.get("GOROOT") or ""
		cand = Path(root) / "bin" / "gofmt"
		gofmt = str(cand) if cand.is_file() else "gofmt"
	subprocess.run([gofmt, "-w", *[str(p) for p in paths]], check=True)


def main() -> int:
	parser = argparse.ArgumentParser(description=__doc__)
	parser.add_argument("--skip-docs", action="store_true")
	args = parser.parse_args()

	changed = False
	changed |= consolidate_prefix("legacy")
	changed |= consolidate_prefix("crd")
	changed |= consolidate_prefix("hybrid")
	if not args.skip_docs:
		changed |= update_docs()
	if changed:
		gofmt()
		print("consolidated provider files" + (" + docs" if DOC.exists() and not args.skip_docs else ""))
	else:
		print("already consolidated")
	return 0


if __name__ == "__main__":
	sys.exit(main())
