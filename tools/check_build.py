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
dễ đánh rơi nhất ở đó KHÔNG phải thư mục của chính nó mà là MÃ DÙNG CHUNG: bảy dịch vụ chung
một `go.mod` và một `core/`, nên một pipeline chỉ kích hoạt theo `<tên>/**` sẽ ngồi im
khi `core/authz` được vá. Mọi pipeline vẫn xanh, không cái nào chạy, và bản vá nằm yên trong
kho mã trong khi tám ảnh đang chạy vẫn mang mã cũ.

Chạy trong `make check`. Trả về 1 khi có vi phạm.
"""

from __future__ import annotations

import fnmatch
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


# Thư mục cấp một ĐƯỢC PHÉP đi theo ngữ cảnh build của dịch vụ Go, ngoài chính bảy dịch vụ.
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
    cảnh build của cả bảy dịch vụ Go. Không có gì đỏ: build vẫn chạy, ảnh vẫn đúng.

    Nên điều được kiểm KHÔNG phải "dòng `apps` còn không" — một phép kiểm như thế mục ruỗng
    cùng nhịp với thứ nó canh. Kiểm chiều ngược lại: mọi thư mục cấp một phải được kể tới ở
    một trong ba chỗ, nên một thư mục MỚI sinh ra cũng đỏ chứ không chỉ một dòng cũ chết đi.
    """
    duong = os.path.join(GOC, ".dockerignore")
    if not os.path.isfile(duong):
        loi.append(
            ".dockerignore KHÔNG TỒN TẠI — toàn bộ kho, gồm .git với đầy đủ lịch sử, đi theo "
            "ngữ cảnh build của bảy dịch vụ."
        )
        return

    with open(duong, encoding="utf-8") as f:
        # Chỉ lấy mẫu CẤP MỘT: không có `/` và không phải phủ định `!`. `**/node_modules` và
        # `!.env.example` không trả lời được câu hỏi "thư mục cấp một này có bị loại không".
        #
        # Mẫu có `*` (`*@tmp`) ĐƯỢC tính, và so bằng fnmatch — Docker dùng cùng cú pháp ấy ở
        # cấp một. Trước 24/09/2026 mọi mẫu có `*` bị bỏ qua, nên thư mục phụ `web-admin@tmp`
        # của Jenkins không có cách nào khai được ngoài một dòng tên cứng cho TỪNG bước `dir()`.
        # Mẫu tệp như `*.pem` cũng lọt vào tập này; nó chỉ khớp một thư mục tên đuôi `.pem`,
        # và thư mục ấy quả thật bị Docker loại, nên kết luận vẫn đúng.
        mau = {
            d.strip() for d in f
            if d.strip() and not d.startswith("#")
            and not any(c in d for c in "!/")
        }

    for ten in sorted(os.listdir(GOC)):
        if ten.startswith(".") or not os.path.isdir(os.path.join(GOC, ten)):
            continue
        if ten in svcs or ten in DUOC_DI_THEO or any(fnmatch.fnmatchcase(ten, m) for m in mau):
            continue
        loi.append(
            f"thư mục cấp một '{ten}/' không bị .dockerignore loại, và cũng không phải mã của "
            f"dịch vụ Go"
            f"\n        → Nó đi theo ngữ cảnh build của CẢ BẢY dịch vụ. Không có gì đỏ vì việc "
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
                f"        → Bảy dịch vụ dùng chung một go.mod và một core/. Thiếu mẫu này thì "
                f"một bản vá trong mã dùng chung KHÔNG kích hoạt dịch vụ này: pipeline vẫn "
                f"xanh, không chạy, và ảnh đang chạy vẫn mang mã cũ."
            )

    # Thẻ di động. Quy ước nêu ở cuối Jenkinsfile gốc; giữ cho tám tệp kia không ai phá.
    if re.search(r":latest\b", noi_dung):
        loi.append(
            f"{tuong_doi(duong)} đẩy thẻ di động `latest`\n"
            f"        → Hai pod cùng một manifest có thể chạy hai đoạn mã khác nhau tuỳ lúc "
            f"kéo ảnh. Với hồ sơ hành chính có giá trị pháp lý, câu 'bản nào đang chạy lúc "
            f"đó' phải trả lời được bằng mã commit."
        )


