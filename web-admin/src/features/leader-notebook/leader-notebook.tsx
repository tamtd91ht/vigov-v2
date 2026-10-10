"use client";

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";

import { CongQuyen } from "@/features/quyen/cong-quyen"; // vi-name-ok: existing permission-gate component
import { danhBaChoNhatKy, docBangNhanTrangThai, quyenNhiemVu } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing readers of the register, imported unchanged
import { chuyenDrawer, type DanhMucNhiemVu, type ViecDrawer } from "@/features/nhiem-vu/so-nhiem-vu"; // vi-name-ok: existing drawer reducer and types of the register
import { TaskDetailHost, useTaskDetailRead, useTaskDetailState } from "@/features/nhiem-vu/task-detail-host";
import { withSpecLabels } from "@/features/nhiem-vu/task-spec";
import { usePhien } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing session hook
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi"; // vi-name-ok: existing staff-directory client
import { layDanhMucBoPhan } from "@/lib/api/danh-muc"; // vi-name-ok: existing unit catalogue client
import { layKhoiNhiemVu, layLoaiNhiemVu, layMucUuTienNhiemVu } from "@/lib/api/danh-muc-nghiep-vu"; // vi-name-ok: existing task catalogue clients
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type
import { getTaskCounts, getTaskExtensionCount, layHangChoLuiHan, layNhiemVu, laySoNhiemVu } from "@/lib/api/nhiem-vu"; // vi-name-ok: existing task clients
import type { identity_danhBaChonNguoiRa, petitions_danhSachTrangThaiNhiemVuRa, petitions_deNghiChoDuyetRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract types
import { layTrangThaiNhiemVu } from "@/lib/api/trang-thai-nhiem-vu"; // vi-name-ok: existing status-label client
import { coQuyen, QUYEN_DUYET_GIA_HAN, QUYEN_DUYET_HOAN_THANH_NHIEM_VU, QUYEN_XEM_NHIEM_VU } from "@/lib/quyen"; // vi-name-ok: existing permission constants

import {
  ASSIGNED_BY_ME_FILTER,
  ASSIGNED_BY_ME_LIST,
  MY_EXTENSION_REQUESTS,
  NO_ACCESS_SENTENCE,
  OVERDUE_FILTER,
  OVERDUE_LIST,
  PAGE_SIZE,
  PENDING_APPROVAL_FILTER,
  sumCounts,
} from "./notebook-queries";
import { NotebookView, type ListState, type Loaded, type Section } from "./notebook-view";

/**
 * Sổ tay lãnh đạo — the reading half, and the drawer a row opens. Every list and every figure is ONE
 * server read through the register's clients (`lib/api/nhiem-vu.ts`); nothing is counted or filtered
 * here (ADR 0071).
 *
 * THE GATE IS `task.read` — every route behind the three columns declares exactly that key, so the
 * gate cannot hide anything the server would serve. It is convenience: each route checks it again
 * (rule 5, forbidden #1). Inside the gate, the `Duyệt hoàn thành` group additionally needs
 * `task.approve` (ADR 0071): without it the group is neither read nor drawn — those are rows this
 * account cannot act on.
 *
 * A ROW OPENS THE TASK'S DETAIL DRAWER HERE, IN PLACE (prototype `LeaderNotebook.tsx:104-114`, owner
 * 09/10/2026): the register's own drawer and its one wiring (`TaskDetailHost`), ONE drawer at a time —
 * no record tabs, no address-bar entry (the register's `?task=` is the register's). Closing it, and
 * every write made in it, re-reads the four sections and their counts, so a task just approved or
 * extended leaves the column it no longer belongs in.
 */
export function LeaderNotebook() {
  return (
    <CongQuyen khoa={QUYEN_XEM_NHIEM_VU} cauThieuQuyen={NO_ACCESS_SENTENCE}>
      <NotebookColumns />
    </CongQuyen>
  );
}

type Page<T> = { readonly items: readonly T[]; readonly next_cursor: string; readonly has_more: boolean };
type PageReader<T> = (cursor: string | null) => Promise<KetQua<Page<T>>>;
type CountReader = () => Promise<KetQua<number>>;

/* MODULE-LEVEL READERS: stable identities, so the effects below run once per reload, not per render. */

const readPastDue: PageReader<petitions_nhiemVuRa> = (cursor) =>
  laySoNhiemVu({ ...OVERDUE_LIST, limit: PAGE_SIZE, cursor });
const countPastDue: CountReader = () => getTaskCounts(OVERDUE_FILTER).then(toFigure);

const readCompletion: PageReader<petitions_nhiemVuRa> = (cursor) =>
  laySoNhiemVu({ ...PENDING_APPROVAL_FILTER, limit: PAGE_SIZE, cursor });
const countCompletion: CountReader = () => getTaskCounts(PENDING_APPROVAL_FILTER).then(toFigure);

const readExtensions: PageReader<petitions_deNghiChoDuyetRa> = (cursor) =>
  layHangChoLuiHan({ ...MY_EXTENSION_REQUESTS, limit: PAGE_SIZE, cursor });
const countExtensions: CountReader = () =>
  getTaskExtensionCount(MY_EXTENSION_REQUESTS).then((r) => (r.ok ? { ok: true, duLieu: r.duLieu.count } : r));

const readAssigned: PageReader<petitions_nhiemVuRa> = (cursor) =>
  laySoNhiemVu({ ...ASSIGNED_BY_ME_LIST, limit: PAGE_SIZE, cursor });
const countAssigned: CountReader = () => getTaskCounts(ASSIGNED_BY_ME_FILTER).then(toFigure);

function toFigure(r: Awaited<ReturnType<typeof getTaskCounts>>): KetQua<number> {
  return r.ok ? { ok: true, duLieu: sumCounts(r.duLieu) } : r;
}

const NO_CATALOGUES: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };

