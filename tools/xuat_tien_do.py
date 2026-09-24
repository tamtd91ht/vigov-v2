#!/usr/bin/env python3
"""Xuất tiến độ dự án ra MỘT tệp .xlsx theo FORMAT CỐ ĐỊNH, để import vào Google Sheet của team.

Chạy:  python tools/xuat_tien_do.py [đường-dẫn.xlsx]
       python tools/tien_do_san_pham.py --excel [đường-dẫn.xlsx]      (cùng một việc)
Mặc định: tmp/tien-do/vigov-tien-do-<YYYY-MM-DD>.xlsx  (tmp/ nằm trong .gitignore)
Mã thoát: 0 = ghi xong · 1 = thiếu nguồn · 3 = có ô trông như dữ liệu cá nhân, KHÔNG ghi tệp
          4 = đường dẫn nằm trong kho mà git không bỏ qua — tệp nhị phân không được vào git

VÌ SAO MỘT FORMAT CỐ ĐỊNH, và "cố định" nghĩa là gì ở đây: team import tệp này bằng
File → Import → *Replace spreadsheet*, rồi dựng công thức, bộ lọc, VLOOKUP lên trên. Một tên
sheet hay tên cột đổi đi là mọi công thức ấy vỡ mà không báo. Nên:
  * tên sheet và tên cột KHÔNG DẤU, KHÔNG BAO GIỜ ĐỔI; cột mới chỉ THÊM VÀO CUỐI
  * mỗi lần thêm cột thì tăng PHIEN_BAN_FORMAT — sheet `Thong_tin` in nó ra
  * cột `Ma` của `Hang_muc` (`module/id`) là khoá ổn định giữa các lần xuất; ghi chú của team
    để ở một sheet RIÊNG tra theo `Ma`, vì *Replace* xoá mọi cột team tự thêm vào sheet nhập

KHÔNG MỘT Ô NÀO VIẾT TAY: số theo phân hệ lấy từ `tien_do_san_pham.dong_chuong/dong_menu` —
CÙNG hàm sinh ra `tien-do-san-pham.md`, không phải một bản tính lại — và hạng mục lấy từ sổ
`kb/90-ephemeral/tien-do/*.json`. Không cột "% hoàn thành" nào, cùng lý do tệp kia ghi.

VÌ SAO THƯ VIỆN CHUẨN, không openpyxl: mọi công cụ của kho chỉ dùng thư viện chuẩn, và máy khác
hay Jenkins có thể không có gói ấy. Một .xlsx chỉ là một zip các tệp XML; phần dưới viết đúng
tập con cần dùng — chuỗi nội tuyến, số, tiêu đề in đậm, đóng băng dòng đầu, bộ lọc, danh sách
chọn cho cột trạng thái.

TRƯỚC KHI GHI, mọi ô chữ được soi bằng CÙNG mẫu số điện thoại / CCCD của `pii_guard`
(`_common.VN_PHONE`, `VN_CCCD`). Sổ tiến độ vốn không được chứa dữ liệu cá nhân (luật 3), nhưng
một Google Sheet chia sẻ cho cả team là một kênh ra ngoài — lớp chặn thứ hai đặt đúng ở cửa ra.
Trúng thì từ chối và chỉ in TOẠ ĐỘ ô, không bao giờ in giá trị.
"""

from __future__ import annotations

import datetime
import io
import json
import os
import re
import subprocess
import sys
import zipfile
from xml.sax.saxutils import escape

for _s in (sys.stdout, sys.stderr):
    try:
        _s.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(ROOT, "tools"))
sys.path.insert(0, os.path.join(ROOT, ".claude", "hooks"))
import tien_do_san_pham as sp  # noqa: E402
import _common as c  # noqa: E402

PHIEN_BAN_FORMAT = 1

NHAN_TT = {"dang_lam": "Đang làm", "chua_lam": "Chưa làm", "treo": "Treo", "xong": "Xong"}
THU_TU_TT = ("dang_lam", "chua_lam", "treo", "xong")

