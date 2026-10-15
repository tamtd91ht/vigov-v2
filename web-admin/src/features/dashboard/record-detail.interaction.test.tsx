// @vitest-environment jsdom
//
// jsdom: a record of a Tổng quan list opens ON Tổng quan (tester bug sheet, Tổng quan: "mở chi tiết
// item đó ngay màn hình Tổng quan, không được mở ra màn hình menu chức năng"). What this pins: each
// register's OWN detail is drawn, read through its own detail route, with no link to the module's
// screen; and the write controls follow the SAME keys as on the register — the denied case first.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { NUT_DONG_CHI_TIET, NUT_GO, NUT_SUA, TIEU_DE_KHOI_CHUYEN } from "@/features/van-ban/nhan-van-ban";
import { nhanTrangThai, STRIP_NEEDS_ASSIGN } from "@/features/phan-anh/nhan-phieu";
import type { documents_vanBanDenRa, finance_duAnRa, petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import { RecordDetail, type RecordDetailTarget } from "./record-detail";

const session = vi.hoisted(() => ({ permissions: [] as string[] }));

vi.mock("@/lib/api/phien", () => ({
  layPhienHienTai: () => Promise.resolve({ ok: true, duLieu: { permissions: session.permissions } }),
}));
vi.mock("@/features/mat-khau/bat-doi-mat-khau", () => ({ duongDanBatDoiMatKhau: () => null }));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let host: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  if (root !== null) act(() => root!.unmount());
  host?.remove();
  host = null;
  root = null;
  vi.unstubAllGlobals();
});

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

/** Answers `routes`; every other read is refused (500) — the detail must still draw. Records URLs. */
function stub(routes: (url: string) => Response | null): string[] {
  const seen: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      seen.push(url);
      return routes(url) ?? json(500, { message: "Không dùng trong bài kiểm." });
    }),
  );
  return seen;
}

async function mount(target: RecordDetailTarget, permissions: string[], onClose = () => {}): Promise<HTMLDivElement> {
  session.permissions = permissions;
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  const node: ReactNode = (
    <PhienProvider>
      <RecordDetail target={target} onClose={onClose} onChanged={() => {}} />
    </PhienProvider>
  );
  act(() => r.render(node));
  for (let i = 0; i < 8; i++) {
    await act(async () => {
      await new Promise((res) => setTimeout(res, 0));
    });
  }
  return host;
}

const DOC: documents_vanBanDenRa = {
  id: "D1",
  number: 12,
  year: 2026,
  received_date: "2026-09-02",
  issuing_body: "UBND huyện",
  document_type: "cong-van",
  summary: "Trích yếu văn bản D1",
  holding_unit: "",
  assignee: "",
  due_at: "2026-09-20T10:00:00Z",
  status: "dang-xu-ly",
  created_by: "CB-0001",
  created_at: "2026-09-02T00:00:00Z",
  updated_at: "2026-09-02T00:00:00Z",
};

function docRoutes(u: string): Response | null {
  if (u === "/api/v1/incoming-documents/D1") return json(200, DOC);
  if (u === "/api/v1/incoming-documents/D1/routings") return json(200, { items: [] });
  return null;
}

