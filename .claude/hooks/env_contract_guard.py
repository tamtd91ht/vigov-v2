"""PreToolUse — enforce rule 11: the infrastructure configuration contract.

WHY THIS HOOK EXISTS: one system serves many communes from one set of k8s manifests, and the
environment is the seam between the code and the cluster. Four defects live in that seam, and
every one of them is invisible in review and expensive afterwards:

  1. A SECOND NAME FOR ONE THING. Service A reads `PG_HOST`, service B reads `POSTGRES_ADDR`.
     Both work. Nobody can answer "which deployments point at the new cluster" any more.
  2. A CLUSTER ORDINAL IN SOURCE. `KAFKA_02_ADDRESS` in Go welds a workload to one cluster:
     moving it stops being one line in one manifest and becomes a code change plus a release
     of every image that reads it.
  3. A DASH IN AN ENV VAR NAME. Legal in a ConfigMap key, illegal in a shell identifier — the
     variable is simply never set, and the service fails with "missing configuration" naming
     a variable that the operator can see right there in the ConfigMap.
  4. A VARIABLE NOBODY CAN DISCOVER. Read somewhere in a handler, absent from `.env.example`,
     so it appears in no template, no manifest and no review.

WHAT IT DELIBERATELY DOES NOT CHECK: whether a value is cluster-shaped at runtime, and whether
the operator bound the right ConfigMap key. Neither is decidable from source. Those live in
`skills/infra-config`, and the shape of a value is a review question, not a hook question.

CALIBRATION: this hook fires only on patterns that cannot be right. A noisy hook is a disabled
hook, and a disabled hook takes the whole layer with it.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "env_contract_guard"

# Every way Go reads the environment. os.LookupEnv is included: it is the same act with a
# second return value, and leaving it out would make the rule trivially routable around.
DOC_ENV = re.compile(r"os\.(?:Getenv|LookupEnv)\s*\(\s*\"([^\"]*)\"\s*\)")

# A cluster ordinal: KAFKA_02_ADDRESS, REDIS_1_DSN, ES_03_ADDRS.
#
# THE NUMBER MUST BE A MIDDLE SEGMENT, AND THAT IS THE WHOLE DISTINCTION. A number wedged
# between two words names WHICH INSTANCE — the thing rule 11 forbidden #2 exists to keep out
# of source. A number at the END qualifies the VALUE and is legitimate: `REDIS_DB_0` is Redis
# database zero, `KAFKA_LOG_PARTITIONS_3` is a count.
#
# Written this way after the first version flagged `REDIS_DB_0` — and after the comment beside
# it claimed it would not. A hook whose comment disagrees with its regex is a hook the next
# reader stops believing.
#
# THE LIMIT, STATED RATHER THAN GLOSSED: a bare `KAFKA_02` with no suffix is not caught. It is
# not a shape anything in this repository uses, and widening the pattern to catch it would take
# `HTTP_PORT_2` with it. Prefer the miss: a noisy hook is a disabled hook.
SO_CUM = re.compile(r"(?:^|_)\d{1,3}_")

# The ONE package allowed to read the environment. Compared on a normalised path so it holds
# on Windows too, where every other hook in this brain has been bitten by backslashes.
GOI_CAU_HINH = "core/config/"

# `// @env-ok: <reason>` — the explicit, logged escape hatch, same discipline as
# `// @cross-tenant:` in rule 1. A reason is mandatory: an exemption without one is an
# exemption nobody dares remove six months later (rule 5, forbidden #4 takes the same line).
THOAT = re.compile(r"//\s*@env-ok:\s*\S+")

WATCH_EXT = (".go",)


def duong_chuan(path: str) -> str:
    return path.replace("\\", "/")


def trong_pham_vi(path: str) -> bool:
    # `core/config` là nơi duy nhất đọc môi trường — nhưng đó là `core/config` CỦA VIGOV. Một
    # kho khác không có gói ấy và không được phép bị đòi nó; đòi thì lối thoát duy nhất của
    # người viết là dính vào core/ của ViGov, tức là chính thứ "tách ra được" bị mất.
    # Xem c.ngoai_du_an: đường dẫn tương đối thì VẪN SOI.
    return (path.endswith(WATCH_EXT) and not c.should_skip(path)
            and not c.ngoai_du_an(path))


def la_goi_cau_hinh(path: str) -> bool:
    """True for the one package allowed to read the environment.

    Matched on the DIRECTORY, not on the file name: `core/config/ha_tang.go` is as much the
    config package as `config.go` is, and a check keyed on one file name would push the next
    variable into a second file to get past the hook — which is the opposite of the point.
    """
    return GOI_CAU_HINH in duong_chuan(path)


def ten_env_trong(noi_dung: str) -> list[tuple[int, str]]:
    """Every environment variable name read in this file, with its 1-based line number."""
    ra: list[tuple[int, str]] = []
    for i, dong in enumerate(noi_dung.splitlines(), 1):
        for m in DOC_ENV.finditer(dong):
            ra.append((i, m.group(1)))
    return ra


def ten_trong_mau(mau: str) -> set[str]:
    """Names declared in `.env.example` — the registry rule 11 invariant 6 names.

    A missing template file yields an empty set rather than an error: this hook must not be
    the thing that stops work because a file it only reads has been moved.
    """
    ra: set[str] = set()
    for dong in mau.splitlines():
        d = dong.strip()
        if not d or d.startswith("#"):
            continue
        if "=" in d:
            ra.add(d.split("=", 1)[0].strip())
    return ra


def quet(noi_dung: str, path: str, mau_env: set[str]) -> list[str]:
    """The pure half — everything this hook decides, with no I/O.

    Kept pure so `tools/test_hooks.py` can exercise it directly. The lesson is written into
    `ban-giao-phien.md`: a hook whose only test is a payload case is a hook that goes on
    passing after its pattern has stopped matching anything real.
    """
    hits: list[str] = []
    la_cau_hinh = la_goi_cau_hinh(path)
    dong_tep = noi_dung.splitlines()

    for dong, ten in ten_env_trong(noi_dung):
        truoc = dong_tep[dong - 2] if dong >= 2 else ""
        if THOAT.search(dong_tep[dong - 1]) or THOAT.search(truoc):
            continue

        if not la_cau_hinh:
            hits.append(
                f"line {dong}: os.Getenv(\"{ten}\") ngoài core/config — "
                "chỉ một gói được đọc môi trường")
            continue

        # Inside core/config the READ is legitimate; the NAME still has to be.
        if "-" in ten:
            hits.append(
                f"line {dong}: \"{ten}\" có dấu gạch ngang — "
                "gạch ngang là cách viết KEY của ConfigMap, tên biến môi trường dùng gạch dưới")
        if SO_CUM.search(ten):
            hits.append(
                f"line {dong}: \"{ten}\" mang số cụm — mã gọi theo VAI TRÒ, "
                "manifest k8s mới chọn cụm")
        if mau_env and ten not in mau_env:
            hits.append(
                f"line {dong}: \"{ten}\" không có dòng nào trong .env.example — "
                "mẫu là sổ đăng ký, biến ngoài sổ là biến không ai tra được")

    return hits


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not trong_pham_vi(path):
        sys.exit(0)

    # The text the EDIT IS ADDING, not the file on disk: this hook must accuse what this change
    # introduces, never a line that was already there. `doc_guard` was patched for taking the
    # opposite input and turning every three-line edit into a whole-file rewrite.
    noi_dung = c.new_content(ti)
    if not noi_dung:
        sys.exit(0)

    mau = ""
    goc = c.project_root(path)
    if goc:
        try:
            with open(os.path.join(goc, ".env.example"), encoding="utf-8") as f:
                mau = f.read()
        except OSError:
            mau = ""

    hits = quet(noi_dung, path, ten_trong_mau(mau))
    if not hits:
        sys.exit(0)

    c.block(
        HOOK,
        f"hợp đồng cấu hình hạ tầng bị vi phạm — {os.path.basename(path)}",
        hits,
        [
            "  MỘT hệ thống, nhiều xã, MỘT bộ manifest. Môi trường là đường nối giữa mã và cụm,",
            "  và mọi lỗi ở đó đều vô hình lúc soi rồi đắt về sau.",
            "",
            "  Đúng cách:",
            "    // trong core/config — nơi DUY NHẤT đọc môi trường",
            "    KafkaLogAddrs: danhSach(os.Getenv(\"KAFKA_LOG_ADDRESS\")),  // VAI TRÒ, không phải cụm",
            "",
            "    # .env.example — mẫu là sổ đăng ký, giá trị chỉ là chỗ giữ chỗ",
            "    KAFKA_LOG_ADDRESS=<dien-vao-tu-configmap>",
            "",
            "    # k8s — chỗ DUY NHẤT chọn cụm vật lý",
            "    - name: KAFKA_LOG_ADDRESS",
            "      valueFrom: { configMapKeyRef: { name: vigov-infra, key: KAFKA-02-ADDRESS } }",
            "",
            "  Hai cách viết, MỘT tên: key ConfigMap dùng `-`, biến môi trường và Go dùng `_`.",
            "",
            "  Thật sự phải đọc môi trường ngoài core/config?",
            "    // @env-ok: <lý do cụ thể>",
            "",
            "  → Luật 11: .claude/rules/critical/11-infra-config-contract.md",
            "  → Skill:   .claude/skills/infra-config/SKILL.md",
        ],
        tool=tool,
        path=path,
    )


if __name__ == "__main__":
    main()
