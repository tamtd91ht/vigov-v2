#!/usr/bin/env python3
"""Sinh `kb/90-ephemeral/tien-do-san-pham.md` — tiến độ theo SẢN PHẨM, không theo module.

VÌ SAO CÓ TỆP NÀY BÊN CẠNH `tien_do.py`, chứ không gộp vào: hai tệp trả lời hai câu khác nhau,
cho hai người đọc khác nhau, và gộp lại thì mỗi câu đều kém đi.

    tien-do.md          "MODULE nào còn nợ gì"     — trục KHO MÃ, người đọc là phiên sau
    tien-do-san-pham.md "PHÂN HỆ nào dùng được"    — trục SẢN PHẨM, người đọc là chủ dự án

Một chủ dự án hỏi "màn Nhiệm vụ xong chưa" không đọc được câu trả lời từ một bảng xếp theo
`service-petitions` — phân hệ Nhiệm vụ nằm rải ở `service-petitions`, `web-admin` và một phần
`service-identity`.

═══════════════════════════════════════════════════════════════════════════════════════════
KHÔNG MỘT CON SỐ NÀO Ở ĐÂY ĐƯỢC VIẾT TAY, và đó là ràng buộc chính của tệp này (luật 9 #1).

Phép thử một dòng của luật 9: *"xoá dòng này đi, một công cụ có dựng lại được từ mã nguồn
không?"* — với mọi cột dưới đây câu trả lời là CÓ, nên viết tay là bị cấm:

    chương đặc tả      docs/ui-ux/NN-*.md                     (tên tệp + tiêu đề `# `)
    tuyến hợp đồng     kb/20-contracts/openapi.json           (`x-vigov-screen` của từng thao tác)
    đã có màn gọi      tasks/web/done/ vs open/               (`x-vigov-task` là id việc)
    mục menu           web-admin/src/components/muc-menu.ts
    màn hình có thật   web-admin/src/app/**/page.tsx
    phần chưa dựng     PHAN_CHUA_DUNG trong features/*/nhan-*.ts
    trạng thái sổ      kb/90-ephemeral/tien-do/<module>.json

⚠ VÀ KHÔNG CÓ CỘT "% HOÀN THÀNH" TỰ NGHĨ RA. Ngày 23/09/2026 chủ dự án đọc một con số tiến độ
rồi nói *"vậy mà tôi tưởng làm xong hết rồi"* — con số ấy đếm MỤC VIỆC trong sổ, không đếm sản
phẩm. Một tỷ lệ bịa ra trông chính xác hơn hẳn thứ nó biết, và người đọc không có cách nào thấy
nó bịa. Nên cột duy nhất mang hình dạng tỷ lệ ở đây là `ĐÃ GỌI / TUYẾN`, một phép chia giữa hai
con số ĐẾM ĐƯỢC, và định nghĩa của nó in ngay trên bảng.

Hệ quả phải chấp nhận, nói thẳng thay vì giấu: tỷ lệ ấy đo BỀ MẶT MÁY CHỦ ĐÃ CÓ MÀN GỌI TỚI.
Nó KHÔNG biết màn ấy dựng tới đâu — đó là việc của cột `CHƯA DỰNG` bên cạnh — và nó KHÔNG biết
phần nào của đặc tả chưa có tuyến nào. Một chương 100% vẫn có thể còn nửa đặc tả chưa ai chạm.
═══════════════════════════════════════════════════════════════════════════════════════════

Chạy:  python tools/tien_do_san_pham.py      (đi kèm `make kb`)
Mã thoát: 0 = sinh xong · 1 = thiếu một nguồn bắt buộc.
"""

from __future__ import annotations

import datetime
import glob
import io
import json
import os
import re
import sys

for _s in (sys.stdout, sys.stderr):
    try:
        _s.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
RA = os.path.join(ROOT, "kb", "90-ephemeral", "tien-do-san-pham.md")

HOP_DONG = os.path.join(ROOT, "kb", "20-contracts", "openapi.json")
MUC_MENU = os.path.join(ROOT, "web-admin", "src", "components", "muc-menu.ts")
DAC_TA = os.path.join(ROOT, "docs", "ui-ux")

# Hạn đo LẦN CUỐI CÓ NGƯỜI CẬP NHẬT một module, không đo ngày sinh tệp — cùng lý do `tien_do.py`
# ghi ở hằng số của nó: một tệp sinh lại mỗi lần `make kb` thì hạn tính theo ngày sinh không bao
# giờ tới, nên nó không nói được điều gì.
HAN_NGAY = 90


