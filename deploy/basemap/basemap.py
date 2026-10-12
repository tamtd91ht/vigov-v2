#!/usr/bin/env python3
"""Decisions behind the task `nen-ban-do` of the Jenkins job `vigov-deploy` (deploy/Jenkinsfile), run
through deploy/basemap/basemap-task.sh. Why: ADR 0072. Procedure: kb/40-runbooks/basemap-pmtiles.md.

WHY A PYTHON FILE: which bucket, which endpoint, which pinned checksum, which Secret holds the MinIO
keys — each is easy to get silently wrong in shell. Here they are plain functions with unit tests
(deploy/basemap/test_basemap.py, run by `make buildfiles`); the shell only moves files.

SECRET VALUES: `mc-host` reads the keys from FILES and writes the MC_HOST_ value to a FILE (mode 0600);
nothing here prints a key, not even a prefix.

Python 3.8+: the build machine is CentOS 7, `python3` there is 3.6 and the task picks the newest
`python3.X` it finds.
"""

from __future__ import annotations

import argparse
import datetime
import io
import json
import os
import re
import sys
from urllib.parse import quote


class PlanError(Exception):
    """A refusal the operator must read. Message is Vietnamese; it never carries a secret."""


def table(headers: list, rows: list) -> str:
    cols = [list(map(str, c)) for c in zip(headers, *rows)] if rows else [[h] for h in headers]
    w = [max(len(x) for x in c) for c in cols]
    line = lambda r: "  ".join(str(x).ljust(w[i]) for i, x in enumerate(r)).rstrip()
    sep = "  ".join("─" * x for x in w)
    return "\n".join([line(headers), sep] + [line(r) for r in rows])


# ─────────────────────────────────────────────────────────────────────────────────────────────
# Self-hosted basemap (task nen-ban-do) — procedure: kb/40-runbooks/basemap-pmtiles.md, why: ADR 0072.
# The job builds on the BUILD MACHINE and uploads to the environment's MinIO; it never changes
# BASEMAP_URL — that waits for the signed sovereignty check.
# ─────────────────────────────────────────────────────────────────────────────────────────────

BASEMAP_DATE = re.compile(r"^20[0-9]{6}$")
# Overture release id, e.g. 2026-09-23.1 (date of the release + revision). It reaches an S3 path and the
# DuckDB SQL of build-basemap.sh, so this pattern is all that may.
OVERTURE_RELEASE = re.compile(r"^20[0-9]{2}-[0-9]{2}-[0-9]{2}\.[0-9]+$")
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

