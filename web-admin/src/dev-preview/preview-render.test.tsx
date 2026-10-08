// @vitest-environment jsdom
//
// jsdom: the preview must render the REAL Giải ngân components POPULATED from fixtures, open the dialog
// or tab its query names, and send nothing to the network.

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

const H = vi.hoisted(() => ({ path: "/xem-thu/giai-ngan", search: "" }));

vi.mock("next/navigation", () => ({
  usePathname: () => H.path,
  useRouter: () => ({ push: () => {}, replace: () => {}, refresh: () => {}, back: () => {}, prefetch: () => {} }),
  useSearchParams: () => new URLSearchParams(H.search),
}));
vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

/** Anything that reaches the REAL fetch is a request leaving the preview — the test fails on it. */
const network = vi.fn(() => Promise.reject(new Error("network")));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  for (const name of ["ResizeObserver", "IntersectionObserver"] as const) {
    if (!(name in globalThis)) {
      (globalThis as unknown as Record<string, unknown>)[name] = class {
        observe() {}
        unobserve() {}
        disconnect() {}
      };
    }
  }
  window.fetch = network as unknown as typeof window.fetch;
});

const { PreviewShell } = await import("./preview-shell");
const { Toaster } = await import("@/components/ui/toaster");
const { SIDEBAR_STORAGE_KEY } = await import("@/components/sidebar-state");
const { DisbursementPreview, ProjectDetailPreview } = await import("./disbursement-preview");
const { TaskPreview } = await import("./tasks-preview");
const { PREVIEW_TASK_PERMISSIONS } = await import("./shell.fixture");
const { DocumentsPreview } = await import("./documents-preview");
const { PREVIEW_DOCUMENT_PERMISSIONS } = await import("./shell.fixture");
const { PREVIEW_DOCUMENT_WITH_ROUTINGS, PREVIEW_DOCUMENT_NO_ROUTING, PREVIEW_REGISTER_ERROR, setDocumentPreviewState } =
  await import("./documents.fixture");
const { PREVIEW_LETTER_OVERDUE } = await import("./letters.fixture");
type PreviewPerson = import("./shell.fixture").PreviewPerson;

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  H.search = "";
});

async function mount(
  path: string,
  body: React.ReactNode,
  fullMenu = false,
  search = "",
  permissions?: readonly string[],
  person?: PreviewPerson,
): Promise<HTMLDivElement> {
  H.path = path;
  H.search = search;
  window.history.pushState({}, "", path);
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  // The root layout's one `Toaster`, a later sibling of the page — where the real app mounts it.
  await act(async () =>
    r.render(
      <>
        <PreviewShell fullMenu={fullMenu} permissions={permissions} person={person}>
          {body}
        </PreviewShell>
        <Toaster />
      </>,
    ),
  );
  return host;
}

/** Lets the fixture promises and the 100ms "press when ready" timer run. */
async function settle(ms = 400): Promise<void> {
  for (let i = 0; i < ms / 50; i++) await act(async () => new Promise((r) => setTimeout(r, 50)));
}

describe("preview — list screen", () => {
  it("the real shell (fixture commune, brand, Giải ngân lit) and the populated register, with no network", async () => {
    const el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />);
    await settle();
    expect(el.querySelector("header")!.textContent).toContain("Xã Thăng Bình");
    expect(el.querySelector(".side-nav-brand-name")!.textContent).toBe("ViGov");
    expect(el.querySelector('.side-nav a[aria-current="page"]')!.getAttribute("href")).toBe("/giai-ngan");
    expect(el.textContent).toContain("Theo dõi giải ngân");
    expect(el.textContent).toContain("Nhà văn hoá thôn Bình An");
    expect(el.textContent).toContain("Kè chống sạt lở bờ sông Ly Ly");
    expect(el.textContent).toContain("Ngân sách thành phố hỗ trợ");
    expect(el.querySelector(".header-user")!.textContent).toContain("Cán bộ A");
    expect(network).not.toHaveBeenCalled();
  });

  it("?modal=hang-muc opens the real category dialog over it; ?modal=them-du-an presses the real button", async () => {
    let el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal="hang-muc" />);
    await settle();
    const dialog = document.querySelector("dialog[open]")!;
    expect(dialog.textContent).toContain("Hạng mục kế hoạch vốn");
    // The four fixture categories, each in its rename field.
    expect([...dialog.querySelectorAll("input")].map((i) => i.value)).toEqual(
      expect.arrayContaining(["Công trình xây dựng mới", "Sửa chữa, cải tạo", "Hạ tầng số", "Môi trường"]),
    );
    act(() => root?.unmount());
    host?.remove();
    el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal="them-du-an" />);
    await settle(600);
    expect(el.querySelector("dialog[open]")).not.toBeNull();
    expect(network).not.toHaveBeenCalled();
  });

  it("?menu=day-du: the sidebar shows the full menu", async () => {
    const el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, true);
    await settle();
    const hrefs = [...el.querySelectorAll(".side-nav a")].map((a) => a.getAttribute("href"));
    expect(hrefs).toContain("/tong-quan");
    expect(hrefs).toContain("/cau-hinh");
  });
});

