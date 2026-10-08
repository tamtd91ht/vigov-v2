import { describe, expect, it } from "vitest";

import type { AutomationJob } from "@/lib/api/automation-jobs";

import {
  INTERVAL_NOT_A_NUMBER,
  NEVER_RAN,
  TIME_MISSING,
  WEEKDAY_MISSING,
  cadenceChanged,
  cadenceSavedToast,
  clockFieldLabel,
  draftFrom,
  intervalLabel,
  intervalOptions,
  jobWords,
  lastRunSentence,
  outcomeLabel,
  runRequestSentence,
  runRequestedToast,
  settingBody,
  switchWord,
  timezoneLabel,
  toggledToast,
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

describe("card words (spec 09)", () => {
  it("configured:false prefills the suggested cadence, switched off", () => {
    const fresh = job({ configured: false, enabled: false, enabled_at: null });
    expect(draftFrom(fresh)).toEqual({ enabled: false, interval: "15", time: "", weekday: "" });
    expect(draftFrom(WEEKLY)).toEqual({ enabled: true, interval: "", time: "07:30", weekday: "1" });
  });

  it("switch word and the three toasts", () => {
    expect(switchWord(true)).toBe("Đang bật");
    expect(switchWord(false)).toBe("Đang tắt");
    expect(toggledToast("Leo thang việc trễ hạn", true)).toBe("Đã lưu: Leo thang việc trễ hạn");
    expect(toggledToast("Leo thang việc trễ hạn", false)).toBe("Đã tắt: Leo thang việc trễ hạn");
    expect(cadenceSavedToast("Leo thang việc trễ hạn")).toBe("Đã lưu: Leo thang việc trễ hạn");
    // "Chạy ngay" only RECORDS a request; the toast never says it ran.
    expect(runRequestedToast("X")).toBe("Đã ghi yêu cầu chạy ngay: X");
  });

  it("clock label is 'Lúc' in Vietnam time; another zone is named, never hidden", () => {
    expect(clockFieldLabel("Asia/Ho_Chi_Minh")).toBe("Lúc");
    expect(clockFieldLabel("UTC")).toBe("Lúc (UTC)");
    expect(timezoneLabel("Asia/Ho_Chi_Minh")).toBe("Giờ Việt Nam");
    expect(weekdayLabel(7)).toBe("Chủ nhật");
  });

  it("'Lưu nhịp' only when the cadence differs from the saved one — the switch does not count", () => {
    expect(cadenceChanged(job(), draftFrom(job()))).toBe(false);
    expect(cadenceChanged(job(), { ...draftFrom(job()), enabled: false })).toBe(false);
    expect(cadenceChanged(job(), { ...draftFrom(job()), interval: "30" })).toBe(true);
    expect(cadenceChanged(WEEKLY, { ...draftFrom(WEEKLY), weekday: "3" })).toBe(true);
    expect(cadenceChanged(DAILY, { ...draftFrom(DAILY), time: "08:00" })).toBe(true);
  });
});

describe("'Cứ mỗi' choices (spec 09)", () => {
  it("5, 10, 15, 30 phút, 1, 3, 6, 12 giờ, 1 ngày", () => {
    expect(intervalOptions(job({ min_interval_minutes: 5 }), "15").map(intervalLabel)).toEqual([
      "5 phút", "10 phút", "15 phút", "30 phút", "1 giờ", "3 giờ", "6 giờ", "12 giờ", "1 ngày",
    ]);
  });

  it("only values from the server's min_interval_minutes up", () => {
    expect(intervalOptions(job({ min_interval_minutes: 15 }), "15")).toEqual([15, 30, 60, 180, 360, 720, 1440]);
  });

  it("a saved value outside the list is offered as its own option, in order", () => {
    expect(intervalOptions(job({ interval_minutes: 20 }), "20")).toEqual([5, 10, 15, 20, 30, 60, 180, 360, 720, 1440]);
    expect(intervalLabel(20)).toBe("20 phút");
    expect(intervalLabel(90)).toBe("90 phút");
    expect(intervalLabel(2880)).toBe("2 ngày");
  });
});

describe("job names and scopes", () => {
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
  it("no runs → 'Chưa chạy lần nào'", () => {
    expect(lastRunSentence(job())).toBe(NEVER_RAN);
    expect(NEVER_RAN).toBe("Chưa chạy lần nào");
  });

  it("REGRESSION: 'Chạy lần cuối' is the LATEST claim, not last_runs[0] (the server sorts by kind of work)", () => {
    const base = {
      run_id: "01JRUN",
      trigger: "schedule",
      outcome: null,
      records_examined: null,
      notices_delivered: null,
      records_without_recipient: null,
      recorded_at: null,
    };
    const runs = [
      { ...base, work_kind: "don-thu", claimed_at: "2026-09-29T01:00:00Z" },
      { ...base, work_kind: "nhiem-vu", claimed_at: "2026-09-29T03:30:00Z" },
      { ...base, work_kind: "phan-anh", claimed_at: "not-a-time" },
    ];
    expect(lastRunSentence(job({ last_runs: runs }))).toBe("Chạy lần cuối 10:30 29/09/2026");
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
