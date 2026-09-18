/// <reference types="vite/client" />
import { beforeAll, describe, expect, it } from "vitest";
import { build } from "vite";

import appConfigRaw from "../app-config.json?raw";
import indexHtml from "../index.html?raw";
import { BRAND_NAVY, COMPANY } from "./content/company-profile";
import { MAN_GIOI_THIEU } from "./features/company-intro/screens";
import { MAN_QUYEN } from "./features/quyen/index";
import {
  CHI_HIEN_LEN_MAN_HINH,
  DAN_NHAP_QUYEN,
  MA_RONG,
  NOI_DUNG_QUYEN,
  QR_RONG,
  TOKEN_KHONG_CHUA_GI,
} from "./features/quyen/noi-dung";
import {
  DEMO_DANH_MUC_XA,
  DEMO_GHI_CHU,
  DEMO_GHI_CHU_TRANG_XA,
  DEMO_TEN_DICH_VU,
} from "./features/kham-pha/demo-danh-muc-xa";
import { LOI_NHAN, nhanNguon } from "./features/kham-pha/goi-y";
import { LOI_NHAN_DANG_LAM } from "./features/kham-pha/TrangXaScreen";

/**
 * WHAT THIS CATCHES THAT NOTHING ELSE DOES:
 *
 *   `base: "./"` in vite.config.ts is the line between a working Mini App and a WHITE SCREEN on
 *   a real phone. A Mini App is served from a platform-chosen path, so an absolute `/assets/…`
 *   URL resolves to nothing. The cruel part: `vite preview`, `npm run dev`, `tsc` and every
 *   other test in this repository stay perfectly green when that line is removed, because they
 *   all serve from a root. The failure appears for the first time on a reviewer's device.
 *
 *   So this test builds the bundle FOR REAL — the same config `npm run build` uses — and reads
 *   the asset URLs that were actually emitted. It is one end-to-end check instead of five unit
 *   tests asserting the config object back at itself.
 *
 * The build runs in memory (`write: false`): nothing is written to `dist/`, and the config is
 * picked up from the package root the way `npm run build` picks it up.
 */

type EmittedFile = { type: string; fileName: string; source?: unknown; code?: unknown };

/**
 * `process` khai tại chỗ, đúng phần dùng tới.
 *
 * Gói này không có `@types/node` — nó chạy trong trình duyệt, và một biến môi trường đọc trong
 * hai dòng test không phải lý do để kéo bộ khai báo của Node vào một app gửi Zalo duyệt. Cùng lý
 * do với cách `accessibility.test.ts` nhập `node:fs`.
 */
declare const process: { env: Record<string, string | undefined> };

/**
 * Dựng MỘT biến thể, trong bộ nhớ, qua đúng `vite.config.ts` mà `npm run build` dùng.
 *
 * Biến thể được chọn bằng `VIGOV_BIEN_THE` — cùng đường mà `scripts/dung.mjs` đi, chứ không phải
 * một cấu hình riêng dựng ra cho test. Một phép kiểm dựng bằng cấu hình của chính nó thì xanh mà
 * không nói gì về thứ sẽ được đẩy lên Zalo.
 */
async function dungBienThe(bien_the: "goc" | "quyen" | "day-du"): Promise<EmittedFile[]> {
  const truoc = process.env["VIGOV_BIEN_THE"];
  process.env["VIGOV_BIEN_THE"] = bien_the;
  try {
    const result = await build({ logLevel: "silent", build: { write: false } });
    const outputs = (Array.isArray(result) ? result : [result]) as unknown as Array<{
      output: EmittedFile[];
    }>;
    return outputs.flatMap((output) => output.output);
  } finally {
    if (truoc === undefined) delete process.env["VIGOV_BIEN_THE"];
    else process.env["VIGOV_BIEN_THE"] = truoc;
  }
}

