import { describe, expect, it } from "vitest";

import { BAY_DANH_MUC_GHI } from "@/lib/api/danh-muc";

import {
  CATALOGUE_IMPORTS,
  ORG_UNIT_IMPORT_TARGET,
  RESIDENTIAL_UNIT_IMPORT_TARGET,
  STAFF_IMPORT_TARGET,
} from "./excel-import-targets";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";

/**
 * The entries of `PHAN_CHUA_DUNG` must stay TRUE: each one is the description behind a "?" on the
 * screen (ADR 0068 §14) and a line `tools/tien_do_san_pham.py` counts. That each entry REACHES the
 * page is held where it is drawn (`automation-tab.pending.test.tsx`); the collapsed bottom block that
 * used to print them all is retired.
 */
describe("phần chưa dựng của màn Cấu hình — mục còn đúng", () => {
  it("mỗi mục có tên và lý do", () => {
    expect(PHAN_CHUA_DUNG.length).toBeGreaterThan(0);
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten.trim()).not.toBe("");
      expect(p.viSao.trim()).not.toBe("");
    }
  });

  it("không còn mục nói thanh chuyển tab chưa dựng — đã quyết (26/09) và đã dựng", () => {
    // Thanh tab dựng ở `khung-tab-cau-hinh.tsx`. Một mục "chưa có thanh tab" còn sót là câu sai
    // hiện ra với cán bộ ngay bên dưới chính thanh tab ấy — và ca đầu tiên của tệp vẫn xanh.
    expect(
      PHAN_CHUA_DUNG.some((p) => /thanh (chuyển )?tab/i.test(`${p.ten} ${p.viSao}`)),
    ).toBe(false);
  });

  it("không còn mục nói tab Phân quyền chưa lưu được — phần ấy đã dựng", () => {
    expect(PHAN_CHUA_DUNG.some((p) => /Lưu.*Phân quyền|Phân quyền.*Lưu/i.test(p.ten))).toBe(false);
  });

  it("Sơ đồ tổ chức: không còn mục thêm/sửa/xoá bộ phận — cả ba đã dựng (xoá: ADR 0056)", () => {
    const soDo = PHAN_CHUA_DUNG.filter((p) => /Sơ đồ tổ chức/.test(p.ten));
    expect(soDo.some((p) => /thêm|sửa|xoá bộ phận/i.test(p.ten))).toBe(false);
  });

  it("không còn mục Trường bản đồ, Máy chủ thư, Thêm vai trò mới — đã dựng / đã quyết (ADR 0055)", () => {
    for (const re of [/Trường bản đồ/, /Máy chủ thư/, /Thêm vai trò/i]) {
      expect(PHAN_CHUA_DUNG.some((p) => re.test(p.ten))).toBe(false);
    }
  });

  it("không còn mục Nhập Excel — Sơ đồ tổ chức, Thôn, Cán bộ và cả bảy nhóm Danh mục đều nhập được", () => {
    // Every import the spec asks for is wired: the three own targets and all seven catalogue groups.
    // An Excel item left here would be a "chưa có" about something that exists.
    expect(RESIDENTIAL_UNIT_IMPORT_TARGET.routes.imports).toBe("/api/v1/residential-units/imports");
    expect(ORG_UNIT_IMPORT_TARGET.routes.imports).toBe("/api/v1/org-units/imports");
    expect(STAFF_IMPORT_TARGET.routes.imports).toBe("/api/v1/staff/imports");
    expect(Object.keys(CATALOGUE_IMPORTS).sort()).toEqual(
      [...BAY_DANH_MUC_GHI.map((m) => m.khoa)].sort(),
    );
    expect(PHAN_CHUA_DUNG.some((p) => /Excel|Nhập từ/i.test(`${p.ten} ${p.viSao}`))).toBe(false);
  });

  it("Tự động hoá: chỉ còn Gửi báo cáo định kỳ (chờ xuất báo cáo); Tính lại số liệu Tổng quan đã bỏ, không có mục", () => {
    // ADR 0053 / ADR 0068 §14: a job the owner refused gets no placeholder and no entry — an entry
    // would make the progress report count it as "not built yet".
    expect(PHAN_CHUA_DUNG.some((p) => /Tính lại số liệu/.test(`${p.ten} ${p.viSao}`))).toBe(false);
    const report = PHAN_CHUA_DUNG.filter((p) => p.ten === "Gửi báo cáo định kỳ");
    expect(report).toHaveLength(1);
    expect(report[0]!.viSao).toMatch(/xuất\s+báo cáo/);
  });

  it("không còn mục Lời hệ thống — nhóm Báo cáo `report.*` đã có chủ và có tuyến (reporting)", () => {
    expect(PHAN_CHUA_DUNG.some((p) => /Lời hệ thống|report\.\*/.test(`${p.ten} ${p.viSao}`))).toBe(false);
  });

  it("không còn mục Xem nhật ký hệ thống — tab đã dựng (ADR 0054)", () => {
    expect(PHAN_CHUA_DUNG.some((p) => /nhật ký hệ thống/i.test(`${p.ten} ${p.viSao}`))).toBe(false);
  });

  it("Danh mục: không còn mục nào nhắc tới một nhóm Danh mục — cả bảy ghi được và nhập được", () => {
    const re =
      /Loại tài nguyên bản đồ|Loại văn bản|Loại đơn vị dân cư|Khối nhiệm vụ|Loại nhiệm vụ|Mức ưu tiên|Hạng mục kế hoạch vốn/i;
    for (const p of PHAN_CHUA_DUNG) {
      expect(re.test(`${p.ten} ${p.viSao}`)).toBe(false);
    }
  });

  it("không còn mục nói đơn vị mới chưa có người quản trị đầu tiên — đã quyết và đã dựng", () => {
    // Quản trị đầu tiên của xã được gieo ở lần đăng nhập đầu tại tên miền của xã (ADR 0046
    // §Quyết định 2, dựng ở `b55835a`). Mục cũ nói "chưa có đường nào" là câu sai hiện ra với
    // cán bộ, và ca đầu tiên của tệp vẫn xanh khi câu ấy sai — nên ca này canh riêng nó.
    expect(
      PHAN_CHUA_DUNG.some((p) => /quản trị (ĐẦU TIÊN|đầu tiên)/i.test(`${p.ten} ${p.viSao}`)),
    ).toBe(false);
  });
});
