"use client";

import { ExternalLink } from "lucide-react";
import { useCallback, useEffect, useMemo, useReducer, useRef, useState, type ReactNode } from "react";

import { useDetailCatalogues } from "@/features/leader-notebook/leader-notebook";
import {
  CHUA_PHAN_CONG,
  danhBaChoNhatKy, // vi-name-ok: existing reader of the register, imported not declared
  docBangNhanTrangThai, // vi-name-ok: existing reader of the register, imported not declared
  nhanCanBoNgan, // vi-name-ok: existing formatter of the register, imported not declared
  nhanNgay, // vi-name-ok: existing formatter of the register, imported not declared
  quyenNhiemVu, // vi-name-ok: existing permission reader of the register, imported not declared
} from "@/features/nhiem-vu/nhan-nhiem-vu";
import { chuyenDrawer, type ViecDrawer } from "@/features/nhiem-vu/so-nhiem-vu"; // vi-name-ok: existing drawer reducer and type of the register
import { TaskDetailHost, useTaskDetailRead, useTaskDetailState } from "@/features/nhiem-vu/task-detail-host";
import { withSpecLabels } from "@/features/nhiem-vu/task-spec";
import { usePhien } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing session hook
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi"; // vi-name-ok: existing staff-directory client
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type
import { getTaskCounts, layNhiemVu, laySoNhiemVu } from "@/lib/api/nhiem-vu"; // vi-name-ok: existing task clients
import type {
  identity_danhBaChonNguoiRa, // vi-name-ok: generated contract type
  petitions_danhSachTrangThaiNhiemVuRa, // vi-name-ok: generated contract type
  petitions_nhiemVuRa, // vi-name-ok: generated contract type
} from "@/lib/api/schema.gen";
import { layTrangThaiNhiemVu } from "@/lib/api/trang-thai-nhiem-vu"; // vi-name-ok: existing status-label client
import { coQuyen, QUYEN_XEM_NHIEM_VU } from "@/lib/quyen";

import {
  DRILL_LOADING,
  DRILL_OPEN,
  DRILL_OPEN_CLASS,
  DRILL_PAGE_SIZE,
  drillDescriptionOf,
  DrillTableDialog,
  type DrillBody,
} from "./drill-dialog";
import { RecordDetail, type RecordDetailTarget } from "./record-detail";
import { RecordDrillDialog } from "./record-drill";
import type { DashboardDrill, DrillRequest, TaskDrillRequest } from "./view";

/**
 * Tổng quan's IN-PLACE task views (customer sheet rows 3 and 5, 10/10/2026):
 *   · a task figure opens THE ROWS BEHIND IT in a dialog — the prototype's `DrillDialog`
 *     (`DashboardWorkspace.tsx:244-247,331`): title, description, Mã · Nội dung · Bộ phận · Người xử lý ·
 *     Hạn, and `Mở`;
 *   · `Mở`, and a task row of "Cần xử lý ngay", open the task's DRAWER here — the register's one wiring
 *     (`TaskDetailHost`), as the Sổ tay lãnh đạo hosts it — never a jump to `/nhiem-vu`.
 *
 * THE ROWS ARE THE FIGURE'S ROWS: `GET /api/v1/tasks?metric=…[&from&to]`, the predicate the server
 * counts the figure with (ADR 0053 §7), and the count in the description is `GET /api/v1/task-counts`
 * under the SAME filter — never the length of a page. No `roots`: the figure counts sub-tasks too.
 *
 * GATES UNCHANGED: a figure or row that opens this exists only behind `report.read` + `task.read`
 * (`DashboardBlocks`); both routes check `task.read` again, and the drawer hides every write the
 * session lacks the key for (rule 5).
 *
 * A write in the drawer re-reads the dialog's rows in place; the page's figures are read again when the
 * drawer closes after a write, as the Sổ tay does — not under the officer while the drawer is open.
 *
 * THE SAME HOOK OPENS THE OTHER REGISTERS' LISTS (sheet web-admin-131026, TỔNG QUAN): a figure of Văn bản
 * & Đơn thư, Phản ánh người dân or Giải ngân ngân sách sends a `DrillRequest` keyed by its register and
 * gets `RecordDrillDialog` (`record-drill.tsx`) — the same frame, its own list route. Its `Mở` opens the
 * record's own detail HERE (`record-detail.tsx`, tester bug sheet Tổng quan), above the dialog, exactly
 * as a task row opens the task drawer; a write there re-reads the rows, and the figures once it closes.
 */
