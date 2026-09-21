#!/usr/bin/env python3
"""Đối chiếu MỌI khoá quyền trong mã Go với bảng `quyen`. Cổng, chạy trong `make check`.

LỚP LỖI: một chuỗi được trao cho `authz.RequirePermission`, ép sang `authz.Perm`, hay cấp qua
một bộ đồ thử mang kiểu ấy, mà bảng `quyen` KHÔNG có. Không quản trị viên xã nào tick được ô
ấy trên màn Phân quyền, nên tuyến giữ nó trả 403 với MỌI tài khoản, mãi mãi — và phép kiểm vẫn
xanh, vì một bộ đồ thử giả cấp bất kỳ chuỗi nào.

Đo ngày 21/09/2026: kho này có BA chuỗi như thế cùng lúc — `finance.read`, `map.read`,
`document.approve` — sống qua nhiều phiên và không rào nào thấy.

VÌ SAO CỔNG NÀY TỒN TẠI BÊN CẠNH `.claude/hooks/quyen_key_guard.py`, chứ không thay nó:
hook chặn lúc GHI, và chỉ nhìn thấy một tệp. Ba khiếm khuyết trên đã nằm sẵn trên đĩa trước
khi có hook nào; không hook nào đọc lại chúng bao giờ. Chỉ một lần quét TOÀN KHO mới trả lời
được câu "hôm nay cả kho còn khoá bịa nào không" — và trả lời lại được sau mỗi lần trộn nhánh,
mỗi lần sửa bằng trình soạn thảo khác, mỗi lần một migration bỏ đi một khoá và bỏ rơi các
tuyến đang gọi tên nó.

Chạy:  python tools/check_quyen.py
Mã thoát: 0 = khớp hết · 1 = có khoá không tồn tại, hoặc không đọc được bảng.
"""

from __future__ import annotations

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import quyen_keys as qk  # noqa: E402

for _s in (sys.stdout, sys.stderr):
    try:
        _s.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def _da_dung(goc: str, bang: set) -> set:
    """Các khoá của bảng thật sự được mã Go gọi tên."""
    ra = set()
    cache: dict[str, dict] = {}
    for _, duong in qk.tep_go(goc):
        try:
            with open(duong, encoding="utf-8") as f:
                src = f.read()
        except Exception:
            continue
        if "Perm" not in src:
            continue
        d = os.path.dirname(duong)
        if d not in cache:
            cache[d] = qk.phu_tro_cua_goi(d)
        ra |= {k for k, _, _ in qk.khoa_trong_go(src, cache[d]) if k in bang}
    return ra


def main() -> int:
    vi_pham, bang, nguon = qk.quet_kho(ROOT)

    # HỎNG THÌ ĐỎ. Một bảng đọc ra rỗng là cổng đã mất nguồn chuẩn — im lặng ở đó là đúng
    # hình dạng "rào trông như đang canh mà đã chết" mà kho này đã gặp nhiều lần.
    if not bang:
        print("[FAIL] Không đọc được khoá nào từ `INSERT INTO quyen` trong "
              f"{qk.THU_MUC_BANG}/ — cổng mất nguồn chuẩn, không phải 'không có vi phạm'.")
        return 1

    dung = sorted({v.split('"')[1] for v in vi_pham})
    print(f"[{'FAIL' if vi_pham else 'PASS'}] Khoá quyền trong mã <-> bảng `quyen` — "
          f"{len(bang)} khoá trong bảng ({len(nguon)} dòng gieo) · "
          f"{len(vi_pham)} chỗ dùng khoá không tồn tại")

    # CHIỀU NGƯỢC LẠI, BÁO CHỨ KHÔNG ĐỎ. Một khoá đã gieo mà chưa tuyến nào gọi tên là trạng
    # thái BÌNH THƯỜNG ở đây: `feedback.classify` và `feedback.unmask` được khách chốt ngày
    # 20/09 (ADR 0030) và tuyến của chúng chưa viết. Bắt cổng đỏ vì điều đó là bắt cổng đỏ vì
    # một câu hỏi khách chưa trả lời (#27) — và một cổng luôn đỏ vì chuyện không ai sửa được
    # là cổng người ta học cách bỏ qua. Con số vẫn phải hiện: nó là nửa còn lại của cùng một
    # sự lệch, và không ai đo nó thì không ai biết nó đang rộng ra.
    print(f"        (thông tin) {len(bang) - len(_da_dung(ROOT, bang))}/{len(bang)} khoá đã "
          "gieo chưa được mã Go nào gọi tên — xem câu hỏi mở #27, KHÔNG phải lỗi")

    if vi_pham:
        for v in vi_pham:
            print(f"        {v}")
        print()
        print("        Mỗi dòng trên là một tuyến hoặc một phép cấp gọi tên một quyền KHÔNG AI")
        print("        CẤP ĐƯỢC. Cách sửa ĐÚNG, theo thứ tự:")
        print("          1. Khoá viết sai chính tả -> sửa cho đúng chuỗi bảng `quyen` lưu.")
        print("          2. Tuyến cần một quyền bảng CHƯA CÓ -> đó là PHÁT HIỆN, báo cho người")
        print("             dùng và ghi vào câu hỏi mở #27. TUYỆT ĐỐI không tự thêm một dòng")
        print("             `INSERT INTO quyen`: đặt tên một quyền là quyết định của khách, và")
        print("             sau khi một xã đã gán nó cho một vai trò thì đổi tên là migration")
        print("             trên dữ liệu phân quyền đang chạy (luật 5 bất biến 3b).")
        print(f"        Khoá bị gọi tên mà bảng không có: {', '.join(dung)}")
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