# ─────────────────────────────────────────────────────────────────────────────────────────
# citizen-app/Jenkinsfile — đẩy Mini App lên Zalo (ADR 0047 câu 8 + §Trả lời mục 3).
#
# Không có Dockerfile, không có ảnh, không có `duongKichHoat()`: nó không đi qua kiem_pipeline. Thứ
# nó CẦM là `ZMP_TOKEN` — một token một App ID, và token của app chung thay được app mà mọi xã ở
# giai đoạn 1 dùng (luật 8). Nên cái phải canh là đường đi của token, và lối đẩy duy nhất.
#
# CHỈ ĐỌC MÃ, KHÔNG ĐỌC CHÚ THÍCH. Khối chú thích đầu tệp NHẮC TỚI `withCredentials`, `.env` và
# `zmp deploy` để giải thích vì sao — quét cả tệp thì một tệp đã gỡ `withCredentials` khỏi mã vẫn
# "chứa" chuỗi ấy và vẫn xanh (cùng bài học của kiem_pipeline). Bỏ dòng mở đầu bằng `//` và khối
# `/* … */`; chú thích CUỐI DÒNG được giữ lại, vì cắt ở `//` là cắt đôi mọi `https://` trong chuỗi.
# Giới hạn thật: một `withCredentials(… ZMP_TOKEN …)` nằm trong chú thích cuối dòng vẫn làm ca
# "phải có" xanh. Các ca "không được có" thì chỉ lệch về phía đỏ oan, không lệch về phía lọt.
# ─────────────────────────────────────────────────────────────────────────────────────────

CITIZEN_JENKINSFILE = os.path.join(GOC, "citizen-app", "Jenkinsfile")


def bo_chu_thich_groovy(noi_dung: str) -> str:
    """Bỏ khối `/* … */` và những dòng mở đầu bằng `//`. Giữ nguyên số dòng."""
    khong_khoi = re.sub(r"/\*.*?\*/", lambda m: "\n" * m.group(0).count("\n"), noi_dung, flags=re.S)
    return "\n".join("" if d.lstrip().startswith("//") else d for d in khong_khoi.split("\n"))


def kiem_jenkins_citizen(noi_dung: str) -> list[str]:
    """THUẦN: vào là văn bản của Jenkinsfile, ra là danh sách vi phạm. Không chạm đĩa, nên
    `tu_kiem_citizen()` chấm được nó bằng chuỗi nguyên văn ở mọi lần chạy."""
    ma = bo_chu_thich_groovy(noi_dung)
    loi: list[str] = []

    # 1. Token vào MÔI TRƯỜNG qua withCredentials — không qua tham số, không qua tệp.
    if not re.search(r"withCredentials\s*\(\s*\[[^\]]*variable\s*:\s*['\"]ZMP_TOKEN['\"]", ma, re.S):
        loi.append(
            "không có `withCredentials([... variable: 'ZMP_TOKEN' ...])`\n"
            "        → Token phải đến từ credential của Jenkins, chỉ trong môi trường của đúng một bước. "
            "Thiếu nó thì hoặc token tới từ một ô nhập / một tệp trên máy build, hoặc zmp-cli rơi về "
            "`citizen-app/.env` — token của app nào người dựng đăng nhập lần cuối (ADR 0047 §Trả lời mục 3)."
        )

    # 2. Credential SUY RA từ đích — một token một App ID. Ghim cả hai nhánh của quy ước tên.
    if "zmp-token-app-chung" not in ma or not re.search(r"[\"']zmp-token-\$\{", ma):
        loi.append(
            "không suy ra credential theo đích (`zmp-token-app-chung` · `zmp-token-${tên-miền}`)\n"
            "        → Đích đẩy là claim appId TRONG token. Một credential dùng chung, hay một ô chọn "
            "credential lúc bấm, là cách ghép tên miền của xã A với token của xã B."
        )

    # 3. Token KHÔNG xuống tệp và KHÔNG ra log.
    for so, dong in enumerate(ma.split("\n"), 1):
        if "ZMP_TOKEN" in dong and re.search(r"(?<![0-9&])>>?(?!&)|\btee\b|writeFile", dong):
            loi.append(
                f"dòng {so} ghi ZMP_TOKEN xuống tệp: `{dong.strip()[:100]}`\n"
                "        → Một token nằm trên đĩa máy build sống lâu hơn lượt chạy, đi theo bản sao lưu "
                "workspace, và không ai xoá hộ khi lượt đỏ giữa chừng (luật 8)."
            )
        # Ca IN neo vào một THAM CHIẾU tới giá trị (`$ZMP_TOKEN`, `${ZMP_TOKEN}`, `env.ZMP_TOKEN`,
        # `printenv ZMP_TOKEN`), không vào chữ trần: một câu báo lỗi NHẮC TÊN biến không làm lộ gì,
        # và lượt đầu của phép kiểm này đã đỏ oan đúng trên một câu như thế ở tệp thật.
        if re.search(r"\$\{?ZMP_TOKEN\b|env\.ZMP_TOKEN\b|printenv\s+ZMP_TOKEN\b", dong) and re.search(
            r"\b(echo|printf|println|print)\b", dong
        ):
            loi.append(
                f"dòng {so} in ZMP_TOKEN ra: `{dong.strip()[:100]}`\n"
                "        → Mặt nạ `****` của Jenkins chỉ che đúng chuỗi nguyên văn; một bản base64, "
                "một đoạn cắt hay một lần in qua `set -x` thì lọt vào log."
            )
    if re.search(r"(?<![0-9&])>>?\s*\S*\.env\b", ma) or re.search(
        r"writeFile[^\n]*file\s*:\s*['\"][^'\"]*\.env['\"]", ma
    ):
        loi.append(
            "ghi một tệp `.env`\n"
            "        → `.env` của citizen-app là chỗ zmp-cli đọc token khi môi trường không có. Job đẩy "
            "không có lý do gì để tạo ra nó."
        )
    for m in re.finditer(r"writeFile\b.{0,400}?ZMP_TOKEN", ma, re.S):
        loi.append(
            f"writeFile mang ZMP_TOKEN: `{m.group(0)[:100]!r}`\n"
            "        → Cùng lý do: token không được xuống đĩa."
        )

    # 4. Lối đẩy duy nhất là scripts/deploy.mjs.
    if not re.search(r"\bnode\s+scripts/deploy\.mjs\b", ma):
        loi.append(
            "không đẩy qua `node scripts/deploy.mjs`\n"
            "        → Script ấy dựng → đồng bộ app-config.json → đẩy trong một mạch, và kiểm claim appId "
            "của token khớp App ID đích trước khi chạy gì cả."
        )
    if re.search(r"\bzmp(?:-cli)?(?:@[\w.]+)?\s+(?:-\S+\s+)*deploy\b", ma):
        loi.append(
            "gọi thẳng `zmp deploy`\n"
            "        → Bỏ qua phép kiểm token–đích của deploy.mjs: một token sai app đè lên app của một "
            "xã khác, và một app-config.json của lần dựng trước nộp lên một app trắng trơn."
        )

    # 5. Hai chốt chặn người bấm — chép từ deploy/Jenkinsfile.
    if not re.search(r"\bnguoiBam\s*\(\s*\)", ma):
        loi.append("không xác định người bấm (`nguoiBam()`) — lượt thay app trên Zalo không có tên người.")
    # Neo vào HÌNH DẠNG CỦA CHỐT (`if (params.PHAT_HANH && … XAC_NHAN …`), không vào việc hai tên
    # cùng một dòng: dòng chặn-tham-số-null cũng nhắc cả hai, nên mẫu lỏng xanh trên chính nó.
    if not re.search(r"if\s*\(\s*params\.PHAT_HANH\s*&&[^\n]*XAC_NHAN", ma):
        loi.append("PHAT_HANH không đòi XAC_NHAN gõ tay — bản phát hành ra người dùng thật chỉ bằng một cú bấm.")

    if re.search(r":latest\b", ma):
        loi.append("dùng thẻ di động `latest`")
    return loi


