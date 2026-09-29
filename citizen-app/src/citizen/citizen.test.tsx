/// <reference types="vite/client" />
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { submitReport, myReports, lookupReport } from "./api/vigov-client";
import { submitReportBody } from "./api/citizen-report-contract";
import { createSendAttempt } from "./api/send-attempt";
import { getVigovSession } from "./api/vigov-session";
import { vigovAddress } from "./api/vigov-address";
import type { MyReport } from "./api/citizen-report-contract";
import {
  ConfirmStep,
  Sending,
  SubmitReportScreen,
  STEP_HEADING_ID,
  SendResult,
  checkReport,
  SendError,
  EMPTY_REPORT,
} from "./screens/SubmitReportScreen";
import { CitizenChannel } from "./screens/CitizenChannel";
import {
  MY_REPORTS,
  CHANNEL_NOT_OPEN,
  SEND,
  SEND_ERROR,
  statusLabel,
  statusExplanation,
  LOOKUP,
  STATUS,
  STATUS_UNLABELLED,
} from "./screens/copy";
import * as CONTENT from "./screens/copy";
import { MyReportsScreen } from "./screens/MyReportsScreen";
import { vnDateTime } from "../lib/date-time";
import { LookupResult, ReportLookupScreen } from "./screens/ReportLookupScreen";

/**
 * KÊNH CÔNG DÂN, VỚI NGUỒN PHIÊN THẬT — tức là ĐÓNG. Tệp này KHÔNG giả lập phiên: mọi ca ở đây chạy
 * đúng mã nằm trong bundle hôm nay. Các ca cần một phiên giả (khoá chống trùng, thân gửi đi,
 * 201/404) ở `api/vigov-client.test.tsx`, nơi `vi.mock` thay nguồn phiên cho RIÊNG tệp ấy.
 */

