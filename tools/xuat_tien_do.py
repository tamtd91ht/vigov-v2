#!/usr/bin/env python3
"""Xuất báo cáo tiến độ kỹ thuật ra MỘT tệp .xlsx bảy sheet, cho BA / PM đọc.

Chạy (qua `/tien-do-san-pham --excel`, xem .claude/commands/tien-do-san-pham.md):
    python tools/xuat_tien_do.py --khung [noi-dung.json]      bước 1 — sinh khung + bằng chứng
    (agent đọc bằng chứng, viết phần chữ vào khoá `viet` của tệp ấy)
    python tools/xuat_tien_do.py --tu noi-dung.json [ra.xlsx]  bước 3 — kiểm rồi ghi .xlsx
    python tools/tien_do_san_pham.py --excel …                (cùng một việc)
Mặc định: tmp/tien-do/noi-dung-<ngày>.json · tmp/tien-do/vigov-tien-do-<ngày>.xlsx (tmp/ bị git bỏ qua)
Mã thoát: 0 = ghi xong · 1 = thiếu nguồn · 3 = có ô trông như dữ liệu cá nhân, KHÔNG ghi tệp ·
          4 = đường dẫn nằm trong kho mà git không bỏ qua · 5 = phần chữ thiếu / sai văn phong

NGƯỜI ĐỌC VÀ CÂU HỎI (format v3, 25/09/2026): BA và PM mở tệp để biết *"từng menu đã làm gì, còn
thiếu gì, API / web / Mini App / deploy đang ở đâu"* mà không phải hỏi lại. Bản v2 (ba sheet có %
và mốc dự kiến cho chủ dự án) được người dùng thay hẳn bằng mẫu này ngày 25/09/2026.

HAI LOẠI Ô, HAI NGUỒN — và ranh giới giữa chúng là lý do tệp này có hai bước:
  * SỐ LIỆU ĐẾM ĐƯỢC (tuyến API, phần chưa dựng, menu, manifest, sổ theo module) sinh từ mã mỗi
    lần xuất, bằng CÙNG các hàm của `tien_do_san_pham.py`. Tệp nội dung KHÔNG chứa được chúng —
    bước 3 đọc lại từ kho, nên một con số trong JSON cũ không bao giờ lên được bảng.
  * PHẦN CHỮ (đã làm / còn thiếu / đang xử lý / chức năng đã dựng / Mini App / checklist deploy)
    là TÓM TẮT, và tóm tắt không sinh được bằng máy mà không thành bản chép dài của sổ. Nên agent
    viết lại nó MỖI LẦN XUẤT từ bằng chứng bước 1 đưa ra, và nó chỉ sống trong tmp/ — không vào
    git, nên không thành bản thứ hai của một sự thật trong kho (luật 9 #2). Văn bản cố định chép
    vào tệp này sẽ đứng yên trong khi mã chạy, đúng loại bảng người đọc tin mà nó đã sai.

VĂN PHONG LÀ MỘT RÀNG BUỘC CÓ RÀO: người dùng quyết 25/09/2026 — báo cáo không đặt câu hỏi cho
người đọc; việc còn chờ xác nhận ghi là việc ĐANG XỬ LÝ. `loi_van_phong` từ chối những cụm biến
một ô thành câu hỏi hay lời đổ lỗi ("chờ khách", "bị chặn", "câu #n", dấu hỏi).

VÌ SAO THƯ VIỆN CHUẨN, không openpyxl: mọi công cụ của kho chỉ dùng thư viện chuẩn, và máy khác
hay Jenkins có thể không có gói ấy. Một .xlsx chỉ là một zip các tệp XML. Công thức ghi không kèm
giá trị đệm; `fullCalcOnLoad` bắt Excel / Google Sheet tính lại lúc mở.

TÊN SHEET VÀ CỘT BẢNG CHÍNH LÀ FORMAT: đổi thì tăng PHIEN_BAN_FORMAT và cập nhật `FORMAT_XLSX`
trong `tools/test_hooks.py` — ca ấy đỏ đúng để không ai đổi format mà quên báo team.

TRƯỚC KHI GHI, mọi ô chữ được soi bằng CÙNG mẫu số điện thoại / CCCD của `pii_guard`. Trúng thì
từ chối và chỉ in TOẠ ĐỘ ô, không bao giờ in giá trị.
"""

from __future__ import annotations

import collections
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

PHIEN_BAN_FORMAT = 3
SO_TIEN_DO = os.path.join("kb", "90-ephemeral", "tien-do")
MAX_O = 32000     # Excel giữ tối đa 32 767 ký tự một ô

S1, S2, S3, S4, S5, S6, S7 = ("1. Tổng quan", "2. Chức năng theo menu", "3. API Backend",
                              "4. Danh mục API", "5. Web Admin", "6. Mini App", "7. Deployment")
# (tên sheet, [(cột bảng chính, độ rộng)]) — THỨ TỰ NÀY LÀ FORMAT. Cột trống ở Tổng quan là ô gộp
# của cột Giá trị: lưới cột phải có đủ để bảng sổ theo module bên dưới dùng lại.
SHEETS = [
    (S1, [("Chỉ số", 42), ("Giá trị", 16), ("", 16), ("", 16), ("", 16), ("Xem chi tiết", 30)]),
    (S2, [("#", 5), ("Menu / Phân hệ", 20), ("Service sở hữu", 17), ("API (gọi/tổng)", 11),
          ("Web Admin", 22), ("Đã làm (backend + web)", 44), ("Còn thiếu", 50),
          ("Đang xử lý", 40), ("Mức độ", 15)]),
    (S3, [("Service", 20), ("Phạm vi nghiệp vụ", 34), ("Tuyến REST", 10), ("Đã làm", 52),
          ("Còn thiếu / rủi ro", 52), ("Trạng thái", 15)]),
    (S4, [("Chương", 9), ("Method", 9), ("Đường dẫn", 44), ("Phân hệ", 26), ("Service", 19),
          ("Chức năng", 52), ("Kiểm soát truy cập", 24), ("Khoá quyền / lý do", 32),
          ("Chống trùng", 11), ("Web gọi", 12)]),
    (S5, [("#", 5), ("Mục menu", 20), ("Đường dẫn", 18), ("Khoá quyền xem", 19), ("Có màn", 9),
          ("Phần đã dựng", 10), ("Chức năng đã dựng", 50), ("Phần chưa dựng", 10), ("Ghi chú", 34)]),
    (S6, [("#", 5), ("Màn / chức năng", 30), ("Nội dung", 50), ("Trạng thái", 15), ("Ghi chú", 58)]),
    (S7, [("#", 5), ("Đơn vị", 20), ("Loại / cổng", 30), ("Dockerfile", 11), ("Jenkinsfile", 11),
          ("Manifest k8s", 13), ("Ghi chú", 56)]),
]

MUC_DO = ("Cơ bản xong", "Đang hoàn thiện", "Đang xử lý", "Chưa khởi công")
TRANG_THAI = ("Xong", "Đã dựng", "Cơ bản xong", "Đang hoàn thiện", "Đang làm", "Đang xử lý",
              "Chưa làm", "Chưa khởi công")


# ---------------------------------------------------------------------------------------------
# Dữ liệu cá nhân — CÙNG mẫu với pii_guard, bỏ cặp dấu nháy (ô là văn bản trần, không phải chuỗi mã)
def _loi(mau: str) -> str:
    return mau[len(r"[\"'`]"):-len(r"[\"'`]")]


MAU_CA_NHAN = re.compile(rf"(?<!\d)(?:{_loi(c.VN_PHONE)}|{_loi(c.VN_CCCD)})(?!\d)")


def co_du_lieu_ca_nhan(s: str) -> bool:
    return any(not c.SAFE_FAKE.match(m.group(0)) for m in MAU_CA_NHAN.finditer(s or ""))


