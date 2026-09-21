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

# Hai hằng cho nhóm ca "ranh giới thẩm quyền" — xem khối chú thích của nhóm ấy trong CASES.
# TÍNH RA chứ không viết cứng: một đường dẫn viết cứng chỉ đúng trên máy người viết nó, và
# ngày nó sai thì ca "VẪN chặn" xanh vì đường dẫn không còn trong kho — xanh sai lý do.
ROOT_URL = ROOT.replace("\\", "/")
KHO_KHAC = os.path.join(os.path.dirname(ROOT), "mot-kho-hoan-toan-khac").replace("\\", "/")

BLOCK, PASS = 2, 0

# Windows terminals default to cp1252. Same reason as utf8_streams() in _common.
for _s in (sys.stdout, sys.stderr):
    try:
        _s.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass


def w(path: str, content: str) -> dict:
    return {"tool_name": "Write", "tool_input": {"file_path": path, "content": content}}


def e(path: str, cu: str, moi: str) -> dict:
    """Một `Edit` vào tệp ĐANG CÓ trên đĩa — không phải một lần ghi mới.

    Ca này tồn tại vì `doc_guard` từng chặn MỌI `Edit` vào `kb/`: nó chấm lần sửa bằng
    `new_string`, mà một lần sửa ba dòng ở giữa tệp không bao giờ mang theo frontmatter. Đường
    vòng duy nhất còn lại là ghi đè toàn tệp — tức một rào dựng để bảo vệ tầng tri thức lại đẩy
    mọi người viết về đúng thao tác có thể xoá sạch nó, và ngày 17/09/2026 nó suýt xoá thật.
    """
    return {"tool_name": "Edit",
            "tool_input": {"file_path": path, "old_string": cu, "new_string": moi}}


def b(cmd: str) -> dict:
    return {"tool_name": "Bash", "tool_input": {"command": cmd}}


def mon(cmd: str) -> dict:
    """The SAME command arriving as `Monitor` instead of `Bash`.

    Monitor hands its `command` to the same shell as Bash, but under a different tool name.
    A PreToolUse matcher of "Bash" therefore never saw it — and neither did `permissions.deny`,
    which is keyed on the tool name too, so every deny entry written as `Bash(...)` had a door
    beside it. These cases are what keeps the two paths from drifting apart again."""
    return {"tool_name": "Monitor", "tool_input": {"command": cmd}}


