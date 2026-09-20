/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

import indexHtmlRaw from "../index.html?raw";

/**
 * THE DEFECT CLASS THIS FILE EXISTS FOR:
 *
 *   Every screen still renders, every existing test stays green, the build succeeds — and the
 *   app now collects something the review never covered. That defect is invisible until it is
 *   an incident, and no other check in this repository can see it.
 *
 *   These tests are the tripwire. They are DELIBERATELY hostile to a silent change: the red is
 *   the conversation that must happen BEFORE collection is added, not after.
 *
 * ⚠ CÁI ĐÃ ĐỔI, VÀ CÁI KHÔNG — ĐỌC BẢNG NÀY TRƯỚC KHI SỬA MỘT DÒNG NÀO Ở ĐÂY.
 *
 *   Tệp này ra đời khi app "không thu thập gì cả": không đăng nhập, không `getPhoneNumber`,
 *   không một lời gọi máy chủ nào. Hai trong ba điều ấy nay KHÔNG CÒN ĐÚNG — ứng dụng có đăng
 *   nhập một chạm (ADR 0020) và CÓ một lời gọi máy chủ, ở CẢ BẢN NỘP.
 *
 *   Mỗi lần một ranh giới bị bước qua, lệnh cấm tương ứng được **THU HẸP**, không bị gỡ, và
 *   luôn kèm một ca chứng minh nó còn bắt được ở ngoài phạm vi mới:
 *
 *     `zmp-sdk` + 10 lời gọi nền tảng  →  miễn ĐÚNG MỘT THƯ MỤC (`features/tinh-nang/`)
 *     `fetch`/XHR/WebSocket/…          →  miễn ĐÚNG MỘT TỆP (`features/dang-nhap/goi-may-chu.ts`)
 *     `getUserInfo`/`getSetting`/`authorize` · lưu trữ · `serverUploadUrl` · geolocation · ô nhập
 *                                      →  KHÔNG miễn cho gì cả, không một dòng nào
 *
 *   MỘT LỆNH CẤM BỊ XOÁ LÀ MỘT LỆNH CẤM KHÔNG AI BIẾT LÀ ĐÃ MẤT. Đó là lý do chưa lệnh cấm nào
 *   trong tệp này biến mất, kể cả khi lý do ban đầu của nó đã hết.
 */

const RAW_SOURCES = import.meta.glob("./**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

/**
 * Comments are stripped before scanning. Several files EXPLAIN in prose that they call no
 * `getPhoneNumber` and open no form; scanning raw text would make those explanations trip the
 * very rule they describe, and a test that is red for a false reason gets disabled.
 * `//` preceded by `:` is left alone so `https://…` inside a string literal survives.
 */
function withoutComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");
}

const PRODUCTION_SOURCES = Object.entries(RAW_SOURCES)
  .filter(([path]) => !path.includes(".test."))
  .map(([path, source]) => ({ path, code: withoutComments(source) }));

/** Hình dạng một số di động Việt Nam sau khi đã bỏ dấu cách và dấu ngăn. */
const SO_DI_DONG = /(^|\D)0[35789]\d{8}(\D|$)/;

/**
 * DẢI SỐ GIẢ ĐÃ THOẢ THUẬN — luật 3, bất biến 5: ví dụ và dữ liệu mẫu dùng `0900000000`.
 *
 * Đây là NGOẠI LỆ DUY NHẤT của phép quét dưới, và nó hẹp có chủ đích: đúng `090000000` cộng một
 * chữ số. Danh mục xã của bản trình diễn cần tám số khác nhau (`demo-danh-muc-xa.ts`), nên biến
 * thể `090000000x` được mở; mọi hình dạng khác vẫn là một số thật cho tới khi chứng minh ngược
 * lại, và một số thật lọt vào một ứng dụng đã xuất bản là sự cố không thu hồi được.
 *
 * Mở rộng dải này là một quyết định về dữ liệu cá nhân, không phải một chỉnh sửa test.
 */
const DAI_SO_GIA = /090000000\d/g;

/** Bỏ dấu cách/dấu ngăn rồi bỏ dải số giả, còn lại là những chữ số phải giải trình. */
function chiSoConLai(ma: string): string {
  return ma.replace(/[\s.\-()]/g, "").replace(DAI_SO_GIA, "SO-GIA");
}