def doc(p: str) -> str:
    try:
        return io.open(p, encoding="utf-8").read()
    except Exception:
        return ""


# ---------------------------------------------------------------------------------------------
# NGUỒN 1 — các chương đặc tả. Lấy từ TÊN TỆP chứ không từ một danh sách viết tay, vì một chương
# mới thêm vào `docs/ui-ux/` phải tự xuất hiện ở đây. Chương KHÔNG có tuyến nào vẫn phải có mặt:
# đó đúng là những chương chưa ai khởi công, tức thứ người đọc bảng này cần thấy nhất.
MAU_CHUONG = re.compile(r"^(\d{2})-([a-z0-9-]+)\.md$")


def cac_chuong() -> list[tuple[str, str, str]]:
    """[(số, slug, tiêu đề)] theo thứ tự chương."""
    ra = []
    for ten in sorted(os.listdir(DAC_TA)):
        m = MAU_CHUONG.match(ten)
        if not m:
            continue
        tieu_de = ""
        for dong in doc(os.path.join(DAC_TA, ten)).split("\n"):
            if dong.startswith("# "):
                tieu_de = dong[2:].strip()
                break
        ra.append((m.group(1), f"{m.group(1)}-{m.group(2)}", tieu_de))
    return ra


# ---------------------------------------------------------------------------------------------
# NGUỒN 2 — tuyến hợp đồng, gom theo chương qua `x-vigov-screen`.
PHUONG_THUC = ("get", "post", "patch", "delete", "put")


def tuyen_theo_chuong() -> tuple[dict[str, list[dict]], int, int]:
    d = json.loads(doc(HOP_DONG) or "{}")
    theo: dict[str, list[dict]] = {}
    tong = 0
    khong_khai = 0
    for duong, muc in (d.get("paths") or {}).items():
        for pt, op in muc.items():
            if pt not in PHUONG_THUC:
                continue
            tong += 1
            man = (op.get("x-vigov-screen") or "").strip()
            # `x-vigov-screen` vắng hoặc ghi "(chưa …)" nghĩa là tuyến chưa khai màn nào dùng nó.
            # Gom riêng thay vì nhét vào một chương nào đó: đoán ở đây là bịa một con số.
            khoa = man.split()[0] if man and not man.startswith("*") else ""
            if not khoa:
                khong_khai += 1
                continue
            theo.setdefault(khoa, []).append({
                "pt": pt.upper(),
                "duong": duong,
                "viec": op.get("x-vigov-task") or "",
                # `kind` là thứ chia BỀ MẶT một cách đo được: `citizen-only` là kênh công dân,
                # còn lại là bề mặt cán bộ. Đọc từ hợp đồng chứ không đoán theo tên đường dẫn —
                # `my-citizen-reports` tình cờ có chữ `citizen`, nhưng `citizen.verify` thì không.
                "kind": ((op.get("x-vigov-permission") or {}).get("kind") or ""),
            })
    return theo, tong, khong_khai


def la_cong_dan(t: dict) -> bool:
    return t["kind"] == "citizen-only"


# ---------------------------------------------------------------------------------------------
# NGUỒN 3 — việc màn hình đã xong. `tools/apidoc` suy trạng thái TỪ MÃ (có hàm gọi trong
# `web-admin/src/lib/api/**`) chứ không từ một thao tác đổi tên tệp thủ công — xem
# `tools/apidoc/manhinh.go`. Nên `done/` ở đây là một phép đo, không phải một lời khai.
def viec_da_xong() -> tuple[set[str], set[str]]:
    """(việc đã xong, MỌI việc màn web đã biết tới).

    TẬP THỨ HAI LÀ MẪU SỐ, VÀ ĐÓ LÀ CHỖ CON SỐ NÀY SUÝT NÓI DỐI. Một tuyến chỉ thuộc bề mặt WEB
    khi `tools/apidoc` có sinh ra một tệp việc màn hình cho nó. Hai tuyến `my-citizen-reports`
    của chương 09 là tuyến KÊNH CÔNG DÂN — chúng không có tệp việc nào, vì web quản trị không
    bao giờ gọi chúng. Đếm chúng vào mẫu số cho ra `6/8` và đọc thành "chương 09 còn thiếu hai
    màn", trong khi web đã gọi đủ `6/6` phần thuộc về nó. Đo và sửa 24/09/2026.
    """
    xong, biet = set(), set()
    for trang_thai in ("done", "open", "claimed", "stale"):
        for p in glob.glob(os.path.join(ROOT, "tasks", "web", trang_thai, "*.json")):
            ma = os.path.basename(p)[:-5]
            biet.add(ma)
            if trang_thai == "done":
                xong.add(ma)
    return xong, biet


