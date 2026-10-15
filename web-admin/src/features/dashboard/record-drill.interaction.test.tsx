// @vitest-environment jsdom
//
// jsdom: what this pins — each register read with the figure's own filter, the rows and their `Mở`
// buttons (the record opens ON Tổng quan, tester bug sheet), the description's n from the server,
// paging — are effects and clicks.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { documents_vanBanDenRa, finance_duAnRa, petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import { DRILL_EMPTY, drillDescriptionOf } from "./drill-dialog";
import { PROJECT_HEADERS, RecordDrillDialog } from "./record-drill";
import type { RecordDetailTarget } from "./record-detail";
import type { DrillRequest } from "./view";

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

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

async function settle(): Promise<void> {
  for (let i = 0; i < 6; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const PERIOD = { from: "2026-09-01T00:00:00+07:00", to: "2026-09-28T16:43:01+07:00" };
const UNIT = { id: "01JUNIT", name: "Văn phòng UBND" };
const DIRECTORY = { items: [{ code: "CB-0001", full_name: "Nguyễn Văn An" }] };

/** Answers the two name reads and whatever `routes` adds; records every URL asked. */
function stub(routes: (url: string) => Response | null): string[] {
  const seen: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      seen.push(url);
      if (url === "/api/v1/org-units") return json(200, { items: [UNIT] });
      if (url.startsWith("/api/v1/staff-directory")) return json(200, DIRECTORY);
      return routes(url) ?? new Response("unexpected", { status: 500 });
    }),
  );
  return seen;
}

let opened: RecordDetailTarget[] = [];

function dialog(request: Exclude<DrillRequest, { list: "tasks" }>): ReactNode {
  opened = [];
  return <RecordDrillDialog request={request} onOpen={(t) => opened.push(t)} onClose={() => {}} />;
}

/** `Mở <code>` — a BUTTON that opens the record on this page; no link leaves Tổng quan. */
function openButton(row: Element, code: string): HTMLButtonElement {
  expect(row.querySelector("a")).toBeNull();
  const b = row.querySelector<HTMLButtonElement>(`button[aria-label="Mở ${code}"]`);
  expect(b).not.toBeNull();
  expect(b!.getAttribute("aria-haspopup")).toBe("dialog");
  return b!;
}

function document_(id: string, patch: Partial<documents_vanBanDenRa> = {}): documents_vanBanDenRa {
  return {
    id,
    number: 12,
    year: 2026,
    received_date: "2026-09-02",
    issuing_body: "UBND huyện",
    document_type: "cong-van",
    summary: `Trích yếu ${id}`,
    holding_unit: UNIT.id,
    assignee: "CB-0001",
    due_at: "2026-09-20T10:00:00Z",
    status: "dang-xu-ly",
    created_by: "CB-0001",
    created_at: "2026-09-02T00:00:00Z",
    updated_at: "2026-09-02T00:00:00Z",
    ...patch,
  };
}

function petition(code: string): petitions_phieuPhanAnhRa {
  return {
    code,
    channel: "web-xa",
    status: "dang-xu-ly",
    field: "moi-truong",
    field_label: "Môi trường",
    content: "Rác thải ở đầu ngõ 0900000000",
    address: "Thôn 1",
    reporter_name: "Người phản ánh",
    reporter_phone: "0900000000",
    anonymous: false,
    clock_from: "2026-09-02T00:00:00Z",
    booked_at: "2026-09-02T00:00:00Z",
    acknowledge_due: "2026-09-03T00:00:00Z",
    resolve_due: "2026-09-21T10:00:00Z",
    classify_due: null,
    unit: UNIT.id,
    assignee: "",
    result: "",
    public: false,
  };
}

function project(id: string, patch: Partial<finance_duAnRa> = {}): finance_duAnRa {
  return {
    id,
    code: `DA-${id}`,
    year: 2026,
    category_id: "01JCAT",
    name: `Đường thôn ${id}`,
    planned_amount: 100,
    approved_amount: 100,
    disbursed_amount: 10,
    remaining_amount: 90,
    disbursed_ratio: 1000,
    delay_score: null,
    is_delayed: true,
    org_unit_id: UNIT.id,
    assignee_id: "CB-0001",
    disbursement_deadline: "2026-12-15",
    delay_threshold: 1000,
    delay_threshold_source: "mac_dinh",
    ...patch,
  };
}