describe("preview — project detail", () => {
  it("the real detail page on the fixture project; ?tab=trao-doi opens the discussion with its two comments", async () => {
    const el = await mount("/xem-thu/giai-ngan/du-an", <ProjectDetailPreview tab="trao-doi" />);
    await settle(600);
    expect(el.textContent).toContain("Nhà văn hoá thôn Bình An");
    expect(el.querySelector("#tab-trao-doi-du-an")!.getAttribute("aria-selected")).toBe("true");
    expect(el.textContent).toContain("Đã liên hệ, đơn vị hẹn nộp trong tuần này.");
    expect(network).not.toHaveBeenCalled();
  });
});

describe("preview — Nhiệm vụ", () => {
  const tasks = (props: Partial<Parameters<typeof TaskPreview>[0]> = {}) => (
    <TaskPreview view="kanban" modal={null} select={false} openTask={null} {...props} />
  );
  const mountTasks = (body: React.ReactNode) =>
    mount("/xem-thu/nhiem-vu", body, false, "", PREVIEW_TASK_PERMISSIONS, "lanh-dao");

  it("the real board on the fixture: Kanban by default, Nhiệm vụ lit, the leader's session, no network", async () => {
    const el = await mountTasks(tasks());
    await settle(600);
    expect(el.querySelector(".side-nav-brand-name")!.textContent).toBe("ViGov");
    expect(el.querySelector('.side-nav a[aria-current="page"]')!.getAttribute("href")).toBe("/nhiem-vu");
    expect(el.querySelector(".header-user")!.textContent).toContain("Cán bộ C");
    expect(el.textContent).toContain("Quản lý nhiệm vụ");
    expect(el.textContent).toContain("Chuẩn bị hội trường tiếp xúc cử tri quý IV");
    expect(el.querySelector('[aria-label="Chế độ xem"] button[aria-pressed="true"]')!.textContent).toBe("Kanban");
    expect(network).not.toHaveBeenCalled();
  });

  it("?che-do=danh-sach and ?che-do=so-theo-doi press the real view switch", async () => {
    let el = await mountTasks(tasks({ view: "danh-sach" }));
    await settle(600);
    expect(el.querySelector('[aria-label="Chế độ xem"] button[aria-pressed="true"]')!.textContent).toBe("Danh sách");
    // A branch state has no Kanban column: seen only in the list.
    expect(el.textContent).toContain("Khảo sát nhu cầu lắp đặt camera an ninh tại các thôn");
    act(() => root?.unmount());
    host?.remove();
    el = await mountTasks(tasks({ view: "so-theo-doi" }));
    await settle(600);
    expect(el.querySelector('[aria-label="Chế độ xem"] button[aria-pressed="true"]')!.textContent).toBe("Sổ theo dõi");
    expect(el.textContent).toContain("45/KH-UBND");
    expect(network).not.toHaveBeenCalled();
  });

  it("?modal=giao-viec opens the real create dialog; ?modal=nhap-excel the real import", async () => {
    await mountTasks(tasks({ modal: "giao-viec" }));
    await settle(600);
    expect(document.querySelector("dialog[open]")!.textContent).toContain("Giao việc mới");
    act(() => root?.unmount());
    host?.remove();
    await mountTasks(tasks({ modal: "nhap-excel" }));
    await settle(600);
    expect(document.querySelector("dialog[open]")).not.toBeNull();
    expect(network).not.toHaveBeenCalled();
  });

  it("?chon=2 ticks two tasks and the real bulk-delete bar shows", async () => {
    const el = await mountTasks(tasks({ select: true }));
    await settle(600);
    expect(el.textContent).toContain("Đã chọn 2 nhiệm vụ");
    expect([...el.querySelectorAll("button")].some((b) => b.textContent?.trim() === "Xoá đã chọn")).toBe(true);
  });

  it("?task=NV105 opens the real detail: documents, children, pending extension, comments, attachment", async () => {
    await mountTasks(tasks({ openTask: "NV105" }));
    await settle(800);
    const dialog = document.querySelector("dialog[open]")!;
    expect(dialog.textContent).toContain("[NV105]");
    expect(dialog.textContent).toContain("12-CV/ĐU");
    expect(dialog.textContent).toContain("Thu thập phiếu rà soát hộ gia đình thôn Bình An");
    expect(dialog.textContent).toContain("Còn hai thôn chưa nộp phiếu rà soát");
    expect(dialog.textContent).toContain("Đề nghị gửi bản tổng hợp trước ngày họp giao ban");
    expect(dialog.textContent).toContain("bao-cao-tien-do-dot-1.pdf");
    expect(network).not.toHaveBeenCalled();
  });
});

