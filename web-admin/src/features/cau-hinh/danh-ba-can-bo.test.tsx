import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BangCanBo } from "./danh-ba-can-bo";
import type { BangTraDanhMuc } from "./tra-danh-muc";

/**
 * HAI CỘT ĐIỆN THOẠI PHẢI Ở RIÊNG — và đây là ca duy nhất chặn việc gộp chúng lại.
 *
 * Câu mở #16, khách chốt 22/09/2026: máy bàn cơ quan là THÔNG TIN CÔNG VỤ, di động cá nhân là
 * DỮ LIỆU CÁ NHÂN theo Nghị định 13. Hai địa vị pháp lý khác nhau nghĩa là hai luật che, hai luật
 * xuất Excel, hai luật công khai ra Mini App.
 *
 * Gộp lại là một lần sửa MỘT DÒNG, và trước ca này không phép kiểm nào thấy được. Hệ quả của lần
 * gộp ấy không phải giao diện xấu: mọi quy tắc về sau buộc áp CHUNG một mức cho hai loại dữ liệu,
 * mà mức an toàn phải lấy theo loại nhạy hơn — nên hoặc số máy bàn của cơ quan bị che vô cớ, hoặc
 * số di động cá nhân đi theo bản xuất ra ngoài.
 */

const TRA_RONG: BangTraDanhMuc = { pha: "xong", ten: new Map() };

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
      moChiTiet={() => {}}
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