const rows = (el: HTMLElement) => [...el.querySelectorAll("tbody tr")];

describe("RecordDrillDialog — Văn bản & Đơn thư", () => {
  it("reads the incoming register with the figure's metric (a stock metric carries no period, no year)", async () => {
    const seen = stub((u) => (u.startsWith("/api/v1/incoming-documents?") ? json(200, { items: [document_("A")], next_cursor: "", has_more: false }) : null));
    const el = mount(dialog({ list: "incoming-documents", metric: "overdue", period: PERIOD, label: "Quá hạn xử lý" }));
    await settle();
    const read = seen.find((u) => u.startsWith("/api/v1/incoming-documents?"))!;
    expect(read).toBe("/api/v1/incoming-documents?limit=100&metric=overdue");
    expect(el.querySelector("h2")?.textContent).toBe("Quá hạn xử lý");
    const first = rows(el)[0]!;
    expect(first.textContent).toContain("12/2026");
    expect(first.textContent).toContain("Trích yếu A");
    expect(first.textContent).toContain("Văn phòng UBND");
    expect(first.textContent).toContain("Nguyễn Văn An");
    expect(first.textContent).toContain("20/9/2026");
    act(() => openButton(first, "12/2026").click());
    expect(opened).toEqual([{ kind: "incoming-document", id: "A" }]);
    expect(el.textContent).toContain(drillDescriptionOf("văn bản", 1));
  });

  it("a period metric carries its period", async () => {
    const seen = stub((u) => (u.startsWith("/api/v1/incoming-documents?") ? json(200, { items: [], next_cursor: "", has_more: false }) : null));
    const el = mount(dialog({ list: "incoming-documents", metric: "arrived", period: PERIOD, label: "Đến trong kỳ" }));
    await settle();
    const read = new URL(seen.find((u) => u.startsWith("/api/v1/incoming-documents?"))!, "http://x");
    expect(read.searchParams.get("metric")).toBe("arrived");
    expect(read.searchParams.get("from")).toBe(PERIOD.from);
    expect(read.searchParams.get("to")).toBe(PERIOD.to);
    expect(el.textContent).toContain(DRILL_EMPTY);
  });

  it("no count route: n is said only once every page is read, `Xem thêm` reads on with the cursor", async () => {
    const seen = stub((u) => {
      if (!u.startsWith("/api/v1/incoming-documents?")) return null;
      return u.includes("cursor=")
        ? json(200, { items: [document_("B", { number: 13 })], next_cursor: "", has_more: false })
        : json(200, { items: [document_("A")], next_cursor: "c1", has_more: true });
    });
    const el = mount(dialog({ list: "incoming-documents", metric: "open", period: PERIOD, label: "Chưa xử lý xong" }));
    await settle();
    expect(el.textContent).not.toContain("đằng sau con số này");
    expect(el.textContent).toContain("Đang hiện 1 văn bản đầu");
    const more = [...el.querySelectorAll("button")].find((b) => b.textContent === "Xem thêm")!;
    act(() => more.click());
    await settle();
    expect(seen).toContain("/api/v1/incoming-documents?limit=100&cursor=c1&metric=open");
    expect(rows(el)).toHaveLength(2);
    expect(el.textContent).toContain(drillDescriptionOf("văn bản", 2));
  });

  it("a refused read shows the server's sentence, not an empty list", async () => {
    stub((u) => (u.startsWith("/api/v1/incoming-documents?") ? json(403, { message: "Bạn không có quyền xem sổ văn bản." }) : null));
    const el = mount(dialog({ list: "incoming-documents", metric: "open", period: PERIOD, label: "Chưa xử lý xong" }));
    await settle();
    expect(el.querySelector('[role="alert"]')).not.toBeNull();
    expect(el.textContent).not.toContain(DRILL_EMPTY);
  });
});

