// @vitest-environment jsdom
//
// jsdom for this file: `?create=1` is an effect on mount and a `history.replaceState` — neither shows in
// a string render. `/nhiem-vu` has its own file (`features/nhiem-vu/open-create.test.tsx`); the page
// wiring of `/nguoi-dung` and `/mini-app` is pinned in their page tests.
//
// Each screen is mounted for real, every read answering an empty page, so the only dialog that can
// appear is the create dialog. Per screen: ALLOWED opens it and drops `create`; DENIED (no key) opens
// nothing; no parameter opens nothing.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { DanhBaCanBo } from "@/features/cau-hinh/danh-ba-can-bo";
import { SoBienBan } from "@/features/bien-ban/so-bien-ban";
import { CitizenDirectoryWorkspace } from "@/features/citizen-directory/citizen-directory-workspace";
import { KhoiThemDuAn } from "@/features/giai-ngan/ghi-du-an";
import { SoNoiDung } from "@/features/noi-dung/so-noi-dung";
import { CONTENT_UPDATE_PERMISSION } from "@/features/noi-dung/nhan-noi-dung";
import { PETITION_INTAKE_PERMISSION } from "@/features/phan-anh/nhan-phieu";
import { StaffIntakeButton } from "@/features/phan-anh/staff-intake";
import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import { SoThongBao } from "@/features/thong-bao/so-thong-bao";
import { DocumentWorkspace } from "@/features/van-ban/document-workspace";
import { parseDrillDown } from "@/lib/drill-down";
import { PETITION_CREATE_PERMISSION, QUYEN_XEM_NOI_DUNG } from "@/lib/quyen";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

let fakeSession: PhienDaDoc = null;
vi.mock("@/features/phien/phien-hien-tai", async (original) => ({
  ...(await original<typeof import("@/features/phien/phien-hien-tai")>()),
  usePhien: () => fakeSession,
}));

function sessionWith(permissions: readonly string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { code: "CB-2026-7K3M9Q" }, permissions: [...permissions] } } as unknown as PhienDaDoc;
}

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

async function mount(node: ReactNode): Promise<void> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() => root!.render(node));
  for (let i = 0; i < 12; i++) await act(async () => {});
}

function dialogOpen(): boolean {
  return document.querySelector("dialog") !== null;
}

/** Arrive on `path`, mount `node`, and report what the link did. */
async function arrive(path: string, node: ReactNode): Promise<{ opened: boolean; search: string }> {
  window.history.replaceState(null, "", path);
  await mount(node);
  return { opened: dialogOpen(), search: window.location.search };
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        // `count`: the count routes (`…/count-queries`, `citizen-letters` count) read it off the same body.
        new Response(JSON.stringify({ items: [], next_cursor: "", has_more: false, count: 0 }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    ),
  );
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  fakeSession = null;
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});

describe("/van-ban — `Vào sổ đơn thư` on the opening (petition) tab, under `petition.create`", () => {
  it("ALLOWED: opens, `create` dropped", async () => {
    fakeSession = sessionWith([PETITION_CREATE_PERMISSION]);
    expect(await arrive("/van-ban?create=1", <DocumentWorkspace openCreateOnLoad />)).toEqual({ opened: true, search: "" });
  });

  it("DENIED: without `petition.create` nothing opens", async () => {
    fakeSession = sessionWith(["petition.read"]);
    expect((await arrive("/van-ban?create=1", <DocumentWorkspace openCreateOnLoad />)).opened).toBe(false);
  });

  it("no parameter: nothing opens", async () => {
    fakeSession = sessionWith([PETITION_CREATE_PERMISSION]);
    expect((await arrive("/van-ban", <DocumentWorkspace />)).opened).toBe(false);
  });

  it("an Overview drill-down opens Văn bản đến: the petition dialog is not opened behind it", async () => {
    fakeSession = sessionWith([PETITION_CREATE_PERMISSION]);
    const drillDown = parseDrillDown("incoming-documents", { metric: "overdue" });
    const r = await arrive("/van-ban?metric=overdue&create=1", <DocumentWorkspace drillDown={drillDown} openCreateOnLoad />);
    expect(r.opened).toBe(false);
  });
});

describe("/thong-bao — `Soạn thông báo` (the button has no client gate, so neither has the link)", () => {
  it("opens, `create` dropped", async () => {
    fakeSession = sessionWith([]);
    expect(await arrive("/thong-bao?create=1", <SoThongBao openCreateOnLoad />)).toEqual({ opened: true, search: "" });
  });

  it("no parameter, or a value other than `1` (server-read prop false): nothing opens", async () => {
    fakeSession = sessionWith([]);
    expect((await arrive("/thong-bao", <SoThongBao />)).opened).toBe(false);
  });
});

