import { afterEach, describe, expect, it } from "vitest";

import { diaChiApi } from "../api/dia-chi";
import { diaChiViGov } from "../cong-dan/api/dia-chi-vigov";
import { SERVICE_API_HOSTS, SERVICE_API_HOSTS_DEV } from "../cong-dan/api/service-hosts.gen";
import { activeProfile, DEV_PROFILE_CODE_SHA256, profileFor, resetRuntimeProfileForTest } from "./runtime-profile";

/**
 * The dev profile is the one switch that sends a launch to another backend. Every case below is the failure that
 * matters: a citizen landing on dev by accident, or a dev link silently testing prod.
 */
const sha256Hex = async (s: string): Promise<string> => {
  const d = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(s));
  return Array.from(new Uint8Array(d), (b) => b.toString(16).padStart(2, "0")).join("");
};

afterEach(() => resetRuntimeProfileForTest());

describe("profileFor", () => {
  it("only a code whose SHA-256 matches selects dev", async () => {
    const code = "AbCdEfGhIjKlMnOpQrStUvWx";
    expect(await profileFor({ pf: code }, await sha256Hex(code))).toBe("dev");
  });

  it("no pf, empty, wrong code, `dev` spelt out, or a digest error → prod", async () => {
    const expected = await sha256Hex("AbCdEfGhIjKlMnOpQrStUvWx");
    const cases: Record<string, string>[] = [{}, { pf: "" }, { pf: "wrong" }, { pf: "dev" }, { env: "dev" }];
    for (const params of cases) {
      expect(await profileFor(params, expected), JSON.stringify(params)).toBe("prod");
    }
    const boom = () => Promise.reject(new Error("no crypto.subtle"));
    expect(await profileFor({ pf: "x" }, expected, boom)).toBe("prod");
    expect(await profileFor({ pf: "x" }, "")).toBe("prod");
  });

  it("the shipped digest is a real SHA-256 and not that of an empty string", () => {
    expect(DEV_PROFILE_CODE_SHA256).toMatch(/^[0-9a-f]{64}$/);
    expect(DEV_PROFILE_CODE_SHA256).not.toBe("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855");
  });
});

describe("host tables follow the profile of the launch", () => {
  it("prod by default: ViGov hosts are the prod table", () => {
    expect(activeProfile()).toBe("prod");
    expect(diaChiViGov("petitions", "/x")).toBe(`${SERVICE_API_HOSTS.petitions}/x`);
  });

  it("dev: every ViGov service moves to the dev table; no service stays on prod", () => {
    resetRuntimeProfileForTest("dev");
    for (const s of ["petitions", "identity", "comms"] as const) {
      expect(diaChiViGov(s, "/x")).toBe(`${SERVICE_API_HOSTS_DEV[s]}/x`);
      expect(diaChiViGov(s, "/x")).not.toBe(`${SERVICE_API_HOSTS[s]}/x`);
    }
  });

  it("dev without a dev vihat-miniapp host: empty (fail closed), never prod's login host", () => {
    resetRuntimeProfileForTest("dev");
    // Vitest builds no `__VIGOV_API_HOST_DEV__`, so the dev login host is empty here: the call must not go anywhere.
    expect(diaChiApi("/api/v1/sessions")).toBe("");
  });
});
