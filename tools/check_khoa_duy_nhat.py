"""Mọi khoá duy nhất trong migration hoặc hợp thành với `tenant_id`, hoặc khai một `@scope` ngoài xã.

VÌ SAO TỆP NÀY TỒN TẠI

Hai luật đang được tuân thủ mà KHÔNG CÓ GÌ KIỂM — luật 1 bất biến 6 và luật 7 bất biến 3. Cả
hai đều thuộc lớp hỏng im lặng nhất của kho này: vi phạm chúng không làm đỏ một bài test nào,
không làm hỏng một lần chạy nào, và chỉ lộ ra ở một XÃ THẬT với DỮ LIỆU THẬT.

ĐO NGÀY 22/09/2026, TRƯỚC KHI VIẾT TỆP NÀY: 32 tệp migration, 49 khai báo duy nhất, **0 vi
phạm**. Tức cổng này ra đời để GIỮ một tính chất đang đúng, không phải để dọn một đống đã hỏng
— và đó đúng là lúc rẻ nhất để dựng nó.

Bằng chứng cho lời "0 vi phạm" không phải lần chạy xanh đầu tiên, mà là hai phép ĐỘT BIẾN ở
cuối tệp này: thêm `WHERE deleted_at IS NULL` vào một chỉ mục thật, và bỏ `tenant_id` khỏi một
khoá thật — cả hai đều làm cổng đỏ đúng chỗ. Một cổng chưa ai thử làm đỏ thì chưa nói lên gì.

    ─────────────────────────────────────────────────────────────────────────

BẪY 1 — `UNIQUE (…) WHERE deleted_at IS NULL`  (luật 7 bất biến 3)

Một chỉ mục duy nhất từng phần chỉ tính dòng chưa xoá mềm. Hệ quả: xoá mềm một dòng rồi thêm
lại **cùng một mã** thì được — tức MỘT MÃ ĐÃ CẤP ĐƯỢC CẤP LẠI.

Luật 7 bất biến 3 cấm đúng điều đó, và lý do là nghiệp vụ chứ không phải kỹ thuật: mã đã cấp
là mã đã in ra giấy, đã đóng dấu, đã gửi đi. Hai hồ sơ mang cùng một số trong sổ lưu trữ là
hai hồ sơ không ai phân biệt được nữa — và một trong hai đã có người ký.

Nó nguy hiểm vì **trông rất đúng**. `WHERE deleted_at IS NULL` là câu người ta viết theo phản
xạ để "xoá rồi thì thêm lại được", và ở một danh mục thường thì đó là ý tốt. Ở một bảng có mã
nghiệp vụ thì đó là một mã bị tái sử dụng.

Kho anh em `../vigov-require` đã trả giá cho đúng bẫy này
(`kb/90-ephemeral/doi-chieu-vigov-require.md` §9 mục 3), và lỗi ấy chỉ lộ ra khi người dùng
thật làm đúng một việc rất thường: **gỡ ra rồi thêm lại**.

BẪY 2 — khoá duy nhất KHÔNG hợp thành với `tenant_id`  (luật 1 bất biến 6)

`UNIQUE (ma)` thay vì `UNIQUE (tenant_id, ma)` nghĩa là xã thứ hai KHÔNG onboard được: mã của
họ đụng mã xã thứ nhất. Với hệ một xã thì không gì đỏ; nó đỏ vào đúng ngày có xã thứ hai, và
lúc ấy sửa là migration trên dữ liệu đang chạy.

LỐI RA HỢP LỆ, và nó phải TƯỜNG MINH: bảng khai `@scope:` là `cross-tenant` hoặc `platform`
trong chú thích ngay trên `CREATE TABLE` — xem `KHAI_NGOAI_XA` bên dưới để biết hai cách viết
ấy khác nhau chỗ nào. Ba bảng hôm nay dùng lối ấy và cả ba đều đúng: định danh công dân sống
trước khi công dân chọn xã (ADR 0005 · 0020 · 0022), còn sổ tỉnh thành và sổ kế thừa xã là dữ
liệu của tầng nền tảng, không của xã nào.

    ─────────────────────────────────────────────────────────────────────────

VÌ SAO LÀ MỘT CỔNG QUÉT CẢ KHO CHỨ KHÔNG PHẢI MỘT HOOK

Cùng lý do luật 5 bất biến 3c cần hai hình dạng: một hook chỉ thấy MỘT tệp đang được ghi và
không bao giờ đọc lại thứ đã nằm sẵn trên đĩa. Câu đáng hỏi ở đây — *"hôm nay cả kho còn khoá
duy nhất nào sai không"* — chỉ lần quét toàn bộ trả lời được.

Chạy trong `make check`. Trả về 1 khi có vi phạm.
"""

