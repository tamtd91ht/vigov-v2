import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_trangThaiNhiemVuRa,
} from "@/lib/api/schema.gen";

import { NUT_BAT_LAI, NUT_SUA, NUT_TAT, NUT_THEM, NUT_XOA } from "./nhan-danh-muc";
import { BieuMauSuaTrangThai, KhoiTrangThaiNhiemVu } from "./nhom-trang-thai-nhiem-vu";
import {
  CHUA_DOI_GI,
  GHI_CHU_NHOM_TRANG_THAI,
  NUT_DAT_LAI,
  THU_TU_KHONG_PHAI_SO,
  chuMacDinh,
  thanDatLaiMacDinh,
  thanSuaTrangThai,
} from "./trang-thai-nhiem-vu";

/**
 * Nhóm thứ tám của tab Danh mục — `Trạng thái nhiệm vụ` (quyết định #21, ADR 0035 §C).
 *
 * Điều canh chính: nhóm này KHÔNG BAO GIỜ vẽ `+ Thêm mục`, `Tắt`, `Bật lại`, `Xoá` — ở bất kỳ tài
 * khoản nào, bất kỳ dòng nào. Bảy nhóm kia có cả bốn nút ở tầng 1, nên một lần "dùng lại cho gọn"
 * bảng của bảy nhóm ấy sẽ mang chúng sang đây mà màn hình vẫn trông bình thường.
 */

function nhuTrongHTML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

/** Bảy dòng như máy chủ trả: `moi-giao` đã đổi nhãn, `cho-duyet` đã đổi thứ tự. */
const BAY: readonly petitions_trangThaiNhiemVuRa[] = [
  { code: "moi-giao", label: "Chưa thực hiện", order: 1, role: "chinh", default_label: "Mới giao", default_order: 1, customised: true },
  { code: "da-tiep-nhan", label: "Đã tiếp nhận", order: 2, role: "chinh", default_label: "Đã tiếp nhận", default_order: 2, customised: false },
  { code: "cho-duyet", label: "Chờ duyệt", order: 3, role: "chinh", default_label: "Chờ duyệt", default_order: 4, customised: true },
  { code: "dang-thuc-hien", label: "Đang thực hiện", order: 3, role: "chinh", default_label: "Đang thực hiện", default_order: 3, customised: false },
  { code: "hoan-thanh", label: "Hoàn thành", order: 5, role: "chinh", default_label: "Hoàn thành", default_order: 5, customised: false },
  { code: "tam-dung", label: "Tạm dừng", order: 6, role: "re-nhanh", default_label: "Tạm dừng", default_order: 6, customised: false },
  { code: "chuyen-tiep", label: "Chuyển tiếp", order: 7, role: "re-nhanh", default_label: "Chuyển tiếp", default_order: 7, customised: false },
];

const DOC_DUOC: KetQua<petitions_danhSachTrangThaiNhiemVuRa> = {
  ok: true,
  duLieu: { items: [...BAY] },
};

function ve(
  tai: KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null,
  coQuyenGhi = true,
): string {
  return renderToStaticMarkup(
    <KhoiTrangThaiNhiemVu
      tai={tai}
      coQuyenGhi={coQuyenGhi}
      dangGui={false}
      cauDaXong=""
      loiNhom=""
      onSua={() => {}}
      onDatLai={() => {}}
      form={null}
    />,
  );
}

function demChuoi(html: string, chuoi: string): number {
  return html.split(chuoi).length - 1;
}

