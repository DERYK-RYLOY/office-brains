"""Pillow glyph oracle: measure real text width with a real font file.
This is the upgrade path for layout/width.go (currently 7px/char estimates).

Usage: python measure.py <font.ttf> <size_pt> <text>...
Output: JSON {font, size_pt, widths_px:[...]} to stdout.
"""
import json
import sys

from PIL import ImageFont


def main(argv):
    if len(argv) < 4:
        print(json.dumps({"ok": False, "error": "need font size text..."}))
        return 2
    font_path, size = argv[1], int(argv[2])
    font = ImageFont.truetype(font_path, size)
    widths = [font.getlength(t) for t in argv[3:]]
    print(json.dumps({"ok": True, "font": font_path, "size_pt": size,
                      "widths_px": widths}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