def td(module: str, them: dict) -> dict:
    """Một lần ghi vào sổ tiến độ, DỰNG TỪ NỘI DUNG THẬT trên đĩa cộng đúng một mục.

    Bản đầu của mấy ca này viết payload cứng và nhằm vào một module "chắc chưa có sổ". Nửa
    tiếng sau một phiên song song tạo đúng tệp ấy: ca PASS đỏ, còn ca BLOCK cạnh nó thì vẫn
    xanh — nhưng xanh vì luật CHỐNG XOÁ bắt, không phải vì luật nó định kiểm. Một ca xanh sai
    lý do là ca đã chết mà không ai biết.

    Đọc đĩa thì luật chống-xoá luôn được thoả, nên mỗi ca chỉ còn chấm đúng thứ nó nói.
    """
    p = os.path.join(ROOT, "kb", "90-ephemeral", "tien-do", f"{module}.json")
    try:
        with open(p, encoding="utf-8") as f:
            d = json.load(f)
    except Exception:
        d = {"module": module, "cap_nhat": "2026-09-20", "muc": []}
    d["muc"] = list(d.get("muc", [])) + [them]
    return w(f"kb/90-ephemeral/tien-do/{module}.json", json.dumps(d, ensure_ascii=False))


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
    # THE CASE THAT WAS MISSING FOR THE WHOLE LIFE OF THIS GUARD, and it is the one that matters:
    # `database/sql` offers every method twice, and this repository uses the Context form
    # everywhere. DB_CALL required `Query(` / `Exec(` with no suffix, so it matched none of the
    # 20 real call sites — the guard rule 1 names as its BLOCK enforcement saw nothing at all,
    # while `make check` stayed 7/7. Measured 2026-09-20, before the fix.
    ("tenant_scope_guard", "unscoped QueryContext — the form this repo really uses", BLOCK,
     w("donthu/internal/store/phien.go",
       "func (s *Store) Moi(ctx context.Context, id string) {\n"
       "\trows, err := s.raw.QueryContext(ctx, `SELECT id FROM phien WHERE cong_dan_id = $1`, id)\n}")),
    # A reason worth writing is longer than one line, and Go puts a declaration between the
    # comment and the statement. Demanding adjacency would flag a correctly annotated query and
    # teach the author to move code to please a hook — which is how a guard gets switched off.
    ("tenant_scope_guard", "@cross-tenant block separated by a declaration", PASS,
     w("danhtinh/internal/store/crosstenant/cong_dan.go",
       "\t// @cross-tenant: bảng này có một dòng cho mỗi công dân trên TOÀN NỀN TẢNG và không có\n"
       "\t// cột tenant_id nào để phạm vi hoá (ADR 0002). Trả về nhiều nhất một dòng.\n"
       "\tvar id string\n"
       "\terr := tx.QueryRowContext(ctx,\n"
       "\t\t`SELECT id FROM dinh_danh_cong_dan WHERE so_dien_thoai = $1`, so).Scan(&id)")),
    # ONE MARK, ONE QUERY. Rule 1 forbidden #6 asks each cross-commune read to carry its own
    # reason; a mark that covered everything below it until the closing brace would turn one
    # sentence into a blanket exemption for a whole function.
    ("tenant_scope_guard", "a second query does not inherit the first one's mark", BLOCK,
     w("baocao/internal/app/huyen.go",
       "\t// @cross-tenant: tổng hợp cấp huyện, chỉ số liệu\n"
       "\trows, _ := s.raw.QueryContext(ctx, tongHop)\n"
       "\tthem, _ := s.raw.QueryContext(ctx, `SELECT ho_ten FROM nguoi_dung`)")),
    # The commune is in the statement, as the first column of the INSERT — just not in the six
    # lines around the call. Blocking this would make the guard fire every time anyone edited a
    # correctly scoped write, and the exemption stays narrow: the constant must name tenant_id.
    ("tenant_scope_guard", "commune inside the named SQL constant", PASS,
     w("danhtinh/internal/store/phien_cong_dan.go",
       "const chenPhien = `INSERT INTO phien_cong_dan (tenant_id, id, bam_token) VALUES ($1,$2,$3)`\n\n"
       "func (s *Store) Tao(ctx context.Context, tx *sql.Tx, xa, sid, bam string) error {\n"
       "\t_, err := tx.ExecContext(ctx, chenPhien, xa, sid, bam)\n\treturn err\n}")),

    # ---- ranh giới THẨM QUYỀN: rào chắn của kho này chỉ phán xử kho này -------
    #
    # Ngày 20/09/2026 việc này đã chặn thật. Một phiên mở ở kho ViGov ghi tệp sang kho
    # `vihat-miniapp` (sản phẩm khác, KHÔNG có xã, KHÔNG có core/): `tenant_scope_guard` nhận
    # diện dịch vụ theo HÌNH DẠNG đường dẫn (`<đoạn>/internal/…`) nên coi nó là dịch vụ ViGov
    # và đòi `tenant_id`; `env_contract_guard` đòi mọi lần đọc môi trường phải nằm trong
    # `core/config` — gói mà kho kia cố ý không có.
    #
    # BỐN CA, VÀ HAI CA "VẪN CHẶN" QUAN TRỌNG NGANG HAI CA KIA: một bản vá kiểu này chết bằng
    # cách nới quá tay, và lúc ấy cả hai rào chắn im lặng trên chính ViGov mà không ai thấy.
    ("tenant_scope_guard", "đường dẫn TUYỆT ĐỐI trong kho ViGov — VẪN chặn", BLOCK,
     w(ROOT_URL + "/donthu/internal/app/list.go",
       "func (s *Svc) List(ctx context.Context) {\n\trows, err := s.db.Find(ctx, filter)\n}")),
    ("tenant_scope_guard", "cùng khuyết tật ấy ở KHO KHÁC — không phải việc của luật 1", PASS,
     w(KHO_KHAC + "/internal/store/kho.go",
       "func (k *Kho) Lay(ctx context.Context) {\n\trows, err := k.pool.Query(ctx, cau)\n}")),
    ("env_contract_guard", "đường dẫn TUYỆT ĐỐI trong kho ViGov — VẪN chặn", BLOCK,
     w(ROOT_URL + "/donthu/internal/app/a.go",
       'func init() {\n\tdsn := os.Getenv("DATABASE_DSN")\n}')),
    ("env_contract_guard", "kho khác tự đọc môi trường của chính nó", PASS,
     w(KHO_KHAC + "/internal/config/config.go",
       'func Nap() {\n\tdsn := os.Getenv("DATABASE_DSN")\n}')),

    # ---- `vihat-miniapp` phải nằm CÙNG CẤP với kho này -------------------------
    #
    # Mini App nằm ở HAI kho, và một phiên chỉ nhìn thấy nửa ở đây sẽ kết luận từ nửa ấy. Đã
    # xảy ra thật: endpoint webhook Zalo được viết vào `service-platform` ngày 20/09/2026 và
    # sống ở đó tới hôm sau mới bị gỡ (ADR 0032). Mã biên dịch được, test xanh, chỉ là nó nằm
    # ở kho sai — không rào chắn nào bắt được, vì không rào chắn nào biết kho kia tồn tại.
    #
    # CA CHẶN KHÔNG DỰA VÀO VIỆC KHO CÓ MẶT HAY KHÔNG, và đó là điều làm nó chạy được trên
    # mọi máy: nó chấm LUẬT VỊ TRÍ — một `vihat-miniapp` nằm chỗ khác thì sai dù nó có tồn
    # tại hay không. Một ca dựa vào sự VẮNG MẶT của kho sẽ xanh trên máy chưa clone và đỏ
    # trên máy đã clone, tức xanh vì lý do sai đúng nửa số máy.
    ("miniapp_sibling_guard", "bản sao đặt ở chỗ khác, không cùng cấp", BLOCK,
     w(KHO_KHAC + "/vihat-miniapp/internal/httpapi/api.go", "package httpapi\n")),
    ("miniapp_sibling_guard", "sửa citizen-app khi kho ấy có mặt đúng chỗ", PASS,
     w("citizen-app/src/content/company-profile.ts", "export const COMPANY = {} as const\n")),
    ("miniapp_sibling_guard", "tệp ViGov bình thường, không thuộc chủ đề Mini App", PASS,
     w("service-petitions/internal/app/a.go", "package app\n")),

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
    # Ca vá ngày 17/09/2026. Sửa ba dòng ở GIỮA một tệp kb/ đang có frontmatter hợp lệ phải
    # PASS: frontmatter là tính chất của TỆP, không phải của lần sửa. Chấm bằng diff thì mọi
    # Edit vào kb/ đều bị chặn, và đường vòng duy nhất còn lại — ghi đè toàn tệp — là đúng thao
    # tác có thể xoá mất việc của người khác. Dùng một tệp THẬT trên đĩa, vì cái đang được kiểm
    # chính là "hook có chịu đọc đĩa không".
    ("doc_guard", "sửa ba dòng giữa một tệp kb/ đã có frontmatter", PASS,
     e("kb/00-foundation/multi-tenant-model.md",
       "## Cấu hình theo xã", "## Cấu hình theo xã (ghi chú)")),
    # Chiều ngược lại vẫn phải chặn: tạo MỚI một tệp kb/ không frontmatter thì không có gì trên
    # đĩa để đọc, và fallback về văn bản đưa vào — đúng như tạo tệp mới nghĩa là vậy.
    ("doc_guard", "tạo tệp kb/ mới không frontmatter qua Edit", BLOCK,
     e("kb/00-foundation/khong-ton-tai-abc.md", "", "# Không có frontmatter")),

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
    ("citizen_commitment_guard", "deadline asked of identity, which owns the calendar", PASS,
     w("petitions/internal/app/sla.go",
       "func deadline(ctx context.Context, from time.Time) (time.Time, error) {\n"
       "\treturn sla.TienGioLamViec(ctx, from, gioXuLyXong)\n}")),
    # THE CASE THE 2026-09-20 RE-CALIBRATION ADDED. `gio * time.Hour` is what an implementer
    # writes the moment the SLA table is stated in hours, and every earlier pattern let it
    # through: CALENDAR_DAYS only knew the multiples 24/48/72. A 2-hour commitment counted
    # this way falls due at 09:30 on a Sunday, and the report says the commune met it.
    ("citizen_commitment_guard", "deadline built by adding a duration locally", BLOCK,
     w("petitions/internal/app/sla.go",
       "func deadline(from time.Time, gio int) time.Time {\n"
       "\treturn from.Add(time.Duration(gio) * time.Hour) // sla\n}")),
    # identity OWNS the arithmetic (ADR 0007) — the same line there is the correct code.
    ("citizen_commitment_guard", "identity itself may add a duration", PASS,
     w("service-identity/internal/domain/tien_gio_lam_viec.go",
       "func tien(from time.Time, gio int) time.Time {\n"
       "\treturn from.Add(time.Duration(gio) * time.Hour) // sla\n}")),
    ("citizen_commitment_guard", "statutory calendar days, declared", PASS,
     w("petitions/internal/app/khieunai.go",
       "// @sla-ok: Law on Complaints art. 28 counts calendar days\n"
       "func due(t time.Time) time.Time { return t.AddDate(0, 0, 30) } // sla")),

    # --- rule 11 — the infrastructure configuration contract ---
    #
    # The `.env.example` check needs the real template on disk, so these cases use variables
    # that are genuinely declared there (DATABASE_DSN) and genuinely absent (KAFKA_*). A case
    # that invented its own template would go green the day the template moved, which is the
    # failure shape this file has already been bitten by.
    ("env_contract_guard", "os.Getenv ngoài core/config", BLOCK,
     w("donthu/internal/http/h.go",
       "func h() {\n\taddr := os.Getenv(\"DATABASE_DSN\")\n}")),
    ("env_contract_guard", "core/config được đọc môi trường", PASS,
     w("core/config/config.go",
       "func Load() {\n\tdsn := os.Getenv(\"DATABASE_DSN\")\n}")),
    # The mechanism the user bought: the role is named in code, the cluster is chosen in the
    # manifest. A cluster ordinal in source throws it away — moving a workload then costs a
    # release of every image that reads it.
    ("env_contract_guard", "số cụm trong tên biến mã đọc", BLOCK,
     w("core/config/config.go",
       "func Load() {\n\tk := os.Getenv(\"KAFKA_02_ADDRESS\")\n}")),
    ("env_contract_guard", "dấu gạch ngang trong tên biến môi trường", BLOCK,
     w("core/config/config.go",
       "func Load() {\n\tk := os.Getenv(\"POSTGRESQL-HOST-AND-PORT\")\n}")),
    ("env_contract_guard", "biến không có dòng nào trong .env.example", BLOCK,
     w("core/config/config.go",
       "func Load() {\n\tk := os.Getenv(\"KAFKA_LOG_ADDRESS\")\n}")),
    # `REDIS_DB_0` is Redis database ZERO — a value, not an instance. The first version of the
    # pattern flagged it, and the comment beside that pattern claimed it would not; both were
    # fixed together. A number in the MIDDLE names an instance, a number at the END qualifies
    # the value.
    # The "number at the end is a VALUE, not an instance" distinction is tested against the
    # pattern itself, in SO_CUM_CASES below — not here. A payload case for it would be blocked
    # by the .env.example check instead and go red for a reason it does not name, which is the
    # failure this repository keeps finding in its own gates.
    ("env_contract_guard", "lối thoát @env-ok có lý do", PASS,
     w("donthu/internal/http/h.go",
       "func h() {\n\t// @env-ok: công cụ dựng, chạy ngoài tiến trình phục vụ\n"
       "\taddr := os.Getenv(\"DATABASE_DSN\")\n}")),

    # ---- tầng tiến độ · progress_guard ---------------------------------------
    #
    ("progress_guard", "khai xong mà không có bằng chứng", BLOCK,
     td("core", {"id": "ca-kiem-thu-rao", "viec": "ca thử rào",
                 "trang_thai": "xong", "bang_chung": ""})),
    ("progress_guard", "trang_thai ngoài bảng", BLOCK,
     td("core", {"id": "ca-kiem-thu-rao", "viec": "ca thử rào",
                 "trang_thai": "gan_xong", "bang_chung": ""})),
    ("progress_guard", "nợ một câu hỏi đã DECIDED", BLOCK,
     td("core", {"id": "ca-kiem-thu-rao", "viec": "ca thử rào",
                 "trang_thai": "chua_lam", "bang_chung": "", "no_confirm": [2]})),
    ("progress_guard", "module không có trên đĩa", BLOCK,
     w("kb/90-ephemeral/tien-do/service-khong-ton-tai.json",
       '{"module": "service-khong-ton-tai", "cap_nhat": "2026-09-20", "muc": []}')),
    ("progress_guard", "ghi tay vào tệp SINH RA", BLOCK,
     w("kb/90-ephemeral/tien-do.md", "# Tiến độ\n\nsửa tay một dòng\n")),
    # Ca nặng nhất: ghi đè trọn tệp từ một bản đọc cũ thì mục của agent trước biến mất, và
    # không có gì đỏ khi nó xảy ra. Ca này cố ý KHÔNG đi qua `td()` — nó phải đọc tệp thật để
    # có cái mà làm mất. Nó đỏ nếu `core.json` bị bỏ trống, và đó là ý đồ: một sổ tiến độ rỗng
    # cũng là dữ liệu đã mất.
    ("progress_guard", "ghi đè làm biến mất mục đang có", BLOCK,
     w("kb/90-ephemeral/tien-do/core.json",
       '{"module": "core", "cap_nhat": "2026-09-20", "muc": []}')),
    ("progress_guard", "thêm một mục hợp lệ, giữ nguyên mục cũ", PASS,
     td("core", {"id": "ca-kiem-thu-rao", "viec": "ca thử rào",
                 "trang_thai": "chua_lam", "bang_chung": "",
                 "no_confirm": [], "tiep_theo": "—"})),
    ("progress_guard", "mã thường không liên quan tới tầng tiến độ", PASS,
     w("service-comms/internal/app/zns.go", "func Send(ctx context.Context) error { return nil }")),
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


