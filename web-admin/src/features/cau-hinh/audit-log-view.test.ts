import { describe, expect, it } from "vitest";

import {
  actorLabel,
  atLabel,
  DATE_ORDER_REFUSED,
  deltaText,
  EMPTY_AUDIT_FILTER,
  filterFromDraft,
  ipLabel,
  sourceErrorLine,
} from "./audit-log-view";

describe("filterFromDraft — whole days in Viet Nam, half-open", () => {
  it("empty form: no filter at all", () => {
    expect(filterFromDraft(EMPTY_AUDIT_FILTER)).toEqual({
      ok: true,
      filter: { from: undefined, to: undefined, actor: undefined, action: undefined, subject: undefined },
    });
  });

  it("the last chosen day is included whole: to = next day 00:00 +07:00", () => {
    const r = filterFromDraft({ ...EMPTY_AUDIT_FILTER, fromDate: "2026-09-01", toDate: "2026-09-30" });
    expect(r).toEqual({
      ok: true,
      filter: expect.objectContaining({
        from: "2026-09-01T00:00:00+07:00",
        to: "2026-10-01T00:00:00+07:00",
      }),
    });
  });

  it("month and year roll over by the calendar", () => {
    const r = filterFromDraft({ ...EMPTY_AUDIT_FILTER, toDate: "2026-12-31" });
    expect(r.ok && r.filter.to).toBe("2027-01-01T00:00:00+07:00");
    const leap = filterFromDraft({ ...EMPTY_AUDIT_FILTER, toDate: "2028-02-28" });
    expect(leap.ok && leap.filter.to).toBe("2028-02-29T00:00:00+07:00");
  });

  it("a single day (from = to) is one whole day, not refused", () => {
    const r = filterFromDraft({ ...EMPTY_AUDIT_FILTER, fromDate: "2026-09-28", toDate: "2026-09-28" });
    expect(r.ok && [r.filter.from, r.filter.to]).toEqual([
      "2026-09-28T00:00:00+07:00",
      "2026-09-29T00:00:00+07:00",
    ]);
  });

  it("a reversed range is refused once, here — not as five 400s", () => {
    expect(filterFromDraft({ ...EMPTY_AUDIT_FILTER, fromDate: "2026-09-30", toDate: "2026-09-01" })).toEqual({
      ok: false,
      message: DATE_ORDER_REFUSED,
    });
  });

  it("text filters are trimmed; blank means not sent", () => {
    const r = filterFromDraft({ ...EMPTY_AUDIT_FILTER, actor: "  CB-00123 ", action: "  ", subject: " PA-X " });
    expect(r.ok && r.filter).toEqual(
      expect.objectContaining({ actor: "CB-00123", action: undefined, subject: "PA-X" }),
    );
  });
});

describe("row wording", () => {
  it("actor: staff code, 'Hệ thống', 'Công dân' — never an empty cell", () => {
    expect(actorLabel({ actor_kind: "staff", actor_code: "CB-00123" })).toBe("CB-00123");
    expect(actorLabel({ actor_kind: "system", actor_code: "system" })).toBe("Hệ thống");
    expect(actorLabel({ actor_kind: "citizen", actor_code: "" })).toBe("Công dân");
    expect(actorLabel({ actor_kind: "staff", actor_code: "" })).not.toBe("");
  });

  it("IP: shown for staff, 'Không hiển thị' for a citizen (ADR 0054 §4)", () => {
    expect(ipLabel({ actor_kind: "staff", actor_ip: "10.0.0.1" })).toBe("10.0.0.1");
    expect(ipLabel({ actor_kind: "citizen", actor_ip: "" })).toBe("Không hiển thị");
  });

  it("time pinned to Asia/Ho_Chi_Minh", () => {
    expect(atLabel("2026-09-22T07:05:09Z")).toBe("14:05:09 22/09/2026");
    expect(atLabel("khong-phai-moc")).toBe("Mốc thời gian không đọc được");
  });

  it("delta: JSON text, or null when there is none", () => {
    expect(deltaText(null)).toBeNull();
    expect(deltaText({})).toBeNull();
    expect(deltaText({ before: "a", after: "<b>" })).toBe('{\n  "before": "a",\n  "after": "<b>"\n}');
  });

  it("the error line names the phân hệ", () => {
    expect(sourceErrorLine("finance", "Lỗi máy chủ.")).toBe(
      "Không tải được nhật ký của phân hệ Tài chính: Lỗi máy chủ.",
    );
  });
});
