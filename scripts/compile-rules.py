#!/usr/bin/env python3
"""Compile JSON rule files into Go source files.

Usage:
    python3 scripts/compile-rules.py
    python3 scripts/compile-rules.py -in rules/data -out rules

Flags:
    -in   <dir>   source directory with JSON files  (default: rules/data)
    -out  <dir>   output directory for Go files      (default: rules)

Output:
    <out>/init.go          — var Rules = map[string]*rtbrules.RTBRules{...}
    <out>/<name>.go        — one file per JSON source in <in>/
"""

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path


REPO_ROOT = Path(__file__).parent.parent
_DEFAULT_IN  = REPO_ROOT / "rules" / "data"
_DEFAULT_OUT = REPO_ROOT / "rules"

MODULE_PATH = "github.com/geniusrabbit/adsource-openrtb/request/rtbrules"
INFO_PATH   = "github.com/geniusrabbit/adcorelib/platform/info"

APLICABLE_MAP: dict[str, str] = {
    "exclude":     "rtbrules.Exclude",
    "exc":         "rtbrules.Exclude",
    "exclude_all": "rtbrules.Exclude",
    "include":     "rtbrules.Include",
    "inc":         "rtbrules.Include",
    "include_all": "rtbrules.Include",
    "any":         "rtbrules.Any",
    "*":           "rtbrules.Any",
    "":            "rtbrules.Any",
}

GO_KEYWORDS = {
    "break", "default", "func", "interface", "select", "case", "defer",
    "go", "map", "struct", "chan", "else", "goto", "package", "switch",
    "const", "fallthrough", "if", "range", "type", "continue", "for",
    "import", "return", "var",
}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def go_ident(name: str) -> str:
    """Return a valid Go identifier for a JSON file stem."""
    if name in GO_KEYWORDS:
        return name + "Rules"
    return name

def go_string_slice(items: list[str]) -> str:
    if not items:
        return "nil"
    quoted = ", ".join(f'"{s}"' for s in items)
    return f"[]string{{{quoted}}}"


def go_value(v, indent: int = 0) -> str:
    """Recursively render a Python value as a Go literal."""
    tab = "\t" * indent
    inner = "\t" * (indent + 1)

    if v is None:
        return "nil"
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, int):
        return str(v)
    if isinstance(v, float):
        return repr(v)
    if isinstance(v, str):
        return json.dumps(v)
    if isinstance(v, list):
        if not v:
            return "nil"
        if all(isinstance(i, str) for i in v):
            return go_string_slice(v)
        lines = ["[]any{"]
        for item in v:
            lines.append(f"{inner}{go_value(item, indent + 1)},")
        lines.append(f"{tab}}}")
        return "\n".join(lines)
    if isinstance(v, dict):
        if not v:
            return "nil"
        lines = ["map[string]any{"]
        for k, val in v.items():
            lines.append(f"{inner}{json.dumps(k)}: {go_value(val, indent + 1)},")
        lines.append(f"{tab}}}")
        return "\n".join(lines)
    return "nil"


def go_aplicable(val: str) -> str:
    return APLICABLE_MAP.get(val.strip().lower() if val else "", "rtbrules.Any")


# ---------------------------------------------------------------------------
# Section generators
# ---------------------------------------------------------------------------

def gen_condition(cond: dict, indent: int) -> str:
    tab  = "\t" * indent
    inner = "\t" * (indent + 1)
    parts: list[str] = []

    if cond.get("formats"):
        parts.append(f"{inner}Formats: {go_string_slice(cond['formats'])},")
    if cond.get("interstitial"):
        parts.append(f"{inner}Interstitial: {go_aplicable(cond['interstitial'])},")
    if cond.get("push"):
        parts.append(f"{inner}Push: {go_aplicable(cond['push'])},")

    if not parts:
        return "rtbrules.Condition{}"
    body = "\n".join(parts)
    return f"rtbrules.Condition{{\n{body}\n{tab}}}"


