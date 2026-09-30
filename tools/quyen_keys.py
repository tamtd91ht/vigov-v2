#!/usr/bin/env python3
"""Đọc HAI nguồn của một khoá quyền, và chỉ đọc — không phán xử.

VÌ SAO TỆP NÀY TỒN TẠI RIÊNG, KHÔNG NẰM TRONG HOOK CŨNG KHÔNG NẰM TRONG CỔNG KIỂM:
phép đối chiếu này có HAI nơi phải chạy và chúng hỏi cùng một câu ở hai thời điểm khác nhau —
`.claude/hooks/quyen_key_guard.py` chặn lúc ghi một tệp, `tools/check_quyen.py` quét cả kho
lúc `make check`. Hai bản sao của bộ phân tích là hai bản sẽ lệch, và bản lệch là bản im lặng:
đúng hình dạng hỏng mà luật 9 nói tới. Một bộ phân tích, hai cửa vào.

HAI NGUỒN:

  1. BẢNG `quyen` — `service-identity/migrations/*.sql`. Nguồn chuẩn DUY NHẤT của tập khoá mà
     một quản trị viên xã có thể tick trên màn Phân quyền. Đo ngày 21/09/2026: ĐÚNG 35 khoá,
     từ ĐÚNG hai câu `INSERT INTO quyen`.

  2. MÃ GO — mọi chuỗi được trao cho `authz.RequirePermission`, ép kiểu `authz.Perm`, hay gán
     vào một khai báo/hằng/literal mang kiểu ấy.

LỚP LỖI NÓ BẮT: một chuỗi ở nguồn 2 mà KHÔNG có ở nguồn 1. Không quản trị viên nào cấp được
khoá ấy, nên tuyến giữ nó trả 403 với MỌI tài khoản, mãi mãi — và không có gì đỏ. Ngày
21/09/2026 kho này có ba chuỗi như thế cùng lúc (`finance.read`, `map.read`,
`document.approve`) và không phép kiểm nào thấy.

PHẠM VI LÀ MÃ GO, CÓ CHỦ Ý — không phải vì tiện. Câu hỏi ở đây là "một tuyến hay một phép cấp
trong mã có gọi tên một khoá không ai cấp được không", và chỉ Go mới trao chuỗi cho
`authz.Perm`. Chuỗi trong `web-admin` là chuỗi ĐEM SO với bộ khoá phiên đăng nhập trả về; ở đó
`admin.users`, `budget.reads` là DỮ LIỆU THỬ CỐ Ý GẦN GIỐNG, dựng để khẳng định "gần giống cũng
không mở được tab" (web-admin/src/features/cau-hinh/quyen-tab.test.ts:74). Đối chiếu chúng với
bảng `quyen` là báo đỏ đúng những ca đang chứng minh điều ngược lại.

TỆP TEST KHÔNG ĐƯỢC BỎ QUA. `_common.should_skip` bỏ `_test.go`, và nếu dùng nó ở đây thì cả
ba khoá bịa hôm nay đều vô hình: cả ba nằm trong `*_test.go`. Một bài kiểm cấp một khoá không
tồn tại là một bài kiểm chứng minh KHÔNG GÌ CẢ — nó dựng một thế giới không có thật rồi khẳng
định điều gì đó về thế giới ấy.

Chỉ thư viện chuẩn. Không phụ thuộc.
"""

from __future__ import annotations

import os
import re

# Hình dạng một khoá quyền: MỘT CHUỖI PHẲNG `<nhóm>.<việc>` (luật 5 bất biến 3b).
#
# Nới hơn quy ước ADR 0030 (`nhóm.mộttừ`, không gạch dưới ở vế sau) một cách CÓ CHỦ Ý: mục
# đích ở đây là NHẬN RA một chuỗi đang tự xưng là khoá quyền để đem đối chiếu, không phải phán
# xử cách đặt tên. Nới ra thì một khoá sai quy ước vẫn bị đem so với bảng và vẫn bị bắt vì
# thiếu — tức chặt hơn, không lỏng hơn. Quy ước đặt tên đã có `drift_guard` tín hiệu 3 của câu
# mở #27 canh.
HINH_DANG_KHOA = re.compile(r"^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$")

