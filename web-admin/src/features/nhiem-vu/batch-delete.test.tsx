import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { page_Result_petitions_nhiemVuRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen";

import { BatchDeleteDialog } from "./batch-delete-bar";
import {
  BATCH_NOTE,
  EMPTY_SELECTION,
  SELECT_ALL_LABEL,
  batchDeleteConfirmLabel,
  batchDeleteTitle,
  batchFailureToasts,
  batchProgressText,
  batchSuccessToast,
  deleteOrder,
  keepFailed,
  runBatchDelete,
  selectAllState,
  selectedCountLabel,
  toggleAllSelected,
  toggleSelected,
  type SelectedTask,
  type TaskSelection,
} from "./batch-delete";
import { BANG_NHAN_MAC_DINH, PHAN_CHUA_DUNG, TRANG_THAI_CHINH } from "./nhan-nhiem-vu";
import {
  BangKanban,
  BangNhiemVu,
  type CotKanban, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
  type DanhMucNhiemVu, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
} from "./so-nhiem-vu";

/**
 * `🗑 Xoá đã chọn` (W4, require a037b76). No DOM here: no click runs. What IS tested — the order and
 * the SEQUENTIAL run with a fake soft-delete (the part a real click would drive), per-task results,
 * what the bar and the checkboxes render in the allowed AND the denied case, and by reading the source
 * that the page wires the run, the refresh and the kept-failed selection.
 */

const REFUSAL = "còn 2 việc con chưa xoá — xử lý hoặc xoá các việc con trước";

/** A selected row: code, parent, and the title + assignee the confirm dialog lists. */
function picked(code: string, parent = ""): SelectedTask {
  return { code, parent, title: `Việc ${code}`, assignee: "" };
}

describe("selection — a new map every time", () => {
  it("tick then untick; the old map is never edited", () => {
    const a = toggleSelected(EMPTY_SELECTION, picked("NV19"));
    expect([...a.keys()]).toEqual(["NV19"]);
    const b = toggleSelected(a, picked("NV19"));
    expect(b.size).toBe(0);
    expect(a.size).toBe(1);
    expect(EMPTY_SELECTION.size).toBe(0);
  });

  it("after a run, ONLY the failed stay selected", () => {
    const sel = [
      picked("NV1"),
      picked("NV2"),
    ].reduce(toggleSelected, EMPTY_SELECTION);
    const kept = keepFailed(sel, [
      { code: "NV1", ok: true },
      { code: "NV2", ok: false, message: REFUSAL },
    ]);
    expect([...kept.keys()]).toEqual(["NV2"]);
  });
});

describe("order — children before parents, within the selection", () => {
  it("a parent selected before its child (and grandchild) goes LAST", () => {
    const order = deleteOrder([
      picked("NV10"),
      picked("NV11", "NV10"),
      picked("NV12", "NV11"),
      picked("NV30"),
    ]);
    expect(order.map((t) => t.code)).toEqual(["NV12", "NV11", "NV10", "NV30"]);
  });

  it("an unselected parent changes nothing; ties keep the order of selection", () => {
    const order = deleteOrder([
      picked("NV2", "NV99"),
      picked("NV1"),
    ]);
    expect(order.map((t) => t.code)).toEqual(["NV2", "NV1"]);
  });

  it("bad data with a cycle terminates", () => {
    expect(
      deleteOrder([
        picked("A", "B"),
        picked("B", "A"),
      ])
        .map((t) => t.code)
        .sort(),
    ).toEqual(["A", "B"]);
  });
});

describe("the run — one call per task, one at a time, never stopping at a refusal", () => {
  it("sequential: never two calls in flight; same reason on every call; results per task", async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    const calls: Array<[string, string]> = [];
    const progress: number[] = [];
    const softDelete = async (code: string, reason: string): Promise<KetQua<void>> => {
      inFlight += 1;
      maxInFlight = Math.max(maxInFlight, inFlight);
      calls.push([code, reason]);
      await new Promise((r) => setTimeout(r, 1));
      inFlight -= 1;
      return code === "NV10" ? { ok: false, thongBao: REFUSAL } : { ok: true, duLieu: undefined };
    };
    const results = await runBatchDelete(
      [
        picked("NV10"),
        picked("NV20"),
        picked("NV30"),
      ],
      "Nhập trùng",
      softDelete,
      (d) => progress.push(d),
    );
    expect(maxInFlight).toBe(1);
    expect(calls).toEqual([
      ["NV10", "Nhập trùng"],
      ["NV20", "Nhập trùng"],
      ["NV30", "Nhập trùng"],
    ]);
    expect(progress).toEqual([1, 2, 3]);
    // The refusal is VERBATIM, and the two after it still ran.
    expect(results).toEqual([
      { code: "NV10", ok: false, message: REFUSAL },
      { code: "NV20", ok: true },
      { code: "NV30", ok: true },
    ]);
  });

  it("the toasts (spec 02 §4): only what the server confirmed; ONE line per distinct refusal, verbatim", () => {
    const results = [
      { code: "A", ok: true as const },
      { code: "B", ok: false as const, message: REFUSAL },
      { code: "C", ok: false as const, message: REFUSAL },
      { code: "D", ok: false as const, message: "x" },
    ];
    expect(batchSuccessToast(results)).toBe("Đã xoá 1 nhiệm vụ.");
    expect(batchSuccessToast([{ code: "B", ok: false, message: "x" }])).toBeNull();
    expect(batchFailureToasts(results)).toEqual([`2 nhiệm vụ không xoá được: ${REFUSAL}`, "1 nhiệm vụ không xoá được: x"]);
    expect(selectedCountLabel(3)).toBe("Đã chọn 3 nhiệm vụ");
    expect(batchDeleteTitle(3)).toBe("Xoá 3 nhiệm vụ khỏi sổ?");
    expect(batchDeleteConfirmLabel(3)).toBe("Xoá 3 nhiệm vụ");
  });
});

