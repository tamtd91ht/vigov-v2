import { describe, expect, it, vi } from "vitest";

import { ApiError } from "./api";
import {
  AUTH_UNAVAILABLE,
  communeError,
  GENERIC_ERROR,
  handleGuardedError,
  NETWORK_ERROR,
  passwordRejectionText,
  waitMessage,
} from "./errors";

const err = (status: number, code: string, retryAfter: number | null = null) =>
  new ApiError(status, code, "", "", retryAfter);

describe("handleGuardedError", () => {
  it("401 sends the person to sign-in and shows nothing", () => {
    const navigate = vi.fn();
    expect(handleGuardedError(err(401, "unauthorized"), "tạo xã", navigate)).toBeNull();
    expect(navigate).toHaveBeenCalledWith("/dang-nhap");
  });

  it("403 forbidden names the action that is not permitted, and stays", () => {
    const navigate = vi.fn();
    const text = handleGuardedError(err(403, "forbidden"), "gắn Mini App", navigate);
    expect(text).toContain("không có quyền gắn Mini App");
    expect(navigate).not.toHaveBeenCalled();
  });

  it("503 is an outage, NEVER a sign-out", () => {
    const navigate = vi.fn();
    expect(handleGuardedError(err(503, "operator_auth_unavailable"), "tạo xã", navigate)).toBe(AUTH_UNAVAILABLE);
    expect(AUTH_UNAVAILABLE).toContain("Hệ thống xác thực tạm không phản hồi");
    expect(navigate).not.toHaveBeenCalled();
  });

  it("no answer at all is a network sentence, not a sign-out", () => {
    const navigate = vi.fn();
    expect(handleGuardedError(err(0, "network_error"), "x", navigate)).toBe(NETWORK_ERROR);
    expect(navigate).not.toHaveBeenCalled();
  });

  it("a code the screen knows gets its sentence; an unknown one the generic sentence", () => {
    const specific = (e: ApiError) => communeError(e)?.text ?? null;
    expect(handleGuardedError(err(409, "domain_taken"), "x", vi.fn(), specific)).toContain("đã được gắn cho một xã");
    expect(handleGuardedError(err(500, "internal"), "x", vi.fn(), specific)).toBe(GENERIC_ERROR);
  });

  it("no message shows the server's code", () => {
    const text = handleGuardedError(err(409, "duplicate_name"), "x", vi.fn(), (e) => communeError(e)?.text ?? null);
    expect(text).not.toContain("duplicate_name");
  });
});

describe("commune error codes map to a field and a next step", () => {
  it.each([
    ["invalid_name", "name"],
    ["duplicate_name", "name"],
    ["unknown_province", "province"],
    ["invalid_domain", "domain"],
    ["reserved_domain", "domain"],
    ["domain_taken", "domain"],
    ["domain_not_in_commune", "domain"],
    ["commune_inactive", "form"],
    ["invalid_reason", "reason"],
    ["invalid_app_id", "appId"],
    ["invalid_note", "note"],
    ["mini_app_taken", "appId"],
    ["commune_not_found", "form"],
  ])("%s → %s", (code, field) => {
    expect(communeError(err(422, code))?.field).toBe(field);
  });
});

describe("waitMessage and passwordRejectionText", () => {
  it("says the wait from Retry-After, in seconds or minutes", () => {
    expect(waitMessage(30)).toContain("30 giây");
    expect(waitMessage(60)).toContain("1 phút");
    expect(waitMessage(899)).toContain("15 phút");
    expect(waitMessage(null)).not.toMatch(/\d/);
  });

  it("uses the SERVER's bounds, never a copy", () => {
    expect(passwordRejectionText({ problem: "too_short", minLength: 17, maxLength: 64 }, "x")).toContain("17");
    expect(passwordRejectionText({ problem: "too_long", minLength: 17, maxLength: 64 }, "x")).toContain("64");
    expect(passwordRejectionText({ problem: "same_as_current", minLength: 0, maxLength: 0 }, "mật khẩu tạm")).toContain(
      "mật khẩu tạm",
    );
  });
});