# Thư mục migration sở hữu bảng `quyen`. MỘT dịch vụ, vì `quyen` thuộc `identity`
# (kb/30-indexes/data-ownership.json). Quét cả kho sẽ làm rào LỎNG đi chứ không chặt hơn: một
# câu INSERT lạc ở dịch vụ khác vừa là vi phạm luật 2, vừa sẽ âm thầm hợp thức hoá thêm khoá.
THU_MUC_BANG = os.path.join("service-identity", "migrations")

# Nơi câu INSERT nằm. Hai vế `ma` là cột ĐẦU TIÊN của mỗi dòng VALUES, nên neo vào dấu `(` mở
# dòng chứ không quét mọi chuỗi trong khối — nhãn tiếng Việt cũng là chuỗi.
RE_INSERT = re.compile(r"INSERT\s+INTO\s+quyen\b", re.IGNORECASE)
RE_DONG_VALUES = re.compile(r"\(\s*'([^']*)'")

BO_QUA_THU_MUC = {
    ".git", "node_modules", "vendor", "dist", "build", "__pycache__", "tmp",
}

# Mã SINH RA từ `.proto`. Nó không khai quyền cho tuyến nào; chuỗi trong đó là chú thích đã
# được buf chép vào, và sửa nó là chuyện `buf generate` (luật 2 cấm #3).
BO_QUA_DUONG = ("/core/gen/", ".pb.go", ".gen.go")


# ---------------------------------------------------------------------------
# NGUỒN 1 — bảng `quyen`
# ---------------------------------------------------------------------------

def doc_bang_quyen(goc: str) -> tuple[set[str], list[str]]:
    """Trả (tập khoá, danh sách `tệp:dòng` đã đọc được khoá).

    HỎNG THÌ ĐÓNG: người gọi phải tự kiểm tập rỗng. Một bảng đọc ra rỗng khiến MỌI khoá trong
    mã thành "không có trong bảng" — rào sẽ hét lên chứ không im, và hét là điều đúng: nó có
    nghĩa nguồn chuẩn không đọc được nữa.
    """
    khoa: set[str] = set()
    nguon: list[str] = []
    thu_muc = os.path.join(goc, THU_MUC_BANG)
    if not os.path.isdir(thu_muc):
        return khoa, nguon
    for ten in sorted(os.listdir(thu_muc)):
        if not ten.endswith(".sql"):
            continue
        duong = os.path.join(thu_muc, ten)
        try:
            with open(duong, encoding="utf-8") as f:
                src = f.read()
        except Exception:
            continue
        for m in RE_INSERT.finditer(src):
            # Khối VALUES kết thúc ở `ON CONFLICT` hoặc ở dấu `;` — lấy cái nào tới trước.
            het = len(src)
            for dau in (r"\bON\s+CONFLICT\b", r";"):
                mm = re.search(dau, src[m.end():], re.IGNORECASE)
                if mm:
                    het = min(het, m.end() + mm.start())
            khoi = src[m.end():het]
            for mk in RE_DONG_VALUES.finditer(khoi):
                ma = mk.group(1)
                khoa.add(ma)
                dong = src.count("\n", 0, m.end() + mk.start()) + 1
                nguon.append(f"{THU_MUC_BANG}/{ten}:{dong} {ma}")
    return khoa, nguon


# ---------------------------------------------------------------------------
# NGUỒN 2 — mã Go
# ---------------------------------------------------------------------------

