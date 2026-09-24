#!/usr/bin/env python3
"""Xuất tiến độ dự án ra MỘT tệp .xlsx ba sheet, viết bằng lời cho CHỦ DỰ ÁN đọc.

Chạy:  python tools/xuat_tien_do.py [đường-dẫn.xlsx]
       python tools/tien_do_san_pham.py --excel [đường-dẫn.xlsx]      (cùng một việc)
Mặc định: tmp/tien-do/vigov-tien-do-<YYYY-MM-DD>.xlsx  (tmp/ nằm trong .gitignore)
Mã thoát: 0 = ghi xong · 1 = thiếu nguồn / hai phép đếm lệch nhau · 3 = có ô trông như dữ liệu
          cá nhân, KHÔNG ghi tệp · 4 = đường dẫn nằm trong kho mà git không bỏ qua

NGƯỜI ĐỌC VÀ CÂU HỎI (format v2, 24/09/2026): chủ dự án mở tệp để biết *"dự án đang ở giai đoạn
nào, còn những gì, khi nào xong"*. Bản v1 trả lời bằng năm sheet số đếm thô cho người dựng công
thức; người cần hình dung thì không đọc ra được. Nên v2 có ba sheet, cột viết bằng lời:

    Tổng quan      giai đoạn · từng khối lớn · tốc độ đo được · cách tính
    Chức năng      mỗi chương đặc tả: hiện trạng · còn gì · % · dự kiến
    Việc còn lại   mỗi hạng mục sổ chưa xong — cột `Mã` (`module/id`) là khoá ổn định

CỘT % CÓ Ở ĐÂY, VÀ NÓ LÀ MỘT PHÉP ĐẾM, không phải một ước lượng. Bản .md vẫn không có cột ấy vì
lý do nó ghi (23/09/2026: một tỷ lệ đếm mục sổ làm chủ dự án tưởng xong hết). Người dùng quyết
24/09/2026 thêm % vào bản Excel với điều kiện truy ngược được. Nên mỗi dòng in kèm cột `Cách tính %`
nêu từng số hạng, và công thức chọn phía BẢO THỦ:
  * phần xong   = màn web đã có trang + API có mã web gọi + API công dân Mini App đã gọi
                  + việc sổ `xong`
  * phần còn    = màn web chưa có + API chưa ai gọi + phần màn tự khai chưa dựng
                  (`PHAN_CHUA_DUNG*`) + việc sổ đang làm / chưa làm / TREO
  TREO NẰM TRONG MẪU SỐ: loại nó ra thì % tăng đúng bằng những việc chưa ai làm — lặp lại chính
  lỗi 23/09. Chương chưa tách được phần nào ghi "chưa khởi công", KHÔNG ghi 0/0 = 100%.

DỰ KIẾN = phần còn ÷ TỐC ĐỘ ĐO ĐƯỢC. Tốc độ lấy từ git: số việc sổ chuyển sang `xong` trong 7 ngày
lịch gần nhất, chia số ngày làm việc (thứ Hai–Sáu). Giới hạn in ngay trong sheet: đơn vị "phần"
không đều nhau, việc ghi vào sổ ở trạng thái xong luôn làm tốc độ cao lên, không trừ ngày lễ,
và chức năng chưa khởi công không có phần nào để chia.

KHÔNG MỘT Ô NÀO VIẾT TAY: mọi số đọc từ CÙNG các hàm sinh `tien-do-san-pham.md`
(`tien_do_san_pham.*`) và từ sổ `kb/90-ephemeral/tien-do/*.json`.

TÊN SHEET VÀ TÊN CỘT LÀ FORMAT: đổi thì tăng PHIEN_BAN_FORMAT và cập nhật `FORMAT_XLSX` trong
`tools/test_hooks.py` — ca ấy đỏ đúng để không ai đổi format mà quên báo team.

VÌ SAO THƯ VIỆN CHUẨN, không openpyxl: mọi công cụ của kho chỉ dùng thư viện chuẩn, và máy khác
hay Jenkins có thể không có gói ấy. Một .xlsx chỉ là một zip các tệp XML.

TRƯỚC KHI GHI, mọi ô chữ được soi bằng CÙNG mẫu số điện thoại / CCCD của `pii_guard`. Một Google
Sheet chia sẻ cho cả team là một kênh ra ngoài — lớp chặn thứ hai đặt đúng ở cửa ra. Trúng thì
từ chối và chỉ in TOẠ ĐỘ ô, không bao giờ in giá trị.
"""

