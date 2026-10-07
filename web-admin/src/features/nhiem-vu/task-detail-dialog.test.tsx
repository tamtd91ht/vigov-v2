// @vitest-environment jsdom
//
// jsdom for this file: what it pins — a click opening the dialog, the history entry it pushes, Back
// closing it, focus going back to the row — are events, history and focus, none of which a string
// render can show. Same reasoning as `features/noi-dung/content-delete-dialog.test.tsx`.

import { act, useState, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { toast } from "sonner";

import { LargeDialog } from "@/components/ui/large-dialog";
import type { KetQua } from "@/lib/api/goi";
import type { petitions_nhiemVuRa, petitions_suaNhiemVuVao } from "@/lib/api/schema.gen";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_TAO_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
} from "@/lib/quyen";

import {
  BANG_NHAN_MAC_DINH,
  CHUA_PHAN_CONG,
  NHAN_LY_DO_TRA_LAI,
  O_TRONG,
  REOPEN_REASON_LABEL,
  quyenNhiemVu,
  reopenNote,
} from "./nhan-nhiem-vu";
import { ChiTietNhiemVu, SoNhiemVu, TASK_DELETE_BUTTON, TASK_INFO_EDIT_LABEL } from "./so-nhiem-vu";
import { readTaskParam, searchWithTask, useTaskDialogUrl } from "./task-dialog-url";
import {
  STATUS_ATTACH_BUTTON,
  STATUS_COMPOSE_ID,
  STATUS_CONFIRM_BUTTON,
  STATUS_CONFIRM_HANDOVER,
  TaskStatusPipeline,
  STATUS_MOVE_DENIED,
  STATUS_NOTE_LABEL,
  reasonMoveName,
  statusMoveDoneText,
  statusMoveName,
} from "./task-status-pipeline";
import { NOT_SENT, serverTransitions } from "./task-transitions.fixture";

type Permissions = ReturnType<typeof quyenNhiemVu>;

const ALL_KEYS = [
  QUYEN_TAO_NHIEM_VU,
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
];

// Outcomes are toasts (ADR 0068 lần 6 #4): the calls are what these cases read.
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

// The session: a staff code and the five write keys. The screen only reads these two fields.
vi.mock("@/features/phien/phien-hien-tai", async (original) => ({
  ...(await original<typeof import("@/features/phien/phien-hien-tai")>()),
  usePhien: () => ({ ok: true, duLieu: { staff: { code: "CB-2026-7K3M9Q" }, permissions: ALL_KEYS } }),
}));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

