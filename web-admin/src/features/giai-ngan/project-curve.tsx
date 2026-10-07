"use client";

import { useEffect, useState } from "react";

import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { getProjectCurve } from "@/lib/api/du-an";
import type { KetQua } from "@/lib/api/goi";
import type { finance_projectCurveOut } from "@/lib/api/schema.gen";

import { CumulativeChart } from "./cumulative-chart";
import { AfterYearNote } from "./disbursement-overview";

/**
 * §8.3 tab `Biểu đồ` (prototype `BudgetItemDetail.tsx:603-609`): the project's cumulative paid per
 * month against its own plan line, from `GET /api/v1/investment-projects/{id}/disbursement-curve`.
 *
 * Mounted only while the tab is selected (`ProjectRecordTabs`), so it reads on every opening — after a
 * voucher write on the Chứng từ tab, the next look at the chart is that write's curve. The plan line is
 * the SERVER's, from the project's own dates: it may start after January and flatten at the expected
 * completion month (`expected_end_month`), marked by the chart's dashed `Hoàn thành dự kiến` line.
 */
export function ProjectCurvePanel({ projectId }: { projectId: string }) {
  const [reads, setReads] = useState(0);
  const key = `${projectId}|${reads}`;
  const [loaded, setLoaded] = useState<{ key: string; kq: KetQua<finance_projectCurveOut> } | null>(null);

  useEffect(() => {
    let dropped = false;
    getProjectCurve(projectId).then((kq) => {
      if (!dropped) setLoaded({ key, kq });
    });
    return () => {
      dropped = true;
    };
  }, [projectId, key]);

  if (loaded === null || loaded.key !== key) {
    return (
      <div>
        <p role="status" className="an-thi-giac">
          Đang tải biểu đồ giải ngân của dự án…
        </p>
        <Skeleton className="h-56 w-full" />
      </div>
    );
  }
  if (!loaded.kq.ok) {
    return (
      <ErrorState
        title="Chưa tải được biểu đồ giải ngân"
        message={<span role="alert">{loaded.kq.thongBao}</span>}
        onRetry={() => setReads((n) => n + 1)}
      />
    );
  }
  return <ProjectCurveView curve={loaded.kq.duLieu} />;
}

/** Presentational half, renderable in tests without the network. */
export function ProjectCurveView({ curve }: { curve: finance_projectCurveOut }) {
  const end = curve.expected_end_month;
  return (
    <div className="min-w-0" data-project-curve="">
      <CumulativeChart
        points={curve.points}
        height={230}
        caption={`Luỹ kế giải ngân của dự án năm ${curve.year} so với kế hoạch, theo tháng`}
        emptyText="Dự án chưa có kế hoạch vốn năm hay khoản chi nào để vẽ."
        expectedEndMonth={end}
      />
      {/* The dashed `Hoàn thành dự kiến` line now shows this month; the SVG is aria-hidden, so the
          words stay for a screen reader only. */}
      {end !== undefined && end !== null && (
        <p className="an-thi-giac" data-expected-end-note="">
          Đường kế hoạch đạt đủ kế hoạch vốn năm ở T{end}, tháng dự kiến hoàn thành của dự án.
        </p>
      )}
      <AfterYearNote amount={curve.disbursed_after_year} year={curve.year} />
    </div>
  );
}
