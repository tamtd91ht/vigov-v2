import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { DanhBaLienHe } from "./danh-ba-lien-he";
import { PHAN_CHUA_DUNG, TIEU_DE_PHAN_CHUA_DUNG } from "./nhan-danh-ba";

/**
 * `PHAN_CHUA_DUNG` CHỈ CÓ NGHĨA KHI NÓ RA TỚI TRANG. `tools/tien_do_san_pham.py` đếm mảng ấy và in
 * con số vào báo cáo tiến độ với lời hứa "số câu cán bộ THẬT SỰ đọc được trên màn" — lời hứa ấy
 * chỉ đúng nếu có một phép kiểm đòi từng mục có mặt trong HTML. Không có phép kiểm này thì một lần
 * sửa bỏ `<KhoiChuaMo />` khỏi trang vẫn để báo cáo in bảy, trong khi cán bộ đọc được không.
 *
 * Dựng tĩnh: effect không chạy khi dựng phía máy chủ, nên không có lời gọi API nào đi ra — khối
 * phần chưa mở không phụ thuộc dữ liệu và luôn có mặt.
 */
describe("khối phần chưa mở — cái ra tới trang", () => {
  it("hiện tiêu đề và TỪNG mục, cả tên lẫn lý do", () => {
    const html = renderToStaticMarkup(<DanhBaLienHe />);

    expect(html).toContain(TIEU_DE_PHAN_CHUA_DUNG);
    expect(PHAN_CHUA_DUNG.length).toBeGreaterThan(0);
    for (const p of PHAN_CHUA_DUNG) {
      expect(html).toContain(p.ten);
      expect(html).toContain(p.viSao);
    }
  });
});
