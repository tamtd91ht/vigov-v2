#!/usr/bin/env python3
"""Pure logic behind the operations job `deploy/van-hanh/Jenkinsfile` (ADR 0089, rebuilt 11/10/2026).

The job has ONE task: point the cluster at a different piece of infrastructure — PostgreSQL, MinIO,
Redis, ClamAV. The operator picks the component and types only that component's values; this file
decides which ConfigMap key and which `<service>-secrets` keys those values become, which services
must restart, and what to probe from inside the cluster before anything is written. Written in
Groovy those decisions would live in a Jenkinsfile no test runs. Here they are plain functions with
unit tests (`deploy/van-hanh/test_ops.py`, run by `make buildfiles`), and the Groovy only moves files.

It also keeps the basemap commands `vigov-deploy` calls through `deploy/van-hanh/basemap-task.sh`.

SECRET VALUES: the commands that touch them (`infra-plan`, `mc-host`) read and write FILES only
(mode 0600) and never print a value — not even a prefix. Every printed line names keys, hosts,
database and account names.

Python 3.8+: the build machine is CentOS 7, `python3` there is 3.6 and the job picks the newest
`python3.X` it finds (same loop as the root Jenkinsfile).
"""

from __future__ import annotations

import argparse
import base64
import datetime
import glob
import io
import json
import os
import re
import sys
from urllib.parse import quote

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
JENKINSFILE = os.path.join(ROOT, "deploy", "van-hanh", "Jenkinsfile")

# ─────────────────────────────────────────────────────────────────────────────────────────────
# Key catalog — DERIVED, never hand-listed (rule 9; ADR 0089 "Ràng buộc mang theo", rule 11 row).
#
#   placement : deploy/cau-hinh/README.md section 1 (secret) · 2 (configmap) · 3 (env) · 4 (unset).
#               That table is itself checked against the code by tools/check_env_map.py.
#   level/users: core/config `r.read("KEY", …, <level>, <groups>)` × `config.Uses(...)` of every
#               service-*/cmd/server/main.go — the same derivation tools/check_env_map.py makes.
#   registered: the key, spelled exactly, has a line in .env.example (rule 11 invariant 4, ADR 0093:
#               key = env name — a dashed key can never be registered).
# ─────────────────────────────────────────────────────────────────────────────────────────────

READ_CALL = re.compile(
    r"\br\.read\(\s*\"([A-Z0-9_]+)\"\s*,.+?,\s*(requiredEverywhere|requiredInProd|optional)\s*((?:,\s*\w+\s*)*)\)",
    re.S,
)
USES = re.compile(r"config\.Uses\(([^)]*)\)", re.S)
GROUP_NAME = re.compile(r"config\.(\w+)")
ENV_LINE = re.compile(r"^([A-Z][A-Z0-9_]*)=")
SECTION = re.compile(r"^##\s+(\d)\.")
# Accepts a dash ON PURPOSE: a dashed README row must be catalogued — and then fail `registered`
# (no `.env.example` line can carry a dash) — rather than be skipped without a word. Rule 11
# invariant 4, ADR 0093: `envFrom` drops a dashed key silently, so a dashed key is a defect.
ROW_KEY = re.compile(r"^`([A-Z][A-Z0-9_-]*)`$")

PLACEMENT_BY_SECTION = {"1": "secret", "2": "configmap", "3": "env", "4": "unset"}
LEVEL = {"requiredEverywhere": "all", "requiredInProd": "prod", "optional": "opt"}
WEB_ADMIN = "web-admin"


def _read(path: str) -> str:
    with io.open(path, encoding="utf-8") as f:
        return f.read()


def _clean_cell(cell: str) -> str:
    s = re.sub(r"\[([^\]]+)\]\([^)]*\)", r"\1", cell)
    s = s.replace("`", "").replace("**", "").replace("\\", "/").replace("'", "’")
    s = re.sub(r"\s+", " ", s).strip()
    if len(s) > 180:
        s = s[:177].rsplit(" ", 1)[0] + "…"
    return s


def service_groups(root: str = ROOT) -> dict:
    """{service short name: set of declared groups} from config.Uses in every main."""
    out = {}
    for f in sorted(glob.glob(os.path.join(root, "service-*", "cmd", "server", "main.go"))):
        name = os.path.basename(os.path.dirname(os.path.dirname(os.path.dirname(f))))[len("service-"):]
        m = USES.search(_read(f))
        out[name] = set(GROUP_NAME.findall(m.group(1))) if m else set()
    return out


def config_reads(root: str = ROOT) -> dict:
    """{KEY: (level, set of groups)} from the r.read calls of core/config."""
    src = "\n".join(
        _read(f) for f in sorted(glob.glob(os.path.join(root, "core", "config", "*.go")))
        if not f.endswith("_test.go")
    )
    return {k: (LEVEL[lv], set(re.findall(r"\w+", g))) for k, lv, g in READ_CALL.findall(src)}