export function useDashboardTaskDrill(onTasksChanged: () => void): { drill: DashboardDrill; overlay: ReactNode } {
  const session = usePhien();
  const sessionOk = session !== null && session.ok ? session.duLieu : null;
  const permissions = sessionOk?.permissions ?? null;

  const [request, setRequest] = useState<DrillRequest | null>(null);
  // Bumped after every write in the drawer: the dialog's rows re-read, kept on screen meanwhile.
  const [listRefresh, setListRefresh] = useState(0);
  const wroteSinceOpen = useRef(false);

  // ── One record of another register, opened from its list (`record-detail.tsx`) ──
  const [record, setRecord] = useState<RecordDetailTarget | null>(null);
  const [recordListRefresh, setRecordListRefresh] = useState(0);
  const recordWrote = useRef(false);
  const closeRecord = useCallback(() => {
    setRecord(null);
    if (recordWrote.current) {
      recordWrote.current = false;
      setRecordListRefresh((n) => n + 1);
      onTasksChanged();
    }
  }, [onTasksChanged]);
  const recordChanged = useCallback(() => {
    recordWrote.current = true;
  }, []);

  // ── The drawer: ONE at a time, the register's reducer without record tabs (as `leader-notebook.tsx`) ──
  const [drawer, dispatchDrawer] = useReducer(chuyenDrawer, null);
  const [opening, setOpening] = useState<{ code: string; error: string | null } | null>(null);
  const openingSeq = useRef(0);
  useTaskDetailRead(drawer, dispatchDrawer);
  const detailState = useTaskDetailState();

  // Read on first need only: a leader who never opens a list costs no extra call. The task catalogues
  // and status labels serve the TASK dialog and drawer only — another register's dialog reads its own
  // unit and staff names (`record-drill.tsx`), so opening it does not call task routes.
  const wanted = request?.list === "tasks" || drawer !== null || opening !== null;
  const catalogues = useDetailCatalogues(wanted);
  const [directoryResult, setDirectoryResult] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  const [labelsResult, setLabelsResult] = useState<KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null>(null);
  const started = wanted || directoryResult !== null;
  useEffect(() => {
    if (!started || directoryResult !== null) return;
    let cancelled = false;
    layDanhBaChonNguoi().then((r) => {
      if (!cancelled) setDirectoryResult(r);
    });
    layTrangThaiNhiemVu().then((r) => {
      if (!cancelled) setLabelsResult(r);
    });
    return () => {
      cancelled = true;
    };
  }, [started, directoryResult]);
  const directory = useMemo(() => danhBaChoNhatKy(directoryResult), [directoryResult]);
  const statusLabels = withSpecLabels(docBangNhanTrangThai(labelsResult).bang);

  const openList = useCallback((r: DrillRequest) => setRequest(r), []);

  /** A row of "Cần xử lý ngay" names its task by code: read it (task.read + commune), then open. */
  const openTask = useCallback((code: string) => {
    const seq = ++openingSeq.current;
    dispatchDrawer({ loai: "dong" });
    setOpening({ code, error: null });
    layNhiemVu(code).then((kq) => {
      if (openingSeq.current !== seq) return;
      if (!kq.ok) {
        setOpening({ code, error: kq.thongBao });
        return;
      }
      setOpening(null);
      dispatchDrawer({ loai: "mo", nhiemVu: kq.duLieu });
    });
  }, []);

  function openRow(task: petitions_nhiemVuRa): void {
    openingSeq.current += 1;
    setOpening(null);
    dispatchDrawer({ loai: "mo", nhiemVu: task });
  }

  function dispatch(action: ViecDrawer): void {
    if (action.loai === "dong") {
      openingSeq.current += 1;
      setOpening(null);
      if (wroteSinceOpen.current) {
        wroteSinceOpen.current = false;
        onTasksChanged();
      }
    }
    dispatchDrawer(action);
  }

  function changed(): void {
    wroteSinceOpen.current = true;
    setListRefresh((n) => n + 1);
  }

  const drill = useMemo<DashboardDrill>(() => ({ openList, openTask }), [openList, openTask]);
  const shownCode = opening?.code ?? drawer?.nhiemVu.code ?? null;

  const overlay = (
    <>
      {request !== null && request.list !== "tasks" && (
        <RecordDrillDialog
          request={request}
          refresh={recordListRefresh}
          onOpen={setRecord}
          onClose={() => setRequest(null)}
        />
      )}
      {record !== null && (
        <RecordDetail
          // Keyed by the record: opening another one is a fresh detail, no draft carried over.
          key={record.kind === "citizen-report" ? `p:${record.code}` : `${record.kind}:${record.id}`}
          target={record}
          onClose={closeRecord}
          onChanged={recordChanged}
        />
      )}
      {request !== null && request.list === "tasks" && (
        <TaskDrillDialog
          request={request}
          refresh={listRefresh}
          unitNames={catalogues.unitNames}
          directory={directory}
          onOpen={openRow}
          onClose={() => setRequest(null)}
        />
      )}
      {shownCode !== null && (
        <TaskDetailHost
          shownCode={shownCode}
          drawer={opening === null ? drawer : null}
          pendingTitle={`Nhiệm vụ ${shownCode}`}
          pendingError={opening?.error ?? null}
          dispatch={dispatch}
          context={{
            catalogues: catalogues.catalogues,
            statusLabels,
            unitNames: catalogues.unitNames,
            directory: directoryResult,
            leaderDirectory: catalogues.leaders,
            staffCode: sessionOk?.staff.code ?? "",
            permissions: quyenNhiemVu(permissions),
            canRevealEmail: permissions !== null && coQuyen(permissions, QUYEN_XEM_NHIEM_VU),
          }}
          state={detailState}
          onRegisterChanged={changed}
          onDeleted={() => dispatch({ loai: "dong" })}
        />
      )}
    </>
  );
  return { drill, overlay };
}

