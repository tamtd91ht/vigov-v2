import { describe, expect, it } from "vitest";

import {
  comparisonNote,
  formatDate,
  formatDateTime,
  periodMetaLabel,
  periodWindows,
  toQueryPeriod,
  toRfc3339,
  zoneYear,
} from "./period";

/**
 * Tests run with TZ=UTC (`vitest.config.mts`) — deliberately NOT the business zone, so a boundary
 * computed in local time instead of Asia/Ho_Chi_Minh turns red here. 17:00 UTC is 00:00 in Viet Nam.
 */

const at = (iso: string) => new Date(iso);
const rfc = (ms: number) => toRfc3339(ms);

describe("month — the default period", () => {
  const w = periodWindows("month", at("2026-09-28T09:43:00Z")); // 16:43 ICT, Monday 28/9

  it("starts at 00:00 ICT on the 1st, not at 00:00 UTC", () => {
    expect(rfc(w.period.start)).toBe("2026-09-01T00:00:00+07:00");
    expect(w.period.start).toBe(Date.parse("2026-08-31T17:00:00Z"));
    expect(rfc(w.period.end)).toBe("2026-10-01T00:00:00+07:00");
  });

  it("the current window ends at now, rounded up to the next whole second", () => {
    expect(rfc(w.current.start)).toBe("2026-09-01T00:00:00+07:00");
    expect(rfc(w.current.end)).toBe("2026-09-28T16:43:01+07:00");
  });

  it("the previous window is the SAME ELAPSED LENGTH from the previous month's start", () => {
    expect(rfc(w.previous.start)).toBe("2026-08-01T00:00:00+07:00");
    expect(rfc(w.previous.end)).toBe("2026-08-28T16:43:01+07:00");
    expect(w.previous.end - w.previous.start).toBe(w.current.end - w.current.start);
  });

  it("the meta line names the whole period, last day inclusive", () => {
    expect(periodMetaLabel(w)).toBe("Kỳ tháng này: 1/9/2026 – 30/9/2026");
  });

  it("the comparison note states the previous window", () => {
    expect(comparisonNote(w)).toBe(
      "So với cùng khoảng thời gian đã trôi qua của kỳ trước: 00:00 01/08/2026 – 16:43 28/08/2026.",
    );
  });

  it("previous month of January is December of the previous year", () => {
    const j = periodWindows("month", at("2026-01-10T03:00:00Z"));
    expect(rfc(j.previous.start)).toBe("2025-12-01T00:00:00+07:00");
  });

  it("the previous window is CAPPED at the current period's start (31 March vs February)", () => {
    const m = periodWindows("month", at("2026-03-31T03:00:00Z")); // 10:00 ICT 31/3
    expect(rfc(m.previous.start)).toBe("2026-02-01T00:00:00+07:00");
    expect(rfc(m.previous.end)).toBe("2026-03-01T00:00:00+07:00");
    expect(m.previous.end).toBe(m.current.start);
  });
});

describe("zone boundaries — 17:00 UTC is 00:00 ICT", () => {
  it("16:59:59 UTC on 30/9 is still September in Viet Nam", () => {
    const w = periodWindows("month", at("2026-09-30T16:59:59Z"));
    expect(rfc(w.period.start)).toBe("2026-09-01T00:00:00+07:00");
  });

  it("17:00:00 UTC on 30/9 is already October in Viet Nam", () => {
    const w = periodWindows("month", at("2026-09-30T17:00:00Z"));
    expect(rfc(w.period.start)).toBe("2026-10-01T00:00:00+07:00");
    expect(periodMetaLabel(w)).toBe("Kỳ tháng này: 1/10/2026 – 31/10/2026");
  });

  it("at the very first instant of a period the window is not empty (servers refuse from >= to)", () => {
    const w = periodWindows("month", at("2026-09-30T17:00:00Z"));
    expect(w.current.end - w.current.start).toBe(1000);
    expect(w.previous.end - w.previous.start).toBe(1000);
  });

  it("the fiscal year follows the zone: 31/12 17:30 UTC is already next year", () => {
    expect(zoneYear(Date.parse("2026-12-31T16:59:59Z"))).toBe(2026);
    expect(zoneYear(Date.parse("2026-12-31T17:30:00Z"))).toBe(2027);
  });
});

describe("week — starts on Monday", () => {
  it("Monday 00:00 ICT opens the week", () => {
    const w = periodWindows("week", at("2026-09-27T17:00:00Z")); // Mon 28/9 00:00 ICT
    expect(rfc(w.period.start)).toBe("2026-09-28T00:00:00+07:00");
    expect(rfc(w.period.end)).toBe("2026-10-05T00:00:00+07:00");
    expect(rfc(w.previous.start)).toBe("2026-09-21T00:00:00+07:00");
  });

  it("Sunday 23:59 ICT still belongs to the week that began the previous Monday", () => {
    const w = periodWindows("week", at("2026-09-27T16:59:00Z")); // Sun 27/9 23:59 ICT
    expect(rfc(w.period.start)).toBe("2026-09-21T00:00:00+07:00");
    expect(periodMetaLabel(w)).toBe("Kỳ tuần này: 21/9/2026 – 27/9/2026");
  });

  it("a week crossing a month boundary", () => {
    const w = periodWindows("week", at("2026-10-01T02:00:00Z")); // Thu 1/10
    expect(rfc(w.period.start)).toBe("2026-09-28T00:00:00+07:00");
  });
});

describe("quarter and year", () => {
  it("September is in Q3: 1/7 – 30/9, previous from 1/4", () => {
    const w = periodWindows("quarter", at("2026-09-28T09:43:00Z"));
    expect(periodMetaLabel(w)).toBe("Kỳ quý này: 1/7/2026 – 30/9/2026");
    expect(rfc(w.previous.start)).toBe("2026-04-01T00:00:00+07:00");
  });

  it("the quarter before Q1 is Q4 of the previous year", () => {
    const w = periodWindows("quarter", at("2026-02-15T03:00:00Z"));
    expect(rfc(w.period.start)).toBe("2026-01-01T00:00:00+07:00");
    expect(rfc(w.previous.start)).toBe("2025-10-01T00:00:00+07:00");
  });

  it("year: 1/1 00:00 ICT (31/12 17:00 UTC), previous year, same elapsed length", () => {
    const w = periodWindows("year", at("2026-09-28T09:43:00Z"));
    expect(w.period.start).toBe(Date.parse("2025-12-31T17:00:00Z"));
    expect(periodMetaLabel(w)).toBe("Kỳ năm nay: 1/1/2026 – 31/12/2026");
    expect(rfc(w.previous.start)).toBe("2025-01-01T00:00:00+07:00");
    expect(rfc(w.previous.end)).toBe("2025-09-28T16:43:01+07:00");
  });
});

describe("formatting — always in Viet Nam, never the machine's zone", () => {
  it("formatDateTime", () => {
    expect(formatDateTime(Date.parse("2026-09-07T09:43:00Z"))).toBe("16:43 07/09/2026");
  });

  it("formatDate is unpadded", () => {
    expect(formatDate(Date.parse("2026-09-06T17:00:00Z"))).toBe("7/9/2026");
  });

  it("toQueryPeriod gives the RFC 3339 pair the servers parse", () => {
    const w = periodWindows("month", at("2026-09-28T09:43:00Z"));
    expect(toQueryPeriod(w.current)).toEqual({
      from: "2026-09-01T00:00:00+07:00",
      to: "2026-09-28T16:43:01+07:00",
    });
  });
});
