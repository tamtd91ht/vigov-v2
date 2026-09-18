/// <reference types="vite/client" />
import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import { describe, expect, it } from "vitest";

import viteConfigRaw from "../../../vite.config.ts?raw";
import dungRaw from "../../../scripts/dung.mjs?raw";
import * as ChanDoanDayDu from "../diagnostics/index";
import * as ChanDoanRong from "../diagnostics/index.rong";
import * as QuyenDayDu from "../quyen/index";
import * as QuyenRong from "../quyen/index.rong";
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
    expect(Object.keys(QuyenRong).sort()).toEqual(Object.keys(QuyenDayDu).sort());
  });

  it("nói ra rằng lớp khám phá KHÔNG có mặt", () => {
    // `App.tsx` đọc cờ này để không bước vào nhánh khám phá. Sai cờ thì một đường liên kết có
    // tham số `t` mở ra một màn hình trắng — thứ người duyệt của Zalo thấy trước tiên.
    expect(DayDu.CO_LOP_KHAM_PHA).toBe(true);
    expect(Rong.CO_LOP_KHAM_PHA).toBe(false);
  });

  it("nói ra rằng ba màn quyền KHÔNG có mặt, và không mang theo nhãn tab nào", () => {
    // `screens.ts` đọc cờ này để KHÔNG thêm tab thứ năm. Sai cờ thì bản `goc` có một tab dẫn tới
    // một màn trống — và bản `goc` là bản nộp.
    expect(QuyenDayDu.CO_MAN_QUYEN).toBe(true);
    expect(QuyenRong.CO_MAN_QUYEN).toBe(false);

    // Nhãn rỗng, chứ không phải nhãn thật: chuỗi trong bản rỗng đi thẳng vào bundle bản `goc`.
    expect(QuyenDayDu.MAN_QUYEN.tabLabel.length).toBeGreaterThan(0);
    expect(QuyenRong.MAN_QUYEN.tabLabel).toBe("");
    expect(QuyenRong.MAN_QUYEN.headerTitle).toBe("");
    // Cùng `id`, vì `ScreenId` là một kiểu chung cho cả ba biến thể.
    expect(QuyenRong.MAN_QUYEN.id).toBe(QuyenDayDu.MAN_QUYEN.id);
  });

  it("khu vực quyền ở bản rỗng vẽ ra rỗng", () => {
    // Bản đầy đủ vẽ ra thứ gì thì `quyen.test.tsx` kiểm; ở đây chỉ cần bản rỗng không vẽ gì —
    // nếu nó vẽ, bản `goc` có một màn hình không ai định đưa vào bản nộp.
    expect(renderToStaticMarkup(createElement(QuyenRong.MAN_QUYEN.component, {}))).toBe("");
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

describe("alias là cửa DUY NHẤT vào ba phần gỡ được", () => {
  const RAW = import.meta.glob("../../**/*.{ts,tsx}", {
    query: "?raw",
    import: "default",
    eager: true,
  }) as Record<string, string>;

  const khongChuThich = (ma: string) =>
    ma.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");

  /** Tệp NGOÀI ba thư mục gỡ được. Bên trong chúng thì nhập thẳng nhau là chuyện bình thường. */
  const NGOAI = (duong: string) =>
    !duong.startsWith("./") &&
    !duong.includes("/diagnostics/") &&
    !duong.includes("/quyen/") &&
    !duong.includes(".test.");

  it("quét đúng cây mã thật — một lượt quét rỗng cũng xanh, và xanh sai lý do", () => {
    const duong = Object.keys(RAW).filter(NGOAI);
    expect(duong).toContain("../../App.tsx");
    expect(duong.length).toBeGreaterThanOrEqual(5);
  });

  it("không tệp nào ngoài ba thư mục ấy nhập thẳng vào trong chúng", () => {
    // Đây là lỗi KHÔNG CÓ TRIỆU CHỨNG: `import { ChonXaScreen } from "./features/kham-pha/…"`
    // đi vòng qua `resolve.alias`, nên bản `goc` vẫn dựng xanh, vẫn chạy, và vẫn mang theo danh
    // mục xã mẫu vào bản gửi duyệt. Cấm theo CHUỖI ĐƯỜNG DẪN nên không cú pháp nhập nào lách
    // được — `import`, `await import`, `require` đều phải gõ đúng cái đường dẫn ấy ra.
    const vi_pham: string[] = [];
    for (const [duong, ma] of Object.entries(RAW)) {
      if (!NGOAI(duong)) continue;
      for (const khop of khongChuThich(ma).matchAll(/["'`]([^"'`\n]*)["'`]/g)) {
        const chuoi = khop[1] ?? "";
        if (/features\/(kham-pha|diagnostics|quyen)\//.test(chuoi)) vi_pham.push(`${duong}: ${chuoi}`);
      }
    }
    expect(
      vi_pham,
      "một tệp nhập thẳng vào lớp khám phá, bảng chẩn đoán hoặc ba màn quyền, không qua " +
        "`bien-the/…`.\nAlias trong vite.config.ts chỉ thay được đúng ba cái tên ấy; mọi đường " +
        "khác lọt vào bản GỐC — bản nộp tối thiểu.",
    ).toEqual([]);
  });
});

/**
 * MỘT DANH SÁCH BIẾN THỂ, HAI TỆP GIỮ NÓ — và đây là ca kiểm giữ cho chúng không lệch.
 *
 * `vite.config.ts` là nơi quyết định alias, `scripts/dung.mjs` là nơi người chạy lệnh gõ tên.
 * Thêm một biến thể vào một tệp mà quên tệp kia thì hỏng theo hai kiểu, cả hai đều im lặng:
 *
 *   • thiếu ở `dung.mjs`  ⇒  `npm run build:<tên>` báo "biến thể không có", dễ thấy;
 *   • thiếu ở `vite.config.ts` ⇒ **dựng tiếp bằng mặc định `day-du`** nếu ai đó lỡ bỏ phép kiểm
 *     ở đó — tức đem bản demo nội bộ đi nộp dưới nhãn một bản khác.
 *
 * Đọc bằng `?raw` chứ không `import` hai tệp ấy: `vite.config.ts` nằm ngoài `tsconfig.include`
 * và nhập nó vào một test sẽ kéo cả nó vào chương trình `tsc` — nơi không có `@types/node`.
 */
describe("danh sách biến thể — một sự thật, hai tệp phải nói giống nhau", () => {
  const danhSach = (ma: string, mau: RegExp): string[] => {
    const khop = mau.exec(ma);
    expect(khop, "không tìm thấy danh sách biến thể trong tệp").not.toBeNull();
    return [...khop![1]!.matchAll(/["']([^"']+)["']/g)].map((m) => m[1]!);
  };

  const CUA_VITE = () => danhSach(viteConfigRaw, /const BIEN_THE_HOP_LE = \[([^\]]*)\]/);
  const CUA_DUNG = () => danhSach(dungRaw, /export const BIEN_THE = \[([^\]]*)\]/);

  it("cùng ba tên, cùng thứ tự", () => {
    expect(CUA_VITE()).toEqual(["goc", "quyen", "day-du"]);
    expect(CUA_DUNG()).toEqual(CUA_VITE());
  });

  it("`quyen` là một biến thể hợp lệ ở cả hai nơi — đây là bản nộp xin quyền", () => {
    expect(CUA_VITE()).toContain("quyen");
    expect(CUA_DUNG()).toContain("quyen");
  });

  it("mỗi biến thể có một dòng mô tả — `zmp deploy` in nó ra trước khi đẩy", () => {
    // `deploy.mjs` in `MO_TA_BIEN_THE[bien_the]` ra trước khi dựng. Thiếu một khoá thì dòng ấy
    // in ra `undefined` đúng lúc người chạy đang đọc để quyết định có phát hành hay không.
    const mo_ta = /export const MO_TA_BIEN_THE = \{([\s\S]*?)\n\};/.exec(dungRaw)?.[1] ?? "";
    for (const ten of CUA_DUNG()) {
      expect(mo_ta, `thiếu mô tả cho biến thể ${ten}`).toMatch(
        new RegExp(`(^|[\\s{])"?${ten}"?\\s*:`, "m"),
      );
    }
  });

  it("sai tên thì NÉM LỖI, không đoán — và cả hai tệp đều tự kiểm", () => {
    // Đọc chính mã của hai tệp: `vite.config.ts` ném `Error`, `dung.mjs` trả mã thoát khác 0.
    // Bản dựng thật của phép kiểm này nằm ở `bundle-for-zalo.test.ts` (dựng với một tên sai và
    // khẳng định lời hứa bị từ chối); ở đây chỉ ghim rằng nhánh ấy còn tồn tại trong mã.
    expect(viteConfigRaw).toMatch(/if \(!\(BIEN_THE_HOP_LE as readonly string\[\]\)\.includes\(dat\)\)/);
    expect(viteConfigRaw).toMatch(/throw new Error\(/);
    expect(dungRaw).toMatch(/if \(!BIEN_THE\.includes\(bien_the\)\)/);
    expect(dungRaw).toMatch(/return 2;/);
  });
});
