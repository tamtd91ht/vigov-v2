import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import { ThongTinPhieu } from "./tra-cuu-phieu";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không. Hai thứ quan trọng nhất trên màn này đều là
 * thứ mà mọi ca test module thuần vẫn xanh nguyên nếu ai đó xoá khỏi JSX: số điện thoại đã che,
 * và hai ô hạn xử lý.
 */

function phieu(sua: Partial<petitions_phieuPhanAnhRa> = {}): petitions_phieuPhanAnhRa {
  return {
    code: "PA-2026-0021",
    channel: "zalo-mini-app",
    status: "dang-phan-loai",
    field: "rac-thai",
    field_label: "Rác thải – Vệ sinh môi trường",
    content: "Rác tồn đọng ở đầu ngõ ba ngày chưa ai dọn.",
    address: "Tổ 6, thôn Hà Lam",
    // Dạng ĐÃ CHE của máy chủ, dựng từ số giả đã thống nhất (luật 3, bất biến 5).
    reporter_name: "Nguyễn V. A.",
    reporter_phone: "09****0000",
    anonymous: false,
    clock_from: "2026-09-09T07:20:00Z",
    booked_at: "2026-09-09T07:21:00Z",
    acknowledge_due: "2026-09-09T09:20:00Z",
    resolve_due: null,
    public: false,
    ...sua,
  };
}

const BAY_GIO = new Date("2026-09-09T10:00:00Z");

describe("phiếu phản ánh kết xuất ra trang", () => {
  it("số điện thoại ra trang ĐÚNG dạng đã che, và không có chữ số nào bị lộ thêm", () => {
    const html = renderToStaticMarkup(<ThongTinPhieu phieu={phieu()} bayGio={BAY_GIO} />);

    expect(html).toContain("09****0000");
    // Không có chuỗi mười chữ số liền nào trên trang: nếu có ngày ai đó "ghép lại cho đẹp" thì
    // ca này đỏ.
    expect(html).not.toMatch(/\d{10}/);
  });

  it("ẩn danh: KHÔNG một mảnh tên hay số nào ra tới trang", () => {
    const html = renderToStaticMarkup(
      <ThongTinPhieu
        phieu={phieu({ anonymous: true, reporter_name: "", reporter_phone: "" })}
        bayGio={BAY_GIO}
      />,
    );

    expect(html).toContain("Người gửi ẩn danh");
    expect(html).not.toContain("Nguyễn");
    expect(html).not.toContain("09****");
  });

  it("hai ô hạn hiện HAI câu khác nhau cho hai loại `null`", () => {
    const html = renderToStaticMarkup(
      <ThongTinPhieu
        phieu={phieu({ acknowledge_due: null, resolve_due: null })}
        bayGio={BAY_GIO}
      />,
    );

    // `acknowledge_due` null = KHÔNG ÁP DỤNG (phiếu cán bộ nhập hộ).
    expect(html).toContain("Không áp dụng");
    // `resolve_due` null = CHƯA CÓ (phiếu chưa phân loại). Gộp hai cái là nói sai về một trong
    // hai loại phiếu.
    expect(html).toContain("Chưa ấn định");
  });

  it("quá hạn: chữ 'Quá hạn' ra tới trang, và KHÔNG kèm số ngày", () => {
    // Đặc tả §8.3 vẽ "Quá hạn 3 ngày". Con số ấy đếm bằng GIỜ LÀM VIỆC và cần ba bảng lịch của
    // chính xã ấy — `identity` sở hữu phép cộng (ADR 0007). Một con số đếm bằng giờ đồng hồ ở
    // trình duyệt sẽ lệch số của máy chủ vào đúng dịp lễ.
    const html = renderToStaticMarkup(
      <ThongTinPhieu phieu={phieu({ resolve_due: "2026-09-01T09:20:00Z" })} bayGio={BAY_GIO} />,
    );

    expect(html).toContain("Quá hạn");
    expect(html).not.toMatch(/Quá hạn\s*\d+\s*ngày/);
  });

  it("gốc đếm hạn và lúc vào sổ là HAI dòng riêng", () => {
    // `clock_from` là mốc cả hai hạn được đếm từ đó — không có nó thì không giải thích được hai
    // hạn ấy cho bất kỳ ai, kể cả một đoàn kiểm tra.
    const html = renderToStaticMarkup(<ThongTinPhieu phieu={phieu()} bayGio={BAY_GIO} />);

    expect(html).toContain("Người dân gửi lúc");
    expect(html).toContain("Vào sổ lúc");
  });

  it("câu trấn an về tra cứu của người dân có mặt nguyên văn khi phiếu chưa công khai", () => {
    const html = renderToStaticMarkup(<ThongTinPhieu phieu={phieu()} bayGio={BAY_GIO} />);
    expect(html).toContain("Người gửi vẫn tra cứu được phiếu của mình");
  });

  it("nội dung phản ánh ra tới trang nguyên văn — nó là thứ cán bộ phải xử lý", () => {
    const html = renderToStaticMarkup(<ThongTinPhieu phieu={phieu()} bayGio={BAY_GIO} />);
    expect(html).toContain("Rác tồn đọng ở đầu ngõ ba ngày chưa ai dọn.");
  });
});
