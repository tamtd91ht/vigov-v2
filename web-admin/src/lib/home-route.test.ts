import { describe, expect, it } from "vitest";

import type { identity_phienHienTaiRa } from "./api/schema.gen";
import { DEFAULT_HOME, homePathFor, LEADER_HOME } from "./home-route";

/** `/` redirect by role (TQ-01, spec 15 §1). Only `role.is_leader` sends to the leader notebook. */

function session(role: identity_phienHienTaiRa["role"]): identity_phienHienTaiRa {
  return {
    sid: "01J000000000000000000SID",
    expires_at: "2026-10-05T12:00:00Z",
    staff: { code: "CB-00001", full_name: "Cán bộ thử", position: "Chủ tịch UBND xã" },
    role,
    permissions: ["task.read"],
    must_change_password: false,
  };
}

describe("homePathFor", () => {
  it("leader role → Sổ tay lãnh đạo", () => {
    expect(homePathFor(session({ code: "chu-tich-ubnd", name: "Chủ tịch UBND", is_leader: true }))).toBe(LEADER_HOME);
    expect(LEADER_HOME).toBe("/nhiem-vu/so-tay");
  });

  it("any other role → Tổng quan, even when the POSITION reads like a leader", () => {
    expect(homePathFor(session({ code: "chu-tich-ubnd", name: "Chủ tịch UBND", is_leader: false }))).toBe(DEFAULT_HOME);
    expect(DEFAULT_HOME).toBe("/tong-quan");
  });

  it("no role assigned, or session unreadable → Tổng quan (user decision 05/10)", () => {
    expect(homePathFor(session(null))).toBe(DEFAULT_HOME);
    expect(homePathFor(null)).toBe(DEFAULT_HOME);
  });

  it("a drifted body without `role` → Tổng quan, never a throw", () => {
    const drifted = { sid: "x" } as unknown as identity_phienHienTaiRa;
    expect(homePathFor(drifted)).toBe(DEFAULT_HOME);
  });
});
