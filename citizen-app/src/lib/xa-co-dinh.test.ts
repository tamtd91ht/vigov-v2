import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AppRieng } from "../App";
import { KenhCongDan } from "../cong-dan";
import { xaTuKetQuaTra } from "../cong-dan/man/TrangXa";
import { APP_RIENG } from "../cong-dan/man/noi-dung";
import { COMPANY } from "../content/company-profile";
import { docXaCoDinh, XA_CO_DINH } from "./xa-co-dinh";

describe("xã cố định của bản dựng", () => {
  it("bản dựng thường (và test) không mang xã nào — app chung", () => {
    expect(XA_CO_DINH).toBeNull();
  });

  it("chỉ nhận tên miền đúng khuôn; mọi thứ khác là app chung, không đoán xã", () => {
    expect(docXaCoDinh("xa-a.vigov.example")).toBe("xa-a.vigov.example");
    for (const v of ["", "localhost", "https://xa-a.vigov.example", undefined, 1]) {
      expect(docXaCoDinh(v)).toBeNull();
    }
  });
});

describe("app riêng: tra xã theo tên miền của bản dựng", () => {
  const xa = { ten: "Xã A", tinh: "Tỉnh B" };

  it("đúng một xã, có tên → xã của app", () => {
    expect(xaTuKetQuaTra({ kieu: "xong", gia_tri: [xa] })).toEqual({ xa });
  });

  it("không thấy / nhiều xã / tên rỗng → câu 'chưa gắn xã'; lỗi mạng → câu 'chưa kết nối'", () => {
    for (const kq of [
      { kieu: "xong", gia_tri: [] },
      { kieu: "xong", gia_tri: [xa, xa] },
      { kieu: "xong", gia_tri: [{ ten: " ", tinh: "" }] },
      { kieu: "khong-thay" },
      { kieu: "khong-hop-le" },
    ] as const) {
      expect(xaTuKetQuaTra(kq as never)).toEqual({ loi: APP_RIENG.khong_thay });
    }
    for (const kieu of ["loi-mang", "loi-may-chu", "tam-ngung", "chua-cau-hinh"] as const) {
      expect(xaTuKetQuaTra({ kieu })).toEqual({ loi: APP_RIENG.chua_ket_noi });
    }
  });

  it("không câu nào của app riêng nhắc phần giới thiệu hay mã QR — app riêng không có cả hai", () => {
    for (const cau of Object.values(APP_RIENG)) {
      expect(cau).not.toMatch(/giới thiệu|QR/i);
    }
  });
});

describe("app riêng: không một chữ ViHAT, không đăng nhập lúc mở (chủ dự án, 27–28/09/2026)", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("màn đầu không mang tên, thanh tab hay nút chat của ViHAT, và không có nút nào", () => {
    const html = renderToStaticMarkup(createElement(AppRieng, { ten_mien: "xa-a.vigov.example" }));
    expect(html).not.toContain(COMPANY.name);
    expect(html).not.toMatch(/vihat/i);
    expect(html).not.toMatch(/tab-bar|tabbar/i);
    expect(html).not.toContain("<button");
    expect(html).toContain('role="status"');
  });

  it("App.tsx của app riêng không dùng hàm mở phiên của đường QR — mở app không bao giờ gọi cầu đăng nhập", async () => {
    // Đọc mã nguồn: `AppRieng` không được nhắc `moPhienViGov` hay `XacNhanXa`. Mở phiên tự động lúc mở
    // app là thứ ghi một lần "xác nhận xã" không ai bấm (ADR 0047 §6). Từ 29/09/2026 `AppRieng` tiêm
    // `openCommuneAppSession` (App ID + số điện thoại, không tên miền) — hàm ấy chỉ chạy sau cú bấm đồng ý
    // ở việc cá nhân đầu tiên (`cong-dan/man/commune-session.ts`), không bao giờ lúc mở app.
    const ma = (await import("../App.tsx?raw")).default as string;
    const than = ma.slice(ma.indexOf("export function AppRieng("), ma.indexOf("function AppChung("));
    expect(than.length).toBeGreaterThan(0);
    expect(than).not.toMatch(/moPhienViGov|XacNhanXa|moPhienCongDanQuaCau/);
  });

  it("kênh công dân không có nút Quay lại khi không truyền onDong", () => {
    const co = renderToStaticMarkup(createElement(KenhCongDan, { onDong: () => {} }));
    const khong = renderToStaticMarkup(createElement(KenhCongDan, {}));
    expect(co).toContain("quay-lai");
    expect(khong).not.toContain("quay-lai");
  });
});
