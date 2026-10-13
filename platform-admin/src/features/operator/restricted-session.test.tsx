import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/xa",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
}));

import { ApiError, changePassword, listCommunes, onPasswordChangeRequired, type OperatorAccount } from "@/lib/api";
import { handleGuardedError } from "@/lib/errors";

import { ResetOperatorPasswordDialog } from "../owner/operator-accounts-screen";
import { OPERATOR_RESET_NOTE } from "../owner/owner-model";
import { ConsoleShellView, FORCED_CHANGE_TEXT, ForcedPasswordChangeView } from "./console-shell";
import { afterPasswordChangeOf, loadOperator, mustChangePasswordOf, type OperatorState } from "./operator-context";

/**
 * The restricted session after an owner reset (ADR 0048 §13/10 #6). vitest runs without a DOM, so
 * the provider's decisions are tested as the pure functions it calls, and the frame is rendered per
 * state. "The page fires no call" is pinned by the page never being RENDERED: a screen's calls live
 * in its effects, and an unmounted screen has none.
 */

type Captured = { url: string; init: RequestInit };
let calls: Captured[] = [];
let answer: (url: string) => Response = () => json(200, {});

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
      return answer(url);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

const RESTRICTED = { operator_code: "VH-00009", permission_keys: ["ops.tenant.manage"], is_owner: false, must_change_password: true };
const OPEN = { ...RESTRICTED, must_change_password: false };

/** Stands in for a page: rendering it is what would mount its effects and send its calls. */
const pageRender = vi.fn();
function Page() {
  pageRender();
  return <p>NỘI DUNG TRANG</p>;
}

function shell(state: OperatorState, mustChangePassword: boolean): string {
  pageRender.mockClear();
  return renderToString(
    <ConsoleShellView state={state} mustChangePassword={mustChangePassword}>
      <Page />
    </ConsoleShellView>,
  );
}

describe("restricted session answer → only the forced change screen", () => {
  it("whoAmI with must_change_password: true is the ONLY call, and restricts the console", async () => {
    answer = () => json(200, RESTRICTED);
    const r = await loadOperator();
    expect(calls.map((c) => c.url)).toEqual(["/api/v1/operator-sessions/current"]);
    expect(r.kind).toBe("ready");
    const state: OperatorState = r.kind === "ready" ? { status: "ready", operator: r.operator } : { status: "loading" };
    expect(mustChangePasswordOf(state, false)).toBe(true);

    const html = shell(state, true);
    expect(html).toContain(FORCED_CHANGE_TEXT);
    expect(html).toContain('name="current_password"');
    expect(html).toContain('name="totp_code"');
    expect(html).toContain("Đăng xuất");
    expect(html).not.toContain("Điều hướng chính");
    expect(html).not.toContain("Danh sách xã");
    expect(html).not.toContain("NỘI DUNG TRANG");
    expect(pageRender).not.toHaveBeenCalled();
    expect(calls).toHaveLength(1);
  });

  it("only an explicit true restricts: false, absent, loading and error do not", () => {
    expect(mustChangePasswordOf({ status: "ready", operator: OPEN }, false)).toBe(false);
    const older = { operator_code: "VH-00009", permission_keys: [] };
    expect(mustChangePasswordOf({ status: "ready", operator: older }, false)).toBe(false);
    expect(mustChangePasswordOf({ status: "loading" }, false)).toBe(false);
    expect(mustChangePasswordOf({ status: "error", message: "x" }, false)).toBe(false);
  });

  it("while the session answer is pending the page is not mounted, and no navigation is drawn", () => {
    const html = shell({ status: "loading" }, false);
    expect(pageRender).not.toHaveBeenCalled();
    expect(html).not.toContain("Điều hướng chính");
    expect(html).toContain("Đang tải thông tin tài khoản");
  });

  it("an open session gets the sidebar and the page", () => {
    const html = shell({ status: "ready", operator: OPEN }, false);
    expect(pageRender).toHaveBeenCalledTimes(1);
    expect(html).toContain("NỘI DUNG TRANG");
    expect(html).toContain("Điều hướng chính");
    expect(html).not.toContain(FORCED_CHANGE_TEXT);
  });
});

