#!/usr/bin/env python3
"""Đối chiếu MỌI `audit.Actor` cán bộ trong mã Go với chính sách "vết lưu mã cán bộ". Cổng.

LỚP LỖI: một lượt ghi vết nạp `authz.Principal.ID` — ULID nội bộ — vào `audit_log.actor_id`,
cột trả lời "AI làm việc này" trên một sổ có giá trị pháp lý. Chính sách đã viết tường minh ở
`service-identity/internal/app/dang_nhap.go:151` từ lâu, nhưng nằm trong một CHÚ THÍCH, nên
không gì đọc được nó.

Đo ngày 22/09/2026: kho này có SÁU chỗ như thế cùng lúc, cả sáu viết trong cùng một đợt, và
không một ca kiểm nào đỏ — một ULID trông y hệt một giá trị hợp lệ, và bộ đồ thử thì khẳng định
đúng cái giá trị sai ấy. Cột khi ấy chứa hai loại định danh cùng lúc.

VÌ SAO CỔNG NÀY TỒN TẠI BÊN CẠNH `.claude/hooks/audit_actor_guard.py`, chứ không thay nó: hook
chặn lúc GHI và chỉ nhìn thấy MỘT tệp. Năm trong sáu khiếm khuyết trên đã nằm sẵn trên đĩa
trước khi có rào nào, và không hook nào đọc lại chúng bao giờ. Chỉ một lần quét TOÀN KHO mới
trả lời được câu "hôm nay cả kho còn chỗ nào ghi sai định danh không" — và trả lời lại được sau
mỗi lần trộn nhánh, mỗi lần sửa bằng trình soạn thảo khác.

GIỚI HẠN CỦA PHÉP KIỂM NÀY NẰM Ở `tools/vet_actor.py` VÀ PHẢI ĐỌC. Tóm tắt một câu: nó bắt HÌNH
DẠNG CÚ PHÁP, không bắt ngữ nghĩa — `id := p.ID` rồi `audit.Actor{ID: id}` thoát sạch.

Chạy:  python tools/check_audit_actor.py
Mã thoát: 0 = sạch · 1 = có chỗ ghi sai định danh, hoặc không nạp được bộ phân tích.
"""

from __future__ import annotations

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

for _s in (sys.stdout, sys.stderr):
    try:
        _s.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def main() -> int:
    # HỎNG THÌ ĐỎ. Một cổng không nạp được bộ phân tích của mình mà trả 0 là đúng hình dạng "rào
    # trông như đang canh mà đã chết" — kho này đã gặp nhiều lần và đã trả giá cho nó.
    try:
        import vet_actor as va
    except Exception as e:
        print(f"[FAIL] Không nạp được tools/vet_actor.py ({e}) — cổng mất bộ phân tích, "
              "KHÔNG phải 'không có vi phạm'.")
        return 1

    vi_pham, dem = va.quet_kho(ROOT)

    if dem == 0:
        print("[FAIL] Không đọc được tệp Go nào — cổng mất nguồn, không phải 'sạch'.")
        return 1

    print(f"[{'FAIL' if vi_pham else 'PASS'}] Chủ thể vết kiểm toán mang MÃ CÁN BỘ — "
          f"{dem} tệp Go · {len(vi_pham)} chỗ nạp định danh nội bộ")

    if not vi_pham:
        return 0

    for v in vi_pham:
        print(f"        {v}")
    print()
    print("        Mỗi dòng trên ghi một ĐỊNH DANH NỘI BỘ vào `audit_log.actor_id` — cột trả lời")
    print("        'AI làm việc này' trên một sổ có giá trị pháp lý, đọc lại nhiều năm sau trong")
    print("        một cuộc khiếu nại hay thanh tra. `CB-00123` gọi tên một người mà không cần")
    print("        lời tra cứu nào còn sống; `01JD9A…` không gọi tên ai (luật 6 bất biến 2).")
    print()
    print("        CÁCH ĐÚNG, theo thứ tự:")
    print("          1. Dùng `authz.Principal.Ma` thay cho `.ID`. Hai trường trả lời hai câu")
    print("             khác nhau — xem khối chú thích trên `authz.Principal`.")
    print("          2. `Ma` rỗng thì TỪ CHỐI lượt ghi, TUYỆT ĐỐI không quay về `.ID`. Rỗng chỉ")
    print("             xảy ra khi dịch vụ đang nói với một `identity` cũ hơn trường `ma`, và một")
    print("             giá trị thay thế ở đó đưa lỗi này trở lại trong im lặng.")
    print("          3. Chủ thể là CÔNG DÂN thì `.ID` là ĐÚNG — công dân không có mã cán bộ. Nếu")
    print("             rào đọc nhầm, hãy để hàm tự khẳng định `p.Kind != \"citizen\"` như")
    print("             service-petitions/internal/http/gui_phan_anh.go, hoặc viết lối thoát")
    print("             `@actor-ok: <lý do>` — lý do là BẮT BUỘC.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
