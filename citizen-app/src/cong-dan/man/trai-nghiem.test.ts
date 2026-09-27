import { describe, expect, it } from "vitest";

import {
  cheSoDienThoai,
  chuCaiDau,
  kiemNhapPhieu,
  layNguoiDungGiaLap,
  loiChao,
  maPhieuTraiNghiem,
  NGUOI_DUNG_GIA_LAP,
  taoPhieuTraiNghiem,
} from "./trai-nghiem";

describe("bản trải nghiệm: người dùng giả lập", () => {
  it("hành động duy nhất hôm nay trả người dùng giả lập cố định, số trong dải số giả", async () => {
    const nd = await layNguoiDungGiaLap();
    expect(nd).toEqual(NGUOI_DUNG_GIA_LAP);
    expect(nd.nguon).toBe("gia-lap");
    expect(nd.so_dien_thoai).toMatch(/^090000000\d$/);
  });

  it("số điện thoại luôn bị che khi hiện ra", () => {
    expect(cheSoDienThoai("0900000000")).toBe("090 •••• 000");
    expect(cheSoDienThoai("12")).toBe("•••");
  });

  it("chữ cái đầu lấy theo tên gọi; lời chào theo giờ", () => {
    expect(chuCaiDau("Nguyễn Văn An")).toBe("A");
    expect(loiChao(8)).toBe("Chào buổi sáng");
    expect(loiChao(14)).toBe("Chào buổi chiều");
    expect(loiChao(20)).toBe("Chào buổi tối");
  });
});

describe("bản trải nghiệm: phiếu chỉ trong máy", () => {
  it("mã phiếu có tiền tố TN- và 8 ký tự không đoán được — không lẫn với mã phiếu thật", () => {
    const ma = maPhieuTraiNghiem();
    expect(ma).toMatch(/^TN-[A-HJ-NP-Z2-9]{8}$/);
    expect(maPhieuTraiNghiem(() => 0)).toBe("TN-AAAAAAAA");
  });

  it("kiểm từng ô trước bước xác nhận", () => {
    expect(kiemNhapPhieu({ tieu_de: "", noi_dung: "", dia_chi: "" })).toEqual({
      tieu_de: expect.any(String),
      noi_dung: expect.any(String),
      dia_chi: expect.any(String),
    });
    expect(kiemNhapPhieu({ tieu_de: "Rác", noi_dung: "x", dia_chi: "y" }).tieu_de).toMatch(/ít nhất 5/);
    expect(kiemNhapPhieu({ tieu_de: "Rác tồn đọng", noi_dung: "x", dia_chi: "y" })).toEqual({});
  });

  it("phiếu mới luôn ở trạng thái 'Mới tiếp nhận', cắt khoảng trắng hai đầu", () => {
    const p = taoPhieuTraiNghiem({ tieu_de: "  Rác tồn đọng ", noi_dung: " a ", dia_chi: " b " }, "08:00 28/09/2026", "TN-AAAAAAAA");
    expect(p).toEqual({
      ma: "TN-AAAAAAAA",
      tieu_de: "Rác tồn đọng",
      noi_dung: "a",
      dia_chi: "b",
      luc_gui: "08:00 28/09/2026",
      trang_thai: "moi",
    });
  });

  it("không tệp nào của bản trải nghiệm gọi mạng hay ghi xuống máy", async () => {
    const tep = import.meta.glob(["./trai-nghiem.ts", "./PhanAnhAppXa.tsx", "./TienIchAppXa.tsx"], {
      query: "?raw",
      import: "default",
      eager: true,
    }) as Record<string, string>;
    expect(Object.keys(tep).length).toBe(3);
    for (const [duong, nguon] of Object.entries(tep)) {
      // Bỏ chú thích: các tệp GIẢI THÍCH bằng lời rằng chúng không dùng localStorage.
      const ma = nguon.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
      expect(ma, duong).not.toMatch(/\bfetch\(|goi-vigov|localStorage|sessionStorage|indexedDB/);
    }
  });
});