describe("preview — Văn bản & Đơn thư", () => {
  const docs = (props: Partial<Parameters<typeof DocumentsPreview>[0]> = {}) => (
    <DocumentsPreview tab={null} modal={null} drawer={null} state={null} {...props} />
  );
  const mountDocs = (body: React.ReactNode) =>
    mount("/xem-thu/van-ban", body, false, "", PREVIEW_DOCUMENT_PERMISSIONS, "lanh-dao");
  const selected = (el: ParentNode) => el.querySelector('[role="tab"][aria-selected="true"]')?.textContent;

  // Several mounts and fixture round-trips per case: under the full parallel run the 5 s default is
  // too short, and a case that times out keeps running into the next one.
  const SLOW = 20_000;

  afterEach(() => setDocumentPreviewState(null));

  it("no word: the real screen on its own default tab (Đơn thư công dân), Văn bản lit, no network", async () => {
    const el = await mountDocs(docs());
    await settle(600);
    expect(el.querySelector('.side-nav a[aria-current="page"]')!.getAttribute("href")).toBe("/van-ban");
    expect(el.querySelector("h1")!.textContent).toBe("Văn bản & đơn thư");
    expect(selected(el)).toBe("Đơn thư công dân");
    expect(network).not.toHaveBeenCalled();
  }, SLOW);

  it("?tab=den presses the real tab: the fixture register with its overdue and long rows", async () => {
    const el = await mountDocs(docs({ tab: "den" }));
    await settle(800);
    expect(selected(el)).toBe("Văn bản đến");
    expect(el.querySelectorAll("tbody tr")).toHaveLength(7);
    expect(el.textContent).toContain("V/v rà soát, cập nhật hồ sơ cán bộ, công chức cấp xã");
    expect(el.textContent).toContain("Văn phòng Ban Chỉ đạo phòng, chống thiên tai");
    expect(el.querySelectorAll("tbody tr.bg-danger\\/4")).toHaveLength(1);
    expect(network).not.toHaveBeenCalled();
  }, SLOW);

  it("?tab=di presses the real tab: four outgoing rows", async () => {
    const el = await mountDocs(docs({ tab: "di" }));
    await settle(800);
    expect(selected(el)).toBe("Văn bản đi");
    expect(el.querySelectorAll("tbody tr")).toHaveLength(4);
    expect(el.textContent).toContain("Ông Nguyễn Văn A, thôn Bình An");
  }, SLOW);

  it("?modal=vao-so-den opens the real intake dialog; ?modal=cap-so-di the real issue dialog", async () => {
    await mountDocs(docs({ modal: "vao-so-den" }));
    await settle(800);
    expect(document.querySelector("dialog[open]")!.textContent).toContain("Nhập tay — vào sổ văn bản đến");
    act(() => root?.unmount());
    host?.remove();
    await mountDocs(docs({ modal: "cap-so-di" }));
    await settle(800);
    expect(document.querySelector("dialog[open]")!.textContent).toContain("Cấp số văn bản đi");
    expect(network).not.toHaveBeenCalled();
  }, SLOW);

  it("?drawer=<id> opens the real detail with its three routings", async () => {
    await mountDocs(docs({ drawer: PREVIEW_DOCUMENT_WITH_ROUTINGS }));
    await settle(1000);
    const dialog = document.querySelector("dialog[open]")!;
    expect(dialog.textContent).toContain("Số đến 11/");
    expect(dialog.querySelectorAll('section[aria-labelledby="tieu-de-dong-thoi-gian"] li')).toHaveLength(3);
    expect(dialog.textContent).toContain("Giao Cán bộ B tổng hợp hồ sơ, báo cáo trước hạn.");
    expect(network).not.toHaveBeenCalled();
  }, SLOW);

  it("?drawer=<id> of a never-routed document: the timeline says so", async () => {
    await mountDocs(docs({ drawer: PREVIEW_DOCUMENT_NO_ROUTING }));
    await settle(1000);
    expect(document.querySelector("dialog[open]")!.textContent).toContain("Chưa chuyển cho bộ phận nào.");
  }, SLOW);

  it("?tab=don-thu: the real citizen-letter register on its fixture rows; the denunciation is masked", async () => {
    const el = await mountDocs(docs({ tab: "don-thu" }));
    await settle(800);
    expect(selected(el)).toBe("Đơn thư công dân");
    expect(el.querySelectorAll("tbody tr")).toHaveLength(9);
    expect(el.textContent).toContain("Người gửi được giữ bí mật");
    expect(el.textContent).toContain("Đã gộp vì trùng đơn trước");
    expect(el.querySelector('a[href^="tel:"]')).toBeNull();
    // The booking button: the preview session holds petition.create.
    expect([...el.querySelectorAll("button")].some((b) => b.textContent?.trim() === "Vào sổ đơn thư")).toBe(true);
    expect(network).not.toHaveBeenCalled();
  }, SLOW);

  it("?drawer=<letter id> opens the real letter drawer; ?modal=vao-so-don&dup=1 shows the duplicate box", async () => {
    await mountDocs(docs({ drawer: PREVIEW_LETTER_OVERDUE }));
    await settle(1000);
    const dialog = document.querySelector("dialog[open]")!;
    expect(dialog.textContent).toContain("Đơn số 11/");
    expect(dialog.querySelectorAll('section[aria-labelledby="tieu-de-dong-thoi-gian-don-thu"] li')).toHaveLength(3);
    act(() => root?.unmount());
    host?.remove();
    await mountDocs(docs({ modal: "vao-so-don", duplicate: true }));
    await settle(1600);
    expect(document.querySelector("dialog[open]")!.textContent).toContain("Công dân này đã có 2 đơn nội dung tương tự");
    expect(network).not.toHaveBeenCalled();
  }, SLOW);

  it("?tab=bao-cao: the real report on the fixture year", async () => {
    const el = await mountDocs(docs({ tab: "bao-cao" }));
    await settle(800);
    expect(selected(el)).toBe("Báo cáo");
    expect(el.textContent).toContain("Tiến độ tiếp nhận và xử lý đơn thư năm");
    expect(el.textContent).toContain("Văn phòng HĐND – UBND");
    expect(network).not.toHaveBeenCalled();
  }, SLOW);

  it("?state=empty|error|loading: the register's real empty, error (+ Tải lại) and first-load states", async () => {
    let el = await mountDocs(docs({ tab: "den", state: "empty" }));
    await settle(800);
    expect(el.textContent).toContain("Sổ chưa có văn bản nào.");
    act(() => root?.unmount());
    host?.remove();
    el = await mountDocs(docs({ tab: "den", state: "error" }));
    await settle(800);
    expect(el.textContent).toContain(PREVIEW_REGISTER_ERROR);
    expect([...el.querySelectorAll("button")].some((b) => b.textContent?.trim() === "Tải lại")).toBe(true);
    act(() => root?.unmount());
    host?.remove();
    el = await mountDocs(docs({ tab: "den", state: "loading" }));
    await settle(800);
    expect(el.textContent).toContain("Đang tải sổ văn bản đến");
    expect(network).not.toHaveBeenCalled();
  }, SLOW);
});

