"use client";

import { useEffect, useMemo, useState } from "react";

import { useCauHinhXa } from "@/components/cau-hinh-xa";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { countCitizenLetters } from "@/lib/api/citizen-letters";
import {
  fetchCitizenReportSummary,
  fetchIncomingDocumentOverdueQueue,
  fetchIncomingDocumentSummary,
  fetchOverdueCitizenReports,
  fetchOverdueTasks,
  fetchTaskSummary,
} from "@/lib/api/dashboard";
import type { SummaryPeriod } from "@/lib/api/dashboard";
import { layLoaiNhiemVu } from "@/lib/api/danh-muc-nghiep-vu";
import { getProjectSummary } from "@/lib/api/du-an";
import type { KetQua } from "@/lib/api/goi";
import { getMapAssetSummary } from "@/lib/api/map-assets";
import { layChiSoNganSach } from "@/lib/api/thu-chi";
import { coQuyen, quyetDinhTheoKhoa, REPORT_EXPORT_PERMISSION, REPORT_READ_PERMISSION } from "@/lib/quyen";

import { mergeQueues } from "./figures";
import type { QueueSource } from "./figures";
import { DEFAULT_PERIOD_KIND, periodWindows, toDateRange, toQueryPeriod, zoneYear } from "./period";
import type { PeriodKind } from "./period";
import { useDashboardTaskDrill } from "./task-drill";
import { blockVisibility, DashboardHeader, DashboardView } from "./view";
import type {
  BlocksData,
  BlockVisibility,
  ComparedWindows,
  DashboardData,
  Loaded,
  SummaryPair,
  TaskTypeLabels,
} from "./view";

/** Sentence shown instead of the page to an account without `report.read`. */
export const MISSING_REPORT_READ =
  "Tài khoản của bạn không có quyền xem báo cáo (report.read), nên trang Tổng quan điều hành " +
  "không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

/**
 * `/tong-quan` body: the page gate, then the hook half.
 *
 * ONE KEY AT THE GATE (`report.read`), the module keys PER BLOCK inside — `CongQuyen` takes one key
 * by design, so the pair is composed in `canSeeBlock` rather than by widening it. All of it is
 * convenience: every figure route checks both keys itself (rule 5, forbidden #1).
 */
export function DashboardPage() {
  const session = usePhien();
  // The full header (period, "tính đến", period buttons) is drawn by `DashboardView` inside the gate.
  // Until the gate opens — session still being read, key missing, session unreadable — the title
  // and the disabled actions are drawn here instead, so the account still reads which page it is on.
  const open = session !== null && quyetDinhTheoKhoa(session, REPORT_READ_PERMISSION).hien;
  return (
    <>
      {!open && <DashboardHeader />}
      <CongQuyen khoa={REPORT_READ_PERMISSION} cauThieuQuyen={MISSING_REPORT_READ}>
        <DashboardOverview />
      </CongQuyen>
    </>
  );
}

/**
 * One summary asked for twice — current window and previous window — as two INDEPENDENT calls: a
 * failed previous-period call must not blank the current figures, and the reverse.
 */
function loadPair<T, P>(
  call: (p: P) => Promise<KetQua<T>>,
  current: P,
  previous: P,
  onUpdate: (pair: SummaryPair<T>) => void,
): void {
  let cur: Loaded<T> = null;
  let prev: Loaded<T> = null;
  call(current).then((kq) => {
    cur = kq;
    onUpdate({ current: cur, previous: prev });
  });
  call(previous).then((kq) => {
    prev = kq;
    onUpdate({ current: cur, previous: prev });
  });
}

type Results = Omit<DashboardData, "windows" | "fetchedAt" | "fiscalYear"> & {
  readonly token: string;
};

function emptyResults(token: string): Results {
  return {
    token,
    tasks: { current: null, previous: null },
    incomingDocuments: { current: null, previous: null },
    citizenReports: { current: null, previous: null },
    citizenLetters: { current: null, previous: null },
    fiscal: null,
    budget: null,
    queue: null,
    taskTypeLabels: null,
    mapAssets: null,
  };
}