describe("/nguoi-dan — `Thêm người dân`, under `citizen.update` (`canEdit`)", () => {
  it("ALLOWED: opens, `create` dropped", async () => {
    expect(await arrive("/nguoi-dan?create=1", <CitizenDirectoryWorkspace canEdit openCreateOnLoad />)).toEqual({
      opened: true,
      search: "",
    });
  });

  it("DENIED: `canEdit` false — nothing opens", async () => {
    expect((await arrive("/nguoi-dan?create=1", <CitizenDirectoryWorkspace canEdit={false} openCreateOnLoad />)).opened).toBe(false);
  });

  it("no parameter: nothing opens", async () => {
    expect((await arrive("/nguoi-dan", <CitizenDirectoryWorkspace canEdit />)).opened).toBe(false);
  });
});

describe("/nguoi-dung — `Thêm cán bộ` (the list mounts only behind `admin.user`: `user-pages.test.tsx`)", () => {
  it("opens, `create` dropped", async () => {
    expect(await arrive("/nguoi-dung?create=1", <DanhBaCanBo openCreateOnLoad />)).toEqual({ opened: true, search: "" });
  });

  it("no parameter: nothing opens", async () => {
    expect((await arrive("/nguoi-dung", <DanhBaCanBo />)).opened).toBe(false);
  });
});

describe("/giai-ngan — `Thêm dự án`, under `budget.update` (`coGhi`)", () => {
  const draw = (coGhi: boolean, openCreateOnLoad: boolean) => (
    <KhoiThemDuAn nam={2026} danhMuc={[]} coGhi={coGhi} daGhiXong={() => {}} openCreateOnLoad={openCreateOnLoad} />
  );

  it("ALLOWED: opens, `create` dropped", async () => {
    expect(await arrive("/giai-ngan?create=1", draw(true, true))).toEqual({ opened: true, search: "" });
  });

  it("DENIED: `coGhi` false — nothing opens", async () => {
    expect((await arrive("/giai-ngan?create=1", draw(false, true))).opened).toBe(false);
  });

  it("no parameter: nothing opens", async () => {
    expect((await arrive("/giai-ngan", draw(true, false))).opened).toBe(false);
  });
});

describe("/nhiem-vu/bien-ban — `Nhập biên bản` (the button has no client gate, so neither has the link)", () => {
  it("opens, `create` dropped, the `#…` anchor kept", async () => {
    fakeSession = sessionWith([]);
    window.history.replaceState(null, "", "/nhiem-vu/bien-ban?create=1#x");
    await mount(<SoBienBan openCreateOnLoad />);
    expect(dialogOpen()).toBe(true);
    expect(window.location.search).toBe("");
    expect(window.location.hash).toBe("#x");
  });

  it("no parameter: nothing opens", async () => {
    fakeSession = sessionWith([]);
    expect((await arrive("/nhiem-vu/bien-ban", <SoBienBan />)).opened).toBe(false);
  });
});

describe("/phan-anh — `Nhập hộ phản ánh`, under `feedback.create`", () => {
  const draw = (permissions: readonly string[], openCreateOnLoad: boolean) => (
    <StaffIntakeButton permissions={permissions} onBooked={() => {}} openCreateOnLoad={openCreateOnLoad} />
  );

  it("ALLOWED: opens; `create` dropped, `?id=` kept", async () => {
    expect(await arrive("/phan-anh?id=PA-7K3M9Q2X&create=1", draw([PETITION_INTAKE_PERMISSION], true))).toEqual({
      opened: true,
      search: "?id=PA-7K3M9Q2X",
    });
  });

  it("DENIED: without `feedback.create` nothing opens", async () => {
    expect((await arrive("/phan-anh?create=1", draw(["feedback.read"], true))).opened).toBe(false);
  });

  it("no parameter: nothing opens", async () => {
    expect((await arrive("/phan-anh", draw([PETITION_INTAKE_PERMISSION], false))).opened).toBe(false);
  });
});

describe("/mini-app (Nội dung) — `Thêm nội dung`, under `content.update`", () => {
  it("ALLOWED: opens; `create` dropped, `?tab=` kept", async () => {
    fakeSession = sessionWith([QUYEN_XEM_NOI_DUNG, CONTENT_UPDATE_PERMISSION]);
    expect(await arrive("/mini-app?tab=noi-dung&create=1", <SoNoiDung openCreateOnLoad />)).toEqual({
      opened: true,
      search: "?tab=noi-dung",
    });
  });

  it("DENIED: `content.read` alone — nothing opens", async () => {
    fakeSession = sessionWith([QUYEN_XEM_NOI_DUNG]);
    expect((await arrive("/mini-app?create=1", <SoNoiDung openCreateOnLoad />)).opened).toBe(false);
  });

  it("no parameter: nothing opens", async () => {
    fakeSession = sessionWith([QUYEN_XEM_NOI_DUNG, CONTENT_UPDATE_PERMISSION]);
    expect((await arrive("/mini-app", <SoNoiDung />)).opened).toBe(false);
  });
});
