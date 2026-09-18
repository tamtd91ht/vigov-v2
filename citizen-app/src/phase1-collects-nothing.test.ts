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
  // (Lệnh cấm nhập `zmp-sdk` chuyển xuống một phép kiểm riêng ở cuối tệp: nó không còn là
  //  "có hay không" mà là "được nhập đúng những gì" — xem §DANH SÁCH TRẮNG.)
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

  it("nhập từ zmp-sdk đúng những gì nằm trong DANH SÁCH TRẮNG, không hơn", () => {
    // §DANH SÁCH TRẮNG — nới đúng một khe, ngày 18/09/2026, và khe này CHẶT HƠN lệnh cấm cũ.
    //
    // Trước đây tệp này cấm thẳng `from "zmp-sdk"`, không ngoại lệ. Nhưng MỤC ĐÍCH của nó là
    // "giai đoạn 1 không thu thập dữ liệu cá nhân", mà `getRouteParams` không thu thập gì của
    // ai: nó đọc tham số chính nền tảng đưa vào lúc mở app — thứ đã nằm trong đường liên kết
    // trước khi app kịp chạy. Giữ lệnh cấm cũ thì phép đo mà ADR 0018 còn treo (tham số deep
    // link có tới app không, có giữ khi app chạy nền không) không làm được, trong khi rủi ro
    // nó chặn bằng không.
    //
    // Một rào đỏ vì thứ KHÔNG vi phạm mục đích của chính nó là rào sắp bị ai đó tắt — bài học
    // đã trả giá hai lần trong ngày 17/09 với `drift_guard`.
    //
    // Vì sao danh sách trắng chặt hơn lệnh cấm cũ: lệnh cấm cũ trả lời "có nhập hay không",
    // danh sách trắng trả lời "nhập đúng cái gì". Thêm `getPhoneNumber` vào cùng dòng import
    // đó thì lệnh cấm cũ vẫn chỉ báo một lỗi chung; danh sách trắng gọi đích danh tên vừa lọt.
    const CHO_PHEP = new Set(["getRouteParams"]);
    const viPham: string[] = [];
    for (const file of PRODUCTION_SOURCES) {
      // Nhập sâu (`zmp-sdk/apis/...`) đi vòng qua phép kiểm tên — chặn thẳng.
      if (/from\s*["']zmp-sdk\/[^"']+["']/.test(file.code)) {
        viPham.push(`${file.path}: nhập sâu vào zmp-sdk`);
      }
      // HAI HÌNH THỨC NHẬP, và bỏ sót hình thức thứ hai là để dây bẫy thành vô hại.
      //
      // Bản đầu của phép kiểm này chỉ khớp `import { X } from "zmp-sdk"`. Ngay sau đó chính
      // tôi phải đổi sang `const { X } = await import("zmp-sdk")` — vì zmp-sdk đụng `window`
      // lúc nhập module và làm sập test chạy trong Node — và phép kiểm im lặng khớp KHÔNG GÌ
      // CẢ. Nó vẫn xanh, vẫn trông như đang canh, và không còn canh gì.
      //
      // Đây là lần thứ ba trong hai ngày một bản vá tự tạo ra một thể hiện mới của đúng cái
      // lỗi nó đang vá. Ghi ra để người sau thêm hình thức nhập thứ ba thì nhớ quay lại đây.
      const nhap = [
        // `[^{}]*` chứ không phải `[^}]*`: bản đầu nuốt cả dấu `{` của `try {` đứng trước và
        // báo vi phạm ở một chỗ không có vi phạm nào. Một dây bẫy kêu sai chỗ cũng bị tắt
        // nhanh y như một dây bẫy câm.
        ...file.code.matchAll(/import\s*\{([^{}]*)\}\s*from\s*["']zmp-sdk["']/g),
        ...file.code.matchAll(/\{([^{}]*)\}\s*=\s*await\s+import\(\s*["']zmp-sdk["']\s*\)/g),
      ];
      for (const m of nhap) {
        for (const ten of m[1]!.split(",").map((s) => s.trim().split(/\s+as\s+/)[0]!.trim())) {
          if (ten && !CHO_PHEP.has(ten)) viPham.push(`${file.path}: ${ten}`);
        }
      }
    }
    expect(
      viPham,
      "một API zmp-sdk ngoài danh sách trắng đã lọt vào mã sản phẩm.\nGiai đoạn 1 được Zalo duyệt như một app KHÔNG thu thập gì. Thêm thu thập là đổi thứ đã nộp — nêu ra trước khi viết, đừng nới test này.",
    ).toEqual([]);
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
