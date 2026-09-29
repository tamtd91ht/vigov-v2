import { describe, expect, it } from "vitest";

import type { PublicStaff, CommuneNewsSummary } from "../api/public-contract";
import { ACCEPTED_FIELDS } from "../api/citizen-report-contract";

import { byDisplayOrder, filterStaff, groupByOrgUnit, unitHeadLine } from "./CommuneDirectory";
import { STATUS } from "./copy";
import { stepsPassed } from "./CommuneAppReports";
import { groupOf, STATUS_GROUP_LABEL, STEP_LABEL } from "./status-groups";
import { NEWS_CHIPS, relatedNews } from "./CommuneAppNews";
import {
  initials,
  checkDraft,
  greeting,
  COMMUNE_APP_STEP_LABEL,
  COMMUNE_APP_GROUP_LABEL,
  communeAppGroupOf,
  type ReportDraft,
  LIFECYCLE,
} from "./commune-app-model";

const TEXT = { missing: "thiếu", missing_reporter: "thiếu người gửi", too_long: (n: number) => `quá ${n}` };
const DRAFT: ReportDraft = {
  linh_vuc: "rac-thai",
  noi_dung: "  Rác tồn đọng đầu ngõ 12 ",
  dia_chi: " Ngõ 12 ",
  ho_ten: "Nguyễn Văn An",
  dien_thoai: "0900000000",
  an_danh: false,
};

describe("họ tên: xin quyền Zalo, không đăng nhập, không người dùng giả lập", () => {
  it("không còn người dùng giả lập hay màn định danh nào trong nửa nhà nước", () => {
    const files = import.meta.glob(["./*.ts", "./*.tsx"], { query: "?raw", import: "default", eager: true }) as Record<string, string>;
    for (const [file_path, code] of Object.entries(files)) {
      if (file_path.includes(".test.")) continue;
      expect(code, file_path).not.toMatch(/NGUOI_DUNG_GIA_LAP|layNguoiDungGiaLap|DinhDanhXa/);
    }
  });

  it("app riêng không gọi Zalo xin số trực tiếp — số chỉ đi qua hàm mở phiên (App ID + mã số), sau cú bấm", () => {
    const app = import.meta.glob("../../App.tsx", { query: "?raw", import: "default", eager: true }) as Record<string, string>;
    const code = Object.values(app)[0]!;
    const body = code.slice(code.indexOf("export function CommuneApp("), code.indexOf("function SharedApp("));
    expect(body.length).toBeGreaterThan(0);
    expect(body).not.toMatch(/xinTokenSoDienThoai|xinHaiMaDangNhap/);
    expect(body).toMatch(/layTenZalo/);
    expect(body).toMatch(/openSession=\{openCommuneAppSession\}/);
  });

  it("chữ cái đầu lấy theo tên gọi; lời chào theo giờ", () => {
    expect(initials("Nguyễn Văn An")).toBe("A");
    expect(greeting(8)).toBe("Chào buổi sáng");
    expect(greeting(14)).toBe("Chào buổi chiều");
    expect(greeting(20)).toBe("Chào buổi tối");
  });
});