def ma_thuc_thi_go(src: str) -> str:
    """Giữ lại phần mã CHẠY ĐƯỢC: xoá chú thích VÀ ruột chuỗi thô. GIỮ NGUYÊN ĐỘ DÀI.

    KHÔNG dùng lại `drift_guard.bo_chu_thich`: hàm ấy cố ý GIỮ LẠI chú thích mang dấu `@` và
    cắt dòng bằng regex thô cho năm ngôn ngữ, vì việc của nó là đếm tín hiệu. Việc ở đây khác
    hẳn — chỉ Go, và phải ĐÚNG, vì một chú thích còn sót là một lần báo sai.

    Ca thật, không phải giả định: `service-comms/cmd/server/main_test.go:149` có dòng
    `// WHAT STOOD HERE UNTIL TODAY: PermissionKeys: []authz.Perm{"map.read"}` — chính chú
    thích ghi lại khiếm khuyết vừa vá. Không bóc chú thích thì lần quét toàn kho báo đỏ đúng
    tệp đã sửa xong, và một rào lên tiếng lần đầu để buộc tội tệp cẩn thận nhất là rào không ai
    đọc lần thứ hai.

    RUỘT CHUỖI THÔ CŨNG BỊ XOÁ, và đó là lần sửa thứ hai — đo, không đoán. Bản đầu báo năm chỗ,
    cả năm ở `tools/apidoc/*_test.go`: `authz.RequirePermission(nil, "hoso.read")` nằm TRONG một
    chuỗi thô `` ` `` làm dữ liệu thử cho trình trích route (lop_xa_test.go:110). Một chuỗi là
    DỮ LIỆU, không phải một lời gọi: không tuyến nào được dựng từ nó, không ai bị 403 vì nó.
    Ngược lại, một khoá THẬT không bao giờ nằm trong chuỗi thô, vì `RequirePermission` phải chạy
    được. Nên xoá ruột chuỗi thô là chặt đúng chỗ, không bỏ sót chỗ nào.

    Chuỗi nháy kép thì GIỮ NGUYÊN — khoá quyền chính là chuỗi nháy kép.

    Giữ nguyên độ dài (và mọi ký tự xuống dòng) để số dòng báo ra vẫn đúng.
    """
    ra = []
    i, n = 0, len(src)
    while i < n:
        c = src[i]
        if c == '"' or c == "'":
            dong = c
            ra.append(c)
            i += 1
            while i < n:
                ra.append(src[i])
                if src[i] == "\\" and i + 1 < n:
                    ra.append(src[i + 1])
                    i += 2
                    continue
                if src[i] == dong:
                    i += 1
                    break
                if src[i] == "\n":      # chuỗi Go không qua dòng — coi như hết, hỏng thì mở
                    i += 1
                    break
                i += 1
            continue
        if c == "`":                    # chuỗi thô: không có ký tự thoát, qua được nhiều dòng
            ra.append(c)
            i += 1
            while i < n:
                if src[i] == "`":
                    ra.append("`")
                    i += 1
                    break
                ra.append("\n" if src[i] == "\n" else " ")
                i += 1
            continue
        if c == "/" and i + 1 < n and src[i + 1] == "/":
            while i < n and src[i] != "\n":
                ra.append(" ")
                i += 1
            continue
        if c == "/" and i + 1 < n and src[i + 1] == "*":
            while i < n and not (src[i] == "*" and i + 1 < n and src[i + 1] == "/"):
                ra.append("\n" if src[i] == "\n" else " ")
                i += 1
            ra.append("  ")
            i += 2
            continue
        ra.append(c)
        i += 1
    return "".join(ra)


RE_REQUIRE = re.compile(r"\bRequirePermission\s*\(")
RE_EP_KIEU = re.compile(r"\b(?:authz\.)?Perm\s*\(")
# `const QuyenHanChe authz.Perm = "feedback.restricted"` và mọi biến thể trong khối
# `const ( ... )`, nơi từ khoá `const` nằm ở dòng khác.
RE_KHAI_BAO = re.compile(r"\b\w+\s+(?:authz\.)?Perm\s*=\s*\"([^\"\n]*)\"")
# Chỗ kiểu `authz.Perm` xuất hiện — điểm bắt đầu để tìm literal ghép mang kiểu ấy.
RE_KIEU = re.compile(r"\b(?:authz\.)?Perm\b")

RE_CHUOI = re.compile(r"\"((?:[^\"\\\n]|\\.)*)\"")