# Cụm làm một ô thành câu hỏi cho người đọc, hoặc thành lời đổ việc cho người khác. Không bắt chữ
# "quyết định" trơn: API có "quyết định lùi hạn", "máy trạng thái quyết định bước nào".
MAU_VAN_PHONG = re.compile(
    r"\?|[Cc]hờ khách|[Cc]ần (?:BA|PM|khách)\b|[Cc]ần (?:người|ai) (?:quyết|chốt|xác nhận)"
    r"|[Bb]ị chặn|\b[Tt]reo\b|[Tt]ạm dừng|câu (?:hỏi )?(?:mở )?#\s*\d+|[Nn]ợ khách|chưa ai quyết")


def loi_van_phong(s: str) -> bool:
    return bool(MAU_VAN_PHONG.search(s or ""))


def cau(s: str, toi_da: int = 300) -> str:
    """Bỏ ký hiệu markdown, gộp khoảng trắng, cắt ở ranh giới câu gần nhất — cho phần bằng chứng."""
    s = re.sub(r"\s+", " ", re.sub(r"[`*]", "", s or "")).strip()
    if len(s) <= toi_da:
        return s
    cat = s[:toi_da]
    i = max(cat.rfind(". "), cat.rfind("; "), cat.rfind(" — "))
    return (cat[:i + 1] if i > toi_da // 2 else cat.rstrip()) + " …"


def git(*a: str) -> str:
    try:
        return subprocess.run(["git", *a], capture_output=True, text=True, encoding="utf-8",
                              cwd=ROOT, timeout=20).stdout
    except Exception:
        return ""


def doc(p: str) -> str:
    try:
        return io.open(p, encoding="utf-8").read()
    except Exception:
        return ""


# ---------------------------------------------------------------------------------------------
# Thu thập SỐ LIỆU — mọi thứ ở đây đọc từ kho, không từ tệp nội dung
def doc_so() -> list[dict]:
    ra = []
    thu = os.path.join(ROOT, SO_TIEN_DO)
    for ten in sorted(os.listdir(thu)):
        if ten.endswith(".json"):
            d = json.loads(doc(os.path.join(thu, ten)))
            d.setdefault("module", ten[:-5])
            ra.append(d)
    return ra


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


def khoa_quyen() -> dict[str, str]:
    """`QUYEN_XEM_NHIEM_VU` → `task.read`: menu khai tên hằng, người đọc cần chuỗi khoá thật."""
    return dict(re.findall(r'export const (QUYEN_\w+)\s*=\s*"([^"]+)"',
                           doc(os.path.join(ROOT, "web-admin", "src", "lib", "quyen.ts"))))


KIND = {"permission": "Theo khoá quyền", "any-authenticated": "Mọi cán bộ đã đăng nhập",
        "public": "Công khai (có lý do)", "citizen-only": "Chỉ công dân (phiên Mini App)"}


def thu_thap() -> dict:
    chuong_goc = sp.cac_chuong()
    theo_chuong, tong_tuyen, _kk = sp.tuyen_theo_chuong()
    xong, biet_web = sp.viec_da_xong()
    menu_goc = sp.cac_muc_menu()
    if not chuong_goc or tong_tuyen == 0 or not menu_goc:
        raise RuntimeError(f"đọc hụt: {len(chuong_goc)} chương · {tong_tuyen} tuyến · "
                           f"{len(menu_goc)} menu")
    ten_ch = {slug: re.sub(r"^\d{2}\s*[—-]\s*", "", t or slug) for _s, slug, t in chuong_goc}
    so_ch = {slug: s for s, slug, _t in chuong_goc}
    chuong = sp.dong_chuong(chuong_goc, theo_chuong, xong, biet_web)
    menu = sp.dong_menu(menu_goc)
    menu_theo_chuong = ghep_menu(chuong, menu)
    app_goi = set(sp.tuyen_app_goi())
    kq = khoa_quyen()

    # ---- tuyến API, từ hợp đồng ------------------------------------------------------------
    hop_dong = json.loads(doc(sp.HOP_DONG) or "{}")
    tuyen = []
    for duong, ops in (hop_dong.get("paths") or {}).items():
        for pt, op in ops.items():
            if pt not in sp.PHUONG_THUC:
                continue
            man = (op.get("x-vigov-screen") or "").strip()
            slug = man.split()[0] if man and not man.startswith("*") else ""
            quyen = op.get("x-vigov-permission") or {}
            kind = quyen.get("kind", "")
            if kind == "citizen-only":
                goi = "Mini App" if duong in app_goi and pt == "post" else "Chưa"
            else:
                goi = "Có" if op.get("x-vigov-task") in xong else "Chưa"
            ly_do = quyen.get("reason", "")
            tuyen.append({
                "so": so_ch.get(slug, ""), "slug": slug,
                "chuong": f"{so_ch[slug]} — {ten_ch[slug]}" if slug in so_ch else "Chưa gắn chương đặc tả",
                "pt": pt.upper(), "duong": duong,
                "service": "service-" + ((op.get("tags") or ["?"])[0]),
                "chuc_nang": op.get("summary", ""), "kind": KIND.get(kind, kind),
                "khoa": quyen.get("key") or (ly_do if len(ly_do) <= 110
                                             else ly_do[:107].rsplit(" ", 1)[0] + " …"),
                "chong_trung": "Bắt buộc" if op.get("x-vigov-idempotency") else "",
                "goi": goi,
            })
    tuyen.sort(key=lambda t: (t["so"] or "99", t["duong"], t["pt"]))

    # ---- sổ tiến độ -----------------------------------------------------------------------
    so = doc_so()
    slug_ngan = {r["slug"].split("-", 1)[1]: r["slug"] for r in chuong}
    viec_chuong: dict[str, list[dict]] = collections.defaultdict(list)
    viec_module: dict[str, list[dict]] = collections.defaultdict(list)
    so_module = []
    for d in so:
        dem = collections.Counter(x.get("trang_thai") for x in d.get("muc", []))
        so_module.append((d["module"], dem["xong"], dem["dang_lam"] + dem["treo"], dem["chua_lam"],
                          d.get("cap_nhat", "")))
        for x in d.get("muc", []):
            v = {"ma": f"{d['module']}/{x.get('id', '?')}", "trang_thai": x.get("trang_thai", ""),
                 "viec": cau(x.get("viec", ""), 300)}
            if x.get("trang_thai") != "xong":
                v["tiep_theo"] = cau(x.get("tiep_theo", "") or "", 400)
            menus = x.get("menu")
            menus = [] if menus is None else (menus if isinstance(menus, list) else [menus])
            for s in menus:
                if s in slug_ngan:
                    viec_chuong[slug_ngan[s]].append(v)
            viec_module[d["module"]].append(v)

    # ---- chương + menu --------------------------------------------------------------------
    dem_sv = collections.defaultdict(collections.Counter)
    for t in tuyen:
        dem_sv[t["slug"]][t["service"]] += 1
    ds_chuong, ds_menu = [], []
    for r in chuong:
        m = menu_theo_chuong.get(r["slug"])
        # Chương không có menu, tuyến hay việc (00) là phần giới thiệu của đặc tả, không phải menu.
        if not m and not r["tuyen"] and not viec_chuong.get(r["slug"]):
            continue
        cd = [t for t in theo_chuong.get(r["slug"], []) if sp.la_cong_dan(t)]
        api = (f"{r['da_goi']}/{r['web']}" if r["web"] else "0")
        if cd:
            api += f" (+{len(cd)} công dân)"
        if m is None:
            web = "Không có mục menu"
        elif m["duong"] is None:
            web = "Chưa có màn (mục menu giữ chỗ)"
        else:
            web = m["duong"] + (" — có màn" if m["co_man"] else " — chưa có trang")
        ds_chuong.append({
            "so": r["so"], "slug": r["slug"], "ten": r["ten"],
            "service": " · ".join(s for s, _n in dem_sv[r["slug"]].most_common()) or "—",
            "api": api, "web": web,
            "chua_dung": ([x for f in sp.feature_cua_trang(m["duong"])
                           for x in (sp.phan_chua_dung(f) or [])]
                          if m and m["duong"] else []),
            "tuyen": [f"{t['pt']} {t['duong']}" for t in tuyen if t["slug"] == r["slug"]],
            "viec": viec_chuong.get(r["slug"], []),
        })
    for m in menu:
        pcd = None
        if m["duong"] is not None:
            khai = [p for p in (sp.phan_chua_dung(f) for f in sp.feature_cua_trang(m["duong"]))
                    if p is not None]
            pcd = [x for p in khai for x in p] if khai else None
            # Hai phép đếm trên cùng một nguồn phải ra một số — lệch là bản Excel in một danh
            # sách khác với con số `.md` đang báo. Từ chối thay vì chọn một bên.
            if (m["chua_dung"] is None) != (pcd is None) or (pcd is not None and len(pcd) != m["chua_dung"]):
                raise RuntimeError(f"phần chưa dựng của {m['nhan']!r}: .md đếm {m['chua_dung']}, "
                                   f"Excel đọc {None if pcd is None else len(pcd)}")
        ds_menu.append({"nhan": m["nhan"], "duong": m["duong"] or "—",
                        "khoa": kq.get(m["khoa"] or "", m["khoa"] or "—"),
                        "co_man": bool(m["duong"] and m["co_man"]), "chua_dung": pcd})

    # ---- service + đơn vị triển khai -------------------------------------------------------
    services = sorted(d for d in os.listdir(ROOT)
                      if d.startswith("service-") and os.path.isdir(os.path.join(ROOT, d)))
    don_vi = []
    for ten in services + ["web-admin", "platform-admin", "citizen-app"]:
        ngan = ten[len("service-"):] if ten.startswith("service-") else ten
        svc_yaml = doc(os.path.join(ROOT, "deploy", "base", ngan, "service.yaml"))
        if ten.startswith("service-"):
            loai = "Go · REST" + (" + gRPC" if "grpc" in svc_yaml else "")
        elif ten == "citizen-app":
            loai = "Zalo Mini App (không chạy thành pod)"
        else:
            loai = "Next.js · HTTP"
        co = lambda p: "Có" if os.path.exists(os.path.join(ROOT, *p)) else "Không"  # noqa: E731
        don_vi.append({
            "ten": ngan, "loai": loai, "dockerfile": co([ten, "Dockerfile"]),
            "jenkinsfile": co([ten, "Jenkinsfile"]),
            "manifest": ("Không áp dụng" if ten == "citizen-app"
                         else co(["deploy", "base", ngan])),
        })

    commit = git("rev-parse", "--short", "HEAD").strip()
    ban = git("status", "--porcelain").splitlines()
    return {"chuong": ds_chuong, "menu": ds_menu, "tuyen": tuyen, "services": services,
            "don_vi": don_vi, "so_module": so_module, "viec_module": dict(viec_module),
            "commit": commit + (f" + {len(ban)} tệp chưa commit" if ban else "")}


# ---------------------------------------------------------------------------------------------
# Bước 1 — khung nội dung
HUONG_DAN = [
    "Điền MỌI ô trong khoá `viet`; KHÔNG sửa `bang_chung` (bước 3 đọc lại số liệu từ kho, mọi thay đổi ở đó bị bỏ qua).",
    "Nguồn viết: `bang_chung` + tệp sổ kb/90-ephemeral/tien-do/<module>.json + deploy/README.md mục 10. Mở tệp kiểm trước khi viết một khẳng định; không viết con số mà tool đã tự đếm (số API, số phần chưa dựng).",
    "Văn phong: kỹ thuật, ngắn, một ý một câu, tiếng Việt có dấu. KHÔNG đặt câu hỏi cho người đọc. Việc còn chờ xác nhận / chờ bên khác → viết thành việc ĐANG XỬ LÝ (không 'chờ khách', 'cần BA quyết', 'bị chặn', 'treo', 'câu #n', không dấu hỏi) — tool từ chối các cụm ấy.",
    f"`muc_do` của chương ∈ {list(MUC_DO)}. `trang_thai` mọi dòng khác ∈ {list(TRANG_THAI)}.",
    "`web.<menu>.da_dung`: tên từng chức năng ĐÃ DỰNG trên màn, mỗi phần tử một chức năng, kèm § đặc tả nếu có. Tool đếm số lượng từ danh sách này.",
    "Không đưa dữ liệu cá nhân (SĐT, CCCD, họ tên công dân) vào bất kỳ ô nào — luật 3.",
]


def khung_viet(sl: dict, cu: dict) -> dict:
    """Khung `viet` đủ khoá cho số liệu hôm nay; giữ lại chữ đã viết ở lần trước nếu còn khớp."""
    def giu(duong: list[str], mac_dinh):
        v = cu
        for k in duong:
            if not isinstance(v, dict) or k not in v:
                return mac_dinh
            v = v[k]
        return v

    return {
        "tom_tat": giu(["tom_tat"], ""),
        "luu_y": giu(["luu_y"], ["", "", "", "", ""]),
        "chuong": {r["so"]: giu(["chuong", r["so"]], {"da_lam": "", "con_thieu": "",
                                                      "dang_xu_ly": "", "muc_do": ""})
                   for r in sl["chuong"]},
        "web": {m["nhan"]: giu(["web", m["nhan"]], {"da_dung": [], "ghi_chu": ""})
                for m in sl["menu"]},
        "web_ghi_chu_ky_thuat": giu(["web_ghi_chu_ky_thuat"], []),
        "service": {s: giu(["service", s], {"pham_vi": "", "da_lam": "", "con_thieu": "",
                                            "trang_thai": ""})
                    for s in sl["services"]},
        "nen_tang": giu(["nen_tang"], [{"hang_muc": "", "noi_dung": "", "da_lam": "",
                                        "con_thieu": "", "trang_thai": ""}]),
        "mini_app": giu(["mini_app"], {"mo_ta": "", "nhom": [
            {"ten": "Giai đoạn 1 — …", "dong": [{"ten": "", "noi_dung": "", "trang_thai": "", "ghi_chu": ""}]},
            {"ten": "Giai đoạn 2 — …", "dong": []},
            {"ten": "Phát hành lên Zalo", "dong": []}]}),
        "deploy": giu(["deploy"], {"mo_ta": "",
                                   "ghi_chu_don_vi": {d["ten"]: "" for d in sl["don_vi"]},
                                   "checklist": [{"buoc": "", "noi_dung": "", "trang_thai": "", "ghi_chu": ""}],
                                   "rui_ro": []}),
    }


def ghi_khung(duong: str, sl: dict) -> None:
    cu = {}
    if os.path.exists(duong):
        try:
            cu = json.loads(doc(duong)).get("viet", {})
        except Exception:
            cu = {}
    bc = {
        "chuong": [{k: r[k] for k in ("so", "ten", "service", "api", "web", "chua_dung", "tuyen", "viec")}
                   for r in sl["chuong"]],
        "services": {s: {"so_tuyen": sum(1 for t in sl["tuyen"] if t["service"] == s),
                         "viec": sl["viec_module"].get(s, [])} for s in sl["services"]},
        "module_khac": {k: v for k, v in sl["viec_module"].items() if not k.startswith("service-")},
        "don_vi_trien_khai": sl["don_vi"],
    }
    ra = {"phien_ban_format": PHIEN_BAN_FORMAT, "commit": sl["commit"],
          "sinh_luc": datetime.datetime.now().strftime("%d/%m/%Y %H:%M"),
          "_huong_dan": HUONG_DAN, "viet": khung_viet(sl, cu), "bang_chung": bc}
    os.makedirs(os.path.dirname(duong) or ".", exist_ok=True)
    io.open(duong, "w", encoding="utf-8").write(json.dumps(ra, ensure_ascii=False, indent=1))


# ---------------------------------------------------------------------------------------------
# Bước 3 — kiểm phần chữ
def kiem_viet(v: dict, sl: dict) -> list[str]:
    loi: list[str] = []

    def chu(duong: str, s, bat_buoc: bool = True):
        if not isinstance(s, str):
            loi.append(f"{duong}: phải là chuỗi")
        elif bat_buoc and not s.strip():
            loi.append(f"{duong}: còn trống")
        elif loi_van_phong(s):
            loi.append(f"{duong}: sai văn phong (câu hỏi / 'chờ…' / 'bị chặn' — viết thành việc đang xử lý)")

    def tt(duong: str, s, cho_phep):
        if s not in cho_phep:
            loi.append(f"{duong}: '{s}' không thuộc {list(cho_phep)}")

    chu("viet.tom_tat", v.get("tom_tat"))
    ly = v.get("luu_y") or []
    if not ly:
        loi.append("viet.luu_y: cần ít nhất một điểm")
    for i, s in enumerate(ly):
        chu(f"viet.luu_y[{i}]", s)
    ch = v.get("chuong") or {}
    for r in sl["chuong"]:
        x = ch.get(r["so"])
        if not isinstance(x, dict):
            loi.append(f"viet.chuong.{r['so']}: thiếu ({r['ten']})")
            continue
        for k in ("da_lam", "con_thieu", "dang_xu_ly"):
            chu(f"viet.chuong.{r['so']}.{k}", x.get(k))
        tt(f"viet.chuong.{r['so']}.muc_do", x.get("muc_do"), MUC_DO)
    for k in set(ch) - {r["so"] for r in sl["chuong"]}:
        loi.append(f"viet.chuong.{k}: chương không còn trong đặc tả — tệp nội dung đã cũ")
    web = v.get("web") or {}
    for m in sl["menu"]:
        x = web.get(m["nhan"])
        if not isinstance(x, dict):
            loi.append(f"viet.web.{m['nhan']}: thiếu")
            continue
        ds = x.get("da_dung") or []
        if m["co_man"] and not ds:
            loi.append(f"viet.web.{m['nhan']}.da_dung: màn đã có mà chưa liệt kê chức năng nào")
        for i, s in enumerate(ds):
            chu(f"viet.web.{m['nhan']}.da_dung[{i}]", s)
        chu(f"viet.web.{m['nhan']}.ghi_chu", x.get("ghi_chu", ""), bat_buoc=False)
    for k in set(web) - {m["nhan"] for m in sl["menu"]}:
        loi.append(f"viet.web.{k}: mục menu không còn — tệp nội dung đã cũ")
    for i, s in enumerate(v.get("web_ghi_chu_ky_thuat") or []):
        chu(f"viet.web_ghi_chu_ky_thuat[{i}]", s)
    sv = v.get("service") or {}
    for s in sl["services"]:
        x = sv.get(s)
        if not isinstance(x, dict):
            loi.append(f"viet.service.{s}: thiếu")
            continue
        for k in ("pham_vi", "da_lam", "con_thieu"):
            chu(f"viet.service.{s}.{k}", x.get(k))
        tt(f"viet.service.{s}.trang_thai", x.get("trang_thai"), TRANG_THAI)
    for i, x in enumerate(v.get("nen_tang") or []):
        for k in ("hang_muc", "noi_dung", "da_lam"):
            chu(f"viet.nen_tang[{i}].{k}", x.get(k))
        chu(f"viet.nen_tang[{i}].con_thieu", x.get("con_thieu", ""), bat_buoc=False)
        tt(f"viet.nen_tang[{i}].trang_thai", x.get("trang_thai"), TRANG_THAI)
    ma = v.get("mini_app") or {}
    chu("viet.mini_app.mo_ta", ma.get("mo_ta"))
    for i, n in enumerate(ma.get("nhom") or []):
        chu(f"viet.mini_app.nhom[{i}].ten", n.get("ten"))
        for j, x in enumerate(n.get("dong") or []):
            p = f"viet.mini_app.nhom[{i}].dong[{j}]"
            chu(p + ".ten", x.get("ten"))
            chu(p + ".noi_dung", x.get("noi_dung"))
            chu(p + ".ghi_chu", x.get("ghi_chu", ""), bat_buoc=False)
            tt(p + ".trang_thai", x.get("trang_thai"), TRANG_THAI)
    dp = v.get("deploy") or {}
    chu("viet.deploy.mo_ta", dp.get("mo_ta"))
    for k, s in (dp.get("ghi_chu_don_vi") or {}).items():
        chu(f"viet.deploy.ghi_chu_don_vi.{k}", s, bat_buoc=False)
    if not dp.get("checklist"):
        loi.append("viet.deploy.checklist: cần ít nhất một bước")
    for i, x in enumerate(dp.get("checklist") or []):
        p = f"viet.deploy.checklist[{i}]"
        chu(p + ".buoc", x.get("buoc"))
        chu(p + ".noi_dung", x.get("noi_dung"))
        chu(p + ".ghi_chu", x.get("ghi_chu", ""), bat_buoc=False)
        tt(p + ".trang_thai", x.get("trang_thai"), TRANG_THAI)
    for i, s in enumerate(dp.get("rui_ro") or []):
        chu(f"viet.deploy.rui_ro[{i}]", s)
    return loi


# ---------------------------------------------------------------------------------------------
# Ghi .xlsx — tập con SpreadsheetML đủ cho định dạng, ô gộp, công thức, liên kết trong tệp
_XML_BAN = re.compile(r"[\x00-\x08\x0b\x0c\x0e-\x1f]")


class CongThuc(str):
    """Ô công thức (không kèm dấu `=`)."""


def ten_cot(i: int) -> str:   # 0 → A
    s = ""
    i += 1
    while i:
        i, r = divmod(i - 1, 26)
        s = chr(65 + r) + s
    return s


F = "Arial"
NAVY, TEAL, GRID = "1F3864", "2E75B6", "BFBFBF"
MAU_TT = {"xanh": ("E2F0D9", "375623"), "vang": ("FFF2CC", "7F6000"), "xam": ("EDEDED", "404040")}
TT_MAU = {"Cơ bản xong": "xanh", "Xong": "xanh", "Có": "xanh", "Đã dựng": "xanh", "Mini App": "xanh",
          "Đang hoàn thiện": "vang", "Đang làm": "vang", "Đang xử lý": "vang",
          "Chưa khởi công": "xam", "Chưa làm": "xam", "Không": "xam", "Không áp dụng": "xam",
          "Chưa": "xam"}


class KieuO:
    """Bảng kiểu: mỗi tổ hợp (font, nền, viền, căn) một chỉ số `xf`, sinh khi dùng lần đầu."""

    def __init__(self):
        self.font = ['<font><sz val="10"/><name val="Arial"/></font>']
        self.fill = ['<fill><patternFill patternType="none"/></fill>',
                     '<fill><patternFill patternType="gray125"/></fill>']
        self.border = ['<border><left/><right/><top/><bottom/><diagonal/></border>']
        self.xf = ['<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>']
        self._idx: dict = {}

    def _them(self, ds: list, x: str) -> int:
        if x not in ds:
            ds.append(x)
        return ds.index(x)

    def __call__(self, sz=10, b=False, i=False, u=False, mau="000000", ten=F, nen=None,
                 vien=None, can="trai", boc=True) -> int:
        k = (sz, b, i, u, mau, ten, nen, vien, can, boc)
        if k in self._idx:
            return self._idx[k]
        fo = self._them(self.font, "<font>" + ("<b/>" if b else "") + ("<i/>" if i else "")
                        + ('<u val="single"/>' if u else "") + f'<sz val="{sz}"/>'
                        + f'<color rgb="FF{mau}"/><name val="{ten}"/></font>')
        fi = 0 if not nen else self._them(
            self.fill, f'<fill><patternFill patternType="solid"><fgColor rgb="FF{nen}"/>'
                       f'<bgColor indexed="64"/></patternFill></fill>')
        if vien == "luoi":
            bo = self._them(self.border, "<border>" + "".join(
                f'<{s} style="thin"><color rgb="FF{GRID}"/></{s}>'
                for s in ("left", "right", "top", "bottom")) + "<diagonal/></border>")
        elif vien == "duoi":
            bo = self._them(self.border, f'<border><left/><right/><top/><bottom style="medium">'
                                         f'<color rgb="FF{TEAL}"/></bottom><diagonal/></border>')
        else:
            bo = 0
        ngang = {"trai": "", "giua": ' horizontal="center"'}[can]
        dung = ' vertical="center"' if can == "giua" and not boc else ' vertical="top"'
        boc_xml = ' wrapText="1"' if boc else ""   # Jenkins chạy python3.9: không `\` trong f-string
        al = f"<alignment{ngang}{dung}{boc_xml}/>"
        self.xf.append(f'<xf numFmtId="0" fontId="{fo}" fillId="{fi}" borderId="{bo}" xfId="0" '
                       f'applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1">{al}</xf>')
        self._idx[k] = len(self.xf) - 1
        return self._idx[k]

    def xml(self) -> str:
        return ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
                '<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">'
                f'<fonts count="{len(self.font)}">{"".join(self.font)}</fonts>'
                f'<fills count="{len(self.fill)}">{"".join(self.fill)}</fills>'
                f'<borders count="{len(self.border)}">{"".join(self.border)}</borders>'
                '<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>'
                f'<cellXfs count="{len(self.xf)}">{"".join(self.xf)}</cellXfs>'
                # Kiểu "Normal": thiếu nó, một số trình đọc tự bù — tệp đi vào Google Sheet của
                # cả team thì không để trình đọc phải đoán.
                '<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>'
                "</styleSheet>")


