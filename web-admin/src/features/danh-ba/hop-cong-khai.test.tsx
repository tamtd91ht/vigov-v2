import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  CANH_BAO_CONG_KHAI,
  CANH_BAO_RUT,
  NUT_XAC_NHAN_CONG_KHAI,
  NUT_XAC_NHAN_RUT,
  O_DA_HOI_Y,
  O_THU_TU,
  type BanCongKhai,
} from "./cong-khai";
import { HopCongKhai, type DangMoCongKhai } from "./hop-cong-khai";

/** Hộp công khai / rút — kiểm cái RA TỚI TRANG: nút mờ khi chưa tick, câu cảnh báo có mặt. */

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
  has_zalo: true,
  published: false,
  display_order: 2,
  consent_recorded_at: null,
};

function ve(dangMo: DangMoCongKhai, ban: BanCongKhai, loiMayChu = ""): string {
  return renderToStaticMarkup(
    <HopCongKhai
      dangMo={dangMo}
      ban={ban}
      datBan={() => undefined}
      loiMayChu={loiMayChu}
      dangGui={false}
      onGui={() => undefined}
      onHuy={() => undefined}
    />,
  );
}

/** Thẻ `<button type="submit" …>` của hộp. */
function nutGui(html: string): string {
  return /<button type="submit"[^>]*>[^<]*<\/button>/.exec(html)?.[0] ?? "";
}

describe("hộp công khai", () => {
  it("chưa tick: nút 'Công khai lên Mini App' MỜ", () => {
    const nut = nutGui(ve({ kieu: "congKhai", canBo: CB }, { daHoiY: false, thuTu: "2" }));
    expect(nut).toContain(NUT_XAC_NHAN_CONG_KHAI);
    expect(nut).toMatch(/disabled=""/);
  });

  it("đã tick: nút bấm được", () => {
    const nut = nutGui(ve({ kieu: "congKhai", canBo: CB }, { daHoiY: true, thuTu: "2" }));
    expect(nut).toContain(NUT_XAC_NHAN_CONG_KHAI);
    expect(nut).not.toMatch(/disabled/);
  });

  it("có ô tick bắt buộc, chưa tick sẵn, và câu cảnh báo dữ liệu cá nhân; tiêu đề gọi tên người", () => {
    const html = ve({ kieu: "congKhai", canBo: CB }, { daHoiY: false, thuTu: "" });
    const oTick = /<input[^>]*id="o-da-hoi-y"[^>]*>/.exec(html)?.[0] ?? "";
    expect(oTick).toContain('type="checkbox"');
    expect(oTick).toContain('required=""');
    expect(oTick).not.toContain("checked");
    expect(html).toContain(O_DA_HOI_Y);
    expect(html).toContain(CANH_BAO_CONG_KHAI);
    expect(html).toContain("Nguyễn Văn A");
    expect(html).toContain(O_THU_TU);
  });

  it("câu máy chủ hiện nguyên văn", () => {
    const cau = "Chưa xác nhận đã hỏi ý và được chính người này đồng ý.";
    expect(ve({ kieu: "congKhai", canBo: CB }, { daHoiY: true, thuTu: "" }, cau)).toContain(cau);
  });
});

describe("hộp rút", () => {
  it("nói dấu đồng ý bị xoá và phải hỏi ý lại; nút xác nhận bấm được; không ô tick, không ô thứ tự", () => {
    const html = ve({ kieu: "rut", canBo: { ...CB, published: true } }, { daHoiY: false, thuTu: "" });
    expect(html).toContain(CANH_BAO_RUT);
    const nut = nutGui(html);
    expect(nut).toContain(NUT_XAC_NHAN_RUT);
    expect(nut).not.toMatch(/disabled/);
    expect(html).not.toContain('type="checkbox"');
    expect(html).not.toContain(O_THU_TU);
  });
});