/**
 * Toàn văn mọi tệp phát ra, đã giải mã `\uXXXX`.
 *
 * Bộ rút gọn có quyền thoát ký tự ngoài ASCII, và tiếng Việt thì toàn ký tự ngoài ASCII. Không
 * giải mã thì phép tìm "Xã …" trong bundle **không khớp gì cả** và ca kiểm xanh vì lý do sai —
 * đúng cái chế độ hỏng mà ca kiểm ấy sinh ra để chặn.
 */
const toanVan = (tep: EmittedFile[]) =>
  tep
    .map((f) => String(f.code ?? f.source ?? ""))
    .join("\n")
    .replace(/\\u([0-9a-fA-F]{4})/g, (_, ma) => String.fromCharCode(Number.parseInt(ma, 16)));

let emitted: EmittedFile[] = [];

beforeAll(async () => {
  emitted = await dungBienThe("day-du");
}, 120_000);

const builtHtml = () => {
  const html = emitted.find((file) => file.fileName === "index.html");
  expect(html, "the build emitted no index.html").toBeDefined();
  return String(html?.source);
};

describe("the bundle that gets uploaded to Zalo", () => {
  it("emits an entry document and at least one asset", () => {
    expect(emitted.map((file) => file.fileName)).toContain("index.html");
    expect(emitted.length).toBeGreaterThan(1);
  });

  it("references every asset relatively — an absolute path is a white screen on a device", () => {
    const references = [...builtHtml().matchAll(/(?:src|href)="([^"]+)"/g)].map((match) => match[1]!);
    const assetReferences = references.filter((reference) => reference.includes("assets/"));
    expect(assetReferences.length, "the built page loads no bundled asset at all").toBeGreaterThan(0);
    for (const reference of assetReferences) {
      expect(reference, `asset served from an absolute path: ${reference}`).toMatch(/^\.\//);
    }
  });

  it("ships no source map — the source stays off a device we do not control", () => {
    const maps = emitted.map((file) => file.fileName).filter((name) => name.endsWith(".map"));
    expect(maps).toEqual([]);
  });

  it("keeps the mount point and the Vietnamese language tag in the built page", () => {
    // `lang="vi"` is what makes a screen reader pronounce these screens as Vietnamese instead of
    // spelling them out as English. It survives the build or it helps nobody.
    expect(builtHtml()).toContain('lang="vi"');
    expect(builtHtml()).toContain('id="app"');
  });

  it("loads the app through a CLASSIC script — a module tag is dropped in silence", () => {
    // ĐÃ THỬ, KHÔNG SUY ĐOÁN. `zmp-cli sync-config` đọc trang này để điền app-config.json, và
    // Zalo chỉ nạp những tệp khai trong đó. Chạy trên hai bản HTML khác nhau đúng một chỗ:
    //
    //   <script type="module" crossorigin src=…>  ->  listSyncJS: ["inline.js"]
    //   <script src=…>                            ->  listSyncJS: ["inline.js", "./assets/app.js"]
    //
    // Bản trên thiếu chính bundle của ứng dụng. Không có lỗi nào: sync-config báo thành công,
    // `vite preview` chạy hoàn hảo, và app trắng trơn trên máy thật.
    expect(builtHtml()).not.toContain('type="module"');
    expect(builtHtml()).toMatch(/<script\s+src="\.\/assets\/app\.js"><\/script>/);
  });

  it("emits stable file names — a content hash makes the committed config wrong", () => {
    // app-config.json được COMMIT và liệt kê đường dẫn bundle. Nếu tên tệp mang mã băm thì tệp
    // ấy sai ngay lần dựng kế tiếp, và sai theo kiểu không có gì báo — app vẫn dựng xanh, chỉ là
    // Zalo đi nạp một tệp không còn tồn tại.
    const ten = emitted.map((f) => f.fileName).filter((n) => n.endsWith(".js"));
    expect(ten).toContain("assets/app.js");
    for (const n of ten) expect(n).not.toMatch(/-[A-Za-z0-9_-]{8}\.js$/);
  });
});

/**
 * BA BIẾN THỂ — và đây là ca kiểm duy nhất biến "bản nộp sạch" từ lời hứa thành sự thật đo được.
 *
 * | Biến thể | Nội dung | Dùng để |
 * |---|---|---|
 * | `goc` | Chỉ app giới thiệu bốn màn | Bản nộp tối thiểu |
 * | `quyen` | Thêm ba màn quyền + `zmp-sdk` | **Bản nộp XIN QUYỀN** |
 * | `day-du` | Thêm lớp khám phá + danh mục xã mẫu + bảng chẩn đoán | Demo nội bộ |
 *
 * Nó dựng THẬT cả hai biến thể rồi đọc bundle, vì không có cách nào khác nói chắc: `resolve.alias`
 * có thể bị một `import` thẳng đi vòng qua, một tệp có thể được nhập lại từ một đường khác, và
 * mọi phép kiểm còn lại của kho này vẫn xanh trong cả hai trường hợp. `grep` trên bundle là câu
 * trả lời cuối cùng.
 *
 * VÌ SAO CA "BẢN ĐẦY ĐỦ CÓ CHỨA" LẠI QUAN TRỌNG NGANG CA "BẢN GỐC KHÔNG CHỨA":
 *
 *   Không có nó, đổi tên một xã trong danh mục là đủ để mọi ca dưới xanh vĩnh viễn — bản gốc
 *   không chứa "Xã An Thịnh" vì **không bản nào** chứa nó nữa. Một phép kiểm xanh vì không tìm
 *   thấy gì là một phép kiểm đã chết mà không ai được báo.
 */
describe("ba biến thể — mỗi bản đúng bằng thứ người duyệt đọc", () => {
  let goc = "";
  let quyen = "";
  let day_du = "";

  beforeAll(async () => {
    goc = toanVan(await dungBienThe("goc"));
    quyen = toanVan(await dungBienThe("quyen"));
    day_du = toanVan(await dungBienThe("day-du"));
  }, 360_000);

  /**
   * Chuỗi của BA MÀN QUYỀN — dùng lại đúng nguồn mà màn hình đọc (`features/quyen/noi-dung.ts`),
   * không chép tay. Một danh sách chép tay sẽ lệch khi ai đó sửa một câu, và lúc nó lệch thì mọi
   * ca "bản goc không chứa" xanh vì **không tìm thấy gì**, chứ không vì bản `goc` sạch.
   */
  const CHUOI_MAN_QUYEN = () => [
    MAN_QUYEN.tabLabel,
    MAN_QUYEN.headerTitle,
    DAN_NHAP_QUYEN,
    CHI_HIEN_LEN_MAN_HINH,
    MA_RONG,
    QR_RONG,
    TOKEN_KHONG_CHUA_GI["so-dien-thoai"],
    TOKEN_KHONG_CHUA_GI["vi-tri"],
    "Đã nhận được mã (token) từ Zalo.",
    "Nội dung quét được:",
    ...NOI_DUNG_QUYEN.flatMap((nd) => [
      nd.nhan_chon,
      nd.tieu_de,
      nd.vi_sao,
      nd.nut,
      nd.dang_cho,
      nd.tu_choi,
      nd.ngoai_zalo,
      nd.khong_lay_duoc,
    ]),
  ];

  const CHUOI_LOP_KHAM_PHA = () => [
    DEMO_GHI_CHU,
    DEMO_GHI_CHU_TRANG_XA,
    LOI_NHAN_DANG_LAM,
    LOI_NHAN["nguon-yeu"],
    LOI_NHAN["khong-tra-duoc"],
    nhanNguon("qr"),
    nhanNguon("zns"),
    "Chọn xã để tiếp tục",
    "Bạn cần liên hệ với xã này?",
    "Chọn xã khác",
    "Dịch vụ của xã",
    "Chưa mở",
    ...Object.values(DEMO_TEN_DICH_VU),
  ];

  it("đo đúng thứ cần đo — bản ĐẦY ĐỦ có chứa tất cả những thứ ấy", () => {
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(day_du, `bản đầy đủ thiếu ${xa.ten}`).toContain(xa.ten);
    }
    for (const chuoi of CHUOI_LOP_KHAM_PHA()) {
      expect(day_du, `bản đầy đủ thiếu: ${chuoi}`).toContain(chuoi);
    }
  });

  it("đo đúng thứ cần đo — bản QUYỀN có chứa đủ chữ của ba màn quyền", () => {
    // CA NÀY QUAN TRỌNG NGANG CA "BẢN GỐC KHÔNG CHỨA". Thiếu nó thì đổi một chữ trong ba màn
    // quyền là đủ để mọi ca dưới xanh vĩnh viễn: bản `goc` không chứa câu ấy vì **không bản nào**
    // chứa nó nữa. Một phép kiểm xanh vì không tìm thấy gì là một phép kiểm đã chết trong im lặng.
    for (const chuoi of CHUOI_MAN_QUYEN()) {
      expect(quyen, `bản quyền thiếu: ${chuoi}`).toContain(chuoi);
    }
  });

  it("không chứa MỘT tên đơn vị hành chính nào của danh mục mẫu", () => {
    // Tám tên này là tên ĐẶT RA. Một cơ quan nhà nước hiển thị đơn vị hành chính không tồn tại
    // là một sự cố, không phải một lỗi giao diện — và bản này là bản người duyệt đọc.
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(goc, `bản gốc vẫn chứa ${xa.ten}`).not.toContain(xa.ten);
      expect(goc, `bản gốc vẫn chứa tỉnh/thành đặt ra: ${xa.tinh}`).not.toContain(xa.tinh);
      expect(goc, `bản gốc vẫn chứa mã xã ${xa.id}`).not.toContain(xa.id);
      expect(goc.replace(/\s/g, ""), `bản gốc vẫn chứa số trực mẫu`).not.toContain(
        xa.dien_thoai_truc,
      );
    }
  });

  it("không chứa chuỗi nào của màn chọn xã, màn xác nhận xã hay trang xã", () => {
    for (const chuoi of CHUOI_LOP_KHAM_PHA()) {
      expect(goc, `bản gốc vẫn chứa: ${chuoi}`).not.toContain(chuoi);
    }
  });

  it("không chứa chữ nào của ba màn quyền, và không chứa `zmp-sdk`", () => {
    // Bản `goc` là bản nộp TỐI THIỂU: một app không xin quyền nào. Có sẵn `getPhoneNumber` trong
    // bundle của một bản nộp như thế là một bản nộp tự mâu thuẫn — và 256 kB trả cho một thứ
    // không màn nào gọi tới.
    for (const chuoi of CHUOI_MAN_QUYEN()) {
      expect(goc, `bản gốc vẫn chứa: ${chuoi}`).not.toContain(chuoi);
    }
    expect(goc, "bản gốc vẫn kéo theo zmp-sdk").not.toContain("zmp-sdk");
    expect(goc).not.toContain("getPhoneNumber");
    expect(goc).not.toContain("scanQRCode");
  });

  it("bản QUYỀN thật sự mang `zmp-sdk` — nếu không thì ba màn ấy không xin được gì", () => {
    // Mặt kia của ca trên. Ba màn quyền không nhập SDK thì bản nộp xin quyền là một bản nộp có
    // ba cái nút không gọi tới nền tảng, và vòng duyệt không có gì để cấp quyền cho.
    expect(quyen, "bản quyền không mang zmp-sdk").toContain("zmp-sdk");
    expect(quyen).toContain("getPhoneNumber");
    expect(quyen).toContain("scanQRCode");
  });

  it("bản QUYỀN là một bản NỘP: không xã mẫu, không số trực mẫu, không bảng chẩn đoán", () => {
    // Đây là toàn bộ lý do `quyen` là một biến thể riêng chứ không phải `day-du`. Nếu nó mang
    // theo tám tên đơn vị hành chính ĐẶT RA thì bản nộp xin quyền lại thành bản nộp có dữ liệu
    // bịa dưới tên một cơ quan nhà nước.
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(quyen, `bản quyền vẫn chứa ${xa.ten}`).not.toContain(xa.ten);
      expect(quyen, `bản quyền vẫn chứa tỉnh/thành đặt ra: ${xa.tinh}`).not.toContain(xa.tinh);
      expect(quyen, `bản quyền vẫn chứa mã xã ${xa.id}`).not.toContain(xa.id);
      expect(quyen.replace(/\s/g, ""), "bản quyền vẫn chứa số trực mẫu").not.toContain(
        xa.dien_thoai_truc,
      );
    }
    for (const chuoi of CHUOI_LOP_KHAM_PHA()) {
      expect(quyen, `bản quyền vẫn chứa: ${chuoi}`).not.toContain(chuoi);
    }
    expect(quyen).not.toContain("Chẩn đoán tham số mở app");
    expect(quyen).not.toContain("URL Zalo dùng để mở app");
  });

  it("không chứa bảng chẩn đoán", () => {
    // Một bảng in tham số nội bộ nằm trong bundle của một đơn vị đang xin duyệt là một câu hỏi
    // phải trả lời ở vòng duyệt, và "đó là công cụ nội bộ" không giúp được gì ở vòng đó.
    expect(goc).not.toContain("Chẩn đoán tham số mở app");
    expect(goc).not.toContain("URL Zalo dùng để mở app");
  });

  it("nói ra thứ bản gốc VẪN mang: hai nhãn của vỏ, ở nhánh không bao giờ chạy", () => {
    // KHÔNG phải một ngoại lệ được tha — là một phép đo được ghi lại, để lần sau `grep` thấy
    // "Chọn xã" trong bản gốc thì đọc được ngay tại sao, thay vì tưởng alias đã thủng.
    //
    // `App.tsx` là VỎ, không nằm sau alias: nó giữ nhánh `props.khamPha ? "Chọn xã" : …` và nút
    // "Đổi xã". Ở bản gốc, mô-đun khám phá rỗng nên `khamPha` luôn false — nhánh chết, nhãn còn.
    //
    // Vì sao để nguyên: hai nhãn ấy là từ ngữ hành chính thông thường, không phải đơn vị hành
    // chính ĐẶT RA, không phải dữ liệu cá nhân, không vẽ ra màn nào. Tách `App.tsx` thành hai
    // biến thể để gỡ chúng là thêm một đường rẽ nữa vào đúng tệp mà cả hai bản cùng đi qua —
    // đắt hơn thứ nó mua. Ranh giới là DỮ LIỆU, không phải từ vựng.
    expect(goc).toContain("Chọn xã");
    expect(goc).toContain("Đổi xã");

    // VÀ THỨ THỨ BA, ĐO ĐƯỢC: các LỚP CSS của ba màn quyền — khoảng 2,2 kB sau khi rút gọn.
    //
    // `src/styles.css` là MỘT tệp, `main.tsx` nạp trọn, và `main.tsx` không nằm sau alias. Tách
    // biểu mẫu kiểu theo biến thể là dựng **cơ chế thứ hai** cạnh `resolve.alias` cho 2,2 kB —
    // đắt hơn thứ nó mua, và một cơ chế thứ hai là chỗ lần sau người ta quên đồng bộ.
    //
    // Vì sao để nguyên là đúng: ranh giới là DỮ LIỆU, không phải từ vựng. Trong 2,2 kB ấy không
    // có một câu giải thích nào, không tên đơn vị hành chính, không số điện thoại — chỉ tên lớp
    // và thuộc tính. Chúng không vẽ ra gì cả, vì không màn nào ở bản `goc` mang các lớp ấy.
    expect(goc, "các lớp CSS của màn quyền đã biến mất — đọc lại chú thích này").toContain(
      ".quyen__nut",
    );
    expect(goc).toContain(".quyen-khu__nut");
  });

  it("vẫn ĐÚNG BẰNG app giới thiệu bốn màn — không phải một bản rỗng", () => {
    // Gỡ nhầm tay thì bản gốc cũng "không chứa tên xã nào", và mọi ca trên vẫn xanh. Ca này là
    // thứ phân biệt "đã gỡ đúng phần thừa" với "đã gỡ mất app".
    expect(goc).toContain(COMPANY.name);
    // `MAN_GIOI_THIEU`, không phải `SCREENS`: `SCREENS` của tiến trình test là sổ màn hình của
    // biến thể ĐẦY ĐỦ, nên nó có thêm tab quyền — và tab ấy đúng là không được có trong bản gốc.
    for (const man of MAN_GIOI_THIEU) {
      expect(goc, `bản gốc thiếu màn ${man.id}`).toContain(man.tabLabel);
    }
    // Bốn màn ấy cũng phải còn nguyên trong bản NỘP XIN QUYỀN: ba màn quyền là phần THÊM, không
    // phải phần thay thế. Một app chỉ có ba nút xin quyền là một app không có nội dung để duyệt.
    expect(quyen).toContain(COMPANY.name);
    for (const man of MAN_GIOI_THIEU) {
      expect(quyen, `bản quyền thiếu màn ${man.id}`).toContain(man.tabLabel);
    }
  });

  it("nhẹ hơn hai bản kia — bằng chứng rằng mã thật sự biến mất, không chỉ bị giấu", () => {
    expect(goc.length).toBeLessThan(quyen.length);
    expect(quyen.length).toBeLessThan(day_du.length);
  });

  it("tên biến thể sai thì DỪNG, không dựng bằng mặc định", () => {
    // Một cái tên gõ nhầm mà vẫn dựng tiếp nghĩa là dựng bản ĐẦY ĐỦ rồi đem nộp dưới nhãn khác —
    // hỏng trong im lặng, đúng chỗ đắt nhất. Ca này gọi thẳng `vite build` với một tên sai.
    return expect(dungBienThe("gôc" as "goc")).rejects.toThrow(/VIGOV_BIEN_THE/);
  });
});

