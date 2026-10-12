import { describe, expect, it } from "vitest";

import {
  communeFrom,
  defaultQrFile,
  identityOriginFrom,
  launchCodeLink,
  outFileFor,
  parseQrFlags,
  sharedAppIdFrom,
  sharedAppLink,
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

  it("máy chủ chưa trả `launch_code`, hoặc trả sai khuôn → `null`, KHÔNG dừng (tệp legacy vẫn ghi)", () => {
    for (const launch_code of [undefined, "", "0A1B2C3D4E5F", "0a1b2c3d4e5", 12, DOMAIN]) {
      const item = { name: "Xã Ví Dụ", province: "Tỉnh Mẫu" };
      if (launch_code !== undefined) item.launch_code = launch_code;
      expect(communeFrom(DOMAIN, http(200, { items: [item] })).launch_code, String(launch_code)).toBeNull();
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

describe("launchCodeLink — QR mới, bản app kế tiếp trở đi", () => {
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

describe("sharedAppLink — QR cũ (d, legacy), bản app đang live trong thời gian chuyển tiếp", () => {
  it("bản phát hành: đúng khuôn service-platform dựng", () => {
    // Same string as service-platform/internal/domain/shared_mini_app_test.go pins for the platform-admin button.
    expect(sharedAppLink({ app_id: "1234567890123456789", domain: "thangbinh-danang.vigov.vn" })).toBe(
      "https://zalo.me/s/1234567890123456789/?d=thangbinh-danang.vigov.vn&src=qr",
    );
  });

  it("bản thử: env/version của Zalo trước, d/src của ta sau", () => {
    expect(sharedAppLink({ app_id: APP_ID, domain: DOMAIN, version: "7" })).toBe(
      `https://zalo.me/s/${APP_ID}/?env=TESTING&version=7&d=${DOMAIN}&src=qr`,
    );
  });
});

describe("tên tệp — hai khuôn và bản thử không ghi đè nhau", () => {
  it("mặc định: theo tên miền, hậu tố -x / -d-legacy", () => {
    expect(defaultQrFile(DOMAIN, null, "x")).toBe(`qr/${DOMAIN}-x.svg`);
    expect(defaultQrFile(DOMAIN, null, "d-legacy")).toBe(`qr/${DOMAIN}-d-legacy.svg`);
    expect(defaultQrFile(DOMAIN, "7", "x")).toBe(`qr/${DOMAIN}.test-v7-x.svg`);
    expect(defaultQrFile(DOMAIN, "7", "d-legacy")).toBe(`qr/${DOMAIN}.test-v7-d-legacy.svg`);
  });

  it("--out: một tên, hai tệp", () => {
    expect(outFileFor("a/b.svg", "x")).toBe("a/b-x.svg");
    expect(outFileFor("a/b.SVG", "d-legacy")).toBe("a/b-d-legacy.SVG");
  });
});