describe("403 password_change_required from any call → forced screen", () => {
  it("the client tells the shell, and the guarded handler shows no sentence", async () => {
    const heard = vi.fn();
    const off = onPasswordChangeRequired(heard);
    answer = () => json(403, { code: "password_change_required", message: "đổi mật khẩu trước", trace_id: "t1" });
    const err = await listCommunes().catch((e: unknown) => e);
    off();
    expect(err).toBeInstanceOf(ApiError);
    expect(heard).toHaveBeenCalledTimes(1);
    const navigate = vi.fn();
    expect(handleGuardedError(err, "xem danh sách xã", navigate)).toBeNull();
    expect(navigate).not.toHaveBeenCalled();

    // The session answer said nothing (or failed): the refusal alone restricts the frame.
    expect(mustChangePasswordOf({ status: "ready", operator: OPEN }, true)).toBe(true);
    expect(mustChangePasswordOf({ status: "error", message: "x" }, true)).toBe(true);
    const html = shell({ status: "ready", operator: OPEN }, true);
    expect(html).toContain(FORCED_CHANGE_TEXT);
    expect(html).not.toContain("Điều hướng chính");
    expect(pageRender).not.toHaveBeenCalled();
  });

  it("denied case: 403 forbidden is NOT the restricted session", async () => {
    const heard = vi.fn();
    const off = onPasswordChangeRequired(heard);
    answer = () => json(403, { code: "forbidden", message: "", trace_id: "" });
    const err = await listCommunes().catch((e: unknown) => e);
    off();
    expect(heard).not.toHaveBeenCalled();
    expect(handleGuardedError(err, "xem danh sách xã", vi.fn())).toContain("không có quyền xem danh sách xã");
  });

  it("an unsubscribed listener hears nothing", async () => {
    const heard = vi.fn();
    onPasswordChangeRequired(heard)();
    answer = () => json(403, { code: "password_change_required", message: "", trace_id: "" });
    await listCommunes().catch(() => undefined);
    expect(heard).not.toHaveBeenCalled();
  });
});

describe("after the forced change", () => {
  it("success then an open session answer → the console", async () => {
    answer = (url) => (url.endsWith("/operators/current/password") ? new Response(null, { status: 204 }) : json(200, OPEN));
    await changePassword({ currentPassword: "tam-thoi", newPassword: "mat-khau-moi-du-dai", totpCode: "123456" });
    const next = afterPasswordChangeOf(await loadOperator());
    expect(next).toBe("continue");
    expect(calls.map((c) => [c.init.method, c.url])).toEqual([
      ["PUT", "/api/v1/operators/current/password"],
      ["GET", "/api/v1/operator-sessions/current"],
    ]);
    const html = shell({ status: "ready", operator: OPEN }, mustChangePasswordOf({ status: "ready", operator: OPEN }, false));
    expect(html).toContain("NỘI DUNG TRANG");
    expect(html).not.toContain(FORCED_CHANGE_TEXT);
  });

  it("the server ended the session → sign in again, never the console", async () => {
    answer = () => json(401, { code: "unauthenticated", message: "", trace_id: "" });
    expect(afterPasswordChangeOf(await loadOperator())).toBe("signed_out");
  });

  it("still restricted, or the read failed → unknown, never the console", async () => {
    answer = () => json(200, RESTRICTED);
    expect(afterPasswordChangeOf(await loadOperator())).toBe("unknown");
    answer = () => json(503, { code: "operator_auth_unavailable", message: "", trace_id: "" });
    expect(afterPasswordChangeOf(await loadOperator())).toBe("unknown");
  });

  it("the forced screen reuses the account form without the 'every session ends' notice", () => {
    const html = renderToString(<ForcedPasswordChangeView onChanged={async () => "continue"} />);
    expect(html).toContain(FORCED_CHANGE_TEXT);
    expect(html).toContain("Mật khẩu hiện tại");
    expect(html).toContain("Mã xác thực");
    expect(html).not.toContain("mọi phiên đăng nhập của tài khoản sẽ kết thúc");
    expect(html).not.toContain("style=");
  });
});

describe("owner reset dialog for an operator account", () => {
  const ACCOUNT: OperatorAccount = {
    code: "VH-00009",
    email: "op@example.vn",
    display_name: "Người Kiểm Thử",
    permission_keys: [],
    is_owner: false,
    disabled: false,
    must_change_password: false,
    locked_until: null,
    created_at: "2026-10-01T03:00:00Z",
  };

  it("says the next sign-in uses the temporary password and the person's own TOTP", () => {
    expect(OPERATOR_RESET_NOTE).toBe(
      "Lần đăng nhập tới người này dùng mật khẩu tạm và mã TOTP của chính họ, rồi phải đổi mật khẩu.",
    );
    const html = renderToString(<ResetOperatorPasswordDialog account={ACCOUNT} onClose={() => {}} onDone={() => {}} />);
    expect(html).toContain(OPERATOR_RESET_NOTE);
    expect(calls).toHaveLength(0);
  });
});
