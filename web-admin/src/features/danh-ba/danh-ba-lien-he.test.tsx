import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { NUT_RUT_MINI_APP, NUT_THEM_MINI_APP } from "./cong-khai";
import { DanhBaLienHe, HangLoc } from "./danh-ba-lien-he";
import { NUT_XOA_DONG } from "./xoa-dong";
import { GOI_Y_O_TIM, TAT_CA_KHOI, TRUY_VAN_DAU } from "./loc-danh-ba";
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

describe("khối phần chưa mở — cái ra tới trang", () => {
  it("hiện tiêu đề và TỪNG mục, cả tên lẫn lý do", () => {
    const html = veMan();

    expect(html).toContain(TIEU_DE_PHAN_CHUA_DUNG);
    expect(PHAN_CHUA_DUNG.length).toBeGreaterThan(0);
    for (const p of PHAN_CHUA_DUNG) {
      expect(html).toContain(p.ten);
      expect(html).toContain(p.viSao);
    }
  });

  it("KHÔNG còn dòng 'chưa mở' nào cho ô tìm hay bộ lọc — chúng đã có trên màn hình", () => {
    const html = veMan();
    expect(html).toContain(GOI_Y_O_TIM);
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten).not.toMatch(/Ô tìm|Bộ lọc theo khối/);
    }
  });
});

describe("màn danh bạ — không thao tác hàng loạt nào, và nút Mini App fail closed", () => {
  it("không ô tick chọn dòng, không thanh hàng loạt, không nút Mini App khi phiên chưa đọc xong", () => {
    const html = veMan();
    expect(html).not.toContain('type="checkbox"');
    expect(html).not.toMatch(/đã chọn|Chọn tất cả/);
    expect(html).not.toContain(NUT_THEM_MINI_APP);
    expect(html).not.toContain(NUT_RUT_MINI_APP);
    expect(html).not.toContain(NUT_XOA_DONG);
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
    expect(html).toContain('<option value="0" selected="">Chưa hiện</option>');
    expect(html).toContain('<option value="BP-CHAN" selected="">');
  });
});
