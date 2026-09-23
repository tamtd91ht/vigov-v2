#!/usr/bin/env python3
"""Sinh `kb/90-ephemeral/tien-do.md` từ `kb/90-ephemeral/tien-do/<module>.json`.

VÌ SAO TÁCH LÀM HAI: yêu cầu là *một tệp tiến độ*, nhưng một tệp mà nhiều agent cùng ghi là
đúng hình dạng `ROUTING.md` §3 cấm — lần ghi sau âm thầm xoá lần ghi trước, và không có gì đỏ.
Nên tách theo đúng trục đang va nhau:

    GHI  -> kb/90-ephemeral/tien-do/<module>.json   mỗi agent đúng một tệp, không ai chạm tệp ai
    ĐỌC  -> kb/90-ephemeral/tien-do.md              MỘT tệp, sinh ra, là thứ phiên sau mở

Hai agent không bao giờ mở cùng một tệp, nên không cần khoá — và không có khoá nghĩa là không
có khoá kẹt lại khi một agent chết giữa chừng.

Chạy:  python tools/tien_do.py      (đi kèm `make kb`)
Mã thoát: 0 = sinh xong, 1 = có tệp module hỏng.
"""

from __future__ import annotations

import datetime
import io
import json
import os
import sys

for _s in (sys.stdout, sys.stderr):
    try:
        _s.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
NGUON = os.path.join(ROOT, "kb", "90-ephemeral", "tien-do")
RA = os.path.join(ROOT, "kb", "90-ephemeral", "tien-do.md")

# Thứ tự này là thứ tự ĐỌC, không phải thứ tự chữ cái: thứ đang chặn nhiều nhất đứng trên.
TRANG_THAI = ("dang_lam", "chua_lam", "treo", "xong")
NHAN = {
    "dang_lam": "ĐANG LÀM",
    "chua_lam": "chưa làm",
    "treo": "treo",
    "xong": "xong",
}

# Hạn của một tệp SINH RA không đo được bằng ngày sinh nó — nó sinh lại mỗi lần `make kb`, nên
# hạn tính kiểu ấy không bao giờ tới. Hạn ở đây đo thứ đáng đo: lần cuối có người CẬP NHẬT một
# module. Quá hạn nghĩa là 90 ngày không ai chạm tiến độ, tức tệp này không còn đáng tin.
HAN_NGAY = 90


def doc(p: str) -> str:
    try:
        return io.open(p, encoding="utf-8").read()
    except Exception:
        return ""


def cau_hoi_mo() -> dict[int, str]:
    """id -> status, từ nguồn chuẩn. Tiến độ LIÊN KẾT tới câu hỏi, không chép nội dung."""
    try:
        ds = json.loads(doc(os.path.join(ROOT, "kb", "00-foundation", "open-questions.json")))
        return {int(q["id"]): q.get("status", "") for q in ds}
    except Exception:
        return {}


def nap() -> tuple[list[dict], list[str]]:
    mods: list[dict] = []
    loi: list[str] = []
    if not os.path.isdir(NGUON):
        return mods, ["kb/90-ephemeral/tien-do/ chưa tồn tại"]
    for ten in sorted(os.listdir(NGUON)):
        if not ten.endswith(".json"):
            continue
        p = os.path.join(NGUON, ten)
        try:
            d = json.loads(doc(p))
        except Exception as ex:
            loi.append(f"{ten}: JSON hỏng — {ex}")
            continue
        if d.get("module") != ten[:-5]:
            loi.append(f"{ten}: khoá `module` là '{d.get('module')}', không khớp tên tệp")
            continue
        mods.append(d)
    return mods, loi


def sap_xep(muc: list[dict]) -> list[dict]:
    return sorted(muc, key=lambda m: TRANG_THAI.index(m.get("trang_thai", "chua_lam"))
                  if m.get("trang_thai") in TRANG_THAI else 99)