/** An extension row whose task is being read before its drawer opens; `error` = the server refused. */
type PendingOpen = { readonly code: string; readonly title: string; readonly error: string | null };

function NotebookColumns() {
  const session = usePhien();
  const sessionOk = session !== null && session.ok ? session.duLieu : null;
  // Inside `CongQuyen`, so the session is read and holds `task.read`. FAIL CLOSED all the same: an
  // unread session holds no key.
  const canApprove = sessionOk !== null && coQuyen(sessionOk.permissions, QUYEN_DUYET_HOAN_THANH_NHIEM_VU);

  // Bumped on closing the drawer and after every write in it: the four sections re-read.
  const [refresh, setRefresh] = useState(0);
  const reloadSections = useCallback(() => setRefresh((n) => n + 1), []);

  const pastDue = useSection(true, readPastDue, countPastDue, refresh);
  const completion = useSection(canApprove, readCompletion, countCompletion, refresh);
  const extensions = useSection(true, readExtensions, countExtensions, refresh);
  const assigned = useSection(true, readAssigned, countAssigned, refresh);

  // Names for the assignee line and the status labels: one read each for the page. A failed read is
  // not an error state — rows then show the staff code, and the badge the default label.
  const [directoryResult, setDirectoryResult] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  const [labelsResult, setLabelsResult] = useState<KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null>(null);
  useEffect(() => {
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
  }, []);
  const directory = useMemo(() => danhBaChoNhatKy(directoryResult), [directoryResult]);
  const labels = docBangNhanTrangThai(labelsResult);
  // THE REGISTER'S WORDS (`withSpecLabels`), the source its drawer uses: a row and the drawer it opens
  // name a status with the same word.
  const statusLabels = withSpecLabels(labels.bang);

  // ── The drawer: ONE at a time, the register's reducer without record tabs ──
  const [drawer, dispatchDrawer] = useReducer(chuyenDrawer, null);
  const [opening, setOpening] = useState<PendingOpen | null>(null);
  // Each opening takes a number; a read answering for an older one is dropped.
  const openingSeq = useRef(0);
  useTaskDetailRead(drawer, dispatchDrawer);
  const detailState = useTaskDetailState();
  const catalogues = useDetailCatalogues(drawer !== null || opening !== null);

  function dispatch(action: ViecDrawer): void {
    if (action.loai === "dong") {
      openingSeq.current += 1;
      setOpening(null);
      reloadSections();
    }
    dispatchDrawer(action);
  }

  function openTask(task: petitions_nhiemVuRa): void {
    openingSeq.current += 1;
    setOpening(null);
    dispatchDrawer({ loai: "mo", nhiemVu: task });
  }

  /**
   * A request row names its task by code: the task is read (the same `task.read` + commune check as
   * every read), then opened through the same `mo` as a task row. Meanwhile the drawer shows the
   * register's pending content; a refusal stays in it, verbatim, closable.
   */
  function openRequest(request: petitions_deNghiChoDuyetRa): void {
    const seq = ++openingSeq.current;
    const { task_code: code, task_title: title } = request;
    dispatchDrawer({ loai: "dong" });
    setOpening({ code, title, error: null });
    layNhiemVu(code).then((kq) => {
      if (openingSeq.current !== seq) return;
      if (!kq.ok) {
        setOpening({ code, title, error: kq.thongBao });
        return;
      }
      setOpening(null);
      dispatchDrawer({ loai: "mo", nhiemVu: kq.duLieu });
    });
  }

  const shownCode = opening?.code ?? drawer?.nhiemVu.code ?? null;
  const permissions = sessionOk?.permissions ?? null;

  return (
    <>
      {labels.canhBao !== null && <p className="ghi-chu">{labels.canhBao}</p>}
      <NotebookView
        pastDue={pastDue}
        completion={canApprove ? completion : null}
        extensions={extensions}
        assigned={assigned}
        directory={directory}
        statusLabels={statusLabels}
        now={new Date()}
        onOpenTask={openTask}
        onOpenRequest={openRequest}
      />
      {shownCode !== null && (
        <TaskDetailHost
          shownCode={shownCode}
          drawer={opening === null ? drawer : null}
          pendingTitle={opening?.title ?? ""}
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
          onRegisterChanged={reloadSections}
          onDeleted={() => dispatch({ loai: "dong" })}
        />
      )}
    </>
  );
}