# ---------------------------------------------------------------------------------------------
# NGUỒN 4 — mục menu của web-admin.
MAU_MUC = re.compile(r"\{\s*nhan:\s*\"([^\"]+)\",\s*duong:\s*([^,]+),\s*khoa:\s*([^\s}]+)\s*\}")


def cac_muc_menu() -> list[tuple[str, str | None, str | None]]:
    ra = []
    for m in MAU_MUC.finditer(doc(MUC_MENU)):
        duong = m.group(2).strip()
        khoa = m.group(3).strip()
        ra.append((
            m.group(1),
            None if duong == "null" else duong.strip('"'),
            None if khoa == "null" else khoa,
        ))
    return ra


# ---------------------------------------------------------------------------------------------
# NGUỒN 5 — phần CHƯA DỰNG ĐƯỢC mà chính màn hình đang hiện ra.
#
# Đếm từ `PHAN_CHUA_DUNG` chứ không từ một danh sách trong tài liệu, vì mảng ấy là thứ RA TỚI
# TRANG: mỗi màn có một ca kiểm đòi từng mục phải có trong HTML. Tức con số này là số câu mà cán
# bộ thật sự đọc được trên màn, không phải số việc ai đó nhớ ra lúc viết tài liệu.
MAU_TEN = re.compile(r"^\s*ten:\s*\"", re.M)


def so_chua_dung(thu_muc: str) -> int | None:
    """Số mục chưa dựng của một thư mục `features/`, hoặc `None` khi màn KHÔNG KHAI khối ấy.

    `None` KHÁC `0`, và gộp hai thứ lại là chỗ bảng này nói dối dễ nhất: `0` nghĩa là màn có
    khai một danh sách và danh sách ấy rỗng — tức mọi phần của đặc tả đã dựng. `None` nghĩa là
    màn chưa bao giờ khai, nên không ai biết nó còn thiếu gì. Ba màn đang ở trạng thái sau
    (`van-ban`, `danh-ba`, `cau-hinh`), và in `—` cho cả hai sẽ đọc ra thành "xong rồi".

    Quét MỌI tệp nguồn trong thư mục, không chỉ `nhan-*.ts`: chỗ đặt hằng số là quy ước, và một
    quy ước không có rào nào canh thì một màn đặt chỗ khác là con số ở đây tụt về 0 trong im lặng.
    """
    thu = os.path.join(ROOT, "web-admin", "src", "features", thu_muc)
    if not os.path.isdir(thu):
        return None
    tong = 0
    thay = False
    for goc, _d, tep in os.walk(thu):
        for t in sorted(tep):
            if not t.endswith((".ts", ".tsx")) or t.endswith((".test.ts", ".test.tsx")):
                continue
            s = doc(os.path.join(goc, t))
            i = s.find("PHAN_CHUA_DUNG")
            if i < 0:
                continue
            thay = True
            tong += len(MAU_TEN.findall(s[i:]))
    return tong if thay else None


# Một trang gọi vào những thư mục `features/` nào — đọc từ chính câu `import`, nên một màn đổi
# chỗ lấy thành phần là bảng này đi theo, không cần ai sửa.
MAU_IMPORT_FEATURE = re.compile(r"from\s+\"@/features/([a-z0-9-]+)/")


def feature_cua_trang(duong: str) -> list[str]:
    p = os.path.join(ROOT, "web-admin", "src", "app", duong.strip("/").replace("/", os.sep),
                     "page.tsx")
    s = doc(p)
    if not s:
        return []
    return sorted(set(MAU_IMPORT_FEATURE.findall(s)))


def co_trang(duong: str) -> bool:
    return bool(doc(os.path.join(ROOT, "web-admin", "src", "app",
                                 duong.strip("/").replace("/", os.sep), "page.tsx")))


