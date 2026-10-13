"""Pillow before/after diff: prove a re-render fixed what the eyes saw.
Usage: python diff_png.py <before.png> <after.png> [out.png]
Output: JSON {changed_pct, mean_abs, bbox, out} to stdout.
"""
import json
import sys

from PIL import Image, ImageChops


def main(argv):
    if len(argv) < 3:
        print(json.dumps({"ok": False, "error": "need before after [out]"}))
        return 2
    a = Image.open(argv[1]).convert("RGB")
    b = Image.open(argv[2]).convert("RGB")
    w, h = min(a.width, b.width), min(a.height, b.height)
    a, b = a.crop((0, 0, w, h)), b.crop((0, 0, w, h))
    diff = ImageChops.difference(a, b)
    bbox = diff.getbbox()
    import statistics
    px = list(diff.getdata())
    mean_abs = sum(sum(p) for p in px) / (len(px) * 3.0)
    changed = sum(1 for p in px if sum(p) > 30) / len(px) * 100.0
    rep = {"ok": True, "size": [w, h],
           "changed_pct": round(changed, 2), "mean_abs": round(mean_abs, 2),
           "bbox": list(bbox) if bbox else []}
    if len(argv) > 3:
        diff.save(argv[3])
        rep["out"] = argv[3]
    print(json.dumps(rep, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
