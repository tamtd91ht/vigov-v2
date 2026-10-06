import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";
import type { page_Result_petitions_nhiemVuRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import { parseDrillDown } from "@/lib/drill-down";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  TASK_ASSIGN_PERMISSION,
} from "@/lib/quyen";

import { KANBAN_CARD_ROLE } from "./kanban-drag";
import { menuKey } from "./kanban-move-menu";
import { serverTransitions } from "./task-transitions.fixture";
import {
  BANG_NHAN_MAC_DINH,
  KANBAN_MOVE_BUTTON,
  MOI_TRANG_THAI,
  TRANG_THAI_CHINH,
  clickableTransitions,
  kanbanMoveDoneText,
  kanbanMovePendingText,
  kanbanMoveRefusedPrefix,
  quyenNhiemVu,
} from "./nhan-nhiem-vu";
import {
  BangKanban,
  SoNhiemVu,
  moveTaskStatus,
  type CotKanban,
  type DanhMucNhiemVu, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
  type KanbanMove,
} from "./so-nhiem-vu";

/**
 * Kanban moves (#14, 28/09/2026): drag-and-drop plus an equal keyboard path, both calling the
 * drawer's route `POST /api/v1/tasks/{ma}/status`.
 *
 * THE LIMIT, STATED: no DOM here (`vitest.config.mts`), so no case fires a real drag or a real
 * click — the drop decisions are pure and tested in `kanban-drag.test.ts`, the click in
 * `kanban-drag.interaction.test.tsx`. What IS tested: the route and body `moveTaskStatus` sends; that the server's refusal
 * survives verbatim to the card; what the board renders in each phase (allowed, denied, pending,
 * refused, done); the menu keys; and — by reading the source, the precedent of
 * `noi-nhan-trang-thai.test.ts` — that the drop handler and the menu call the SAME function.
 */

const ASSIGNEE = "CB-2026-3H8N2W";
const OTHER = "CB-2026-0P4X1Z";

const TASK: petitions_nhiemVuRa = {
  code: "NV19",
  child_count: 0,
  allowed_transitions: ["cho-duyet", "hoan-thanh", "tam-dung"],
  updated_at: "2026-06-01T02:00:00Z",
  type: "co-ban",
  bloc: "",
  priority: "",
  title: "Rà soát hồ sơ tồn đọng",
  description: "",
  status: "dang-thuc-hien",
  source: "truc-tiep",
  source_id: "",
  unit: "",
  assignee: ASSIGNEE,
  assigner: "",
  due_at: null,
  original_due_at: null,
  completed_at: null,
  progress: 0,
  result_summary: "",
  note: "",
  leader_approved: false,
  superior_acknowledged: false,
  parent: "",
  created_by: "CB-2026-VANTHU",
  created_at: "2026-06-01T02:00:00Z",
};

const CATALOGUES: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };
const UPDATE_ONLY = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]);
const UPDATE_AND_APPROVE = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU]);
const NO_KEYS = quyenNhiemVu([]);
const APPROVE_ONLY = quyenNhiemVu([QUYEN_DUYET_HOAN_THANH_NHIEM_VU]);

/** A row in `status` carrying the server's list for it. */
function at(status: string, patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return { ...TASK, status, allowed_transitions: serverTransitions(status), ...patch };
}

const CHILDREN_LEFT =
  "còn 3 việc con (NV20, NV21, NV22) — hoàn thành hết việc con rồi mới hoàn thành việc cha";

function page(items: readonly petitions_nhiemVuRa[]): page_Result_petitions_nhiemVuRa {
  return { items: [...items], next_cursor: "", has_more: false };
}