# (tên sheet, [(tên cột, độ rộng, ý nghĩa)]) — THỨ TỰ NÀY LÀ FORMAT. Chỉ thêm vào cuối.
SHEETS = [
    ("Tong_quan", [
        ("Ma_chuong", 11, "Số chương đặc tả docs/ui-ux/NN-*.md"),
        ("Phan_he", 34, "Tên phân hệ, lấy từ tiêu đề đặc tả"),
        ("Tong_tuyen", 11, "Số tuyến API chương ấy khai trong hợp đồng"),
        ("Tuyen_web", 11, "Trong đó thuộc bề mặt web quản trị (mẫu số của Da_goi)"),
        ("Da_goi", 9, "Tuyến web đã có mã client gọi tới — ĐẾM được, không phải % hoàn thành"),
        ("Ngoai_web", 11, "Tuyến của kênh khác (Mini App công dân) — không phải thiếu sót"),
        ("Man_web", 10, "Có = đã có màn gọi · Chưa = có tuyến web mà chưa màn nào gọi · — = không có tuyến"),
        ("Dang_lam", 10, "Số hạng mục sổ gắn menu này đang làm"),
        ("Chua_lam", 10, "… chưa làm"),
        ("Treo", 8, "… treo (cố ý chưa làm)"),
        ("Xong", 8, "… xong"),
    ]),
    ("Menu_web", [
        ("STT", 6, "Thứ tự trong menu web-admin"),
        ("Muc_menu", 26, "Nhãn menu"),
        ("Duong_dan", 24, "Đường dẫn màn; trống = mục cố ý hiện mà chưa bấm được"),
        ("Khoa_quyen", 28, "Khoá quyền mở mục"),
        ("Co_man", 10, "Có · Thiếu (khai đường mà chưa có trang) · Chưa mở"),
        ("Chua_dung", 11, "Số phần chưa dựng màn đang hiện; 'không khai' KHÁC 0 — chưa ai nói màn thiếu gì"),
    ]),
    ("Hang_muc", [
        ("Ma", 40, "module/id — KHOÁ ỔN ĐỊNH, dùng để VLOOKUP ghi chú của team"),
        ("Module", 20, "Module kho mã"),
        ("Menu", 22, "Menu nghiệp vụ (slug docs/ui-ux), có thể nhiều"),
        ("Viec", 60, "Việc"),
        ("Trang_thai", 12, "Đang làm · Chưa làm · Treo · Xong"),
        ("No_cau_hoi", 12, "Số câu hỏi còn chờ khách chốt"),
        ("Buoc_ke_tiep", 70, "Bước kế tiếp"),
        ("Cap_nhat", 12, "Ngày module cập nhật sổ lần cuối"),
    ]),
    ("Cho_khach", [
        ("So_cau", 8, "Số câu trong kb/00-foundation/open-questions.json"),
        ("Trang_thai", 12, "Trạng thái câu hỏi"),
        ("Cau_hoi", 80, "Nội dung câu hỏi"),
        ("Dang_chan", 50, "Các hạng mục (Ma) đang chờ câu này"),
    ]),
    ("Thong_tin", [
        ("Khoa", 22, ""),
        ("Gia_tri", 90, ""),
    ]),
]

MAX_O = 32000     # Excel giữ tối đa 32 767 ký tự một ô


# ---------------------------------------------------------------------------------------------
# Dữ liệu cá nhân — CÙNG mẫu với pii_guard, bỏ cặp dấu nháy (ô là văn bản trần, không phải chuỗi mã)
def _loi(mau: str) -> str:
    return mau[len(r"[\"'`]"):-len(r"[\"'`]")]


MAU_CA_NHAN = re.compile(rf"(?<!\d)(?:{_loi(c.VN_PHONE)}|{_loi(c.VN_CCCD)})(?!\d)")


def co_du_lieu_ca_nhan(s: str) -> bool:
    return any(not c.SAFE_FAKE.match(m.group(0)) for m in MAU_CA_NHAN.finditer(s or ""))


# ---------------------------------------------------------------------------------------------
# Thu thập
def doc_so() -> list[tuple[str, dict]]:
    ra = []
    thu = os.path.join(ROOT, "kb", "90-ephemeral", "tien-do")
    for ten in sorted(os.listdir(thu)):
        if ten.endswith(".json"):
            d = json.loads(io.open(os.path.join(thu, ten), encoding="utf-8").read())
            ra.append((d.get("module", ten[:-5]), d))
    return ra


def menu_cua(x: dict) -> list[str]:
    v = x.get("menu")
    return [] if v is None else (v if isinstance(v, list) else [v])


