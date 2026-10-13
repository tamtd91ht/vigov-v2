import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/xa",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
}));

import { OWNER_ITEM, SidebarView, visibleNavItems } from "@/components/sidebar";
import {
  ALL_OPS_KEYS,
  ApiError,
  createCommuneAdminAccount,
  createOperatorAccount,
  deleteCommune,
  listCommuneStaffAccounts,
  listOperatorAccounts,
  OPS_KEY_LABELS,
  resetOperatorPassword,
  resetStaffPassword,
  setOperatorPermissions,
  type CommuneDetail,
  type OperatorAccount,
  type StaffAccount,
} from "@/lib/api";
import { AUTH_UNAVAILABLE, handleGuardedError } from "@/lib/errors";

import { CommuneDetailBody } from "../communes/commune-detail";
import { CommuneDeletionSection, DELETE_LABEL } from "./commune-deletion";
import { CREATE_ADMIN_INACTIVE, CREATE_ADMIN_LABEL, CreateAdminControl, StaffAccountTable } from "./commune-staff-accounts";
import {
  CREATE_OPERATOR_LABEL,
  OperatorAccountsGate,
  OperatorAccountTable,
  OWNER_ONLY_TEXT,
  OWNER_ROW_LABEL,
  PermissionKeyChecklist,
} from "./operator-accounts-screen";
import {
  ACCOUNTS_NOT_READY,
  canSubmitDeletion,
  checkCreateCommuneAdmin,
  checkCreateOperator,
  checkReason,
  checkReset,
  checkTemporaryPassword,
  communeDeletionError,
  CONFIRM_DOMAIN_MISMATCH,
  DELETION_EXPLANATION,
  DELETION_NEEDS_INACTIVE,
  operatorStatus,
  ownerAccountError,
  STAFF_DISABLED_RESET,
  staffStatus,
} from "./owner-model";
import { TemporaryPasswordFields } from "./owner-parts";

/**
 * The owner-only screens (ADR 0048 §Sửa đổi 13/10/2026). Denied cases first: a non-owner sees no
 * control and — for the staff list, an audited read of personal data — makes no request.
 */

// --- fetch stub ------------------------------------------------------------------------------------

type Captured = { url: string; init: RequestInit };
let calls: Captured[] = [];
let answer: () => Response = () => json(200, {});

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

beforeEach(() => {
  calls = [];
  answer = () => json(200, {});
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return answer();
    }),
  );
});

afterEach(() => vi.unstubAllGlobals());

function last() {
  const c = calls.at(-1)!;
  return {
    url: c.url,
    method: String(c.init.method),
    body: typeof c.init.body === "string" ? (JSON.parse(c.init.body) as unknown) : undefined,
  };
}

// --- fixtures --------------------------------------------------------------------------------------

const NOW = new Date("2026-10-13T03:00:00Z");
const noop = () => {};
const FAKE_PASSWORD = "mat-khau-tam-FAKE-1";

const COMMUNE: CommuneDetail = {
  id: "01J0000000000000000000000A",
  name: "Xã Kiểm Thử",
  province: "Tỉnh Kiểm Thử",
  active: true,
  domains: ["chinh.example.vn", "phu.example.vn"],
  mini_apps: [],
  unbound_secrets: [],
};

const OWNER: OperatorAccount = {
  code: "VH-00001",
  email: "owner@example.vn",
  display_name: "Người Owner",
  permission_keys: [...ALL_OPS_KEYS],
  is_owner: true,
  disabled: false,
  must_change_password: false,
  locked_until: null,
  created_at: "2026-09-28T03:00:00Z",
};

const OTHER: OperatorAccount = {
  ...OWNER,
  code: "VH-00002",
  email: "vanhanh@example.vn",
  display_name: "Người Vận Hành",
  permission_keys: ["ops.tenant.manage"],
  is_owner: false,
};

const STAFF: StaffAccount = {
  user_id: "01J0000000000000000000000S",
  staff_code: "CB-00001",
  email: "quantri@example.vn",
  full_name: "Cán Bộ Kiểm Thử",
  role_code: "quan-tri-he-thong",
  role_name: "Quản trị hệ thống",
  is_admin: true,
  disabled: false,
  must_change_password: true,
  sign_in_locked_until: null,
};

// --- visibility: denied first ----------------------------------------------------------------------

