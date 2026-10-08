// @vitest-environment jsdom

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it } from "vitest";

import type { documents_letterReportOut } from "@/lib/api/schema.gen";

import { LetterReportView, REPORT_NO_TYPE_ROWS, REPORT_NO_UNIT, REPORT_NO_UNIT_ROWS, reportTitle, vietnamYear } from "./letter-report";

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
    // The disabled export with its "?".
    const exportButton = [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Xuất Excel")!;
    expect(exportButton.disabled).toBe(true);
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
