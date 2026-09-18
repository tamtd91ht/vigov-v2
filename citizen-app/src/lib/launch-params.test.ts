import { describe, expect, it } from "vitest";

import { batChanDoan, type KetQuaDo, thamSo, thamSoMoApp } from "./launch-params";

/**
 * VÌ SAO TỆP NHỎ NÀY ĐÁNG CÓ:
 *
 *   Từ khi `zmp-sdk` bị gỡ, đây là **cửa vào duy nhất** của cả lớp khám phá. Mọi thứ phía sau —
 *   mức tin theo nguồn, màn xác nhận xã, tên xã trên header — đều bắt đầu từ bảng tham số mà
 *   hàm dưới đây trả về. Một thay đổi ở đây hỏng cả luồng, và không màn hình nào trông sai.
 *
 *   Hai bất biến được ghim: cổng `debug` (bảng kỹ thuật không được lọt ra trước người dân hay
 *   người duyệt của Zalo), và việc đọc tham số **không được ném lỗi** ở nơi không có `window`.
 */

const do_thu = (url: Record<string, string>): KetQuaDo => ({ url, href: "" });

describe("đọc tham số mở app", () => {
  it("trả thẳng bảng tham số của URL — một nguồn, không còn gì phải gộp", () => {
    expect(thamSo(do_thu({ t: "01JDEMXA00000000000000000A", src: "qr" }))).toEqual({
      t: "01JDEMXA00000000000000000A",
      src: "qr",
    });
  });

  it("không có tham số nào thì trả bảng rỗng, không ném lỗi", () => {
    // Đây là đường phổ biến NHẤT từ lần mở thứ hai: mở từ danh sách app ghim, tìm trong Zalo,
    // quay lại tuần sau. ADR 0005 bắt buộc app phải chạy đúng ở đường này.
    expect(thamSo(do_thu({}))).toEqual({});
  });

  it("chạy được ở nơi không có `window`, và trả về 'không có tham số'", () => {
    // Bộ test này chạy trong Node, nên chính lượt chạy này là phép kiểm. Nếu hàm ném lỗi thay vì
    // bắt lại, cây React chết trước khi màn hình đầu tiên kịp vẽ — app trắng trơn trên máy thật
    // vì một hàm đọc tham số.
    const ra = thamSoMoApp();
    expect(ra.url).toEqual({});
    expect(ra.href).toBe("");
  });
});

describe("cổng của bảng chẩn đoán", () => {
  it("đóng khi không có `debug` — người dân và người duyệt Zalo không bao giờ thấy bảng", () => {
    expect(batChanDoan(do_thu({}))).toBe(false);
    expect(batChanDoan(do_thu({ t: "01JDEMXA00000000000000000A", src: "qr" }))).toBe(false);
    expect(batChanDoan(do_thu({ env: "TESTING", version: "7" }))).toBe(false);
  });

  it("mở khi có `debug`, kể cả khi giá trị rỗng", () => {
    // `&debug=1` và `&debug` phải cùng mở: người đang đo gõ link bằng tay trên điện thoại.
    expect(batChanDoan(do_thu({ debug: "1" }))).toBe(true);
    expect(batChanDoan(do_thu({ debug: "" }))).toBe(true);
  });
});