# Ca PHẢI XANH và PHẢI ĐỎ của kiem_jenkins_citizen, chạy ở MỌI lần chạy tệp này (tức mọi `make
# check`). Một phép kiểm mà không có gì chứng minh nó còn đỏ được là một phép kiểm không phân biệt
# được với một phép kiểm đã chết — bài học của kiem_dong_em ở dưới.
_CITIZEN_TOT = """
// withCredentials, zmp deploy và > .env chỉ nhắc trong chú thích: không được tính.
stage('x') { steps { script {
  env.CRED_ZMP = tenMien ? "zmp-token-${tenMien}" : 'zmp-token-app-chung'
  if (params.PHAT_HANH && params.XAC_NHAN.trim() != env.DICH) { error('x') }
  env.NGUOI_BAM = nguoiBam()
}
  sh 'echo "https://vidu.test" 2>/dev/null >/dev/null'
  withCredentials([string(credentialsId: env.CRED_ZMP, variable: 'ZMP_TOKEN')]) {
    sh '''
      cd citizen-app
      node scripts/deploy.mjs "$@"
    '''
  }
} }
"""

# (đoạn gốc, đoạn thay, mảnh câu báo PHẢI có). Mảnh câu để mỗi ca chứng minh ĐÚNG phép kiểm của nó
# còn sống, không chỉ "có cái gì đó đỏ" — một đột biến làm đỏ hai phép kiểm sẽ che mất việc một
# trong hai đã chết.
_CITIZEN_HONG = {
    "mất withCredentials": (
        "withCredentials([string(credentialsId: env.CRED_ZMP, variable: 'ZMP_TOKEN')]) {", "{",
        "withCredentials",
    ),
    "withCredentials chỉ còn trong chú thích": (
        "  withCredentials([string(credentialsId: env.CRED_ZMP, variable: 'ZMP_TOKEN')]) {",
        "  // withCredentials([string(credentialsId: env.CRED_ZMP, variable: 'ZMP_TOKEN')])\n  {",
        "withCredentials",
    ),
    "token ghi xuống tệp": (
        "      cd citizen-app\n", "      cd citizen-app\n      printf '%s' \"$ZMP_TOKEN\" >tok\n", "xuống tệp",
    ),
    "tạo tệp .env không nhắc tên biến": (
        "      cd citizen-app\n", "      cd citizen-app\n      env | grep ZMP >> citizen-app/.env\n",
        "ghi một tệp `.env`",
    ),
    "token qua tee": ("      cd citizen-app\n", "      cd citizen-app\n      printenv ZMP_TOKEN | tee tok\n", "xuống tệp"),
    "token in ra log": ("      cd citizen-app\n", "      cd citizen-app\n      echo ${ZMP_TOKEN}\n", "in ZMP_TOKEN ra"),
    "writeFile mang token, trải nhiều dòng": (
        "  sh 'echo \"https://vidu.test\" 2>/dev/null >/dev/null'\n",
        "  writeFile(file: 'x.txt',\n            text: env.ZMP_TOKEN)\n",
        "writeFile mang ZMP_TOKEN",
    ),
    "gọi thẳng zmp deploy, bên cạnh deploy.mjs": (
        "node scripts/deploy.mjs \"$@\"",
        "node scripts/deploy.mjs \"$@\"\n      npx --yes zmp-cli@4.0.3 deploy -o dist -p",
        "gọi thẳng `zmp deploy`",
    ),
    "không qua deploy.mjs": ("node scripts/deploy.mjs \"$@\"", "npm run build", "scripts/deploy.mjs"),
    "một credential cho mọi đích": (
        "tenMien ? \"zmp-token-${tenMien}\" : 'zmp-token-app-chung'", "'zmp-token'", "suy ra credential",
    ),
    "bỏ xác nhận phát hành, còn dòng chặn null": (
        "if (params.PHAT_HANH && params.XAC_NHAN.trim() != env.DICH) { error('x') }",
        "if (params.PHAT_HANH == null || params.XAC_NHAN == null) { error('x') }",
        "XAC_NHAN",
    ),
    "bỏ người bấm": ("env.NGUOI_BAM = nguoiBam()", "env.NGUOI_BAM = 'ai-do'", "nguoiBam"),
}


