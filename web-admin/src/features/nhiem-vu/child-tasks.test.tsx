import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import type { ReactElement, ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import { duongDanSoNhiemVu, taskCountsPath } from "@/lib/api/nhiem-vu";
import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import { parseDrillDown } from "@/lib/drill-down";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
  QUYEN_TAO_NHIEM_VU,
} from "@/lib/quyen";

import { ChildTaskList, childTasksQuery, childTickLabel } from "./child-tasks";
import {
  ADD_CHILD_BUTTON,
  BANG_NHAN_MAC_DINH,
  CHILD_TASK_WEIGHT_NONE,
  childTasksHeading,
  CHILD_TASKS_EMPTY,
  CHILD_TASKS_LOADING,
  CHILD_TASKS_TITLE,
  DETACH_PARENT_BODY,
  KANBAN_COUNTS_ERROR,
  NO_DEADLINE_LAST_NOTE,
  PARENT_NONE,
  PARENT_TITLE,
  childCountLabel,
  childFormNote,
  mergeChildPages,
  parentPatchBody,
  quyenNhiemVu,
  thanGiaoViec,
} from "./nhan-nhiem-vu";
import { BangNhiemVu, ChiTietNhiemVu, FormGiaoViec, SoNhiemVu } from "./so-nhiem-vu";
import type { DanhMucNhiemVu } from "./so-nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import { NOT_SENT } from "./task-transitions.fixture";

/**
 * TASK-03 web pass 2 (28/09/2026): #6 children, #10 add child / move parent, #13 due-date sort,
 * #15 real Kanban counts — the parts that are not the extension block (that one lives in
 * `so-nhiem-vu.test.tsx`, next to the ADR 0038 group it extends).
 *
 * THE LIMIT, STATED: no DOM (`vitest.config.mts`), so no case clicks `Thêm việc con`, types a
 * parent code or waits for an effect. What IS tested: the queries the effects send, what each
 * phase renders, the permission gate in both directions, the request bodies, and — by reading the
 * source, the `noi-nhan-trang-thai.test.ts` precedent — the wiring those effects use.
 */

function html(node: ReactElement): string {
  return renderToStaticMarkup(node);
}

/** As the string really sits in the HTML — `renderToStaticMarkup` escapes `"` and `&`. */
function asHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

