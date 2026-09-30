import { describe, expect, it } from "vitest";

import {
  CANH_BAO_GO_KHONG_TRA_SO,
  MA_TRANG_THAI,
  lopHanVanBan,
  nhanBoPhanDangGiu,
  nhanDoKhan,
  nhanHanVanBan,
  nhanLoaiVanBan,
  nhanNgayCoThe,
  nhanSoVaoSo,
  nhanTrangThai,
  trangThaiHanVanBan,
} from "./nhan-van-ban";

/**
 * Phép suy ra "quá hạn" là thứ đắt nhất trong tệp này: nó phải đúng ở CẢ HAI phía của một mốc, và
 * nó phải KHÔNG BAO GIỜ trở thành một giá trị được lưu lại (luật 10, bất biến 3).
 */

const HAN = "2026-09-25T08:00:00Z";

describe("quá hạn SUY RA từ hạn so với hiện tại, không phải một trường máy chủ trả", () => {
  it("trước mốc là còn hạn, sau mốc là quá hạn — cùng một dữ liệu, chỉ khác đồng hồ", () => {
    // CẢ HAI VẾ, và đó là lý do thời điểm hiện tại được TRUYỀN VÀO: một hàm tự đọc `new Date()`
    // không kiểm được ở đúng lúc quan trọng nhất, tức là ngay trước và ngay sau hạn.
    expect(trangThaiHanVanBan(HAN, new Date("2026-09-25T07:59:00Z")).loai).toBe("conHan");
    expect(trangThaiHanVanBan(HAN, new Date("2026-09-25T08:01:00Z")).loai).toBe("quaHan");
  });

  it("hạn rỗng KHÔNG bị đọc thành quá hạn", () => {
    // VẾ CHỊU LỰC. `new Date("")` cho ra `NaN`, và một phép so sánh cẩu thả sẽ xếp mọi dòng chưa có
    // hạn vào nhóm trễ hạn — tức báo một xã đang trễ những việc chưa từng có cam kết nào.
    const h = trangThaiHanVanBan("", new Date("2030-01-01T00:00:00Z"));
    expect(h.loai).toBe("chuaCo");
    expect(nhanHanVanBan(h)).toBe("Chưa ấn định hạn");
  });

  it("chuỗi hạn hỏng thì hiện nguyên văn và KHÔNG bị gọi là quá hạn", () => {
    const h = trangThaiHanVanBan("không-phải-thời-điểm", new Date("2030-01-01T00:00:00Z"));
    expect(h.loai).toBe("conHan");
    expect(nhanHanVanBan(h)).toContain("không-phải-thời-điểm");
  });

  it("câu quá hạn nói rõ bằng CHỮ, và lớp màu chỉ là dấu hiệu thứ hai", () => {
    const h = trangThaiHanVanBan(HAN, new Date("2026-10-01T00:00:00Z"));
    expect(nhanHanVanBan(h)).toMatch(/^Quá hạn/);
    expect(lopHanVanBan(h)).toContain("chip-cham");
    // Dòng còn hạn KHÔNG mang lớp cảnh báo nào — vế phủ định của cùng một quyết định.
    expect(lopHanVanBan(trangThaiHanVanBan(HAN, new Date("2026-09-01T00:00:00Z")))).toBeUndefined();
  });

  it("KHÔNG đếm 'trễ N ngày' — con số ấy đếm bằng giờ làm việc và chỉ `identity` cộng được", () => {
    // Đặc tả §3.1 vẽ `d/M/yyyy (trễ N ngày)`. Con số ấy cố ý KHÔNG có ở đây: nó cần lịch làm việc,
    // ngày nghỉ lễ và ngày làm bù của chính xã ấy (ADR 0007). Một con số đếm bằng giờ đồng hồ ở
    // trình duyệt sẽ khác con số của máy chủ vào đúng dịp lễ — và con số trên màn hình cán bộ là
    // con số được báo cáo lên trên.
    const h = trangThaiHanVanBan(HAN, new Date("2026-10-05T00:00:00Z"));
    expect(nhanHanVanBan(h)).not.toMatch(/trễ\s*\d/i);
    expect(nhanHanVanBan(h)).not.toMatch(/\d+\s*ngày/i);
  });
});

