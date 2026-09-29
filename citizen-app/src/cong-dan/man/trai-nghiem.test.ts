import { describe, expect, it } from "vitest";

import type { CanBoCongKhai, TinXaTomTat } from "../api/hop-dong-cong-khai";
import { TRUONG_DUOC_NHAN } from "../api/hop-dong-phan-anh";

import { byDisplayOrder, locCanBo, nhomTheoBoPhan, unitHeadLine } from "./DanhBaXa";
import { TRANG_THAI } from "./noi-dung";
import { buocDaQua } from "./PhanAnhAppXa";
import { groupOf, STATUS_GROUP_LABEL, STEP_LABEL } from "./status-groups";
import { NEWS_CHIPS, tinLienQuan } from "./TinTucAppXa";
import {
  chuCaiDau,
  kiemNhapPhieu,
  LINH_VUC_TAM,
  loiChao,
  NHAN_BUOC,
  NHAN_NHOM,
  nhomCua,
  type NhapPhieu,
  VONG_DOI,
} from "./trai-nghiem";

const CAU = { thieu: "thiếu", thieu_nguoi_gui: "thiếu người gửi", qua_dai: (n: number) => `quá ${n}` };
const NHAP: NhapPhieu = {
  linh_vuc: "Rác thải – Vệ sinh môi trường",
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

  it("app riêng không gọi Zalo xin số trực tiếp — số chỉ đi qua hàm mở phiên (App ID + mã số), sau cú bấm", () => {
    const app = import.meta.glob("../../App.tsx", { query: "?raw", import: "default", eager: true }) as Record<string, string>;
    const ma = Object.values(app)[0]!;
    const than = ma.slice(ma.indexOf("export function AppRieng("), ma.indexOf("function AppChung("));
    expect(than.length).toBeGreaterThan(0);
    expect(than).not.toMatch(/xinTokenSoDienThoai|xinHaiMaDangNhap/);
    expect(than).toMatch(/layTenZalo/);
    expect(than).toMatch(/openSession=\{openCommuneAppSession\}/);
  });

  it("chữ cái đầu lấy theo tên gọi; lời chào theo giờ", () => {
    expect(chuCaiDau("Nguyễn Văn An")).toBe("A");
    expect(loiChao(8)).toBe("Chào buổi sáng");
    expect(loiChao(14)).toBe("Chào buổi chiều");
    expect(loiChao(20)).toBe("Chào buổi tối");
  });
});