# Khai báo hàm — dùng để tìm hàm phụ trợ NHẬN một `authz.Perm`. Xem `ham_nhan_perm`.
RE_HAM = re.compile(r"\bfunc\s+(?:\([^()]*\)\s*)?(\w+)\s*\(")

# Kiểu nào là "một khoá quyền" ở vị trí tham số.
KIEU_PERM = {"authz.Perm", "Perm", "[]authz.Perm", "[]Perm"}


def _khoi_can_bang(src: str, i: int, mo: str, dong: str) -> int:
    """Chỉ số ngay SAU dấu đóng khớp với dấu mở ở `src[i]`. -1 nếu không khớp."""
    sau = 0
    n = len(src)
    while i < n:
        c = src[i]
        if c == '"':
            i += 1
            while i < n and src[i] != '"':
                i += 2 if src[i] == "\\" else 1
            i += 1
            continue
        if c == "`":
            i += 1
            while i < n and src[i] != "`":
                i += 1
            i += 1
            continue
        if c == mo:
            sau += 1
        elif c == dong:
            sau -= 1
            if sau == 0:
                return i + 1
        i += 1
    return -1


def _chuoi_trong(khoi: str, chi_cap_mot: bool) -> list[tuple[str, int]]:
    """Các chuỗi trong một khối, kèm offset. `chi_cap_mot` giữ lại chuỗi ở ngoài cùng.

    Vì sao cần: `RequirePermission(mustChecker("x"), "task.read")` — `"x"` là đối của một lời
    gọi khác, không phải một khoá quyền. Ở vị trí đối số thì cấp lồng nhau phân biệt được hai
    thứ ấy; ở literal ghép thì KHÔNG, vì khoá nằm sâu trong map lồng map.
    """
    ra: list[tuple[str, int]] = []
    cap = 0
    i, n = 0, len(khoi)
    while i < n:
        c = khoi[i]
        if c == '"':
            m = RE_CHUOI.match(khoi, i)
            if m:
                if not chi_cap_mot or cap <= 1:
                    ra.append((m.group(1), m.start()))
                i = m.end()
                continue
            i += 1
            continue
        if c == "`":
            i += 1
            while i < n and khoi[i] != "`":
                i += 1
            i += 1
            continue
        if c in "([{":
            cap += 1
        elif c in ")]}":
            cap -= 1
        i += 1
    return ra


def _mo_literal_ghep(src: str, i: int) -> int:
    """Từ ngay sau chỗ kiểu `Perm` xuất hiện, tìm dấu `{` mở literal ghép. -1 nếu không có.

    DỪNG SỚM LÀ CỐ Ý. `func (c checkerGia) Allows(_ context.Context, p authz.Principal,
    perm authz.Perm) bool {` — dấu `{` gần nhất ở đó là THÂN HÀM, và nuốt cả thân hàm thì mọi
    chuỗi hình dạng `a.b` bên trong (kể cả `application.json`) thành một khoá quyền tưởng
    tượng. Gặp `)`, `,`, `;` hay xuống dòng là bỏ: ở những chỗ ấy `Perm` đang là kiểu của một
    tham số hay một phần tử, không phải kiểu của một literal.

    `struct{}` và `interface{}` được bước qua: `map[authz.Perm]struct{}{...}` là hình dạng có
    thật (core/staffauth/staffauth.go:124) và cặp ngoặc rỗng của nó không phải literal.
    """
    n = len(src)
    while i < n:
        c = src[i]
        if c in " \t]":
            i += 1
            continue
        if src.startswith("struct{}", i) or src.startswith("interface{}", i):
            i += src[i:].index("{}") + 2
            continue
        if c == "{":
            return i
        if c.isalnum() or c in "._*&":   # tên kiểu: `bool`, `tenant.ID`, `[]byte`
            i += 1
            continue
        if c == "[":                      # `map[...]` lồng — bước qua trọn cặp
            j = _khoi_can_bang(src, i, "[", "]")
            if j < 0:
                return -1
            i = j
            continue
        return -1
    return -1


