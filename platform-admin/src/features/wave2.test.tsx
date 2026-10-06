import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/xa",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
}));

import { ACCOUNT_ITEMS, NAV_ITEMS, SidebarView } from "@/components/sidebar";
import { ALL_OPS_KEYS, ApiError, type CommuneDetail, type LaunchLinks, type OperatorAuditEntry, type PetitionField, type UploadPolicy } from "@/lib/api";
import { communeError, handleGuardedError, launchLinkError } from "@/lib/errors";
import {
  canIssueQr,
  canManageMiniApps,
  canManagePetitionFields,
  canManageUploadPolicies,
  canReadConsole,
} from "@/lib/permissions";

import { CommuneDetailBody } from "./communes/commune-detail";
import { LaunchLinkView, SHARED_APP_PAGE, launchSourceLabel } from "./communes/launch-link-card";
import { OperatorLogMore, OperatorLogTable, PLATFORM_WIDE } from "./operator-log/operator-log";
import { actionLabel, compactValue, logRange, VALUE_MAX_CHARS } from "./operator-log/operator-log-model";
import {
  CODE_FOREVER_NOTICE,
  PetitionFieldActions,
  PetitionFieldFormFields,
  PetitionFieldTable,
  type FieldForm,
} from "./petition-fields/petition-field-screen";
import {
  DECLARE_NOTICE,
  DeclareSharedMiniAppFields,
  REPLACE_WARNING,
  SharedMiniAppView,
} from "./shared-mini-app/shared-mini-app-screen";
import {
  buildUploadPolicyChange,
  bytesToMegabytes,
  megabytesToBytes,
  type UploadPolicyForm,
} from "./upload-policies/upload-policy-model";
import { PROPAGATION_NOTICE, UNLIMITED_LABEL, UploadPolicyFormFields, UploadPolicyTable } from "./upload-policies/upload-policy-screen";

const TENANT = "ops.tenant.manage";
const DOMAIN = "ops.domain.manage";
const PROFILE = "ops.profile.manage";
const MINI_APP = "ops.mini_app.manage";
const UPLOAD = "ops.upload_policy.manage";
const QR = "ops.qr.issue";
const FIELD = "ops.petition_field.manage";
const SEVEN = [TENANT, DOMAIN, PROFILE, MINI_APP, UPLOAD, QR, FIELD];

const noop = () => {};

const MIB = 1024 * 1024;

const POLICY: UploadPolicy = {
  purpose: "petition-photo",
  max_bytes: 10 * MIB,
  allowed_mime_types: ["image/jpeg", "image/png"],
  max_files_per_subject: 5,
  updated_at: "2026-10-04T03:00:00Z",
  updated_by: "VH-00001",
  mime_choices: ["image/jpeg", "image/png", "image/webp"],
  max_bytes_cap: 50 * MIB,
};

const UNCLASSED: UploadPolicy = { ...POLICY, purpose: "future-purpose", mime_choices: [], max_bytes_cap: 0 };

const FIELD_ACTIVE: PetitionField = {
  code: "moi-truong",
  default_label: "Môi trường",
  sort_order: 10,
  icon: "Trash2",
  tone: "green",
  active: true,
};
const FIELD_RETIRED: PetitionField = { ...FIELD_ACTIVE, code: "cu", default_label: "Lĩnh vực cũ", active: false };

const COMMUNE: CommuneDetail = {
  id: "01J0000000000000000000000A",
  name: "Xã Kiểm Thử",
  province: "Tỉnh Kiểm Thử",
  active: true,
  domains: ["chinh.example.vn"],
  mini_apps: [],
  unbound_secrets: [],
};

const ENTRIES: OperatorAuditEntry[] = [
  {
    at: "2026-10-04T03:00:00Z",
    actor: "VH-00001",
    action: "upload_policy.changed",
    commune: null,
    subject: "petition-photo",
    before: { max_bytes: 10485760 },
    after: { max_bytes: 20971520 },
    reason: "Tăng giới hạn",
  },
  {
    at: "2026-10-03T03:00:00Z",
    actor: "VH-00002",
    action: "them_ten_mien",
    commune: { id: COMMUNE.id, name: COMMUNE.name },
    subject: "phu.example.vn",
    before: null,
    after: { ten_mien: "phu.example.vn" },
    reason: "",
  },
  {
    at: "2026-10-02T03:00:00Z",
    actor: "VH-00003",
    action: "verb.not_known_yet",
    commune: null,
    subject: "",
    before: null,
    after: null,
    reason: "",
  },
];

