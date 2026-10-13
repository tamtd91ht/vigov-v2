import { describe, expect, it } from "vitest";

import {
  communeFrom,
  defaultQrFile,
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
    expect(parseQrFlags([`--domain=${DOMAIN}`])).toEqual({ domain: DOMAIN, version: null, out: null });
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
  });
});