def main() -> None:
    mods, loi = nap()
    if loi:
        for x in loi:
            print(f"[tien_do] LỖI {x}", file=sys.stderr)
        sys.exit(1)

    trang_thai_cau = cau_hoi_mo()
    moi_nhat = max([m.get("cap_nhat", "") for m in mods] or [""])
    try:
        han = (datetime.date.fromisoformat(moi_nhat)
               + datetime.timedelta(days=HAN_NGAY)).isoformat()
    except Exception:
        han = (datetime.date.today() + datetime.timedelta(days=HAN_NGAY)).isoformat()

    dem = {k: 0 for k in TRANG_THAI}
    for m in mods:
        for x in m.get("muc", []):
            if x.get("trang_thai") in dem:
                dem[x["trang_thai"]] += 1

    L: list[str] = []
    L.append("---")
    L.append("id: tien-do")
    L.append("tier: T5")
    L.append("source: GENERATED")
    L.append("owner: architecture")
    L.append(f"derived_from_commit: {os.environ.get('VIGOV_COMMIT', '')}")
    L.append(f"expires: {han}")
    L.append("owns_facts:")
    L.append('  - "tiến độ từng module: mục nào đã làm, chưa làm, đang treo, và nợ câu hỏi nào"')
    L.append("---")
    L.append("")
    L.append("# Tiến độ theo module")
    L.append("")
    L.append("**SINH RA — đừng sửa tệp này.** Sửa `kb/90-ephemeral/tien-do/<module>.json` rồi chạy")
    L.append("`make kb`. `hooks/progress_guard.py` chặn mọi lần ghi thẳng vào đây.")
    L.append("")
    L.append("Tệp này trả lời đúng một câu: **module nào còn nợ gì.** Vì sao làm thế → `kb/10-decisions/`.")
    L.append("Đã làm gì → `git log`. Cạm bẫy và quyết định đã chốt → `kb/90-ephemeral/ban-giao-phien.md`.")
    L.append("")
    L.append(f"Cập nhật gần nhất **{moi_nhat}** · hết hạn **{han}**. Hạn đo lần cuối có người cập nhật")
    L.append("một module, không phải lần cuối sinh tệp — quá hạn nghĩa là 90 ngày không ai chạm tới,")
    L.append("tức tin `git log` chứ đừng tin tệp này.")
    L.append("")
    L.append("| | |")
    L.append("|---|---|")
    for k in TRANG_THAI:
        L.append(f"| {NHAN[k]} | {dem[k]} |")
    L.append("")

    # Nợ confirm gom lên đầu: đó là thứ duy nhất phiên sau KHÔNG tự giải được.
    #
    # `no_confirm` CHỈ NHẬN SỐ HIỆU CÂU HỎI, và chỗ này gọi tên khiếm khuyết thay vì đổ bằng
    # traceback. VÌ SAO PHÉP KIỂM NÀY PHẢI CÓ Ở ĐÂY dù `hooks/progress_guard.py` đã kiểm cùng
    # một thứ: hook gắn vào `Edit|Write|MultiEdit`, nên một lần ghi sổ bằng
    # `python -c "...json.dump..."` qua `Bash` KHÔNG đi qua nó. Đo ngày 23/09/2026: năm mục văn
    # xuôi vào được `no_confirm` đúng theo đường ấy, và thứ phát hiện ra chúng là bộ sinh này —
    # bằng một `ValueError` không ai đọc ra nguyên nhân.
    #
    # Cùng lý do luật 5 #3c và luật 6 #8 mỗi luật cần HAI hình dạng: hook thấy một tệp và không
    # bao giờ đọc lại thứ đã nằm trên đĩa; chỉ một lần quét toàn kho mới trả lời được câu "hôm
    # nay cả kho còn mục nào hỏng không".
    no: dict[int, list[str]] = {}
    hong: list[str] = []
    for m in mods:
        for x in m.get("muc", []):
            for q in x.get("no_confirm", []) or []:
                try:
                    qi = int(q)
                except (TypeError, ValueError):
                    hong.append(
                        f"  {m['module']}/{x['id']}: `no_confirm` chứa {type(q).__name__} "
                        f"{str(q)[:70]!r}"
                    )
                    continue
                no.setdefault(qi, []).append(f"{m['module']}/{x['id']}")
    if hong:
        print("[tien_do] ĐỎ — `no_confirm` chỉ nhận SỐ HIỆU câu hỏi trong "
              "kb/00-foundation/open-questions.json:", file=sys.stderr)
        print("\n".join(hong), file=sys.stderr)
        print("\n  Văn xuôi thuộc về `tiep_theo`. `no_confirm` là LIÊN KẾT tới nguồn chuẩn — "
              "chép nội dung câu hỏi\n  vào đây là bản sao thứ hai sẽ trôi khỏi bản gốc "
              "(luật 9).", file=sys.stderr)
        sys.exit(1)
    if no:
        L.append("## Nợ khách chốt — chặn thật, không tự quyết được")
        L.append("")
        L.append("Nội dung câu hỏi ở `kb/00-foundation/open-questions.json`. Đây chỉ là ai đang chờ ai.")
        L.append("")
        L.append("| Câu | Trạng thái | Đang chặn |")
        L.append("|---|---|---|")
        for q in sorted(no):
            tt = trang_thai_cau.get(q, "KHÔNG CÓ TRONG open-questions.json")
            L.append(f"| #{q} | {tt} | {' · '.join(sorted(no[q]))} |")
        L.append("")

    for m in sorted(mods, key=lambda x: x["module"]):
        muc = sap_xep(m.get("muc", []))
        L.append(f"## `{m['module']}`")
        L.append("")
        L.append(f"Cập nhật {m.get('cap_nhat', '—')} · {len(muc)} mục")
        L.append("")
        L.append("| Mục | Trạng thái | Bằng chứng | Nợ | Kế tiếp |")
        L.append("|---|---|---|---|---|")
        for x in muc:
            nc = " ".join(f"#{q}" for q in (x.get("no_confirm") or [])) or "—"
            L.append("| `{}` — {} | {} | {} | {} | {} |".format(
                x.get("id", "?"),
                x.get("viec", ""),
                NHAN.get(x.get("trang_thai", ""), x.get("trang_thai", "?")),
                x.get("bang_chung") or "—",
                nc,
                x.get("tiep_theo") or "—",
            ))
        L.append("")

    with io.open(RA, "w", encoding="utf-8", newline="\n") as f:
        f.write("\n".join(L))
    print(f"[tien_do] {os.path.relpath(RA, ROOT)} — {len(mods)} module · "
          f"{sum(dem.values())} mục · hạn {han}")


if __name__ == "__main__":
    main()
