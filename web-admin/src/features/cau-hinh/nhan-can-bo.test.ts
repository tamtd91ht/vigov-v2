import { describe, expect, it } from "vitest";

import {
  nhanDangNhapGanNhat,
  nhanNgayTao,
  nhanTaiKhoan,
  nhanTrangThai,
} from "./nhan-can-bo";

describe("cột Đăng nhập gần nhất", () => {
  it("`null` là 'Chưa đăng nhập' — một sự thật máy chủ khẳng định, không phải một lỗi", () => {
    // Hợp đồng khai `last_login_at: string | null`. `null` nghĩa là người này chưa đăng nhập
    // lần nào — đúng dòng mà người quản trị đi tìm trên màn hình này.
    expect(nhanDangNhapGanNhat(null)).toBe("Chưa đăng nhập");
  });

  it("mốc thật hiện theo giờ Việt Nam, đúng dạng của đặc tả §3", () => {
    // 04:10:38Z = 11:10:38 giờ Việt Nam. Múi giờ được ghim trong mã, nên bài test này ra cùng
    // một kết quả trên máy của người viết và trên máy chạy CI ở múi giờ khác.
    expect(nhanDangNhapGanNhat("2026-09-16T04:10:38Z")).toBe("11:10:38 16/9/2026");
  });

  it("mốc không đọc được nói ra bằng câu KHÁC — không gộp vào 'Chưa đăng nhập'", () => {
    const loi = nhanDangNhapGanNhat("khong-phai-thoi-gian");
    expect(loi).not.toBe("Chưa đăng nhập");
    expect(loi).toBe("Mốc thời gian không đọc được");
  });
});

describe("cột Ngày tạo", () => {
  it("cùng cách hiện mốc với cột đăng nhập — một dạng, một nơi định nghĩa", () => {
    expect(nhanNgayTao("2026-01-05T00:05:00Z")).toBe("07:05:00 5/1/2026");
  });
});

describe("Trạng thái và Tài khoản là hai câu hỏi khác nhau", () => {
  it("`active` nói người này còn làm việc hay đã ngừng", () => {
    expect(nhanTrangThai(true)).toBe("Đang hoạt động");
    expect(nhanTrangThai(false)).toBe("Đã ngừng");
  });

  it("`has_account` nói người này đăng nhập được hay chỉ có trong danh bạ", () => {
    expect(nhanTaiKhoan(true)).toBe("Có tài khoản");
    expect(nhanTaiKhoan(false)).toBe("Chỉ trong danh bạ");
  });

  it("hai nhãn không trùng nhau ở bất kỳ tổ hợp nào", () => {
    // Đọc cái này thay cho cái kia là báo một dòng danh bạ thành một tài khoản đang hoạt động —
    // một con số xã gửi lên cấp trên.
    const nhan = new Set([
      nhanTrangThai(true),
      nhanTrangThai(false),
      nhanTaiKhoan(true),
      nhanTaiKhoan(false),
    ]);
    expect(nhan.size).toBe(4);
  });
});
