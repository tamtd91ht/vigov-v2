/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

import indexHtmlRaw from "../index.html?raw";

/**
 * THE DEFECT CLASS THIS FILE EXISTS FOR:
 *
 *   Phase 1 was submitted to Zalo as an app that collects nothing — no sign-in, no
 *   `getPhoneNumber`, no OTP, no form, no backend call (README §"Phase 1 collects no personal
 *   data"). That is what makes rule 3 hold BY CONSTRUCTION rather than by argument.
 *
 *   Nothing about that is self-enforcing. Six weeks from now somebody adds a "gửi góp ý" form,
 *   or a `getPhoneNumber` to "make support easier", or a `fetch` to a staging backend left in
 *   while debugging. Every screen still renders, every existing test stays green, the build
 *   succeeds — and the app now collects personal data under a review that was granted to an app
 *   that asked for nothing. The defect is invisible until it is an incident.
 *
 *   These tests are the tripwire. They are DELIBERATELY hostile to a silent change: phase 2 will
 *   make some of them red, and that is the point — the red is the conversation that must happen
 *   before collection is added, not after. When phase 2 lands, this file is replaced with the
 *   rules of phase 2, not deleted.
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

type Tripwire = {
  /** What a reader of a failure needs to know: what was found and what to do about it. */
  what: string;
  pattern: RegExp;
};

const TRIPWIRES: readonly Tripwire[] = [
  {
    what: "a Zalo SDK call that asks the platform for citizen data (phone, profile, location, token)",
    pattern: /\b(getPhoneNumber|getUserInfo|getAccessToken|getLocation|getSetting|authorize)\s*\(/,
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
  },
  {
    what: "an outbound request — phase 1 talks to no backend, so nothing about a citizen can leave the device",
    pattern: /\bfetch\s*\(|XMLHttpRequest|sendBeacon|new\s+WebSocket|new\s+EventSource|\baxios\b/,
  },
  {
    what: "device-side storage of user state — nothing is collected, so nothing needs keeping",
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
  });

  for (const tripwire of TRIPWIRES) {
    it(`finds no ${tripwire.what.split(" — ")[0]}`, () => {
      const offenders = PRODUCTION_SOURCES.filter((file) => tripwire.pattern.test(file.code)).map(
        (file) => file.path,
      );
      expect(
        offenders,
        `${tripwire.what}.\nPhase 1 was reviewed by Zalo as an app that collects nothing (README §"Phase 1 collects no personal data"). Adding collection changes what was submitted — raise it before writing it, do not relax this test.`,
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

  it("keeps personal data out of the source itself, not only out of the content file", () => {
    // company-profile.test.ts sweeps the exported strings. A number typed into a component, a
    // comment or a fixture is outside that sweep and inside the shipped bundle.
    for (const file of PRODUCTION_SOURCES) {
      const digits = file.code.replace(/[\s.\-()]/g, "");
      expect(digits, `${file.path} contains a Vietnamese mobile number`).not.toMatch(
        /(^|\D)0[35789]\d{8}(\D|$)/,
      );
      expect(digits, `${file.path} contains a 12-digit identity number`).not.toMatch(
        /(^|\D)\d{12}(\D|$)/,
      );
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
