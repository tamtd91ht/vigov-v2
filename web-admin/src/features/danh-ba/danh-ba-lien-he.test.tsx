import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { BULK_DELETE_BUTTON } from "./bulk-publication";
import { NUT_RUT_MINI_APP, NUT_THEM_MINI_APP } from "./cong-khai";
import { DanhBaLienHe, HangLoc } from "./danh-ba-lien-he";
import { NHAN_XOA_DONG } from "./xoa-dong";
import { GOI_Y_O_TIM, TAT_CA_KHOI, TRUY_VAN_DAU } from "./loc-danh-ba";

import { ADD_BUTTON, IMPORT_BUTTON, KPI_PUBLISHED, KPI_TOTAL, MO_TA_TRANG, NHAN_SO_KHOI } from "./nhan-danh-ba";

/**
 * Dựng tĩnh: effect không chạy khi dựng phía máy chủ, nên không có lời gọi API nào đi ra.
 *
 * Màn đọc phiên qua `usePhien`, nên phải có `PhienProvider` bao ngoài. Dựng tĩnh thì phiên ở trạng
 * thái CHƯA ĐỌC XONG — tức nút Mini App, cột chọn và nút 🗑 đều ẩn: đúng nhánh fail closed.
 */
function veMan(): string {
  return renderToStaticMarkup(
    <PhienProvider>
      <DanhBaLienHe />
    </PhienProvider>,
  );
}

describe("đầu tab theo bản mẫu (StaffDirectoryWorkspace.tsx:146-171)", () => {
  it("h1, câu mô tả nguyên văn, rồi hai NÚT (không còn liên kết sang /nguoi-dung)", () => {
    const html = veMan();
    expect(html).toContain(">Danh bạ cán bộ</h1>");
    expect(html).toContain(MO_TA_TRANG);
    const buttons = [...html.matchAll(/<button[^>]*>(.*?)<\/button>/g)].map((m) => m[1]?.replace(/<[^>]*>/g, ""));
    expect(buttons).toContain(IMPORT_BUTTON);
    expect(buttons).toContain(ADD_BUTTON);
    expect(html).not.toContain('href="/nguoi-dung"');
    expect(html.indexOf(IMPORT_BUTTON)).toBeLessThan(html.indexOf(ADD_BUTTON));
  });

  it("không còn hộp ghi chú pháp lý dưới bảng, không icon đầu trang", () => {
    const html = veMan();
    expect(html).not.toContain("Nghị định 13/2023/NĐ-CP: không sao chép");
    expect(html).not.toContain("Công khai nhiều người");
  });
});

describe("ba thẻ KPI — đọc từ GET /api/v1/staff-counts, không bao giờ số 0 khi chưa đếm", () => {
  it("ba nhãn của bản mẫu, đều 'đang đếm…' trước khi đọc xong; không còn dấu '?' nào ở đây", () => {
    const html = veMan();
    for (const label of [KPI_TOTAL, KPI_PUBLISHED, NHAN_SO_KHOI]) expect(html).toContain(label);
    expect(html.match(/đang đếm…/g)).toHaveLength(3);
    expect(html).not.toContain("tính năng đang phát triển");
  });
});

describe("màn danh bạ — nút Mini App, cột chọn và thanh chọn fail closed", () => {
  it("phiên chưa đọc xong: không ô tick, không nút Mini App, không 🗑, không 'Xoá đã chọn'", () => {
    const html = veMan();
    expect(html).not.toContain('type="checkbox"');
    expect(html).not.toMatch(/Đã chọn \d+ người/);
    // The page description quotes the bar's button by name; the BUTTONS are what must be absent.
    expect(html).not.toContain(`aria-label="${NUT_THEM_MINI_APP}:`);
    expect(html).not.toContain(`aria-label="${NUT_RUT_MINI_APP}:`);
    expect(html).not.toMatch(new RegExp(`<button[^>]*>(<svg.*?</svg>)?${NUT_THEM_MINI_APP}</button>`));
    expect(html).not.toContain(NHAN_XOA_DONG);
    expect(html).not.toContain(BULK_DELETE_BUTTON);
  });

  it("đang tải: một khối Skeleton, chưa có câu 'Hiển thị … cán bộ.' nào", () => {
    const html = veMan();
    expect(html).toContain("Đang tải danh bạ…");
    expect(html).not.toMatch(/Hiển thị \d+ cán bộ/);
  });
});

