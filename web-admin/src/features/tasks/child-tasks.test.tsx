import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import type { ReactElement, ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/session/current-session";
import type { KetQua } from "@/lib/api/request";
import { duongDanSoNhiemVu, taskCountsPath } from "@/lib/api/tasks";
import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import { parseDrillDown } from "@/lib/drill-down";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
  QUYEN_TAO_NHIEM_VU,
} from "@/lib/permissions";

import { ChildTaskList, ParentTaskField, childTasksQuery } from "./child-tasks";
import {
  ADD_CHILD_BUTTON,
  BANG_NHAN_MAC_DINH,
  CHILD_TASKS_EMPTY,
  CHILD_TASKS_LOADING,
  CHILD_TASKS_TITLE,
  DETACH_PARENT_BODY,
  KANBAN_COUNTS_ERROR,
  NO_DEADLINE_LAST_NOTE,
  PARENT_DETACH_BUTTON,
  PARENT_NONE,
  PARENT_SAVE_BUTTON,
  childCountLabel,
  childFormNote,
  mergeChildPages,
  parentPatchBody,
  quyenNhiemVu,
  thanGiaoViec,
} from "./task-labels";
import { BangNhiemVu, ChiTietNhiemVu, FormGiaoViec, SoNhiemVu } from "./tasks-screen";
import type { DanhMucNhiemVu } from "./tasks-screen"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)

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
    lead_unit: "",
    monitor: "",
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

describe("(#6) chip `{n} việc con` on list rows", () => {
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

  it("shown only when `child_count > 0`, with the server's number", () => {
    expect(list(2)).toContain('<span class="chip chip-ngung">2 việc con</span>');
    expect(list(0)).not.toContain("việc con");
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
        labels={BANG_NHAN_MAC_DINH}
        directory={null}
        now={NOW}
        openTask={() => {}}
        loadingMore={false}
        moreError={null}
        loadMore={() => {}}
      />,
    );
  }

  it("each child: code, title, status, assignee, deadline — and a button opening its drawer", () => {
    const out = block({
      phase: "done",
      rows: [
        task({ code: "NV25", title: "Khảo sát thôn 3", status: "moi-giao", assignee: "CB-2026-3H8N2W" }),
      ],
      more: false,
    });
    expect(out).toContain(asHtml(CHILD_TASKS_TITLE));
    expect(out).toContain('aria-label="Mở NV25: Khảo sát thôn 3"');
    expect(out).toContain("Mới giao");
    expect(out).toContain("CB-2026-3H8N2W");
    expect(out).toContain("Hạn 20/6/2026");
    expect(out).toContain("(trễ 86 ngày)");
    expect(out).not.toContain("Xem thêm");
  });

  it("`has_more` gives `Xem thêm`; pages merge by code without duplicates", () => {
    expect(block({ phase: "done", rows: [task({ code: "NV25" })], more: true })).toContain(
      "Xem thêm",
    );
    const merged = mergeChildPages([task({ code: "NV25" })], [task({ code: "NV25" }), task({ code: "NV26" })]);
    expect(merged.map((t) => t.code)).toEqual(["NV25", "NV26"]);
  });

  it("three phases, three screens: loading, verbatim error (never `no children`), empty", () => {
    expect(block({ phase: "loading" })).toContain(asHtml(CHILD_TASKS_LOADING));
    const err = block({ phase: "error", message: TREE_REFUSAL });
    expect(err).toContain(`role="alert">${asHtml(TREE_REFUSAL)}</p>`);
    expect(err).not.toContain(asHtml(CHILD_TASKS_EMPTY));
    expect(block({ phase: "done", rows: [], more: false })).toContain(asHtml(CHILD_TASKS_EMPTY));
  });
});