# ---------------------------------------------------------------------------------------------
# BỘ LỌC — `[BỀ MẶT][-PHÂN HỆ]`, mỗi vế bỏ được.
#
#     API-Nhiệm vụ      WebAdmin-Giải ngân      Miniapp      02      webadmin
#
# CHUẨN HOÁ MẠNH TAY, và có lý do: tên phân hệ là tiếng Việt CÓ DẤU và có khoảng trắng, gõ trên
# dòng lệnh thì mỗi người gõ một kiểu. Bỏ dấu, bỏ hoa thường, bỏ mọi ký tự không phải chữ số —
# nên `API-Nhiệm vụ`, `api nhiem vu`, `APINhiemVu` là một.
DAU = {
    "à": "a", "á": "a", "ả": "a", "ã": "a", "ạ": "a", "ă": "a", "ằ": "a", "ắ": "a", "ẳ": "a",
    "ẵ": "a", "ặ": "a", "â": "a", "ầ": "a", "ấ": "a", "ẩ": "a", "ẫ": "a", "ậ": "a",
    "è": "e", "é": "e", "ẻ": "e", "ẽ": "e", "ẹ": "e", "ê": "e", "ề": "e", "ế": "e", "ể": "e",
    "ễ": "e", "ệ": "e", "ì": "i", "í": "i", "ỉ": "i", "ĩ": "i", "ị": "i",
    "ò": "o", "ó": "o", "ỏ": "o", "õ": "o", "ọ": "o", "ô": "o", "ồ": "o", "ố": "o", "ổ": "o",
    "ỗ": "o", "ộ": "o", "ơ": "o", "ờ": "o", "ớ": "o", "ở": "o", "ỡ": "o", "ợ": "o",
    "ù": "u", "ú": "u", "ủ": "u", "ũ": "u", "ụ": "u", "ư": "u", "ừ": "u", "ứ": "u", "ử": "u",
    "ữ": "u", "ự": "u", "ỳ": "y", "ý": "y", "ỷ": "y", "ỹ": "y", "ỵ": "y", "đ": "d",
}

# Mỗi bề mặt kèm MỌI cách gọi nó mà người ta sẽ gõ. Thà nhận rộng còn hơn từ chối một chữ đúng
# nghĩa rồi bắt người dùng đi đọc mã để biết mình phải gõ gì.
BE_MAT = {
    "api": ("api", "backend", "may chu", "maychu", "go"),
    "webadmin": ("webadmin", "web", "admin", "webquantri"),
    "miniapp": ("miniapp", "mini", "app", "congdan", "zalo", "citizenapp"),
}


def gon(s: str) -> str:
    s = s.lower()
    return "".join(c for c in ("".join(DAU.get(c, c) for c in s)) if c.isalnum())


def doc_bo_loc(tham: str, chuong: list[tuple[str, str, str]]) -> tuple[str | None, str | None]:
    """('api'|'webadmin'|'miniapp'|None, slug chương|None). Ném ValueError khi không hiểu.

    KHÔNG HIỂU THÌ ĐỎ, không âm thầm in ra toàn bộ: một người gõ nhầm `WebAdim-Nhiem vu` mà nhận
    được cả bảng sẽ đọc nó như bảng của riêng phân hệ mình hỏi. Đây đúng lớp "xanh vì lý do sai"
    mà kho này đã trả giá nhiều lần.
    """
    g = gon(tham)
    if not g:
        return None, None

    be = None
    for ten, cach_goi in BE_MAT.items():
        for cg in sorted(cach_goi, key=len, reverse=True):
            if g.startswith(cg):
                be, g = ten, g[len(cg):]
                break
        if be:
            break

    if not g:
        return be, None

    # Còn lại là phân hệ: khớp theo SỐ chương, theo slug, hoặc theo tiêu đề.
    for so_ch, slug, tieu_de in chuong:
        ten_sach = re.sub(r"^\d{2}\s*[—-]\s*", "", tieu_de or "")
        if g in (so_ch, gon(slug), gon(ten_sach)) or (len(g) >= 4 and g in gon(slug)):
            return be, slug

    raise ValueError(g)


# ---------------------------------------------------------------------------------------------
# NGUỒN 6 — sổ tiến độ theo module, để cột cuối trỏ về được chỗ có chi tiết.
def so_tien_do() -> dict[str, dict[str, int]]:
    ra: dict[str, dict[str, int]] = {}
    for p in sorted(glob.glob(os.path.join(ROOT, "kb", "90-ephemeral", "tien-do", "*.json"))):
        try:
            d = json.loads(doc(p))
        except Exception:
            continue
        dem = {"xong": 0, "dang_lam": 0, "chua_lam": 0, "treo": 0}
        for x in d.get("muc", []):
            if x.get("trang_thai") in dem:
                dem[x["trang_thai"]] += 1
        ra[d.get("module", os.path.basename(p)[:-5])] = dem
    return ra