describe("owner visibility — the nav entry follows is_owner, never a key", () => {
  it("DENIED: every ops.* key but not the owner — no Phân quyền entry", () => {
    expect(visibleNavItems(ALL_OPS_KEYS)).not.toContain(OWNER_ITEM);
    expect(visibleNavItems(ALL_OPS_KEYS, false)).not.toContain(OWNER_ITEM);
    expect(renderToString(<SidebarView permissionKeys={ALL_OPS_KEYS} />)).not.toContain('href="/phan-quyen"');
  });

  it("ALLOWED: the owner gets it, keys or not", () => {
    expect(visibleNavItems(ALL_OPS_KEYS, true)).toContain(OWNER_ITEM);
    expect(visibleNavItems([], true)).toEqual([OWNER_ITEM]);
    expect(renderToString(<SidebarView permissionKeys={[]} isOwner />)).toContain('href="/phan-quyen"');
  });
});

describe("/phan-quyen gate", () => {
  it("DENIED: a non-owner reads why, gets no create button and makes no request", () => {
    const html = renderToString(<OperatorAccountsGate status="ready" isOwner={false} />);
    expect(html).toContain(OWNER_ONLY_TEXT);
    expect(html).not.toContain(CREATE_OPERATOR_LABEL);
    expect(calls).toHaveLength(0);
  });

  it("still loading: neither the workspace nor the refusal", () => {
    const html = renderToString(<OperatorAccountsGate status="loading" isOwner={false} />);
    expect(html).not.toContain(CREATE_OPERATOR_LABEL);
    expect(html).not.toContain(OWNER_ONLY_TEXT);
  });

  it("ALLOWED: the owner gets the workspace", () => {
    const html = renderToString(<OperatorAccountsGate status="ready" isOwner />);
    expect(html).toContain(CREATE_OPERATOR_LABEL);
    expect(html).not.toContain(OWNER_ONLY_TEXT);
  });
});

describe("commune detail — owner sections", () => {
  const OWNER_TEXTS = ["Tài khoản cán bộ", DELETE_LABEL, CREATE_ADMIN_LABEL, DELETION_EXPLANATION];

  it("DENIED: without is_owner (absent or false), even with every key — no section, no request", () => {
    for (const isOwner of [undefined, false]) {
      const html = renderToString(
        <CommuneDetailBody commune={COMMUNE} permissionKeys={ALL_OPS_KEYS} isOwner={isOwner} onChanged={noop} onMiniAppAttached={noop} />,
      );
      for (const t of OWNER_TEXTS) expect(html).not.toContain(t);
    }
    expect(calls.filter((c) => c.url.includes("staff-accounts"))).toHaveLength(0);
  });

  it("ALLOWED: the owner sees the staff section and the danger zone", () => {
    const html = renderToString(
      <CommuneDetailBody commune={{ ...COMMUNE, active: false }} permissionKeys={[]} isOwner onChanged={noop} onMiniAppAttached={noop} />,
    );
    for (const t of OWNER_TEXTS) expect(html).toContain(t);
  });

  it("Xoá xã is disabled on an active commune, with the reason; enabled on an inactive one", () => {
    const active = renderToString(<CommuneDeletionSection commune={COMMUNE} />);
    expect(active).toContain(DELETION_NEEDS_INACTIVE);
    expect(active).toContain('disabled=""');
    const inactive = renderToString(<CommuneDeletionSection commune={{ ...COMMUNE, active: false }} />);
    expect(inactive).not.toContain(DELETION_NEEDS_INACTIVE);
    expect(inactive).not.toContain('disabled=""');
  });

  it("Thêm tài khoản quản trị is disabled on an inactive commune, with the reason", () => {
    const off = renderToString(<CreateAdminControl active={false} onOpen={noop} />);
    expect(off).toContain(CREATE_ADMIN_INACTIVE);
    expect(off).toContain('disabled=""');
    const on = renderToString(<CreateAdminControl active onOpen={noop} />);
    expect(on).not.toContain(CREATE_ADMIN_INACTIVE);
    expect(on).not.toContain('disabled=""');
  });
});

// --- tables ----------------------------------------------------------------------------------------