# ---- pure-function cases: what drift_guard READS, and when it speaks -----------------------
#
# Same exemption, same trap, second hook. drift_guard is the only mechanism watching for the
# code silently deciding a customer's open question — the failure that cost v1 20-28 days — and
# it shipped with a WHITELIST of seven root directories. After the flat layout exactly one of
# them existed, so it read almost nothing and said nothing, session after session, while
# check_brain stayed green: that invariant checks a rule NAMES a hook, never that the hook can
# SEE anything.
#
# What gets read is the whole hook. These cases name one real path per shape a deployable unit
# takes, so reverting to a whitelist goes red on the spot.
DUOC_QUET_CASES = [
    ("service-identity/internal/http/routes.go", True, "mã dịch vụ — ca từng thủng"),
    ("service-identity/migrations/0004_x.sql", True, "migration của dịch vụ — ca từng thủng"),
    ("core/httpx/citizen.go", True, "mã dùng chung — ca từng thủng"),
    ("web-admin/src/lib/api/can-bo.ts", True, "mã web — ca từng thủng"),
    ("citizen-app/src/content/company-profile.ts", True, "Mini App — đơn vị triển khai mới nhất"),
    ("proto/vigov/platform/v1/platform.proto", True, "hợp đồng — thư mục DUY NHẤT từng được quét"),
    ("core/gen/vigov/platform/v1/platform.pb.go", False, "mã SINH — không quyết định gì"),
    ("core/httpx/citizen_test.go", False, "tệp test"),
    ("kb/10-decisions/0023-thuat-ngu.md", False, "tài liệu — phát biểu VỀ quyết định"),
    ("docs/ui-ux/11-noi-dung-mini-app.md", False, "đặc tả giao diện"),
    ("web-admin/node_modules/react/index.js", False, "phụ thuộc bên thứ ba"),
]

