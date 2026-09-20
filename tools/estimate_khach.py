#!/usr/bin/env python3
"""Sinh bản GỬI KHÁCH của estimate từ bản NỘI BỘ.

VÌ SAO SINH CHỨ KHÔNG CHÉP: hai tệp estimate gần giống nhau là hai tệp sẽ lệch, và bản lệch
là bản ai đó gửi ra ngoài (luật 9, bất biến 2). Một nguồn duy nhất — bản nội bộ — và bản khách
là kết quả cắt bỏ, không phải một tài liệu thứ hai được bảo trì song song.

CÁCH ĐÁNH DẤU trong bản nội bộ:

    <!-- NOI-BO -->        mọi dòng tới khi gặp <!-- /NOI-BO --> BỊ CẮT khỏi bản khách
    <!-- /NOI-BO -->
    ... <!-- NOI-BO -->đoạn giữa dòng<!-- /NOI-BO --> ...   cắt ngay trong một dòng

Cắt gì: số đo nội bộ (sản lượng đã tiêu, khối lượng từng lát cắt) và ví dụ kỹ thuật lộ chi
tiết cài đặt. KHÔNG cắt: hệ số, các phần đệm và tỉ lệ của chúng, rủi ro, phạm vi. Khách có
quyền biết mình đang trả cho cái gì và đang được đệm bao nhiêu.

Chạy:  python tools/estimate_khach.py      (đi kèm `make kb`)
Mã thoát: 0 = sinh xong, 1 = đánh dấu không cân.
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
NGUON = os.path.join(ROOT, "kb", "90-ephemeral", "estimate-vigov.md")
RA = os.path.join(ROOT, "kb", "90-ephemeral", "estimate-vigov-khach.md")

MO, DONG = "<!-- NOI-BO -->", "<!-- /NOI-BO -->"
TRONG_DONG = re.compile(re.escape(MO) + r".*?" + re.escape(DONG))

# Frontmatter của bản khách. `owns_facts` phải KHÁC bản nội bộ — hai tệp cùng khai một sự thật
# là đúng thứ `doc_guard` chặn, và ở đây chúng thật sự khai hai thứ khác nhau: bản nội bộ sở
# hữu con số, bản này sở hữu CÁCH TRÌNH BÀY con số ấy ra ngoài.
FRONTMATTER = """---
id: estimate-vigov-khach
tier: T5
source: GENERATED
owner: architecture
derived_from_commit: {commit}
expires: {han}
owns_facts:
  - "bản trình bày ra ngoài của estimate ViGov v2 — sinh từ bản nội bộ, đã cắt số đo nội bộ"
---

<!-- SINH RA bởi `python tools/estimate_khach.py` từ kb/90-ephemeral/estimate-vigov.md.
     KHÔNG sửa tệp này — sửa bản nội bộ rồi chạy `make kb`. Sửa tay sẽ mất ở lần sinh sau. -->
"""


def doc(p: str) -> str:
    return io.open(p, encoding="utf-8").read()


def main() -> None:
    if not os.path.exists(NGUON):
        print(f"[estimate_khach] LỖI không có {NGUON}", file=sys.stderr)
        sys.exit(1)
    goc = doc(NGUON)

    m = re.match(r"\A---\r?\n(.*?)\r?\n---\r?\n", goc, re.S)
    if not m:
        print("[estimate_khach] LỖI bản nội bộ không có frontmatter", file=sys.stderr)
        sys.exit(1)
    fm, than = m.group(1), goc[m.end():]

    commit = (re.search(r"^derived_from_commit:\s*(\S+)", fm, re.M) or [None, ""])[1]
    han = (re.search(r"^expires:\s*(\S+)", fm, re.M) or [None, ""])[1]

    # 1. cắt các đoạn NẰM GỌN trong một dòng trước, để phần còn lại của dòng được giữ
    than, trong_dong_dem = TRONG_DONG.subn("", than)

    # 2. cắt các khối trải nhiều dòng
    ra: list[str] = []
    trong_khoi = False
    mo_dem = 0
    for ln in than.split("\n"):
        if ln.strip() == MO:
            trong_khoi, mo_dem = True, mo_dem + 1
            continue
        if ln.strip() == DONG:
            if not trong_khoi:
                print("[estimate_khach] LỖI gặp <!-- /NOI-BO --> mà chưa mở", file=sys.stderr)
                sys.exit(1)
            trong_khoi = False
            continue
        if not trong_khoi:
            ra.append(ln)
    if trong_khoi:
        print("[estimate_khach] LỖI một khối <!-- NOI-BO --> chưa đóng", file=sys.stderr)
        sys.exit(1)

    than = "\n".join(ra)
    # gộp ba dòng trống trở lên thành hai — cắt khối hay để lại khoảng trắng thừa
    than = re.sub(r"\n{3,}", "\n\n", than)

    with io.open(RA, "w", encoding="utf-8", newline="\n") as f:
        f.write(FRONTMATTER.format(commit=commit, han=han))
        f.write(than.lstrip("\n"))

    print(f"[estimate_khach] {os.path.relpath(RA, ROOT)} — cắt {mo_dem} khối "
          f"+ {trong_dong_dem} đoạn trong dòng")


if __name__ == "__main__":
    main()
