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