export { DRILL_EMPTY, DRILL_LOADING, DRILL_MORE, DRILL_OPEN, DRILL_TITLE_ID } from "./drill-dialog";

/** The prototype's description: `{n} nhiệm vụ đằng sau con số này.` — n from `/task-counts`. */
export function drillDescription(count: number | null): string {
  return drillDescriptionOf("nhiệm vụ", count);
}

type DrillPage = {
  readonly key: string;
  readonly result: KetQua<{ readonly items: readonly petitions_nhiemVuRa[]; readonly next_cursor: string; readonly has_more: boolean }>;
};

/**
 * The rows behind one task figure — the prototype's `DrillDialog`, in the shared frame
 * (`drill-dialog.tsx`). `Mở` opens the drawer ABOVE this dialog, so closing the drawer comes back to
 * the list.
 */
export function TaskDrillDialog({
  request,
  refresh,
  unitNames,
  directory,
  onOpen,
  onClose,
}: {
  request: TaskDrillRequest;
  /** A newer number = read again, keeping the rows on screen until the answer replaces them. */
  refresh: number;
  unitNames: ReadonlyMap<string, string>;
  directory: ReturnType<typeof danhBaChoNhatKy>;
  onOpen: (task: petitions_nhiemVuRa) => void;
  onClose: () => void;
}) {
  const filter = useMemo(
    () => ({ metric: request.metric, from: request.period.from, to: request.period.to }),
    [request],
  );
  const base = `${request.metric}|${request.period.from}|${request.period.to}`;
  const key = `${base}|${refresh}`;
  const [page, setPage] = useState<DrillPage | null>(null);
  const [count, setCount] = useState<{ key: string; value: number | null } | null>(null);
  const [more, setMore] = useState<{ key: string; items: readonly petitions_nhiemVuRa[]; cursor: string; hasMore: boolean } | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);

  useEffect(() => {
    let cancelled = false;
    laySoNhiemVu({ ...filter, limit: DRILL_PAGE_SIZE }).then((result) => {
      if (!cancelled) setPage({ key, result });
    });
    getTaskCounts(filter).then((r) => {
      if (!cancelled) setCount({ key, value: r.ok ? r.duLieu.by_status.reduce((n, s) => n + s.count, 0) : null });
    });
    return () => {
      cancelled = true;
    };
  }, [filter, key]);

  // The answer on screen: this key's, or — re-reading after a write — the previous one of the SAME figure.
  const sameFigure = (k: string) => k.slice(0, k.lastIndexOf("|")) === base;
  const shown = page !== null && (page.key === key || (sameFigure(page.key) && page.result.ok)) ? page : null;
  const shownCount = count !== null && (count.key === key || sameFigure(count.key)) ? count.value : null;
  const extra = more !== null && shown !== null && more.key === shown.key ? more : null;
  const first = shown !== null && shown.result.ok ? shown.result.duLieu : null;
  const rows = first === null ? [] : [...first.items, ...(extra?.items ?? [])];
  const cursor = extra !== null ? extra.cursor : (first?.next_cursor ?? "");
  const hasMore = (extra !== null ? extra.hasMore : (first?.has_more ?? false)) && cursor !== "";

  function loadMore(): void {
    if (loadingMore || !hasMore || shown === null) return;
    const at = shown.key;
    setLoadingMore(true);
    laySoNhiemVu({ ...filter, limit: DRILL_PAGE_SIZE, cursor }).then((r) => {
      setLoadingMore(false);
      if (!r.ok) return;
      setMore((m) => ({
        key: at,
        items: [...(m !== null && m.key === at ? m.items : []), ...r.duLieu.items],
        cursor: r.duLieu.next_cursor,
        hasMore: r.duLieu.has_more,
      }));
    });
  }

  const body: DrillBody =
    shown === null
      ? { kind: "loading" }
      : !shown.result.ok
        ? { kind: "error", message: shown.result.thongBao }
        : {
            kind: "rows",
            rows: rows.map((t) => ({
              key: t.code,
              code: t.code,
              title: t.title,
              unit: t.unit === "" ? "—" : (unitNames.get(t.unit) ?? "—"),
              assignee: nhanCanBoNgan(t.assignee, directory, CHUA_PHAN_CONG),
              due: nhanNgay(t.due_at),
              open: (
                <button
                  type="button"
                  className={DRILL_OPEN_CLASS}
                  aria-label={`${DRILL_OPEN} ${t.code}`}
                  aria-haspopup="dialog"
                  onClick={() => onOpen(t)}
                >
                  {DRILL_OPEN}
                  <ExternalLink aria-hidden="true" focusable="false" className="size-3" />
                </button>
              ),
            })),
          };

  return (
    <DrillTableDialog
      title={request.label}
      description={shown === null ? DRILL_LOADING : drillDescription(shownCount)}
      body={body}
      hasMore={hasMore}
      loadingMore={loadingMore}
      onMore={loadMore}
      onClose={onClose}
    />
  );
}
