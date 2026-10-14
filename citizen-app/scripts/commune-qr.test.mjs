import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import {
  communeFrom,
  defaultQrFile,
  DEV_PROFILE_CODE_VAR,
  devProfileCodeFrom,
  devProfileDigestFrom,
  identityOriginFrom,
  launchCodeLink,
  parseQrFlags,
  sharedAppIdFrom,
} from "./commune-qr.mjs";

/**
 * QR APP CHUNG VÀO MỘT XÃ. Mỗi ca là một tấm QR in sai mà không ai biết cho tới lúc người dân quét:
 * App ID đoán, tên miền identity không biết, tham số bản thử thiếu, cờ của lệnh đẩy lọt vào đây.
 * App ID ở đây là GIẢ.
 */
const DOMAIN = "xa-vi-du.vigov.example";
const APP_ID = "1111111111111111111";
const CODE = "0a1b2c3d4e5f";

describe("parseQrFlags", () => {
  it("cần --domain, tên miền trần", () => {
    expect(() => parseQrFlags([])).toThrow(/Thiếu --domain/);
    expect(() => parseQrFlags(["--domain="])).toThrow(/để trống/);
    expect(() => parseQrFlags(["--domain=https://xa.vigov.vn"])).toThrow(/scheme/);
    expect(parseQrFlags([`--domain=${DOMAIN}`])).toEqual({ domain: DOMAIN, version: null, out: null, profile: "prod" });
  });

  it("--profile: dev hoặc prod, vắng = prod; giá trị khác thì DỪNG", () => {
    expect(parseQrFlags([`--domain=${DOMAIN}`, "--profile=dev"]).profile).toBe("dev");
    expect(parseQrFlags([`--domain=${DOMAIN}`, "--profile=prod"]).profile).toBe("prod");
    for (const bad of ["", "DEV", "staging", "TESTING"]) {
      expect(() => parseQrFlags([`--domain=${DOMAIN}`, `--profile=${bad}`])).toThrow(/--profile/);
    }
  });

  it("--version chỉ nhận số nguyên dương", () => {
    expect(parseQrFlags([`--domain=${DOMAIN}`, "--version=7"]).version).toBe("7");
    for (const bad of ["0", "-1", "a", "", "1.2"]) {
      expect(() => parseQrFlags([`--domain=${DOMAIN}`, `--version=${bad}`])).toThrow(/không phải số bản/);
    }
  });

  it("--out phải là .svg", () => {
    expect(parseQrFlags([`--domain=${DOMAIN}`, "--out=a/b.SVG"]).out).toBe("a/b.SVG");
    expect(() => parseQrFlags([`--domain=${DOMAIN}`, "--out=a.png"])).toThrow(/\.svg/);
  });

  it("cờ lạ và cờ của lệnh đẩy thì DỪNG, không bỏ qua", () => {
    expect(() => parseQrFlags(["--domian=x.vigov.vn"])).toThrow(/không có/);
    for (const c of ["--app=vihat", "--app-id=1", "--phat-hanh", "--thu"]) {
      expect(() => parseQrFlags([`--domain=${DOMAIN}`, c])).toThrow(/zmp:deploy/);
    }
  });
});

describe("identityOriginFrom", () => {
  it("đọc host identity từ tệp sinh", () => {
    const gen = 'export const SERVICE_API_HOSTS = {\n  petitions: "https://p.example",\n  identity: "https://identity.api.vigov.vn",\n} as const;';
    expect(identityOriginFrom(gen)).toBe("https://identity.api.vigov.vn");
  });

  it("profile dev đọc ĐÚNG bảng dev, không lấy dòng identity đầu tiên (của prod)", () => {
    const gen =
      'export const SERVICE_API_HOSTS = {\n  identity: "https://identity.api.vigov.vn",\n} as const;\n' +
      'export const SERVICE_API_HOSTS_DEV = {\n  identity: "https://identity-dev.api.vigov.vn",\n} as const;';
    expect(identityOriginFrom(gen)).toBe("https://identity.api.vigov.vn");
    expect(identityOriginFrom(gen, "dev")).toBe("https://identity-dev.api.vigov.vn");
    expect(() => identityOriginFrom(gen.split("\n").slice(0, 3).join("\n"), "dev")).toThrow(/service-hosts\.gen\.ts/);
  });

  it("không có, rỗng, http hoặc có đường dẫn thì DỪNG", () => {
    for (const gen of ["", 'identity: ""', 'identity: "http://identity.example"', 'identity: "https://identity.example/api"']) {
      expect(() => identityOriginFrom(gen)).toThrow(/service-hosts\.gen\.ts/);
    }
  });
});