# PINNED IMAGES of the places archive (owner 12/10/2026): DuckDB cuts Overture, tippecanoe tiles it. The
# build machine (CentOS 7) can run neither natively, so both run in Docker, pulled BY DIGEST — a tag is
# whatever the registry says that day. Each digest was read from the registry itself (registry-1.docker.io,
# Docker-Content-Digest of the multi-arch index) on 12/10/2026 and matches Docker Hub's tag page.
# basemap_images() REFUSES while one is UNPINNED_IMAGE. Changing a tag means changing its digest in the same edit.
UNPINNED_IMAGE = "UNPINNED-copy-the-index-digest-from-the-official-registry"
# DuckDB Labs' own image (gcr.io/distroless/cc-debian12 base, runs /duckdb). 1.5.5 rather than 1.5.6:
# 1.5.6 was 13 days old on 12/10/2026; a patch release that has been out for months is the cheaper bet.
DUCKDB_TAG = "1.5.5"
DUCKDB_DIGEST = "sha256:d17f30055ff2eeb7f45c7ac2b7e574542b91cfc7f41b73c049b99e40f5dc5b73"
# felt/tippecanoe publishes no image and no binary: deploy/basemap/tippecanoe.Dockerfile compiles it from
# TIPPECANOE_COMMIT on this base. 22.04 = the base of felt's own Dockerfile at that commit.
UBUNTU_TAG = "22.04"
UBUNTU_DIGEST = "sha256:5ec03bb3441e8b0bf3b4f9cd4629a1ae763010dc3035bb8da3ae6cf026486401"
TIPPECANOE_VERSION = "2.79.0"       # latest GitHub release of felt/tippecanoe on 12/10/2026 (>= 2.17 writes .pmtiles)
TIPPECANOE_COMMIT = "68ab8dcc229f95b8b25877697d5e8d66783af503"   # refs/tags/2.79.0
BASEMAP_IMAGES = [
    {"name": "duckdb", "repo": "docker.io/duckdb/duckdb", "tag": DUCKDB_TAG, "digest": DUCKDB_DIGEST,
     "where": f"https://hub.docker.com/r/duckdb/duckdb/tags?name={DUCKDB_TAG} (Index digest) hoặc "
              f"`docker buildx imagetools inspect duckdb/duckdb:{DUCKDB_TAG}` → dòng Digest → hằng DUCKDB_DIGEST"},
    {"name": "tippecanoe-base", "repo": "docker.io/library/ubuntu", "tag": UBUNTU_TAG, "digest": UBUNTU_DIGEST,
     "where": f"https://hub.docker.com/_/ubuntu/tags?name={UBUNTU_TAG} (Index digest) hoặc "
              f"`docker buildx imagetools inspect ubuntu:{UBUNTU_TAG}` → dòng Digest → hằng UBUNTU_DIGEST"},
]
TIPPECANOE_WHERE = (f"https://github.com/felt/tippecanoe/releases/tag/{TIPPECANOE_VERSION} → mã commit đầy đủ "
                    f"(40 ký tự) → hằng TIPPECANOE_COMMIT")

# Where the existing MinIO key pair is read from (owner 11/10/2026: no new MinIO user, no Jenkins
# credential). comms first — it already writes the public bucket (deploy/cau-hinh/README.md mục 1,
# OBJECT_STORAGE_ACCESS_KEY row); platform second, for the same reason. Key names as envFrom reads them.
BASEMAP_KEY_SOURCES = ["comms-secrets", "platform-secrets"]
STORAGE_KEY_NAMES = ("OBJECT_STORAGE_ACCESS_KEY", "OBJECT_STORAGE_SECRET_KEY")
# Object prefix inside the public bucket (owner 11/10/2026). The dated subfolder stays: a new date is the
# safe refresh (a live prefix is read in byte ranges); the same date overwrites, with a warning (owner,
# 12/10/2026 — basemap-task.sh).
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


def check_overture_release(release: str) -> str:
    r = (release or "").strip()
    if not OVERTURE_RELEASE.match(r):
        raise PlanError(f"BAN_OVERTURE '{r[:20]}' phải là mã bản phát hành Overture dạng 2026-09-23.1")
    try:
        datetime.datetime.strptime(r.split(".")[0], "%Y-%m-%d")
    except ValueError:
        raise PlanError(f"BAN_OVERTURE '{r}': phần ngày không phải một ngày có thật")
    return r


def https_origin(key: str, value: str) -> str:
    """`https://host[:port]` → itself without a trailing slash. http is REFUSED (rule 13)."""
    v = (value or "").strip()
    if v.startswith("http://"):
        raise PlanError(f"{key} là http:// — luật 13: khoá MinIO không bao giờ đi qua kênh không mã hoá")
    m = HTTPS_ORIGIN.match(v)
    if not m:
        raise PlanError(f"{key}: phải là https://host[:cổng], không đường dẫn, không thông tin đăng nhập")
    return "https://" + m.group(1)


