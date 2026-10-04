import { describe, expect, it } from "vitest";

import { PERIOD_KINDS, periodWindows, toQueryPeriod, toRfc3339 } from "@/features/dashboard/period";
import {
  citizenReportSummaryPath,
  incomingDocumentSummaryPath,
  taskSummaryPath,
} from "@/lib/api/dashboard";

import {
  customWindows,
  INVALID_DATE,
  MISSING_DATES,
  reportComparisonNote,
  reportPeriodLabel,
  reportWindows,
  REVERSED_RANGE,
  toDateInputValue,
  zoneDayStart,
} from "./report-period";

/**
 * Tests run with TZ=UTC (`vitest.config.mts`) — deliberately NOT the business zone, so a boundary
 * computed in local time instead of Asia/Ho_Chi_Minh turns red here. 17:00 UTC is 00:00 in Viet Nam.
 */

const rfc = (ms: number) => toRfc3339(ms);

function accepted(from: string, to: string) {
  const r = customWindows(from, to);
  if (!r.ok) throw new Error(`refused: ${r.message}`);
  return r.windows;
}

describe("custom period — both days included, Asia/Ho_Chi_Minh (ADR 0053 B5e)", () => {
  it("[D1 00:00, D2+1 00:00) in Viet Nam, not in UTC", () => {
    const w = accepted("2026-09-01", "2026-09-17");
    expect(rfc(w.current.start)).toBe("2026-09-01T00:00:00+07:00");
    expect(w.current.start).toBe(Date.parse("2026-08-31T17:00:00Z"));
    expect(rfc(w.current.end)).toBe("2026-09-18T00:00:00+07:00");
    expect(w.days).toBe(17);
    expect(w.period).toEqual(w.current);
  });

  it("the comparison window is the SAME NUMBER OF DAYS immediately before, ending at D1", () => {
    const w = accepted("2026-09-01", "2026-09-17");
    expect(rfc(w.previous.start)).toBe("2026-08-15T00:00:00+07:00");
    expect(rfc(w.previous.end)).toBe("2026-09-01T00:00:00+07:00");
    expect(w.previous.end - w.previous.start).toBe(w.current.end - w.current.start);
  });

  it("a single day is one whole day, compared with the day before", () => {
    const w = accepted("2026-01-01", "2026-01-01");
    expect(w.days).toBe(1);
    expect(rfc(w.current.end)).toBe("2026-01-02T00:00:00+07:00");
    expect(rfc(w.previous.start)).toBe("2025-12-31T00:00:00+07:00");
  });

  it("crosses a leap day and a year boundary without drifting off midnight", () => {
    const w = accepted("2028-02-28", "2028-03-01");
    expect(w.days).toBe(3);
    expect(rfc(w.current.end)).toBe("2028-03-02T00:00:00+07:00");
    expect(rfc(w.previous.start)).toBe("2028-02-25T00:00:00+07:00");
  });

  it("refuses a reversed range, an empty field and a date that does not exist — no windows to send", () => {
    expect(customWindows("2026-09-17", "2026-09-01")).toEqual({ ok: false, message: REVERSED_RANGE });
    expect(customWindows("", "2026-09-01")).toEqual({ ok: false, message: MISSING_DATES });
    expect(customWindows("2026-09-01", " ")).toEqual({ ok: false, message: MISSING_DATES });
    expect(customWindows("2026-02-30", "2026-03-01")).toEqual({ ok: false, message: INVALID_DATE });
    expect(customWindows("17/09/2026", "2026-09-18")).toEqual({ ok: false, message: INVALID_DATE });
  });

  it("reportWindows never invents a period for a refused range — it throws instead", () => {
    expect(() =>
      reportWindows({ kind: "custom", from: "2026-09-17", to: "2026-09-01" }, new Date()),
    ).toThrow();
  });

  it("date-input values round-trip in Viet Nam: 23:30 UTC on the 16th is already the 17th", () => {
    expect(toDateInputValue(Date.parse("2026-09-16T23:30:00Z"))).toBe("2026-09-17");
    expect(zoneDayStart("2026-09-17")).toBe(Date.parse("2026-09-16T17:00:00Z"));
    expect(zoneDayStart("2026-13-01")).toBeNull();
  });

  it("states the period and its comparison in words", () => {
    const w = accepted("2026-09-01", "2026-09-17");
    expect(reportPeriodLabel(w)).toBe("Kỳ tuỳ chọn: 1/9/2026 – 17/9/2026 (17 ngày)");
    expect(reportComparisonNote(w)).toBe("So với cùng số ngày liền trước (17 ngày): 15/8/2026 – 31/8/2026.");
  });
});

describe("named periods — /bao-cao and /tong-quan ask the identical query (spec 13 §10)", () => {
  const now = new Date("2026-09-28T09:43:00Z");

  it.each(PERIOD_KINDS)("%s: same windows, same three summary query strings, current and previous", (kind) => {
    const report = reportWindows({ kind }, now);
    const overview = periodWindows(kind, now);
    expect(report).toEqual(overview);
    for (const path of [taskSummaryPath, incomingDocumentSummaryPath, citizenReportSummaryPath]) {
      expect(path(toQueryPeriod(report.current))).toBe(path(toQueryPeriod(overview.current)));
      expect(path(toQueryPeriod(report.previous))).toBe(path(toQueryPeriod(overview.previous)));
    }
  });

  it("keeps /tong-quan's comparison sentence for a named period (ADR 0053 B5a)", () => {
    expect(reportComparisonNote(reportWindows({ kind: "month" }, now))).toBe(
      "So với cùng khoảng thời gian đã trôi qua của kỳ trước: 00:00 01/08/2026 – 16:43 28/08/2026.",
    );
    expect(reportPeriodLabel(reportWindows({ kind: "month" }, now))).toBe("Kỳ tháng này: 1/9/2026 – 30/9/2026");
  });
});
