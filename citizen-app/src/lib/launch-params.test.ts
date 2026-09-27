import { describe, expect, it } from "vitest";

import { type KetQuaDo, thamSo, thamSoMoApp } from "./launch-params";

/**
 * VÌ SAO TỆP NHỎ NÀY ĐÁNG CÓ:
 *
 *   Từ khi `zmp-sdk` bị gỡ khỏi đường đọc tham số, đây là **cửa vào duy nhất** của lớp khám phá.
 *   Mọi thứ phía sau — mức tin theo nguồn, màn xác nhận xã, tên xã trên header — đều bắt đầu từ
 *   bảng tham số mà hàm dưới đây trả về. Một thay đổi ở đây hỏng cả luồng, và không màn hình nào
 *   trông sai.
 *
 *   Bất biến được ghim: việc đọc tham số **không được ném lỗi** ở nơi không có `window`.
 *
 *   (Cổng `debug` của bảng chẩn đoán từng được ghim ở đây. Bảng ấy đã bị xoá khỏi app ngày
 *   27/09/2026 cùng hai biến thể bản dựng, nên cổng và hai ca của nó đi theo.)
 */

const do_thu = (url: Record<string, string>): KetQuaDo => ({ url });

describe("đọc tham số mở app", () => {
  it("trả thẳng bảng tham số của URL — một nguồn, không còn gì phải gộp", () => {
    expect(thamSo(do_thu({ t: "xa-vi-du.vigov.example", src: "qr" }))).toEqual({
      t: "xa-vi-du.vigov.example",
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
    expect(thamSoMoApp()).toEqual({ url: {} });
  });
});