/**
 * The hook half: reads the session's permissions, owns the period and the clock, and fetches ONLY
 * what the visible blocks show. A hidden block's routes are never called — a 403 nobody displays
 * is still a request the server has to refuse.
 *
 * THE CLOCK IS READ ON AN ACT (mount, a period button), never during a render: the instant is the
 * page's "tính đến", and every window of that load is computed from the same one.
 */
function DashboardOverview() {
  const session = usePhien();
  const permissions = session !== null && session.ok ? session.duLieu.permissions : [];
  const visible = blockVisibility(permissions);

  const [request, setRequest] = useState<{ kind: PeriodKind; at: number }>(() => ({
    kind: DEFAULT_PERIOD_KIND,
    at: new Date().getTime(),
  }));
  const windows = useMemo(() => periodWindows(request.kind, new Date(request.at)), [request]);
  const shown = useDashboardFigures(windows, request.at, visible, true);
  const data: DashboardData = { ...shown, windows, fetchedAt: request.at };
  // The commune printed on the export files: the runtime configuration the server resolved from
  // `Host` (rule 1 inv. 10) — the same value the page header prints. Never a constant.
  const commune = useCauHinhXa();
  // Trình chiếu: page state, never the URL — a drill-down navigates away and the page comes back
  // normal (`presentation.tsx`).
  const [presenting, setPresenting] = useState(false);
  // Task figures and "Cần xử lý ngay" task rows open IN PLACE (customer sheet rows 3 and 5). A write in
  // the drawer reads the figures again once it closes — the same act as pressing the period again.
  const { drill, overlay } = useDashboardTaskDrill(() =>
    setRequest((r) => ({ kind: r.kind, at: new Date().getTime() })),
  );
  // Without `task.read` the task drawer is withheld (its reads would only be refused), but the other
  // registers' figures still open their own lists — each tile exists only behind its block's keys.
  const tasksVisible = visible.tasks;
  const pageDrill = useMemo(
    () => (tasksVisible ? drill : { openList: drill.openList }),
    [tasksVisible, drill],
  );

  return (
    <DashboardView
      data={data}
      visible={visible}
      onPeriodChange={(kind) => setRequest({ kind, at: new Date().getTime() })}
      exportAccess={{
        commune: { displayName: commune.displayName, parentAuthority: commune.parentAuthority },
        canExport: coQuyen(permissions, REPORT_EXPORT_PERMISSION),
      }}
      presentation={{ on: presenting, onChange: setPresenting }}
      drill={pageDrill}
      overlay={overlay}
    />
  );
}

/**
 * The loader of the overview's figures, shared with `/bao-cao` so the two pages ask the servers the
 * very same query for the same period (spec 13 §10).
 *
 * `at` is the instant of the act that asked (mount, a period button): a new `at` re-asks even for the
 * same windows — that is "Tải lại". `withQueues` = also read the three overdue queues and the
 * task-type catalogue that only "Cần xử lý ngay" draws; `/bao-cao` has no such block and does not ask.
 */
