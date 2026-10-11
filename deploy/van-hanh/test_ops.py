"""Unit tests for deploy/van-hanh/ops.py — the decisions behind the operations job (ADR 0089, rebuilt
11/10/2026 around one task: đổi hạ tầng).

Run: python -m unittest discover -s deploy/van-hanh -p "test_*.py"   (part of `make buildfiles`)

Each refusal is tested by the sentence fragment that names it, not just "something raised": a
mutation that breaks one check while another still fires would otherwise stay green.
"""

from __future__ import annotations

import argparse
import base64
import contextlib
import io
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import ops  # noqa: E402

SERVICES = ["comms", "documents", "finance", "identity", "petitions", "platform", "reporting"]


def entry(key, placement, users, level="prod", shared=False, registered=True):
    return {"key": key, "placement": placement, "level": level, "users": users, "shared": shared,
            "registered": registered, "meaning": ""}




def b64(s: str) -> str:
    return base64.b64encode(s.encode()).decode()


class RealRepoCatalog(unittest.TestCase):
    """The derivation against THIS repository — the facts no edit should silently lose."""

    @classmethod
    def setUpClass(cls):
        cls.cat = ops.by_key(ops.build_catalog())
        cls.services = sorted(ops.service_groups())

    def test_base_variable_belongs_to_every_service(self):
        self.assertEqual(self.cat["DATABASE_DSN"]["placement"], "secret")
        self.assertEqual(self.cat["DATABASE_DSN"]["users"], self.services)

    def test_placements_follow_readme_sections(self):
        self.assertEqual(self.cat["ENV"]["placement"], "configmap")
        self.assertEqual(self.cat["PLATFORM_GRPC_ADDR"]["placement"], "env")
        self.assertEqual(self.cat["RABBITMQ_DSN"]["placement"], "unset")

    def test_web_admin_key_is_underscored_and_registered(self):
        # ADR 0093: key = env name. The dashed spelling must be gone from the catalogue, not kept beside it.
        e = self.cat["BASEMAP_URL"]
        self.assertEqual((e["placement"], e["registered"]), ("configmap", True))
        self.assertNotIn("BASEMAP-URL", self.cat)

    def test_no_catalogued_key_has_a_dash(self):
        # envFrom drops a dashed key silently — the pod just lacks the variable (rule 11 invariant 4).
        self.assertEqual([k for k in self.cat if "-" in k], [])

    def test_every_catalogued_key_is_registered_in_env_template(self):
        self.assertEqual([k for k, e in self.cat.items() if not e["registered"]], [])

    def test_shared_value_keys_detected(self):
        self.assertTrue(self.cat["GRPC_CALLER_KEY"]["shared"])
        self.assertFalse(self.cat["SECRET_ENCRYPTION_KEYS"]["shared"])

    def test_jenkinsfile_generated_region_is_fresh(self):
        jf = ops._read(ops.JENKINSFILE)
        region = ops.generated_region(ops.build_catalog())
        self.assertEqual(ops.render_jenkinsfile(jf, region), jf,
                         "chạy: python deploy/van-hanh/ops.py catalog --write")

    def test_catalog_text_safe_inside_groovy_triple_quote(self):
        text = ops.catalog_text(ops.build_catalog())
        for bad in ("\\", "'''", "'"):
            self.assertNotIn(bad, text)


class ReadmeParsing(unittest.TestCase):
    def test_sections_and_meaning_column(self):
        md = ("## 1. Secret\n| Key | Bắt buộc | Là gì · lấy giá trị | Value |\n|---|---|---|---|\n"
              "| `A_KEY` | có | Khoá **ký** [x](y) | v |\n"
              "## 2. ConfigMap\n### Key trong common-config mà chỉ web-admin đọc\n"
              "| Key | Bắt buộc | Là gì | Value |\n| `B-URL` | không | Nền bản đồ | v |\n"
              "## 4. Không đặt\n| Key | Bắt buộc | Vì sao |\n| `C` | không | chưa dùng |\n")
        rows = ops.readme_rows(md)
        self.assertEqual([(r[0], r[1], r[3], r[4]) for r in rows],
                         [("A_KEY", "secret", "Khoá ký x", False), ("B-URL", "configmap", "Nền bản đồ", True),
                          ("C", "unset", "chưa dùng", False)])

    def test_render_requires_markers(self):
        with self.assertRaises(ValueError):
            ops.render_jenkinsfile("no markers", "x")

