import { describe, expect, it } from "vitest";

import { type KetQuaDo, laTenMien, thamSo, thamSoMoApp, thamSoXa } from "./launch-params";

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
    expect(thamSo(do_thu({ d: "xa-vi-du.vigov.example", src: "qr" }))).toEqual({
      d: "xa-vi-du.vigov.example",
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

/**
 * `d` + `src` (quyết định 27/09/2026, ADR 0047 §Trả lời). `d` không kèm một `src` tin được thì bị bỏ
 * qua — cư xử như mở không tham số, tức KHÔNG hỏi máy chủ về xã nào. `t` và `v` đã bỏ hẳn.
 */
describe("gợi ý xã trên đường liên kết — `d` chỉ được đọc khi `src` là qr hoặc zns", () => {
  const D = "xa-vi-du.vigov.example";

  it("qr và zns: trả tên miền và nguồn", () => {
    expect(thamSoXa(do_thu({ d: D, src: "qr" }))).toEqual({ ten_mien: D, nguon: "qr" });
    expect(thamSoXa(do_thu({ d: D, src: "zns" }))).toEqual({ ten_mien: D, nguon: "zns" });
  });

  it("không `src`, `src` lạ, hay `src` viết hoa: bỏ qua `d` — không có mặc định 'coi như qr'", () => {
    for (const src of [undefined, "", "share", "QR", "email", "constructor", "__proto__"]) {
      const tham_so: Record<string, string> = { d: D };
      if (src !== undefined) tham_so["src"] = src;
      expect(thamSoXa(do_thu(tham_so)), `src=${String(src)}`).toBeNull();
    }
  });

  it("`t` và `v` không còn nghĩa gì: không thay được `d`, không đi kèm được `d`", () => {
    expect(thamSoXa(do_thu({ t: D, src: "qr" }))).toBeNull();
    expect(thamSoXa(do_thu({ t: "01JDEMXA00000000000000000A", src: "qr" }))).toBeNull();
    expect(thamSoXa(do_thu({ v: "2", src: "zns" }))).toBeNull();
    expect(thamSoXa(do_thu({ d: D, t: "x", v: "2", src: "qr" }))).toEqual({ ten_mien: D, nguon: "qr" });
  });

  it("`d` sai khuôn tên miền thì bỏ qua — chặn rác ở giao diện, máy chủ vẫn là bên quyết", () => {
    for (const d of [
      "",
      "localhost",
      "xa a.vigov.vn",
      "https://xa-a.vigov.vn",
      "xa-a.vigov.vn/duong",
      "xa-a.vigov.vn:443",
      "nguoi@xa-a.vigov.vn",
      "-xa.vigov.vn",
      "xa-.vigov.vn",
      "xa..vigov.vn",
      ".vigov.vn",
      "xã-a.vigov.vn",
      `${"a".repeat(64)}.vigov.vn`,
    ]) {
      expect(thamSoXa(do_thu({ d, src: "qr" })), `d=${d}`).toBeNull();
      expect(laTenMien(d), `d=${d}`).toBe(false);
    }
  });

  it("chữ hoa trong `d` được hạ xuống — tên miền không phân biệt hoa thường", () => {
    expect(thamSoXa(do_thu({ d: "XA-Vi-Du.VIGOV.example", src: "qr" }))).toEqual({ ten_mien: D, nguon: "qr" });
  });

  it("không tham số nào thì không có gợi ý", () => {
    expect(thamSoXa(do_thu({}))).toBeNull();
    expect(thamSoXa(thamSoMoApp())).toBeNull();
  });
});
