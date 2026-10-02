import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { BULK_OPEN_BUTTON } from "./bulk-publication";
import { NUT_RUT_MINI_APP, NUT_THEM_MINI_APP } from "./cong-khai";
import { DanhBaLienHe, HangLoc } from "./danh-ba-lien-he";
import { NHAN_XOA_DONG } from "./xoa-dong";
import { GOI_Y_O_TIM, TAT_CA_KHOI, TRUY_VAN_DAU } from "./loc-danh-ba";
import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { PHAN_CHUA_DUNG } from "./nhan-danh-ba";

/**
 * `PHAN_CHUA_DUNG` CHỈ CÓ NGHĨA KHI NÓ RA TỚI TRANG. `tools/tien_do_san_pham.py` đếm mảng ấy với lời
 * hứa "số câu cán bộ THẬT SỰ đọc được trên màn" — nay mỗi mục là mô tả sau một dấu "?" ở đúng chỗ đặc
 * tả đặt nó (ADR 0068 §14). Hai thẻ KPI không phụ thuộc dữ liệu nên luôn có mặt; cột Ảnh đại diện chỉ
 * có khi bảng có dòng (`bang-lien-he.test.tsx`).
 *
 * Dựng tĩnh: effect không chạy khi dựng phía máy chủ, nên không có lời gọi API nào đi ra.
 */
/**
 * Màn đọc phiên qua `usePhien` (để quyết định hai nút Mini App), nên phải có `PhienProvider` bao
 * ngoài. Dựng tĩnh thì effect của provider không chạy: phiên ở trạng thái CHƯA ĐỌC XONG, tức hai nút
 * Mini App ẩn — đúng nhánh fail closed.
 */
function veMan(): string {
  return renderToStaticMarkup(
    <PhienProvider>
      <DanhBaLienHe />
    </PhienProvider>,
  );
}

describe("phần chưa mở — dấu '?' ở đúng chỗ, không còn khối gập cuối màn", () => {
  it("hai thẻ KPI TỔNG SỐ CÁN BỘ / ĐANG HIỆN TRÊN MINI APP mang '?', không con số", () => {
    const html = veMan();
    for (const ten of ["Tổng số cán bộ", "Đang hiện trên Mini App"]) {
      expect(PHAN_CHUA_DUNG.some((p) => p.ten === ten)).toBe(true);
      expect(html).toContain(`aria-label="${pendingMarkerLabel(ten)}"`);
    }
    // The description is behind the "?", not printed on the page; the old <details> block is gone.
    for (const p of PHAN_CHUA_DUNG) expect(html).not.toContain(p.viSao);
    expect(html).not.toContain("<details");
    expect(html).not.toMatch(/phần chưa mở/);
  });

  it("KHÔNG còn dòng 'chưa mở' nào cho ô tìm hay bộ lọc — chúng đã có trên màn hình", () => {
    const html = veMan();
    expect(html).toContain(GOI_Y_O_TIM);
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten).not.toMatch(/Ô tìm|Bộ lọc theo khối/);
    }
  });
});

describe("màn danh bạ — nút Mini App (một người lẫn nhiều người) fail closed", () => {
  it("không ô tick, không nút Mini App, không nút 'Công khai nhiều người' khi phiên chưa đọc xong", () => {
    const html = veMan();
    expect(html).not.toContain('type="checkbox"');
    expect(html).not.toMatch(/đã chọn|Chọn tất cả/);
    expect(html).not.toContain(NUT_THEM_MINI_APP);
    expect(html).not.toContain(NUT_RUT_MINI_APP);
    expect(html).not.toContain(NHAN_XOA_DONG);
    expect(html).not.toContain(BULK_OPEN_BUTTON);
  });
});

describe("hàng lọc — chữ tìm không có đường lên URL", () => {
  const BO_PHAN = [
    { id: "BP-LE", name: "VĂN PHÒNG ĐẢNG ỦY" },
    { id: "BP-CHAN", name: "THƯỜNG TRỰC ĐẢNG UỶ" },
  ];

  function dung(loc = TRUY_VAN_DAU.loc) {
    return renderToStaticMarkup(<HangLoc loc={loc} boPhan={BO_PHAN} doiLoc={() => {}} />);
  }

  it("ô tìm KHÔNG có `name`, form là POST — trước khi JS chạy, Enter không đưa chữ lên URL", () => {
    // Một ô có `name` trong một form GET là `?ten=chu-da-go` trên thanh địa chỉ ngay lần Enter đầu
    // tiên khi bundle chưa nạp xong.
    const html = dung();
    const oTim = /<input[^>]*id="tim-danh-ba"[^>]*>/.exec(html)?.[0] ?? "";
    expect(oTim).not.toBe("");
    expect(oTim).not.toMatch(/\sname=/);
    expect(oTim).toMatch(/autocomplete="off"/i);
    expect(html).toMatch(/<form[^>]*method="post"/);
    expect(html).not.toMatch(/<form[^>]*action=/);
  });

  it("ô tìm mang đúng gợi ý của đặc tả §3, không `maxLength` (đếm sai đơn vị)", () => {
    const html = dung();
    expect(html).toContain(`placeholder="${GOI_Y_O_TIM}"`);
    expect(html).not.toMatch(/maxlength/i);
  });

  it("ô khối: 'Tất cả khối / đơn vị' đứng đầu, rồi từng khối của danh mục", () => {
    const html = dung();
    expect(html).toMatch(new RegExp(`<option value="" selected="">${TAT_CA_KHOI}</option>`));
    for (const bp of BO_PHAN) expect(html).toContain(`<option value="${bp.id}">${bp.name}</option>`);
  });

  it("ô trạng thái: đánh dấu đúng lựa chọn đang áp", () => {
    const html = dung({ tuKhoa: null, boPhan: "BP-CHAN", hienThi: "0" });
    // A segmented control now (spec §7): exactly one radio checked, and it is the "0" one.
    expect(html.match(/name="loc-hien-thi-danh-ba"/g)).toHaveLength(3);
    expect(html.match(/checked=""/g)).toHaveLength(1);
    expect(html).toMatch(/<input (?=[^>]*name="loc-hien-thi-danh-ba")(?=[^>]*value="0")(?=[^>]*checked="")[^>]*>/);
    expect(html).toContain('<option value="BP-CHAN" selected="">');
  });
});