const err = (status: number, code: string) => new ApiError(status, code, "", "");

// --- permissions -----------------------------------------------------------------------------------

describe("reads: ANY ops key (ADR 0073 #1) — denied case first", () => {
  it("no key, or a string outside the closed set, reads nothing", () => {
    expect(canReadConsole([])).toBe(false);
    expect(canReadConsole(["ops.*"])).toBe(false);
    expect(canReadConsole(["ops.unknown.thing", "admin"])).toBe(false);
  });

  it("each of the seven keys alone reads the console", () => {
    // Eight since ADR 0074 #3 (`ops.zalo_bot.manage`, covered in zalo-bot/zalo-bot.test.tsx).
    expect(ALL_OPS_KEYS).toHaveLength(8);
    expect(ALL_OPS_KEYS).toContain(FIELD);
    for (const k of SEVEN) expect(canReadConsole([k])).toBe(true);
  });

  it("the sidebar lists no section without a key, and every section with any single key", () => {
    const none = renderToString(<SidebarView permissionKeys={[]} />);
    for (const item of NAV_ITEMS) expect(none).not.toContain(`href="${item.href}"`);
    // The account pages need only a session.
    for (const item of ACCOUNT_ITEMS) expect(none).toContain(`href="${item.href}"`);

    for (const k of SEVEN) {
      const html = renderToString(<SidebarView permissionKeys={[k]} />);
      for (const item of NAV_ITEMS) expect(html).toContain(`href="${item.href}"`);
    }
    const html = renderToString(<SidebarView permissionKeys={[QR]} />);
    for (const label of ["Danh sách xã", "Mini App dùng chung", "Lĩnh vực phản ánh", "Giới hạn tải lên", "Nhật ký vận hành"]) {
      expect(html).toContain(label);
    }
  });

  it("each write follows its own key only", () => {
    for (const k of SEVEN.filter((x) => x !== UPLOAD)) expect(canManageUploadPolicies([k])).toBe(false);
    for (const k of SEVEN.filter((x) => x !== QR)) expect(canIssueQr([k])).toBe(false);
    for (const k of SEVEN.filter((x) => x !== FIELD)) expect(canManagePetitionFields([k])).toBe(false);
    for (const k of SEVEN.filter((x) => x !== MINI_APP)) expect(canManageMiniApps([k])).toBe(false);
    expect(canManageUploadPolicies([UPLOAD])).toBe(true);
    expect(canIssueQr([QR])).toBe(true);
    expect(canManagePetitionFields([FIELD])).toBe(true);
  });

  it("a commune opened with only ops.mini_app.manage shows the record and the log, not the QR card", () => {
    const html = renderToString(
      <CommuneDetailBody commune={COMMUNE} permissionKeys={[MINI_APP]} onChanged={noop} onMiniAppAttached={noop} />,
    );
    expect(html).toContain("chinh.example.vn");
    expect(html).toContain("Nhật ký vận hành của xã");
    expect(html).not.toContain("Mã QR mở Mini App");
    const withQr = renderToString(
      <CommuneDetailBody commune={COMMUNE} permissionKeys={[QR]} onChanged={noop} onMiniAppAttached={noop} />,
    );
    expect(withQr).toContain("Mã QR mở Mini App");
  });
});

// --- upload limits ---------------------------------------------------------------------------------

const FORM: UploadPolicyForm = { megabytes: "12.5", mimeTypes: ["image/png", "image/jpeg"], limited: false, maxFiles: "", reason: "Tăng giới hạn" };

