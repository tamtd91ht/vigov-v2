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
import { BRAND_NAVY, COMPANY } from "./content/company-profile";
import { MAN_GIOI_THIEU } from "./features/company-intro/screens";
import { MAN_DANH_THIEP } from "./features/tinh-nang/index";
import {
  CHI_HIEN_LEN_MAN_HINH,
  DANH_THIEP,
  DUONG_TRUYEN,
  KIEU_KET_NOI,
  LOI_MO_NGOAI,
  MA_RONG,
  NOI_DUNG_TINH_NANG,
  SO_HOA_THIEP,
  THIEP_CUA_CHUNG_TOI,
  TOKEN_KHONG_CHUA_GI,
  TU_VAN,
  VAN_PHONG,
} from "./features/tinh-nang/noi-dung";
import {
  DEMO_DANH_MUC_XA,
  DEMO_GHI_CHU,
  DEMO_GHI_CHU_TRANG_XA,
  DEMO_TEN_DICH_VU,
} from "./features/kham-pha/demo-danh-muc-xa";
import { NHAN_KHAM_PHA } from "./features/kham-pha/index";
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
    MAN_DANH_THIEP.tabLabel,
    MAN_DANH_THIEP.headerTitle,
    CHI_HIEN_LEN_MAN_HINH,
    MA_RONG,
    LOI_MO_NGOAI,
    TOKEN_KHONG_CHUA_GI["tu-van"],
    TOKEN_KHONG_CHUA_GI["van-phong"],
    ...Object.values(DANH_THIEP),
    ...Object.values(VAN_PHONG),
    ...Object.values(TU_VAN),
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
   *   Bản nộp là một **ứng dụng sản phẩm của VihatSoftware**, một doanh nghiệp công nghệ. Không
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

  it("chính sách quyền riêng tư có mặt trong CẢ HAI bản, đủ mọi mục", () => {
    // Xin ba quyền mà không có chính sách thì vòng duyệt trả về. Mục về ba quyền nay nằm trong
    // danh sách chung, vì ba quyền có mặt ở MỌI bản dựng — không còn gì để tách.
    for (const [ten, ban] of [
      ["goc", goc],
      ["day-du", day_du],
    ] as const) {
      expect(ban, `bản ${ten} thiếu tiêu đề chính sách`).toContain(TIEU_DE_CHINH_SACH);
      expect(ban, `bản ${ten} thiếu câu mở đầu chính sách`).toContain(CAU_DAU);
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
      expect(goc, `bản nộp thiếu màn ${man.id}`).toContain(man.tabLabel);
    }
    expect(goc, "bản nộp thiếu tab Danh thiếp").toContain(MAN_DANH_THIEP.tabLabel);
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
