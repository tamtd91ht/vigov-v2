import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BangCanBo, type ThaoTacDong } from "./danh-ba-can-bo";
import type { BangTraDanhMuc } from "./tra-danh-muc";

/**
 * HAI ĐIỀU TỆP NÀY CANH, và cả hai đều là những thứ một lần sửa MỘT DÒNG phá được mà không phép
 * kiểm nào khác thấy:
 *
 * 1. HAI CỘT ĐIỆN THOẠI PHẢI Ở RIÊNG — câu mở #16, khách chốt 22/09/2026. Máy bàn cơ quan là
 *    THÔNG TIN CÔNG VỤ, di động cá nhân là DỮ LIỆU CÁ NHÂN theo Nghị định 13. Hai địa vị pháp lý
 *    khác nhau nghĩa là hai luật che, hai luật xuất Excel, hai luật công khai ra Mini App. Hệ quả
 *    của một lần gộp không phải giao diện xấu: mọi quy tắc về sau buộc áp CHUNG một mức cho hai
 *    loại dữ liệu, mà mức an toàn phải lấy theo loại nhạy hơn — nên hoặc số máy bàn của cơ quan
 *    bị che vô cớ, hoặc số di động cá nhân đi theo bản xuất ra ngoài.
 *
 * 2. KHÔNG CÓ NÚT XOÁ Ở BẤT KỲ DÒNG NÀO — câu mở #10, cùng ngày. Xoá mềm một cán bộ là thao tác
 *    RIÊNG mang QUYỀN RIÊNG, và bảng `quyen` chưa có khoá nào mang nghĩa ấy (phát hiện cho câu mở
 *    #27). Đặc tả thì vẽ nút ấy (`docs/ui-ux/12-danh-ba-can-bo.md:61`), nên áp lực thêm lại nó là
 *    có thật và đến từ một tài liệu trông có thẩm quyền. Một nút gọi vào tuyến không tồn tại là
 *    lời hứa suông: người quản trị bấm, nhận một lỗi, và kết luận hệ thống hỏng — trong khi thứ
 *    đang thiếu là một quyết định của khách.
 */

const TRA_RONG: BangTraDanhMuc = { pha: "xong", ten: new Map() };

const KHONG_LAM_GI: ThaoTacDong = {
  chiTiet: () => {},
  sua: () => {},
  doiVaiTro: () => {},
  datKhoa: () => {},
};

/**
 * HAI ĐẦU SỐ KHÁC HẲN NHAU, có chủ ý: nếu hai giá trị giống nhau thì một lần vẽ nhầm cột vẫn cho
 * ra HTML y hệt, và ca kiểm xanh mà không chứng minh gì. Cả hai là số giả đã thoả thuận của kho
 * (luật 3, bất biến 5).
 */
const MAY_BAN = "02350000000";
const DI_DONG = "0900000000";

const CAN_BO: identity_canBoTomTat = {
  id: "01J000000000000000000001",
  code: "CB001",
  full_name: "Huỳnh Văn A",
  email: "demo@thangbinh.test",
  position: "Chuyên viên",
  department_id: "01J0000000000000000BOPHAN",
  role_id: "01J00000000000000000VAITRO",
  phone: MAY_BAN,
  mobile: DI_DONG,
  has_account: true,
  active: true,
  last_login_at: null,
  created_at: "2026-09-22T08:00:00Z",
};

function ve(danhSach: readonly identity_canBoTomTat[] = [CAN_BO]) {
  return renderToStaticMarkup(
    <BangCanBo
      danhSach={danhSach}
      khoa="code"
      chieu="asc"
      doiSapXep={() => {}}
      thaoTac={KHONG_LAM_GI}
      idDangMo={null}
      traBoPhan={TRA_RONG}
      traVaiTro={TRA_RONG}
    />,
  );
}

