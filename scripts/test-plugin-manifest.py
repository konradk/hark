#!/usr/bin/env python3
import json
import re
from pathlib import Path


root = Path(__file__).resolve().parent.parent
manifest_path = root / "manifest.json"
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))

# "author" and "description" are optional for omarchy itself, but the plugin
# marketplace rejects a listing without them.
required = {
    "schemaVersion",
    "id",
    "name",
    "version",
    "author",
    "description",
    "kinds",
    "entryPoints",
}
missing = sorted(required - manifest.keys())
if missing:
    raise SystemExit(f"manifest is missing: {', '.join(missing)}")
if manifest["schemaVersion"] != 1:
    raise SystemExit("schemaVersion must be 1")
if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", manifest["id"]):
    raise SystemExit("plugin id is invalid")
if manifest["id"].startswith("omarchy."):
    raise SystemExit("plugin id uses the reserved omarchy namespace")
if not re.fullmatch(r"\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?", manifest["version"]):
    raise SystemExit("plugin version is not SemVer")
if not isinstance(manifest["kinds"], list) or not manifest["kinds"]:
    raise SystemExit("plugin kinds must be a non-empty list")

required_entry_points = {
    "service": "service",
    "overlay": "overlay",
    "bar-widget": "barWidget",
    "panel": "panel",
    "menu": "menu",
    "bar": "bar",
}
entry_points = manifest["entryPoints"]
if not isinstance(entry_points, dict):
    raise SystemExit("entryPoints must be an object")
for kind in manifest["kinds"]:
    key = required_entry_points.get(kind)
    if key and key not in entry_points:
        raise SystemExit(f"kind {kind!r} requires entryPoints.{key}")

if "bar-widget" in manifest["kinds"]:
    bar_widget = manifest.get("barWidget")
    if not isinstance(bar_widget, dict):
        raise SystemExit("bar-widget requires barWidget metadata")
    if bar_widget.get("defaultSection") not in {"left", "center", "right"}:
        raise SystemExit("barWidget.defaultSection must be left, center, or right")
    if not isinstance(bar_widget.get("allowMultiple"), bool):
        raise SystemExit("barWidget.allowMultiple must be a boolean")

for name, value in entry_points.items():
    if not isinstance(value, str) or not value or value.startswith("/") or ".." in value:
        raise SystemExit(f"unsafe entry point {name!r}: {value!r}")
    if not (root / value).is_file():
        raise SystemExit(f"missing entry point {name!r}: {value}")

for path in root.rglob("*"):
    if ".git" in path.parts:
        continue
    if path.is_symlink():
        raise SystemExit(f"symlink is not allowed in a plugin: {path.relative_to(root)}")

print("Plugin manifest test passed.")
