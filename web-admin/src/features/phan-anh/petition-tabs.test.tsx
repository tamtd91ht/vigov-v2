// @vitest-environment jsdom
//
// jsdom for this file: the main tabs, the list total, the composer's files and the classify act's hamlet
// are BEHAVIOUR — which reads start when a tab is pressed, what a denied account sees, what an act sends.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { identity_thonToDanPhoRa, petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";
import { BASEMAP_MISSING_SENTENCE } from "@/lib/basemap/assets";

import { congThaoTac, REPORT_READ_DENIED } from "./nhan-phieu";
import { ChiTietPhieu, SoPhanAnh } from "./so-phan-anh";

const session = vi.hoisted(() => ({ permissions: [] as string[] }));
vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => ({ ok: true, duLieu: { permissions: session.permissions, staff: { code: "CB-00123" } } }),
}));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

async function flush(times = 8) {
  for (let i = 0; i < times; i++) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

const PETITION: petitions_phieuPhanAnhRa = {
  code: "PA-2026-0021",
  channel: "zalo-mini-app",
  status: "dang-xu-ly",
  field: "rac-thai",
  field_label: "Rác thải – Vệ sinh môi trường",
  content: "Rác tồn đọng ở đầu ngõ.",
  address: "Đầu ngõ thôn Hà Lam",
  reporter_name: "",
  reporter_phone: "",
  anonymous: true,
  clock_from: "2026-09-09T07:20:00Z",
  booked_at: "2026-09-09T07:21:00Z",
  acknowledge_due: "2026-09-09T09:20:00Z",
  resolve_due: "2026-09-10T09:20:00Z",
  classify_due: "2026-09-09T11:20:00Z",
  unit: "",
  assignee: "",
  result: "",
  public: false,
};

function json(body: unknown, status = 200): Promise<Response> {
  return Promise.resolve(new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }));
}

/** A fake server: the list (two cards), the count (57), everything else never answers. */
function stubServer() {
  const fetchSpy = vi.fn((url: string) => {
    const u = String(url);
    if (u.startsWith("/api/v1/citizen-reports?") || u === "/api/v1/citizen-reports") {
      return json({ items: [PETITION, { ...PETITION, code: "PA-2026-0022" }], has_more: true, next_cursor: "c2" });
    }
    if (u.startsWith("/api/v1/citizen-report-counts")) return json({ total: 57 });
    return new Promise<Response>(() => {});
  });
  vi.stubGlobal("fetch", fetchSpy);
  return fetchSpy;
}

const urls = (spy: ReturnType<typeof stubServer>) => spy.mock.calls.map(([u]) => String(u));

function tab(el: HTMLElement, startsWith: string): HTMLButtonElement {
  const t = [...el.querySelectorAll<HTMLButtonElement>('[role="tablist"] [role="tab"]')].find((x) =>
    x.textContent?.startsWith(startsWith),
  );
  if (t === undefined) throw new Error(`no tab ${startsWith}`);
  return t;
}