describe("preview — shell states for screenshots", () => {
  it("?sidebar=thu-gon collapses the real sidebar; without it the preview is expanded whatever was remembered", async () => {
    let el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, false, "sidebar=thu-gon");
    await settle(100);
    expect(window.localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("1");
    expect(el.querySelector(".side-nav")!.classList.contains("is-collapsed")).toBe(true);
    act(() => root?.unmount());
    host?.remove();
    el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />);
    await settle(100);
    expect(window.localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("0");
    expect(el.querySelector(".side-nav")!.classList.contains("is-collapsed")).toBe(false);
  });

  it("?menu-tai-khoan=1 opens the real account menu; ?chuong=1 opens the real bell panel", async () => {
    await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, false, "menu-tai-khoan=1");
    await settle();
    expect(document.querySelector('[aria-label="Tài khoản"]')).not.toBeNull();
    act(() => root?.unmount());
    host?.remove();
    const el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, false, "chuong=1");
    await settle();
    expect(el.querySelector("button.nut-chuong")!.getAttribute("aria-expanded")).toBe("true");
    expect(el.querySelector("#bang-thong-bao-chuong")).not.toBeNull();
    expect(network).not.toHaveBeenCalled();
  });

  it("?toast=1 shows one toast through the real Toaster; no word, no toast", async () => {
    await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, false, "toast=1");
    await settle(600);
    expect(document.body.textContent).toContain("Đã thêm hạng mục.");
  });

  it("no shell word opens nothing", async () => {
    const el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />);
    await settle();
    expect(el.querySelector("button.nut-chuong")!.getAttribute("aria-expanded")).toBe("false");
    expect(document.querySelector('[aria-label="Tài khoản"]')).toBeNull();
  });
});
