"""Unit tests for deploy/van-hanh/ops.py — the decisions behind the operations job (ADR 0089).

Run: python -m unittest discover -s deploy/van-hanh -p "test_*.py"   (part of `make buildfiles`)

Each refusal is tested by the sentence fragment that names it, not just "something raised": a
mutation that breaks one check while another still fires would otherwise stay green.
"""

from __future__ import annotations

import argparse
import base64
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


CAT = [
    entry("ENV", "configmap", SERVICES, "all"),
    entry("MALWARE_SCANNER_ADDRESS", "configmap", ["comms", "petitions", "platform"]),
    entry("OBJECT_STORAGE_ENDPOINT", "configmap", ["comms", "petitions", "platform"]),
    entry("TRUSTED_PROXY_CIDRS", "configmap", SERVICES),
    entry("BASEMAP-URL", "configmap", ["web-admin"], "opt"),
    entry("CITIZEN_SESSION_TTL", "configmap", ["identity"], "opt"),
    entry("PLATFORM_GRPC_ADDR", "env", ["identity"]),
    entry("RABBITMQ_DSN", "unset", [], "unused"),
    entry("NOT_IN_TEMPLATE", "configmap", ["platform"], registered=False),
    entry("DATABASE_DSN", "secret", SERVICES, "all"),
    entry("GRPC_CALLER_KEY", "secret", SERVICES, "all", shared=True),
    entry("SESSION_SIGNING_KEYS", "secret", ["identity"]),
    entry("SECRET_ENCRYPTION_KEYS", "secret", ["comms", "identity"]),
    entry("OPERATOR_TOTP_ENCRYPTION_KEY", "secret", ["identity"]),
    entry("IDENTITY_ADMIN_SEED_PASSWORD", "secret", ["identity"], "opt"),
]


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

    def test_web_admin_key_keeps_its_dashed_spelling_and_is_registered(self):
        e = self.cat["BASEMAP-URL"]
        self.assertEqual((e["placement"], e["users"], e["registered"]), ("configmap", ["web-admin"], True))

    def test_every_catalogued_key_is_registered_in_env_template(self):
        self.assertEqual([k for k, e in self.cat.items() if not e["registered"]], [])

    def test_shared_value_keys_detected(self):
        self.assertTrue(self.cat["GRPC_CALLER_KEY"]["shared"])
        self.assertFalse(self.cat["SECRET_ENCRYPTION_KEYS"]["shared"])

    def test_jenkinsfile_generated_region_is_fresh(self):
        jf = ops._read(ops.JENKINSFILE)
        region = ops.generated_region(ops.build_catalog(), self.services)
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


class ConfigPlan(unittest.TestCase):
    cur = {"ENV": "prod", "MALWARE_SCANNER_ADDRESS": "vigov-clamav:3310"}

    def plan(self, text, env="prod"):
        return ops.plan_config(CAT, self.cur, text, env)

    def refuses(self, text, fragment, env="prod"):
        with self.assertRaises(ops.PlanError) as cm:
            self.plan(text, env)
        self.assertIn(fragment, str(cm.exception))

    def test_change_restarts_only_readers(self):
        rows, patch, restart = self.plan("MALWARE_SCANNER_ADDRESS=clamav-a:3310,clamav-b:3310")
        self.assertEqual(patch, {"data": {"MALWARE_SCANNER_ADDRESS": "clamav-a:3310,clamav-b:3310"}})
        self.assertEqual(restart, ["vigov-service-comms", "vigov-service-petitions", "vigov-service-platform"])
        self.assertEqual(rows[0][1], "vigov-clamav:3310")

    def test_web_admin_key_restarts_web_admin(self):
        _, _, restart = self.plan("BASEMAP-URL=https://minio.example.vn/p-public/basemap/20261010")
        self.assertEqual(restart, ["vigov-web-admin"])

    def test_empty_value_keeps_and_unchanged_is_no_change(self):
        rows, patch, restart = self.plan("MALWARE_SCANNER_ADDRESS=\nENV=prod\n# note")
        self.assertEqual((rows, patch["data"], restart), ([], {}, []))

    def test_missing_key_is_settable(self):
        rows, _, _ = self.plan("CITIZEN_SESSION_TTL=720h")
        self.assertEqual(rows[0][1], "(chưa có)")

    def test_refusals(self):
        self.refuses("DATABASE_DSN=x", "SECRET")
        self.refuses("PLATFORM_GRPC_ADDR=platform:9090", "env của Deployment")
        self.refuses("RABBITMQ_DSN=x", "Không đặt")
        self.refuses("UNKNOWN_KEY=x", "luật 11 cấm #4")
        self.refuses("NOT_IN_TEMPLATE=x", "luật 11 cấm #4")
        self.refuses("ENV=staging", "ENV phải là 'prod'")
        self.refuses("MALWARE_SCANNER_ADDRESS=tcp://clamav:3310", "host:port")
        self.refuses("OBJECT_STORAGE_ENDPOINT=http://minio:9000", "https")
        self.refuses("TRUSTED_PROXY_CIDRS=10.42.0.0/16,0.0.0.0/0", "0.0.0.0/0")
        self.refuses("ENV=prod\nENV=prod", "hai lần")
        self.refuses("no equals sign", "thiếu dấu")

    def test_audit_lists_missing_and_extra(self):
        missing, extra = ops.audit_config(CAT, {"ENV": "prod", "STRAY": "1"})
        self.assertIn("MALWARE_SCANNER_ADDRESS", [m[0] for m in missing])
        self.assertNotIn("ENV", [m[0] for m in missing])
        self.assertEqual(extra, ["STRAY"])


