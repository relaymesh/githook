#!/usr/bin/env python3

import json
import re
import sys
from pathlib import Path


def normalize_version(raw: str) -> str:
    version = raw.strip()
    if version.startswith("v"):
        version = version[1:]
    if not re.fullmatch(r"\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.]+)?", version):
        raise SystemExit(f"invalid version: {raw}")
    return version


def update_chart(root: Path, version: str) -> None:
    chart_path = root / "charts" / "githook" / "Chart.yaml"
    text = chart_path.read_text(encoding="utf-8")
    text = re.sub(r"(?m)^version:\s*.*$", f"version: {version}", text)
    text = re.sub(r"(?m)^appVersion:\s*.*$", f'appVersion: "{version}"', text)
    chart_path.write_text(text, encoding="utf-8")


def update_typescript(root: Path, version: str) -> None:
    pkg_path = root / "sdk" / "typescript" / "worker" / "package.json"
    pkg = json.loads(pkg_path.read_text(encoding="utf-8"))
    pkg["version"] = version
    pkg_path.write_text(json.dumps(pkg, indent=2) + "\n", encoding="utf-8")

    lock_path = root / "sdk" / "typescript" / "worker" / "package-lock.json"
    if lock_path.exists():
        lock = json.loads(lock_path.read_text(encoding="utf-8"))
        lock["version"] = version
        if isinstance(lock.get("packages"), dict) and "" in lock["packages"]:
            lock["packages"][""]["version"] = version
        lock_path.write_text(json.dumps(lock, indent=2) + "\n", encoding="utf-8")


def update_python(root: Path, version: str) -> None:
    setup_path = root / "sdk" / "python" / "worker" / "setup.py"
    text = setup_path.read_text(encoding="utf-8")
    text = re.sub(r'return raw or "[^"]+"', f'return raw or "{version}"', text)
    setup_path.write_text(text, encoding="utf-8")


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: bump_versions.py <version>")
        return 1

    version = normalize_version(sys.argv[1])
    root = Path(__file__).resolve().parents[1]

    update_chart(root, version)
    update_typescript(root, version)
    update_python(root, version)

    print(f"updated chart/typescript/python versions to {version}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