function task(patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    child_count: 0,
    extension_count: 0,
    pending_extension: false,
    allowed_transitions: serverTransitions(patch.status ?? "moi-giao"),
    updated_at: "2026-06-01T02:00:00Z",
    type: "co-ban",
    bloc: "",
    priority: "",
    title: "Rà soát danh sách hộ nghèo quý III",
    description: "",
    status: "moi-giao",
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: "",
    assigner: "CB-2026-7K3M9Q",
    due_at: "2026-10-20T23:59:59+07:00",
    original_due_at: "2026-10-20T23:59:59+07:00",
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

const NOT_CALLED = (): Promise<KetQua<never>> => Promise.resolve({ ok: false, thongBao: "không gọi" });

type Writes = {
  move?: (target: string, note?: string) => Promise<KetQua<petitions_nhiemVuRa>>;
  remove?: (reason: string) => Promise<KetQua<unknown>>;
  save?: (body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
};

function detail(permissions: Permissions, patch: Partial<petitions_nhiemVuRa> = {}, writes: Writes = {}) {
  return (
    <ChiTietNhiemVu
      nhiemVu={task(patch)}
      vanBan={{ pha: "dangTai" }}
      danhMuc={{ loai: [], mucUuTien: [], khoi: [], boPhan: [] }}
      nhanTT={BANG_NHAN_MAC_DINH}
      tenBoPhan={new Map()}
      bayGio={new Date("2026-09-15T03:00:00Z")}
      maNguoiDangNhap="CB-2026-7K3M9Q"
      quyen={permissions}
      dangGui={false}
      dong={() => {}}
      doiTrangThai={writes.move ?? NOT_SENT}
      xoa={writes.remove ?? NOT_SENT}
      guiDeNghiLuiHan={NOT_CALLED}
      quyetDinh={NOT_CALLED}
      suaKhoiVanBan={writes.save ?? NOT_CALLED}
      docLaiChiTiet={NOT_CALLED}
      extensionRefreshKey="0"
      onExtensionDecided={() => {}}
      openTask={() => {}}
      addChild={null}
      reassign={NOT_CALLED}
    />
  );
}

function detailHtml(permissions: Permissions, patch: Partial<petitions_nhiemVuRa> = {}) {
  return renderToStaticMarkup(detail(permissions, patch));
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

function mount(node: ReactNode): void {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() => root!.render(node));
}

/** Let the stubbed fetches, their `.json()` and the effects they feed settle. */
async function settle(): Promise<void> {
  for (let i = 0; i < 12; i++) await act(async () => {});
}

beforeEach(() => {
  window.history.replaceState(null, "", "/nhiem-vu");
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("searchWithTask / readTaskParam — the `?task=` parameter, nothing else", () => {
  it("adds, replaces and drops `task`; every other parameter stays, in order", () => {
    expect(searchWithTask("?status=moi-giao&unit=bp1", "NV19")).toBe("?status=moi-giao&unit=bp1&task=NV19");
    expect(searchWithTask("?task=NV19&status=moi-giao", "NV20")).toBe("?status=moi-giao&task=NV20");
    expect(searchWithTask("?status=moi-giao&task=NV19", null)).toBe("?status=moi-giao");
    expect(searchWithTask("?task=NV19", null)).toBe("");
  });

  it("reads under the server-side rule: a repeated or empty `task` is no task", () => {
    expect(readTaskParam("?task=NV19")).toBe("NV19");
    expect(readTaskParam("?task=NV19&task=NV20")).toBeNull();
    expect(readTaskParam("?task=%20")).toBeNull();
    expect(readTaskParam("?status=moi-giao")).toBeNull();
  });
});

function UrlHarness({ code, onNavigate }: { code: string | null; onNavigate: (c: string | null) => void }) {
  useTaskDialogUrl(code, onNavigate);
  return null;
}

describe("useTaskDialogUrl — one pushed entry per opening, Back closes", () => {
  it("open from the list PUSHES once, keeping filters and #hash; parent/child REPLACES", () => {
    window.history.replaceState(null, "", "/nhiem-vu?status=moi-giao#hang-cho-lui-han");
    const push = vi.spyOn(window.history, "pushState");
    const replace = vi.spyOn(window.history, "replaceState");
    const nav = vi.fn();
    mount(<UrlHarness code={null} onNavigate={nav} />);
    expect(push).not.toHaveBeenCalled();

    act(() => root!.render(<UrlHarness code="NV19" onNavigate={nav} />));
    expect(push).toHaveBeenCalledTimes(1);
    expect(window.location.search).toBe("?status=moi-giao&task=NV19");
    expect(window.location.hash).toBe("#hang-cho-lui-han");

    act(() => root!.render(<UrlHarness code="NV20" onNavigate={nav} />));
    expect(push).toHaveBeenCalledTimes(1);
    expect(replace).toHaveBeenCalledTimes(1);
    expect(window.location.search).toBe("?status=moi-giao&task=NV20");
  });

  it("closing what it pushed goes BACK (one Back = the list with its filters)", () => {
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    const nav = vi.fn();
    mount(<UrlHarness code={null} onNavigate={nav} />);
    act(() => root!.render(<UrlHarness code="NV19" onNavigate={nav} />));
    act(() => root!.render(<UrlHarness code={null} onNavigate={nav} />));
    expect(back).toHaveBeenCalledTimes(1);
  });

  it("a `?task=` link opened on load is closed by REPLACE — Back would leave the screen", () => {
    window.history.replaceState(null, "", "/nhiem-vu?status=moi-giao&task=NV19");
    const push = vi.spyOn(window.history, "pushState");
    const back = vi.spyOn(window.history, "back");
    const nav = vi.fn();
    mount(<UrlHarness code={null} onNavigate={nav} />);
    // The detail read answered: the dialog shows the code already in the address bar.
    act(() => root!.render(<UrlHarness code="NV19" onNavigate={nav} />));
    expect(push).not.toHaveBeenCalled();
    act(() => root!.render(<UrlHarness code={null} onNavigate={nav} />));
    expect(back).not.toHaveBeenCalled();
    expect(window.location.search).toBe("?status=moi-giao");
  });

  it("a link whose read failed leaves the address alone", () => {
    window.history.replaceState(null, "", "/nhiem-vu?task=NV99");
    const replace = vi.spyOn(window.history, "replaceState");
    mount(<UrlHarness code={null} onNavigate={() => {}} />);
    expect(replace).not.toHaveBeenCalled();
    expect(window.location.search).toBe("?task=NV99");
  });

  it("Back / Forward report the code now in the address bar", () => {
    const nav = vi.fn();
    mount(<UrlHarness code={null} onNavigate={nav} />);
    window.history.replaceState(null, "", "/nhiem-vu?status=moi-giao");
    act(() => {
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    expect(nav).toHaveBeenLastCalledWith(null);
    window.history.replaceState(null, "", "/nhiem-vu?task=NV21");
    act(() => {
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    expect(nav).toHaveBeenLastCalledWith("NV21");
  });
});

describe("LargeDialog — Esc asks, focus returns to the opener", () => {
  function Opener({ dismiss }: { dismiss: () => void }) {
    const [open, setOpen] = useState(false);
    return (
      <>
        <button type="button" onClick={() => setOpen(true)}>
          Mở NV19
        </button>
        <button type="button" onClick={() => setOpen(false)}>
          đóng ngoài
        </button>
        {open && (
          <LargeDialog titleId="t" onDismiss={dismiss}>
            <h2 id="t">[NV19]</h2>
            <button type="button">bên trong</button>
          </LargeDialog>
        )}
      </>
    );
  }

  it("Esc calls `onDismiss` and does not close by itself; unmounting gives focus back", () => {
    const dismiss = vi.fn();
    mount(<Opener dismiss={dismiss} />);
    const [opener, outside] = Array.from(host!.querySelectorAll("button"));
    opener!.focus();
    act(() => opener!.click());
    const dialog = host!.querySelector("dialog")!;
    expect(dialog.getAttribute("aria-labelledby")).toBe("t");
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    const cancel = new Event("cancel", { cancelable: true });
    act(() => {
      dialog.dispatchEvent(cancel);
    });
    expect(dismiss).toHaveBeenCalledTimes(1);
    expect(cancel.defaultPrevented).toBe(true);
    host!.querySelector<HTMLButtonElement>("dialog button")!.focus();
    act(() => outside!.click());
    expect(host!.querySelector("dialog")).toBeNull();
    expect(document.activeElement).toBe(opener);
  });
});

/** The register's routes, answered from one fixture task. Anything else: an empty page. */
function stubServer(t: petitions_nhiemVuRa): void {
  const json = (body: unknown) =>
    new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      const url = new URL(path, "http://localhost");
      if (url.pathname === `/api/v1/tasks/${t.code}`) return json({ ...t, documents: [] });
      if (url.pathname === "/api/v1/tasks" && !url.searchParams.has("parent")) {
        const status = url.searchParams.get("status");
        return json({ items: status === null || status === t.status ? [t] : [], next_cursor: "", has_more: false });
      }
      if (url.pathname === "/api/v1/task-counts") return json({ by_status: [] });
      return json({ items: [], next_cursor: "", has_more: false });
    }),
  );
}

function buttonByText(text: string): HTMLButtonElement {
  // `Mở NV19` = the Kanban card's open button (the whole card body since 06/10/2026).
  const card = /^Mở (\S+)$/.exec(text);
  if (card !== null) {
    const open = document.querySelector<HTMLButtonElement>(
      `article[aria-labelledby="the-nhiem-vu-${card[1]}"] button[aria-expanded]:not([aria-haspopup])`,
    );
    if (open === null) throw new Error(`no card "${card[1]}"`);
    return open;
  }
  const b = Array.from(document.querySelectorAll("button")).find((x) => x.textContent?.trim() === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

describe("the register opens the detail as a dialog", () => {
  it("a click on the card opens the dialog, pushes `?task=`; ✕ goes Back and focus returns", async () => {
    stubServer(task());
    const push = vi.spyOn(window.history, "pushState");
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    mount(<SoNhiemVu />);
    await settle();

    const open = buttonByText("Mở NV19");
    open.focus();
    await act(async () => open.click());
    await settle();

    const dialog = document.querySelector("dialog");
    expect(dialog).not.toBeNull();
    // Spec 07 §1: the code chip and the title share the header; the title names the dialog.
    const title = document.getElementById(dialog!.getAttribute("aria-labelledby")!)!;
    expect(title.textContent).toBe("Rà soát danh sách hộ nghèo quý III");
    expect(title.closest("header")?.textContent).toContain("NV19");
    expect(push).toHaveBeenCalledTimes(1);
    expect(window.location.search).toBe("?task=NV19");

    const close = document.querySelector<HTMLButtonElement>('dialog header button[aria-label="Đóng"]')!;
    await act(async () => close.click());
    expect(document.querySelector("dialog")).toBeNull();
    expect(back).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(open);
  });

  it("`?task=NV19` on load opens the dialog without pushing a second entry", async () => {
    stubServer(task());
    window.history.replaceState(null, "", "/nhiem-vu?task=NV19");
    const push = vi.spyOn(window.history, "pushState");
    mount(<SoNhiemVu openTask="NV19" />);
    await settle();
    expect(document.querySelector("dialog")).not.toBeNull();
    expect(push).not.toHaveBeenCalled();
  });

  it("Back onto the list closes the dialog", async () => {
    stubServer(task());
    mount(<SoNhiemVu />);
    await settle();
    await act(async () => buttonByText("Mở NV19").click());
    await settle();
    expect(document.querySelector("dialog")).not.toBeNull();
    window.history.replaceState(null, "", "/nhiem-vu");
    await act(async () => {
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    expect(document.querySelector("dialog")).toBeNull();
  });
});

describe("the detail — one view (user decision 07/10/2026): no action tabs", () => {
  const ALL = quyenNhiemVu(ALL_KEYS);

  it("no `Xem chi tiết` / `Chỉnh sửa` / `Xoá` tabs, no panels — every block drawn once", () => {
    const html = detailHtml(ALL);
    expect(html).not.toContain('role="tablist"');
    expect(html).not.toContain('role="tabpanel"');
    expect(html).not.toContain("task-detail-tab-");
    // One title field id at most — the inline form is closed, so none.
    expect(html).not.toContain('id="sua-tieu-de"');
  });

  it("header (spec 07 §1): code chip + title, then `bộ phận · người thực hiện`, [Xoá], ✕ — no `Tạo bởi` line", () => {
    const html = detailHtml(ALL);
    expect(html).toMatch(
      /<span class="border-line text-ink-muted mt-0.5 shrink-0 rounded border bg-\[#F7FAFC\] px-1.5 py-0.5 text-\[10.5px\] font-semibold">NV19<\/span><h2 id="tieu-de-chi-tiet-nhiem-vu" class="text-navy m-0 min-w-0 text-\[16px\] leading-snug font-bold[^"]*">Rà soát danh sách hộ nghèo quý III<\/h2>/,
    );
    expect(html).toContain(`${O_TRONG} · ${CHUA_PHAN_CONG}</p>`);
    expect(html).not.toContain("Tạo bởi");
    // `border-b` alone — never with `border-solid` (preflight off: the other three sides go 3px).
    expect(html).toContain('<header class="border-line flex shrink-0 items-start gap-3 border-b px-5 py-4">');
    expect(html).not.toMatch(/border-b border-solid/);
    const header = html.slice(html.indexOf("<header"), html.indexOf("</header>"));
    expect(header.indexOf(`aria-label="${TASK_DELETE_BUTTON}"`)).toBeLessThan(header.indexOf('aria-label="Đóng"'));
  });

  it("NO right icon rail (spec 07, owner 07/10/2026 #7): no copy-link / source / attach shortcuts; Xoá is in the header", () => {
    for (const html of [detailHtml(ALL), detailHtml(ALL, { type: "theo-van-ban" })]) {
      expect(html).not.toContain('aria-label="Thao tác phụ"');
      expect(html).not.toContain('aria-label="Sao chép liên kết"');
      expect(html).not.toContain('aria-label="Đính kèm vào nhật ký"');
      expect(html).not.toContain('aria-label="Xem văn bản chỉ đạo"');
      const header = html.slice(html.indexOf("<header"), html.indexOf("</header>"));
      expect(header).toContain(`aria-label="${TASK_DELETE_BUTTON}"`);
    }
  });
});

describe("status pipeline — a chip is a button ONLY for a move this account may take", () => {
  const ALL = quyenNhiemVu(ALL_KEYS);
  const READ_ONLY = quyenNhiemVu(["task.read"]);

  function stepButtons(html: string): string[] {
    const steps = html.slice(html.indexOf('<ol tabindex="-1" aria-label="Các bước'), html.indexOf("</ol>"));
    return [...steps.matchAll(/<button[^>]*aria-label="([^"]+)"/g)].map((m) => m[1]!);
  }

  it("ALLOWED: `dang-thuc-hien` — exactly the server's plain moves on the flow are pressable; the rest are disabled", () => {
    const html = detailHtml(ALL, { status: "dang-thuc-hien" });
    // Spec 07 §2: `Chờ duyệt` is drawn only while current — the move into it stays on the Kanban.
    expect(stepButtons(html)).toEqual([statusMoveName("Hoàn thành")]);
    expect(html).toContain('aria-current="step" data-step="current">Đang thực hiện</span>');
    // The current step is filled with its status colour (spec 07 §2), a disabled button.
    expect(html).toMatch(/<button type="button" disabled="" title="Trạng thái hiện tại" class="[^"]*border-transparent text-white bg-brand"/);
    // Not listed by the server from here ⇒ a DISABLED step with the spec's title, never pressable.
    expect(stepButtons(html).join(" ")).not.toContain("Mới giao");
    expect(html).toMatch(/<button type="button" disabled="" title="Không chuyển thẳng sang bước này được" class="[^"]*border-line\/60 text-ink-muted\/60/);
    // The branch is offered only because it is a move (prototype: branch chips only when clickable).
    expect(html).toContain(`aria-label="${statusMoveName("Tạm dừng")}"`);
    // The old row of `Chuyển sang …` buttons is gone.
    expect(html).not.toContain(">Chuyển trạng thái</h4>");
  });

  it("`Chờ duyệt` sits on the flow only when current (spec 07 §2); `Tạm dừng` only when current or a move", () => {
    const fresh = detailHtml(ALL, { status: "moi-giao" });
    const steps = fresh.slice(fresh.indexOf("<ol"), fresh.indexOf("</ol>"));
    expect(steps).not.toContain("Chờ duyệt");
    const working = detailHtml(ALL, { status: "dang-thuc-hien" });
    expect(working.slice(working.indexOf("<ol"), working.indexOf("</ol>"))).not.toContain("Chờ duyệt");
    const waiting = detailHtml(ALL, { status: "cho-duyet" });
    expect(waiting.slice(waiting.indexOf("<ol"), waiting.indexOf("</ol>"))).toContain('data-step="current">Chờ duyệt</span>');
    const done = detailHtml(ALL, { status: "hoan-thanh" });
    expect(done).not.toContain(">Tạm dừng</span>");
  });

  it("DENIED: without `task.update` and not the assignee — no chip is a button, and the sentence says why", () => {
    const html = detailHtml(READ_ONLY, { status: "dang-thuc-hien" });
    expect(stepButtons(html)).toEqual([]);
    expect(html).not.toContain("Chuyển sang");
    expect(html).toContain(STATUS_MOVE_DENIED);
  });

  it("a plain move: the press opens the compose box; a refusal is the error toast, the box and note kept; success clears it", async () => {
    stubQuietServer();
    const move = vi
      .fn<(target: string, note?: string) => Promise<KetQua<petitions_nhiemVuRa>>>()
      .mockResolvedValueOnce({ ok: false, thongBao: "Máy chủ từ chối bước này." })
      .mockResolvedValueOnce({ ok: true, duLieu: task({ status: "hoan-thanh" }) });
    mount(detail(ALL, { status: "dang-thuc-hien" }, { move }));
    await settle();

    await act(async () => byLabel(statusMoveName("Hoàn thành")).click());
    const box = document.getElementById(STATUS_COMPOSE_ID)!;
    expect(box).not.toBeNull();
    const note = box.querySelector<HTMLTextAreaElement>("textarea")!;
    expect(box.querySelector(`label[for="${note.id}"]`)?.textContent).toBe(STATUS_NOTE_LABEL);
    expect(note.required).toBe(false);
    expect(document.activeElement).toBe(note);

    typeInto(note, "  Đã gửi bản dự thảo  ");
    await act(async () => (box as HTMLFormElement).requestSubmit());
    await settle();
    // No hand-over chosen and no file: the extras carry nothing (the client then sends neither).
    expect(move).toHaveBeenLastCalledWith("hoan-thanh", "Đã gửi bản dự thảo", { handover: undefined, attachments: [] });
    expect(toast.error).toHaveBeenCalledWith("Máy chủ từ chối bước này.");
    expect(document.getElementById(STATUS_COMPOSE_ID)?.querySelector("textarea")?.value).toBe("  Đã gửi bản dự thảo  ");

    await act(async () => (document.getElementById(STATUS_COMPOSE_ID) as HTMLFormElement).requestSubmit());
    await settle();
    expect(move).toHaveBeenCalledTimes(2);
    expect(document.getElementById(STATUS_COMPOSE_ID)).toBeNull();
    expect(toast.success).toHaveBeenCalledWith(statusMoveDoneText("Hoàn thành"));
  });

  it("return for rework: the reason is MANDATORY — `Xác nhận` stays disabled on blank, the note is trimmed", async () => {
    stubQuietServer();
    const move = vi.fn<(target: string, note?: string) => Promise<KetQua<petitions_nhiemVuRa>>>(NOT_SENT);
    mount(detail(ALL, { status: "cho-duyet" }, { move }));
    await settle();

    await act(async () => byLabel(reasonMoveName("return", "Đang thực hiện")).click());
    const reason = document.querySelector<HTMLTextAreaElement>("#ly-do-tra-lai")!;
    expect(document.querySelector('label[for="ly-do-tra-lai"]')?.textContent).toBe(NHAN_LY_DO_TRA_LAI);
    expect(reason.required).toBe(true);
    const confirm = buttonByText(STATUS_CONFIRM_BUTTON);
    expect(confirm.disabled).toBe(true);
    typeInto(reason, "    ");
    expect(confirm.disabled).toBe(true);
    // A submit forced on a blank reason sends nothing.
    await act(async () => (document.getElementById(STATUS_COMPOSE_ID) as HTMLFormElement).requestSubmit());
    expect(move).not.toHaveBeenCalled();

    typeInto(reason, "  Thiếu số liệu thôn 3  ");
    expect(confirm.disabled).toBe(false);
    await act(async () => confirm.click());
    await settle();
    expect(move).toHaveBeenCalledWith("dang-thuc-hien", "Thiếu số liệu thôn 3", { handover: undefined, attachments: [] });
  });

  it("reopen: same mandatory reason, its own label and note", async () => {
    stubQuietServer();
    mount(detail(ALL, { status: "hoan-thanh", completed_at: "2026-06-25T02:00:00Z" }));
    await settle();
    await act(async () => byLabel(reasonMoveName("reopen", "Đang thực hiện")).click());
    expect(document.querySelector('label[for="ly-do-mo-lai"]')?.textContent).toBe(REOPEN_REASON_LABEL);
    expect(document.querySelector<HTMLTextAreaElement>("#ly-do-mo-lai")!.required).toBe(true);
    expect(document.getElementById(STATUS_COMPOSE_ID)?.textContent).toContain(reopenNote(BANG_NHAN_MAC_DINH));
    expect(buttonByText(STATUS_CONFIRM_BUTTON).disabled).toBe(true);
  });
});

describe("information block — inline `✎ Sửa` (task.update)", () => {
  const ALL = quyenNhiemVu(ALL_KEYS);
  const READ_ONLY = quyenNhiemVu(["task.read"]);

  it("DENIED: without `task.update` there is no `✎ Sửa`", () => {
    expect(detailHtml(READ_ONLY)).not.toContain(`aria-label="${TASK_INFO_EDIT_LABEL}"`);
    expect(detailHtml(ALL)).toContain(`aria-label="${TASK_INFO_EDIT_LABEL}"`);
  });

  it("co-ban: the form edits the title (and deadline); PATCH carries the title and the lock, NEVER `code`", async () => {
    stubQuietServer();
    const save = vi
      .fn<(body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>>()
      .mockResolvedValue({ ok: true, duLieu: task({ title: "Rà soát hộ nghèo quý IV" }) });
    mount(detail(ALL, { type: "co-ban" }, { save }));
    await settle();

    await act(async () => byLabel(TASK_INFO_EDIT_LABEL).click());
    await settle();
    const title = document.querySelector<HTMLInputElement>("#sua-tieu-de")!;
    expect(title.value).toBe("Rà soát danh sách hộ nghèo quý III");
    // The code is shown as text, never an input (ADR 0065 NV3).
    expect(Array.from(document.querySelectorAll("input")).some((i) => i.value === "NV19")).toBe(false);
    // One title field in the page — the read view is replaced, not duplicated.
    expect(document.querySelectorAll("#sua-tieu-de")).toHaveLength(1);

    typeInto(title, "Rà soát hộ nghèo quý IV");
    await act(async () => title.form!.requestSubmit());
    await settle();
    expect(save).toHaveBeenCalledTimes(1);
    const body = save.mock.calls[0]![0];
    expect(body).toEqual({ title: "Rà soát hộ nghèo quý IV", expected_updated_at: "2026-06-01T02:00:00Z" });
    expect(body).not.toHaveProperty("code");
    // Saved: back to the read view, focus on `✎ Sửa`.
    expect(document.querySelector("#sua-tieu-de")).toBeNull();
    expect(document.activeElement).toBe(byLabel(TASK_INFO_EDIT_LABEL));
  });

  it("a refusal (409) is the error toast, verbatim; the form stays with the typed title kept", async () => {
    stubQuietServer();
    const refusal = "Nhiệm vụ vừa được người khác sửa — mở lại để xem bản mới.";
    const save = vi
      .fn<(body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>>()
      .mockResolvedValue({ ok: false, thongBao: refusal });
    mount(detail(ALL, { type: "co-ban" }, { save }));
    await settle();
    await act(async () => byLabel(TASK_INFO_EDIT_LABEL).click());
    await settle();
    const title = document.querySelector<HTMLInputElement>("#sua-tieu-de")!;
    typeInto(title, "Tên mới");
    await act(async () => title.form!.requestSubmit());
    await settle();
    expect(toast.error).toHaveBeenCalledWith(refusal);
    expect(document.querySelector<HTMLInputElement>("#sua-tieu-de")!.value).toBe("Tên mới");
  });
});

describe("delete — the header's button behind `task.delete`, the existing confirm with a mandatory reason", () => {
  const ALL = quyenNhiemVu(ALL_KEYS);

  it("DENIED: without `task.delete` there is no delete button and no delete form", () => {
    const html = detailHtml(quyenNhiemVu(ALL_KEYS.filter((k) => k !== QUYEN_XOA_NHIEM_VU)));
    expect(html).not.toContain(`aria-label="${TASK_DELETE_BUTTON}"`);
    expect(html).not.toContain('id="ly-do-xoa-nhiem-vu"');
  });

  it("ALLOWED: the header button opens the confirm; `Xoá` disabled until a reason; a refusal shows in the dialog", async () => {
    stubQuietServer();
    const remove = vi
      .fn<(reason: string) => Promise<KetQua<unknown>>>()
      .mockResolvedValue({ ok: false, thongBao: "còn 2 việc con chưa xoá — xử lý hoặc xoá các việc con trước" });
    mount(detail(ALL, {}, { remove }));
    await settle();
    expect(document.querySelector("#ly-do-xoa-nhiem-vu")).toBeNull();

    await act(async () => byLabel(TASK_DELETE_BUTTON).click());
    const dialog = document.querySelector("dialog")!;
    expect(dialog).not.toBeNull();
    expect(document.getElementById(dialog.getAttribute("aria-labelledby")!)?.textContent).toBe(
      "Xoá nhiệm vụ NV19 khỏi sổ?",
    );
    const reason = dialog.querySelector<HTMLInputElement>("#ly-do-xoa-nhiem-vu")!;
    const confirm = dialog.querySelector<HTMLButtonElement>('button[type="submit"]')!;
    expect(confirm.disabled).toBe(true);
    typeInto(reason, "   ");
    expect(confirm.disabled).toBe(true);
    typeInto(reason, "  Nhập trùng  ");
    expect(confirm.disabled).toBe(false);
    await act(async () => confirm.click());
    await settle();
    expect(remove).toHaveBeenCalledWith("Nhập trùng");
    expect(dialog.querySelector('[role="alert"]')?.textContent).toContain("còn 2 việc con chưa xoá");
  });
});

/** Every read the detail makes on mount answers an empty page — enough for its blocks to settle. */
function stubQuietServer(): void {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        new Response(JSON.stringify({ items: [], next_cursor: "", has_more: false }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    ),
  );
}

function byLabel(label: string): HTMLButtonElement {
  const b = document.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
  if (b === null) throw new Error(`no button labelled "${label}"`);
  return b;
}

/** A React-controlled field: set through the native setter, then the `input` event React listens to. */
function typeInto(el: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
  act(() => {
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

describe("compose box hand-over and files (prototype `TaskStatusPipeline.tsx:261-355`, `…/status` cebe0d47)", () => {
  const UNITS = [
    { id: "01JA", code: "a", name: "Văn phòng", parent_id: "", order: 0, staff_count: 1 },
    { id: "01JB", code: "b", name: "Địa chính", parent_id: "", order: 1, staff_count: 1 },
  ];
  const DIRECTORY = {
    ok: true as const,
    duLieu: { items: [{ code: "CB-2026-0P4X1Z", full_name: "Nguyễn Thị Thực", position: "", department_id: "01JB" }] },
  };

  function pipeline(staffCode: string, move: (t: string, n?: string, e?: unknown) => Promise<KetQua<petitions_nhiemVuRa>>) {
    return (
      <TaskStatusPipeline
        task={task({ status: "dang-thuc-hien", unit: "01JA", assignee: "CB-2026-7K3M9Q" })}
        labels={BANG_NHAN_MAC_DINH}
        permissions={quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU])}
        staffCode={staffCode}
        showAssignment={false}
        sending={false}
        move={move}
        now={new Date("2026-09-15T03:00:00Z")}
        units={UNITS}
        directory={DIRECTORY}
      />
    );
  }

  it("DENIED — neither `task.assign` nor the assignee: no hand-over fields in the box", async () => {
    stubQuietServer();
    mount(pipeline("CB-2026-NGUOIKHAC", NOT_SENT));
    await act(async () => byLabel(statusMoveName("Hoàn thành")).click());
    expect(document.getElementById(`${STATUS_COMPOSE_ID}-bo-phan`)).toBeNull();
    expect(document.getElementById(STATUS_COMPOSE_ID)?.textContent).toContain(STATUS_ATTACH_BUTTON);
  });

  it("ALLOWED — the assignee: a new unit relabels `Xác nhận` and is SENT as `handover`", async () => {
    stubQuietServer();
    const move = vi.fn(async () => ({ ok: true as const, duLieu: task({ status: "hoan-thanh" }) }));
    mount(pipeline("CB-2026-7K3M9Q", move));
    await act(async () => byLabel(statusMoveName("Hoàn thành")).click());
    const unit = document.querySelector<HTMLSelectElement>(`#${STATUS_COMPOSE_ID}-bo-phan`)!;
    expect(unit.value).toBe("01JA");
    act(() => {
      unit.value = "01JB";
      unit.dispatchEvent(new Event("change", { bubbles: true }));
    });
    const submit = document.querySelector<HTMLButtonElement>(`#${STATUS_COMPOSE_ID} button[type="submit"]`)!;
    expect(submit.textContent).toBe(STATUS_CONFIRM_HANDOVER);
    await act(async () => (document.getElementById(STATUS_COMPOSE_ID) as HTMLFormElement).requestSubmit());
    await settle();
    expect(move).toHaveBeenCalledWith("hoan-thanh", "", {
      handover: { unit: "01JB", assignee: undefined },
      attachments: [],
    });
  });
});
