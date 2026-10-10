"use client";

import { ExternalLink, Loader2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useReducer, useRef, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
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
import { TABLE_CLASS, TABLE_FRAME_CLASS, TD_CLASS, TH_CLASS, withSpecLabels } from "@/features/nhiem-vu/task-spec";
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
import { cn } from "@/lib/cn";
import { coQuyen, QUYEN_XEM_NHIEM_VU } from "@/lib/quyen";

import { formatCount } from "./figures";
import type { DashboardDrill, TaskDrillRequest } from "./view";

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
 */
export function useDashboardTaskDrill(onTasksChanged: () => void): { drill: DashboardDrill; overlay: ReactNode } {
  const session = usePhien();
  const sessionOk = session !== null && session.ok ? session.duLieu : null;
  const permissions = sessionOk?.permissions ?? null;

  const [request, setRequest] = useState<TaskDrillRequest | null>(null);
  // Bumped after every write in the drawer: the dialog's rows re-read, kept on screen meanwhile.
  const [listRefresh, setListRefresh] = useState(0);
  const wroteSinceOpen = useRef(false);

  // ── The drawer: ONE at a time, the register's reducer without record tabs (as `leader-notebook.tsx`) ──
  const [drawer, dispatchDrawer] = useReducer(chuyenDrawer, null);
  const [opening, setOpening] = useState<{ code: string; error: string | null } | null>(null);
  const openingSeq = useRef(0);
  useTaskDetailRead(drawer, dispatchDrawer);
  const detailState = useTaskDetailState();

  // Read on first need only: a leader who never opens a list costs no extra call.
  const wanted = request !== null || drawer !== null || opening !== null;
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

  const openTaskList = useCallback((r: TaskDrillRequest) => setRequest(r), []);

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

  const drill = useMemo<DashboardDrill>(() => ({ openTaskList, openTask }), [openTaskList, openTask]);
  const shownCode = opening?.code ?? drawer?.nhiemVu.code ?? null;

  const overlay = (
    <>
      {request !== null && (
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

/** Rows per read — the register's page size; `Xem thêm` reads the next page with the server's cursor. */
const DRILL_PAGE_SIZE = 100;

export const DRILL_TITLE_ID = "tieu-de-danh-sach-con-so";
export const DRILL_LOADING = "Đang lấy danh sách…";
export const DRILL_EMPTY = "Không có bản ghi nào.";
export const DRILL_OPEN = "Mở";
export const DRILL_MORE = "Xem thêm";

/** The prototype's description: `{n} nhiệm vụ đằng sau con số này.` — n from `/task-counts`. */
export function drillDescription(count: number | null): string {
  return count === null ? DRILL_LOADING : `${formatCount(count)} nhiệm vụ đằng sau con số này.`;
}

type DrillPage = {
  readonly key: string;
  readonly result: KetQua<{ readonly items: readonly petitions_nhiemVuRa[]; readonly next_cursor: string; readonly has_more: boolean }>;
};

/**
 * The rows behind one task figure — the prototype's `DrillDialog`, in the app's `ModalDialog` (native,
 * centred, Esc asks). The table alone scrolls; `Mở` opens the drawer ABOVE this dialog, so closing the
 * drawer comes back to the list.
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

  return (
    <ModalDialog titleId={DRILL_TITLE_ID} size="lg" className="max-w-5xl" onDismiss={onClose}>
      <ModalDialogHeader
        titleId={DRILL_TITLE_ID}
        title={request.label}
        description={shown === null ? DRILL_LOADING : drillDescription(shownCount)}
      />
      {shown === null ? (
        <p role="status" className="text-ink-muted m-0 flex items-center gap-2 py-10 text-[13px]">
          <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
          {DRILL_LOADING}
        </p>
      ) : !shown.result.ok ? (
        <p role="alert" className="thong-bao-loi m-0">
          {shown.result.thongBao}
        </p>
      ) : (
        <div
          role="region"
          aria-label={`Danh sách: ${request.label}`}
          tabIndex={0}
          className={cn(TABLE_FRAME_CLASS, "max-h-[56vh] min-h-0 overflow-auto")}
        >
          <table className={TABLE_CLASS}>
            <thead>
              <tr className="border-line border-b">
                <th scope="col" className={TH_CLASS}>Mã</th>
                <th scope="col" className={TH_CLASS}>Nội dung</th>
                <th scope="col" className={TH_CLASS}>Bộ phận</th>
                <th scope="col" className={TH_CLASS}>Người xử lý</th>
                <th scope="col" className={TH_CLASS}>Hạn</th>
                <th scope="col" className={TH_CLASS}>
                  <span className="an-thi-giac">Thao tác</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={6} className="text-ink-muted py-10 text-center">
                    {DRILL_EMPTY}
                  </td>
                </tr>
              ) : (
                rows.map((t) => (
                  <tr key={t.code} className="border-line border-b last:border-b-0">
                    <td className={TD_CLASS}>
                      <code className="text-[11.5px]">{t.code === "" ? "—" : t.code}</code>
                    </td>
                    <td className={cn(TD_CLASS, "text-navy max-w-[26rem] font-medium whitespace-normal")}>
                      <span className="line-clamp-2">{t.title}</span>
                    </td>
                    <td className={cn(TD_CLASS, "text-ink-muted")}>
                      {t.unit === "" ? "—" : (unitNames.get(t.unit) ?? "—")}
                    </td>
                    <td className={cn(TD_CLASS, "text-ink-muted")}>{nhanCanBoNgan(t.assignee, directory, CHUA_PHAN_CONG)}</td>
                    <td className={TD_CLASS}>{nhanNgay(t.due_at)}</td>
                    <td className={TD_CLASS}>
                      <button
                        type="button"
                        className="text-brand inline-flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[12px] [font-family:inherit] focus-visible:outline-2 focus-visible:outline-brand-500"
                        aria-label={`${DRILL_OPEN} ${t.code}`}
                        aria-haspopup="dialog"
                        onClick={() => onOpen(t)}
                      >
                        {DRILL_OPEN}
                        <ExternalLink aria-hidden="true" focusable="false" className="size-3" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
      {hasMore && (
        <div>
          <Button type="button" variant="outline" size="sm" disabled={loadingMore} aria-busy={loadingMore || undefined} onClick={loadMore}>
            {DRILL_MORE}
          </Button>
        </div>
      )}
    </ModalDialog>
  );
}