describe("nhãn trạng thái và độ khẩn", () => {
  it("sáu mã trạng thái của đặc tả §3.2, đủ và đúng chữ", () => {
    expect(MA_TRANG_THAI).toHaveLength(6);
    expect(nhanTrangThai("moi-vao-so")).toBe("Mới vào sổ");
    expect(nhanTrangThai("luu-khong-thu-ly")).toBe("Lưu, không thụ lý");
  });

  it("mã lạ KHÔNG bị nuốt — hiện nguyên mã và nói rõ chưa có nhãn", () => {
    // Hợp đồng khai `status` là `string` trơn (không `enum`), nên máy chủ thêm một mã thì `tsc`
    // không đỏ. Nhánh dự phòng vì thế phải NÓI RA, không được giấu bằng một ô trống.
    expect(nhanTrangThai("ma-moi-cua-may-chu")).toContain("ma-moi-cua-may-chu");
    expect(nhanTrangThai("ma-moi-cua-may-chu")).toMatch(/chưa có nhãn/);
  });

  it("độ khẩn rỗng là một câu trả lời THẬT, không phải 'Thường'", () => {
    // `van_ban.go:61`: chuỗi rỗng tới CSDL thành NULL. Mặc định nó thành "Thường" là xếp loại một
    // văn bản nhà nước bằng một lựa chọn không ai làm.
    expect(nhanDoKhan("")).toBe("Không ghi độ khẩn");
    expect(nhanDoKhan("")).not.toMatch(/Thường/);
    expect(nhanDoKhan("hoa-toc")).toBe("Hoả tốc");
  });
});

describe("tra danh mục — năm ca, không ca nào là một ô trống", () => {
  it("loại văn bản: mã không còn trong danh mục vẫn được nói ra", () => {
    expect(nhanLoaiVanBan({ loai: "khongTraDuoc" })).toMatch(/không còn trong danh mục/);
    expect(nhanLoaiVanBan({ loai: "coTen", ten: "Công văn" })).toBe("Công văn");
  });

  it("bộ phận: 'chưa chuyển bộ phận nào' KHÁC 'không tra được'", () => {
    // Hai câu cho hai việc khác nhau: ca đầu là việc của cán bộ văn phòng (chuyển tiếp), ca sau là
    // dữ liệu lệch mà chỉ người quản trị sửa được.
    expect(nhanBoPhanDangGiu({ loai: "chuaGan" })).toBe("Chưa chuyển bộ phận nào");
    expect(nhanBoPhanDangGiu({ loai: "khongTraDuoc" })).toMatch(/Không tra được/);
  });
});

describe("số vào sổ và ngày", () => {
  it("số luôn đi kèm năm", () => {
    // Dãy số chạy lại từ 1 mỗi năm, nên "số 7" một mình không chỉ ra văn bản nào — mà đây đúng là
    // con số cán bộ đọc qua điện thoại cho một cơ quan khác.
    expect(nhanSoVaoSo(7, 2026)).toBe("7/2026");
  });

  it("ngày tuỳ chọn rỗng thành chữ, không thành ô trống", () => {
    expect(nhanNgayCoThe("")).toBe("Không ghi");
    expect(nhanNgayCoThe("2026-09-18")).toBe("18/09/2026");
  });
});

describe("câu cảnh báo trước khi gỡ — ba điều phải nói đủ", () => {
  it("nói rằng số KHÔNG trả về dãy, KHÔNG được cấp lại, và bản ghi vẫn được giữ", () => {
    // Ba vế, và thiếu vế nào cũng sai theo một kiểu riêng: thiếu vế đầu thì cán bộ tưởng gỡ xong
    // số ấy dùng lại được; thiếu vế cuối thì họ tưởng vừa xoá mất một hồ sơ lưu trữ.
    expect(CANH_BAO_GO_KHONG_TRA_SO).toMatch(/KHÔNG trả số về dãy/);
    expect(CANH_BAO_GO_KHONG_TRA_SO).toMatch(/không bao giờ/);
    expect(CANH_BAO_GO_KHONG_TRA_SO).toMatch(/không bị\s*\n?\s*xoá khỏi hệ thống|không bị xoá/);
  });
});
