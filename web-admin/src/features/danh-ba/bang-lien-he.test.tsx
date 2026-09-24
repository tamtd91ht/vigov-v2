import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { KhungQuyen } from "@/features/quyen/cong-quyen";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BangLienHe } from "./bang-lien-he";
import { CAU_THIEU_QUYEN, NUT_SUA_THONG_TIN } from "./nhan-danh-ba";

/**
 * KIỂM CÁI RA TỚI TRANG, KHÔNG CHỈ KIỂM QUYẾT ĐỊNH.
 *
 * `vitest.config.mts` ghi lại vì sao cần cả hai loại: một quyết định đúng nằm trong module thuần
 * mà không component nào đưa ra trang là một quyết định không tồn tại với người dùng. Ở đây thứ
 * phải ra tới trang gồm hai nhóm — số liên hệ của cán bộ (ca được phép) và KHÔNG GÌ CẢ (ca bị từ
 * chối vì thiếu `admin.user`).
 *
 * SỐ ĐIỆN THOẠI TRONG TỆP NÀY LÀ SỐ GIẢ thuộc dải đã thống nhất `0900000xxx` (luật 3, bất biến 5),
 * và tên người cũng là tên giả — đúng bộ dữ liệu `docs/ui-ux/12-danh-ba-can-bo.md §10` đã thay.
 */

function canBo(ghiDe: Partial<identity_canBoTomTat> = {}): identity_canBoTomTat {
  return {
    id: "01J00000000000000000000001",
    code: "CB-00123",
    full_name: "Nguyễn Văn A",
    email: "nva@demo.invalid",
    position: "Bí thư Đảng ủy",
    department_id: "01J0000000000000000000BP01",
    role_id: "",
    phone: "02350000001",
    mobile: "0900000001",
    has_account: true,
    active: true,
    last_login_at: null,
    created_at: "2026-09-01T02:00:00Z",
    has_zalo: false,
    published: false,
    display_order: null,
    consent_recorded_at: null,
    ...ghiDe,
  };
}

const TRA_XONG = {
  pha: "xong",
  ten: new Map([["01J0000000000000000000BP01", "THƯỜNG TRỰC ĐẢNG UỶ"]]),
} as const;

function dung(danhSach: readonly identity_canBoTomTat[]): string {
  return renderToStaticMarkup(
    <BangLienHe danhSach={danhSach} traBoPhan={TRA_XONG} onSua={() => undefined} />,
  );
}

