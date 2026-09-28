import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";
import type { page_Result_petitions_nhiemVuRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import { parseDrillDown } from "@/lib/drill-down";
import { QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU } from "@/lib/quyen";

import { menuKey } from "./kanban-move-menu";
import {
  BANG_NHAN_MAC_DINH,
  KANBAN_MOVE_BUTTON,
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
 * THE LIMIT, STATED: no DOM here (`vitest.config.mts`), so no case fires a real `drop` or a real
 * click. What IS tested: the route and body `moveTaskStatus` sends; that the server's refusal
 * survives verbatim to the card; what the board renders in each phase (allowed, denied, pending,
 * refused, done); the menu keys; and — by reading the source, the precedent of
 * `noi-nhan-trang-thai.test.ts` — that the drop handler and the menu call the SAME function.
 */

const TASK: petitions_nhiemVuRa = {
  code: "NV19",
  type: "co-ban",
  bloc: "",
  priority: "",
  title: "Rà soát hồ sơ tồn đọng",
  description: "",
  status: "dang-thuc-hien",
  source: "truc-tiep",
  source_id: "",
  unit: "",
  assignee: "",
  assigner: "",
  lead_unit: "",
  monitor: "",
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
      move={move}
    />,
  );
}

function moveWith(sua: Partial<KanbanMove> = {}): KanbanMove {
  return { permissions: UPDATE_ONLY, pending: null, result: null, move: () => {}, ...sua };
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

describe("which moves a card offers — the drawer's list, one source", () => {
  it("`task.update`: forward step and the two branch steps; never `hoan-thanh`", () => {
    expect(clickableTransitions("dang-thuc-hien", UPDATE_ONLY)).toEqual([
      "cho-duyet",
      "tam-dung",
      "chuyen-tiep",
    ]);
    expect(clickableTransitions("cho-duyet", UPDATE_ONLY)).toEqual(["tam-dung", "chuyen-tiep"]);
  });

  it("`task.approve` adds `hoan-thanh`; the RETURN step stays out (it needs a reason)", () => {
    expect(clickableTransitions("cho-duyet", UPDATE_AND_APPROVE)).toEqual([
      "hoan-thanh",
      "tam-dung",
      "chuyen-tiep",
    ]);
  });

  it("no keys, or a dead-end status: nothing", () => {
    expect(clickableTransitions("dang-thuc-hien", NO_KEYS)).toEqual([]);
    expect(clickableTransitions("hoan-thanh", UPDATE_AND_APPROVE)).toEqual([]);
  });
});

describe("the board — allowed, denied, pending, refused, done", () => {
  it("ALLOWED: the card is draggable and has the `Chuyển sang cột…` menu button", () => {
    const html = board(TASK, moveWith());
    expect(html).toContain('draggable="true"');
    expect(html).toContain(`aria-label="${KANBAN_MOVE_BUTTON} (NV19)"`);
    expect(html).toContain('aria-haspopup="menu"');
    expect(html).toContain('aria-expanded="false"');
    expect(html).toContain(`>${KANBAN_MOVE_BUTTON}</button>`);
  });

  it("DENIED — no `task.update`: no drag handle, no menu; the card still opens", () => {
    const html = board(TASK, moveWith({ permissions: NO_KEYS }));
    expect(html).not.toContain("draggable");
    expect(html).not.toContain(KANBAN_MOVE_BUTTON);
    expect(html).toContain("Mở NV19");
  });

  it("DENIED — a finished task offers no move at all, whatever the keys", () => {
    const html = board({ ...TASK, status: "hoan-thanh" }, moveWith({ permissions: UPDATE_AND_APPROVE }));
    expect(html).not.toContain("draggable");
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
    expect(html).toMatch(/aria-haspopup="menu"[^>]*disabled=""/);
  });

  it("REFUSED: the server's sentence VERBATIM, `role=alert`, on the card that did not move", () => {
    const html = board(
      { ...TASK, status: "cho-duyet" },
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
    expect(SOURCE).toContain('move.move(task, c.ma, "drag")');
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
