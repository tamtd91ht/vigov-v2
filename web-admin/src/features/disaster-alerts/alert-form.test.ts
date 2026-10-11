import { describe, expect, it } from "vitest";

import type { DisasterAlert } from "@/lib/api/disaster-alerts";

import {
  ALERT_KINDS,
  alertStatus,
  canEndEarly,
  createBody,
  EMPTY_FORM,
  formatInstant,
  formFromAlert,
  hasErrors,
  kindLabel,
  localizeAlertRefusal,
  patchBody,
  toGridPoint,
  validateAlertForm,
  validityText,
  zoneProblem,
  type AlertFormValues,
} from "./alert-form";

const ALERT: DisasterAlert = {
  id: "01JALERT0000000000000000A1",
  kind: "lu",
  title: "Lũ trên sông Lam",
  level: "Cấp 3",
  detail: "Nước lên nhanh ở thôn 4, thôn 5.",
  effective_from: "2026-10-11T01:00:00Z",
  effective_to: "2026-10-12T01:00:00Z",
  in_force: true,
  danger_zone: [
    { x: 10, y: 10 },
    { x: 60, y: 12 },
    { x: 40, y: 70 },
  ],
  evacuation_points: [{ name: "Trường tiểu học", address: "Thôn 2", phone: "0900000000", capacity: 200 }],
  emergency_contacts: [{ role: "Trực ban", name: "Cán bộ trực", phone: "0900000000" }],
  created_at: "2026-10-11T00:00:00Z",
  created_by: "CB-001",
  updated_at: "2026-10-11T00:00:00Z",
  updated_by: "CB-001",
};

const VALID: AlertFormValues = {
  ...EMPTY_FORM,
  kind: "bao",
  title: "  Bão số 7  ",
  detail: "Bà con chằng chống nhà cửa.",
  fromLocal: "2026-10-11T08:00",
};

describe("the six kinds", () => {
  it("are the owner's six, in Vietnamese", () => {
    expect(ALERT_KINDS.map((k) => [k.code, k.label])).toEqual([
      ["bao", "Bão"],
      ["lu", "Lũ"],
      ["sat-lo", "Sạt lở"],
      ["mua-lon", "Mưa lớn"],
      ["dich-benh", "Dịch bệnh"],
      ["chay", "Cháy"],
    ]);
    expect(kindLabel("sat-lo")).toBe("Sạt lở");
    expect(kindLabel("dong-dat")).toBe("dong-dat");
  });
});

describe("client-side checks (UX only — the server decides)", () => {
  it("an empty form names kind, title, detail and start; no kind is preselected", () => {
    const e = validateAlertForm(EMPTY_FORM);
    expect(Object.keys(e).sort()).toEqual(["detail", "from", "kind", "title"]);
  });

  it("a filled form passes", () => {
    expect(hasErrors(validateAlertForm(VALID))).toBe(false);
  });

  it("the end must be after the start", () => {
    expect(validateAlertForm({ ...VALID, toLocal: "2026-10-11T08:00" }).to).toBeDefined();
    expect(validateAlertForm({ ...VALID, toLocal: "2026-10-11T07:00" }).to).toBeDefined();
    expect(validateAlertForm({ ...VALID, toLocal: "2026-10-11T09:00" }).to).toBeUndefined();
  });

  it("the zone: none, or 3..200 points inside 0..100", () => {
    expect(zoneProblem([])).toBeNull();
    expect(zoneProblem([{ x: 1, y: 1 }, { x: 2, y: 2 }])).toContain("ít nhất 3 điểm");
    expect(zoneProblem([{ x: 1, y: 1 }, { x: 2, y: 2 }, { x: 3, y: 3 }])).toBeNull();
    expect(zoneProblem(Array.from({ length: 201 }, (_, i) => ({ x: i % 100, y: 1 })))).toContain("tối đa 200");
    expect(zoneProblem([{ x: 1, y: 1 }, { x: 2, y: 2 }, { x: 101, y: 3 }])).toContain("Điểm thứ 3");
  });

  it("evacuation rows need a name and a whole capacity; contacts need all three", () => {
    expect(validateAlertForm({ ...VALID, evacuation: [{ name: "", address: "", phone: "", capacity: "" }] }).evacuation).toContain("thứ 1");
    expect(validateAlertForm({ ...VALID, evacuation: [{ name: "Trường", address: "", phone: "", capacity: "1.5" }] }).evacuation).toContain("Sức chứa");
    expect(validateAlertForm({ ...VALID, contacts: [{ role: "Trực ban", name: "", phone: "0900000000" }] }).contacts).toContain("thứ 1");
  });
});

