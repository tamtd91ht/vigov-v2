import { describe, expect, it } from "vitest";

import {
  GHI_CHU_CHI_XEM_THON,
  loaiDonVi,
  lopLoaiDonVi,
  lopTrangThaiDiaBan,
  nhanLoaiDonVi,
  nhanSoDem,
  nhanTrangThaiDiaBan,
  THON_RONG,
} from "./nhan-thon";

describe("cột Loại của một địa bàn — ba ca, không ca nào là ô trống", () => {
  it("tra được nhãn thì hiện nhãn", () => {
    const l = loaiDonVi("thon", "Thôn");
    expect(l).toEqual({ loai: "coNhan", nhan: "Thôn" });
    expect(nhanLoaiDonVi(l)).toBe("Thôn");
    expect(lopLoaiDonVi(l)).toBeUndefined();
  });

  it("chưa phân loại (`type_code` rỗng) là trạng thái HỢP LỆ, không phải dữ liệu hỏng", () => {
    // Máy chủ gọi đây là "a legitimate state"
    // (`service-identity/internal/http/thon_to_dan_pho.go`, trường `TypeCode`).
    const l = loaiDonVi("", "");
    expect(l).toEqual({ loai: "chuaPhanLoai" });
    expect(nhanLoaiDonVi(l)).toBe("Chưa phân loại");
    expect(lopLoaiDonVi(l)).toBe("nhan-trong");
  });

  it("có mã mà nhãn rỗng: hiện MÃ và nói rõ vì sao — không im lặng, không tưởng là tên loại", () => {
    // Mục loại đã bị xoá mềm trong khi các thôn vẫn mang mã của nó. Máy chủ dặn thẳng: "Fall back
    // to showing the code", và GIỮ dòng thôn lại — bỏ nó đi là biến một danh mục lộn xộn thành một
    // thôn biến mất khỏi danh sách của chính đơn vị.
    const l = loaiDonVi("to-dan-pho", "");
    expect(l).toEqual({ loai: "chiCoMa", ma: "to-dan-pho" });
    expect(nhanLoaiDonVi(l)).toBe("to-dan-pho (nhãn không còn trong danh mục)");
    expect(lopLoaiDonVi(l)).toBe("nhan-lech");
  });

  it("'chưa phân loại' và 'nhãn đã biến mất' KHÔNG được nhìn giống nhau", () => {
    // Hai việc khác nhau, hai người sửa khác nhau: ca đầu là việc của cán bộ nhập liệu, ca sau là
    // dữ liệu lệch mà chỉ người quản trị sửa được. Gộp làm một là báo sai một trong hai.
    const chua = loaiDonVi("", "");
    const lech = loaiDonVi("thon", "");
    expect(nhanLoaiDonVi(chua)).not.toBe(nhanLoaiDonVi(lech));
    expect(lopLoaiDonVi(chua)).not.toBe(lopLoaiDonVi(lech));
  });
});

describe("Số hộ và Nhân khẩu — `null` KHÔNG PHẢI `0`", () => {
  it("`null` là 'chưa nhập', và tuyệt đối không hiện thành 0", () => {
    // `0` là một KHẲNG ĐỊNH về địa bàn ("không có hộ nào"); `null` là sự VẮNG MẶT của một khẳng
    // định. Một đơn vị nhập bảng tính thiếu cột sẽ công bố những số không nó chưa bao giờ khẳng
    // định — và một số không đi tiếp vào báo cáo gửi lên trên như một con số thật
    // (`service-identity/internal/http/thon_to_dan_pho.go`, trường `HouseholdCount`).
    expect(nhanSoDem(null)).toBe("Chưa nhập");
    expect(nhanSoDem(null)).not.toBe("0");
    expect(nhanSoDem(null)).not.toBe(nhanSoDem(0));
  });

  it("`0` hiện thành 0 — nó là một con số đơn vị đã khẳng định", () => {
    expect(nhanSoDem(0)).toBe("0");
  });

  it("định dạng nghìn theo vi-VN, đúng ví dụ §2 của đặc tả", () => {
    // Ghim `vi-VN` chứ không theo cài đặt máy: ở miền địa phương khác, `1.132` đọc thành một phẩy
    // một ba hai.
    expect(nhanSoDem(1132)).toBe("1.132");
    expect(nhanSoDem(284)).toBe("284");
  });

  it("giá trị không phải số hữu hạn thì nói ra bằng một câu KHÁC, không hiện 'NaN'", () => {
    expect(nhanSoDem(Number.NaN)).toBe("Không đọc được");
    expect(nhanSoDem(Number.NaN)).not.toBe(nhanSoDem(null));
  });
});

describe("trạng thái địa bàn dùng chung một hàm với tab Danh mục", () => {
  it("hai chữ của đặc tả, và đúng cặp lớp CSS", () => {
    // Máy chủ nói `active` trên một thôn mang đúng nghĩa `active` trên một mục danh mục ("same
    // reading as the catalogues"). Hai hàm cùng trả hai chữ ấy là hai bản sao sẽ trôi.
    expect(nhanTrangThaiDiaBan(true)).toBe("Đang dùng");
    expect(nhanTrangThaiDiaBan(false)).toBe("Đã tắt");
    expect(lopTrangThaiDiaBan(false)).toBe("chip chip-ngung");
  });
});

describe("TRẠNG THÁI RỖNG của tab Thôn / Tổ dân phố", () => {
  it("nói đủ hai điều, và không nói là lỗi", () => {
    expect(THON_RONG).toBe(
      "Đơn vị chưa có thôn hoặc tổ dân phố nào. " +
        "Màn hình này chỉ xem, không thêm được địa bàn mới.",
    );
    for (const tu of ["lỗi", "thất bại", "thử lại", "đang phát triển"]) {
      expect(THON_RONG.toLowerCase()).not.toContain(tu);
    }
  });

  it("ghi chú đầu tab nói lý do thật, không hứa hẹn", () => {
    expect(GHI_CHU_CHI_XEM_THON).toContain("chỉ xem");
    expect(GHI_CHU_CHI_XEM_THON.toLowerCase()).not.toContain("đang phát triển");
  });
});
