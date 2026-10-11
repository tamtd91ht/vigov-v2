"use client";

import { useEffect, useMemo, useState } from "react";

import { useCauHinhXa } from "@/components/cau-hinh-xa";
import { useDashboardFigures } from "@/features/dashboard/overview";
import { DEFAULT_PERIOD_KIND, toQueryPeriod } from "@/features/dashboard/period";
import { figuresSettled } from "@/features/dashboard/view";
import type { BlocksData, Loaded } from "@/features/dashboard/view";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { fetchTaskUnitSummary } from "@/lib/api/dashboard";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import { quyetDinhTheoKhoa, REPORT_READ_PERMISSION } from "@/lib/quyen";

import { reportAccess } from "./report-access";
import { CUSTOM_PERIOD, reportWindows } from "./report-period";
import type { ReportSelection, ReportWindows } from "./report-period";
import { ReportHeader, ReportView } from "./report-view";
import { unitRowsFrom } from "./unit-table";
import type { UnitRow } from "./unit-table";

/** The prototype's auto-refresh period (`useReports.ts`). */
const REPORT_REFRESH_MS = 120_000;

/** Sentence shown instead of the page to an account without `report.read`. */
export const MISSING_REPORT_READ =
  "Tài khoản của bạn không có quyền xem báo cáo (report.read), nên trang Báo cáo điều hành " +
  "không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

/**
 * `/bao-cao` body: the page gate (`report.read`, as `/tong-quan`), then the hook half. Each block,
 * the unit table and the export row add their own key inside (`reportAccess`); the server checks
 * every pair.
 *
 * The full header (period line, period buttons) is drawn by `ReportView` inside the gate, beside the
 * figures it describes. Until the gate opens — session still being read, key missing, session
 * unreadable — the bare title is drawn here instead, so the account still reads which page it is on.
 */
export function ReportPage() {
  const session = usePhien();
  const open = session !== null && quyetDinhTheoKhoa(session, REPORT_READ_PERMISSION).hien;
  return (
    <>
      {!open && (
        <div className="mb-5">
          <ReportHeader />
        </div>
      )}
      <CongQuyen khoa={REPORT_READ_PERMISSION} cauThieuQuyen={MISSING_REPORT_READ}>
        <ReportOverview />
      </CongQuyen>
    </>
  );
}

/**
 * Owns the period and the clock. THE CLOCK IS READ ON AN ACT (mount, a period, a date, "Tải lại", the 2-minute refresh),
 * never during a render: that instant is the page's "tính đến", and every window of the load is
 * computed from it. The six KPI groups come from `/tong-quan`'s own loader (`useDashboardFigures`),
 * without its overdue queues — this page has no "Cần xử lý ngay".
 */
function ReportOverview() {
  const session = usePhien();
  const permissions = session !== null && session.ok ? session.duLieu.permissions : [];
  const access = reportAccess(permissions);

  const [request, setRequest] = useState<ReportRequest>(() => ({
    selection: { kind: DEFAULT_PERIOD_KIND },
    at: new Date().getTime(),
    silent: false,
  }));
  const windows = useMemo(
    () => reportWindows(request.selection, new Date(request.at)),
    [request],
  );
  const figures = useDashboardFigures(windows, request.at, access.blocks, false);

  const [unitAttempt, setUnitAttempt] = useState(0);
  const units = useUnitRows(windows.current, request.at, access.unitTable, unitAttempt);

  // A person's act (period, date, "Tải lại") is never silent: it shows loading, as before.
  const ask = (selection: ReportSelection) => setRequest({ selection, at: new Date().getTime(), silent: false });

  // The prototype refreshes the report every 2 minutes (`useReports.ts` `refetchInterval: 120_000`).
  // Each tick re-asks the same period with the clock read ON THE TICK — figures and unit table both.
  // Keyed on the selection object: a new period (a new object) starts a fresh 120 s; a tick or
  // "Tải lại" reuses the object and keeps the rhythm.
  const selection = request.selection;
  useEffect(() => {
    const id = setInterval(
      () => setRequest({ selection, at: new Date().getTime(), silent: true }),
      REPORT_REFRESH_MS,
    );
    return () => clearInterval(id);
  }, [selection]);

  // A tick is a BACKGROUND refetch, like react-query's: the last settled numbers — and their "tính
  // đến" — stay on screen until every new answer is in, so the page never flashes a skeleton every two
  // minutes. Once settled the new result shows, A FAILURE INCLUDED: stale numbers must never hide an
  // error. `useDashboardFigures` (shared with Tổng quan) is untouched; the hold lives only here.
  const settled = figuresSettled(figures, access.blocks, false) && (!access.unitTable || units !== null);
  const key = `${request.at}|${unitAttempt}`;
  const [lastSettled, setLastSettled] = useState<SettledView | null>(null);
  if (settled && lastSettled?.key !== key) {
    // Render-phase update (React's "storing information from previous renders"): no extra frame.
    setLastSettled({ key, windows, fetchedAt: request.at, figures, units });
  }
  const shown: Omit<SettledView, "key"> =
    request.silent && !settled && lastSettled !== null
      ? lastSettled
      : { windows, fetchedAt: request.at, figures, units };

  // The commune printed on the export files: the runtime configuration the server resolved from
  // `Host` (rule 1 inv. 10) — the value the page header prints. Never a constant.
  const commune = useCauHinhXa();

  return (
    <ReportView
      windows={shown.windows}
      fetchedAt={shown.fetchedAt}
      figures={shown.figures}
      access={access}
      units={shown.units}
      commune={{ displayName: commune.displayName, parentAuthority: commune.parentAuthority }}
      onNamedPeriod={(kind) => ask({ kind })}
      onCustomPeriod={(from, to) => ask({ kind: CUSTOM_PERIOD, from, to })}
      onReload={() => ask(request.selection)}
      onReloadUnits={() => setUnitAttempt((n) => n + 1)}
    />
  );
}

/** `silent` = made by the 2-minute timer, not by a person: the old numbers stay until the new settle. */
type ReportRequest = { selection: ReportSelection; at: number; silent: boolean };

/** What the page last drew with every visible block answered. */
type SettledView = {
  key: string;
  windows: ReportWindows;
  fetchedAt: number;
  figures: BlocksData;
  units: Loaded<UnitRow[]>;
};

/**
 * The unit table's two reads, for the CURRENT window only (the table has no previous-period line,
 * B3). Not called at all without `task.read` + `report.read`: a 403 nobody displays is still a
 * request the server has to refuse.
 */
function useUnitRows(
  current: { start: number; end: number },
  at: number,
  enabled: boolean,
  attempt: number,
): Loaded<UnitRow[]> {
  const { from, to } = toQueryPeriod(current);
  const token = `${from}|${to}|${at}|${enabled}|${attempt}`;
  const [result, setResult] = useState<{ token: string; value: Loaded<UnitRow[]> }>({
    token: "",
    value: null,
  });

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    Promise.all([fetchTaskUnitSummary({ from, to }), layDanhMucBoPhan()]).then(([summary, orgUnits]) => {
      if (!cancelled) setResult({ token, value: unitRowsFrom(summary, orgUnits) });
    });
    return () => {
      cancelled = true;
    };
  }, [token, from, to, enabled]);

  return result.token === token ? result.value : null;
}