describe("what the submission says the app is called", () => {
  // Parsed inside each test on purpose: a syntax error then fails the test that is about syntax,
  // with the name of that test, instead of crashing the whole file at import time.
  const appConfig = () => JSON.parse(appConfigRaw) as { app?: { title?: string; headerColor?: string } };

  it("parses as JSON — a trailing comma here is a submission sent back", () => {
    // Nothing else reads this file: the build ignores it and the compiler never sees it. A typo
    // survives every other check in this package and surfaces at the upload.
    expect(appConfig().app).toBeDefined();
  });

  it("names the publishing entity in the native header and the page title", () => {
    expect(appConfig().app?.title).toBe(COMPANY.name);
    expect(builtHtml()).toContain(`<title>${COMPANY.name}</title>`);
  });

  it("uses one brand navy in all three places it is written down", () => {
    // The native Zalo header (app-config.json), the browser theme colour (index.html) and the
    // stylesheet (--navy, read out of the CSS the build actually emitted) meet at a visible
    // seam: the platform header sits directly above the app header. Two of three updated is a
    // two-tone bar that looks like a rendering bug in a government-adjacent app.
    expect(appConfig().app?.headerColor?.toLowerCase()).toBe(BRAND_NAVY);
    expect(indexHtml).toContain(`content="${BRAND_NAVY}"`);

    // Tìm trong MỌI tệp phát ra, không riêng tệp `.css`: từ khi bundle chuyển sang một tệp
    // `iife` duy nhất (xem vite.config.ts), Vite nhét CSS thẳng vào JS và không còn tệp `.css`
    // nào cả. Ghim vào đuôi tệp thì test đỏ mỗi lần đổi hình dạng bundle vì một lý do chẳng liên
    // quan gì tới màu — và một test đỏ sai lý do là một test sắp bị ai đó xoá.
    // Rollup để nội dung của một chunk JS ở `code`; chỉ asset mới dùng `source`.
    const noi_dung = emitted.map((file) => String(file.code ?? file.source ?? "")).join("\n");
    const navy = /--navy:\s*([^;}]+)/.exec(noi_dung)?.[1]?.trim().toLowerCase();
    expect(navy, "bản dựng không chứa biến --navy ở đâu cả").toBe(BRAND_NAVY);
  });
});
