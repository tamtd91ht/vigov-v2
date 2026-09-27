import { describe, expect, it } from "vitest";

import { buocTuDongSauTraXa } from "../cong-dan/man/XacNhanXa";
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

describe("app riêng: tra xã xong là mở phiên, không hỏi", () => {
  const xa = { ten: "Xã A", tinh: "Tỉnh B" };

  it("đúng một xã, có tên → mở", () => {
    expect(buocTuDongSauTraXa({ kieu: "xong", gia_tri: [xa] })).toEqual({ mo: xa });
  });

  it("không thấy / nhiều xã / tên rỗng / lỗi mạng → fail closed, không mở gì", () => {
    for (const kq of [
      { kieu: "xong", gia_tri: [] },
      { kieu: "xong", gia_tri: [xa, xa] },
      { kieu: "xong", gia_tri: [{ ten: " ", tinh: "" }] },
      { kieu: "khong-thay" },
      { kieu: "loi-mang" },
    ] as const) {
      const b = buocTuDongSauTraXa(kq as never);
      expect("ket_thuc" in b && b.ket_thuc.kieu).toBe("ve-gioi-thieu");
    }
  });
});

describe("app riêng: không một chữ ViHAT, không nút xác nhận (chủ dự án, 27/09/2026)", async () => {
  const { createElement } = await import("react");
  const { renderToStaticMarkup } = await import("react-dom/server");
  const { AppRieng } = await import("../App");
  const { COMPANY } = await import("../content/company-profile");
  const { KenhCongDan } = await import("../cong-dan");
  const { XacNhanXa, buocTuDongSauMoPhien } = await import("../cong-dan/man/XacNhanXa");

  it("màn đầu của app riêng không mang tên, thanh tab hay nút chat của ViHAT", () => {
    const html = renderToStaticMarkup(createElement(AppRieng, { ten_mien: "xa-a.vigov.example" }));
    expect(html).not.toContain(COMPANY.name);
    expect(html).not.toMatch(/vihat/i);
    expect(html).not.toMatch(/tab-bar|tabbar/i);
    expect(html).not.toContain("<button");
  });

  it("đang mở: chỉ một dòng trạng thái, không nút nào để bấm", () => {
    const html = renderToStaticMarkup(
      createElement(XacNhanXa, {
        ten_mien: "xa-a.vigov.example",
        nguon: "app-rieng",
        moPhienViGov: async () => ({ kieu: "chua-mo" as const }),
        onKetThuc: () => {},
        tu_dong: true,
      }),
    );
    expect(html).not.toContain("<button");
    expect(html).toContain('role="status"');
  });

  it("mở phiên không thành thì vẫn vào kênh (không phiên) — không bao giờ quay lại hỏi", () => {
    const xa = { ten: "Xã A", tinh: "Tỉnh B" };
    for (const kieu of ["thu-lai", "chua-mo", "ngoai-zalo"] as const) {
      expect(buocTuDongSauMoPhien(xa, { kieu })).toEqual({ kieu: "xac-nhan-khong-phien", xa });
    }
    expect(buocTuDongSauMoPhien(xa, { kieu: "da-mo", ten_xa: "Xã A", ten_mien: null })).toEqual({
      kieu: "da-mo",
      ten_xa: "Xã A",
      ten_mien: null,
    });
  });

  it("kênh công dân không có nút Quay lại khi không truyền onDong", () => {
    const co = renderToStaticMarkup(createElement(KenhCongDan, { onDong: () => {} }));
    const khong = renderToStaticMarkup(createElement(KenhCongDan, {}));
    expect(co).toContain("quay-lai");
    expect(khong).not.toContain("quay-lai");
  });
});