describe("biểu mẫu gửi phản ánh của app riêng", () => {
  it("năm ô người dân gõ ứng đúng năm trường máy chủ nhận, cộng lĩnh vực dân chọn (ADR 0050)", () => {
    // noi_dung · dia_chi · ho_ten · dien_thoai · an_danh ↔ content · address · reporter_name · reporter_phone · anonymous
    expect(Object.keys(DRAFT).filter((k) => k !== "linh_vuc")).toHaveLength(ACCEPTED_FIELDS.length);
  });

  it("no built-in field list is left anywhere in the commune app (ADR 0060 §3: no fallback)", () => {
    const files = import.meta.glob(["./*.ts", "./*.tsx"], { query: "?raw", import: "default", eager: true }) as Record<string, string>;
    for (const [path, src] of Object.entries(files)) {
      if (path.includes(".test.")) continue;
      const code = src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
      expect(code, path).not.toMatch(/LINH_VUC_TAM|STAFF_CONDUCT_FIELD|"Rác thải – Vệ sinh môi trường"/);
    }
  });

  it("vòng đời dùng đúng các trạng thái có nhãn trong STATUS", () => {
    for (const status of LIFECYCLE) expect(STATUS[status], status).toBeDefined();
  });

  it("bắt buộc theo SRS M4.2: mô tả, và người gửi khi không ẩn danh; giới hạn độ dài của máy chủ", () => {
    expect(checkDraft({ ...DRAFT, noi_dung: " " }, TEXT)).toEqual({ noi_dung: "thiếu" });
    // Không ẩn danh mà bỏ trống họ tên: thiếu người gửi. Nơi xảy ra và số điện thoại vẫn tuỳ chọn.
    expect(checkDraft({ ...DRAFT, dia_chi: "", ho_ten: "", dien_thoai: "" }, TEXT)).toEqual({ ho_ten: "thiếu người gửi" });
    expect(checkDraft({ ...DRAFT, dia_chi: "", dien_thoai: "" }, TEXT)).toEqual({});
    // Ẩn danh thì không cần họ tên.
    expect(checkDraft({ ...DRAFT, an_danh: true, ho_ten: "" }, TEXT)).toEqual({});
    expect(checkDraft({ ...DRAFT, noi_dung: "a".repeat(4001) }, TEXT).noi_dung).toBe("quá 4000");
    // Ẩn danh thì họ tên dài không còn là lỗi: ô ấy không được gửi.
    expect(checkDraft({ ...DRAFT, an_danh: true, ho_ten: "a".repeat(300) }, TEXT)).toEqual({});
  });

  it("chín trạng thái gộp về đúng bốn nhóm người dân thấy (ADR 0050 #5)", () => {
    const table: Record<string, string> = {
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
    for (const [status, group] of Object.entries(table)) expect(communeAppGroupOf(status), status).toBe(group);
    for (const [code, group] of Object.entries(table)) expect(groupOf(code), code).toBe(group);
    // PINNED: neither app guesses a group for an unknown code — the commune app's `communeAppGroupOf` is `groupOf`,
    // so a ticket still open is never shown as "Đã đóng" (both apps show the neutral sentence instead).
    expect(groupOf("trang-thai-moi")).toBeNull();
    expect(groupOf("toString")).toBeNull();
    expect(communeAppGroupOf("trang-thai-moi")).toBeNull();
    expect(communeAppGroupOf("toString")).toBeNull();
    expect(Object.keys(COMMUNE_APP_GROUP_LABEL)).toHaveLength(4);
    // One table, two names — not two copies that can drift.
    expect(COMMUNE_APP_GROUP_LABEL).toBe(STATUS_GROUP_LABEL);
    expect(COMMUNE_APP_STEP_LABEL).toBe(STEP_LABEL);
    // Mọi trạng thái của cán bộ có nhãn bước trên dòng thời gian — không bước nào hiện mã thô.
    for (const status of Object.keys(STATUS)) expect(COMMUNE_APP_STEP_LABEL[status], status).toBeDefined();
  });

  it("dòng thời gian chỉ các bước ĐÃ QUA; nhánh kết thúc dừng sau phân loại", () => {
    expect(stepsPassed("da-tiep-nhan")).toEqual(["da-tiep-nhan"]);
    expect(stepsPassed("dang-xu-ly")).toEqual(["da-tiep-nhan", "dang-phan-loai", "da-chuyen-xu-ly", "dang-xu-ly"]);
    expect(stepsPassed("khong-tiep-nhan")).toEqual(["da-tiep-nhan", "dang-phan-loai", "khong-tiep-nhan"]);
  });

  it("no screen file of the commune app calls the network itself or writes to the phone", () => {
    // `CommuneAppReports.tsx` now USES the ViGov client (`vigov-client.ts`, the one file allowed to `fetch`), but
    // calls no `fetch` of its own and touches no storage; the other two stay offline entirely.
    const files = import.meta.glob(["./commune-app-model.ts", "./CommuneAppReports.tsx", "./CommuneAppUtilities.tsx"], {
      query: "?raw",
      import: "default",
      eager: true,
    }) as Record<string, string>;
    expect(Object.keys(files).length).toBe(3);
    for (const [file_path, source] of Object.entries(files)) {
      // Bỏ chú thích: các tệp GIẢI THÍCH bằng lời rằng chúng không dùng localStorage.
      const code = source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
      expect(code, file_path).not.toMatch(/\bfetch\(|localStorage|sessionStorage|indexedDB/);
      if (!file_path.endsWith("CommuneAppReports.tsx")) expect(code, file_path).not.toMatch(/vigov-client/);
    }
  });

  it("the in-memory experience is gone: no TN- code, no 'bản trải nghiệm' in any commune-app source", () => {
    const files = import.meta.glob(["./*.ts", "./*.tsx"], { query: "?raw", import: "default", eager: true }) as Record<
      string,
      string
    >;
    for (const [file_path, source] of Object.entries(files)) {
      if (file_path.includes(".test.")) continue;
      const code = source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
      expect(code, file_path).not.toMatch(/`TN-|"TN-|trải nghiệm|TRẢI NGHIỆM|taoPhieuTraiNghiem|maPhieuTraiNghiem/);
    }
  });
});

describe("tin tức và danh bạ: lọc và nhóm trên dữ liệu đã tải", () => {
  const news = (id: string, category: string): CommuneNewsSummary => ({
    id,
    title: id,
    summary: "",
    category,
    published_on: "2026-09-28",
    type: null,
  });

  it("news chips are the server's TYPES (?type=), not the free-text categories", () => {
    expect(NEWS_CHIPS).toEqual(["tin-tuc", "su-kien", "thong-bao"]);
    // The old guess is gone: no screen file matches "sự kiện" in a category any more.
    const src = import.meta.glob(["./CommuneHome.tsx", "./CommuneAppNews.tsx"], { query: "?raw", import: "default", eager: true }) as Record<
      string,
      string
    >;
    expect(Object.keys(src)).toHaveLength(2);
    for (const [path, code] of Object.entries(src)) expect(code, path).not.toMatch(/\/sự kiện\/i|chuyenMucCua/);
  });

  it("tin liên quan: cùng chuyên mục, bỏ tin đang đọc, tối đa 3", () => {
    const list = [news("1", "A"), news("2", "A"), news("3", "B"), news("4", "A"), news("5", "A"), news("6", "A")];
    expect(relatedNews(list, list[0]!).map((t) => t.id)).toEqual(["2", "4", "5"]);
    expect(relatedNews(list, news("x", ""))).toEqual([]);
  });

  const person = (full_name: string, display_order: number | null, units: string[] = []): PublicStaff => ({
    full_name,
    org_unit: "",
    position: "",
    office_phone: "",
    mobile: "",
    has_zalo: false,
    display_order,
    residential_units_headed: units,
  });

  it("directory: the commune's display_order first (ascending, stable), then the rest in server order", () => {
    const list = [person("A", null), person("B", 2), person("C", null), person("D", 1), person("E", 2)];
    expect(byDisplayOrder(list).map((c) => c.full_name)).toEqual(["D", "B", "E", "A", "C"]);
    // Nothing ordered: the server's order, untouched.
    expect(byDisplayOrder([person("X", null), person("Y", null)]).map((c) => c.full_name)).toEqual(["X", "Y"]);
  });

  it("village heads: 'Trưởng thôn <X>', without doubling the unit's own kind", () => {
    expect(unitHeadLine("Hà Lam")).toBe("Trưởng thôn Hà Lam");
    expect(unitHeadLine("Thôn Hà Lam")).toBe("Trưởng thôn Hà Lam");
    expect(unitHeadLine("Tổ dân phố 3")).toBe("Trưởng tổ dân phố 3");
    // A name that merely starts with the letters is not a kind.
    expect(unitHeadLine("Thônxyz")).toBe("Trưởng thôn Thônxyz");
  });

  it("the search finds a village head by the village's name", () => {
    const list = [person("Lê Văn Bình", null, ["Thôn Hà Lam"]), person("Trần Thị Đào", null)];
    expect(filterStaff(list, "ha lam").map((c) => c.full_name)).toEqual(["Lê Văn Bình"]);
  });

  it("tìm danh bạ không phân biệt dấu, và theo số điện thoại khi từ khoá toàn là số", () => {
    const staff = (full_name: string, position: string, mobile: string) =>
      ({ full_name, org_unit: "", position, office_phone: "", mobile, has_zalo: false, display_order: null, residential_units_headed: [] }) as PublicStaff;
    const list = [staff("Trần Thị Đào", "Chủ tịch", "0900 000 000"), staff("Lê Văn Bình", "Công an xã", "")];
    expect(filterStaff(list, "chu tich").map((c) => c.full_name)).toEqual(["Trần Thị Đào"]);
    expect(filterStaff(list, "dao").map((c) => c.full_name)).toEqual(["Trần Thị Đào"]);
    expect(filterStaff(list, "000 000").map((c) => c.full_name)).toEqual(["Trần Thị Đào"]);
    // Chữ lẫn số không so theo số: "xa 000" không được khớp mọi số có chữ 0.
    expect(filterStaff(list, "xa 000")).toEqual([]);
  });

  it("danh bạ nhóm theo bộ phận, giữ thứ tự máy chủ, người không ghi bộ phận ở cuối", () => {
    const staff = (full_name: string, org_unit: string) =>
      ({ full_name, org_unit, position: "", office_phone: "", mobile: "", has_zalo: false, display_order: null, residential_units_headed: [] }) as PublicStaff;
    const n = groupByOrgUnit([staff("A", "Địa chính"), staff("B", ""), staff("C", "Văn phòng"), staff("D", "Địa chính")], "Khác");
    expect(n.map((x) => [x.org_unit, x.staff.map((c) => c.full_name)])).toEqual([
      ["Địa chính", ["A", "D"]],
      ["Văn phòng", ["C"]],
      ["Khác", ["B"]],
    ]);
  });
});