def readme_rows(text: str) -> list:
    """[(key, placement, row text, meaning, web_admin_only)] from deploy/cau-hinh/README.md."""
    rows, placement, header, web_only = [], None, None, False
    for line in text.splitlines():
        m = SECTION.match(line)
        if m:
            placement, header, web_only = PLACEMENT_BY_SECTION.get(m.group(1)), None, False
            continue
        if line.startswith("### "):
            web_only = WEB_ADMIN in line
            header = None
            continue
        if placement is None or not line.startswith("|"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if cells and cells[0] == "Key":
            header = cells
            continue
        km = ROW_KEY.match(cells[0]) if cells else None
        if not km or header is None:
            continue
        idx = next((i for i, h in enumerate(header) if h.startswith("Là gì") or h.startswith("Vì sao")), None)
        meaning = _clean_cell(cells[idx]) if idx is not None and idx < len(cells) else ""
        rows.append((km.group(1), placement, line, meaning, web_only))
    return rows


def build_catalog(root: str = ROOT) -> list:
    reads = config_reads(root)
    groups = service_groups(root)
    all_services = sorted(groups)
    env_keys = {m.group(1) for ln in _read(os.path.join(root, ".env.example")).splitlines()
                if (m := ENV_LINE.match(ln))}
    out = []
    for key, placement, row, meaning, web_only in readme_rows(
            _read(os.path.join(root, "deploy", "cau-hinh", "README.md"))):
        level, users = "opt", []
        if key in reads:
            lv, gs = reads[key]
            users = all_services if not gs else sorted(s for s, d in groups.items() if d & gs)
            level = lv
        if web_only:
            # The web-admin section ADDS a reader; it does not replace the Go ones. BASEMAP_URL has both
            # (comms since ADR 0072 §Sửa đổi 11/10/2026 (Mini App) M1) — dropping comms would leave its pod
            # out of the restart list after a change, so the Mini App map keeps the old prefix.
            users = users + [WEB_ADMIN]
        out.append({
            "key": key,
            "placement": placement,
            "level": level if users else "unused",
            "users": users,
            "shared": "cùng một giá trị" in row,
            "registered": key in env_keys,  # exact: key = env name, no `-`→`_` mapping (ADR 0093)
            "meaning": meaning,
        })
    return out


def catalog_text(cat: list) -> str:
    """One key per line, TAB-separated — embedded in the Jenkinsfile for the form scripts, which run
    on the controller and have no checkout. No backslash, no quote: it sits inside a ''' literal."""
    lines = []
    for e in cat:
        lines.append("\t".join([
            e["key"], e["placement"], e["level"], ",".join(e["users"]) or "-",
            "1" if e["shared"] else "0", "1" if e["registered"] else "0", e["meaning"],
        ]))
    return "\n".join(lines)


REGION = re.compile(r"(// BEGIN GENERATED ops-data[^\n]*\n)(.*?)(// END GENERATED ops-data)", re.S)


def generated_region(cat: list) -> str:
    """The Jenkinsfile block the FORM scripts and checkParameters() read (the form runs on the
    controller, with no checkout). Each method is a ''' literal holding plain lines: no backslash,
    no quote, no `'''`."""

    def method(name: str, body: str) -> str:
        return f"String {name}() {{\n  return '''\n{body}\n'''\n}}\n"

    return method("opsCatalog", catalog_text(cat)) + method("opsInfra", infra_text(cat))


def render_jenkinsfile(text: str, region: str) -> str:
    if not REGION.search(text):
        raise ValueError("không thấy vùng `// BEGIN GENERATED ops-data` trong Jenkinsfile")
    return REGION.sub(lambda m: m.group(1) + region + m.group(3), text, count=1)


def by_key(cat: list) -> dict:
    return {e["key"]: e for e in cat}


def deployment_of(user: str) -> str:
    return "vigov-web-admin" if user == WEB_ADMIN else "vigov-service-" + user


# ─────────────────────────────────────────────────────────────────────────────────────────────
# Shared parsing and value checks
# ─────────────────────────────────────────────────────────────────────────────────────────────

class PlanError(Exception):
    """A refusal the operator must read. Message is Vietnamese; it never carries a secret."""


def table(headers: list, rows: list) -> str:
    cols = [list(map(str, c)) for c in zip(headers, *rows)] if rows else [[h] for h in headers]
    w = [max(len(x) for x in c) for c in cols]
    line = lambda r: "  ".join(str(x).ljust(w[i]) for i, x in enumerate(r)).rstrip()
    sep = "  ".join("─" * x for x in w)
    return "\n".join([line(headers), sep] + [line(r) for r in rows])


HOSTPORT = re.compile(r"^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?:\d{1,5}$")


def check_config_value(key: str, value: str) -> None:
    if len(value) > 4096 or any(ord(ch) < 32 for ch in value):
        raise PlanError(f"{key}: giá trị quá dài hoặc có ký tự điều khiển")
    if key.endswith(("_ADDRESS", "_ADDR")):
        # Rule 11 invariant 5: an address is a LIST from its first line, even with one node.
        for item in value.split(","):
            if not HOSTPORT.match(item.strip()):
                raise PlanError(f"{key}: '{item.strip()}' không đúng dạng host:port "
                                f"(danh sách cách nhau bởi dấu phẩy, không tcp://)")
        return
    if key.endswith(("_ENDPOINT", "_URL")):
        # Rule 13 invariant 1: traffic to data stores is encrypted. http:// is refused, not warned.
        if not re.fullmatch(r"https://[^\s@?#]+", value):
            raise PlanError(f"{key}: phải là https://…, không thông tin đăng nhập, không truy vấn")


def parse_hosts(hosts: str, port: str) -> str:
    """`h1,h2:5433` → `h1:5432,h2:5433` — a list from the first line (rule 11 invariant 5)."""
    out = []
    for h in (hosts or "").split(","):
        h = h.strip()
        if not h:
            continue
        if ":" not in h:
            h = f"{h}:{port}"
        if not HOSTPORT.match(h):
            raise PlanError(f"máy chủ '{h[:60]}' không đúng dạng host hoặc host:port")
        out.append(h)
    if not out:
        raise PlanError("chưa nhập máy chủ")
    return ",".join(out)


def secret_manifest(name: str, data: dict, labels: dict, notes: dict = None) -> dict:
    """A backup Secret. It is attached to NO Deployment (ADR 0089 open question #4): a key added to
    an envFrom'ed Secret would become an env var with no .env.example line (rule 11 forbidden #4)."""
    meta = {"name": name, "labels": labels}
    if notes:
        meta["annotations"] = notes
    return {"apiVersion": "v1", "kind": "Secret", "type": "Opaque", "metadata": meta, "data": data}


def ordered_services(services: list) -> list:
    """platform → identity → the rest: the order deploy/README.md mục 2 gives for bringing services up
    (every service resolves its commune through platform, and its staff through identity)."""
    head = [s for s in ("platform", "identity") if s in services]
    return head + [s for s in sorted(services) if s not in head]


# ─────────────────────────────────────────────────────────────────────────────────────────────
# One-shot pods — `kubectl create -f` + poll, never `kubectl run -i` (it hung, deploy/Jenkinsfile:35)
# ─────────────────────────────────────────────────────────────────────────────────────────────

PSQL_IMAGE = "postgres:16-alpine"   # the cluster's PostgreSQL is 16; has psql, pg_isready, busybox nc


def _secret_env(spec: str) -> dict:
    """`VAR=secret/key` → env entry reading a Secret key. `optional` so one missing Secret is
    reported by the script instead of failing the whole pod."""
    var, _, ref = spec.partition("=")
    sec, _, key = ref.partition("/")
    if not (re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", var) and re.fullmatch(r"[a-z0-9-]{1,253}", sec)
            and re.fullmatch(r"[A-Za-z0-9_.-]{1,253}", key)):
        raise PlanError(f"--secret-env '{spec[:60]}' không đúng dạng VAR=secret/key")
    return {"name": var, "valueFrom": {"secretKeyRef": {"name": sec, "key": key, "optional": True}}}


def build_pod(a) -> dict:
    """Pod manifest: postgres image, the script travels base64 in an env var (no stdin, no attach)."""
    if not a.script:
        raise PlanError("pod: cần --script")
    labels = {"app.kubernetes.io/name": "vigov-ops-oneshot", "app.kubernetes.io/managed-by": "vigov-van-hanh"}
    script = base64.b64encode(_read(a.script).encode("utf-8")).decode("ascii")
    env = [{"name": "SCRIPT_B64", "value": script}] + [_secret_env(s) for s in a.secret_env]
    env_from = [{"secretRef": {"name": s}} for s in a.envfrom_secret]
    container = {"name": "oneshot", "image": PSQL_IMAGE, "env": env, "envFrom": env_from,
                 "command": ["sh", "-c", "echo \"$SCRIPT_B64\" | base64 -d > /tmp/s.sh && sh /tmp/s.sh"],
                 "securityContext": {"allowPrivilegeEscalation": False, "capabilities": {"drop": ["ALL"]}},
                 "resources": {"requests": {"cpu": "20m", "memory": "64Mi"}, "limits": {"memory": "256Mi"}}}
    spec = {"restartPolicy": "Never", "automountServiceAccountToken": False,
            "securityContext": {"runAsNonRoot": True, "runAsUser": 70,    # `postgres` in the alpine image
                                "seccompProfile": {"type": "RuntimeDefault"}},
            "imagePullSecrets": [{"name": p} for p in a.pull_secret], "containers": [container]}
    # NO Deployment label: carrying one would make the Service send real traffic to this pod.
    return {"apiVersion": "v1", "kind": "Pod", "metadata": {"name": a.name, "labels": labels}, "spec": spec}


# ─────────────────────────────────────────────────────────────────────────────────────────────
# Infrastructure — the job's one task (owner 11/10/2026: rebuilt around "đổi hạ tầng").
#
# A component is the set of keys an operator thinks of as ONE thing ("our MinIO"), spread over
# common-config and several <service>-secrets. WHICH services read each key comes from the catalog
# (derived from core/config); WHICH keys make a component is the only hand-written fact here.
#
# Form → job contract: the form's textarea sends `FIELD=value` lines, a per-service field written
# `FIELD.<service>=value`. An EMPTY value means "keep what the cluster has" — the operator types only
# what changes. Secret values never travel in that text: a secret field names a password slot
# (`1`..`7` = Jenkins password parameters BI_MAT_1..7) or `giu`.
# ─────────────────────────────────────────────────────────────────────────────────────────────

INFRA = [
    {"id": "postgresql", "label": "PostgreSQL — CSDL của từng dịch vụ", "keys": ["DATABASE_DSN"]},
    {"id": "minio", "label": "MinIO — lưu tệp đính kèm, ảnh, logo", "keys": [
        "OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_BUCKET_PREFIX",
        "OBJECT_STORAGE_REGION", "OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL",
        "OBJECT_STORAGE_ACCESS_KEY", "OBJECT_STORAGE_SECRET_KEY"]},
    {"id": "redis", "label": "Redis — chống gửi trùng, giới hạn tần suất", "keys": ["REDIS_DSN"]},
    {"id": "clamav", "label": "ClamAV — quét mã độc tệp tải lên", "keys": ["MALWARE_SCANNER_ADDRESS"]},
]
# = the Jenkinsfile's BI_MAT_1..7 (test_ops checks both). Seven because PostgreSQL may need one
# password per service holding DATABASE_DSN, and that is the largest need of any component.
SECRET_SLOTS = 7
KEEP = "giu"
FIELD = re.compile(r"^([A-Z][A-Z0-9_]*)(?:\.([a-z]{2,30}))?$")
PG_SSLMODES = ("require", "verify-ca", "verify-full")   # never a plaintext mode (rule 13 forbidden #1)
IDENT = re.compile(r"^[a-z_][a-z0-9_]{0,62}$")
S3_REGION = re.compile(r"^[a-z0-9-]{1,32}$")
# The URL shape deploy/cau-hinh/README.md mục 1 gives: scheme, account, optional password, a host
# list, the database, optional query. Anything else is "not readable" — never guessed at.
PG_URL = re.compile(r"^(postgres(?:ql)?)://([^:@/?#]+)(?::([^@/?#]*))?@([^/?#@]+)/([^/?#]+)(?:\?([^#]*))?$")
# What go-redis ParseURL reads (core/idem, core/ratelimit): scheme, optional account and password,
# ONE host:port, optional db number.
REDIS_URL = re.compile(r"^(rediss?)://(?:([^:@/]*)(?::([^@/]*))?@)?([^/?#@]+?)(?:/(\d{1,2}))?/?$")


def infra_component(cid: str) -> dict:
    for c in INFRA:
        if c["id"] == cid:
            return c
    raise PlanError(f"hạ tầng '{cid[:40]}' không có — {', '.join(c['id'] for c in INFRA)}")


def infra_text(cat: list) -> str:
    """One key per line, TAB-separated: component · label · key · placement · readers. Embedded in the
    Jenkinsfile for the form scripts (they run on the controller, with no checkout)."""
    idx = by_key(cat)
    lines = []
    for c in INFRA:
        for k in c["keys"]:
            e = idx.get(k)
            if e is None:
                raise PlanError(f"{k} (hạ tầng {c['id']}) không có trong deploy/cau-hinh/README.md")
            lines.append("\t".join([c["id"], c["label"], k, e["placement"], ",".join(e["users"]) or "-"]))
    return "\n".join(lines)


def secret_services(cat: list, cid: str) -> list:
    """Services whose <service>-secrets this component writes — the job reads exactly these."""
    idx = by_key(cat)
    out = set()
    for k in infra_component(cid)["keys"]:
        if idx[k]["placement"] == "secret":
            out.update(idx[k]["users"])
    return sorted(out)


def parse_fields(text: str) -> dict:
    """`FIELD=value` / `FIELD.svc=value` lines → {(FIELD, svc or ''): value}. Blank values are kept
    (they mean "keep"); blank lines and `#` comments are skipped."""
    out = {}
    for n, raw in enumerate(text.replace("\r", "").split("\n"), 1):
        s = raw.strip()
        if not s or s.startswith("#"):
            continue
        if "=" not in s:
            raise PlanError(f"dòng {n}: thiếu dấu '=' — viết Ô=giá trị")
        k, v = (x.strip() for x in s.split("=", 1))
        m = FIELD.match(k)
        if not m:
            raise PlanError(f"dòng {n}: '{k[:40]}' không phải tên ô (CHỮ_HOA hoặc CHỮ_HOA.dịch-vụ)")
        key = (m.group(1), m.group(2) or "")
        if key in out:
            raise PlanError(f"dòng {n}: ô {k} xuất hiện hai lần")
        out[key] = v
    return out


def slot_value(slots: dict, choice: str, what: str):
    """`giu`/'' → None; `1`..`7` → that password parameter's value. Never returns ''."""
    c = (choice or KEEP).strip().lower()
    if c == KEEP:
        return None
    if not re.fullmatch(r"[1-9]", c) or int(c) > SECRET_SLOTS:
        raise PlanError(f"{what}: '{c[:10]}' — chọn giu hoặc ô 1..{SECRET_SLOTS}")
    v = slots.get(c, "")
    if not v:
        raise PlanError(f"{what}: chọn ô BI_MAT_{c} nhưng ô ấy đang trống")
    if any(ord(ch) < 32 for ch in v) or len(v) > 1024:
        raise PlanError(f"{what}: giá trị ô BI_MAT_{c} có ký tự điều khiển hoặc quá dài")
    return v


def split_pg(dsn: str):
    m = PG_URL.match((dsn or "").strip())
    if not m:
        return None
    params = [tuple(p.split("=", 1)) if "=" in p else (p, "") for p in (m.group(6) or "").split("&") if p]
    return {"scheme": m.group(1), "user": m.group(2), "password": m.group(3), "hosts": m.group(4),
            "db": m.group(5), "params": params}


def join_pg(p: dict) -> str:
    auth = p["user"] + (":" + p["password"] if p["password"] is not None else "")
    q = "&".join(k + ("=" + v if v != "" else "") for k, v in p["params"])
    return f"{p['scheme']}://{auth}@{p['hosts']}/{p['db']}" + ("?" + q if q else "")


def pg_sslmode(p: dict) -> str:
    return next((v for k, v in p["params"] if k == "sslmode"), "")


def describe_pg(p) -> str:
    """What the plan may print of a DSN: account, hosts, database, sslmode — never the password."""
    if p is None:
        return "(chưa có / không đọc được)"
    return f"{p['user']}@{p['hosts']}/{p['db']} sslmode={pg_sslmode(p) or '(trống)'}"


def split_redis(dsn: str):
    m = REDIS_URL.match((dsn or "").strip())
    if not m:
        return None
    return {"scheme": m.group(1), "user": m.group(2) or "", "password": m.group(3), "host": m.group(4),
            "db": m.group(5) or ""}


def join_redis(p: dict) -> str:
    auth = ""
    if p["user"] or p["password"] is not None:
        auth = p["user"] + (":" + p["password"] if p["password"] is not None else "") + "@"
    return f"{p['scheme']}://{auth}{p['host']}" + ("/" + p["db"] if p["db"] else "")


def describe_redis(p) -> str:
    if p is None:
        return "(chưa có / không đọc được)"
    return f"{p['scheme']}://{p['host']}/{p['db'] or '0'} mật khẩu: {'có' if p['password'] else 'không'}"


def _plan_postgres(cat, f, secrets, slots, out):
    hosts = parse_hosts(f[("HOSTS", "")], "5432") if f.get(("HOSTS", "")) else None
    sslmode = f.get(("SSLMODE", ""), "")
    if sslmode and sslmode not in PG_SSLMODES:
        raise PlanError(f"sslmode '{sslmode[:20]}' không được phép — {' | '.join(PG_SSLMODES)} (luật 13)")
    for svc in by_key(cat)["DATABASE_DSN"]["users"]:
        raw = secrets.get(svc, {}).get("DATABASE_DSN")
        cur = split_pg(raw) if raw else None
        db, user = f.get(("DB", svc), ""), f.get(("USER", svc), "")
        for x in (db, user):
            if x and not IDENT.match(x):
                raise PlanError(f"{svc}: '{x[:40]}' — tên CSDL/tài khoản chỉ chữ thường, số, _ (tối đa 63)")
        pw = slot_value(slots, f.get(("PASSWORD", svc), KEEP), f"{svc}: mật khẩu")
        if not (hosts or sslmode or db or user or pw):
            continue
        if cur is None:
            if not (hosts and db and user and pw):
                raise PlanError(f"{svc}: DATABASE_DSN hiện tại {'không đúng dạng URL postgres' if raw else 'chưa có'} — "
                                f"phải điền đủ máy chủ, CSDL, tài khoản và ô mật khẩu cho dịch vụ này")
            new = {"scheme": "postgres", "user": user, "password": None, "hosts": hosts, "db": db, "params": []}
        else:
            new = dict(cur, params=list(cur["params"]))
            if user and user != cur["user"] and pw is None:
                raise PlanError(f"{svc}: đổi tài khoản {cur['user']} → {user} thì phải kèm ô mật khẩu")
        if hosts:
            new["hosts"] = hosts
        if db:
            new["db"] = db
        if user:
            new["user"] = user
        if pw is not None:
            new["password"] = quote(pw, safe="")
        if sslmode:
            new["params"] = [(k, v) for k, v in new["params"] if k != "sslmode"] + [("sslmode", sslmode)]
        if pg_sslmode(new) not in PG_SSLMODES:
            raise PlanError(f"{svc}: sslmode đang là '{pg_sslmode(new) or 'trống'}' — đường tới CSDL không chắc được "
                            f"mã hoá (luật 13); chọn sslmode require/verify-*")
        dsn = join_pg(new)
        if dsn == raw:
            continue
        out["secrets"].setdefault(svc, {})["DATABASE_DSN"] = dsn
        out["secret_rows"].append([svc, "DATABASE_DSN", describe_pg(cur), describe_pg(new),
                                   "đổi" if pw is not None else "giữ"])
        out["probe_dsn"][svc] = dsn


def _plan_redis(cat, f, secrets, slots, out):
    host = f.get(("HOST", ""), "")
    if host:
        if "," in host:
            raise PlanError("Redis: đúng MỘT host:port — core/idem và core/ratelimit đọc REDIS_DSN bằng go-redis "
                            "ParseURL, danh sách máy chủ làm dịch vụ từ chối khởi động")
        check_config_value("REDIS_ADDRESS", host)
    db = f.get(("DB", ""), "")
    if db and not re.fullmatch(r"\d{1,2}", db):
        raise PlanError(f"Redis: số db '{db[:10]}' phải là số 0..99")
    scheme = f.get(("SCHEME", ""), "")
    if scheme and scheme not in ("redis", "rediss"):
        raise PlanError(f"Redis: giao thức '{scheme[:10]}' — redis hoặc rediss")
    pw = slot_value(slots, f.get(("PASSWORD", ""), KEEP), "Redis: mật khẩu")
    if not (host or db or scheme or pw):
        return
    for svc in by_key(cat)["REDIS_DSN"]["users"]:
        raw = secrets.get(svc, {}).get("REDIS_DSN")
        cur = split_redis(raw) if raw else None
        if cur is None:
            if not (host and scheme and pw):
                raise PlanError(f"{svc}: REDIS_DSN hiện tại {'không đúng dạng URL redis' if raw else 'chưa có'} — "
                                f"phải điền máy chủ, giao thức và ô mật khẩu")
            new = {"scheme": scheme, "user": "", "password": None, "host": host, "db": db}
        else:
            new = dict(cur)
        if host:
            new["host"] = host
        if db:
            new["db"] = db
        if scheme:
            new["scheme"] = scheme
        if pw is not None:
            new["password"] = quote(pw, safe="")
        dsn = join_redis(new)
        if dsn == raw:
            continue
        out["secrets"].setdefault(svc, {})["REDIS_DSN"] = dsn
        out["secret_rows"].append([svc, "REDIS_DSN", describe_redis(cur), describe_redis(new),
                                   "đổi" if pw is not None else "giữ"])
        out["probe_tcp"].add(("redis", new["host"]))


def _plan_minio(cat, f, cm, secrets, slots, out):
    idx = by_key(cat)
    want = {}
    for k in ("OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_PUBLIC_ENDPOINT"):
        if f.get((k, "")):
            want[k] = https_origin(k, f[(k, "")])
    prefix = f.get(("OBJECT_STORAGE_BUCKET_PREFIX", ""), "")
    if prefix:
        if not BUCKET_PREFIX.match(prefix):
            raise PlanError(f"OBJECT_STORAGE_BUCKET_PREFIX '{prefix[:60]}' không làm thành tên bucket hợp lệ "
                            f"(chữ thường, số, . -)")
        want["OBJECT_STORAGE_BUCKET_PREFIX"] = prefix
    region = f.get(("OBJECT_STORAGE_REGION", ""), "")
    if region:
        if not S3_REGION.match(region):
            raise PlanError(f"OBJECT_STORAGE_REGION '{region[:40]}' — chữ thường, số, -")
        want["OBJECT_STORAGE_REGION"] = region
    media = f.get(("OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL", ""), "")
    if media:
        check_config_value("OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL", media)
        want["OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL"] = media.rstrip("/")
    else:
        # Left empty: rebuilt only when what it is made of moved (deploy/cau-hinh/README.md mục 6:
        # <PUBLIC_ENDPOINT>/<prefix>-public). Otherwise kept — never silently re-derived.
        pub_changed = "OBJECT_STORAGE_PUBLIC_ENDPOINT" in want and want["OBJECT_STORAGE_PUBLIC_ENDPOINT"] != (
            cm.get("OBJECT_STORAGE_PUBLIC_ENDPOINT") or "").rstrip("/")
        pre_changed = "OBJECT_STORAGE_BUCKET_PREFIX" in want and want["OBJECT_STORAGE_BUCKET_PREFIX"] != cm.get(
            "OBJECT_STORAGE_BUCKET_PREFIX")
        if pub_changed or pre_changed:
            pub = want.get("OBJECT_STORAGE_PUBLIC_ENDPOINT") or cm.get("OBJECT_STORAGE_PUBLIC_ENDPOINT") or ""
            pre = want.get("OBJECT_STORAGE_BUCKET_PREFIX") or cm.get("OBJECT_STORAGE_BUCKET_PREFIX") or ""
            if not (pub and pre):
                raise PlanError("OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL: không tự dựng được — thiếu public endpoint "
                                "hoặc tiền tố bucket; điền thẳng giá trị")
            want["OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL"] = (
                https_origin("OBJECT_STORAGE_PUBLIC_ENDPOINT", pub) + "/" + pre + "-public")
    for k, v in want.items():
        old = cm.get(k)
        if old is not None and old.rstrip("/") == v:
            continue
        out["configmap"][k] = v
        out["config_rows"].append([k, "(chưa có)" if old is None else old, v, ", ".join(idx[k]["users"])])

    for svc in idx["OBJECT_STORAGE_ACCESS_KEY"]["users"]:
        ak = slot_value(slots, f.get(("ACCESS_KEY", svc), KEEP), f"{svc}: access key")
        sk = slot_value(slots, f.get(("SECRET_KEY", svc), KEEP), f"{svc}: secret key")
        if ak is None and sk is None:
            continue
        if (ak is None) != (sk is None):
            raise PlanError(f"{svc}: đổi cặp khoá MinIO thì đổi CẢ access key lẫn secret key — một nửa cặp cũ "
                            f"với một nửa cặp mới không đăng nhập được")
        have = secrets.get(svc, {})
        for key, v in (("OBJECT_STORAGE_ACCESS_KEY", ak), ("OBJECT_STORAGE_SECRET_KEY", sk)):
            if have.get(key) == v:
                continue
            out["secrets"].setdefault(svc, {})[key] = v
            out["secret_rows"].append([svc, key, "có" if key in have else "chưa có", "giá trị mới", "đổi"])

    endpoint = out["configmap"].get("OBJECT_STORAGE_ENDPOINT")
    target = endpoint or cm.get("OBJECT_STORAGE_ENDPOINT") or ""
    if (out["configmap"] or out["secrets"]) and target:
        hp = https_origin("OBJECT_STORAGE_ENDPOINT", target)[len("https://"):]
        out["probe_tcp"].add(("minio", hp if ":" in hp else hp + ":443"))
    old_ep = (cm.get("OBJECT_STORAGE_ENDPOINT") or "").rstrip("/")
    if endpoint and old_ep and (cm.get("BASEMAP_URL") or "").startswith(old_ep + "/"):
        out["warnings"].append("BASEMAP_URL vẫn trỏ MinIO CŨ (" + old_ep + "). Job không đổi nó: tệp nền bản đồ phải có "
                               "trên MinIO mới trước — job vigov-deploy, việc nen-ban-do, rồi đặt BASEMAP_URL "
                               "(kb/40-runbooks/basemap-pmtiles.md).")


def _plan_clamav(cat, f, cm, out):
    v = f.get(("MALWARE_SCANNER_ADDRESS", ""), "")
    if not v:
        return
    check_config_value("MALWARE_SCANNER_ADDRESS", v)
    v = ",".join(x.strip() for x in v.split(","))
    old = cm.get("MALWARE_SCANNER_ADDRESS")
    if old == v:
        return
    out["configmap"]["MALWARE_SCANNER_ADDRESS"] = v
    out["config_rows"].append(["MALWARE_SCANNER_ADDRESS", "(chưa có)" if old is None else old, v,
                               ", ".join(by_key(cat)["MALWARE_SCANNER_ADDRESS"]["users"])])
    for hp in v.split(","):
        out["probe_tcp"].add(("clamav", hp))


def plan_infra(cat: list, cid: str, edits_text: str, cm: dict, secrets: dict, slots: dict) -> dict:
    """→ the whole change, or PlanError. `secrets` = {service: {key: plain value}} as on the cluster;
    `slots` = {"1": value, …} of the password parameters that are filled. Pure: no I/O."""
    infra_component(cid)
    f = parse_fields(edits_text)
    out = {"configmap": {}, "secrets": {}, "config_rows": [], "secret_rows": [], "warnings": [],
           "probe_tcp": set(), "probe_dsn": {}}
    if cid == "postgresql":
        _plan_postgres(cat, f, secrets, slots, out)
    elif cid == "redis":
        _plan_redis(cat, f, secrets, slots, out)
    elif cid == "minio":
        _plan_minio(cat, f, cm, secrets, slots, out)
    elif cid == "clamav":
        _plan_clamav(cat, f, cm, out)
    idx = by_key(cat)
    services = set(out["secrets"])
    for k in out["configmap"]:
        services.update(idx[k]["users"])
    out["restart"] = [deployment_of(s) for s in ordered_services(sorted(s for s in services if s != WEB_ADMIN))]
    if WEB_ADMIN in services:
        out["restart"].append(deployment_of(WEB_ADMIN))
    return out


def probe_script(plan: dict) -> str:
    """Run inside the cluster BEFORE anything is written: TCP to every new address, and a real login
    with every new DSN (env DSN_<service>, from a temporary Secret). Prints hosts, never a DSN."""
    out = ["bad=0",
           "tcp() { h=${2%:*}; p=${2##*:}; if nc -z -w 5 \"$h\" \"$p\" 2>/dev/null; then echo \"   $1 $2: mở\"; "
           "else echo \"   $1 $2: KHÔNG tới được\"; bad=1; fi; }",
           # psql's own error names host, port and account; the sed masks URL credentials regardless.
           "pg() { d=$(printenv \"DSN_$1\" || true); if r=$(psql \"$d\" -XAtqc 'SELECT 1' 2>&1); then "
           "echo \"   postgresql $1: đăng nhập được\"; else echo \"   postgresql $1: KHÔNG đăng nhập được — "
           "$(printf '%s' \"$r\" | head -1 | sed -e 's#://[^ ]*@#://***@#g')\"; bad=1; fi; }"]
    for kind, hp in sorted(plan["probe_tcp"]):
        out.append(f"tcp {kind} {hp}")
    for svc in ordered_services(sorted(plan["probe_dsn"])):
        out.append(f"pg {svc}")
    out.append("exit $bad")
    return "\n".join(out) + "\n"

# ─────────────────────────────────────────────────────────────────────────────────────────────
# Self-hosted basemap (task nen-ban-do) — procedure: kb/40-runbooks/basemap-pmtiles.md, why: ADR 0072.
# The job builds on the BUILD MACHINE and uploads to the environment's MinIO; it never changes
# BASEMAP_URL — that waits for the signed sovereignty check.
# ─────────────────────────────────────────────────────────────────────────────────────────────

BASEMAP_DATE = re.compile(r"^20[0-9]{6}$")
# The prefix part of an S3 bucket name; "-public" is appended (3..63 chars total, lowercase).
BUCKET_PREFIX = re.compile(r"^[a-z0-9][a-z0-9.-]{0,53}[a-z0-9]$")
# Scheme + host[:port], nothing else: a path would be read by mc as part of the bucket, and
# credentials in a ConfigMap value would be a secret outside a Secret (rule 8).
HTTPS_ORIGIN = re.compile(r"^https://([A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?(:\d{1,5})?)/?$")

# PINNED TOOLS. A binary fetched at run time and run with the MinIO write key is supply chain: an
# unpinned download is whatever the server returns that day. Each value below was copied from the
# OFFICIAL release on 11/10/2026 (sources in the `where` of each tool). basemap_tools() REFUSES to run
# while a value is UNPINNED. Changing a version means changing its checksum in the same edit.
UNPINNED = "UNPINNED-copy-the-sha256-from-the-official-release-page"
PMTILES_VERSION = "1.31.2"          # = the version kb/40-runbooks/basemap-pmtiles.md names
# GitHub asset digest of go-pmtiles_1.31.2_Linux_x86_64.tar.gz (the release has no checksums.txt).
PMTILES_SHA256 = "3ed7dbf4ec2e6dfe5e25b6f70d1ffc932729f93c86db353bf514dd71010a312f"
MC_RELEASE = "RELEASE.2025-08-13T08-35-41Z"
# GitHub file mc.linux-amd64.<MC_RELEASE>.sha256sum (dl.min.io answers 410 Gone).
MC_SHA256 = "01f866e9c5f9b87c2b09116fa5d7c06695b106242d829a8bb32990c00312e891"
BASEMAP_TOOLS = [
    {"name": "pmtiles", "sha256": PMTILES_SHA256, "kind": "tar-gz:pmtiles",
     "url": f"https://github.com/protomaps/go-pmtiles/releases/download/v{PMTILES_VERSION}/"
            f"go-pmtiles_{PMTILES_VERSION}_Linux_x86_64.tar.gz",
     # The release has NO checksums.txt (404, checked 11/10/2026): GitHub shows each asset's sha256 digest.
     "where": f"https://github.com/protomaps/go-pmtiles/releases/tag/v{PMTILES_VERSION} → mục Assets, "
              f"digest sha256 của go-pmtiles_{PMTILES_VERSION}_Linux_x86_64.tar.gz → hằng PMTILES_SHA256"},
    # GitHub, not dl.min.io: dl.min.io answers 410 Gone for mc binaries (checked 11/10/2026).
    {"name": "mc", "sha256": MC_SHA256, "kind": "binary",
     "url": f"https://github.com/minio/mc/releases/download/{MC_RELEASE}/mc.linux-amd64.{MC_RELEASE}",
     "where": f"https://github.com/minio/mc/releases/download/{MC_RELEASE}/mc.linux-amd64.{MC_RELEASE}.sha256sum "
              f"(cột đầu) → hằng MC_SHA256"},
]

# Where the existing MinIO key pair is read from (owner 11/10/2026: no new MinIO user, no Jenkins
# credential). comms first — it already writes the public bucket (deploy/cau-hinh/README.md mục 1,
# OBJECT_STORAGE_ACCESS_KEY row); platform second, for the same reason. Key names as envFrom reads them.
BASEMAP_KEY_SOURCES = ["comms-secrets", "platform-secrets"]
STORAGE_KEY_NAMES = ("OBJECT_STORAGE_ACCESS_KEY", "OBJECT_STORAGE_SECRET_KEY")
# Object prefix inside the public bucket (owner 11/10/2026). The dated subfolder stays: a live prefix is
# read in byte ranges, so a refresh is always a new folder, never an overwrite.
BASEMAP_PREFIX = "config/map"


def check_basemap_date(date: str) -> str:
    d = (date or "").strip()
    if not BASEMAP_DATE.match(d):
        raise PlanError(f"NGAY_NEN '{d[:20]}' phải là YYYYMMDD (20xxxxxx) của một bản dựng hằng ngày Protomaps")
    try:
        datetime.datetime.strptime(d, "%Y%m%d")
    except ValueError:
        raise PlanError(f"NGAY_NEN '{d}' không phải một ngày có thật")
    return d


def https_origin(key: str, value: str) -> str:
    """`https://host[:port]` → itself without a trailing slash. http is REFUSED (rule 13)."""
    v = (value or "").strip()
    if v.startswith("http://"):
        raise PlanError(f"{key} là http:// — luật 13: khoá MinIO không bao giờ đi qua kênh không mã hoá")
    m = HTTPS_ORIGIN.match(v)
    if not m:
        raise PlanError(f"{key}: phải là https://host[:cổng], không đường dẫn, không thông tin đăng nhập")
    return "https://" + m.group(1)


def plan_basemap(cm: dict, date: str) -> dict:
    """common-config data + date → where the files go and the BASEMAP_URL they will be read at."""
    d = check_basemap_date(date)
    for k in ("OBJECT_STORAGE_BUCKET_PREFIX", "OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_ENDPOINT"):
        if not (cm.get(k) or "").strip():
            raise PlanError(f"common-config thiếu {k} — đặt bằng job vigov-van-hanh, hạ tầng minio, trước")
    prefix = cm["OBJECT_STORAGE_BUCKET_PREFIX"].strip()
    if not BUCKET_PREFIX.match(prefix):
        raise PlanError(f"OBJECT_STORAGE_BUCKET_PREFIX '{prefix[:60]}' không làm thành tên bucket hợp lệ")
    bucket = prefix + "-public"
    # Upload from the BUILD MACHINE: the internal endpoint is cluster DNS it cannot resolve.
    upload = https_origin("OBJECT_STORAGE_PUBLIC_ENDPOINT", cm["OBJECT_STORAGE_PUBLIC_ENDPOINT"])
    # Read by the web-admin POD: the internal endpoint (netpol allow-web-admin-egress-storage).
    internal = https_origin("OBJECT_STORAGE_ENDPOINT", cm["OBJECT_STORAGE_ENDPOINT"])
    return {"date": d, "bucket": bucket, "upload_endpoint": upload,
            "dest": f"{bucket}/{BASEMAP_PREFIX}/{d}/", "basemap_url": f"{internal}/{bucket}/{BASEMAP_PREFIX}/{d}"}


def pick_key_source(present: dict) -> str:
    """{secret name: set of key names} → the first Secret in BASEMAP_KEY_SOURCES holding BOTH keys."""
    for s in BASEMAP_KEY_SOURCES:
        if set(STORAGE_KEY_NAMES) <= present.get(s, set()):
            return s
    raise PlanError("không Secret nào trong " + ", ".join(BASEMAP_KEY_SOURCES) + " có đủ "
                    + " + ".join(STORAGE_KEY_NAMES) + " — đặt cặp khoá MinIO của comms bằng job vigov-van-hanh, hạ tầng minio, rồi bấm lại")


def basemap_tools(tools: list = None) -> list:
    """[(name, url, sha256, kind)] — refuses while any checksum is not a real SHA-256."""
    tools = BASEMAP_TOOLS if tools is None else tools
    missing = [t for t in tools if not re.fullmatch(r"[0-9a-f]{64}", t["sha256"])]
    if missing:
        raise PlanError("chưa ghim SHA-256 cho " + ", ".join(t["name"] for t in missing)
                        + " — job không chạy công cụ tải về mà không kiểm. Lấy giá trị ở trang phát hành CHÍNH THỨC: "
                        + " · ".join(t["where"] for t in missing)
                        + "; sửa trong deploy/van-hanh/ops.py, commit, chạy lại (deploy/van-hanh/README.md §Nền bản đồ)")
    return [(t["name"], t["url"], t["sha256"], t["kind"]) for t in tools]


def mc_host_url(endpoint: str, access_key: str, secret_key: str) -> str:
    """Value of MC_HOST_<alias>: mc reads the alias from the environment, so the keys go neither on
    argv (visible in `ps`) nor into an mc config file. Keys are percent-encoded — a `/` or `@` in a
    MinIO secret key would otherwise change where mc connects."""
    origin = https_origin("OBJECT_STORAGE_PUBLIC_ENDPOINT", endpoint)
    for name, v in (("access key", access_key), ("secret key", secret_key)):
        if not v or any(ord(ch) < 32 for ch in v):
            raise PlanError(f"khoá MinIO đọc từ Secret: {name} rỗng hoặc có ký tự điều khiển")
    return "https://" + quote(access_key, safe="") + ":" + quote(secret_key, safe="") + "@" + origin[len("https://"):]


# ─────────────────────────────────────────────────────────────────────────────────────────────
# CLI
# ─────────────────────────────────────────────────────────────────────────────────────────────

def _write_private(path: str, text: str) -> None:
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        f.write(text)


def _json(path: str):
    with io.open(path, encoding="utf-8") as f:
        return json.load(f)


def _b64(s: str) -> str:
    return base64.b64encode(s.encode("utf-8")).decode("ascii")


def _labels(extra: dict) -> dict:
    return dict({"app.kubernetes.io/managed-by": "vigov-van-hanh"}, **extra)


def _notes(items: list) -> dict:
    """Backup annotations: who · ticket · build. Free text, but bounded and single-line."""
    out = {}
    for kv in items:
        k, _, v = kv.partition("=")
        if re.fullmatch(r"vigov\.vn/[a-z-]{1,40}", k):
            out[k] = "".join(ch if ord(ch) >= 32 else " " for ch in v)[:200]
    return out


def main(argv=None) -> int:
    for s in (sys.stdout, sys.stderr):
        try:
            s.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass
    ap = argparse.ArgumentParser(prog="ops.py")
    sub = ap.add_subparsers(dest="cmd", required=True)

    c = sub.add_parser("catalog")
    c.add_argument("--write", action="store_true")
    c.add_argument("--check", action="store_true")

    c = sub.add_parser("infra-services")      # services whose <svc>-secrets the component writes
    c.add_argument("--component", required=True)

    c = sub.add_parser("infra-plan")
    c.add_argument("--component", required=True)
    c.add_argument("--edits", required=True)
    c.add_argument("--configmap", required=True)     # kubectl get configmap common-config -o json
    c.add_argument("--secrets-dir", required=True)   # <svc>.json = kubectl get secret <svc>-secrets -o json
    c.add_argument("--slot-dir", required=True)      # slotN = value of password parameter BI_MAT_N
    c.add_argument("--out-dir", required=True)
    c.add_argument("--stamp", required=True)
    c.add_argument("--probe-secret", required=True)  # name of the temporary Secret holding new DSNs
    c.add_argument("--note", action="append", default=[])

    c = sub.add_parser("verify-data")   # read-back: every key of the patch has exactly that value
    c.add_argument("--current", required=True)
    c.add_argument("--patch", required=True)

    c = sub.add_parser("basemap-plan")
    c.add_argument("--current", required=True)
    c.add_argument("--date", required=True)
    c.add_argument("--out", required=True)

    c = sub.add_parser("basemap-tools")
    c.add_argument("--out", required=True)

    c = sub.add_parser("basemap-key-source")   # TSV `<secret>\t<key name>` (names only) → Secret name
    c.add_argument("--present", required=True)

    c = sub.add_parser("mc-host")      # key FILES in, MC_HOST_ value FILE out — nothing printed
    c.add_argument("--endpoint", required=True)
    c.add_argument("--access-key-file", required=True)
    c.add_argument("--secret-key-file", required=True)
    c.add_argument("--out", required=True)

    c = sub.add_parser("pod")
    c.add_argument("--name", required=True)
    c.add_argument("--out", required=True)
    c.add_argument("--script", required=True)
    c.add_argument("--secret-env", action="append", default=[])
    c.add_argument("--envfrom-secret", action="append", default=[])
    c.add_argument("--pull-secret", action="append", default=[])

    a = ap.parse_args(argv)
    try:
        return _run(a)
    except PlanError as e:
        print(f"DỪNG: {e}. Không ghi gì.")
        return 2


def _read_secrets(d: str, services: list) -> dict:
    """{svc: {key: plain value}} from `kubectl get secret -o json` files. Values stay in memory."""
    out = {}
    for svc in services:
        p = os.path.join(d, svc + ".json")
        if not os.path.exists(p):
            raise PlanError(f"không đọc được secret/{svc}-secrets")
        data = _json(p).get("data", {}) or {}
        out[svc] = {k: base64.b64decode(v).decode("utf-8") for k, v in data.items()}
    return out


def _run(a) -> int:
    if a.cmd == "catalog":
        cat = build_catalog()
        text = catalog_text(cat)
        if a.write or a.check:
            jf = _read(JENKINSFILE)
            new = render_jenkinsfile(jf, generated_region(cat))
            if a.check:
                if new != jf:
                    print("[FAIL] deploy/van-hanh/Jenkinsfile: vùng SINH RA (opsCatalog/opsInfra) cũ so với "
                          "core/config + deploy/cau-hinh/README.md + .env.example + INFRA của ops.py.\n"
                          "       chạy: python deploy/van-hanh/ops.py catalog --write")
                    return 1
                print(f"[PASS] danh mục khoá của job vận hành khớp mã — {text.count(chr(10)) + 1} khoá, "
                      f"{len(INFRA)} hạ tầng")
                return 0
            with io.open(JENKINSFILE, "w", encoding="utf-8", newline="\n") as f:
                f.write(new)
            print("đã ghi vùng SINH RA vào deploy/van-hanh/Jenkinsfile")
            return 0
        print(text)
        return 0

    if a.cmd == "infra-services":
        print(" ".join(secret_services(build_catalog(), a.component)))
        return 0

    if a.cmd == "infra-plan":
        cat = build_catalog()
        comp = infra_component(a.component)
        cm = _json(a.configmap).get("data", {}) or {}
        secrets = _read_secrets(a.secrets_dir, secret_services(cat, a.component))
        slots = {}
        for n in range(1, SECRET_SLOTS + 1):
            p = os.path.join(a.slot_dir, f"slot{n}")
            if os.path.exists(p):
                with io.open(p, encoding="utf-8") as f:
                    slots[str(n)] = f.read().strip()
        plan = plan_infra(cat, a.component, _read(a.edits), cm, secrets, slots)
        print(f"Hạ tầng: {comp['label']}")
        if not (plan["configmap"] or plan["secrets"]):
            print("Không có giá trị nào khác cụm đang chạy — không có gì để ghi. (Ô để trống = giữ nguyên.)")
            return 3
        if plan["config_rows"]:
            print("\n── configmap/common-config")
            print(table(["KHOÁ", "CŨ", "MỚI", "DỊCH VỤ ĐỌC"], plan["config_rows"]))
        if plan["secret_rows"]:
            print("\n── <dịch vụ>-secrets (không in giá trị bí mật)")
            print(table(["DỊCH VỤ", "KHOÁ", "CŨ", "MỚI", "MẬT KHẨU/KHOÁ"], plan["secret_rows"]))
            print("Giá trị cũ của các khoá trên được sao lưu vào <dịch vụ>-secrets-backup-" + a.stamp
                  + " (Secret riêng, không gắn Deployment nào) trước khi ghi.")
        for w in plan["warnings"]:
            print("\n⚠ " + w)
        print("\nKhởi động lại theo thứ tự: " + (", ".join(plan["restart"]) or "(không)"))
        os.makedirs(a.out_dir, exist_ok=True)
        notes = _notes(a.note)
        for svc, data in plan["secrets"].items():
            _write_private(os.path.join(a.out_dir, f"secret-{svc}.json"),
                           json.dumps({"data": {k: _b64(v) for k, v in data.items()}}))
            old = {k: _b64(secrets[svc][k]) for k in data if k in secrets.get(svc, {})}
            if old:
                _write_private(os.path.join(a.out_dir, f"backup-{svc}.json"), json.dumps(secret_manifest(
                    f"{svc}-secrets-backup-{a.stamp}", old, _labels({"vigov.vn/backup-of": f"{svc}-secrets"}), notes)))
        if plan["configmap"]:
            _write_private(os.path.join(a.out_dir, "configmap.json"), json.dumps({"data": plan["configmap"]},
                                                                                ensure_ascii=False))
        _write_private(os.path.join(a.out_dir, "restart.txt"), "\n".join(plan["restart"]) + "\n")
        _write_private(os.path.join(a.out_dir, "probe.sh"), probe_script(plan))
        if plan["probe_dsn"]:
            _write_private(os.path.join(a.out_dir, "probe-secret.json"), json.dumps(secret_manifest(
                a.probe_secret, {f"DSN_{s}": _b64(d) for s, d in plan["probe_dsn"].items()},
                _labels({"app.kubernetes.io/name": "vigov-ops-probe"}))))
        return 0

    if a.cmd == "verify-data":
        cur = _json(a.current).get("data", {}) or {}
        want = _json(a.patch).get("data", {}) or {}
        off = sorted(k for k, v in want.items() if cur.get(k) != v)
        if off:
            print("✗ đọc lại từ cụm, khoá KHÔNG khớp giá trị vừa ghi: " + ", ".join(off) + " (không in giá trị)")
            return 1
        print(f"✓ đọc lại từ cụm khớp {len(want)} khoá: {', '.join(sorted(want))} (so trên tệp, không in giá trị)")
        return 0

    if a.cmd == "pod":
        _write_private(a.out, json.dumps(build_pod(a)))
        return 0

    if a.cmd == "basemap-plan":
        p = plan_basemap(_json(a.current).get("data", {}) or {}, a.date)
        print(table(["", ""], [["Bản dựng Protomaps", p["date"]], ["Bucket", p["bucket"]],
                               ["Tải lên qua (máy build)", p["upload_endpoint"]],
                               ["Đích", p["dest"]], ["BASEMAP_URL sẽ là", p["basemap_url"]]]))
        _write_private(a.out, f"BUCKET={p['bucket']}\nUPLOAD_ENDPOINT={p['upload_endpoint']}\n"
                              f"BASEMAP_URL={p['basemap_url']}\n")
        return 0

    if a.cmd == "basemap-key-source":
        present = {}
        for ln in _read(a.present).splitlines():
            s, _, k = ln.partition("\t")
            if s.strip() and k.strip():
                present.setdefault(s.strip(), set()).add(k.strip())
        print(pick_key_source(present))
        return 0

    if a.cmd == "basemap-tools":
        tools = basemap_tools()
        _write_private(a.out, "".join("\t".join(t) + "\n" for t in tools))
        print("công cụ đã ghim: " + " · ".join(f"{n} {s[:12]}…" for n, _, s, _ in tools))
        return 0

    if a.cmd == "mc-host":
        with io.open(a.access_key_file, encoding="utf-8") as f:
            ak = f.read().strip()
        with io.open(a.secret_key_file, encoding="utf-8") as f:
            sk = f.read().strip()
        _write_private(a.out, mc_host_url(a.endpoint, ak, sk))
        return 0

    raise PlanError(f"lệnh '{a.cmd}' không biết")


if __name__ == "__main__":
    sys.exit(main())
