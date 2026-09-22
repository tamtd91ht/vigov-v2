#!/usr/bin/env python3
"""RULE 6 — chủ thể một dòng vết phải là MÃ CÁN BỘ, không phải id nội bộ. BLOCK, PreToolUse.

Chặn đúng một thứ: một tệp Go đang được ghi nạp một ĐỊNH DANH NỘI BỘ vào trường `ID` của một
`audit.Actor` CỦA CÁN BỘ. Cột ấy là `audit_log.actor_id` — câu trả lời cho "AI làm việc này"
trên một sổ có giá trị pháp lý, đọc lại nhiều năm sau trong một cuộc khiếu nại hay thanh tra.

VÌ SAO NÓ TỒN TẠI, ĐO CHỨ KHÔNG ĐOÁN. Chính sách đã viết tường minh từ khi tuyến đăng nhập ra
đời — `service-identity/internal/app/dang_nhap.go:151`, `Subject: cb.Ma, // business code, never
the internal id`. Nó nằm trong một CHÚ THÍCH, và không gì đọc được chú thích. Ngày 22/09/2026
sáu chỗ ghi vết mới viết `p.ID` vào đúng cột ấy; mọi phép kiểm vẫn xanh, vì một ULID trông y hệt
một giá trị hợp lệ và bộ đồ thử thì khẳng định đúng cái giá trị sai. Cột khi ấy chứa hai loại
định danh cùng lúc — một cột không ai truy vấn được.

HAI HÌNH DẠNG, HAI VAI TRÒ KHÁC NHAU, VÀ ĐÓ KHÔNG PHẢI SỰ THỪA.

  hook này                    chặn lúc GHI, thấy MỘT tệp.   Rẻ nhất để sửa: sai bị chặn trước
                                                            khi có tuyến, có test và một màn
                                                            hình dựng quanh nó.
  tools/check_audit_actor.py  quét TOÀN KHO.                Thứ duy nhất trả lời được "cả kho
                                                            hôm nay còn chỗ nào ghi sai không".

Hook một mình KHÔNG ĐỦ: năm trong sáu khiếm khuyết ngày 22/09 đã nằm sẵn trên đĩa, và một hook
không bao giờ đọc lại tệp nó không được yêu cầu ghi.

BỘ PHÂN TÍCH NẰM Ở `tools/vet_actor.py`, KHÔNG CHÉP LẠI Ở ĐÂY — và GIỚI HẠN CỦA NÓ ĐƯỢC VIẾT RA
ĐẦY ĐỦ Ở ĐẤY, phải đọc trước khi tin rào này. Một câu: nó bắt HÌNH DẠNG CÚ PHÁP (`p.ID`, hằng
chuỗi `nd-…`/ULID), KHÔNG bắt ngữ nghĩa. `id := p.ID` rồi `audit.Actor{ID: id}` thoát sạch, và
điều đó là CÓ Ý — đoán thêm thì báo sai, và báo sai giết một rào chắn nhanh hơn bỏ sót.

Rào này CỐ Ý KHÔNG dùng `_common.should_skip`: hàm ấy bỏ qua `_test.go`, mà năm bộ đồ thử khẳng
định đúng giá trị sai đều nằm trong `*_test.go`. Một bài kiểm chốt một giá trị sai là một bài
kiểm giữ khiếm khuyết ở nguyên chỗ của nó.
"""

from __future__ import annotations

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from _common import (GOC_DU_AN, block, input_of, ngoai_du_an, noi_dung_sau_sua,  # noqa: E402
                     path_of, read_input, tool_of, utf8_streams)

HOOK = "audit_actor_guard"


def main() -> int:
    utf8_streams()
    data = read_input()
    ti = input_of(data)
    duong = path_of(ti)

    if not duong.endswith(".go"):
        return 0
    # Kho khác không có `core/audit` và không có xã — luật 6 là luật CỦA VIGOV.
    if ngoai_du_an(duong):
        return 0
    # Mã sinh từ `.proto`: sửa nó là chuyện `buf generate` (luật 2 cấm #3).
    if "/core/gen/" in duong or duong.endswith((".pb.go", ".gen.go")):
        return 0

    noi_dung = noi_dung_sau_sua(ti)
    if "Actor" not in noi_dung:
        return 0

    # HỎNG THÌ MỞ, và nói rõ cái giá: bộ phân tích không nạp được thì hook im. Đổi lại,
    # `tools/test_hooks.py` giữ một ca BLOCK cho chính hook này, nên một lần nạp hỏng làm
    # `make check` ĐỎ thay vì trôi qua im lặng — và `tools/check_audit_actor.py` vẫn quét kho.
    try:
        sys.path.insert(0, os.path.join(GOC_DU_AN, "tools"))
        import vet_actor as va
    except Exception:
        return 0

    thieu = va.vi_pham_trong_go(noi_dung)
    if not thieu:
        return 0

    rel = os.path.relpath(os.path.abspath(duong), GOC_DU_AN).replace("\\", "/")
    block(
        HOOK,
        "Luật 6 — vết kiểm toán mang ĐỊNH DANH NỘI BỘ, không phải mã cán bộ",
        [f"{rel}:{d}  {gt}  ({ly})" for d, gt, ly in thieu],
        [
            "  `audit_log.actor_id` trả lời câu 'AI làm việc này' trên một sổ có giá trị pháp lý,",
            "  đọc lại nhiều năm sau trong một cuộc khiếu nại hay thanh tra. `CB-00123` gọi tên",
            "  một người mà không cần lời tra cứu nào còn sống; `01JD9A…` không gọi tên ai.",
            "",
            "  CÁCH ĐÚNG, theo thứ tự:",
            "    1. Dùng `authz.Principal.Ma` thay cho `.ID` — hai trường trả lời hai câu khác",
            "       nhau, xem khối chú thích trên `authz.Principal`.",
            "    2. `Ma` rỗng thì TỪ CHỐI lượt ghi, TUYỆT ĐỐI không quay về `.ID`. Rỗng chỉ xảy",
            "       ra khi dịch vụ đang nói với một `identity` cũ hơn trường `ma`; một giá trị",
            "       thay thế ở đó đưa lỗi này trở lại trong im lặng, với mọi ca kiểm vẫn xanh.",
            "    3. Chủ thể là CÔNG DÂN thì `.ID` là ĐÚNG — công dân không có mã cán bộ. Cho hàm",
            "       tự khẳng định `p.Kind != \"citizen\"` như service-petitions/internal/http/",
            "       gui_phan_anh.go, và rào tự im.",
            "    4. Dữ liệu thử CỐ Ý sai (một ca chứng minh chủ thể ấy BỊ TỪ CHỐI) -> lối thoát",
            "       `@actor-ok: <lý do>`. Lý do là BẮT BUỘC.",
            "",
            "  Quét cả kho:  python tools/check_audit_actor.py",
        ],
        tool_of(data), duong,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
