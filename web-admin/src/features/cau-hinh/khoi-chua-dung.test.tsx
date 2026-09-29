import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import {
  CATALOGUE_IMPORTS,
  ORG_UNIT_IMPORT_TARGET,
  RESIDENTIAL_UNIT_IMPORT_TARGET,
  STAFF_IMPORT_TARGET,
} from "./excel-import-targets";
import { KhoiChuaDung } from "./khoi-chua-dung";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";

/**
 * Mỗi mục `PHAN_CHUA_DUNG` phải RA TỚI TRANG — đó là lý do mảng ấy tồn tại, và là thứ
 * `tools/tien_do_san_pham.py` giả định khi đếm nó. Ca này KHÔNG kiểm mục còn đúng hay không.
 */
describe("khối phần chưa dựng của màn Cấu hình", () => {
  it("hiện từng mục, cả tên lẫn lý do", () => {
    const html = renderToStaticMarkup(<KhoiChuaDung />);
    expect(PHAN_CHUA_DUNG.length).toBeGreaterThan(0);
    for (const p of PHAN_CHUA_DUNG) {
      expect(html).toContain(`<dt>${p.ten}</dt>`);
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

  it("mục Nhập Excel kể đúng những gì nhập được HÔM NAY, và chỉ còn ba nhóm đang dựng", () => {
    const excel = PHAN_CHUA_DUNG.filter((p) => /Excel/.test(p.ten));
    expect(excel).toHaveLength(1);
    const muc = excel[0]!;
    // §1, §2 and §3 (Người dùng) have their own import targets — none may still be named as missing.
    expect(muc.ten).not.toMatch(/§1|§2|§3|Thôn|Người dùng/);
    expect(muc.ten).toMatch(/ba nhóm còn lại của Danh mục \(§5\)/);
    expect(muc.viSao).toMatch(
      /nhập được từ Excel: Sơ đồ tổ chức, Thôn \/ Tổ dân phố, danh sách cán bộ ở tab Người dùng, và bốn nhóm ở tab Danh mục — Loại tài nguyên bản đồ, Loại văn bản, Loại đơn vị dân cư, Khối nhiệm vụ\./,
    );
    expect(muc.viSao).toMatch(/Ba nhóm Loại nhiệm vụ, Mức ưu tiên nhiệm vụ và Hạng mục kế hoạch vốn/);
    expect(RESIDENTIAL_UNIT_IMPORT_TARGET.routes.imports).toBe("/api/v1/residential-units/imports");
    expect(ORG_UNIT_IMPORT_TARGET.routes.imports).toBe("/api/v1/org-units/imports");
    expect(STAFF_IMPORT_TARGET.routes.imports).toBe("/api/v1/staff/imports");
    // The sentence names every catalogue group wired today — and only those. A group wired in
    // `CATALOGUE_IMPORTS` without this sentence moving is the stale "chưa có" this block exists to avoid.
    expect(Object.keys(CATALOGUE_IMPORTS)).toEqual(["loaiTaiNguyenBanDo", "loaiVanBan", "loaiDonViDanCu", "khoiNhiemVu"]);
    for (const missing of ["loaiNhiemVu", "mucUuTienNhiemVu", "hangMucKeHoachVon"]) {
      expect(Object.keys(CATALOGUE_IMPORTS)).not.toContain(missing);
    }
  });

  it("Tự động hoá đã dựng: chỉ còn mục hai việc không dựng (bỏ / hoãn), kèm lý do", () => {
    const automation = PHAN_CHUA_DUNG.filter((p) => /Tự động hoá/.test(p.ten));
    expect(automation).toHaveLength(1);
    expect(automation[0]!.ten).not.toMatch(/^Tab Tự động hoá/);
    expect(automation[0]!.ten).toMatch(/Tính lại số liệu Tổng quan/);
    expect(automation[0]!.ten).toMatch(/Gửi báo cáo định kỳ/);
    expect(automation[0]!.viSao).toMatch(/ADR 0053/);
    expect(automation[0]!.viSao).toMatch(/ADR 0058/);
  });

  it("không còn mục Lời hệ thống — nhóm Báo cáo `report.*` đã có chủ và có tuyến (reporting)", () => {
    expect(PHAN_CHUA_DUNG.some((p) => /Lời hệ thống|report\.\*/.test(`${p.ten} ${p.viSao}`))).toBe(false);
  });

  it("không còn mục Xem nhật ký hệ thống — tab đã dựng (ADR 0054)", () => {
    expect(PHAN_CHUA_DUNG.some((p) => /nhật ký hệ thống/i.test(`${p.ten} ${p.viSao}`))).toBe(false);
  });

  it("Danh mục: không còn mục nói Loại đơn vị dân cư / Khối nhiệm vụ chưa ghi được — đã dựng", () => {
    // The Excel item names both groups as ones that IMPORT today; it must not name them in its gap
    // (the part from "Ba nhóm" on). Every other item must not name them at all.
    const re = /Loại đơn vị dân cư|Khối nhiệm vụ/i;
    for (const p of PHAN_CHUA_DUNG) {
      if (/Excel/.test(p.ten)) {
        expect(re.test(p.ten)).toBe(false);
        expect(re.test(p.viSao.slice(p.viSao.indexOf("Ba nhóm")))).toBe(false);
      } else {
        expect(re.test(`${p.ten} ${p.viSao}`)).toBe(false);
      }
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