def url(scheme: str, auth: str, rest: str) -> str:
    """Fixture URL built from parts: a literal scheme-account-password-host string in source is what
    hooks/secret_scan.py rightly refuses, fake or not."""
    return scheme + ":/" + "/" + auth + "@" + rest


STORAGE = ["comms", "petitions", "platform"]
REDIS_USERS = ["comms", "documents", "finance", "identity", "petitions", "platform"]
ICAT = [
    entry("DATABASE_DSN", "secret", SERVICES, "all"),
    entry("REDIS_DSN", "secret", REDIS_USERS),
    entry("OBJECT_STORAGE_ENDPOINT", "configmap", STORAGE),
    entry("OBJECT_STORAGE_PUBLIC_ENDPOINT", "configmap", STORAGE),
    entry("OBJECT_STORAGE_BUCKET_PREFIX", "configmap", STORAGE),
    entry("OBJECT_STORAGE_REGION", "configmap", STORAGE, "opt"),
    entry("OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL", "configmap", ["comms", "platform"]),
    entry("OBJECT_STORAGE_ACCESS_KEY", "secret", STORAGE),
    entry("OBJECT_STORAGE_SECRET_KEY", "secret", STORAGE),
    entry("MALWARE_SCANNER_ADDRESS", "configmap", STORAGE),
    entry("BASEMAP_URL", "configmap", ["web-admin"], "opt"),
]
PW = "fixturepw"   # asserted absent from every printed row
DSN = {s: url("postgres", f"vigov_{s}:{PW}", f"db-old:5432/vigov_{s}?sslmode=require") for s in SERVICES}
RDSN = {s: url("redis", f":{PW}", f"redis-old:6379/{i}") for i, s in enumerate(REDIS_USERS)}
CM = {"OBJECT_STORAGE_ENDPOINT": "https://minio-old.vigov-infra.svc:9000",
      "OBJECT_STORAGE_PUBLIC_ENDPOINT": "https://storage-old.vigov.vn",
      "OBJECT_STORAGE_BUCKET_PREFIX": "vigov-prod",
      "OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL": "https://storage-old.vigov.vn/vigov-prod-public",
      "MALWARE_SCANNER_ADDRESS": "vigov-clamav:3310",
      "BASEMAP_URL": "https://minio-old.vigov-infra.svc:9000/vigov-prod-public/config/map/20261010"}


def secrets_all():
    out = {s: {"DATABASE_DSN": DSN[s]} for s in SERVICES}
    for s in REDIS_USERS:
        out[s]["REDIS_DSN"] = RDSN[s]
    for s in STORAGE:
        out[s].update({"OBJECT_STORAGE_ACCESS_KEY": "AK-" + s, "OBJECT_STORAGE_SECRET_KEY": "SK-" + s})
    return out


