/// <reference types="vite/client" />
import { beforeAll, describe, expect, it } from "vitest";

import { build } from "vite";

import appConfigRaw from "../app-config.json?raw";
import indexHtml from "../index.html?raw";
import domainMapRaw from "../scripts/ung-dung-theo-ten-mien.mjs?raw";
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
// HAI HỢP ĐỒNG, HAI HÀM "THÂN YÊU CẦU": `thanYeuCau` của tuyến yêu cầu (nửa thương mại, giữ tên) và
// `sessionRequestBody` của tuyến đăng nhập. Mỗi hàm là thân của đúng tuyến nó phục vụ.
import { DUONG_DAN_YEU_CAU, thanYeuCau } from "./api/hop-dong-yeu-cau";
import { SESSION_PATH, LOCATION_PATH, sessionRequestBody } from "./features/log-in/contract";
import { nhanNguon, TIEU_DE_XAC_NHAN_XA } from "./features/kham-pha/goi-y";
import { vigovAddress } from "./citizen/api/vigov-address";
import { MY_REPORTS_PATH } from "./citizen/api/citizen-report-contract";
import { CITIZEN_CHANNEL_LABEL } from "./citizen/screens/CitizenChannel";
import { MY_REPORTS, CHANNEL_NOT_OPEN, EMERGENCY, LOOKUP } from "./citizen/screens/copy";

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
 * App ID và token GIẢ đặt vào môi trường TRONG LÚC DỰNG — để ca "không App ID, không token" ở dưới
 * đo trên một bundle dựng ra khi hai thứ ấy CÓ MẶT, đúng tình huống của máy chạy `zmp deploy`.
 * Không có chúng trong môi trường thì ca ấy xanh vì không có gì để lọt.
 */
const FAKE_APP_ID = "1111111111111111111";
const FAKE_ZMP_TOKEN = "gia-lap-zmp-token-khong-duoc-lot-vao-bundle";

/**
 * Dựng bản đẩy lên Zalo, trong bộ nhớ, qua đúng `vite.config.ts` mà `npm run build` dùng.
 *
 * MỘT BẢN, KHÔNG BIẾN THỂ (27/09/2026, quyết định của chủ sản phẩm — xem đầu `vite.config.ts`).
 * Trước ngày này tệp này dựng hai biến thể (`goc` · `day-du`) rồi so hai bundle; nay app chung và
 * app riêng của xã chạy CÙNG MỘT bundle, nên chỉ có một thứ để đo. Ngoại lệ `--vao-thang` (ADR 0047
 * §6) chỉ thêm một tên miền qua `VIGOV_XA_CO_DINH` — biến ấy KHÔNG có trong môi trường test, nên
 * đây là bản không cờ; `scripts/cau-hinh.test.mjs` và `lib/fixed-commune.test.ts` canh phần có cờ.
 */
async function buildBundle(): Promise<EmittedFile[]> {
  const previous = { APP_ID: process.env["APP_ID"], ZMP_TOKEN: process.env["ZMP_TOKEN"] };
  process.env["APP_ID"] = FAKE_APP_ID;
  process.env["ZMP_TOKEN"] = FAKE_ZMP_TOKEN;
  try {
    const result = await build({ logLevel: "silent", build: { write: false } });
    const outputs = (Array.isArray(result) ? result : [result]) as unknown as Array<{
      output: EmittedFile[];
    }>;
    return outputs.flatMap((output) => output.output);
  } finally {
    for (const [k, v] of Object.entries(previous)) {
      if (v === undefined) delete process.env[k];
      else process.env[k] = v;
    }
  }
}

/**
 * Toàn văn mọi tệp phát ra, đã giải mã `\uXXXX`.
 *
 * Bộ rút gọn có quyền thoát ký tự ngoài ASCII, và tiếng Việt thì toàn ký tự ngoài ASCII. Không
 * giải mã thì phép tìm "cơ quan" trong bundle **không khớp gì cả** và ca kiểm xanh vì lý do sai —
 * đúng cái chế độ hỏng mà ca kiểm ấy sinh ra để chặn.
 */
const fullText = (files: EmittedFile[]) =>
  files
    .map((f) => String(f.code ?? f.source ?? ""))
    .join("\n")
    .replace(/\\u([0-9a-fA-F]{4})/g, (_, code) => String.fromCharCode(Number.parseInt(code, 16)));

let emitted: EmittedFile[] = [];
/** Toàn văn bundle, đã giải mã. Dựng MỘT lần cho cả tệp. */
let bundle = "";