class SecretPlan(unittest.TestCase):
    def plan(self, text, present=(), slots=("1",), service="identity"):
        return ops.plan_secret(CAT, service, set(present), text, set(slots))

    def refuses(self, text, fragment, **kw):
        with self.assertRaises(ops.PlanError) as cm:
            self.plan(text, **kw)
        self.assertIn(fragment, str(cm.exception))

    def test_modes(self):
        rows, actions = self.plan("SESSION_SIGNING_KEYS=xoay\nOPERATOR_TOTP_ENCRYPTION_KEY=sinh\nDATABASE_DSN=o1\n"
                                  "GRPC_CALLER_KEY=giu", present=("SESSION_SIGNING_KEYS", "DATABASE_DSN"))
        self.assertEqual(actions, [("SESSION_SIGNING_KEYS", "xoay", 48), ("OPERATOR_TOTP_ENCRYPTION_KEY", "sinh", 32),
                                   ("DATABASE_DSN", "o1", 48)])
        self.assertEqual(len(rows), 3)

    def test_refusals(self):
        self.refuses("GRPC_CALLER_KEY=sinh", "CÙNG MỘT GIÁ TRỊ")
        self.refuses("SESSION_SIGNING_KEYS=sinh", "đã có", present=("SESSION_SIGNING_KEYS",))
        self.refuses("SESSION_SIGNING_KEYS=xoay", "chưa có")
        self.refuses("OPERATOR_TOTP_ENCRYPTION_KEY=xoay", "_KEYS", present=("OPERATOR_TOTP_ENCRYPTION_KEY",))
        self.refuses("IDENTITY_ADMIN_SEED_PASSWORD=sinh", "không phải khoá")
        self.refuses("DATABASE_DSN=o2", "BI_MAT_2")
        self.refuses("SESSION_SIGNING_KEYS=o1", "không phải khoá của platform-secrets", service="platform")
        self.refuses("ENV=o1", "không phải khoá")
        self.refuses("DATABASE_DSN=xoa", "không hợp lệ")

    def test_apply_never_loses_the_old_value(self):
        with tempfile.TemporaryDirectory() as d:
            with open(os.path.join(d, "slot1"), "w", encoding="utf-8") as f:
                f.write("new-value\n")
            current = {"SESSION_SIGNING_KEYS": b64("old1,old2"), "DATABASE_DSN": b64("old-dsn")}
            patch, backup = ops.apply_secret(current, [("SESSION_SIGNING_KEYS", "xoay", 48), ("DATABASE_DSN", "o1", 48),
                                                       ("OPERATOR_TOTP_ENCRYPTION_KEY", "sinh", 32)], d)
        rotated = base64.b64decode(patch["SESSION_SIGNING_KEYS"]).decode()
        new_key, rest = rotated.split(",", 1)
        self.assertEqual(rest, "old1,old2")
        self.assertEqual(len(base64.b64decode(new_key)), 48)
        self.assertEqual(base64.b64decode(patch["DATABASE_DSN"]).decode(), "new-value")
        self.assertEqual(len(base64.b64decode(base64.b64decode(patch["OPERATOR_TOTP_ENCRYPTION_KEY"]))), 32)
        self.assertEqual(backup, current)

    def test_backup_secret_is_attached_to_nothing(self):
        m = ops.secret_manifest("identity-secrets-backup-20261010-101010", {"K": "dg=="},
                                {"vigov.vn/backup-of": "identity-secrets"}, {"vigov.vn/ticket": "T-1"})
        self.assertEqual(m["kind"], "Secret")
        self.assertTrue(ops.BACKUP_NAME.match(m["metadata"]["name"]))
        self.assertEqual(m["metadata"]["annotations"], {"vigov.vn/ticket": "T-1"})


