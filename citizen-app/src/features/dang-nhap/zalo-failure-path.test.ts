import { describe, expect, it, vi } from "vitest";

import {
  sangKieuCongDan,
  toCommuneAppSessionResult,
  toReopenWithPhoneResult,
  toSceneLocationResult,
  toZaloFailure,
} from "../../App";
import type { KetQuaXin, LocationCodes, MaDangNhap, SdkFailure } from "../tinh-nang/zalo-api";

import { moPhienCongDanQuaCau, openCommuneAppSessionWithPhone, reopenCitizenSessionWithPhone } from "./cau-vigov";
import { getCurrentLocation } from "./current-location";

/**
 * ZALO'S CODE CROSSES FROM THE SDK STEP TO THE STATE HALF UNCHANGED — the bridge files (`cau-vigov.ts`,
 * `current-location.ts`) and the shell's tables (`App.tsx`). A code lost on the way is the old "thử lại"
 * loop again, with every test above still green.
 */

const PHONE: SdkFailure = { capability: "phone", code: -1401, transient: false };
const LOCATION: SdkFailure = { capability: "location", code: -1403, transient: false };
const ACCESS: SdkFailure = { capability: "access-token", code: -1408, transient: true };

const failed = <T>(failure: SdkFailure): KetQuaXin<T> => ({ kieu: "khong-lay-duoc", failure });
const never = vi.fn(async () => {
  throw new Error("must not be called");
});

describe("the bridge keeps the failure and calls nothing", () => {
  it("commune app sign-in: phone refused by Zalo", async () => {
    const result = await openCommuneAppSessionWithPhone(
      "https://identity.vidu.example",
      () => "1234567890",
      async () => failed<MaDangNhap>(PHONE),
      never,
    );
    expect(result).toEqual({ kieu: "khong-lay-duoc-ma", failure: PHONE });
    expect(never).not.toHaveBeenCalled();
  });

  it("commune app sign-in: -201 is still refusal", async () => {
    const result = await openCommuneAppSessionWithPhone("https://identity.vidu.example", () => "1234567890", async () => ({ kieu: "tu-choi" }), never);
    expect(result).toEqual({ kieu: "tu-choi" });
  });

  it("reopen with phone", async () => {
    expect(await reopenCitizenSessionWithPhone("xa.vidu.vn", async () => failed<MaDangNhap>(PHONE), never)).toEqual({
      kieu: "khong-lay-duoc-ma",
      failure: PHONE,
    });
  });

  it("session after the commune is confirmed (session code only)", async () => {
    expect(await moPhienCongDanQuaCau("xa.vidu.vn", async () => failed<string>(ACCESS), never)).toEqual({
      kieu: "khong-lay-duoc-ma",
      failure: ACCESS,
    });
  });

  it("location", async () => {
    expect(await getCurrentLocation(async () => failed<LocationCodes>(LOCATION), never)).toEqual({
      kieu: "khong-lay-duoc-ma",
      failure: LOCATION,
    });
  });

  it("an empty code measured nothing — no failure appears", async () => {
    const empty: KetQuaXin<LocationCodes> = { kieu: "xong", du_lieu: { access_token: "a", location_token: "" } };
    expect(await getCurrentLocation(async () => empty, never)).toEqual({ kieu: "khong-lay-duoc-ma" });
  });
});

describe("the shell's tables hand it to the state half", () => {
  const noCode = (failure: SdkFailure) => ({ kieu: "khong-lay-duoc-ma" as const, failure });

  it("commune app session", () => {
    expect(toCommuneAppSessionResult(noCode(PHONE))).toEqual({ kieu: "thu-lai", zalo: PHONE });
  });

  it("session after confirmation, and the reopen", () => {
    expect(sangKieuCongDan(noCode(ACCESS))).toEqual({ kieu: "thu-lai", zalo: ACCESS });
    expect(toReopenWithPhoneResult(noCode(PHONE))).toEqual({ kieu: "thu-lai", zalo: PHONE });
  });

  it("location", () => {
    expect(toSceneLocationResult(noCode(LOCATION))).toEqual({ kind: "thu-lai", zalo: LOCATION });
  });

  it("no failure → the plain retry branch, with no `zalo` field", () => {
    expect(toCommuneAppSessionResult({ kieu: "khong-lay-duoc-ma" })).toEqual({ kieu: "thu-lai" });
    expect(toSceneLocationResult({ kieu: "khong-lay-duoc-ma" })).toEqual({ kind: "thu-lai" });
  });

  it("a capability the state half never asks for crosses as nothing", () => {
    expect(toZaloFailure({ capability: "other", code: -1401, transient: false })).toBeUndefined();
  });
});
