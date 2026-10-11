import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { HopXoa } from "./hop-xoa";
import { CAU_CO_TAI_KHOAN, GIAI_THICH_XOA } from "./xoa-dong";

/** Hộp xoá — kiểm cái RA TỚI TRANG: câu hỏi, câu giải thích, KHÔNG ô lý do, ca dòng có tài khoản. */

const CB: identity_canBoTomTat = {
  id: "01J00000000000000000000001",
  code: "CB-00123",
  full_name: "Nguyễn Văn A",
  email: "nva@demo.invalid",
  position: "Chuyên viên",
  department_id: "",
  role_id: "",
  phone: "02350000001",
  mobile: "0900000001",
  has_account: false,
  active: true,
  last_login_at: null,
  created_at: "2026-09-01T02:00:00Z",
  has_zalo: false,
  published: false,
  display_order: null,
  consent_recorded_at: null,
};

function ve(canBo: identity_canBoTomTat, loiMayChu = ""): string {
  return renderToStaticMarkup(
    <HopXoa
      canBo={canBo}
      loiMayChu={loiMayChu}
      dangGui={false}
      onGui={() => undefined}
      onHuy={() => undefined}
    />,
  );
}

describe("hộp xoá — dòng không có tài khoản", () => {
  it("câu hỏi gọi tên người, mã; câu giải thích có mặt TRƯỚC nút gửi", () => {
    const html = ve(CB);
    expect(html).toContain("Xoá Nguyễn Văn A khỏi danh bạ?");
    expect(html).toContain("CB-00123");
    expect(html.indexOf(GIAI_THICH_XOA)).toBeGreaterThan(-1);
    expect(html.indexOf(GIAI_THICH_XOA)).toBeLessThan(html.indexOf('type="submit"'));
  });

  it("KHÔNG có ô lý do (bug sheet row 27); Huỷ và nút Xoá", () => {
    const html = ve(CB);
    expect(html).not.toContain("<textarea");
    expect(html).not.toMatch(/Lý do/i);
    expect(html).toContain("Huỷ");
    expect(html).toMatch(/type="submit"[^>]*>[\s\S]*?Xoá/);
  });

  it("câu từ chối của máy chủ hiện NGUYÊN VĂN", () => {
    const cau = "Xã phải luôn còn ít nhất một người quản trị.";
    expect(ve(CB, cau)).toContain(cau);
  });
});

describe("hộp xoá — dòng CÓ tài khoản đăng nhập", () => {
  it("nói vì sao không xoá được, KHÔNG có nút xác nhận", () => {
    const html = ve({ ...CB, has_account: true });
    expect(html).toContain(CAU_CO_TAI_KHOAN);
    expect(html).not.toContain("<textarea");
    expect(html).not.toContain('type="submit"');
    expect(html).toContain("Huỷ");
  });
});