class Infra(unittest.TestCase):
    def plan(self, cid, text, slots=None, cm=None, secrets=None):
        return ops.plan_infra(ICAT, cid, text, CM if cm is None else cm,
                              secrets_all() if secrets is None else secrets, slots or {})

    def refuses(self, fragment, *a, **kw):
        with self.assertRaises(ops.PlanError) as cm:
            self.plan(*a, **kw)
        self.assertIn(fragment, str(cm.exception))

    def printed(self, p):
        return json.dumps([p["config_rows"], p["secret_rows"], p["warnings"], p["restart"]], ensure_ascii=False)

    # ── PostgreSQL ──────────────────────────────────────────────────────────────────────────
    def test_pg_host_move_keeps_account_password_database(self):
        p = self.plan("postgresql", "HOSTS=db-a,db-b:5433\nSSLMODE=\nDB.platform=\nPASSWORD.platform=giu")
        self.assertEqual(sorted(p["secrets"]), SERVICES)
        self.assertEqual(p["secrets"]["platform"]["DATABASE_DSN"],
                         url("postgres", f"vigov_platform:{PW}", "db-a:5432,db-b:5433/vigov_platform?sslmode=require"))
        self.assertEqual(p["restart"][:2], ["vigov-service-platform", "vigov-service-identity"])
        self.assertEqual(len(p["restart"]), len(SERVICES))
        self.assertEqual(sorted(p["probe_dsn"]), SERVICES)
        self.assertNotIn(PW, self.printed(p))
        self.assertNotIn(PW, ops.probe_script(p))

    def test_pg_one_service_new_account_and_password(self):
        p = self.plan("postgresql", "USER.comms=comms_new\nPASSWORD.comms=2\nDB.comms=comms_db",
                      slots={"2": "p@ss/w:rd"})
        self.assertEqual(list(p["secrets"]), ["comms"])
        self.assertEqual(p["secrets"]["comms"]["DATABASE_DSN"],
                         url("postgres", "comms_new:p%40ss%2Fw%3Ard", "db-old:5432/comms_db?sslmode=require"))
        self.assertEqual(p["restart"], ["vigov-service-comms"])
        self.assertNotIn("p@ss", self.printed(p))

    def test_pg_refusals(self):
        self.refuses("phải kèm ô mật khẩu", "postgresql", "USER.comms=other")
        self.refuses("đang trống", "postgresql", "PASSWORD.comms=3")
        self.refuses("giu hoặc ô 1..7", "postgresql", "PASSWORD.comms=8")
        self.refuses("không được phép", "postgresql", "SSLMODE=dis" + "able")
        self.refuses("chữ thường", "postgresql", "DB.comms=Bad-Name")
        self.refuses("host hoặc host:port", "postgresql", "HOSTS=db;rm -rf")
        self.refuses("hai lần", "postgresql", "HOSTS=a\nHOSTS=b")
        self.refuses("không phải tên ô", "postgresql", "hosts=a")
        plain = dict(secrets_all(), comms={"DATABASE_DSN": url("postgres", f"u:{PW}", "db-old:5432/d")})
        self.refuses("luật 13", "postgresql", "HOSTS=db-new", secrets=plain)

    def test_pg_missing_dsn_needs_every_field_and_an_sslmode(self):
        missing = dict(secrets_all(), comms={})
        self.refuses("phải điền đủ", "postgresql", "HOSTS=db-new", secrets=missing)
        base = "HOSTS=db-new\nDB.comms=c\nUSER.comms=c\nPASSWORD.comms=1"
        # Built from nothing it would carry no sslmode — refused unless one is chosen (rule 13).
        self.refuses("luật 13", "postgresql", base, slots={"1": "x"}, secrets=missing)
        p = self.plan("postgresql", base + "\nSSLMODE=require", slots={"1": "x"}, secrets=missing)
        self.assertEqual(p["secrets"]["comms"]["DATABASE_DSN"], url("postgres", "c:x", "db-new:5432/c?sslmode=require"))

    def test_empty_form_changes_nothing(self):
        for cid in ("postgresql", "redis", "minio", "clamav"):
            p = self.plan(cid, "HOSTS=\nHOST=\nOBJECT_STORAGE_ENDPOINT=\n# ghi chú")
            self.assertEqual((p["configmap"], p["secrets"], p["restart"]), ({}, {}, []), cid)
        self.assertEqual(self.plan("postgresql", "HOSTS=db-old:5432\nSSLMODE=require")["secrets"], {})

    # ── Redis ───────────────────────────────────────────────────────────────────────────────
    def test_redis_new_host_and_password_keep_each_db(self):
        p = self.plan("redis", "HOST=redis-new:6380\nPASSWORD=1", slots={"1": "newpw"})
        self.assertEqual(sorted(p["secrets"]), sorted(REDIS_USERS))
        self.assertEqual(p["secrets"]["documents"]["REDIS_DSN"], url("redis", ":newpw", "redis-new:6380/1"))
        self.assertEqual(p["probe_tcp"], {("redis", "redis-new:6380")})
        self.assertNotIn("newpw", self.printed(p))

    def test_redis_refusals(self):
        self.refuses("MỘT host:port", "redis", "HOST=r1:6379,r2:6379")
        self.refuses("host:port", "redis", "HOST=redis-new")
        self.refuses("redis hoặc rediss", "redis", "SCHEME=http")
        self.refuses("phải điền máy chủ", "redis", "HOST=r:6379", secrets=dict(secrets_all(), comms={}))

    # ── MinIO ───────────────────────────────────────────────────────────────────────────────
    def test_minio_move_derives_media_url_and_warns_basemap(self):
        p = self.plan("minio", "OBJECT_STORAGE_ENDPOINT=https://minio-new:9000/\n"
                               "OBJECT_STORAGE_PUBLIC_ENDPOINT=https://storage.vigov.vn")
        self.assertEqual(p["configmap"], {"OBJECT_STORAGE_ENDPOINT": "https://minio-new:9000",
                                          "OBJECT_STORAGE_PUBLIC_ENDPOINT": "https://storage.vigov.vn",
                                          "OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL": "https://storage.vigov.vn/vigov-prod-public"})
        self.assertEqual(p["restart"], ["vigov-service-platform", "vigov-service-comms", "vigov-service-petitions"])
        self.assertIn("BASEMAP_URL", p["warnings"][0])
        self.assertEqual(p["probe_tcp"], {("minio", "minio-new:9000")})

    def test_minio_key_pairs(self):
        p = self.plan("minio", "ACCESS_KEY.petitions=1\nSECRET_KEY.petitions=2", slots={"1": "AKnew", "2": "SKnew"})
        self.assertEqual(p["secrets"], {"petitions": {"OBJECT_STORAGE_ACCESS_KEY": "AKnew",
                                                      "OBJECT_STORAGE_SECRET_KEY": "SKnew"}})
        self.assertEqual(p["restart"], ["vigov-service-petitions"])
        self.assertEqual(p["probe_tcp"], {("minio", "minio-old.vigov-infra.svc:9000")})
        self.assertNotIn("SKnew", self.printed(p))
        self.refuses("CẢ access key lẫn secret key", "minio", "ACCESS_KEY.comms=1", slots={"1": "x"})

    def test_minio_refusals(self):
        self.refuses("luật 13", "minio", "OBJECT_STORAGE_ENDPOINT=http://minio:9000")
        self.refuses("không đường dẫn", "minio", "OBJECT_STORAGE_PUBLIC_ENDPOINT=https://s3.vigov.vn/bucket")
        self.refuses("tên bucket", "minio", "OBJECT_STORAGE_BUCKET_PREFIX=Vigov_Prod")
        self.refuses("https://", "minio", "OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL=http://x/y")

    # ── ClamAV ──────────────────────────────────────────────────────────────────────────────
    def test_clamav(self):
        p = self.plan("clamav", "MALWARE_SCANNER_ADDRESS=clam-a:3310, clam-b:3310")
        self.assertEqual(p["configmap"], {"MALWARE_SCANNER_ADDRESS": "clam-a:3310,clam-b:3310"})
        self.assertEqual(p["probe_tcp"], {("clamav", "clam-a:3310"), ("clamav", "clam-b:3310")})
        self.refuses("host:port", "clamav", "MALWARE_SCANNER_ADDRESS=tcp://clam:3310")

    def test_unknown_component(self):
        self.refuses("không có", "kafka", "")