describe("upload limits: MB ↔ bytes and the PUT body", () => {
  it("1 MB = 1 048 576 bytes, both ways", () => {
    expect(megabytesToBytes("12.5")).toBe(13107200);
    expect(megabytesToBytes("12,5")).toBe(13107200);
    expect(megabytesToBytes("50")).toBe(50 * MIB);
    expect(bytesToMegabytes(50 * MIB)).toBe("50");
    expect(bytesToMegabytes(1572864)).toBe("1.5");
    for (const bad of ["", "0", "-1", "abc", "1.234", "1e3"]) expect(megabytesToBytes(bad)).toBeNull();
  });

  it("'Không giới hạn' sends null — never 0, never an absent key", () => {
    const r = buildUploadPolicyChange(FORM, POLICY);
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.body).toEqual({
      max_bytes: 13107200,
      // In the server's order of choices, not the order ticked.
      allowed_mime_types: ["image/jpeg", "image/png"],
      max_files_per_subject: null,
      reason: "Tăng giới hạn",
    });
    expect(Object.keys(r.body)).toContain("max_files_per_subject");
  });

  it("a count limit is sent as a number, in 1..100", () => {
    const r = buildUploadPolicyChange({ ...FORM, limited: true, maxFiles: "7" }, POLICY);
    expect(r.ok && r.body.max_files_per_subject).toBe(7);
    for (const bad of ["0", "101", "", "2.5"]) {
      const x = buildUploadPolicyChange({ ...FORM, limited: true, maxFiles: bad }, POLICY);
      expect(x.ok ? null : x.field).toBe("maxFiles");
    }
  });

  it("refuses over the cap, no type, no reason — each under its own field", () => {
    const over = buildUploadPolicyChange({ ...FORM, megabytes: "51" }, POLICY);
    expect(over.ok ? null : over.field).toBe("maxBytes");
    const none = buildUploadPolicyChange({ ...FORM, mimeTypes: [] }, POLICY);
    expect(none.ok ? null : none.field).toBe("mimeTypes");
    const offered = buildUploadPolicyChange({ ...FORM, mimeTypes: ["image/heic"] }, POLICY);
    expect(offered.ok ? null : offered.field).toBe("mimeTypes");
    const reason = buildUploadPolicyChange({ ...FORM, reason: "  " }, POLICY);
    expect(reason.ok ? null : reason.field).toBe("reason");
  });

  it("the table shows Sửa only with the key, and never on an unclassed purpose", () => {
    const denied = renderToString(<UploadPolicyTable items={[POLICY]} canEdit={false} onEdit={noop} />);
    expect(denied).toContain("petition-photo");
    expect(denied).toContain("10 MB");
    expect(denied).not.toContain("Sửa");
    const allowed = renderToString(<UploadPolicyTable items={[POLICY, UNCLASSED]} canEdit onEdit={noop} />);
    expect(allowed.match(/>Sửa</g)).toHaveLength(1);
    expect(renderToString(<UploadPolicyTable items={[{ ...POLICY, max_files_per_subject: null }]} canEdit={false} onEdit={noop} />)).toContain(
      UNLIMITED_LABEL,
    );
  });

  it("the dialog offers the server's choices and cap, and says 60 seconds", () => {
    const html = renderToString(<UploadPolicyFormFields policy={POLICY} form={FORM} onChange={noop} busy={false} error={null} />);
    for (const m of POLICY.mime_choices) expect(html).toContain(`value="${m}"`);
    expect(html).not.toContain('value="image/heic"');
    expect(html).toContain("50 MB");
    expect(PROPAGATION_NOTICE).toContain("60 giây");
    expect(html).toContain("Không giới hạn");
  });
});

// --- shared Mini App -------------------------------------------------------------------------------

describe("shared Mini App", () => {
  it("not declared: the sentence, and Khai báo only with ops.mini_app.manage", () => {
    const denied = renderToString(<SharedMiniAppView state={{ status: "none" }} canManage={false} onDeclare={noop} />);
    expect(denied).toContain("chưa khai báo");
    expect(denied).not.toContain("Khai báo App ID");
    expect(renderToString(<SharedMiniAppView state={{ status: "none" }} canManage onDeclare={noop} />)).toContain("Khai báo App ID");
  });

  it("declared: the App ID, and Đổi only with the key", () => {
    const state = { status: "ready", app: { app_id: "1234567", created_at: "2026-10-04T03:00:00Z", created_by: "VH-00001" } } as const;
    const denied = renderToString(<SharedMiniAppView state={state} canManage={false} onDeclare={noop} />);
    expect(denied).toContain("1234567");
    expect(denied).not.toContain("Đổi App ID");
    expect(renderToString(<SharedMiniAppView state={state} canManage onDeclare={noop} />)).toContain("Đổi App ID dùng chung");
  });

  it("replacing warns that printed QR codes and sign-ins through the old ID break", () => {
    const current = { app_id: "1234567", created_at: "2026-10-04T03:00:00Z", created_by: "VH-00001" };
    const html = renderToString(
      <DeclareSharedMiniAppFields current={current} appId="" reason="" onAppId={noop} onReason={noop} busy={false} error={null} />,
    );
    expect(html).toContain(REPLACE_WARNING);
    expect(REPLACE_WARNING).toMatch(/mã QR đã in/);
    expect(REPLACE_WARNING).toMatch(/đăng nhập/);
    const first = renderToString(
      <DeclareSharedMiniAppFields current={null} appId="" reason="" onAppId={noop} onReason={noop} busy={false} error={null} />,
    );
    expect(first).toContain(DECLARE_NOTICE);
    expect(first).not.toContain(REPLACE_WARNING);
  });
});

