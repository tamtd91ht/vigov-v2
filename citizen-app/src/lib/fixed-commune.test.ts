import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CommuneApp } from "../App";
import { CitizenChannel } from "../citizen";
import { communeFromLookup } from "../citizen/screens/CommuneHome";
import { COMMUNE_APP } from "../citizen/screens/copy";
import { COMPANY } from "../content/company-profile";
import { readFixedCommuneDomain, FIXED_COMMUNE_DOMAIN } from "./fixed-commune";

describe("xã cố định của bản dựng", () => {
  it("bản dựng thường (và test) không mang xã nào — app chung", () => {
    expect(FIXED_COMMUNE_DOMAIN).toBeNull();
  });

  it("chỉ nhận tên miền đúng khuôn; mọi thứ khác là app chung, không đoán xã", () => {
    expect(readFixedCommuneDomain("xa-a.vigov.example")).toBe("xa-a.vigov.example");
    for (const v of ["", "localhost", "https://xa-a.vigov.example", undefined, 1]) {
      expect(readFixedCommuneDomain(v)).toBeNull();
    }
  });
});

describe("app riêng: tra xã theo tên miền của bản dựng", () => {
  const commune = { ten: "Xã A", tinh: "Tỉnh B" };

  it("đúng một xã, có tên → xã của app", () => {
    expect(communeFromLookup({ kind: "xong", value: [commune] })).toEqual({ commune });
  });

  it("không thấy / nhiều xã / tên rỗng → câu 'chưa gắn xã'; lỗi mạng → câu 'chưa kết nối'", () => {
    for (const result of [
      { kind: "xong", value: [] },
      { kind: "xong", value: [commune, commune] },
      { kind: "xong", value: [{ ten: " ", tinh: "" }] },
      { kind: "khong-thay" },
      { kind: "khong-hop-le" },
    ] as const) {
      expect(communeFromLookup(result as never)).toEqual({ error: COMMUNE_APP.not_found });
    }
    for (const kind of ["loi-mang", "loi-may-chu", "tam-ngung", "chua-cau-hinh"] as const) {
      expect(communeFromLookup({ kind })).toEqual({ error: COMMUNE_APP.not_connected });
    }
  });

  it("không câu nào của app riêng nhắc phần giới thiệu hay mã QR — app riêng không có cả hai", () => {
    for (const text of Object.values(COMMUNE_APP)) {
      expect(text).not.toMatch(/giới thiệu|QR/i);
    }
  });
});

describe("app riêng: không một chữ ViHAT, không đăng nhập lúc mở (chủ dự án, 27–28/09/2026)", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("màn đầu không mang tên, thanh tab hay nút chat của ViHAT, và không có nút nào", () => {
    const html = renderToStaticMarkup(createElement(CommuneApp, { domain: "xa-a.vigov.example" }));
    expect(html).not.toContain(COMPANY.name);
    expect(html).not.toMatch(/vihat/i);
    expect(html).not.toMatch(/tab-bar|tabbar/i);
    expect(html).not.toContain("<button");
    expect(html).toContain('role="status"');
  });

  it("App.tsx của app riêng không dùng hàm mở phiên của đường QR — mở app không bao giờ gọi cầu đăng nhập", async () => {
    // Đọc mã nguồn: `CommuneApp` không được nhắc `openVigovSession` hay `CommuneConfirmation`. Mở phiên tự động lúc mở
    // app là thứ ghi một lần "xác nhận xã" không ai bấm (ADR 0047 §6). Từ 29/09/2026 `CommuneApp` tiêm
    // `openCommuneAppSession` (App ID + số điện thoại, không tên miền) — hàm ấy chỉ chạy sau cú bấm đồng ý
    // ở việc cá nhân đầu tiên (`citizen/screens/commune-session.ts`), không bao giờ lúc mở app.
    const code = (await import("../App.tsx?raw")).default as string;
    const body = code.slice(code.indexOf("export function CommuneApp("), code.indexOf("function SharedApp("));
    expect(body.length).toBeGreaterThan(0);
    expect(body).not.toMatch(/openVigovSession|CommuneConfirmation|openCitizenSessionViaBridge/);
  });

  it("kênh công dân không có nút Quay lại khi không truyền onClose", () => {
    const with_close = renderToStaticMarkup(createElement(CitizenChannel, { onClose: () => {} }));
    const without_close = renderToStaticMarkup(createElement(CitizenChannel, {}));
    expect(with_close).toContain("quay-lai");
    expect(without_close).not.toContain("quay-lai");
  });
});
