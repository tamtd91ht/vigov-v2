"use client";

import { useEffect, useMemo, useState } from "react";

import { CongQuyen } from "@/features/quyen/cong-quyen";
import { usePhien } from "@/features/phien/phien-hien-tai";
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
import type { KetQua } from "@/lib/api/goi";
import { layChiSoNganSach } from "@/lib/api/thu-chi";
import { REPORT_READ_PERMISSION } from "@/lib/quyen";

import { mergeQueues } from "./figures";
import type { QueueSource } from "./figures";
import { DEFAULT_PERIOD_KIND, periodWindows, toQueryPeriod, zoneYear } from "./period";
import type { PeriodKind } from "./period";
import { blockVisibility, DashboardView } from "./view";
import type { DashboardData, Loaded, SummaryPair, TaskTypeLabels } from "./view";

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
  return (
    <CongQuyen khoa={REPORT_READ_PERMISSION} cauThieuQuyen={MISSING_REPORT_READ}>
      <DashboardOverview />
    </CongQuyen>
  );
}

/**
 * One summary asked for twice — current window and previous window — as two INDEPENDENT calls: a
 * failed previous-period call must not blank the current figures, and the reverse.
 */
function loadPair<T>(
  call: (p: SummaryPeriod) => Promise<KetQua<T>>,
  current: SummaryPeriod,
  previous: SummaryPeriod,
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
    fiscal: null,
    queue: null,
    taskTypeLabels: null,
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
  const visibleKey = `${visible.tasks}|${visible.incomingDocuments}|${visible.citizenReports}|${visible.budget}`;

  const [request, setRequest] = useState<{ kind: PeriodKind; at: number }>(() => ({
    kind: DEFAULT_PERIOD_KIND,
    at: new Date().getTime(),
  }));
  const windows = useMemo(() => periodWindows(request.kind, new Date(request.at)), [request]);
  const fiscalYear = zoneYear(request.at);
  const token = `${request.kind}|${request.at}|${visibleKey}`;

  const [results, setResults] = useState<Results>(() => emptyResults(""));

  useEffect(() => {
    let cancelled = false;
    const put = (patch: Partial<Omit<Results, "token">>) => {
      if (cancelled) return;
      setResults((prev) => ({ ...(prev.token === token ? prev : emptyResults(token)), ...patch }));
    };
    const current = toQueryPeriod(windows.current);
    const previous = toQueryPeriod(windows.previous);
    const [showTasks, showDocuments, showReports, showBudget] = visibleKey
      .split("|")
      .map((v) => v === "true");

    if (showTasks) loadPair(fetchTaskSummary, current, previous, (tasks) => put({ tasks }));
    if (showDocuments) {
      loadPair(fetchIncomingDocumentSummary, current, previous, (incomingDocuments) =>
        put({ incomingDocuments }),
      );
    }
    if (showReports) {
      loadPair(fetchCitizenReportSummary, current, previous, (citizenReports) =>
        put({ citizenReports }),
      );
    }
    if (showBudget) layChiSoNganSach(fiscalYear).then((kq) => put({ fiscal: kq }));

    const queues: Promise<QueueSource>[] = [];
    if (showTasks) {
      queues.push(fetchOverdueTasks().then((result) => ({ module: "task" as const, result })));
      layLoaiNhiemVu().then((kq) => {
        const labels: TaskTypeLabels = kq.ok
          ? new Map(kq.duLieu.items.map((t) => [t.code, t.label]))
          : null;
        put({ taskTypeLabels: labels });
      });
    }
    if (showReports) {
      queues.push(
        fetchOverdueCitizenReports().then((result) => ({ module: "citizen-report" as const, result })),
      );
    }
    if (showDocuments) {
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
  }, [token, windows, visibleKey, fiscalYear]);

  const shown = results.token === token ? results : emptyResults(token);
  const data: DashboardData = {
    windows,
    fetchedAt: request.at,
    fiscalYear,
    tasks: shown.tasks,
    incomingDocuments: shown.incomingDocuments,
    citizenReports: shown.citizenReports,
    fiscal: shown.fiscal,
    queue: shown.queue,
    taskTypeLabels: shown.taskTypeLabels,
  };

  return (
    <DashboardView
      data={data}
      visible={visible}
      onPeriodChange={(kind) => setRequest({ kind, at: new Date().getTime() })}
    />
  );
}
