"""Oracle strict check: independently open Office files with the reference
Python libraries (python-docx / openpyxl / python-pptx). Anything THEY reject
is a bug in our emitter, no matter what our own verify says.

Usage: python verify.py <file> [<file> ...]
Output: JSON to stdout, exit 0 always (brains maps ok flags).
"""
import json
import sys
import traceback


def check_docx(path):
    from docx import Document
    d = Document(path)
    tables = [{"rows": len(t.rows), "cols": len(t.rows[0].cells) if t.rows else 0}
              for t in d.tables]
    paras = [p.text for p in d.paragraphs if p.text.strip()]
    return {"type": "docx", "paragraphs": len(paras),
            "tables": tables, "images": len(d.inline_shapes),
            "author": (d.core_properties.author or "")}


def check_xlsx(path):
    from openpyxl import load_workbook
    wb = load_workbook(path)
    sheets = []
    for ws in wb.worksheets:
        charts = len(getattr(ws, "_charts", []))
        images = len(getattr(ws, "_images", []))
        merges = [str(r) for r in ws.merged_cells.ranges]
        sheets.append({"name": ws.title, "dimension": str(ws.dimensions),
                       "max_row": ws.max_row, "max_column": ws.max_column,
                       "merges": merges, "charts": charts, "images": images})
    return {"type": "xlsx", "sheets": sheets,
            "calc_full_on_load": bool(getattr(wb.calculation, "fullCalcOnLoad", False))}


def check_pptx(path):
    from pptx import Presentation
    from pptx.enum.shapes import MSO_SHAPE_TYPE
    prs = Presentation(path)
    slides = []
    for sl in prs.slides:
        kinds = {}
        for sh in sl.shapes:
            k = "other"
            if sh.has_table:
                k = "table"
            elif sh.has_chart:
                k = "chart"
            elif sh.shape_type == MSO_SHAPE_TYPE.PICTURE:
                k = "picture"
            elif sh.has_text_frame:
                k = "text"
            kinds[k] = kinds.get(k, 0) + 1
        slides.append({"shapes": kinds})
    w = prs.slide_width / 914400.0
    h = prs.slide_height / 914400.0
    return {"type": "pptx", "slides": len(slides), "details": slides,
            "size_in": [round(w, 2), round(h, 2)]}


def check_one(path):
    low = path.lower()
    try:
        if low.endswith(".docx"):
            facts = check_docx(path)
        elif low.endswith(".xlsx"):
            facts = check_xlsx(path)
        elif low.endswith(".pptx"):
            facts = check_pptx(path)
        else:
            return {"file": path, "ok": False, "error": "ERR_ORACLE_TYPE"}
        facts["file"] = path
        facts["ok"] = True
        return facts
    except Exception as e:  # noqa: BLE001 - oracle must report, never crash
        return {"file": path, "ok": False,
                "error": "ERR_ORACLE_REJECTED",
                "detail": "%s: %s" % (type(e).__name__, str(e)[:300]),
                "trace": traceback.format_exc(limit=3)}


def main(argv):
    if len(argv) < 2:
        print(json.dumps({"ok": False, "error": "need file args"}))
        return 2
    out = {"ok": True, "files": [check_one(p) for p in argv[1:]]}
    out["ok"] = all(f["ok"] for f in out["files"])
    print(json.dumps(out, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