def thu_thap() -> dict[str, list[list]]:
    chuong = sp.cac_chuong()
    theo_chuong, tong_tuyen, _kk = sp.tuyen_theo_chuong()
    xong, biet_web = sp.viec_da_xong()
    menu = sp.cac_muc_menu()
    if not chuong or tong_tuyen == 0 or not menu:
        raise RuntimeError(f"đọc hụt: {len(chuong)} chương · {tong_tuyen} tuyến · {len(menu)} menu")
    so = doc_so()

    # Hạng mục gắn `menu` → chương cùng slug (menu `nhiem-vu` ↔ chương `02-nhiem-vu`).
    dem_menu: dict[str, dict[str, int]] = {}
    for _m, d in so:
        for x in d.get("muc", []):
            for s in menu_cua(x):
                dem_menu.setdefault(s, {k: 0 for k in THU_TU_TT})
                if x.get("trang_thai") in THU_TU_TT:
                    dem_menu[s][x["trang_thai"]] += 1

    tq = []
    for r in sp.dong_chuong(chuong, theo_chuong, xong, biet_web):
        dm = dem_menu.get(r["slug"].split("-", 1)[1], {k: 0 for k in THU_TU_TT})
        man = "Có" if r["da_goi"] else ("Chưa" if r["web"] else "—")
        tq.append([r["so"], r["ten"], r["tuyen"], r["web"], r["da_goi"], r["ngoai_web"], man,
                   dm["dang_lam"], dm["chua_lam"], dm["treo"], dm["xong"]])

    mw = []
    for r in sp.dong_menu(menu):
        if r["duong"] is None:
            mw.append([r["stt"], r["nhan"], "", "", "Chưa mở", ""])
            continue
        mw.append([r["stt"], r["nhan"], r["duong"], r["khoa"] or "",
                   "Có" if r["co_man"] else "Thiếu",
                   r["chua_dung"] if r["chua_dung"] is not None else "không khai"])

    hm = []
    chan: dict[int, list[str]] = {}
    for mod, d in so:
        for x in sorted(d.get("muc", []), key=lambda y: THU_TU_TT.index(y.get("trang_thai"))
                        if y.get("trang_thai") in THU_TU_TT else 9):
            ma = f"{mod}/{x.get('id', '?')}"
            nc = [int(q) for q in (x.get("no_confirm") or []) if str(q).isdigit()]
            for q in nc:
                chan.setdefault(q, []).append(ma)
            hm.append([ma, mod, ", ".join(menu_cua(x)), x.get("viec", ""),
                       NHAN_TT.get(x.get("trang_thai", ""), x.get("trang_thai", "")),
                       " ".join(f"#{q}" for q in nc), x.get("tiep_theo", "") or "",
                       d.get("cap_nhat", "")])

    ck = []
    try:
        cau = json.loads(io.open(os.path.join(ROOT, "kb", "00-foundation", "open-questions.json"),
                                 encoding="utf-8").read())
    except Exception:
        cau = []
    for q in cau:
        qi = int(q.get("id", 0))
        if q.get("status") != "DECIDED" or qi in chan:
            ck.append([qi, q.get("status", ""), q.get("question", ""), ", ".join(chan.get(qi, []))])

    commit = ""
    try:
        commit = subprocess.run(["git", "rev-parse", "--short", "HEAD"], capture_output=True,
                                text=True, cwd=ROOT, timeout=10).stdout.strip()
    except Exception:
        pass
    tt = [["Ngay_xuat", datetime.datetime.now().strftime("%Y-%m-%d %H:%M")],
          ["Commit", commit],
          ["Phien_ban_format", PHIEN_BAN_FORMAT],
          ["Nguon", "tools/xuat_tien_do.py — sinh từ mã và sổ tiến độ, không ô nào viết tay"],
          ["Cach_cap_nhat", "File → Import → Tải lên → Replace spreadsheet. Ghi chú của team để ở "
                            "sheet RIÊNG, tra theo cột Ma của Hang_muc — Replace xoá cột tự thêm"],
          ["Luu_y", "Da_goi KHÔNG phải % hoàn thành: một chương đủ tuyến vẫn có thể còn phần đặc "
                    "tả chưa có tuyến nào. 'không khai' ở Chua_dung KHÁC 0"],
          ["", ""]]
    for ten, cot in SHEETS[:-1]:
        for ten_cot, _w, y in cot:
            tt.append([f"{ten}.{ten_cot}", y])
    return {"Tong_quan": tq, "Menu_web": mw, "Hang_muc": hm, "Cho_khach": ck, "Thong_tin": tt}