export type DetailCatalogues = {
  readonly catalogues: DanhMucNhiemVu;
  readonly unitNames: ReadonlyMap<string, string>;
  readonly leaders: KetQua<identity_danhBaChonNguoiRa> | null;
};

/**
 * The catalogues the drawer draws with (types, priorities, blocs, units, the `task.extend` holders) —
 * read ONCE, the first time a drawer opens: a leader who only glances at the columns costs no extra
 * read. A failed catalogue is an empty one, as on the register; the drawer still opens.
 *
 * Exported for Tổng quan's in-place drawer (`features/dashboard/task-drill.tsx`): one reader of these
 * five catalogues for every screen that hosts the drawer without the register.
 */
export function useDetailCatalogues(wanted: boolean): DetailCatalogues {
  const [loaded, setLoaded] = useState<{ catalogues: DanhMucNhiemVu; leaders: KetQua<identity_danhBaChonNguoiRa> } | null>(null);
  const start = wanted || loaded !== null;
  useEffect(() => {
    if (!start || loaded !== null) return;
    let cancelled = false;
    Promise.all([
      layLoaiNhiemVu(),
      layMucUuTienNhiemVu(),
      layKhoiNhiemVu(),
      layDanhMucBoPhan(),
      layDanhBaChonNguoi(undefined, QUYEN_DUYET_GIA_HAN),
    ]).then(([types, priorities, blocs, units, leaders]) => {
      if (cancelled) return;
      setLoaded({
        catalogues: {
          loai: types.ok ? types.duLieu.items : [],
          mucUuTien: priorities.ok ? priorities.duLieu.items : [],
          khoi: blocs.ok ? blocs.duLieu.items : [],
          boPhan: units.ok ? units.duLieu.items : [],
        },
        leaders,
      });
    });
    return () => {
      cancelled = true;
    };
  }, [start, loaded]);
  return useMemo(() => {
    const catalogues = loaded?.catalogues ?? NO_CATALOGUES;
    return {
      catalogues,
      unitNames: new Map(catalogues.boPhan.map((b) => [b.id, b.name])),
      leaders: loaded?.leaders ?? null,
    };
  }, [loaded]);
}