describe("operator account table", () => {
  it("the owner row has no action, only its label; other rows get both actions", () => {
    const html = renderToString(<OperatorAccountTable accounts={[OWNER, OTHER]} now={NOW} onAction={noop} />);
    expect(html).toContain(OWNER_ROW_LABEL);
    expect(html.match(/Phân quyền/g)?.length).toBe(1);
    expect(html.match(/Đặt lại mật khẩu/g)?.length).toBe(1);
    expect(html).toContain("VH-00002");
    expect(html).toContain("ops.tenant.manage");
  });

  it("status reads Owner / Bị khoá đến … / Phải đổi mật khẩu / Đã vô hiệu; a past lock is no lock", () => {
    expect(operatorStatus(OWNER, NOW).map((c) => c.text)).toEqual(["Owner"]);
    const locked = { ...OTHER, locked_until: "2026-10-13T04:30:00Z", must_change_password: true, disabled: true };
    // 04:30Z = 11:30 +07:00
    expect(operatorStatus(locked, NOW).map((c) => c.text)).toEqual([
      "Đã vô hiệu",
      "Bị khoá đến 13/10/2026 11:30",
      "Phải đổi mật khẩu",
    ]);
    expect(operatorStatus({ ...OTHER, locked_until: "2026-10-12T00:00:00Z" }, NOW).map((c) => c.text)).toEqual(["Đang dùng"]);
  });

  it("the checklist offers every closed-set key with its Vietnamese label", () => {
    const html = renderToString(<PermissionKeyChecklist selected={["ops.qr.issue"]} onChange={noop} disabled={false} legend="Quyền" />);
    for (const k of ALL_OPS_KEYS) {
      expect(html).toContain(k);
      expect(OPS_KEY_LABELS[k]).toBeTruthy();
    }
    expect(html.match(/checked=""/g)?.length).toBe(1);
  });
});

describe("staff account table", () => {
  it("a disabled account's reset is disabled, with the reason", () => {
    const html = renderToString(<StaffAccountTable accounts={[{ ...STAFF, disabled: true }]} now={NOW} onReset={noop} />);
    expect(html).toContain(STAFF_DISABLED_RESET);
    expect(html).toContain('disabled=""');
  });

  it("an enabled account: code, name, role, admin mark and status", () => {
    const html = renderToString(<StaffAccountTable accounts={[STAFF]} now={NOW} onReset={noop} />);
    for (const t of ["CB-00001", "Cán Bộ Kiểm Thử", "Quản trị hệ thống", "Quản trị", "Phải đổi mật khẩu"]) expect(html).toContain(t);
    expect(html).not.toContain(STAFF_DISABLED_RESET);
    expect(html).not.toContain('disabled=""');
    expect(staffStatus({ ...STAFF, must_change_password: false }, NOW).map((c) => c.text)).toEqual(["Đang dùng"]);
  });
});

// --- dialog checks ---------------------------------------------------------------------------------

describe("dialog validation (client side, before any request)", () => {
  it("temporary password: empty, then mismatch", () => {
    expect(checkTemporaryPassword("", "")).toEqual({ field: "password", text: "Hãy nhập mật khẩu tạm." });
    expect(checkTemporaryPassword(FAKE_PASSWORD, FAKE_PASSWORD + "x")?.field).toBe("confirm");
    expect(checkTemporaryPassword(FAKE_PASSWORD, FAKE_PASSWORD)).toBeNull();
  });

  it("reason: empty or blank refused, over 500 refused", () => {
    expect(checkReason("")?.field).toBe("reason");
    expect(checkReason("   \n ")?.field).toBe("reason");
    expect(checkReason("x".repeat(501))?.field).toBe("reason");
    expect(checkReason("Người vận hành quên mật khẩu")).toBeNull();
  });

  it("reset: password first, then reason", () => {
    expect(checkReset({ password: FAKE_PASSWORD, confirm: "khac", reason: "" })?.field).toBe("confirm");
    expect(checkReset({ password: FAKE_PASSWORD, confirm: FAKE_PASSWORD, reason: "" })?.field).toBe("reason");
    expect(checkReset({ password: FAKE_PASSWORD, confirm: FAKE_PASSWORD, reason: "lý do" })).toBeNull();
  });

  it("create: email, name, then password", () => {
    const ok = { email: "a@example.vn", displayName: "Tên", password: FAKE_PASSWORD, confirm: FAKE_PASSWORD };
    expect(checkCreateOperator({ ...ok, email: "" })?.field).toBe("email");
    expect(checkCreateOperator({ ...ok, email: "khong-co-a-cong" })?.field).toBe("email");
    expect(checkCreateOperator({ ...ok, displayName: " " })?.field).toBe("name");
    expect(checkCreateOperator({ ...ok, confirm: "x" })?.field).toBe("confirm");
    expect(checkCreateOperator(ok)).toBeNull();
    expect(checkCreateCommuneAdmin({ email: "a@example.vn", fullName: "", password: FAKE_PASSWORD, confirm: FAKE_PASSWORD })?.field).toBe(
      "name",
    );
  });

  it("deletion submit needs both the re-typed domain and a reason", () => {
    expect(canSubmitDeletion("", "")).toBe(false);
    expect(canSubmitDeletion("chinh.example.vn", " ")).toBe(false);
    expect(canSubmitDeletion(" ", "lý do")).toBe(false);
    expect(canSubmitDeletion("chinh.example.vn", "lý do")).toBe(true);
  });

  it("password inputs are type=password with autocomplete=new-password", () => {
    const html = renderToString(
      <TemporaryPasswordFields password="" confirm="" onPassword={noop} onConfirm={noop} disabled={false} error={null} />,
    );
    expect(html.match(/type="password"/g)?.length).toBe(2);
    expect(html.match(/autoComplete="new-password"|autocomplete="new-password"/g)?.length).toBe(2);
  });
});

