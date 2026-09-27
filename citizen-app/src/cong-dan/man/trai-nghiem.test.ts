import { describe, expect, it } from "vitest";

import type { CanBoCongKhai, TinXaTomTat } from "../api/hop-dong-cong-khai";
import { TRUONG_DUOC_NHAN } from "../api/hop-dong-phan-anh";

import { nhomTheoBoPhan } from "./DanhBaXa";
import { TRANG_THAI } from "./noi-dung";
import { chuyenMucCua, tinLienQuan } from "./TinTucAppXa";
import {
  cheHoTen,
  cheSoDienThoai,
  chuCaiDau,
  kiemNhapPhieu,
  loiChao,
  maPhieuTraiNghiem,
  nhomCua,
  type NhapPhieu,
  taoPhieuTraiNghiem,
  traPhieuTraiNghiem,
  VONG_DOI,
} from "./trai-nghiem";

const CAU = { thieu: "thiếu", qua_dai: (n: number) => `quá ${n}` };
const NHAP: NhapPhieu = {
  noi_dung: "  Rác tồn đọng đầu ngõ 12 ",
  dia_chi: " Ngõ 12 ",
  ho_ten: "Nguyễn Văn An",
  dien_thoai: "0900000000",
  an_danh: false,
};

describe("họ tên: xin quyền Zalo, không đăng nhập, không người dùng giả lập", () => {
  it("không còn người dùng giả lập hay màn định danh nào trong nửa nhà nước", () => {
    const tep = import.meta.glob(["./*.ts", "./*.tsx"], { query: "?raw", import: "default", eager: true }) as Record<string, string>;
    for (const [duong, ma] of Object.entries(tep)) {
      if (duong.includes(".test.")) continue;
      expect(ma, duong).not.toMatch(/NGUOI_DUNG_GIA_LAP|layNguoiDungGiaLap|DinhDanhXa/);
    }
  });

  it("app riêng không xin quyền số điện thoại — Zalo chỉ trả mã, không có máy chủ đổi", () => {
    const app = import.meta.glob("../../App.tsx", { query: "?raw", import: "default", eager: true }) as Record<string, string>;
    const ma = Object.values(app)[0]!;
    const than = ma.slice(ma.indexOf("export function AppRieng("), ma.indexOf("function AppChung("));
    expect(than.length).toBeGreaterThan(0);
    expect(than).not.toMatch(/xinTokenSoDienThoai|xinHaiMaDangNhap/);
    expect(than).toMatch(/layTenZalo/);
  });

  it("che số điện thoại và họ tên cùng khuôn máy chủ che cho người gửi", () => {
    expect(cheSoDienThoai("0900000000")).toBe("09****0000");
    expect(cheSoDienThoai("12")).toBe("••••");
    expect(cheHoTen("Nguyễn Văn An")).toBe("Nguyễn V. A.");
    expect(cheHoTen("  ")).toBe("");
  });

  it("chữ cái đầu lấy theo tên gọi; lời chào theo giờ", () => {
    expect(chuCaiDau("Nguyễn Văn An")).toBe("A");
    expect(loiChao(8)).toBe("Chào buổi sáng");
    expect(loiChao(14)).toBe("Chào buổi chiều");
    expect(loiChao(20)).toBe("Chào buổi tối");
  });
});

