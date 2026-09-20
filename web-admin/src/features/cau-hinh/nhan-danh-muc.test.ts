import { describe, expect, it } from "vitest";

import { nhanTrangThai } from "./nhan-can-bo";
import {
  GHI_CHU_CHI_XEM,
  GIAI_THICH_THANG_BAC,
  lopTrangThaiMuc,
  nhanMacDinh,
  nhanNhomRong,
  nhanSoMuc,
  nhanTrangThaiMuc,
} from "./nhan-danh-muc";

describe("chip trạng thái của một mục danh mục", () => {
  it("dùng đúng hai chữ của đặc tả §5", () => {
    expect(nhanTrangThaiMuc(true)).toBe("Đang dùng");
    expect(nhanTrangThaiMuc(false)).toBe("Đã tắt");
  });

  it("KHÔNG mượn chữ của tab Người dùng — hai màn hình nói về hai thứ khác nhau", () => {
    // Ở tab Người dùng, `active` nói một CON NGƯỜI còn công tác hay đã nghỉ. Ở đây nó nói một MÃ
    // còn được chọn khi lập hồ sơ mới hay không. Ngày hai bên dùng chung một câu chữ, cán bộ sẽ
    // suy ra một quan hệ không có giữa hai bảng đứng cạnh nhau trên cùng một trang.
    expect(nhanTrangThaiMuc(false)).not.toBe(nhanTrangThai(false));
    expect(nhanTrangThaiMuc(true)).not.toBe(nhanTrangThai(true));
  });

  it("mục đã tắt KHÔNG dùng lớp của mục đang dùng", () => {
    expect(lopTrangThaiMuc(true)).toBe("chip chip-hoat-dong");
    expect(lopTrangThaiMuc(false)).toBe("chip chip-ngung");
  });
});

describe("cột Mặc định", () => {
  it("cả hai ca đều có chữ — không ca nào để lại một ô trống", () => {
    expect(nhanMacDinh(true)).toBe("Mặc định");
    expect(nhanMacDinh(false)).toBe("Không");
    // Một ô trống hoặc một dấu `—` đọc bằng trình đọc màn hình thì thành im lặng, và người dùng
    // không phân biệt được "ô chưa có dữ liệu" với "mục này không phải mặc định".
    expect(nhanMacDinh(false)).not.toBe("");
  });
});

describe("số mục cạnh tên nhóm", () => {
  it("đếm cả mục đã tắt, vì bảng liệt kê cả mục đã tắt", () => {
    expect(nhanSoMuc(7)).toBe("7 mục");
    expect(nhanSoMuc(1)).toBe("1 mục");
  });
});

describe("TRẠNG THÁI RỖNG — câu quan trọng nhất của màn hình này", () => {
  it("nói ĐỦ HAI điều: đơn vị chưa có mục, và không thêm được từ đây", () => {
    // Thiếu điều thứ nhất thì màn hình trông như đang hỏng. Thiếu điều thứ hai thì cán bộ đi tìm
    // nút `+ Thêm mục` mà đặc tả §5 có vẽ nhưng hợp đồng không có tuyến phía sau (câu hỏi mở #21).
    const cau = nhanNhomRong("Loại văn bản");

    expect(cau).toBe(
      "Đơn vị chưa có mục nào trong danh mục Loại văn bản. " +
        "Màn hình này chỉ xem, không thêm được mục mới.",
    );
  });

  it("NÊU TÊN NHÓM trong chính câu, không chỉ ở tiêu đề phía trên", () => {
    // Trình đọc màn hình đọc từng đoạn một. Một câu "chưa có mục nào trong danh mục này" tách khỏi
    // tiêu đề thì không còn biết đang nói về danh mục nào trong bảy danh mục.
    expect(nhanNhomRong("Mức ưu tiên nhiệm vụ")).toContain("Mức ưu tiên nhiệm vụ");
    expect(nhanNhomRong("Khối nhiệm vụ")).toContain("Khối nhiệm vụ");
  });

  it("KHÔNG nói là lỗi, và không hứa hẹn gì", () => {
    const cau = nhanNhomRong("Loại nhiệm vụ");
    for (const tu of ["lỗi", "thất bại", "sắp", "đang phát triển", "thử lại"]) {
      expect(cau.toLowerCase()).not.toContain(tu);
    }
  });
});

describe("ghi chú đầu tab", () => {
  it("nói LÝ DO THẬT vì sao không có nút ghi nào, không nói 'sắp có'", () => {
    expect(GHI_CHU_CHI_XEM).toContain("chỉ xem");
    expect(GHI_CHU_CHI_XEM.toLowerCase()).not.toContain("sắp");
    expect(GHI_CHU_CHI_XEM.toLowerCase()).not.toContain("đang phát triển");
  });
});

describe("câu giải thích thang bậc mức ưu tiên", () => {
  it("KHÔNG khẳng định đầu nào của thang là cao nhất", () => {
    // Máy chủ cố ý không khẳng định điều đó ("Whichever end that is" —
    // `service-petitions/internal/store/muc_uu_tien_nhiem_vu.go`). Màn hình tự đặt ra quy ước
    // "bậc 1 là cao nhất" là bịa một sự thật về dữ liệu của đơn vị, và cán bộ sẽ lập nhiệm vụ
    // theo đúng quy ước bịa ra ấy.
    for (const tu of ["cao nhất", "thấp nhất", "gấp nhất", "bậc 1", "đầu tiên"]) {
      expect(GIAI_THICH_THANG_BAC.toLowerCase()).not.toContain(tu);
    }
    expect(GIAI_THICH_THANG_BAC).toContain("không");
    expect(GIAI_THICH_THANG_BAC).toContain("sắp xếp lại");
  });
});
