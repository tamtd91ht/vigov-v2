import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

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

  it("mục Nhập Excel nói Sơ đồ tổ chức nhập được rồi, và chỉ còn §2 · §3 · §5", () => {
    const excel = PHAN_CHUA_DUNG.filter((p) => /Excel/.test(p.ten));
    expect(excel).toHaveLength(1);
    const muc = excel[0]!;
    expect(muc.ten).not.toMatch(/§1/);
    expect(muc.ten).toMatch(/§2.*§3.*§5/);
    expect(muc.viSao).toMatch(/chỉ nhập được Sơ đồ tổ chức/);
  });

  it("còn Tự động hoá; Lời hệ thống chỉ còn 32 câu `report.*`", () => {
    expect(PHAN_CHUA_DUNG.some((p) => /Tự động hoá/.test(p.ten))).toBe(true);
    const messageItems = PHAN_CHUA_DUNG.filter((p) => /Lời hệ thống/.test(p.ten));
    expect(messageItems).toHaveLength(1);
    // Tab Lời hệ thống đã dựng cho Phản ánh và Thu – Chi: mục còn lại chỉ nói về nhóm Báo cáo, và
    // lý do là chủ sở hữu chưa chốt — không phải "chưa có tab".
    expect(messageItems[0]!.ten).toMatch(/report\.\*/);
    expect(messageItems[0]!.ten).toMatch(/32/);
    expect(messageItems[0]!.viSao).toMatch(/ADR 0024/);
    expect(messageItems[0]!.ten).not.toMatch(/^Tab Lời hệ thống/);
  });

  it("không còn mục Xem nhật ký hệ thống — tab đã dựng (ADR 0054)", () => {
    expect(PHAN_CHUA_DUNG.some((p) => /nhật ký hệ thống/i.test(`${p.ten} ${p.viSao}`))).toBe(false);
  });

  it("Danh mục: không còn mục nói Loại đơn vị dân cư / Khối nhiệm vụ chưa ghi được — đã dựng", () => {
    expect(
      PHAN_CHUA_DUNG.some((p) => /Loại đơn vị dân cư|Khối nhiệm vụ/i.test(`${p.ten} ${p.viSao}`)),
    ).toBe(false);
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