# Ngưỡng của TỪNG tín hiệu phải được dùng thật. Luật cũ dùng `top_n < 3` cứng, nên mọi
# "threshold": 1 trong open-questions.json là chữ chết — mà 1 đúng là ngưỡng các câu ĐẮT NHẤT
# khai báo, vì với chúng một lần xuất hiện đã là quyết định.
# Tín hiệu nằm trong CHÚ THÍCH không phải là quyết định. Ca đầu là ca thật, lấy nguyên văn từ
# 0003_nguoi_dung_co_tai_khoan.sql: một khối giải thích VÌ SAO CỐ Ý KHÔNG đặt ràng buộc, kèm
# mẫu ALTER TABLE đã bị chú thích cho sau này. Bản vá drift_guard đầu tiên đếm cả dòng ấy và
# bắn cảnh báo — tức phạt đúng tệp lập luận cẩn thận nhất kho, vì nó đã giải thích lý do.
# Kho này đã biết hình dạng đó: tenant_scope_guard có ca "khai báo trong chú thích SQL → PASS".
BO_CHU_THICH_CASES = [
    ("x.sql", "--          CHECK (NOT co_tai_khoan OR mat_khau_hash <> '');",
     "CHECK", False, "mẫu ràng buộc ĐÃ BỊ CHÚ THÍCH — ca âm tính giả thật"),
    ("x.sql", "ALTER TABLE nguoi_dung ADD CONSTRAINT x CHECK (a);",
     "CHECK", True, "ràng buộc THẬT — vẫn phải đếm"),
    ("x.go", "// tenantID := \"xa-tan-phu\"", "xa-tan-phu", False, "mã Go đã chú thích"),
    ("x.go", "tenantID := \"xa-tan-phu\"", "xa-tan-phu", True, "mã Go thật"),
    ("x.go", "/* nhiều dòng\n tenantID = \"xa-tan-phu\"\n */", "xa-tan-phu", False, "chú thích khối"),
    ("x.ts", "const u = \"https://vigov.vn/api\"; // ghi chú", "vigov.vn", True,
     "https:// trong chuỗi KHÔNG được cắt mất phần trước nó"),
    # DẤU KHAI BÁO không phải văn xuôi. `@cross-tenant` bắt buộc phải nằm trong chú thích
    # (luật 1 cấm #6) và là mẫu tín hiệu DUY NHẤT của câu mở #4 — bỏ nó đi cùng văn xuôi là
    # làm hook vĩnh viễn không báo được câu ấy, trong khi vẫn trông như đang canh.
    ("x.go", "// @cross-tenant: picker phải hiện xã TRƯỚC khi biết xã", "@cross-tenant", True,
     "DẤU khai báo trong chú thích — phải GIỮ"),
    ("x.sql", "-- @entity: CitizenIdentity", "@entity", True, "dấu @entity trong SQL — phải giữ"),
    ("x.go", "// giải thích dài dòng, không có dấu nào", "giải thích", False,
     "văn xuôi thuần trong cùng loại chú thích — vẫn bỏ"),
]