describe("danh bạ cán bộ giữ hai cột điện thoại riêng", () => {
  it("có HAI đầu cột, mỗi cột nói rõ loại số", () => {
    const html = ve();

    expect(html).toContain("Máy bàn cơ quan");
    expect(html).toContain("Di động cá nhân");

    // VẾ CHỊU LỰC: nhãn trung tính "Điện thoại" là đúng hình dạng cột gộp, và là thứ tệp này tồn
    // tại để chặn. Người sắp bấm nút xuất, hay sắp tick ô công khai, phải biết mình đang đụng loại
    // nào — một nhãn trung tính là lúc họ không biết.
    expect(html).not.toContain(">Điện thoại<");
  });

  it("hai số hiện ở HAI ô, không gộp và không nối chuỗi", () => {
    const html = ve();

    expect(html).toContain(`<td>${MAY_BAN}</td>`);
    expect(html).toContain(`<td>${DI_DONG}</td>`);

    // Ba cách gộp thường gặp, chặn cả ba: nối bằng dấu phẩy, nối bằng gạch, và `a || b`.
    expect(html).not.toContain(`${MAY_BAN}, ${DI_DONG}`);
    expect(html).not.toContain(`${MAY_BAN} · ${DI_DONG}`);
    expect(html).not.toContain(`${MAY_BAN} / ${DI_DONG}`);
  });

  it("số KHÔNG bị che trên màn nội bộ — #11", () => {
    const html = ve();

    // #11 chốt 22/09/2026: cán bộ cùng xã không bị che số của nhau, vì họ phải gọi nhau để làm
    // việc; che thì họ truyền số qua kênh riêng và cơ quan mất cả vết lẫn quyền kiểm soát. Máy chủ
    // trả số nguyên vẹn (`soRaManHinhNoiBo`), nên một dấu sao ở đây là giao diện tự che thêm.
    expect(html).not.toContain("****");
  });

  it("cán bộ KHÔNG có di động vẫn hiện được, và ô ấy để trống chứ không mượn số máy bàn", () => {
    // Cột `di_dong_ca_nhan` là `NOT NULL DEFAULT ''` (migration 0009) — "không có số" là chuỗi
    // rỗng, một trạng thái THẬT, không phải lỗi. Lấp nó bằng số máy bàn là ghép hai loại dữ liệu
    // vào một ô bằng đường khác.
    const html = ve([{ ...CAN_BO, mobile: "" }]);

    expect(html).toContain(`<td>${MAY_BAN}</td>`);
    expect(html).toContain("<td></td>");
  });
});

describe("cụm nút của một dòng — ba thao tác ghi, KHÔNG có Xoá", () => {
  it("mỗi dòng có Sửa hồ sơ và Đổi vai trò", () => {
    const html = ve();

    expect(html).toContain("Sửa hồ sơ");
    expect(html).toContain("Đổi vai trò");
    expect(html).toContain("Chi tiết");
  });

  it("KHÔNG có nút Xoá dưới bất kỳ cách viết nào", () => {
    const html = ve();

    // BỐN CÁCH VIẾT, KHÔNG MỘT. Ca này tồn tại để đỏ khi ai đó thêm lại nút của đặc tả, và đặc tả
    // viết nó là `🗑 Xoá khỏi danh bạ` — nên chặn cả biểu tượng lẫn chữ, cả chữ hoa lẫn chữ
    // thường. Một phép kiểm chỉ tìm đúng một chuỗi là phép kiểm né được bằng cách đổi nhãn.
    expect(html).not.toContain("🗑");
    expect(html).not.toMatch(/Xo[áa] kh[ỏo]i danh b[ạa]/i);
    expect(html).not.toMatch(/>\s*Xoá\s*</);
    expect(html).not.toContain("nut-xoa");
  });

  it("nhãn nút khoá đi theo `active`, không theo một cờ riêng", () => {
    // `active` là `dang_hoat_dong` của máy chủ và cũng là thứ tuyến lockout ghi vào, nên nhãn nút
    // không thể lệch với việc nút ấy sắp làm. Đọc nhầm sang `has_account` là mời người dùng bấm
    // "Mở khoá" lên một tài khoản đang chạy bình thường.
    expect(ve([{ ...CAN_BO, active: true }])).toContain("Khoá tài khoản");
    expect(ve([{ ...CAN_BO, active: false }])).toContain("Mở khoá tài khoản");
  });

  it("mỗi nút mang tên người trong nhãn trợ năng", () => {
    // Hai mươi dòng cho ra hai mươi nút đọc lên giống hệt nhau là danh sách mà người dùng trình
    // đọc màn hình không chọn đúng được dòng nào — và chọn nhầm dòng ở đây là khoá nhầm tài khoản.
    const html = ve();

    expect(html).toContain('aria-label="Sửa hồ sơ: Huỳnh Văn A"');
    expect(html).toContain('aria-label="Khoá tài khoản: Huỳnh Văn A"');
  });
});
