import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { FormDoiMatKhau } from "./form-doi-mat-khau";
import { KhungNhacBatDoi } from "./nhac-bat-doi";

/**
 * Điều tệp này canh: **màn hình có thật sự đưa được ba ô và câu cảnh báo ra trang hay không**.
 *
 * VÌ SAO CẦN, KHI ĐÃ CÓ `doi-mat-khau.test.ts`: tệp kia canh QUYẾT ĐỊNH, và một quyết định đúng
 * trong một thành phần không bao giờ vẽ ô nào là một quyết định không ai chạm tới. Kho này đã đo
 * đúng lỗ ấy một lần — xoá trắng câu quan trọng nhất của màn Danh mục để lại 167 ca xanh
 * (`vitest.config.ts`). `react-dom/server` kết xuất ra chuỗi trong Node, và một chuỗi là đủ để
 * hỏi "câu ấy có trên trang không".
 *
 * Không có sự kiện, không có tiêu điểm, không có bố cục ở đây: môi trường kiểm là Node, không
 * phải trình duyệt.
 */

describe("biểu mẫu tự đổi mật khẩu", () => {
  it("có ĐỦ BA ô, và cả ba là ô mật khẩu", () => {
    const html = renderToStaticMarkup(<FormDoiMatKhau />);

    expect(html).toContain("Mật khẩu hiện tại");
    expect(html).toContain("Mật khẩu mới");
    expect(html).toContain("Nhập lại mật khẩu mới");

    // Ba ô, ba lần `type="password"` — không hai, không bốn. Một ô để lộ chữ đang gõ ở trạng
    // thái mặc định là mật khẩu cán bộ hiện giữa phòng làm việc.
    expect(html.split('type="password"').length - 1).toBe(3);
  });

  it("chỉ HAI ô mang tên trường của hợp đồng — ô nhập lại không mang tên nào gửi đi được", () => {
    const html = renderToStaticMarkup(<FormDoiMatKhau />);

    expect(html).toContain('name="current_password"');
    expect(html).toContain('name="new_password"');

    // Ô thứ ba cố ý KHÔNG mang tên của hợp đồng. Đặt cho nó một tên như `new_password_confirm`
    // là mời người sau gom biểu mẫu bằng `FormData` và gửi thẳng cả ba lên máy chủ.
    expect(html).not.toContain("confirm");
    expect(html).toContain('name="xac-nhan-mat-khau-moi"');
  });

  it("có nút hiện/ẩn mật khẩu, bằng CHỮ chứ không bằng biểu tượng", () => {
    const html = renderToStaticMarkup(<FormDoiMatKhau />);

    // Cán bộ lớn tuổi gõ sai nhiều hơn hẳn khi không nhìn thấy chữ mình gõ
    // (`skills/accessibility-elderly`). Một con mắt gạch chéo là thứ phải đoán nghĩa.
    expect(html).toContain("Hiện mật khẩu");
    expect(html).toContain('aria-pressed="false"');
  });

  it("NÓI TRƯỚC rằng đổi xong thì mọi phiên bị kết thúc và phải đăng nhập lại", () => {
    const html = renderToStaticMarkup(<FormDoiMatKhau />);

    // Không nói trước thì lần bị đá ra màn đăng nhập đọc y hệt một lần hệ thống hỏng — ngay sau
    // khi người dùng vừa làm một việc đúng.
    expect(html).toContain("kết thúc tất cả phiên đang mở");
    expect(html).toContain("đăng nhập lại");
  });

  it("có một vùng thông báo sẵn trong DOM, kể cả khi chưa có gì để báo", () => {
    const html = renderToStaticMarkup(<FormDoiMatKhau />);

    // Thêm/bớt một phần tử làm cả biểu mẫu nhảy chỗ, và vùng aria-live thêm vào sau không phải
    // trình đọc màn hình nào cũng đọc.
    expect(html).toContain('role="alert"');
    expect(html).toContain('aria-live="assertive"');
  });
});

describe("màn hình khi vào bằng đường BẮT ĐỔI lần đầu", () => {
  it("hiện lời nhắc, và ô MẬT KHẨU HIỆN TẠI VẪN CÒN NGUYÊN", () => {
    // ĐÂY LÀ CA TỒN TẠI ĐỂ ĐỎ khi ai đó bỏ ô "mật khẩu hiện tại" đi cho gọn ở màn bắt đổi. Đề
    // xuất ấy sẽ tới, vì ở đây ô ấy trông như một lần gõ thừa — người dùng vừa đăng nhập xong.
    // Nhưng máy ở bộ phận một cửa là máy DÙNG CHUNG (câu mở #18): ô ấy là thứ chặn một trình
    // duyệt bỏ quên thành một lần chiếm tài khoản vĩnh viễn.
    const html = renderToStaticMarkup(
      <>
        <KhungNhacBatDoi batDoi={true} />
        <FormDoiMatKhau />
      </>,
    );

    expect(html).toContain("mật khẩu tạm");
    expect(html).toContain("Mật khẩu hiện tại");
    expect(html).toContain('name="current_password"');
  });

  it("KHÔNG hiện lời nhắc cho người tự vào đổi", () => {
    const html = renderToStaticMarkup(<KhungNhacBatDoi batDoi={false} />);

    expect(html).toBe("");
  });
});