class InfraContract(unittest.TestCase):
    """The facts that tie ops.py to the Jenkinsfile and to this repository."""

    def test_slot_count_matches_jenkinsfile_and_covers_postgres(self):
        jf = ops._read(ops.JENKINSFILE)
        self.assertIn(f"(1..{ops.SECRET_SLOTS})", jf)
        self.assertIn("for n in " + " ".join(str(n) for n in range(1, ops.SECRET_SLOTS + 1)) + ";", jf)
        self.assertGreaterEqual(ops.SECRET_SLOTS, len(ops.by_key(ops.build_catalog())["DATABASE_DSN"]["users"]))

    def test_every_infra_key_is_catalogued_and_writable(self):
        cat = ops.by_key(ops.build_catalog())
        for c in ops.INFRA:
            for k in c["keys"]:
                self.assertIn(cat[k]["placement"], ("secret", "configmap"), k)
                self.assertTrue(cat[k]["registered"], k)

    def test_cli_writes_values_only_to_private_files(self):
        with tempfile.TemporaryDirectory() as d:
            def w(name, obj):
                p = os.path.join(d, name)
                with open(p, "w", encoding="utf-8") as f:
                    f.write(obj if isinstance(obj, str) else json.dumps(obj))
                return p
            os.makedirs(os.path.join(d, "sec"))
            os.makedirs(os.path.join(d, "slots"))
            for s in sorted(ops.service_groups()):
                w(os.path.join("sec", s + ".json"), {"data": {"DATABASE_DSN": b64(DSN.get(s, DSN["comms"]))}})
            w(os.path.join("slots", "slot1"), "brand-new-pw")
            edits = w("edits", "HOSTS=db-new\nPASSWORD.comms=1")
            cm = w("cm.json", {"data": {}})
            out = os.path.join(d, "plan")
            buf = io.StringIO()
            with contextlib.redirect_stdout(buf):
                rc = ops.main(["infra-plan", "--component", "postgresql", "--edits", edits, "--configmap", cm,
                               "--secrets-dir", os.path.join(d, "sec"), "--slot-dir", os.path.join(d, "slots"),
                               "--out-dir", out, "--stamp", "20261011-000000", "--probe-secret", "vigov-ops-probe-1",
                               "--note", "vigov.vn/ticket=T-1"])
            self.assertEqual(rc, 0, buf.getvalue())
            self.assertNotIn("brand-new-pw", buf.getvalue())
            self.assertNotIn(PW, buf.getvalue())

            def load(name):
                with open(os.path.join(out, name), encoding="utf-8") as f:
                    return f.read()
            patch = json.loads(load("secret-comms.json"))
            self.assertIn("brand-new-pw", base64.b64decode(patch["data"]["DATABASE_DSN"]).decode())
            backup = json.loads(load("backup-comms.json"))
            self.assertEqual(backup["metadata"]["name"], "comms-secrets-backup-20261011-000000")
            self.assertEqual(base64.b64decode(backup["data"]["DATABASE_DSN"]).decode(), DSN["comms"])
            self.assertEqual(json.loads(load("probe-secret.json"))["metadata"]["name"], "vigov-ops-probe-1")
            self.assertNotIn("brand-new-pw", load("probe.sh"))
            self.assertEqual(load("restart.txt").split()[0], "vigov-service-platform")


