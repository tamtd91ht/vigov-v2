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

  it("Sơ đồ tổ chức: không còn mục thêm/sửa bộ phận — đã dựng; mục xoá bộ phận vẫn còn", () => {
    const soDo = PHAN_CHUA_DUNG.filter((p) => /Sơ đồ tổ chức/.test(p.ten));
    expect(soDo.some((p) => /thêm|sửa/i.test(p.ten))).toBe(false);
    expect(soDo.some((p) => /xoá bộ phận/i.test(p.ten))).toBe(true);
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
