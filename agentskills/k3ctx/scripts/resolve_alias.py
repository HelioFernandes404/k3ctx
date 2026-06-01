#!/usr/bin/env python3
"""Resolve a human alias/term to a k3ctx context name by searching host manifests."""

import json
import os
import re
import sys

try:
    import yaml
except ImportError:
    print(json.dumps({"ok": False, "error": "PyYAML not installed — run: pip install pyyaml"}))
    sys.exit(1)

HOSTS_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "hosts")

SCORE_EXACT_ALIAS = 100
SCORE_PARTIAL_ALIAS = 80
SCORE_SFID = 60
SCORE_DESCRIPTION = 40
SCORE_TAG = 20


def parse_frontmatter(path):
    with open(path) as f:
        content = f.read()
    m = re.match(r"^---\n(.*?)\n---", content, re.DOTALL)
    if not m:
        return None
    try:
        return yaml.safe_load(m.group(1))
    except yaml.YAMLError:
        return None


def score_host(host, term):
    t = term.lower()
    aliases = [a.lower() for a in host.get("aliases", [])]
    sid = host.get("systemframe_id", "").lower()
    desc = host.get("description", "").lower()
    tags = [g.lower() for g in host.get("tags", [])]

    if t in aliases:
        return SCORE_EXACT_ALIAS, "exact_alias"
    if any(t in a for a in aliases):
        return SCORE_PARTIAL_ALIAS, "partial_alias"
    if t in sid:
        return SCORE_SFID, "systemframe_id"
    if t in desc:
        return SCORE_DESCRIPTION, "description"
    if any(t in g for g in tags):
        return SCORE_TAG, "tag"
    return 0, None


def main():
    if len(sys.argv) < 2:
        print(json.dumps({"ok": False, "error": "Usage: resolve-alias.sh <term>"}))
        sys.exit(1)

    term = " ".join(sys.argv[1:])
    hosts_dir = os.path.abspath(HOSTS_DIR)

    if not os.path.isdir(hosts_dir):
        print(json.dumps({"ok": False, "error": f"hosts/ dir not found: {hosts_dir}"}))
        sys.exit(1)

    results = []
    for fname in sorted(os.listdir(hosts_dir)):
        if not fname.endswith(".md"):
            continue
        host = parse_frontmatter(os.path.join(hosts_dir, fname))
        if not host:
            continue
        numeric_score, reason = score_host(host, term)
        if numeric_score > 0:
            results.append({
                "_score": numeric_score,
                "match_reason": reason,
                "systemframe_id": host.get("systemframe_id"),
                "context_name": host.get("context_name"),
                "netbird_fqdn": host.get("netbird_fqdn"),
                "aliases": host.get("aliases", []),
                "description": host.get("description", ""),
            })

    results.sort(key=lambda x: -x["_score"])

    clean = [{k: v for k, v in r.items() if k != "_score"} for r in results]

    print(json.dumps({"ok": bool(clean), "query": term, "matches": clean}, indent=2))

    if not clean:
        sys.exit(1)


if __name__ == "__main__":
    main()
