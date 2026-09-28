import { afterEach, describe, expect, it, vi } from "vitest";

import {
  citizenReportSummaryPath,
  fetchIncomingDocumentOverdueQueue,
  fetchOverdueTasks,
  fetchTaskSummary,
  incomingDocumentOverdueQueuePath,
  incomingDocumentSummaryPath,
  overdueCitizenReportsPath,
  overdueTasksPath,
  taskSummaryPath,
} from "./dashboard";

/**
 * The six figure routes. What is guarded: the exact parameter NAMES the handlers read (the contract
 * declares `from`/`to` optional while the handlers 400 without them), the `+` of the offset
 * surviving the query string, relative paths on the commune's own host, no `tenant_id`, and a 503
 * coming back as the server's sentence rather than as an empty queue.
 */

function stubFetch(response: Response) {
  const fake = vi.fn(async (_path: string, _init?: RequestInit) => response);
  vi.stubGlobal("fetch", fake);
  return fake;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const PERIOD = { from: "2026-09-01T00:00:00+07:00", to: "2026-09-28T16:43:01+07:00" };

function query(path: string): URLSearchParams {
  return new URL(path, "https://xa.example").searchParams;
}

describe("summary paths — `from` and `to`, verbatim, `+` encoded", () => {
  it.each([
    ["task-summary", taskSummaryPath],
    ["citizen-report-summary", citizenReportSummaryPath],
    ["incoming-document-summary", incomingDocumentSummaryPath],
  ] as const)("%s", (route, build) => {
    const path = build(PERIOD);
    expect(path.startsWith(`/api/v1/${route}?`)).toBe(true);
    expect(path).toContain("%2B07%3A00");
    expect(query(path).get("from")).toBe(PERIOD.from);
    expect(query(path).get("to")).toBe(PERIOD.to);
    expect([...query(path).keys()].sort()).toEqual(["from", "to"]);
  });
});

describe("queue paths — limit 10, nothing else", () => {
  it.each([
    ["overdue-tasks", overdueTasksPath],
    ["overdue-citizen-reports", overdueCitizenReportsPath],
    ["incoming-document-overdue-queue", incomingDocumentOverdueQueuePath],
  ] as const)("%s", (route, build) => {
    const path = build();
    expect(path).toBe(`/api/v1/${route}?limit=10`);
  });
});

describe("network calls", () => {
  it("GET, relative path, same-origin credentials, no tenant anywhere", async () => {
    const fake = stubFetch(
      new Response(
        JSON.stringify({
          in_progress: 0,
          overdue: 0,
          suspended: 0,
          completed: 0,
          on_time_sample: 0,
          on_time: 0,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    const r = await fetchTaskSummary(PERIOD);
    expect(r.ok).toBe(true);
    const [path, init] = fake.mock.calls[0] ?? [];
    expect(path?.startsWith("/api/v1/task-summary?")).toBe(true);
    expect(init?.method).toBe("GET");
    expect(init?.credentials).toBe("same-origin");
    expect(path).not.toMatch(/tenant/i);
  });

  it("a 503 of the working calendar returns the server's sentence, not an empty list", async () => {
    stubFetch(
      new Response(
        JSON.stringify({
          code: "working_calendar_unavailable",
          message: "Chưa tính được lịch làm việc của xã. Vui lòng thử lại sau.",
          trace_id: "t",
        }),
        { status: 503, headers: { "Content-Type": "application/json" } },
      ),
    );
    const r = await fetchIncomingDocumentOverdueQueue();
    expect(r).toEqual({
      ok: false,
      thongBao: "Chưa tính được lịch làm việc của xã. Vui lòng thử lại sau.",
    });
  });

  it("a network failure is a failure, not zero rows", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("network");
      }),
    );
    const r = await fetchOverdueTasks();
    expect(r.ok).toBe(false);
  });
});