def tu_kiem_citizen() -> list[str]:
    """Chấm chính phép kiểm. Trả về danh sách ca SAI (rỗng là phép kiểm còn sống)."""
    sai: list[str] = []
    tot = kiem_jenkins_citizen(_CITIZEN_TOT)
    if tot:
        sai.append(f"ca PHẢI XANH bị đỏ oan: {tot[0].splitlines()[0]}")
    for ten, (cu, moi, manh) in _CITIZEN_HONG.items():
        if cu not in _CITIZEN_TOT:
            sai.append(f"ca '{ten}' không đột biến được gì — mẫu gốc đã đổi, ca này đang chết")
            continue
        bao = kiem_jenkins_citizen(_CITIZEN_TOT.replace(cu, moi, 1))
        if not any(manh in b for b in bao):
            sai.append(f"ca PHẢI ĐỎ lọt qua: {ten} (không câu báo nào chứa '{manh}')")
    return sai


# deploy/Jenkinsfile — the action picker and its "removed action" guard must list the SAME actions.
#
# WHY: the guard refuses any HANH_DONG not in its own hand-written list (so a "Rebuild" of an old run
# with a since-removed action cannot go green doing nothing). On 02/10/2026 `tao-tai-khoan-van-hanh`
# was added to `choices` but not to the guard, so the job refused the very action it offered and no
# operator account could be created. Two hand-kept copies of one list drift; this check is the second
# copy's owner.
DEPLOY_JENKINSFILE = os.path.join(GOC, "deploy", "Jenkinsfile")


def _quoted(items: str) -> list[str]:
    return re.findall(r"'([^']+)'", items)


def check_deploy_actions(text: str) -> list[str]:
    """PURE: Jenkinsfile text in, violations out — so `self_test_deploy_actions()` can grade it."""
    code = bo_chu_thich_groovy(text)
    choices = re.search(r"choice\s*\(\s*name\s*:\s*'HANH_DONG'\s*,\s*choices\s*:\s*\[([^\]]*)\]", code, re.S)
    guard = re.search(r"params\.HANH_DONG\s+in\s+\[([^\]]*)\]\s*\)\s*\)", code, re.S)
    if choices is None:
        return ["không tìm thấy `choice(name: 'HANH_DONG', choices: [...])`"]
    if guard is None:
        return ["không tìm thấy phép kiểm `!(params.HANH_DONG in [...])` của việc đã gỡ"]
    offered, allowed = _quoted(choices.group(1)), _quoted(guard.group(1))
    out: list[str] = []
    for a in offered:
        if a not in allowed:
            out.append(f"việc '{a}' có trong `choices` nhưng phép kiểm việc-đã-gỡ từ chối nó "
                       "— job tự từ chối việc nó vừa cho chọn")
    for a in allowed:
        if a not in offered:
            out.append(f"việc '{a}' còn trong phép kiểm việc-đã-gỡ nhưng đã ra khỏi `choices` "
                       "— gỡ ở cả hai chỗ")
    return out


_DEPLOY_GOOD = (
    "choice(name: 'HANH_DONG', choices: ['kiem-tra', 'xem-log'], description: 'x')\n"
    "if (!(params.HANH_DONG in ['kiem-tra', 'xem-log'])) { error('x') }\n"
)


