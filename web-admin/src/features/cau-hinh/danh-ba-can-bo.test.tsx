import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { LOI_KHONG_RO } from "@/lib/api/goi";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BangCanBo, type ThaoTacDong } from "./danh-ba-can-bo";
import {
  CAU_CHI_HIEN_MOT_LAN,
  NUT_CAP_TAI_KHOAN,
  NUT_DAT_LAI_MAT_KHAU,
  NUT_DA_GHI_LAI,
  OMatKhauTam,
  XacNhanTaiKhoan,
  cauKhongRoKetQua,
} from "./mat-khau-tam";
import type { BangTraDanhMuc } from "./tra-danh-muc";

/**
 * BA ĐIỀU TỆP NÀY CANH, và cả ba đều là những thứ một lần sửa MỘT DÒNG phá được mà không phép
 * kiểm nào khác thấy (điều thứ ba ở cuối tệp, cùng với lý do của nó):
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
  capTaiKhoan: () => {},
  datLaiMatKhau: () => {},
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
  has_zalo: false,
  published: false,
  display_order: null,
  consent_recorded_at: null,
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

/**
 * ĐIỀU THỨ BA: PHẦN QUẢN TRỊ VIÊN CỦA THÔNG TIN ĐĂNG NHẬP — `14-cau-hinh §3`.
 *
 * Hai tuyến, hai nút LOẠI TRỪ NHAU theo `has_account`, và một ô hiện mật khẩu tạm đúng một lần.
 * Mỗi ca dưới đây canh một thứ mà một dòng sửa "cho tiện" phá được:
 *
 *   · Gộp hai nút thành một nút đổi nhãn, hoặc hiện cả hai và làm mờ cái không dùng được. Máy chủ
 *     tách hai việc bằng hai tuyến và hai điều kiện loại trừ nhau trong mệnh đề WHERE, nên không
 *     nút nào làm được việc của nút kia. VẾ PHỦ ĐỊNH là vế chịu lực ở đây: một lần đọc nhầm cờ chỉ
 *     lộ ra ở chỗ nút KHÔNG được có mặt.
 *   · Che mật khẩu tạm bằng `type="password"` hay dấu sao — che một giá trị mà mục đích duy nhất
 *     của nó là được đọc to cho người khác.
 *   · Bỏ mất câu "chỉ hiện một lần". Không có câu ấy, quản trị viên đánh mất giá trị sẽ đi tìm một
 *     nút "xem lại" không tồn tại, và không ai nói cho họ biết rằng bấm Đặt lại sinh giá trị KHÁC.
 */

/** Giá trị GIẢ, và trông rõ là giả: không một mật khẩu thật nào được viết vào kho này (luật 8). */
const MAT_KHAU_GIA = "mat-khau-gia-de-kiem-tra";

describe("hai nút thông tin đăng nhập loại trừ nhau theo `has_account`", () => {
  it("chưa có tài khoản → có Cấp tài khoản, KHÔNG có Đặt lại mật khẩu", () => {
    const html = ve([{ ...CAN_BO, has_account: false }]);

    expect(html).toContain(NUT_CAP_TAI_KHOAN);
    expect(html).toContain(`aria-label="${NUT_CAP_TAI_KHOAN}: Huỳnh Văn A"`);

    // VẾ CHỊU LỰC. Đặt lại mật khẩu lên một người chưa có tài khoản là một nút gọi vào tuyến chắc
    // chắn từ chối — và người quản trị bấm nó sẽ kết luận hệ thống hỏng.
    expect(html).not.toContain(NUT_DAT_LAI_MAT_KHAU);
  });

  it("đã có tài khoản → có Đặt lại mật khẩu, KHÔNG có Cấp tài khoản", () => {
    const html = ve([{ ...CAN_BO, has_account: true }]);

    expect(html).toContain(NUT_DAT_LAI_MAT_KHAU);
    expect(html).toContain(`aria-label="${NUT_DAT_LAI_MAT_KHAU}: Huỳnh Văn A"`);

    // VẾ CHỊU LỰC. `POST /staff/{id}/account` mang `AND NOT co_tai_khoan`, nên nút này trên một
    // dòng đã có tài khoản chỉ dẫn tới 409 — và việc thật sự cần làm lúc ấy là ĐẶT LẠI.
    expect(html).not.toContain(NUT_CAP_TAI_KHOAN);
  });

  it("KHÔNG BAO GIỜ hiện cả hai, kể cả dưới dạng một nút bị làm mờ", () => {
    // Một nút `disabled` là hình dạng "cả hai cùng có mặt" mà một phép kiểm chỉ đếm chữ sẽ bỏ lọt.
    // Nó mời người dùng hỏi "vì sao không bấm được" và đi tìm một quyền họ không hề thiếu; nhiều
    // trình đọc màn hình còn bỏ qua hẳn nút bị vô hiệu, nên họ không biết là có gì ở đó.
    for (const coTaiKhoan of [true, false]) {
      const html = ve([{ ...CAN_BO, has_account: coTaiKhoan }]);
      const soNut =
        Number(html.includes(NUT_CAP_TAI_KHOAN)) + Number(html.includes(NUT_DAT_LAI_MAT_KHAU));

      expect(soNut).toBe(1);
      expect(html).not.toContain("disabled");
    }
  });
});

