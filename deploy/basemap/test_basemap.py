"""Unit tests for deploy/basemap/basemap.py — the decisions behind `vigov-deploy`'s task `nen-ban-do`.

Run: python -m unittest discover -s deploy/basemap -p "test_*.py"   (part of `make buildfiles`)

Each refusal is tested by the sentence fragment that names it, not just "something raised": a
mutation that breaks one check while another still fires would otherwise stay green.
"""

from __future__ import annotations

import contextlib
import io
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import basemap  # noqa: E402


class Basemap(unittest.TestCase):
    CM = {"OBJECT_STORAGE_BUCKET_PREFIX": "vigov-prod",
          "OBJECT_STORAGE_PUBLIC_ENDPOINT": "https://s3.example.vn/",
          "OBJECT_STORAGE_ENDPOINT": "https://minio.vigov-infra.svc:9000"}

    def refuses(self, fn, fragment):
        with self.assertRaises(basemap.PlanError) as cm:
            fn()
        self.assertIn(fragment, str(cm.exception))

    def test_plan_derives_bucket_endpoint_and_url(self):
        p = basemap.plan_basemap(self.CM, "20261008")
        self.assertEqual(p["bucket"], "vigov-prod-public")
        self.assertEqual(p["upload_endpoint"], "https://s3.example.vn")
        self.assertEqual(p["dest"], "vigov-prod-public/config/map/20261008/")
        # Read by the pod through the INTERNAL endpoint, never the public one the upload used.
        self.assertEqual(p["basemap_url"], "https://minio.vigov-infra.svc:9000/vigov-prod-public/config/map/20261008")

    def test_key_source_prefers_comms_then_platform(self):
        both = set(basemap.STORAGE_KEY_NAMES)
        self.assertEqual(basemap.pick_key_source({"comms-secrets": both, "platform-secrets": both}), "comms-secrets")
        self.assertEqual(basemap.pick_key_source({"comms-secrets": {"OBJECT_STORAGE_ACCESS_KEY"},
                                                  "platform-secrets": both}), "platform-secrets")
        # Dashed spelling is what envFrom ignores — it is not the key the services read.
        self.refuses(lambda: basemap.pick_key_source({"comms-secrets": {"OBJECT-STORAGE-ACCESS-KEY",
                                                                        "OBJECT-STORAGE-SECRET-KEY"}}),
                     "không Secret nào")
        self.refuses(lambda: basemap.pick_key_source({"petitions-secrets": both}), "không Secret nào")

    def test_date_validation(self):
        for bad, frag in [("2026-10-08", "YYYYMMDD"), ("19991231", "YYYYMMDD"), ("20261399", "có thật"),
                          ("", "YYYYMMDD"), ("20261008; rm -rf /", "YYYYMMDD")]:
            self.refuses(lambda b=bad: basemap.check_basemap_date(b), frag)
        self.assertEqual(basemap.check_basemap_date(" 20261008 "), "20261008")

    def test_http_is_refused_not_warned(self):
        for key in ("OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_ENDPOINT"):
            cm = dict(self.CM, **{key: "http://minio:9000"})
            self.refuses(lambda c=cm: basemap.plan_basemap(c, "20261008"), "luật 13")

    def test_endpoint_shape(self):
        for bad in ("https://u:p@s3.example.vn", "https://s3.example.vn/bucket", "s3.example.vn", "https://"):
            self.refuses(lambda b=bad: basemap.https_origin("K", b), "không đường dẫn")

    def test_missing_key_and_bad_prefix(self):
        for k in self.CM:
            cm = dict(self.CM)
            del cm[k]
            self.refuses(lambda c=cm: basemap.plan_basemap(c, "20261008"), f"thiếu {k}")
        self.refuses(lambda: basemap.plan_basemap(dict(self.CM, OBJECT_STORAGE_BUCKET_PREFIX="Vigov_Prod"), "20261008"),
                     "tên bucket")

    def test_placeholder_checksum_refuses(self):
        tools = [dict(basemap.BASEMAP_TOOLS[0], sha256=basemap.UNPINNED), dict(basemap.BASEMAP_TOOLS[1], sha256="a" * 64)]
        with self.assertRaises(basemap.PlanError) as cm:
            basemap.basemap_tools(tools)
        self.assertIn("chưa ghim SHA-256 cho pmtiles", str(cm.exception))
        self.assertNotIn("mc,", str(cm.exception))
        self.assertIn("digest sha256", str(cm.exception))
        for t in tools:
            t["sha256"] = "b" * 64
        self.assertEqual([r[0] for r in basemap.basemap_tools(tools)], ["pmtiles", "mc"])
        self.refuses(lambda: basemap.basemap_tools([dict(tools[0], sha256="B" * 64)]), "chưa ghim")

    def test_real_pins_are_whole_or_untouched(self):
        # A half-pasted value must not pass as "pinned"; untouched placeholders are the only other state.
        for t in basemap.BASEMAP_TOOLS:
            self.assertTrue(t["sha256"] == basemap.UNPINNED or len(t["sha256"]) == 64, t["name"])
            self.assertTrue(t["url"].startswith("https://"), t["name"])
        self.assertIn(basemap.PMTILES_VERSION, basemap.BASEMAP_TOOLS[0]["url"])
        self.assertIn(basemap.MC_RELEASE, basemap.BASEMAP_TOOLS[1]["url"])
        # dl.min.io answers 410 Gone for mc binaries (11/10/2026) — the GitHub release is the source.
        self.assertTrue(basemap.BASEMAP_TOOLS[1]["url"].startswith("https://github.com/minio/mc/releases/download/"))

    def test_mc_host_percent_encodes_keys(self):
        url = basemap.mc_host_url("https://s3.example.vn", "AK", "s/k+e@y:1")
        self.assertEqual(url, "https://AK:s%2Fk%2Be%40y%3A1@s3.example.vn")
        self.refuses(lambda: basemap.mc_host_url("https://s3.example.vn", "", "x"), "rỗng")
        self.refuses(lambda: basemap.mc_host_url("http://s3.example.vn", "a", "b"), "luật 13")


class Cli(unittest.TestCase):
    """The subcommand names basemap-task.sh calls — renaming one would break the job, not a test."""

    def test_task_script_calls_only_existing_subcommands(self):
        here = os.path.dirname(os.path.abspath(__file__))
        with open(os.path.join(here, "basemap-task.sh"), encoding="utf-8") as f:
            script = f.read()
        for cmd in ("plan", "tools", "key-source", "mc-host"):
            self.assertIn(f"OPS {cmd} ", script)
        self.assertIn("deploy/basemap/basemap.py", script)

    def test_plan_writes_env_file(self):
        with tempfile.TemporaryDirectory() as d:
            cm = os.path.join(d, "cm.json")
            with open(cm, "w", encoding="utf-8") as f:
                json.dump({"data": Basemap.CM}, f)
            out = os.path.join(d, "bm.env")
            with contextlib.redirect_stdout(io.StringIO()):
                rc = basemap.main(["plan", "--current", cm, "--date", "20261008", "--out", out])
            self.assertEqual(rc, 0)
            with open(out, encoding="utf-8") as f:
                self.assertIn("BUCKET=vigov-prod-public\n", f.read())
            with contextlib.redirect_stdout(io.StringIO()) as buf:
                rc = basemap.main(["plan", "--current", cm, "--date", "bad", "--out", out])
            self.assertEqual(rc, 2)
            self.assertIn("DỪNG", buf.getvalue())


if __name__ == "__main__":
    unittest.main()
