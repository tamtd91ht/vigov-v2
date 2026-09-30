"""Shared helpers for every ViGov hook.

WHY THIS FILE EXISTS: brain v1 loaded hooks with `exec(open(...).read())`, so hooks could
not import each other. The result: `_utf8_streams()` copied 7 times, `read_input()` 7 times,
the PII pattern twice. Fixing one pattern meant remembering every copy, and missing one was
invisible.

v2 invokes hooks directly (`python "${CLAUDE_PROJECT_DIR}/.claude/hooks/<name>.py"`), so
imports work. Every shared
pattern lives here — one place to fix.

Constraints: standard library only. A broken hook must NEVER block work — wrap everything.
"""

from __future__ import annotations

import datetime
import json
import os
import re
import sys

# --------------------------------------------------------------------------
# I/O
# --------------------------------------------------------------------------

def utf8_streams() -> None:
    """Force UTF-8 on all three standard streams. Call this first in every main().

    stdout/stderr: Windows terminals default to cp1252 and cannot PRINT Vietnamese, which
    kills a hook with UnicodeEncodeError.

    stdin: the more dangerous one, and it was missing until 2026-09-17. The payload arrives as
    UTF-8 bytes; cp1252 decodes almost all of them WITHOUT raising, so `json.load` succeeds and
    hands back content where every Vietnamese character has become mojibake. No error, no
    warning — just different text. Every hook rule that matches Vietnamese was therefore
    partially blind on the platform this project is developed on, and blind in the way that
    only ever produces silence: the guard reads a string that never matches and passes.

    Found through doc_guard: a second file claiming an existing `owns_facts` entry was not
    detected, because the fact arriving over stdin no longer equalled the fact read from disk
    with an explicit encoding.
    """
    for stream in (sys.stdin, sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass


def read_input() -> dict:
    try:
        return json.load(sys.stdin)
    except Exception:
        return {}


def tool_of(data: dict) -> str:
    return data.get("tool_name") or data.get("tool") or ""


# Tools that hand a shell command straight to the system.
#
# `Monitor` belongs here for exactly one reason: its `command` runs in the same shell as
# `Bash`, but it arrives under a DIFFERENT TOOL NAME. A PreToolUse matcher of "Bash" never
# sees it, and neither does the `permissions.deny` list -- which is also keyed on the tool
# name, so every deny entry written as `Bash(...)` (recursive deletion, hard reset, force
# push, a direct database client) was reachable through `Monitor` while the settings file
# looked fully locked down.
#
# That is the shape this project keeps finding: something that looks like a control and is
# not one. Measured 2026-09-17.
#
# Every guard that inspects a shell command gates on THIS, never on a bare == "Bash". One
# list, one place: a second tool that runs shell commands gets added here and every guard
# picks it up at once.
VO_SHELL = ("Bash", "Monitor")


def la_vo_shell(tool: str) -> bool:
    """True when this tool hands `tool_input["command"]` to a shell."""
    return tool in VO_SHELL


def input_of(data: dict) -> dict:
    return data.get("tool_input") or data.get("input") or {}


# --------------------------------------------------------------------------
# Đơn vị triển khai — đọc TỪ ĐĨA, không suy từ chuỗi đường dẫn
# --------------------------------------------------------------------------
#
# Trước 2026-09-17 mọi hook nhận ra một dịch vụ bằng cách tìm đoạn `/services/` trong đường
# dẫn. Bố cục nay phẳng — `identity/`, `comms/`, `web-admin/` ngang cấp với `.claude/` — nên
# đoạn ấy không còn tồn tại.
#
# Đây là kiểu thay đổi nguy hiểm nhất với một lớp thực thi: hook vẫn chạy, vẫn thoát 0, và
# không còn khớp gì nữa. Không có gì đỏ. Một quy tắc không bao giờ khớp trông y hệt một quy
# tắc chưa bị ai vi phạm.
#
# Nên câu hỏi "đây có phải mã của một dịch vụ không" nay hỏi ĐĨA: một thư mục cấp một là một
# dịch vụ Go khi nó có `<tên>/cmd/server`. Thêm dịch vụ thứ chín thì mọi hook nhận ra nó ngay,
# không ai phải sửa danh sách.

_DICH_VU_CACHE: tuple[str, ...] | None = None


def dich_vu_tren_dia(root: str | None = None) -> tuple[str, ...]:
    """Tên các dịch vụ Go: thư mục cấp một có `<tên>/cmd/server`."""
    global _DICH_VU_CACHE
    if _DICH_VU_CACHE is not None and root is None:
        return _DICH_VU_CACHE
    goc = root or project_root()
    ra: list[str] = []
    try:
        for ten in sorted(os.listdir(goc)):
            if ten.startswith(".") or not os.path.isdir(os.path.join(goc, ten)):
                continue
            if os.path.isdir(os.path.join(goc, ten, "cmd", "server")):
                ra.append(ten)
    except Exception:
        return _DICH_VU_CACHE or ()
    kq = tuple(ra)
    if root is None:
        _DICH_VU_CACHE = kq
    return kq


# Thư mục cấp một KHÔNG BAO GIỜ là một dịch vụ Go. Khai tường minh vì `dich_vu_cua` dưới đây
# nhận diện theo HÌNH DẠNG đường dẫn, và `core/internal/...` thì không phải vi phạm ranh giới.
KHONG_PHAI_DICH_VU = {
    "core", "tools", "kb", "docs", "proto", "gen", "tasks", "build",
    "web-admin", "platform-admin", "citizen-app", "node_modules", "vendor",
}

# Tiền tố thư mục của một dịch vụ backend. `service-identity/` là TÊN THƯ MỤC; tên nghiệp vụ
# của nó vẫn là `identity` — xem ten_nghiep_vu().
TIEN_TO_DICH_VU = "service-"

# Thư mục con cho biết đoạn đứng trước nó là một dịch vụ.
DAU_HIEU_DICH_VU = ("internal", "cmd", "migrations")


def dich_vu_cua(path: str) -> str | None:
    """Dịch vụ sở hữu đường dẫn này, hoặc None.

    NHẬN DIỆN THEO HÌNH DẠNG, KHÔNG CHỈ THEO ĐĨA, và lý do đáng nêu: một dịch vụ mới đang
    được viết chưa có `cmd/server`, nên nếu chỉ tra danh sách trên đĩa thì đúng lúc mã của nó
    còn non nhất — chưa ai đọc lại, chưa có test — nó lại nằm ngoài tầm mọi guard.

    `<đoạn>/internal/...`, `<đoạn>/cmd/...`, `<đoạn>/migrations/...` là hình dạng của một dịch
    vụ. `core/`, `tools/`, `kb/` bị loại tường minh qua KHONG_PHAI_DICH_VU.
    """
    norm = (path or "").replace("\\", "/").strip("/")
    segs = [x for x in norm.split("/") if x not in ("", ".")]
    for i in range(len(segs) - 1):
        if segs[i + 1] in DAU_HIEU_DICH_VU and segs[i] not in KHONG_PHAI_DICH_VU:
            return segs[i]
    tren_dia = dich_vu_tren_dia()
    for seg in segs:
        if seg in tren_dia:
            return seg
    # Tiền tố `service-` tự nó đã là một lời khai: thư mục này là một dịch vụ backend. Nhận nó
    # kể cả khi dịch vụ chưa có cmd/server và chưa có internal/ — tức từ commit đầu tiên.
    for seg in segs:
        if seg.startswith(TIEN_TO_DICH_VU) and len(seg) > len(TIEN_TO_DICH_VU):
            return seg
    return None


def ten_nghiep_vu(ten_thu_muc: str) -> str:
    """Tên NGHIỆP VỤ của một dịch vụ, bỏ tiền tố thư mục.

    `service-identity` (thư mục, và cũng là đường module) -> `identity` (cái mà hợp đồng REST,
    chỉ mục kb và hàng đợi việc web gọi nó). Tiền tố là dấu hiệu hạ tầng để phân biệt backend
    với web; nó không phải một phần của tên nghiệp vụ, và để nó lọt vào hợp đồng thì mọi bên
    đọc hợp đồng đều phải học cách bỏ nó đi.
    """
    if ten_thu_muc.startswith(TIEN_TO_DICH_VU):
        return ten_thu_muc[len(TIEN_TO_DICH_VU):]
    return ten_thu_muc


def path_of(tool_input: dict) -> str:
    p = tool_input.get("file_path") or tool_input.get("notebook_path") or ""
    return p.replace("\\", "/")


def new_content(tool_input: dict) -> str:
    """Only the text about to be written — never the text being replaced."""
    if "content" in tool_input:
        return tool_input.get("content") or ""
    parts: list[str] = []
    if tool_input.get("new_string") is not None:
        parts.append(tool_input.get("new_string") or "")
    for edit in tool_input.get("edits") or []:
        parts.append(edit.get("new_string") or "")
    return "\n".join(parts)


def noi_dung_sau_sua(tool_input: dict) -> str:
    """The document AS IT WILL BE — disk content with the edits applied.

    WHY THIS EXISTS BESIDE new_content(), AND WHY THE TWO MUST NOT BE MERGED.

    `new_content` answers *"what text is being introduced"* — the right question for
    secret_scan and pii_guard, which must flag what this edit ADDS and must not accuse a file
    of material that was already there.

    This one answers *"what will this file contain"* — the right question for any rule that is
    a property of the FILE. doc_guard's frontmatter rule is exactly that: `tier`, `source`,
    `owner` live at the top of the document and an Edit three lines down never carries them,
    so judging an Edit by its diff reported EVERY edit to kb/ as "missing frontmatter".

    That misfire was not merely noisy. Its only workaround was rewriting the whole file, so a
    guard meant to protect the knowledge layer was pushing every author toward the one
    operation that can destroy it — and on 2026-09-17 it nearly did: a whole-file rewrite
    composed from a stale read was seconds away from erasing six edits another session had just
    made to the same file.

    Missing file -> fall back to the introduced text, which is what creating a file means.
    """
    if "content" in tool_input:
        return tool_input.get("content") or ""

    duong = path_of(tool_input)
    try:
        with open(duong, encoding="utf-8") as f:
            goc = f.read()
    except Exception:
        return new_content(tool_input)

    sua = []
    if tool_input.get("old_string") is not None:
        sua.append((tool_input.get("old_string") or "", tool_input.get("new_string") or "",
                    bool(tool_input.get("replace_all"))))
    for e in tool_input.get("edits") or []:
        sua.append((e.get("old_string") or "", e.get("new_string") or "",
                    bool(e.get("replace_all"))))

    for cu, moi, tat_ca in sua:
        if cu and cu in goc:
            goc = goc.replace(cu, moi) if tat_ca else goc.replace(cu, moi, 1)
    return goc


def project_root(start: str | None = None) -> str:
    d = os.path.abspath(start or os.getcwd())
    for _ in range(6):
        if os.path.isdir(os.path.join(d, ".claude")):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            break
        d = parent
    return os.path.abspath(start or os.getcwd())


# --------------------------------------------------------------------------
# Skip zones
# --------------------------------------------------------------------------

SKIP_FRAGMENTS = (
    "/test/", "/tests/", "/testdata/", "/fixtures/", "/mocks/", "/__mocks__/",
    "/node_modules/", "/vendor/", "/dist/", "/build/", "/.git/", "/.claude/",
    "/kb/90-ephemeral/", "_test.go", ".spec.", ".test.",
    # The hooks' own self-test. It must CONTAIN the dangerous patterns — that is what it
    # tests. Without this exemption a guard blocks the very case that proves it works, and
    # the block case can never be written down.
    "tools/test_hooks.py",
)

# Paths that are genuinely produced by a generator. This is a PATH list on purpose.
#
# WHY NOT A CONTENT MARKER: until 2026-09-16 this was a content check — any file whose
# first 400 chars said "Code generated by" was exempted from EVERY content hook. That is a
# one-line universal opt-out: prepend one comment and tenant_scope_guard, pii_guard,
# secret_scan, rbac_guard, audit_guard, citizen_scope_guard, service_boundary_guard and
# data_safety_guard all stop looking. An agent writing the marker is exactly the case the
# hooks exist for, so the marker cannot be the thing that grants the exemption.
#
# A path cannot be forged by the content being written, so the decision moved here.
GENERATED_PATHS = (
    ".pb.go", "_grpc.pb.go", ".pb.gw.go", ".connect.go", "_pb2.py", ".gen.go",
    "/kb/20-contracts/", "/kb/30-indexes/",
)

# Kept only as a SECONDARY signal on an already-generated path (see is_generated).
GENERATED_MARKS = (
    "// Code generated by", "/* Code generated by", "# Code generated by",
    "DO NOT EDIT", "@generated",
)


def should_skip(path: str) -> bool:
    norm = path.replace("\\", "/").lower()
    return any(f in norm for f in SKIP_FRAGMENTS)


# --------------------------------------------------------------------------
# RANH GIỚI THẨM QUYỀN — rào chắn của kho này chỉ phán xử kho này
# --------------------------------------------------------------------------

# Gốc kho, suy từ vị trí của CHÍNH TỆP NÀY: <gốc>/.claude/hooks/_common.py.
#
# KHÔNG đọc `CLAUDE_PROJECT_DIR`: một biến môi trường thiếu, hoặc trỏ sai một ký tự, sẽ tắt
# lặng lẽ cả lớp cưỡng chế — đúng kiểu hỏng tệ nhất, vì mọi thứ vẫn trông bình thường. Vị trí
# tệp thì không thể sai trong khi hook vẫn chạy được.
GOC_DU_AN = os.path.dirname(os.path.dirname(os.path.dirname(
    os.path.abspath(__file__)))).replace("\\", "/").rstrip("/").lower()

# Đường dẫn tuyệt đối: `/x/y` (POSIX) hoặc `D:/x/y` (Windows). Cả hai đều tới đây ở dạng đã
# thay `\` bằng `/` (path_of làm việc đó).
_TUYET_DOI = re.compile(r"^(?:/|[a-z]:/)", re.IGNORECASE)


def ngoai_du_an(path: str) -> bool:
    """True khi `path` là đường dẫn TUYỆT ĐỐI nằm NGOÀI gốc kho này.

    VÌ SAO CẦN: luật 1 (tenant_id) và luật 11 (core/config là nơi duy nhất đọc môi trường) là
    luật CỦA VIGOV. Một phiên mở ở kho này nhưng ghi tệp sang kho khác — một sản phẩm khác,
    không có xã, không có core/ — vẫn đi qua các hook này, và chúng đòi thứ kho kia cố ý không
    có. Ngày 20/09/2026 điều đó đã chặn thật: `tenant_scope_guard` coi
    `vihat-miniapp/internal/store/kho.go` là một dịch vụ ViGov, và chặn cả
    `r.Header.Get("Origin")` vì `Get` nằm trong DB_CALL.

    HỎNG THÌ ĐÓNG — đường dẫn TƯƠNG ĐỐI trả về False, tức VẪN SOI. Đa số ca test và một số
    công cụ đưa đường dẫn tương đối; coi chúng là "ngoài kho" sẽ tắt rào chắn cho đúng những
    đường dẫn khó kiểm nhất.

    CHỈ HAI HOOK DÙNG HÀM NÀY, và đó là một quyết định chứ không phải chỗ còn thiếu:
    `tenant_scope_guard` và `env_contract_guard` cưỡng chế những luật chỉ có nghĩa BÊN TRONG
    ViGov. `secret_scan` và `pii_guard` thì KHÔNG gọi tới nó và không được gọi: một khoá bí mật
    viết cứng, hay một số điện thoại lọt vào log, sai ở mọi kho — kể cả kho của sản phẩm khác.
    """
    norm = path.replace("\\", "/").lower()
    if not _TUYET_DOI.match(norm):
        return False
    # Gom `..` lại TRƯỚC khi so tiền tố. Không có bước này thì
    # `<gốc>/../kho-khac/internal/x.go` vẫn khớp tiền tố và bị coi là trong kho — hỏng về phía
    # SOI THÊM chứ không phải bỏ sót, nhưng nó biến một đường dẫn hợp lệ thành một lần chặn
    # không ai hiểu vì sao.
    norm = os.path.normpath(norm).replace("\\", "/").rstrip("/")
    return not (norm == GOC_DU_AN or norm.startswith(GOC_DU_AN + "/"))


def is_generated(content: str, path: str = "") -> bool:
    """True only when the FILE PATH is a generated location.

    `content` is accepted for backwards compatibility and is deliberately NOT trusted on
    its own: a marker in the text being written is attacker-controlled (see GENERATED_PATHS).
    With no path given, nothing is exempt.
    """
    norm = (path or "").replace("\\", "/").lower()
    return any(g in norm for g in GENERATED_PATHS)


# --------------------------------------------------------------------------
# Shared patterns — FIX IN ONE PLACE
# --------------------------------------------------------------------------

# Fields carrying citizen personal data or auth secrets (rule 3)
#
# THE VIETNAMESE HALF WAS WRITTEN IN camelCase ONLY, AND THIS LIST IS CASE-SENSITIVE — so it
# matched a spelling this repository does not use. Go exports its struct fields in PascalCase,
# and that is how every personal-data field here is actually named:
# `HoTen`, `DienThoai`, `MatKhau`, `MatKhauHash` (service-identity/internal/domain/can_bo.go:13,19).
# Measured 2026-09-21: `slog.Info("dang nhap", "ten", cb.HoTen)` passed, and so did
# `slog.Debug("kiem tra", "mk", yc.MatKhau)`. Rule 3's only BLOCK hook was blind to the
# repository's own naming convention while looking fully armed.
#
# The second group below is written `[Hh]oTen` rather than adding `(?i)` to the whole pattern:
# a case-insensitive list would make `\bphone\b` match the word "phone" in every English
# comment and `address` match `Address` in every URL helper — trading a miss for the kind of
# noise that gets a hook switched off. The snake_case forms are the JSON tags and SQL columns
# of the same fields, which is what a log call's key argument usually spells.
PII_TOKEN = (
    r"(?:citizenPhone|CitizenPhone|applicantPhone|phoneNumber|soDienThoai|"
    r"\bphone\b|\bPhone\b|cccd|CCCD|cmnd|canCuoc|identityNumber|idNumber|"
    r"\botp\b|\bOTP\b|otpCode|maXacThuc|password|Password|passwordHash|matKhau|"
    r"accessToken|refreshToken|bearer|Bearer|sessionId|fullName|hoTen|diaChi|address|"
    # the spellings this codebase really uses — Go fields, JSON tags, SQL columns
    r"[Hh]oTen|ho_ten|[Mm]atKhau|mat_khau|[Dd]ienThoai|dien_thoai|so_dien_thoai|"
    r"[Dd]iaChi|dia_chi|[Cc]anCuoc|can_cuoc|[Mm]aXacThuc|ma_xac_thuc|[Nn]gaySinh|ngay_sinh)"
)

# A value that has ALREADY been masked is what rule 3 invariant 3 asks for — pii_guard's own
# block message says "log a MASKED value". Without this the widened token list above turns the
# hook into something that punishes the correct line, and a hook that is wrong about the right
# answer is a hook nobody keeps.
#
# Deliberately anchored to the CALL: only a token sitting inside `MaskPhone(`/`MaskCccd(`/
# `MaskName(` (core/privacy/mask.go) is exempt. A variable merely NAMED `masked` is not — the
# claim has to be visible at the call site, the same discipline `// @cross-tenant:` follows.
MASK_CALL = re.compile(r"\bMask[A-Z]\w*\s*\(")

# Logging calls in Go and TS/JS
LOG_CALL = (
    r"(?:log\.(?:Print|Printf|Println|Debug|Info|Warn|Error|Fatal)\w*|"
    r"logger\.(?:Debug|Info|Warn|Error|Fatal)\w*|slog\.(?:Debug|Info|Warn|Error)|"
    r"fmt\.(?:Print|Printf|Println|Sprintf)|"
    r"console\.(?:log|info|warn|error|debug|trace))"
)

# Real Vietnamese mobile numbers (current prefixes)
VN_PHONE = r"[\"'`]0(?:3[2-9]|5[2689]|7[06-9]|8[1-9]|9[0-9])\d{7}[\"'`]"

# 12-digit national ID
VN_CCCD = r"[\"'`]0\d{11}[\"'`]"

# Agreed fake numbers for examples — not real personal data
SAFE_FAKE = re.compile(r"^0(?:9|3|7|8|5)0{6,9}\d?$")


# --------------------------------------------------------------------------
# Blocking and the guard log
# --------------------------------------------------------------------------

def log_guard(hook: str, tool: str, path: str, label: str, count: int = 1) -> None:
    """Append ONE line to .claude/logs/guard.jsonl.

    Records the violation LABEL and never the violating content — the guard log must not
    become a store of exactly what it just blocked.
    """
    try:
        d = os.path.join(project_root(), ".claude", "logs")
        os.makedirs(d, exist_ok=True)
        rec = {
            "at": datetime.datetime.now().isoformat(timespec="seconds"),
            "hook": hook,
            "tool": tool,
            "file": os.path.basename(path) if path else "",
            "label": label,
            "hits": count,
        }
        with open(os.path.join(d, "guard.jsonl"), "a", encoding="utf-8") as f:
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")
    except Exception:
        pass


def block(hook: str, title: str, details: list[str], tail: list[str],
          tool: str = "", path: str = "") -> None:
    """Block: message on stderr + exit(2).

    The message MUST state the correct alternative. Blocking without pointing the way just
    sends the agent looking for another route — that is how enforcement layers die.
    """
    utf8_streams()
    msg = ["", f"[ViGov · BLOCKED] {title}"]
    msg += [f"  - {d}" for d in details[:6]]
    if len(details) > 6:
        msg.append(f"  ... and {len(details) - 6} more")
    msg += [""] + tail
    print("\n".join(msg), file=sys.stderr)
    log_guard(hook, tool, path, title, len(details))
    sys.exit(2)


def warn(hook: str, title: str, details: list[str], tail: list[str],
         tool: str = "", path: str = "") -> None:
    """Advise (PostToolUse): also exit(2), so the message enters the agent's context."""
    utf8_streams()
    msg = ["", f"[ViGov] {title}"]
    msg += [f"  - {d}" for d in details[:8]]
    if len(details) > 8:
        msg.append(f"  ... and {len(details) - 8} more")
    msg += [""] + tail
    print("\n".join(msg), file=sys.stderr)
    log_guard(hook, tool, path, title, len(details))
    sys.exit(2)


# --------------------------------------------------------------------------
# Menu catalogue — the `menu` key of a ledger item, and `/develop-* <menu>`
# --------------------------------------------------------------------------
#
# WHY THE CATALOGUE IS `docs/ui-ux/NN-<slug>.md` AND NOT A LIST WRITTEN HERE: those files ARE the
# menus — each one is the customer's spec of one business area, and a menu without one has no
# agreed behaviour to build against. A list in code would be a second copy that drifts the day a
# spec is added (rule 9). The two files that are not menus are named below, with the reason.
MENU_DIR = "docs/ui-ux"
KHONG_PHAI_MENU = {
    "tong-quan-he-thong",       # 00 — system overview, spans every menu
    "phu-luc-giao-dien-chung",  # 15 — shared UI appendix, spans every menu
}


def khong_dau(s: str) -> str:
    """'Nhiệm vụ' -> 'nhiem-vu'. The form a menu is compared in, typed with or without accents."""
    import unicodedata
    s = (s or "").replace("đ", "d").replace("Đ", "D")
    s = "".join(ch for ch in unicodedata.normalize("NFD", s) if unicodedata.category(ch) != "Mn")
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-")


def cac_menu(root: str | None = None) -> dict[str, str]:
    """slug -> spec path relative to the repo root. Empty when the spec directory is absent."""
    root = root or project_root()
    ra: dict[str, str] = {}
    try:
        for ten in sorted(os.listdir(os.path.join(root, MENU_DIR))):
            m = re.match(r"^\d{2}-([a-z0-9-]+)\.md$", ten)
            if m and m.group(1) not in KHONG_PHAI_MENU:
                ra[m.group(1)] = f"{MENU_DIR}/{ten}"
    except Exception:
        pass
    return ra


def tim_menu(ten: str, cac: dict[str, str]) -> list[str]:
    """Candidate slugs for what a person typed. Exactly one = resolved; zero or several = ASK.

    Exact slug first, then every slug that contains the typed words as whole segments — so
    'Phản ánh' finds `phan-anh-nguoi-dan`, but 'an' does not find every slug with an 'an' in it.
    Never 'closest match': a guessed menu is work done on the wrong business area.
    """
    q = khong_dau(ten)
    if not q:
        return []
    if q in cac:
        return [q]
    return [s for s in cac if f"-{q}-" in f"-{s}-"]