def self_test_deploy_actions() -> list[str]:
    """Grade the check itself: the good text must pass, each broken variant must fail."""
    bad: list[str] = []
    if check_deploy_actions(_DEPLOY_GOOD):
        bad.append("ca PHẢI XANH bị đỏ oan")
    for name, broken in {
        "thêm vào choices mà quên phép kiểm": _DEPLOY_GOOD.replace("'xem-log'], desc", "'xem-log', 'moi'], desc"),
        "gỡ khỏi choices mà quên phép kiểm": _DEPLOY_GOOD.replace("['kiem-tra', 'xem-log'], desc", "['kiem-tra'], desc"),
    }.items():
        if broken == _DEPLOY_GOOD or not check_deploy_actions(broken):
            bad.append(f"ca PHẢI ĐỎ lọt qua: {name}")
    return bad


# deploy/van-hanh/Jenkinsfile — the operations job (ADR 0089). Same lesson as check_deploy_actions: the
# form's task table and the dispatcher are two lists in one file, and a task offered by the form with no
# `case` would be refused at run time — or, worse, a `case` left for a task the table marks `off` would
# still run on a "Rebuild". Plus the invariants copied from the old job that nothing else guards here.
VAN_HANH_JENKINSFILE = os.path.join(GOC, "deploy", "van-hanh", "Jenkinsfile")


def check_van_hanh(text: str) -> list[str]:
    """PURE: Jenkinsfile text in, violations out — graded by `self_test_van_hanh()`."""
    code = bo_chu_thich_groovy(text)
    out: list[str] = []
    table = re.search(r"String\s+tasksTable\(\)\s*\{\s*return\s*'''(.*?)'''", code, re.S)
    if table is None:
        return ["không tìm thấy `tasksTable()`"]
    rows = [ln.split("|") for ln in table.group(1).splitlines() if ln.strip()]
    on = {r[1] for r in rows if len(r) == 5 and r[2] == "on"}
    off = {r[1] for r in rows if len(r) == 5 and r[2] == "off"}
    bad_rows = [r for r in rows if len(r) != 5 or r[2] not in ("on", "off") or r[3] not in ("doc", "ghi")]
    for r in bad_rows:
        out.append(f"dòng bảng việc sai dạng (nhóm|việc|on/off|doc/ghi|mô tả): {'|'.join(r)[:80]}")
    dispatch = re.search(r"void\s+runTask\(String\s+\w+\)\s*\{(.*?)\n\}", code, re.S)
    cases = set(re.findall(r"case\s+'([a-z-]+)'\s*:", dispatch.group(1))) if dispatch else set()
    if dispatch is None:
        out.append("không tìm thấy `runTask(...)`")
    for t in sorted(on - cases):
        out.append(f"việc '{t}' bật (on) trong bảng nhưng runTask() không có `case` — form cho chọn, job từ chối")
    for t in sorted(cases - on):
        out.append(f"runTask() có `case '{t}'` nhưng bảng không bật việc ấy"
                   + (" (đang `off` — một lượt Rebuild cũ vẫn chạy được nó)" if t in off else ""))
    if "KUBECONFIG=/u01/rancher/rancher-vigov.yaml" not in code:
        out.append("không đặt KUBECONFIG=/u01/rancher/rancher-vigov.yaml — kubectl rơi về ~/.kube/config")
    if "rancher-omi" in code:
        out.append("nhắc `rancher-omi` trong mã — kubeconfig của dự án KHÁC")
    if re.search(r"kubectl\s+(?:-n\s+\S+\s+)?run\b", code):
        out.append("dùng `kubectl run` — pod một lượt phải `kubectl create -f` + hỏi pha (`run -i` đã treo)")
    if re.search(r"\bset\s+image\b", code):
        out.append("`set image` — đặt ảnh là việc của job dịch vụ (ADR 0089 · deploy/README.md mục 1)")
    if re.search(r"kubectl\s+(?:-n\s+\S+\s+)?apply\b", code):
        out.append("`kubectl apply` — job vận hành không áp manifest/Ingress (ADR 0046 #4)")
    if re.search(r"sslmode=dis" + r"able", code):
        out.append("sslmode tắt TLS (luật 13 cấm #1)")
    if not re.search(r"params\.MT\s*==\s*null\s*\|\|\s*params\.VIEC\s*==\s*null", code):
        out.append("thiếu chốt lượt-đầu-tham-số-null (`params.MT == null || params.VIEC == null`)")
    if not re.search(r"\bnguoiBam\s*\(\s*\)", code):
        out.append("không xác định người bấm (`nguoiBam()`)")
    if not re.search(r"params\.TICKET\s*==~", code):
        out.append("TICKET không được kiểm dạng")
    if not re.search(r"params\.XAC_NHAN[^\n]*!=\s*env\.NS", code):
        out.append("ghi không đòi XAC_NHAN = namespace")
    if "Active Choices" not in code:
        out.append("thiếu câu báo tên plugin \"Active Choices\" khi form không dựng được")
    if re.search(r"^\s*set\s+-x", code, re.M):
        out.append("`set -x` — bí mật lọt vào log qua trace")
    return out