NEN_CANH_BAO_CASES = [
    ({"A": 1}, {"A": 1}, True, "ngưỡng 1 và có 1 — phải bắn (ca từng chết)"),
    ({"A": 2}, {}, False, "không khai ngưỡng, mặc định 3, mới có 2 — im"),
    ({"A": 5}, {}, True, "mặc định 3, có 5, một chiều — bắn"),
    ({"A": 5, "B": 4}, {}, False, "hai chiều ngang nhau — mã chưa nhất quán, không phải đã quyết"),
    ({"A": 9, "B": 1}, {}, True, "một chiều áp đảo — bắn"),
    ({}, {}, False, "không tín hiệu nào"),
]


# env_contract_guard.SO_CUM — a cluster ordinal, or a number that qualifies the value.
#
# THE DISTINCTION IS THE WHOLE RULE: a number wedged BETWEEN two words names which instance
# (`KAFKA_02_ADDRESS`), and rule 11 forbidden #2 keeps that out of source so a workload can be
# moved in the manifest. A number at the END qualifies the value (`REDIS_DB_0` is Redis
# database zero) and is legitimate.
#
# The first version of the pattern flagged `REDIS_DB_0` while the comment beside it claimed it
# would not. These cases exist so the pattern and its comment cannot disagree again.
SO_CUM_CASES = [
    ("KAFKA_02_ADDRESS", True, "số cụm ở giữa — tên cụm nằm trong mã"),
    ("REDIS_1_DSN", True, "số cụm một chữ số"),
    ("ES_03_ADDRS", True, "số cụm có số 0 đứng đầu"),
    ("REDIS_DB_0", False, "số CUỐI là số database, không phải số cụm"),
    ("KAFKA_LOG_PARTITIONS_3", False, "số CUỐI là một phép đếm"),
    ("ELASTICSEARCH_ADDRS", False, "không có số nào"),
    ("DATABASE_DSN", False, "không có số nào"),
]