export function useDashboardFigures(
  windows: ComparedWindows,
  at: number,
  visible: BlockVisibility,
  withQueues: boolean,
): BlocksData {
  // "Đơn thư trong kỳ" sits INSIDE the documents block, so it needs that block's keys AND its own.
  const lettersVisible = visible.incomingDocuments && visible.citizenLetters;
  const visibleKey =
    `${visible.tasks}|${visible.incomingDocuments}|${visible.citizenReports}|${visible.budget}` +
    `|${lettersVisible}|${visible.economy}`;
  // Primitives, not the window objects: the effect re-runs on what is ASKED, not on object identity.
  const { from: curFrom, to: curTo } = toQueryPeriod(windows.current);
  const { from: prevFrom, to: prevTo } = toQueryPeriod(windows.previous);
  // The letter register is dated by DAY: the same two windows as inclusive wall dates (`toDateRange`).
  const { from: curDayFrom, to: curDayTo } = toDateRange(windows.current);
  const { from: prevDayFrom, to: prevDayTo } = toDateRange(windows.previous);
  const fiscalYear = zoneYear(at);
  const token = `${curFrom}|${curTo}|${prevFrom}|${prevTo}|${at}|${visibleKey}|${withQueues}`;

  const [results, setResults] = useState<Results>(() => emptyResults(""));

  useEffect(() => {
    let cancelled = false;
    const put = (patch: Partial<Omit<Results, "token">>) => {
      if (cancelled) return;
      setResults((prev) => ({ ...(prev.token === token ? prev : emptyResults(token)), ...patch }));
    };
    const current: SummaryPeriod = { from: curFrom, to: curTo };
    const previous: SummaryPeriod = { from: prevFrom, to: prevTo };
    const [showTasks, showDocuments, showReports, showBudget, showLetters, showEconomy] = visibleKey
      .split("|")
      .map((v) => v === "true");

    if (showTasks) loadPair(fetchTaskSummary, current, previous, (tasks) => put({ tasks }));
    if (showDocuments) {
      loadPair(fetchIncomingDocumentSummary, current, previous, (incomingDocuments) =>
        put({ incomingDocuments }),
      );
    }
    if (showLetters) {
      // `scope` is not sent: the server's default is the whole register of the commune (`all`), the
      // same scope the documents block counts. The commune is the Host's (rule 1).
      loadPair(
        (d: { from: string; to: string }) => countCitizenLetters({ receivedFrom: d.from, receivedTo: d.to }),
        { from: curDayFrom, to: curDayTo },
        { from: prevDayFrom, to: prevDayTo },
        (citizenLetters) => put({ citizenLetters }),
      );
    }
    // Khối Kinh tế & Tài nguyên: the map register as it stands now — one call, no period, no pair.
    if (showEconomy) getMapAssetSummary().then((kq) => put({ mapAssets: kq }));
    if (showReports) {
      loadPair(fetchCitizenReportSummary, current, previous, (citizenReports) =>
        put({ citizenReports }),
      );
    }
    if (showBudget) layChiSoNganSach(fiscalYear).then((kq) => put({ fiscal: kq }));
    // Khối Giải ngân: the SAME summary `/giai-ngan` reads, of the same year as Thu – Chi (`budget.read`).
    if (showBudget) getProjectSummary(fiscalYear).then((kq) => put({ budget: kq }));

    const queues: Promise<QueueSource>[] = [];
    if (withQueues && showTasks) {
      queues.push(fetchOverdueTasks().then((result) => ({ module: "task" as const, result })));
      layLoaiNhiemVu().then((kq) => {
        const labels: TaskTypeLabels = kq.ok
          ? new Map(kq.duLieu.items.map((t) => [t.code, t.label]))
          : null;
        put({ taskTypeLabels: labels });
      });
    }
    if (withQueues && showReports) {
      queues.push(
        fetchOverdueCitizenReports().then((result) => ({ module: "citizen-report" as const, result })),
      );
    }
    if (withQueues && showDocuments) {
      queues.push(
        fetchIncomingDocumentOverdueQueue().then((result) => ({
          module: "incoming-document" as const,
          result,
        })),
      );
    }
    if (queues.length > 0) {
      Promise.all(queues).then((sources) => put({ queue: { ok: true, duLieu: mergeQueues(sources) } }));
    }

    return () => {
      cancelled = true;
    };
  }, [
    token,
    curFrom,
    curTo,
    prevFrom,
    prevTo,
    curDayFrom,
    curDayTo,
    prevDayFrom,
    prevDayTo,
    visibleKey,
    fiscalYear,
    withQueues,
  ]);

  const shown = results.token === token ? results : emptyResults(token);
  return {
    windows,
    fiscalYear,
    tasks: shown.tasks,
    incomingDocuments: shown.incomingDocuments,
    citizenReports: shown.citizenReports,
    citizenLetters: shown.citizenLetters,
    fiscal: shown.fiscal,
    budget: shown.budget,
    queue: shown.queue,
    taskTypeLabels: shown.taskTypeLabels,
    mapAssets: shown.mapAssets,
  };
}
