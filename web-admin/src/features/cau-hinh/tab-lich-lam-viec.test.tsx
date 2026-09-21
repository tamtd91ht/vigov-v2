import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type {
  identity_danhSachCaLamBuRa,
  identity_danhSachCaLamViecRa,
  identity_danhSachNgayNghiLeRa,
} from "@/lib/api/schema.gen";

import { LICH_TUAN_RONG, nhanNgayLamBuRong } from "./nhan-lich-lam-viec";
import { BangLichTuan, BangNgayLamBu, BangNgayNghiLe } from "./tab-lich-lam-viec";

/**
 * Canh những gì PHẢI RA TỚI TRANG trên ba bảng lịch. Câu quan trọng nhất ở đây không phải một
 * dòng dữ liệu — nó là câu `problems` do máy chủ viết: "xã chưa cấu hình giờ làm việc, chưa tính
 * được hạn xử lý nào". Bôi trắng nó thì màn hình trông hoàn toàn bình thường, và cả xã không
 * biết mọi hạn xử lý đang không tính được.
 */

function lichTuan(sua: Partial<identity_danhSachCaLamViecRa> = {}): identity_danhSachCaLamViecRa {
  return { items: [], problems: [], ...sua };
}

describe("bảng giờ làm việc trong tuần", () => {
  it("lịch TRỐNG: câu của máy chủ ra tới trang, nguyên văn, và ở dạng báo động", () => {
    const cau = "Xã chưa cấu hình giờ làm việc. Chưa tính được hạn xử lý nào cho tới khi có ít nhất một ca làm việc.";
    const html = renderToStaticMarkup(
      <BangLichTuan
        trangThai={{
          pha: "xong",
          duLieu: lichTuan({
            problems: [{ kind: "empty_calendar", weekday: null, session_ids: [], message: cau }],
          }),
        }}
      />,
    );

    expect(html).toContain(cau);
    // Đây là ca ĐÁNG báo động, khác hẳn một danh mục rỗng: xã không tính được hạn nào.
    expect(html).toContain('role="alert"');
    // Và trạng thái rỗng của bảng vẫn có chữ, không phải một ô trống.
    expect(html).toContain(LICH_TUAN_RONG);
  });

  it("hai ca chồng giờ: câu của máy chủ ra tới trang, kể cả khi bảng vẫn có dòng", () => {
    const cau = "Hai ca làm việc thứ Ba chồng giờ nhau — số giờ trong khoảng chồng bị tính hai lần.";
    const html = renderToStaticMarkup(
      <BangLichTuan
        trangThai={{
          pha: "xong",
          duLieu: lichTuan({
            items: [
              { id: "01JCA1", weekday: 2, start: "07:30:00", end: "11:30:00", note: "Buổi sáng" },
              { id: "01JCA2", weekday: 2, start: "10:00:00", end: "12:00:00", note: "" },
            ],
            problems: [{ kind: "overlapping_sessions", weekday: 2, session_ids: ["01JCA1", "01JCA2"], message: cau }],
          }),
        }}
      />,
    );

    expect(html).toContain(cau);
    expect(html).toContain("07:30 – 11:30");
    // Thứ hiện bằng TÊN, không bằng số ISO trần: cán bộ không phải quy đổi trong đầu để tìm dòng
    // cần sửa.
    expect(html).toContain("Thứ Ba");
    expect(html).not.toContain(">2<");
  });

  it("ghi chú rỗng KHÔNG để lại ô trống", () => {
    const html = renderToStaticMarkup(
      <BangLichTuan
        trangThai={{
          pha: "xong",
          duLieu: lichTuan({
            items: [{ id: "01JCA1", weekday: 1, start: "07:30:00", end: "11:30:00", note: "" }],
          }),
        }}
      />,
    );

    expect(html).toContain("nhan-trong");
  });
});

describe("bảng ngày nghỉ lễ", () => {
  it("409 lịch mâu thuẫn: câu của máy chủ ra tới trang NGUYÊN VĂN, kèm những ngày cụ thể", () => {
    // Một lời từ chối không nói ngày nào là một lời từ chối không ai sửa được theo.
    const cau =
      "Cấu hình lịch của xã đang mâu thuẫn: ngày 2026-09-02 vừa được khai là ngày nghỉ lễ vừa " +
      "được khai là ngày làm bù. Hệ thống không tự chọn bên nào — vui lòng sửa một trong hai.";
    const html = renderToStaticMarkup(
      <BangNgayNghiLe trangThai={{ pha: "loi", thongBao: cau }} nam={2026} />,
    );

    expect(html).toContain(cau);
    expect(html).toContain("2026-09-02");
  });

  it("có ngày nghỉ: ngày đọc theo dd/MM/yyyy và tên lễ ra tới trang", () => {
    const duLieu: identity_danhSachNgayNghiLeRa = {
      items: [{ id: "01JNL1", date: "2026-09-02", name: "Quốc khánh" }],
    };
    const html = renderToStaticMarkup(
      <BangNgayNghiLe trangThai={{ pha: "xong", duLieu }} nam={2026} />,
    );

    expect(html).toContain("02/09/2026");
    expect(html).toContain("Quốc khánh");
  });

  it("năm rỗng: có chữ cho người đọc, và KHÔNG dựng như một lỗi", () => {
    const html = renderToStaticMarkup(
      <BangNgayNghiLe trangThai={{ pha: "xong", duLieu: { items: [] } }} nam={2026} />,
    );

    expect(html).toContain('class="trang-thai-rong"');
    expect(html).not.toContain('class="trang-thai-rong"></p>');
    expect(html).not.toContain('role="alert"');
  });
});

describe("bảng ngày làm bù", () => {
  it("một dòng là một CA: hai ca cùng ngày ra thành hai dòng, không gộp", () => {
    // Gộp theo ngày sẽ nuốt mất giờ nghỉ trưa của chính ngày ấy, và mỗi giờ nuốt mất là một giờ
    // cộng sai vào mọi hạn đi qua ngày đó.
    const duLieu: identity_danhSachCaLamBuRa = {
      items: [
        { id: "01JLB1", date: "2026-02-21", start: "07:30:00", end: "11:30:00", name: "Làm bù nghỉ Tết" },
        { id: "01JLB2", date: "2026-02-21", start: "13:30:00", end: "17:00:00", name: "Làm bù nghỉ Tết" },
      ],
      problems: [],
    };
    const html = renderToStaticMarkup(
      <BangNgayLamBu trangThai={{ pha: "xong", duLieu }} nam={2026} />,
    );

    expect(html).toContain("07:30 – 11:30");
    expect(html).toContain("13:30 – 17:00");
    // Tên thông báo là câu trả lời khi đoàn kiểm tra hỏi vì sao một hạn chạy qua ngày thứ Bảy.
    expect(html).toContain("Làm bù nghỉ Tết");
  });

  it("năm không có ngày làm bù nào nói một câu KHÁC với lịch tuần trống", () => {
    const html = renderToStaticMarkup(
      <BangNgayLamBu
        trangThai={{ pha: "xong", duLieu: { items: [], problems: [] } }}
        nam={2026}
      />,
    );

    expect(html).toContain(nhanNgayLamBuRong(2026));
    // Phần lớn các năm là như vậy — không có gì để báo động.
    expect(html).not.toContain('role="alert"');
  });
});
