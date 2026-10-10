// @vitest-environment jsdom
//
// jsdom: what this pins — the rows read with the figure's own filter, `Mở` handing the row to the
// drawer, a re-read after a write keeping the rows on screen — are effects and clicks.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";

import { DRILL_EMPTY, drillDescription, TaskDrillDialog } from "./task-drill";
import type { TaskDrillRequest } from "./view";

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

function task(code: string, patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code,
    type: "co-ban",
    bloc: "",
    priority: "",
    title: `Việc ${code}`,
    description: "",
    status: "dang-thuc-hien",
    allowed_transitions: [],
    source: "manual",
    source_id: "",
    unit: "01JUNIT",
    assignee: "CB-0001",
    assigner: "",
    due_at: "2026-09-20T10:00:00Z",
    original_due_at: null,
    completed_at: null,
    progress: 0,
    result_summary: "",
    note: "",
    leader_approved: false,
    superior_acknowledged: false,
    parent: "",
    child_count: 0,
    extension_count: 0,
    pending_extension: false,
    created_by: "",
    created_at: "2026-09-01T00:00:00Z",
    updated_at: "2026-09-01T00:00:00Z",
    ...patch,
  };
}

const OVERDUE: TaskDrillRequest = {
  metric: "overdue",
  period: { from: "2026-09-01T00:00:00+07:00", to: "2026-09-28T16:43:01+07:00" },
  label: "Quá hạn",
};

function stub(items: petitions_nhiemVuRa[]): string[] {
  const seen: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      seen.push(url);
      if (url.startsWith("/api/v1/tasks?")) return json(200, { items, next_cursor: "", has_more: false });
      if (url.startsWith("/api/v1/task-counts")) {
        return json(200, { by_status: [{ status: "dang-thuc-hien", count: items.length }] });
      }
      return new Response("unexpected", { status: 500 });
    }),
  );
  return seen;
}

function dialog(props: Partial<Parameters<typeof TaskDrillDialog>[0]> = {}): ReactNode {
  return (
    <TaskDrillDialog
      request={OVERDUE}
      refresh={0}
      unitNames={new Map([["01JUNIT", "Văn phòng UBND"]])}
      directory={null}
      onOpen={() => {}}
      onClose={() => {}}
      {...props}
    />
  );
}

describe("TaskDrillDialog — the rows behind a task figure (customer sheet row 3)", () => {
  it("reads the figure's OWN filter (metric, no period on a stock metric, no roots) and its count", async () => {
    const seen = stub([task("NV-1")]);
    mount(dialog());
    await settle();
    expect(seen).toContain("/api/v1/tasks?metric=overdue&limit=100");
    expect(seen).toContain("/api/v1/task-counts?metric=overdue");
    expect(seen.some((u) => u.includes("roots="))).toBe(false);
  });

  it("title, description and the prototype's columns, a `Mở` per row", async () => {
    stub([task("NV-1"), task("NV-2", { unit: "", assignee: "" })]);
    const el = mount(dialog());
    await settle();
    expect(el.querySelector("h2")?.textContent).toBe("Quá hạn");
    expect(el.textContent).toContain(drillDescription(2));
    expect([...el.querySelectorAll("thead th")].map((th) => th.textContent)).toEqual([
      "Mã",
      "Nội dung",
      "Bộ phận",
      "Người xử lý",
      "Hạn",
      "Thao tác",
    ]);
    const first = el.querySelectorAll("tbody tr")[0]!;
    expect(first.textContent).toContain("NV-1");
    expect(first.textContent).toContain("Việc NV-1");
    expect(first.textContent).toContain("Văn phòng UBND");
    expect(first.textContent).toContain("20/9/2026");
    const second = el.querySelectorAll("tbody tr")[1]!;
    expect(second.textContent).toContain("Chưa phân công");
  });

  it("`Mở` hands the row to the drawer — no navigation", async () => {
    stub([task("NV-1")]);
    const opened: string[] = [];
    const el = mount(dialog({ onOpen: (t) => opened.push(t.code) }));
    await settle();
    act(() => el.querySelector<HTMLButtonElement>('button[aria-label="Mở NV-1"]')!.click());
    expect(opened).toEqual(["NV-1"]);
    expect(el.querySelector("a[href]")).toBeNull();
  });

  it("a period metric carries the period; an empty answer says so", async () => {
    const seen = stub([]);
    const el = mount(dialog({ request: { ...OVERDUE, metric: "completed", label: "Hoàn thành" } }));
    await settle();
    expect(seen.find((u) => u.startsWith("/api/v1/tasks?"))).toContain("from=");
    expect(el.textContent).toContain(DRILL_EMPTY);
  });

  it("a re-read after a write keeps the rows on screen until the answer replaces them", async () => {
    stub([task("NV-1")]);
    const el = mount(dialog());
    await settle();
    const row = el.querySelector("tbody tr");
    act(() => root!.render(dialog({ refresh: 1 })));
    expect(el.querySelector("tbody tr")).toBe(row);
    await settle();
    expect(el.querySelector("tbody tr")?.textContent).toContain("NV-1");
  });
});
