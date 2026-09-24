#!/usr/bin/env python3
"""In danh mục lệnh `/…` của kho — SINH từ frontmatter của `.claude/commands/*.md`.

VÌ SAO SINH CHỨ KHÔNG VIẾT TAY: một bảng lệnh viết tay lệch ngay ở lệnh kế tiếp được thêm —
luật 9, bất biến 1 cấm đúng thứ ấy. Mỗi tệp lệnh tự khai ba điều về chính nó: `description`,
`argument-hint`, `group`. Tệp này chỉ đọc và in, nên không bao giờ sai hơn chính các lệnh.

Chạy:  python tools/vigov_help.py            danh mục, gom theo nhóm
       python tools/vigov_help.py <lệnh>     chi tiết một lệnh (có hay không có dấu '/')
Mã thoát: 0 = in xong · 1 = có lệnh thiếu `description` · 2 = không có lệnh tên ấy
"""

from __future__ import annotations

import io
import os
import re
import sys

for _s in (sys.stdout, sys.stderr):
    try:
        _s.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
THU_MUC = os.path.join(ROOT, ".claude", "commands")

# Thứ tự ĐỌC của các nhóm — việc hằng ngày trước, kiểm bộ não sau. Một nhóm mới mà một lệnh
# khai nhưng không có ở đây vẫn được in (cuối danh sách): danh sách này chỉ quyết thứ tự, không
# quyết lệnh nào được hiện. Lệnh không khai `group` rơi vào "Chưa xếp nhóm" — hiện ra để có người
# sửa, chứ không lặng lẽ biến mất.
THU_TU_NHOM = ("Phát triển", "Tiến độ & bàn giao", "Yêu cầu & quyết định", "Rà soát", "Bộ não")
CHUA_XEP = "Chưa xếp nhóm"


def frontmatter(van_ban: str) -> dict[str, str]:
    m = re.match(r"^---\r?\n(.*?)\r?\n---", van_ban, re.S)
    if not m:
        return {}
    ra: dict[str, str] = {}
    for dong in m.group(1).splitlines():
        k = re.match(r"^([A-Za-z-]+):\s*(.*)$", dong)
        if k:
            ra[k.group(1)] = k.group(2).strip().strip('"').strip("'")
    return ra


def doc_lenh() -> list[dict]:
    ds = []
    for ten in sorted(os.listdir(THU_MUC)):
        if not ten.endswith(".md"):
            continue
        p = os.path.join(THU_MUC, ten)
        van_ban = io.open(p, encoding="utf-8").read()
        fm = frontmatter(van_ban)
        ds.append({
            "ten": ten[:-3],
            "mo_ta": fm.get("description", ""),
            "cu_phap": fm.get("argument-hint", ""),
            "nhom": fm.get("group") or CHUA_XEP,
            "de_muc": re.findall(r"^##\s+(.+)$", van_ban, re.M),
            "tep": os.path.relpath(p, ROOT).replace("\\", "/"),
        })
    return ds


def cu_phap(x: dict) -> str:
    # `argument-hint` thường là "<đối số> — giải thích"; phần trước dấu gạch là cú pháp.
    doi_so = re.split(r"\s+—\s+", x["cu_phap"], maxsplit=1)[0] if x["cu_phap"] else ""
    if doi_so.lower() in ("(none)", "none", "(không)"):
        doi_so = ""
    return f"/{x['ten']} {doi_so}".rstrip()


def o(s: str) -> str:
    """A table cell: `[API|WebAdmin]` and `diff | all` would otherwise split the row."""
    return s.replace("|", "\\|")


def in_danh_muc(ds: list[dict]) -> None:
    nhom = sorted({x["nhom"] for x in ds},
                  key=lambda n: (THU_TU_NHOM.index(n) if n in THU_TU_NHOM else len(THU_TU_NHOM)
                                 + (1 if n == CHUA_XEP else 0), n))
    print(f"# Lệnh của ViGov — {len(ds)} lệnh\n")
    print("Sinh từ frontmatter của `.claude/commands/*.md`. Chi tiết một lệnh: `/vigov-help <lệnh>`.\n")
    for n in nhom:
        print(f"## {n}\n")
        print("| Cú pháp | Chức năng |")
        print("|---|---|")
        for x in (y for y in ds if y["nhom"] == n):
            print(f"| `{o(cu_phap(x))}` | {o(x['mo_ta']) or '**THIẾU description**'} |")
        print()


def in_chi_tiet(x: dict) -> None:
    print(f"# `/{x['ten']}`\n")
    print(f"**Chức năng:** {x['mo_ta'] or '—'}\n")
    print(f"**Cú pháp:** `{cu_phap(x)}`\n")
    if x["cu_phap"]:
        print(f"**Đối số:** {x['cu_phap']}\n")
    print(f"**Nhóm:** {x['nhom']} · **Tệp:** `{x['tep']}`\n")
    if x["de_muc"]:
        print("**Các bước / mục trong lệnh:**\n")
        for d in x["de_muc"]:
            print(f"- {d}")


def main() -> int:
    ds = doc_lenh()
    if len(sys.argv) > 1:
        # Git Bash rewrites a leading-slash argument into a Windows path
        # ("/develop-feature" -> "C:/Program Files/Git/develop-feature"): keep the last segment.
        q = sys.argv[1].replace("\\", "/").rstrip("/").rsplit("/", 1)[-1].strip().lower()
        hit = [x for x in ds if x["ten"] == q] or [x for x in ds if q in x["ten"]]
        if len(hit) != 1:
            print(f"Không có đúng một lệnh khớp '{sys.argv[1]}' — khớp: "
                  f"{', '.join('/' + x['ten'] for x in hit) or 'không có'}.", file=sys.stderr)
            print("Có: " + ", ".join("/" + x["ten"] for x in ds), file=sys.stderr)
            return 2
        in_chi_tiet(hit[0])
        return 0
    in_danh_muc(ds)
    return 1 if any(not x["mo_ta"] for x in ds) else 0


if __name__ == "__main__":
    sys.exit(main())
