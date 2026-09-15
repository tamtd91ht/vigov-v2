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


# (hook, case label, expected exit code, payload)
CASES = [
    # ---- rule 1 · tenant isolation -------------------------------------------
    ("tenant_scope_guard", "query without tenant scope", BLOCK,
     w("services/donthu/internal/app/list.go",
       "func (s *Svc) List(ctx context.Context) {\n\trows, err := s.db.Find(ctx, filter)\n}")),
    ("tenant_scope_guard", "goes through a scoped repository", PASS,
     w("services/donthu/internal/app/list.go",
       "func (s *Svc) List(ctx context.Context) {\n\trepo := s.scoped(ctx)\n\trows, err := repo.Find(ctx, filter)\n}")),
    ("tenant_scope_guard", "single-column unique key", BLOCK,
     w("services/donthu/internal/store/schema.go",
       "// UNIQUE (code)\nconst ddl = `CREATE TABLE don_thu (code text, UNIQUE (code))`")),
    ("tenant_scope_guard", "composite unique key", PASS,
     w("services/donthu/internal/store/schema.go",
       "const ddl = `CREATE TABLE don_thu (tenant_id text, code text, UNIQUE (tenant_id, code))`")),
    ("tenant_scope_guard", "tenant taken from the client", BLOCK,
     w("services/donthu/internal/http/h.go",
       'func h(w http.ResponseWriter, r *http.Request) {\n\ttid := r.URL.Query().Get("tenant_id")\n}')),
    ("tenant_scope_guard", "explicit @cross-tenant escape", PASS,
     w("services/baocao/internal/app/huyen.go",
       "// @cross-tenant: tổng hợp phản ánh cấp huyện, chỉ số liệu, đã ghi nhật ký\n\trows, _ := s.db.Find(ctx, f)")),

    # ---- rule 2 · service boundary ---------------------------------------
    ("service_boundary_guard", "imports another service's internal", BLOCK,
     w("services/baocao/internal/app/a.go",
       'import (\n\t"vigov/services/donthu/internal/store"\n)')),
    ("service_boundary_guard", "imports a shared package", PASS,
     w("services/baocao/internal/app/a.go",
       'import (\n\t"vigov/pkg/tenant"\n)')),
    ("service_boundary_guard", "hand-edits a proto-generated file", BLOCK,
     w("kb/20-contracts/grpc/donthu.pb.go", "package pb\n// sửa tay")),

    # ---- rule 3 · personal data -----------------------------------------
    ("pii_guard", "logs a phone number", BLOCK,
     w("services/congdan/internal/app/otp.go", "log.Info(citizenPhone)")),
    ("pii_guard", "logs a business code", PASS,
     w("services/congdan/internal/app/otp.go", 'log.Info("da gui otp", "code", dt.Code)')),
    ("pii_guard", "prints a whole struct", BLOCK,
     w("services/congdan/internal/app/otp.go", 'fmt.Printf("%+v", user)')),
    ("pii_guard", "hardcoded real phone number", BLOCK,
     w("services/congdan/internal/app/seed.go", 'phone := "0912345678"')),
    ("pii_guard", "agreed fake number", PASS,
     w("services/congdan/internal/app/seed.go", 'phone := "0900000000"')),
    ("pii_guard", "personal data in .json", BLOCK,
     w("services/congdan/seed/cd.json", '[{"phone":"0912345678","cccd":"079123456789"}]')),

    # ---- rule 4 · citizen isolation ----------------------------------------
    ("citizen_scope_guard", "identity from the query string", BLOCK,
     w("services/congdan/internal/http/citizen.go",
       'phone := r.URL.Query().Get("phone")')),
    ("citizen_scope_guard", "identity from the session", PASS,
     w("services/congdan/internal/http/citizen.go",
       "cit := auth.CitizenFrom(ctx)")),

    # ---- rule 5 · authorisation ----------------------------------------------
    ("rbac_guard", "route with no permission", BLOCK,
     w("services/donthu/internal/http/router.go",
       'r.Get("/don-thu", h.List)')),
    ("rbac_guard", "route with a permission", PASS,
     w("services/donthu/internal/http/router.go",
       'r.With(auth.RequirePermission("donthu", "view")).Get("/don-thu", h.List)')),
    ("rbac_guard", "adjacent routes, second one unguarded", BLOCK,
     w("services/donthu/internal/http/router.go",
       'r.With(auth.RequirePermission("donthu", "view")).Get("/don-thu", h.List)\n'
       'r.Delete("/don-thu/{id}", h.Remove)')),
    ("rbac_guard", "Public() with no reason", BLOCK,
     w("services/donthu/internal/http/router.go",
       'r.With(auth.Public()).Get("/tra-cuu", h.Lookup)')),

    # ---- rule 6 · audit trail -------------------------------------------------
    ("audit_guard", "write with no audit entry", BLOCK,
     w("services/donthu/internal/app/update.go",
       "func (s *Svc) Update(ctx context.Context) error {\n\treturn s.repo.Save(ctx, dt)\n}")),
    ("audit_guard", "write with an audit entry", PASS,
     w("services/donthu/internal/app/update.go",
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
    ("data_safety_guard", "DELETE FROM on business data", BLOCK,
     w("services/donthu/internal/store/q.go", 'const q = "DELETE FROM don_thu WHERE id=$1"')),

    # ---- rule 8 · secrets --------------------------------------------------
    ("secret_scan", "hardcoded secret", BLOCK,
     w("services/donthu/internal/cfg.go", 'const jwtSecret = "sieu-bi-mat-1234567"')),
    ("secret_scan", "read from an environment variable", PASS,
     w("services/donthu/internal/cfg.go", 'jwtSecret := os.Getenv("JWT_SECRET")')),
    ("secret_scan", "real connection string in documentation", BLOCK,
     w("kb/40-runbooks/khoi-phuc.md",
       "dsn: postgres://admin:P4ssw0rd-that-is-real@10.0.0.5:5432/vigov")),
    ("secret_scan", "real value in a template file", BLOCK,
     w("deploy/.env.example", "JWT_SECRET=aX9fQ2mZ7pL1kR4tY8wB")),
    ("secret_scan", "placeholder in a template file", PASS,
     w("deploy/.env.example", "JWT_SECRET=change-me-truoc-khi-chay")),

    # ---- bash_content_guard · closing the detour ------------------------------
    ("bash_content_guard", "secret written via heredoc", BLOCK,
     b('cat > services/a/cfg.go <<EOF\nconst api_key = "sk-live-abcdefghijk"\nEOF')),
    ("bash_content_guard", "personal-data log written via redirect", BLOCK,
     b('echo "log.Info(citizenPhone)" >> services/a/x.go')),
    ("bash_content_guard", "writes harmless content", PASS,
     b('echo "package main" > services/a/x.go')),
    ("bash_content_guard", "a read command", PASS, b("cat services/a/x.go")),

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
     w("services/donthu/README.md", "# Service Đơn thư")),
]


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

    print()
    print(f"Total: {len(CASES)} cases · passed: {len(CASES)-len(fails)} · failed: {len(fails)}")
    if fails:
        print("\nCa sai:")
        for hook, label, expect, got in fails:
            print(f"  {hook} — {label}: expected exit={expect}, got exit={got}")
    sys.exit(1 if fails else 0)