describe("nhóm Trạng thái nhiệm vụ — bảy dòng, không thêm · tắt · xoá (#21)", () => {
  it("vẽ đủ BẢY dòng, đúng thứ tự máy chủ trả, mã font mono", () => {
    const html = ve(DOC_DUOC);
    const tbody = html.slice(html.indexOf("<tbody>"));
    expect(demChuoi(tbody, "<tr>")).toBe(7);
    // Thứ tự vẽ = thứ tự nhận (máy chủ đã sắp; hoà thì theo mặc định). `cho-duyet` đứng trước
    // `dang-thuc-hien` vì xã đã đặt nó lên thứ 3.
    expect(tbody.indexOf(">cho-duyet<")).toBeLessThan(tbody.indexOf(">dang-thuc-hien<"));
    expect(html).toContain('<td class="ma-muc">moi-giao</td>');
  });

  it("KHÔNG có `+ Thêm mục`, `Tắt`, `Bật lại`, `Xoá` — kể cả với quyền admin.lookup", () => {
    const html = ve(DOC_DUOC, true);
    expect(html).not.toContain(nhuTrongHTML(NUT_THEM));
    expect(html).not.toContain(`>${NUT_TAT}<`);
    expect(html).not.toContain(`>${NUT_BAT_LAI}<`);
    expect(html).not.toContain(nhuTrongHTML(NUT_XOA));
    // Và nói lý do một lần, ngay dưới tiêu đề.
    expect(html).toContain(nhuTrongHTML(GHI_CHU_NHOM_TRANG_THAI));
  });

  it("có quyền: mỗi dòng một nút Sửa; `Đặt lại mặc định` CHỈ ở dòng đã đổi", () => {
    const html = ve(DOC_DUOC, true);
    expect(demChuoi(html, `>${NUT_SUA}<`)).toBe(7);
    expect(demChuoi(html, `>${NUT_DAT_LAI}<`)).toBe(2);
  });

  it("KHÔNG có quyền admin.lookup: không Sửa, không Đặt lại — bảng VẪN hiện", () => {
    // CA BỊ TỪ CHỐI. Ẩn nút là tiện dụng; máy chủ vẫn kiểm `RequirePermission` trên từng PATCH.
    const html = ve(DOC_DUOC, false);
    expect(html).not.toContain(`>${NUT_SUA}<`);
    expect(html).not.toContain(`>${NUT_DAT_LAI}<`);
    expect(html).toContain("moi-giao");
    expect(html).toContain("Chưa thực hiện");
  });

  it("cột Vai trò bằng chữ người đọc, badge Đã đổi kèm chữ mặc định", () => {
    const html = ve(DOC_DUOC);
    expect(html).toContain("Chính");
    expect(html).toContain("Rẽ nhánh");
    expect(demChuoi(html, ">Đã đổi<")).toBe(2);
    expect(html).toContain(nhuTrongHTML(chuMacDinh(BAY[0] as petitions_trangThaiNhiemVuRa)));
  });

  it("đọc hỏng: câu máy chủ nguyên văn, role=alert, không vẽ bảng", () => {
    const html = ve({ ok: false, thongBao: "Đã xảy ra lỗi. Vui lòng thử lại." });
    expect(html).toContain("Đã xảy ra lỗi. Vui lòng thử lại.");
    expect(html).toContain('role="alert"');
    expect(html).not.toContain("<table");
  });

  it("biểu mẫu sửa: chỉ ô Nhãn và ô Thứ tự — không ô mã, không ô Đang dùng", () => {
    const html = renderToStaticMarkup(
      <BieuMauSuaTrangThai
        dong={BAY[0] as petitions_trangThaiNhiemVuRa}
        ban={{ nhan: "Chưa thực hiện", thuTu: "1" }}
        datBan={() => {}}
        loiTaiCho=""
        loiMayChu=""
        dangGui={false}
        onGui={() => {}}
        onHuy={() => {}}
      />,
    );
    expect(html).toContain('id="o-nhan-trang-thai"');
    expect(html).toContain('id="o-thu-tu-trang-thai"');
    expect(html).not.toContain('name="ma"');
    expect(html).not.toContain('type="checkbox"');
  });
});

describe("thân PATCH — dựng từng trường, chỉ trường đã đổi", () => {
  const dong = BAY[1] as petitions_trangThaiNhiemVuRa; // da-tiep-nhan, "Đã tiếp nhận", 2

  it("chỉ đổi nhãn → thân CHỈ có `label`, đã cắt khoảng trắng", () => {
    expect(thanSuaTrangThai(dong, { nhan: "  Đã nhận việc  ", thuTu: "2" })).toEqual({
      ok: true,
      than: { label: "Đã nhận việc" },
    });
  });

  it("chỉ đổi thứ tự → thân CHỈ có `order`; ô thứ tự rỗng là KHÔNG ĐỔI", () => {
    expect(thanSuaTrangThai(dong, { nhan: "Đã tiếp nhận", thuTu: "8" })).toEqual({
      ok: true,
      than: { order: 8 },
    });
    expect(thanSuaTrangThai(dong, { nhan: "Mới", thuTu: "" })).toEqual({
      ok: true,
      than: { label: "Mới" },
    });
  });

  it("KHÔNG BAO GIỜ có `code` hay `active` trong thân", () => {
    const kq = thanSuaTrangThai(dong, { nhan: "X", thuTu: "9" });
    expect(kq.ok && Object.keys(kq.than).sort()).toEqual(["label", "order"]);
  });

  it("thứ tự gõ sai: báo tại chỗ, không lặng lẽ bỏ trường đi", () => {
    expect(thanSuaTrangThai(dong, { nhan: "X", thuTu: "3 chữ" })).toEqual({
      ok: false,
      loi: THU_TU_KHONG_PHAI_SO,
    });
  });

  it("không đổi gì: không gửi", () => {
    expect(thanSuaTrangThai(dong, { nhan: "Đã tiếp nhận", thuTu: "2" })).toEqual({
      ok: false,
      loi: CHUA_DOI_GI,
    });
  });

  it("`Đặt lại mặc định` = đúng `default_label` + `default_order` máy chủ trả", () => {
    expect(thanDatLaiMacDinh(BAY[2] as petitions_trangThaiNhiemVuRa)).toEqual({
      label: "Chờ duyệt",
      order: 4,
    });
  });
});