def plan_basemap(cm: dict, date: str, overture: str) -> dict:
    """common-config data + date + Overture release → where the files go and the BASEMAP_URL they will be read at."""
    d = check_basemap_date(date)
    rel = check_overture_release(overture)
    for k in ("OBJECT_STORAGE_BUCKET_PREFIX", "OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_ENDPOINT"):
        if not (cm.get(k) or "").strip():
            raise PlanError(f"common-config thiếu {k} — đặt khoá ấy trong common-config trước (deploy/cau-hinh/README.md mục 2)")
    prefix = cm["OBJECT_STORAGE_BUCKET_PREFIX"].strip()
    if not BUCKET_PREFIX.match(prefix):
        raise PlanError(f"OBJECT_STORAGE_BUCKET_PREFIX '{prefix[:60]}' không làm thành tên bucket hợp lệ")
    bucket = prefix + "-public"
    # Upload from the BUILD MACHINE: the internal endpoint is cluster DNS it cannot resolve.
    upload = https_origin("OBJECT_STORAGE_PUBLIC_ENDPOINT", cm["OBJECT_STORAGE_PUBLIC_ENDPOINT"])
    # Read by the web-admin POD: the internal endpoint (netpol allow-web-admin-egress-storage).
    internal = https_origin("OBJECT_STORAGE_ENDPOINT", cm["OBJECT_STORAGE_ENDPOINT"])
    return {"date": d, "overture": rel, "bucket": bucket, "upload_endpoint": upload,
            "dest": f"{bucket}/{BASEMAP_PREFIX}/{d}/", "basemap_url": f"{internal}/{bucket}/{BASEMAP_PREFIX}/{d}"}


def pick_key_source(present: dict) -> str:
    """{secret name: set of key names} → the first Secret in BASEMAP_KEY_SOURCES holding BOTH keys."""
    for s in BASEMAP_KEY_SOURCES:
        if set(STORAGE_KEY_NAMES) <= present.get(s, set()):
            return s
    raise PlanError("không Secret nào trong " + ", ".join(BASEMAP_KEY_SOURCES) + " có đủ "
                    + " + ".join(STORAGE_KEY_NAMES) + " — đặt cặp khoá MinIO của comms (deploy/cau-hinh/README.md mục 1) rồi bấm lại")


def basemap_tools(tools: list = None) -> list:
    """[(name, url, sha256, kind)] — refuses while any checksum is not a real SHA-256."""
    tools = BASEMAP_TOOLS if tools is None else tools
    missing = [t for t in tools if not re.fullmatch(r"[0-9a-f]{64}", t["sha256"])]
    if missing:
        raise PlanError("chưa ghim SHA-256 cho " + ", ".join(t["name"] for t in missing)
                        + " — job không chạy công cụ tải về mà không kiểm. Lấy giá trị ở trang phát hành CHÍNH THỨC: "
                        + " · ".join(t["where"] for t in missing)
                        + "; sửa trong deploy/basemap/basemap.py, commit, chạy lại (deploy/README.md §nen-ban-do)")
    return [(t["name"], t["url"], t["sha256"], t["kind"]) for t in tools]


def basemap_images(images: list = None, commit: str = None) -> dict:
    """{DUCKDB_IMAGE, TIPPECANOE_BASE, TIPPECANOE_COMMIT, TIPPECANOE_TAG} — refuses while any digest or the
    commit is not real. The refs carry NO tag: `repo@sha256:…` is the only form docker cannot re-resolve."""
    images = BASEMAP_IMAGES if images is None else images
    commit = TIPPECANOE_COMMIT if commit is None else commit
    missing = [i["name"] + ": " + i["where"] for i in images if not re.fullmatch(r"sha256:[0-9a-f]{64}", i["digest"])]
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        missing.append("tippecanoe: " + TIPPECANOE_WHERE)
    if missing:
        raise PlanError("chưa ghim ảnh/commit cho " + " · ".join(missing)
                        + " — job không chạy ảnh Docker theo tag. Sửa trong deploy/basemap/basemap.py, commit, chạy lại")
    ref = {i["name"]: f"{i['repo']}@{i['digest']}" for i in images}
    return {"DUCKDB_IMAGE": ref["duckdb"], "TIPPECANOE_BASE": ref["tippecanoe-base"],
            "TIPPECANOE_COMMIT": commit,
            # Local tag of the image this job builds; version + commit make a stale one visible in the log.
            "TIPPECANOE_TAG": f"vigov-basemap/tippecanoe:{TIPPECANOE_VERSION}-{commit[:12]}"}


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