class Sheet:
    def __init__(self, ten: str, cot: list[tuple[str, int]], tab: str, k: KieuO):
        self.ten, self.cot, self.tab, self.k = ten, cot, tab, k
        self.o: dict[tuple[int, int], tuple] = {}
        self.gop: list[str] = []
        self.cao: dict[int, float] = {}
        self.lien_ket: list[tuple[str, str]] = []
        self.dong_bang: str | None = None
        self.loc: str | None = None
        self.r = 1
        self.nc = len(cot)

    def rong(self, j: int) -> int:
        return self.cot[j - 1][1]

    def dat(self, r: int, j: int, v, s: int):
        self.o[(r, j)] = (v, s)

    def gop_o(self, r1, j1, r2, j2):
        self.gop.append(f"{ten_cot(j1 - 1)}{r1}:{ten_cot(j2 - 1)}{r2}")

    def uoc_cao(self, r: int, vals: list, rong_rieng: dict | None = None):
        dong = 1
        for j, v in enumerate(vals, 1):
            if v is None or isinstance(v, CongThuc):
                continue
            w = (rong_rieng or {}).get(j, self.rong(j))
            moi = max(1, int(w * 1.0))   # ngắt theo từ nên một dòng chứa ít ký tự hơn độ rộng cột
            dong = max(dong, sum(max(1, math.ceil(len(seg) / moi)) for seg in str(v).split("\n")))
        self.cao[r] = min(409, 13.5 * dong + 5)

    # ---- khối dựng sẵn ---------------------------------------------------------------------
    def tieu_de(self, chu: str, phu: str):
        k = self.k
        self.gop_o(1, 1, 1, self.nc)
        self.dat(1, 1, chu, k(15, b=True, mau="FFFFFF", nen=NAVY, boc=False, can="trai"))
        for j in range(2, self.nc + 1):
            self.dat(1, j, None, k(nen=NAVY))
        self.cao[1] = 30
        self.gop_o(2, 1, 2, self.nc)
        self.dat(2, 1, phu, k(9, i=True, mau="595959"))
        self.cao[2] = max(30, 13 * math.ceil(len(phu) / (sum(w for _t, w in self.cot) * 1.2)) + 6)
        self.r = 4

    def muc(self, chu: str):
        s = self.k(11, b=True, mau=NAVY, vien="duoi")
        self.gop_o(self.r, 1, self.r, self.nc)
        self.dat(self.r, 1, chu, s)
        for j in range(2, self.nc + 1):
            self.dat(self.r, j, None, s)
        self.cao[self.r] = 20
        self.r += 1

    def dau_bang(self, cot: list[str], gop: list[tuple[int, int]] = ()):
        s = self.k(10, b=True, mau="FFFFFF", nen=TEAL, vien="luoi", can="giua")
        for j, t in enumerate(cot, 1):
            self.dat(self.r, j, t, s)
        for a, b in gop:
            self.gop_o(self.r, a, self.r, b)
        self.uoc_cao(self.r, cot, {a: sum(self.rong(x) for x in range(a, b + 1)) for a, b in gop})
        self.cao[self.r] = max(30, self.cao[self.r])
        self.r += 1

    def dong(self, vals: list, tt=(), giua=(), dam=(), soc=False, gop: list[tuple[int, int]] = (),
             font_rieng: dict | None = None):
        k = self.k
        rong_rieng = {a: sum(self.rong(x) for x in range(a, b + 1)) for a, b in gop}
        for j, v in enumerate(vals, 1):
            can = "giua" if j in giua else "trai"
            nen = "F7F9FC" if soc else None
            if j in tt and isinstance(v, str) and v in TT_MAU:
                bg, fg = MAU_TT[TT_MAU[v]]
                s = k(10, b=True, mau=fg, nen=bg, vien="luoi", can=can)
            elif font_rieng and j in font_rieng:
                s = k(vien="luoi", nen=nen, can=can, **font_rieng[j])
            else:
                s = k(10, b=(j in dam), nen=nen, vien="luoi", can=can)
            self.dat(self.r, j, v, s)
        for a, b in gop:
            self.gop_o(self.r, a, self.r, b)
        self.uoc_cao(self.r, vals, rong_rieng)
        self.r += 1
        return self.r - 1

    def ghi_chu(self, chu: str, nghieng: bool = True):
        self.gop_o(self.r, 1, self.r, self.nc)
        self.dat(self.r, 1, chu, self.k(9 if nghieng else 10, i=nghieng,
                                        mau="595959" if nghieng else "262626"))
        tong = sum(w for _t, w in self.cot)
        self.cao[self.r] = max(15, 13.5 * math.ceil(len(chu) / (tong * 1.15)) + 4)
        self.r += 1

    # ---- XML ------------------------------------------------------------------------------
    def xml(self) -> str:
        x = ['<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
             '<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" '
             'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">',
             f'<sheetPr><tabColor rgb="FF{self.tab}"/><pageSetUpPr fitToPage="1"/></sheetPr>',
             '<sheetViews><sheetView showGridLines="0" workbookViewId="0">']
        if self.dong_bang:
            m = re.match(r"([A-Z]+)(\d+)", self.dong_bang)
            cot_n = sum((ord(ch) - 64) * 26 ** i for i, ch in enumerate(reversed(m.group(1)))) - 1
            dong_n = int(m.group(2)) - 1
            x.append(f'<pane xSplit="{cot_n}" ySplit="{dong_n}" topLeftCell="{self.dong_bang}" '
                     f'activePane="bottomRight" state="frozen"/>')
        x.append("</sheetView></sheetViews>")
        x.append("<cols>" + "".join(f'<col min="{i}" max="{i}" width="{w}" customWidth="1"/>'
                                    for i, (_t, w) in enumerate(self.cot, 1)) + "</cols><sheetData>")
        for r in sorted({r for r, _j in self.o} | set(self.cao)):
            cao = f' ht="{self.cao[r]:.1f}" customHeight="1"' if r in self.cao else ""
            o = []
            for j in sorted(jj for rr, jj in self.o if rr == r):
                v, s = self.o[(r, j)]
                ref = f"{ten_cot(j - 1)}{r}"
                if v is None:
                    o.append(f'<c r="{ref}" s="{s}"/>')
                elif isinstance(v, CongThuc):
                    o.append(f'<c r="{ref}" s="{s}"><f>{escape(str(v))}</f></c>')
                elif isinstance(v, (int, float)) and not isinstance(v, bool):
                    o.append(f'<c r="{ref}" s="{s}"><v>{v}</v></c>')
                else:
                    t = _XML_BAN.sub("", str(v))[:MAX_O]
                    o.append(f'<c r="{ref}" s="{s}" t="inlineStr"><is><t xml:space="preserve">'
                             f'{escape(t)}</t></is></c>')
            x.append(f'<row r="{r}"{cao}>' + "".join(o) + "</row>")
        x.append("</sheetData>")
        if self.loc:
            x.append(f'<autoFilter ref="{self.loc}"/>')
        if self.gop:
            x.append(f'<mergeCells count="{len(self.gop)}">'
                     + "".join(f'<mergeCell ref="{g}"/>' for g in self.gop) + "</mergeCells>")
        if self.lien_ket:
            x.append("<hyperlinks>" + "".join(
                f'<hyperlink ref="{ref}" location="{escape(loc)}" display="{escape(loc)}"/>'
                for ref, loc in self.lien_ket) + "</hyperlinks>")
        x.append('<pageMargins left="0.4" right="0.4" top="0.5" bottom="0.5" header="0.3" footer="0.3"/>'
                 '<pageSetup orientation="landscape" fitToWidth="1" fitToHeight="0"/></worksheet>')
        return "".join(x)


