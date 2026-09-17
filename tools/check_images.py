"""Mỗi dịch vụ có Dockerfile riêng, và không dịch vụ nào đánh rơi bất biến an toàn.

VÌ SAO TỆP NÀY TỒN TẠI

Dockerfile nằm trong từng dịch vụ, không nằm ở một tệp dùng chung — cách đóng gói là một
phần hợp đồng của dịch vụ với nền tảng, và ngày `reporting` cần font tiếng Việt để sinh PDF
thì nó phải sửa được tệp của nó mà không đụng vào bảy dịch vụ khác.

Cái giá của việc tách ra là tám bản sao sẽ TRÔI. Và chúng không trôi đều nhau: phần bị đánh
rơi trước tiên luôn là phần không gây lỗi ngay — `USER` không phải root, `CGO_ENABLED=0`,
zoneinfo. Một ảnh chạy bằng root vẫn chạy hoàn hảo. Một ảnh thiếu zoneinfo chỉ hỏng vào ngày
ai đó viết phần tính hạn theo giờ làm việc. Không có gì đỏ, và không ai đọc lại tám tệp
Dockerfile để so.

Nên phần CHUNG không được giữ bằng một tệp nữa, mà bằng một phép kiểm. Tách tệp là để dịch
vụ tự quyết phần RIÊNG của nó, không phải để lặng lẽ bỏ phần chung.

Chạy trong `make check`. Trả về 1 khi có vi phạm.
"""

from __future__ import annotations

import os
import re
import sys

GOC = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# (tên, biểu thức phải khớp, vì sao nó quan trọng khi bị thiếu)
BAT_BIEN = [
    (
        "chạy không phải root",
        re.compile(r"^\s*USER\s+(?!root\b|0\b)\S+", re.M),
        "Một ảnh chạy bằng root vẫn chạy hoàn hảo — cho tới khi một lỗ trong mã Go trở thành "
        "quyền root trong container. Phía k8s nên đặt runAsNonRoot: true để pod ĐỔ thay vì "
        "âm thầm leo quyền, nhưng ảnh phải tự đúng trước đã.",
    ),
    (
        "nhị phân tĩnh (CGO_ENABLED=0)",
        re.compile(r"CGO_ENABLED\s*=\s*0"),
        "Thiếu nó, nhị phân liên kết động với libc và sẽ không chạy trên ảnh nền distroless "
        "static — hoặc tệ hơn, chạy được nhưng kéo theo một ảnh nền nặng hơn hẳn.",
    ),
    (
        "ảnh nền distroless",
        re.compile(r"distroless/static"),
        "Ảnh nền có shell nghĩa là một lỗ RCE tìm thấy `sh` để leo tiếp. Đổi ảnh nền là một "
        "quyết định có thật, nhưng phải là quyết định chứ không phải một dòng bị sửa lúc gỡ lỗi.",
    ),
    (
        "chép zoneinfo",
        re.compile(r"COPY\s+--from=\S+\s+/usr/share/zoneinfo"),
        "distroless/static không có /usr/share/zoneinfo. Luật 10 bất biến 4 bắt đếm hạn xử lý "
        "theo GIỜ LÀM VIỆC của từng xã, và `time.LoadLocation` sẽ lỗi khi thiếu nó. Hôm nay "
        "chưa hỏng gì; nó hỏng vào đúng ngày ai đó viết phần tính hạn, ở môi trường thật.",
    ),
    (
        "bỏ đường dẫn máy build (-trimpath)",
        re.compile(r"-trimpath"),
        "Không có nó, stack trace in ra đường dẫn tuyệt đối của máy chủ build.",
    ),
    (
        "kiểm gen/ trước khi biên dịch",
        re.compile(r"test\s+-d\s+gen"),
        "gen/ nằm trong .gitignore và mã nguồn import nó. Thiếu phép kiểm này, một bản "
        "checkout sạch đổ ở lỗi 'package not found' không chỉ ra được nguyên nhân.",
    ),
]


def dich_vu() -> list[str]:
    """Danh sách dịch vụ đọc TỪ ĐĨA, không chép tay — cùng lý do với Jenkinsfile."""
    thu_muc = os.path.join(GOC, "services")
    if not os.path.isdir(thu_muc):
        return []
    return sorted(
        d for d in os.listdir(thu_muc)
        if os.path.isdir(os.path.join(thu_muc, d, "cmd", "server"))
    )


def main() -> int:
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass

    svcs = dich_vu()
    if not svcs:
        print("[FAIL] không thấy dịch vụ nào dưới services/*/cmd/server")
        return 1

    loi: list[str] = []

    for svc in svcs:
        duong = os.path.join(GOC, "services", svc, "Dockerfile")
        if not os.path.isfile(duong):
            loi.append(
                f"services/{svc}/Dockerfile KHÔNG TỒN TẠI — dịch vụ này sẽ không có ảnh, "
                f"và pipeline vẫn xanh vì nó đọc danh sách dịch vụ từ đĩa chứ không biết "
                f"dịch vụ nào đáng lẽ phải đóng gói được."
            )
            continue

        with open(duong, encoding="utf-8") as f:
            noi_dung = f.read()

        for ten, mau, vi_sao in BAT_BIEN:
            if not mau.search(noi_dung):
                loi.append(f"services/{svc}/Dockerfile thiếu: {ten}\n        → {vi_sao}")

        # Dịch vụ phải biên dịch CHÍNH NÓ. Một tệp chép từ dịch vụ khác mà quên sửa đường
        # dẫn sẽ đóng gói nhị phân của dịch vụ kia dưới cái tên của dịch vụ này — ảnh chạy
        # được, healthz xanh, và nó phục vụ sai toàn bộ.
        if f"./services/{svc}/cmd/server" not in noi_dung:
            loi.append(
                f"services/{svc}/Dockerfile không biên dịch ./services/{svc}/cmd/server "
                f"— nhiều khả năng chép từ dịch vụ khác mà quên sửa đường dẫn."
            )

    web = os.path.join(GOC, "apps", "commune-admin", "Dockerfile")
    if not os.path.isfile(web):
        loi.append("apps/commune-admin/Dockerfile KHÔNG TỒN TẠI")
    else:
        with open(web, encoding="utf-8") as f:
            noi_dung = f.read()
        # Rào NEXT_PUBLIC_ là thứ duy nhất chặn một giá trị của một xã bị nung vào bundle
        # rồi đem chạy cho mọi xã (luật 1 bất biến 10, luật 8 bất biến 4).
        if "NEXT_PUBLIC_" not in noi_dung:
            loi.append(
                "apps/commune-admin/Dockerfile mất rào chắn NEXT_PUBLIC_*\n"
                "        → Biến đó bị nung vào bundle trình duyệt. Hỏng không lộ ở xã đầu "
                "tiên mà ở xã THỨ HAI, dưới dạng tên xã khác hiện trên màn hình một cơ quan "
                "nhà nước, khi ảnh đã chạy ở mọi nơi."
            )
        if re.search(r"^\s*USER\s+(?!root\b|0\b)\S+", noi_dung, re.M) is None:
            loi.append("apps/commune-admin/Dockerfile không khai USER không phải root")

    if loi:
        print(f"[FAIL] Dockerfile — {len(loi)} vấn đề trên {len(svcs)} dịch vụ + web")
        for l in loi:
            print(f"      - {l}")
        return 1

    print(f"[PASS] Dockerfile — {len(svcs)} dịch vụ + web · "
          f"{len(BAT_BIEN)} bất biến an toàn · 0 vi phạm")
    return 0


if __name__ == "__main__":
    sys.exit(main())
