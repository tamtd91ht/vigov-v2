#!/usr/bin/env python3
"""Kho `vihat-miniapp` phải nằm CÙNG CẤP với kho này, và phải có mặt khi ai đó làm việc
về Mini App.

VÌ SAO CẦN MỘT HOOK CHO CHUYỆN NÀY — nó không phải một quy ước cho gọn thư mục.

Mini App nằm ở HAI KHO: kho này giữ `citizen-app/` (giao diện chạy BÊN TRONG app), còn
`vihat-miniapp` giữ CHÍNH APP — đăng ký với Zalo, App ID, app secret, backend đăng nhập,
webhook, và QR sinh ra tham số nạp vào `citizen-app`. Một phiên chỉ nhìn thấy nửa ở đây sẽ
kết luận từ nửa ấy, và kết luận ấy sai theo một kiểu rất khó thấy: mã biên dịch được, test
xanh, chỉ là nó được viết vào kho sai.

ĐÓ KHÔNG PHẢI GIẢ THUYẾT. Ngày 20/09/2026 endpoint nhận webhook Zalo được viết vào
`service-platform` của ViGov, biện hộ bằng một câu trích thiếu từ `domain-boundaries.md`, và
nó sống ở đó tới 21/09/2026 mới bị gỡ (ADR 0032). Không rào chắn nào bắt được, vì không rào
chắn nào biết kho kia tồn tại.

HAI LUẬT, và luật thứ hai mới là luật người dùng chốt ngày 21/09/2026:

  1. Làm việc về Mini App mà KHÔNG CÓ kho ấy bên cạnh  -> CHẶN, kèm lệnh clone
  2. Kho ấy nằm ở CHỖ KHÁC, không cùng cấp             -> CHẶN

Luật 2 tồn tại vì công việc này chạy trên NHIỀU THIẾT BỊ. "Cùng cấp với `vigov-v2`" là thứ
duy nhất nói được một lần mà đúng trên mọi máy — một đường dẫn tuyệt đối thì chỉ đúng trên
máy của người viết nó, và ngày nó sai thì không ai biết vì mọi thứ vẫn chạy.

HOOK NÀY KHÔNG ĐỌC NỘI DUNG TỆP, chỉ đọc ĐƯỜNG DẪN và lệnh shell. Nếu nó bắt theo nội dung
thì chính tệp này, `CLAUDE.md` và ADR 0032 đều chứa chuỗi `vihat-miniapp` và đều bị chặn —
một rào chắn tự chặn tài liệu mô tả chính nó là rào chắn sẽ bị gỡ trong tuần.
"""

from __future__ import annotations

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from _common import (  # noqa: E402
    GOC_DU_AN, block, input_of, la_vo_shell, path_of, read_input, tool_of, utf8_streams,
)

# Tên thư mục kho, và nó là tên GIT của kho — đổi tên thư mục lúc clone là tự tạo ra đúng
# tình huống luật 2 chặn.
TEN_KHO = "vihat-miniapp"

REMOTE = "https://github.com/tamtd91ht/vihat-miniapp"

# Thư mục cha của kho này. `GOC_DU_AN` suy từ vị trí của `_common.py`, KHÔNG từ biến môi
# trường — xem chú thích ở đó: một biến thiếu hoặc sai một ký tự sẽ tắt lặng lẽ cả lớp cưỡng
# chế, và đó là kiểu hỏng tệ nhất vì mọi thứ vẫn trông bình thường.
GOC_CHA = os.path.dirname(GOC_DU_AN).replace("\\", "/").rstrip("/")

# Chỗ DUY NHẤT hợp lệ.
DUONG_CHUAN = f"{GOC_CHA}/{TEN_KHO}"


def co_kho() -> bool:
    """Kho có mặt ở đúng chỗ chưa.

    Đòi `.git` chứ không chỉ đòi thư mục: một thư mục rỗng mang đúng tên là thứ dễ tạo ra
    nhất khi ai đó muốn hook im đi, và nó thoả mọi phép kiểm trừ phép kiểm này. Kho được
    clone thật thì `.git` luôn có.
    """
    return os.path.isdir(os.path.join(DUONG_CHUAN, ".git"))


def _doan(path: str) -> list[str]:
    return [x for x in (path or "").replace("\\", "/").split("/") if x not in ("", ".")]


