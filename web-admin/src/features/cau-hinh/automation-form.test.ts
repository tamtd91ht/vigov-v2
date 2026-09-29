import { describe, expect, it } from "vitest";

import type { AutomationJob } from "@/lib/api/automation-jobs";

import {
  INTERVAL_NOT_A_NUMBER,
  NO_RUNS_YET,
  TIME_MISSING,
  WEEKDAY_MISSING,
  cadenceSentence,
  draftFrom,
  jobWords,
  outcomeLabel,
  runRequestSentence,
  settingBody,
  stateSentence,
  timezoneLabel,
  triggerLabel,
  weekdayLabel,
  when,
  workKindLabel,
} from "./automation-form";

function job(patch: Partial<AutomationJob> = {}): AutomationJob {
  return {
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
    ...patch,
  };
}

const DAILY = job({ job: "escalation", schedule_kind: "daily", interval_minutes: null, min_interval_minutes: null, run_hour: 7, run_minute: 0 });
const WEEKLY = job({ job: "weekly_digest", schedule_kind: "weekly", interval_minutes: null, min_interval_minutes: null, run_hour: 7, run_minute: 30, weekday: 1 });

describe("the PUT body — only the fields of the job's kind, the others null", () => {
  it("interval: interval_minutes only", () => {
    expect(settingBody(job(), { ...draftFrom(job()), interval: " 20 " })).toEqual({
      ok: true,
      body: { enabled: true, interval_minutes: 20, run_hour: null, run_minute: null, weekday: null },
    });
  });

  it("daily: hour and minute only", () => {
    expect(settingBody(DAILY, { ...draftFrom(DAILY), enabled: false, time: "06:45" })).toEqual({
      ok: true,
      body: { enabled: false, interval_minutes: null, run_hour: 6, run_minute: 45, weekday: null },
    });
  });

  it("weekly: ISO weekday (1 = Monday) + hour and minute", () => {
    expect(settingBody(WEEKLY, { ...draftFrom(WEEKLY), weekday: "7" })).toEqual({
      ok: true,
      body: { enabled: true, interval_minutes: null, run_hour: 7, run_minute: 30, weekday: 7 },
    });
  });

  it("refuses only what cannot be SENT; the bounds (min 5 …) are the server's", () => {
    expect(settingBody(job(), { ...draftFrom(job()), interval: "15 phút" })).toEqual({ ok: false, message: INTERVAL_NOT_A_NUMBER });
    expect(settingBody(job(), { ...draftFrom(job()), interval: "" })).toEqual({ ok: false, message: INTERVAL_NOT_A_NUMBER });
    expect(settingBody(DAILY, { ...draftFrom(DAILY), time: "" })).toEqual({ ok: false, message: TIME_MISSING });
    expect(settingBody(WEEKLY, { ...draftFrom(WEEKLY), weekday: "" })).toEqual({ ok: false, message: WEEKDAY_MISSING });
    // 3 minutes is sent — the server answers with the sentence naming the bound.
    expect(settingBody(job(), { ...draftFrom(job()), interval: "3" }).ok).toBe(true);
  });
});