def _tach_cap_khong(khoi: str, dau: str) -> list[tuple[str, int]]:
    """Tách ở CẤP NGOÀI CÙNG, kèm offset. `map[a]b{x: 1}, "y"` -> hai phần, không phải ba.

    Trả kèm offset chứ không để người gọi tự `index()` lại: hai đối giống hệt nhau trong cùng
    một lời gọi sẽ làm `index()` trả về chỗ đầu tiên cho cả hai, tức số dòng báo ra sai — và một
    rào chỉ sai số dòng là một rào người ta mở đúng tệp rồi không thấy gì.
    """
    ra: list[tuple[str, int]] = []
    sau, cuoi = 0, 0
    i, n = 0, len(khoi)
    while i < n:
        c = khoi[i]
        if c == '"':
            m = RE_CHUOI.match(khoi, i)
            i = m.end() if m else i + 1
            continue
        if c == "`":
            i += 1
            while i < n and khoi[i] != "`":
                i += 1
            i += 1
            continue
        if c in "([{":
            sau += 1
        elif c in ")]}":
            sau -= 1
        elif c == dau and sau == 0:
            ra.append((khoi[cuoi:i], cuoi))
            cuoi = i + 1
        i += 1
    ra.append((khoi[cuoi:], cuoi))
    return ra


def ham_nhan_perm(src: str) -> dict[str, tuple[set[int], int]]:
    """Các hàm trong GÓI này nhận một khoá quyền. Trả tên -> (vị trí Perm, vị trí biến thiên).

    VÌ SAO CẦN — ĐO CHỨ KHÔNG ĐOÁN. Bản đầu của bộ phân tích bỏ sót 15 chỗ chỉ riêng trong
    `service-finance/internal/http/du_an_test.go`, tất cả cùng một hình dạng:

        func coQuyen(perm authz.Perm) checkerGia          // routes_test.go:158
        m := dungMayChuVoi(t, coQuyen("budget.read"))     // du_an_test.go:134

    Chuỗi ấy được ép sang `authz.Perm` ở chính lời gọi, nên nó là một phép CẤP QUYỀN đầy đủ
    nghĩa. Bỏ sót hình dạng này là bỏ sót hình dạng PHỔ BIẾN NHẤT của bộ đồ thử trong kho, tức
    rào trông như đang canh mà mù đúng chỗ khiếm khuyết đã sống.

    GIỚI HẠN, VIẾT RA CHỨ KHÔNG ĐỂ NGƯỜI SAU TỰ ĐOÁN: chỉ MỘT tầng, và chỉ trong cùng một gói.
    Một hàm phụ trợ gọi một hàm phụ trợ khác thì tầng thứ hai không được lần theo. Đóng nốt
    tầng ấy đòi một bộ kiểm kiểu thật (`go/types`); ranh giới này bắt được mọi hình dạng kho
    đang có, và cái giá của việc đoán thêm là báo sai — thứ giết rào chắn nhanh hơn bỏ sót.

    KHÔNG BẮT LỜI KHẲNG ĐỊNH, cũng có chủ ý. `c.hoiGi[0] != "task.extend"` là một lời khẳng
    định VỀ khoá, không phải một phép cấp: viết sai chuỗi ở đó thì chính bài kiểm ấy ĐỎ, tức nó
    không thuộc lớp lỗi im lặng rào này dựng ra để bắt. Một fixture CẤP một khoá bịa thì ngược
    lại — bài kiểm XANH, và đó đúng là cách `finance.read` sống sót.
    """
    ra: dict[str, tuple[set[int], int]] = {}
    for m in RE_HAM.finditer(src):
        het = _khoi_can_bang(src, m.end() - 1, "(", ")")
        if het < 0:
            continue
        muc = _tach_cap_khong(src[m.end():het - 1], ",")
        # Go cho gộp tên: `a, b authz.Perm`. Kiểu đứng sau cùng áp cho mọi tên đứng trước nó.
        kieu: list[str | None] = [None] * len(muc)
        hien: str | None = None
        for i in range(len(muc) - 1, -1, -1):
            phan = muc[i][0].strip().split(None, 1)
            if len(phan) == 2:
                hien = phan[1].strip()
            kieu[i] = hien
        vi_tri = {i for i, k in enumerate(kieu) if k in KIEU_PERM}
        bien_thien = next((i for i, k in enumerate(kieu)
                           if k and k.startswith("...") and k[3:] in KIEU_PERM), -1)
        if vi_tri or bien_thien >= 0:
            ra[m.group(1)] = (vi_tri, bien_thien)
    return ra


