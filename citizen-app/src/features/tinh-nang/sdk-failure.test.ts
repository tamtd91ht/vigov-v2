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
}));
vi.mock("zmp-sdk", () => sdk);
// Coded failures here would POST a real report on a machine whose `.env.local` sets the API host.
vi.mock("../dang-nhap/goi-may-chu", () => ({ reportClientError: async () => {} }));

import { layTenZalo, requestLocationCodes, xinMaDangNhap, xinTokenSoDienThoai, xinTokenViTri } from "./zalo-api";

const zaloError = (code: number) => Object.assign(new Error("platform text that must not travel"), { code });

beforeEach(() => {
  sdk.getAccessToken.mockReset().mockResolvedValue("access-test");
  sdk.getPhoneNumber.mockReset().mockResolvedValue({ token: "phone-test" });
  sdk.getLocation.mockReset().mockResolvedValue({ token: "location-test" });
  sdk.getUserInfo.mockReset().mockResolvedValue({ userInfo: { name: "Nguyễn Văn Thử" } });
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
    expect(await xinTokenViTri()).toEqual({
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
    ["location", () => sdk.getLocation, xinTokenViTri],
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
    const result = await xinTokenViTri();
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