def chay_thuan() -> list[tuple[str, str, bool, bool]]:
    """Trả về các ca SAI của phần THUẦN. Import tại chỗ: hook tự thêm thư mục của nó vào sys.path."""
    sys.path.insert(0, HOOKS)
    import stop_verify_guard as svg  # noqa: E402
    import drift_guard as dg  # noqa: E402
    import env_contract_guard as ecg  # noqa: E402

    sai = []

    for ten, mong, nhan in SO_CUM_CASES:
        duoc = bool(ecg.SO_CUM.search(ten))
        ok = duoc == mong
        mark = "  OK   " if ok else "  FAIL "
        want = "CỤM " if mong else "GIÁ TRỊ"
        print(f"{mark} [{want}] {'env_contract_guard.SO_CUM':24s} {nhan}")
        if not ok:
            sai.append((ten, nhan, mong, duoc))
    for duong, mong, nhan in IS_CODE_CASES:
        duoc = svg.is_code(duong)
        ok = duoc == mong
        mark = "  OK   " if ok else "  FAIL "
        want = "CODE " if mong else "BỎ QUA"
        print(f"{mark} [{want}] {'stop_verify_guard.is_code':24s} {nhan}")
        if not ok:
            sai.append((duong, nhan, mong, duoc))

    for duong, mong, nhan in DUOC_QUET_CASES:
        duoc = dg.duoc_quet(duong)
        ok = duoc == mong
        mark = "  OK   " if ok else "  FAIL "
        want = "ĐỌC " if mong else "BỎ QUA"
        print(f"{mark} [{want}] {'drift_guard.duoc_quet':24s} {nhan}")
        if not ok:
            sai.append((duong, nhan, mong, duoc))

    for ten, noi_dung, mau, mong, nhan in BO_CHU_THICH_CASES:
        duoc = mau in dg.bo_chu_thich(noi_dung, ten)
        ok = duoc == mong
        mark = "  OK   " if ok else "  FAIL "
        want = "ĐẾM " if mong else "BỎ QUA"
        print(f"{mark} [{want}] {'drift_guard.bo_chu_thich':24s} {nhan}")
        if not ok:
            sai.append((noi_dung[:40], nhan, mong, duoc))

    for counts, nguong, mong, nhan in NEN_CANH_BAO_CASES:
        duoc = dg.nen_canh_bao(counts, nguong) is not None
        ok = duoc == mong
        mark = "  OK   " if ok else "  FAIL "
        want = "BẮN " if mong else "IM   "
        print(f"{mark} [{want}] {'drift_guard.nen_canh_bao':24s} {nhan}")
        if not ok:
            sai.append((str(counts), nhan, mong, duoc))

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

    tong = (len(CASES) + len(IS_CODE_CASES) + len(DUOC_QUET_CASES)
            + len(BO_CHU_THICH_CASES) + len(NEN_CANH_BAO_CASES))
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
