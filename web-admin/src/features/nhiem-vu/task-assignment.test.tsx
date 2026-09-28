import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import { reassignTask } from "@/lib/api/nhiem-vu";
import type {
  identity_boPhanRa,
  identity_danhBaChonNguoiRa,
  petitions_nhiemVuRa,
  petitions_taskAssignmentIn,
} from "@/lib/api/schema.gen";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_TAO_NHIEM_VU,
  TASK_ASSIGN_PERMISSION,
} from "@/lib/quyen";

import {
  BANG_NHAN_MAC_DINH,
  docBangNhanTrangThai,
  quyenNhiemVu,
  type QuyenNhiemVu, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
} from "./nhan-nhiem-vu";
import { ChiTietNhiemVu, type DanhMucNhiemVu } from "./so-nhiem-vu"; // vi-name-ok: existing names, imported not declared (rule 12 inv 3)
import {
  ASSIGNMENT_BLOCK_ID,
  ASSIGNMENT_NO_CHANGE,
  ASSIGNMENT_NOTE_LABEL,
  ASSIGNMENT_TITLE,
  ASSIGNMENT_UNIT_FIELD_ID,
  ASSIGNMENT_UNIT_REQUIRED,
  assignmentBody,
  assignmentEffectNote,
  assignmentFormFromTask,
  canShowAssignment,
  unitOptions,
  type AssignmentForm,
} from "./task-assignment";
import { TaskAssignmentForm } from "./task-assignment-block";

/**
 * §5.7 "Giao việc, chuyển việc" (owner decision 28/09/2026, 764bb92) and the removal of
 * `chuyen-tiep` from the status moves.
 *
 * THE LIMIT, STATED: no DOM in vitest (`vitest.config.mts`). No case here clicks a button, changes
 * a select or checks where focus lands; what IS tested is the body the block would send, the gate,
 * what each state renders, and the wire call.
 */

const UNIT_A = "01JBOPHANA";
const UNIT_B = "01JBOPHANB";
const STAFF_A = "CB-2026-AAAAAA";
const STAFF_B = "CB-2026-BBBBBB";
const MONITOR = "CB-2026-MMMMMM";

function task(patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    child_count: 0,
    allowed_transitions: [],
    type: "theo-van-ban",
    bloc: "",
    priority: "",
    title: "Rà soát hồ sơ tồn đọng",
    description: "",
    status: "dang-thuc-hien",
    source: "truc-tiep",
    source_id: "",
    unit: UNIT_A,
    assignee: STAFF_A,
    assigner: "",
    lead_unit: UNIT_A,
    monitor: MONITOR,
    due_at: "2026-06-20T23:59:59+07:00",
    original_due_at: "2026-06-20T23:59:59+07:00",
    completed_at: null,
    progress: 0,
    result_summary: "",
    note: "",
    leader_approved: false,
    superior_acknowledged: false,
    parent: "",
    created_by: "CB-2026-VANTHU",
    created_at: "2026-06-01T02:00:00Z",
    ...patch,
  };
}

function form(t: petitions_nhiemVuRa, patch: Partial<AssignmentForm> = {}): AssignmentForm {
  return { ...assignmentFormFromTask(t), ...patch };
}

const UNITS: identity_boPhanRa[] = [
  { id: UNIT_A, code: "vp", name: "VĂN PHÒNG", parent_id: "", order: 0, staff_count: 0 },
  { id: UNIT_B, code: "tp", name: "TƯ PHÁP", parent_id: "", order: 1, staff_count: 0 },
];

const DIRECTORY: KetQua<identity_danhBaChonNguoiRa> = {
  ok: true,
  duLieu: {
    items: [
      { code: STAFF_A, full_name: "Nguyễn Văn A", position: "", department_id: UNIT_A },
      { code: STAFF_B, full_name: "Trần Thị B", position: "", department_id: UNIT_B },
    ],
  },
};

/** `renderToStaticMarkup` escapes `"` and `&` — compare against what is really in the HTML. */
function asInHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