describe("sharedAppIdFrom", () => {
  const http = (status, body) => ({ kind: "http", status, body });

  it("200 source=chung → App ID", () => {
    expect(sharedAppIdFrom(http(200, { app_id: APP_ID, source: "chung" }))).toBe(APP_ID);
  });

  it("source=rieng hoặc thân hỏng thì DỪNG", () => {
    expect(() => sharedAppIdFrom(http(200, { app_id: APP_ID, source: "rieng" }))).toThrow(/không phải app chung/);
    expect(() => sharedAppIdFrom(http(200, { app_id: "abc", source: "chung" }))).toThrow(/hợp đồng/);
    expect(() => sharedAppIdFrom(http(200, null))).toThrow(/hợp đồng/);
  });

  it("platform không trả lời được thì DỪNG — không có lối --app-id", () => {
    expect(() => sharedAppIdFrom({ kind: "network", reason: "ECONNREFUSED" })).toThrow(/fail closed/);
    expect(() => sharedAppIdFrom(http(503, null))).toThrow(/fail closed/);
    expect(() => sharedAppIdFrom(http(429, null))).toThrow(/fail closed/);
    expect(() => sharedAppIdFrom(http(404, null))).toThrow(/chưa có tuyến/);
    expect(() => sharedAppIdFrom(http(404, { code: "mini_app_id_not_found" }))).toThrow(/platform-admin/);
    expect(() => sharedAppIdFrom(http(409, { code: "x" }))).toThrow(/hơn một/);
    expect(() => sharedAppIdFrom(http(401, null))).toThrow(/không đúng hợp đồng/);
  });
});

describe("communeFrom", () => {
  const http = (status, body) => ({ kind: "http", status, body });

  it("đúng một xã → tên, tỉnh và mã mở app", () => {
    expect(communeFrom(DOMAIN, http(200, { items: [{ name: "Xã Ví Dụ", province: "Tỉnh Mẫu", launch_code: CODE }] }))).toEqual({
      name: "Xã Ví Dụ",
      province: "Tỉnh Mẫu",
      launch_code: CODE,
    });
  });

  it("identity không trả `launch_code`, hoặc sai khuôn → DỪNG; không lùi về `d`", () => {
    for (const launch_code of [undefined, "", "0A1B2C3D4E5F", "0a1b2c3d4e5", 12, DOMAIN]) {
      const item = { name: "Xã Ví Dụ", province: "Tỉnh Mẫu" };
      if (launch_code !== undefined) item.launch_code = launch_code;
      expect(() => communeFrom(DOMAIN, http(200, { items: [item] })), String(launch_code)).toThrow(/platform-admin/);
    }
  });

  it("identity không biết tên miền thì DỪNG — QR ấy chỉ mở phần giới thiệu", () => {
    expect(() => communeFrom(DOMAIN, http(200, { items: [] }))).toThrow(/không biết xã nào/);
  });

  it("nhiều xã, thân hỏng, lỗi, mạng thì DỪNG", () => {
    expect(() => communeFrom(DOMAIN, http(200, { items: [{ name: "A" }, { name: "B" }] }))).toThrow(/2 xã/);
    expect(() => communeFrom(DOMAIN, http(200, {}))).toThrow(/items/);
    expect(() => communeFrom(DOMAIN, http(200, { items: [{ name: "" }] }))).toThrow(/không có tên/);
    expect(() => communeFrom(DOMAIN, http(500, null))).toThrow(/500/);
    expect(() => communeFrom(DOMAIN, { kind: "network", reason: "ENOTFOUND" })).toThrow(/ENOTFOUND/);
  });
});