def khoa_trong_go(src: str, phu_tro: dict[str, tuple[set[int], int]] | None = None) -> list[tuple[str, int, str]]:
    """Mọi chuỗi mã Go này trao cho tầng quyền. Trả (khoá, số dòng, hình dạng đã khớp).

    THUẦN — vào là văn bản, ra là danh sách. Không chạm đĩa, nên `tools/test_hooks.py` kiểm
    được nó bằng chuỗi nguyên văn.
    """
    ma = ma_thuc_thi_go(src)
    thay: dict[tuple[str, int], str] = {}

    def ghi(chuoi: str, off: int, loai: str) -> None:
        # CHUỖI RỖNG KHÔNG PHẢI MỘT LỜI KHAI VỀ KHOÁ — trừ khi nó là khoá một tuyến đòi hỏi.
        # `c.Allows(daCo, chuThe, "")` (core/staffauth/staffauth_test.go:521) khẳng định "khoá
        # rỗng không khớp gì", tức đúng điều rào này muốn. Báo đỏ ở đó là phạt lời khẳng định
        # đang bảo vệ mình. Nhưng `RequirePermission(c, "")` thì là một tuyến 403 vĩnh viễn
        # thật, nên ở riêng vị trí ấy chuỗi rỗng vẫn bị chấm.
        if chuoi == "" and loai != "RequirePermission":
            return
        thay.setdefault((chuoi, ma.count("\n", 0, off) + 1), loai)

    # 1. Đối số của RequirePermission — vị trí quyền không thể nhầm lẫn, nên MỌI chuỗi ở cấp
    #    một đều bị chấm, kể cả chuỗi sai hình dạng (`"taskread"` cũng là một khoá không tồn tại).
    for m in RE_REQUIRE.finditer(ma):
        het = _khoi_can_bang(ma, m.end() - 1, "(", ")")
        if het < 0:
            continue
        khoi = ma[m.end() - 1:het]
        for s, off in _chuoi_trong(khoi, chi_cap_mot=True):
            ghi(s, m.end() - 1 + off, "RequirePermission")

    # 2. Ép kiểu `authz.Perm("...")`.
    for m in RE_EP_KIEU.finditer(ma):
        het = _khoi_can_bang(ma, m.end() - 1, "(", ")")
        if het < 0:
            continue
        khoi = ma[m.end() - 1:het]
        for s, off in _chuoi_trong(khoi, chi_cap_mot=True):
            ghi(s, m.end() - 1 + off, "authz.Perm(...)")

    # 3. Khai báo mang kiểu: `const X authz.Perm = "..."`.
    for m in RE_KHAI_BAO.finditer(ma):
        ghi(m.group(1), m.start(1), "khai báo authz.Perm")

    # 4. Literal ghép mang kiểu ấy: `[]authz.Perm{...}`, `map[authz.Perm]bool{...}`, và
    #    `map[tenant.ID]map[string]map[authz.Perm]bool{xa: {ai: {"task.read": true}}}` — khoá
    #    thật nằm ở literal LỒNG BÊN TRONG, kiểu của nó được lược đi, nên phải lấy trọn khối.
    #    Đổi lại, ở đây chỉ nhận chuỗi ĐÚNG HÌNH DẠNG khoá: các cấp ngoài mang id vai trò, id
    #    cán bộ, id xã — chấm chúng là báo sai.
    for m in RE_KIEU.finditer(ma):
        mo = _mo_literal_ghep(ma, m.end())
        if mo < 0:
            continue
        het = _khoi_can_bang(ma, mo, "{", "}")
        if het < 0:
            continue
        for s, off in _chuoi_trong(ma[mo:het], chi_cap_mot=False):
            if HINH_DANG_KHOA.match(s):
                ghi(s, mo + off, "literal []authz.Perm / map[authz.Perm]")

    # 5. Hàm phụ trợ của GÓI nhận một `authz.Perm`: `coQuyen("budget.read")`. Chuỗi được ép
    #    kiểu ngay tại lời gọi, nên nó cấp quyền thật sự. Xem `ham_nhan_perm`.
    for ten, (vi_tri, bien_thien) in (phu_tro or {}).items():
        for m in re.finditer(r"\b" + re.escape(ten) + r"\s*\(", ma):
            het = _khoi_can_bang(ma, m.end() - 1, "(", ")")
            if het < 0:
                continue
            for i, (doi, goc) in enumerate(_tach_cap_khong(ma[m.end():het - 1], ",")):
                if i not in vi_tri and not (bien_thien >= 0 and i >= bien_thien):
                    continue
                for s, off in _chuoi_trong(doi, chi_cap_mot=True):
                    ghi(s, m.end() + goc + off, f"đối của {ten}(...)")

    return sorted(((k, d, l) for (k, d), l in thay.items()), key=lambda x: x[1])