describe("state and cadence sentences", () => {
  it("configured:false is OFF and the cadence is called a suggestion", () => {
    const fresh = job({ configured: false, enabled: false, enabled_at: null });
    expect(stateSentence(fresh)).toMatch(/^Đang tắt — xã chưa lưu cấu hình/);
    expect(stateSentence(fresh)).toMatch(/nhịp gợi ý/);
    // The suggested prefill (15 min) lands in the form.
    expect(draftFrom(fresh)).toEqual({ enabled: false, interval: "15", time: "", weekday: "" });
  });

  it("saved on / off", () => {
    expect(stateSentence(job())).toBe("Đang bật");
    expect(stateSentence(job({ enabled: false }))).toBe("Đang tắt");
  });

  it("cadence in words, Vietnam time named", () => {
    expect(cadenceSentence(job())).toBe("Cứ 15 phút một lần");
    expect(cadenceSentence(DAILY)).toBe("Hằng ngày lúc 07:00");
    expect(cadenceSentence(WEEKLY)).toBe("Thứ Hai hằng tuần, lúc 07:30");
    expect(timezoneLabel("Asia/Ho_Chi_Minh")).toBe("Giờ Việt Nam");
    expect(weekdayLabel(7)).toBe("Chủ nhật");
    expect(draftFrom(WEEKLY)).toEqual({ enabled: true, interval: "", time: "07:30", weekday: "1" });
  });

  it("the three jobs have their §9 names; an unknown key is shown as itself", () => {
    expect(jobWords("sla_reminders").title).toBe("Nhắc việc sắp đến hạn và đã quá hạn");
    expect(jobWords("escalation").title).toBe("Leo thang việc trễ hạn");
    expect(jobWords("weekly_digest").title).toBe("Bản tin đầu tuần cho lãnh đạo");
    // Escalation thresholds are working hours from the SLA tab, never "số ngày" (ADR 0058 §5).
    expect(jobWords("escalation").description).toMatch(/giờ làm việc/);
    expect(jobWords("escalation").description).not.toMatch(/số ngày/);
    expect(jobWords("refresh_dashboards")).toEqual({ title: "refresh_dashboards", description: "" });
  });

  it("scope DECIDED 29/09/2026 (ADR 0058, 0029 §Bổ sung): each job names the kinds of work it covers", () => {
    const reminders = jobWords("sla_reminders").description;
    for (const kind of ["nhiệm vụ", "văn bản đến", "phản ánh", "đơn thư"]) expect(reminders).toContain(kind);
    const escalation = jobWords("escalation").description;
    for (const kind of ["nhiệm vụ", "văn bản đến", "phản ánh"]) expect(escalation).toContain(kind);
    // Petitions escalate against their RESOLVE deadline; đơn thư are reminded, not escalated.
    expect(escalation).toMatch(/hạn xử lý xong/);
    expect(escalation).not.toContain("đơn thư");
  });
});

describe("runs — said from data only", () => {
  it("no runs → 'Chưa có lượt chạy nào.'", () => {
    expect(NO_RUNS_YET).toBe("Chưa có lượt chạy nào.");
  });

  it("outcome null is 'claimed, no result' — not success; the four outcomes in words", () => {
    expect(outcomeLabel(null)).toBe("Chưa báo kết quả");
    expect(outcomeLabel("succeeded")).toBe("Hoàn thành");
    expect(outcomeLabel("configuration_missing")).toBe("Thiếu cấu hình của xã");
    expect(outcomeLabel("dependency_unavailable")).toBe("Dịch vụ liên quan không phản hồi");
    expect(outcomeLabel("failed")).toBe("Lỗi");
    expect(outcomeLabel("khac")).toBe("Kết quả không rõ (khac)");
    expect(triggerLabel("schedule")).toBe("Theo lịch");
    expect(triggerLabel("request")).toBe("Chạy ngay");
    expect(workKindLabel("don-thu")).toBe("Đơn thư");
    expect(workKindLabel("phan-anh")).toBe("Phản ánh của người dân");
  });

  it("run now pressed, no run picked it up → says exactly that, promises no minute", () => {
    const s = runRequestSentence(job({ run_requested_at: "2026-09-29T03:00:00Z" }));
    expect(s).toBe("Đã ghi yêu cầu chạy ngay lúc 10:00 29/09/2026. Chưa có lượt chạy nào nhận yêu cầu này.");
    expect(s).not.toMatch(/phút/);
  });

  it("run now picked up by a `request` run claimed at or after the mark", () => {
    const run = {
      work_kind: "nhiem-vu",
      run_id: "01JRUN",
      trigger: "request",
      claimed_at: "2026-09-29T03:00:40Z",
      outcome: null,
      records_examined: null,
      notices_delivered: null,
      records_without_recipient: null,
      recorded_at: null,
    };
    expect(runRequestSentence(job({ run_requested_at: "2026-09-29T03:00:00Z", last_runs: [run] }))).toMatch(/đã được nhận/);
    // A scheduled run, or a request run from BEFORE the mark, does not count.
    expect(
      runRequestSentence(job({ run_requested_at: "2026-09-29T03:00:00Z", last_runs: [{ ...run, trigger: "schedule" }] })),
    ).toMatch(/Chưa có lượt chạy nào nhận/);
    expect(
      runRequestSentence(job({ run_requested_at: "2026-09-29T03:00:00Z", last_runs: [{ ...run, claimed_at: "2026-09-29T02:00:00Z" }] })),
    ).toMatch(/Chưa có lượt chạy nào nhận/);
    expect(runRequestSentence(job())).toBeNull();
  });

  it("times in Vietnam, an unreadable instant said in words", () => {
    expect(when("2026-09-22T07:05:00Z")).toBe("14:05 22/09/2026");
    expect(when("not-a-time")).toBe("mốc thời gian không đọc được");
    expect(when(null)).toBe("—");
  });
});