def sai_cho(path: str) -> bool:
    """True khi `path` trỏ vào MỘT kho `vihat-miniapp` KHÔNG nằm cùng cấp với kho này.

    Chuẩn hoá `..` trước khi so — không có bước ấy thì `<gốc>/../vihat-miniapp/x.go` trông
    như một đường dẫn lạ trong khi nó chính là đường chuẩn.
    """
    norm = (path or "").replace("\\", "/")
    if TEN_KHO not in _doan(norm):
        return False
    # Đường dẫn TƯƠNG ĐỐI không kết luận được nó nằm ở đâu -> không chặn. Hỏng về phía cho
    # qua ở ĐÂY là đúng: luật 1 ngay dưới vẫn bắt được ca thật (thiếu kho), còn chặn nhầm một
    # đường dẫn tương đối hợp lệ thì người ta không có cách nào đi tiếp.
    if not (norm.startswith("/") or (len(norm) > 2 and norm[1] == ":")):
        return False
    tuyet_doi = os.path.normpath(norm).replace("\\", "/").rstrip("/")
    return not (tuyet_doi.lower() == DUONG_CHUAN.lower()
                or tuyet_doi.lower().startswith(DUONG_CHUAN.lower() + "/"))


def la_viec_mini_app(path: str, cmd: str) -> bool:
    """Công việc này có thuộc chủ đề Mini App không.

    Ba dấu hiệu, tất cả đều là ĐƯỜNG DẪN hoặc lệnh — không phải nội dung tệp:

      * sửa tệp dưới `citizen-app/`  — nửa Mini App nằm trong kho này
      * sửa tệp dưới `vihat-miniapp/` — nửa kia
      * một lệnh shell nhắc tên kho ấy (`cd`, `go test`, `git clone`)
    """
    doan = _doan(path)
    if "citizen-app" in doan or TEN_KHO in doan:
        return True
    return TEN_KHO in (cmd or "")


def main() -> None:
    utf8_streams()
    data = read_input()
    tool = tool_of(data)
    ti = input_of(data)

    path = path_of(ti)
    cmd = ti.get("command", "") if la_vo_shell(tool) else ""

    # LUẬT 2 trước LUẬT 1, có chủ ý: khi có người trỏ vào một bản sao đặt sai chỗ thì câu trả
    # lời đúng là "để sai chỗ", không phải "chưa clone". Đảo thứ tự thì người ta clone thêm
    # một bản thứ hai và có hai kho lệch nhau — tệ hơn hẳn không có kho nào.
    if sai_cho(path):
        block(
            "miniapp_sibling_guard",
            f"kho {TEN_KHO} đặt sai chỗ",
            [f"đường dẫn: {path}", f"chỗ duy nhất hợp lệ: {DUONG_CHUAN}"],
            [
                f"  `{TEN_KHO}` PHẢI nằm cùng cấp với kho này. Người dùng chốt 21/09/2026, và lý do",
                "  là công việc chạy trên NHIỀU THIẾT BỊ: 'cùng cấp' đúng trên mọi máy, một đường dẫn",
                "  tuyệt đối thì chỉ đúng trên một máy — và ngày nó sai, không ai biết.",
                "",
                "  Hai bản sao ở hai chỗ là hai bản sẽ lệch nhau, và bản bị bỏ quên là bản có người đọc.",
                "",
                f"  Đúng chỗ:  cd {GOC_CHA} && git clone {REMOTE}",
                "",
                "  → CLAUDE.md §THE MINI APP SPANS TWO REPOSITORIES · ADR 0032",
            ],
            tool, path,
        )

    if la_viec_mini_app(path, cmd) and not co_kho():
        block(
            "miniapp_sibling_guard",
            f"làm việc về Mini App mà thiếu kho {TEN_KHO}",
            [f"không thấy: {DUONG_CHUAN}", f"việc đang làm: {path or cmd}"],
            [
                "  Mini App nằm ở HAI KHO. Kho này giữ `citizen-app/` — giao diện chạy BÊN TRONG app.",
                f"  `{TEN_KHO}` giữ CHÍNH APP: đăng ký Zalo, App ID, app secret, backend đăng nhập,",
                "  webhook, và QR sinh tham số nạp vào `citizen-app`.",
                "",
                "  Chỉ nhìn thấy nửa ở đây thì kết luận rút ra từ nửa ấy sẽ sai theo kiểu khó thấy nhất:",
                "  mã biên dịch được, test xanh, chỉ là nó được viết vào KHO SAI. Đã xảy ra thật —",
                "  webhook Zalo sống một ngày trong `service-platform` trước khi bị gỡ (ADR 0032).",
                "",
                f"  Clone về:  cd {GOC_CHA} && git clone {REMOTE}",
                "",
                "  → CLAUDE.md §THE MINI APP SPANS TWO REPOSITORIES",
            ],
            tool, path,
        )

    sys.exit(0)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except Exception:
        # Một hook hỏng KHÔNG BAO GIỜ được chặn công việc — cùng kỷ luật với mọi hook khác
        # trong thư mục này.
        sys.exit(0)