function board(task: petitions_nhiemVuRa, move: KanbanMove | null): string {
  const cot: readonly CotKanban[] = TRANG_THAI_CHINH.map((ma) => ({
    ma,
    tai: { pha: "xong" as const, duLieu: page(ma === task.status ? [task] : []) },
  }));
  return renderToStaticMarkup(
    <BangKanban
      cot={cot}
      danhMuc={CATALOGUES}
      nhanTT={BANG_NHAN_MAC_DINH}
      bayGio={new Date("2026-09-15T03:00:00Z")}
      maDangMo={null}
      moNhiemVu={() => {}}
      counts={{ pha: "dangTai" }}
      move={move}
    />,
  );
}

function moveWith(sua: Partial<KanbanMove> = {}): KanbanMove {
  return {
    permissions: UPDATE_ONLY,
    staffCode: OTHER,
    pending: null,
    result: null,
    move: () => {},
    ...sua,
  };
}

/** The HTML of ONE column, by status code. */
function column(html: string, status: string): string {
  const start = html.indexOf(`aria-labelledby="cot-kanban-${status}"`);
  expect(start).toBeGreaterThanOrEqual(0);
  return html.slice(start, html.indexOf("</section>", start));
}

/** A string as it sits in the HTML — `renderToStaticMarkup` escapes `"` and `&`. */
function asInHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("which moves a card offers — the SERVER's list, one source", () => {
  // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: every case here used to read the screen's own copy of the
  // lifecycle (`CHUYEN_DUOC`, `clickableTransitions(status, keys)`). The row now carries
  // `allowed_transitions` (3b2330b) and the copy is gone; the cases pin that the screen draws the
  // server's list, gated by (row: `task.update` or assignee) and (move: `task.approve` if needed).

  it("`task.update`, not the assignee: the listed moves minus those needing approval", () => {
    // ADR 0065 NV1: the direct completion needs no `task.approve`.
    expect(clickableTransitions(at("dang-thuc-hien"), UPDATE_ONLY, OTHER)).toEqual([
      "cho-duyet",
      "hoan-thanh",
      "tam-dung",
    ]);
    expect(clickableTransitions(at("moi-giao"), UPDATE_ONLY, OTHER)).toEqual([
      "da-tiep-nhan",
      "dang-thuc-hien",
      "tam-dung",
    ]);
    // `cho-duyet` can no longer be paused — the server stopped listing it.
    expect(clickableTransitions(at("cho-duyet"), UPDATE_ONLY, OTHER)).toEqual([]);
  });

  it("`task.approve` adds the sign-off from `cho-duyet`; reason moves stay out", () => {
    expect(clickableTransitions(at("dang-thuc-hien"), UPDATE_AND_APPROVE, OTHER)).toEqual([
      "cho-duyet",
      "hoan-thanh",
      "tam-dung",
    ]);
    // Return (cho-duyet → dang-thuc-hien) and reopen (hoan-thanh → dang-thuc-hien) need a reason.
    expect(clickableTransitions(at("cho-duyet"), UPDATE_AND_APPROVE, OTHER)).toEqual(["hoan-thanh"]);
    expect(clickableTransitions(at("hoan-thanh"), UPDATE_AND_APPROVE, OTHER)).toEqual([]);
  });

  it("`Tiếp tục` after a pause: exactly the server's three, no history rule", () => {
    expect(clickableTransitions(at("tam-dung"), UPDATE_ONLY, OTHER)).toEqual([
      "moi-giao",
      "da-tiep-nhan",
      "dang-thuc-hien",
    ]);
  });

  it("THE ASSIGNEE without `task.update` moves their own task (ea55113)", () => {
    // ADR 0065 NV1: completing straight from `dang-thuc-hien` needs no `task.approve`.
    expect(clickableTransitions(at("dang-thuc-hien"), NO_KEYS, ASSIGNEE)).toEqual([
      "cho-duyet",
      "hoan-thanh",
      "tam-dung",
    ]);
    // ...but work already sent up for review waits for a reviewer: no sign-off without the key.
    expect(clickableTransitions(at("cho-duyet"), NO_KEYS, ASSIGNEE)).toEqual([]);
    // With `task.approve`, the assignee may sign off their own reviewed task — the server's layers.
    expect(clickableTransitions(at("cho-duyet"), APPROVE_ONLY, ASSIGNEE)).toEqual(["hoan-thanh"]);
  });

  it("DENIED — neither `task.update` nor the assignee: nothing, even with `task.approve`", () => {
    expect(clickableTransitions(at("dang-thuc-hien"), NO_KEYS, OTHER)).toEqual([]);
    expect(clickableTransitions(at("dang-thuc-hien"), APPROVE_ONLY, OTHER)).toEqual([]);
  });

  it("DENIED — an unread session (empty code) never matches, not even an UNASSIGNED task", () => {
    // `"" === ""` would hand every unassigned task to every account whose session failed to load.
    expect(clickableTransitions(at("dang-thuc-hien", { assignee: "" }), NO_KEYS, "")).toEqual([]);
  });

  it("only what the row lists: an empty list draws nothing, an unknown code is left out", () => {
    expect(
      clickableTransitions(at("dang-thuc-hien", { allowed_transitions: [] }), UPDATE_AND_APPROVE, OTHER),
    ).toEqual([]);
    expect(
      clickableTransitions(
        at("dang-thuc-hien", { allowed_transitions: ["da-ban-giao", "tam-dung"] }),
        UPDATE_ONLY,
        OTHER,
      ),
    ).toEqual(["tam-dung"]);
  });

  it("`chuyen-tiep` is offered from NO status, with every key — the server lists it nowhere", () => {
    const allKeys = quyenNhiemVu([
      QUYEN_CAP_NHAT_NHIEM_VU,
      QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
      TASK_ASSIGN_PERMISSION,
    ]);
    for (const from of MOI_TRANG_THAI) {
      expect(clickableTransitions(at(from), allKeys, OTHER)).not.toContain("chuyen-tiep");
    }
  });

  it("menu, drop target and drawer buttons all read that ONE list (source wiring)", () => {
    // The menu is CLOSED in static markup, so rendering it proves nothing about its items; the
    // cases above are the proof, and this pins that all three places still read it.
    const source = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(source).toContain(
      "move === null ? [] : clickableTransitions(nhiemVu, move.permissions, move.staffCode);",
    );
    // The drop targets: `dragTargets` in `kanban-drag.ts`, which is `clickableTransitions` again.
    expect(source).toContain("const allowedTargets = dragging === null ? [] : dragTargets(dragging, move);");
    const drag = readFileSync(fileURLToPath(new URL("./kanban-drag.ts", import.meta.url)), "utf8");
    expect(drag).toContain("return clickableTransitions(task, gate.permissions, gate.staffCode);");
    expect(source).toContain("const buocBamDuoc = clickableTransitions(nhiemVu, quyen, maNguoiDangNhap);");
    expect(source).toContain("staffCode: maNguoiDangNhap,");
    // No second lifecycle map may come back.
    expect(source).not.toContain("CHUYEN_DUOC");
  });
});