type Tripwire = {
  /** What a reader of a failure needs to know: what was found and what to do about it. */
  what: string;
  pattern: RegExp;
  /**
   * TIỀN TỐ ĐƯỜNG DẪN DUY NHẤT được phép chứa thứ này — một THƯ MỤC, hoặc ĐÚNG MỘT TỆP. Không
   * khai thì cấm ở mọi tệp.
   *
   * ⚠ ĐÂY LÀ THU HẸP PHẠM VI, KHÔNG PHẢI GỠ LỆNH CẤM — và khác biệt ấy là toàn bộ vấn đề.
   *
   *   Biến thể `quyen` là bản nộp XIN QUYỀN: Zalo chỉ cấp `getPhoneNumber` · `getLocation` ·
   *   `scanQRCode` khi bản nộp có chỗ dùng chúng nhìn thấy được. Nên ranh giới giai đoạn 1 ở
   *   đúng ba lời gọi ấy được bước qua **có chủ đích**, và bước qua ở **đúng một thư mục**.
   *
   *   Xoá hẳn lệnh cấm thì một `getPhoneNumber` xuất hiện trong `App.tsx` sáu tuần nữa sẽ không
   *   có gì đỏ lên. Một lệnh cấm bị xoá là một lệnh cấm không ai biết là đã mất — nên nó ở lại,
   *   hẹp đi, và có ca kiểm ở cuối tệp chứng minh nó vẫn bắt được ở ngoài thư mục ấy.
   */
  chi_trong?: string;
};

/**
 * Thư mục của ba tính năng. Một hằng, vì cả lệnh cấm lẫn ca kiểm về lệnh cấm đều đọc nó.
 *
 * ĐỔI TÊN, KHÔNG MỞ RỘNG. `features/quyen/` đổi tên thành `features/tinh-nang/` khi ba màn quyền
 * trở thành ba TÍNH NĂNG thật của ứng dụng sản phẩm. Phạm vi miễn vẫn đúng bằng MỘT thư mục, và
 * ca kiểm ở cuối tệp vẫn cho lệnh cấm ăn một vi phạm đặt ngoài nó.
 */
const THU_MUC_TINH_NANG = "./features/tinh-nang/";

/**
 * TỆP DUY NHẤT ĐƯỢC GỌI MẠNG — một TỆP, không phải một thư mục, và khác biệt ấy là cả vấn đề.
 *
 * ADR 0020 chốt đăng nhập bằng `getPhoneNumber`, và hai mã lấy được chỉ đổi được ở MÁY CHỦ
 * (bất biến 2: khoá bí mật của Mini App không ra khỏi backend). Nên ứng dụng phải gọi được một
 * tuyến — ở CẢ HAI biến thể, kể cả bản nộp — và lệnh cấm
 * `fetch`/XHR/WebSocket/EventSource/axios được THU HẸP, không gỡ.
 *
 * VÌ SAO HẸP TỚI MỘT TỆP CHỨ KHÔNG PHẢI MỘT THƯ MỤC như `zmp-sdk`: `zmp-sdk` là một bề mặt rộng
 * mà chín lời gọi có lý do nằm cạnh nhau. Một lời gọi mạng thì không: mỗi cái là một đường dữ
 * liệu rời khỏi máy người dùng. Miễn cho cả thư mục thì lời gọi thứ hai, thứ ba mọc lên trong
 * đó mà không có gì đỏ lên; miễn cho một tệp thì mỗi lời gọi mới phải đi qua đúng chỗ có chú
 * thích nói vì sao nó được phép.
 *
 * Ca "lệnh cấm CÓ PHẠM VI vẫn bắt được vi phạm đặt NGOÀI…" ở cuối tệp cho lệnh cấm này ăn một
 * `fetch` đặt trong `features/dang-nhap/PhatHanhPhien.tsx` — tệp nằm NGAY CẠNH tệp được miễn — và
 * khẳng định nó vẫn bắt. Không có ca ấy thì `chi_trong` có thể bị sửa thành thư mục và không
 * có gì báo.
 */
const TEP_GOI_MANG = "./features/dang-nhap/goi-may-chu.ts";

/** Tệp vi phạm một dây bẫy: khớp mẫu, và KHÔNG nằm trong thư mục được miễn. */
function viPham(
  day: Tripwire,
  tep: readonly { path: string; code: string }[],
): string[] {
  return tep
    .filter((f) => day.pattern.test(f.code))
    .filter((f) => !(day.chi_trong !== undefined && f.path.startsWith(day.chi_trong)))
    .map((f) => f.path);
}

