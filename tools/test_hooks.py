#!/usr/bin/env python3
"""ViGov hook verification — every hook gets a must-BLOCK case and a must-PASS case.

WHY THIS IS NEEDED: hooks are the ONLY enforcement layer of the brain. A handful of Python
files carry the entire safety of a system handling citizen data — and without tests nobody
knows when one has died. One syntax error, one broken pattern, and the protection is off in
silence: everything still looks normal, it just stops blocking.

Run:  python tools/test_hooks.py        (or `make check`, or /check-hooks)
Exit code: 0 = all passed, 1 = something failed.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HOOKS = os.path.join(ROOT, ".claude", "hooks")

BLOCK, PASS = 2, 0

# Windows terminals default to cp1252. Same reason as utf8_streams() in _common.
for _s in (sys.stdout, sys.stderr):
    try:
        _s.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass


def w(path: str, content: str) -> dict:
    return {"tool_name": "Write", "tool_input": {"file_path": path, "content": content}}


def b(cmd: str) -> dict:
    return {"tool_name": "Bash", "tool_input": {"command": cmd}}


def mon(cmd: str) -> dict:
    """The SAME command arriving as `Monitor` instead of `Bash`.

    Monitor hands its `command` to the same shell as Bash, but under a different tool name.
    A PreToolUse matcher of "Bash" therefore never saw it — and neither did `permissions.deny`,
    which is keyed on the tool name too, so every deny entry written as `Bash(...)` had a door
    beside it. These cases are what keeps the two paths from drifting apart again."""
    return {"tool_name": "Monitor", "tool_input": {"command": cmd}}


def wpost(path: str, content: str) -> dict:
    """A Write seen at PostToolUse. rest_api_guard blocks a bad PATH before the write, but
    only advises on a missing declaration after it — a route assembled over several edits
    must not be blocked half-written (the lesson rule 5 already took with rbac_guard)."""
    d = w(path, content)
    d["hook_event_name"] = "PostToolUse"
    return d


# (hook, case label, expected exit code, payload)
CASES = [
    # ---- rule 1 · tenant isolation -------------------------------------------
    ("tenant_scope_guard", "query without tenant scope", BLOCK,
     w("donthu/internal/app/list.go",
       "func (s *Svc) List(ctx context.Context) {\n\trows, err := s.db.Find(ctx, filter)\n}")),
    ("tenant_scope_guard", "goes through a scoped repository", PASS,
     w("donthu/internal/app/list.go",
       "func (s *Svc) List(ctx context.Context) {\n\trepo := s.scoped(ctx)\n\trows, err := repo.Find(ctx, filter)\n}")),
    ("tenant_scope_guard", "single-column unique key", BLOCK,
     w("donthu/internal/store/schema.go",
       "// UNIQUE (code)\nconst ddl = `CREATE TABLE don_thu (code text, UNIQUE (code))`")),
    ("tenant_scope_guard", "composite unique key", PASS,
     w("donthu/internal/store/schema.go",
       "const ddl = `CREATE TABLE don_thu (tenant_id text, code text, UNIQUE (tenant_id, code))`")),
    ("tenant_scope_guard", "tenant taken from the client", BLOCK,
     w("donthu/internal/http/h.go",
       'func h(w http.ResponseWriter, r *http.Request) {\n\ttid := r.URL.Query().Get("tenant_id")\n}')),
    # Six of eight services shipped a partitioned audit_log with ZERO partitions, and no hook
    # saw it because this guard watched only .go. A partitioned table with no partitions
    # rejects every INSERT, and the audit entry shares the business transaction — so the first
    # real business write rolls back entirely.
    ("tenant_scope_guard", "partitioned table with no partitions", BLOCK,
     w("petitions/migrations/0001_init.sql",
       "CREATE TABLE IF NOT EXISTS audit_log (\n"
       "  tenant_id text NOT NULL,\n  id BIGSERIAL\n) PARTITION BY HASH (tenant_id);")),
    ("tenant_scope_guard", "partitioned table with its partitions", PASS,
     w("petitions/migrations/0001_init.sql",
       "CREATE TABLE IF NOT EXISTS audit_log (\n"
       "  tenant_id text NOT NULL,\n  id BIGSERIAL\n) PARTITION BY HASH (tenant_id);\n"
       "DO $$ BEGIN FOR i IN 0..31 LOOP EXECUTE format(\n"
       "  'CREATE TABLE IF NOT EXISTS %I PARTITION OF audit_log "
       "FOR VALUES WITH (MODULUS 32, REMAINDER %s)',\n"
       "  'audit_log_p' || lpad(i::text,2,'0'), i); END LOOP; END $$;")),
    # The skeleton template shows the declaration in a comment. Flagging the very comment that
    # teaches the shape would be a guard nobody keeps.
    ("tenant_scope_guard", "declaration inside a SQL comment", PASS,
     w("comms/migrations/0001_init.sql",
       "-- CREATE TABLE bai_viet (\n--   tenant_id text NOT NULL\n"
       "-- ) PARTITION BY HASH (tenant_id);\n")),
    ("tenant_scope_guard", "explicit @cross-tenant escape", PASS,
     w("baocao/internal/app/huyen.go",
       "// @cross-tenant: tổng hợp phản ánh cấp huyện, chỉ số liệu, đã ghi nhật ký\n\trows, _ := s.db.Find(ctx, f)")),

    # ---- rule 2 · service boundary ---------------------------------------
    ("service_boundary_guard", "imports another service's internal", BLOCK,
     w("baocao/internal/app/a.go",
       'import (\n\t"vigov/donthu/internal/store"\n)')),
    ("service_boundary_guard", "imports a shared package", PASS,
     w("baocao/internal/app/a.go",
       'import (\n\t"vigov/core/tenant"\n)')),
    ("service_boundary_guard", "hand-edits a proto-generated file", BLOCK,
     w("kb/20-contracts/grpc/donthu.pb.go", "package pb\n// sửa tay")),

    # ---- rule 3 · personal data -----------------------------------------
    ("pii_guard", "logs a phone number", BLOCK,
     w("congdan/internal/app/otp.go", "log.Info(citizenPhone)")),
    ("pii_guard", "logs a business code", PASS,
     w("congdan/internal/app/otp.go", 'log.Info("da gui otp", "code", dt.Code)')),
    ("pii_guard", "prints a whole struct", BLOCK,
     w("congdan/internal/app/otp.go", 'fmt.Printf("%+v", user)')),
    ("pii_guard", "hardcoded real phone number", BLOCK,
     w("congdan/internal/app/seed.go", 'phone := "0912345678"')),
    ("pii_guard", "agreed fake number", PASS,
     w("congdan/internal/app/seed.go", 'phone := "0900000000"')),
    ("pii_guard", "personal data in .json", BLOCK,
     w("congdan/seed/cd.json", '[{"phone":"0912345678","cccd":"079123456789"}]')),

    # ---- rule 4 · citizen isolation ----------------------------------------
    ("citizen_scope_guard", "identity from the query string", BLOCK,
     w("congdan/internal/http/citizen.go",
       'phone := r.URL.Query().Get("phone")')),
    ("citizen_scope_guard", "identity from the session", PASS,
     w("congdan/internal/http/citizen.go",
       "cit := auth.CitizenFrom(ctx)")),

    # ---- rule 5 · authorisation ----------------------------------------------
    ("rbac_guard", "route with no permission", BLOCK,
     w("donthu/internal/http/router.go",
       'r.Get("/don-thu", h.List)')),
    ("rbac_guard", "route with a permission", PASS,
     w("donthu/internal/http/router.go",
       'r.With(auth.RequirePermission("donthu", "view")).Get("/don-thu", h.List)')),
    ("rbac_guard", "adjacent routes, second one unguarded", BLOCK,
     w("donthu/internal/http/router.go",
       'r.With(auth.RequirePermission("donthu", "view")).Get("/don-thu", h.List)\n'
       'r.Delete("/don-thu/{id}", h.Remove)')),
    ("rbac_guard", "Public() with no reason", BLOCK,
     w("donthu/internal/http/router.go",
       'r.With(auth.Public()).Get("/tra-cuu", h.Lookup)')),
    ("rbac_guard", "HandleFunc route with no permission", BLOCK,
     w("petitions/internal/http/routes.go",
       'mux.HandleFunc("POST /api/v1/citizen-reports", h.Create)')),
    ("rbac_guard", "HandleFunc route with a permission", PASS,
     w("petitions/internal/http/routes.go",
       'mux.Handle("POST /api/v1/citizen-reports",\n'
       '\tauthz.RequirePermission(d.Checker, "feedback.create")(http.HandlerFunc(h.Create)))')),

    # ---- REST surface · path language + duplicate requests --------------------
    ("rest_api_guard", "Vietnamese path segment, transliterated", BLOCK,
     w("identity/internal/http/routes.go",
       'mux.Handle("POST /dang-nhap", authz.Public("man hinh dang nhap")(h.Login))')),
    ("rest_api_guard", "Vietnamese path segment with diacritics", BLOCK,
     w("petitions/internal/http/routes.go",
       'mux.Handle("GET /api/v1/phản-ánh/{code}", authz.CitizenOnly()(h.Get))')),
    ("rest_api_guard", "Vietnamese segment under a versioned prefix", BLOCK,
     w("petitions/internal/http/routes.go",
       'mux.Handle("GET /api/v1/phan-anh/{ma}", authz.CitizenOnly()(h.Get))')),
    ("rest_api_guard", "English path, versioned, with idempotency", PASS,
     w("petitions/internal/http/routes.go",
       'mux.Handle("POST /api/v1/citizen-reports",\n'
       '\tauthz.RequirePermission(d.Checker, "feedback.create")(\n'
       '\t\tidem.Required(idem.MoKhiHong)(http.HandlerFunc(h.Create))))')),
    ("rest_api_guard", "healthz stays outside /api/v1", PASS,
     wpost("platform/cmd/server/main.go",
           'mux.HandleFunc("GET /healthz", ok)')),
    ("rest_api_guard", "state-changing route, no duplicate declaration", BLOCK,
     wpost("petitions/internal/http/routes.go",
           'mux.Handle("POST /api/v1/citizen-reports",\n'
           '\tauthz.RequirePermission(d.Checker, "feedback.create")(http.HandlerFunc(h.Create)))')),
    ("rest_api_guard", "idem.KhongCan() states no reason", BLOCK,
     wpost("identity/internal/http/routes.go",
           'mux.Handle("DELETE /api/v1/sessions/{sid}",\n'
           '\tauthz.AnyAuthenticated("ends its own session")(\n'
           '\t\tidem.KhongCan()(http.HandlerFunc(h.Revoke))))')),
    ("rest_api_guard", "verb used as a path segment", BLOCK,
     wpost("petitions/internal/http/routes.go",
           'mux.Handle("POST /api/v1/citizen-reports/{code}/close",\n'
           '\tauthz.RequirePermission(d.Checker, "feedback.resolve")(\n'
           '\t\tidem.Required(idem.DongKhiHong)(http.HandlerFunc(h.Close))))')),
    # core/idem refuses this at runtime, but it cannot refuse at wiring time: authz wraps
    # outside idem, so idem never sees the Public declaration. The route statement is the one
    # place both are visible together.
    ("rest_api_guard", "Public route with idem.Required", BLOCK,
     w("petitions/internal/http/routes.go",
       'mux.Handle("POST /api/v1/citizen-reports",\n'
       '\tauthz.Public("công dân gửi phản ánh qua Mini App")(\n'
       '\t\tidem.Required(idem.MoKhiHong)(http.HandlerFunc(h.Create))))')),
    ("rest_api_guard", "Public route with idem.KhongCan", PASS,
     w("identity/internal/http/routes.go",
       'mux.Handle("POST /api/v1/sessions",\n'
       '\tauthz.Public("màn hình đăng nhập")(\n'
       '\t\tidem.KhongCan("đăng nhập lần hai mở một phiên thứ hai")(h.DangNhap)))')),
    # A route with no @summary/@reply block never reaches kb/20-contracts/openapi.json, and a
    # route absent from the contract is a screen the web side builds by guessing the response
    # shape — the v1 failure where the type source of truth moved into the frontend.
    ("rest_api_guard", "route with no contract block", BLOCK,
     wpost("petitions/internal/http/routes.go",
           'mux.Handle("GET /api/v1/citizen-reports",\n'
           '\tauthz.RequirePermission(d.Checker, "feedback.read")(http.HandlerFunc(h.List)))')),
    # The block must sit IMMEDIATELY above — that is the rule tools/apidoc applies, and a guard
    # accepting a looser shape would pass routes the generator then silently drops.
    ("rest_api_guard", "contract block separated by a blank line", BLOCK,
     wpost("petitions/internal/http/routes.go",
           '// @summary  Danh sách phản ánh\n'
           '// @reply    200 danhSachPhanAnh\n'
           '\n'
           'mux.Handle("GET /api/v1/citizen-reports",\n'
           '\tauthz.RequirePermission(d.Checker, "feedback.read")(http.HandlerFunc(h.List)))')),
    ("rest_api_guard", "a typo in a contract tag", BLOCK,
     wpost("petitions/internal/http/routes.go",
           '// @summary  Danh sách phản ánh\n'
           '// @replies  200 danhSachPhanAnh\n'
           'mux.Handle("GET /api/v1/citizen-reports",\n'
           '\tauthz.RequirePermission(d.Checker, "feedback.read")(http.HandlerFunc(h.List)))')),
    ("rest_api_guard", "route with a complete contract block", PASS,
     wpost("petitions/internal/http/routes.go",
           '// @summary  Danh sách phản ánh của xã\n'
           '// @screen   09-phan-anh-nguoi-dan §4\n'
           '// @reply    200 danhSachPhanAnh\n'
           '// @reply    403 httpx.Error\n'
           'mux.Handle("GET /api/v1/citizen-reports",\n'
           '\tauthz.RequirePermission(d.Checker, "feedback.read")(http.HandlerFunc(h.List)))')),
    ("rest_api_guard", "a route inside a comment is not a route", PASS,
     wpost("comms/internal/http/routes.go",
           '// Example of the shape every real route must take:\n'
           '//\tmux.Handle("POST /dang-nhap", authz.Public("x")(h.Login))\n'
           'func Register(mux *http.ServeMux, d Deps) { _ = d }')),
    ("rest_api_guard", "nominalised action with a failure mode", PASS,
     wpost("petitions/internal/http/routes.go",
           '// @summary  Đóng phiếu phản ánh và trả kết quả cho công dân\n'
           '// @screen   09-phan-anh-nguoi-dan §14\n'
           '// @reply    200 phanHoiDongPhieu\n'
           '// @reply    409 httpx.Error\n'
           'mux.Handle("POST /api/v1/citizen-reports/{code}/closure",\n'
           '\tauthz.RequirePermission(d.Checker, "feedback.resolve")(\n'
           '\t\tidem.Required(idem.DongKhiHong)(http.HandlerFunc(h.Close))))')),

    # check_brain invariant 6 names docs/ui-ux/ as an exception; doc_guard did not know, so the
    # checker passed the directory while the guard refused every edit to it. The file that
    # exposed it held real staff names and real mobile numbers — rule 3, forbidden #5 — and the
    # redaction was the thing being blocked.
    # Rule 9 names "two files both claiming ownership of one fact" as a STOP CONDITION, and
    # until now nothing checked it: owned_facts() assigns into a dict, so the second claimant
    # silently overwrote the first and the map looked healthy. Worst on ADRs, which are never
    # edited — two owning one decision cannot be merged afterwards.
    #
    # THIS CASE ALSO GUARDS THE STDIN ENCODING. The fact below is Vietnamese, so it only
    # matches the copy on disk when the payload is decoded as UTF-8. `_common.utf8_streams`
    # used to reconfigure stdout and stderr but not stdin; on Windows cp1252 decoded the bytes
    # without raising and every Vietnamese character arrived as mojibake. If someone drops
    # stdin from that list, this case goes red — which is the only visible symptom that bug
    # ever had.
    ("doc_guard", "a second file claiming ADR 0012's fact", BLOCK,
     w("kb/10-decisions/0099-thu-nghiem.md",
       "---\nid: 0099-thu-nghiem\ntier: T1\nsource: CURATED\nowner: architecture\n"
       "derived_from_commit: null\nexpires: null\nowns_facts:\n"
       "  - \"vì sao platform không tới được thì mọi Host thành 404 chứ không 503\"\n"
       "---\n\n# 0099\n")),
    ("doc_guard", "an ADR claiming a fact nobody owns", PASS,
     w("kb/10-decisions/0099-thu-nghiem.md",
       "---\nid: 0099-thu-nghiem\ntier: T1\nsource: CURATED\nowner: architecture\n"
       "derived_from_commit: null\nexpires: null\nowns_facts:\n"
       "  - \"vì sao hàng đợi việc cho web dùng thư mục làm trạng thái\"\n"
       "---\n\n# 0099\n")),
    ("doc_guard", "editing the transcribed UI spec", PASS,
     w("docs/ui-ux/12-danh-ba-can-bo.md", "| Nguyễn Văn A | Bí thư | 0900000001 |")),
    # Narrower than check_brain on purpose: "a NAMED exception, not an open door".
    ("doc_guard", "a NEW .md under docs/", BLOCK,
     w("docs/ui-ux/99-ghi-chu-moi.md", "# Ghi chú")),
    ("doc_guard", "a new .md outside every allowed place", BLOCK,
     w("notes/ke-hoach.md", "# Kế hoạch")),

    # ---- rule 6 · audit trail -------------------------------------------------
    ("audit_guard", "write with no audit entry", BLOCK,
     w("donthu/internal/app/update.go",
       "func (s *Svc) Update(ctx context.Context) error {\n\treturn s.repo.Save(ctx, dt)\n}")),
    ("audit_guard", "write with an audit entry", PASS,
     w("donthu/internal/app/update.go",
       "func (s *Svc) Update(ctx context.Context) error {\n\ts.repo.Save(ctx, dt)\n\treturn audit.Write(ctx, e)\n}")),

    # ---- rule 7 · data preservation ----------------------------------------
    ("data_safety_guard", "rm -rf", BLOCK, b("rm -rf ./tmp")),
    ("data_safety_guard", "rm with split flags -r -f", BLOCK, b("rm -r -f ./tmp")),
    ("data_safety_guard", "rm --recursive --force", BLOCK, b("rm --recursive --force ./tmp")),
    ("data_safety_guard", "find -delete", BLOCK, b("find . -name '*.go' -delete")),
    ("data_safety_guard", "rimraf", BLOCK, b("npx rimraf web/dist")),
    ("data_safety_guard", "git push --force", BLOCK, b("git push origin main --force")),
    ("data_safety_guard", "--force-with-lease is allowed", PASS,
     b("git push origin main --force-with-lease")),
    ("data_safety_guard", "an ordinary read command", PASS, b("go test ./...")),
    # The same commands arriving as Monitor. They reach the same shell, so they must meet the
    # same guard; the tool name is the only thing that differs and it is not a safety property.
    ("data_safety_guard", "destructive command arriving as Monitor", BLOCK, mon("rm -rf ./tmp")),
    ("data_safety_guard", "harmless command arriving as Monitor", PASS, mon("go test ./...")),
    ("bash_content_guard", "secret written to a file via Monitor", BLOCK,
     mon("echo 'aws_secret_access_key = AKIAIOSFODNN7EXAMPLE' > .env")),
    ("bash_content_guard", "ordinary command arriving as Monitor", PASS, mon("ls -la ")),
    # THIRD false positive of this guard. A commit message explaining a migration used the
    # words TRUNCATE and DROP, and the heredoc body was scanned as if it were a command. The
    # body of `git commit -F -` is prose that never executes; blocking it teaches the next
    # agent to describe destructive SQL vaguely, which is the opposite of the point.
    ("data_safety_guard", "commit message that mentions TRUNCATE", PASS,
     b("git commit -F - <<'MSG'\nfeat(db): chan TRUNCATE va DROP TABLE tren audit_log\nMSG")),
    # But a heredoc fed to a database client IS the command.
    ("data_safety_guard", "heredoc piped into psql", BLOCK,
     b("psql \"$DSN\" <<'SQL'\nTRUNCATE audit_log;\nSQL")),
    # The interpreter must be looked for in the COMMAND part, not in the body. Checking the
    # whole string reopened the bug: a commit message that NAMES psql while explaining this
    # rule kept the body and fired again.
    ("data_safety_guard", "commit message naming psql and TRUNCATE", PASS,
     b("git commit -F - <<'MSG'\nvi sao heredoc vao psql van bi chan: TRUNCATE la lenh that\nMSG")),
    ("data_safety_guard", "DELETE FROM on business data", BLOCK,
     w("donthu/internal/store/q.go", 'const q = "DELETE FROM don_thu WHERE id=$1"')),
    ("data_safety_guard", "DELETE with no WHERE", BLOCK,
     w("donthu/internal/store/q.go", 'const q = "DELETE FROM don_thu"')),
    ("data_safety_guard", "UPDATE with no WHERE", BLOCK,
     w("donthu/internal/store/q.go", 'const q = "UPDATE don_thu SET trang_thai = 1"')),
    # Go's built-in delete() removes one key from an in-memory map. Blocking it made every Go
    # file holding a map unwritable, and a guard that cries wolf gets switched off.
    ("data_safety_guard", "Go built-in delete on a map", PASS,
     w("nentang/internal/store/cache.go", "delete(c.entries, host)")),
    # A readable UPDATE puts its WHERE on the next line. A line-bounded check flagged every one.
    ("data_safety_guard", "UPDATE with WHERE on the next line", PASS,
     w("donthu/internal/store/q.go",
       'const q = `UPDATE don_thu SET trang_thai = $3\n\t WHERE tenant_id = $1 AND id = $2`')),

    # ---- rule 8 · secrets --------------------------------------------------
    ("secret_scan", "hardcoded secret", BLOCK,
     w("donthu/internal/cfg.go", 'const jwtSecret = "sieu-bi-mat-1234567"')),
    ("secret_scan", "read from an environment variable", PASS,
     w("donthu/internal/cfg.go", 'jwtSecret := os.Getenv("JWT_SECRET")')),
    ("secret_scan", "real connection string in documentation", BLOCK,
     w("kb/40-runbooks/khoi-phuc.md",
       "dsn: postgres://admin:P4ssw0rd-that-is-real@10.0.0.5:5432/vigov")),
    ("secret_scan", "real value in a template file", BLOCK,
     w("deploy/.env.example", "JWT_SECRET=aX9fQ2mZ7pL1kR4tY8wB")),
    ("secret_scan", "placeholder in a template file", PASS,
     w("deploy/.env.example", "JWT_SECRET=change-me-truoc-khi-chay")),

    # ---- bash_content_guard · closing the detour ------------------------------
    ("bash_content_guard", "secret written via heredoc", BLOCK,
     b('cat > a/cfg.go <<EOF\nconst api_key = "sk-live-abcdefghijk"\nEOF')),
    ("bash_content_guard", "personal-data log written via redirect", BLOCK,
     b('echo "log.Info(citizenPhone)" >> a/x.go')),
    ("bash_content_guard", "writes harmless content", PASS,
     b('echo "package main" > a/x.go')),
    ("bash_content_guard", "a read command", PASS, b("cat a/x.go")),

    # ---- rule 9 · knowledge ------------------------------------------------
    ("doc_guard", "documentation in the wrong place", BLOCK,
     w("notes/ghi-chu-hom-nay.md", "# ghi chú")),
    ("doc_guard", "kb document missing frontmatter", BLOCK,
     w("kb/00-foundation/x.md", "# Miền nghiệp vụ")),
    ("doc_guard", "hand-edits the GENERATED tier", BLOCK,
     w("kb/30-indexes/data-ownership.json.md", "# bảng sở hữu")),
    ("doc_guard", "T5 with no expiry", BLOCK,
     w("kb/90-ephemeral/plan.md",
       "---\nid: plan\ntier: T5\nsource: CURATED\nowner: ai\nexpires: null\n---\n# plan")),
    ("doc_guard", "T5 with an expiry", PASS,
     w("kb/90-ephemeral/plan.md",
       "---\nid: plan\ntier: T5\nsource: CURATED\nowner: ai\nexpires: 2026-10-15\n---\n# plan")),
    ("doc_guard", "a service README", PASS,
     w("donthu/README.md", "# Service Đơn thư")),

    # --- rule 10 — the commitment made to the citizen ---
    ("citizen_commitment_guard", "overdue stored as a struct field", BLOCK,
     w("petitions/internal/domain/p.go",
       "type Petition struct {\n\tCode string\n\tIsOverdue bool\n}")),
    ("citizen_commitment_guard", "overdue added as a column", BLOCK,
     w("petitions/migrations/0002_overdue.sql",
       "ALTER TABLE petitions ADD COLUMN is_overdue BOOLEAN DEFAULT false;")),
    ("citizen_commitment_guard", "deadline counted in calendar days", BLOCK,
     w("petitions/internal/app/sla.go",
       "func deadline(t time.Time) time.Time { return t.AddDate(0, 0, slaDays) }")),
    ("citizen_commitment_guard", "overdue DERIVED by a method", PASS,
     w("petitions/internal/domain/p.go",
       "func (p Petition) IsOverdue(now time.Time) bool {\n"
       "\treturn p.ClosedAt.IsZero() && now.After(p.SLADeadline)\n}")),
    ("citizen_commitment_guard", "deadline in working days", PASS,
     w("petitions/internal/app/sla.go",
       "func deadline(ctx context.Context, from time.Time) time.Time {\n"
       "\treturn sla.WorkingDays(ctx, from, n)\n}")),
    ("citizen_commitment_guard", "statutory calendar days, declared", PASS,
     w("petitions/internal/app/khieunai.go",
       "// @sla-ok: Law on Complaints art. 28 counts calendar days\n"
       "func due(t time.Time) time.Time { return t.AddDate(0, 0, 30) } // sla")),
]


# ---- pure-function cases: what stop_verify_guard COUNTS AS CODE ---------------------------
#
# WHY THESE SIT APART FROM THE CASES ABOVE. Three hooks are exempt from the payload cases
# because they need a real session transcript. That exemption was read too widely:
# `stop_verify_guard.is_code()` decides **what counts as code at all**, it is a pure function of
# a path, it needs no environment — and it had no test. So on the day `/tools/` was missing from
# its directory list, edits to the contract GENERATOR were invisible to the gate: change
# tools/apidoc, say "done", and nothing had to run.
#
# A whitelist of directory segments is exactly the kind of list that stops matching in silence —
# the flat layout (ADR 0015) emptied four of them at once and nothing went red. The only thing
# that catches it is an assertion naming one real path per shape a deployable unit can take.
IS_CODE_CASES = [
    ("service-identity/internal/http/routes.go", True, "mã dịch vụ"),
    ("service-identity/migrations/0004_x.sql", True, "migration của dịch vụ"),
    ("core/httpx/edge.go", True, "mã dùng chung"),
    ("web-admin/src/lib/api/can-bo.ts", True, "mã web"),
    ("proto/vigov/identity/v1/identity.proto", True, "hợp đồng giữa service"),
    ("tools/apidoc/route.go", True, "TRÌNH SINH hợp đồng REST — ca từng thủng"),
    ("tools/kb/main.go", True, "trình sinh các chỉ mục kb/"),
    ("kb/INDEX.yaml", False, "tài liệu, không phải mã"),
    ("docs/ui-ux/15-phu-luc.md", False, "đặc tả giao diện, không phải mã"),
]


def chay_thuan() -> list[tuple[str, str, bool, bool]]:
    """Trả về các ca SAI của is_code. Import tại chỗ: hook thêm thư mục của nó vào sys.path."""
    sys.path.insert(0, HOOKS)
    import stop_verify_guard as svg  # noqa: E402

    sai = []
    for duong, mong, nhan in IS_CODE_CASES:
        duoc = svg.is_code(duong)
        ok = duoc == mong
        mark = "  OK   " if ok else "  FAIL "
        want = "CODE " if mong else "BỎ QUA"
        print(f"{mark} [{want}] {'stop_verify_guard.is_code':24s} {nhan}")
        if not ok:
            sai.append((duong, nhan, mong, duoc))
    return sai


def run(hook: str, payload: dict) -> int:
    p = os.path.join(HOOKS, hook + ".py")
    r = subprocess.run([sys.executable, p], input=json.dumps(payload, ensure_ascii=False),
                       capture_output=True, text=True, encoding="utf-8", cwd=ROOT)
    return r.returncode


if __name__ == "__main__":
    fails = []
    for hook, label, expect, payload in CASES:
        got = run(hook, payload)
        ok = (got == expect)
        mark = "  OK   " if ok else "  FAIL "
        want = "BLOCK" if expect == BLOCK else "PASS "
        print(f"{mark} [{want}] {hook:24s} {label}")
        if not ok:
            fails.append((hook, label, expect, got))

    sai_thuan = chay_thuan()

    tong = len(CASES) + len(IS_CODE_CASES)
    hong = len(fails) + len(sai_thuan)
    print()
    print(f"Total: {tong} cases · passed: {tong-hong} · failed: {hong}")
    if fails:
        print("\nCa sai:")
        for hook, label, expect, got in fails:
            print(f"  {hook} — {label}: expected exit={expect}, got exit={got}")
    if sai_thuan:
        print("\nCa sai (hàm thuần):")
        for duong, nhan, mong, duoc in sai_thuan:
            print(f"  is_code({duong!r}) — {nhan}: muốn {mong}, nhận {duoc}")
    sys.exit(1 if hong else 0)