describe("RecordDrillDialog — Phản ánh người dân", () => {
  it("reads the register and its count under the SAME filter; n is the server's count", async () => {
    const seen = stub((u) => {
      if (u.startsWith("/api/v1/citizen-reports?")) return json(200, { items: [petition("PA-7F3K-9QXR-MNPT")], next_cursor: "", has_more: false });
      if (u.startsWith("/api/v1/citizen-report-counts")) return json(200, { total: 1 });
      return null;
    });
    const el = mount(dialog({ list: "citizen-reports", metric: "late", period: PERIOD, label: "Trễ hạn trong kỳ" }));
    await settle();
    const list = new URL(seen.find((u) => u.startsWith("/api/v1/citizen-reports?"))!, "http://x");
    const count = new URL(seen.find((u) => u.startsWith("/api/v1/citizen-report-counts"))!, "http://x");
    for (const q of [list, count]) {
      expect(q.searchParams.get("metric")).toBe("late");
      expect(q.searchParams.get("from")).toBe(PERIOD.from);
      expect(q.searchParams.get("to")).toBe(PERIOD.to);
    }
    expect(el.textContent).toContain(drillDescriptionOf("phản ánh", 1));
    const first = rows(el)[0]!;
    act(() => openButton(first, "PA-7F3K-9QXR-MNPT").click());
    expect(opened).toEqual([{ kind: "citizen-report", code: "PA-7F3K-9QXR-MNPT" }]);
    expect(first.textContent).toContain("21/9/2026");
    expect(first.textContent).toContain("Chưa phân công");
  });

  it("rule 3: the citizen's words are cell text only — never in an attribute or a link", async () => {
    stub((u) => {
      if (u.startsWith("/api/v1/citizen-reports?")) return json(200, { items: [petition("PA-1")], next_cursor: "", has_more: false });
      if (u.startsWith("/api/v1/citizen-report-counts")) return json(200, { total: 1 });
      return null;
    });
    const el = mount(dialog({ list: "citizen-reports", metric: "received", period: PERIOD, label: "Nhận vào trong kỳ" }));
    await settle();
    expect(el.textContent).toContain("Rác thải ở đầu ngõ");
    const attributes = [...el.querySelectorAll("*")].flatMap((n) => [...n.attributes].map((a) => a.value));
    expect(attributes.some((v) => v.includes("Rác thải") || v.includes("0900000000"))).toBe(false);
    // The reporter's name and phone are not columns of this list at all.
    expect(el.textContent).not.toContain("Người phản ánh");
  });
});

describe("RecordDrillDialog — Giải ngân ngân sách", () => {
  it("'Dự án chậm' reads the year's delayed projects; n is the list's length (the route is unpaged)", async () => {
    const seen = stub((u) => (u.startsWith("/api/v1/investment-projects?") ? json(200, { items: [project("1")], year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" }) : null));
    const el = mount(dialog({ list: "projects", year: 2026, delayedOnly: true, label: "Dự án chậm" }));
    await settle();
    expect(seen).toContain("/api/v1/investment-projects?year=2026&delayed_only=true");
    expect([...el.querySelectorAll("thead th")].map((th) => th.textContent)).toEqual([...PROJECT_HEADERS, "Thao tác"]);
    const first = rows(el)[0]!;
    expect(first.textContent).toContain("DA-1");
    expect(first.textContent).toContain("Đường thôn 1");
    expect(first.textContent).toContain("Văn phòng UBND");
    expect(first.textContent).toContain("Nguyễn Văn An");
    expect(first.textContent).toContain("15/12/2026");
    act(() => openButton(first, "DA-1").click());
    expect(opened).toEqual([{ kind: "project", id: "1" }]);
    expect(el.textContent).toContain(drillDescriptionOf("dự án", 1));
    expect(el.textContent).not.toContain("Xem thêm");
  });

  it("'Tỷ lệ giải ngân' reads every project of the year — no delayed filter", async () => {
    const seen = stub((u) => (u.startsWith("/api/v1/investment-projects?") ? json(200, { items: [], year: 2026, delay_threshold: 1000, delay_threshold_source: "mac_dinh" }) : null));
    const el = mount(dialog({ list: "projects", year: 2026, delayedOnly: false, label: "Tỷ lệ giải ngân" }));
    await settle();
    expect(seen).toContain("/api/v1/investment-projects?year=2026");
    expect(el.textContent).toContain(DRILL_EMPTY);
  });
});