describe("(#10) `Việc cha` field — set and detach, server's 409 verbatim", () => {
  function field(parent: string, canEdit: boolean): string {
    return html(
      <ParentTaskField
        code="NV25"
        parent={parent}
        canEdit={canEdit}
        save={NOT_CALLED}
        openParent={NOT_CALLED}
      />,
    );
  }

  it("with `task.update`: a code box and a save button; detach only when there IS a parent", () => {
    const root = field("", true);
    expect(root).toContain(asHtml(PARENT_NONE));
    expect(root).toContain(PARENT_SAVE_BUTTON);
    expect(root).not.toContain(PARENT_DETACH_BUTTON);
    const child = field("NV19", true);
    expect(child).toContain("Mở việc cha NV19");
    expect(child).toContain(PARENT_DETACH_BUTTON);
  });

  it("DENIED: without `task.update` the parent is shown, nothing to change it", () => {
    const out = field("NV19", false);
    expect(out).toContain("Mở việc cha NV19");
    expect(out).not.toContain(PARENT_SAVE_BUTTON);
    expect(out).not.toContain(PARENT_DETACH_BUTTON);
    expect(out).not.toContain("<input");
  });

  it("bodies: set = trimmed register code; unchanged/empty = nothing to send; detach = `\"\"`", () => {
    expect(parentPatchBody("  NV19 ", "")).toEqual({ parent: "NV19" });
    expect(parentPatchBody("NV19", "NV19")).toBeNull();
    expect(parentPatchBody("   ", "NV19")).toBeNull();
    // Not case-folded: the register code is whatever the commune issued.
    expect(parentPatchBody("nv19", "")).toEqual({ parent: "nv19" });
    expect(DETACH_PARENT_BODY).toEqual({ parent: "" });
  });

  it("the 409 sentence reaches the field unchanged (source wiring: `setError(result.thongBao)`)", () => {
    const src = readFileSync(fileURLToPath(new URL("./child-tasks.tsx", import.meta.url)), "utf8");
    expect(src).toContain("setError(result.thongBao);");
    expect(src).toMatch(/role="alert">\s*\{error\}/);
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
        coQuanChuTri: "",
        chuyenVien: "",
        han: "",
        vanBan: [],
        ghiChu: "",
      },
      { coDanhSachVanBan: true, maCha: "NV19" },
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
    const q = quyenNhiemVu(keys);
    return html(
      <ChiTietNhiemVu
        nhiemVu={task()}
        vanBan={{ pha: "dangTai" }}
        danhMuc={CATALOGUES}
        nhanTT={BANG_NHAN_MAC_DINH}
        tenBoPhan={new Map()}
        bayGio={NOW}
        maNguoiDangNhap=""
        quyen={q}
        dangGui={false}
        loiGhi={null}
        dong={() => {}}
        doiTrangThai={() => {}}
        xoa={() => {}}
        guiDeNghiLuiHan={() => Promise.resolve({ ok: false, thongBao: "" })}
        quyetDinh={() => Promise.resolve({ ok: false, thongBao: "" })}
        suaKhoiVanBan={NOT_CALLED}
        docLaiChiTiet={NOT_CALLED}
        extensionRefreshKey="0"
        onExtensionDecided={() => {}}
        openTask={() => {}}
        openTaskByCode={NOT_CALLED}
        saveParent={NOT_CALLED}
        reassign={NOT_CALLED}
        // What `SoNhiemVu` passes: `null` without `task.create` (see its `addChild` prop).
        addChild={
          q.giaoViec
            ? { open, toggle: () => {}, form: <p>FORM-SLOT</p>, created: null }
            : null
        }
      />,
    );
  }

  it("with `task.create`: the button in the drawer; open → the form slot renders there", () => {
    expect(drawer([QUYEN_TAO_NHIEM_VU], false)).toContain(asHtml(ADD_CHILD_BUTTON));
    expect(drawer([QUYEN_TAO_NHIEM_VU], false)).not.toContain("FORM-SLOT");
    expect(drawer([QUYEN_TAO_NHIEM_VU], true)).toContain("FORM-SLOT");
  });

  it("DENIED: `task.update` + `task.extend` without `task.create` → no button", () => {
    const out = drawer([QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_GIA_HAN], false);
    expect(out).not.toContain(asHtml(ADD_CHILD_BUTTON));
    // The block itself stays — reading children needs no write key.
    expect(out).toContain(asHtml(CHILD_TASKS_TITLE));
  });

  it("`SoNhiemVu` gates the button on `quyen.giaoViec` — the key of `+ Giao việc mới`", () => {
    const src = readFileSync(fileURLToPath(new URL("./tasks-screen.tsx", import.meta.url)), "utf8");
    expect(src).toMatch(/addChild=\{[\s\S]*?quyen\.giaoViec\s*\?/);
    expect(src).toContain("maChaCoSan={drawer.nhiemVu.code}");
  });
});

describe("(#13) `Hạn` sortable — `sort=due_at`, with the note about tasks without a deadline", () => {
  it("the list sends `sort=due_at&order=asc|desc`", () => {
    expect(duongDanSoNhiemVu({ sapXep: "due_at", chieu: "asc", limit: 20 })).toBe(
      "/api/v1/tasks?sort=due_at&order=asc&limit=20",
    );
    expect(duongDanSoNhiemVu({ sapXep: "due_at", chieu: "desc" })).toBe(
      "/api/v1/tasks?sort=due_at&order=desc",
    );
  });

  it("the header is a sort button next to Mã and Ngày giao", () => {
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
    expect(out.split('class="nut-sap-xep"').length - 1).toBe(5);
    expect(out).toContain('aria-sort="ascending"><button type="button" class="nut-sap-xep">Hạn ↑</button>');
  });

  it("the no-deadline note is on the list view", () => {
    const src = readFileSync(fileURLToPath(new URL("./tasks-screen.tsx", import.meta.url)), "utf8");
    expect(src).toContain("<p className=\"ghi-chu\">{NO_DEADLINE_LAST_NOTE}</p>");
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

  it("`SoNhiemVu` counts with the board's own `loc`, inside the Kanban effect only", () => {
    const src = readFileSync(fileURLToPath(new URL("./tasks-screen.tsx", import.meta.url)), "utf8");
    expect(src).toContain("getTaskCounts(loc).then(");
    const effect = src.slice(src.indexOf('if (!daDocDuongDan || viewMode !== "kanban") return;'));
    expect(effect.indexOf("getTaskCounts(loc)")).toBeGreaterThan(0);
    expect(effect.indexOf("getTaskCounts(loc)")).toBeLessThan(effect.indexOf("}, [loc, khoaKanban"));
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
    const src = readFileSync(fileURLToPath(new URL("./tasks-screen.tsx", import.meta.url)), "utf8");
    expect(src).toContain('const viewMode: CheDoXem = drillDownActive ? "danh-sach" : cheDoXem;');
    expect(src).toContain(
      "? { ...locDuongDan, sapXep: locDaDoi?.sapXep, chieu: locDaDoi?.chieu }",
    );
  });
});