// --- QR launch link --------------------------------------------------------------------------------

describe("launch link: every 409 is a Vietnamese next step", () => {
  it.each([
    ["commune_inactive", false],
    ["commune_no_primary_domain", false],
    ["shared_mini_app_not_declared", true],
    ["shared_mini_app_ambiguous", false],
    ["own_mini_app_ambiguous", false],
  ])("%s", (code, toShared) => {
    const known = launchLinkError(err(409, code));
    expect(known?.toSharedAppPage).toBe(toShared);
    const text = handleGuardedError(err(409, code), "tạo mã QR", vi.fn(), (e) => launchLinkError(e)?.text ?? null);
    expect(text).toBe(known?.text);
    expect(text).not.toContain(code);
  });

  it("not declared links to the shared-app page; the others do not", () => {
    const html = renderToString(
      <LaunchLinkView state={{ status: "error", message: "x", toSharedAppPage: true }} onCopy={noop} copyNotice="" />,
    );
    expect(html).toContain(`href="${SHARED_APP_PAGE}"`);
    const other = renderToString(
      <LaunchLinkView state={{ status: "error", message: "x", toSharedAppPage: false }} onCopy={noop} copyNotice="" />,
    );
    expect(other).not.toContain(`href="${SHARED_APP_PAGE}"`);
  });

  const SHARED_LINK = {
    url: "https://zalo.me/s/1234567/?d=chinh.example.vn&src=qr",
    domain: "chinh.example.vn",
    app_id: "1234567",
    source: "chung" as const,
  };
  const OWN_LINK = { url: "https://zalo.me/s/4096/?src=qr", domain: "chinh.example.vn", app_id: "4096", source: "rieng" as const };
  const ready = (links: LaunchLinks) => renderToString(<LaunchLinkView state={{ status: "ready", links }} onCopy={noop} copyNotice="" />);

  it("a link renders as a QR, its URL and a copy button — no download", () => {
    const html = ready({ links: [SHARED_LINK], unavailable: [] });
    expect(html).toContain("<svg");
    expect(html).toContain("https://zalo.me/s/1234567/?d=chinh.example.vn&amp;src=qr");
    expect(html).toContain("Chép liên kết");
    expect(html).not.toMatch(/Tải về|PNG/);
  });

  it("a shared-app link is labelled as the ViHAT app and shows the primary domain", () => {
    const html = ready({ links: [SHARED_LINK], unavailable: [] });
    expect(html).toContain("Mở bằng app ViHAT");
    expect(html).toContain("App ID Mini App dùng chung");
    expect(html).toContain("chinh.example.vn");
    expect(html).not.toContain("Mở bằng app riêng của xã");
  });

  it("a commune with its own app gets TWO QR cards, shared first, each labelled and with its domain (owner 06/10/2026)", () => {
    const html = ready({ links: [SHARED_LINK, OWN_LINK], unavailable: [] });
    expect(html.match(/Mã QR: /g)?.length).toBe(2);
    expect(html.match(/Chép liên kết/g)?.length).toBe(2);
    const shared = html.indexOf("Mở bằng app ViHAT");
    const own = html.indexOf("Mở bằng app riêng của xã");
    expect(shared).toBeGreaterThan(-1);
    expect(own).toBeGreaterThan(shared);
    expect(html.match(/chinh\.example\.vn<\/dd>/g)?.length).toBe(2);
    expect(html).toContain("https://zalo.me/s/4096/?src=qr");
    expect(html).toContain("App ID Mini App riêng của xã");
  });

  it("an own-app link with an empty domain shows no domain row", () => {
    const html = ready({ links: [{ ...OWN_LINK, domain: "" }], unavailable: [] });
    expect(html).toContain("Mở bằng app riêng của xã");
    expect(html).not.toContain("Tên miền chính của xã");
  });

  it("a link the server could not build is shown with its Vietnamese reason, never silently missing", () => {
    const html = ready({
      links: [OWN_LINK],
      unavailable: [{ source: "chung", code: "shared_mini_app_not_declared", message: "server text" }],
    });
    expect(html.match(/Mã QR: /g)?.length).toBe(1);
    expect(html).toContain("Mở bằng app ViHAT");
    expect(html).toContain(launchLinkError(err(409, "shared_mini_app_not_declared"))!.text);
    expect(html).toContain(`href="${SHARED_APP_PAGE}"`);
    expect(html).not.toContain("shared_mini_app_not_declared");
    const unknown = ready({ links: [SHARED_LINK], unavailable: [{ source: "rieng", code: "mot_ma_moi", message: "Văn bản của máy chủ." }] });
    expect(unknown).toContain("Văn bản của máy chủ.");
  });

  it("an unknown source is shown as it is, never guessed", () => {
    expect(launchSourceLabel("khac")).toBe("Nguồn không xác định: khac");
  });
});