def _read(path: str) -> str:
    with io.open(path, encoding="utf-8") as f:
        return f.read()


def _write_private(path: str, text: str) -> None:
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        f.write(text)


def main(argv=None) -> int:
    for s in (sys.stdout, sys.stderr):
        try:
            s.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass
    ap = argparse.ArgumentParser(prog="basemap.py")
    sub = ap.add_subparsers(dest="cmd", required=True)

    c = sub.add_parser("plan")
    c.add_argument("--current", required=True)    # kubectl get configmap common-config -o json
    c.add_argument("--date", required=True)
    c.add_argument("--overture", required=True)
    c.add_argument("--out", required=True)

    c = sub.add_parser("tools")
    c.add_argument("--out", required=True)

    c = sub.add_parser("images")       # KEY=value file of the pinned image refs + tippecanoe commit
    c.add_argument("--out", required=True)

    c = sub.add_parser("key-source")   # TSV `<secret>\t<key name>` (names only) → Secret name
    c.add_argument("--present", required=True)

    c = sub.add_parser("mc-host")      # key FILES in, MC_HOST_ value FILE out — nothing printed
    c.add_argument("--endpoint", required=True)
    c.add_argument("--access-key-file", required=True)
    c.add_argument("--secret-key-file", required=True)
    c.add_argument("--out", required=True)

    a = ap.parse_args(argv)
    try:
        return _run(a)
    except PlanError as e:
        print(f"DỪNG: {e}. Không ghi gì.")
        return 2


def _run(a) -> int:
    if a.cmd == "plan":
        p = plan_basemap(json.loads(_read(a.current)).get("data", {}) or {}, a.date, a.overture)
        print(table(["", ""], [["Bản dựng Protomaps", p["date"]], ["Bản Overture (địa điểm)", p["overture"]],
                               ["Bucket", p["bucket"]],
                               ["Tải lên qua (máy build)", p["upload_endpoint"]],
                               ["Đích", p["dest"]], ["BASEMAP_URL sẽ là", p["basemap_url"]]]))
        _write_private(a.out, f"BUCKET={p['bucket']}\nUPLOAD_ENDPOINT={p['upload_endpoint']}\n"
                              f"BASEMAP_URL={p['basemap_url']}\nOVERTURE={p['overture']}\n")
        return 0
    if a.cmd == "images":
        imgs = basemap_images()
        _write_private(a.out, "".join(f"{k}={v}\n" for k, v in imgs.items()))
        print("ảnh đã ghim: " + " · ".join(f"{k} …{v[-12:]}" for k, v in imgs.items() if k != "TIPPECANOE_TAG"))
        return 0
    if a.cmd == "key-source":
        present = {}
        for ln in _read(a.present).splitlines():
            s, _, k = ln.partition("\t")
            if s.strip() and k.strip():
                present.setdefault(s.strip(), set()).add(k.strip())
        print(pick_key_source(present))
        return 0
    if a.cmd == "tools":
        tools = basemap_tools()
        _write_private(a.out, "".join("\t".join(t) + "\n" for t in tools))
        print("công cụ đã ghim: " + " · ".join(f"{n} {s[:12]}…" for n, _, s, _ in tools))
        return 0
    if a.cmd == "mc-host":
        ak = _read(a.access_key_file).strip()
        sk = _read(a.secret_key_file).strip()
        _write_private(a.out, mc_host_url(a.endpoint, ak, sk))
        return 0
    raise PlanError(f"lệnh '{a.cmd}' không biết")


if __name__ == "__main__":
    sys.exit(main())