// --- error mapping ---------------------------------------------------------------------------------

function apiError(status: number, code: string, extra: Partial<{ earliest: string | null }> = {}): ApiError {
  return new ApiError(status, code, "", "", null, null, null, null, extra.earliest ?? null);
}

describe("error code → sentence", () => {
  it("commune_inactive_too_recent names the earliest moment, date first, Viet Nam time", () => {
    // 03:05Z = 10:05 +07:00
    expect(communeDeletionError(apiError(409, "commune_inactive_too_recent", { earliest: "2026-10-20T03:05:00Z" }))).toEqual({
      field: "form",
      text: "Xã chưa ngừng hoạt động đủ 7 ngày. Có thể xoá từ 20/10/2026 10:05.",
    });
    expect(communeDeletionError(apiError(409, "commune_inactive_too_recent"))?.text).toBe(
      "Xã chưa ngừng hoạt động đủ 7 ngày. Thử lại sau.",
    );
  });

  it("confirm_domain_mismatch sits under the domain input; commune_active and invalid_reason too", () => {
    expect(communeDeletionError(apiError(422, "confirm_domain_mismatch"))).toEqual({
      field: "confirmDomain",
      text: CONFIRM_DOMAIN_MISMATCH,
    });
    expect(communeDeletionError(apiError(409, "commune_active"))?.text).toContain("đủ 7 ngày");
    expect(communeDeletionError(apiError(422, "invalid_reason"))?.field).toBe("reason");
  });

  it("503 account_management_not_ready reads 'đang được triển khai'; operator_auth_unavailable stays the auth outage", () => {
    const specific = (e: ApiError) => ownerAccountError(e)?.text ?? null;
    expect(handleGuardedError(apiError(503, "account_management_not_ready"), "x", noop, specific)).toBe(ACCOUNTS_NOT_READY);
    expect(ACCOUNTS_NOT_READY).toBe("Chức năng đang được triển khai phía máy chủ, vui lòng thử lại sau");
    expect(handleGuardedError(apiError(503, "operator_auth_unavailable"), "x", noop, specific)).toBe(AUTH_UNAVAILABLE);
  });

  it("account refusals land under their field", () => {
    expect(ownerAccountError(apiError(409, "account_email_taken"))?.field).toBe("email");
    expect(ownerAccountError(apiError(409, "staff_account_locked"))?.text).toContain("đang bị xã khoá");
    expect(ownerAccountError(apiError(404, "operator_account_not_found"))?.field).toBe("form");
    const rejected = new ApiError(422, "new_password_rejected", "", "", null, { problem: "too_short", minLength: 12, maxLength: 128 });
    expect(ownerAccountError(rejected)).toEqual({ field: "password", text: "Mật khẩu mới phải có ít nhất 12 ký tự." });
  });

  it("the 409 body's earliest_deletion_at reaches the ApiError", async () => {
    answer = () =>
      json(409, { code: "commune_inactive_too_recent", message: "", trace_id: "t", earliest_deletion_at: "2026-10-20T03:05:00Z" });
    const err = await deleteCommune(COMMUNE.id, { confirmDomain: "chinh.example.vn", reason: "r" }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).earliestDeletionAt).toBe("2026-10-20T03:05:00Z");
    answer = () => json(409, { code: "commune_active", message: "", trace_id: "t", earliest_deletion_at: null });
    const active = await deleteCommune(COMMUNE.id, { confirmDomain: "x", reason: "r" }).catch((e: unknown) => e);
    expect((active as ApiError).earliestDeletionAt).toBeNull();
  });
});