const RAW = import.meta.glob("./**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

const stripComments = (code: string) =>
  code.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");

const PRODUCTION = Object.entries(RAW)
  .filter(([p]) => !p.includes(".test."))
  .map(([path, code]) => ({ path, code: stripComments(code) }));

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("nguồn phiên ViGov đóng — không một lời gọi mạng nào đi ra", () => {
  it("nguồn phiên trả `null` — dù host của `petitions` đã có (ADR 0046)", () => {
    expect(getVigovSession()).toBeNull();
    // Host đã chốt; cổng DUY NHẤT còn giữ yêu cầu lại là phiên. Ca ngay dưới chứng minh điều đó.
    expect(vigovAddress("petitions", "/api/v1/my-citizen-reports")).toBe(
      "https://petitions.api.vigov.vn/api/v1/my-citizen-reports",
    );
  });

  it("một tên dịch vụ ngoài bảng ra RỖNG, không ra `undefined/…` hay khoá của `Object.prototype`", () => {
    // `identity` từng đứng đầu danh sách này; nó vào bảng host ngày 27/09/2026 (màn xác nhận xã,
    // danh bạ). `platform` giữ chỗ ấy: một dịch vụ CÓ THẬT mà Mini App không bao giờ được gọi thẳng.
    for (const name of ["platform", "toString", "__proto__", ""]) {
      expect(vigovAddress(name as "petitions", "/api/v1/x"), name).toBe("");
    }
  });

  it("gửi và tra cứu dừng ở `chua-co-phien`, fetch KHÔNG được gọi", async () => {
    const fake_fetch = vi.fn();
    vi.stubGlobal("fetch", fake_fetch);

    const attempt = createSendAttempt(submitReportBody({ ...EMPTY_REPORT, content: "Ổ gà trước cổng chợ" }));
    expect(await submitReport(attempt)).toEqual({ kind: "chua-co-phien" });
    expect(await lookupReport("MA-THU-01")).toEqual({ kind: "chua-co-phien" });
    expect(await lookupReport("")).toEqual({ kind: "chua-co-phien" });
    expect(await myReports("")).toEqual({ kind: "chua-co-phien" });
    expect(await myReports("c1")).toEqual({ kind: "chua-co-phien" });
    expect(fake_fetch).not.toHaveBeenCalled();
  });

  it("ba màn nói 'kênh chưa mở', không vẽ ô nhập nào, không gọi mạng", () => {
    const fake_fetch = vi.fn();
    vi.stubGlobal("fetch", fake_fetch);

    for (const screen of [
      createElement(SubmitReportScreen, { onBack: () => {} }),
      createElement(ReportLookupScreen, { onBack: () => {} }),
      createElement(ReportLookupScreen, { onBack: () => {}, initial_code: "PA7K2QX9M4TD" }),
      createElement(MyReportsScreen, { onBack: () => {}, onOpenReport: () => {}, onSubmitReport: () => {} }),
    ]) {
      const html = renderToStaticMarkup(screen);
      expect(html).toContain(CHANNEL_NOT_OPEN.title);
      expect(html).not.toMatch(/<(input|textarea|form)[\s/>]/);
      // Không nút gửi nào khi kênh đóng.
      expect(html).not.toContain(SEND.next_button);
      // Không đang tải, không "chưa gửi phản ánh nào" — chưa có phiên thì chưa biết gì cả.
      expect(html).not.toContain(MY_REPORTS.loading);
      expect(html).not.toContain(MY_REPORTS.empty);
    }
    expect(fake_fetch).not.toHaveBeenCalled();
  });

  it("màn chọn việc có lối vào 'Phản ánh của tôi'", () => {
    const html = renderToStaticMarkup(createElement(CitizenChannel, { onClose: () => {} }));
    expect(html).toContain(`<button type="button" class="cd-nut">${MY_REPORTS.title}</button>`);
  });
});

// (Ca "`index.rong.ts` chỉ nhập KIỂU" đã bị xoá 27/09/2026 cùng bản rỗng ấy: bản dựng chỉ còn một
// và nó MANG kênh công dân — `bundle-for-zalo.test.ts` đo sự có mặt ấy trên bundle thật.)

describe("bearer chỉ đến từ `api/vigov-session.ts` — kiểm tĩnh trên mã nguồn", () => {
  it("quét đúng cây mã thật của nửa nhà nước", () => {
    const paths = PRODUCTION.map((f) => f.path);
    for (const file of ["./api/vigov-session.ts", "./api/vigov-client.ts", "./api/citizen-report-contract.ts"]) {
      expect(paths).toContain(file);
    }
  });

  it("nguồn phiên KHÔNG nhập gì cả — không có đường nào để nó đọc một phiếu phiên khác", () => {
    const code = PRODUCTION.find((f) => f.path === "./api/vigov-session.ts")!.code;
    expect(code).not.toMatch(/\bimport\b|\brequire\s*\(/);
  });

  it("không tệp nào của nửa nhà nước chạm phiếu phiên hay client của `vihat-miniapp`", () => {
    // Khối đăng nhập (`features/log-in/`, `session-store`) và `src/api/` là của backend THƯƠNG MẠI.
    // Phiếu phiên của nó KHÔNG phải phiên công dân ViGov (ADR 0032).
    const BANNED = /["'`][^"'`]*(log-in|session-store|server-calls|hop-dong-yeu-cau|\.\.\/\.\.\/api\/|\.\.\/api\/dia-chi|tinh-nang|zmp-sdk)[^"'`]*["'`]/;
    const violations = PRODUCTION.filter((f) => BANNED.test(f.code)).map((f) => f.path);
    expect(violations).toEqual([]);
  });

  it("chữ `Bearer` và `Authorization` chỉ có trong tệp gọi mạng, và tệp ấy lấy token từ `getVigovSession`", () => {
    const with_bearer = PRODUCTION.filter((f) => /Bearer|Authorization/.test(f.code)).map((f) => f.path);
    expect(with_bearer).toEqual(["./api/vigov-client.ts"]);

    const call = PRODUCTION.find((f) => f.path === "./api/vigov-client.ts")!.code;
    expect(call).toMatch(/from\s+"\.\/vigov-session"/);
    expect(call).toMatch(/getVigovSession\(\)/);
    // Không hàm xuất ra nào nhận token qua tham số — một tham số là một khe nhét phiên khác vào.
    for (const match of call.matchAll(/export\s+async\s+function\s+(\w+)\s*\(([^)]*)\)/g)) {
      expect(match[2], `${match[1]} nhận một tham số giống phiên`).not.toMatch(/token|session|phien|bearer/i);
    }
  });

  it("phép kiểm tĩnh còn sống: nó bắt được một đường nhập phiếu phiên thương mại", () => {
    const BANNED = /["'`][^"'`]*(log-in|session-store)[^"'`]*["'`]/;
    expect(BANNED.test('import { useSession } from "../../features/log-in/session-store";')).toBe(true);
    expect(BANNED.test('import { getVigovSession } from "./vigov-session";')).toBe(false);
  });
});

describe("chữ của màn hình", () => {
  it("giờ hiện theo +07, KHÔNG theo múi giờ của máy", () => {
    expect(vnDateTime("2026-09-24T01:30:00Z")).toBe("24/09/2026 08:30");
    // Qua nửa đêm giờ Việt Nam: ngày cũng phải sang.
    expect(vnDateTime("2026-09-24T20:15:00Z")).toBe("25/09/2026 03:15");
    expect(vnDateTime("2026-09-24T10:00:00+07:00")).toBe("24/09/2026 10:00");
    expect(vnDateTime("khong-phai-thoi-diem")).toBeNull();
  });

  it("đủ chín trạng thái của ADR 0027, người dân thấy bốn nhóm, và `da-tiep-nhan` có câu cho người dân", () => {
    expect(Object.keys(STATUS).sort()).toEqual(
      [
        "da-tiep-nhan",
        "dang-phan-loai",
        "da-chuyen-xu-ly",
        "dang-xu-ly",
        "da-xu-ly",
        "cho-dan-xac-nhan",
        "da-dong",
        "khong-tiep-nhan",
        "chuyen-cap-tren",
      ].sort(),
    );
    expect(statusLabel("da-tiep-nhan")).toBe("Đã tiếp nhận");
    expect(statusExplanation("da-tiep-nhan")).toContain("chờ cán bộ");
    // Mã lạ: không hiện mã thô cho người dân.
    expect(statusLabel("ma-la")).not.toContain("ma-la");
    // PINNED (ADR 0050 #5 in the shared app): an unknown code is the neutral sentence, NOT a guessed group.
    // "Đã đóng" on a ticket that may still be open tells the citizen the commune has stopped working on it.
    expect(statusLabel("ma-la")).toBe(STATUS_UNLABELLED);
    for (const label of ["Đã đóng", "Đã tiếp nhận", "Đang xử lý", "Đã xử lý xong"]) {
      expect(statusLabel("ma-la")).not.toBe(label);
    }
    // Inherited keys are not codes: `groupOf` reads own keys only.
    expect(statusLabel("toString")).toBe(STATUS_UNLABELLED);
    // The nine staff labels that were merged away no longer reach the citizen.
    expect(statusLabel("cho-dan-xac-nhan")).toBe("Đã xử lý xong");
    expect(statusLabel("khong-tiep-nhan")).toBe("Đã đóng");
  });

  it("nội dung bắt buộc, độ dài theo máy chủ, và ẩn danh không tính họ tên", () => {
    expect(checkReport(EMPTY_REPORT)).toBe(SEND.missing_content);
    expect(checkReport({ ...EMPTY_REPORT, content: "   \n " })).toBe(SEND.missing_content);
    expect(checkReport({ ...EMPTY_REPORT, content: "Đèn đường hỏng" })).toBeNull();
    expect(checkReport({ ...EMPTY_REPORT, content: "ă".repeat(4001) })).not.toBeNull();
    expect(checkReport({ ...EMPTY_REPORT, content: "ă".repeat(4000) })).toBeNull();
    expect(
      checkReport({ ...EMPTY_REPORT, content: "x", full_name: "a".repeat(201), anonymous: true }),
    ).toBeNull();
  });

  it("không câu chữ nào của nửa nhà nước là một mã lỗi hay tên trường kỹ thuật", () => {
    const content = PRODUCTION.find((f) => f.path === "./screens/copy.ts")!.code;
    const strings = [...content.matchAll(/"([^"\n]{12,})"/g)].map((m) => m[1]!);
    expect(strings.length).toBeGreaterThan(20);
    for (const c of strings) {
      expect(c, `câu có mã lỗi: ${c}`).not.toMatch(/\b(4\d\d|5\d\d)\b|error|content|reporter_/i);
    }
  });

  it("không câu nào dạy việc 'đổi xã' — một phiên, một xã, không có lối đổi (ADR 0044)", () => {
    // Đọc GIÁ TRỊ lúc chạy, không đọc mã nguồn: một câu dựng từ hàm vẫn bị quét.
    const collect = (v: unknown): string[] =>
      typeof v === "string"
        ? [v]
        : typeof v === "function"
          ? collect(v(...Array<string>(v.length).fill("mẫu")))
          : v && typeof v === "object"
            ? Object.values(v).flatMap(collect)
            : [];
    const CHANGE_COMMUNE = /đổi xã/i;
    const text = collect(CONTENT);
    expect(text.length).toBeGreaterThan(40);
    expect(text.filter((c) => CHANGE_COMMUNE.test(c))).toEqual([]);
    // Phép kiểm còn sống: câu cũ bị bắt.
    expect(CHANGE_COMMUNE.test("hãy quay lại và Đổi xã trước khi gửi")).toBe(true);
  });

  it("không `console.*` ở bất kỳ tệp nào của nửa nhà nước (luật 3)", () => {
    expect(PRODUCTION.filter((f) => /\bconsole\s*\./.test(f.code)).map((f) => f.path)).toEqual([]);
  });
});

/**
 * TRÌNH ĐỌC MÀN HÌNH — kiểm trên markup từng bước. `renderToStaticMarkup` không chạy hiệu ứng nên
 * việc TIÊU ĐIỂM THẬT SỰ DỜI tới đầu bước KHÔNG được kiểm ở đây (cần bấm, mà khung gắn tự dựng của
 * `screens/no-session-effects.test.tsx` không mô phỏng sự kiện). Ở đây chỉ chứng minh: mỗi bước CÓ chỗ
 * nhận tiêu điểm đúng `id` hiệu ứng tìm, và thông báo nằm đúng vùng.
 */
describe("trình đọc màn hình: đổi bước và kết quả được báo ra", () => {
  const REPORT: MyReport = {
    lookup_code: "PA7K2QX9M4TD",
    status: "da-tiep-nhan",
    field: "",
    field_label: "",
    content: "Đèn đường hỏng",
    address: "",
    masked_reporter_name: "",
    masked_reporter_phone: "",
    anonymous: true,
    clock_from: "2026-09-24T01:30:00Z",
    acknowledge_due: null,
    resolve_due: null,
    result: "",
    reason: "",
    receiving_body: "",
    rating: null,
    rated_at: null,
  };

  /** Thẻ mở của phần tử mang `id` — rỗng khi không có. */
  const openingTag = (html: string, id: string) => html.match(new RegExp(`<[a-z0-9]+[^>]* id="${id}"[^>]*>`))?.[0] ?? "";

  it("mỗi bước có đúng một chỗ nhận tiêu điểm, `tabindex=\"-1\"`, mang `id` hiệu ứng tìm", () => {
    const steps: Array<[keyof typeof STEP_HEADING_ID, string]> = [
      ["xac-nhan", renderToStaticMarkup(createElement(ConfirmStep, { commune_name: "Xã Thử Nghiệm", onSend: () => {}, onEdit: () => {} }))],
      ["dang-gui", renderToStaticMarkup(createElement(Sending))],
      ["xong", renderToStaticMarkup(createElement(SendResult, { report: REPORT, onSendAnother: () => {} }))],
      ["loi", renderToStaticMarkup(createElement(SendError, { branch: "loi-mang", onResend: () => {}, onEdit: () => {} }))],
    ];
    for (const [kind, html] of steps) {
      const tag = openingTag(html, STEP_HEADING_ID[kind]);
      expect(tag, kind).not.toBe("");
      expect(tag, kind).toContain('tabindex="-1"');
      expect(html.split(`id="${STEP_HEADING_ID[kind]}"`).length - 1, kind).toBe(1);
    }
    // Bước "nhập" trỏ vào tiêu đề chung của màn — chỉ vẽ được khi có phiên, nên kiểm trên mã nguồn.
    const screen = PRODUCTION.find((f) => f.path === "./screens/SubmitReportScreen.tsx")!.code;
    expect(screen).toMatch(/<h1 className="cd-tieu-de" id=\{STEP_HEADING_ID\.nhap\} tabIndex=\{-1\}>/);
    // Hiệu ứng tìm đúng bảng ấy, theo `kind` của bước.
    expect(screen).toMatch(/getElementById\(STEP_HEADING_ID\[step\.kind\]\)\?\.focus\(\)/);
    expect(new Set(Object.values(STEP_HEADING_ID)).size).toBe(Object.keys(STEP_HEADING_ID).length);
  });

  it("'đang gửi' là một vùng `role=\"status\"`", () => {
    expect(openingTag(renderToStaticMarkup(createElement(Sending)), STEP_HEADING_ID["dang-gui"])).toContain('role="status"');
  });

  it("gửi xong: `role=\"status\"` bọc câu và mã, KHÔNG bọc tiêu đề, thẻ phiếu hay nút", () => {
    const html = renderToStaticMarkup(createElement(SendResult, { report: REPORT, onSendAnother: () => {} }));
    expect(html.split('role="status"').length - 1).toBe(1);
    const region = html.match(/<div role="status">([\s\S]*?)<\/div>/)?.[1] ?? "";
    expect(region).toContain(SEND.done_code);
    expect(region).toContain(REPORT.lookup_code);
    expect(region).not.toContain(SEND.done_title);
    expect(region).not.toContain("<button");
  });

  it("câu lỗi gửi vẫn là `role=\"alert\"`", () => {
    const tag = openingTag(
      renderToStaticMarkup(createElement(SendError, { branch: "loi-mang", onResend: () => {}, onEdit: () => {} })),
      STEP_HEADING_ID.loi,
    );
    expect(tag).toContain('role="alert"');
    expect(SEND_ERROR["loi-mang"].text.length).toBeGreaterThan(0);
  });

  it("tra cứu thấy phiếu: một câu ngắn `role=\"status\"`, thẻ phiếu nằm NGOÀI vùng ấy", () => {
    const html = renderToStaticMarkup(createElement(LookupResult, { result: { kind: "xong", report: REPORT } }));
    expect(html).toContain(`<p class="cd-cau" role="status">${LOOKUP.found}</p>`);
    expect(html.split('role="status"').length - 1).toBe(1);
    // Thẻ phiếu vẫn vẽ, và nằm SAU câu thông báo đã đóng — không lọt vào vùng `status`.
    expect(html).toContain(REPORT.content);
    expect(html.indexOf(`${LOOKUP.found}</p>`)).toBeLessThan(html.indexOf('class="cd-phieu"'));
    expect(html).not.toContain('role="alert"');
  });

  it("tra cứu không thấy: vẫn `role=\"alert\"`, không có câu 'đã tìm thấy'", () => {
    const html = renderToStaticMarkup(createElement(LookupResult, { result: { kind: "khong-thay" } }));
    expect(html).toContain('role="alert"');
    expect(html).not.toContain(LOOKUP.found);
  });
});
