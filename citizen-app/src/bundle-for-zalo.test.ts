/// <reference types="vite/client" />
import { beforeAll, describe, expect, it } from "vitest";

import { build } from "vite";

import appConfigRaw from "../app-config.json?raw";
import indexHtml from "../index.html?raw";
import {
  CAU_DAU,
  MUC_CHINH_SACH,
  PHIEN_BAN_CHINH_SACH,
  TIEU_DE_CHINH_SACH,
} from "./content/chinh-sach-rieng-tu";
import {
  BRAND_NAVY,
  CAU_SAN_PHAM,
  COMPANY,
  NHAN_LOAI_HINH,
  SLOGAN_HERO,
} from "./content/company-profile";
import { DICH_MO_RA_NGOAI, DUONG_DAN_CHAT_OA } from "./content/dich-ra-ngoai";
import { NGAY_CHUP_TIN, TIN_VIHAT } from "./content/tin-tuc";
import { MUC_MENU_NHANH } from "./features/company-intro/MenuNhanh";
import { NHAN_NUT_CHAT } from "./features/company-intro/NutChatOA";
import { MAN_GIOI_THIEU, TABS } from "./features/company-intro/screens";
import { MAN_DANH_THIEP } from "./features/tinh-nang/index";
import {
  CHI_HIEN_LEN_MAN_HINH,
  DANG_NHAP,
  DANH_THIEP,
  DUONG_TRUYEN,
  KIEU_KET_NOI,
  LOI_MO_NGOAI,
  MA_RONG,
  NOI_DUNG_TINH_NANG,
  SO_HOA_THIEP,
  THIEP_CUA_CHUNG_TOI,
  TOKEN_KHONG_CHUA_GI,
  VAN_PHONG,
} from "./features/tinh-nang/noi-dung";
// HAI HỢP ĐỒNG, HAI HÀM CÙNG TÊN `thanYeuCau` — và chúng được ĐẶT BÍ DANH ở đây thay vì đổi tên
// một trong hai: mỗi hàm là "thân yêu cầu" của đúng tuyến nó phục vụ, và đổi tên để tiện cho một
// tệp test là để lại một cái tên không còn nói đúng việc ở hai tệp sản xuất.
import { DUONG_DAN_YEU_CAU, thanYeuCau } from "./api/hop-dong-yeu-cau";
import { DUONG_DAN_PHIEN, thanYeuCau as thanYeuCauPhien } from "./features/dang-nhap/hop-dong";
import {
  DEMO_DANH_MUC_XA,
  DEMO_GHI_CHU,
  DEMO_GHI_CHU_TRANG_XA,
  DEMO_TEN_DICH_VU,
} from "./features/kham-pha/demo-danh-muc-xa";
import { NHAN_KHAM_PHA } from "./features/kham-pha/index";
import { LOI_NHAN, nhanNguon } from "./features/kham-pha/goi-y";
import { LOI_NHAN_DANG_LAM } from "./features/kham-pha/TrangXaScreen";
import { diaChiViGov } from "./cong-dan/api/dia-chi-vigov";
import { DUONG_DAN_PHAN_ANH_CUA_TOI } from "./cong-dan/api/hop-dong-phan-anh";
import { NHAN_KENH_CONG_DAN } from "./cong-dan/man/KenhCongDan";
import { CUA_TOI, KENH_CHUA_MO, KHAN_CAP, TRA_CUU } from "./cong-dan/man/noi-dung";

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
async function dungBienThe(bien_the: "goc" | "day-du"): Promise<EmittedFile[]> {
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
 * giải mã thì phép tìm "cơ quan" trong bundle **không khớp gì cả** và ca kiểm xanh vì lý do sai —
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
 * HAI BIẾN THỂ — và đây là ca kiểm duy nhất biến "bản nộp đúng bằng thứ người duyệt đọc" từ lời
 * hứa thành sự thật đo được.
 *
 * | Biến thể | Nội dung | Dùng để |
 * |---|---|---|
 * | `goc` | Ứng dụng sản phẩm đầy đủ, gồm sáu tính năng dùng chín quyền nền tảng | **BẢN NỘP** |
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
describe("hai biến thể — mỗi bản đúng bằng thứ người duyệt đọc", () => {
  let goc = "";
  let day_du = "";

  beforeAll(async () => {
    goc = toanVan(await dungBienThe("goc"));
    day_du = toanVan(await dungBienThe("day-du"));
  }, 360_000);

  /**
   * CHỮ CỦA BA TÍNH NĂNG — dùng lại đúng nguồn mà màn hình đọc (`features/tinh-nang/noi-dung.ts`),
   * không chép tay. Một danh sách chép tay sẽ lệch khi ai đó sửa một câu, và lúc nó lệch thì ca
   * "bản nộp CÓ chứa" xanh vì **không tìm thấy gì**, chứ không vì bản nộp đúng.
   */
  const CHUOI_SAU_TINH_NANG = () => [
    // `tabLabel` không còn: màn Danh thiếp bỏ tab từ 21/09/2026 (khuya). Tiêu đề màn vẫn là chuỗi
    // người duyệt đọc, và hai nhãn menu nhanh dẫn tới nó được kiểm ở ca "bốn khối mới" bên dưới.
    MAN_DANH_THIEP.headerTitle,
    CHI_HIEN_LEN_MAN_HINH,
    MA_RONG,
    LOI_MO_NGOAI,
    TOKEN_KHONG_CHUA_GI["dang-nhap"],
    TOKEN_KHONG_CHUA_GI["van-phong"],
    ...Object.values(DANH_THIEP),
    ...Object.values(VAN_PHONG),
    ...Object.values(DANG_NHAP),
    // Ba tính năng thêm vào. `KIEU_KET_NOI` là một bản đồ lồng nhau, nên trải phẳng ra tường
    // minh: `Object.values` trên nó sẽ cho ra những đối tượng, và `toContain` trên một đối
    // tượng là một phép kiểm xanh vì lý do sai.
    ...Object.values(DUONG_TRUYEN),
    ...Object.values(THIEP_CUA_CHUNG_TOI),
    ...Object.values(SO_HOA_THIEP),
    ...Object.values(KIEU_KET_NOI).flatMap((mot) => [mot.nhan, mot.y_nghia]),
    ...NOI_DUNG_TINH_NANG.flatMap((nd) => [
      nd.nhan_ngan,
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
    NHAN_KHAM_PHA.tieu_de_chon_xa,
    NHAN_KHAM_PHA.nut_doi_xa,
    "Chọn xã để tiếp tục",
    "Bạn cần liên hệ với xã này?",
    "Chọn xã khác",
    "Dịch vụ của xã",
    "Chưa mở",
    ...Object.values(DEMO_TEN_DICH_VU),
  ];

  /**
   * DANH SÁCH TỪ CẤM — yêu cầu trực tiếp của người dùng, nên nó có một phép kiểm chứ không phải
   * một lời hứa.
   *
   *   Bản nộp là một **ứng dụng sản phẩm của ViHAT Group**, một doanh nghiệp công nghệ. Không
   *   một chi tiết nào trong nó được dính tới một cơ quan nhà nước: người đọc nó là khách hàng
   *   doanh nghiệp và người duyệt của Zalo, và một app của một công ty phần mềm mà nói chuyện
   *   "thủ tục" với "công dân" là một app không ai hiểu nổi nó bán gì.
   *
   * ⚠ MỘT CÁI BẪY ĐÃ TRÁNH, GHI LẠI ĐỂ KHÔNG AI ĐẶT LẠI: **không được cấm chuỗi "xã" trần**.
   * "xã hội" là một từ thường và nằm trong câu tầm nhìn thương hiệu của ViHAT Group
   * (`company-profile.ts`) — một câu đã công bố, không được sửa cho vừa một cái test. Nên danh
   * sách dưới chỉ có những cụm từ chỉ mang nghĩa hành chính công.
   */
  const TU_CAM = [
    "cơ quan",
    "công dân",
    "chính quyền",
    "hành chính",
    "thủ tục",
    NHAN_KHAM_PHA.tieu_de_chon_xa,
    NHAN_KHAM_PHA.nut_doi_xa,
  ];

  it("đo đúng thứ cần đo — bản NỘP có đủ chữ của sáu tính năng", () => {
    // CA NÀY QUAN TRỌNG NGANG MỌI CA "KHÔNG CHỨA" DƯỚI ĐÂY. Thiếu nó thì đổi một chữ trong ba
    // tính năng là đủ để mọi ca cấm xanh vĩnh viễn: bản `goc` không chứa câu ấy vì **không bản
    // nào** chứa nó nữa. Một phép kiểm xanh vì không tìm thấy gì là một phép kiểm đã chết trong
    // im lặng.
    const chuoi = CHUOI_SAU_TINH_NANG();
    expect(chuoi.length).toBeGreaterThan(20);
    for (const mot of chuoi) {
      expect(goc, `bản nộp thiếu: ${mot}`).toContain(mot);
    }
  });

  it("bản NỘP không chứa MỘT từ nào của ngữ cảnh cơ quan nhà nước", () => {
    for (const tu of TU_CAM) {
      const so_lan = goc.split(tu).length - 1;
      expect(so_lan, `bản nộp còn ${so_lan} lần "${tu}"`).toBe(0);
    }
  });

  it('nhưng "xã hội" — một từ thường trong câu tầm nhìn đã công bố — vẫn còn nguyên', () => {
    // Mặt kia của ca trên, và là thứ ngăn người sau "dọn sạch" bằng cách cấm chuỗi `xã`: câu tầm
    // nhìn của ViHAT Group là văn bản đã công bố dưới tên một pháp nhân, không phải chữ của ta.
    expect(goc, "câu tầm nhìn đã công bố bị sửa để vừa một phép kiểm").toContain("xã hội");
  });

  it("đo đúng thứ cần đo — bản ĐẦY ĐỦ có chứa cả lớp khám phá", () => {
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(day_du, `bản đầy đủ thiếu ${xa.ten}`).toContain(xa.ten);
    }
    for (const chuoi of CHUOI_LOP_KHAM_PHA()) {
      expect(day_du, `bản đầy đủ thiếu: ${chuoi}`).toContain(chuoi);
    }
    // Và bản đầy đủ là bản gốc CỘNG THÊM, không phải một app khác: sáu tính năng vẫn còn nguyên.
    for (const mot of CHUOI_SAU_TINH_NANG()) {
      expect(day_du, `bản đầy đủ thiếu: ${mot}`).toContain(mot);
    }
  });

  it("bản NỘP không chứa MỘT tên đơn vị hành chính nào của danh mục mẫu", () => {
    // Tám tên này là tên ĐẶT RA. Một ứng dụng công bố dưới tên một pháp nhân có thật mà hiển thị
    // những đơn vị không tồn tại là một sự cố, không phải một lỗi giao diện — và bản này là bản
    // người duyệt đọc.
    for (const xa of DEMO_DANH_MUC_XA) {
      expect(goc, `bản nộp vẫn chứa ${xa.ten}`).not.toContain(xa.ten);
      expect(goc, `bản nộp vẫn chứa tỉnh/thành đặt ra: ${xa.tinh}`).not.toContain(xa.tinh);
      expect(goc, `bản nộp vẫn chứa mã xã ${xa.id}`).not.toContain(xa.id);
      expect(goc.replace(/\s/g, ""), "bản nộp vẫn chứa số trực mẫu").not.toContain(
        xa.dien_thoai_truc,
      );
    }
  });

  it("bản NỘP không chứa chuỗi nào của lớp khám phá, kể cả hai nhãn của vỏ", () => {
    for (const chuoi of CHUOI_LOP_KHAM_PHA()) {
      // Bản rỗng trả chuỗi rỗng cho hai nhãn của vỏ; `""` thì `toContain` nào cũng đúng, nên
      // chúng được kiểm bằng danh sách từ cấm ở trên chứ không ở đây.
      if (chuoi === "") continue;
      expect(goc, `bản nộp vẫn chứa: ${chuoi}`).not.toContain(chuoi);
    }
  });

  /**
   * CẢ HAI BẢN ĐỀU GỌI MÁY CHỦ — VÀ PHÉP ĐO NÀY VỪA ĐƯỢC DỰNG LẠI VÌ TIỀN ĐỀ ĐÃ ĐẢO CHIỀU.
   *
   * ⚠ BẢN TRƯỚC CỦA CA NÀY ĐÃ CHẾT TRONG IM LẶNG NẾU KHÔNG SỬA — ghi lại vì đây là đúng chế độ
   * hỏng mà cả tệp này sinh ra để chặn, và lần này nó suýt xảy ra với chính tệp này:
   *
   *   Ngày 20/09 sáng, lời gọi máy chủ chỉ có ở bản `day-du`, nên ca này đo bằng HIỆU giữa hai
   *   bản: đường dẫn vắng ở `goc`, tên trường hơn đúng 1, `fetch(` hơn đúng 1. Chiều cùng ngày,
   *   bản nộp bắt đầu gọi thật. Cả ba vế ấy lập tức thành `0 === 0` — **xanh vĩnh viễn, vì không
   *   còn gì để tìm**. Không một ca nào khác đỏ lên để báo rằng ca này vừa mất hết nội dung.
   *
   * NÊN PHÉP ĐO NAY ĐO ĐÚNG THỨ PHẢI ĐÚNG HÔM NAY: **cả hai bản đều PHẢI mang lời gọi của ta.**
   *
   *   1. Đường dẫn tuyến có mặt ở CẢ HAI, và **đúng một lần** ở mỗi bản. `zmp-sdk` không thể
   *      tình cờ chứa chuỗi ấy, nên đây là vế chắc nhất — và "đúng một lần" là cách đo "đúng
   *      một chỗ gọi" mà không phải ghim một con số của SDK.
   *   2. Tên hai trường gửi đi có mặt ở cả hai, đọc từ chính `thanYeuCau` chứ không gõ lại: đổi
   *      hợp đồng thì ca này đi theo, thay vì xanh vì không tìm thấy gì.
   *   3. `fetch(` có mặt ở cả hai, và hai bản chênh nhau ĐÚNG 0 lần — khối đăng nhập không còn
   *      cửa biến thể nào, nên một chênh lệch xuất hiện nghĩa là ai đó vừa dựng lại một cửa.
   *
   * ⚠ KHÔNG THỂ KHẲNG ĐỊNH MỘT CON SỐ TUYỆT ĐỐI CHO `fetch(`. ĐÃ ĐO 20/09/2026: riêng `zmp-sdk`
   * đóng góp **14 lần** `fetch(` và **4 lần** `XMLHttpRequest` cho mọi bản dựng; mã của ta thêm
   * đúng 1. Ghim "15" là một ca đỏ vào ngày Zalo phát hành một bản SDK khác — một lý do ta không
   * sửa được, và một test đỏ vì lý do không sửa được là một test sắp bị ai đó xoá.
   */
  it("CẢ HAI bản đều mang đúng MỘT đường gọi máy chủ của ta", () => {
    const ten_truong = Object.keys(
      JSON.parse(thanYeuCauPhien({ ma_so_dien_thoai: "x", ma_truy_cap: "y" })) as Record<
        string,
        unknown
      >,
    );
    expect(ten_truong.length, "hợp đồng không còn trường nào để đo").toBe(2);

    const dem = (ban: string, chuoi: string) => ban.split(chuoi).length - 1;

    for (const [ten, ban] of [
      ["goc", goc],
      ["day-du", day_du],
    ] as const) {
      expect(
        dem(ban, DUONG_DAN_PHIEN),
        `bản ${ten} phải nhắc tuyến đăng nhập ĐÚNG MỘT lần. 0 lần = khối đăng nhập không gọi ` +
          "được máy chủ (nộp một nút không đăng nhập nổi); 2 lần trở lên = có chỗ gọi thứ hai.",
      ).toBe(1);

      for (const truong of ten_truong) {
        expect(
          dem(ban, truong),
          `bản ${ten} không mang tên trường "${truong}" của hợp đồng đăng nhập`,
        ).toBeGreaterThan(0);
      }

      expect(
        (ban.match(/fetch\s*\(/g) ?? []).length,
        `bản ${ten} không có một lời gọi mạng nào`,
      ).toBeGreaterThan(0);
    }

    // CHÊNH ĐÚNG MỘT, VÀ MỘT ẤY CÓ TÊN — 24/09/2026. Trước ngày này hai bản phải bằng nhau (khối
    // đăng nhập không còn cửa biến thể). Nay kênh công dân đứng sau `bien-the/cong-dan` và mang
    // ĐÚNG MỘT `fetch(` (`cong-dan/api/goi-vigov.ts`) chỉ ở bản `day-du`. Chênh 0 = kênh công dân
    // lọt vào bản nộp hoặc biến mất khỏi bản thử; chênh 2 = ai đó dựng thêm một cửa có lời gọi mạng.
    // Ca "tuyến ViGov chỉ ở bản thử" ngay dưới trả lời CÁI MỘT ẤY là gì.
    const demGoi = (ban: string) => (ban.match(/fetch\s*\(/g) ?? []).length;
    expect(
      demGoi(day_du) - demGoi(goc),
      "hai biến thể phải chênh nhau ĐÚNG MỘT lời gọi mạng — client ViGov của kênh công dân",
    ).toBe(1);
  });

  /**
   * TUYẾN THỨ HAI — BỀ MẶT YÊU CẦU (22/09/2026, giai đoạn B).
   *
   * Cùng khuôn với ca ngay trên, và cùng lý do: "đúng một lần" là cách đo "đúng một chỗ gọi" mà
   * không phải ghim một con số của SDK. 0 lần = bề mặt yêu cầu không gọi được máy chủ (nộp hai màn
   * không làm được gì); 2 lần trở lên = có chỗ gọi thứ hai mà không ai khai.
   *
   * ⚠ VÀ NĂM TÊN TRƯỜNG ĐỌC TỪ CHÍNH `thanYeuCau`, KHÔNG GÕ LẠI. Gõ lại là tạo bản sao thứ hai,
   * và ngày hợp đồng đổi thì bản sao ấy làm ca này xanh vì KHÔNG TÌM THẤY GÌ.
   */
  it("CẢ HAI bản đều mang đúng MỘT đường gọi tuyến yêu cầu của ta", () => {
    const ten_truong = Object.keys(
      JSON.parse(
        thanYeuCau({ loai: "consult", quan_tam: ["messaging"], quy_mo: "", ghi_chu: "", nguon: "" }),
      ) as Record<string, unknown>,
    );
    expect(ten_truong.length, "hợp đồng yêu cầu không còn trường nào để đo").toBe(5);

    const dem = (ban: string, chuoi: string) => ban.split(chuoi).length - 1;

    for (const [ten, ban] of [
      ["goc", goc],
      ["day-du", day_du],
    ] as const) {
      expect(
        dem(ban, DUONG_DAN_YEU_CAU),
        `bản ${ten} phải nhắc tuyến yêu cầu ĐÚNG MỘT lần`,
      ).toBe(1);
      // TÌM `ten:` CHỨ KHÔNG TÌM TÊN TRẦN, và khác biệt ấy quyết định ca này có nội dung hay không:
      // `note`, `source`, `scale` là những từ tiếng Anh thường, gần như chắc chắn có mặt đâu đó
      // trong bundle của `zmp-sdk` bất kể mã của ta gửi gì. Một `toContain("note")` vì thế xanh
      // ngay cả khi hợp đồng đã bị xoá sạch — đúng kiểu xanh vì không tìm thấy gì. Bộ rút gọn
      // KHÔNG đổi được tên khoá của một object literal đi lên dây, nên `note:` là cái neo đúng.
      for (const truong of ten_truong) {
        expect(
          dem(ban, `${truong}:`),
          `bản ${ten} không mang tên trường "${truong}" của hợp đồng yêu cầu`,
        ).toBeGreaterThan(0);
      }
    }
  });

  /**
   * KHỐI MỚI CỦA MÀN CHỦ (21/09/2026, tối) — menu nhanh · tin ViHAT · nút chat · câu slogan.
   *
   * Cùng lý do với ca "bản nộp có đủ chữ của sáu tính năng" ngay trên: không có ca "CÓ CHỨA" thì
   * mọi ca "KHÔNG CHỨA" ở dưới xanh vĩnh viễn ngay khi một khối biến mất khỏi bản dựng.
   */
  it("bản NỘP mang đủ chữ của bốn khối mới trên màn chủ", () => {
    const chuoi = [
      SLOGAN_HERO.cau,
      // Câu sản phẩm của bản mẫu — câu thứ hai trong hero, cùng nguồn với câu slogan.
      CAU_SAN_PHAM.cau,
      NHAN_LOAI_HINH,
      NHAN_NUT_CHAT,
      DUONG_DAN_CHAT_OA,
      // CÂU GHI CHÚ KIỂM THEO MẢNH, KHÔNG KIỂM CẢ CÂU — và đây là ĐÚNG cái bẫy mà tệp này đã mắc
      // một lần vào 18/09: `GHI_CHU_TIN` ghép `NGAY_CHUP_TIN` vào lúc chạy, nên bundle chỉ chứa
      // hai mảnh rời. Một `toContain` trên chuỗi ghép xanh vì KHÔNG BẢN NÀO chứa nó.
      NGAY_CHUP_TIN,
      "không tự tải tin mới",
      ...TIN_VIHAT.flatMap((bai) => [bai.tieu_de, bai.trich, bai.duong_dan]),
      ...MUC_MENU_NHANH.flatMap((muc) => [muc.nhan, muc.phu]),
    ];
    expect(chuoi.length).toBeGreaterThan(15);
    for (const mot of chuoi) {
      expect(goc, `bản nộp thiếu: ${mot}`).toContain(mot);
    }
  });

  /**
   * CÂU KHAI "MỞ MỘT TRANG BÊN NGOÀI" — KIỂM THEO TỪNG MẢNH, KHÔNG KIỂM CHUỖI GHÉP.
   *
   * Câu ấy được GHÉP LÚC CHẠY từ `DICH_MO_RA_NGOAI` (`cauKhaiDichRaNgoai`), nên chuỗi đầy đủ
   * KHÔNG hề có trong bundle — bundle chỉ chứa các mảnh rời. Kiểm chuỗi ghép ở đây là một phép
   * kiểm xanh vì **không bản nào** chứa nó, đúng kiểu hỏng mà tệp này đã tự mắc một lần (18/09).
   *
   * Cũng KHÔNG kiểm con số đọc thành chữ: "năm" là một từ thường, có mặt khắp bundle ("12 năm",
   * "thành lập"), nên một `toContain("năm")` xanh mà không nói lên gì. Số ấy được canh ở
   * `content/dich-ra-ngoai.test.ts`, nơi đọc được cả danh sách lẫn văn bản.
   */
  it("từng đích mở ra ngoài đều được khai trong CẢ HAI bản dựng", () => {
    for (const [ten, ban] of [
      ["goc", goc],
      ["day-du", day_du],
    ] as const) {
      expect(ban, `bản ${ten} không còn câu khai số chỗ mở trang ngoài`).toContain(
        "chỗ ứng dụng mở một trang bên ngoài",
      );
      for (const dich of DICH_MO_RA_NGOAI) {
        expect(ban, `bản ${ten} không khai đích "${dich.ma}"`).toContain(dich.trong_chinh_sach);
      }
    }
  });

  /**
   * KÊNH CÔNG DÂN (24/09/2026) — CHỈ Ở BẢN THỬ, cho tới ngày cầu phiên ViGov có thật.
   *
   * Hai vế, như mọi ca ở tệp này: bản `day-du` PHẢI mang (nếu không, ca "bản nộp không chứa" xanh vì
   * không bản nào chứa), bản `goc` KHÔNG được mang. Đường dẫn tuyến và tên tiêu đề chống trùng là
   * hai neo chắc nhất: `zmp-sdk` không thể tình cờ chứa chúng.
   */
  it("tuyến ViGov và hai màn của kênh công dân CHỈ có ở bản thử, không có ở bản nộp", () => {
    const dem = (ban: string, chuoi: string) => ban.split(chuoi).length - 1;
    expect(dem(day_du, DUONG_DAN_PHAN_ANH_CUA_TOI), "bản thử phải nhắc tuyến ViGov đúng một lần").toBe(1);
    expect(dem(goc, DUONG_DAN_PHAN_ANH_CUA_TOI), "bản NỘP mang tuyến ViGov của kênh công dân").toBe(0);
    expect(day_du).toContain("Idempotency-Key");
    expect(goc, "bản NỘP mang tiêu đề chống gửi trùng của tuyến ViGov").not.toContain("Idempotency-Key");

    // HOST CỦA `service-petitions` (ADR 0046, 26/09/2026). Đọc từ chính `diaChiViGov`, không gõ lại:
    // đổi host thì ca này đi theo, thay vì xanh vì không bản nào chứa chuỗi gõ tay. Vế "bản thử CÓ"
    // là thứ giữ cho vế "bản nộp KHÔNG" còn nội dung. Bản nộp là app của ViHAT Group, và một host
    // `.vigov.vn` trong đó là một chi tiết cơ quan nhà nước lọt vào thứ người duyệt Zalo đọc.
    const host_petitions = new URL(diaChiViGov("petitions", "/")).host;
    expect(host_petitions, "bảng host của kênh công dân mất dòng `petitions`").toBe("petitions.api.vigov.vn");
    expect(day_du, "bản thử không mang host của petitions").toContain(host_petitions);
    expect(goc, "bản NỘP mang host ViGov của kênh công dân").not.toContain(host_petitions);
    expect(goc, "bản NỘP mang một host `.api.vigov.vn` nào đó").not.toContain(".api.vigov.vn");

    // `CUA_TOI` (26/09/2026): màn "Phản ánh của tôi" — cũng CHỈ ở bản thử.
    for (const chuoi of [
      KENH_CHUA_MO.tieu_de,
      KHAN_CAP,
      TRA_CUU.khong_thay,
      NHAN_KENH_CONG_DAN,
      CUA_TOI.tieu_de,
      CUA_TOI.trong,
    ]) {
      expect(day_du, `bản thử thiếu: ${chuoi}`).toContain(chuoi);
      expect(goc, `bản NỘP vẫn chứa: ${chuoi}`).not.toContain(chuoi);
    }
  });

  it("bản NỘP không chứa bảng chẩn đoán", () => {
    // Một bảng in tham số nội bộ nằm trong bundle của một đơn vị đang xin duyệt là một câu hỏi
    // phải trả lời ở vòng duyệt, và "đó là công cụ nội bộ" không giúp được gì ở vòng đó.
    expect(goc).not.toContain("Chẩn đoán tham số mở app");
    expect(goc).not.toContain("URL Zalo dùng để mở app");
  });

  it("bản NỘP MANG `zmp-sdk` — nếu không thì sáu tính năng ấy không gọi được nền tảng", () => {
    // Đảo chiều so với trước: `zmp-sdk` từng bị cấm trong bản `goc` vì bản ấy là một app giới
    // thiệu tĩnh không xin quyền nào. Nay ba quyền thuộc về chính ứng dụng sản phẩm, nên SDK
    // PHẢI có mặt — một bản nộp xin ba quyền mà không gọi tới nền tảng là một bản nộp có ba cái
    // nút không làm gì, và vòng duyệt không có gì để cấp quyền cho.
    expect(goc, "bản nộp không mang zmp-sdk").toContain("zmp-sdk");
    for (const ten of [
      "getPhoneNumber",
      // Lời gọi thứ mười, thêm cùng khối đăng nhập (ADR 0020). Thiếu nó trong bản nộp thì luồng
      // đăng nhập một chạm không có gì để nộp lên vòng duyệt.
      "getAccessToken",
      "getLocation",
      "scanQRCode",
      // Sáu quyền xin thêm. Zalo chỉ cấp khi bản nộp CÓ chỗ dùng chúng nhìn thấy được, và "nhìn
      // thấy được" bắt đầu từ việc chính lời gọi ấy có mặt trong tệp được nộp.
      "getNetworkType",
      "keepScreen",
      "vibrate",
      "requestCameraPermission",
      "openMediaPicker",
      "downloadFile",
    ]) {
      expect(goc, `bản nộp không gọi ${ten}`).toContain(ten);
    }
  });

  /**
   * `serverUploadUrl` — CA NÀY KHÔNG KHẲNG ĐỊNH MỘT SỰ VẮNG MẶT, VÌ SỰ VẮNG MẶT ẤY KHÔNG CÓ THẬT.
   *
   * ĐÃ ĐO, 18/09/2026, TRÊN BUNDLE THẬT: chuỗi ấy xuất hiện **2 lần** trong bản `goc`, và cả hai
   * đều nằm trong mã của chính `zmp-sdk` — lược đồ tham số zod của `openMediaPicker`, và thân
   * hàm đọc `e.serverUploadUrl` để truyền xuống tầng dưới. Trước khi ba tính năng này tồn tại nó
   * đã có sẵn 1 lần (chỉ lược đồ); lần thứ hai xuất hiện vì `openMediaPicker` nay thật sự được
   * gọi, nên thân hàm của nó không còn bị tree-shaking loại đi. Không có cách nào gỡ chuỗi ấy ra
   * mà vẫn giữ SDK, và giữ SDK là điều kiện để chín quyền kia gọi được.
   *
   * NÊN PHÉP KIỂM ĐÚNG KHÔNG PHẢI "ĐẾM SỐ LẦN": một con số ghim cứng sẽ đỏ lên lần đầu Zalo phát
   * hành một bản SDK khác, vì một lý do ta không sửa được — và một test đỏ vì lý do không sửa
   * được là một test sắp bị ai đó xoá.
   *
   * PHÉP KIỂM ĐÚNG LÀ HÌNH DẠNG CỦA CHÍNH KHUYẾT TẬT: để ảnh rời khỏi máy, mã của ta phải GÁN
   * MỘT CHUỖI cho tham số ấy. Lược đồ của SDK viết `serverUploadUrl:K().url().optional()`, thân
   * hàm viết `e.serverUploadUrl` — không dạng nào là một chuỗi được gán. Còn phép bảo đảm mạnh
   * thì nằm ở tầng mã nguồn: `phase1-collects-nothing.test.ts` cấm chuỗi ấy ở MỌI tệp, kể cả
   * trong chính thư mục tính năng.
   */
  it("không tệp nào GÁN một địa chỉ cho `serverUploadUrl` — ảnh không có đường rời khỏi máy", () => {
    for (const [ten, ban] of [
      ["goc", goc],
      ["day-du", day_du],
    ] as const) {
      const gan = ban.match(/serverUploadUrl\s*:\s*["'`]/g) ?? [];
      expect(
        gan,
        `bản ${ten} gán một chuỗi cho serverUploadUrl. Tham số ấy là đường DUY NHẤT ` +
          "openMediaPicker tải ảnh người dùng lên một máy chủ, và ứng dụng này không có máy chủ nào.",
      ).toEqual([]);
    }
  });

  /**
   * CHÍNH SÁCH MÔ TẢ ĐÚNG BẢN DỰNG NÓ NẰM TRONG — nay là MỘT văn bản cho cả hai bản dựng.
   *
   * Bản trước của ca này kiểm hai câu mở đầu khác nhau, vì bản nộp khi ấy không gọi mạng. Tiền
   * đề đó không còn: cả hai biến thể gọi máy chủ thật, nên hai bản phải nói **y hệt nhau** về
   * quyền riêng tư. Ca này đổi chiều theo: nó khẳng định câu mở đầu MỚI có trong cả hai, và câu
   * mở đầu CŨ — thứ đã thành sai — không còn trong bản nào.
   */
  it("một văn bản chính sách, giống hệt nhau ở cả hai bản dựng", () => {
    const CAU_DAU_DA_THANH_SAI = "không lưu trữ và không gửi đi bất kỳ dữ liệu nào của bạn";
    for (const [ten, ban] of [
      ["goc", goc],
      ["day-du", day_du],
    ] as const) {
      expect(ban, `bản ${ten} thiếu câu mở đầu chính sách`).toContain(CAU_DAU);
      expect(
        ban,
        `bản ${ten} vẫn mang câu mở đầu CŨ — câu ấy thành sai từ ngày khối đăng nhập gọi máy chủ`,
      ).not.toContain(CAU_DAU_DA_THANH_SAI);
    }
  });

  it("mục Đăng nhập của chính sách có mặt ĐỦ trong cả hai bản", () => {
    // Đây là mục khai việc gửi hai mã đi và việc máy chủ lưu số điện thoại. Thiếu nó trong bản
    // nộp là giấu một hành vi mà mã CÓ — đúng thứ Nghị định 13 nhắm tới.
    const muc = MUC_CHINH_SACH.find((m) => m.ma === "dang-nhap");
    expect(muc, "chính sách không còn mục nào về đăng nhập").toBeDefined();
    for (const [ten, ban] of [
      ["goc", goc],
      ["day-du", day_du],
    ] as const) {
      for (const doan of muc!.doan) {
        expect(ban, `bản ${ten} thiếu đoạn chính sách: ${doan.slice(0, 40)}…`).toContain(doan);
      }
    }
  });

  it("chính sách quyền riêng tư có mặt trong CẢ HAI bản, đủ mọi mục", () => {
    // Xin ba quyền mà không có chính sách thì vòng duyệt trả về. Mục về ba quyền nay nằm trong
    // danh sách chung, vì ba quyền có mặt ở MỌI bản dựng — không còn gì để tách.
    for (const [ten, ban] of [
      ["goc", goc],
      ["day-du", day_du],
    ] as const) {
      expect(ban, `bản ${ten} thiếu tiêu đề chính sách`).toContain(TIEU_DE_CHINH_SACH);
      // Câu mở đầu kiểm ở ca ngay trên, cùng với câu CŨ phải biến mất khỏi cả hai bản.
      expect(ban, `bản ${ten} ghi sai phiên bản chính sách`).toContain(PHIEN_BAN_CHINH_SACH);
      for (const m of MUC_CHINH_SACH) {
        expect(ban, `bản ${ten} thiếu mục "${m.tieu_de}"`).toContain(m.tieu_de);
      }
    }
  });

  /**
   * MẢNH CHỮ CÓ THẬT TRONG BUNDLE của mục chính sách về ba quyền.
   *
   * VÌ SAO KHÔNG KIỂM THẲNG TỪNG `doan`: một số đoạn được GHÉP LÚC CHẠY từ `NOI_DUNG_TINH_NANG`
   * (`${nhan_ngan} — ${vi_sao}`), nên chuỗi ghép ấy KHÔNG hề có trong bundle — bundle chỉ chứa
   * hai mảnh rời. Kiểm chuỗi ghép thì phép kiểm xanh vì **không bản nào** chứa nó. Đó đúng là
   * kiểu hỏng tệp này sinh ra để bắt, và nó đã bắt được chính mình một lần vào 18/09 — giữ lại
   * chú thích này thay vì sửa xong rồi xoá dấu vết.
   */
  it("chính sách nói ĐỦ mục đích của cả ba quyền, trong cả hai bản", () => {
    const muc = MUC_CHINH_SACH.find((m) => m.ma === "cac-quyen");
    expect(muc, "chính sách không còn mục nào về ba quyền").toBeDefined();
    const manh = muc!.doan.flatMap((doan) => doan.split(" — ")).filter((m) => m.length >= 30);
    expect(manh.length).toBeGreaterThan(3);
    for (const m of manh) {
      expect(goc, `chính sách bản nộp thiếu: ${m.slice(0, 50)}…`).toContain(m);
      expect(day_du, `chính sách bản đầy đủ thiếu: ${m.slice(0, 50)}…`).toContain(m);
    }
  });

  it("vẫn ĐÚNG BẰNG một ứng dụng có nội dung — không phải một bản rỗng", () => {
    // Gỡ nhầm tay thì bản nộp cũng "không chứa tên xã nào", và mọi ca trên vẫn xanh. Ca này là
    // thứ phân biệt "đã gỡ đúng phần thừa" với "đã gỡ mất app".
    expect(goc).toContain(COMPANY.name);
    for (const man of MAN_GIOI_THIEU) {
      expect(goc, `bản nộp thiếu màn ${man.id}`).toContain(man.headerTitle);
    }
    // BỐN NHÃN TAB, đọc từ chính danh sách thanh tab vẽ ra. Một bản nộp thiếu chúng là một bản
    // nộp không có thanh tab — thứ ca "không chứa tên xã nào" ở trên vẫn cho qua.
    expect(TABS, "thanh tab rỗng — ca này sẽ xanh vì không tìm thấy gì").toHaveLength(4);
    for (const man of TABS) {
      expect(goc, `bản nộp thiếu nhãn tab ${man.id}`).toContain(man.cho.nhan);
    }
    expect(goc, "bản nộp thiếu màn Danh thiếp").toContain(MAN_DANH_THIEP.headerTitle);
  });

  it("nhẹ hơn bản đầy đủ — bằng chứng rằng mã thật sự biến mất, không chỉ bị giấu", () => {
    expect(goc.length).toBeLessThan(day_du.length);
  });

  it("tên biến thể sai thì DỪNG, không dựng bằng mặc định", () => {
    // Một cái tên gõ nhầm mà vẫn dựng tiếp nghĩa là dựng bản ĐẦY ĐỦ rồi đem nộp dưới nhãn khác —
    // hỏng trong im lặng, đúng chỗ đắt nhất. Ca này gọi thẳng `vite build` với một tên sai.
    return expect(dungBienThe("gôc" as "goc")).rejects.toThrow(/VIGOV_BIEN_THE/);
  });

  it("tên biến thể `quyen` nay cũng là một tên SAI", () => {
    // Biến thể ấy đã bị gộp vào `goc`. Nếu nó vẫn dựng được thì ai đó đang đẩy lên Zalo một bản
    // không tệp nào còn định nghĩa.
    return expect(dungBienThe("quyen" as "goc")).rejects.toThrow(/VIGOV_BIEN_THE/);
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