describe("bản trải nghiệm: phiếu theo đúng hợp đồng thật", () => {
  it("năm ô người dân gõ ứng đúng năm trường máy chủ nhận", () => {
    // noi_dung · dia_chi · ho_ten · dien_thoai · an_danh ↔ content · address · reporter_name · reporter_phone · anonymous
    expect(Object.keys(NHAP)).toHaveLength(TRUONG_DUOC_NHAN.length);
  });

  it("vòng đời dùng đúng các trạng thái có nhãn trong TRANG_THAI", () => {
    for (const tt of VONG_DOI) expect(TRANG_THAI[tt], tt).toBeDefined();
  });

  it("nội dung bắt buộc, các ô khác không; giới hạn độ dài của máy chủ", () => {
    expect(kiemNhapPhieu({ ...NHAP, noi_dung: " " }, CAU)).toEqual({ noi_dung: "thiếu" });
    expect(kiemNhapPhieu({ ...NHAP, dia_chi: "", ho_ten: "", dien_thoai: "" }, CAU)).toEqual({});
    expect(kiemNhapPhieu({ ...NHAP, noi_dung: "a".repeat(4001) }, CAU).noi_dung).toBe("quá 4000");
    // Ẩn danh thì họ tên dài không còn là lỗi: ô ấy không được gửi.
    expect(kiemNhapPhieu({ ...NHAP, an_danh: true, ho_ten: "a".repeat(300) }, CAU)).toEqual({});
  });

  it("phiếu mới: đã tiếp nhận, chưa phân loại, không bịa hạn, người gửi đã che", () => {
    const p = taoPhieuTraiNghiem(NHAP, "2026-09-28T01:00:00Z", "TN-AAAAAAAA");
    expect(p).toMatchObject({
      ma_tra_cuu: "TN-AAAAAAAA",
      trang_thai: "da-tiep-nhan",
      nhan_linh_vuc: "",
      noi_dung: "Rác tồn đọng đầu ngõ 12",
      dia_chi: "Ngõ 12",
      ho_ten_da_che: "Nguyễn V. A.",
      dien_thoai_da_che: "09****0000",
      han_tiep_nhan: null,
      han_xu_ly_xong: null,
    });
  });

  it("ẩn danh: không giữ họ tên, không giữ số điện thoại — cùng lời hứa của thanGuiPhanAnh", () => {
    const p = taoPhieuTraiNghiem({ ...NHAP, an_danh: true }, "2026-09-28T01:00:00Z", "TN-AAAAAAAA");
    expect(p.ho_ten_da_che).toBe("");
    expect(p.dien_thoai_da_che).toBe("");
    expect(p.an_danh).toBe(true);
  });

  it("mã tra cứu có tiền tố TN- và 8 ký tự không đoán được; tra cứu không phân biệt hoa thường", () => {
    expect(maPhieuTraiNghiem()).toMatch(/^TN-[A-HJ-NP-Z2-9]{8}$/);
    const p = taoPhieuTraiNghiem(NHAP, "2026-09-28T01:00:00Z", "TN-ABCDEFGH");
    expect(traPhieuTraiNghiem([p], " tn-abcdefgh ")).toBe(p);
    expect(traPhieuTraiNghiem([p], "TN-ZZZZZZZZ")).toBeNull();
  });

  it("nhóm lọc theo việc người dân muốn biết", () => {
    expect(nhomCua("da-tiep-nhan")).toBe("dang-cho");
    expect(nhomCua("dang-xu-ly")).toBe("dang-xu-ly");
    expect(nhomCua("da-dong")).toBe("da-xong");
    expect(nhomCua("khong-tiep-nhan")).toBe("da-xong");
  });

  it("không tệp nào của bản trải nghiệm gọi mạng hay ghi xuống máy", () => {
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

describe("tin tức và danh bạ: lọc và nhóm trên dữ liệu đã tải", () => {
  const tin = (id: string, chuyen_muc: string): TinXaTomTat => ({
    id,
    tieu_de: id,
    tom_tat: "",
    chuyen_muc,
    ngay_dang: "2026-09-28",
  }) as TinXaTomTat;

  it("chip chuyên mục lấy đúng những chuyên mục xã đã đăng, theo thứ tự gặp", () => {
    expect(chuyenMucCua([tin("1", "Thông báo"), tin("2", ""), tin("3", "Sự kiện"), tin("4", "Thông báo")])).toEqual([
      "Thông báo",
      "Sự kiện",
    ]);
  });

  it("tin liên quan: cùng chuyên mục, bỏ tin đang đọc, tối đa 3", () => {
    const ds = [tin("1", "A"), tin("2", "A"), tin("3", "B"), tin("4", "A"), tin("5", "A"), tin("6", "A")];
    expect(tinLienQuan(ds, ds[0]!).map((t) => t.id)).toEqual(["2", "4", "5"]);
    expect(tinLienQuan(ds, tin("x", ""))).toEqual([]);
  });

  it("danh bạ nhóm theo bộ phận, giữ thứ tự máy chủ, người không ghi bộ phận ở cuối", () => {
    const cb = (ho_ten: string, bo_phan: string) => ({ ho_ten, bo_phan, chuc_vu: "", so_co_quan: "", di_dong: "", co_zalo: false }) as CanBoCongKhai;
    const n = nhomTheoBoPhan([cb("A", "Địa chính"), cb("B", ""), cb("C", "Văn phòng"), cb("D", "Địa chính")], "Khác");
    expect(n.map((x) => [x.bo_phan, x.can_bo.map((c) => c.ho_ten)])).toEqual([
      ["Địa chính", ["A", "D"]],
      ["Văn phòng", ["C"]],
      ["Khác", ["B"]],
    ]);
  });
});