def main() -> int:
    if not os.path.exists(HOP_DONG):
        print("[tien_do_san_pham] ĐỎ — thiếu kb/20-contracts/openapi.json. Chạy `make kb` trước.",
              file=sys.stderr)
        return 1
    if not os.path.isdir(DAC_TA):
        print(f"[tien_do_san_pham] ĐỎ — thiếu {os.path.relpath(DAC_TA, ROOT)}.", file=sys.stderr)
        return 1

    chuong = cac_chuong()
    theo_chuong, tong_tuyen, khong_khai = tuyen_theo_chuong()
    xong, biet_web = viec_da_xong()
    menu = cac_muc_menu()
    so = so_tien_do()

    # BỘ LỌC ĐỌC SAU KHI CÓ DANH SÁCH CHƯƠNG, vì tên phân hệ được đối chiếu với chương thật —
    # không có bảng tên viết tay nào, nên thêm một chương vào `docs/ui-ux/` là lọc được ngay.
    tham_loc = " ".join(sys.argv[1:]).strip()
    try:
        be, slug_loc = doc_bo_loc(tham_loc, chuong)
    except ValueError as ex:
        print(f"[tien_do_san_pham] ĐỎ — không hiểu bộ lọc {tham_loc!r} (phần {str(ex)!r}).",
              file=sys.stderr)
        print("  Bề mặt: API · WebAdmin · Miniapp — bỏ trống là cả ba.", file=sys.stderr)
        print("  Phân hệ: số chương, hoặc tên chương. Ví dụ đúng:", file=sys.stderr)
        print("      API-Nhiệm vụ · WebAdmin-Giải ngân · Miniapp · 02 · webadmin",
              file=sys.stderr)
        print("  Có dấu hay không, hoa hay thường, gạch nối hay khoảng trắng đều được.",
              file=sys.stderr)
        # KHÔNG âm thầm in toàn bộ khi gõ nhầm: người gõ `WebAdim-Nhiem vu` mà nhận được cả bảng
        # sẽ đọc nó như bảng của riêng phân hệ mình hỏi.
        return 1

    # HỎNG THÌ ĐỎ. Đọc được 0 chương hay 0 tuyến nghĩa là một biểu thức đọc hụt, và một bảng rỗng
    # in ra "chưa làm gì cả" là câu sai theo chiều tệ nhất.
    if not chuong or tong_tuyen == 0 or not menu:
        print(f"[tien_do_san_pham] ĐỎ — đọc hụt: {len(chuong)} chương · {tong_tuyen} tuyến · "
              f"{len(menu)} mục menu. Sửa biểu thức đọc, ĐỪNG in một bảng rỗng.", file=sys.stderr)
        return 1

    hom_nay = datetime.date.today()
    han = (hom_nay + datetime.timedelta(days=HAN_NGAY)).isoformat()

    L_dau: list[str] = []
    L = L_dau
    L.append("---")
    L.append("id: tien-do-san-pham")
    L.append("tier: T5")
    L.append("source: GENERATED")
    L.append("owner: architecture")
    L.append(f"derived_from_commit: {os.environ.get('VIGOV_COMMIT', '')}")
    L.append(f"expires: {han}")
    L.append("owns_facts:")
    L.append('  - "tiến độ theo PHÂN HỆ SẢN PHẨM: chương đặc tả nào có bao nhiêu tuyến, bao nhiêu '
             'tuyến đã có màn gọi, và mỗi mục menu web-admin đang ở đâu"')
    L.append("---")
    L.append("")
    L.append("# Tiến độ theo sản phẩm")
    L.append("")
    L.append("**SINH RA — đừng sửa tệp này.** Sửa nguồn rồi chạy `make kb`. Mọi con số dưới đây "
             "đọc từ mã;")
    L.append("không dòng nào viết tay (luật 9, bất biến 1).")
    L.append("")
    L.append("Trục ở đây là **phân hệ sản phẩm**. Trục theo **module kho mã** nằm ở "
             "`kb/90-ephemeral/tien-do.md`;")
    L.append("hai tệp trả lời hai câu khác nhau và không chép của nhau.")
    L.append("")
    L.append(f"Sinh ngày **{hom_nay.isoformat()}** · hết hạn **{han}**.")
    L.append("")

    p1: list[str] = []
    L = p1
    L.append("## 1 · Theo chương đặc tả")
    L.append("")
    L.append("`ĐÃ GỌI / TUYẾN` là **một phép chia giữa hai con số đếm được**: bao nhiêu tuyến của "
             "chương ấy")
    L.append("đã có mã client gọi tới, trên tổng số tuyến chương ấy khai trong hợp đồng. Nó "
             "**không phải** một")
    L.append("ước lượng hoàn thành — nó không biết màn dựng tới đâu (xem cột **chưa dựng**) và "
             "không biết")
    L.append("phần nào của đặc tả chưa có tuyến nào. Một chương `4/4` vẫn có thể còn nửa đặc tả "
             "chưa ai chạm.")
    L.append("")
    L.append("| Chương | Tuyến | Đã gọi | Màn web |")
    L.append("|---|---|---|---|")

    # Ghép chương -> mục menu qua các trang web thật sự tồn tại, để cột "màn web" không phải đoán.
    trang_theo_feature: dict[str, str] = {}
    for nhan, duong, _k in menu:
        if duong and co_trang(duong):
            for ft in feature_cua_trang(duong):
                trang_theo_feature.setdefault(ft, duong)

    for so_ch, slug, tieu_de in chuong:
        ds = theo_chuong.get(slug, [])
        n = len(ds)
        # MẪU SỐ CHỈ GỒM TUYẾN THUỘC BỀ MẶT WEB — xem `viec_da_xong`. Tuyến kênh công dân không
        # có tệp việc màn hình nào, và đếm chúng vào đây là trừ điểm web vì việc của app khác.
        web = [t for t in ds if t["viec"] in biet_web]
        khac = n - len(web)
        da = sum(1 for t in web if t["viec"] in xong)
        ty = (f"{da}/{len(web)}" if web else "—") + (f" +{khac} ngoài web" if khac else "")
        # Màn web: chương có tuyến nào đã được gọi thì chắc chắn có mã client; không tuyến nào
        # thì chưa. Suy từ ĐO, không từ một bảng ánh xạ viết tay.
        man = "✓" if da else ("✗" if web else "—")
        # Tiêu đề `# ` của chương đã tự mang số ("02 — Quản lý nhiệm vụ"), nên in cả hai là in
        # số hai lần. Cắt tiền tố, giữ nguyên phần chữ.
        ten = re.sub(r"^\d{2}\s*[—-]\s*", "", tieu_de or slug)
        L.append(f"| **{so_ch}** {ten} | {n or '—'} | {ty} | {man} |")
    L.append("")
    L.append(f"Tổng **{tong_tuyen} tuyến** trong hợp đồng. "
             f"**{khong_khai}** tuyến chưa khai `@screen` nên không gom được vào chương nào — "
             "chúng không mất đi, chỉ chưa nói được mình phục vụ màn nào.")
    L.append("")

    p2: list[str] = []
    L = p2
    L.append("## 2 · web-admin, theo từng mục menu")
    L.append("")
    L.append("`duong: null` nghĩa là mục **cố ý hiện mà không bấm được** — `muc-menu.ts` giải "
             "thích vì sao giữ")
    L.append("chúng thay vì xoá. **Chưa dựng** đếm các mục trong `PHAN_CHUA_DUNG`, tức số câu "
             "cán bộ THẬT SỰ")
    L.append("đọc được trên màn, không phải số việc ai đó nhớ ra lúc viết tài liệu.")
    L.append("")
    L.append("| # | Mục menu | Đường dẫn | Khoá quyền | Màn | Chưa dựng |")
    L.append("|---|---|---|---|---|---|")
    co_man = 0
    tong_chua = 0
    khong_khai_man = 0
    for i, (nhan, duong, khoa) in enumerate(menu, 1):
        if duong is None:
            L.append(f"| {i} | {nhan} | — | — | ✗ | |")
            continue
        that = co_trang(duong)
        if that:
            co_man += 1
        dem = [so_chua_dung(f) for f in feature_cua_trang(duong)]
        co_khai = [x for x in dem if x is not None]
        if co_khai:
            n = sum(co_khai)
            tong_chua += n
            o_chua = str(n)
        else:
            o_chua = "**không khai**"
            khong_khai_man += 1
        L.append(f"| {i} | {nhan} | `{duong}` | `{khoa or '—'}` | {'✓' if that else '⚠'} | "
                 f"{o_chua} |")
    L.append("")
    L.append(f"**{co_man}/{len(menu)}** mục menu có màn thật. **{tong_chua}** phần chưa dựng đang "
             "hiện trên các màn ấy.")
    if khong_khai_man:
        L.append("")
        L.append(f"⚠ **{khong_khai_man} màn KHÔNG KHAI khối `PHAN_CHUA_DUNG`**, và ô của chúng "
                 "đọc là `không khai` chứ không phải `0` —")
        L.append("hai thứ khác hẳn nhau. `0` nghĩa là màn có khai một danh sách và danh sách ấy "
                 "rỗng, tức mọi phần")
        L.append("đặc tả đã dựng. `không khai` nghĩa là **chưa ai nói màn ấy còn thiếu gì**, nên "
                 "con số 0 ở đó sẽ là")
        L.append("một lời trấn an không có gì đứng sau.")
    L.append("")

    # ---- 3 · Mini App công dân -----------------------------------------------------------
    # BỀ MẶT NÀY VẮNG HẲN KHỎI BẢN ĐẦU CỦA TỆP NÀY, và sự vắng mặt ấy tự nó là một con số sai:
    # một bảng "tiến độ toàn dự án" không có kênh công dân đọc ra như thể kênh ấy không tồn tại.
    p3: list[str] = []
    L = p3
    L.append("## 3 · Mini App công dân")
    L.append("")
    cong_dan = [t for ds in theo_chuong.values() for t in ds if la_cong_dan(t)]
    ft_app = sorted(
        d for d in os.listdir(os.path.join(ROOT, "citizen-app", "src", "features"))
        if os.path.isdir(os.path.join(ROOT, "citizen-app", "src", "features", d))
    ) if os.path.isdir(os.path.join(ROOT, "citizen-app", "src", "features")) else []
    goi_vigov = sorted(set(re.findall(
        r'"(/api/v1/[a-z0-9/{}-]+)"',
        "".join(doc(os.path.join(g, t))
                for g, _d, tp in os.walk(os.path.join(ROOT, "citizen-app", "src"))
                for t in tp if t.endswith((".ts", ".tsx")) and not t.endswith(".test.ts")),
    )))
    L.append("| | |")
    L.append("|---|---|")
    L.append(f"| Tuyến ViGov dành riêng kênh công dân (`citizen-only`) | "
             f"{len(cong_dan)} |")
    L.append(f"| Trong đó `citizen-app` đang gọi | "
             f"{sum(1 for t in cong_dan if t['duong'] in goi_vigov)} |")
    L.append(f"| Thư mục tính năng trong `citizen-app/src/features/` | {len(ft_app)} |")
    L.append("")
    if cong_dan:
        L.append("| | Tuyến | `citizen-app` gọi chưa |")
        L.append("|---|---|---|")
        for t in sorted(cong_dan, key=lambda x: x["duong"]):
            L.append(f"| {t['pt']} | `{t['duong']}` | "
                     f"{'✓' if t['duong'] in goi_vigov else '✗'} |")
        L.append("")
    L.append("⚠ **`citizen-app` hôm nay chưa gọi một tuyến ViGov nào**, và đó không phải thiếu "
             "sót của nó: nó đang")
    L.append(f"gọi {', '.join('`' + x + '`' for x in goi_vigov) or '(không tuyến nào)'} — bề mặt "
             "của **kho anh em**")
    L.append("`vihat-miniapp`, không phải của ViGov (CLAUDE.md, mục hai kho). Giai đoạn 2 — màn "
             "nghiệp vụ xã —")
    L.append("bị chặn ở `service-identity`, xem `kb/90-ephemeral/tien-do/citizen-app.json`.")
    L.append("")

    p4: list[str] = []
    L = p4
    L.append("## 4 · Sổ tiến độ theo module")
    L.append("")
    L.append("Chi tiết từng mục — bằng chứng, câu chờ khách, bước kế tiếp — ở "
             "`kb/90-ephemeral/tien-do.md`.")
    L.append("")
    L.append("| Module | xong | đang làm | chưa làm | treo |")
    L.append("|---|---|---|---|---|")
    for mod in sorted(so):
        d = so[mod]
        L.append(f"| `{mod}` | {d['xong']} | {d['dang_lam']} | {d['chua_lam']} | {d['treo']} |")
    L.append("")

    # ---- LẮP RÁP ---------------------------------------------------------------------------
    # KHÔNG LỌC  → ghi tệp sinh ra (đủ bốn phần), như `make kb` vẫn làm.
    # CÓ LỌC     → CHỈ IN RA MÀN, tuyệt đối KHÔNG ghi đè tệp: một bản đã lọc nằm ở
    #              `kb/90-ephemeral/tien-do-san-pham.md` là một tệp trông như toàn cảnh mà chỉ
    #              chứa một phân hệ — đúng loại tài liệu nói dối mà không ai phát hiện được.
    if be is None and slug_loc is None:
        toan = L_dau + p1 + p2 + p3 + p4
        with io.open(RA, "w", encoding="utf-8", newline="\n") as f:
            f.write("\n".join(toan) + "\n")
        print(f"[tien_do_san_pham] {os.path.relpath(RA, ROOT)} — {len(chuong)} chương · "
              f"{tong_tuyen} tuyến · {len(menu)} mục menu · {co_man} có màn")
        return 0

    chon = {"api": [p1], "webadmin": [p2], "miniapp": [p3]}.get(be or "", [p1, p2, p3])
    ra = [f"# Tiến độ — bộ lọc `{tham_loc}`", ""]
    if slug_loc:
        ten_ch = next((re.sub(r"^\d{2}\s*[—-]\s*", "", td or sl)
                       for sc, sl, td in chuong if sl == slug_loc), slug_loc)
        ra.append(f"Phân hệ **{ten_ch}** (`{slug_loc}`)"
                  + (f", bề mặt **{be}**." if be else ", cả ba bề mặt."))
        ra.append("")
        ra += loc_theo_chuong(be, slug_loc, theo_chuong, xong, biet_web, menu)
    else:
        ra.append(f"Bề mặt **{be}**, mọi phân hệ.")
        ra.append("")
        for kh in chon:
            ra += kh
    ra.append("")
    ra.append("Không ghi tệp — bản đã lọc chỉ in ra. Toàn cảnh: `python tools/tien_do_san_pham.py`"
              " (hoặc `make kb`).")
    print("\n".join(ra))
    return 0


