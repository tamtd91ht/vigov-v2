import { afterEach, describe, expect, it, vi } from "vitest";

import {
  budgetPeriodClosesPath,
  closeBudgetPeriod,
  listBudgetPeriodCloses,
  reopenBudgetPeriodClose,
} from "./budget-period-close";
import { ghiDot } from "./thu-chi"; // vi-name-ok: existing client function under test

/**
 * The three budget period close routes, plus `adjustment_reason` on the entry route.
 *
 * The failures that matter are silent ones: a POST without `Idempotency-Key` (400 on every close),
 * a year close sent with `month: null` instead of no `month`, a reason in the URL, and a 409 whose
 * sentence is rewritten on the client instead of shown as the server wrote it.
 */

function stubFetch(reply: Response) {
  const fake = vi.fn(async (_path: string, _init?: RequestInit) => reply);
  vi.stubGlobal("fetch", fake);
  return fake;
}

function json(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function sentBody(fake: ReturnType<typeof stubFetch>): Record<string, unknown> {
  const raw = fake.mock.calls[0]?.[1]?.body;
  return typeof raw === "string" ? (JSON.parse(raw) as Record<string, unknown>) : {};
}

function sentHeaders(fake: ReturnType<typeof stubFetch>): Record<string, string> {
  return (fake.mock.calls[0]?.[1]?.headers ?? {}) as Record<string, string>;
}

const CLOSE = {
  id: "01JCLOSE",
  code: "CK-2026-09-01",
  year: 2026,
  month: 9,
  scope: "month",
  revision: 1,
  active: true,
  closed_by: "CB-00123",
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("GET /api/v1/budget-period-closes", () => {
  it("path carries `year`, is relative, and names no commune", () => {
    const path = budgetPeriodClosesPath(2026);
    expect(path).toBe("/api/v1/budget-period-closes?year=2026");
    expect(path).not.toMatch(/tenant/i);
    expect(path.startsWith("/")).toBe(true);
  });

  it("reads the year's history with GET", async () => {
    const fake = stubFetch(json({ year: 2026, closes: [CLOSE] }, 200));

    const result = await listBudgetPeriodCloses(2026);

    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/budget-period-closes?year=2026");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
    expect(result).toEqual({ ok: true, duLieu: { year: 2026, closes: [CLOSE] } });
  });
});

describe("POST /api/v1/budget-period-closes", () => {
  it("month close: body {year, month}, POST, Idempotency-Key from the caller, 201", async () => {
    const fake = stubFetch(json(CLOSE, 201));

    const result = await closeBudgetPeriod({ year: 2026, month: 9 }, "key-close-1");

    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/budget-period-closes");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentHeaders(fake)["Idempotency-Key"]).toBe("key-close-1");
    expect(sentBody(fake)).toEqual({ year: 2026, month: 9 });
    expect(result.ok).toBe(true);
  });

  it("year close: `month` is ABSENT from the body, even when the caller passed null", async () => {
    const fake = stubFetch(json({ ...CLOSE, month: null, scope: "year" }, 201));

    await closeBudgetPeriod({ year: 2026, month: null }, "k");
    expect(sentBody(fake)).toEqual({ year: 2026 });
    expect(sentBody(fake)).not.toHaveProperty("month");
  });

  it("409 `budget_period_already_closed` comes back as the server's sentence, verbatim", async () => {
    const sentence = "Tháng 9/2026 đã được chốt (mã CK-2026-09-01).";
    stubFetch(json({ code: "budget_period_already_closed", message: sentence, trace_id: "01JT" }, 409));

    expect(await closeBudgetPeriod({ year: 2026, month: 9 }, "k")).toEqual({
      ok: false,
      thongBao: sentence,
    });
  });
});

describe("POST /api/v1/budget-period-closes/{code}/reopening", () => {
  it("reason in the BODY, code encoded in the path, Idempotency-Key, 200", async () => {
    const fake = stubFetch(json({ ...CLOSE, active: false, reopen_reason: "Nhập thiếu chứng từ" }, 200));

    const result = await reopenBudgetPeriodClose("CK/2026", "Nhập thiếu chứng từ", "key-reopen-1");

    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/budget-period-closes/CK%2F2026/reopening");
    expect(fake.mock.calls[0]?.[0]).not.toContain("Nh");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentHeaders(fake)["Idempotency-Key"]).toBe("key-reopen-1");
    expect(sentBody(fake)).toEqual({ reason: "Nhập thiếu chứng từ" });
    expect(result.ok).toBe(true);
  });

  it("409 `budget_period_close_reopened` and 404 are the server's sentences, verbatim", async () => {
    const reopened = "Lần chốt CK-2026-09-01 đã được mở chốt.";
    stubFetch(json({ code: "budget_period_close_reopened", message: reopened, trace_id: "x" }, 409));
    expect(await reopenBudgetPeriodClose("CK-2026-09-01", "x", "k")).toEqual({
      ok: false,
      thongBao: reopened,
    });

    vi.unstubAllGlobals();
    stubFetch(json({ code: "not_found", message: "Không tìm thấy lần chốt kỳ này.", trace_id: "x" }, 404));
    expect(await reopenBudgetPeriodClose("CK-X", "x", "k")).toEqual({
      ok: false,
      thongBao: "Không tìm thấy lần chốt kỳ này.",
    });
  });
});

describe("POST /api/v1/budget-lines/{id}/entries — adjustment entry", () => {
  const base = { date: "2026-10-02", content: "Điều chỉnh đợt tháng 9", values: { C1: 5 } };

  it("`adjustment_reason` is sent when it carries text", async () => {
    const fake = stubFetch(json({ id: "01JDOT" }, 201));

    await ghiDot("01JDONG", { ...base, adjustment_reason: "Sai số chứng từ PT-12" }, "k");
    expect(sentBody(fake)["adjustment_reason"]).toBe("Sai số chứng từ PT-12");
  });

  it("blank or null `adjustment_reason` is NOT sent — the server refuses a blank one", async () => {
    const fake = stubFetch(json({ id: "01JDOT" }, 201));
    await ghiDot("01JDONG", { ...base, adjustment_reason: "" }, "k");
    expect(sentBody(fake)).not.toHaveProperty("adjustment_reason");

    vi.unstubAllGlobals();
    const fake2 = stubFetch(json({ id: "01JDOT" }, 201));
    await ghiDot("01JDONG", { ...base, adjustment_reason: null }, "k");
    expect(sentBody(fake2)).not.toHaveProperty("adjustment_reason");
  });

  it("409 `budget_period_closed` on an entry write is the server's sentence, verbatim", async () => {
    const sentence = "Kỳ tháng 9/2026 đã chốt (mã CK-2026-09-01): không thêm, gỡ được đợt thu chi.";
    stubFetch(json({ code: "budget_period_closed", message: sentence, trace_id: "x" }, 409));

    expect(await ghiDot("01JDONG", { ...base, date: "2026-09-10" }, "k")).toEqual({
      ok: false,
      thongBao: sentence,
    });
  });
});