describe("own Mini App refusals of the 05/10/2026 amendment are Vietnamese next steps", () => {
  it.each([
    ["mini_app_removed", 409, "appId"],
    ["mini_app_reactivation_removed", 422, "form"],
  ])("%s", (code, status, field) => {
    const known = communeError(err(status, code));
    expect(known?.field).toBe(field);
    const text = handleGuardedError(err(status, code), "gắn Mini App", vi.fn(), (e) => communeError(e)?.text ?? null);
    expect(text).toBe(known?.text);
    expect(text).not.toContain(code);
    expect(text).toMatch(/[Đđ]ăng ký Mini App mới trên Zalo/);
  });

  it("no sentence still tells the operator to 'Bật lại' an App ID", () => {
    for (const code of ["mini_app_not_bound", "mini_app_inactive", "mini_app_removed", "mini_app_reactivation_removed"]) {
      expect(communeError(err(409, code))?.text).not.toMatch(/Bật lại App ID/);
    }
  });
});

// --- operator log ----------------------------------------------------------------------------------

describe("operator log", () => {
  it("known verbs in words, an unknown verb raw, a platform-wide row named so", () => {
    const html = renderToString(<OperatorLogTable items={ENTRIES} showCommune />);
    expect(html).toContain("Sửa giới hạn tải lên");
    expect(html).toContain("Thêm tên miền");
    expect(html).toContain("verb.not_known_yet");
    expect(html).toContain(PLATFORM_WIDE);
    expect(html).toContain(`href="/xa/${COMMUNE.id}"`);
    expect(html).toContain("VH-00001");
    expect(html).toContain("Tăng giới hạn");
    expect(html).toContain("Trước:");
  });

  it("the commune's own log has no commune column", () => {
    const html = renderToString(<OperatorLogTable items={ENTRIES} showCommune={false} />);
    expect(html).not.toContain(">Xã<");
    expect(html).not.toContain(PLATFORM_WIDE);
  });

  it("Xem thêm only while has_more", () => {
    expect(renderToString(<OperatorLogMore hasMore loading={false} onMore={noop} />)).toContain("Xem thêm");
    expect(renderToString(<OperatorLogMore hasMore={false} loading={false} onMore={noop} />)).toBe("");
  });

  it("the date filter is the server's half-open +07:00 range, end date inclusive", () => {
    expect(logRange("2026-10-01", "2026-10-04")).toEqual({
      ok: true,
      range: { from: "2026-10-01T00:00:00+07:00", to: "2026-10-05T00:00:00+07:00" },
    });
    expect(logRange("", "2026-10-31")).toEqual({ ok: true, range: { to: "2026-11-01T00:00:00+07:00" } });
    expect(logRange("", "")).toEqual({ ok: true, range: {} });
    expect(logRange("2026-10-05", "2026-10-01").ok).toBe(false);
  });

  it("before/after are text, clipped; actionLabel never invents", () => {
    expect(compactValue(null)).toBe("");
    expect(compactValue({ a: 1 })).toBe('{"a":1}');
    expect(compactValue("x".repeat(500))).toHaveLength(VALUE_MAX_CHARS);
    expect(actionLabel("something.else")).toBe("something.else");
  });
});

// --- petition fields -------------------------------------------------------------------------------

const FIELD_FORM: FieldForm = { code: "", label: "", sortOrder: "", icon: "", tone: "", reason: "" };

