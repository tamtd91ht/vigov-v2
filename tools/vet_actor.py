#!/usr/bin/env python3
"""Đọc xem một `audit.Actor` của CÁN BỘ đang được nạp bằng định danh nào. Chỉ đọc, không phán xử.

VÌ SAO TỆP NÀY TỒN TẠI RIÊNG — cùng lý do `tools/quyen_keys.py` tồn tại riêng: phép kiểm này có
HAI nơi phải chạy, `.claude/hooks/audit_actor_guard.py` lúc ghi một tệp và
`tools/check_audit_actor.py` lúc quét cả kho. Hai bản sao của một bộ phân tích là hai bản sẽ
lệch, và bản lệch là bản im lặng.

LỚP LỖI NÓ BẮT, ĐO CHỨ KHÔNG ĐOÁN. `audit_log.actor_id` trả lời câu "AI làm việc này" trên một
sổ có giá trị pháp lý. Chính sách đã viết tường minh từ khi tuyến đăng nhập ra đời —
`service-identity/internal/app/dang_nhap.go:151`, `Subject: cb.Ma, // business code, never the
internal id` — nhưng KHÔNG GÌ ĐỌC ĐƯỢC NÓ. Ngày 22/09/2026 SÁU chỗ ghi vết mới nạp `p.ID` (ULID
nội bộ) vào đúng cột ấy, và không một ca kiểm nào đỏ: một ULID trông y như một giá trị hợp lệ.
Cột khi ấy chứa hai loại định danh cùng lúc, tức một cột không ai truy vấn được, và một dòng vết
mang `01JD9A…` không gọi tên ai với người đọc nó ba năm sau trong một cuộc thanh tra.

═══════════════════════════════════════════════════════════════════════════════════════════════
GIỚI HẠN — ĐỌC TRƯỚC KHI TIN. MỘT RÀO ĐƯỢC MÔ TẢ QUÁ LỜI CÒN TỆ HƠN KHÔNG CÓ RÀO.
═══════════════════════════════════════════════════════════════════════════════════════════════

ĐÂY LÀ PHÉP KIỂM HÌNH DẠNG CÚ PHÁP, KHÔNG PHẢI PHÉP KIỂM NGỮ NGHĨA. Nó không biết một giá trị
lúc chạy là mã cán bộ hay là ULID. Nó chỉ trả lời được đúng một câu hẹp hơn nhiều:

    "biểu thức nạp vào trường `ID` của một `audit.Actor` CỦA CÁN BỘ có phải là một trong hai
     hình dạng ĐÃ BIẾT LÀ SAI không"

Hai hình dạng ấy, và chỉ hai:

  (a) một selector kết thúc bằng `.ID`  — `p.ID`, `principal.ID`, `chuThe.ID`. Đây ĐÚNG hình
      dạng của sáu chỗ hỏng ngày 22/09, vì `authz.Principal.ID` là trường mọi phép phân quyền
      join vào và là thứ nằm sẵn trong tầm tay người viết tuyến.
  (b) một hằng chuỗi mang hình dạng định danh nội bộ — `"nd-…"`, hoặc 26 ký tự ULID. Đây là
      hình dạng của năm bộ đồ thử đã khẳng định giá trị sai suốt từ lúc chúng được viết.

NHỮNG GÌ NÓ KHÔNG BẮT, viết ra chứ không để người sau tự phát hiện bằng một cột hỏng:

  1. GIÁ TRỊ ĐI VÒNG QUA MỘT BIẾN THÌ THOÁT SẠCH.  `id := p.ID` rồi `audit.Actor{ID: id}` không
     bị bắt, và không thể bắt được ở tầng này — cần `go/types` và một đồ thị luồng dữ liệu.
     Đây là lỗ hổng lớn nhất và nó là CÓ Ý: đoán thêm thì báo sai, và báo sai giết một rào
     chắn nhanh hơn bỏ sót. Đổi lại, hình dạng trực tiếp là hình dạng người ta thật sự viết —
     cả sáu chỗ hỏng hôm nay đều viết thẳng `p.ID`.
  2. `Kind` TÍNH LÚC CHẠY thì không phân giải được. Xem `la_can_bo` để biết chỗ này đoán ra sao
     và đoán theo chiều nào.
  3. Một trường tên khác `ID` mang định danh (không có trong kho hôm nay) thì vô hình.
  4. Chiều ngược lại — một `ma` ĐÚNG hình dạng nhưng của NHẦM NGƯỜI — không thuộc lớp lỗi này
     và không gì ở đây thấy được.

Nói cách khác: rào này bắt được LẦN TÁI DIỄN Y HỆT của khiếm khuyết 22/09, và không hứa gì hơn.

Chỉ thư viện chuẩn. Không phụ thuộc.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# MỘT BỘ QUÉT GO CHO CẢ HAI RÀO, KHÔNG CHÉP LẠI. `quyen_keys.ma_thuc_thi_go` bóc chú thích và
# ruột chuỗi thô, giữ nguyên độ dài để số dòng báo ra vẫn đúng; `_khoi_can_bang` và
# `_tach_cap_khong` cân bằng ngoặc mà không bị chuỗi đánh lừa. Viết lại ba hàm ấy ở đây là dựng
# bản sao thứ hai của đúng thứ luật 9 cấm nhân đôi — và bản lệch sẽ là bản im lặng.
import quyen_keys as qk  # noqa: E402

# Hình dạng ĐỊNH DANH NỘI BỘ trong kho này, đo trên mã thật:
#   `nd-01JINTERNALIDCUACANBO`, `nd-01JCANBONOIBOCUAXA`   — tiền tố `nd-` của bảng `nguoi_dung`
#   `01JD9AAAAAAAAAAAAAAAAAAAAA`                          — ULID trần, 26 ký tự Crockford base32
#
# Mã nghiệp vụ thì KHÔNG khớp cái nào: `CB-00123`, `CB001`, `CB-007` (luật 5 câu hỏi mở #15 giữ
# nó ngắn và không-ULID CHÍNH VÌ người đọc sổ phải tra được người từ nó).
ID_NOI_BO = re.compile(r"^(?:nd-|01[0-9A-HJKMNP-TV-Z]{24}$)")

# Selector kết thúc bằng `.ID` — `p.ID`, `principal.ID`, `m.chuThe.ID`.
SELECTOR_ID = re.compile(r"^[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*\.ID$")

# Lối thoát, và nó ĐÒI MỘT LÝ DO. `@actor-ok` trần không mở được gì: một lối thoát không có lý do
# là một lối thoát không ai dám gỡ sáu tháng sau (cùng lập luận với luật 5 cấm #4 về `Public()`).
LOI_THOAT = re.compile(r"@actor-ok:\s*(\S.*)")

# Tên trường của một `audit.Actor`, để nhận ra một literal LỒNG BÊN TRONG đã lược kiểu.
RE_ACTOR = re.compile(r"\bActor\s*\{")
RE_KHOA = re.compile(r"^\s*([A-Za-z_]\w*)\s*:\s*(.*)$", re.DOTALL)

# Khai báo hàm, để tìm thân hàm bao quanh một literal. Xem `than_ham_quanh`.
RE_FUNC = re.compile(r"\bfunc\b")


def _cap_dau(khoi: str) -> list[tuple[str, int]]:
    """Các cặp `Khoá: giá trị` ở CẤP NGOÀI CÙNG của một literal, kèm offset của giá trị."""
    ra: list[tuple[str, int]] = []
    for phan, goc in qk._tach_cap_khong(khoi, ","):
        m = RE_KHOA.match(phan)
        if not m:
            continue
        ra.append((m.group(1), goc + phan.index(m.group(2), len(m.group(1)))))
    return ra


def _gia_tri(khoi: str, ten: str) -> str | None:
    for phan, _ in qk._tach_cap_khong(khoi, ","):
        m = RE_KHOA.match(phan)
        if m and m.group(1) == ten:
            return m.group(2).strip()
    return None


def than_ham_quanh(ma: str, off: int) -> str:
    """Thân của hàm bao quanh `off`. Chuỗi rỗng nếu không nằm trong hàm nào.

    CẮT ĐÚNG THÂN HÀM, KHÔNG GỘP TỚI HÀM KẾ. `audit_guard.funcs()` gộp mọi dòng sau một đầu hàm
    cho tới đầu hàm kế tiếp, và kho này đã ghi nhận hậu quả: một hằng cấp gói viết giữa hai hàm
    bị tính vào thân hàm đứng trước. Ở đây sai theo chiều ấy đắt hơn nhiều — thân hàm là thứ
    quyết định một chủ thể có phải cán bộ không, nên một thân hàm nuốt quá tay sẽ nhặt được một
    phép kiểm `Kind != "citizen"` của hàm KHÁC và miễn trừ một vi phạm thật.
    """
    dau = -1
    for m in RE_FUNC.finditer(ma[:off]):
        dau = m.start()
    if dau < 0:
        return ""
    mo = ma.find("{", dau)
    if mo < 0:
        return ""
    het = qk._khoi_can_bang(ma, mo, "{", "}")
    if het < 0 or het <= off:
        return ""
    return ma[mo:het]


def la_can_bo(khoi: str, than: str) -> bool:
    """Literal này có phải chủ thể CÁN BỘ không.

    BA NHÁNH, VÀ NHÁNH KHÔNG BIẾT ĐƯỢC COI LÀ CÁN BỘ — hỏng thì ĐÓNG. Một chủ thể không phân
    loại được mà bỏ qua là đúng cách sáu chỗ hôm nay lọt: cả sáu viết `Kind: p.Kind`.

      Kind: "staff"              cán bộ, chắc chắn.
      Kind: "citizen"/"system"   không phải cán bộ, bỏ qua.
      Kind: <x>.Kind             KHÔNG BIẾT ở tầng cú pháp. Tra thân hàm bao quanh: nếu hàm ấy
                                 tự khẳng định chủ thể là công dân hay hệ thống — `p.Kind !=
                                 "citizen"` rồi trả về sớm — thì tin nó và bỏ qua. Đây là ca
                                 THẬT và là lý do nhánh này tồn tại:
                                 `service-petitions/internal/http/gui_phan_anh.go:224` chặn
                                 đúng như thế, và `p.ID` ở đó LÀ định danh công dân — công dân
                                 không có mã cán bộ, nên bắt nó là bắt một chỗ đúng.
                                 Không tìm thấy phép kiểm nào → coi là cán bộ.
    """
    kind = _gia_tri(khoi, "Kind")
    if kind is None:
        return True                      # thiếu Kind: audit.Entry.validate sẽ từ chối, vẫn chấm
    k = kind.strip('"')
    if kind.startswith('"'):
        return k == "staff"
    # Một selector/biến: đi tìm lời tự khẳng định trong chính thân hàm.
    if re.search(r'\.Kind\s*[!=]=\s*"(?:citizen|system)"', than):
        return False
    return True


def vi_pham_trong_go(src: str) -> list[tuple[int, str, str]]:
    """Mọi `audit.Actor` cán bộ nạp ID bằng một hình dạng đã biết là sai.

    Trả (số dòng, biểu thức, lý do). THUẦN — vào là văn bản, ra là danh sách, không chạm đĩa,
    nên `tools/test_hooks.py` chấm được nó bằng chuỗi nguyên văn.
    """
    ma = qk.ma_thuc_thi_go(src)
    dong_goc = src.split("\n")
    ra: list[tuple[int, str, str]] = []

    for m in RE_ACTOR.finditer(ma):
        mo = ma.index("{", m.end() - 1)
        het = qk._khoi_can_bang(ma, mo, "{", "}")
        if het < 0:
            continue
        ngoai = ma[mo + 1:het - 1]

        # Ứng viên: chính khối này nếu nó mang `ID:` ở cấp ngoài; và MỌI literal lồng bên trong
        # mang `ID:` — `map[string]audit.Actor{"x": {ID: …}}` là hình dạng có thật
        # (service-petitions/internal/app/gui_phan_anh_test.go:413), kiểu của phần tử được lược
        # đi nên không có chữ `Actor` nào để neo vào.
        ung_vien: list[tuple[str, int]] = []
        if _gia_tri(ngoai, "ID") is not None:
            ung_vien.append((ngoai, mo + 1))
        else:
            i = mo + 1
            while i < het - 1:
                if ma[i] == "{":
                    j = qk._khoi_can_bang(ma, i, "{", "}")
                    if j < 0:
                        break
                    trong = ma[i + 1:j - 1]
                    if _gia_tri(trong, "ID") is not None:
                        ung_vien.append((trong, i + 1))
                    i = j
                    continue
                i += 1

        for khoi, goc in ung_vien:
            gia_tri = _gia_tri(khoi, "ID")
            if not gia_tri:
                continue
            than = than_ham_quanh(ma, goc)
            if not la_can_bo(khoi, than):
                continue

            ly_do = ""
            if SELECTOR_ID.match(gia_tri):
                ly_do = "trường `.ID` của chủ thể — ĐỊNH DANH NỘI BỘ, không phải mã cán bộ"
            elif gia_tri.startswith('"') and ID_NOI_BO.match(gia_tri.strip('"')):
                ly_do = "hằng chuỗi mang hình dạng định danh nội bộ (`nd-…` hoặc ULID)"
            if not ly_do:
                continue

            # Số dòng của chính giá trị, không phải của chữ `Actor{` — hai thứ ấy cách nhau vài
            # dòng ở literal viết nhiều dòng, và một rào chỉ sai số dòng là một rào người ta mở
            # đúng tệp rồi không thấy gì.
            off = goc + khoi.index(gia_tri)
            dong = ma.count("\n", 0, off) + 1
            if _co_loi_thoat(dong_goc, dong):
                continue
            ra.append((dong, gia_tri, ly_do))

    return sorted(set(ra))


def _co_loi_thoat(dong_goc: list[str], dong: int) -> bool:
    """`@actor-ok: <lý do>` trên cùng dòng, hoặc trong KHỐI chú thích ngay trên literal.

    CẢ KHỐI, KHÔNG CHỈ MỘT DÒNG. `citizen_scope_guard` đọc lối thoát của nó chỉ ở `lines[i-1]`
    và kho này đã ghi đó là một khuyết tật: một lý do đáng viết không bao giờ gói trong một
    dòng, nên lối thoát hợp lệ vẫn bị chặn và người viết được dạy cách rút ngắn lý do — tức rào
    chắn đang tối ưu hoá cho việc viết lời giải thích tệ đi. Đi ngược lên hết khối chú thích
    dính liền, đúng như `tenant_scope_guard.khoi_chu_thich_tren` đã chữa.
    """
    i = dong - 1
    if i < len(dong_goc) and LOI_THOAT.search(dong_goc[i]):
        return True
    i -= 1
    while i >= 0:
        s = dong_goc[i].strip()
        if not s.startswith("//"):
            break
        if LOI_THOAT.search(s):
            return True
        i -= 1
    return False


def quet_kho(goc: str) -> tuple[list[str], int]:
    """Trả (các vi phạm dạng `tệp:dòng biểu thức (lý do)`, số tệp đã đọc)."""
    vi_pham: list[str] = []
    dem = 0
    for rel, duong in qk.tep_go(goc):
        try:
            with open(duong, encoding="utf-8") as f:
                src = f.read()
        except Exception:
            continue
        dem += 1
        if "Actor{" not in src.replace(" ", ""):
            continue
        for dong, gt, ly_do in vi_pham_trong_go(src):
            vi_pham.append(f"{rel}:{dong}  {gt}  ({ly_do})")
    return vi_pham, dem
