import { afterEach, describe, expect, it, vi } from "vitest";

import { listAutomationJobs, requestAutomationRun, saveAutomationJob } from "./automation-jobs";

/**
 * The three `automation-jobs` routes. What a working screen never shows: the PUT body carries the
 * other kinds' cadence fields as `null`, "run now" succeeds on 202 (not 200/201), neither write invents
 * an Idempotency-Key, and both 409 sentences reach the caller verbatim.
 */

function reply(status: number, body?: unknown) {
  return body === undefined
    ? new Response(null, { status })
    : new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function stubFetch(answer: () => Response) {
  const fake = vi.fn(async () => answer());
  vi.stubGlobal("fetch", fake);
  return fake;
}

function call(fake: ReturnType<typeof stubFetch>, i = 0) {
  const [path, init] = fake.mock.calls[i] as unknown as [string, RequestInit];
  return {
    path,
    method: init.method,
    credentials: init.credentials,
    headers: new Headers(init.headers),
    body: init.body === undefined ? undefined : (JSON.parse(String(init.body)) as unknown),
  };
}

const JOB = {
  job: "sla_reminders",
  schedule_kind: "interval",
  configured: true,
  enabled: true,
  interval_minutes: 15,
  min_interval_minutes: 5,
  run_hour: null,
  run_minute: null,
  weekday: null,
  timezone: "Asia/Ho_Chi_Minh",
  enabled_at: "2026-09-29T01:00:00Z",
  run_requested_at: null,
  last_runs: [],
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("listAutomationJobs — GET /api/v1/automation-jobs", () => {
  it("relative path, same-origin, no tenant anywhere; returns the items", async () => {
    const fake = stubFetch(() => reply(200, { items: [JOB] }));
    expect(await listAutomationJobs()).toEqual({ ok: true, duLieu: [JOB] });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/automation-jobs");
    expect(c.method).toBe("GET");
    expect(c.credentials).toBe("same-origin");
    expect(c.path).not.toMatch(/tenant/i);
  });

  it("403 (no admin.sla) is the server's sentence", async () => {
    stubFetch(() => reply(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." }));
    expect(await listAutomationJobs()).toEqual({ ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." });
  });
});

describe("saveAutomationJob — PUT /api/v1/automation-jobs/{job}", () => {
  it("PUTs the body as given to the job's path, no Idempotency-Key", async () => {
    const fake = stubFetch(() => reply(200, JOB));
    const body = { enabled: true, interval_minutes: 15, run_hour: null, run_minute: null, weekday: null };
    expect(await saveAutomationJob("sla_reminders", body)).toEqual({ ok: true, duLieu: JOB });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/automation-jobs/sla_reminders");
    expect(c.method).toBe("PUT");
    expect(c.body).toEqual(body);
    expect(c.headers.has("Idempotency-Key")).toBe(false);
  });

  it("a job key cannot escape its path segment", async () => {
    const fake = stubFetch(() => reply(404, { code: "automation_job_not_found", message: "Không có việc tự động hoá này." }));
    await saveAutomationJob("a/../b", { enabled: false, interval_minutes: null, run_hour: null, run_minute: null, weekday: null });
    expect(call(fake).path).toBe("/api/v1/automation-jobs/a%2F..%2Fb");
  });

  it("400 (a bound) and 409 automation_job_changed are the server's sentences, verbatim", async () => {
    const bound = "tự động hoá: nhịp nhắc việc phải từ 5 đến 10080 phút";
    stubFetch(() => reply(400, { code: "invalid_request", message: bound }));
    const empty = { enabled: true, interval_minutes: 3, run_hour: null, run_minute: null, weekday: null };
    expect(await saveAutomationJob("sla_reminders", empty)).toEqual({ ok: false, thongBao: bound });

    const changed = "Cấu hình việc này vừa được người khác lưu. Hãy tải lại trang rồi thử lại.";
    stubFetch(() => reply(409, { code: "automation_job_changed", message: changed }));
    expect(await saveAutomationJob("sla_reminders", empty)).toEqual({ ok: false, thongBao: changed });
  });
});

describe("requestAutomationRun — POST /api/v1/automation-jobs/{job}/runs", () => {
  it("POST with no body; 202 is success and carries the job", async () => {
    const marked = { ...JOB, run_requested_at: "2026-09-29T03:00:00Z" };
    const fake = stubFetch(() => reply(202, marked));
    expect(await requestAutomationRun("escalation")).toEqual({ ok: true, duLieu: marked });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/automation-jobs/escalation/runs");
    expect(c.method).toBe("POST");
    expect(c.body).toBeUndefined();
    expect(c.headers.has("Content-Type")).toBe(false);
    expect(c.headers.has("Idempotency-Key")).toBe(false);
  });

  it("200 is NOT the contract's answer — treated as a failure, not a silent success", async () => {
    stubFetch(() => reply(200, JOB));
    expect((await requestAutomationRun("escalation")).ok).toBe(false);
  });

  it("409 automation_job_disabled is the server's sentence", async () => {
    const off = "Việc này đang tắt. Bật việc trước khi yêu cầu chạy ngay.";
    stubFetch(() => reply(409, { code: "automation_job_disabled", message: off }));
    expect(await requestAutomationRun("weekly_digest")).toEqual({ ok: false, thongBao: off });
  });
});