from __future__ import annotations

import io
import os
import re
import sys
import glob

GOC = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# `CREATE TABLE … (` tới dấu `;` cân bằng ngoặc — đủ để lấy thân bảng mà không cần trình phân
# tích SQL. Migration của kho này viết bằng tay theo một khuôn, nên phép này đọc đúng cả 32 tệp;
# ngày nào một tệp viết khác đi thì `SO_KHAI_BAO_TOI_THIEU` bên dưới sẽ tụt và nói ra.
MAU_BANG = re.compile(r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([A-Za-z_][\w.]*)\s*\(', re.I)
MAU_CHI_MUC = re.compile(r'CREATE\s+UNIQUE\s+INDEX[^;]*;', re.I | re.S)
MAU_UNIQUE_TRONG_BANG = re.compile(r'UNIQUE\s*\(([^)]*)\)', re.I)

# Số khai báo duy nhất THẬT, đo ngày 22/09/2026: **49**. Ngưỡng đặt dưới nó một khoảng.
#
# Nó ở đây vì một biểu thức chính quy đọc hụt thân bảng là một cổng in [PASS] mà không kiểm gì —
# cùng hình dạng `kiem_dong_em` đã chết lặng lẽ trong `check_build.py` cùng ngày. Một con số tụt
# đột ngột là tín hiệu đọc hụt, không phải tin vui.
#
# ⚠ 49, KHÔNG PHẢI 59 — và sự khác nhau ấy là một cái bẫy người sau sẽ rơi vào y hệt. Một lần
# `grep 'UNIQUE ('` trên cả tệp cho ra 59, vì mười chỗ nằm trong CHÚ THÍCH: chính các migration
# viết `-- (1) UNIQUE (tenant_id, ma) COUNTS SOFT-DELETED ROWS TOO…` để giải thích vì sao khoá
# ấy cố ý không từng phần. Đếm cả chúng là đếm lời văn nói về khoá thành khoá. Ngưỡng đầu tiên
# của tệp này đặt theo con số 59 ấy và làm chính nó đỏ ngay lần chạy đầu.
SO_KHAI_BAO_TOI_THIEU = 45


def than_bang(s: str, tu: int) -> str:
    """Thân của `CREATE TABLE` bắt đầu ở ngoặc mở tại `tu`, cân bằng ngoặc."""
    sau = 0
    for i in range(tu, len(s)):
        if s[i] == '(':
            sau += 1
        elif s[i] == ')':
            sau -= 1
            if sau == 0:
                return s[tu + 1:i]
    return s[tu:]


# HAI CÁCH VIẾT, CẢ HAI HỢP LỆ — và chúng KHÔNG đồng nghĩa, chỉ cùng hệ quả ở phép kiểm này.
#
#	`platform`      dòng thuộc SỔ CỦA NỀN TẢNG: tỉnh thành, kế thừa xã. Nhà cung cấp gieo,
#	                xã chỉ được CHỌN. Không xã nào sở hữu, nên không có `tenant_id` để hợp thành.
#	`cross-tenant`  dòng nói về một thứ SỐNG QUA NHIỀU XÃ: định danh công dân, tồn tại TRƯỚC
#	                khi người ấy chọn xã nào (ADR 0005 · 0020 · 0022).
#
# Bản đầu của tệp này chỉ nhận `cross-tenant` và làm `tinh_thanh` đỏ oan — bảng ấy khai
# `@scope: platform` từ đầu và hoàn toàn đúng. Ghi ra vì nhận HỤT một cách viết hợp lệ là cách
# nhanh nhất để có người gỡ cổng đi thay vì sửa nó.
#
# `tenant` (21 bảng) KHÔNG nằm ở đây, có chủ ý: đó là lời khai rằng bảng THUỘC một xã, nên nó
# BẮT BUỘC phải có `tenant_id` trong khoá — chính điều phép kiểm này đang đòi.
KHAI_NGOAI_XA = ('cross-tenant', 'platform')


def khai_ngoai_xa(s: str, het: int) -> bool:
    """Bảng có khai `@scope:` là một phạm vi NGOÀI xã trong chú thích ngay trên nó không.

    Chỉ nhìn 40 dòng ngay trước `CREATE TABLE`. Quét cả tệp sẽ cho một bảng mượn lời khai của
    bảng khác trong cùng tệp — và `0005_don_vi_dan_cu_va_danh_muc.sql` có tới bốn bảng.

    ⚠ `split('\\n')[-40:]`, KHÔNG PHẢI `rsplit('\\n', 40)`. Bản đầu của hàm này viết `rsplit`,
    và `rsplit` trả về phần tử ĐẦU là TOÀN BỘ phần còn lại của tệp gộp thành một chuỗi — nên
    `any(...)` khớp một `@scope: platform` nằm cách đó bốn trăm dòng, ở một bảng khác. Kết quả:
    mọi bảng trong mọi tệp có một lời khai ngoài-xã ở bất kỳ đâu đều được tha, và nhánh
    `tenant_id` của cổng này **chết hoàn toàn**.

    Nó chỉ lộ ra vì một phép ĐỘT BIẾN: bỏ `tenant_id` khỏi `UNIQUE (tenant_id, ma_tra_cuu)` của
    `phieu_phan_anh` — một khoá mà mất `tenant_id` là mã tra cứu của xã này đụng mã xã khác —
    và cổng vẫn in [PASS]. Lần chạy xanh đầu tiên không phân biệt được với một cổng đã chết.
    """
    truoc = s[:het].split('\n')[-40:]
    return any('@scope:' in d and any(k in d for k in KHAI_NGOAI_XA) for d in truoc)


def kiem() -> tuple[list[str], int]:
    loi: list[str] = []
    dem = 0

    for f in sorted(glob.glob(os.path.join(GOC, 'service-*', 'migrations', '*.sql'))):
        ngan = os.path.relpath(f, GOC).replace('\\', '/')
        s = io.open(f, encoding='utf-8').read()

        # 1. UNIQUE bên trong CREATE TABLE
        for m in MAU_BANG.finditer(s):
            ten_bang = m.group(1)
            than = than_bang(s, m.end() - 1)
            cross = khai_ngoai_xa(s, m.start())
            for u in MAU_UNIQUE_TRONG_BANG.finditer(than):
                dem += 1
                cot = [c.strip().lower() for c in u.group(1).split(',')]
                dong = s[:m.end()].count('\n') + than[:u.start()].count('\n') + 1
                if 'tenant_id' not in cot and not cross:
                    loi.append(
                        f"{ngan}:{dong} bảng `{ten_bang}` — UNIQUE ({', '.join(cot)}) "
                        f"KHÔNG hợp thành với `tenant_id`\n"
                        f"        → Xã thứ hai không onboard được: mã của họ đụng mã xã thứ "
                        f"nhất. Với một xã thì không gì đỏ; nó đỏ vào đúng ngày có xã thứ hai, "
                        f"và lúc ấy sửa là migration trên dữ liệu đang chạy (luật 1 bất biến 6).\n"
                        f"        → Nếu bảng này THẬT SỰ thuộc tầng nền tảng, khai "
                        f"`-- @scope:  cross-tenant` trong chú thích ngay trên CREATE TABLE."
                    )

        # 2. CREATE UNIQUE INDEX — chỗ duy nhất `WHERE` xuất hiện được
        for m in MAU_CHI_MUC.finditer(s):
            dem += 1
            doan = m.group(0)
            dong = s[:m.start()].count('\n') + 1
            ten = re.search(r'INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\S+)', doan, re.I)
            ten = ten.group(1) if ten else '?'

            if re.search(r'WHERE[^;]*deleted_at\s+IS\s+NULL', doan, re.I):
                loi.append(
                    f"{ngan}:{dong} chỉ mục `{ten}` — UNIQUE kèm `WHERE deleted_at IS NULL`\n"
                    f"        → Chỉ mục duy nhất TỪNG PHẦN chỉ tính dòng chưa xoá mềm, nên xoá "
                    f"mềm rồi thêm lại CÙNG MỘT MÃ là được: MỘT MÃ ĐÃ CẤP ĐƯỢC CẤP LẠI "
                    f"(luật 7 bất biến 3).\n"
                    f"        → Mã đã cấp là mã đã in ra giấy, đã đóng dấu, đã gửi đi. Hai hồ sơ "
                    f"cùng số trong sổ lưu trữ là hai hồ sơ không ai phân biệt được nữa — và một "
                    f"trong hai đã có người ký.\n"
                    f"        → Bỏ mệnh đề `WHERE`. Muốn cho phép thêm lại thì đó là một quyết "
                    f"định của khách, không phải một dòng trong migration."
                )

            cot = re.search(r'\(([^)]*)\)', doan)
            cot = [c.strip().lower() for c in cot.group(1).split(',')] if cot else []
            if 'tenant_id' not in cot and not khai_ngoai_xa(s, m.start()):
                loi.append(
                    f"{ngan}:{dong} chỉ mục `{ten}` — KHÔNG hợp thành với `tenant_id`, và bảng "
                    f"không khai `@scope:` ngoài xã (luật 1 bất biến 6)"
                )

    return loi, dem


def main() -> int:
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding='utf-8', errors='replace')
        except Exception:
            pass

    tep = glob.glob(os.path.join(GOC, 'service-*', 'migrations', '*.sql'))
    if not tep:
        print('[FAIL] không thấy tệp migration nào — phép quét này đang không kiểm gì cả')
        return 1

    loi, dem = kiem()

    # ĐỌC HỤT THÂN BẢNG LÀ MỘT CỔNG CHẾT, và nó không tự nói ra. Con số đo được hôm viết là 59;
    # tụt xuống dưới ngưỡng nghĩa là biểu thức chính quy không còn đọc được khuôn migration nữa,
    # chứ không phải kho vừa bớt khoá duy nhất.
    if dem < SO_KHAI_BAO_TOI_THIEU:
        print(f'[FAIL] chỉ đọc được {dem} khai báo duy nhất trên {len(tep)} tệp, '
              f'ngưỡng là {SO_KHAI_BAO_TOI_THIEU}')
        print('      - Nhiều khả năng một tệp migration viết khác khuôn và phép đọc thân bảng '
              'đang hụt. Sửa `than_bang`/`MAU_BANG`, ĐỪNG hạ ngưỡng.')
        return 1

    if loi:
        print(f'[FAIL] khoá duy nhất — {len(loi)} vi phạm trên {dem} khai báo, {len(tep)} tệp')
        for l in loi:
            print(f'      - {l}')
        return 1

    print(f'[PASS] khoá duy nhất — {len(tep)} tệp migration · {dem} khai báo · '
          f'0 khoá thiếu `tenant_id` · 0 khoá tính sót dòng đã xoá mềm')
    return 0


if __name__ == '__main__':
    sys.exit(main())