def q(ten: str) -> str:
    return "'" + ten.replace("'", "''") + "'"


# ---------------------------------------------------------------------------------------------
# Dựng bảy sheet
def dung(sl: dict, v: dict, ngay: str) -> tuple[list[Sheet], KieuO]:
    k = KieuO()
    cot = dict(SHEETS)
    sh = {t: Sheet(t, cot[t], tab, k) for t, tab in
          [(S1, NAVY), (S2, TEAL), (S3, "548235"), (S4, "548235"), (S5, "BF8F00"),
           (S6, "7030A0"), (S7, "C00000")]}
    hdr = lambda t: [ct for ct, _w in cot[t]]  # noqa: E731

    # ---- 2. Chức năng theo menu -------------------------------------------------------------
    s = sh[S2]
    s.tieu_de("CHỨC NĂNG THEO TỪNG MENU / PHÂN HỆ",
              "Mỗi dòng = một chương đặc tả (docs/ui-ux/NN-*.md) = một mục menu Web Admin. Cột API: "
              "\"số tuyến web đã gọi / tổng tuyến web của chương\", tuyến kênh công dân ghi riêng. "
              "Danh sách đầy đủ phần chưa dựng của từng màn ở sheet 5.")
    s.dau_bang(hdr(S2))
    c2a = s.r
    for i, r in enumerate(sl["chuong"]):
        x = v["chuong"][r["so"]]
        s.dong([r["so"], r["ten"], r["service"], r["api"], r["web"], x["da_lam"], x["con_thieu"],
                x["dang_xu_ly"], x["muc_do"]], tt=(9,), giua=(1, 4), dam=(2,), soc=i % 2 == 1)
    c2b = s.r - 1
    s.dong_bang, s.loc = f"C{c2a}", f"A{c2a - 1}:I{c2b}"
    s.r += 1
    s.ghi_chu("Mức độ: Cơ bản xong = luồng chính chạy, còn hạng mục phụ · Đang hoàn thiện = có API + "
              "màn web gọi được, còn phần đặc tả chưa dựng · Đang xử lý = đang hoàn thiện đặc tả / phụ "
              "thuộc trước khi dựng · Chưa khởi công = chưa có API và màn.")

    # ---- 4. Danh mục API ------------------------------------------------------------------
    s = sh[S4]
    s.tieu_de("DANH MỤC API REST (SINH TỪ HỢP ĐỒNG)",
              "Nguồn: kb/20-contracts/openapi.json. Cột \"Web gọi\": Có = đã có mã web-admin gọi tuyến "
              "này; Mini App = citizen-app đang gọi. Lọc theo cột bằng nút ▼ ở dòng tiêu đề.")
    s.dau_bang(hdr(S4))
    c4a = s.r
    for i, t in enumerate(sl["tuyen"]):
        s.dong([t["so"] or "—", t["pt"], t["duong"], t["chuong"], t["service"], t["chuc_nang"],
                t["kind"], t["khoa"], t["chong_trung"], t["goi"]], tt=(10,), giua=(1, 2, 9, 10),
               soc=i % 2 == 1, font_rieng={3: {"sz": 9, "ten": "Consolas"}})
    c4b = s.r - 1
    s.dong_bang, s.loc = f"D{c4a}", f"A{c4a - 1}:J{c4b}"

    # ---- 3. API Backend -------------------------------------------------------------------
    s = sh[S3]
    s.tieu_de("BACKEND — GO MICROSERVICES",
              "Mỗi service sở hữu CSDL riêng; gọi chéo chỉ qua gRPC hoặc event. Mọi request mang "
              "tenant_id (xã) lấy từ domain, không nhận từ client. Số tuyến REST đếm từ hợp đồng; "
              "danh mục từng tuyến ở sheet 4.")
    s.dau_bang(hdr(S3))
    c3a = s.r
    for i, ten in enumerate(sl["services"]):
        x = v["service"][ten]
        r = s.dong([ten, x["pham_vi"], CongThuc(f'COUNTIF({q(S4)}!$E:$E,A{s.r})'), x["da_lam"],
                    x["con_thieu"], x["trang_thai"]], tt=(6,), giua=(3,), dam=(1, 3), soc=i % 2 == 1)
    c3b = s.r - 1
    s.dong(["Tổng", "", CongThuc(f"SUM(C{c3a}:C{c3b})"), "", "", ""], giua=(3,), dam=(1, 3))
    if v.get("nen_tang"):
        s.r += 1
        s.muc("Nền tảng kỹ thuật dùng chung (core/)")
        s.dau_bang(["Hạng mục", "Nội dung", "", "Đã làm", "Còn thiếu", "Trạng thái"], gop=[(2, 3)])
        for i, x in enumerate(v["nen_tang"]):
            s.dong([x["hang_muc"], x["noi_dung"], None, x["da_lam"], x.get("con_thieu", ""),
                    x["trang_thai"]], tt=(6,), dam=(1,), soc=i % 2 == 1, gop=[(2, 3)])
    s.dong_bang = f"B{c3a}"

    # ---- 5. Web Admin ---------------------------------------------------------------------
    s = sh[S5]
    s.tieu_de("WEB ADMIN (NEXT.JS) — MÀN HÌNH CÁN BỘ",
              "Cấu hình từng xã đọc lúc chạy theo domain (không đóng vào bundle). Kiểm quyền nằm ở máy "
              "chủ; UI chỉ ẩn/hiện. \"Phần đã dựng\" đếm số chức năng ở cột bên cạnh (mỗi dòng một chức "
              "năng). \"Phần chưa dựng\" là danh sách chính màn hình tự khai (PHAN_CHUA_DUNG) — chi tiết ở mục B.")
    s.muc("A. Tổng hợp theo mục menu")
    s.dau_bang(hdr(S5))
    c5a = s.r
    hang_menu = {}
    for i, m in enumerate(sl["menu"], 1):
        x = v["web"][m["nhan"]]
        ds = "\n".join("• " + d for d in x.get("da_dung") or [])
        r = s.r
        s.dong([i, m["nhan"], m["duong"], m["khoa"], "Có" if m["co_man"] else "Không",
                CongThuc(f'IF(LEN(G{r})=0,0,LEN(G{r})-LEN(SUBSTITUTE(G{r},CHAR(10),""))+1)'),
                ds, None, x.get("ghi_chu", "")], tt=(5,), giua=(1, 5, 6, 8), dam=(2, 6, 8),
               soc=i % 2 == 0)
        s.cao[r] = max(s.cao[r], 13.5 * max(1, len(x.get("da_dung") or [])) + 5)
        hang_menu[m["nhan"]] = r
    c5b = s.r - 1
    tong5 = s.dong(["", "Tổng", "", "", CongThuc(f'COUNTIF(E{c5a}:E{c5b},"Có")&"/"&COUNTA(E{c5a}:E{c5b})'),
                    CongThuc(f"SUM(F{c5a}:F{c5b})"), "", CongThuc(f"SUM(H{c5a}:H{c5b})"), ""],
                   giua=(5, 6, 8), dam=(2, 5, 6, 8))
    s.r += 1
    s.muc("B. Chi tiết phần chưa dựng trên từng màn (§ = mục trong đặc tả)")
    s.dau_bang(["#", "Mục menu", "Phần chưa dựng", "", "", "", "", "Nguồn", ""], gop=[(3, 7), (8, 9)])
    la, n = s.r, 0
    for m in sl["menu"]:
        if not m["co_man"]:
            continue
        if m["chua_dung"] is None:
            ds, nguon = ["Màn chưa khai danh sách phần chưa dựng (PHAN_CHUA_DUNG)"], "Không khai — chưa đếm được"
        else:
            ds, nguon = m["chua_dung"], "Màn tự khai (PHAN_CHUA_DUNG)"
        for it in ds:
            n += 1
            s.dong([n, m["nhan"], it, None, None, None, None, nguon, None], giua=(1,), soc=n % 2 == 0,
                   gop=[(3, 7), (8, 9)])
    lb = s.r - 1
    for m in sl["menu"]:
        r = hang_menu[m["nhan"]]
        dem = (CongThuc(f"COUNTIF($B${la}:$B${lb},B{r})") if m["co_man"] and m["chua_dung"] is not None
               else 0)
        s.o[(r, 8)] = (dem, s.o[(r, 8)][1])
    s.dong_bang = f"C{c5a}"
    if v.get("web_ghi_chu_ky_thuat"):
        s.r += 1
        s.muc("C. Ghi chú kỹ thuật Web Admin")
        for t in v["web_ghi_chu_ky_thuat"]:
            s.ghi_chu("• " + t, nghieng=False)

    # ---- 6. Mini App ----------------------------------------------------------------------
    s = sh[S6]
    s.tieu_de("ZALO MINI APP — KÊNH CÔNG DÂN", v["mini_app"]["mo_ta"])
    dem_ma = 0
    for nh in v["mini_app"].get("nhom") or []:
        s.muc(nh["ten"])
        s.dau_bang(hdr(S6))
        for i, x in enumerate(nh.get("dong") or [], 1):
            dem_ma += 1
            s.dong([i, x["ten"], x["noi_dung"], x["trang_thai"], x.get("ghi_chu", "")],
                   tt=(4,), giua=(1, 4), dam=(2,), soc=i % 2 == 0)
        s.r += 1

    # ---- 7. Deployment --------------------------------------------------------------------
    s = sh[S7]
    dp = v["deploy"]
    s.tieu_de("DEPLOYMENT — CI/CD & KUBERNETES", dp["mo_ta"])
    s.muc("A. Thành phần triển khai (đọc từ kho: Dockerfile, Jenkinsfile, deploy/base/<đơn vị>)")
    s.dau_bang(hdr(S7))
    for i, d in enumerate(sl["don_vi"], 1):
        s.dong([i, d["ten"], d["loai"], d["dockerfile"], d["jenkinsfile"], d["manifest"],
                (dp.get("ghi_chu_don_vi") or {}).get(d["ten"], "")],
               tt=(4, 5, 6), giua=(1, 4, 5, 6), dam=(2,), soc=i % 2 == 0)
    s.r += 1
    s.muc("B. Checklist đưa lên môi trường (theo thứ tự thực hiện)")
    s.dau_bang(["#", "Bước", "Nội dung", "Trạng thái", "", "", "Ghi chú / điều kiện xanh"], gop=[(4, 6)])
    ka = s.r
    for i, x in enumerate(dp["checklist"], 1):
        s.dong([i, x["buoc"], x["noi_dung"], x["trang_thai"], None, None, x.get("ghi_chu", "")],
               tt=(4,), giua=(1, 4), dam=(2,), soc=i % 2 == 0, gop=[(4, 6)])
    kb_ = s.r - 1
    tong7 = s.dong(["", "Tiến độ checklist",
                    CongThuc(f'COUNTIF(D{ka}:D{kb_},"Xong")&" / "&COUNTA(D{ka}:D{kb_})&" bước xong"'),
                    None, None, None, ""], dam=(2, 3), gop=[(3, 6)])
    if dp.get("rui_ro"):
        s.r += 1
        s.muc("C. Rủi ro vận hành cần biết")
        for t in dp["rui_ro"]:
            s.ghi_chu("• " + t, nghieng=False)

    # ---- 1. Tổng quan (dựng sau cùng vì trỏ vào các sheet kia) ------------------------------
    s = sh[S1]
    s.tieu_de("ViGov v2 — BÁO CÁO TIẾN ĐỘ KỸ THUẬT",
              f"Ngày xuất: {ngay} · Commit {sl['commit']} · Số liệu đếm được sinh từ mã, hợp đồng "
              "kb/20-contracts/openapi.json và sổ kb/90-ephemeral/tien-do/; phần nhận định tóm tắt từ "
              "sổ tiến độ. Ô công thức tự cập nhật khi sửa các sheet chi tiết.")
    s.muc("Tóm tắt")
    s.ghi_chu(v["tom_tat"], nghieng=False)
    s.dat(s.r - 1, 1, v["tom_tat"], k(10, b=True, mau=NAVY))
    s.r += 1
    s.muc("Chỉ số chính")
    s.dau_bang(hdr(S1), gop=[(2, 5)])
    kpi = [
        ("Phân hệ trong đặc tả (menu)", f"COUNTA({q(S2)}!B{c2a}:B{c2b})", S2, True),
        *[(f"   · {md}", f'COUNTIF({q(S2)}!I{c2a}:I{c2b},"{md}")', S2, False) for md in MUC_DO],
        ("Tuyến API REST trong hợp đồng", f"COUNTA({q(S4)}!C{c4a}:C{c4b})", S4, True),
        ("   · Tuyến cán bộ đã có màn Web Admin gọi",
         f'COUNTIF({q(S4)}!J{c4a}:J{c4b},"Có")&" / "&(COUNTA({q(S4)}!C{c4a}:C{c4b})'
         f'-COUNTIF({q(S4)}!G{c4a}:G{c4b},"Chỉ công dân*"))', S4, False),
        ("   · Kênh công dân (Mini App gọi / tổng)",
         f'COUNTIF({q(S4)}!J{c4a}:J{c4b},"Mini App")&" / "&COUNTIF({q(S4)}!G{c4a}:G{c4b},"Chỉ công dân*")',
         S4, False),
        ("Mục menu Web Admin có màn thật", f"{q(S5)}!E{tong5}", S5, True),
        ("Chức năng đã dựng trên các màn web", f"{q(S5)}!F{tong5}", S5, True),
        ("Phần đặc tả chưa dựng trên các màn web", f"{q(S5)}!H{tong5}", S5, True),
        ("Checklist deployment", f"{q(S7)}!C{tong7}", S7, True),
    ]
    for i, (nhan, ct, dich, dam) in enumerate(kpi):
        r = s.dong([nhan, CongThuc(ct), None, None, None, dich], giua=(2,), dam=(2,) if dam else (),
                   soc=i % 2 == 1, gop=[(2, 5)],
                   font_rieng={6: {"mau": "0563C1", "u": True}})
        s.lien_ket.append((f"F{r}", f"{q(dich)}!A1"))
    s.r += 1
    s.muc("Điểm cần lưu ý")
    for i, t in enumerate(v["luu_y"], 1):
        s.ghi_chu(f"{i}. {t}", nghieng=False)
    s.r += 1
    s.muc("Sổ tiến độ theo module mã nguồn (số hạng mục công việc)")
    s.dau_bang(["Module", "Xong", "Đang xử lý", "Chưa làm", "Cập nhật lần cuối", ""])
    ma_, mb_ = s.r, s.r + len(sl["so_module"]) - 1
    for i, (mod, a, b, cl, cn) in enumerate(sl["so_module"]):
        s.dong([mod, a, b, cl, cn, ""], giua=(2, 3, 4, 5), soc=i % 2 == 1)
    s.dong(["Tổng", CongThuc(f"SUM(B{ma_}:B{mb_})"), CongThuc(f"SUM(C{ma_}:C{mb_})"),
            CongThuc(f"SUM(D{ma_}:D{mb_})"), CongThuc(f'SUM(B{ma_}:D{mb_})&" hạng mục"'), ""],
           giua=(2, 3, 4, 5), dam=(1, 2, 3, 4, 5))
    s.ghi_chu("Số hạng mục KHÔNG phải % hoàn thành sản phẩm — mỗi hạng mục to nhỏ khác nhau. Dùng để thấy "
              "khối lượng đang mở, không dùng để tính %.")
    s.r += 1
    s.muc("Cách đọc file")
    for t in ["Sheet 2 — Chức năng theo từng menu: đã làm gì, còn thiếu gì, đang xử lý gì.",
              "Sheet 3 & 4 — Backend: tổng hợp theo service và danh mục đầy đủ từng API (lọc được).",
              "Sheet 5 — Web Admin: từng mục menu, chức năng đã dựng và danh sách chi tiết phần chưa dựng.",
              "Sheet 6 — Mini App: từng giai đoạn và phát hành lên Zalo.",
              "Sheet 7 — Deployment: thành phần, checklist đưa lên cụm, rủi ro vận hành.",
              f"Nguồn: tools/xuat_tien_do.py (format v{PHIEN_BAN_FORMAT}) — lệnh /tien-do-san-pham --excel."]:
        s.ghi_chu("• " + t, nghieng=False)
    s.r += 1
    s.dat(s.r, 1, "Chú giải màu:", k(10, b=True))
    for j, (nhan, mau) in enumerate([("Xong / Cơ bản xong", "xanh"), ("Đang xử lý / hoàn thiện", "vang"),
                                     ("Chưa làm / khởi công", "xam")], 2):
        bg, fg = MAU_TT[mau]
        s.dat(s.r, j, nhan, k(9, b=True, mau=fg, nen=bg, vien="luoi", can="giua"))
    s.cao[s.r] = 28

    return [sh[t] for t, _c in SHEETS], k