describe("biểu mẫu gửi phản ánh của app riêng", () => {
  it("năm ô người dân gõ ứng đúng năm trường máy chủ nhận, cộng lĩnh vực dân chọn (ADR 0050)", () => {
    // noi_dung · dia_chi · ho_ten · dien_thoai · an_danh ↔ content · address · reporter_name · reporter_phone · anonymous
    expect(Object.keys(NHAP).filter((k) => k !== "linh_vuc")).toHaveLength(TRUONG_DUOC_NHAN.length);
  });

  it("danh mục tạm: mười hai tên, không một con số giờ nào (luật 10 cấm #3)", () => {
    expect(LINH_VUC_TAM).toHaveLength(12);
    for (const lv of LINH_VUC_TAM) expect(lv).not.toMatch(/\d+\s*(giờ|ngày|h\b)/i);
  });

  it("vòng đời dùng đúng các trạng thái có nhãn trong TRANG_THAI", () => {
    for (const tt of VONG_DOI) expect(TRANG_THAI[tt], tt).toBeDefined();
  });

  it("bắt buộc theo SRS M4.2: mô tả, và người gửi khi không ẩn danh; giới hạn độ dài của máy chủ", () => {
    expect(kiemNhapPhieu({ ...NHAP, noi_dung: " " }, CAU)).toEqual({ noi_dung: "thiếu" });
    // Không ẩn danh mà bỏ trống họ tên: thiếu người gửi. Nơi xảy ra và số điện thoại vẫn tuỳ chọn.
    expect(kiemNhapPhieu({ ...NHAP, dia_chi: "", ho_ten: "", dien_thoai: "" }, CAU)).toEqual({ ho_ten: "thiếu người gửi" });
    expect(kiemNhapPhieu({ ...NHAP, dia_chi: "", dien_thoai: "" }, CAU)).toEqual({});
    // Ẩn danh thì không cần họ tên.
    expect(kiemNhapPhieu({ ...NHAP, an_danh: true, ho_ten: "" }, CAU)).toEqual({});
    expect(kiemNhapPhieu({ ...NHAP, noi_dung: "a".repeat(4001) }, CAU).noi_dung).toBe("quá 4000");
    // Ẩn danh thì họ tên dài không còn là lỗi: ô ấy không được gửi.
    expect(kiemNhapPhieu({ ...NHAP, an_danh: true, ho_ten: "a".repeat(300) }, CAU)).toEqual({});
  });

  it("chín trạng thái gộp về đúng bốn nhóm người dân thấy (ADR 0050 #5)", () => {
    const bang: Record<string, string> = {
      "da-tiep-nhan": "da-tiep-nhan",
      "dang-phan-loai": "da-tiep-nhan",
      "da-chuyen-xu-ly": "dang-xu-ly",
      "dang-xu-ly": "dang-xu-ly",
      "da-xu-ly": "da-xu-ly-xong",
      "cho-dan-xac-nhan": "da-xu-ly-xong",
      "da-dong": "da-dong",
      "khong-tiep-nhan": "da-dong",
      "chuyen-cap-tren": "da-dong",
    };
    for (const [tt, nhom] of Object.entries(bang)) expect(nhomCua(tt), tt).toBe(nhom);
    for (const [code, group] of Object.entries(bang)) expect(groupOf(code), code).toBe(group);
    // PINNED: neither app guesses a group for an unknown code — the commune app's `nhomCua` is `groupOf`,
    // so a ticket still open is never shown as "Đã đóng" (both apps show the neutral sentence instead).
    expect(groupOf("trang-thai-moi")).toBeNull();
    expect(groupOf("toString")).toBeNull();
    expect(nhomCua("trang-thai-moi")).toBeNull();
    expect(nhomCua("toString")).toBeNull();
    expect(Object.keys(NHAN_NHOM)).toHaveLength(4);
    // One table, two names — not two copies that can drift.
    expect(NHAN_NHOM).toBe(STATUS_GROUP_LABEL);
    expect(NHAN_BUOC).toBe(STEP_LABEL);
    // Mọi trạng thái của cán bộ có nhãn bước trên dòng thời gian — không bước nào hiện mã thô.
    for (const tt of Object.keys(TRANG_THAI)) expect(NHAN_BUOC[tt], tt).toBeDefined();
  });

  it("dòng thời gian chỉ các bước ĐÃ QUA; nhánh kết thúc dừng sau phân loại", () => {
    expect(buocDaQua("da-tiep-nhan")).toEqual(["da-tiep-nhan"]);
    expect(buocDaQua("dang-xu-ly")).toEqual(["da-tiep-nhan", "dang-phan-loai", "da-chuyen-xu-ly", "dang-xu-ly"]);
    expect(buocDaQua("khong-tiep-nhan")).toEqual(["da-tiep-nhan", "dang-phan-loai", "khong-tiep-nhan"]);
  });

  it("no screen file of the commune app calls the network itself or writes to the phone", () => {
    // `PhanAnhAppXa.tsx` now USES the ViGov client (`goi-vigov.ts`, the one file allowed to `fetch`), but
    // calls no `fetch` of its own and touches no storage; the other two stay offline entirely.
    const tep = import.meta.glob(["./trai-nghiem.ts", "./PhanAnhAppXa.tsx", "./TienIchAppXa.tsx"], {
      query: "?raw",
      import: "default",
      eager: true,
    }) as Record<string, string>;
    expect(Object.keys(tep).length).toBe(3);
    for (const [duong, nguon] of Object.entries(tep)) {
      // Bỏ chú thích: các tệp GIẢI THÍCH bằng lời rằng chúng không dùng localStorage.
      const ma = nguon.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
      expect(ma, duong).not.toMatch(/\bfetch\(|localStorage|sessionStorage|indexedDB/);
      if (!duong.endsWith("PhanAnhAppXa.tsx")) expect(ma, duong).not.toMatch(/goi-vigov/);
    }
  });

  it("the in-memory experience is gone: no TN- code, no 'bản trải nghiệm' in any commune-app source", () => {
    const tep = import.meta.glob(["./*.ts", "./*.tsx"], { query: "?raw", import: "default", eager: true }) as Record<
      string,
      string
    >;
    for (const [duong, nguon] of Object.entries(tep)) {
      if (duong.includes(".test.")) continue;
      const code = nguon.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
      expect(code, duong).not.toMatch(/`TN-|"TN-|trải nghiệm|TRẢI NGHIỆM|taoPhieuTraiNghiem|maPhieuTraiNghiem/);
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
    type: null,
  });

  it("news chips are the server's TYPES (?type=), not the free-text categories", () => {
    expect(NEWS_CHIPS).toEqual(["tin-tuc", "su-kien", "thong-bao"]);
    // The old guess is gone: no screen file matches "sự kiện" in a category any more.
    const src = import.meta.glob(["./TrangXa.tsx", "./TinTucAppXa.tsx"], { query: "?raw", import: "default", eager: true }) as Record<
      string,
      string
    >;
    for (const [path, code] of Object.entries(src)) expect(code, path).not.toMatch(/\/sự kiện\/i|chuyenMucCua/);
  });

  it("tin liên quan: cùng chuyên mục, bỏ tin đang đọc, tối đa 3", () => {
    const ds = [tin("1", "A"), tin("2", "A"), tin("3", "B"), tin("4", "A"), tin("5", "A"), tin("6", "A")];
    expect(tinLienQuan(ds, ds[0]!).map((t) => t.id)).toEqual(["2", "4", "5"]);
    expect(tinLienQuan(ds, tin("x", ""))).toEqual([]);
  });

  const person = (ho_ten: string, display_order: number | null, units: string[] = []): CanBoCongKhai => ({
    ho_ten,
    bo_phan: "",
    chuc_vu: "",
    so_co_quan: "",
    di_dong: "",
    co_zalo: false,
    display_order,
    residential_units_headed: units,
  });

  it("directory: the commune's display_order first (ascending, stable), then the rest in server order", () => {
    const ds = [person("A", null), person("B", 2), person("C", null), person("D", 1), person("E", 2)];
    expect(byDisplayOrder(ds).map((c) => c.ho_ten)).toEqual(["D", "B", "E", "A", "C"]);
    // Nothing ordered: the server's order, untouched.
    expect(byDisplayOrder([person("X", null), person("Y", null)]).map((c) => c.ho_ten)).toEqual(["X", "Y"]);
  });

  it("village heads: 'Trưởng thôn <X>', without doubling the unit's own kind", () => {
    expect(unitHeadLine("Hà Lam")).toBe("Trưởng thôn Hà Lam");
    expect(unitHeadLine("Thôn Hà Lam")).toBe("Trưởng thôn Hà Lam");
    expect(unitHeadLine("Tổ dân phố 3")).toBe("Trưởng tổ dân phố 3");
    // A name that merely starts with the letters is not a kind.
    expect(unitHeadLine("Thônxyz")).toBe("Trưởng thôn Thônxyz");
  });

  it("the search finds a village head by the village's name", () => {
    const ds = [person("Lê Văn Bình", null, ["Thôn Hà Lam"]), person("Trần Thị Đào", null)];
    expect(locCanBo(ds, "ha lam").map((c) => c.ho_ten)).toEqual(["Lê Văn Bình"]);
  });

  it("tìm danh bạ không phân biệt dấu, và theo số điện thoại khi từ khoá toàn là số", () => {
    const cb = (ho_ten: string, chuc_vu: string, di_dong: string) =>
      ({ ho_ten, bo_phan: "", chuc_vu, so_co_quan: "", di_dong, co_zalo: false, display_order: null, residential_units_headed: [] }) as CanBoCongKhai;
    const ds = [cb("Trần Thị Đào", "Chủ tịch", "0900 000 000"), cb("Lê Văn Bình", "Công an xã", "")];
    expect(locCanBo(ds, "chu tich").map((c) => c.ho_ten)).toEqual(["Trần Thị Đào"]);
    expect(locCanBo(ds, "dao").map((c) => c.ho_ten)).toEqual(["Trần Thị Đào"]);
    expect(locCanBo(ds, "000 000").map((c) => c.ho_ten)).toEqual(["Trần Thị Đào"]);
    // Chữ lẫn số không so theo số: "xa 000" không được khớp mọi số có chữ 0.
    expect(locCanBo(ds, "xa 000")).toEqual([]);
  });

  it("danh bạ nhóm theo bộ phận, giữ thứ tự máy chủ, người không ghi bộ phận ở cuối", () => {
    const cb = (ho_ten: string, bo_phan: string) =>
      ({ ho_ten, bo_phan, chuc_vu: "", so_co_quan: "", di_dong: "", co_zalo: false, display_order: null, residential_units_headed: [] }) as CanBoCongKhai;
    const n = nhomTheoBoPhan([cb("A", "Địa chính"), cb("B", ""), cb("C", "Văn phòng"), cb("D", "Địa chính")], "Khác");
    expect(n.map((x) => [x.bo_phan, x.can_bo.map((c) => c.ho_ten)])).toEqual([
      ["Địa chính", ["A", "D"]],
      ["Văn phòng", ["C"]],
      ["Khác", ["B"]],
    ]);
  });
});
