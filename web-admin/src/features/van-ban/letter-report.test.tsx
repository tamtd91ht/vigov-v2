// @vitest-environment jsdom

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { toast } from "sonner";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";
import type { documents_letterReportOut } from "@/lib/api/schema.gen";
import { PETITION_READ_PERMISSION, REPORT_EXPORT_PERMISSION } from "@/lib/quyen";

import {
  LetterReport,
  LetterReportView,
  REPORT_NO_TYPE_ROWS,
  REPORT_NO_UNIT,
  REPORT_NO_UNIT_ROWS,
  reportTitle,
  vietnamYear,
} from "./letter-report";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

const months = Array.from({ length: 12 }, (_, i) => ({ month: i + 1, received: 0, resolved: 0 }));

function mount(report: documents_letterReportOut | null): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <LetterReportView
        year={2026}
        answer={report === null ? null : { ok: true, duLieu: report }}
        units={{ pha: "xong", ten: new Map([["U1", "Văn phòng"]]) }}
      />,
    ),
  );
  return host;
}

const cards = (el: HTMLElement) =>
  [...el.querySelectorAll(".grid > div")].map((c) => [c.querySelector("p")!.textContent, c.querySelectorAll("p")[1]!.textContent]);

describe("report tab", () => {
  it("six prototype cards; null percent and average are “—”, never 0; no invented seventh card", () => {
    const el = mount({
      year: 2026,
      received: 14,
      resolved: 5,
      closed_in_processing: 3,
      in_progress: 7,
      overdue: 0,
      on_time_percent: null,
      average_days: null,
      by_type: [
        { letter_type: "khieu-nai", total: 4, resolved: 1, in_progress: 3, overdue: 0 },
        { letter_type: "kien-nghi-phan-anh", total: 9, resolved: 3, in_progress: 6, overdue: 0 },
        { letter_type: "de-nghi", total: 4, resolved: 1, in_progress: 3, overdue: 0 },
        { letter_type: "to-cao", total: 1, resolved: 0, in_progress: 1, overdue: 0 },
      ],
      by_unit: [
        { unit_id: "U1", total: 6, in_progress: 4, resolved: 2, overdue: 0, on_time_percent: null },
        { unit_id: "U-UNKNOWN", total: 1, in_progress: 1, resolved: 0, overdue: 0, on_time_percent: 50 },
        { unit_id: null, total: 2, in_progress: 2, resolved: 0, overdue: 0, on_time_percent: null },
      ],
      by_month: months,
    });
    expect(el.textContent).toContain(reportTitle(2026));
    expect(cards(el)).toEqual([
      ["14", "Tiếp nhận trong năm"],
      ["5", "Đã giải quyết"],
      ["7", "Đang xử lý"],
      ["0", "Quá hạn"],
      ["—", "Giải quyết đúng hạn"],
      ["—", "Số ngày xử lý trung bình"],
    ]);
    const unitRows = [...el.querySelectorAll("section")].find((s) => s.querySelector("h3")?.textContent === "Tiến độ xử lý theo đơn vị")!;
    const names = [...unitRows.querySelectorAll("tbody tr")].map((tr) => tr.querySelector("td")!.textContent);
    // Name from the org chart; the id when the chart does not know it; a fixed phrase for "no unit".
    expect(names).toEqual(["Văn phòng", "U-UNKNOWN", REPORT_NO_UNIT]);
    expect(unitRows.textContent).toContain("50%");
    // THEO LOẠI ĐƠN by Tổng số descending (prototype `service.py:635`); a tie keeps the server's order.
    const typeRows = [...el.querySelectorAll("section")].find((s) => s.querySelector(":scope > h3")?.textContent === "Theo loại đơn")!;
    expect([...typeRows.querySelectorAll("tbody tr")].map((tr) => tr.querySelector("td")!.textContent)).toEqual([
      "Kiến nghị, phản ánh",
      "Khiếu nại",
      "Đề nghị",
      "Tố cáo",
    ]);
    expect(REPORT_NO_UNIT).toBe("Chưa phân công");
    // No `onExport` (no permission): no export button at all, and no "?" left.
    expect([...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Xuất Excel")).toBeUndefined();
    expect(el.querySelector("[data-pending-marker]")).toBeNull();
  });

  it("an empty year: both tables say so; twelve month labels still drawn", () => {
    const el = mount({
      year: 2026,
      received: 0,
      resolved: 0,
      closed_in_processing: 0,
      in_progress: 0,
      overdue: 0,
      on_time_percent: null,
      average_days: null,
      by_type: [],
      by_unit: [],
      by_month: months,
    });
    expect(el.textContent).toContain(REPORT_NO_TYPE_ROWS);
    expect(el.textContent).toContain(REPORT_NO_UNIT_ROWS);
    expect(el.textContent).toContain("T12");
  });

  it("no year select (Q8); the current year is Vietnam's, not the machine zone's", () => {
    const el = mount(null);
    expect(el.querySelector("select")).toBeNull();
    // 31/12 18:00 UTC is already 01/01 in Vietnam.
    expect(vietnamYear(new Date("2026-12-31T18:00:00Z"))).toBe(2027);
    expect(vietnamYear(new Date("2026-12-31T16:00:00Z"))).toBe(2026);
  });

  it("loading: a placeholder, no figure", () => {
    const el = mount(null);
    expect(el.textContent).toContain("Đang tải báo cáo đơn thư");
    expect(el.textContent).not.toContain("Tiếp nhận trong năm");
  });
});

describe("report tab — “Xuất Excel” (ADR 0084 #6; report.export AND petition.read)", () => {
  type Call = { url: string; method: string };

  function server(permissions: string[], exportAnswer: () => Response): Call[] {
    const calls: Call[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init?: RequestInit) => {
        calls.push({ url, method: (init?.method ?? "GET").toUpperCase() });
        const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
        if (url === "/api/v1/sessions/current") {
          return json({
            sid: "s",
            expires_at: "2099-01-01T00:00:00Z",
            staff: { code: "CB-00009", full_name: "Cán bộ X", position: "" },
            role: { code: "r", name: "R", is_leader: false },
            permissions,
            must_change_password: false,
          });
        }
        if (url.startsWith("/api/v1/citizen-letter-report/exports")) return exportAnswer();
        if (url.startsWith("/api/v1/citizen-letter-report")) {
          return json({
            year: 2026, received: 0, resolved: 0, closed_in_processing: 0, in_progress: 0, overdue: 0,
            on_time_percent: null, average_days: null, by_type: [], by_unit: [], by_month: months,
          });
        }
        if (url.startsWith("/api/v1/org-units")) return json({ items: [] });
        return new Response("{}", { status: 404 });
      }),
    );
    return calls;
  }

  async function mountReport(): Promise<HTMLDivElement> {
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() =>
      r.render(
        <PhienProvider>
          <LetterReport />
        </PhienProvider>,
      ),
    );
    for (let i = 0; i < 5; i++) await act(async () => {});
    return host;
  }

  // jsdom has no object URLs; `saveFile` needs one. Put back after each test.
  const realCreate = URL.createObjectURL;
  const realRevoke = URL.revokeObjectURL;
  function stubObjectUrl(create: () => string) {
    URL.createObjectURL = create;
    URL.revokeObjectURL = () => {};
  }

  const exportButton = (el: HTMLElement) => [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Xuất Excel");

  afterEach(() => {
    URL.createObjectURL = realCreate;
    URL.revokeObjectURL = realRevoke;
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("DENIED: report.export without petition.read — and petition.read without report.export — draw no export button", async () => {
    server([REPORT_EXPORT_PERMISSION], () => new Response("", { status: 200 }));
    expect(exportButton(await mountReport())).toBeUndefined();
    act(() => root?.unmount());
    host?.remove();
    server([PETITION_READ_PERMISSION], () => new Response("", { status: 200 }));
    expect(exportButton(await mountReport())).toBeUndefined();
  });

  it("ALLOWED: enabled; the click downloads THE YEAR SHOWN and saves the server's file, with the prototype's toast", async () => {
    const success = vi.spyOn(toast, "success");
    // `saveFile` clicks a download anchor; jsdom would try to navigate.
    const clicked = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    const createUrl = vi.fn(() => "blob:x");
    stubObjectUrl(createUrl);
    const calls = server([REPORT_EXPORT_PERMISSION, PETITION_READ_PERMISSION], () =>
      new Response("PK", { status: 200, headers: { "Content-Disposition": `attachment; filename="bao-cao-don-thu-nam-${vietnamYear()}.xlsx"` } }),
    );
    const el = await mountReport();
    const b = exportButton(el)!;
    expect(b.disabled).toBe(false);
    await act(async () => b.click());
    for (let i = 0; i < 5; i++) await act(async () => {});
    const sent = calls.find((c) => c.url.startsWith("/api/v1/citizen-letter-report/exports"))!;
    expect(sent.url).toBe(`/api/v1/citizen-letter-report/exports?year=${vietnamYear()}`);
    expect(el.textContent).toContain(reportTitle(vietnamYear()));
    expect(createUrl).toHaveBeenCalledTimes(1);
    expect(clicked).toHaveBeenCalledTimes(1);
    expect(success).toHaveBeenCalledWith(`Đã tải bao-cao-don-thu-nam-${vietnamYear()}.xlsx.`);
  });

  it("a refusal is a toast: the prototype's sentence, then the server's verbatim; nothing is saved", async () => {
    const error = vi.spyOn(toast, "error");
    const createUrl = vi.fn(() => "blob:x");
    stubObjectUrl(createUrl);
    server([REPORT_EXPORT_PERMISSION, PETITION_READ_PERMISSION], () =>
      new Response(JSON.stringify({ code: "unavailable", message: "Chưa đọc được danh mục bộ phận.", trace_id: "" }), { status: 503 }),
    );
    const el = await mountReport();
    await act(async () => exportButton(el)!.click());
    for (let i = 0; i < 5; i++) await act(async () => {});
    expect(error).toHaveBeenCalledWith("Không xuất được báo cáo. Chưa đọc được danh mục bộ phận.");
    expect(createUrl).not.toHaveBeenCalled();
  });
});