describe("the board — allowed, denied, pending, refused, done", () => {
  it("ALLOWED: the card is draggable and has the `Chuyển sang cột…` menu button", () => {
    const html = board(TASK, moveWith());
    // @dnd-kit (06/10/2026): the card is focusable and announced as draggable, not `draggable="true"`.
    expect(html).toContain(`aria-roledescription="${KANBAN_CARD_ROLE}"`);
    expect(html).toMatch(/<article[^>]*tabindex="0"/);
    expect(html).toContain(`aria-label="${KANBAN_MOVE_BUTTON} (NV19)"`);
    expect(html).toContain('aria-haspopup="menu"');
    expect(html).toContain('aria-expanded="false"');
    // Compact on the card (06/10/2026): an icon button whose tooltip carries the words; the
    // accessible name above is unchanged.
    expect(html).toContain('class="nut-chuyen-cot-gon"');
    expect(html).toContain(`title="${KANBAN_MOVE_BUTTON}"`);
  });

  it("DENIED — no `task.update` and not the assignee: no drag handle, no menu; the card still opens", () => {
    const html = board(TASK, moveWith({ permissions: NO_KEYS }));
    expect(html).not.toContain("draggable");
    expect(html).not.toContain(KANBAN_CARD_ROLE);
    expect(html).not.toMatch(/<article[^>]*tabindex=/);
    expect(html).not.toContain(KANBAN_MOVE_BUTTON);
    // The card body is its open button (prototype, 06/10/2026).
    expect(html).toMatch(/aria-expanded="false"><span class="ma-muc[^"]*">NV19</);
  });

  it("ALLOWED — the ASSIGNEE without `task.update` gets the drag handle and the menu", () => {
    const html = board(TASK, moveWith({ permissions: NO_KEYS, staffCode: ASSIGNEE }));
    expect(html).toContain(`aria-roledescription="${KANBAN_CARD_ROLE}"`);
    expect(html).toContain(`aria-label="${KANBAN_MOVE_BUTTON} (NV19)"`);
  });

  it("a finished task: no drag, no menu — its only move (reopen) needs a reason, in the drawer", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: was "no move at all, whatever the keys" — `hoan-thanh` was a
    // dead end. The server now lists the reopen; it is a reason form, never a card move.
    const html = board(at("hoan-thanh"), moveWith({ permissions: UPDATE_AND_APPROVE }));
    expect(html).not.toContain("draggable");
    expect(html).not.toContain(KANBAN_CARD_ROLE);
    expect(html).not.toContain(KANBAN_MOVE_BUTTON);
  });

  it("PENDING: the card stays in its OWN column, says so, and every move control is off", () => {
    const html = board(TASK, moveWith({ pending: { code: "NV19", target: "cho-duyet" } }));
    const pending = asInHtml(kanbanMovePendingText(BANG_NHAN_MAC_DINH, "NV19", "cho-duyet"));
    // No optimistic move: NV19 is still under `dang-thuc-hien`, not under `cho-duyet`.
    expect(column(html, "dang-thuc-hien")).toContain("NV19");
    expect(column(html, "cho-duyet")).not.toContain("NV19");
    expect(html).toContain(pending);
    expect(html).toMatch(/role="status"[^>]*>[^<]*Đang chuyển NV19/);
    expect(html).not.toContain("draggable");
    expect(html).not.toContain(KANBAN_CARD_ROLE);
    expect(html).toMatch(/aria-haspopup="menu"[^>]*disabled=""/);
  });

  it("REFUSED: the server's sentence VERBATIM, `role=alert`, on the card that did not move", () => {
    const html = board(
      at("cho-duyet"),
      moveWith({
        permissions: UPDATE_AND_APPROVE,
        result: { code: "NV19", target: "hoan-thanh", via: "menu", ok: false, message: CHILDREN_LEFT },
      }),
    );
    const own = column(html, "cho-duyet");
    expect(own).toContain('role="alert"');
    expect(own).toContain(
      `${asInHtml(kanbanMoveRefusedPrefix(BANG_NHAN_MAC_DINH, "hoan-thanh"))} ${CHILDREN_LEFT}`,
    );
    expect(column(html, "hoan-thanh")).not.toContain("NV19");
  });

  it("DONE: the live region says where the card went", () => {
    const html = board(
      { ...TASK, status: "cho-duyet" },
      moveWith({ result: { code: "NV19", target: "cho-duyet", via: "drag", ok: true } }),
    );
    expect(html).toContain(asInHtml(kanbanMoveDoneText(BANG_NHAN_MAC_DINH, "NV19", "cho-duyet")));
  });

  it("read-only board (no `move`): no live region, no controls", () => {
    const html = board(TASK, null);
    expect(html).not.toContain("trang-thai-chuyen-cot");
    expect(html).not.toContain(KANBAN_MOVE_BUTTON);
  });
});

