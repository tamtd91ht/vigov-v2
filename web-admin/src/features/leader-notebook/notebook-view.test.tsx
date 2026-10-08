import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { BANG_NHAN_MAC_DINH, oHan } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing register wording under comparison
import { KhungQuyen } from "@/features/quyen/cong-quyen"; // vi-name-ok: existing gate component
import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing staff-directory lookup type
import type { petitions_deNghiChoDuyetRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract types

import {
  COLUMN_EMPTY,
  COMPLETION_GROUP_TITLE,
  EXTENSION_GROUP_EMPTY,
  EXTENSION_GROUP_TITLE,
  LOAD_ERROR_TITLE,
  NO_ACCESS_SENTENCE,
  PAGE_SUBTITLE,
} from "./notebook-queries";
import { NotebookView, type ListState, type Loaded, type NotebookViewProps, type Section } from "./notebook-view";
import { extensionDeadlineText, taskRowText } from "./task-mini-list";

/** 10:00 on 16/9/2026 in Hà Nội. */
const NOW = new Date("2026-09-16T03:00:00Z");
const ASSIGNEE = "CB-2026-7K3M9Q";
const DIRECTORY: DanhBaTheoMa = new Map([
  [ASSIGNEE, { code: ASSIGNEE, full_name: "Huỳnh Văn Sáu", position: "", department_id: "", email_masked: null }],
]);

function task(patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    type: "co-ban",
    bloc: "",
    priority: "",
    title: "Xử lý tồn đọng giải phóng mặt bằng",
    description: "",
    status: "dang-thuc-hien",
    allowed_transitions: [],
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: ASSIGNEE,
    assigner: "",
    due_at: "2026-09-10T16:59:59Z",
    original_due_at: "2026-09-10T16:59:59Z",
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
    created_by: ASSIGNEE,
    created_at: "2026-09-01T02:00:00Z",
    updated_at: "2026-09-01T02:00:00Z",
    ...patch,
  };
}

function request(patch: Partial<petitions_deNghiChoDuyetRa> = {}): petitions_deNghiChoDuyetRa {
  return {
    id: "01JDENGHI0001",
    task_code: "NV21",
    task_title: "Báo cáo tổng kết",
    task_due_at: "2026-06-20T16:59:59Z",
    task_assigner: "",
    new_due_at: "2026-07-15T16:59:59Z",
    reason: "Chờ số liệu",
    requested_by: ASSIGNEE,
    requested_at: "2026-06-18T02:20:00Z",
    ...patch,
  };
}

const noop = () => {};

function list<T>(load: Loaded<readonly T[]>, extra: Partial<ListState<T>> = {}): ListState<T> {
  return { load, hasMore: false, loadingMore: false, moreError: null, onRetry: noop, onMore: noop, ...extra };
}

function section<T>(items: readonly T[], count: number): Section<T> {
  return { list: list<T>({ phase: "done", value: items }), count: { phase: "done", value: count } };
}

function render(patch: Partial<NotebookViewProps> = {}): string {
  return renderToStaticMarkup(
    <NotebookView
      pastDue={section([task()], 18)}
      completion={section([task({ code: "NV30", status: "cho-duyet", due_at: null })], 2)}
      extensions={section([request()], 1)}
      assigned={section([task({ code: "NV40", status: "tam-dung" })], 26)}
      directory={DIRECTORY}
      statusLabels={BANG_NHAN_MAC_DINH}
      now={NOW}
      {...patch}
    />,
  );
}

/** How many columns say the prototype's empty sentence. */
function emptyColumns(html: string): number {
  return html.split(COLUMN_EMPTY).length - 1;
}

/** The badge figures in document order — column 1, 2, 3. */
function badges(html: string): string[] {
  return [...html.matchAll(/notebook-count[^>]*>(?:<svg.*?<\/svg>)?([^<]*)</g)].map((m) => m[1]!);
}

describe("badges are the SERVER's counts, never the length of the page", () => {
  it("one row on screen, 18 in the commune: the badge says 18", () => {
    expect(badges(render())).toEqual(["18", "3", "26"]);
  });

  it("column 2 adds only the groups that are shown", () => {
    expect(badges(render({ completion: null }))).toEqual(["18", "1", "26"]);
  });

  it("a count still loading or failed shows a dash, never 0 and never the row count", () => {
    const html = render({
      pastDue: { list: list({ phase: "done", value: [task()] }), count: { phase: "error", message: "Máy chủ bận." } },
      assigned: { list: list({ phase: "done", value: [task()] }), count: { phase: "loading" } },
    });
    expect(badges(html)).toEqual(["—", "3", "—"]);
    expect(html).toContain("Chưa đếm được: Máy chủ bận.");
  });
});

describe("group 'Duyệt hoàn thành' needs task.approve (ADR 0071)", () => {
  it("drawn with its title and rows when the session holds the key", () => {
    const html = render();
    expect(html).toContain(COMPLETION_GROUP_TITLE);
    expect(html).toContain("/nhiem-vu?task=NV30");
    expect(html).toContain(EXTENSION_GROUP_TITLE);
  });

  it("DENIED: not drawn at all — no title, no rows, no empty sentence", () => {
    const html = render({ completion: null });
    expect(html).not.toContain(COMPLETION_GROUP_TITLE);
    expect(html).not.toContain("NV30");
    expect(html).toContain(EXTENSION_GROUP_TITLE);
  });

  it("every visible group empty: the column's own empty sentence, once", () => {
    const html = render({ completion: section([], 0), extensions: section([], 0) });
    expect(emptyColumns(html)).toBe(1);
    expect(html).not.toContain(COMPLETION_GROUP_TITLE);
  });

  it("one group empty while the other is not: said inside that group", () => {
    const html = render({ extensions: section([], 0) });
    expect(html).toContain(EXTENSION_GROUP_EMPTY);
    expect(emptyColumns(html)).toBe(0);
  });
});