# ---------------------------------------------------------------------------------------------
# Ghi .xlsx — tập con tối thiểu của SpreadsheetML
_XML_BAN = re.compile(r"[\x00-\x08\x0b\x0c\x0e-\x1f]")


def ten_cot(i: int) -> str:
    s = ""
    i += 1
    while i:
        i, r = divmod(i - 1, 26)
        s = chr(65 + r) + s
    return s


def o_xml(ref: str, v, kieu: int) -> str:
    if isinstance(v, (int, float)) and not isinstance(v, bool):
        return f'<c r="{ref}" s="{kieu}"><v>{v}</v></c>'
    s = _XML_BAN.sub("", str(v))[:MAX_O]
    return (f'<c r="{ref}" s="{kieu}" t="inlineStr"><is><t xml:space="preserve">'
            f'{escape(s)}</t></is></c>')


def sheet_xml(cot: list[tuple], dong: list[list], chon_tt: int | None) -> str:
    n = len(dong) + 1
    cuoi = ten_cot(len(cot) - 1)
    x = ['<?xml version="1.0" encoding="UTF-8" standalone="yes"?>',
         '<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">',
         '<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" '
         'activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>',
         "<cols>" + "".join(f'<col min="{i+1}" max="{i+1}" width="{w}" customWidth="1"/>'
                            for i, (_t, w, _y) in enumerate(cot)) + "</cols>",
         "<sheetData>",
         '<row r="1">' + "".join(o_xml(f"{ten_cot(i)}1", t, 1) for i, (t, _w, _y) in
                                 enumerate(cot)) + "</row>"]
    for r, d in enumerate(dong, 2):
        x.append(f'<row r="{r}">' + "".join(o_xml(f"{ten_cot(i)}{r}", v, 2)
                                            for i, v in enumerate(d)) + "</row>")
    x.append("</sheetData>")
    x.append(f'<autoFilter ref="A1:{cuoi}{n}"/>')
    if chon_tt is not None:
        col = ten_cot(chon_tt)
        ds = ",".join(NHAN_TT[k] for k in THU_TU_TT)
        x.append(f'<dataValidations count="1"><dataValidation type="list" allowBlank="1" '
                 f'sqref="{col}2:{col}{max(n, 2)}"><formula1>"{escape(ds)}"</formula1>'
                 f"</dataValidation></dataValidations>")
    x.append("</worksheet>")
    return "".join(x)


STYLES = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
          '<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">'
          '<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font>'
          '<font><b/><sz val="11"/><name val="Calibri"/></font></fonts>'
          '<fills count="3"><fill><patternFill patternType="none"/></fill>'
          '<fill><patternFill patternType="gray125"/></fill>'
          '<fill><patternFill patternType="solid"><fgColor rgb="FFD9E1F2"/></patternFill></fill>'
          '</fills>'
          '<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>'
          '<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>'
          '<cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>'
          '<xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1"/>'
          '<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1">'
          '<alignment vertical="top" wrapText="1"/></xf></cellXfs>'
          # Kiểu "Normal" mặc định: thiếu nó, openpyxl cảnh báo và một số trình đọc tự bù — tệp
          # đi vào Google Sheet của cả team thì không để trình đọc phải đoán.
          '<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>'
          "</styleSheet>")