function pick(s: HTMLSelectElement, v: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")?.set;
  act(() => {
    setter?.call(s, v);
    s.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

describe("main tabs (prototype `FeedbackWorkspace.tsx:134-138`)", () => {
  it("three live tabs; `Danh sách (n)` carries the COUNT ROUTE's total; the pager says `1–2 trên 57 mục`", async () => {
    session.permissions = ["feedback.read"];
    const spy = stubServer();
    const el = mount(<SoPhanAnh />);
    await flush();
    const tabs = [...el.querySelectorAll<HTMLButtonElement>('[role="tablist"] [role="tab"]')];
    expect(tabs.map((t) => t.textContent)).toEqual(["Danh sách (57)", "Bản đồ nhiệt", "Báo cáo"]);
    expect(tabs.every((t) => !t.disabled)).toBe(true);
    expect(tabs[0]!.getAttribute("aria-selected")).toBe("true");
    expect(urls(spy).some((u) => u === "/api/v1/citizen-report-counts")).toBe(true);
    expect(el.querySelector("[data-page-range]")?.textContent).toBe("1–2 trên 57 mục");
  });

  it("a refused count: the bare word, no number and no pager line (never a page passed off as the total)", async () => {
    session.permissions = ["feedback.read"];
    vi.stubGlobal(
      "fetch",
      vi.fn((url: string) =>
        String(url).startsWith("/api/v1/citizen-report-counts")
          ? json({ code: "forbidden", message: "Không có quyền." }, 403)
          : String(url).startsWith("/api/v1/citizen-reports?")
            ? json({ items: [PETITION], has_more: false, next_cursor: "" })
            : new Promise<Response>(() => {}),
      ),
    );
    const el = mount(<SoPhanAnh />);
    await flush();
    expect(tab(el, "Danh sách").textContent).toBe("Danh sách");
    expect(el.querySelector("[data-page-range]")).toBeNull();
  });

  it("Bản đồ nhiệt without the basemap: the sentence, and NO coordinate read", async () => {
    session.permissions = ["feedback.read"];
    const spy = stubServer();
    const el = mount(<SoPhanAnh />);
    await flush();
    act(() => tab(el, "Bản đồ nhiệt").click());
    await flush();
    expect(el.querySelector("#petition-map-panel")?.textContent).toContain(BASEMAP_MISSING_SENTENCE);
    expect(urls(spy).some((u) => u.startsWith("/api/v1/citizen-report-points"))).toBe(false);
    expect(urls(spy).some((u) => u.startsWith("/api/v1/map-frame"))).toBe(false);
    // The list's filter row belongs to the list tab (prototype): not drawn here.
    expect(el.querySelector("#petition-filters")).toBeNull();
  });

  it("Bản đồ nhiệt with the basemap: the points of the LIST's filters, and the commune frame", async () => {
    session.permissions = ["feedback.read"];
    const spy = stubServer();
    const el = mount(<SoPhanAnh basemapAvailable />);
    await flush();
    pick(el.querySelector<HTMLSelectElement>("#loc-trang-thai")!, "dang-xu-ly");
    await flush();
    act(() => tab(el, "Bản đồ nhiệt").click());
    await flush();
    expect(urls(spy)).toContain("/api/v1/citizen-report-points?status=dang-xu-ly");
    expect(urls(spy)).toContain("/api/v1/map-frame");
  });

  it("Báo cáo DENIED without `report.read`: the denial sentence, no report read", async () => {
    session.permissions = ["feedback.read"];
    const spy = stubServer();
    const el = mount(<SoPhanAnh />);
    await flush();
    act(() => tab(el, "Báo cáo").click());
    await flush();
    expect(el.querySelector("#petition-report-panel")?.textContent).toContain(REPORT_READ_DENIED);
    expect(urls(spy).some((u) => u.startsWith("/api/v1/citizen-report-breakdown"))).toBe(false);
  });

  it("Báo cáo with `report.read`: the breakdown of the current month is read", async () => {
    session.permissions = ["feedback.read", "report.read"];
    const spy = stubServer();
    const el = mount(<SoPhanAnh />);
    await flush();
    act(() => tab(el, "Báo cáo").click());
    await flush();
    expect(el.querySelector("#petition-report-panel")?.textContent).not.toContain(REPORT_READ_DENIED);
    expect(urls(spy).some((u) => /^\/api\/v1\/citizen-report-breakdown\?from=.+&to=.+/.test(u))).toBe(true);
  });
});

function unit(id: string, name: string, active: boolean): identity_thonToDanPhoRa {
  return {
    id,
    code: id.toLowerCase(),
    name,
    type_code: "thon",
    type_label: "Thôn",
    household_count: null,
    population_count: null,
    active,
    head_staff_code: "",
    head_staff_name: "",
    order: 0,
  };
}

type DrawerProps = Parameters<typeof ChiTietPhieu>[0];

function drawer(p: petitions_phieuPhanAnhRa, step: string, more: Partial<DrawerProps> = {}) {
  return mount(
    <ChiTietPhieu
      phieu={p}
      bayGio={new Date("2026-09-09T08:00:00Z")}
      cong={congThaoTac(true, true, true)}
      tenBoPhan={new Map()}
      boPhan={[]}
      danhBa={null}
      dangGui={false}
      loiGhi={null}
      dong={() => {}}
      phanLoai={() => {}}
      chuyenXuLy={() => {}}
      tienTrangThai={() => {}}
      dongPhieuLai={() => {}}
      khongTiepNhan={() => {}}
      chuyenCapTren={() => {}}
      initialStep={step}
      {...more}
    />,
  );
}

function submitComposer(el: HTMLElement) {
  act(() =>
    el
      .querySelector<HTMLFormElement>('[role="group"] form')!
      .dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })),
  );
}