const TRIPWIRES: readonly Tripwire[] = [
  {
    // `getAccessToken` ĐÃ RỜI KHỎI LỆNH CẤM NÀY, SANG LỆNH CẤM CÓ PHẠM VI NGAY DƯỚI — không
    // phải bị xoá. Luồng đăng nhập của ADR 0020 cần nó, và nó chỉ được gọi trong đúng một thư
    // mục như chín lời gọi kia.
    //
    // BA TÊN CÒN LẠI KHÔNG ĐƯỢC NỚI THEO, VÀ ĐÓ LÀ CẢ Ý NGHĨA CỦA VIỆC TÁCH RA: `getUserInfo`
    // trả về TÊN và ẢNH ĐẠI DIỆN — dữ liệu cá nhân thật, không phải một mã; `authorize` là đường
    // xin thêm scope; `getSetting` đọc trạng thái quyền. Không tính năng nào trong app này cần
    // chúng, nên chúng bị cấm ở MỌI tệp, kể cả trong `features/tinh-nang/`. Có một ca kiểm riêng
    // ở cuối tệp cho đúng điều đó.
    what: "a Zalo SDK call that asks the platform for citizen data (profile, permission state)",
    pattern: /\b(getUserInfo|getSetting|authorize)\s*\(/,
  },
  {
    // BA LỜI GỌI CỦA BA TÍNH NĂNG, và chỉ trong `src/features/tinh-nang/`. Ở mọi tệp khác chúng
    // vẫn bị cấm y như trước: một `getPhoneNumber` trong `App.tsx` là thu thập dữ liệu ngoài
    // phạm vi đã nộp, và nó sẽ đi vào CẢ bản `goc` vì `App.tsx` không nằm sau alias.
    //
    // `scanQRCode` được THÊM VÀO lệnh cấm ở lần này — trước đây không tên nào canh nó, nên một
    // lời gọi máy ảnh lọt vào bất kỳ tệp nào mà không có gì đỏ lên.
    //
    // SÁU TÊN NỮA ĐƯỢC THÊM VÀO ĐÚNG LỆNH CẤM NÀY, KHÔNG PHẢI VÀO MỘT LỆNH CẤM MỚI, và không mở
    // rộng phạm vi sang thư mục nào khác: `getNetworkType` · `keepScreen` · `vibrate` ·
    // `requestCameraPermission` · `openMediaPicker` · `downloadFile`. Chín cái tên, MỘT thư mục.
    // Một `openMediaPicker` xuất hiện trong `App.tsx` là mở cửa sổ chọn ảnh ngoài phạm vi đã
    // nộp, và nó sẽ đi vào CẢ bản `goc` vì `App.tsx` không nằm sau alias.
    // `getAccessToken` LÀ TÊN THỨ MƯỜI, THÊM VÀO LẦN NÀY. Khối `getPhoneNumber` trên màn Liên hệ
    // nay là KHỐI ĐĂNG NHẬP (ADR 0020), và luồng ấy gửi đi hai mã: mã số điện thoại và access
    // token của phiên Zalo. Nó chuyển từ lệnh cấm tuyệt đối ở trên xuống đây, tức là **được phép
    // trong đúng một thư mục** — không phải được phép ở mọi nơi. Số QUYỀN phải xin ở Developer
    // Console vẫn là chín: `index.d.ts` dòng 3009 ghi rằng từ SDK 2.35.0 lời gọi này không cần
    // người dùng xác nhận.
    what:
      'a platform call outside "src/features/tinh-nang/" — those ten are the ONLY platform ' +
      "calls this app makes, and they live in exactly one directory",
    pattern:
      /\b(getPhoneNumber|getAccessToken|getLocation|scanQRCode|getNetworkType|keepScreen|vibrate|requestCameraPermission|openMediaPicker|downloadFile)\s*\(/,
    chi_trong: THU_MUC_TINH_NANG,
  },
  {
    /**
     * `serverUploadUrl` — CẤM Ở MỌI TỆP, KHÔNG MIỄN CHO THƯ MỤC NÀO.
     *
     * Đây là lệnh cấm duy nhất trong tệp này gắt hơn cả lệnh cấm `zmp-sdk`, và có lý do:
     *
     *   `openMediaPicker` nhận một tham số tuỳ chọn `serverUploadUrl`. Truyền nó vào thì SDK tự
     *   tải ảnh người dùng vừa chọn LÊN MỘT MÁY CHỦ — không qua `fetch`, không qua `XMLHttpRequest`,
     *   nên MỌI dây bẫy còn lại trong tệp này đều không thấy gì. Một chữ thêm vào một lời gọi là
     *   đủ để ảnh một tấm danh thiếp giấy — dữ liệu cá nhân của người thứ ba — rời khỏi máy, và
     *   không có gì đỏ lên.
     *
     *   Bỏ tham số ấy đi thì `index.d.ts` dòng 4721 bảo đảm SDK trả về đường dẫn tạm trên máy và
     *   KHÔNG tải lên đâu cả. Câu "ảnh không rời khỏi máy" trong chính sách quyền riêng tư đứng
     *   được là nhờ đúng một dòng này.
     *
     * KHÔNG MIỄN CHO `src/features/tinh-nang/` vì thư mục ấy là nơi DUY NHẤT gọi `openMediaPicker`
     * — tức là nơi duy nhất tham số này có thể lọt vào. Một ngoại lệ ở đúng chỗ ấy là không có
     * lệnh cấm nào cả.
     */
    what:
      "the `serverUploadUrl` parameter of openMediaPicker — it makes the SDK upload the user's " +
      "photo to a server, bypassing every other tripwire in this file",
    pattern: /serverUploadUrl/,
  },
  {
    // LỆNH CẤM CẢ GÓI, VÀ NÓ CẤM **CHUỖI TÊN MÔ-ĐUN**, KHÔNG CẤM CÚ PHÁP NHẬP.
    //
    // Lịch sử của khe này, ghi lại vì một lệnh cấm không kể lịch sử của mình là một lệnh cấm
    // người sau sẽ nới lại y hệt — và lần đó có thể không có ai đọc kết quả của phép đo:
    //
    //   17/09  Cấm thẳng `from "zmp-sdk"`, không ngoại lệ.
    //   18/09  Nới thành DANH SÁCH TRẮNG cho `getRouteParams`, để đo một câu ADR 0018 còn treo:
    //          tham số deep link có tới app không, và `location.search` có mang đủ những gì
    //          `getRouteParams()` mang không. Lập luận lúc ấy đúng: `getRouteParams` không thu
    //          thập gì của ai, nó đọc thứ nền tảng đã đặt vào đường liên kết trước khi mã của
    //          ta chạy.
    //   18/09  **Phép đo đã xong.** Cả hai câu đều có đáp (README §"Hai thứ đã kiểm bằng cách
    //          chạy thật"), lớp khám phá đã dựng và demo được ở commit 25591e8, rồi được gỡ
    //          khỏi bản nộp. Lý do mở khe đã hết, nên khe đóng lại.
    //   18/09  Biến thể `quyen` — bản nộp XIN QUYỀN. Zalo chỉ cấp ba quyền khi bản nộp có chỗ
    //          dùng chúng nhìn thấy được, nên lệnh cấm chuyển thành **có phạm vi**: `zmp-sdk`
    //          chỉ được nhắc trong `src/features/tinh-nang/`, mọi tệp khác vẫn bị cấm như cũ. Khác
    //          lần 18/09 ở trên ở chỗ: lần ấy nới theo TÊN HÀM (một danh sách trắng đi cùng cú
    //          pháp, và nó đã chết trong im lặng); lần này thu hẹp theo THƯ MỤC, và có ca kiểm
    //          cho nó ăn một vi phạm đặt NGOÀI thư mục ấy.
    //
    // MỘT CHUỖI, KHÔNG PHẢI HAI CÚ PHÁP — và đây là bài học đắt nhất của lần trước. Bản danh
    // sách trắng đầu tiên chỉ khớp `import { X } from "zmp-sdk"`. Ngay sau đó chính mã sản phẩm
    // phải đổi sang `const { X } = await import("zmp-sdk")` (vì zmp-sdk đụng `window` lúc nhập
    // mô-đun và làm sập test chạy trong Node), và phép kiểm im lặng khớp KHÔNG GÌ CẢ: vẫn xanh,
    // vẫn trông như đang canh, và không còn canh gì. Cấm chuỗi tên mô-đun thì không hình thức
    // nhập nào đi vòng được — `import`, `await import`, `require`, nhập sâu `zmp-sdk/apis/...`
    // đều phải gõ đúng cái tên ấy ra.
    //
    // VÌ SAO KHÔNG GỠ `zmp-sdk` KHỎI package.json: ADR 0020 chốt `getPhoneNumber` là đường đăng
    // nhập của giai đoạn 2, nên SDK sẽ quay lại. Một phụ thuộc KHÔNG ĐƯỢC NHẬP thì không vào
    // bundle và không tốn gì — chính phép kiểm này là thứ giữ cho nó không được nhập.
    what:
      'a reference to the "zmp-sdk" module — importing it costs 256 kB raw / 64 kB gzip and is ' +
      "the doorway to every citizen-data API the platform offers",
    pattern: /["'`]zmp-sdk(\/[^"'`]*)?["'`]/,
    chi_trong: THU_MUC_TINH_NANG,
  },
  {
    /**
     * LỜI GỌI MẠNG — THU HẸP VỀ ĐÚNG MỘT TỆP, KHÔNG GỠ. Xem `TEP_GOI_MANG` ở trên.
     *
     * ⚠ TỆP ẤY NAY CÓ MẶT TRONG CẢ BẢN NỘP (20/09/2026) — khối đăng nhập không còn cửa biến thể,
     * cả hai bản gọi máy chủ thật. Ứng dụng vì thế KHÔNG còn nói "không gửi đi đâu" nữa; câu ấy
     * đã bị gỡ khỏi chính sách quyền riêng tư trong cùng một lượt (bản 1.3).
     *
     * Điều lệnh cấm này giữ nay là một câu KHÁC, và vẫn đáng giá y như vậy: **ứng dụng gửi đi
     * đúng MỘT việc, từ đúng MỘT tệp.** Một lời gọi mạng thứ hai — một "gửi log lỗi cho tiện",
     * một bộ đo hành vi — không lọt được vào đâu mà không đỏ lên ở đây.
     */
    what:
      'an outbound request outside "src/features/dang-nhap/goi-may-chu.ts" — that ONE file is ' +
      "the only place this app may talk to a server, and it talks to exactly one route",
    pattern: /\bfetch\s*\(|XMLHttpRequest|sendBeacon|new\s+WebSocket|new\s+EventSource|\baxios\b/,
    chi_trong: TEP_GOI_MANG,
  },
  {
    // KHÔNG NỚI MỘT DÒNG NÀO, kể cả cho tệp gọi mạng: phiếu phiên nhận về sống trong `useState`
    // và mất khi đóng app (xã đã chọn của lớp khám phá cũng vậy). Ghi một bearer xuống máy là
    // để nó ở lại sau khi người dùng đóng ứng dụng, trên một thiết bị có thể cho mượn — và câu
    // "không lưu lại" trong chính sách quyền riêng tư đứng được là nhờ đúng lệnh cấm này.
    what: "device-side storage of user state — the session ticket lives in memory, never on the device",
    pattern: /localStorage|sessionStorage|document\.cookie|indexedDB/,
  },
  {
    what: "geolocation — GPS in this system may suggest a commune and never decide one, and phase 1 has no commune at all",
    pattern: /navigator\.geolocation/,
  },
  {
    what: "an input control — a form is a collection point, and phase 1 has none",
    pattern: /<(form|input|textarea|select)[\s/>]/,
  },
];

describe("phase 1 collects nothing, and cannot start collecting quietly", () => {
  it("scans the real source tree — an empty sweep would pass for the wrong reason", () => {
    const paths = PRODUCTION_SOURCES.map((file) => file.path);
    expect(paths).toContain("./App.tsx");
    expect(paths).toContain("./main.tsx");
    expect(paths).toContain("./content/company-profile.ts");
    expect(paths.length).toBeGreaterThanOrEqual(8);

    // THƯ MỤC ĐƯỢC MIỄN PHẢI NẰM TRONG LƯỢT QUÉT. Một ngoại lệ trỏ vào một thư mục không được
    // quét thì không miễn gì cả — và ngày thư mục ấy đổi tên, lệnh cấm có phạm vi trở thành lệnh
    // cấm toàn phần mà không ai biết, hoặc ngược lại.
    expect(paths).toContain(`${THU_MUC_TINH_NANG}zalo-api.ts`);
    // Cùng lý do, cho ngoại lệ hẹp nhất trong tệp này: một tệp được miễn mà lượt quét không hề
    // đọc tới thì "miễn" và "không tồn tại" là một, và ngày nó đổi tên sẽ không có gì báo.
    expect(paths).toContain(TEP_GOI_MANG);
  });

  for (const tripwire of TRIPWIRES) {
    it(`finds no ${tripwire.what.split(" — ")[0]}`, () => {
      const offenders = viPham(tripwire, PRODUCTION_SOURCES);
      expect(
        offenders,
        `${tripwire.what}.\nPhase 1 was reviewed by Zalo as an app that collects nothing (README §"Phase 1 collects no personal data"). Adding collection changes what was submitted — raise it before writing it, do not relax this test.${
          tripwire.chi_trong === undefined
            ? ""
            : `\nBa lời gọi quyền chỉ sống trong "src${tripwire.chi_trong.slice(1)}", và ba màn ở đó chỉ HIỆN kết quả lên màn hình: các lệnh cấm fetch/XHR/WebSocket và localStorage/cookie/indexedDB KHÔNG được nới theo — chúng là thứ biến "không gửi đi đâu" thành một ràng buộc kiểm được.`
        }`,
      ).toEqual([]);
    });
  }

  it("lệnh cấm zmp-sdk bắt được MỌI hình thức nhập, không riêng hình thức đang dùng", () => {
    // MỘT PHÉP KIỂM VỀ CHÍNH DÂY BẪY, và nó tồn tại vì một sự cố đã xảy ra thật.
    //
    // Bản trước của lệnh cấm này khớp theo CÚ PHÁP `import { X } from "zmp-sdk"`. Khi mã sản
    // phẩm đổi sang `const { X } = await import("zmp-sdk")`, nó im lặng khớp không gì cả: vẫn
    // xanh, vẫn trông như đang canh, và không còn canh gì. Không có test nào đỏ để báo rằng
    // một test khác vừa chết.
    //
    // Nên hôm nay lệnh cấm khớp theo CHUỖI TÊN MÔ-ĐUN, và phép kiểm dưới đây chứng minh điều
    // đó bằng cách cho nó ăn từng hình thức nhập một. Thêm hình thức thứ tư thì thêm một dòng
    // ở đây — nếu nó lọt, dòng ấy đỏ ngay, thay vì dây bẫy chết trong im lặng.
    const cam = TRIPWIRES.find((t) => t.what.includes("zmp-sdk"))!.pattern;
    const HINH_THUC_NHAP = [
      'import { getRouteParams } from "zmp-sdk";',
      "import zmp from 'zmp-sdk';",
      'const { getRouteParams } = await import("zmp-sdk");',
      'const sdk = require("zmp-sdk");',
      'import { getPhoneNumber } from "zmp-sdk/apis";',
      'import "zmp-sdk/dist/style.css";',
      "await import(`zmp-sdk`);",
    ];
    for (const dong of HINH_THUC_NHAP) {
      expect(cam.test(dong), `lệnh cấm zmp-sdk không bắt được: ${dong}`).toBe(true);
    }

    // Và nó KHÔNG được kêu oan: một dây bẫy kêu sai chỗ bị tắt nhanh y như một dây bẫy câm.
    for (const dong of ['import { useState } from "react";', "// zmp-sdk sẽ quay lại ở giai đoạn 2"]) {
      expect(cam.test(dong), `lệnh cấm zmp-sdk kêu oan ở: ${dong}`).toBe(false);
    }
  });

  it("lệnh cấm CÓ PHẠM VI vẫn bắt được vi phạm đặt NGOÀI src/features/tinh-nang/", () => {
    // CA KIỂM VỀ CHÍNH CÁI NGOẠI LỆ VỪA MỞ — cùng lý do với ca "miễn ĐÚNG dải số giả" bên dưới.
    //
    // Thu hẹp phạm vi một lệnh cấm trông y hệt gỡ nó: cả hai đều làm lượt quét xanh trở lại. Thứ
    // phân biệt hai việc ấy là một ca cho lệnh cấm ăn một vi phạm đặt ở NGOÀI thư mục được miễn
    // và khẳng định nó vẫn bắt. Không có ca này thì sáu tuần nữa `chi_trong` có thể bị sửa thành
    // `"./"` và không có gì đỏ lên.
    const co_pham_vi = TRIPWIRES.filter((t) => t.chi_trong !== undefined);
    expect(co_pham_vi.length, "không còn lệnh cấm có phạm vi nào để kiểm").toBe(3);

    const VI_PHAM = [
      { path: "./App.tsx", code: 'const { token } = await getPhoneNumber();' },
      { path: "./App.tsx", code: 'await getLocation();' },
      { path: "./lib/launch-params.ts", code: "await scanQRCode();" },
      { path: "./components/TabBar.tsx", code: 'const sdk = await import("zmp-sdk");' },
      // Sát bên thư mục được miễn, nhưng không nằm trong nó: tiền tố phải khớp cả dấu `/`.
      { path: "./features/tinh-nang-cu.ts", code: "await getPhoneNumber();" },
      // SÁU TÊN THÊM VÀO LẦN NÀY, mỗi tên một dòng. Thiếu một dòng ở đây là một quyền không ai
      // canh, và lần lọt đầu tiên của nó sẽ là trong một tệp không ai ngờ tới.
      { path: "./App.tsx", code: "await getNetworkType();" },
      { path: "./App.tsx", code: "await keepScreen({ keepScreenOn: true });" },
      { path: "./components/TabBar.tsx", code: "await vibrate();" },
      { path: "./lib/launch-params.ts", code: "await requestCameraPermission();" },
      { path: "./App.tsx", code: 'await openMediaPicker({ type: "photo" });' },
      { path: "./main.tsx", code: 'await downloadFile({ url: "https://vidu.vn/a.pdf" });' },
      // TÊN THỨ MƯỜI: `getAccessToken` nay được phép trong `features/tinh-nang/`, và chỉ ở đó.
      { path: "./App.tsx", code: "const ma = await getAccessToken();" },

      // LỜI GỌI MẠNG — NGOẠI LỆ HẸP NHẤT TRONG TỆP NÀY, và ba dòng dưới là thứ chứng minh nó
      // hẹp đúng bằng MỘT TỆP. Dòng thứ hai nằm trong chính thư mục của tệp được miễn, ngay
      // cạnh nó: nếu `chi_trong` bị sửa thành thư mục, đúng dòng ấy đỏ lên.
      { path: "./App.tsx", code: 'await fetch("https://vidu.vn/a");' },
      { path: "./features/dang-nhap/PhatHanhPhien.tsx", code: 'await fetch("https://vidu.vn/a");' },
      { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: "new XMLHttpRequest();" },
      { path: "./lib/launch-params.ts", code: 'navigator.sendBeacon("/do", d);' },
      { path: "./components/TabBar.tsx", code: 'new WebSocket("wss://vidu.vn");' },
      { path: "./main.tsx", code: "import axios from 'axios';" },
    ];
    for (const tep of VI_PHAM) {
      const bat = co_pham_vi.some((day) => viPham(day, [tep]).length === 1);
      expect(bat, `lệnh cấm có phạm vi không bắt được: ${tep.path} — ${tep.code}`).toBe(true);
    }

    // Và ở đúng chỗ được miễn thì được phép — nếu không thì sáu tính năng và khối đăng nhập
    // không dựng nổi, và một dây bẫy chặn cả mã sản phẩm là một dây bẫy sắp bị ai đó tắt.
    const TRONG = [
      { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: 'await (await import("zmp-sdk")).getPhoneNumber();' },
      { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: "await scanQRCode();" },
      { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: 'await openMediaPicker({ type: "photo" });' },
      { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: "await keepScreen({ keepScreenOn: false });" },
      { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: "const ma = await sdk.getAccessToken();" },
      { path: TEP_GOI_MANG, code: 'await fetch(dia_chi, { method: "POST" });' },
    ];
    for (const tep of TRONG) {
      for (const day of co_pham_vi) {
        expect(viPham(day, [tep]), `miễn không ăn trong chính thư mục của nó: ${tep.code}`).toEqual(
          [],
        );
      }
    }
  });

  it("ba lệnh cấm KHÔNG có phạm vi vẫn bắt ngay trong thư mục được miễn của những lệnh cấm khác", () => {
    // CA VỀ THỨ ĐÃ **KHÔNG** ĐƯỢC NỚI, và nó tồn tại vì lần thu hẹp này đã tách một tên ra khỏi
    // một lệnh cấm tuyệt đối: `getAccessToken` rời sang lệnh cấm có phạm vi để luồng đăng nhập
    // của ADR 0020 chạy được. Ba tên ở lại thì KHÔNG được đi theo, và "không được đi theo" phải
    // có một ca cho nó ăn một vi phạm — đặt ngay trong thư mục mà chín lời gọi kia được phép.
    //
    //   getUserInfo → tên và ảnh đại diện: dữ liệu cá nhân thật, không phải một mã;
    //   authorize   → xin thêm scope, tức mở rộng đúng thứ vòng duyệt đã cấp;
    //   getSetting  → đọc trạng thái quyền của người dùng.
    //
    // Cùng ca ấy ghim lệnh cấm lưu trữ: phiếu phiên đăng nhập sống trong `useState`, và lệnh
    // cấm `localStorage`/`sessionStorage`/`cookie`/`indexedDB` không được nới cho một tệp nào —
    // kể cả cho chính tệp gọi mạng.
    const TUYET_DOI = TRIPWIRES.filter((t) => t.chi_trong === undefined);
    expect(TUYET_DOI.length, "một lệnh cấm tuyệt đối vừa được cho một phạm vi").toBe(5);

    const VI_PHAM = [
      { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: "const me = await getUserInfo();" },
      { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: 'await authorize({ scopes: ["scope.userInfo"] });' },
      { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: "const q = await getSetting();" },
      { path: TEP_GOI_MANG, code: 'localStorage.setItem("phien", phien.token);' },
      { path: TEP_GOI_MANG, code: 'document.cookie = "phien=" + token;' },
      { path: "./features/dang-nhap/PhatHanhPhien.tsx", code: "sessionStorage.setItem(k, v);" },
      { path: "./App.tsx", code: "indexedDB.open('phien');" },
    ];
    for (const tep of VI_PHAM) {
      const bat = TUYET_DOI.some((day) => viPham(day, [tep]).length === 1);
      expect(bat, `lệnh cấm tuyệt đối không bắt được: ${tep.path} — ${tep.code}`).toBe(true);
    }
  });

  it("lệnh cấm `serverUploadUrl` KHÔNG miễn cho thư mục tính năng — đó là chỗ duy nhất nó lọt được", () => {
    // Ca kiểm về chính lệnh cấm mới. `openMediaPicker` chỉ được gọi trong `features/tinh-nang/`,
    // nên một ngoại lệ cho đúng thư mục ấy sẽ là không có lệnh cấm nào cả. Ca này cho nó ăn một
    // vi phạm đặt NGAY TRONG thư mục được miễn của mọi lệnh cấm khác, và khẳng định nó vẫn bắt.
    const cam = TRIPWIRES.find((t) => t.what.includes("serverUploadUrl"));
    expect(cam, "lệnh cấm serverUploadUrl đã biến mất").toBeDefined();
    expect(cam!.chi_trong, "ai đó vừa miễn serverUploadUrl cho một thư mục").toBeUndefined();

    const VI_PHAM = [
      {
        path: `${THU_MUC_TINH_NANG}zalo-api.ts`,
        code: 'await openMediaPicker({ type: "photo", serverUploadUrl: "https://vidu.vn/upload" });',
      },
      { path: "./App.tsx", code: "const serverUploadUrl = DIA_CHI;" },
    ];
    for (const tep of VI_PHAM) {
      expect(viPham(cam!, [tep]), `lọt serverUploadUrl ở ${tep.path}`).toEqual([tep.path]);
    }

    // Và nó không kêu oan ở lời gọi ĐÚNG — lời gọi mà chính mã sản phẩm đang dùng.
    expect(
      viPham(cam!, [
        { path: `${THU_MUC_TINH_NANG}zalo-api.ts`, code: 'await openMediaPicker({ type: "photo" });' },
      ]),
    ).toEqual([]);
  });

  it("keeps personal data out of the source itself, not only out of the content file", () => {
    // company-profile.test.ts sweeps the exported strings. A number typed into a component, a
    // comment or a fixture is outside that sweep and inside the shipped bundle.
    for (const file of PRODUCTION_SOURCES) {
      const digits = chiSoConLai(file.code);
      expect(digits, `${file.path} contains a Vietnamese mobile number`).not.toMatch(SO_DI_DONG);
      expect(digits, `${file.path} contains a 12-digit identity number`).not.toMatch(
        /(^|\D)\d{12}(\D|$)/,
      );
    }
  });

  it("miễn ĐÚNG dải số giả đã thoả thuận, và không miễn gì thêm", () => {
    // MỘT PHÉP KIỂM VỀ CHÍNH CÁI NGOẠI LỆ VỪA MỞ. Một ngoại lệ không có phép kiểm là một ngoại lệ
    // sẽ được nới rộng bởi người sau — và lần nới ấy đưa một số thật vào một ứng dụng đã xuất
    // bản, tức một sự cố dữ liệu cá nhân không thu hồi được.
    expect(chiSoConLai('const so = "0900000000";')).not.toMatch(SO_DI_DONG);
    expect(chiSoConLai('const so = "0900000007";')).not.toMatch(SO_DI_DONG);
    // Dấu cách để đọc cũng nằm trong dải ấy: phép quét bỏ khoảng trắng trước khi so.
    expect(chiSoConLai('const so = "0900 000 003";')).not.toMatch(SO_DI_DONG);

    // Còn lại thì vẫn bị bắt, kể cả những số chỉ lệch dải giả một chữ số.
    for (const that of ["0901000000", "0912345678", "0387654321", "0777123456", "0900000012"]) {
      expect(chiSoConLai(`const so = "${that}";`), `lọt số ${that}`).toMatch(SO_DI_DONG);
    }
  });
});

/** Comments stripped for the same reason as above: index.html explains what it does NOT do. */
const indexHtml = indexHtmlRaw.replace(/<!--[\s\S]*?-->/g, " ");

describe("the page shell collects nothing either", () => {
  it("opens no form and no field", () => {
    expect(indexHtml).not.toMatch(/<(form|input|textarea|select)[\s/>]/);
  });

  it("loads no third-party script or stylesheet", () => {
    // A remote script is data leaving the device on every launch — the device identity, the IP,
    // the time of use — with no way to say what was sent. `src="/src/main.tsx"` is the local
    // entry Vite rewrites at build time.
    const remote = indexHtml.match(/(?:src|href)="(https?:)?\/\/[^"]*"/g) ?? [];
    expect(remote, "index.html pulls something from a third-party origin").toEqual([]);
  });

  it("keeps the root element main.tsx mounts into", () => {
    // main.tsx throws when `#app` is missing, and a Mini App that throws at start-up is
    // indistinguishable from one that crashed: a white screen on a real device.
    const mountId = /getElementById\(\s*["']([^"']+)["']\s*\)/.exec(
      RAW_SOURCES["./main.tsx"] ?? "",
    )?.[1];
    expect(mountId, "main.tsx no longer mounts by id").toBeDefined();
    expect(indexHtml).toContain(`id="${mountId}"`);
  });
});