describe("bảng danh bạ — cái ra tới trang", () => {
  it("dựng đủ năm cột dữ liệu của một dòng", () => {
    const html = dung([canBo()]);

    expect(html).toContain("Nguyễn Văn A");
    expect(html).toContain("nva@demo.invalid");
    expect(html).toContain("Bí thư Đảng ủy");
    expect(html).toContain("THƯỜNG TRỰC ĐẢNG UỶ");
    expect(html).toContain("02350000001");
  });

  it("số di động cá nhân hiện ĐẦY ĐỦ ở màn nội bộ — câu mở #11", () => {
    // #11 chốt 22/09/2026: KHÔNG che trong nội bộ xã. Cán bộ cùng xã cần gọi nhau để làm việc; che
    // thì họ truyền số qua kênh riêng và hệ thống mất cả vết lẫn quyền kiểm soát. Quyết định ấy
    // CHỈ nói về màn hình nội bộ — bản xuất Excel và mọi đường ra ngoài cơ quan vẫn che.
    const html = dung([canBo()]);

    expect(html).toContain("0900000001");
    // Nếu có ngày ai đó "che cho an toàn", ca này đỏ và buộc người sửa đọc lại #11 trước.
    expect(html).not.toContain("****");
    expect(html).not.toContain("•••");
  });

  it("hai cột số, hai nhãn nói rõ loại — câu mở #16", () => {
    // Máy bàn cơ quan là thông tin công vụ; di động cá nhân là dữ liệu cá nhân theo Nghị định 13.
    // Gộp về một cột `Di động` như đặc tả §4 là đặt hai địa vị pháp lý dưới một cái tên, và mọi
    // luật che / xuất / công khai về sau áp sai mức cho một trong hai.
    const html = dung([canBo()]);

    expect(html).toContain("Máy bàn cơ quan");
    expect(html).toContain("Di động cá nhân");
  });

  it("KHÔNG có ô chọn dòng, KHÔNG có chip Mini App, KHÔNG có nút xoá", () => {
    // Ba thứ đặc tả vẽ (§2, §4) mà ba lý do khác nhau chặn: ô chọn chỉ phục vụ thao tác bật hàng
    // loạt mà #12 đã bỏ; chip Mini App không có trường nào trong hợp đồng; nút xoá không có tuyến
    // và cần một khoá quyền bảng `quyen` chưa có (#10, phát hiện cho #27).
    const html = dung([canBo()]);

    expect(html).not.toContain('type="checkbox"');
    expect(html).not.toContain("Mini App");
    expect(html).not.toContain("Xoá khỏi danh bạ");
    expect(html).not.toContain("Khoá tài khoản");
  });

  it("nút sửa mang TÊN NGƯỜI trong aria-label, không chỉ một nhãn chung", () => {
    // Hai mươi dòng cho ra hai mươi nút đọc lên giống hệt nhau là danh sách mà người dùng trình
    // đọc màn hình không chọn đúng được dòng nào — và chọn nhầm dòng ở đây là sửa hồ sơ của một
    // cán bộ khác. Chuỗi `15-phu-luc §8` yêu cầu giữ vẫn nằm nguyên trong nhãn.
    const html = dung([canBo(), canBo({ id: "b", full_name: "Trần Thị B" })]);

    expect(html).toContain(`aria-label="${NUT_SUA_THONG_TIN}: Nguyễn Văn A"`);
    expect(html).toContain(`aria-label="${NUT_SUA_THONG_TIN}: Trần Thị B"`);
  });

  it("bộ phận không tra được thì NÓI RA, không để ô trống", () => {
    // Một ô trống trông y hệt "chưa phân bộ phận", mà hai thứ ấy cần hai hành động khác nhau: một
    // là việc phân công nhân sự, một là một dòng dữ liệu lệch chỉ người quản trị sửa được.
    const html = dung([canBo({ department_id: "01J0000000000000000000XXXX" })]);

    expect(html).toContain("Không tra được trong danh mục");
  });

  it("chưa phân bộ phận là một câu riêng, không lẫn với ca trên", () => {
    expect(dung([canBo({ department_id: "" })])).toContain("Chưa phân bộ phận");
  });
});

describe("bảng danh bạ — CA BỊ TỪ CHỐI vì thiếu `admin.user`", () => {
  /**
   * Nhánh này không ai nhìn thấy trong lúc dựng: tài khoản người viết luôn có đủ quyền. Nó là
   * nhánh sẽ chạy trên máy của một cán bộ chuyên môn, và là nhánh phải đúng — vì thứ nó giữ lại
   * là số di động cá nhân của toàn bộ cán bộ trong xã.
   */
  function dungCong(quyetDinh: Parameters<typeof KhungQuyen>[0]["quyetDinh"]): string {
    return renderToStaticMarkup(
      <KhungQuyen quyetDinh={quyetDinh} cauThieuQuyen={CAU_THIEU_QUYEN}>
        <BangLienHe danhSach={[canBo()]} traBoPhan={TRA_XONG} onSua={() => undefined} />
      </KhungQuyen>,
    );
  }

  it("thiếu quyền: KHÔNG một dòng danh bạ nào ra tới trang", () => {
    const html = dungCong({ hien: false, vi: "khong-du-quyen" });

    expect(html).not.toContain("Nguyễn Văn A");
    expect(html).not.toContain("0900000001");
    expect(html).not.toContain("nva@demo.invalid");
    expect(html).toContain(CAU_THIEU_QUYEN);
  });

  it("không đọc được quyền: cũng không dựng gì — 'chưa rõ' hành xử như 'không có'", () => {
    // Fail closed. Trên đường cách ly không có giá trị mặc định nào (luật 1, cấm #1).
    const html = dungCong({
      hien: false,
      vi: "khong-doc-duoc",
      thongBao: "Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.",
    });

    expect(html).not.toContain("0900000001");
    expect(html).toContain("Phiên làm việc đã hết hạn. Vui lòng đăng nhập lại.");
  });

  it("chưa đọc xong quyền: chưa dựng, và chưa nói là thiếu quyền", () => {
    const html = dungCong(null);

    expect(html).not.toContain("0900000001");
    expect(html).not.toContain(CAU_THIEU_QUYEN);
  });

  it("đủ quyền: danh bạ ra tới trang", () => {
    expect(dungCong({ hien: true })).toContain("0900000001");
  });
});
