import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * WHAT ZALO ANSWERS REACHES THE SCREEN — `xin` in `zalo-api.ts` (user rule 30/09/2026: the test build is
 * the build that ships; a permission the App ID lacks must show as that, with Zalo's code).
 *
 * What must hold:
 *   · `-201` is still the citizen's refusal (`tu-choi`), for every capability
 *   · any other numeric code is `khong-lay-duoc` WITH `{ capability, code, transient }` — the code verbatim
 *   · the capability is the STEP that failed: a sign-in asks for the session code, then the phone
 *   · only the four "try again later" codes of the SDK are `transient`
 *   · a throw with no numeric code measures nothing — no `failure`, never an invented code
 *
 * `zmp-sdk` is replaced here because the real module needs a Zalo runtime; the calls under test are the
 * real functions of `zalo-api.ts`.
 */
const sdk = vi.hoisted(() => ({
  getAccessToken: vi.fn<() => Promise<string>>(),
  getPhoneNumber: vi.fn<() => Promise<{ token?: string }>>(),
  getLocation: vi.fn<() => Promise<{ token?: string }>>(),
  getUserInfo: vi.fn<() => Promise<{ userInfo: { name?: string } }>>(),
  getSetting: vi.fn<() => Promise<{ authSetting: Record<string, boolean> }>>(),
  authorize: vi.fn<(o: { scopes: string[] }) => Promise<Record<string, boolean>>>(),
}));
vi.mock("zmp-sdk", () => sdk);
// Coded failures here would POST a real report on a machine whose `.env.local` sets the API host.
vi.mock("../dang-nhap/goi-may-chu", () => ({ reportClientError: async () => {} }));

import { layTenZalo, requestLocationCodes, xinMaDangNhap, xinTokenSoDienThoai } from "./zalo-api";

const zaloError = (code: number) => Object.assign(new Error("platform text that must not travel"), { code });

beforeEach(() => {
  sdk.getAccessToken.mockReset().mockResolvedValue("access-test");
  sdk.getPhoneNumber.mockReset().mockResolvedValue({ token: "phone-test" });
  sdk.getLocation.mockReset().mockResolvedValue({ token: "location-test" });
  sdk.getUserInfo.mockReset().mockResolvedValue({ userInfo: { name: "Nguyễn Văn Thử" } });
  sdk.getSetting.mockReset().mockResolvedValue({ authSetting: {} });
  sdk.authorize.mockReset().mockResolvedValue({ "scope.userPhonenumber": true });
});

describe("the phone permission Zalo remembers (owner 12/10/2026: agreed, yet the switch stayed off)", () => {
  it.each([
    ["token", xinTokenSoDienThoai],
    ["sign-in", xinMaDangNhap],
  ] as const)("%s: asks authorize for the phone scope only, before getPhoneNumber", async (_, call) => {
    expect((await call()).kieu).toBe("xong");
    expect(sdk.authorize).toHaveBeenCalledWith({ scopes: ["scope.userPhonenumber"] });
    expect(sdk.authorize.mock.invocationCallOrder[0]).toBeLessThan(sdk.getPhoneNumber.mock.invocationCallOrder[0]!);
  });

  it("already granted: Zalo's dialog is not shown again", async () => {
    sdk.getSetting.mockResolvedValue({ authSetting: { "scope.userPhonenumber": true } });
    expect((await xinTokenSoDienThoai()).kieu).toBe("xong");
    expect(sdk.authorize).not.toHaveBeenCalled();
  });

  it.each([
    ["answers false", () => sdk.authorize.mockResolvedValue({ "scope.userPhonenumber": false })],
    ["throws -201", () => sdk.authorize.mockRejectedValue(zaloError(-201))],
  ])("refusal %s: the citizen's NO, and the number is never asked", async (_, arrange) => {
    arrange();
    expect((await xinTokenSoDienThoai()).kieu).toBe("tu-choi");
    expect(sdk.getPhoneNumber).not.toHaveBeenCalled();
  });

  it("an older Zalo without authorize falls back to getPhoneNumber's own dialog", async () => {
    sdk.getSetting.mockRejectedValue(zaloError(-1404));
    sdk.authorize.mockRejectedValue(zaloError(-1404));
    expect(await xinTokenSoDienThoai()).toEqual({ kieu: "xong", du_lieu: "phone-test" });
  });
});

describe("a code other than -201 carries the capability and Zalo's code", () => {
  it("phone", async () => {
    sdk.getPhoneNumber.mockRejectedValue(zaloError(-1401));
    expect(await xinTokenSoDienThoai()).toEqual({
      kieu: "khong-lay-duoc",
      failure: { capability: "phone", code: -1401, transient: false },
    });
  });

  it("location", async () => {
    sdk.getLocation.mockRejectedValue(zaloError(-1403));
    expect(await requestLocationCodes()).toEqual({
      kieu: "khong-lay-duoc",
      failure: { capability: "location", code: -1403, transient: false },
    });
  });

  it("name (user info)", async () => {
    sdk.getUserInfo.mockRejectedValue(zaloError(-1401));
    expect(await layTenZalo(true)).toEqual({
      kieu: "khong-lay-duoc",
      failure: { capability: "name", code: -1401, transient: false },
    });
  });

  it("carries no platform message — only the number", async () => {
    sdk.getPhoneNumber.mockRejectedValue(zaloError(-1401));
    expect(JSON.stringify(await xinTokenSoDienThoai())).not.toContain("platform text");
  });
});

describe("-201 is still the citizen's refusal", () => {
  it.each([
    ["phone", () => sdk.getPhoneNumber, xinTokenSoDienThoai],
    ["location", () => sdk.getLocation, requestLocationCodes],
    ["name", () => sdk.getUserInfo, () => layTenZalo(true)],
  ] as const)("%s", async (_, fn, call) => {
    fn().mockRejectedValue(zaloError(-201));
    expect(await call()).toEqual({ kieu: "tu-choi" });
  });
});

describe("two-step calls name the step that failed", () => {
  it("sign-in: the session code fails before the phone is asked", async () => {
    sdk.getAccessToken.mockRejectedValue(zaloError(-1401));
    expect(await xinMaDangNhap()).toEqual({
      kieu: "khong-lay-duoc",
      failure: { capability: "access-token", code: -1401, transient: false },
    });
    expect(sdk.getPhoneNumber).not.toHaveBeenCalled();
  });

  it("sign-in: the phone step fails", async () => {
    sdk.getPhoneNumber.mockRejectedValue(zaloError(-1401));
    expect(await xinMaDangNhap()).toEqual({
      kieu: "khong-lay-duoc",
      failure: { capability: "phone", code: -1401, transient: false },
    });
  });

  it("location codes: the location step fails", async () => {
    sdk.getLocation.mockRejectedValue(zaloError(-1403));
    expect(await requestLocationCodes()).toEqual({
      kieu: "khong-lay-duoc",
      failure: { capability: "location", code: -1403, transient: false },
    });
  });
});

describe("transient is only what the SDK itself calls 'try again later'", () => {
  it.each([-2000, -1408, -1409, -1410])("%i is transient", async (code) => {
    sdk.getPhoneNumber.mockRejectedValue(zaloError(code));
    expect(await xinTokenSoDienThoai()).toEqual({
      kieu: "khong-lay-duoc",
      failure: { capability: "phone", code, transient: true },
    });
  });

  it.each([-1401, -1403, -1404, -2002, -1])("%i is not", async (code) => {
    sdk.getLocation.mockRejectedValue(zaloError(code));
    const result = await requestLocationCodes();
    expect(result.kieu === "khong-lay-duoc" && result.failure?.transient).toBe(false);
  });
});

describe("nothing measured, nothing invented", () => {
  it.each([new Error("no code"), "a string", { code: "-1401" }, null])("throw %j → no failure", async (thrown) => {
    sdk.getPhoneNumber.mockRejectedValue(thrown);
    expect(await xinTokenSoDienThoai()).toEqual({ kieu: "khong-lay-duoc" });
  });

  it("success is unchanged", async () => {
    expect(await xinMaDangNhap()).toEqual({
      kieu: "xong",
      du_lieu: { ma_truy_cap: "access-test", ma_so_dien_thoai: "phone-test" },
    });
  });
});