describe("the confirm dialog (spec 02 §4)", () => {
  const ROWS = [
    { code: "NV10", title: "Rà soát hộ nghèo", holder: "Cán bộ B" },
    { code: "NV20", title: "Kiểm kê tài sản", holder: "Chưa phân công" },
  ];
  const render = (p: Partial<Parameters<typeof BatchDeleteDialog>[0]> = {}) =>
    renderToStaticMarkup(<BatchDeleteDialog tasks={ROWS} progress={null} onRun={() => {}} onClose={() => {}} {...p} />);

  it("the question, the consequence, every task listed (title + assignee), the MANDATORY reason, Huỷ / red `Xoá N nhiệm vụ`", () => {
    const html = render();
    expect(html).toContain(">Xoá 2 nhiệm vụ khỏi sổ?</h2>");
    expect(html).toContain(BATCH_NOTE);
    expect(html).toContain("Rà soát hộ nghèo</span>");
    expect(html).toContain("Cán bộ B</span>");
    expect(html).toContain("Kiểm kê tài sản</span>");
    expect(html).toMatch(/<input id="ly-do-xoa-da-chon"[^>]*required=""/);
    expect(html.indexOf(">Huỷ</button>")).toBeLessThan(html.indexOf(">Xoá 2 nhiệm vụ</button>"));
    // Presentation pin (ADR 0068 §5): the confirm is solid red, off until the reason has text.
    expect(html).toMatch(/<button class="[^"]*bg-danger[^"]*text-white[^"]*" type="submit" disabled=""><svg[^>]*>.*?<[/]svg>Xoá 2 nhiệm vụ<[/]button>/);
  });

  it("running: progress in the live region, every control off", () => {
    const html = render({ progress: { done: 1, total: 2 } });
    expect(html).toContain(batchProgressText(1, 2));
    expect(html).toMatch(/<input id="ly-do-xoa-da-chon"[^>]*disabled=""/);
    expect(html).toMatch(/<button class="[^"]*" type="button" disabled="">Huỷ<[/]button>/);
  });
});

describe("select all (spec 04 `SelectAllBox`)", () => {
  it("ticks every row shown; all ticked → unticks them; the state says all / some / none", () => {
    const rows = [picked("NV1"), picked("NV2")];
    const some = toggleSelected(EMPTY_SELECTION, rows[0]!);
    expect(selectAllState(some, rows)).toBe("some");
    const all = toggleAllSelected(some, rows);
    expect([...all.keys()].sort()).toEqual(["NV1", "NV2"]);
    expect(selectAllState(all, rows)).toBe("all");
    expect(toggleAllSelected(all, rows).size).toBe(0);
    expect(selectAllState(EMPTY_SELECTION, rows)).toBe("none");
  });
});

function task(code: string, patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code,
    child_count: 0,
    extension_count: 0,
    pending_extension: false,
    allowed_transitions: [],
    updated_at: "2026-06-01T02:00:00Z",
    type: "co-ban",
    bloc: "",
    priority: "",
    title: "Việc giả",
    description: "",
    status: "dang-thuc-hien",
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: "",
    assigner: "",
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
    ...patch,
  };
}

const CATALOGUES: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };
const SELECTION: TaskSelection = {
  selected: toggleSelected(EMPTY_SELECTION, picked("NV19")),
  toggle: () => {},
  disabled: false,
};