def ghi_xlsx(duong: str, cac: list[Sheet], k: KieuO) -> None:
    n = len(cac)
    loc = [(i, s) for i, s in enumerate(cac) if s.loc]
    tep = {
        "[Content_Types].xml":
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
            '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
            '<Default Extension="xml" ContentType="application/xml"/>'
            '<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>'
            '<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>'
            + "".join(f'<Override PartName="/xl/worksheets/sheet{i}.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>'
                      for i in range(1, n + 1)) + "</Types>",
        "_rels/.rels":
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
            '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>'
            "</Relationships>",
        "xl/workbook.xml":
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" '
            'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
            '<bookViews><workbookView activeTab="0"/></bookViews><sheets>'
            + "".join(f'<sheet name="{escape(s.ten)}" sheetId="{i}" r:id="rId{i}"/>'
                      for i, s in enumerate(cac, 1))
            + "</sheets>"
            + ("<definedNames>" + "".join(
                f'<definedName name="_xlnm._FilterDatabase" localSheetId="{i}" hidden="1">'
                f"{escape(q(s.ten))}!${s.loc.split(':')[0][0]}${s.loc.split(':')[0][1:]}:"
                f"${s.loc.split(':')[1][0]}${s.loc.split(':')[1][1:]}</definedName>"
                for i, s in loc) + "</definedNames>" if loc else "")
            + '<calcPr calcId="191029" fullCalcOnLoad="1"/></workbook>',
        "xl/_rels/workbook.xml.rels":
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
            + "".join(f'<Relationship Id="rId{i}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet{i}.xml"/>'
                      for i in range(1, n + 1))
            + f'<Relationship Id="rId{n + 1}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>'
            "</Relationships>",
    }
    for i, s in enumerate(cac, 1):
        tep[f"xl/worksheets/sheet{i}.xml"] = s.xml()
    tep["xl/styles.xml"] = k.xml()   # sau cùng: các sheet thêm kiểu trong lúc sinh XML

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