describe("tier-1 petition fields", () => {
  it("without ops.petition_field.manage: the list incl. retired codes, no write control", () => {
    const html = renderToString(<PetitionFieldTable items={[FIELD_ACTIVE, FIELD_RETIRED]} canManage={false} onEdit={noop} onToggle={noop} />);
    expect(html).toContain("moi-truong");
    expect(html).toContain("Lĩnh vực cũ");
    expect(html).toContain("Ngừng dùng"); // the badge, in text
    expect(html).not.toContain(">Sửa<");
    expect(html).not.toContain("Dùng lại");
    expect(renderToString(<PetitionFieldActions canManage={false} onCreate={noop} />)).toBe("");
  });

  it("with the key: retire an active code, bring back a retired one, add new", () => {
    const html = renderToString(<PetitionFieldTable items={[FIELD_ACTIVE, FIELD_RETIRED]} canManage onEdit={noop} onToggle={noop} />);
    expect(html).toContain("Ngừng dùng lĩnh vực moi-truong");
    expect(html).toContain("Dùng lại lĩnh vực cu");
    expect(renderToString(<PetitionFieldActions canManage onCreate={noop} />)).toContain("Thêm lĩnh vực");
  });

  it("the code is an input on create only; editing shows it as text", () => {
    const create = renderToString(
      <PetitionFieldFormFields field={null} tones={["green"]} form={FIELD_FORM} onChange={noop} busy={false} error={null} />,
    );
    expect(create).toContain('name="code"');
    expect(create).toContain(CODE_FOREVER_NOTICE);
    const edit = renderToString(
      <PetitionFieldFormFields field={FIELD_ACTIVE} tones={["green"]} form={FIELD_FORM} onChange={noop} busy={false} error={null} />,
    );
    expect(edit).not.toContain('name="code"');
    expect(edit).toContain("moi-truong");
    expect(edit).toContain("Xanh lá");
  });
});

// --- CSP ---------------------------------------------------------------------------------------------

describe("the CSP holds: no wave-2 view draws a style attribute", () => {
  it("every new view renders without style=", () => {
    const link = { url: "https://zalo.me/s/1/?d=a.example.vn&src=qr", domain: "a.example.vn", app_id: "1", source: "chung" as const };
    const own = { url: "https://zalo.me/s/2/?src=qr", domain: "a.example.vn", app_id: "2", source: "rieng" as const };
    const views = [
      <SidebarView key="sb" permissionKeys={SEVEN} />,
      <UploadPolicyTable key="ut" items={[POLICY, UNCLASSED]} canEdit onEdit={noop} />,
      <UploadPolicyFormFields key="uf" policy={POLICY} form={{ ...FORM, limited: true, maxFiles: "3" }} onChange={noop} busy={false} error={{ field: "mimeTypes", text: "x" }} />,
      <SharedMiniAppView key="sn" state={{ status: "none" }} canManage onDeclare={noop} />,
      <SharedMiniAppView key="sl" state={{ status: "loading" }} canManage onDeclare={noop} />,
      <DeclareSharedMiniAppFields key="sd" current={{ app_id: "1", created_at: "", created_by: "" }} appId="" reason="" onAppId={noop} onReason={noop} busy={false} error={null} />,
      <PetitionFieldTable key="pt" items={[FIELD_ACTIVE, FIELD_RETIRED]} canManage onEdit={noop} onToggle={noop} />,
      <PetitionFieldFormFields key="pf" field={null} tones={["green"]} form={FIELD_FORM} onChange={noop} busy={false} error={{ field: "tone", text: "x" }} />,
      <LaunchLinkView
        key="ll"
        state={{ status: "ready", links: { links: [link, own], unavailable: [{ source: "chung", code: "commune_no_primary_domain", message: "x" }] } }}
        onCopy={noop}
        copyNotice=""
      />,
      <LaunchLinkView key="le" state={{ status: "error", message: "x", toSharedAppPage: true }} onCopy={noop} copyNotice="" />,
      <OperatorLogTable key="ol" items={ENTRIES} showCommune />,
      <OperatorLogTable key="oe" items={[]} showCommune />,
      <OperatorLogMore key="om" hasMore loading={false} onMore={noop} />,
      <CommuneDetailBody key="cd" commune={COMMUNE} permissionKeys={SEVEN} onChanged={noop} onMiniAppAttached={noop} />,
    ];
    for (const v of views) expect(renderToString(v)).not.toContain("style=");
  });
});
