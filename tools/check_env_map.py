#!/usr/bin/env python3
"""Đối chiếu bảng map biến môi trường ở `deploy/README.md` với mã và với sổ đăng ký.

LỚP LỖI: bảng ở mục 5 của `deploy/README.md` là thứ DUY NHẤT trong kho trả lời câu "biến này
do ConfigMap hay Secret cấp". `core/config` không biết — nó chỉ đọc `os.Getenv`. `.env.example`
không biết — nó chỉ giữ chỗ. `hooks/env_contract_guard.py` cũng không, và nó nói thẳng điều đó
trong phần WHAT IT DELIBERATELY DOES NOT CHECK.

Vì thế bảng ấy là tài liệu viết tay, và tài liệu viết tay cạnh một danh sách sinh ra từ mã là
đúng hình dạng sẽ trôi: thêm một biến vào `config.Load` là việc năm giây, cập nhật bảng là việc
người ta định làm sau. Lúc bảng thiếu một dòng, người vận hành đọc nó và tin rằng đã khai đủ —
rồi pod `CrashLoopBackOff` với một cái tên không có trong tài liệu nào.

Cổng này không kiểm NỘI DUNG của cột "k8s cấp bằng" — không gì quyết được điều đó từ mã. Nó
kiểm đúng thứ quyết được: **bảng có đúng những biến mà `core/config` thật sự đọc không**, và
**biến bắt buộc ở `config.Load` có được đánh dấu bắt buộc trong bảng không**.

VÌ SAO NÓ ĐỨNG CẠNH `hooks/env_contract_guard.py` chứ không thay: hook chặn lúc GHI và chỉ
nhìn một tệp. Nó không bao giờ đọc lại `deploy/README.md`, nên một biến thêm hôm nay và một
bảng quên cập nhật hôm qua là hai tệp nó không bao giờ nhìn cùng lúc.

Chạy:  python tools/check_env_map.py
Mã thoát: 0 = khớp · 1 = lệch, hoặc không đọc được một trong ba tệp nguồn.
"""

from __future__ import annotations

import io
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

CONFIG_GO = os.path.join(ROOT, "core", "config", "config.go")
ENV_MAU = os.path.join(ROOT, ".env.example")
README = os.path.join(ROOT, "deploy", "README.md")

# `os.Getenv("X")` / `os.LookupEnv("X")` — cùng biểu thức hook dùng, cố ý: hai nơi đọc cùng
# một thứ mà bằng hai biểu thức khác nhau là hai câu trả lời sẽ lệch.
DOC_ENV = re.compile(r"os\.(?:Getenv|LookupEnv)\s*\(\s*\"([A-Z0-9_]+)\"\s*\)")

# Dòng `thieu = append(thieu, "X")` trong `config.Load` — danh sách biến thiếu thì DỪNG.
BAT_BUOC = re.compile(r"thieu\s*=\s*append\s*\(\s*thieu\s*,\s*\"([A-Z0-9_]+)\"\s*\)")

# Dòng bảng: `| \`TEN_BIEN\` | <bắt buộc> | <k8s cấp bằng> | ...`
DONG_BANG = re.compile(r"^\|\s*`([A-Z0-9_]+)`\s*\|([^|]*)\|")

# Dòng khai trong `.env.example`.
DONG_ENV = re.compile(r"^([A-Z0-9_]+)=")

# Biến chỉ dùng cho phép kiểm, không dịch vụ nào đọc lúc chạy. Nó có dòng trong `.env.example`
# một cách chính đáng, nhưng nó KHÔNG thuộc bảng map: không pod nào cần nó.
CHI_DE_KIEM = {"VIGOV_TEST_DSN"}


def _doc(duong_dan: str) -> str:
    with io.open(duong_dan, encoding="utf-8") as f:
        return f.read()


def _bang_trong_readme(noi_dung: str) -> dict[str, str]:
    """Trả về {tên biến: ô 'Bắt buộc'} lấy từ bảng map ở mục 5."""
    ra: dict[str, str] = {}
    for dong in noi_dung.splitlines():
        m = DONG_BANG.match(dong)
        if m:
            ra[m.group(1)] = m.group(2).strip()
    return ra


def main() -> int:
    for f in (CONFIG_GO, ENV_MAU, README):
        if not os.path.exists(f):
            print(f"[ĐỎ] không đọc được {os.path.relpath(f, ROOT)}")
            return 1

    cau_hinh = _doc(CONFIG_GO)
    ma_doc = set(DOC_ENV.findall(cau_hinh))
    ma_bat_buoc = set(BAT_BUOC.findall(cau_hinh))

    env_mau = {
        m.group(1)
        for dong in _doc(ENV_MAU).splitlines()
        if (m := DONG_ENV.match(dong))
    }

    bang = _bang_trong_readme(_doc(README))

    vi_pham: list[str] = []

    for ten in sorted(ma_doc - set(bang)):
        vi_pham.append(
            f"`core/config` đọc {ten} nhưng bảng map ở deploy/README.md KHÔNG có dòng nào. "
            f"Người vận hành không biết phải khai nó ở ConfigMap hay Secret."
        )

    for ten in sorted(set(bang) - ma_doc - CHI_DE_KIEM):
        vi_pham.append(
            f"bảng map có {ten} nhưng `core/config` không đọc biến nào tên ấy — "
            f"đổi tên trong mã mà quên bảng, hoặc một dòng đã chết."
        )

    for ten in sorted(ma_doc - env_mau):
        vi_pham.append(
            f"`core/config` đọc {ten} nhưng `.env.example` không có dòng nào (luật 11, bất "
            f"biến 6). Biến ngoài sổ đăng ký là biến người sau phát hiện từ một stack trace."
        )

    for ten in sorted(ma_bat_buoc):
        o = bang.get(ten)
        if o is None:
            continue  # đã báo ở vòng trên
        if not o.lstrip("*").startswith("có"):
            vi_pham.append(
                f"{ten} nằm trong danh sách `thieu` của `config.Load` (thiếu nó thì dịch vụ "
                f"KHÔNG khởi động) nhưng bảng map ghi cột Bắt buộc là {o!r}."
            )

    if vi_pham:
        print("[ĐỎ] bảng map biến môi trường lệch với mã")
        for v in vi_pham:
            print(f"        {v}")
        print()
        print("        Sửa ở deploy/README.md mục 5 — và nhớ cột 'k8s cấp bằng': phép thử là")
        print("        'in ra một dòng log thì có đau không', không phải 'có nhạy cảm không'.")
        print("        Một DSN có mật khẩu là Secret dù nó trông như một địa chỉ.")
        print("        Dependency HOÀN TOÀN MỚI (Kafka chẳng hạn) là STOP CONDITION của luật")
        print("        11 câu 1: hỏi chủ cụm trước, đừng tự đặt tên rồi tự chọn ConfigMap.")
        return 1

    print(
        f"[PASS] map biến môi trường — {len(ma_doc)} biến `core/config` đọc · "
        f"{len(ma_bat_buoc)} bắt buộc tại Load · tất cả có dòng trong `.env.example` "
        f"và trong bảng map của deploy/README.md"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