class Resources(unittest.TestCase):
    deploys = {"items": [
        {"metadata": {"name": "vigov-service-platform"}, "spec": {"replicas": 1, "template": {"spec": {"containers": [
            {"name": "server", "resources": {"requests": {"cpu": "100m", "memory": "128Mi"},
                                             "limits": {"cpu": "500m", "memory": "512Mi"}}}]}}}},
        {"metadata": {"name": "vigov-web-admin"}, "spec": {"replicas": 1, "template": {"spec": {"containers": [
            {"name": "web"}]}}}},
    ]}

    def setUp(self):
        self.cur = ops.current_resources(self.deploys)

    def test_prefilled_lines_unchanged_are_skipped(self):
        rows, patches = ops.plan_resources(self.cur, ops.resource_lines(self.cur))
        self.assertEqual((rows, patches), ([], {}))

    def test_edit_and_preset(self):
        rows, patches = ops.plan_resources(self.cur, "vigov-service-platform replicas=2 cpu=200m/1\n"
                                                     "vigov-web-admin preset=vua mem=300Mi/1Gi")
        p = patches["vigov-service-platform"]["spec"]
        self.assertEqual(p["replicas"], 2)
        self.assertEqual(p["template"]["spec"]["containers"][0],
                         {"name": "server", "resources": {"requests": {"cpu": "200m", "memory": "128Mi"},
                                                          "limits": {"cpu": "1", "memory": "512Mi"}}})
        w = patches["vigov-web-admin"]["spec"]
        self.assertEqual(w["replicas"], ops.PRESETS["vua"]["replicas"])
        self.assertEqual(w["template"]["spec"]["containers"][0]["resources"]["requests"]["memory"], "300Mi")

    def refuses(self, text, fragment):
        with self.assertRaises(ops.PlanError) as cm:
            ops.plan_resources(self.cur, text)
        self.assertIn(fragment, str(cm.exception))

    def test_refusals(self):
        self.refuses("vigov-service-platform cpu=600m/500m", "cpu request lớn hơn limit")
        self.refuses("vigov-service-platform mem=1Gi/512Mi", "memory request lớn hơn limit")
        self.refuses("vigov-service-platform replicas=0", "1..10")
        self.refuses("vigov-service-x replicas=2", "không có Deployment")
        self.refuses("vigov-service-platform preset=huge", "nho | vua | lon")
        self.refuses("vigov-web-admin replicas=2", "thiếu request/limit")
        self.refuses("vigov-service-platform mem=1G/2G", "memory")


class Databases(unittest.TestCase):
    def test_default_names_differ_from_today(self):
        self.assertEqual(ops.default_db_name("platform", "prod"), "vigov_platform_prod")
        self.assertEqual(ops.default_db_name("platform", "staging"), "vigov_platform_stg")

    def test_plan_refuses_the_database_in_use(self):
        with self.assertRaises(ops.PlanError) as cm:
            ops.plan_databases(SERVICES, "platform vigov_platform u1", {"platform": "vigov_platform"})
        self.assertIn("CSDL cũ không bao giờ bị đụng", str(cm.exception))

    def test_plan_validation(self):
        for text, frag in [("platform Bad-Name u", "chữ thường"), ("nope db u", "không có"),
                           ("platform db1 u1\ncomms db1 u2", "trùng"), ("platform db", "<dịch vụ>"), ("", "trống")]:
            with self.assertRaises(ops.PlanError) as cm:
                ops.plan_databases(SERVICES, text, {})
            self.assertIn(frag, str(cm.exception))
        self.assertEqual(ops.plan_databases(SERVICES, "platform a_db a_user", {}), [("platform", "a_db", "a_user")])

    def test_create_sql_carries_no_password_and_checks_existence_first(self):
        sql = ops.create_sql([("platform", "vigov_platform_prod", "vigov_platform_prod")])
        self.assertIn("\\getenv pw_platform PW_PLATFORM", sql)
        self.assertLess(sql.index("RAISE EXCEPTION"), sql.index("CREATE ROLE"))
        self.assertIn("GRANT %I TO %I", sql)
        self.assertNotIn("DROP", sql.upper())
        self.assertNotIn("DELETE", sql.upper())

    def test_dsn_shape(self):
        pw = "a" * 48
        dsn = ops.build_dsn(ops.parse_hosts("db1,db2:5433", "5432"), "vigov_x", "vigov_x", pw, "require")
        self.assertEqual(dsn, "postgres://vigov_x:" + pw + "@db1:5432,db2:5433/vigov_x?sslmode=require")
        for mode in ("dis" + "able", "allow", "prefer", ""):
            with self.assertRaises(ops.PlanError):
                ops.build_dsn("db:5432", "d", "u", pw, mode)
        with self.assertRaises(ops.PlanError):
            ops.build_dsn("db:5432", "d", "u", "short", "require")
        with self.assertRaises(ops.PlanError):
            ops.parse_hosts("", "5432")
        with self.assertRaises(ops.PlanError):
            ops.parse_hosts("db;rm -rf", "5432")

    def test_db_name_never_returns_more_than_a_name(self):
        uri = "postgres://u:" + "secretpw" + "@h:5432/vigov_platform?sslmode=require"
        self.assertEqual(ops.db_name_of(uri), "vigov_platform")
        self.assertEqual(ops.db_name_of("host=h dbname=vigov_x user=u"), "vigov_x")
        self.assertEqual(ops.db_name_of("garbage with secretpw"), "")