describe("hàng lọc — chữ tìm không có đường lên URL", () => {
  const BO_PHAN = [
    { id: "BP-LE", name: "VĂN PHÒNG ĐẢNG ỦY" },
    { id: "BP-CHAN", name: "THƯỜNG TRỰC ĐẢNG UỶ" },
  ];

  function dung(loc = TRUY_VAN_DAU.loc, tallies: Parameters<typeof HangLoc>[0]["tallies"] = null) {
    return renderToStaticMarkup(<HangLoc loc={loc} boPhan={BO_PHAN} tallies={tallies} doiLoc={() => {}} />);
  }

  it("ô tìm KHÔNG có `name`, form là POST — trước khi JS chạy, Enter không đưa chữ lên URL", () => {
    const html = dung();
    const oTim = /<input[^>]*id="tim-danh-ba"[^>]*>/.exec(html)?.[0] ?? "";
    expect(oTim).not.toBe("");
    expect(oTim).not.toMatch(/\sname=/);
    expect(oTim).toMatch(/autocomplete="off"/i);
    expect(html).toMatch(/<form[^>]*method="post"/);
    expect(html).not.toMatch(/<form[^>]*action=/);
  });

  it("MỘT HÀNG (bảng lỗi dòng 25): mỗi cặp label + select nằm trong span `contents`, không là con trực tiếp của hàng", () => {
    // As direct children of the row `<div>`, the pair matched the legacy `:where(p, div):has(> label + select)`
    // rule, which turns the whole row into a column — search and both selects stacked.
    const html = dung();
    expect(html).toMatch(/<span class="contents"><label for="loc-khoi-danh-ba"[^>]*>[^<]*<\/label><select id="loc-khoi-danh-ba"/);
    expect(html).toMatch(/<span class="contents"><label for="loc-hien-thi-danh-ba"[^>]*>[^<]*<\/label><select id="loc-hien-thi-danh-ba"/);
    expect(html.match(/<\/label><select/g)?.length).toBe(html.match(/<span class="contents"><label/g)?.length);
  });

  it("không còn nút Tìm (áp theo lúc gõ); gợi ý nguyên văn bản mẫu; không `maxLength`", () => {
    const html = dung();
    expect(html).not.toMatch(/<button[^>]*>Tìm<\/button>/);
    expect(html).toContain(`placeholder="${GOI_Y_O_TIM}"`);
    expect(html).not.toMatch(/maxlength/i);
  });

  it("ô khối: 'Tất cả khối / đơn vị' đứng đầu; chưa có số liệu thì chỉ tên khối", () => {
    const html = dung();
    expect(html).toMatch(new RegExp(`<option value="" selected="">${TAT_CA_KHOI}</option>`));
    for (const bp of BO_PHAN) expect(html).toContain(`<option value="${bp.id}">${bp.name}</option>`);
  });

  it("ô khối có số liệu: 'Tên (đang hiện/tổng)'; khối vắng trong `departments` là 0/0", () => {
    const html = dung(TRUY_VAN_DAU.loc, {
      total: 9,
      published: 3,
      departments: [{ id: "BP-LE", total: 5, published: 2 }],
    });
    expect(html).toContain('<option value="BP-LE">VĂN PHÒNG ĐẢNG ỦY (2/5)</option>');
    expect(html).toContain('<option value="BP-CHAN">THƯỜNG TRỰC ĐẢNG UỶ (0/0)</option>');
  });

  it("ô trạng thái: ba lựa chọn của bản mẫu, đánh dấu đúng lựa chọn đang áp, nằm ngoài form tìm", () => {
    const html = dung({ tuKhoa: null, boPhan: "BP-CHAN", hienThi: "0" });
    const select = /<select id="loc-hien-thi-danh-ba"[^>]*>(.*?)<\/select>/.exec(html)?.[1] ?? "";
    expect([...select.matchAll(/<option value="([^"]*)"/g)].map((m) => m[1])).toEqual(["", "1", "0"]);
    expect(select).toContain('<option value="0" selected="">Chưa hiện</option>');
    expect(select).toContain(">Hiện và chưa hiện</option>");
    expect(select).toContain(">Đang hiện trên Mini App</option>");
    expect(select.match(/selected=""/g)).toHaveLength(1);
    expect(html.indexOf('id="loc-hien-thi-danh-ba"')).toBeGreaterThan(html.indexOf("</form>"));
    expect(html).toContain('<option value="BP-CHAN" selected="">');
  });
});