beforeAll(async () => {
  emitted = await buildBundle();
  bundle = fullText(emitted);
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
    const names = emitted.map((f) => f.fileName).filter((n) => n.endsWith(".js"));
    expect(names).toContain("assets/app.js");
    for (const n of names) expect(n).not.toMatch(/-[A-Za-z0-9_-]{8}\.js$/);
  });
});

/**
 * MỘT BUNDLE — và đây là ca kiểm duy nhất biến "bản đẩy lên đúng bằng thứ người duyệt đọc" từ lời
 * hứa thành sự thật đo được. Nó dựng THẬT rồi đọc bundle, vì `grep` trên bundle là câu trả lời cuối.
 *
 * ⚠ 27/09/2026 — HAI BIẾN THỂ ĐÃ GỘP MỘT (quyết định của chủ sản phẩm, thay ADR 0047 D5). Những ca so
 * `goc` với `day-du` đã bị xoá hoặc viết lại cho một bundle. Riêng một luật bị GỠ, không viết lại:
 *
 *   "Bản `goc` không chứa MỘT từ nào của ngữ cảnh cơ quan nhà nước" (danh sách `TU_CAM`: "cơ quan",
 *   "công dân", "chính quyền", "hành chính", "thủ tục", "Chọn xã", "Đổi xã") — cùng ca đối ứng của
 *   nó, "`xã hội` vẫn còn nguyên". Luật ấy chỉ đúng khi có một bản nộp KHÔNG mang kênh công dân. Nay
 *   app chung giai đoạn 1 và app riêng của xã chạy cùng một citizen-app có kênh công dân, nên luật
 *   ấy đã bị thay thế, không phải bị quên. Đừng dựng lại nó.
 *
 * VÌ SAO MỌI CA "KHÔNG CHỨA" ĐI KÈM MỘT CA "CÓ CHỨA": không có vế "có", đổi một chữ là đủ để vế
 * "không" xanh vĩnh viễn — bundle không chứa câu ấy vì KHÔNG BẢN NÀO chứa nó nữa.
 */