class Migrations(unittest.TestCase):
    def test_rows(self):
        image = {"a": [("0001.sql", "s1"), ("0002.sql", "s2")], "b": [("0001.sql", "t1")], "c": [("0001.sql", "u1")]}
        applied = {"a": {"0001.sql": "s1"}, "b": {"0001.sql": "XX", "0009.sql": "z"}, "c": {"0001.sql": "u1"}}
        rows, blocking, ok = ops.migration_rows(["a", "b", "c", "d"], image, applied)
        self.assertIn("CHƯA ÁP: 0002.sql", rows[0][4])
        self.assertIn("TỆP ĐÃ ÁP BỊ SỬA: 0001.sql", rows[1][4])
        self.assertIn("CSDL MỚI HƠN ẢNH: 0009.sql", rows[1][4])
        self.assertEqual(rows[2][3], "khớp")
        self.assertEqual(rows[3][3], "?")
        self.assertTrue(blocking)
        self.assertFalse(ok)

    def test_all_match(self):
        _, blocking, ok = ops.migration_rows(["a"], {"a": [("1.sql", "s")]}, {"a": {"1.sql": "s"}})
        self.assertEqual((blocking, ok), (False, True))

    def test_empty_database_is_not_a_match(self):
        _, _, ok = ops.migration_rows(["a"], {"a": [("1.sql", "s")]}, {})
        self.assertFalse(ok)

    def test_order(self):
        self.assertEqual(ops.ordered_services(SERVICES)[:2], ["platform", "identity"])


class Pods(unittest.TestCase):
    def args(self, **kw):
        base = dict(name="p", out="x", script=None, from_deploy=None, command_json=None, secret_env=[], env=[],
                    envfrom_secret=[], envfrom_configmap=[], pull_secret=[])
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

    def test_from_deploy_overrides_dsn_and_keeps_security_context(self):
        dep = {"metadata": {"name": "vigov-service-platform"}, "spec": {"template": {"spec": {
            "securityContext": {"runAsUser": 65532}, "imagePullSecrets": [{"name": "harbor-vigov"}],
            "containers": [{"name": "s", "image": "harbor/x:abc", "envFrom": [{"secretRef": {"name": "platform-secrets"}}],
                            "env": [{"name": "DATABASE_DSN", "value": "old"}, {"name": "LISTEN_ADDR", "value": ":8080"}]}]}}}}
        with tempfile.TemporaryDirectory() as d:
            f = os.path.join(d, "dep.json")
            with open(f, "w", encoding="utf-8") as fh:
                json.dump(dep, fh)
            pod = ops.build_pod(self.args(from_deploy=f, secret_env=["DATABASE_DSN=platform-dsn-pending/DATABASE_DSN"]))
        c = pod["spec"]["containers"][0]
        names = [e["name"] for e in c["env"]]
        self.assertEqual(names.count("DATABASE_DSN"), 1)
        self.assertEqual([e for e in c["env"] if e["name"] == "DATABASE_DSN"][0]["valueFrom"]["secretKeyRef"]["name"],
                         "platform-dsn-pending")
        self.assertEqual(pod["spec"]["securityContext"], {"runAsUser": 65532})
        self.assertNotIn("command", c)
        self.assertEqual(c["image"], "harbor/x:abc")

    def test_bad_secret_env(self):
        with self.assertRaises(ops.PlanError):
            ops._secret_env("X=Bad_Secret/key")


if __name__ == "__main__":
    unittest.main()