describe("RecordDetail — văn bản đến", () => {
  it("DENIED (no document.route): the register's drawer, read by id, with NO routing box and no Sửa / Gỡ", async () => {
    const seen = stub(docRoutes);
    const el = await mount({ kind: "incoming-document", id: "D1" }, ["report.read", "document.read"]);
    expect(seen).toContain("/api/v1/incoming-documents/D1");
    expect(seen).toContain("/api/v1/incoming-documents/D1/routings");
    const dialog = el.querySelector("dialog")!;
    expect(dialog.getAttribute("aria-labelledby")).toBe("tieu-de-ngan-van-ban-den");
    expect(dialog.textContent).toContain("Số đến 12/2026");
    expect(dialog.textContent).toContain("Trích yếu văn bản D1");
    expect(dialog.textContent).not.toContain(TIEU_DE_KHOI_CHUYEN);
    const buttons = [...dialog.querySelectorAll("button")].map((b) => b.textContent?.trim());
    expect(buttons).not.toContain(NUT_SUA);
    expect(buttons).not.toContain(NUT_GO);
    expect([...dialog.querySelectorAll("a")].some((a) => (a.getAttribute("href") ?? "").startsWith("/van-ban"))).toBe(false);
  });

  it("ALLOWED (document.route): the routing box is drawn, as on the register", async () => {
    stub(docRoutes);
    const el = await mount({ kind: "incoming-document", id: "D1" }, ["report.read", "document.read", "document.route"]);
    expect(el.querySelector("dialog")!.textContent).toContain(TIEU_DE_KHOI_CHUYEN);
  });

  it("✕ closes through the caller — nothing navigates", async () => {
    stub(docRoutes);
    const onClose = vi.fn();
    const el = await mount({ kind: "incoming-document", id: "D1" }, ["document.read"], onClose);
    const close = el.querySelector<HTMLButtonElement>(`dialog header button[aria-label="${NUT_DONG_CHI_TIET}"]`)!;
    act(() => close.click());
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe("RecordDetail — phản ánh", () => {
  it("reads the petition through the detail route; a refusal is the server's sentence, verbatim", async () => {
    const seen = stub((u) =>
      u === "/api/v1/citizen-reports/PA-7F3K-9QXR-MNPT" ? json(404, { message: "Không tìm thấy phiếu phản ánh." }) : null,
    );
    const el = await mount({ kind: "citizen-report", code: "PA-7F3K-9QXR-MNPT" }, ["report.read", "feedback.read"]);
    expect(seen).toContain("/api/v1/citizen-reports/PA-7F3K-9QXR-MNPT");
    const dialog = el.querySelector("dialog")!;
    expect(dialog.querySelector("h2")?.textContent).toBe("Phản ánh PA-7F3K-9QXR-MNPT");
    expect(dialog.querySelector('[role="alert"]')?.textContent).toBe("Không tìm thấy phiếu phản ánh.");
    expect(dialog.querySelector("a")).toBeNull();
  });

  const SCREENING: petitions_phieuPhanAnhRa = {
    code: "PA-1",
    channel: "web-xa",
    status: "dang-phan-loai",
    field: "moi-truong",
    field_label: "Môi trường",
    content: "Rác thải ở đầu ngõ",
    address: "Thôn 1",
    reporter_name: "Người phản ánh",
    reporter_phone: "0900000000",
    anonymous: false,
    clock_from: "2026-09-02T00:00:00Z",
    booked_at: "2026-09-02T00:00:00Z",
    acknowledge_due: "2026-09-03T00:00:00Z",
    resolve_due: null,
    classify_due: null,
    unit: "",
    assignee: "",
    result: "",
    public: false,
  };
  const petitionRoutes = (u: string) => (u === "/api/v1/citizen-reports/PA-1" ? json(200, SCREENING) : null);
  const assignChip = (el: HTMLElement) =>
    [...el.querySelectorAll<HTMLButtonElement>("dialog button")].find((b) => b.textContent?.includes(nhanTrangThai("da-chuyen-xu-ly")));

  it("DENIED (no feedback.assign): the register's drawer, its 'Đã chuyển xử lý' step NOT pressable", async () => {
    stub(petitionRoutes);
    const el = await mount({ kind: "citizen-report", code: "PA-1" }, ["report.read", "feedback.read"]);
    expect(el.querySelector("dialog")!.textContent).toContain("Rác thải ở đầu ngõ");
    const chip = assignChip(el)!;
    expect(chip.getAttribute("aria-disabled")).toBe("true");
    expect(chip.title).toBe(STRIP_NEEDS_ASSIGN);
  });

  it("ALLOWED (feedback.assign): the same step is pressable — the register's gate, unchanged", async () => {
    stub(petitionRoutes);
    const el = await mount({ kind: "citizen-report", code: "PA-1" }, ["report.read", "feedback.read", "feedback.assign"]);
    const chip = assignChip(el)!;
    expect(chip.getAttribute("aria-disabled")).toBeNull();
  });
});

const PROJECT: finance_duAnRa = {
  id: "P1",
  code: "DA-P1",
  year: 2026,
  category_id: "01JCAT",
  name: "Đường thôn P1",
  planned_amount: 100,
  approved_amount: 100,
  disbursed_amount: 10,
  remaining_amount: 90,
  disbursed_ratio: 1000,
  delay_score: null,
  is_delayed: false,
  org_unit_id: "",
  assignee_id: "",
  disbursement_deadline: "2026-12-15",
  delay_threshold: 1000,
  delay_threshold_source: "mac_dinh",
};

describe("RecordDetail — dự án", () => {
  it("the project's own detail in a panel on this page, WITHOUT the '← Theo dõi giải ngân' link", async () => {
    const seen = stub((u) => (u === "/api/v1/investment-projects/P1" ? json(200, PROJECT) : null));
    const el = await mount({ kind: "project", id: "P1" }, ["report.read", "budget.read"]);
    expect(seen).toContain("/api/v1/investment-projects/P1");
    const dialog = el.querySelector("dialog")!;
    expect(dialog.getAttribute("aria-labelledby")).toBe("tieu-de-chi-tiet-du-an-tong-quan");
    expect(dialog.textContent).toContain("Đường thôn P1");
    expect(dialog.textContent).not.toContain("Theo dõi giải ngân");
    expect([...dialog.querySelectorAll("a")].some((a) => a.getAttribute("href") === "/giai-ngan")).toBe(false);
  });

  it("DENIED (no budget.update): the project's edit control is not drawn — its own gate, unchanged", async () => {
    stub((u) => (u === "/api/v1/investment-projects/P1" ? json(200, PROJECT) : null));
    const el = await mount({ kind: "project", id: "P1" }, ["report.read", "budget.read"]);
    const buttons = [...el.querySelectorAll("dialog button")].map((b) => b.textContent?.trim() ?? "");
    expect(buttons.some((t) => t.includes("Sửa dự án"))).toBe(false);
  });

  it("ALLOWED (budget.update): the same control is drawn — the gate is the project's own", async () => {
    stub((u) => (u === "/api/v1/investment-projects/P1" ? json(200, PROJECT) : null));
    const el = await mount({ kind: "project", id: "P1" }, ["report.read", "budget.read", "budget.update"]);
    const buttons = [...el.querySelectorAll("dialog button")].map((b) => b.textContent?.trim() ?? "");
    expect(buttons.some((t) => t.includes("Sửa dự án"))).toBe(true);
  });
});
