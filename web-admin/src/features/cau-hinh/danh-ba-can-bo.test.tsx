import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { LOI_KHONG_RO } from "@/lib/api/goi";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BangCanBo, staffRowMenuItems, type ThaoTacDong } from "./danh-ba-can-bo";
import {
  CAU_CHI_HIEN_MOT_LAN,
  NUT_CAP_TAI_KHOAN,
  NUT_DAT_LAI_MAT_KHAU,
  NUT_DA_GHI_LAI,
  OMatKhauTam,
  XacNhanTaiKhoan,
  cauKhongRoKetQua,
} from "./mat-khau-tam";
import { NO_EMAIL_ACCOUNT_REASON, NO_EMAIL_MENU_HINT, NO_EMAIL_SUBLINE } from "./nhan-can-bo";
import type { BangTraDanhMuc } from "./tra-danh-muc";

/**
 * BA ĐIỀU TỆP NÀY CANH, và cả ba đều là những thứ một lần sửa MỘT DÒNG phá được mà không phép
 * kiểm nào khác thấy (điều thứ ba ở cuối tệp, cùng với lý do của nó):
 *
 * 1. MỘT CỘT "ĐIỆN THOẠI", NHƯNG HAI LOẠI SỐ VẪN KHÔNG LẪN — người dùng chốt 09/10/2026 bảng theo
 *    prototype: ô hiện di động, không có thì máy bàn. Câu mở #16 (khách chốt 22/09/2026) vẫn giữ ở
 *    chỗ khác: hai ô nhập riêng trong hộp thoại, giá trị NGUYÊN VĂN máy chủ trả (che hay không là
 *    việc của máy chủ), và `title` của ô nói đó là loại số nào. Không nối hai số vào một ô.
 *
 * 2. NÚT XOÁ CHỈ ĐI SAU QUYỀN RIÊNG — câu mở #10 tách khoá khỏi xoá; xoá mềm một dòng nhập trùng mang
 *    `admin.user.delete` (ADR 0035). Chủ dự án chốt 08/10/2026 đưa `Trash2` của prototype về màn này,
 *    sau đúng khoá ấy; phần kiểm nó nằm ở `user-list-prototype.test.tsx`. Ở đây chỉ canh: không có
 *    khoá thì không có nút.
 */

const TRA_RONG: BangTraDanhMuc = { pha: "xong", ten: new Map() };

const KHONG_LAM_GI: ThaoTacDong = {
  sua: () => {},
  datKhoa: () => {},
  capTaiKhoan: () => {},
  datLaiMatKhau: () => {},
  xoa: () => {},
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
      thaoTac={KHONG_LAM_GI}
      traBoPhan={TRA_RONG}
    />,
  );
}

describe("cột Điện thoại — di động, không có thì máy bàn, nguyên văn máy chủ (người dùng 09/10/2026)", () => {
  it("một đầu cột 'Điện thoại'; hai cột cũ không còn", () => {
    const html = ve();
    expect(html).toContain(">Điện thoại<");
    expect(html).not.toContain(">Máy bàn cơ quan<");
    expect(html).not.toContain(">Di động cá nhân<");
  });

  it("có di động → hiện di động, và `title` nói đó là di động cá nhân; KHÔNG nối thêm máy bàn", () => {
    const html = ve();
    expect(html).toContain(`<td title="Di động cá nhân">${DI_DONG}</td>`);
    expect(html).not.toContain(MAY_BAN);
    expect(html).not.toContain(`${MAY_BAN}, ${DI_DONG}`);
    expect(html).not.toContain(`${DI_DONG} · ${MAY_BAN}`);
  });

  it("không có di động → hiện máy bàn, `title` nói đó là máy bàn cơ quan", () => {
    const html = ve([{ ...CAN_BO, mobile: "" }]);
    expect(html).toContain(`<td title="Máy bàn cơ quan">${MAY_BAN}</td>`);
  });

  it("không có số nào → '—', không `title`", () => {
    expect(ve([{ ...CAN_BO, mobile: "", phone: "  " }])).toContain("<td>—</td>");
  });

  it("giá trị đi ra ĐÚNG như máy chủ gửi — màn hình không che thêm, không gỡ che", () => {
    // #11: máy chủ trả số nguyên vẹn trong nội bộ xã; một chuỗi đã che (nếu máy chủ che) cũng đi ra
    // nguyên văn. Màn hình không có quy tắc che nào của riêng nó.
    expect(ve()).not.toContain("****");
    const masked = "090****000";
    expect(ve([{ ...CAN_BO, mobile: masked }])).toContain(`>${masked}</td>`);
  });
});