describe("ô mật khẩu tạm — hiện một lần, đọc được, đóng bằng tay", () => {
  function veO() {
    return renderToStaticMarkup(
      <OMatKhauTam
        matKhauTam={{
          kieu: "cap",
          maCanBo: "CB001",
          hoTen: "Huỳnh Văn A",
          matKhau: MAT_KHAU_GIA,
        }}
        onDong={() => {}}
      />,
    );
  }

  it("hiện giá trị dạng chữ đọc được, KHÔNG che", () => {
    const html = veO();

    expect(html).toContain(MAT_KHAU_GIA);
    // Là nội dung văn bản của một phần tử, không phải giá trị của một ô nhập: `type="password"`
    // biến một giá trị sinh ra để đọc to thành một hàng chấm, và `<input>` còn kéo theo khả năng
    // bị trình duyệt nhớ vào bộ nhớ điền tự động.
    expect(html).toContain(`>${MAT_KHAU_GIA}<`);
    expect(html).not.toContain('type="password"');
    expect(html).not.toContain("****");
    expect(html).not.toContain("<input");
  });

  it("nói rõ CHỈ HIỆN MỘT LẦN, và nói ra rằng đặt lại sinh giá trị khác", () => {
    const html = veO();

    expect(html).toContain(CAU_CHI_HIEN_MOT_LAN);
    expect(CAU_CHI_HIEN_MOT_LAN).toMatch(/CHỈ HIỆN MỘT LẦN/);
    expect(CAU_CHI_HIEN_MOT_LAN).toMatch(/KHÁC/);
  });

  it("giá trị KHÔNG nằm trong một thuộc tính nào", () => {
    // Luật 3, cấm #4: `aria-label`, `title`, URL và tên tệp đều là chỗ giá trị này bị mang đi —
    // vào cây trợ năng, vào nhật ký của trình duyệt, vào ảnh chụp màn hình gửi cho hỗ trợ.
    const html = veO();

    expect(html).not.toContain(`aria-label="${MAT_KHAU_GIA}`);
    expect(html).not.toContain(`title="${MAT_KHAU_GIA}`);
    expect(html).not.toContain(`value="${MAT_KHAU_GIA}`);
  });

  it("đóng bằng một hành động rõ ràng, không có đếm ngược", () => {
    const html = veO();

    expect(html).toContain(NUT_DA_GHI_LAI);
    // "Tôi đã ghi lại" là một lời khẳng định; "Đóng" là một phản xạ dọn màn hình. Sự khác nhau ấy
    // đúng bằng nửa giây cần thiết trước khi một giá trị không lấy lại được biến mất.
    expect(NUT_DA_GHI_LAI).not.toMatch(/^(Đóng|OK)$/);
  });
});

describe("lỗi của máy chủ ra nguyên văn, và ca im lặng có câu riêng", () => {
  function veXacNhan(loiMayChu: string) {
    return renderToStaticMarkup(
      <XacNhanTaiKhoan
        dangMo={{ kieu: "datLai", canBo: CAN_BO, khoaChongTrung: "khoa-gia-cua-bai-kiem" }}
        loiMayChu={loiMayChu}
        dangGui={false}
        onGui={() => {}}
        onHuy={() => {}}
      />,
    );
  }

  it("câu tiếng Việt của máy chủ ra nguyên văn, không thêm không bớt", () => {
    // Câu 409 thật của tuyến cấp tài khoản là một câu máy chủ viết sẵn; việc của màn hình chỉ là
    // đưa nó ra trang. Không rẽ nhánh theo `code`, không hiện `trace_id`, không hiện số hiệu HTTP.
    const cauCuaMayChu = "Cán bộ này đã có tài khoản đăng nhập.";
    const html = veXacNhan(cauCuaMayChu);

    expect(html).toContain(cauCuaMayChu);
    expect(html).not.toContain("409");
    expect(html).not.toContain("trace");
  });

  it("máy chủ im lặng thì KHÔNG nói `thất bại` cụt lủn — mời kiểm tra lại và đặt lại", () => {
    // `LOI_KHONG_RO` phủ cả hai đường: mạng đứt TRƯỚC khi yêu cầu tới nơi, và mạng đứt SAU khi máy
    // chủ đã ghi. Ở đường thứ hai thì mật khẩu tạm đã sinh ra và vừa mất vĩnh viễn, nên một câu
    // "Thất bại, vui lòng thử lại" là câu sai ở đúng nửa nguy hiểm.
    const html = veXacNhan(LOI_KHONG_RO);

    expect(html).toContain(LOI_KHONG_RO);
    expect(html).toContain(cauKhongRoKetQua("datLai"));
    // Câu ấy phải MỜI ĐẶT LẠI, không được dừng ở "thất bại": mật khẩu của lần vừa rồi có thể đã
    // sinh ra và đã mất, và lần đặt lại sinh một giá trị KHÁC — người đọc phải biết cả hai vế.
    expect(cauKhongRoKetQua("datLai")).toMatch(/Đặt lại mật khẩu/);
    expect(cauKhongRoKetQua("datLai")).toMatch(/KHÁC/);
    expect(cauKhongRoKetQua("cap")).toMatch(/KHÁC/);
  });

  it("câu bổ sung ấy KHÔNG xuất hiện khi máy chủ đã trả lời rõ ràng", () => {
    // Vế phủ định: dán câu "chưa biết đã ghi hay chưa" vào một lần từ chối 403 là nói với quản trị
    // viên rằng có thể đã có gì đó xảy ra — trong khi máy chủ vừa nói rõ là không.
    const html = veXacNhan("Tài khoản của bạn không có quyền quản lý người dùng.");

    expect(html).not.toContain(cauKhongRoKetQua("datLai"));
  });
});