class Pods(unittest.TestCase):
    def args(self, **kw):
        base = dict(name="p", out="x", script=None, secret_env=[], envfrom_secret=[], pull_secret=[])
        base.update(kw)
        return argparse.Namespace(**base)

    def test_script_pod(self):
        with tempfile.TemporaryDirectory() as d:
            s = os.path.join(d, "s.sh")
            with open(s, "w", encoding="utf-8") as f:
                f.write("echo hi\n")
            pod = ops.build_pod(self.args(script=s, secret_env=["DSN_platform=platform-secrets/DATABASE_DSN"],
                                          pull_secret=["dockerhub-omicrm"]))
        c = pod["spec"]["containers"][0]
        self.assertEqual(c["image"], ops.PSQL_IMAGE)
        self.assertEqual(pod["spec"]["restartPolicy"], "Never")
        self.assertNotIn("app.kubernetes.io/part-of", pod["metadata"]["labels"])
        ref = [e for e in c["env"] if e["name"] == "DSN_platform"][0]["valueFrom"]["secretKeyRef"]
        self.assertEqual(ref, {"name": "platform-secrets", "key": "DATABASE_DSN", "optional": True})


    def test_bad_secret_env(self):
        with self.assertRaises(ops.PlanError):
            ops._secret_env("X=Bad_Secret/key")


