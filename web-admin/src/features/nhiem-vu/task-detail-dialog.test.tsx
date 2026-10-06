// @vitest-environment jsdom
//
// jsdom for this file: what it pins — a click opening the dialog, the history entry it pushes, Back
// closing it, focus going back to the row — are events, history and focus, none of which a string
// render can show. Same reasoning as `features/noi-dung/content-delete-dialog.test.tsx`.

import { act, useState, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { LargeDialog } from "@/components/ui/large-dialog";
import type { KetQua } from "@/lib/api/goi";
import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_TAO_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
} from "@/lib/quyen";

import { BANG_NHAN_MAC_DINH, quyenNhiemVu } from "./nhan-nhiem-vu";
import { ChiTietNhiemVu, SoNhiemVu, type TaskDetailTab } from "./so-nhiem-vu";
import { readTaskParam, searchWithTask, useTaskDialogUrl } from "./task-dialog-url";
import { serverTransitions } from "./task-transitions.fixture";

type Permissions = ReturnType<typeof quyenNhiemVu>;

const ALL_KEYS = [
  QUYEN_TAO_NHIEM_VU,
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
];

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

function detailHtml(permissions: Permissions, patch: Partial<petitions_nhiemVuRa> = {}, tab?: TaskDetailTab) {
  return renderToStaticMarkup(
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
      loiGhi={null}
      dong={() => {}}
      doiTrangThai={() => {}}
      xoa={() => {}}
      guiDeNghiLuiHan={NOT_CALLED}
      quyetDinh={NOT_CALLED}
      suaKhoiVanBan={NOT_CALLED}
      docLaiChiTiet={NOT_CALLED}
      extensionRefreshKey="0"
      onExtensionDecided={() => {}}
      openTask={() => {}}
      openTaskByCode={NOT_CALLED}
      saveParent={NOT_CALLED}
      addChild={null}
      reassign={NOT_CALLED}
      initialTab={tab}
    />,
  );
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
    expect(document.getElementById(dialog!.getAttribute("aria-labelledby")!)?.textContent).toContain("[NV19]");
    expect(push).toHaveBeenCalledTimes(1);
    expect(window.location.search).toBe("?task=NV19");

    const close = document.querySelector<HTMLButtonElement>('dialog button[aria-label="Đóng chi tiết nhiệm vụ"]')!;
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

describe("the dialog's tabs, chips and rail", () => {
  const ALL = quyenNhiemVu(ALL_KEYS);
  const READ_ONLY = quyenNhiemVu(["task.read"]);

  it("ALLOWED: `task.delete` ⇒ a `Xoá` tab and the soft-delete form with its mandatory reason", () => {
    const html = detailHtml(ALL, {}, "delete");
    expect(html).toContain('id="task-detail-tab-delete"');
    expect(html).toContain('id="ly-do-xoa-nhiem-vu"');
    expect(html).toMatch(/id="task-detail-panel-delete" role="tabpanel" aria-labelledby="task-detail-tab-delete">/);
  });

  it("DENIED: without `task.delete` there is no `Xoá` tab and no delete form", () => {
    const html = detailHtml(quyenNhiemVu(ALL_KEYS.filter((k) => k !== QUYEN_XOA_NHIEM_VU)));
    expect(html).not.toContain('id="task-detail-tab-delete"');
    expect(html).not.toContain('id="ly-do-xoa-nhiem-vu"');
  });

  it("DENIED: without `task.update` there is no `Chỉnh sửa` tab and no edit block", () => {
    const html = detailHtml(READ_ONLY);
    expect(html).not.toContain('id="task-detail-tab-edit"');
    expect(html).not.toContain('id="task-detail-panel-edit"');
    // `Xem chi tiết` is always there, and selected.
    const viewTab = html.match(/<button[^>]*id="task-detail-tab-view"[^>]*>/)?.[0] ?? "";
    expect(viewTab).toContain('aria-selected="true"');
  });

  it("only the selected panel is shown; the others are mounted but `hidden`", () => {
    const html = detailHtml(ALL);
    expect(html).toMatch(/id="task-detail-panel-view" role="tabpanel" aria-labelledby="task-detail-tab-view" class=/);
    expect(html).toMatch(/id="task-detail-panel-edit"[^>]*hidden=""/);
    expect(html).toMatch(/id="task-detail-panel-delete"[^>]*hidden=""/);
  });

  it("step chips are text, never buttons — no move outside `allowed_transitions`", () => {
    const html = detailHtml(ALL, { status: "dang-thuc-hien" });
    const steps = html.slice(html.indexOf('<ol aria-label="Các bước'), html.indexOf("</ol>"));
    expect(steps).toContain('aria-current="step" data-step="current">Đang thực hiện</span>');
    expect(steps).not.toContain("<button");
    // The moves stay the server's list, as buttons with words.
    expect(html).toContain("Chuyển sang Chờ duyệt");
  });

  it("every rail button has an accessible name; the link and attach buttons follow the task", () => {
    const html = detailHtml(ALL);
    const rail = html.slice(html.indexOf('aria-label="Thao tác phụ"'));
    expect(rail).toContain('aria-label="Sao chép liên kết"');
    // The assigner writes in the log (`canWriteLogEntry`), so the attach shortcut is offered.
    expect(rail).toContain('aria-label="Đính kèm vào nhật ký"');
    // A basic task has no document block and no source minutes.
    expect(rail).not.toContain('aria-label="Xem văn bản chỉ đạo"');
    expect(rail).not.toContain('aria-label="Mở biên bản nguồn"');
    expect(rail.match(/<button[^>]*>/g)?.every((b) => b.includes("aria-label="))).toBe(true);
    const withDocs = detailHtml(ALL, { type: "theo-van-ban" });
    expect(withDocs).toContain('aria-label="Xem văn bản chỉ đạo"');
  });

  it("the header names who created the task and when", () => {
    const html = detailHtml(ALL);
    expect(html).toContain("Tạo bởi CB-2026-VANTHU");
  });
});