describe("assignmentBody — ONLY what differs from the task as read", () => {
  it("nothing changed ⇒ `null` (button off)", () => {
    const t = task();
    expect(assignmentBody(form(t), t)).toBeNull();
  });

  it("a note alone changes nothing ⇒ `null` — the server would answer 400", () => {
    const t = task();
    expect(assignmentBody(form(t, { note: "Chuyển cho đúng bộ phận" }), t)).toBeNull();
  });

  it("unit changed: ONLY `unit` goes — an unchanged monitor is NOT resent", () => {
    // The load-bearing case: every staff code sent is checked with identity, so resending MONITOR
    // (who may have left) would refuse a change of unit the officer actually asked for.
    const t = task();
    expect(assignmentBody(form(t, { unit: UNIT_B }), t)).toEqual({ unit: UNIT_B });
  });

  it("unit cleared on a task that HAS one ⇒ `null`; the server refuses `unit: \"\"`", () => {
    const t = task();
    expect(assignmentBody(form(t, { unit: "" }), t)).toBeNull();
    // Even with another real change beside it: the body is never sent with an empty unit.
    expect(assignmentBody(form(t, { unit: "", assignee: STAFF_B }), t)).toBeNull();
  });

  it("task with NO unit: leaving it empty is not a change, and the assignee alone may go", () => {
    const t = task({ unit: "" });
    expect(assignmentBody(form(t), t)).toBeNull();
    expect(assignmentBody(form(t, { assignee: STAFF_B }), t)).toEqual({ assignee: STAFF_B });
  });

  it("clearing the assignee SENDS \"\" — `— Để bộ phận tự phân công —` is a real choice", () => {
    const t = task();
    expect(assignmentBody(form(t, { assignee: "" }), t)).toEqual({ assignee: "" });
  });

  it("note goes trimmed, and only alongside a change", () => {
    const t = task();
    expect(assignmentBody(form(t, { assignee: STAFF_B, note: "  Đổi người phụ trách \n" }), t)).toEqual({
      assignee: STAFF_B,
      note: "Đổi người phụ trách",
    });
    expect(assignmentBody(form(t, { assignee: STAFF_B, note: "   " }), t)).toEqual({ assignee: STAFF_B });
  });

  it("`theo-van-ban`: lead unit and monitor go when they differ, \"\" clears them", () => {
    const t = task();
    expect(assignmentBody(form(t, { leadUnit: UNIT_B }), t)).toEqual({ lead_unit: UNIT_B });
    expect(assignmentBody(form(t, { monitor: "" }), t)).toEqual({ monitor: "" });
    expect(assignmentBody(form(t, { unit: UNIT_B, leadUnit: "", monitor: STAFF_B }), t)).toEqual({
      unit: UNIT_B,
      lead_unit: "",
      monitor: STAFF_B,
    });
  });

  it("another type: lead unit and monitor are NEVER sent, whatever the form holds", () => {
    const t = task({ type: "co-ban", lead_unit: "", monitor: "" });
    expect(assignmentBody(form(t, { leadUnit: UNIT_B, monitor: STAFF_B }), t)).toBeNull();
    expect(assignmentBody(form(t, { unit: UNIT_B, leadUnit: UNIT_B, monitor: STAFF_B }), t)).toEqual({
      unit: UNIT_B,
    });
  });
});

describe("gate — `task.assign` AND a task that is not terminal, both directions", () => {
  const WITH = quyenNhiemVu([TASK_ASSIGN_PERMISSION]);
  const WITHOUT = quyenNhiemVu([QUYEN_TAO_NHIEM_VU, QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU]);

  it("the flag is `task.assign` exactly — not implied by `task.update`, no prefix match", () => {
    expect(WITH.reassign).toBe(true);
    expect(WITHOUT.reassign).toBe(false);
    expect(quyenNhiemVu(["task.*", "task"]).reassign).toBe(false);
    expect(quyenNhiemVu(null).reassign).toBe(false);
  });

  it("allowed on every non-terminal status, `tam-dung` included", () => {
    for (const s of ["moi-giao", "da-tiep-nhan", "dang-thuc-hien", "cho-duyet", "tam-dung"]) {
      expect(canShowAssignment(WITH, s)).toBe(true);
    }
  });

  it("denied on a terminal task, whatever the keys — and without the key, everywhere", () => {
    expect(canShowAssignment(WITH, "hoan-thanh")).toBe(false);
    expect(canShowAssignment(WITH, "chuyen-tiep")).toBe(false);
    expect(canShowAssignment(WITHOUT, "dang-thuc-hien")).toBe(false);
  });
});

