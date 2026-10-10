#!/usr/bin/env python3
"""Pure logic behind the operations job `deploy/van-hanh/Jenkinsfile` (ADR 0089).

WHY THIS FILE EXISTS: the job decides things that are easy to get silently wrong — which keys a
ConfigMap edit may touch, which services must restart for a changed key, whether a resource
request exceeds its limit, what a DSN looks like. Written in Groovy those decisions would live in
a Jenkinsfile no test ever runs. Here they are plain functions with unit tests
(`deploy/van-hanh/test_ops.py`, run by `make buildfiles`), and the Groovy only moves files.

SECRET VALUES: the commands that touch them (`secret-apply`, `dsn`, `db-passwords`) read and write
FILES only (mode 0600) and never print a value — not even a prefix. Every printed line names keys.

Python 3.8+: the build machine is CentOS 7, `python3` there is 3.6 and the job picks the newest
`python3.X` it finds (same loop as the root Jenkinsfile).
"""

from __future__ import annotations

import argparse
import base64
import glob
import hashlib
import io
import json
import os
import re
import secrets
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
JENKINSFILE = os.path.join(ROOT, "deploy", "van-hanh", "Jenkinsfile")

# ─────────────────────────────────────────────────────────────────────────────────────────────
# Key catalog — DERIVED, never hand-listed (rule 9; ADR 0089 "Ràng buộc mang theo", rule 11 row).
#
#   placement : deploy/cau-hinh/README.md section 1 (secret) · 2 (configmap) · 3 (env) · 4 (unset).
#               That table is itself checked against the code by tools/check_env_map.py.
#   level/users: core/config `r.read("KEY", …, <level>, <groups>)` × `config.Uses(...)` of every
#               service-*/cmd/server/main.go — the same derivation tools/check_env_map.py makes.
#   registered: the key (dashes read as underscores, rule 11 invariant 4) has a line in .env.example.
# ─────────────────────────────────────────────────────────────────────────────────────────────