describe("cụm nút của một dòng — Sửa · ⋯ · Xoá (người dùng 09/10/2026)", () => {
  it("mỗi dòng có Sửa tài khoản; KHÔNG còn nút Đổi vai trò (vai trò sửa trong hộp thoại Sửa)", () => {
    const html = ve();
    expect(html).toContain("Sửa tài khoản");
    expect(html).not.toContain("Đổi vai trò");
  });

  it("không có khoá xoá → KHÔNG có nút Xoá dưới bất kỳ cách viết nào", () => {
    const html = ve();

    // BỐN CÁCH VIẾT, KHÔNG MỘT: một phép kiểm chỉ tìm đúng một chuỗi là phép kiểm né được bằng
    // cách đổi nhãn.
    expect(html).not.toContain("🗑");
    expect(html).not.toMatch(/Xo[áa] (kh[ỏo]i danh b[ạa]|t[àa]i kho[ảa]n)/i);
    expect(html).not.toMatch(/>\s*Xoá\s*</);
    expect(html).not.toContain("nut-xoa");
  });

  it("nhãn mục khoá trong menu ⋯ đi theo `active`, không theo một cờ riêng", () => {
    // `active` là `dang_hoat_dong` của máy chủ và cũng là thứ tuyến lockout ghi vào, nên nhãn không
    // thể lệch với việc mục ấy sắp làm. Đọc nhầm sang `has_account` là mời người dùng bấm "Mở khoá"
    // lên một tài khoản đang chạy bình thường.
    expect(staffRowMenuItems({ ...CAN_BO, active: true }, KHONG_LAM_GI)[0]).toMatchObject({ label: "Khoá tài khoản" });
    expect(staffRowMenuItems({ ...CAN_BO, active: false }, KHONG_LAM_GI)[0]).toMatchObject({ label: "Mở khoá tài khoản" });
  });

  it("mục khoá gọi ĐÚNG hành động của màn hình với CẢ DÒNG", () => {
    const datKhoa = vi.fn();
    const item = staffRowMenuItems(CAN_BO, { ...KHONG_LAM_GI, datKhoa })[0];
    if (item?.kind !== "item") throw new Error("cần một mục");
    item.onSelect();
    expect(datKhoa).toHaveBeenCalledWith(CAN_BO);
  });

  it("prototype row: icon-only actions, the action in `title`, the person in `aria-label`; locked = `Tạm khoá`", () => {
    const html = ve([{ ...CAN_BO, active: false }]);
    expect(html).toContain('title="Sửa tài khoản"');
    expect(html).toContain('aria-label="Thao tác khác: Huỳnh Văn A"');
    // No visible action WORDS in the row: the names live in the attributes, so the row stays one line.
    expect(html).not.toContain(">Sửa tài khoản<");
    expect(html).toContain(">Tạm khoá<");
  });

  it("mỗi nút mang tên người trong nhãn trợ năng", () => {
    // Hai mươi dòng cho ra hai mươi nút đọc lên giống hệt nhau là danh sách mà người dùng trình
    // đọc màn hình không chọn đúng được dòng nào — và chọn nhầm dòng ở đây là khoá nhầm tài khoản.
    const html = ve();

    expect(html).toContain('aria-label="Sửa tài khoản: Huỳnh Văn A"');
    expect(html).toContain('aria-label="Thao tác khác: Huỳnh Văn A"');
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

/** Labels of the row's `⋯` menu, in order. */
function menuLabels(cb: identity_canBoTomTat): string[] {
  return staffRowMenuItems(cb, KHONG_LAM_GI).map((i) => (i.kind === "item" ? i.label : "—"));
}

describe("hai mục thông tin đăng nhập trong menu ⋯ loại trừ nhau theo `has_account`", () => {
  it("chưa có tài khoản → có Cấp tài khoản, KHÔNG có Đặt lại mật khẩu", () => {
    const labels = menuLabels({ ...CAN_BO, has_account: false });

    expect(labels).toContain(NUT_CAP_TAI_KHOAN);
    // VẾ CHỊU LỰC. Đặt lại mật khẩu lên một người chưa có tài khoản là một mục gọi vào tuyến chắc
    // chắn từ chối — và người quản trị bấm nó sẽ kết luận hệ thống hỏng.
    expect(labels).not.toContain(NUT_DAT_LAI_MAT_KHAU);
  });

  it("đã có tài khoản → có Đặt lại mật khẩu, KHÔNG có Cấp tài khoản", () => {
    const labels = menuLabels({ ...CAN_BO, has_account: true });

    expect(labels).toContain(NUT_DAT_LAI_MAT_KHAU);
    // VẾ CHỊU LỰC. `POST /staff/{id}/account` mang `AND NOT co_tai_khoan`, nên mục này trên một
    // dòng đã có tài khoản chỉ dẫn tới 409 — và việc thật sự cần làm lúc ấy là ĐẶT LẠI.
    expect(labels).not.toContain(NUT_CAP_TAI_KHOAN);
  });

  it("KHÔNG BAO GIỜ có cả hai; có thư điện tử thì không mục nào bị làm mờ", () => {
    for (const coTaiKhoan of [true, false]) {
      const items = staffRowMenuItems({ ...CAN_BO, has_account: coTaiKhoan }, KHONG_LAM_GI);
      const labels = items.map((i) => (i.kind === "item" ? i.label : ""));
      expect(Number(labels.includes(NUT_CAP_TAI_KHOAN)) + Number(labels.includes(NUT_DAT_LAI_MAT_KHAU))).toBe(1);
      expect(items.some((i) => i.kind === "item" && i.disabled === true)).toBe(false);
    }
  });

  it("the table itself carries no disabled control for a row with an email and no account", () => {
    const html = ve([{ ...CAN_BO, has_account: false }]);
    const body = /<tbody[^>]*>([\s\S]*)<\/tbody>/.exec(html)?.[1] ?? "";
    expect(body).not.toContain('disabled=""');
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

describe("staff without an email — optional since 4cf87b6", () => {
  const NO_EMAIL: identity_canBoTomTat = { ...CAN_BO, email: "", has_account: false };

  /** The "Cấp tài khoản" item of the row's `⋯` menu. */
  function issueItem(cb: identity_canBoTomTat) {
    const item = staffRowMenuItems(cb, KHONG_LAM_GI).find((i) => i.kind === "item" && i.label === NUT_CAP_TAI_KHOAN);
    if (item === undefined || item.kind !== "item") throw new Error("không thấy mục Cấp tài khoản");
    return item;
  }

  it("the list says 'Chưa có thư điện tử' under the name, never an empty line", () => {
    const html = ve([NO_EMAIL]);
    expect(html).toContain(`<div class="text-ink-muted text-[11.5px]">${NO_EMAIL_SUBLINE}</div>`);
    expect(html).not.toContain('<div class="text-ink-muted text-[11.5px]"></div>');
  });

  it("DENIED: 'Cấp tài khoản' is in the menu but disabled, its reason as the item's second line", () => {
    const item = issueItem(NO_EMAIL);
    expect(item.disabled).toBe(true);
    expect(item.hint).toBe(NO_EMAIL_MENU_HINT);
  });

  it("the account chip carries the long reason in its title and in hidden text", () => {
    const html = ve([NO_EMAIL]);
    expect(html).toContain(`title="${NO_EMAIL_ACCOUNT_REASON}"`);
    expect(html).toContain(`id="account-reason-${NO_EMAIL.id}" class="an-thi-giac">${NO_EMAIL_ACCOUNT_REASON}<`);
  });

  it("whitespace-only email counts as no email", () => {
    expect(issueItem({ ...NO_EMAIL, email: "   " }).disabled).toBe(true);
  });

  it("ALLOWED: with an email the item is enabled and the long no-email reason is nowhere", () => {
    const withEmail = { ...NO_EMAIL, email: "demo@thangbinh.test" };
    expect(issueItem(withEmail).disabled).toBeUndefined();
    expect(ve([withEmail])).not.toContain(NO_EMAIL_ACCOUNT_REASON);
  });

  it("the server's 409 staff_has_no_email sentence still reaches the page verbatim", () => {
    // The disabled button is UX. If the call is made anyway (stale row, another tab), the server
    // refuses and its sentence is what the administrator reads.
    const sentence =
      "Cán bộ này chưa có thư điện tử công vụ — đó là tên đăng nhập. Hãy thêm thư điện tử " +
      "trong hồ sơ trước khi cấp tài khoản.";
    const html = renderToStaticMarkup(
      <XacNhanTaiKhoan
        dangMo={{ kieu: "cap", canBo: NO_EMAIL }}
        loiMayChu={sentence}
        dangGui={false}
        onGui={() => {}}
        onHuy={() => {}}
      />,
    );
    expect(html).toContain(sentence);
    expect(html).not.toContain("staff_has_no_email");
  });
});
