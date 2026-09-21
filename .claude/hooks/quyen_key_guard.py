#!/usr/bin/env python3
"""RULE 5 — một khoá quyền phải TỒN TẠI trong bảng `quyen`. BLOCK, PreToolUse.

Chặn đúng một thứ: một tệp Go đang được ghi trao cho tầng quyền một chuỗi mà
`service-identity/migrations/*.sql` chưa bao giờ gieo. Không quản trị viên xã nào tick được ô
ấy, nên tuyến giữ nó trả 403 với MỌI tài khoản, mãi mãi — và không gì đỏ, vì một bộ đồ thử giả
cấp bất kỳ chuỗi nào.

HAI HÌNH DẠNG, HAI VAI TRÒ KHÁC NHAU, VÀ ĐÓ KHÔNG PHẢI SỰ THỪA.

  hook này              chặn lúc GHI, thấy MỘT tệp.        Rẻ nhất để sửa: khoá sai bị chặn
                                                           trước khi có tuyến, có test và có
                                                           một màn hình dựng quanh nó.
  tools/check_quyen.py  quét TOÀN KHO lúc `make check`.    Thứ duy nhất trả lời được "cả kho
                                                           hôm nay còn khoá bịa nào không".

Hook một mình KHÔNG ĐỦ, và đây là lý do đo được chứ không phải lý do trên giấy:
  * Ba khoá bịa của ngày 21/09/2026 (`finance.read`, `map.read`, `document.approve`) đã nằm
    sẵn trên đĩa. Một hook không bao giờ đọc lại tệp nó không được yêu cầu ghi — nó sẽ không
    thấy cả ba, mãi mãi, trong khi trông hoàn toàn như đang canh.
  * Mã vào kho qua nhiều đường không đi qua một lời gọi công cụ: trộn nhánh, `git pull`, một
    trình soạn thảo khác.
  * Chiều ngược lại — một migration BỎ ĐI hay đổi tên một khoá — bỏ rơi mọi tuyến đang gọi tên
    nó, ở những tệp lần ghi ấy không chạm tới.

Cổng một mình cũng KHÔNG ĐỦ: nó nói sau khi tuyến, bài kiểm và màn hình đã viết xong, và trên
máy này không có `make` nên nó là bước người ta phải nhớ chạy.

BỘ PHÂN TÍCH NẰM Ở `tools/quyen_keys.py`, KHÔNG CHÉP LẠI Ở ĐÂY. Hai bản sao của một bộ phân
tích là hai bản sẽ lệch, và bản lệch là bản im lặng — đúng lớp lỗi rào này dựng ra để chặn.

Rào này CỐ Ý KHÔNG dùng `_common.should_skip`: hàm ấy bỏ qua `_test.go`, mà cả ba khiếm khuyết
thật đều nằm trong `*_test.go`. Một bài kiểm cấp một khoá không tồn tại là một bài kiểm dựng
lên một thế giới không có thật rồi khẳng định điều gì đó về thế giới ấy.
"""

from __future__ import annotations

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from _common import (GOC_DU_AN, block, input_of, ngoai_du_an, noi_dung_sau_sua,  # noqa: E402
                     path_of, read_input, tool_of, utf8_streams)

HOOK = "quyen_key_guard"


def main() -> int:
    utf8_streams()
    data = read_input()
    ti = input_of(data)
    duong = path_of(ti)

    if not duong.endswith(".go"):
        return 0
    # Kho khác không có `core/authz` và không có bảng `quyen` — luật 5 là luật CỦA VIGOV.
    if ngoai_du_an(duong):
        return 0
    # Mã sinh từ `.proto`: sửa nó là chuyện `buf generate` (luật 2 cấm #3).
    if "/core/gen/" in duong or duong.endswith((".pb.go", ".gen.go")):
        return 0

    noi_dung = noi_dung_sau_sua(ti)
    if "Perm" not in noi_dung:
        return 0

    # HỎNG THÌ MỞ, và nói rõ cái giá: bộ phân tích không nạp được thì hook im. Đổi lại,
    # `tools/test_hooks.py` giữ một ca BLOCK cho chính hook này, nên một lần nạp hỏng làm
    # `make check` ĐỎ thay vì trôi qua im lặng.
    try:
        sys.path.insert(0, os.path.join(GOC_DU_AN, "tools"))
        import quyen_keys as qk
    except Exception:
        return 0

    goc = GOC_DU_AN
    bang, _ = qk.doc_bang_quyen(goc)
    if not bang:
        return 0

    # Gói = thư mục. `coQuyen(perm authz.Perm)` có thể được khai ở một tệp KHÁC trong cùng thư
    # mục với tệp đang ghi — đọc một tệp thôi thì không bao giờ thấy quan hệ ấy.
    thu_muc = os.path.dirname(os.path.abspath(duong))
    phu_tro = qk.phu_tro_cua_goi(thu_muc, bo_qua=os.path.basename(duong))
    phu_tro.update(qk.ham_nhan_perm(qk.ma_thuc_thi_go(noi_dung)))

    thieu = [(k, d, l) for k, d, l in qk.khoa_trong_go(noi_dung, phu_tro) if k not in bang]
    if not thieu:
        return 0

    rel = os.path.relpath(os.path.abspath(duong), goc).replace("\\", "/")
    block(
        HOOK,
        "Luật 5 — khoá quyền KHÔNG CÓ trong bảng `quyen`",
        [f'{rel}:{d}  "{k}"  ({l})' for k, d, l in thieu],
        [
            f"  Bảng `quyen` có {len(bang)} khoá, gieo ở service-identity/migrations/*.sql.",
            "  Một khoá ngoài bảng là một quyền KHÔNG QUẢN TRỊ VIÊN NÀO CẤP ĐƯỢC: màn Phân",
            "  quyền không có ô ấy, nên tuyến trả 403 với MỌI tài khoản, mãi mãi.",
            "",
            "  CÁCH ĐÚNG, theo thứ tự:",
            "    1. Sai chính tả -> sửa cho đúng chuỗi bảng lưu (luật 5 bất biến 3b: khoá là",
            "       MỘT CHUỖI PHẲNG `<nhóm>.<việc>`, chính chuỗi màn Phân quyền hiện).",
            "    2. Tuyến thật sự cần một quyền bảng CHƯA CÓ -> ĐÂY LÀ PHÁT HIỆN, KHÔNG PHẢI",
            "       CHỖ ĐỂ TỰ THÊM `INSERT INTO quyen`. Dừng lại, báo người dùng, ghi vào câu",
            "       hỏi mở #27 (kb/00-foundation/open-questions.json). Đặc tả ghi 43 quyền mà",
            "       chỉ liệt kê 33; tám khoá còn thiếu là câu KHÁCH chưa trả lời, và một khoá",
            "       tự nghĩ ra là một quyền không ai cấp — sau khi một xã đã gán nó cho một",
            "       vai trò thì đổi tên là migration trên dữ liệu phân quyền đang chạy.",
            "",
            "  Quét cả kho:  python tools/check_quyen.py",
        ],
        tool_of(data), duong,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
