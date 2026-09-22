import { describe, expect, it } from "vitest";

import { nhanTrangThai } from "./nhan-can-bo";
import {
  GHI_CHU_BA_TANG,
  GHI_CHU_NHOM_CHI_XEM,
  GIAI_THICH_THANG_BAC,
  giaiThichKhongThaoTac,
  lopTrangThaiMuc,
  nhanMacDinh,
  nhanNguon,
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

describe("cột Nguồn", () => {
  it("dịch đúng hai giá trị của ràng buộc CHECK", () => {
    expect(nhanNguon("he-thong")).toBe("Hệ thống");
    expect(nhanNguon("don-vi")).toBe("Đơn vị");
  });

  it("GIÁ TRỊ LẠ hiện nguyên văn, không đoán bừa thành một trong hai", () => {
    // Ràng buộc CHECK chỉ nhận hai giá trị, nên giá trị thứ ba nghĩa là hợp đồng đã trôi khỏi
    // CSDL. Dịch bừa nó thành "Đơn vị" là giấu một sự cố sau một chữ trông bình thường — và chính
    // chữ ấy quyết định dòng này có nút Xoá hay không.
    expect(nhanNguon("he-thong-2")).toBe("he-thong-2");
    expect(nhanNguon("")).toBe("");
  });
});

describe("TRẠNG THÁI RỖNG — câu quan trọng nhất của màn hình này", () => {
  it("nói ĐỦ HAI điều: đơn vị chưa có mục, và làm gì tiếp theo", () => {
    // Thiếu điều thứ nhất thì màn hình trông như đang hỏng. Thiếu điều thứ hai thì cán bộ đứng
    // trước một bảng trống mà không biết bước kế tiếp là bấm nút nào.
    expect(nhanNhomRong("Loại văn bản", "themDuoc")).toBe(
      "Đơn vị chưa có mục nào trong danh mục Loại văn bản. " +
        "Bấm + Thêm mục ở trên để lập mục đầu tiên.",
    );
  });

  it("BA LÝ DO, BA CÂU KHÁC NHAU — không gộp 'thiếu quyền' với 'chưa có tuyến'", () => {
    // Một người cần đi xin quyền; người kia xin quyền cũng không có gì mở ra. Một câu chung cho
    // cả hai là một câu sai với một trong hai người đọc.
    const ds = (["themDuoc", "thieuQuyen", "khongCoTuyen"] as const).map((l) =>
      nhanNhomRong("Loại văn bản", l),
    );
    expect(new Set(ds).size).toBe(3);
    expect(nhanNhomRong("Loại văn bản", "thieuQuyen")).toContain("quyền");
  });

  it("NÊU TÊN NHÓM trong chính câu, không chỉ ở tiêu đề phía trên", () => {
    // Trình đọc màn hình đọc từng đoạn một. Một câu "chưa có mục nào trong danh mục này" tách khỏi
    // tiêu đề thì không còn biết đang nói về danh mục nào trong bảy danh mục.
    expect(nhanNhomRong("Mức ưu tiên nhiệm vụ", "themDuoc")).toContain("Mức ưu tiên nhiệm vụ");
    expect(nhanNhomRong("Khối nhiệm vụ", "khongCoTuyen")).toContain("Khối nhiệm vụ");
  });

  it("KHÔNG nói là lỗi, và không hứa hẹn gì", () => {
    for (const l of ["themDuoc", "thieuQuyen", "khongCoTuyen"] as const) {
      const cau = nhanNhomRong("Loại nhiệm vụ", l).toLowerCase();
      for (const tu of ["lỗi", "thất bại", "sắp", "đang phát triển", "thử lại"]) {
        expect(cau).not.toContain(tu);
      }
    }
  });
});

describe("ghi chú đầu tab — quy tắc ba tầng", () => {
  it("nói ra cả ba tầng TRƯỚC, để người dùng không phát hiện quy tắc bằng cách thiếu nút", () => {
    // Một cán bộ thấy dòng này có `Xoá` còn dòng kia không sẽ kết luận màn hình hỏng rồi gọi hỗ
    // trợ — trong khi đó là quy tắc đang làm đúng việc của nó (ADR 0024).
    expect(GHI_CHU_BA_TANG).toContain("đơn vị tự thêm");
    expect(GHI_CHU_BA_TANG).toContain("không xoá được");
    expect(GHI_CHU_BA_TANG).toContain("đổi được nhãn");
    expect(GHI_CHU_BA_TANG.toLowerCase()).not.toContain("sắp");
  });

  it("nhóm chưa có tuyến ghi nói rõ là CHƯA, không hứa khi nào có", () => {
    expect(GHI_CHU_NHOM_CHI_XEM).toContain("chỉ xem");
    expect(GHI_CHU_NHOM_CHI_XEM.toLowerCase()).not.toContain("sắp");
    expect(GHI_CHU_NHOM_CHI_XEM.toLowerCase()).not.toContain("đang phát triển");
  });
});

describe("câu giải thích vì sao một dòng thiếu nút", () => {
  it("tầng 2 và tầng 3 nói hai chuyện khác nhau", () => {
    // Gộp lại thành "mục này không sửa được" là bỏ mất điều người đọc cần nhất: tầng 2 tắt được,
    // tầng 3 thì không.
    expect(giaiThichKhongThaoTac(2)).toContain("tắt được");
    expect(giaiThichKhongThaoTac(3)).toContain("nhãn hiển thị");
    expect(giaiThichKhongThaoTac(2)).not.toBe(giaiThichKhongThaoTac(3));
  });

  it("KHÔNG BAO GIỜ rỗng — ô trống đọc bằng trình đọc màn hình thành im lặng", () => {
    for (const t of [null, 0, 1, 2, 3, 9]) {
      expect(giaiThichKhongThaoTac(t).length).toBeGreaterThan(0);
    }
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