describe("launchCodeLink", () => {
  it("bản phát hành: `?x=<mã>&src=qr`, không `d`", () => {
    expect(launchCodeLink({ app_id: APP_ID, launch_code: CODE })).toBe(`https://zalo.me/s/${APP_ID}/?x=${CODE}&src=qr`);
  });

  it("bản thử: env/version của Zalo trước, x/src của ta sau", () => {
    expect(launchCodeLink({ app_id: APP_ID, launch_code: CODE, version: "7" })).toBe(
      `https://zalo.me/s/${APP_ID}/?env=TESTING&version=7&x=${CODE}&src=qr`,
    );
  });

  it("profile dev: `pf` đứng sau x/src; không bao giờ dùng `env` của Zalo", () => {
    const pf = "AbCdEfGhIjKlMnOpQrStUvWx";
    expect(launchCodeLink({ app_id: APP_ID, launch_code: CODE, profile_code: pf })).toBe(
      `https://zalo.me/s/${APP_ID}/?x=${CODE}&src=qr&pf=${pf}`,
    );
    expect(launchCodeLink({ app_id: APP_ID, launch_code: CODE, version: "7", profile_code: pf })).toBe(
      `https://zalo.me/s/${APP_ID}/?env=TESTING&version=7&x=${CODE}&src=qr&pf=${pf}`,
    );
  });

  it("mã sai khuôn thì DỪNG — không in một QR mà app sẽ bỏ qua", () => {
    for (const bad of [null, "", "0A1B2C3D4E5F", DOMAIN]) {
      expect(() => launchCodeLink({ app_id: APP_ID, launch_code: bad })).toThrow(/12 ký tự hex/);
    }
  });
});

describe("tên tệp — bản thử không ghi đè bản phát hành", () => {
  it("mặc định: theo tên miền", () => {
    expect(defaultQrFile(DOMAIN)).toBe(`qr/${DOMAIN}.svg`);
    expect(defaultQrFile(DOMAIN, "7")).toBe(`qr/${DOMAIN}.test-v7.svg`);
    expect(defaultQrFile(DOMAIN, null, "dev")).toBe(`qr/${DOMAIN}.dev.svg`);
    expect(defaultQrFile(DOMAIN, "7", "dev")).toBe(`qr/${DOMAIN}.dev.test-v7.svg`);
  });
});

describe("mã profile dev", () => {
  const sha256Hex = (s) => createHash("sha256").update(s, "utf8").digest("hex");
  const code = "AbCdEfGhIjKlMnOpQrStUvWx";
  const expectedSha256 = sha256Hex(code);

  it("digest đọc từ runtime-profile.ts thật, đúng 64 hex", () => {
    const src = readFileSync(fileURLToPath(new URL("../src/lib/runtime-profile.ts", import.meta.url)), "utf8");
    expect(devProfileDigestFrom(src)).toMatch(/^[0-9a-f]{64}$/);
    expect(() => devProfileDigestFrom("")).toThrow(/DEV_PROFILE_CODE_SHA256/);
  });

  it("shell thắng tệp; mã khớp digest thì nhận", () => {
    expect(devProfileCodeFrom({ env: {}, fileText: `# c\n${DEV_PROFILE_CODE_VAR}=${code}\n`, expectedSha256, sha256Hex })).toBe(code);
    expect(
      devProfileCodeFrom({ env: { [DEV_PROFILE_CODE_VAR]: code }, fileText: `${DEV_PROFILE_CODE_VAR}=khac\n`, expectedSha256, sha256Hex }),
    ).toBe(code);
  });

  it("thiếu, sai khuôn, hoặc không khớp digest thì DỪNG — QR ấy sẽ mở prod", () => {
    expect(() => devProfileCodeFrom({ env: {}, fileText: null, expectedSha256, sha256Hex })).toThrow(/thiếu/);
    expect(() => devProfileCodeFrom({ env: { [DEV_PROFILE_CODE_VAR]: "ngan" }, expectedSha256, sha256Hex })).toThrow(/sai khuôn/);
    expect(() =>
      devProfileCodeFrom({ env: { [DEV_PROFILE_CODE_VAR]: "w34$@225w34$@225w34$@225" }, expectedSha256, sha256Hex }),
    ).toThrow(/sai khuôn/);
    expect(() =>
      devProfileCodeFrom({ env: { [DEV_PROFILE_CODE_VAR]: "ZZZZZZZZZZZZZZZZZZZZZZZZ" }, expectedSha256, sha256Hex }),
    ).toThrow(/mở PROD/);
  });
});