# ---------------------------------------------------------------------------
# Quét kho
# ---------------------------------------------------------------------------

def tep_go(goc: str):
    for dp, dn, fn in os.walk(goc):
        dn[:] = [d for d in dn if d not in BO_QUA_THU_MUC]
        for f in fn:
            if not f.endswith(".go"):
                continue
            duong = os.path.join(dp, f)
            rel = os.path.relpath(duong, goc).replace("\\", "/")
            if any(b.strip("/") in ("/" + rel) or rel.endswith(b) for b in BO_QUA_DUONG):
                continue
            yield rel, duong


def phu_tro_cua_goi(thu_muc: str, bo_qua: str = "") -> dict[str, tuple[set[int], int]]:
    """Các hàm nhận `authz.Perm` khai báo ở BẤT KỲ tệp nào trong cùng thư mục (= cùng gói).

    Một gói Go là một thư mục, và `coQuyen` được khai ở `routes_test.go` rồi gọi từ
    `du_an_test.go`. Đọc một tệp thôi thì không bao giờ thấy được quan hệ ấy — và đó chính là
    lý do bộ phân tích này không sống được bên trong một hook đọc đúng tệp đang ghi.

    `bo_qua` là tệp mà người gọi đã có nội dung MỚI trong tay (hook), nên bản trên đĩa của nó đã
    cũ và không được đọc lại.
    """
    ra: dict[str, tuple[set[int], int]] = {}
    try:
        ten_tep = sorted(os.listdir(thu_muc))
    except Exception:
        return ra
    for ten in ten_tep:
        if not ten.endswith(".go") or ten == bo_qua:
            continue
        try:
            with open(os.path.join(thu_muc, ten), encoding="utf-8") as f:
                ra.update(ham_nhan_perm(ma_thuc_thi_go(f.read())))
        except Exception:
            continue
    return ra


def quet_kho(goc: str) -> tuple[list[str], set[str], list[str]]:
    """Trả (các vi phạm dạng `tệp:dòng khoá (hình dạng)`, tập khoá bảng, nguồn bảng)."""
    bang, nguon = doc_bang_quyen(goc)
    vi_pham: list[str] = []
    if not bang:
        return ["KHÔNG ĐỌC ĐƯỢC BẢNG quyen — không có gì để đối chiếu"], bang, nguon
    cache: dict[str, dict[str, tuple[set[int], int]]] = {}
    for rel, duong in tep_go(goc):
        try:
            with open(duong, encoding="utf-8") as f:
                src = f.read()
        except Exception:
            continue
        if "Perm" not in src:
            continue
        thu_muc = os.path.dirname(duong)
        if thu_muc not in cache:
            cache[thu_muc] = phu_tro_cua_goi(thu_muc)
        for khoa, dong, loai in khoa_trong_go(src, cache[thu_muc]):
            if khoa not in bang:
                vi_pham.append(f"{rel}:{dong}  \"{khoa}\"  ({loai})")
    return vi_pham, bang, nguon