function list(selection: TaskSelection | null): string {
  return renderToStaticMarkup(
    <BangNhiemVu
      nhiemVu={[task("NV19"), task("NV20")]}
      danhMuc={CATALOGUES}
      nhanTT={BANG_NHAN_MAC_DINH}
      tenBoPhan={new Map()}
      bayGio={new Date("2026-09-15T03:00:00Z")}
      maDangMo={null}
      moNhiemVu={() => {}}
      sapXep={{ cot: "created_at", chieu: "desc" }}
      doiSapXep={() => {}}
      selection={selection}
    />,
  );
}

function board(selection: TaskSelection | null): string {
  const page = (items: petitions_nhiemVuRa[]): page_Result_petitions_nhiemVuRa => ({
    items,
    next_cursor: "",
    has_more: false,
  });
  const cot: CotKanban[] = TRANG_THAI_CHINH.map((ma) => ({
    ma,
    tai: { pha: "xong" as const, duLieu: page(ma === "dang-thuc-hien" ? [task("NV19"), task("NV20")] : []) },
  }));
  return renderToStaticMarkup(
    <BangKanban
      cot={cot}
      danhMuc={CATALOGUES}
      nhanTT={BANG_NHAN_MAC_DINH}
      bayGio={new Date("2026-09-15T03:00:00Z")}
      maDangMo={null}
      moNhiemVu={() => {}}
      counts={{ pha: "dangTai" }}
      selection={selection}
    />,
  );
}

describe("checkboxes — allowed and DENIED", () => {
  it("list with `task.delete`: a `☐` column, one named checkbox per row, ticked ones checked", () => {
    const html = list(SELECTION);
    expect(html.split('type="checkbox"').length - 1).toBe(2);
    expect(html).toMatch(/<input type="checkbox" class="accent-brand m-0 size-3.5" aria-label="Chọn NV19" checked=""/);
    expect(html).not.toMatch(/aria-label="Chọn NV20" checked=""/);
    expect(html).toContain('<span class="an-thi-giac">Chọn</span>');
  });

  it("with `toggleAll`: the header box (prototype `SelectAllBox`), half-ticked rows read as not all", () => {
    const html = list({ ...SELECTION, toggleAll: () => {} });
    expect(html.split('type="checkbox"').length - 1).toBe(3);
    expect(html).toContain(`aria-label="${SELECT_ALL_LABEL}"`);
    expect(html).not.toMatch(new RegExp(`aria-label="${SELECT_ALL_LABEL}"[^>]*checked=""`));
  });

  it("DENIED — list without `task.delete` (`selection` null): no column, no checkbox", () => {
    const html = list(null);
    expect(html).not.toContain('type="checkbox"');
    expect(html).not.toContain(">Chọn<");
  });

  it("Kanban with `task.delete`: `☐ Chọn` on every card (§4.1); without it: none", () => {
    expect(board(SELECTION).split('type="checkbox"').length - 1).toBe(2);
    expect(board(SELECTION)).toContain('aria-label="Chọn NV20"');
    expect(board(null)).not.toContain('type="checkbox"');
  });

  it("frozen while a batch runs", () => {
    expect(list({ ...SELECTION, disabled: true })).toMatch(/aria-label="Chọn NV19"[^>]*disabled=""/);
  });
});

describe("page wiring (source) and PHAN_CHUA_DUNG", () => {
  const SRC = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");

  it("the run uses the ONE soft-delete call, keeps failed selected, refreshes after; gate is `task.delete`", () => {
    expect(SRC).toContain("await runBatchDelete(tasks, reason, xoaNhiemVu,");
    expect(SRC).toContain("setSelection((s) => keepFailed(s, results));");
    expect(SRC).toContain("datLanTai((n) => n + 1);\n  }");
    expect(SRC).toContain("const taskSelection: TaskSelection | null = quyen.xoa");
    // 06/10/2026 (prototype): `Đã chọn N · Xoá đã chọn` sits in the filter row; the reason — still
    // MANDATORY (rule 7) — is asked in the dialog it opens.
    expect(SRC).toContain("const selectionOn = quyen.xoa && selection.size > 0;");
    expect(SRC).toContain("{quyen.xoa && batchOpen && (");
    expect(SRC).toMatch(/<BatchDeleteDialog\s+tasks=\{\[\.\.\.selection\.values\(\)\]/);
    // The outcome is told by toasts (spec 02 §4), one per distinct refusal.
    expect(SRC).toContain("for (const line of batchFailureToasts(results)) toast.error(line);");
  });

  it("`Xoá đã chọn` left PHAN_CHUA_DUNG; no Excel entry remains either", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W4): the entry named three parts; one is built.
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W6): the tracking-book export is built too.
    const ten = PHAN_CHUA_DUNG.map((p) => p.ten).join(" | ");
    expect(ten).not.toContain("Xoá đã chọn");
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W7): the Excel import is built — the entry is gone.
    expect(ten).not.toContain("Nhập từ Excel");
    expect(ten).not.toContain("Xuất Sổ theo dõi");
  });
});