describe("request bodies", () => {
  it("POST: trimmed texts, the instant with +07:00, optional fields absent when empty", () => {
    expect(createBody(VALID)).toEqual({
      kind: "bao",
      title: "Bão số 7",
      detail: "Bà con chằng chống nhà cửa.",
      effective_from: "2026-10-11T08:00:00+07:00",
    });
  });

  it("POST: lists in the contract's shape; empty optional cells left out, capacity a number", () => {
    const body = createBody({
      ...VALID,
      level: "Cấp 4",
      toLocal: "2026-10-12T08:00",
      zone: [{ x: 1, y: 2 }, { x: 3, y: 4 }, { x: 5, y: 6 }],
      evacuation: [{ name: " Nhà văn hoá ", address: "", phone: "", capacity: "150" }],
      contacts: [{ role: "Công an xã", name: "Trực ban", phone: "0900000000" }],
    });
    expect(body.level).toBe("Cấp 4");
    expect(body.effective_to).toBe("2026-10-12T08:00:00+07:00");
    expect(body.danger_zone).toEqual([{ x: 1, y: 2 }, { x: 3, y: 4 }, { x: 5, y: 6 }]);
    expect(body.evacuation_points).toEqual([{ name: "Nhà văn hoá", capacity: 150 }]);
    expect(body.emergency_contacts).toEqual([{ role: "Công an xã", name: "Trực ban", phone: "0900000000" }]);
  });

  it("PATCH: nothing changed → empty body", () => {
    const before = formFromAlert(ALERT);
    expect(patchBody(before, before)).toEqual({});
  });

  it("PATCH: only the changed fields; an emptied end and level are sent as \"\" (clear)", () => {
    const before = formFromAlert(ALERT);
    const after = { ...before, title: "Lũ lớn trên sông Lam", toLocal: "", level: "" };
    expect(patchBody(before, after)).toEqual({ title: "Lũ lớn trên sông Lam", effective_to: "", level: "" });
  });

  it("PATCH: a list is sent whole when it changed; `[]` clears it", () => {
    const before = formFromAlert(ALERT);
    const moved = { ...before, zone: [...before.zone.slice(0, 2), { x: 45, y: 75 }] };
    expect(patchBody(before, moved)).toEqual({ danger_zone: [{ x: 10, y: 10 }, { x: 60, y: 12 }, { x: 45, y: 75 }] });
    expect(patchBody(before, { ...before, contacts: [] })).toEqual({ emergency_contacts: [] });
  });

  it("the edit form opens on the alert in Vietnam time", () => {
    const v = formFromAlert(ALERT);
    expect(v.fromLocal).toBe("2026-10-11T08:00");
    expect(v.evacuation[0]).toEqual({ name: "Trường tiểu học", address: "Thôn 2", phone: "0900000000", capacity: "200" });
  });
});

describe("status badge", () => {
  const now = Date.parse("2026-10-11T03:00:00Z");

  it("the server's in_force wins", () => {
    expect(alertStatus(ALERT, now)).toBe("in-force");
  });

  it("not yet started → Sắp hiệu lực; ended early or past → Đã kết thúc", () => {
    expect(alertStatus({ ...ALERT, in_force: false, effective_from: "2026-10-12T00:00:00Z" }, now)).toBe("upcoming");
    expect(alertStatus({ ...ALERT, in_force: false, ended_at: "2026-10-11T02:00:00Z" }, now)).toBe("ended");
    expect(alertStatus({ ...ALERT, in_force: false, effective_from: "2026-10-01T00:00:00Z" }, now)).toBe("ended");
  });

  it("Kết thúc sớm only while shown or about to be", () => {
    expect(canEndEarly(ALERT, now)).toBe(true);
    expect(canEndEarly({ ...ALERT, in_force: false, ended_at: "2026-10-11T02:00:00Z" }, now)).toBe(false);
  });

  it("the window reads in Vietnam time; an open end says so", () => {
    expect(formatInstant("2026-10-11T01:00:00Z")).toBe("08:00 11/10/2026");
    expect(validityText(ALERT)).toBe("08:00 11/10/2026 – 08:00 12/10/2026");
    expect(validityText({ effective_from: ALERT.effective_from })).toBe("Từ 08:00 11/10/2026 — tới khi kết thúc");
  });
});

describe("canvas geometry", () => {
  const rect = { left: 100, top: 50, width: 200, height: 200 };

  it("pointer → grid, one decimal, clamped to 0..100", () => {
    expect(toGridPoint(200, 150, rect)).toEqual({ x: 50, y: 50 });
    expect(toGridPoint(133.33, 50, rect)).toEqual({ x: 16.7, y: 0 });
    expect(toGridPoint(999, -10, rect)).toEqual({ x: 100, y: 0 });
    expect(toGridPoint(1, 1, { left: 0, top: 0, width: 0, height: 0 })).toBeNull();
  });
});

describe("the server's sentence", () => {
  it("only the field token becomes the box's label", () => {
    expect(localizeAlertRefusal("`danger_zone` không hợp lệ: một vùng cần ít nhất 3 điểm")).toBe(
      "Vùng nguy hiểm không hợp lệ: một vùng cần ít nhất 3 điểm",
    );
    expect(localizeAlertRefusal("Cảnh báo đã kết thúc nên không sửa hay kết thúc lại được.")).toBe(
      "Cảnh báo đã kết thúc nên không sửa hay kết thúc lại được.",
    );
    expect(localizeAlertRefusal("`unknown_field` lạ")).toBe("`unknown_field` lạ");
  });
});