READ_CALL = re.compile(
    r"\br\.read\(\s*\"([A-Z0-9_]+)\"\s*,.+?,\s*(requiredEverywhere|requiredInProd|optional)\s*((?:,\s*\w+\s*)*)\)",
    re.S,
)
USES = re.compile(r"config\.Uses\(([^)]*)\)", re.S)
GROUP_NAME = re.compile(r"config\.(\w+)")
ENV_LINE = re.compile(r"^([A-Z][A-Z0-9_]*)=")
SECTION = re.compile(r"^##\s+(\d)\.")
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
        if web_only:
            level, users = "opt", [WEB_ADMIN]
        elif key in reads:
            lv, gs = reads[key]
            users = all_services if not gs else sorted(s for s, d in groups.items() if d & gs)
            level = lv
        else:
            level, users = "opt", []
        out.append({
            "key": key,
            "placement": placement,
            "level": level if users else "unused",
            "users": users,
            "shared": "cùng một giá trị" in row,
            "registered": key.replace("-", "_") in env_keys,
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


def generated_region(cat: list, services: list) -> str:
    """The Jenkinsfile block the FORM scripts read (they run on the controller, with no checkout).
    Three methods, each a ''' literal holding plain lines: no backslash, no quote, no `'''`."""
    presets = "\n".join(
        f"{n}\t{p['replicas']}\t{p['cpu'][0]}/{p['cpu'][1]}\t{p['mem'][0]}/{p['mem'][1]}"
        for n, p in PRESETS.items())

    def method(name: str, body: str) -> str:
        return f"String {name}() {{\n  return '''\n{body}\n'''\n}}\n"

    return (method("opsCatalog", catalog_text(cat)) + method("opsPresets", presets)
            + method("opsDbLinesProd", db_lines(services, "prod"))
            + method("opsDbLinesStaging", db_lines(services, "staging")))


def render_jenkinsfile(text: str, region: str) -> str:
    if not REGION.search(text):
        raise ValueError("không thấy vùng `// BEGIN GENERATED ops-data` trong Jenkinsfile")
    return REGION.sub(lambda m: m.group(1) + region + m.group(3), text, count=1)


def by_key(cat: list) -> dict:
    return {e["key"]: e for e in cat}


def deployment_of(user: str) -> str:
    return "vigov-web-admin" if user == WEB_ADMIN else "vigov-service-" + user


# ─────────────────────────────────────────────────────────────────────────────────────────────
# Shared parsing
# ─────────────────────────────────────────────────────────────────────────────────────────────

class PlanError(Exception):
    """A refusal the operator must read. Message is Vietnamese; it never carries a secret."""


def parse_assignments(text: str) -> list:
    """`KEY=value` lines → [(key, value)]. Blank lines and `#` comments skipped."""
    out, seen = [], set()
    for n, raw in enumerate(text.replace("\r", "").split("\n"), 1):
        s = raw.strip()
        if not s or s.startswith("#"):
            continue
        if "=" not in s:
            raise PlanError(f"dòng {n}: thiếu dấu '=' — viết KHOÁ=giá trị")
        k, v = s.split("=", 1)
        k, v = k.strip(), v.strip()
        if not re.fullmatch(r"[A-Z][A-Z0-9_-]*", k):
            raise PlanError(f"dòng {n}: '{k[:40]}' không phải tên khoá (chữ hoa, số, _ hoặc -)")
        if k in seen:
            raise PlanError(f"dòng {n}: khoá {k} xuất hiện hai lần")
        seen.add(k)
        out.append((k, v))
    return out


def table(headers: list, rows: list) -> str:
    cols = [list(map(str, c)) for c in zip(headers, *rows)] if rows else [[h] for h in headers]
    w = [max(len(x) for x in c) for c in cols]
    line = lambda r: "  ".join(str(x).ljust(w[i]) for i, x in enumerate(r)).rstrip()
    sep = "  ".join("─" * x for x in w)
    return "\n".join([line(headers), sep] + [line(r) for r in rows])


# ─────────────────────────────────────────────────────────────────────────────────────────────
# ConfigMap common-config
# ─────────────────────────────────────────────────────────────────────────────────────────────

HOSTPORT = re.compile(r"^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?:\d{1,5}$")
HOSTNAME = re.compile(r"^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$")
OPEN_CIDRS = {"0.0.0.0/0", "::/0"}


def check_config_value(key: str, value: str, env_expected: str) -> None:
    if len(value) > 4096 or any(ord(ch) < 32 for ch in value):
        raise PlanError(f"{key}: giá trị quá dài hoặc có ký tự điều khiển")
    if key == "ENV":
        if value != env_expected:
            raise PlanError(f"ENV phải là '{env_expected}' ở namespace này, không phải '{value}'")
        return
    if key.endswith(("_ADDRESS", "_ADDR")):
        # Rule 11 invariant 5: an address is a LIST from its first line, even with one node.
        for item in value.split(","):
            if not HOSTPORT.match(item.strip()):
                raise PlanError(f"{key}: '{item.strip()}' không đúng dạng host:port "
                                f"(danh sách cách nhau bởi dấu phẩy, không tcp://)")
        return
    if key.endswith(("_ENDPOINT", "_URL")):
        # Rule 13 invariant 1: traffic to data stores is encrypted. http:// is refused, not warned.
        if not re.fullmatch(r"https://[^\s@]+", value):
            raise PlanError(f"{key}: phải là https://… và không mang thông tin đăng nhập")
        return
    if key.endswith("_HOST"):
        if not HOSTNAME.match(value):
            raise PlanError(f"{key}: tên host trần, chữ thường, không giao thức/cổng/đường dẫn")
        return
    if key == "TRUSTED_PROXY_CIDRS":
        for item in value.split(","):
            it = item.strip()
            if it in OPEN_CIDRS:
                raise PlanError("TRUSTED_PROXY_CIDRS: không bao giờ 0.0.0.0/0 hay ::/0 (pod từ chối khởi động)")
            if not re.fullmatch(r"[0-9a-fA-F:.]+/\d{1,3}", it):
                raise PlanError(f"TRUSTED_PROXY_CIDRS: '{it}' không phải dải CIDR")


def plan_config(cat: list, current: dict, edits_text: str, env_expected: str):
    """→ (rows, patch dict, restart deployment list). Raises PlanError on any refusal."""
    idx = by_key(cat)
    rows, patch, restart = [], {}, set()
    for key, value in parse_assignments(edits_text):
        e = idx.get(key)
        if e is None or not e["registered"]:
            raise PlanError(f"{key}: không có trong .env.example / bảng deploy/cau-hinh/README.md — "
                            f"luật 11 cấm #4: khoá ngoài sổ đăng ký không được thêm")
        if e["placement"] == "secret":
            raise PlanError(f"{key}: là khoá SECRET — dùng việc sua-secret, không bao giờ ConfigMap")
        if e["placement"] == "env":
            raise PlanError(f"{key}: đặt thẳng trong env của Deployment (README mục 3), không ở common-config")
        if e["placement"] == "unset":
            raise PlanError(f"{key}: README mục 4 — 'Không đặt'")
        if value == "":
            continue  # empty = keep
        check_config_value(key, value, env_expected)
        old = current.get(key)
        if old == value:
            continue
        patch[key] = value
        deps = [deployment_of(u) for u in e["users"]]
        restart.update(deps)
        rows.append([key, "(chưa có)" if old is None else old, value, ", ".join(e["users"]) or "(không dịch vụ nào đọc)"])
    return rows, {"data": patch}, sorted(restart)


def audit_config(cat: list, current: dict):
    """→ (missing rows, extra keys) of common-config against the catalog."""
    idx = by_key(cat)
    missing = [[e["key"], {"all": "bắt buộc", "prod": "bắt buộc ở staging/prod"}.get(e["level"], "không"),
                ", ".join(e["users"])]
               for e in cat if e["placement"] == "configmap" and e["key"] not in current]
    extra = sorted(k for k in current if k not in idx or not idx[k]["registered"])
    return missing, extra


# ─────────────────────────────────────────────────────────────────────────────────────────────
# Secret <service>-secrets
# ─────────────────────────────────────────────────────────────────────────────────────────────

MODES = {"giu", "o1", "o2", "o3", "sinh", "xoay"}


def key_bytes(key: str) -> int:
    # README section 1: encryption keys are `openssl rand -base64 32`, signing/caller keys 48.
    return 32 if "ENCRYPTION" in key else 48


def secret_allowed(e: dict, service: str) -> bool:
    return e["placement"] == "secret" and e["registered"] and service in e["users"]


def plan_secret(cat: list, service: str, present: set, edits_text: str, slots_filled: set):
    """→ (rows, actions [(key, mode, nbytes)]). Values never enter this function."""
    idx = by_key(cat)
    rows, actions, slot_users = [], [], {}
    for key, mode in parse_assignments(edits_text):
        e = idx.get(key)
        if e is None or not secret_allowed(e, service):
            raise PlanError(f"{key}: không phải khoá của {service}-secrets theo bảng README mục 1 + "
                            f"config.Uses của {service} — không thêm khoá ngoài sổ (luật 11 cấm #4)")
        mode = mode.lower()
        if mode not in MODES:
            raise PlanError(f"{key}: chế độ '{mode}' không hợp lệ — giu | o1 | o2 | o3 | sinh | xoay")
        if mode == "giu":
            continue
        is_key = key.endswith(("_KEY", "_KEYS"))
        if mode in ("sinh", "xoay"):
            if not is_key:
                raise PlanError(f"{key}: không phải khoá ký/mã hoá — chỉ nhập qua ô o1/o2/o3")
            if e["shared"]:
                raise PlanError(f"{key}: phải CÙNG MỘT GIÁ TRỊ ở nhiều Secret ({', '.join(e['users'])}) — "
                                f"sinh riêng ở một nơi là làm lệch; nhập cùng giá trị qua ô o1/o2/o3 cho từng dịch vụ")
        if mode == "sinh" and key in present:
            raise PlanError(f"{key}: đã có — 'sinh' chỉ dành cho khoá CHƯA có. Đổi khoá đang dùng: 'xoay' "
                            f"(thêm khoá mới lên đầu, giữ khoá cũ) — ghi đè khoá mã hoá là mất mọi dữ liệu đã niêm")
        if mode == "xoay":
            if not key.endswith("_KEYS"):
                raise PlanError(f"{key}: chỉ khoá dạng danh sách (_KEYS) mới xoay được")
            if key not in present:
                raise PlanError(f"{key}: chưa có — dùng 'sinh'")
        if mode.startswith("o"):
            if mode[1] not in slots_filled:
                raise PlanError(f"{key}: ô BI_MAT_{mode[1]} đang trống")
            slot_users.setdefault(mode, []).append(key)
        what = {"sinh": "sinh ngẫu nhiên (CSPRNG)", "xoay": "thêm khoá mới lên đầu, giữ khoá cũ"}.get(
            mode, f"giá trị ô BI_MAT_{mode[1:]}")
        rows.append([key, "có" if key in present else "chưa có", what])
        actions.append((key, mode, key_bytes(key)))
    return rows, actions


def random_key(nbytes: int) -> str:
    return base64.b64encode(secrets.token_bytes(nbytes)).decode("ascii")


def apply_secret(current_data: dict, actions: list, slot_dir: str):
    """→ (patch data base64, backup data base64). Values handled here, never printed."""
    patch, backup = {}, {}
    for key, mode, nbytes in actions:
        old_b64 = current_data.get(key)
        if old_b64 is not None:
            backup[key] = old_b64
        if mode == "sinh":
            value = random_key(nbytes)
        elif mode == "xoay":
            old = base64.b64decode(old_b64).decode("utf-8").strip()
            value = random_key(nbytes) + "," + old
        else:
            with io.open(os.path.join(slot_dir, "slot" + mode[1]), encoding="utf-8") as f:
                value = f.read().strip()
            if not value or any(ord(ch) < 32 for ch in value):
                raise PlanError(f"{key}: giá trị ô BI_MAT_{mode[1]} rỗng hoặc có ký tự điều khiển")
        patch[key] = base64.b64encode(value.encode("utf-8")).decode("ascii")
    return patch, backup


BACKUP_NAME = re.compile(r"^([a-z]+)-secrets-backup-\d{8}-\d{6}$")


def secret_manifest(name: str, data: dict, labels: dict, notes: dict = None) -> dict:
    """A backup Secret. It is attached to NO Deployment (ADR 0089 open question #4): a key added to
    an envFrom'ed Secret would become an env var with no .env.example line (rule 11 forbidden #4)."""
    meta = {"name": name, "labels": labels}
    if notes:
        meta["annotations"] = notes
    return {"apiVersion": "v1", "kind": "Secret", "type": "Opaque", "metadata": meta, "data": data}


# ─────────────────────────────────────────────────────────────────────────────────────────────
# One-shot pods — `kubectl create -f` + poll, never `kubectl run -i` (it hung, deploy/Jenkinsfile:35)
# ─────────────────────────────────────────────────────────────────────────────────────────────

PSQL_IMAGE = "postgres:16-alpine"   # the cluster's PostgreSQL is 16; same image the old job uses


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
    """Pod manifest. Two shapes:
      --script FILE : postgres image, the script travels base64 in an env var (no stdin, no attach)
      --from-deploy F: the service's OWN image, env and pull secrets read from its Deployment, with
                       --secret-env overriding (an explicit `env` entry wins over envFrom)."""
    labels = {"app.kubernetes.io/name": "vigov-ops-oneshot", "app.kubernetes.io/managed-by": "vigov-van-hanh"}
    env, env_from, pull = [], [], [{"name": p} for p in a.pull_secret]
    image, command = PSQL_IMAGE, None
    if a.from_deploy:
        d = _json(a.from_deploy)
        cs = d["spec"]["template"]["spec"]["containers"]
        if len(cs) != 1:
            raise PlanError(f"{d['metadata']['name']} có {len(cs)} container — không biết lấy ảnh nào")
        image = cs[0]["image"]
        env = [e for e in cs[0].get("env", []) if e.get("name") not in {s.split("=", 1)[0] for s in a.secret_env}]
        env_from = cs[0].get("envFrom", [])
        pull = d["spec"]["template"]["spec"].get("imagePullSecrets", []) or pull
        if a.command_json:
            command = json.loads(a.command_json)
    elif a.script:
        script = base64.b64encode(_read(a.script).encode("utf-8")).decode("ascii")
        env.append({"name": "SCRIPT_B64", "value": script})
        command = ["sh", "-c", "echo \"$SCRIPT_B64\" | base64 -d > /tmp/s.sh && sh /tmp/s.sh"]
    else:
        raise PlanError("pod: cần --script hoặc --from-deploy")
    env += [_secret_env(s) for s in a.secret_env]
    for kv in a.env:
        k, _, v = kv.partition("=")
        env.append({"name": k, "value": v})
    env_from += [{"secretRef": {"name": s}} for s in a.envfrom_secret]
    env_from += [{"configMapRef": {"name": c}} for c in a.envfrom_configmap]
    container = {"name": "oneshot", "image": image, "env": env, "envFrom": env_from,
                 "securityContext": {"allowPrivilegeEscalation": False, "capabilities": {"drop": ["ALL"]}},
                 "resources": {"requests": {"cpu": "20m", "memory": "64Mi"}, "limits": {"memory": "256Mi"}}}
    pod_sc = {"runAsNonRoot": True, "runAsUser": 70,    # `postgres` in the alpine image
              "seccompProfile": {"type": "RuntimeDefault"}}
    if a.from_deploy:
        # The service's own settings, not ours: a distroless image whose USER is a NAME fails
        # `runAsNonRoot` unless the Deployment states the uid, and a migration needs the memory
        # the service runs with.
        tpl = d["spec"]["template"]["spec"]
        pod_sc = tpl.get("securityContext", {})
        container["securityContext"] = cs[0].get("securityContext", container["securityContext"])
        container["resources"] = cs[0].get("resources", {})
    if command:
        container["command"] = command
    spec = {"restartPolicy": "Never", "automountServiceAccountToken": False,
            "securityContext": pod_sc, "imagePullSecrets": pull, "containers": [container]}
    # NO Deployment label: carrying one would make the Service send real traffic to this pod.
    return {"apiVersion": "v1", "kind": "Pod", "metadata": {"name": a.name, "labels": labels}, "spec": spec}


# ─────────────────────────────────────────────────────────────────────────────────────────────
# Resources (replicas, requests/limits)
# ─────────────────────────────────────────────────────────────────────────────────────────────

# PROPOSALS ONLY (ADR 0089 open question #9) — the owner picks the numbers.
PRESETS = {
    "nho": {"replicas": 1, "cpu": ("50m", "500m"), "mem": ("128Mi", "256Mi")},
    "vua": {"replicas": 2, "cpu": ("100m", "1000m"), "mem": ("256Mi", "512Mi")},
    "lon": {"replicas": 3, "cpu": ("250m", "2000m"), "mem": ("512Mi", "1Gi")},
}
CPU = re.compile(r"^(\d+)m$|^(\d+(?:\.\d+)?)$")
MEM = re.compile(r"^(\d+)(Mi|Gi)$")


def cpu_milli(q: str) -> int:
    m = CPU.match(q)
    if not m:
        raise PlanError(f"cpu '{q}' không hợp lệ — viết 100m hoặc 1")
    return int(m.group(1)) if m.group(1) else int(float(m.group(2)) * 1000)


def mem_mi(q: str) -> int:
    m = MEM.match(q)
    if not m:
        raise PlanError(f"memory '{q}' không hợp lệ — viết 256Mi hoặc 1Gi")
    return int(m.group(1)) * (1024 if m.group(2) == "Gi" else 1)


def current_resources(deploy_list: dict) -> dict:
    """{deployment: {container, replicas, cpu:(req,lim), mem:(req,lim)}} — single-container only."""
    out = {}
    for d in deploy_list.get("items", []):
        name = d["metadata"]["name"]
        cs = d["spec"]["template"]["spec"]["containers"]
        if len(cs) != 1:
            continue
        r = cs[0].get("resources", {})
        req, lim = r.get("requests", {}), r.get("limits", {})
        out[name] = {"container": cs[0]["name"], "replicas": d["spec"].get("replicas", 1),
                     "cpu": (req.get("cpu", ""), lim.get("cpu", "")),
                     "mem": (req.get("memory", ""), lim.get("memory", ""))}
    return out


def resource_lines(cur: dict) -> str:
    """The editable text the form pre-fills: one deployment per line."""
    return "\n".join(
        f"{n} replicas={v['replicas']} cpu={v['cpu'][0] or '-'}/{v['cpu'][1] or '-'} "
        f"mem={v['mem'][0] or '-'}/{v['mem'][1] or '-'}" for n, v in sorted(cur.items()))


def _pair(field: str, text: str):
    if "/" not in text:
        raise PlanError(f"{field}={text}: viết request/limit, ví dụ {field}=100m/500m")
    a, b = (x.strip() for x in text.split("/", 1))
    # `-` is how resource_lines() writes "not set"; reading it back as empty keeps an untouched
    # pre-filled line equal to the cluster, so it is skipped instead of refused.
    return ("" if a == "-" else a), ("" if b == "-" else b)


def plan_resources(cur: dict, edits_text: str):
    """→ (rows, {deployment: strategic-merge patch}). Lines: `<deploy> [preset=X] [replicas=N]
    [cpu=req/lim] [mem=req/lim]`; a preset is applied first, explicit fields override it."""
    rows, patches = [], {}
    for n, raw in enumerate(edits_text.replace("\r", "").split("\n"), 1):
        s = raw.strip()
        if not s or s.startswith("#"):
            continue
        parts = s.split()
        dep = parts[0]
        if dep not in cur:
            raise PlanError(f"dòng {n}: không có Deployment '{dep}' (một container) trong namespace")
        c = cur[dep]
        want = {"replicas": c["replicas"], "cpu": c["cpu"], "mem": c["mem"]}
        for p in parts[1:]:
            if "=" not in p:
                raise PlanError(f"dòng {n}: '{p}' — viết trường=giá trị")
            f, v = p.split("=", 1)
            if f == "preset":
                if v not in PRESETS:
                    raise PlanError(f"dòng {n}: mẫu '{v}' không có — nho | vua | lon")
                want = dict(PRESETS[v])
            elif f == "replicas":
                if not re.fullmatch(r"\d{1,2}", v) or not 1 <= int(v) <= 10:
                    raise PlanError(f"dòng {n}: replicas phải 1..10 (0 là tắt dịch vụ — không làm qua đây)")
                want["replicas"] = int(v)
            elif f in ("cpu", "mem"):
                want[f] = _pair(f, v)
            else:
                raise PlanError(f"dòng {n}: trường '{f}' không biết — preset | replicas | cpu | mem")
        if (want["replicas"], tuple(want["cpu"]), tuple(want["mem"])) == (c["replicas"], tuple(c["cpu"]), tuple(c["mem"])):
            continue  # untouched line — the form pre-fills every Deployment
        if "" in tuple(want["cpu"]) + tuple(want["mem"]):
            raise PlanError(f"dòng {n}: {dep} thiếu request/limit — điền đủ cả bốn số")
        if cpu_milli(want["cpu"][0]) > cpu_milli(want["cpu"][1]):
            raise PlanError(f"dòng {n}: {dep} cpu request lớn hơn limit")
        if mem_mi(want["mem"][0]) > mem_mi(want["mem"][1]):
            raise PlanError(f"dòng {n}: {dep} memory request lớn hơn limit")
        fmt = lambda x: f"r={x['replicas']} cpu={x['cpu'][0] or '-'}/{x['cpu'][1] or '-'} mem={x['mem'][0] or '-'}/{x['mem'][1] or '-'}"
        rows.append([dep, fmt(c), fmt(want)])
        patches[dep] = {"spec": {"replicas": want["replicas"], "template": {"spec": {"containers": [{
            "name": c["container"],
            "resources": {"requests": {"cpu": want["cpu"][0], "memory": want["mem"][0]},
                          "limits": {"cpu": want["cpu"][1], "memory": want["mem"][1]}}}]}}}}
    return rows, patches


# ─────────────────────────────────────────────────────────────────────────────────────────────
# New databases
# ─────────────────────────────────────────────────────────────────────────────────────────────

SSLMODES = ("require", "verify-ca", "verify-full")   # never the plaintext mode (rule 13 forbidden #1)
IDENT = re.compile(r"^[a-z_][a-z0-9_]{0,62}$")
DB_STEPS = ["tao-csdl", "chay-migration", "kiem-luoc-do", "dat-dsn", "khoi-dong-lai"]


def default_db_name(service: str, mt: str) -> str:
    """Proposal (state it as such): `vigov_<service>_<prod|stg>`. A name distinct from today's
    `vigov_<service>` so the new database can sit on the SAME server as the old one."""
    return f"vigov_{service}_{'prod' if mt == 'prod' else 'stg'}"


def db_lines(services: list, mt: str) -> str:
    return "\n".join(f"{s} {default_db_name(s, mt)} {default_db_name(s, mt)}" for s in services)


def parse_hosts(hosts: str, port: str) -> str:
    """`h1,h2:5433` → `h1:5432,h2:5433` — a list from the first line (rule 11 invariant 5)."""
    if not re.fullmatch(r"\d{1,5}", port or ""):
        raise PlanError("cổng PostgreSQL phải là số")
    out = []
    for h in (hosts or "").split(","):
        h = h.strip()
        if not h:
            continue
        if ":" not in h:
            h = f"{h}:{port}"
        if not HOSTPORT.match(h):
            raise PlanError(f"máy chủ '{h}' không đúng dạng host hoặc host:port")
        out.append(h)
    if not out:
        raise PlanError("chưa nhập máy chủ PostgreSQL")
    return ",".join(out)


def build_dsn(hostlist: str, db: str, user: str, password: str, sslmode: str) -> str:
    if sslmode not in SSLMODES:
        raise PlanError(f"sslmode '{sslmode}' không được phép — require | verify-ca | verify-full")
    if not IDENT.match(db) or not IDENT.match(user):
        raise PlanError("tên CSDL / tài khoản không hợp lệ")
    if not re.fullmatch(r"[A-Za-z0-9]{24,128}", password):
        raise PlanError("mật khẩu sinh ra không đúng dạng — dừng")
    return "postgres://" + user + ":" + password + "@" + hostlist + "/" + db + "?sslmode=" + sslmode


def plan_databases(services: list, edits_text: str, current_dbs: dict):
    """Lines `<service> <database> <user>` → [(service, db, user)]. Refuses a name equal to the
    database the service uses TODAY: on the same server that would be the old database (rule 7)."""
    out, seen_db, seen_user = [], set(), set()
    for n, raw in enumerate(edits_text.replace("\r", "").split("\n"), 1):
        s = raw.strip()
        if not s or s.startswith("#"):
            continue
        parts = s.split()
        if len(parts) != 3:
            raise PlanError(f"dòng {n}: viết <dịch vụ> <tên CSDL> <tài khoản>")
        svc, db, user = parts
        if svc not in services:
            raise PlanError(f"dòng {n}: dịch vụ '{svc}' không có — {', '.join(services)}")
        for x in (db, user):
            if not IDENT.match(x):
                raise PlanError(f"dòng {n}: '{x}' — chỉ chữ thường, số, _ (tối đa 63)")
        if db in seen_db or user in seen_user:
            raise PlanError(f"dòng {n}: tên CSDL/tài khoản trùng một dòng khác")
        if current_dbs.get(svc) == db:
            raise PlanError(f"dòng {n}: '{db}' là CSDL {svc} đang dùng — CSDL cũ không bao giờ bị đụng")
        seen_db.add(db)
        seen_user.add(user)
        out.append((svc, db, user))
    if not out:
        raise PlanError("bảng dịch vụ trống")
    return out


def create_sql(rows: list) -> str:
    """psql script. Names are validated identifiers; passwords arrive as env vars PW_<SERVICE>
    through `\\getenv` (psql 15+) and are quoted by format(%L) — never in the script text."""
    dbs = ", ".join(f"'{d}'" for _, d, _ in rows)
    users = ", ".join(f"'{u}'" for _, _, u in rows)
    out = ["\\set ON_ERROR_STOP on", "\\set VERBOSITY terse",
           "DO $$ BEGIN",
           f"  IF EXISTS (SELECT 1 FROM pg_database WHERE datname IN ({dbs})) THEN",
           "    RAISE EXCEPTION 'DUNG: mot CSDL trong danh sach da ton tai - khong tao gi, khong dung CSDL cu'; END IF;",
           f"  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname IN ({users})) THEN",
           "    RAISE EXCEPTION 'DUNG: mot tai khoan trong danh sach da ton tai - khong tao gi'; END IF;",
           "END $$;"]
    for svc, db, user in rows:
        var = "pw_" + svc
        out += [f"\\getenv {var} PW_{svc.upper()}",
                f"SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', '{user}', :'{var}') \\gexec",
                # PostgreSQL 16: a CREATEROLE admin gets ADMIN on the new role but not SET, and
                # CREATE DATABASE … OWNER needs SET. Harmless when the admin is a superuser.
                f"SELECT format('GRANT %I TO %I', '{user}', current_user) \\gexec",
                f"CREATE DATABASE {db} OWNER {user};",
                f"REVOKE CONNECT ON DATABASE {db} FROM PUBLIC;",
                f"GRANT CONNECT ON DATABASE {db} TO {user};",
                f"\\echo tao {svc}: CSDL {db}, tai khoan {user}"]
    return "\n".join(out) + "\n"


def db_name_of(dsn: str) -> str:
    """Only the database name of a DSN — what the plan may print. '' when unsure."""
    s = dsn.strip().split("?", 1)[0]
    m = re.search(r"dbname=([A-Za-z0-9_]+)", dsn)
    name = m.group(1) if m else s.rsplit("/", 1)[-1] if "://" in s else ""
    return name if re.fullmatch(r"[A-Za-z0-9_]{1,63}", name) else ""


CONSEQUENCES = """HỆ QUẢ CỦA CSDL MỚI TRÊN PROD (ADR 0089 §Hệ quả) — đọc trước khi chạy thật:
  1. CSDL mới RỖNG: không có xã nào → mọi tên miền xã trên prod trả 404 cho tới khi tạo lại xã.
  2. Không có tài khoản vận hành → platform-admin không đăng nhập được cho tới khi tạo lại
     (việc tao-tai-khoan-van-hanh của job này, SAU khi identity đã chạy trên CSDL mới).
  3. Dòng mini_app và App Secret của xã ở lại CSDL cũ — app riêng của xã ngừng đăng nhập công dân.
  4. Mọi hồ sơ đã ghi trên prod ở lại CSDL cũ (bản duy nhất — luật 7) và KHÔNG hiện trên prod.
  5. Staging đang dùng chung CSDL cũ và Ở LẠI đó — job không đổi staging.
  6. vihat-miniapp (CSDL trong vihat-miniapp-bi-mat) không thuộc việc này — giữ nguyên.
  CSDL cũ không bị ghi, không bị xoá. Quay lui: việc khoi-phuc-secret với bản sao lưu DSN."""


# ─────────────────────────────────────────────────────────────────────────────────────────────
# Migrations: image vs database
# ─────────────────────────────────────────────────────────────────────────────────────────────

def image_migrations(repo: str, service: str, commit: str) -> list:
    """[(file, sha256)] of service-<svc>/migrations/*.sql at the image's commit — the same bytes
    the binary embeds, so the same checksum core/migrate records."""
    if not re.fullmatch(r"[0-9a-f]{7,40}", commit):
        raise PlanError(f"thẻ ảnh '{commit}' không phải commit")
    base = f"service-{service}/migrations"
    names = subprocess.run(["git", "-C", repo, "ls-tree", "--name-only", commit, base + "/"],
                           capture_output=True, text=True, check=True).stdout.split()
    out = []
    for p in sorted(n for n in names if n.endswith(".sql")):
        blob = subprocess.run(["git", "-C", repo, "show", f"{commit}:{p}"], capture_output=True, check=True).stdout
        out.append((os.path.basename(p), hashlib.sha256(blob).hexdigest()))
    return out


def migration_diff(image: list, applied: dict):
    """→ (pending, edited, ahead). applied: {file: checksum}."""
    names = {f for f, _ in image}
    pending = [f for f, _ in image if f not in applied]
    edited = [f for f, s in image if f in applied and applied[f] != s]
    ahead = sorted(f for f in applied if f not in names)
    return pending, edited, ahead


def migration_rows(svcs: list, image: dict, applied: dict):
    """→ (table rows, any service core/migrate would REFUSE, every service fully matches).
    A service with no readable side is never a match: "could not read" is not "in sync"."""
    rows, blocking, all_match = [], False, True
    for s in svcs:
        img, app = image.get(s), applied.get(s)
        if not img or app is None:
            all_match = False
            rows.append([s, img[-1][0] if img else "(không đọc được)",
                         max(app) if app else "(không đọc được / trống)", "?", ""])
            continue
        pending, edited, ahead = migration_diff(img, app)
        note = []
        if pending:
            note.append("CHƯA ÁP: " + " ".join(pending))
        if edited:
            note.append("TỆP ĐÃ ÁP BỊ SỬA: " + " ".join(edited))
        if ahead:
            note.append("CSDL MỚI HƠN ẢNH: " + " ".join(ahead))
        blocking = blocking or bool(edited or ahead)
        all_match = all_match and not note
        rows.append([s, img[-1][0], max(app) if app else "(trống)", "khớp" if not note else "LỆCH", "; ".join(note)])
    return rows, blocking, all_match


def ordered_services(services: list) -> list:
    """platform → identity → the rest: the order deploy/README.md mục 2 gives for bringing services up
    (every service resolves its commune through platform, and its staff through identity)."""
    head = [s for s in ("platform", "identity") if s in services]
    return head + [s for s in sorted(services) if s not in head]


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

    c = sub.add_parser("config-plan")
    c.add_argument("--current", required=True)
    c.add_argument("--edits", required=True)
    c.add_argument("--env", required=True)
    c.add_argument("--patch-out", required=True)
    c.add_argument("--restart-out", required=True)

    c = sub.add_parser("config-audit")
    c.add_argument("--current", required=True)

    c = sub.add_parser("secret-plan")
    c.add_argument("--service", required=True)
    c.add_argument("--present", required=True)
    c.add_argument("--edits", required=True)
    c.add_argument("--slots", default="")
    c.add_argument("--actions-out", required=True)

    c = sub.add_parser("secret-apply")
    c.add_argument("--current", required=True)
    c.add_argument("--actions", required=True)
    c.add_argument("--slot-dir", required=True)
    c.add_argument("--patch-out", required=True)
    c.add_argument("--backup-out", required=True)
    c.add_argument("--backup-name", required=True)
    c.add_argument("--labels", default="")
    c.add_argument("--note", action="append", default=[])

    c = sub.add_parser("secret-copy")   # backup Secret → patch for <svc>-secrets (+ backup of current)
    c.add_argument("--from", dest="src", required=True)
    c.add_argument("--current", required=True)
    c.add_argument("--patch-out", required=True)
    c.add_argument("--backup-out", required=True)
    c.add_argument("--backup-name", required=True)
    c.add_argument("--labels", default="")
    c.add_argument("--note", action="append", default=[])

    c = sub.add_parser("resources-lines")
    c.add_argument("--current", required=True)

    c = sub.add_parser("resources-plan")
    c.add_argument("--current", required=True)
    c.add_argument("--edits", required=True)
    c.add_argument("--out-dir", required=True)

    c = sub.add_parser("db-plan")
    c.add_argument("--mt", required=True)
    c.add_argument("--edits", required=True)
    c.add_argument("--current-dbs", required=True)
    c.add_argument("--hosts", required=True)
    c.add_argument("--port", required=True)
    c.add_argument("--sslmode", required=True)
    c.add_argument("--rows-out", required=True)
    c.add_argument("--sql-out", required=True)

    c = sub.add_parser("dsn")
    c.add_argument("--hosts", required=True)
    c.add_argument("--port", required=True)
    c.add_argument("--db", required=True)
    c.add_argument("--user", required=True)
    c.add_argument("--sslmode", required=True)
    c.add_argument("--password-file", required=True)
    c.add_argument("--out", required=True)

    c = sub.add_parser("db-name")      # stdin: one DSN → stdout: its database name only
    c = sub.add_parser("image-migrations")
    c.add_argument("--service", required=True)
    c.add_argument("--commit", required=True)

    c = sub.add_parser("migration-report")
    c.add_argument("--image", required=True)     # TSV svc file sha
    c.add_argument("--applied", required=True)   # TSV svc file checksum
    c.add_argument("--strict", action="store_true")   # rc 1 unless every service matches its image

    sub.add_parser("services")

    sub.add_parser("consequences")

    c = sub.add_parser("secret-verify")
    c.add_argument("--current", required=True)
    c.add_argument("--patch", required=True)

    c = sub.add_parser("pg-env")
    c.add_argument("--hosts", required=True)
    c.add_argument("--port", required=True)

    c = sub.add_parser("pod")
    c.add_argument("--name", required=True)
    c.add_argument("--out", required=True)
    c.add_argument("--script")
    c.add_argument("--from-deploy")
    c.add_argument("--command-json")
    c.add_argument("--secret-env", action="append", default=[])
    c.add_argument("--env", action="append", default=[])
    c.add_argument("--envfrom-secret", action="append", default=[])
    c.add_argument("--envfrom-configmap", action="append", default=[])
    c.add_argument("--pull-secret", action="append", default=[])

    a = ap.parse_args(argv)
    try:
        return _run(a)
    except PlanError as e:
        print(f"DỪNG: {e}. Không ghi gì.")
        return 2


def _labels(text: str) -> dict:
    out = {"app.kubernetes.io/managed-by": "vigov-van-hanh"}
    for kv in filter(None, text.split(",")):
        k, _, v = kv.partition("=")
        if re.fullmatch(r"[A-Za-z0-9._/-]{1,63}", k) and re.fullmatch(r"[A-Za-z0-9._-]{0,63}", v):
            out[k] = v
    return out


def _notes(items: list) -> dict:
    """Backup annotations: who · ticket · build. Free text, but bounded and single-line."""
    out = {}
    for kv in items:
        k, _, v = kv.partition("=")
        if re.fullmatch(r"vigov\.vn/[a-z-]{1,40}", k):
            out[k] = "".join(ch if ord(ch) >= 32 else " " for ch in v)[:200]
    return out


def _run(a) -> int:
    if a.cmd == "catalog":
        cat = build_catalog()
        text = catalog_text(cat)
        if a.write or a.check:
            jf = _read(JENKINSFILE)
            new = render_jenkinsfile(jf, generated_region(cat, sorted(service_groups())))
            if a.check:
                if new != jf:
                    print("[FAIL] deploy/van-hanh/Jenkinsfile: danh mục khoá (opsCatalog) cũ so với "
                          "core/config + deploy/cau-hinh/README.md + .env.example.\n"
                          "       chạy: python deploy/van-hanh/ops.py catalog --write")
                    return 1
                print(f"[PASS] danh mục khoá của job vận hành khớp mã — {text.count(chr(10)) + 1} khoá")
                return 0
            with io.open(JENKINSFILE, "w", encoding="utf-8", newline="\n") as f:
                f.write(new)
            print("đã ghi opsCatalog() vào deploy/van-hanh/Jenkinsfile")
            return 0
        print(text)
        return 0

    if a.cmd == "consequences":
        print(CONSEQUENCES)
        return 0

    if a.cmd == "config-plan":
        cat = build_catalog()
        current = _json(a.current).get("data", {}) or {}
        rows, patch, restart = plan_config(cat, current, _read(a.edits), a.env)
        if not rows:
            print("Không có khoá nào đổi — không có gì để ghi.")
            return 3
        print(table(["KHOÁ", "CŨ", "MỚI", "DỊCH VỤ ĐỌC"], rows))
        print("\nKhởi động lại (chỉ dịch vụ đọc khoá vừa đổi): " + (", ".join(restart) or "(không)"))
        _write_private(a.patch_out, json.dumps(patch, ensure_ascii=False))
        _write_private(a.restart_out, "\n".join(restart) + ("\n" if restart else ""))
        return 0

    if a.cmd == "config-audit":
        missing, extra = audit_config(build_catalog(), _json(a.current).get("data", {}) or {})
        print("── khoá common-config có trong sổ (.env.example + README mục 2) nhưng THIẾU trên cụm")
        print(table(["KHOÁ", "BẮT BUỘC", "DỊCH VỤ ĐỌC"], missing) if missing else "   (không thiếu khoá nào)")
        print("── khoá trên cụm KHÔNG có trong sổ (không dịch vụ nào đọc qua core/config)")
        print("   " + (", ".join(extra) if extra else "(không có)"))
        return 0

    if a.cmd == "secret-plan":
        cat = build_catalog()
        present = {ln.strip() for ln in _read(a.present).splitlines() if ln.strip()}
        rows, actions = plan_secret(cat, a.service, present, _read(a.edits), set(filter(None, a.slots.split(","))))
        if not rows:
            print("Không có khoá nào đổi — không có gì để ghi.")
            return 3
        print(table(["KHOÁ", "HIỆN", "SẼ ĐẶT"], rows))
        _write_private(a.actions_out, "\n".join(f"{k}\t{m}\t{n}" for k, m, n in actions) + "\n")
        return 0

    if a.cmd == "secret-apply":
        cur = _json(a.current).get("data", {}) or {}
        actions = []
        for ln in _read(a.actions).splitlines():
            if ln.strip():
                k, m, n = ln.split("\t")
                actions.append((k, m, int(n)))
        patch, backup = apply_secret(cur, actions, a.slot_dir)
        _write_private(a.patch_out, json.dumps({"data": patch}))
        if backup:
            _write_private(a.backup_out, json.dumps(secret_manifest(a.backup_name, backup, _labels(a.labels), _notes(a.note))))
            print(f"sao lưu {len(backup)} khoá cũ vào {a.backup_name}: {', '.join(sorted(backup))}")
        else:
            print("không có giá trị cũ nào để sao lưu (mọi khoá đều mới)")
        return 0

    if a.cmd == "secret-verify":
        cur = _json(a.current).get("data", {}) or {}
        want = _json(a.patch).get("data", {}) or {}
        off = sorted(k for k, v in want.items() if cur.get(k) != v)
        if off:
            print("✗ đọc lại từ cụm, khoá KHÔNG khớp giá trị vừa ghi: " + ", ".join(off) + " (không in giá trị)")
            return 1
        print(f"✓ đọc lại từ cụm khớp {len(want)} khoá: {', '.join(sorted(want))} (so trên tệp, không in giá trị)")
        return 0

    if a.cmd == "pg-env":
        hosts = parse_hosts(a.hosts, a.port).split(",")
        print("PGHOST=" + ",".join(h.rsplit(":", 1)[0] for h in hosts))
        print("PGPORT=" + ",".join(h.rsplit(":", 1)[1] for h in hosts))
        return 0

    if a.cmd == "pod":
        _write_private(a.out, json.dumps(build_pod(a)))
        return 0

    if a.cmd == "secret-copy":
        src = _json(a.src).get("data", {}) or {}
        cur = _json(a.current).get("data", {}) or {}
        if not src:
            raise PlanError("bản sao lưu không có khoá nào")
        backup = {k: cur[k] for k in src if k in cur}
        _write_private(a.patch_out, json.dumps({"data": src}))
        if backup:
            _write_private(a.backup_out, json.dumps(secret_manifest(a.backup_name, backup, _labels(a.labels), _notes(a.note))))
        print(f"khôi phục {len(src)} khoá: {', '.join(sorted(src))} · giá trị hiện tại sao lưu vào {a.backup_name}")
        return 0

    if a.cmd == "resources-lines":
        print(resource_lines(current_resources(_json(a.current))))
        return 0

    if a.cmd == "resources-plan":
        rows, patches = plan_resources(current_resources(_json(a.current)), _read(a.edits))
        if not rows:
            print("Không có Deployment nào đổi — không có gì để ghi.")
            return 3
        print(table(["DEPLOYMENT", "CŨ", "MỚI"], rows))
        os.makedirs(a.out_dir, exist_ok=True)
        for dep, p in patches.items():
            _write_private(os.path.join(a.out_dir, dep + ".json"), json.dumps(p))
        return 0

    if a.cmd == "db-plan":
        services = sorted(service_groups())
        cur = {}
        for ln in _read(a.current_dbs).splitlines():
            if "\t" in ln:
                k, v = ln.split("\t", 1)
                cur[k.strip()] = v.strip()
        hosts = parse_hosts(a.hosts, a.port)
        if a.sslmode not in SSLMODES:
            raise PlanError(f"sslmode '{a.sslmode}' không được phép")
        rows = plan_databases(services, _read(a.edits), cur)
        print(f"Máy chủ: {hosts} · sslmode={a.sslmode}")
        print(table(["DỊCH VỤ", "CSDL ĐANG DÙNG", "CSDL MỚI", "TÀI KHOẢN MỚI"],
                    [[s, cur.get(s) or "(không đọc được)", d, u] for s, d, u in rows]))
        _write_private(a.rows_out, "\n".join(f"{s}\t{d}\t{u}" for s, d, u in rows) + "\n")
        _write_private(a.sql_out, create_sql(rows))
        return 0

    if a.cmd == "dsn":
        with io.open(a.password_file, encoding="utf-8") as f:
            pw = f.read().strip()
        _write_private(a.out, build_dsn(parse_hosts(a.hosts, a.port), a.db, a.user, pw, a.sslmode))
        return 0

    if a.cmd == "db-name":
        print(db_name_of(sys.stdin.read()))
        return 0

    if a.cmd == "image-migrations":
        for f, s in image_migrations(ROOT, a.service, a.commit):
            print(f"{a.service}\t{f}\t{s}")
        return 0

    if a.cmd == "migration-report":
        image, applied, svcs = {}, {}, []
        for ln in _read(a.image).splitlines():
            p = ln.split("\t")
            if len(p) == 3:
                image.setdefault(p[0], []).append((p[1], p[2]))
                if p[0] not in svcs:
                    svcs.append(p[0])
        for ln in _read(a.applied).splitlines():
            p = ln.split("\t")
            if len(p) == 3:
                applied.setdefault(p[0], {})[p[1]] = p[2]
                if p[0] not in svcs:
                    svcs.append(p[0])
        rows, blocking, all_match = migration_rows(svcs, image, applied)
        print(table(["DỊCH VỤ", "MỚI NHẤT TRONG ẢNH", "MỚI NHẤT ĐÃ ÁP", "", "CHÊNH"], rows))
        if blocking:
            print("\n⚠ Tệp đã áp bị sửa / CSDL mới hơn ảnh: dịch vụ sẽ TỪ CHỐI khởi động (core/migrate). "
                  "Khởi động lại không chữa được — cần người xem.")
        if a.strict and not all_match:
            print("✗ chưa khớp: mọi dịch vụ phải có đủ migration của ảnh, không tệp sửa, không tệp lạ.")
            return 1
        return 0

    if a.cmd == "services":
        print(" ".join(ordered_services(sorted(service_groups()))))
        return 0

    raise PlanError(f"lệnh '{a.cmd}' không biết")


if __name__ == "__main__":
    sys.exit(main())
