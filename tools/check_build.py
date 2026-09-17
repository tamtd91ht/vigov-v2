"""Mỗi dịch vụ tự dựng và tự đóng gói — và không dịch vụ nào đánh rơi bất biến an toàn.

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

CÙNG MỘT LẬP LUẬN ÁP CHO Jenkinsfile. Mỗi dịch vụ tự quyết build cái gì và khi nào. Phần
dễ đánh rơi nhất ở đó KHÔNG phải thư mục của chính nó mà là MÃ DÙNG CHUNG: tám dịch vụ chung
một `go.mod` và một `core/`, nên một pipeline chỉ kích hoạt theo `<tên>/**` sẽ ngồi im
khi `core/authz` được vá. Mọi pipeline vẫn xanh, không cái nào chạy, và bản vá nằm yên trong
kho mã trong khi tám ảnh đang chạy vẫn mang mã cũ.

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
        "kiểm core/gen trước khi biên dịch",
        re.compile(r"test\s+-d\s+core/gen"),
        "core/gen nằm trong .gitignore và mã nguồn import nó. Thiếu phép kiểm này, một bản "
        "checkout sạch đổ ở lỗi 'package not found' không chỉ ra được nguyên nhân.",
    ),
]


def dich_vu() -> list[str]:
    """Danh sách dịch vụ đọc TỪ ĐĨA, không chép tay — cùng lý do với Jenkinsfile."""
    # Bố cục phẳng: mỗi dịch vụ là một thư mục CẤP MỘT, ngang cấp với core/ và web-admin/.
    # Dấu hiệu nhận biết là `<tên>/cmd/server` — không phải một danh sách gõ tay, và cũng
    # không phải vị trí trong cây thư mục.
    return sorted(
        d for d in os.listdir(GOC)
        if not d.startswith(".")
        and os.path.isdir(os.path.join(GOC, d, "cmd", "server"))
    )


# Đường dẫn mà MỌI pipeline dịch vụ phải coi là tín hiệu dựng lại.
#
# `gen/` cố ý KHÔNG có mặt: nó nằm trong .gitignore nên không bao giờ xuất hiện trong
# changeset. `proto/**` là thứ sinh ra nó, nên proto/ mới là tín hiệu đúng.
DUONG_DUNG_CHUNG = ["core/**", "proto/**", "go.work"]


# Thư mục cấp một ĐƯỢC PHÉP đi theo ngữ cảnh build của dịch vụ Go, ngoài chính tám dịch vụ.
#
# `core/` vì mọi go.mod `replace` nó bằng `../core`, và `proto/` vì `core/gen` sinh ra từ đó.
# Không có mục thứ ba: mọi thứ khác ở cấp một là mã không chạy trong runtime của dịch vụ Go.
DUOC_DI_THEO = {"core", "proto"}


def kiem_dockerignore(svcs: list[str], loi: list[str]) -> None:
    """Mọi thư mục cấp một hoặc là mã của dịch vụ Go, hoặc bị loại khỏi ngữ cảnh build.

    VÌ SAO PHÉP KIỂM NÀY TỒN TẠI — nó ra đời từ một lần hỏng thật. Bố cục cũ gom ba ứng dụng
    web dưới `apps/`, và .dockerignore loại trừ đúng một dòng `apps`. Bố cục phẳng (ADR 0015)
    bỏ tầng đó đi; dòng `apps` ở lại, trỏ vào một thư mục không còn tồn tại. Nó không loại trừ
    gì nữa, mà vẫn nằm đó trông y hệt một biện pháp — và ba cây nguồn web lặng lẽ đi theo ngữ
    cảnh build của cả tám dịch vụ Go. Không có gì đỏ: build vẫn chạy, ảnh vẫn đúng.

    Nên điều được kiểm KHÔNG phải "dòng `apps` còn không" — một phép kiểm như thế mục ruỗng
    cùng nhịp với thứ nó canh. Kiểm chiều ngược lại: mọi thư mục cấp một phải được kể tới ở
    một trong ba chỗ, nên một thư mục MỚI sinh ra cũng đỏ chứ không chỉ một dòng cũ chết đi.
    """
    duong = os.path.join(GOC, ".dockerignore")
    if not os.path.isfile(duong):
        loi.append(
            ".dockerignore KHÔNG TỒN TẠI — toàn bộ kho, gồm .git với đầy đủ lịch sử, đi theo "
            "ngữ cảnh build của tám dịch vụ."
        )
        return

    with open(duong, encoding="utf-8") as f:
        # Chỉ lấy mẫu là TÊN THƯ MỤC TRẦN. `**/node_modules`, `*.pem`, `!.env.example` là mẫu
        # tệp — chúng không trả lời được câu hỏi "thư mục cấp một này có bị loại không".
        loai = {
            d.strip() for d in f
            if d.strip() and not d.startswith("#")
            and not any(c in d for c in "*!/")
        }

    for ten in sorted(os.listdir(GOC)):
        if ten.startswith(".") or not os.path.isdir(os.path.join(GOC, ten)):
            continue
        if ten in svcs or ten in DUOC_DI_THEO or ten in loai:
            continue
        loi.append(
            f"thư mục cấp một '{ten}/' không bị .dockerignore loại, và cũng không phải mã của "
            f"dịch vụ Go"
            f"\n        → Nó đi theo ngữ cảnh build của CẢ TÁM dịch vụ. Không có gì đỏ vì việc "
            f"này không làm hỏng ảnh — nó chỉ đưa mã không thuộc dịch vụ vào tầm với của mọi "
            f"lệnh COPY, và một tệp đã vào ảnh rồi tới registry thì không gọi về được (luật 8)."
            f"\n        → Thêm '{ten}' vào .dockerignore, hoặc vào DUOC_DI_THEO nếu ảnh Go thật "
            f"sự cần nó."
        )


def tuong_doi(duong: str) -> str:
    """Đường dẫn theo gốc kho. Báo cáo đi vào log CI; đường tuyệt đối của máy chủ build
    chỉ là nhiễu, và là nhiễu khác nhau trên mỗi máy."""
    return os.path.relpath(duong, GOC).replace("\\", "/")


def kiem_pipeline(duong: str, ten_rieng: str, loi: list[str]) -> None:
    """Một pipeline phải kích hoạt theo mã dùng chung, và không được đẩy thẻ di động."""
    if not os.path.isfile(duong):
        loi.append(
            f"{tuong_doi(duong)} KHÔNG TỒN TẠI — thành phần này không có pipeline, nên nó sẽ không bao "
            f"giờ được dựng lại và cũng không có gì đỏ để báo."
        )
        return

    with open(duong, encoding="utf-8") as f:
        noi_dung = f.read()

    # CHỈ ĐỌC THÂN `duongKichHoat()`, không quét cả tệp.
    #
    # Quét cả tệp là phép kiểm tự vô hiệu hoá mình: khối chú thích ở cuối mỗi pipeline có
    # NHẮC TỚI `<tên>/**` và `core/**` để giải thích vì sao chúng phải có mặt — nên
    # một tệp đã gỡ chúng khỏi danh sách thật vẫn "chứa" đủ chuỗi và vẫn xanh. Đo được điều
    # này bằng một lượt đột biến, không phải bằng đọc lại.
    than = re.search(r"List<String>\s+duongKichHoat\(\)\s*\{(.*?)\}", noi_dung, re.S)
    if than is None:
        loi.append(
            f"{tuong_doi(duong)} không khai `duongKichHoat()` — không xác định được nó dựng lại khi nào."
        )
        return
    danh_sach = than.group(1)

    if ten_rieng not in danh_sach:
        loi.append(
            f"{tuong_doi(duong)} không kích hoạt theo '{ten_rieng}' — nhiều khả năng chép từ thành phần "
            f"khác mà quên sửa, tức nó đang dựng lại theo nhịp của một thành phần khác."
        )

    for mau in DUONG_DUNG_CHUNG:
        if mau not in danh_sach:
            loi.append(
                f"{tuong_doi(duong)} thiếu đường kích hoạt '{mau}'\n"
                f"        → Tám dịch vụ dùng chung một go.mod và một core/. Thiếu mẫu này thì "
                f"một bản vá trong mã dùng chung KHÔNG kích hoạt dịch vụ này: pipeline vẫn "
                f"xanh, không chạy, và ảnh đang chạy vẫn mang mã cũ."
            )

    # Thẻ di động. Quy ước nêu ở cuối Jenkinsfile gốc; giữ cho chín tệp kia không ai phá.
    if re.search(r":latest\b", noi_dung):
        loi.append(
            f"{tuong_doi(duong)} đẩy thẻ di động `latest`\n"
            f"        → Hai pod cùng một manifest có thể chạy hai đoạn mã khác nhau tuỳ lúc "
            f"kéo ảnh. Với hồ sơ hành chính có giá trị pháp lý, câu 'bản nào đang chạy lúc "
            f"đó' phải trả lời được bằng mã commit."
        )


def main() -> int:
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass

    svcs = dich_vu()
    if not svcs:
        print("[FAIL] không thấy dịch vụ nào dưới */cmd/server")
        return 1

    loi: list[str] = []

    for svc in svcs:
        duong = os.path.join(GOC, svc, "Dockerfile")
        if not os.path.isfile(duong):
            loi.append(
                f"{svc}/Dockerfile KHÔNG TỒN TẠI — dịch vụ này sẽ không có ảnh, "
                f"và pipeline vẫn xanh vì nó đọc danh sách dịch vụ từ đĩa chứ không biết "
                f"dịch vụ nào đáng lẽ phải đóng gói được."
            )
            continue

        with open(duong, encoding="utf-8") as f:
            noi_dung = f.read()

        for ten, mau, vi_sao in BAT_BIEN:
            if not mau.search(noi_dung):
                loi.append(f"{svc}/Dockerfile thiếu: {ten}\n        → {vi_sao}")

        # Dịch vụ phải biên dịch CHÍNH NÓ. Một tệp chép từ dịch vụ khác mà quên sửa đường
        # dẫn sẽ đóng gói nhị phân của dịch vụ kia dưới cái tên của dịch vụ này — ảnh chạy
        # được, healthz xanh, và nó phục vụ sai toàn bộ.
        # Dockerfile đặt WORKDIR vào module của dịch vụ rồi build `./cmd/server`. Phép kiểm
        # phải bám vào WORKDIR, vì đó là dòng duy nhất còn mang tên dịch vụ — chép tệp từ
        # dịch vụ khác mà quên sửa nó thì ảnh đóng gói nhị phân của dịch vụ kia dưới cái tên
        # của dịch vụ này: ảnh chạy được, healthz xanh, và nó phục vụ sai toàn bộ.
        if f"WORKDIR /src/{svc}" not in noi_dung:
            loi.append(
                f"{svc}/Dockerfile không đặt WORKDIR /src/{svc} trước khi biên dịch "
                f"— nhiều khả năng chép từ dịch vụ khác mà quên sửa đường dẫn."
            )

        kiem_pipeline(os.path.join(GOC, svc, "Jenkinsfile"),
                      f"{svc}/**", loi)

    kiem_dockerignore(svcs, loi)

    web = os.path.join(GOC, "web-admin", "Dockerfile")
    if not os.path.isfile(web):
        loi.append("web-admin/Dockerfile KHÔNG TỒN TẠI")
    else:
        with open(web, encoding="utf-8") as f:
            noi_dung = f.read()
        # Rào NEXT_PUBLIC_ là thứ duy nhất chặn một giá trị của một xã bị nung vào bundle
        # rồi đem chạy cho mọi xã (luật 1 bất biến 10, luật 8 bất biến 4).
        if "NEXT_PUBLIC_" not in noi_dung:
            loi.append(
                "web-admin/Dockerfile mất rào chắn NEXT_PUBLIC_*\n"
                "        → Biến đó bị nung vào bundle trình duyệt. Hỏng không lộ ở xã đầu "
                "tiên mà ở xã THỨ HAI, dưới dạng tên xã khác hiện trên màn hình một cơ quan "
                "nhà nước, khi ảnh đã chạy ở mọi nơi."
            )
        if re.search(r"^\s*USER\s+(?!root\b|0\b)\S+", noi_dung, re.M) is None:
            loi.append("web-admin/Dockerfile không khai USER không phải root")

    # Web có danh sách kích hoạt riêng (hợp đồng REST, không phải core/), nên nó không đi
    # qua kiem_pipeline — chỉ kiểm hai điều thật sự bắt buộc với nó.
    wj = os.path.join(GOC, "web-admin", "Jenkinsfile")
    if not os.path.isfile(wj):
        loi.append("web-admin/Jenkinsfile KHÔNG TỒN TẠI")
    else:
        with open(wj, encoding="utf-8") as f:
            nd = f.read()
        than_web = re.search(r"List<String>\s+duongKichHoat\(\)\s*\{(.*?)\}", nd, re.S)
        ds_web = than_web.group(1) if than_web else ""
        if "kb/20-contracts/**" not in ds_web:
            loi.append(
                "web-admin/Jenkinsfile không kích hoạt theo 'kb/20-contracts/**'\n"
                "        → Hợp đồng REST đổi mà web không dựng lại thì nó vẫn gọi hình dạng "
                "cũ, và TypeScript vẫn xanh vì đang tin vào schema.gen.ts cũ."
            )
        if re.search(r":latest\b", nd):
            loi.append("web-admin/Jenkinsfile đẩy thẻ di động `latest`")

    if loi:
        print(f"[FAIL] hồ sơ dựng — {len(loi)} vấn đề trên {len(svcs)} dịch vụ + web")
        for l in loi:
            print(f"      - {l}")
        return 1

    print(f"[PASS] hồ sơ dựng — {len(svcs)} dịch vụ + web · "
          f"{len(svcs) + 1} Dockerfile · {len(svcs) + 1} Jenkinsfile · "
          f"{len(BAT_BIEN)} bất biến an toàn · ngữ cảnh build kín · 0 vi phạm")
    return 0


if __name__ == "__main__":
    sys.exit(main())
