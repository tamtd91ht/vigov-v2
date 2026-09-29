"""English identifier guard — rule 12, ADR 0051 (BLOCK, PreToolUse).

WHAT IT BLOCKS: a NEW Vietnamese identifier (Go / TS / TSX / SQL / proto) or a NEW source file or
directory with a Vietnamese name. Existing names are never touched: an Edit is judged by the
declarations it ADDS (in new_string, absent from old_string and from the file on disk), a Write by
the declarations the file on disk does not already carry. The decision (2026-09-28) was "new code
in English, no renaming" — a guard that flagged the old names would push exactly the mass rename
the user ruled out.

WHY A SYLLABLE LIST AND A THRESHOLD OF TWO. Code here is transliterated (`TaoBienBan`, no
diacritics), so a diacritic check alone catches nothing. Many Vietnamese syllables are also English
words or common English identifier tokens (`can`, `to`, `do`, `the`, `con`, `mem`), so the list has
two tiers: VN_ONLY (syllables with ~zero hits in an English identifier corpus — Go stdlib and
TypeScript/React/Next type definitions — then read by hand) and VN_WEAK (the colliding ones, which
count only beside a VN_ONLY syllable). A name blocks at two syllables with at least one strong:
a real Vietnamese name almost always carries two (`danhSach`, `bien_ban`, `lyDo`), while one stray
hit is what makes a guard noisy — and a noisy guard is a disabled guard (.claude/README.md,
"adding a hook").
MEASURED BEFORE ENABLING (2026-09-28), over every declaration this guard extracts:
  English corpus — Go stdlib + @types/react, @types/node, next/dist, typescript/lib: 493,914
    declarations, 9 flagged, all proper nouns (Unicode scripts `Tai_Le`/`Tai_Tham`, a font
    `Kay_Pho_Du`, `C_UU12CON`, `removeHopByHopHeaders`). Greek identifiers (`μ`, `ρ`) were 80 of
    the first 81 false positives, hence VN_DIACRITIC instead of "any non-ASCII".
  This repository — 48,913 existing declarations, 16,078 flagged (every sampled hit Vietnamese).
    The unflagged rest is mostly one-token names (`err`, `ctx`, `ten`, `xa`) and English; ~1,000
    distinct names mixing ONE Vietnamese syllable with English (`xaA`, `sinhID`) are missed.

WHAT IT DOES NOT SEE: function parameters, TS object/interface members (they mirror JSON wire
fields owned by existing contracts), string values (enum VALUES stay Vietnamese — ADR 0011),
and a Vietnamese name that uses only ONE syllable from VN_ONLY. Rule 12 still applies to them;
this hook enforces the decidable part.

ESCAPE HATCH: a line comment `vi-name-ok: <reason>` (`//` in Go/TS/proto, `--` in SQL) on the
declaration's line or in the comment block directly above it. The reason is mandatory.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "english_identifier_guard"

# Repo root derived from this file's own location, case preserved (c.GOC_DU_AN is lower-cased,
# which would break os.path.exists on a case-sensitive CI filesystem).
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

EXTS = (".go", ".ts", ".tsx", ".sql", ".proto", ".yaml", ".yml")

# Where new code is written. `platform-admin/src/` is the second Next.js app (vendor console);
# the decision covers all new code, so it is in scope with the same route-directory exemption.
SCOPE = ("service-", "core/", "tools/", "web-admin/src/", "citizen-app/src/",
         "platform-admin/src/", "proto/", "deploy/")

# k8s manifests (user decision 2026-09-29): object names, envFrom configMapRef/secretRef names,
# configMapGenerator names and label keys/values are identifiers an operator types into Rancher —
# `cau-hinh-chung`, `bi-mat-platform`, `cho-phep-duong-ra` drifted from the real cluster
# (`common-config`, `platform-secrets`) exactly because nothing checked them.
YAML_NAME = re.compile(r"(?:^|[{\s,-])name:\s*\"?([A-Za-z0-9][A-Za-z0-9._-]*)\"?\s*(?=[,}\s]|$)", re.M)
YAML_LABEL = re.compile(r"^\s*([a-z0-9.-]+/[A-Za-z0-9._-]+):\s*\"?([A-Za-z0-9._-]*)\"?\s*$", re.M)

# Next.js App Router: a directory under src/app/ IS a URL path segment a user sees, and ADR 0051
# keeps those Vietnamese. The Mini App has no router (citizen-app/src/App.tsx, "WHY NOT A
# ROUTER"), so it has no route directories to exempt.
ROUTE_ROOTS = ("web-admin/src/app/", "platform-admin/src/app/")

GENERATED = (".pb.go", "_grpc.pb.go", ".pb.gw.go", ".gen.go", ".gen.ts", ".d.ts",
             "core/gen/", "/gen/", "schema.gen.ts")

SKIP = ("/node_modules/", "/vendor/", "/dist/", "/build/", "/.next/", "/.git/", "/.claude/")

EXEMPT = re.compile(r"vi-name-ok:\s*\S")

# ---------------------------------------------------------------------------------------------
# STRONG: Vietnamese syllables (no diacritics) with zero or near-zero hits as a token in the
# English corpus. Syllables with many hits were moved to VN_WEAK below (`so` 1,324 hits via
# SO_* socket constants, `doc` 206, `chan` 161, `loc` 150, `cap` 190, `le` 234, `long` 335).
# `kem` (50 hits, crypto KEM) and `tai` (26, Unicode `Tai_*` scripts) stay here because
# `dinh_kem` / `tai_len` are everywhere in this repo and one hit alone never blocks.
# ---------------------------------------------------------------------------------------------
VN_ONLY = frozenset("""
    anh bai bam ban bao bien bieu biet binh boc boi buoc cac cach cai canh cau cay chay che chen
    chep chet chi chieu chien chinh cho choi chon chong chot chu chuan chuc chung chuoi chuyen
    chua coi cong cua cuoc cuoi cung cuu dai dan danh dang dap dau dem den deu dia dich diem dien dieu
    dinh doan doi don dong duoc duoi duong duy duyet dung gach ghep ghi ghim gia giai gian giao
    giay gieo gio gioi giong giu giua goc goi gom gon gui hai hanh hay het hien hieu hinh ho hoa
    hoac hoach hoan hoat hoc hoi hon hong hop huong huy huyen kem kenh kep ket khac khach khai
    kham khan khau khi khien kho khoa khoan khoang khoi khong khop khu khung khuyen kiem kien
    kieu ky lai lam lanh lap lay lech lenh lich lien lieu linh loai loi lua luan luat luc luoc
    luon luong luot luu luy ly ma manh mau menh mien minh moc moi mot muc muoi muon nam nang nao
    nay nen neu ngan ngay nghi nghiem nghiep ngoai ngu nguoc nguoi nguon nguong nguyen ngung nhac
    nhan nhanh nhap nhat nhau nhieu nhiem nho nhom nhu nhung noi nua nuot nut pha pham phai phan
    phang phap phat phep phien phieu pho phoi phong phu phuc phuong phut qua quan quet quy quyen
    quyet rieng rong rut sach sai sanh sau soan som sua suy tach tai tang tao tham tat tay ten tep
    thai thang thanh thao thay them theo thi thich thieu thiep thiet tho thoai thoi thon
    thong thu thua thue thuc thuoc thuong tich tieng tien tiep tiet tieu tinh toan toi tong tra
    tram trang tren trich trinh tro trong truc trung truoc truong tru truy tuan tuc tung tuoi
    tuong tuy tuyen uoc uu vai van vang vao viec vien voi von vong vua vuc vuot xa xac xau xem xep xet xin
    xoa xong xu xuat xung xuong yeu