def _chan_trong_kho(duong: str) -> bool:
    if git_bo_qua(duong) is False:
        print(f"[xuat_tien_do] ĐỎ — {duong} nằm trong kho và git KHÔNG bỏ qua nó. Tệp sinh ra "
              "không được vào git. Để mặc định (tmp/) hoặc một đường ngoài kho.", file=sys.stderr)
        return True
    return False


def main(argv: list[str]) -> int:
    hom_nay = datetime.date.today().isoformat()
    mac_json = os.path.join(ROOT, "tmp", "tien-do", f"noi-dung-{hom_nay}.json")
    mac_xlsx = os.path.join(ROOT, "tmp", "tien-do", f"vigov-tien-do-{hom_nay}.xlsx")
    khung = "--khung" in argv
    tu_tep = None
    con: list[str] = []
    it = iter(argv)
    for a in it:
        if a == "--khung":
            continue
        if a == "--tu":
            tu_tep = next(it, None)
            continue
        con.append(a)

    try:
        sl = thu_thap()
    except Exception as ex:
        print(f"[xuat_tien_do] ĐỎ — {ex}. Chạy `make kb` trước.", file=sys.stderr)
        return 1

    if khung:
        duong = con[0] if con else mac_json
        if _chan_trong_kho(duong):
            return 4
        ghi_khung(duong, sl)
        print(f"[xuat_tien_do] khung nội dung: {duong} — {len(sl['chuong'])} chương · "
              f"{len(sl['menu'])} menu · {len(sl['services'])} service. Điền khoá `viet`, rồi chạy "
              f"--tu {duong}")
        return 0

    if not tu_tep:
        print("[xuat_tien_do] ĐỎ — thiếu phần chữ. Bản Excel có hai bước: `--excel --khung` sinh khung "
              "+ bằng chứng, agent viết khoá `viet`, rồi `--excel --tu <tệp.json>`. Chạy qua lệnh "
              "/tien-do-san-pham --excel.", file=sys.stderr)
        return 5
    try:
        nd = json.loads(doc(tu_tep))
        v = nd["viet"]
    except Exception as ex:
        print(f"[xuat_tien_do] ĐỎ — không đọc được {tu_tep}: {ex}", file=sys.stderr)
        return 5
    loi = kiem_viet(v, sl)
    if loi:
        print(f"[xuat_tien_do] ĐỎ — phần chữ chưa đạt ({len(loi)} chỗ), KHÔNG ghi tệp:", file=sys.stderr)
        for x in loi[:60]:
            print("  " + x, file=sys.stderr)
        if len(loi) > 60:
            print(f"  … và {len(loi) - 60} chỗ nữa", file=sys.stderr)
        return 5
    if nd.get("commit", "").split(" ")[0] != sl["commit"].split(" ")[0]:
        print(f"[xuat_tien_do] lưu ý — phần chữ viết ở commit {nd.get('commit')}, kho đang ở "
              f"{sl['commit']}. Số liệu là của hôm nay; rà lại phần chữ nếu mã đã đổi nhiều.")

    duong = con[0] if con else mac_xlsx
    if not duong.lower().endswith(".xlsx"):
        duong += ".xlsx"
    if _chan_trong_kho(duong):
        return 4
    cac, k = dung(sl, v, datetime.datetime.now().strftime("%d/%m/%Y %H:%M"))

    trung = [f"'{s.ten}'!{ten_cot(j - 1)}{r}" for s in cac for (r, j), (val, _st) in s.o.items()
             if isinstance(val, str) and not isinstance(val, CongThuc) and co_du_lieu_ca_nhan(val)]
    if trung:
        print("[xuat_tien_do] ĐỎ — ô trông như số điện thoại / CCCD thật, KHÔNG ghi tệp (luật 3):",
              file=sys.stderr)
        print("  " + ", ".join(trung[:20]) + (" …" if len(trung) > 20 else ""), file=sys.stderr)
        print("  Sửa nguồn — không bao giờ in giá trị ra đây.", file=sys.stderr)
        return 3

    try:
        ghi_xlsx(duong, cac, k)
    except PermissionError:
        # Windows khoá tệp đang mở trong Excel. Người chạy lệnh này thường vừa mở bản hôm nay.
        print(f"[xuat_tien_do] ĐỎ — không ghi được {duong}: tệp đang mở (thường là trong Excel). "
              "Đóng tệp rồi chạy lại, hoặc truyền một đường dẫn khác.", file=sys.stderr)
        return 1
    print(f"[xuat_tien_do] {os.path.relpath(duong, ROOT) if git_bo_qua(duong) is not None else duong}"
          f" — format v{PHIEN_BAN_FORMAT} · {len(sl['chuong'])} chương · {len(sl['tuyen'])} tuyến · "
          f"{len(sl['menu'])} menu · {dem_menu_co_man(sl)} màn")
    return 0


def dem_menu_co_man(sl: dict) -> int:
    return sum(1 for m in sl["menu"] if m["co_man"])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
