import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

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
  ASSIGNMENT_NOTHING_CHOSEN,
  assignmentOutcomeText,
  assignmentSubmitLabel,
  staffInUnit,
  unitWouldBeCleared,
  ASSIGNMENT_TITLE,
  ASSIGNMENT_UNIT_FIELD_ID,
  assignmentBody,
  assignmentEffectNote,
  assignmentFormFromTask,
  canShowAssignment,
  unitOptions,
  type AssignmentForm,
} from "./task-assignment";
import { TaskAssignmentForm } from "./task-assignment-block";
import { NOT_SENT } from "./task-transitions.fixture";

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

function task(patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    child_count: 0,
    allowed_transitions: [],
    updated_at: "2026-06-01T02:00:00Z",
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

  it("unit changed: ONLY `unit` goes — an unchanged assignee is NOT resent", () => {
    // The load-bearing case: every staff code sent is checked with identity, so resending STAFF_A
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

  it("\"đổi chuyên viên theo dõi\" IS changing the assignee (ADR 0065 NV5) — `assignee`, never `monitor`", () => {
    for (const type of ["theo-van-ban", "co-ban"]) {
      const t = task({ type });
      const body = assignmentBody(form(t, { assignee: STAFF_B }), t);
      expect(body).toEqual({ assignee: STAFF_B });
    }
  });

  it("NEVER `lead_unit` / `monitor`, whatever a stale form object holds — the server answers 400", () => {
    const t = task();
    // A form shaped before ADR 0065 NV5 (an old draft, a spread of a task read from a cache).
    const stale = { ...form(t), leadUnit: UNIT_B, monitor: STAFF_B, lead_unit: UNIT_B } as AssignmentForm;
    expect(assignmentBody(stale, t)).toBeNull();
    const withChange = assignmentBody({ ...stale, unit: UNIT_B, assignee: STAFF_B }, t);
    expect(withChange).toEqual({ unit: UNIT_B, assignee: STAFF_B });
    expect(withChange).not.toHaveProperty("lead_unit");
    expect(withChange).not.toHaveProperty("monitor");
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

  it("denied on a finished task, whatever the keys — and without the key, everywhere", () => {
    expect(canShowAssignment(WITH, "hoan-thanh")).toBe(false);
    expect(canShowAssignment(WITHOUT, "dang-thuc-hien")).toBe(false);
  });

  it("an old `chuyen-tiep` row is NOT finished — the block shows (server `CheckAssignable`)", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: this pinned `chuyen-tiep` as terminal (false). The server now
    // moves such rows on and refuses assignment only on `hoan-thanh` (`task_assignment.go:121-126`).
    expect(canShowAssignment(WITH, "chuyen-tiep")).toBe(true);
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
      dong={() => {}}
      doiTrangThai={NOT_SENT}
      xoa={NOT_SENT}
      guiDeNghiLuiHan={() => Promise.resolve({ ok: false, thongBao: "" })}
      quyetDinh={() => Promise.resolve({ ok: false, thongBao: "" })}
      suaKhoiVanBan={NOT_CALLED}
      docLaiChiTiet={NOT_CALLED}
      extensionRefreshKey="0"
      onExtensionDecided={() => {}}
      openTask={() => {}}
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
    // Spec 07 §6e: the drawer's section card carries the title; the block is the page-grey box.
    expect(html).toContain(`id="tieu-de-giao-viec-chuyen-viec" class="text-navy m-0 mb-2.5 text-[12.5px] font-bold">${ASSIGNMENT_TITLE}</h3>`);
    expect(html).toContain(`<label for="${ASSIGNMENT_UNIT_FIELD_ID}" class="text-ink block text-[11.5px] font-medium">Bộ phận</label>`);
    expect(html).toContain(">Người thực hiện</label>");
    expect(html).toContain(`>${ASSIGNMENT_NOTE_LABEL}</label>`);
    // `[UserPlus] Giao việc` — a primary button (`nut-chinh` is the legacy hook it still carries).
    expect(html).toMatch(/<button class="nut-chinh[^"]*" type="submit"[^>]*><svg[^>]*lucide-user-plus[^>]*>.*?<\/svg>Giao việc<\/button>/);
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

  it("an OLD `chuyen-tiep` row: its label still renders, lit; it moves on as the server lists", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: this pinned the row as terminal — no block, no moves, "không có
    // lối ra". The server's map now lets it go to `da-tiep-nhan` / `dang-thuc-hien` and assignment
    // refuses only `hoan-thanh`; the moves drawn are the row's `allowed_transitions`.
    const html = drawer(
      task({ status: "chuyen-tiep", allowed_transitions: ["da-tiep-nhan", "dang-thuc-hien"] }),
      quyenNhiemVu([...ALL_WRITE_KEYS, TASK_ASSIGN_PERMISSION]),
    );
    expect(html).toMatch(/data-step="current">Chuyển tiếp<\/span>/);
    expect(html).toContain(BLOCK_MARK);
    expect(html).toContain("Chuyển sang Đã tiếp nhận");
    expect(html).toContain("Chuyển sang Đang thực hiện");
    expect(html).not.toContain("không có lối ra khỏi trạng thái này");
  });

  it("stepper: with the block shown, `Chuyển tiếp` is a button pointing at the block — not a move", () => {
    // The server's list for `dang-thuc-hien` (3b2330b) — the drawer draws only what the row carries.
    const html = drawer(
      task({ allowed_transitions: ["cho-duyet", "hoan-thanh", "tam-dung"] }),
      quyenNhiemVu([...ALL_WRITE_KEYS, TASK_ASSIGN_PERMISSION]),
    );
    expect(html).toMatch(
      new RegExp(
        `<button type="button" class="[^"]*" aria-controls="${ASSIGNMENT_UNIT_FIELD_ID}"[^>]*>` +
          // The chip's second line (time in status, a dash) follows the label inside the button.
        '<span class="[^"]*" data-step="branch">Chuyển tiếp</span><span[^>]*>—</span></button>',
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
  extra: { directory?: KetQua<identity_danhBaChonNguoiRa> | null } = {},
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
      submit={() => {}}
    />,
  );
}

describe("the block — what each state says", () => {
  // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (spec 07 §6e): the button is no longer greyed out with a sentence
  // above it; a press with nothing to hand over says so by toast, as the prototype does.
  const BLOCK_SRC = readFileSync(fileURLToPath(new URL("./task-assignment-block.tsx", import.meta.url)), "utf8");

  it("opens with nothing changed: the button is ON; the press says what to choose (toast), nothing is sent", () => {
    const html = renderForm(task());
    expect(html).not.toMatch(/<button class="nut-chinh[^"]*" type="submit" disabled=""/);
    expect(html).not.toContain(asInHtml(ASSIGNMENT_NO_CHANGE));
    expect(assignmentBody(assignmentFormFromTask(task()), task())).toBeNull();
    expect(ASSIGNMENT_NOTHING_CHOSEN).toBe("Chọn bộ phận hoặc người nhận việc.");
    expect(BLOCK_SRC).toContain("if (body === null) {\n      toast.error(ASSIGNMENT_NOTHING_CHOSEN);\n      return;");
  });

  it("unit cleared: the press says the unit sentence (toast) — never sends a hand-over to nobody", () => {
    const t = task();
    expect(unitWouldBeCleared(form(t, { unit: "" }), t)).toBe(true);
    expect(assignmentBody(form(t, { unit: "" }), t)).toBeNull();
    expect(BLOCK_SRC).toContain("if (unitWouldBeCleared(form, task)) {\n      toast.error(ASSIGNMENT_UNIT_REQUIRED);");
  });

  it("the button reads `Chuyển việc` once a reason is typed; the toast names what happened", () => {
    const t = task();
    expect(renderForm(t, form(t, { unit: UNIT_B, note: "Nghỉ phép" }))).toMatch(/<\/svg>Chuyển việc<\/button>/);
    expect(assignmentSubmitLabel({ note: "  " })).toBe("Giao việc");
    expect(assignmentOutcomeText({ unit: UNIT_B })).toBe("Đã giao việc.");
    expect(assignmentOutcomeText({ unit: UNIT_B, note: "Nghỉ phép" })).toBe("Đã chuyển việc và ghi vết.");
  });

  it("the person list follows the chosen unit (spec 07 §6e); an empty unit says so", () => {
    const staff = [
      { code: STAFF_A, full_name: "A", position: "", department_id: UNIT_A },
      { code: STAFF_B, full_name: "B", position: "", department_id: UNIT_B },
    ];
    expect(staffInUnit(staff, UNIT_A).map((c) => c.code)).toEqual([STAFF_A]);
    expect(staffInUnit(staff, "").map((c) => c.code)).toEqual([STAFF_A, STAFF_B]);
    expect(staffInUnit(staff, "01JKHONGAI")).toEqual([]);
  });

  it("a real change: button on, no blocking sentence", () => {
    const t = task();
    const html = renderForm(t, form(t, { unit: UNIT_B }));
    expect(html).not.toMatch(/<button type="submit" class="nut-chinh" disabled=""/);
    expect(html).not.toContain(asInHtml(ASSIGNMENT_NO_CHANGE));
  });

  it("the server's refusal VERBATIM — the error toast (lần 6 #4)", () => {
    expect(BLOCK_SRC).toContain("if (!result.ok) {\n        toast.error(result.thongBao);");
  });

  it("no lead unit / monitor field on ANY type — only `Bộ phận` and `Người thực hiện` (ADR 0065 NV5)", () => {
    for (const type of ["theo-van-ban", "co-ban"]) {
      const html = renderForm(task({ type }));
      expect(html).not.toContain('id="giao-lai-co-quan-chu-tri"');
      expect(html).not.toContain('id="giao-lai-chuyen-vien"');
      expect(html).not.toContain("Cơ quan chủ trì");
      expect(html).not.toContain("Chuyên viên");
      expect(html).toContain(`id="${ASSIGNMENT_UNIT_FIELD_ID}"`);
      expect(html).toContain(">Người thực hiện</label>");
    }
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
    const note = assignmentEffectNote(bang);
    expect(note).toContain("“Việc mới về xã”");
    expect(note).not.toContain("Mới giao");
    expect(note).toContain("Hạn xử lý giữ nguyên");
    expect(assignmentEffectNote(BANG_NHAN_MAC_DINH)).not.toContain("cơ quan chủ trì");
    expect(assignmentEffectNote(BANG_NHAN_MAC_DINH)).not.toContain("theo dõi");
    // The form no longer draws the note: the prototype box has none (review r2 F-1, 07/10/2026).
    expect(renderForm(task())).not.toContain(asInHtml(assignmentEffectNote(BANG_NHAN_MAC_DINH)));
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
    await reassignTask("NV19", { assignee: "", note: "" });
    expect(JSON.parse(String(fake.mock.calls[0]?.[1]?.body))).toEqual({ assignee: "" });
  });

  it("retired `lead_unit` / `monitor` NEVER reach the wire, even when a stale caller passes them (400 since ADR 0065 NV5)", async () => {
    const fake = stubFetch(
      new Response(JSON.stringify(task()), { status: 200, headers: { "Content-Type": "application/json" } }),
    );
    const stale = { assignee: STAFF_B, lead_unit: UNIT_B, monitor: STAFF_A } as petitions_taskAssignmentIn;
    await reassignTask("NV19", stale);
    const sent = JSON.parse(String(fake.mock.calls[0]?.[1]?.body)) as Record<string, unknown>;
    expect(sent).toEqual({ assignee: STAFF_B });
  });

  it("409 `no_change`: the server's sentence comes back unchanged", async () => {
    const sentence =
      "nhiệm vụ: bộ phận và người thực hiện đã đúng như yêu cầu — không có gì để đổi";
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