// --- client: exact paths, methods, bodies ----------------------------------------------------------

describe("client — the eight owner calls", () => {
  it("GET operator accounts and a commune's staff accounts", async () => {
    answer = () => json(200, { items: [OWNER] });
    expect(await listOperatorAccounts()).toEqual([OWNER]);
    expect(last()).toEqual({ url: "/api/v1/operator-accounts", method: "GET", body: undefined });

    answer = () => json(200, { items: [STAFF] });
    expect(await listCommuneStaffAccounts(COMMUNE.id)).toEqual([STAFF]);
    expect(last()).toEqual({ url: `/api/v1/communes/${COMMUNE.id}/staff-accounts`, method: "GET", body: undefined });
  });

  it("POST operator account always sends permission_keys, empty included", async () => {
    answer = () => json(201, OTHER);
    await createOperatorAccount({ email: "a@example.vn", displayName: "Tên", temporaryPassword: FAKE_PASSWORD, permissionKeys: [] });
    expect(last()).toEqual({
      url: "/api/v1/operator-accounts",
      method: "POST",
      body: { email: "a@example.vn", display_name: "Tên", temporary_password: FAKE_PASSWORD, permission_keys: [] },
    });
  });

  it("PUT permissions and password escape the code in the path", async () => {
    answer = () => json(200, OTHER);
    await setOperatorPermissions("VH-00002/x", { permissionKeys: ["ops.qr.issue"], reason: "r" });
    expect(last()).toEqual({
      url: "/api/v1/operator-accounts/VH-00002%2Fx/permissions",
      method: "PUT",
      body: { permission_keys: ["ops.qr.issue"], reason: "r" },
    });
    await resetOperatorPassword("VH-00002", { temporaryPassword: FAKE_PASSWORD, reason: "r" });
    expect(last()).toEqual({
      url: "/api/v1/operator-accounts/VH-00002/password",
      method: "PUT",
      body: { temporary_password: FAKE_PASSWORD, reason: "r" },
    });
  });

  it("POST admin account, PUT staff password, POST deletion", async () => {
    answer = () => json(201, STAFF);
    await createCommuneAdminAccount(COMMUNE.id, { email: "q@example.vn", fullName: "Họ Tên", temporaryPassword: FAKE_PASSWORD });
    expect(last()).toEqual({
      url: `/api/v1/communes/${COMMUNE.id}/admin-accounts`,
      method: "POST",
      body: { email: "q@example.vn", full_name: "Họ Tên", temporary_password: FAKE_PASSWORD },
    });

    answer = () => json(200, STAFF);
    await resetStaffPassword(COMMUNE.id, STAFF.user_id, { temporaryPassword: FAKE_PASSWORD, reason: "r" });
    expect(last()).toEqual({
      url: `/api/v1/communes/${COMMUNE.id}/staff-accounts/${STAFF.user_id}/password`,
      method: "PUT",
      body: { temporary_password: FAKE_PASSWORD, reason: "r" },
    });

    answer = () => json(201, { id: COMMUNE.id, name: COMMUNE.name, deleted_at: "2026-10-20T03:05:00Z", deleted_by: "VH-00001" });
    await deleteCommune(COMMUNE.id, { confirmDomain: "chinh.example.vn", reason: "r" });
    expect(last()).toEqual({
      url: `/api/v1/communes/${COMMUNE.id}/deletion`,
      method: "POST",
      body: { confirm_domain: "chinh.example.vn", reason: "r" },
    });
  });
});

describe("the CSP holds for the owner screens", () => {
  it("no style= attribute", () => {
    const screens = [
      <OperatorAccountTable key="t" accounts={[OWNER, OTHER]} now={NOW} onAction={noop} />,
      <PermissionKeyChecklist key="c" selected={[]} onChange={noop} disabled={false} legend="Quyền" />,
      <StaffAccountTable key="s" accounts={[STAFF, { ...STAFF, user_id: "2", disabled: true }]} now={NOW} onReset={noop} />,
      <CommuneDeletionSection key="d" commune={COMMUNE} />,
      <CreateAdminControl key="a" active={false} onOpen={noop} />,
      <OperatorAccountsGate key="g" status="ready" isOwner />,
      <CommuneDetailBody key="b" commune={COMMUNE} permissionKeys={[]} isOwner onChanged={noop} onMiniAppAttached={noop} />,
    ];
    for (const s of screens) expect(renderToString(s)).not.toContain("style=");
  });
});
