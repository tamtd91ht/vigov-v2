"use client";

import { useEffect, useMemo, useState } from "react";

import { useCauHinhXa } from "@/components/cau-hinh-xa";
import { useDashboardFigures } from "@/features/dashboard/overview";
import { DEFAULT_PERIOD_KIND, toQueryPeriod } from "@/features/dashboard/period";
import type { Loaded } from "@/features/dashboard/view";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { fetchTaskUnitSummary } from "@/lib/api/dashboard";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import { quyetDinhTheoKhoa, REPORT_READ_PERMISSION } from "@/lib/quyen";

import { reportAccess } from "./report-access";
import { CUSTOM_PERIOD, reportWindows } from "./report-period";
import type { ReportSelection } from "./report-period";
import { ReportHeader, ReportView } from "./report-view";
import { unitRowsFrom } from "./unit-table";
import type { UnitRow } from "./unit-table";

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
 * Owns the period and the clock. THE CLOCK IS READ ON AN ACT (mount, a period, a date, "Tải lại"),
 * never during a render: that instant is the page's "tính đến", and every window of the load is
 * computed from it. The six KPI groups come from `/tong-quan`'s own loader (`useDashboardFigures`),
 * without its overdue queues — this page has no "Cần xử lý ngay".
 */
function ReportOverview() {
  const session = usePhien();
  const permissions = session !== null && session.ok ? session.duLieu.permissions : [];
  const access = reportAccess(permissions);

  const [request, setRequest] = useState<{ selection: ReportSelection; at: number }>(() => ({
    selection: { kind: DEFAULT_PERIOD_KIND },
    at: new Date().getTime(),
  }));
  const windows = useMemo(
    () => reportWindows(request.selection, new Date(request.at)),
    [request],
  );
  const figures = useDashboardFigures(windows, request.at, access.blocks, false);

  const [unitAttempt, setUnitAttempt] = useState(0);
  const units = useUnitRows(windows.current, request.at, access.unitTable, unitAttempt);

  const ask = (selection: ReportSelection) => setRequest({ selection, at: new Date().getTime() });
  // The commune printed on the export files: the runtime configuration the server resolved from
  // `Host` (rule 1 inv. 10) — the value the page header prints. Never a constant.
  const commune = useCauHinhXa();

  return (
    <ReportView
      windows={windows}
      fetchedAt={request.at}
      figures={figures}
      access={access}
      units={units}
      commune={{ displayName: commune.displayName, parentAuthority: commune.parentAuthority }}
      onNamedPeriod={(kind) => ask({ kind })}
      onCustomPeriod={(from, to) => ask({ kind: CUSTOM_PERIOD, from, to })}
      onReload={() => ask(request.selection)}
      onReloadUnits={() => setUnitAttempt((n) => n + 1)}
    />
  );
}

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