describe("composer — `Đính kèm ảnh, tệp` is live (ADR 0088 §2)", () => {
  it("an enabled button over a hidden file input (PDF/JPG/PNG); the act sends the STORED ids (none → [])", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {})));
    const advance = vi.fn(async () => true);
    const el = drawer({ ...PETITION, status: "dang-xu-ly" }, "da-xu-ly", { tienTrangThai: advance });
    const attach = [...el.querySelectorAll<HTMLButtonElement>("button")].find((b) =>
      b.textContent?.includes("Đính kèm ảnh, tệp"),
    )!;
    expect(attach.disabled).toBe(false);
    const input = el.querySelector<HTMLInputElement>("#petition-composer-files")!;
    expect(input.type).toBe("file");
    expect(input.accept).toContain("application/pdf");
    const clicked = vi.spyOn(input, "click").mockImplementation(() => {});
    act(() => attach.click());
    expect(clicked).toHaveBeenCalledTimes(1);
    // No '?' left on the composer.
    expect(el.querySelector('[data-pending-marker][aria-label*="Đính kèm"]')).toBeNull();
    submitComposer(el);
    expect(advance).toHaveBeenCalledWith("", []);
  });
});

describe("classify act — `Thôn, tổ dân phố` (ADR 0088 §1)", () => {
  const UNITS = [
    unit("01JTHON1", "Thôn Hà Lam", true),
    unit("01JTHON2", "Thôn Bình An", true),
    unit("01JTHON9", "Thôn Cũ", false),
  ];

  it("no stored hamlet: the empty choice + units IN USE; a pick is sent with the field", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {})));
    const classify = vi.fn(async () => true);
    const el = drawer({ ...PETITION, status: "da-tiep-nhan", field: "" }, "dang-phan-loai", {
      thon: UNITS,
      phanLoai: classify,
    });
    const select = el.querySelector<HTMLSelectElement>("#chon-thon-phan-loai")!;
    expect([...select.options].map((o) => o.value)).toEqual(["", "01JTHON1", "01JTHON2"]);
    pick(el.querySelector<HTMLSelectElement>("#chon-linh-vuc")!, "rac-thai");
    pick(select, "01JTHON2");
    submitComposer(el);
    expect(classify).toHaveBeenCalledWith("rac-thai", "", { residentialUnitId: "01JTHON2", attachments: [] });
  });

  it("a stored hamlet: PREFILLED (sending it = confirm), no empty choice; a retired stored unit stays, marked", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {})));
    const el = drawer(
      { ...PETITION, status: "da-tiep-nhan", residential_unit_id: "01JTHON9", residential_unit_name: "Thôn Cũ" },
      "dang-phan-loai",
      { thon: UNITS },
    );
    const select = el.querySelector<HTMLSelectElement>("#chon-thon-phan-loai")!;
    expect(select.value).toBe("01JTHON9");
    expect([...select.options].map((o) => o.textContent)).toEqual([
      "Thôn Cũ (ngừng dùng)",
      "Thôn Hà Lam",
      "Thôn Bình An",
    ]);
  });

  it("a refusal of the act (400 `residential_unit_not_offered`) is shown IN the classify act, verbatim", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {})));
    const sentence = "Thôn, tổ dân phố đã chọn không thuộc xã hoặc đã ngừng dùng.";
    const el = drawer({ ...PETITION, status: "da-tiep-nhan" }, "dang-phan-loai", {
      thon: UNITS,
      classifyRefusal: sentence,
    });
    expect(el.querySelector('[role="group"] [role="alert"]')?.textContent).toBe(sentence);
  });
});