const CATALOGUES: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: UNITS };
const NOT_CALLED = (): Promise<KetQua<petitions_nhiemVuRa>> =>
  Promise.resolve({ ok: false, thongBao: "không gọi trong bài kiểm" });

function drawer(t: petitions_nhiemVuRa, permissions: QuyenNhiemVu): string {
  return renderToStaticMarkup(
    <ChiTietNhiemVu
      nhiemVu={t}
      vanBan={{ pha: "dangTai" }}
      danhMuc={CATALOGUES}
      nhanTT={BANG_NHAN_MAC_DINH}
      tenBoPhan={new Map(UNITS.map((u) => [u.id, u.name]))}
      danhBa={DIRECTORY}
      bayGio={new Date("2026-09-15T03:00:00Z")}
      maNguoiDangNhap=""
      quyen={permissions}
      dangGui={false}
      loiGhi={null}
      dong={() => {}}
      doiTrangThai={() => {}}
      xoa={() => {}}
      guiDeNghiLuiHan={() => Promise.resolve({ ok: false, thongBao: "" })}
      quyetDinh={() => Promise.resolve({ ok: false, thongBao: "" })}
      suaKhoiVanBan={NOT_CALLED}
      docLaiChiTiet={NOT_CALLED}
      extensionRefreshKey="0"
      onExtensionDecided={() => {}}
      openTask={() => {}}
      openTaskByCode={NOT_CALLED}
      saveParent={NOT_CALLED}
      addChild={null}
      reassign={NOT_CALLED}
    />,
  );
}

const BLOCK_MARK = `id="${ASSIGNMENT_BLOCK_ID}"`;
const ALL_WRITE_KEYS = [QUYEN_TAO_NHIEM_VU, QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU];