type PageLoad<T> = {
  /** The `Tải lại` turn this answers — a different turn draws the loading state, not these rows. */
  readonly token: number;
  /** The drawer refresh this answers — a newer one re-reads with these rows kept on screen. */
  readonly generation: number;
  readonly load: Loaded<readonly T[]>;
  readonly cursor: string;
  readonly hasMore: boolean;
};

/**
 * One list and its server count. `Tải lại` re-reads BOTH from the first page: the two must describe the
 * same moment. A late answer for an older reload is dropped, never drawn over a newer one.
 *
 * `refresh` (the drawer closed, or wrote) re-reads both too, but KEEPS the rows and figure on screen
 * until the answer replaces them — three columns flashing grey on every close would read as a fault.
 */
function useSection<T>(enabled: boolean, readPage: PageReader<T>, readCount: CountReader, refresh: number): Section<T> {
  const [reload, setReload] = useState(0);
  const [page, setPage] = useState<PageLoad<T> | null>(null);
  const [count, setCount] = useState<{ token: number; load: Loaded<number> } | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreError, setMoreError] = useState<string | null>(null);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    readPage(null).then((r) => {
      if (cancelled) return;
      setPage(
        r.ok
          ? {
              token: reload,
              generation: refresh,
              load: { phase: "done", value: r.duLieu.items },
              cursor: r.duLieu.next_cursor,
              hasMore: r.duLieu.has_more && r.duLieu.next_cursor !== "",
            }
          : { token: reload, generation: refresh, load: { phase: "error", message: r.thongBao }, cursor: "", hasMore: false },
      );
    });
    readCount().then((r) => {
      if (cancelled) return;
      setCount({ token: reload, load: r.ok ? { phase: "done", value: r.duLieu } : { phase: "error", message: r.thongBao } });
    });
    return () => {
      cancelled = true;
    };
  }, [enabled, readPage, readCount, reload, refresh]);

  const current = page !== null && page.token === reload ? page : null;

  function more(): void {
    if (current === null || current.load.phase !== "done" || !current.hasMore || loadingMore) return;
    const before = current;
    const shown = current.load.value;
    setLoadingMore(true);
    setMoreError(null);
    readPage(before.cursor).then((r) => {
      setLoadingMore(false);
      if (!r.ok) {
        setMoreError(r.thongBao);
        return;
      }
      setPage((p) =>
        p === null || p.token !== before.token || p.generation !== before.generation
          ? p
          : {
              token: p.token,
              generation: p.generation,
              load: { phase: "done", value: [...shown, ...r.duLieu.items] },
              cursor: r.duLieu.next_cursor,
              hasMore: r.duLieu.has_more && r.duLieu.next_cursor !== "",
            },
      );
    });
  }

  const list: ListState<T> = {
    load: current === null ? { phase: "loading" } : current.load,
    hasMore: current?.hasMore ?? false,
    loadingMore,
    moreError,
    onRetry: () => {
      setMoreError(null);
      setReload((n) => n + 1);
    },
    onMore: more,
  };
  return { list, count: count !== null && count.token === reload ? count.load : { phase: "loading" } };
}