def loc_theo_chuong(be, slug, theo_chuong, xong, biet_web, menu) -> list[str]:
    """Phần thân khi bộ lọc nêu ĐÍCH DANH một phân hệ — in từng tuyến, không in bảng tổng.

    Một bảng tổng đã lọc xuống một dòng thì dòng ấy không nói thêm gì so với bảng đầy đủ. Thứ
    người hỏi "API-Nhiệm vụ" thật sự cần là DANH SÁCH TUYẾN và tuyến nào đã có màn gọi.
    """
    ds = theo_chuong.get(slug, [])
    ra: list[str] = []
    ra.append("| | Tuyến | Ai gọi | Đã gọi |")
    ra.append("|---|---|---|---|")
    for t in sorted(ds, key=lambda x: (x["duong"], x["pt"])):
        bm = "công dân" if la_cong_dan(t) else ("web" if t["viec"] in biet_web else "—")
        goi = "✓" if t["viec"] in xong else ("✗" if t["viec"] in biet_web else "—")
        ra.append(f"| {t['pt']} | `{t['duong']}` | {bm} | {goi} |")
    if not ds:
        ra.append("| — | *chương này chưa có tuyến nào trong hợp đồng* | — | — |")
    ra.append("")

    # Mục menu của chính phân hệ ấy, nếu có — ghép qua đường dẫn trang, không qua bảng viết tay.
    dong = [] if be == "api" or be == "miniapp" else [
        f"| {nhan} | `{duong}` | {so_chua_dung_gop(duong)} |"
        for nhan, duong, _k in menu
        if duong and slug.split("-", 1)[1] in duong.strip("/").replace("/", "-")]
    if dong:
        ra.append("| Mục menu | Đường dẫn | Chưa dựng |")
        ra.append("|---|---|---|")
        ra += dong
        ra.append("")
    return ra


def so_chua_dung_gop(duong: str) -> str:
    dem = [so_chua_dung(f) for f in feature_cua_trang(duong)]
    co = [x for x in dem if x is not None]
    return str(sum(co)) if co else "**không khai**"


if __name__ == "__main__":
    sys.exit(main())