def gen_rule_config(config: dict, indent: int) -> str | None:
    """Return Go RuleConfig literal or None when empty."""
    ext              = config.get("ext")
    no_request_obj   = config.get("no_request_object", False)
    if not ext and not no_request_obj:
        return None
    tab   = "\t" * indent
    inner = "\t" * (indent + 1)
    parts: list[str] = []
    if ext:
        parts.append(f"{inner}Ext: {go_value(ext, indent + 1)},")
    if no_request_obj:
        parts.append(f"{inner}NoRequestObject: true,")
    return f"rtbrules.RuleConfig{{\n" + "\n".join(parts) + f"\n{tab}}}"


def gen_map_response(mr: dict, indent: int) -> str:
    tab   = "\t" * indent
    inner = "\t" * (indent + 1)
    a_tab = "\t" * (indent + 2)

    assets = mr.get("assets", [])
    if not assets:
        return "&rtbrules.MapResponse{}"

    lines = ["&rtbrules.MapResponse{"]
    lines.append(f"{inner}Assets: []rtbrules.MapResponseAsset{{")
    for a in assets:
        fields: list[str] = []
        if "id" in a:
            fields.append(f"ID: {a['id']}")
        if "name" in a:
            fields.append(f'Name: {json.dumps(a["name"])}')
        if "field" in a:
            fields.append(f'Field: {json.dumps(a["field"])}')
        lines.append(f"{a_tab}{{{', '.join(fields)}}},")
    lines.append(f"{inner}}},")
    lines.append(f"{tab}}}")
    return "\n".join(lines)


def gen_rule_item(rule: dict, indent: int) -> str:
    tab   = "\t" * indent
    inner = "\t" * (indent + 1)
    lines = [f"{tab}{{"]

    if "condition" in rule:
        cond_str = gen_condition(rule["condition"], indent + 1)
        lines.append(f"{inner}Condition: {cond_str},")

    if "config" in rule:
        cfg_str = gen_rule_config(rule["config"], indent + 1)
        if cfg_str:
            lines.append(f"{inner}Config: {cfg_str},")

    if "map_response" in rule:
        mr_str = gen_map_response(rule["map_response"], indent + 1)
        lines.append(f"{inner}MapResponse: {mr_str},")

    lines.append(f"{tab}}},")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# File generators
# ---------------------------------------------------------------------------

def gen_meta(meta: dict, indent: int) -> str:
    tab   = "\t" * indent
    inner = "\t" * (indent + 1)
    d_tab = "\t" * (indent + 2)
    parts: list[str] = []

    if meta.get("title"):
        parts.append(f"{inner}Title: {json.dumps(meta['title'])},")
    if meta.get("description"):
        parts.append(f"{inner}Description: {json.dumps(meta['description'])},")
    if meta.get("version"):
        parts.append(f"{inner}Version: {json.dumps(meta['version'])},")
    if meta.get("modified_at"):
        dt = datetime.fromisoformat(meta["modified_at"])
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        tz = "time.UTC" if dt.utcoffset().total_seconds() == 0 else "time.Local"
        parts.append(
            f"{inner}ModifiedAt: time.Date("
            f"{dt.year}, {dt.month}, {dt.day}, "
            f"{dt.hour}, {dt.minute}, {dt.second}, 0, {tz}),"
        )
    if meta.get("docs"):
        lines = [f"{inner}Docs: []info.Documentation{{"]
        for doc in meta["docs"]:
            title = json.dumps(doc.get("title", ""))
            link  = json.dumps(doc.get("link", ""))
            lines.append(f"{d_tab}{{Title: {title}, Link: {link}}},")
        lines.append(f"{inner}}},")
        parts.append("\n".join(lines))

    if not parts:
        return "rtbrules.Meta{}"
    body = "\n".join(parts)
    return f"rtbrules.Meta{{\n{body}\n{tab}}}"