from __future__ import annotations

import datetime
import io
import json
import math
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

PHIEN_BAN_FORMAT = 2

NHAN_TT = {"dang_lam": "Đang làm", "chua_lam": "Chưa làm", "treo": "Tạm dừng", "xong": "Xong"}
THU_TU_TT = ("dang_lam", "chua_lam", "treo", "xong")
SO_TIEN_DO = os.path.join("kb", "90-ephemeral", "tien-do")
CUA_SO_TOC_DO = 7   # ngày lịch — đủ dài để một ngày nghỉ không kéo tốc độ về 0

# (tên sheet, [(tên cột, độ rộng)]) — THỨ TỰ NÀY LÀ FORMAT.
SHEETS = [
    ("Tổng quan", [("Hạng mục", 30), ("Hiện trạng", 60), ("Còn nội dung gì", 70),
                   ("% hoàn thành", 13), ("Dự kiến xong", 40)]),
    ("Chức năng", [("Mã", 6), ("Chức năng", 30), ("Hiện trạng", 50), ("Còn nội dung gì", 80),
                   ("% hoàn thành", 13), ("Dự kiến xong", 40), ("Cách tính %", 50)]),
    ("Việc còn lại", [("Mã", 36), ("Thuộc", 28), ("Việc", 60), ("Tình trạng", 12),
                      ("Bước kế tiếp", 90)]),
]

MAX_O = 32000     # Excel giữ tối đa 32 767 ký tự một ô


class PhanTram(float):
    """Ô phần trăm (0..1) — ghi thành số với định dạng `0%`, để Google Sheet sắp xếp được."""


# ---------------------------------------------------------------------------------------------
# Dữ liệu cá nhân — CÙNG mẫu với pii_guard, bỏ cặp dấu nháy (ô là văn bản trần, không phải chuỗi mã)
def _loi(mau: str) -> str:
    return mau[len(r"[\"'`]"):-len(r"[\"'`]")]


MAU_CA_NHAN = re.compile(rf"(?<!\d)(?:{_loi(c.VN_PHONE)}|{_loi(c.VN_CCCD)})(?!\d)")


def co_du_lieu_ca_nhan(s: str) -> bool:
    return any(not c.SAFE_FAKE.match(m.group(0)) for m in MAU_CA_NHAN.finditer(s or ""))


