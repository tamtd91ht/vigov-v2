import { describe, expect, it } from "vitest";

import { thoiDiemVN } from "./thoi-diem";

/**
 * GIỜ GHIM +07 Ở CẢ HAI NỬA — quyết định 27/09/2026.
 *
 * ⚠ CÁC CA DƯỚI SO CHUỖI ĐÃ ĐỊNH DẠNG, NGUYÊN VĂN, VỚI NHỮNG MỐC SÁT NỬA ĐÊM UTC. Đó là nơi múi giờ
 * đổi cả NGÀY chứ không chỉ giờ: 17:00Z là 00:00 hôm sau ở Việt Nam, 23:30Z là 06:30 hôm sau. Một
 * ca chỉ kiểm "không rỗng" (như ca cũ của `ngayDoc`) xanh với bất kỳ định dạng nào, theo bất kỳ
 * múi nào — tức không canh gì cả.
 *
 * Máy chạy test thường ĐẶT +07, nên một bản quay về đọc theo múi của máy có thể ra đúng giờ ở đây.
 * Nó vẫn đỏ vì hai lẽ: `toLocaleString("vi-VN")` in "HH:mm:ss dd/MM/yyyy", không phải định dạng
 * được ghim; và `yeu-cau.test.tsx` · `dang-nhap.test.tsx` canh hai nơi dùng của nửa doanh nghiệp.
 */
describe("thoiDiemVN — mốc sát nửa đêm UTC", () => {
  const CA: ReadonlyArray<readonly [string, string]> = [
    ["2026-09-24T16:59:00Z", "24/09/2026 23:59"],
    ["2026-09-24T17:00:00Z", "25/09/2026 00:00"],
    ["2026-09-24T23:30:00Z", "25/09/2026 06:30"],
    ["2026-09-25T00:00:00Z", "25/09/2026 07:00"],
    // Qua năm: 31/12 17:30Z là mùng 1 Tết Dương lịch ở Việt Nam.
    ["2026-12-31T17:30:00Z", "01/01/2027 00:30"],
    // Chuỗi đã mang +07 thì giữ nguyên giờ ấy.
    ["2026-09-25T00:15:00+07:00", "25/09/2026 00:15"],
    // Phần thập phân của giây (Go RFC3339Nano) không làm lệch phút.
    ["2026-09-24T17:00:59.999Z", "25/09/2026 00:00"],
  ];
  for (const [vao, ra] of CA) {
    it(`${vao} → ${ra}`, () => {
      expect(thoiDiemVN(vao)).toBe(ra);
    });
  }

  it("chuỗi không đọc được thì `null`, không phải 'Invalid Date'", () => {
    expect(thoiDiemVN("")).toBeNull();
    expect(thoiDiemVN("khong-phai-thoi-diem")).toBeNull();
  });
});
