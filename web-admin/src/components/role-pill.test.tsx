import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

import { ROLE_PILL_PREFIX, RolePill, sessionRoleName } from "./role-pill";

/**
 * The topbar role pill reads ONLY the session the page already holds. The denied cases — session
 * not read, unreadable, no role assigned — are the ones a developer never sees, because their own
 * account always has a role; each of them must draw nothing rather than a guessed role.
 */
const read: Exclude<PhienDaDoc, null> = {
  ok: true,
  duLieu: {
    sid: "01J000000000000000000000SD",
    expires_at: "2026-10-02T03:00:00Z",
    staff: { code: "CB-001", full_name: "Nguyễn Văn Hùng", position: "Công chức Văn phòng" },
    role: { code: "van-thu", name: "Văn thư", is_leader: false },
    permissions: [],
    must_change_password: false,
  },
};

function withRole(role: { code: string; name: string; is_leader: boolean } | null): PhienDaDoc {
  if (!read.ok) throw new Error("fixture");
  return { ok: true, duLieu: { ...read.duLieu, role } };
}

describe("sessionRoleName", () => {
  it("a read session with a role: the role's name", () => {
    expect(sessionRoleName(read)).toBe("Văn thư");
  });

  it("session not read yet: no pill", () => {
    expect(sessionRoleName(null)).toBeNull();
  });

  it("session unreadable: no pill", () => {
    expect(sessionRoleName({ ok: false, thongBao: "Phiên làm việc đã hết hạn." })).toBeNull();
  });

  it("no role assigned, or a blank name: no pill — never a guessed role", () => {
    expect(sessionRoleName(withRole(null))).toBeNull();
    expect(sessionRoleName(withRole({ code: "x", name: "  ", is_leader: false }))).toBeNull();
  });

  it("reads the human name, never the slug", () => {
    expect(sessionRoleName(read)).not.toBe("van-thu");
  });
});

describe("RolePill", () => {
  it("draws ShieldCheck + 'Vai trò:' + the name", () => {
    const html = renderToStaticMarkup(<RolePill roleName="Chủ tịch UBND" />);
    expect(html).toContain("lucide-shield-check");
    expect(html).toContain(ROLE_PILL_PREFIX);
    expect(html).toContain("<strong>Chủ tịch UBND</strong>");
  });
});