_VAN_HANH_GOOD = """
withEnv(['KUBECONFIG=/u01/rancher/rancher-vigov.yaml']) {}
error('plugin Active Choices')
String tasksTable() {
  return '''
A|tong-quan|on|doc|x
A|tao-xa|off|ghi|x
'''
}
if (params.MT == null || params.VIEC == null || params.CHAY_THU == null) { error('x') }
env.NGUOI_BAM = nguoiBam()
if (params.TICKET && !(params.TICKET ==~ /^[A-Z]$/)) { error('x') }
if ((params.XAC_NHAN ?: '').trim() != env.NS) { error('x') }
void runTask(String viec) {
  switch (viec) {
    case 'tong-quan': taskOverview(); break
  }
}
"""


def self_test_van_hanh() -> list[str]:
    bad: list[str] = []
    if check_van_hanh(_VAN_HANH_GOOD):
        bad.append(f"ca PHẢI XANH bị đỏ oan: {check_van_hanh(_VAN_HANH_GOOD)[0]}")
    for name, (old, new, frag) in {
        "việc bật không có case": ("A|tao-xa|off|", "A|tao-xa|on|", "không có `case`"),
        "case cho việc off": ("case 'tong-quan'", "case 'tao-xa': x(); break\n    case 'tong-quan'", "đang `off`"),
        "kubeconfig dự án khác": ("rancher-vigov.yaml']", "rancher-omi.yaml']", "rancher-omi"),
        "kubectl run": ("env.NGUOI_BAM = nguoiBam()", "env.NGUOI_BAM = nguoiBam()\nsh 'kubectl -n x run p'", "kubectl run"),
        "set image": ("env.NGUOI_BAM = nguoiBam()", "env.NGUOI_BAM = nguoiBam()\nsh 'kubectl set image deploy/x *=y'", "set image"),
        "apply": ("env.NGUOI_BAM = nguoiBam()", "env.NGUOI_BAM = nguoiBam()\nsh 'kubectl -n x apply -f i.yaml'", "kubectl apply"),
        "bỏ chốt null": ("params.MT == null || params.VIEC == null", "params.MT == null", "tham-số-null"),
        "bỏ XAC_NHAN": ("!= env.NS", "!= ''", "XAC_NHAN"),
        "set -x": ("env.NGUOI_BAM = nguoiBam()", "env.NGUOI_BAM = nguoiBam()\n  set -x", "set -x"),
    }.items():
        broken = _VAN_HANH_GOOD.replace(old, new, 1)
        if broken == _VAN_HANH_GOOD or not any(frag in v for v in check_van_hanh(broken)):
            bad.append(f"ca PHẢI ĐỎ lọt qua: {name}")
    return bad


