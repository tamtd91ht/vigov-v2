import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { identity_nhomQuyenRa, identity_vaiTroCotRa } from "@/lib/api/schema.gen";

import { UnheldPermissionWarnings } from "./ma-tran-phan-quyen";
import { dungBangDaCap } from "./ma-tran-quyen";
import { RoleTemplateSeedView } from "./role-template-seed-panel";
import type { SeedPhase } from "./role-template-seed-panel";
import {
  DEFAULT_ADMIN_ROLE_CODE,
  SEED_BUTTON,
  SEED_CONFIRM_BUTTON,
  SEED_CONFIRM_TEXT,
  seedSummary,
  unheldPermissionWarnings,
} from "./role-templates";

/**
 * ADR 0055 on the Phân quyền tab: the seed button (confirm, three result lists, the 403 sentence
 * verbatim) and the warning when `feedback.classify` / `feedback.unmask` is held by no working role.
 */

const GROUPS: identity_nhomQuyenRa[] = [
  {
    name: "PHẢN ÁNH",
    permissions: [
      { code: "feedback.read", label: "Xem phản ánh" },
      { code: "feedback.classify", label: "Phân loại phản ánh, ấn định hạn xử lý" },
      { code: "feedback.unmask", label: "Xem đầy đủ họ tên và số điện thoại người gửi" },
    ],
  },
];

function role(id: string, code: string, name: string): identity_vaiTroCotRa {
  return { id, code, name, is_leader: false, staff_count: 1, active_account_count: 1 };
}

const ADMIN = role("01JQT", DEFAULT_ADMIN_ROLE_CODE, "Quản trị hệ thống");
const CHAIR = role("01JCT", "chu-tich-ubnd", "Chủ tịch UBND");

describe("unheldPermissionWarnings", () => {
  it("only the default administrator holds both keys → two warnings, each naming its key", () => {
    const granted = dungBangDaCap([
      { role_id: ADMIN.id, permission: "feedback.classify" },
      { role_id: ADMIN.id, permission: "feedback.unmask" },
      { role_id: CHAIR.id, permission: "feedback.read" },
    ]);
    const w = unheldPermissionWarnings(GROUPS, [ADMIN, CHAIR], granted);
    expect(w.map((x) => x.code)).toEqual(["feedback.classify", "feedback.unmask"]);
    expect(w[0]!.sentence).toContain("Phân loại phản ánh, ấn định hạn xử lý");
    expect(w[0]!.sentence).toContain("feedback.classify");
    expect(w[0]!.sentence).toMatch(/không có ai|trễ hạn/);
  });

  it("a working role holding the key silences that key's warning only", () => {
    const granted = dungBangDaCap([{ role_id: CHAIR.id, permission: "feedback.classify" }]);
    const w = unheldPermissionWarnings(GROUPS, [ADMIN, CHAIR], granted);
    expect(w.map((x) => x.code)).toEqual(["feedback.unmask"]);
  });

  it("a key the catalogue does not list is not warned about", () => {
    const only = [{ name: "X", permissions: [{ code: "feedback.read", label: "Xem" }] }];
    expect(unheldPermissionWarnings(only, [ADMIN], dungBangDaCap([]))).toEqual([]);
  });

  it("renders as text lines, nothing when there is no warning", () => {
    const w = unheldPermissionWarnings(GROUPS, [ADMIN, CHAIR], dungBangDaCap([]));
    const html = renderToStaticMarkup(<UnheldPermissionWarnings warnings={w} />);
    expect(html).toContain("feedback.classify");
    expect(html).toContain("feedback.unmask");
    expect(renderToStaticMarkup(<UnheldPermissionWarnings warnings={[]} />)).toBe("");
  });
});

describe("seed button", () => {
  const view = (phase: SeedPhase) =>
    renderToStaticMarkup(<RoleTemplateSeedView phase={phase} onOpen={() => {}} onConfirm={() => {}} onCancel={() => {}} />);

  it("idle: one button, no confirm text yet", () => {
    const html = view({ kind: "idle" });
    expect(html).toContain(SEED_BUTTON);
    expect(html).not.toContain(SEED_CONFIRM_TEXT);
  });

  it("confirming: says existing roles are left untouched and deleted ones are not revived", () => {
    const html = view({ kind: "confirming" });
    expect(html).toContain(SEED_CONFIRM_BUTTON);
    expect(SEED_CONFIRM_TEXT).toMatch(/giữ nguyên/);
    expect(SEED_CONFIRM_TEXT).toMatch(/đã xoá thì không được tạo lại/);
  });

  it("403 permission_escalation: the server's sentence, verbatim", () => {
    const sentence = "Tài khoản của bạn còn thiếu: budget.confirm, task.approve. Chưa có vai trò nào được tạo.";
    const html = view({ kind: "done", result: { ok: false, thongBao: sentence } });
    expect(html).toContain(sentence);
    expect(html).toContain('role="alert"');
  });

  it("200: created / kept / deleted-not-recreated listed under their own headings", () => {
    const html = view({
      kind: "done",
      result: {
        ok: true,
        duLieu: {
          created: [{ code: "ke-toan", name: "Kế toán" }],
          skipped_existing: [{ code: "chu-tich-ubnd", name: "Chủ tịch UBND" }],
          skipped_deleted: [{ code: "truong-thon", name: "Trưởng thôn, Tổ trưởng dân phố" }],
        },
      },
    });
    expect(html).toContain("Kế toán");
    expect(html).toContain("Đã tạo");
    expect(html).toContain("giữ nguyên");
    expect(html).toContain("không tạo lại");
    expect(html).toContain("Trưởng thôn, Tổ trưởng dân phố");
  });

  it("a second run that created nothing says so plainly, not as a failure", () => {
    const s = seedSummary({ created: [], skipped_existing: [{ code: "a", name: "A" }], skipped_deleted: [] });
    expect(s.lead).toMatch(/Không tạo thêm/);
    expect(s.blocks.map((b) => b.heading)).toHaveLength(1);
  });
});