function task(patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    child_count: 0,
    extension_count: 0,
    pending_extension: false,
    allowed_transitions: [],
    updated_at: "2026-06-01T02:00:00Z",
    type: "co-ban",
    bloc: "",
    priority: "",
    title: "Rà soát hộ nghèo",
    description: "",
    status: "dang-thuc-hien",
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: "",
    assigner: "",
    due_at: "2026-06-20T23:59:59+07:00",
    original_due_at: "2026-06-20T23:59:59+07:00",
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
const NOW = new Date("2026-09-15T03:00:00Z");
const NOT_CALLED = (): Promise<KetQua<petitions_nhiemVuRa>> =>
  Promise.resolve({ ok: false, thongBao: "not called in tests" });
const TREE_REFUSAL = "Việc cha NV99 không tồn tại trong sổ của xã.";

describe("(#6) `{n} việc con` — on the Kanban card only (spec 04: the list's second line is the source)", () => {
  function list(childCount: number): string {
    return html(
      <BangNhiemVu
        nhiemVu={[task({ child_count: childCount })]}
        danhMuc={CATALOGUES}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={new Map()}
        bayGio={NOW}
        maDangMo={null}
        moNhiemVu={() => {}}
        sapXep={{ cot: "created_at", chieu: "desc" }}
        doiSapXep={() => {}}
      />,
    );
  }

  it("the list's second line is the source label alone; the count is the card's (`childCountLabel`)", () => {
    // ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (spec 04): `nguồn · đã gia hạn n lần` — no child count in the list.
    // `extension_count` 0 ⇒ the source alone (prototype `TaskListTable.tsx:86-88`).
    expect(list(2)).toContain('<span class="text-ink-muted block truncate text-[11px]">Giao trực tiếp</span>');
    expect(list(2)).not.toContain("việc con");
    expect(childCountLabel(2)).toBe("2 việc con");
    expect(childCountLabel(0)).toBeNull();
    expect(childCountLabel(-1)).toBeNull();
  });
});

describe("(#6) the drawer's `Nhiệm vụ con` block", () => {
  it("reads `GET /api/v1/tasks?parent=NV19` — only `parent`, no register filter leaks in", () => {
    expect(duongDanSoNhiemVu(childTasksQuery("NV19", null))).toBe(
      "/api/v1/tasks?parent=NV19&limit=20",
    );
    expect(duongDanSoNhiemVu(childTasksQuery("NV19", "c2"))).toBe(
      "/api/v1/tasks?parent=NV19&limit=20&cursor=c2",
    );
  });

  function block(load: Parameters<typeof ChildTaskList>[0]["load"]): string {
    return html(
      <ChildTaskList
        load={load}
        openTask={() => {}}
        loadingMore={false}
        moreError={null}
        loadMore={() => {}}
      />,
    );
  }

  it("each child (spec 07 §6d): its title — a button opening its drawer — and the weight; done = struck", () => {
    const out = block({
      phase: "done",
      rows: [
        task({ code: "NV25", title: "Khảo sát thôn 3", status: "moi-giao", assignee: "CB-2026-3H8N2W" }),
        task({ code: "NV26", title: "Tổng hợp phiếu", status: "hoan-thanh" }),
      ],
      more: false,
    });
    expect(out).toContain(asHtml(childTasksHeading(1, 2)));
    expect(out).toContain('aria-label="Mở NV25: Khảo sát thôn 3"');
    expect(out).toMatch(/text-ink-muted line-through"[^>]*>Tổng hợp phiếu<\/button>/);
    // Presentation pin (ADR 0068 §5): the prototype's bordered row, no status chip / assignee / deadline.
    expect(out).toContain('class="border-line mb-2 flex items-start gap-2.5 rounded-[9px] border bg-white px-3 py-2.5"');
    expect(out).not.toContain("Chưa thực hiện");
    expect(out).not.toContain("CB-2026-3H8N2W");
    expect(out).not.toContain("Xem thêm");
  });

  it("prototype tick + weight per child: DISABLED, one `?` marker, the count only when every page is in", () => {
    // User decision 07/10/2026: drawn at the prototype's place, never working — the server stores no
    // weight, and completing a child is that child's own status move (ADR 0068 §14).
    const rows = [
      task({ code: "NV25", status: "hoan-thanh" }),
      task({ code: "NV26", status: "dang-thuc-hien" }),
    ];
    const out = block({ phase: "done", rows, more: false });
    const ticks = out.match(/<input[^>]*type="checkbox"[^>]*>/g) ?? [];
    expect(ticks).toHaveLength(2);
    expect(ticks.every((t) => t.includes('disabled=""'))).toBe(true);
    expect(ticks[0]).toContain('checked=""');
    expect(ticks[1]).not.toContain('checked=""');
    expect(out).toContain(`aria-label="${childTickLabel("NV25")}"`);
    expect(out.split("data-pending-marker").length - 1).toBe(1);
    expect(out).toContain(`(${CHILD_TASK_WEIGHT_NONE})`);
    expect(out).toContain(asHtml(childTasksHeading(1, 2)));
    // A partial list carries no count — a figure over the first page nobody can check.
    expect(block({ phase: "done", rows, more: true })).not.toContain("hoàn thành)");
  });

  it("`has_more` gives `Xem thêm`; pages merge by code without duplicates", () => {
    expect(block({ phase: "done", rows: [task({ code: "NV25" })], more: true })).toContain(
      "Xem thêm",
    );
    const merged = mergeChildPages([task({ code: "NV25" })], [task({ code: "NV25" }), task({ code: "NV26" })]);
    expect(merged.map((t) => t.code)).toEqual(["NV25", "NV26"]);
  });

  it("drawn only when there ARE children (spec 07 §6d); a failed read is the server's sentence, never `no children`", () => {
    expect(block({ phase: "loading" })).toBe("");
    expect(block({ phase: "done", rows: [], more: false })).toBe("");
    const err = block({ phase: "error", message: TREE_REFUSAL });
    expect(err).toContain(`role="alert">${asHtml(TREE_REFUSAL)}</p>`);
    expect(err).toContain(asHtml(CHILD_TASKS_TITLE));
    expect(err).not.toContain(asHtml(CHILD_TASKS_EMPTY));
    expect(err).not.toContain(asHtml(CHILD_TASKS_LOADING));
  });
});

describe("(#10) `Việc cha` — REMOVED from the drawer (owner 07/10/2026, ADR 0076 lần 2 #6)", () => {
  it("the parent-body helpers stay for the API client: set = trimmed register code; detach = `\"\"`", () => {
    expect(parentPatchBody("  NV19 ", "")).toEqual({ parent: "NV19" });
    expect(parentPatchBody("NV19", "NV19")).toBeNull();
    expect(parentPatchBody("   ", "NV19")).toBeNull();
    // Not case-folded: the register code is whatever the commune issued.
    expect(parentPatchBody("nv19", "")).toEqual({ parent: "nv19" });
    expect(DETACH_PARENT_BODY).toEqual({ parent: "" });
  });

  it("no parent field, no parent link, no `ParentTaskField` anywhere in the screen", () => {
    const out = drawerFor(task({ parent: "NV19" }), [QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_TAO_NHIEM_VU]);
    expect(out).not.toContain(asHtml(PARENT_TITLE));
    expect(out).not.toContain(asHtml(PARENT_NONE));
    expect(out).not.toContain("Mở việc cha");
    const src = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(src).not.toContain("ParentTaskField");
    expect(src).not.toContain("saveParent");
    const child = readFileSync(fileURLToPath(new URL("./child-tasks.tsx", import.meta.url)), "utf8");
    expect(child).not.toContain("export function ParentTaskField");
  });
});

describe("(#10) `+ Thêm việc con` — the create form, prefilled with the parent's code", () => {
  const DIRECTORY = { ok: true as const, duLieu: { items: [] } };

  it("the form says whose child it is, and the body carries `parent` = that register code", () => {
    const out = html(
      <FormGiaoViec
        danhMuc={CATALOGUES}
        danhBa={DIRECTORY}
        danhBaLanhDao={DIRECTORY}
        maChaCoSan="NV19"
        dangGui={false}
        loi={null}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(out).toContain(asHtml(childFormNote("NV19")));
    const body = thanGiaoViec(
      {
        tuSinhMa: true,
        ma: "",
        loai: "co-ban",
        khoi: "",
        tieuDe: "Khảo sát thôn 3",
        moTa: "",
        mucUuTien: "",
        boPhan: "",
        nguoiThucHien: "",
        lanhDaoGiaoViec: "",
        han: "",
        dueTime: "",
        vanBan: [],
        ghiChu: "",
      },
      { coDanhSachVanBan: true, directive: false, maCha: "NV19" },
    );
    expect(body.parent).toBe("NV19");
  });

  it("a server refusal shows VERBATIM in the form (`loi`)", () => {
    const out = html(
      <FormGiaoViec
        danhMuc={CATALOGUES}
        danhBa={DIRECTORY}
        danhBaLanhDao={DIRECTORY}
        maChaCoSan="NV99"
        dangGui={false}
        loi={TREE_REFUSAL}
        huy={() => {}}
        giaoViec={() => {}}
      />,
    );
    expect(out).toContain(`role="alert">${asHtml(TREE_REFUSAL)}</p>`);
  });

  function drawer(keys: readonly string[], open: boolean): string {
    return drawerFor(task(), keys, open);
  }

  it("with `task.create`: the button in the drawer; open → the form slot renders there", () => {
    expect(drawer([QUYEN_TAO_NHIEM_VU], false)).toContain(">Thêm việc con</button>");
    expect(ADD_CHILD_BUTTON).toBe("+ Thêm việc con");
    expect(drawer([QUYEN_TAO_NHIEM_VU], false)).not.toContain("FORM-SLOT");
    expect(drawer([QUYEN_TAO_NHIEM_VU], true)).toContain("FORM-SLOT");
  });

  it("DENIED: `task.update` + `task.extend` without `task.create` → no button", () => {
    const out = drawer([QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_GIA_HAN], false);
    expect(out).not.toContain(">Thêm việc con</button>");
  });

  it("`SoNhiemVu` gates the button on `quyen.giaoViec` — the key of `+ Giao việc mới`", () => {
    const src = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(src).toMatch(/addChild=\{[\s\S]*?quyen\.giaoViec\s*\?/);
    expect(src).toContain("maChaCoSan={drawer.nhiemVu.code}");
  });
});

function drawerFor(t: petitions_nhiemVuRa, keys: readonly string[], open = false): string {
  const q = quyenNhiemVu(keys);
  return html(
      <ChiTietNhiemVu
        nhiemVu={t}
        vanBan={{ pha: "dangTai" }}
        danhMuc={CATALOGUES}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={new Map()}
        bayGio={NOW}
        maNguoiDangNhap=""
        quyen={q}
        dangGui={false}
        dong={() => {}}
        doiTrangThai={NOT_SENT}
        xoa={NOT_SENT}
        guiDeNghiLuiHan={() => Promise.resolve({ ok: false, thongBao: "" })}
        quyetDinh={() => Promise.resolve({ ok: false, thongBao: "" })}
        suaKhoiVanBan={NOT_CALLED}
        docLaiChiTiet={NOT_CALLED}
        extensionRefreshKey="0"
        onExtensionDecided={() => {}}
        openTask={() => {}}
        reassign={NOT_CALLED}
        // What `SoNhiemVu` passes: `null` without `task.create` (see its `addChild` prop).
        addChild={q.giaoViec ? { open, toggle: () => {}, form: <p>FORM-SLOT</p> } : null}
      />,
  );
}

describe("(#13) `Hạn` sortable — `sort=due_at`, with the note about tasks without a deadline", () => {
  it("the list sends `sort=due_at&order=asc|desc`", () => {
    expect(duongDanSoNhiemVu({ sapXep: "due_at", chieu: "asc", limit: 20 })).toBe(
      "/api/v1/tasks?sort=due_at&order=asc&limit=20",
    );
    expect(duongDanSoNhiemVu({ sapXep: "due_at", chieu: "desc" })).toBe(
      "/api/v1/tasks?sort=due_at&order=desc",
    );
  });

  it("the header is a sort button next to Mã, Tên việc and Ưu tiên", () => {
    const out = html(
      <BangNhiemVu
        nhiemVu={[task()]}
        danhMuc={CATALOGUES}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={new Map()}
        bayGio={NOW}
        maDangMo={null}
        moNhiemVu={() => {}}
        sapXep={{ cot: "due_at", chieu: "asc" }}
        doiSapXep={() => {}}
      />,
    );
    // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3b): 3 → 5 — `Tên việc` and `Ưu tiên` sort too (backend P9).
    // 06/10/2026 (prototype columns): 5 → 4 — the `Ngày giao` column is gone.
    // 07/10/2026 (spec 04): the label + `ArrowUpDown`; the active column's icon at full opacity.
    // 08/10/2026 (ADR 0082 #5): 4 → 5 — `Trạng thái` sorts on the server (`sort=status`).
    expect(out.split("aria-sort=").length - 1).toBe(5);
    expect(out).toMatch(/aria-sort="ascending"[^>]*><button type="button"[^>]*>Hạn<svg[^>]*lucide-arrow-up-down[^>]*opacity-100/);
  });

  it("ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026: the sort notes are gone from the list (spec 04 has none); the rule stays the server's", () => {
    const src = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(src).not.toContain("{NO_DEADLINE_LAST_NOTE}");
    expect(NO_DEADLINE_LAST_NOTE).toContain("không có hạn luôn nằm cuối");
  });
});

describe("(#15) Kanban counts use the SAME filter builder as the list", () => {
  const FULL = {
    phamVi: "mine" as const,
    trangThai: "cho-duyet",
    nguonGiao: "phan-anh",
    loai: "co-ban",
    khoi: "khoi-dang",
    mucUuTien: "cao",
    boPhanID: "01JBOPHAN",
    nguoiThucHienMa: "CB-2026-3H8N2W",
    tim: "hộ nghèo",
    chiTreHan: true,
    parent: "NV19",
  };

  it("same filter params, in the same order; sort and page never reach the counts route", () => {
    const list = new URL(
      duongDanSoNhiemVu({ ...FULL, sapXep: "code", chieu: "asc", limit: 20, cursor: "c1" }),
      "https://xa.example",
    ).searchParams;
    const counts = new URL(
      taskCountsPath({ ...FULL, sapXep: "code", chieu: "asc", limit: 20, cursor: "c1" }),
      "https://xa.example",
    ).searchParams;
    for (const k of ["sort", "order", "limit", "cursor"]) list.delete(k);
    expect([...counts.entries()]).toEqual([...list.entries()]);
    expect(taskCountsPath()).toBe("/api/v1/task-counts");
  });

  it("the Tổng quan metric and its period travel to the counts too — one filter, one builder", () => {
    const loc = { metric: "completed" as const, from: "2026-09-01T00:00:00+07:00", to: "2026-10-01T00:00:00+07:00" };
    expect(new URL(taskCountsPath(loc), "https://xa.example").searchParams.get("metric")).toBe(
      "completed",
    );
    expect(taskCountsPath(loc).replace("/api/v1/task-counts", "")).toBe(
      duongDanSoNhiemVu(loc).replace("/api/v1/tasks", ""),
    );
  });

  it("`SoNhiemVu` counts with the VIEW's own query — the board's `loc` on Kanban, the forced type on Sổ theo dõi", () => {
    const src = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(src).toContain("getTaskCounts(viewLoc).then(");
    expect(src).toContain('viewMode === "so-theo-doi" && typeof directiveType === "string"');
    expect(src).toContain("? { ...scoped, loai: directiveType }");
    const effect = src.slice(src.indexOf("getTaskCounts(viewLoc).then("));
    expect(effect.indexOf("}, [viewLoc, khoaCounts, daDocDuongDan, registerHeld]);")).toBeGreaterThan(0);
  });
});

describe("drill-down from /tong-quan (ADR 0053 §7) — unchanged", () => {
  const render = (node: ReactNode) => renderToStaticMarkup(<PhienProvider>{node}</PhienProvider>);

  it("forced list view: no board, no column counts, no counts error", () => {
    const out = render(<SoNhiemVu drillDown={parseDrillDown("tasks", { metric: "suspended" })} />);
    expect(out).not.toContain('aria-label="Bảng Kanban nhiệm vụ"');
    expect(out).not.toContain("cot-kanban-");
    expect(out).not.toContain(KANBAN_COUNTS_ERROR);
  });

  it("the drill-down still overrides the screen's filters (source wiring kept verbatim)", () => {
    const src = readFileSync(fileURLToPath(new URL("./so-nhiem-vu.tsx", import.meta.url)), "utf8");
    expect(src).toContain('const viewMode: CheDoXem = drillDownActive ? "danh-sach" : cheDoXem;');
    expect(src).toContain(
      "? { ...locDuongDan, sapXep: locDaDoi?.sapXep, chieu: locDaDoi?.chieu }",
    );
  });
});