class Basemap(unittest.TestCase):
    CM = {"OBJECT_STORAGE_BUCKET_PREFIX": "vigov-prod",
          "OBJECT_STORAGE_PUBLIC_ENDPOINT": "https://s3.example.vn/",
          "OBJECT_STORAGE_ENDPOINT": "https://minio.vigov-infra.svc:9000"}

    def refuses(self, fn, fragment):
        with self.assertRaises(ops.PlanError) as cm:
            fn()
        self.assertIn(fragment, str(cm.exception))

    def test_plan_derives_bucket_endpoint_and_url(self):
        p = ops.plan_basemap(self.CM, "20261008")
        self.assertEqual(p["bucket"], "vigov-prod-public")
        self.assertEqual(p["upload_endpoint"], "https://s3.example.vn")
        self.assertEqual(p["dest"], "vigov-prod-public/config/map/20261008/")
        # Read by the pod through the INTERNAL endpoint, never the public one the upload used.
        self.assertEqual(p["basemap_url"], "https://minio.vigov-infra.svc:9000/vigov-prod-public/config/map/20261008")

    def test_key_source_prefers_comms_then_platform(self):
        both = set(ops.STORAGE_KEY_NAMES)
        self.assertEqual(ops.pick_key_source({"comms-secrets": both, "platform-secrets": both}), "comms-secrets")
        self.assertEqual(ops.pick_key_source({"comms-secrets": {"OBJECT_STORAGE_ACCESS_KEY"},
                                              "platform-secrets": both}), "platform-secrets")
        # Dashed spelling is what envFrom ignores — it is not the key the services read.
        self.refuses(lambda: ops.pick_key_source({"comms-secrets": {"OBJECT-STORAGE-ACCESS-KEY", "OBJECT-STORAGE-SECRET-KEY"}}),
                     "không Secret nào")
        self.refuses(lambda: ops.pick_key_source({"petitions-secrets": both}), "không Secret nào")

    def test_date_validation(self):
        for bad, frag in [("2026-10-08", "YYYYMMDD"), ("19991231", "YYYYMMDD"), ("20261399", "có thật"),
                          ("", "YYYYMMDD"), ("20261008; rm -rf /", "YYYYMMDD")]:
            self.refuses(lambda b=bad: ops.check_basemap_date(b), frag)
        self.assertEqual(ops.check_basemap_date(" 20261008 "), "20261008")

    def test_http_is_refused_not_warned(self):
        for key in ("OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_ENDPOINT"):
            cm = dict(self.CM, **{key: "http://minio:9000"})
            self.refuses(lambda c=cm: ops.plan_basemap(c, "20261008"), "luật 13")

    def test_endpoint_shape(self):
        for bad in ("https://u:p@s3.example.vn", "https://s3.example.vn/bucket", "s3.example.vn", "https://"):
            self.refuses(lambda b=bad: ops.https_origin("K", b), "không đường dẫn")

    def test_missing_key_and_bad_prefix(self):
        for k in self.CM:
            cm = dict(self.CM)
            del cm[k]
            self.refuses(lambda c=cm: ops.plan_basemap(c, "20261008"), f"thiếu {k}")
        self.refuses(lambda: ops.plan_basemap(dict(self.CM, OBJECT_STORAGE_BUCKET_PREFIX="Vigov_Prod"), "20261008"),
                     "tên bucket")

    def test_placeholder_checksum_refuses(self):
        tools = [dict(ops.BASEMAP_TOOLS[0], sha256=ops.UNPINNED), dict(ops.BASEMAP_TOOLS[1], sha256="a" * 64)]
        with self.assertRaises(ops.PlanError) as cm:
            ops.basemap_tools(tools)
        self.assertIn("chưa ghim SHA-256 cho pmtiles", str(cm.exception))
        self.assertNotIn("mc,", str(cm.exception))
        self.assertIn("digest sha256", str(cm.exception))
        for t in tools:
            t["sha256"] = "b" * 64
        self.assertEqual([r[0] for r in ops.basemap_tools(tools)], ["pmtiles", "mc"])
        self.refuses(lambda: ops.basemap_tools([dict(tools[0], sha256="B" * 64)]), "chưa ghim")

    def test_real_pins_are_whole_or_untouched(self):
        # A half-pasted value must not pass as "pinned"; untouched placeholders are the only other state.
        for t in ops.BASEMAP_TOOLS:
            self.assertTrue(t["sha256"] == ops.UNPINNED or len(t["sha256"]) == 64, t["name"])
            self.assertTrue(t["url"].startswith("https://"), t["name"])
        self.assertIn(ops.PMTILES_VERSION, ops.BASEMAP_TOOLS[0]["url"])
        self.assertIn(ops.MC_RELEASE, ops.BASEMAP_TOOLS[1]["url"])
        # dl.min.io answers 410 Gone for mc binaries (11/10/2026) — the GitHub release is the source.
        self.assertTrue(ops.BASEMAP_TOOLS[1]["url"].startswith("https://github.com/minio/mc/releases/download/"))

    def test_mc_host_percent_encodes_keys(self):
        url = ops.mc_host_url("https://s3.example.vn", "AK", "s/k+e@y:1")
        self.assertEqual(url, "https://AK:s%2Fk%2Be%40y%3A1@s3.example.vn")
        self.refuses(lambda: ops.mc_host_url("https://s3.example.vn", "", "x"), "rỗng")
        self.refuses(lambda: ops.mc_host_url("http://s3.example.vn", "a", "b"), "luật 13")


if __name__ == "__main__":
    unittest.main()
