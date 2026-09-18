/// <reference types="vite/client" />
import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import { describe, expect, it } from "vitest";

import * as ChanDoanDayDu from "../diagnostics/index";
import * as ChanDoanRong from "../diagnostics/index.rong";
import * as DayDu from "./index";
import * as Rong from "./index.rong";

/**
 * HAI BIẾN THỂ BẢN DỰNG — phép kiểm về chính cơ chế, không về giao diện.
 *
 * `bundle-for-zalo.test.ts` dựng bản `goc` thật rồi đọc bundle: đó là bằng chứng cuối cùng. Nhưng
 * nó chạy một lần mỗi lần dựng và mất vài giây, và khi nó đỏ thì thông điệp là "bundle chứa một
 * chuỗi không được chứa" — đúng nhưng muộn. Những ca ở đây bắt cùng một lỗi sớm hơn một tầng, và
 * nói thẳng ra nguyên nhân:
 *
 *   • bản rỗng thiếu một export mà bản thật có  ⇒  bản `goc` vỡ lúc dựng;
 *   • một tệp nhập thẳng vào trong lớp khám phá ⇒  alias bị đi vòng, bản `goc` vẫn dựng xanh và
 *     vẫn mang theo tám tên đơn vị hành chính đặt ra vào bản gửi Zalo duyệt.
 *
 * Lỗi thứ hai là lỗi không có triệu chứng nào cả — đó là lý do nó có một ca riêng.
 */

describe("bản rỗng khai đúng bề mặt của bản đầy đủ", () => {
  it("xuất đúng những cái tên bản thật xuất, không thiếu không thừa", () => {
    // Thiếu một tên thì bản `goc` vỡ lúc dựng với một thông báo về mô-đun, không về nguyên nhân.
    // Thừa một tên thì có thứ chỉ tồn tại ở bản `goc` — tức hai app khác nhau, không phải hai
    // biến thể của một app.
    expect(Object.keys(Rong).sort()).toEqual(Object.keys(DayDu).sort());
    expect(Object.keys(ChanDoanRong).sort()).toEqual(Object.keys(ChanDoanDayDu).sort());
  });

  it("nói ra rằng lớp khám phá KHÔNG có mặt", () => {
    // `App.tsx` đọc cờ này để không bước vào nhánh khám phá. Sai cờ thì một đường liên kết có
    // tham số `t` mở ra một màn hình trắng — thứ người duyệt của Zalo thấy trước tiên.
    expect(DayDu.CO_LOP_KHAM_PHA).toBe(true);
    expect(Rong.CO_LOP_KHAM_PHA).toBe(false);
  });

  it("không gợi ý xã nào, và ba màn đều vẽ ra rỗng", () => {
    expect(Rong.phanGiaiGoiY("01JDEMXA00000000000000000A", "qr")).toEqual({
      kieu: "phai-chon",
      li_do: "khong-tra-duoc",
    });

    const rong = (phan_tu: Parameters<typeof renderToStaticMarkup>[0]) =>
      renderToStaticMarkup(phan_tu);
    expect(rong(createElement(Rong.TrangXaScreen, { xa: undefined as never }))).toBe("");
    expect(
      rong(
        createElement(Rong.ChonXaScreen, {
          li_do: null,
          onChon: () => {},
          onXemGioiThieu: () => {},
        }),
      ),
    ).toBe("");
    expect(
      rong(
        createElement(Rong.GoiYXaScreen, {
          xa: undefined as never,
          nguon: "qr",
          onXacNhan: () => {},
          onChonXaKhac: () => {},
        }),
      ),
    ).toBe("");
    expect(rong(createElement(ChanDoanRong.LaunchParamsPanel, { thamSo: undefined as never }))).toBe(
      "",
    );
  });
});

describe("alias là cửa DUY NHẤT vào hai phần bị gỡ ở bản gốc", () => {
  const RAW = import.meta.glob("../../**/*.{ts,tsx}", {
    query: "?raw",
    import: "default",
    eager: true,
  }) as Record<string, string>;

  const khongChuThich = (ma: string) =>
    ma.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");

  /** Tệp NGOÀI hai thư mục bị gỡ. Bên trong chúng thì nhập thẳng nhau là chuyện bình thường. */
  const NGOAI = (duong: string) =>
    !duong.startsWith("./") && !duong.includes("/diagnostics/") && !duong.includes(".test.");

  it("quét đúng cây mã thật — một lượt quét rỗng cũng xanh, và xanh sai lý do", () => {
    const duong = Object.keys(RAW).filter(NGOAI);
    expect(duong).toContain("../../App.tsx");
    expect(duong.length).toBeGreaterThanOrEqual(5);
  });

  it("không tệp nào ngoài hai thư mục ấy nhập thẳng vào trong chúng", () => {
    // Đây là lỗi KHÔNG CÓ TRIỆU CHỨNG: `import { ChonXaScreen } from "./features/kham-pha/…"`
    // đi vòng qua `resolve.alias`, nên bản `goc` vẫn dựng xanh, vẫn chạy, và vẫn mang theo danh
    // mục xã mẫu vào bản gửi duyệt. Cấm theo CHUỖI ĐƯỜNG DẪN nên không cú pháp nhập nào lách
    // được — `import`, `await import`, `require` đều phải gõ đúng cái đường dẫn ấy ra.
    const vi_pham: string[] = [];
    for (const [duong, ma] of Object.entries(RAW)) {
      if (!NGOAI(duong)) continue;
      for (const khop of khongChuThich(ma).matchAll(/["'`]([^"'`\n]*)["'`]/g)) {
        const chuoi = khop[1] ?? "";
        if (/features\/(kham-pha|diagnostics)\//.test(chuoi)) vi_pham.push(`${duong}: ${chuoi}`);
      }
    }
    expect(
      vi_pham,
      "một tệp nhập thẳng vào lớp khám phá hoặc bảng chẩn đoán, không qua `bien-the/…`.\n" +
        "Alias trong vite.config.ts chỉ thay được đúng hai cái tên ấy; mọi đường khác lọt vào " +
        "bản GỐC — bản gửi Zalo duyệt.",
    ).toEqual([]);
  });
});