def generate_rules_file(name: str, data: dict) -> str:
    ident    = go_ident(name)
    meta     = data.get("meta") or {}
    has_meta = bool(meta)
    std_imports: list[str] = []
    ext_imports: list[str] = [f'\t"{MODULE_PATH}"']
    if meta.get("modified_at"):
        std_imports.append('\t"time"')
    if meta.get("docs"):
        ext_imports.append(f'\t"{INFO_PATH}"')
    ext_imports.sort()

    import_lines = ["import ("]
    if std_imports:
        import_lines.extend(std_imports)
        if ext_imports:
            import_lines.append("")
    import_lines.extend(ext_imports)
    import_lines.append(")")

    lines: list[str] = [
        "// Code generated by scripts/compile-rules.py; DO NOT EDIT.",
        "",
        "package rules",
        "",
        *import_lines,
        "",
        f"var {ident} = &rtbrules.RTBRules{{",
    ]

    if has_meta:
        meta_str = gen_meta(data["meta"], indent=1)
        lines.append(f"\tMeta: {meta_str},")
    if data.get("formats"):
        lines.append(f"\tFormats: {go_string_slice(data['formats'])},")
    if data.get("interstitial_formats"):
        lines.append(f"\tInterstitialFormats: {go_string_slice(data['interstitial_formats'])},")
    if data.get("push_formats"):
        lines.append(f"\tPushFormats: {go_string_slice(data['push_formats'])},")

    rules = data.get("rules", [])
    if rules:
        lines.append("\tRules: []*rtbrules.RuleItem{")
        for rule in rules:
            lines.append(gen_rule_item(rule, indent=2))
        lines.append("\t},")

    lines.append("}")
    lines.append("")
    return "\n".join(lines)


def generate_init_file(entries: list[tuple[str, str]]) -> str:
    lines: list[str] = [
        "// Code generated by rules/compile.py; DO NOT EDIT.",
        "",
        "package rules",
        "",
        "import (",
        f'\t"{MODULE_PATH}"',
        ")",
        "",
        "// Rules maps RTB source names to their compiled rule sets.",
        "var Rules = map[string]*rtbrules.RTBRules{",
    ]
    for key, ident in sorted(entries, key=lambda e: e[0]):
        lines.append(f'\t"{key}": {ident},')
    lines.append("}")
    lines.append("")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Compile JSON rule files into Go source files.",
    )
    parser.add_argument(
        "-in", dest="input", metavar="DIR",
        default=str(_DEFAULT_IN),
        help=f"source directory with JSON files (default: {_DEFAULT_IN})",
    )
    parser.add_argument(
        "-out", dest="output", metavar="DIR",
        default=str(_DEFAULT_OUT),
        help=f"output directory for Go files (default: {_DEFAULT_OUT})",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    data_dir   = Path(args.input)
    output_dir = Path(args.output)

    if not data_dir.exists():
        print(f"error: input directory not found: {data_dir}", file=sys.stderr)
        sys.exit(1)

    json_files = sorted(data_dir.glob("*.json"))
    if not json_files:
        print(f"error: no JSON files found in {data_dir}", file=sys.stderr)
        sys.exit(1)

    output_dir.mkdir(parents=True, exist_ok=True)

    entries: list[tuple[str, str]] = []
    for json_file in json_files:
        name = json_file.stem
        ident = go_ident(name)
        entries.append((name, ident))

        with open(json_file, encoding="utf-8") as f:
            data = json.load(f)

        go_code = generate_rules_file(name, data)
        out_path = output_dir / f"{name}.go"
        out_path.write_text(go_code, encoding="utf-8")
        print(f"generated: {out_path}")

    init_code = generate_init_file(entries)
    init_path = output_dir / "init.go"
    init_path.write_text(init_code, encoding="utf-8")
    print(f"generated: {init_path}")


if __name__ == "__main__":
    main()