describe("empty is not error, error is not empty", () => {
  it("an empty column says the prototype's sentence, 'Không có việc nào.'", () => {
    const html = render({ pastDue: section([], 0), assigned: section([], 0) });
    expect(COLUMN_EMPTY).toBe("Không có việc nào.");
    expect(emptyColumns(html)).toBe(2);
    expect(html).not.toContain(LOAD_ERROR_TITLE);
  });

  it("a failed read shows the server's sentence verbatim with Tải lại — and NO empty sentence", () => {
    const html = render({
      pastDue: { list: list({ phase: "error", message: "Không đọc được bộ lọc nhiệm vụ." }), count: { phase: "loading" } },
    });
    expect(html).toContain(LOAD_ERROR_TITLE);
    expect(html).toContain("Không đọc được bộ lọc nhiệm vụ.");
    expect(html).toContain("Tải lại");
    expect(emptyColumns(html)).toBe(0);
  });

  it("loading draws the skeleton, not an empty sentence", () => {
    const html = render({ assigned: { list: list({ phase: "loading" }), count: { phase: "loading" } } });
    expect(html).toContain("skeleton-rows");
    expect(emptyColumns(html)).toBe(0);
  });

  it("Xem thêm only when the server says there is more", () => {
    expect(render()).not.toContain("Xem thêm");
    const html = render({
      pastDue: { list: list({ phase: "done", value: [task()] }, { hasMore: true }), count: { phase: "done", value: 80 } },
    });
    expect(html).toContain("Xem thêm");
  });
});

describe("row wording equals the register's (`oHan`)", () => {
  it("assignee name · hạn d/M/yyyy · trễ N ngày — the register's figure, without its brackets", () => {
    const row = taskRowText(task(), DIRECTORY, NOW);
    const cell = oHan(task().due_at, NOW);
    expect(row.meta).toBe(`Huỳnh Văn Sáu · hạn ${cell.ngay}`);
    expect(`(${row.late})`).toBe(cell.phanTre);
    expect(row.late).toBe("trễ 5 ngày");
  });

  it("late by less than a day: 'trễ dưới 1 ngày', never 'trễ 0 ngày'", () => {
    const row = taskRowText(task({ due_at: "2026-09-16T01:00:00Z" }), DIRECTORY, NOW);
    expect(row.late).toBe("trễ dưới 1 ngày");
  });

  it("no deadline: 'hạn —' and no late part; unassigned: 'Chưa phân công'", () => {
    const row = taskRowText(task({ due_at: null, assignee: "" }), DIRECTORY, NOW);
    expect(row.meta).toBe("Chưa phân công · hạn —");
    expect(row.late).toBe("");
  });

  it("directory not loaded: the staff code, never an empty name", () => {
    expect(taskRowText(task(), null, NOW).meta.startsWith(ASSIGNEE)).toBe(true);
  });

  it("a sub-task names its parent; the late part is drawn red", () => {
    const html = render({ pastDue: section([task({ parent: "NV7" })], 1) });
    expect(html).toContain("việc con của NV7");
    expect(html).toContain('<span class="font-semibold text-danger-600">trễ 5 ngày</span>');
  });

  it("a paused task shows its status pill in the commune's label", () => {
    const html = render();
    expect(html).toContain("Tạm dừng");
  });

  it("an extension row reads current deadline → proposed deadline", () => {
    expect(extensionDeadlineText(request())).toBe("hạn 20/6/2026 → 15/7/2026");
    expect(extensionDeadlineText(request({ task_due_at: null }))).toBe("hạn — → 15/7/2026");
    expect(render()).toContain("/nhiem-vu?task=NV21");
  });

  it("no personal data in URLs or aria: rows link by register code only", () => {
    const html = render();
    for (const m of html.matchAll(/href="([^"]*)"/g)) expect(m[1]).toMatch(/^\/nhiem-vu\?task=NV\d+$/);
    expect(html).not.toMatch(/aria-label="[^"]*Huỳnh/);
  });
});

describe("prototype composition (ADR 0068 lần 5)", () => {
  it("three columns side by side, one column at 1280px and below", () => {
    expect(render()).toContain("grid min-w-0 grid-cols-3 gap-4 max-[1280px]:grid-cols-1");
  });

  it("column order and titles are the prototype's", () => {
    const html = render();
    const order = ["Việc quá hạn", "Chờ tôi duyệt", "Việc tôi đã giao"].map((t) => html.indexOf(`>${t}<`));
    expect(order.every((i) => i >= 0)).toBe(true);
    expect([...order].sort((a, b) => a - b)).toEqual(order);
  });

  it("the count is a neutral pill after the title, and each body scrolls past 560px", () => {
    const html = render();
    expect(html.match(/notebook-count ml-auto/g)).toHaveLength(3);
    expect(html.match(/max-h-\[560px\] overflow-y-auto/g)).toHaveLength(3);
  });

  it("the page subtitle is the prototype's sentence", () => {
    expect(PAGE_SUBTITLE).toBe("Ba việc cần biết ngay: việc trễ, việc chờ duyệt và việc mình đã giao.");
  });
});

describe("page gate (task.read)", () => {
  it("DENIED: the shared NoAccess with the sentence naming the right; no column drawn", () => {
    const html = renderToStaticMarkup(
      <KhungQuyen quyetDinh={{ hien: false, vi: "khong-du-quyen" }} cauThieuQuyen={NO_ACCESS_SENTENCE}>
        {render()}
      </KhungQuyen>,
    );
    expect(html).toContain(NO_ACCESS_SENTENCE);
    expect(html).not.toContain("leader-notebook");
  });
});
