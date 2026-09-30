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

/*
 * ─── NỐI SỰ KIỆN CỦA HỘP — gọi thẳng component như một hàm ─────────────────────────────────────
 *
 * `HopCongKhai` không có hook, nên gọi nó như một hàm trả về CÂY PHẦN TỬ React, và cây ấy mang
 * nguyên các `onChange` / `onSubmit` thật. Không cần DOM: tìm phần tử theo `id`, gọi đúng hàm nó
 * mang, xem `datBan` nhận gì. `renderToStaticMarkup` ở trên không làm được việc này — HTML tĩnh
 * không có trình xử lý sự kiện nào.
 *
 * Điều chịu lực: CHỈ Ô TICK đổi `daHoiY`. Ô thứ tự mà tự bật `daHoiY` là một lần công khai số di
 * động mang `consent_confirmed: true` mà không ai tick (#12, Nghị định 13) — và `yeuCauCongKhai`
 * không chặn được, vì nó tin `ban.daHoiY`.
 */

type PhanTu = { type: unknown; props: Record<string, unknown> };

function laPhanTu(x: unknown): x is PhanTu {
  return typeof x === "object" && x !== null && "props" in x && "type" in x;
}

function timTheoId(goc: unknown, id: string): PhanTu {
  const ra: PhanTu[] = [];
  const duyet = (n: unknown) => {
    if (Array.isArray(n)) return n.forEach(duyet);
    if (!laPhanTu(n)) return;
    if (n.props.id === id) ra.push(n);
    duyet(n.props.children);
  };
  duyet(goc);
  if (ra.length !== 1) throw new Error(`phải có đúng một phần tử id="${id}", thấy ${ra.length}`);
  return ra[0] as PhanTu;
}

function cayHop(ban: BanCongKhai, datBan: (b: BanCongKhai) => void, onGui = () => undefined) {
  return HopCongKhai({
    dangMo: { kieu: "congKhai", canBo: CB },
    ban,
    datBan,
    loiMayChu: "",
    dangGui: false,
    onGui,
    onHuy: () => undefined,
  });
}

describe("hộp công khai — chỉ ô tick đổi được dấu đồng ý", () => {
  it("gõ vào ô thứ tự KHÔNG tick hộ: `daHoiY` vẫn `false`", () => {
    const nhan: BanCongKhai[] = [];
    const cay = cayHop({ daHoiY: false, thuTu: "2" }, (b) => nhan.push(b));
    const onChange = timTheoId(cay, "o-thu-tu-mini-app").props.onChange as (e: unknown) => void;
    onChange({ target: { value: "5" } });
    expect(nhan).toEqual([{ daHoiY: false, thuTu: "5" }]);
  });

  it("bỏ tick là bỏ thật — `checked: false` cho ra `daHoiY: false`, không lật trạng thái cũ", () => {
    const nhan: BanCongKhai[] = [];
    const oTick = (ban: BanCongKhai) =>
      timTheoId(cayHop(ban, (b) => nhan.push(b)), "o-da-hoi-y").props.onChange as (e: unknown) => void;
    oTick({ daHoiY: true, thuTu: "" })({ target: { checked: false } });
    oTick({ daHoiY: false, thuTu: "" })({ target: { checked: true } });
    // Lặp một sự kiện `checked: true` (hai lần bấm dồn trước một lượt render) KHÔNG được lật về false.
    oTick({ daHoiY: true, thuTu: "" })({ target: { checked: true } });
    expect(nhan.map((b) => b.daHoiY)).toEqual([false, true, true]);
  });

  it("ô tick hiển thị ĐÚNG `ban.daHoiY` — không tự tick khi bản nháp chưa tick", () => {
    expect(timTheoId(cayHop({ daHoiY: false, thuTu: "" }, () => undefined), "o-da-hoi-y").props.checked).toBe(false);
    expect(timTheoId(cayHop({ daHoiY: true, thuTu: "" }, () => undefined), "o-da-hoi-y").props.checked).toBe(true);
  });
});
