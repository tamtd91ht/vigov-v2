"""REST API surface guard — path language and duplicate-request declaration.

WHY TWO EVENTS, ONE HOOK:

  PreToolUse  (BLOCK)  a Vietnamese segment in a path. A path is decided inside ONE string
                       literal — there is no half-written state to protect, so catching it
                       before the write means a wrong path never reaches disk. And a path is
                       the one thing here that cannot be renamed later: it is the contract a
                       commune's integrations call.

  PostToolUse (WARN)   a missing duplicate-request declaration, a verb used as a path
                       segment, a missing /api/v1 prefix. These live across a statement that
                       an author assembles over several edits. Rule 5 already learned this
                       with rbac_guard: blocking a route mid-write teaches the agent to
                       write routes somewhere the hook cannot see.

WHY A WORD LIST AND NOT ONLY DIACRITICS: `POST /dang-nhap` carries no diacritic. Every
Vietnamese path actually written in this project is already transliterated, so a diacritic
check alone would catch none of them. The list matches WHOLE SEGMENTS only — a substring
match would flag `/tasks` for containing `ta`.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "rest_api_guard"

WATCH = ("router.go", "routes.go", "handler.go", "server.go", "api.go", "main.go")

# Go 1.22 ServeMux ("POST /path") and chi/echo-style (.Post("/path")).
MUX = re.compile(
    r"""\.\s*Handle(?:Func)?\s*\(\s*["'`](GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(/[^"'`]*)["'`]""")
CHI = re.compile(
    r"""\.\s*(Get|Post|Put|Patch|Delete|Head|Options)\s*\(\s*["'`](/[^"'`]*)["'`]""")

CHANGES_STATE = {"POST", "PUT", "PATCH", "DELETE"}

# Declared in the SAME statement as the route, exactly like the permission declaration.
IDEM_DECL = re.compile(r"idem\.(?:Required|KhongCan)\s*\(")
IDEM_NO_REASON = re.compile(r"idem\.KhongCan\s*\(\s*\)")
IDEM_NO_MODE = re.compile(r"idem\.Required\s*\(\s*\)")

# Infrastructure paths that are deliberately outside /api/v1 and outside the tenant edge.
INFRA_PATHS = ("/healthz", "/readyz", "/livez", "/metrics", "/debug")

# Whole path segments that are Vietnamese. Transliterated — diacritics are caught separately.
VN_SEGMENTS = {
    # auth / identity
    "dang-nhap", "dang-xuat", "dang-ky", "doi-mat-khau", "quen-mat-khau", "phien",
    "nguoi-dung", "can-bo", "cong-dan", "vai-tro", "quyen", "phan-quyen", "bo-phan",
    "to-chuc", "danh-ba", "thon", "to-dan-pho", "xa", "phuong", "huyen", "tinh",
    # documents
    "van-ban", "van-ban-den", "van-ban-di", "so-den", "so-di", "cong-van",
    "luan-chuyen", "ban-hanh", "thu-hoi", "cap-so", "dinh-kem", "tep",
    # petitions / letters / tasks
    "phan-anh", "don-thu", "khieu-nai", "to-cao", "kien-nghi", "de-nghi",
    "nhiem-vu", "cong-viec", "so-tay", "bien-ban", "bien-ban-hop", "ket-luan",
    "tiep-nhan", "thu-ly", "phan-cong", "phan-loai", "nghiem-thu", "dong-phieu",
    "mo-lai", "gia-han", "lui-han", "phe-duyet", "danh-gia", "hai-long", "linh-vuc",
    # dossiers
    "ho-so", "thu-tuc", "mot-cua", "hoso", "thu-tuc-hanh-chinh",
    # finance
    "giai-ngan", "ngan-sach", "du-toan", "chung-tu", "du-an", "nguon-von",
    "khoan-muc", "thu-chi", "vuong-mac", "quyet-toan",
    # comms
    "thong-bao", "tin-tuc", "noi-dung", "ban-do", "truyen-thanh", "su-kien",
    "bai-viet", "danh-muc",
    # reporting / config / common
    "bao-cao", "thong-ke", "tong-quan", "cau-hinh", "he-thong", "nhat-ky",
    "tra-cuu", "tim-kiem", "xuat-excel", "nhap-excel", "lich-lam-viec",
    "ngay-nghi-le", "tac-vu", "loi-he-thong",
}

# A verb as a segment. Nominalise it: the action must be a record you can GET back.
VERB_SEGMENTS = {
    "create", "update", "delete", "remove", "add", "edit", "save", "set",
    "close", "open", "reopen", "approve", "reject", "assign", "classify",
    "submit", "send", "publish", "withdraw", "revoke", "issue", "confirm",
    "lock", "unlock", "verify", "escalate", "extend", "activate", "deactivate",
    "login", "logout", "register", "export", "import", "sync", "run",
}

NOMINALISED = {
    "close": "closure", "approve": "approval", "assign": "assignment",
    "classify": "classification", "reopen": "reopening", "publish": "publication",
    "withdraw": "withdrawal", "revoke": "revocation", "issue": "issuance",
    "confirm": "confirmation", "verify": "verification", "extend": "extension",
    "escalate": "escalation", "login": "sessions (POST)", "logout": "sessions (DELETE)",
    "export": "exports", "import": "imports", "run": "runs", "sync": "sync-runs",
}


def strip_comments(src: str) -> str:
    """Remove Go comments, keeping newlines so line numbers stay correct.

    MEASURED BEFORE ENABLING (README, "adding a hook", step 3): without this the hook fired on
    all 8 services, every single hit being the commented-out example route in the header of
    `routes.go`. A hook that fires on a comment is a hook somebody turns off, and then the
    whole layer is gone — worse than having no hook.
    """
    out: list[str] = []
    i, n, quote = 0, len(src), ""
    while i < n:
        ch = src[i]
        if quote:
            out.append(ch)
            if ch == "\\" and quote != "`" and i + 1 < n:
                out.append(src[i + 1])
                i += 2
                continue
            if ch == quote:
                quote = ""
            i += 1
            continue
        if ch in "\"'`":
            quote = ch
            out.append(ch)
            i += 1
            continue
        if ch == "/" and i + 1 < n and src[i + 1] == "/":
            while i < n and src[i] != "\n":
                i += 1
            continue
        if ch == "/" and i + 1 < n and src[i + 1] == "*":
            i += 2
            while i + 1 < n and not (src[i] == "*" and src[i + 1] == "/"):
                if src[i] == "\n":
                    out.append("\n")
                i += 1
            i += 2
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def statements(content: str) -> list[tuple[int, str]]:
    """Join multi-line statements: accumulate until parentheses balance.

    Same shape as rbac_guard — a route and its declarations form ONE statement, and anchoring
    to line distance lets one route borrow the declaration written above a different one.
    """
    out, buf, start, depth = [], [], 0, 0
    for i, line in enumerate(content.splitlines(), 1):
        if not buf:
            start = i
        buf.append(line)
        depth += line.count("(") - line.count(")")
        if depth <= 0:
            out.append((start, " ".join(x.strip() for x in buf)))
            buf, depth = [], 0
    if buf:
        out.append((start, " ".join(x.strip() for x in buf)))
    return out


def routes(stmt: str) -> list[tuple[str, str]]:
    found = [(m.group(1).upper(), m.group(2)) for m in MUX.finditer(stmt)]
    found += [(m.group(1).upper(), m.group(2)) for m in CHI.finditer(stmt)]
    return found


def segments(path: str) -> list[str]:
    """Path segments, with {params} and the version prefix dropped."""
    out = []
    for raw in path.split("?")[0].split("/"):
        s = raw.strip().lower()
        if not s or s.startswith("{") or s.startswith(":"):
            continue
        out.append(s)
    return out


def is_infra(path: str) -> bool:
    return any(path.startswith(p) for p in INFRA_PATHS)


def watched(path: str) -> bool:
    base = os.path.basename(path)
    return base in WATCH or "/transport/" in path or "/http/" in path


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    event = data.get("hook_event_name") or data.get("event") or ""
    pre = event != "PostToolUse"

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path.endswith(".go") or c.should_skip(path) or not watched(path):
        sys.exit(0)

    content = c.new_content(ti)
    if not content or c.is_generated(content, path):
        sys.exit(0)

    base = os.path.basename(path)
    vn_hits: list[str] = []
    other: list[str] = []

    for lineno, stmt in statements(strip_comments(content)):
        for method, route in routes(stmt):
            segs = segments(route)

            # ---- language of the path (PreToolUse, BLOCK) ----------------------
            for s in segs:
                if any(ord(ch) > 127 for ch in s):
                    vn_hits.append(f"line {lineno}: {method} {route} — '{s}' has diacritics")
                elif s in VN_SEGMENTS:
                    vn_hits.append(f"line {lineno}: {method} {route} — '{s}' is Vietnamese")

            if pre:
                continue

            # ---- everything below is advisory (PostToolUse) --------------------
            if is_infra(route):
                continue

            if method in CHANGES_STATE and not IDEM_DECL.search(stmt):
                other.append(
                    f"line {lineno}: {method} {route} — NO duplicate-request declaration")
            if IDEM_NO_REASON.search(stmt):
                other.append(f"line {lineno}: idem.KhongCan() states NO reason")
            if IDEM_NO_MODE.search(stmt):
                other.append(f"line {lineno}: idem.Required() picks NO failure mode")

            for s in segs:
                if s in VERB_SEGMENTS:
                    fix = NOMINALISED.get(s)
                    other.append(f"line {lineno}: {method} {route} — '{s}' is a verb"
                                 + (f"; use '{fix}'" if fix else ""))

            if not route.startswith("/api/v"):
                other.append(f"line {lineno}: {method} {route} — not under /api/v1/")

    if vn_hits:
        c.block(HOOK, f"Vietnamese in a URL path — {base}", vn_hits,
                ["  A path is the contract a commune's integrations call. It is the one name",
                 "  here that cannot be renamed once a commune is live, and the one surface",
                 "  read by people who do not read Vietnamese.",
                 "",
                 "    POST /dang-nhap             ->  POST /api/v1/sessions",
                 "    GET  /phan-anh/{ma}         ->  GET  /api/v1/citizen-reports/{code}",
                 "    GET  /van-ban-den           ->  GET  /api/v1/incoming-documents",
                 "    POST /don-thu               ->  POST /api/v1/citizen-letters",
                 "",
                 "  BUT ENUM VALUES STAY VIETNAMESE, without diacritics:",
                 "    GET /api/v1/citizen-letters?type=khieu-nai        correct",
                 "    GET /api/v1/citizen-letters?type=complaint        WRONG",
                 "",
                 "  Translating a path is a naming choice. Translating a VALUE is a business",
                 "  claim: khieu_nai and to_cao run under two different statutes with two",
                 "  different clocks, and English collapses them.",
                 "",
                 "  → .claude/skills/rest-api-design/SKILL.md §1",
                 "  → kb/00-foundation/ubiquitous-language.md:87"],
                tool=tool, path=path)

    if other:
        c.warn(HOOK, f"REST surface — {base}", other,
               ["  Declare duplicate protection in the SAME statement as the route. A double",
                "  POST creates a second petition with a second lookup code already shown to",
                "  the citizen — and rule 7 forbids hard delete, so it is permanent.",
                "",
                '    mux.Handle("POST /api/v1/citizen-reports",',
                '        authz.RequirePermission(d.Checker, "feedback.create")(',
                "            idem.Required(idem.MoKhiHong)(          // Redis down -> let through",
                "                http.HandlerFunc(h.create))))",
                "",
                '    mux.Handle("POST /api/v1/disbursements/{id}/confirmation",',
                '        authz.RequirePermission(d.Checker, "budget.confirm")(',
                "            idem.Required(idem.DongKhiHong)(        // Redis down -> 503",
                "                http.HandlerFunc(h.confirm))))",
                "",
                '    idem.KhongCan("<why this route is naturally idempotent>")',
                "",
                "  MoKhiHong for intake paths — refusing a citizen because a cache is down is",
                "  worse than a rare duplicate. DongKhiHong where the consequence is legal:",
                "  money, document numbers, issuance, closure, privilege changes.",
                "",
                "  Actions are nominalised sub-resources, not verbs: POST .../closure, so that",
                "  'who closed this, when, with what result' is a GET and a second identical",
                "  request returns the existing record instead of acting twice.",
                "",
                "  → .claude/skills/rest-api-design/SKILL.md §3 §4"],
               tool=tool, path=path)

    sys.exit(0)


if __name__ == "__main__":
    main()