# ---------------------------------------------------------------------------------------------
# Lời văn
def cau(s: str, toi_da: int = 220) -> str:
    """Bỏ ký hiệu markdown, gộp khoảng trắng, cắt ở ranh giới câu gần nhất.

    Sổ tiến độ viết cho phiên sau đọc — có backtick, có in đậm, có bước kế tiếp dài cả nghìn chữ.
    Chủ dự án cần câu đầu; phần còn lại vẫn nguyên trong sổ, tra theo cột `Mã`.
    """
    s = re.sub(r"\s+", " ", re.sub(r"[`*]", "", s or "")).strip()
    if len(s) <= toi_da:
        return s
    cat = s[:toi_da]
    i = max(cat.rfind(". "), cat.rfind("; "), cat.rfind(" — "))
    return (cat[:i + 1] if i > toi_da // 2 else cat.rstrip()) + " …"


def danh_sach(dong: list[str]) -> str:
    return "\n".join(f"• {d}" for d in dong)


# ---------------------------------------------------------------------------------------------
# Thời gian
def cong_ngay_lam_viec(d: datetime.date, n: int) -> datetime.date:
    while n > 0:
        d += datetime.timedelta(days=1)
        if d.weekday() < 5:
            n -= 1
    return d


def so_ngay_lam_viec(tu: datetime.date, den: datetime.date) -> int:
    return sum(1 for i in range(1, (den - tu).days + 1)
               if (tu + datetime.timedelta(days=i)).weekday() < 5)


def git(*a: str) -> str:
    try:
        return subprocess.run(["git", *a], capture_output=True, text=True, encoding="utf-8",
                              cwd=ROOT, timeout=20).stdout
    except Exception:
        return ""


def trang_thai_theo_ma(ds: list[dict]) -> dict[str, str]:
    return {f"{d.get('module')}/{x.get('id')}": x.get("trang_thai", "")
            for d in ds for x in d.get("muc", [])}


def toc_do(so_hien: list[dict], hom_nay: datetime.date) -> dict:
    """Việc sổ chuyển sang `xong` mỗi ngày làm việc, đo từ git. `v` = None khi không đo được.

    CHỈ ĐẾM CHUYỂN TRẠNG THÁI THẬT: việc đã có trong sổ ở mốc cũ, lúc ấy chưa xong, nay xong.
    Đếm hiệu số `xong` thô thì mọi việc được GHI BÙ vào sổ ở trạng thái xong cũng thành tốc độ —
    đo 24/09/2026 ra 26 việc/ngày và mốc "xong sau 7 ngày", đúng loại con số làm chủ dự án tưởng
    xong hết (23/09/2026).
    """
    thu = SO_TIEN_DO.replace(os.sep, "/")

    def so_tai(rev: str) -> list[dict]:
        ra = []
        for ten in git("ls-tree", "--name-only", rev, thu + "/").split():
            if ten.endswith(".json"):
                try:
                    d = json.loads(git("show", f"{rev}:{ten}"))
                    d.setdefault("module", os.path.basename(ten)[:-5])
                    ra.append(d)
                except Exception:
                    pass
        return ra

    moc = (hom_nay - datetime.timedelta(days=CUA_SO_TOC_DO)).isoformat()
    rev = git("rev-list", "-1", f"--before={moc}T23:59:59", "HEAD").strip()
    cu = so_tai(rev) if rev else []
    if not cu:
        # Sổ trẻ hơn cửa sổ: mốc cũ chưa có sổ, so với nó thì mọi việc đều "mới" và tốc độ ra 0.
        # Đo từ commit ĐẦU TIÊN có sổ — ô tốc độ in ngày mốc, nên người đọc thấy cửa sổ ngắn hơn.
        rev = git("log", "--reverse", "--format=%H", "--", thu).split("\n", 1)[0].strip()
        cu = so_tai(rev) if rev else []
    if not cu:
        return {"v": None, "ly_do": "không đọc được lịch sử git của sổ tiến độ"}
    ngay = datetime.date.fromisoformat(git("show", "-s", "--format=%cs", rev).strip())
    nlv = so_ngay_lam_viec(ngay, hom_nay)
    truoc, nay = trang_thai_theo_ma(cu), trang_thai_theo_ma(so_hien)
    tang = sum(1 for ma, tt in truoc.items() if tt != "xong" and nay.get(ma) == "xong")
    if nlv <= 0 or tang <= 0:
        return {"v": None, "ly_do": f"từ {ngay:%d/%m/%Y} tới nay không có việc nào chuyển sang Xong "
                                    f"({nlv} ngày làm việc)"}
    return {"v": tang / nlv, "tang": tang, "nlv": nlv, "tu": ngay, "rev": rev[:7]}


def du_kien(con: int, td: dict, hom_nay: datetime.date, rieng: bool) -> str:
    if con == 0:
        return "Không còn phần nào được ghi nhận"
    if td["v"] is None:
        return "Chưa đo được tốc độ — " + td["ly_do"]
    n = max(1, math.ceil(con / td["v"]))
    ngay = cong_ngay_lam_viec(hom_nay, n)
    if rieng:
        return f"≈ {n} ngày làm việc — khoảng {ngay:%d/%m/%Y} nếu dồn sức riêng cho phần này"
    return f"Khoảng {ngay:%d/%m/%Y} (≈ {n} ngày làm việc ở tốc độ hiện tại)"


# ---------------------------------------------------------------------------------------------
# Thu thập
def doc_so() -> list[dict]:
    ra = []
    thu = os.path.join(ROOT, SO_TIEN_DO)
    for ten in sorted(os.listdir(thu)):
        if ten.endswith(".json"):
            d = json.loads(io.open(os.path.join(thu, ten), encoding="utf-8").read())
            d.setdefault("module", ten[:-5])
            ra.append(d)
    return ra


def menu_cua(x: dict) -> list[str]:
    v = x.get("menu")
    return [] if v is None else (v if isinstance(v, list) else [v])


# Nhóm cho hạng mục sổ KHÔNG gắn menu nào — phần nền, hạ tầng, quy trình. Module lạ in đúng tên
# module của nó: một nhóm "Khác" là chỗ một module mới biến mất khỏi tổng quan mà không ai thấy.
NHOM = [
    ("Backend API — phần nền", lambda m: m in ("core", "proto") or m.startswith("service-")),
    ("Web quản trị — phần nền", lambda m: m == "web-admin"),
    ("Web quản trị nền tảng", lambda m: m == "platform-admin"),
    ("Mini App công dân", lambda m: m == "citizen-app"),
    ("Hạ tầng & triển khai", lambda m: m == "deploy"),
    ("Quy trình & công cụ dự án", lambda m: m in ("_chung", "tools")),
]


def nhom_cua(module: str) -> str:
    return next((t for t, f in NHOM if f(module)), module)


def tu(s: str) -> list[str]:
    return [g for g in (sp.gon(w) for w in re.split(r"[\s&–—-]+", s)) if g]


def ghep_menu(chuong: list[dict], menu: list[dict]) -> dict[str, dict]:
    """slug chương → dòng menu. Khớp khi MỌI từ của nhãn menu có trong tiêu đề chương; hoà thì
    chọn tiêu đề ít từ thừa nhất ("Tổng quan" → 01 "Tổng quan điều hành", không phải 00).

    Không khớp theo thứ tự: menu và chương tình cờ cùng thứ tự hôm nay, và một mục chèn thêm là
    mọi dòng sau lệch một bậc mà không ô nào báo.
    """
    ra: dict[str, dict] = {}
    for m in menu:
        nhan = set(tu(m["nhan"]))
        ung = [(len(set(tu(r["ten"])) - nhan), r["slug"]) for r in chuong
               if nhan and nhan <= set(tu(r["ten"]))]
        if ung:
            ung.sort()
            if len(ung) > 1 and ung[0][0] == ung[1][0]:
                raise RuntimeError(f"mục menu {m['nhan']!r} khớp hai chương như nhau: "
                                   f"{ung[0][1]}, {ung[1][1]}")
            if ung[0][1] in ra:
                raise RuntimeError(f"hai mục menu cùng khớp chương {ung[0][1]}")
            ra[ung[0][1]] = m
    return ra


def thu_thap() -> dict[str, list[list]]:
    hom_nay = datetime.date.today()
    chuong_goc = sp.cac_chuong()
    theo_chuong, tong_tuyen, _kk = sp.tuyen_theo_chuong()
    xong, biet_web = sp.viec_da_xong()
    menu_goc = sp.cac_muc_menu()
    if not chuong_goc or tong_tuyen == 0 or not menu_goc:
        raise RuntimeError(f"đọc hụt: {len(chuong_goc)} chương · {tong_tuyen} tuyến · "
                           f"{len(menu_goc)} menu")
    chuong = sp.dong_chuong(chuong_goc, theo_chuong, xong, biet_web)
    menu = sp.dong_menu(menu_goc)
    app_goi = set(sp.tuyen_app_goi())
    so = doc_so()
    td = toc_do(so, hom_nay)
    menu_theo_chuong = ghep_menu(chuong, menu)
    slug_ngan = {r["slug"].split("-", 1)[1]: r["slug"] for r in chuong}

    # Mỗi hạng mục sổ được đếm ĐÚNG MỘT LẦN: vào chương của menu đầu tiên nó gắn, hoặc vào nhóm
    # của module nó. Đếm hai lần là % toàn dự án lệch theo số menu một việc gắn vào.
    viec_chuong: dict[str, list[tuple[str, dict]]] = {}
    viec_nhom: dict[str, list[tuple[str, dict]]] = {}
    thuoc: dict[str, str] = {}
    ten_chuong = {r["slug"]: f"{r['so']} · {r['ten']}" for r in chuong}
    for d in so:
        mod = d["module"]
        for x in d.get("muc", []):
            ma = f"{mod}/{x.get('id', '?')}"
            slug = next((slug_ngan[s] for s in menu_cua(x) if s in slug_ngan), None)
            if slug:
                viec_chuong.setdefault(slug, []).append((ma, x))
                thuoc[ma] = ten_chuong[slug]
            else:
                viec_nhom.setdefault(nhom_cua(mod), []).append((ma, x))
                thuoc[ma] = nhom_cua(mod)

    # ---- Chức năng --------------------------------------------------------------------------
    cn: list[list] = []
    tong = {"xong": 0, "con": 0, "treo": 0}
    co_man, chua_kc = [], []
    for r in chuong:
        m = menu_theo_chuong.get(r["slug"])
        viec = viec_chuong.get(r["slug"], [])
        dem = {k: sum(1 for _ma, x in viec if x.get("trang_thai") == k) for k in THU_TU_TT}
        cong_dan = [t for t in theo_chuong.get(r["slug"], []) if sp.la_cong_dan(t)]
        app_xong = sum(1 for t in cong_dan if t["duong"] in app_goi)
        pcd: list[str] | None = None
        if m and m["duong"] is not None:
            khai = [p for p in (sp.phan_chua_dung(f) for f in sp.feature_cua_trang(m["duong"]))
                    if p is not None]
            pcd = [x for p in khai for x in p] if khai else None
            # Hai phép đếm trên cùng một nguồn phải ra một số — lệch là bản Excel in một danh
            # sách khác với con số `.md` đang báo. Từ chối thay vì chọn một bên.
            so_md = m["chua_dung"]
            if (so_md is None) != (pcd is None) or (pcd is not None and len(pcd) != so_md):
                raise RuntimeError(f"phần chưa dựng của {m['nhan']!r}: .md đếm {so_md}, "
                                   f"Excel đọc {None if pcd is None else len(pcd)}")
        api_con = r["web"] - r["da_goi"]
        app_con = len(cong_dan) - app_xong
        # Màn web là MỘT PHẦN của chức năng có mục menu. Thiếu phần này, chương 10 (một API đã có
        # mã gọi, màn chưa bấm được) in 100% — đo 24/09/2026.
        man_xong = 1 if m and m["duong"] is not None and m["co_man"] else 0
        man_con = 1 if m and not man_xong else 0
        n_xong = man_xong + r["da_goi"] + app_xong + dem["xong"]
        n_con = (man_con + api_con + app_con + len(pcd or [])
                 + dem["dang_lam"] + dem["chua_lam"] + dem["treo"])
        chua_tach = r["tuyen"] == 0 and not viec and not pcd

        if not m and not r["tuyen"] and not viec:
            cn.append([r["so"], r["ten"], "Chương giới thiệu của bản đặc tả, không phải một chức năng",
                       "", "", "", "Không tính vào % dự án"])
            continue

        ht = []
        if m is None:
            ht.append("không có mục menu web")
        elif m["duong"] is None:
            ht.append("chưa có màn web (mục menu hiện nhưng chưa bấm được)")
        elif m["co_man"]:
            ht.append("đã có màn web")
            co_man.append(r["ten"])
        else:
            ht.append("menu khai đường dẫn nhưng chưa có trang")
        if r["web"]:
            ht.append(f"mã web đã gọi {r['da_goi']}/{r['web']} API")
        elif not r["tuyen"]:
            ht.append("chưa có API nào")
        if cong_dan:
            ht.append(f"Mini App công dân đã gọi {app_xong}/{len(cong_dan)} API dành cho người dân")
        if viec:
            ht.append("sổ tiến độ: " + ", ".join(f"{dem[k]} {NHAN_TT[k].lower()}"
                                                   for k in THU_TU_TT if dem[k]))
        hien_trang = "; ".join(ht)
        hien_trang = hien_trang[0].upper() + hien_trang[1:]

        con = ["Màn web của chức năng này chưa dựng"] if man_con else []
        con += [cau(p, 160) for p in (pcd or [])]
        con += [f"{cau(x.get('viec', ''), 160)} ({NHAN_TT.get(x.get('trang_thai'), '?').lower()})"
                for _ma, x in viec if x.get("trang_thai") != "xong"]
        if api_con:
            con.append(f"{api_con} API máy chủ đã có nhưng chưa có màn web gọi tới")
        if app_con:
            con.append(f"{app_con} API dành cho người dân mà Mini App chưa gọi")
        if m and m["duong"] is not None and pcd is None:
            con.append("Màn này CHƯA KHAI phần nào còn thiếu so với đặc tả — cần rà lại; "
                       "% bên cạnh có thể cao hơn thực tế")

        if chua_tach:
            chua_kc.append(r["ten"])
            cn.append([r["so"], r["ten"], hien_trang + ". Chưa khởi công",
                       "Toàn bộ chương — chưa tách thành phần việc nào",
                       PhanTram(0), "Chưa ước được: chưa có phần việc nào để đo",
                       "Chưa có phần nào được tách ra — ghi 0%, không phải 0/0"])
            continue
        tong["xong"] += n_xong
        tong["con"] += n_con
        tong["treo"] += dem["treo"]
        cach = (f"{n_xong} xong / {n_xong + n_con} phần. Xong = "
                + (f"{man_xong} màn web + " if m else "")
                + f"{r['da_goi']} API có mã web gọi"
                + (f" + {app_xong} API Mini App đã gọi" if cong_dan else "")
                + f" + {dem['xong']} việc sổ xong. Còn = "
                + (f"{man_con} màn web + " if m else "")
                + f"{api_con} API chưa có mã web gọi"
                + (f" + {app_con} API Mini App chưa gọi" if cong_dan else "")
                + f" + {len(pcd or [])} phần màn chưa dựng + "
                  f"{dem['dang_lam'] + dem['chua_lam'] + dem['treo']} việc sổ chưa xong")
        cn.append([r["so"], r["ten"], hien_trang, danh_sach(con) or "Không còn phần nào được ghi nhận",
                   PhanTram(n_xong / (n_xong + n_con)), du_kien(n_con, td, hom_nay, True), cach])

    # ---- Tổng quan --------------------------------------------------------------------------
    tq: list[list] = []
    nhom_rows = []
    for ten_nhom, _f in NHOM + [(k, None) for k in viec_nhom if k not in {t for t, _ in NHOM}]:
        viec = viec_nhom.get(ten_nhom, [])
        if not viec:
            continue
        dem = {k: sum(1 for _ma, x in viec if x.get("trang_thai") == k) for k in THU_TU_TT}
        n_con = dem["dang_lam"] + dem["chua_lam"] + dem["treo"]
        tong["xong"] += dem["xong"]
        tong["con"] += n_con
        tong["treo"] += dem["treo"]
        mo = [x for _ma, x in viec if x.get("trang_thai") != "xong"]
        con = [cau(x.get("viec", ""), 140) for x in mo[:8]]
        if len(mo) > 8:
            con.append(f"… và {len(mo) - 8} việc nữa — xem sheet Việc còn lại")
        nhom_rows.append([ten_nhom,
                          f"{dem['xong']} việc xong, {dem['dang_lam']} đang làm, "
                          f"{dem['chua_lam']} chưa làm, {dem['treo']} tạm dừng",
                          danh_sach(con) or "Không còn việc nào trong sổ",
                          PhanTram(dem["xong"] / len(viec)),
                          du_kien(n_con, td, hom_nay, True)])

    n_cn = sum(1 for d in cn if d[4] != "")
    tong_phan = tong["xong"] + tong["con"]
    giai_doan = ("Đang ở giai đoạn THI CÔNG (lập trình). " if tong["con"] else
                 "Phần thi công đo được đã xong. ")
    tq.append([
        "TOÀN DỰ ÁN",
        giai_doan + f"{len(co_man)}/{n_cn} chức năng đã có màn web dùng được"
        + (f"; {len(chua_kc)} chức năng chưa khởi công ({', '.join(chua_kc)})" if chua_kc else "")
        + ".",
        f"{tong['con']} phần việc còn lại, trong đó {tong['treo']} đang tạm dừng chờ điều kiện. "
        "Sau thi công còn: triển khai lên hạ tầng thật, nộp và chờ Zalo duyệt Mini App, kiểm thử "
        "nghiệm thu với khách, bàn giao (estimate §5–§8).",
        PhanTram(tong["xong"] / tong_phan) if tong_phan else PhanTram(0),
        du_kien(tong["con"], td, hom_nay, False)
        + (f". CHƯA tính {len(chua_kc)} chức năng chưa khởi công" if chua_kc else "")
        + " và các bước sau thi công",
    ])
    cn_xong = sum(1 for d in cn if isinstance(d[4], PhanTram) and d[4] >= 1)
    cn_phan = [(d[4], d) for d in cn if isinstance(d[4], PhanTram)]
    tq.append([
        "Chức năng nghiệp vụ (chi tiết ở sheet Chức năng)",
        f"{n_cn} chức năng: {cn_xong} đạt 100%, {len(co_man)} đã có màn web, "
        f"{len(chua_kc)} chưa khởi công",
        danh_sach([f"{d[1]}: {round(p * 100)}%" for p, d in cn_phan]),
        "", "",
    ])
    tq.extend(nhom_rows)

    tt_td = (f"{td['v']:.1f} việc sổ chuyển sang Xong mỗi ngày làm việc — {td['tang']} việc "
             f"đã có trong sổ mà chưa xong, nay xong, từ "
             f"{td['tu']:%d/%m/%Y} (commit {td['rev']}) tới hôm nay, {td['nlv']} ngày làm việc"
             if td["v"] is not None else "Chưa đo được — " + td["ly_do"])
    commit = git("rev-parse", "--short", "HEAD").strip()
    # Số liệu đọc từ CÂY LÀM VIỆC, không từ commit — cây bẩn thì nói ra (đo 24/09/2026, khi một
    # tệp done/ sinh sai của phiên song song làm lệch một ô).
    ban = git("status", "--porcelain").splitlines()
    if ban:
        commit += f" + {len(ban)} tệp chưa commit — số liệu MỚI HƠN commit này"
    tq += [["", "", "", "", ""],
           ["CÁCH ĐỌC", "", "", "", ""],
           ["Ngày xuất", datetime.datetime.now().strftime("%d/%m/%Y %H:%M"), "", "", ""],
           ["Commit", commit, "", "", ""],
           ["Tốc độ đo được", tt_td, "", "", ""],
           ["Cách tính %", "Phần xong ÷ tổng phần. Phần xong = màn web đã có + API có mã web gọi "
            "+ API Mini App đã gọi + việc sổ Xong. Phần còn = màn web chưa có + API chưa ai gọi + "
            "phần màn tự khai chưa dựng + việc sổ đang làm, chưa làm VÀ tạm dừng. Mỗi dòng sheet "
            "Chức năng in từng số hạng ở cột Cách tính %", "", "", ""],
           ["Cách tính dự kiến", "Phần còn ÷ tốc độ đo được, đếm ngày làm việc thứ Hai–Sáu từ hôm "
            "nay. Dòng chức năng giả định dồn sức riêng cho chức năng ấy; các chức năng làm song "
            "song nên KHÔNG cộng dồn các dòng — mốc cả dự án là dòng TOÀN DỰ ÁN", "", "", ""],
           ["Giới hạn", "Các \"phần\" không đều nhau (một API, một ô trên màn, một việc sổ đều tính "
            "là 1). Tốc độ chỉ đếm việc sổ chuyển sang Xong, còn phần còn lại gồm cả API và ô "
            "trên màn. Việc mới phát sinh làm phần còn tăng, nên mốc dịch dần nếu phạm vi còn nở. "
            "Không trừ ngày lễ. Chức năng chưa khởi công chưa có phần nào nên chưa có trong mốc "
            "dự kiến", "", "", ""],
           ["Phiên bản format", PHIEN_BAN_FORMAT, "", "", ""],
           ["Nguồn", "tools/xuat_tien_do.py — sinh từ mã và sổ tiến độ, không ô nào viết tay. "
            "Ghi chú của team để ở sheet RIÊNG tra theo cột Mã của Việc còn lại — Import → Replace "
            "xoá mọi cột tự thêm", "", "", ""]]

    # ---- Việc còn lại -----------------------------------------------------------------------
    vc = []
    for d in so:
        for x in d.get("muc", []):
            if x.get("trang_thai") == "xong":
                continue
            ma = f"{d['module']}/{x.get('id', '?')}"
            vc.append([ma, thuoc[ma], cau(x.get("viec", ""), 400),
                       NHAN_TT.get(x.get("trang_thai", ""), x.get("trang_thai", "")),
                       cau(x.get("tiep_theo", "") or "", 400)])
    vc.sort(key=lambda r: (r[1], [NHAN_TT[k] for k in THU_TU_TT].index(r[3])
                           if r[3] in NHAN_TT.values() else 9, r[0]))

    return {"Tổng quan": tq, "Chức năng": cn, "Việc còn lại": vc}


# ---------------------------------------------------------------------------------------------
# Ghi .xlsx — tập con tối thiểu của SpreadsheetML
_XML_BAN = re.compile(r"[\x00-\x08\x0b\x0c\x0e-\x1f]")
CO_LOC = {"Chức năng", "Việc còn lại"}   # Tổng quan có khối CÁCH ĐỌC bên dưới — lọc sẽ giấu nó


def ten_cot(i: int) -> str:
    s = ""
    i += 1
    while i:
        i, r = divmod(i - 1, 26)
        s = chr(65 + r) + s
    return s


def o_xml(ref: str, v, kieu: int) -> str:
    if isinstance(v, PhanTram):
        return f'<c r="{ref}" s="3"><v>{round(float(v), 4)}</v></c>'
    if isinstance(v, (int, float)) and not isinstance(v, bool):
        return f'<c r="{ref}" s="{kieu}"><v>{v}</v></c>'
    s = _XML_BAN.sub("", str(v))[:MAX_O]
    return (f'<c r="{ref}" s="{kieu}" t="inlineStr"><is><t xml:space="preserve">'
            f'{escape(s)}</t></is></c>')


def sheet_xml(cot: list[tuple], dong: list[list], loc: bool) -> str:
    n = len(dong) + 1
    x = ['<?xml version="1.0" encoding="UTF-8" standalone="yes"?>',
         '<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">',
         '<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" '
         'activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>',
         "<cols>" + "".join(f'<col min="{i+1}" max="{i+1}" width="{w}" customWidth="1"/>'
                            for i, (_t, w) in enumerate(cot)) + "</cols>",
         "<sheetData>",
         '<row r="1">' + "".join(o_xml(f"{ten_cot(i)}1", t, 1) for i, (t, _w) in
                                 enumerate(cot)) + "</row>"]
    for r, d in enumerate(dong, 2):
        x.append(f'<row r="{r}">' + "".join(o_xml(f"{ten_cot(i)}{r}", v, 2)
                                            for i, v in enumerate(d)) + "</row>")
    x.append("</sheetData>")
    if loc:
        x.append(f'<autoFilter ref="A1:{ten_cot(len(cot) - 1)}{n}"/>')
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
          '<cellXfs count="4"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>'
          '<xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1"/>'
          '<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1">'
          '<alignment vertical="top" wrapText="1"/></xf>'
          '<xf numFmtId="9" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1" '
          'applyAlignment="1"><alignment vertical="top"/></xf></cellXfs>'
          # Kiểu "Normal" mặc định: thiếu nó, openpyxl cảnh báo và một số trình đọc tự bù — tệp
          # đi vào Google Sheet của cả team thì không để trình đọc phải đoán.
          '<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>'
          "</styleSheet>")


def ghi_xlsx(duong: str, du_lieu: dict[str, list[list]]) -> None:
    ten = [t for t, _c in SHEETS]
    loc = [(i, t, c_) for i, (t, c_) in enumerate(SHEETS) if t in CO_LOC]
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
            + "".join(f'<sheet name="{escape(t)}" sheetId="{i}" r:id="rId{i}"/>'
                      for i, t in enumerate(ten, 1))
            + "</sheets>"
            + "<definedNames>" + "".join(
                f'<definedName name="_xlnm._FilterDatabase" localSheetId="{i}" hidden="1">'
                f"'{escape(t)}'!$A$1:${ten_cot(len(c_) - 1)}${len(du_lieu[t]) + 1}</definedName>"
                for i, t, c_ in loc) + "</definedNames></workbook>",
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
        tep[f"xl/worksheets/sheet{i}.xml"] = sheet_xml(cot, du_lieu[t], t in CO_LOC)

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

    trung = [f"'{t}'!{ten_cot(j)}{i + 2}"
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