describe("the call — the drawer's route, the server's words", () => {
  function stubFetch(res: Response) {
    const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
    vi.stubGlobal("fetch", fake);
    return fake;
  }

  it("keyboard path: POST `/api/v1/tasks/NV19/status` with the target and no note", async () => {
    const fake = stubFetch(
      new Response(JSON.stringify({ ...TASK, status: "cho-duyet" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const r = await moveTaskStatus("NV19", "cho-duyet", "menu");
    expect(fake).toHaveBeenCalledTimes(1);
    expect(fake.mock.calls[0]?.[0]).toMatch(/\/api\/v1\/tasks\/NV19\/status$/);
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(JSON.parse(String(fake.mock.calls[0]?.[1]?.body))).toEqual({ status: "cho-duyet" });
    expect(r).toEqual({ code: "NV19", target: "cho-duyet", via: "menu", ok: true });
  });

  it("409 completing a parent: the child-task list comes back UNCHANGED", async () => {
    stubFetch(
      new Response(JSON.stringify({ code: "task_tree", message: CHILDREN_LEFT, trace_id: "01JTRACE" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      }),
    );
    expect(await moveTaskStatus("NV19", "hoan-thanh", "drag")).toEqual({
      code: "NV19",
      target: "hoan-thanh",
      via: "drag",
      ok: false,
      message: CHILDREN_LEFT,
    });
  });
});

describe("both paths call the SAME function (source wiring)", () => {
  const SOURCE = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");

  it("drop and menu both go through `move.move`, which is `moveOnKanban` → `moveTaskStatus`", () => {
    expect(SOURCE).toContain("dropOnKanban(task, e.over?.id ?? null, move);");
    const drag = readFileSync(fileURLToPath(new URL("./kanban-drag.ts", import.meta.url)), "utf8");
    expect(drag).toContain('gate.move(task, target, "drag");');
    expect(SOURCE).toContain('move.move(nhiemVu, t, "menu")');
    expect(SOURCE).toContain("move: moveOnKanban,");
    expect(SOURCE).toContain("moveTaskStatus(task.code, target, via)");
    expect(SOURCE).toMatch(/return doiTrangThaiNhiemVu\(code, target\)/);
  });

  it("the Kanban renders only in Kanban view — which a drill-down never is", () => {
    expect(SOURCE).toContain('const viewMode: CheDoXem = drillDownActive ? "danh-sach" : cheDoXem;');
    expect(SOURCE).toContain('{viewMode === "kanban" && (');
  });
});

describe("drill-down: forced list view, so no Kanban and no move controls", () => {
  const render = (node: ReactNode) => renderToStaticMarkup(<PhienProvider>{node}</PhienProvider>);

  it("a drill-down from /tong-quan draws no board", () => {
    const html = render(<SoNhiemVu drillDown={parseDrillDown("tasks", { metric: "suspended" })} />);
    expect(html).not.toContain('aria-label="Bảng Kanban nhiệm vụ"');
    expect(html).not.toContain(KANBAN_MOVE_BUTTON);
  });

  it("without one, the board is the default view", () => {
    expect(render(<SoNhiemVu />)).toContain('aria-label="Bảng Kanban nhiệm vụ"');
  });
});

describe("menu keys — WAI-ARIA menu button (`menuKey`)", () => {
  it("arrows wrap; Home/End jump", () => {
    expect(menuKey(2, "ArrowDown", 3)).toEqual({ active: 0, close: false });
    expect(menuKey(0, "ArrowUp", 3)).toEqual({ active: 2, close: false });
    expect(menuKey(1, "Home", 3)).toEqual({ active: 0, close: false });
    expect(menuKey(0, "End", 3)).toEqual({ active: 2, close: false });
  });

  it("Escape and Tab close; other keys are left to the button (Enter/Space activate it)", () => {
    expect(menuKey(1, "Escape", 3)?.close).toBe(true);
    expect(menuKey(1, "Tab", 3)?.close).toBe(true);
    expect(menuKey(1, "Enter", 3)).toBeNull();
    expect(menuKey(1, "a", 3)).toBeNull();
  });
});