describe("the drawer — block present / absent, and the stepper's `Chuyển tiếp` branch", () => {
  it("ALLOWED: `task.assign` on an active task ⇒ the block with the §5.7 labels", () => {
    const html = drawer(task(), quyenNhiemVu([TASK_ASSIGN_PERMISSION]));
    expect(html).toContain(BLOCK_MARK);
    expect(html).toContain(`>${ASSIGNMENT_TITLE}</h4>`);
    expect(html).toContain(`<label for="${ASSIGNMENT_UNIT_FIELD_ID}">Bộ phận</label>`);
    expect(html).toContain(">Người thực hiện</label>");
    expect(html).toContain(`>${ASSIGNMENT_NOTE_LABEL}</label>`);
    expect(html).toMatch(/<button type="submit" class="nut-chinh"[^>]*>Giao việc<\/button>/);
  });

  it("DENIED — every other write key but not `task.assign` ⇒ no block, no stepper button", () => {
    const html = drawer(task(), quyenNhiemVu(ALL_WRITE_KEYS));
    expect(html).not.toContain(BLOCK_MARK);
    expect(html).not.toContain(ASSIGNMENT_TITLE);
    expect(html).not.toContain(`aria-controls="${ASSIGNMENT_UNIT_FIELD_ID}"`);
  });

  it("DENIED — a finished task has no block even with `task.assign`", () => {
    const html = drawer(
      task({ status: "hoan-thanh", completed_at: "2026-06-25T02:00:00Z" }),
      quyenNhiemVu([TASK_ASSIGN_PERMISSION]),
    );
    expect(html).not.toContain(BLOCK_MARK);
  });

  it("an OLD `chuyen-tiep` row: its label still renders, lit; terminal — no block, no moves", () => {
    const html = drawer(
      task({ status: "chuyen-tiep" }),
      quyenNhiemVu([...ALL_WRITE_KEYS, TASK_ASSIGN_PERMISSION]),
    );
    expect(html).toContain('<span class="chip chip-hoat-dong">Chuyển tiếp</span>');
    expect(html).not.toContain(BLOCK_MARK);
    expect(html).not.toContain(`aria-controls="${ASSIGNMENT_UNIT_FIELD_ID}"`);
    expect(html).not.toContain("Chuyển sang");
    expect(html).toContain("không có lối ra khỏi trạng thái này");
  });

  it("stepper: with the block shown, `Chuyển tiếp` is a button pointing at the block — not a move", () => {
    const html = drawer(task(), quyenNhiemVu([...ALL_WRITE_KEYS, TASK_ASSIGN_PERMISSION]));
    expect(html).toMatch(
      new RegExp(
        `<button type="button" class="nut-phu" aria-controls="${ASSIGNMENT_UNIT_FIELD_ID}"[^>]*>` +
          '<span class="chip chip-ngung">Chuyển tiếp</span></button>',
      ),
    );
    // And the status block no longer offers it as a move.
    expect(html).not.toContain("Chuyển sang Chuyển tiếp");
    expect(html).toContain("Chuyển sang Tạm dừng");
  });

  it("the §5.4 edit form's read-only line now points at this block", () => {
    // The sentence itself is checked in `so-nhiem-vu.test.tsx` (FormSuaKhoiVanBan); here: it names
    // the block's title verbatim, so the pointer cannot drift from the heading.
    expect(ASSIGNMENT_TITLE).toBe("Giao việc, chuyển việc");
  });
});

function renderForm(
  t: petitions_nhiemVuRa,
  f: AssignmentForm = assignmentFormFromTask(t),
  extra: { error?: string | null; directory?: KetQua<identity_danhBaChonNguoiRa> | null } = {},
): string {
  return renderToStaticMarkup(
    <TaskAssignmentForm
      task={t}
      units={UNITS}
      directory={extra.directory === undefined ? DIRECTORY : extra.directory}
      labels={BANG_NHAN_MAC_DINH}
      form={f}
      change={() => {}}
      sending={false}
      error={extra.error ?? null}
      done={null}
      submit={() => {}}
    />,
  );
}

