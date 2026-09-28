import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { page_Result_petitions_nhiemVuRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen";

import { BatchDeleteBar } from "./batch-delete-bar";
import {
  BATCH_DELETE_BUTTON,
  EMPTY_SELECTION,
  batchProgressText,
  batchResultLine,
  batchSummary,
  deleteOrder,
  keepFailed,
  runBatchDelete,
  selectedCountLabel,
  toggleSelected,
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

describe("selection — a new map every time", () => {
  it("tick then untick; the old map is never edited", () => {
    const a = toggleSelected(EMPTY_SELECTION, { code: "NV19", parent: "" });
    expect([...a.keys()]).toEqual(["NV19"]);
    const b = toggleSelected(a, { code: "NV19", parent: "" });
    expect(b.size).toBe(0);
    expect(a.size).toBe(1);
    expect(EMPTY_SELECTION.size).toBe(0);
  });

  it("after a run, ONLY the failed stay selected", () => {
    const sel = [
      { code: "NV1", parent: "" },
      { code: "NV2", parent: "" },
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
      { code: "NV10", parent: "" },
      { code: "NV11", parent: "NV10" },
      { code: "NV12", parent: "NV11" },
      { code: "NV30", parent: "" },
    ]);
    expect(order.map((t) => t.code)).toEqual(["NV12", "NV11", "NV10", "NV30"]);
  });

  it("an unselected parent changes nothing; ties keep the order of selection", () => {
    const order = deleteOrder([
      { code: "NV2", parent: "NV99" },
      { code: "NV1", parent: "" },
    ]);
    expect(order.map((t) => t.code)).toEqual(["NV2", "NV1"]);
  });

  it("bad data with a cycle terminates", () => {
    expect(
      deleteOrder([
        { code: "A", parent: "B" },
        { code: "B", parent: "A" },
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
        { code: "NV10", parent: "" },
        { code: "NV20", parent: "" },
        { code: "NV30", parent: "" },
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

  it("the words: per-task line with the server's sentence; summary never claims more than happened", () => {
    expect(batchResultLine({ code: "NV10", ok: false, message: REFUSAL })).toBe(`NV10 — chưa xoá: ${REFUSAL}`);
    expect(batchResultLine({ code: "NV20", ok: true })).toBe("NV20 — đã xoá.");
    expect(batchSummary([{ code: "A", ok: true }])).toBe("Đã xoá 1/1 nhiệm vụ.");
    expect(
      batchSummary([
        { code: "A", ok: true },
        { code: "B", ok: false, message: "x" },
      ]),
    ).toContain("Đã xoá 1/2 nhiệm vụ; 1 việc chưa xoá được");
    expect(selectedCountLabel(3)).toBe("Đã chọn 3 nhiệm vụ");
  });
});

describe("the bar", () => {
  const render = (p: Partial<Parameters<typeof BatchDeleteBar>[0]> = {}) =>
    renderToStaticMarkup(
      <BatchDeleteBar count={2} progress={null} results={null} onRun={() => {}} onClear={() => {}} {...p} />,
    );

  it("`Đã chọn {N} nhiệm vụ` + the red button, disabled while the shared reason is empty", () => {
    const html = render();
    expect(html).toContain(">Đã chọn 2 nhiệm vụ</h3>");
    expect(html).toMatch(/<input id="ly-do-xoa-da-chon"[^>]*required=""/);
    expect(html).toContain(`<button type="submit" class="nut-xoa" disabled="">${BATCH_DELETE_BUTTON}</button>`);
  });

  it("running: progress in the live region, every control off", () => {
    const html = render({ progress: { done: 1, total: 2 } });
    expect(html).toContain(batchProgressText(1, 2));
    expect(html).toMatch(/<input id="ly-do-xoa-da-chon"[^>]*disabled=""/);
  });

  it("after a run: one line per task, the refused one marked as an error, shown even with nothing left selected", () => {
    const html = render({
      count: 0,
      results: [
        { code: "NV10", ok: false, message: REFUSAL },
        { code: "NV20", ok: true },
      ],
    });
    expect(html).toContain(`<li class="thong-bao-loi">NV10 — chưa xoá: ${REFUSAL}</li>`);
    expect(html).toContain("<li>NV20 — đã xoá.</li>");
    expect(html).not.toContain('id="ly-do-xoa-da-chon"');
  });
});

function task(code: string, patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code,
    child_count: 0,
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
    lead_unit: "",
    monitor: "",
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
  selected: toggleSelected(EMPTY_SELECTION, { code: "NV19", parent: "" }),
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
    expect(html).toMatch(/<input type="checkbox" aria-label="Chọn NV19" checked=""/);
    expect(html).not.toMatch(/aria-label="Chọn NV20" checked=""/);
    expect(html).toContain('<span class="an-thi-giac">Chọn</span>');
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
    expect(SRC).toContain(
      "{quyen.xoa && (selection.size > 0 || batchResults !== null || batchProgress !== null) && (",
    );
  });

  it("`Xoá đã chọn` left PHAN_CHUA_DUNG; only the Excel import stays", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W4): the entry named three parts; one is built.
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W6): the tracking-book export is built too.
    const ten = PHAN_CHUA_DUNG.map((p) => p.ten).join(" | ");
    expect(ten).not.toContain("Xoá đã chọn");
    expect(ten).toContain("Nhập từ Excel");
    expect(ten).not.toContain("Xuất Sổ theo dõi");
  });
});