describe("bản đẩy lên Zalo — một bundle, đúng bằng thứ người duyệt đọc", () => {
  /**
   * CHỮ CỦA BA TÍNH NĂNG — dùng lại đúng nguồn mà màn hình đọc (`features/tinh-nang/noi-dung.ts`),
   * không chép tay. Một danh sách chép tay sẽ lệch khi ai đó sửa một câu, và lúc nó lệch thì ca
   * "CÓ chứa" xanh vì **không tìm thấy gì**, chứ không vì bundle đúng.
   */
  const FEATURE_STRINGS = () => [
    MAN_DANH_THIEP.headerTitle,
    CHI_HIEN_LEN_MAN_HINH,
    MA_RONG,
    LOI_MO_NGOAI,
    TOKEN_KHONG_CHUA_GI["dang-nhap"],
    TOKEN_KHONG_CHUA_GI["van-phong"],
    ...Object.values(DANH_THIEP),
    ...Object.values(VAN_PHONG),
    ...Object.values(DANG_NHAP),
    // `KIEU_KET_NOI` là một bản đồ lồng nhau, nên trải phẳng ra tường minh: `Object.values` trên
    // nó sẽ cho ra những đối tượng, và `toContain` trên một đối tượng là xanh vì lý do sai.
    ...Object.values(DUONG_TRUYEN),
    ...Object.values(THIEP_CUA_CHUNG_TOI),
    ...Object.values(SO_HOA_THIEP),
    ...Object.values(KIEU_KET_NOI).flatMap((item) => [item.nhan, item.y_nghia]),
    ...NOI_DUNG_TINH_NANG.flatMap((feature) => [
      feature.nhan_ngan,
      feature.tieu_de,
      feature.vi_sao,
      feature.nut,
      feature.dang_cho,
      feature.tu_choi,
      feature.ngoai_zalo,
      feature.khong_lay_duoc,
    ]),
  ];

  const count = (str: string) => bundle.split(str).length - 1;

  it("mang đủ chữ của sáu tính năng", () => {
    const strings = FEATURE_STRINGS();
    expect(strings.length).toBeGreaterThan(20);
    for (const item of strings) {
      expect(bundle, `bundle thiếu: ${item}`).toContain(item);
    }
  });

  it("mang màn xác nhận xã — vỏ để card sau nối vào nguồn xã phía máy chủ", () => {
    // Vế "CÓ" của ca "không còn bộ chọn xã" ngay dưới: màn xác nhận vẫn phải có mặt.
    for (const str of [
      TIEU_DE_XAC_NHAN_XA,
      nhanNguon("qr"),
      nhanNguon("zns"),
      "Bạn cần liên hệ với xã này?",
      "Không phải xã này",
    ]) {
      expect(bundle, `bundle thiếu: ${str}`).toContain(str);
    }
  });

  it("KHÔNG mang bộ chọn xã, nút đổi xã, trang xã mẫu hay danh mục xã mẫu (ADR 0044 câu 4 · 0047)", () => {
    for (const str of [
      "Chọn xã khác",
      "Đổi xã",
      "Chọn xã để tiếp tục",
      "Dịch vụ của xã",
      "Số điện thoại trực",
      // Tiền tố mã xã của danh mục mẫu cũ. Nó lọt lại là danh mục ấy lọt lại.
      "01JDEMXA",
    ]) {
      expect(bundle, `bundle vẫn chứa: ${str}`).not.toContain(str);
    }
  });

  it("KHÔNG mang chữ 'demo' / 'dữ liệu mẫu' / 'trình diễn' — đây là app dùng thật", () => {
    // Chủ sản phẩm, 27/09/2026: "demo ở đây là ngôn ngữ nói hiểu về cách làm, không phải là khái
    // niệm kỹ thuật, về kỹ thuật nó là app dùng thật". Chỉ cấm những cụm tiếng Việt: chữ Latin
    // "demo" trần có thể nằm trong mã của `zmp-sdk`, và một ca đỏ vì SDK là một ca sắp bị xoá.
    for (const str of ["trình diễn", "dữ liệu mẫu", "Dữ liệu mẫu", "danh mục mẫu", "Danh mục mẫu", "bản demo", "Bản demo"]) {
      expect(bundle, `bundle vẫn chứa: ${str}`).not.toContain(str);
    }
  });

  it("KHÔNG mang bảng chẩn đoán", () => {
    // Bảng in tham số nội bộ đã bị xoá khỏi mã. Nó lọt lại là một câu hỏi ở vòng duyệt.
    expect(bundle).not.toContain("Chẩn đoán tham số mở app");
    expect(bundle).not.toContain("URL Zalo dùng để mở app");
  });

  /**
   * KHÔNG App ID, KHÔNG token, KHÔNG tên miền xã nào trong bundle — ADR 0047 điều kiện dừng #2.
   *
   * Tên miền chỉ chọn App ID ĐÍCH lúc đẩy (`scripts/dich-den.mjs`); nó không vào nội dung — trừ bản
   * dựng `--vao-thang` của đúng xã ấy (ADR 0047 §6), thứ ca này không dựng. Một App ID hay tên miền xã trong bundle là một giá trị theo xã nung vào một bundle dùng
   * chung — đúng thứ luật 1 bất biến 10 cấm. Bundle ở đây được dựng khi `APP_ID` và `ZMP_TOKEN`
   * CÓ trong môi trường (`buildBundle`), nên ca này đo đúng tình huống của máy chạy `zmp deploy`.
   */
  it("không mang App ID, token, hay tên miền / App ID nào của tệp ánh xạ", () => {
    expect(bundle, "APP_ID của môi trường dựng lọt vào bundle").not.toContain(FAKE_APP_ID);
    expect(bundle, "ZMP_TOKEN của môi trường dựng lọt vào bundle").not.toContain(FAKE_ZMP_TOKEN);

    // Khoá và giá trị của tệp ánh xạ, đọc từ chính tệp (`?raw`), không gõ lại.
    const in_file = [...domainMapRaw.matchAll(/^\s*"([^"]+)"\s*:\s*"([^"]+)"/gm)].flatMap((m) => [
      m[1]!,
      m[2]!,
    ]);
    expect(in_file.length, "không đọc được dòng nào của tệp ánh xạ — ca này sẽ xanh vì rỗng").toBeGreaterThan(0);
    for (const str of in_file) {
      expect(bundle, `bundle mang "${str}" của tệp ánh xạ tên miền → App ID`).not.toContain(str);
    }
  });

  /**
   * ĐÚNG MỘT ĐƯỜNG GỌI CHO MỖI TUYẾN — "đúng một lần" là cách đo "đúng một chỗ gọi" mà không phải
   * ghim một con số của SDK.
   *
   * ⚠ KHÔNG THỂ KHẲNG ĐỊNH MỘT CON SỐ TUYỆT ĐỐI CHO `fetch(`. ĐÃ ĐO 20/09/2026: riêng `zmp-sdk` đóng
   * góp **14 lần** `fetch(` cho mọi bản dựng. Ghim một con số là một ca đỏ vào ngày Zalo phát hành
   * một bản SDK khác — một lý do ta không sửa được.
   */
  it("mang đúng MỘT đường gọi tuyến đăng nhập", () => {
    const field_names = Object.keys(
      JSON.parse(sessionRequestBody({ ma_so_dien_thoai: "x", ma_truy_cap: "y" })) as Record<
        string,
        unknown
      >,
    );
    expect(field_names.length, "hợp đồng không còn trường nào để đo").toBe(2);
    expect(
      count(SESSION_PATH),
      "bundle phải nhắc tuyến đăng nhập ĐÚNG MỘT lần. 0 lần = khối đăng nhập không gọi được máy " +
        "chủ; 2 lần trở lên = có chỗ gọi thứ hai.",
    ).toBe(1);
    for (const field_name of field_names) {
      expect(count(field_name), `bundle không mang tên trường "${field_name}" của hợp đồng đăng nhập`).toBeGreaterThan(0);
    }
    expect((bundle.match(/fetch\s*\(/g) ?? []).length, "bundle không có một lời gọi mạng nào").toBeGreaterThan(0);
  });

  it("mang đúng MỘT đường gọi tuyến đổi mã vị trí, và hai tên trường của nó", () => {
    // 29/09/2026 — `vihat-miniapp` `POST /api/v1/location`. 0 = the location button cannot reach the
    // server; 2+ = a second caller appeared somewhere.
    expect(count(LOCATION_PATH), "bundle phải nhắc tuyến đổi mã vị trí ĐÚNG MỘT lần").toBe(1);
    expect(count("locationToken"), "bundle không mang tên trường locationToken").toBeGreaterThan(0);
  });

  it("mang đúng MỘT đường gọi tuyến yêu cầu", () => {
    const field_names = Object.keys(
      JSON.parse(
        thanYeuCau({ loai: "consult", quan_tam: ["messaging"], quy_mo: "", ghi_chu: "", nguon: "" }),
      ) as Record<string, unknown>,
    );
    expect(field_names.length, "hợp đồng yêu cầu không còn trường nào để đo").toBe(5);
    expect(count(DUONG_DAN_YEU_CAU), "bundle phải nhắc tuyến yêu cầu ĐÚNG MỘT lần").toBe(1);
    // TÌM `ten:` CHỨ KHÔNG TÌM TÊN TRẦN: `note`, `source`, `scale` là những từ tiếng Anh thường,
    // gần như chắc chắn có mặt trong `zmp-sdk`. Bộ rút gọn KHÔNG đổi được tên khoá của một object
    // literal đi lên dây, nên `note:` là cái neo đúng.
    for (const field_name of field_names) {
      expect(count(`${field_name}:`), `bundle không mang tên trường "${field_name}" của hợp đồng yêu cầu`).toBeGreaterThan(0);
    }
  });

  /**
   * KÊNH CÔNG DÂN — CÓ MẶT trong bundle từ 27/09/2026 (trước đó chỉ ở bản `day-du`). Vẫn ĐÓNG: cầu
   * phiên ViGov chưa có, nên không lời gọi nào đi ra (`citizen.test.tsx` đo điều đó). Đường dẫn
   * tuyến và tiêu đề chống trùng là hai neo chắc nhất: `zmp-sdk` không thể tình cờ chứa chúng.
   */
  it("mang kênh công dân: đúng MỘT đường gọi tuyến ViGov, và đủ chữ của các màn", () => {
    expect(count(MY_REPORTS_PATH), "bundle phải nhắc tuyến ViGov đúng một lần").toBe(1);
    expect(bundle).toContain("Idempotency-Key");

    // HOST CỦA `service-petitions` (ADR 0046). Đọc từ chính `vigovAddress`, không gõ lại.
    const host_petitions = new URL(vigovAddress("petitions", "/")).host;
    expect(host_petitions, "bảng host của kênh công dân mất dòng `petitions`").toBe("petitions.api.vigov.vn");
    expect(bundle, "bundle không mang host của petitions").toContain(host_petitions);

    for (const str of [
      CHANNEL_NOT_OPEN.title,
      EMERGENCY,
      LOOKUP.not_found,
      CITIZEN_CHANNEL_LABEL,
      MY_REPORTS.title,
      MY_REPORTS.empty,
    ]) {
      expect(bundle, `bundle thiếu: ${str}`).toContain(str);
    }
  });

  /**
   * KHỐI CỦA MÀN CHỦ (21/09/2026, tối) — menu nhanh · tin ViHAT · nút chat · câu slogan.
   *
   * ⚠ CÂU GHI CHÚ KIỂM THEO MẢNH, KHÔNG KIỂM CẢ CÂU: `GHI_CHU_TIN` ghép `NGAY_CHUP_TIN` vào lúc chạy,
   * nên bundle chỉ chứa hai mảnh rời. Một `toContain` trên chuỗi ghép xanh vì KHÔNG BẢN NÀO chứa nó.
   */
  it("mang đủ chữ của bốn khối trên màn chủ", () => {
    const strings = [
      SLOGAN_HERO.cau,
      CAU_SAN_PHAM.cau,
      NHAN_LOAI_HINH,
      NHAN_NUT_CHAT,
      DUONG_DAN_CHAT_OA,
      NGAY_CHUP_TIN,
      "không tự tải tin mới",
      ...TIN_VIHAT.flatMap((article) => [article.tieu_de, article.trich, article.duong_dan]),
      ...MUC_MENU_NHANH.flatMap((entry) => [entry.nhan, entry.phu]),
    ];
    expect(strings.length).toBeGreaterThan(15);
    for (const item of strings) {
      expect(bundle, `bundle thiếu: ${item}`).toContain(item);
    }
  });

  /**
   * CÂU KHAI "MỞ MỘT TRANG BÊN NGOÀI" — KIỂM THEO TỪNG MẢNH, KHÔNG KIỂM CHUỖI GHÉP: câu ấy được ghép
   * lúc chạy từ `DICH_MO_RA_NGOAI`, nên chuỗi đầy đủ không có trong bundle.
   */
  it("khai từng đích mở ra ngoài", () => {
    expect(bundle, "bundle không còn câu khai số chỗ mở trang ngoài").toContain(
      "chỗ ứng dụng mở một trang bên ngoài",
    );
    for (const destination of DICH_MO_RA_NGOAI) {
      expect(bundle, `bundle không khai đích "${destination.ma}"`).toContain(destination.trong_chinh_sach);
    }
  });

  it("MANG `zmp-sdk` và đủ mười lời gọi nền tảng — nếu không thì các tính năng không gọi được", () => {
    expect(bundle, "bundle không mang zmp-sdk").toContain("zmp-sdk");
    for (const name of [
      "getPhoneNumber",
      "getAccessToken",
      "getLocation",
      "scanQRCode",
      "getNetworkType",
      "keepScreen",
      "vibrate",
      "requestCameraPermission",
      "openMediaPicker",
      "downloadFile",
    ]) {
      expect(bundle, `bundle không gọi ${name}`).toContain(name);
    }
  });

  /**
   * `serverUploadUrl` — CA NÀY KHÔNG KHẲNG ĐỊNH MỘT SỰ VẮNG MẶT, VÌ SỰ VẮNG MẶT ẤY KHÔNG CÓ THẬT.
   *
   * ĐÃ ĐO, 18/09/2026: chuỗi ấy xuất hiện trong mã của chính `zmp-sdk` (lược đồ tham số và thân hàm
   * của `openMediaPicker`). Phép kiểm đúng là HÌNH DẠNG của khuyết tật: để ảnh rời khỏi máy, mã của
   * ta phải GÁN MỘT CHUỖI cho tham số ấy. Phép bảo đảm mạnh nằm ở `phase1-collects-nothing.test.ts`.
   */
  it("không tệp nào GÁN một địa chỉ cho `serverUploadUrl` — ảnh không có đường rời khỏi máy", () => {
    expect(
      bundle.match(/serverUploadUrl\s*:\s*["'`]/g) ?? [],
      "bundle gán một chuỗi cho serverUploadUrl — đường DUY NHẤT openMediaPicker tải ảnh lên máy chủ",
    ).toEqual([]);
  });

  it("mang câu mở đầu chính sách hiện hành, không mang câu cũ đã thành sai", () => {
    expect(bundle, "bundle thiếu câu mở đầu chính sách").toContain(CAU_DAU);
    expect(
      bundle,
      "bundle vẫn mang câu mở đầu CŨ — câu ấy thành sai từ ngày khối đăng nhập gọi máy chủ",
    ).not.toContain("không lưu trữ và không gửi đi bất kỳ dữ liệu nào của bạn");
  });

  it("mang ĐỦ mục Đăng nhập của chính sách", () => {
    // Đây là mục khai việc gửi hai mã đi và việc máy chủ lưu số điện thoại. Thiếu nó là giấu một
    // hành vi mà mã CÓ — đúng thứ Nghị định 13 nhắm tới.
    const section = MUC_CHINH_SACH.find((m) => m.ma === "dang-nhap");
    expect(section, "chính sách không còn mục nào về đăng nhập").toBeDefined();
    for (const paragraph of section!.doan) {
      expect(bundle, `bundle thiếu đoạn chính sách: ${paragraph.slice(0, 40)}…`).toContain(paragraph);
    }
  });

  it("mang chính sách quyền riêng tư, đủ mọi mục, đúng phiên bản", () => {
    expect(bundle, "bundle thiếu tiêu đề chính sách").toContain(TIEU_DE_CHINH_SACH);
    expect(bundle, "bundle ghi sai phiên bản chính sách").toContain(PHIEN_BAN_CHINH_SACH);
    for (const m of MUC_CHINH_SACH) {
      expect(bundle, `bundle thiếu mục "${m.tieu_de}"`).toContain(m.tieu_de);
    }
  });

  /**
   * MẢNH CHỮ CÓ THẬT TRONG BUNDLE của mục chính sách về các quyền. Một số đoạn được GHÉP LÚC CHẠY
   * (`${nhan_ngan} — ${vi_sao}`), nên kiểm theo mảnh, không theo chuỗi ghép.
   */
  it("chính sách nói ĐỦ mục đích của các quyền", () => {
    const section = MUC_CHINH_SACH.find((m) => m.ma === "cac-quyen");
    expect(section, "chính sách không còn mục nào về các quyền").toBeDefined();
    const fragments = section!.doan.flatMap((paragraph) => paragraph.split(" — ")).filter((m) => m.length >= 30);
    expect(fragments.length).toBeGreaterThan(3);
    for (const m of fragments) {
      expect(bundle, `chính sách thiếu: ${m.slice(0, 50)}…`).toContain(m);
    }
  });

  it("vẫn ĐÚNG BẰNG một ứng dụng có nội dung — không phải một bản rỗng", () => {
    // Gỡ nhầm tay thì bundle cũng "không chứa bộ chọn xã nào", và mọi ca trên vẫn xanh. Ca này là
    // thứ phân biệt "đã gỡ đúng phần thừa" với "đã gỡ mất app".
    expect(bundle).toContain(COMPANY.name);
    for (const screen of MAN_GIOI_THIEU) {
      expect(bundle, `bundle thiếu màn ${screen.id}`).toContain(screen.headerTitle);
    }
    expect(TABS, "thanh tab rỗng — ca này sẽ xanh vì không tìm thấy gì").toHaveLength(4);
    for (const screen of TABS) {
      expect(bundle, `bundle thiếu nhãn tab ${screen.id}`).toContain(screen.cho.nhan);
    }
    expect(bundle, "bundle thiếu màn Danh thiếp").toContain(MAN_DANH_THIEP.headerTitle);
  });

  it("một `VIGOV_BIEN_THE` còn sót trong môi trường không đổi được gì — không còn biến thể để chọn", async () => {
    // Trước 27/09 tên sai thì bước dựng DỪNG. Nay biến ấy không còn được đọc ở đâu: một máy CI còn
    // đặt `VIGOV_BIEN_THE=goc` phải dựng ra ĐÚNG bundle này, không phải một bản khác.
    const previous = process.env["VIGOV_BIEN_THE"];
    process.env["VIGOV_BIEN_THE"] = "goc";
    try {
      const again = fullText(await buildBundle());
      expect(again.length).toBe(bundle.length);
      expect(again).toContain(CITIZEN_CHANNEL_LABEL);
    } finally {
      if (previous === undefined) delete process.env["VIGOV_BIEN_THE"];
      else process.env["VIGOV_BIEN_THE"] = previous;
    }
  }, 120_000);
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
    const content = emitted.map((file) => String(file.code ?? file.source ?? "")).join("\n");
    const navy = /--navy:\s*([^;}]+)/.exec(content)?.[1]?.trim().toLowerCase();
    expect(navy, "bản dựng không chứa biến --navy ở đâu cả").toBe(BRAND_NAVY);
  });
});
