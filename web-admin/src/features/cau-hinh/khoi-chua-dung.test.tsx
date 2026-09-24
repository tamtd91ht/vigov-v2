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

  it("không còn mục nói tab Phân quyền chưa lưu được — phần ấy đã dựng", () => {
    expect(PHAN_CHUA_DUNG.some((p) => /Lưu.*Phân quyền|Phân quyền.*Lưu/i.test(p.ten))).toBe(false);
  });
});