""".split())

# Vietnamese syllables that ARE English words or English identifier tokens (`do`, `to`, `can`,
# `mem`, `long`). Alone they prove nothing — `canBo` and `toDo` look identical to the tokenizer.
# They count only NEXT TO a VN_ONLY syllable: `lyDo`, `XoaMem`, `ThuTu`, `tieuDe` are Vietnamese
# beyond doubt, and English code almost never pairs one of these with a VN_ONLY token (measured).
VN_WEAK = frozenset("""
    an ba ban bat bay ben bi bo ca cam can cap cha chan co con cot cu da dao dat day de di do doc
    du gan ha ham han hang hat la lan le lo loc lon long ma mac man mat may me mem mi mo nap
    no on quay roi sang
    sinh so ta tan than the tim tin to ton tran tri tu vay ve vi vo vu xe
    bang bu gi hom ke sap su tac that tom
""".split()) - VN_ONLY

# Two weak syllables that are, together, one Vietnamese word common enough to name on their own.
# `canBo` (cán bộ), `hoSo` (hồ sơ), `viTri` (vị trí): neither half is Vietnamese evidence alone.
VN_PAIRS = frozenset({("can", "bo"), ("ho", "so"), ("vi", "tri"), ("mo", "ta"), ("du", "an"),
                      ("co", "so"), ("bi", "mat"), ("cap", "so")})

# Common English identifier tokens that happen to equal a VN syllable above after tokenising.
# (Measured: none needed so far — kept as the one place to add one.)
ENGLISH_ALLOW: frozenset[str] = frozenset()

VN_DIACRITIC = re.compile(r"[À-ÿĂăĐđĨĩŨũ"
                          r"ƠơƯưẠ-ỹ]")

_TOKEN = re.compile(r"[A-Z]+(?=[A-Z][a-z])|[A-Z]?[a-z]+|[A-Z]+|\d+")


def tokens(name: str) -> list[str]:
    """`TaoBienBan` -> [tao, bien, ban]; `bien_ban-moi` -> [bien, ban, moi]; `HTTPServer` ->
    [http, server]."""
    out: list[str] = []
    for part in re.split(r"[_\-.\s]+", name):
        out += [t.lower() for t in _TOKEN.findall(part)]
    return out


def _segment(tok: str) -> int:
    """How many VN_ONLY syllables `tok` splits into EXACTLY (0 = it does not). Catches the
    all-lowercase concatenation `danhsach`, `phananh`, which carries no case boundary."""
    n = len(tok)
    best = [-1] * (n + 1)
    best[0] = 0
    for i in range(n):
        if best[i] < 0:
            continue
        for j in range(i + 3, min(n, i + 7) + 1):
            if tok[i:j] in VN_ONLY and tok[i:j] not in ENGLISH_ALLOW:
                best[j] = max(best[j], best[i] + 1)
    return best[n] if best[n] > 0 else 0


def vn_syllables(name: str) -> list[str]:
    """The Vietnamese syllables in `name` that count toward the threshold."""
    # Latin letters with Vietnamese diacritics only. Greek (`μ`, `ρ` in Go's crypto and math
    # packages) is not Vietnamese — measured: 80 of 81 corpus false positives were Greek.
    if VN_DIACRITIC.search(name):
        return ["<diacritics>", "<diacritics>"]
    strong: list[str] = []
    weak: list[str] = []
    for t in tokens(name):
        if t in ENGLISH_ALLOW:
            continue
        if t in VN_ONLY:
            strong.append(t)
        elif t in VN_WEAK:
            weak.append(t)
        elif len(t) >= 5:
            k = _segment(t)
            if k >= 2:
                strong += [t] * k
    toks = tokens(name)
    pairs = [f"{a} {b}" for a, b in zip(toks, toks[1:]) if (a, b) in VN_PAIRS]
    if pairs and not strong:
        return pairs * 2
    return strong + weak if strong else []


def is_vietnamese(name: str) -> bool:
    return len(vn_syllables(name)) >= 2


# ---------------------------------------------------------------------------------------------
# Comment / string stripping — keeps newlines so line numbers survive
# ---------------------------------------------------------------------------------------------

def strip_code(src: str, lang: str) -> str:
    """Blank out comments and string literals, preserving line structure.

    Strings hold UI text and SQL — Vietnamese on purpose. A declaration regex that read them would
    flag `"Tạo biên bản"` in a label, the one place ADR 0051 keeps Vietnamese."""
    out: list[str] = []
    i, n = 0, len(src)
    line_c = "--" if lang == "sql" else "//"
    quotes = "'" if lang == "sql" else "\"'`"
    while i < n:
        ch = src[i]
        if src.startswith(line_c, i):
            while i < n and src[i] != "\n":
                i += 1
            continue
        if src.startswith("/*", i):
            j = src.find("*/", i + 2)
            j = n if j < 0 else j + 2
            out.append("\n" * src.count("\n", i, j))
            i = j
            continue
        if ch in quotes:
            q = ch
            j = i + 1
            while j < n and src[j] != q:
                if src[j] == "\\" and q != "`" and lang != "sql":
                    j += 1
                elif src[j] == "\n" and q != "`" and lang != "sql":
                    break
                j += 1
            seg = src[i:j + 1]
            out.append(q + "\n" * seg.count("\n") + q)
            i = j + 1
            continue
        out.append(ch)
        i += 1
    return "".join(out)


# ---------------------------------------------------------------------------------------------
# Declarations
# ---------------------------------------------------------------------------------------------

ID = r"[A-Za-z_À-ỹ][\wÀ-ỹ]*"

GO_FUNC = re.compile(rf"^\s*func\s+(?:\([^)]*\)\s*)?({ID})", re.M)
GO_SINGLE = re.compile(rf"^\s*(type|const|var)\s+({ID})", re.M)
GO_BLOCK_OPEN = re.compile(r"^\s*(type|const|var)\s*\(\s*$")
GO_SHORT = re.compile(rf"(?:^|[\s;{{(])((?:{ID}\s*,\s*)*{ID})\s*:=")
GO_FIELD = re.compile(rf"^\s*((?:{ID}\s*,\s*)*{ID})\s+[\*\[\]\w.{{(]")
GO_METHOD = re.compile(rf"^\s*({ID})\s*\(")        # interface method
GO_BODY = re.compile(r"\b(struct|interface)\s*\{")
GO_KEYWORDS = {"return", "if", "for", "switch", "case", "go", "defer", "else", "func",
               "select", "range", "map", "chan", "struct", "interface", "package", "import",
               "default", "break", "continue", "goto", "fallthrough", "type", "const", "var"}

TS_DECL = re.compile(
    rf"(?:^|[^\w.$])(?:function\*?|class|interface|enum|type|const|let|var|namespace)\s+({ID})",
    re.M)

SQL_TABLE = re.compile(
    rf"\bCREATE\s+(?:UNLOGGED\s+|TEMP(?:ORARY)?\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?"
    rf"(?:{ID}\.)?({ID})\s*\(", re.I)
SQL_OTHER = re.compile(
    rf"\bCREATE\s+(?:OR\s+REPLACE\s+)?(?:MATERIALIZED\s+)?(?:TYPE|VIEW|DOMAIN)\s+"
    rf"(?:IF\s+NOT\s+EXISTS\s+)?(?:{ID}\.)?({ID})", re.I)
SQL_ADD_COL = re.compile(rf"\bADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?({ID})", re.I)
SQL_RENAME = re.compile(rf"\bRENAME\s+(?:COLUMN\s+{ID}\s+)?TO\s+({ID})", re.I)
SQL_NOT_COLUMN = {"constraint", "primary", "unique", "foreign", "check", "exclude", "like"}

PROTO_DECL = re.compile(rf"^\s*(?:message|service|enum|rpc|oneof)\s+({ID})", re.M)
PROTO_FIELD = re.compile(
    rf"^\s*(?:repeated\s+|optional\s+)?[\w.<>, ]+\s+({ID})\s*=\s*\d+", re.M)


def _line_of(src: str, pos: int) -> int:
    return src.count("\n", 0, pos) + 1


def _go_decls(s: str) -> list[tuple[str, int]]:
    out: list[tuple[str, int]] = []
    for m in GO_FUNC.finditer(s):
        out.append((m.group(1), _line_of(s, m.start(1))))
    for m in GO_SINGLE.finditer(s):
        out.append((m.group(2), _line_of(s, m.start(2))))
    for m in GO_SHORT.finditer(s):
        ln = _line_of(s, m.start(1))
        for name in re.split(r"\s*,\s*", m.group(1)):
            if name != "_" and name not in GO_KEYWORDS:
                out.append((name, ln))

    lines = s.split("\n")
    block = ""       # "const" / "var" / "type" while inside a ( ... ) group
    depth = 0        # brace depth inside a struct / interface body
    for ln, raw in enumerate(lines, 1):
        line = raw.strip()
        if depth:
            # members of a struct / interface body; an embedded field (`sync.Mutex`, `*Base`)
            # is one word and names nothing new
            if line and not line.startswith("}"):
                m = GO_FIELD.match(raw) if len(line.split()) >= 2 else None
                m = m or GO_METHOD.match(raw)
                if m:
                    for name in re.split(r"\s*,\s*", m.group(1)):
                        if name not in GO_KEYWORDS:
                            out.append((name, ln))
            depth = max(0, depth + raw.count("{") - raw.count("}"))
            continue
        opens = raw.count("{") - raw.count("}")
        if GO_BODY.search(raw) and opens > 0 and (line.startswith("type ") or block == "type"):
            if block == "type":
                m = re.match(rf"^\s*({ID})", raw)
                if m:
                    out.append((m.group(1), ln))
            depth = opens
            continue
        if block:
            if line.startswith(")"):
                block = ""
                continue
            m = re.match(rf"^\s*((?:{ID}\s*,\s*)*{ID})\b", raw)
            if m:
                for name in re.split(r"\s*,\s*", m.group(1)):
                    if name not in GO_KEYWORDS:
                        out.append((name, ln))
            continue
        bm = GO_BLOCK_OPEN.match(raw)
        if bm:
            block = bm.group(1)
    return out


def _ts_decls(s: str) -> list[tuple[str, int]]:
    return [(m.group(1), _line_of(s, m.start(1))) for m in TS_DECL.finditer(s)]


def _sql_decls(s: str) -> list[tuple[str, int]]:
    out: list[tuple[str, int]] = []
    for rx in (SQL_OTHER, SQL_ADD_COL, SQL_RENAME):
        for m in rx.finditer(s):
            out.append((m.group(1), _line_of(s, m.start(1))))
    for m in SQL_TABLE.finditer(s):
        out.append((m.group(1), _line_of(s, m.start(1))))
        # the column list: from the opening paren to its matching close
        i, depth = m.end(), 1
        start = i
        while i < len(s) and depth:
            if s[i] == "(":
                depth += 1
            elif s[i] == ")":
                depth -= 1
            i += 1
        body = s[start:i - 1]
        # split on top-level commas
        parts, buf, d = [], [], 0
        off = start
        pos = start
        for ch in body:
            if ch == "(":
                d += 1
            elif ch == ")":
                d -= 1
            if ch == "," and d == 0:
                parts.append(("".join(buf), pos))
                buf = []
                pos = off + 1
            else:
                buf.append(ch)
            off += 1
        parts.append(("".join(buf), pos))
        for text, p in parts:
            cm = re.match(rf"\s*\"?({ID})\"?\s+\w", text)
            if cm and cm.group(1).lower() not in SQL_NOT_COLUMN:
                lead = len(text) - len(text.lstrip())
                out.append((cm.group(1), _line_of(s, p + lead)))
    return out


def _proto_decls(s: str) -> list[tuple[str, int]]:
    out = [(m.group(1), _line_of(s, m.start(1))) for m in PROTO_DECL.finditer(s)]
    out += [(m.group(1), _line_of(s, m.start(1))) for m in PROTO_FIELD.finditer(s)]
    return out


def _yaml_decls(s: str) -> list[tuple[str, int]]:
    out = [(m.group(1), _line_of(s, m.start(1))) for m in YAML_NAME.finditer(s)]
    for m in YAML_LABEL.finditer(s):
        out.append((m.group(1).split("/", 1)[1], _line_of(s, m.start(1))))
        if m.group(2):
            out.append((m.group(2), _line_of(s, m.start(2))))
    return out


def lang_of(path: str) -> str:
    p = path.lower()
    if p.endswith((".yaml", ".yml")):
        return "yaml"
    if p.endswith(".go"):
        return "go"
    if p.endswith(".sql"):
        return "sql"
    if p.endswith(".proto"):
        return "proto"
    return "ts"


def declarations(src: str, lang: str) -> list[tuple[str, int]]:
    """(name, 1-based line) for every declaration the language form defines."""
    if not src:
        return []
    if lang == "yaml":
        # Only `#` comments; YAML strings are values, and names are unquoted or double-quoted.
        s = re.sub(r"(^|\s)#[^\n]*", r"\1", src)
    else:
        s = strip_code(src, "sql" if lang == "sql" else "c")
    fn = {"go": _go_decls, "ts": _ts_decls, "sql": _sql_decls, "proto": _proto_decls,
          "yaml": _yaml_decls}[lang]
    try:
        return fn(s)
    except Exception:
        return []


def exempted(src: str, line: int, lang: str) -> bool:
    """`vi-name-ok: <reason>` on the line itself or in the comment block directly above."""
    lines = src.split("\n")
    if 1 <= line <= len(lines) and EXEMPT.search(lines[line - 1]):
        return True
    mark = {"sql": "--", "yaml": "#"}.get(lang, "//")
    i = line - 2
    while i >= 0:
        s = lines[i].strip()
        if not s.startswith(mark):
            break
        if EXEMPT.search(s):
            return True
        i -= 1
    return False


def new_vietnamese(before: str, after: str, lang: str, disk: str = "") -> list[tuple[str, int]]:
    """Vietnamese declarations present in `after` and in neither `before` nor `disk`."""
    known = {n for n, _ in declarations(before, lang)} | {n for n, _ in declarations(disk, lang)}
    hits, seen = [], set()
    for name, line in declarations(after, lang):
        if name in known or name in seen:
            continue
        seen.add(name)
        if is_vietnamese(name) and not exempted(after, line, lang):
            hits.append((name, line))
    return hits


# ---------------------------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------------------------

def rel_path(path: str) -> str:
    p = path.replace("\\", "/")
    root = ROOT.replace("\\", "/").rstrip("/")
    if p.lower().startswith(root.lower() + "/"):
        p = p[len(root) + 1:]
    return p[2:] if p.startswith("./") else p


def in_scope(rel: str) -> bool:
    low = "/" + rel.lower()
    if not rel.lower().endswith(EXTS):
        return False
    if any(s in low for s in SKIP):
        return False
    if any(g in low for g in GENERATED):
        return False
    return any(rel.startswith(s) for s in SCOPE)


def vietnamese_new_segments(rel: str, exists=os.path.exists) -> list[str]:
    """Path segments that do not exist yet and carry a Vietnamese name.

    Only NEW segments: `features/dang-nhap/` exists, so a new English file inside it is fine, and
    blocking the old directory name would be the rename the decision ruled out. Route directories
    under src/app/ are exempt (ADR 0051 keeps user-facing paths Vietnamese). A test file named
    after an existing source file (`danh_muc_test.go` beside `danh_muc.go`) follows its subject.
    """
    segs = rel.split("/")
    hits: list[str] = []
    for i, seg in enumerate(segs):
        sub = "/".join(segs[:i + 1])
        if exists(os.path.join(ROOT, sub)):
            continue
        if any((sub + "/").startswith(r) and len(sub) > len(r.rstrip("/")) for r in ROUTE_ROOTS):
            if i < len(segs) - 1:     # a directory under app/ is a URL segment
                continue
        stem = seg
        if i == len(segs) - 1:
            stem = re.sub(r"(\.test|\.spec)?\.(tsx?|go|sql|proto|ya?ml)$", "", seg)
            stem = re.sub(r"_test$", "", stem)
            subject = [seg.replace("_test.go", ".go"),
                       re.sub(r"\.(test|spec)\.(tsx?)$", r".\2", seg)]
            parent = "/".join(segs[:i])
            if any(s != seg and exists(os.path.join(ROOT, parent, s)) for s in subject):
                continue
            stem = re.sub(r"^\d+[_-]", "", stem)       # migration number prefix
        if is_vietnamese(stem):
            hits.append(sub)
    return hits


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)
    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path or c.ngoai_du_an(path):
        sys.exit(0)
    rel = rel_path(path)
    if not in_scope(rel):
        sys.exit(0)
    lang = lang_of(rel)

    abs_path = path if os.path.isabs(path) else os.path.join(ROOT, rel)
    try:
        with open(abs_path, encoding="utf-8") as f:
            disk = f.read()
        file_exists = True
    except Exception:
        disk, file_exists = "", False

    if tool == "Write":
        before, after = "", ti.get("content") or ""
    else:
        pairs = []
        if ti.get("old_string") is not None or ti.get("new_string") is not None:
            pairs.append((ti.get("old_string") or "", ti.get("new_string") or ""))
        for e in ti.get("edits") or []:
            pairs.append((e.get("old_string") or "", e.get("new_string") or ""))
        before = "\n".join(p[0] for p in pairs)
        after = "\n".join(p[1] for p in pairs)

    names = new_vietnamese(before, after, lang, disk)
    # Only a Write creates a path; an Edit targets a file that already exists by definition.
    segs = vietnamese_new_segments(rel) if tool == "Write" and not file_exists else []

    if not names and not segs:
        sys.exit(0)

    details = [f"line {ln}: `{n}` — Vietnamese syllables: {', '.join(vn_syllables(n))}"
               for n, ln in names]
    details += [f"new path segment `{s}` is Vietnamese" for s in segs]
    c.block(HOOK, f"New Vietnamese identifier — {os.path.basename(rel)}", details,
            ["  Rule 12 (user decisions 2026-09-28/29): EVERY name is English — functions,",
             "  types, variables, constants, struct fields, exports, files, directories,",
             "  tables, columns, enum values, buckets, k8s objects/labels (deploy/**).",
             "  The ONLY Vietnamese names left are API URL path segments. Existing Vietnamese",
             "  names are being renamed service by service (rename campaign ADR) — never add one.",
             "",
             "    func TaoBienBan()        ->  func CreateMeeting()",
             "    const layDanhSach = ...  ->  const listTasks = ...",
             "    CREATE TABLE bien_ban_x  ->  CREATE TABLE meeting_x",
             "    features/bien-ban-moi/   ->  features/meeting-drafts/",
             "",
             "  Take the English word from kb/00-foundation/ubiquitous-language.md (the",
             "  `@entity` and URL-resource columns) — never invent a second English name",
             "  for a concept that already has one. A business concept with no entry",
             "  there -> ask the user (legal terms khieu_nai/to_cao/phan_anh: ADR 0051 open #1).",
             "",
             "    configMapRef: cau-hinh-chung  ->  common-config",
             "    secretRef: bi-mat-platform    ->  platform-secrets",
             "",
             "  Vietnamese STAYS only in: API URL paths, route directories under",
             "  web-admin/src/app/** (they are URL paths), UI strings, kb/ prose.",
             "",
             "  Genuinely needed (mirrors an existing wire field, a legal term with no",
             "  English equivalent)? Mark it:  // vi-name-ok: <reason>   (SQL: -- vi-name-ok:)",
             "",
             "  → .claude/skills/naming-english/SKILL.md",
             "  → .claude/rules/critical/12-english-identifiers.md"],
            tool=tool, path=path)


if __name__ == "__main__":
    main()