def kiem_dong_em(svcs: list[str], loi: list[str]) -> int:
    """Hạn `Shutdown` trong mã phải NHỎ HƠN `terminationGracePeriodSeconds` của manifest.

    VÌ SAO PHÉP KIỂM NÀY TỒN TẠI, và vì sao nó không phải chuyện vận hành. Hai con số ở hai
    tệp khác nhau, hai ngôn ngữ khác nhau, hai người sửa vào hai lúc khác nhau — và khi chúng
    lệch thì KHÔNG CÓ GÌ HỎNG THEO CÁCH NHÌN THẤY ĐƯỢC. Pod vẫn khởi động, healthz vẫn xanh,
    mọi bài test vẫn qua. Chỉ mỗi lượt deploy là cắt ngang những yêu cầu đang chạy, và không ai
    biết vì đó chính là lúc không ai nhìn.

    HƯỚNG LỆCH DUY NHẤT GÂY HẠI LÀ `grace <= shutdown`: k8s gửi SIGTERM, mã bắt đầu rút êm với
    hạn của nó, rồi k8s `SIGKILL` TRƯỚC khi hạn ấy trôi hết. Đoạn đóng êm chạy được một nửa —
    tức nó tồn tại, được đọc qua như một biện pháp, và không bao giờ làm xong việc của mình.
    Đó đúng là hình dạng "rào chắn trông như đang canh mà đã chết" mà kho này đã mất cả ngày
    17/09 để dọn ở bốn chỗ khác.

    KHÔNG BẮT DỊCH VỤ PHẢI CÓ ĐÓNG ÊM. Phép kiểm chỉ nói: nếu mã CÓ `signal.Notify`, thì hai số
    phải đứng đúng thứ tự. Một dịch vụ chưa có đóng êm là một mục trong sổ tiến độ, không phải
    một lần đỏ ở đây — bắt đỏ thứ chưa ai hứa làm là cách nhanh nhất để có người thêm `# noqa`.

    ⚠ HAI CÂY THƯ MỤC ĐẶT TÊN KHÁC NHAU, và bản đầu tiên của hàm này đã chết vì đúng chỗ đó:
    `dich_vu()` trả `service-petitions` (tên thư mục module), còn manifest nằm ở
    `deploy/base/petitions/`. Ghép thẳng hai cái cho ra một đường dẫn không tồn tại, hàm `continue`
    qua mọi dịch vụ, và cổng in [PASS] — kể cả khi hạ `terminationGracePeriodSeconds` xuống 20.
    Nó được phát hiện bằng một phép ĐỘT BIẾN, không bằng lần chạy xanh đầu tiên.

    Vì thế hàm trả về SỐ CẶP ĐÃ ĐỐI CHIẾU và `main` in con số ấy ra. Một phép kiểm không nói nó
    đã kiểm bao nhiêu thứ là một phép kiểm không phân biệt được với một phép kiểm đã chết.
    """
    da_kiem = 0
    for svc in svcs:
        duong_go = os.path.join(GOC, svc, "cmd", "server", "main.go")
        # `service-petitions` -> `petitions`. Cắt tiền tố chứ không gõ tay bảng ánh xạ: một bảng
        # gõ tay thiếu một dòng thì dịch vụ ấy lặng lẽ không được kiểm, y như lần hỏng vừa rồi.
        ten_deploy = svc[len("service-"):] if svc.startswith("service-") else svc
        duong_yaml = os.path.join(GOC, "deploy", "base", ten_deploy, "deployment.yaml")
        if not os.path.isfile(duong_go) or not os.path.isfile(duong_yaml):
            continue

        with open(duong_go, encoding="utf-8") as f:
            ma = f.read()
        if "signal.Notify" not in ma:
            continue  # chưa hứa đóng êm — xem sổ tiến độ, không phải việc của cổng này
        da_kiem += 1

        # `context.WithTimeout(..., N*time.Second)` — hạn rút êm. Lấy số LỚN NHẤT: identity có
        # hai đoạn chờ song song, và thứ quyết định pod sống bao lâu là đoạn dài nhất.
        hans = [int(m) for m in re.findall(r"WithTimeout\([^,]+,\s*(\d+)\s*\*\s*time\.Second", ma)]
        if not hans:
            loi.append(
                f"{svc}/cmd/server/main.go có `signal.Notify` nhưng không tìm thấy hạn "
                f"`context.WithTimeout(..., N*time.Second)` nào\n"
                f"        → Một đoạn đóng êm không có hạn là một lượt deploy có thể treo vô "
                f"hạn, và k8s sẽ cắt nó ở `terminationGracePeriodSeconds` mà không ai chọn "
                f"con số ấy cho việc này."
            )
            continue
        han = max(hans)

        with open(duong_yaml, encoding="utf-8") as f:
            yaml_txt = f.read()
        m = re.search(r"terminationGracePeriodSeconds:\s*(\d+)", yaml_txt)
        if not m:
            loi.append(
                f"deploy/base/{svc}/deployment.yaml không khai "
                f"`terminationGracePeriodSeconds`, trong khi mã CÓ đóng êm {han}s\n"
                f"        → Mặc định của k8s là 30s. Dựa vào một mặc định không viết ra là "
                f"để hạn ấy đổi theo phiên bản cụm mà không ai đọc lại mã."
            )
            continue
        grace = int(m.group(1))

        if grace <= han:
            loi.append(
                f"{svc}: `terminationGracePeriodSeconds: {grace}` KHÔNG LỚN HƠN hạn rút êm "
                f"{han}s trong cmd/server/main.go\n"
                f"        → k8s sẽ SIGKILL trước khi `Shutdown` xong. Đoạn đóng êm vẫn nằm "
                f"đó, vẫn đọc qua như một biện pháp, và chạy được một nửa — yêu cầu đang dở "
                f"bị cắt giữa chừng đúng vào lúc không ai nhìn."
            )

    return da_kiem


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
    so_dong_em = kiem_dong_em(svcs, loi)

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

    # platform-admin — bàn điều khiển vận hành ViHAT (ADR 0048), cùng khuôn với web-admin. Danh sách
    # kích hoạt chỉ `platform-admin/**`: nó không sinh kiểu từ hợp đồng REST nên không có
    # `kb/20-contracts/**`, và ngữ cảnh dựng là thư mục của nó nên không gì ngoài đó vào được ảnh.
    pa = os.path.join(GOC, "platform-admin", "Dockerfile")
    if not os.path.isfile(pa):
        loi.append("platform-admin/Dockerfile KHÔNG TỒN TẠI — bàn điều khiển vận hành không có ảnh")
    else:
        with open(pa, encoding="utf-8") as f:
            noi_dung = f.read()
        # Với bàn điều khiển, một NEXT_PUBLIC_* là bật hay trỏ khu vận hành bằng hằng số lúc build
        # — ĐIỀU KIỆN DỪNG #5 của ADR 0048.
        if "NEXT_PUBLIC_" not in noi_dung:
            loi.append(
                "platform-admin/Dockerfile mất rào chắn NEXT_PUBLIC_*\n"
                "        → Biến đó bị nung vào bundle trình duyệt; bật hay trỏ khu vận hành bằng một "
                "hằng số lúc build là điều kiện dừng #5 của ADR 0048."
            )
        if re.search(r"^\s*USER\s+(?!root\b|0\b)\S+", noi_dung, re.M) is None:
            loi.append("platform-admin/Dockerfile không khai USER không phải root")

    pj = os.path.join(GOC, "platform-admin", "Jenkinsfile")
    if not os.path.isfile(pj):
        loi.append("platform-admin/Jenkinsfile KHÔNG TỒN TẠI")
    else:
        with open(pj, encoding="utf-8") as f:
            nd = f.read()
        than_pa = re.search(r"List<String>\s+duongKichHoat\(\)\s*\{(.*?)\}", nd, re.S)
        ds_pa = than_pa.group(1) if than_pa else ""
        if "'platform-admin/**'" not in ds_pa:
            loi.append(
                "platform-admin/Jenkinsfile không kích hoạt theo 'platform-admin/**'\n"
                "        → Mã của bàn điều khiển đổi mà job không dựng lại: Harbor thiếu ảnh, cụm chạy "
                "mã cũ, và không có gì đỏ để ai nhìn thấy."
            )
        if re.search(r":latest\b", nd):
            loi.append("platform-admin/Jenkinsfile đẩy thẻ di động `latest`")

    # citizen-app: không Dockerfile, không ảnh — chỉ Jenkinsfile đẩy lên Zalo. Tự chấm phép kiểm
    # TRƯỚC, rồi mới chấm tệp thật: một phép kiểm đã chết thì dòng [PASS] của tệp thật vô nghĩa.
    so_ca_citizen = 1 + len(_CITIZEN_HONG)
    for s in tu_kiem_citizen():
        loi.append(f"tools/check_build.py — phép kiểm citizen-app/Jenkinsfile: {s}")
    if not os.path.isfile(CITIZEN_JENKINSFILE):
        loi.append(
            "citizen-app/Jenkinsfile KHÔNG TỒN TẠI — Mini App chỉ đẩy được từ máy của một người, bằng "
            "token trong `.env` của máy ấy (ADR 0047 câu 8)."
        )
    else:
        with open(CITIZEN_JENKINSFILE, encoding="utf-8") as f:
            for l in kiem_jenkins_citizen(f.read()):
                loi.append(f"citizen-app/Jenkinsfile {l}")

    # deploy/Jenkinsfile: action picker vs removed-action guard. Self-test first, same reason.
    for s in self_test_deploy_actions():
        loi.append(f"tools/check_build.py — phép kiểm deploy/Jenkinsfile: {s}")
    if os.path.isfile(DEPLOY_JENKINSFILE):
        with open(DEPLOY_JENKINSFILE, encoding="utf-8") as f:
            for l in check_deploy_actions(f.read()):
                loi.append(f"deploy/Jenkinsfile {l}")
    else:
        loi.append("deploy/Jenkinsfile KHÔNG TỒN TẠI")

    # deploy/van-hanh/Jenkinsfile: task table vs dispatcher + copied invariants. Self-test first.
    for s in self_test_van_hanh():
        loi.append(f"tools/check_build.py — phép kiểm deploy/van-hanh/Jenkinsfile: {s}")
    if os.path.isfile(VAN_HANH_JENKINSFILE):
        with open(VAN_HANH_JENKINSFILE, encoding="utf-8") as f:
            for l in check_van_hanh(f.read()):
                loi.append(f"deploy/van-hanh/Jenkinsfile {l}")

    if loi:
        print(f"[FAIL] hồ sơ dựng — {len(loi)} vấn đề trên {len(svcs)} dịch vụ + web-admin + platform-admin")
        for l in loi:
            print(f"      - {l}")
        return 1

    # `so_dong_em` IN RA CHỨ KHÔNG ẨN, và đó là bài học của chính phép kiểm ấy: bản đầu ghép sai
    # tên thư mục nên không đối chiếu được cặp nào, mà dòng [PASS] vẫn y hệt lúc nó chạy đúng.
    # Một con số 0 ở đây là câu "cổng này chưa canh gì cả", đọc được mà không cần đột biến.
    print(f"[PASS] hồ sơ dựng — {len(svcs)} dịch vụ + web-admin + platform-admin · "
          f"{len(svcs) + 2} Dockerfile · {len(svcs) + 2} Jenkinsfile · "
          f"{len(BAT_BIEN)} bất biến an toàn · ngữ cảnh build kín · "
          f"{so_dong_em} cặp hạn đóng-êm đối chiếu · "
          f"citizen-app/Jenkinsfile + {so_ca_citizen} ca tự chấm · "
          f"deploy/van-hanh/Jenkinsfile {'có' if os.path.isfile(VAN_HANH_JENKINSFILE) else 'KHÔNG CÓ'} + 10 ca tự chấm · 0 vi phạm")
    return 0


if __name__ == "__main__":
    sys.exit(main())