describe("the block — what each state says", () => {
  it("opens with nothing changed: button off, and the sentence says why", () => {
    const html = renderForm(task());
    expect(html).toMatch(/<button type="submit" class="nut-chinh" disabled=""[^>]*>Giao việc/);
    expect(html).toContain(asInHtml(ASSIGNMENT_NO_CHANGE));
  });

  it("unit cleared: button off, the unit sentence — not the no-change one", () => {
    const t = task();
    const html = renderForm(t, form(t, { unit: "" }));
    expect(html).toContain(asInHtml(ASSIGNMENT_UNIT_REQUIRED));
    expect(html).not.toContain(asInHtml(ASSIGNMENT_NO_CHANGE));
    expect(html).toMatch(/<button type="submit" class="nut-chinh" disabled=""/);
  });

  it("a real change: button on, no blocking sentence", () => {
    const t = task();
    const html = renderForm(t, form(t, { unit: UNIT_B }));
    expect(html).not.toMatch(/<button type="submit" class="nut-chinh" disabled=""/);
    expect(html).not.toContain(asInHtml(ASSIGNMENT_NO_CHANGE));
  });

  it("the server's refusal VERBATIM, `role=alert`", () => {
    const refusal = "Cán bộ được chọn không nhận được việc. Hãy chọn người khác trong danh sách.";
    expect(renderForm(task(), undefined, { error: refusal })).toContain(
      `<p class="thong-bao-loi" role="alert">${refusal}</p>`,
    );
  });

  it("`theo-van-ban` draws lead unit and monitor; `co-ban` does not", () => {
    const withFields = renderForm(task());
    expect(withFields).toContain('id="giao-lai-co-quan-chu-tri"');
    expect(withFields).toContain('id="giao-lai-chuyen-vien"');
    const plain = renderForm(task({ type: "co-ban", lead_unit: "", monitor: "" }));
    expect(plain).not.toContain('id="giao-lai-co-quan-chu-tri"');
    expect(plain).not.toContain('id="giao-lai-chuyen-vien"');
  });

  it("the effect sentence is always there and uses the COMMUNE's label of `moi-giao`", () => {
    const items = BANG_NHAN_MAC_DINH.thuTu.map((code, i) => ({
      code,
      label: code === "moi-giao" ? "Việc mới về xã" : BANG_NHAN_MAC_DINH.nhan[code],
      order: i + 1,
      role: "chinh",
      default_label: BANG_NHAN_MAC_DINH.nhan[code],
      default_order: i + 1,
      customised: code === "moi-giao",
    }));
    const { bang, canhBao } = docBangNhanTrangThai({ ok: true, duLieu: { items } });
    expect(canhBao).toBeNull();
    const note = assignmentEffectNote(bang, true);
    expect(note).toContain("“Việc mới về xã”");
    expect(note).not.toContain("Mới giao");
    expect(note).toContain("Hạn xử lý giữ nguyên");
    expect(assignmentEffectNote(BANG_NHAN_MAC_DINH, false)).not.toContain("cơ quan chủ trì");
    expect(renderForm(task())).toContain(asInHtml(assignmentEffectNote(BANG_NHAN_MAC_DINH, true)));
  });

  it("a unit no longer in the catalogue still shows as the current value, not as `Chưa xác định`", () => {
    expect(unitOptions(UNITS, "01JGONE")).toContainEqual({ id: "01JGONE", name: "01JGONE" });
    expect(unitOptions(UNITS, UNIT_A)).toHaveLength(UNITS.length);
    expect(unitOptions(UNITS, "")).toHaveLength(UNITS.length);
  });
});

describe("the call — POST `/api/v1/tasks/{ma}/assignment`, the body as built", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  function stubFetch(res: Response) {
    const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
    vi.stubGlobal("fetch", fake);
    return fake;
  }

  it("path, method and exactly the fields given — nothing smuggled in", async () => {
    const fake = stubFetch(
      new Response(JSON.stringify(task({ unit: UNIT_B, status: "moi-giao" })), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    // A structurally typed object carrying `status` passes `tsc`; the call must not forward it.
    const smuggled = { unit: UNIT_B, note: "Lý do", status: "hoan-thanh" } as petitions_taskAssignmentIn;
    const result = await reassignTask("NV/19", smuggled);
    expect(fake).toHaveBeenCalledTimes(1);
    expect(fake.mock.calls[0]?.[0]).toMatch(/\/api\/v1\/tasks\/NV%2F19\/assignment$/);
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(JSON.parse(String(fake.mock.calls[0]?.[1]?.body))).toEqual({ unit: UNIT_B, note: "Lý do" });
    expect(result.ok).toBe(true);
  });

  it("\"\" survives to the wire (it means CLEAR); an empty note does not", async () => {
    const fake = stubFetch(
      new Response(JSON.stringify(task()), { status: 200, headers: { "Content-Type": "application/json" } }),
    );
    await reassignTask("NV19", { assignee: "", monitor: "", note: "" });
    expect(JSON.parse(String(fake.mock.calls[0]?.[1]?.body))).toEqual({ assignee: "", monitor: "" });
  });

  it("409 `no_change`: the server's sentence comes back unchanged", async () => {
    const sentence =
      "nhiệm vụ: bộ phận, người thực hiện, cơ quan chủ trì và chuyên viên theo dõi đã đúng như yêu cầu — không có gì để đổi";
    stubFetch(
      new Response(JSON.stringify({ code: "no_change", message: sentence, trace_id: "01JTRACE" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const result = await reassignTask("NV19", { unit: UNIT_B });
    expect(result).toEqual({ ok: false, thongBao: sentence });
  });
});