def ghi_xlsx(duong: str, du_lieu: dict[str, list[list]]) -> None:
    ten = [t for t, _c in SHEETS]
    tep = {
        "[Content_Types].xml":
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
            '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
            '<Default Extension="xml" ContentType="application/xml"/>'
            '<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>'
            '<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>'
            + "".join(f'<Override PartName="/xl/worksheets/sheet{i}.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>'
                      for i in range(1, len(ten) + 1)) + "</Types>",
        "_rels/.rels":
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
            '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>'
            "</Relationships>",
        "xl/workbook.xml":
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" '
            'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>'
            + "".join(f'<sheet name="{t}" sheetId="{i}" r:id="rId{i}"/>' for i, t in enumerate(ten, 1))
            + "</sheets>"
            + "<definedNames>" + "".join(
                f'<definedName name="_xlnm._FilterDatabase" localSheetId="{i}" hidden="1">'
                f"'{t}'!$A$1:${ten_cot(len(c_) - 1)}${len(du_lieu[t]) + 1}</definedName>"
                for i, (t, c_) in enumerate(SHEETS)) + "</definedNames></workbook>",
        "xl/_rels/workbook.xml.rels":
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
            + "".join(f'<Relationship Id="rId{i}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet{i}.xml"/>'
                      for i in range(1, len(ten) + 1))
            + f'<Relationship Id="rId{len(ten) + 1}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>'
            "</Relationships>",
        "xl/styles.xml": STYLES,
    }
    for i, (t, cot) in enumerate(SHEETS, 1):
        chon = next((j for j, (tc, _w, _y) in enumerate(cot) if tc == "Trang_thai"), None) \
            if t == "Hang_muc" else None
        tep[f"xl/worksheets/sheet{i}.xml"] = sheet_xml(cot, du_lieu[t], chon)

    os.makedirs(os.path.dirname(duong) or ".", exist_ok=True)
    with zipfile.ZipFile(duong, "w", zipfile.ZIP_DEFLATED) as z:
        for ten_tep, noi_dung in tep.items():
            # Mốc thời gian cố định: hai lần xuất cùng dữ liệu ra cùng một tệp, từng byte.
            zi = zipfile.ZipInfo(ten_tep, date_time=(1980, 1, 1, 0, 0, 0))
            zi.compress_type = zipfile.ZIP_DEFLATED
            z.writestr(zi, noi_dung.encode("utf-8"))


# ---------------------------------------------------------------------------------------------
def git_bo_qua(duong: str) -> bool | None:
    """True = git bỏ qua đường ấy · False = sẽ vào git · None = nằm ngoài kho / không có git."""
    tuyet_doi = os.path.abspath(duong)
    try:
        if os.path.commonpath([tuyet_doi, ROOT]) != ROOT:
            return None
    except ValueError:
        # Windows: khác ổ đĩa (Downloads ở C:, kho ở D:) — chắc chắn nằm ngoài kho.
        return None
    try:
        r = subprocess.run(["git", "check-ignore", "-q", tuyet_doi], cwd=ROOT, timeout=10)
    except Exception:
        return None
    return r.returncode == 0


def main(argv: list[str]) -> int:
    duong = argv[0] if argv else os.path.join(
        ROOT, "tmp", "tien-do", f"vigov-tien-do-{datetime.date.today().isoformat()}.xlsx")
    if not duong.lower().endswith(".xlsx"):
        duong += ".xlsx"
    if git_bo_qua(duong) is False:
        print(f"[xuat_tien_do] ĐỎ — {duong} nằm trong kho và git KHÔNG bỏ qua nó. Tệp nhị phân "
              "sinh ra không được vào git. Để mặc định (tmp/) hoặc một đường ngoài kho.",
              file=sys.stderr)
        return 4
    try:
        du_lieu = thu_thap()
    except Exception as ex:
        print(f"[xuat_tien_do] ĐỎ — {ex}. Chạy `make kb` trước.", file=sys.stderr)
        return 1

    trung = [f"{t}!{ten_cot(j)}{i + 2}"
             for t, dong in du_lieu.items() for i, d in enumerate(dong)
             for j, v in enumerate(d) if isinstance(v, str) and co_du_lieu_ca_nhan(v)]
    if trung:
        print("[xuat_tien_do] ĐỎ — ô trông như số điện thoại / CCCD thật, KHÔNG ghi tệp (luật 3):",
              file=sys.stderr)
        print("  " + ", ".join(trung[:20]) + (" …" if len(trung) > 20 else ""), file=sys.stderr)
        print("  Sửa nguồn (thường là một mục sổ tiến độ) — không bao giờ in giá trị ra đây.",
              file=sys.stderr)
        return 3

    ghi_xlsx(duong, du_lieu)
    print(f"[xuat_tien_do] {os.path.relpath(duong, ROOT) if git_bo_qua(duong) is not None else duong}"
          f" — format v{PHIEN_BAN_FORMAT} · " + " · ".join(
              f"{t} {len(du_lieu[t])}" for t, _c in SHEETS))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
