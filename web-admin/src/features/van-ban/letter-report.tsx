"use client";

import { CloudOff, Download, Loader2, Plus, RefreshCw } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { bangTraTuKetQua, traTen } from "@/features/cau-hinh/tra-danh-muc";
import type { BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { saveFile } from "@/features/nhiem-vu/save-file";
import { downloadLetterReport } from "@/lib/api/citizen-letter-import";
import { getCitizenLetterReport } from "@/lib/api/citizen-letters";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import type { KetQua } from "@/lib/api/goi";
import type { documents_letterReportOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";
import { PETITION_READ_PERMISSION, REPORT_EXPORT_PERMISSION } from "@/lib/quyen";

import { HeaderOr, LetterImportButton } from "./document-pending";
import { Glyph, plainFrame, type RegisterFrame } from "./document-ui";
import { canBookLetters, letterNumber, letterTypeLabel, reportFigure } from "./letter-display";
import { LETTER_ENTRY_SUBMIT, LetterEntryDialog } from "./letter-entry-dialog";
import { LetterImportDialog } from "./letter-import-dialog";

/** The prototype's words (`PetitionReportPanel.tsx`). */
export function reportTitle(year: number): string {
  return `Tiến độ tiếp nhận và xử lý đơn thư năm ${year}`;
}
export const REPORT_NO_TYPE_ROWS = "Chưa có đơn nào trong năm.";
export const REPORT_NO_UNIT_ROWS = "Chưa có đơn nào được phân công.";
/** A row of `by_unit` with `unit_id: null` — letters held by no unit. The prototype's words (owner request v2 §5). */
export const REPORT_NO_UNIT = "Chưa phân công";

/** The prototype's export words (`PetitionReportPanel.tsx:52-61`). */
export const REPORT_EXPORT_BUTTON = "Xuất Excel";
export function reportExportedText(fileName: string): string {
  return `Đã tải ${fileName}.`;
}
/** The prototype's refusal toast, followed by the server's own sentence (as the Nhiệm vụ export does). */
export const REPORT_EXPORT_REFUSED = "Không xuất được báo cáo.";

/**
 * Whether the session may export: `report.export` AND `petition.read` — the route's two keys
 * (`routes_citizen_letter_import.go`). Convenience only: the server checks both, and audits the export.
 */
export function canExportLetterReport(permissions: readonly string[]): boolean {
  return permissions.includes(REPORT_EXPORT_PERMISSION) && permissions.includes(PETITION_READ_PERMISSION);
}

/**
 * `THEO LOẠI ĐƠN` by Tổng số, largest first — the prototype's order (`service.py:635`,
 * `sorted(..., key=-total)`). A stable sort: a tie keeps the server's order. Ordering only — no figure
 * is touched.
 */
export function byTotalDescending<T extends { total: number }>(rows: readonly T[]): T[] {
  return [...rows].sort((a, b) => b.total - a.total);
}

/** The current year IN VIETNAM — not the machine's zone: 31/12 evening abroad is already 01/01 here. */
export function vietnamYear(now: Date = new Date()): number {
  return Number(new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Ho_Chi_Minh", year: "numeric" }).format(now));
}

/**
 * `Báo cáo` — the prototype's `PetitionReportPanel`, wired to `GET /api/v1/citizen-letter-report?year=`.
 * NO YEAR SELECT (owner decision 08/10/2026, Q8, as the prototype `:40-63`): the report is always the
 * CURRENT year in Vietnam time, read once when the tab opens. The server requires `year`, so the screen
 * still names the year it asked for — in the title.
 *
 * The header keeps the register's buttons (`Vào sổ đơn thư` · `Nhập từ Excel`): in the prototype they
 * belong to the page, so they work on this tab too — a booking or an import from here re-reads the report
 * and the tab's count.
 *
 * `Xuất Excel` downloads THE YEAR SHOWN (the one in the title) — never a server default. Drawn only for
 * `report.export` + `petition.read`, like every gated control of this screen; the outcome is a toast.
 */
export function LetterReport({
  frame = plainFrame,
  onBooked,
}: {
  frame?: RegisterFrame;
  /** A letter was booked or imported — the workspace re-reads the tab's count. */
  onBooked?: () => void;
} = {}) {
  const [year] = useState(() => vietnamYear());
  const [reads, setReads] = useState(0);
  const [result, setResult] = useState<{ year: number; reads: number; answer: KetQua<documents_letterReportOut> } | null>(null);
  const [units, setUnits] = useState<BangTraDanhMuc>({ pha: "dangDoc" });
  const [entry, setEntry] = useState<number | null>(null);
  const [importing, setImporting] = useState<number | null>(null);
  const [exporting, setExporting] = useState(false);

  const session = usePhien();
  const permissions = session !== null && session.ok ? session.duLieu.permissions : [];
  const canBook = canBookLetters(permissions);
  const canExport = canExportLetterReport(permissions);

  function exportReport(): void {
    if (exporting) return;
    setExporting(true);
    downloadLetterReport(year).then((r) => {
      setExporting(false);
      if (!r.ok) {
        toast.error(`${REPORT_EXPORT_REFUSED} ${r.thongBao}`);
        return;
      }
      saveFile(r.duLieu.blob, r.duLieu.fileName);
      toast.success(reportExportedText(r.duLieu.fileName));
    });
  }

  useEffect(() => {
    let dropped = false;
    getCitizenLetterReport(year).then((answer) => {
      if (!dropped) setResult({ year, reads, answer });
    });
    return () => {
      dropped = true;
    };
  }, [year, reads]);

  useEffect(() => {
    let dropped = false;
    layDanhMucBoPhan().then((k) => {
      if (!dropped) setUnits(bangTraTuKetQua(k));
    });
    return () => {
      dropped = true;
    };
  }, []);

  const answer = result !== null && result.year === year && result.reads === reads ? result.answer : null;

  const headerActions = canBook ? (
    <>
      <Button
        type="button"
        variant="primary"
        className="h-10 justify-center px-4 text-[13px]"
        icon={<Glyph icon={Plus} />}
        aria-haspopup="dialog"
        onClick={() => setEntry((n) => (n ?? 0) + 1)}
      >
        {LETTER_ENTRY_SUBMIT}
      </Button>
      <HeaderOr />
      <LetterImportButton onClick={() => setImporting((n) => (n ?? 0) + 1)} />
    </>
  ) : null;

  const body = (
    <>
      <LetterReportView
        year={year}
        answer={answer}
        units={units}
        onReload={() => setReads((n) => n + 1)}
        onExport={canExport ? exportReport : undefined}
        exporting={exporting}
      />
      {entry !== null && (
        <LetterEntryDialog
          key={entry}
          units={units}
          onClose={() => setEntry(null)}
          onBooked={(letter) => {
            setEntry(null);
            toast.success(`Đã vào sổ đơn thư số ${letterNumber(letter.number, letter.year)}.`);
            setReads((n) => n + 1);
            onBooked?.();
          }}
        />
      )}
      {importing !== null && (
        <LetterImportDialog
          key={importing}
          onClose={() => setImporting(null)}
          onImported={() => {
            setReads((n) => n + 1);
            onBooked?.();
          }}
        />
      )}
    </>
  );
  return frame(headerActions, body);
}

/**
 * The report, PURE PRESENTATION (`PetitionReportPanel.tsx:38-230`): title row with `Xuất Excel` (absent
 * without `onExport` — no permission), six figure cards, two tables, the monthly bars. A `null` figure is "—",
 * never 0. The server's seventh figure (`closed_in_processing`) is not drawn: the prototype has six
 * cards, and a card of our own invention is a figure nobody asked to see.
 */
export function LetterReportView({
  year,
  answer,
  units,
  onReload,
  onExport,
  exporting = false,
}: {
  year: number;
  answer: KetQua<documents_letterReportOut> | null;
  units: BangTraDanhMuc;
  onReload?: () => void;
  /** Download the year's .xlsx. Absent = the session may not export: no button is drawn. */
  onExport?: () => void;
  exporting?: boolean;
}) {
  return (
    <section className="flex min-w-0 flex-col gap-5 [&>*]:my-0" aria-labelledby="tieu-de-bao-cao-don-thu">
      <div className="flex flex-wrap items-center gap-3">
        <h2 id="tieu-de-bao-cao-don-thu" className="m-0 text-[15px] font-bold text-navy">
          {reportTitle(year)}
        </h2>
        {onExport !== undefined && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="ml-auto"
            icon={
              exporting ? (
                <Loader2 aria-hidden="true" focusable="false" className="size-3.5 animate-spin" />
              ) : (
                <Download aria-hidden="true" focusable="false" className="size-3.5" />
              )
            }
            disabled={exporting}
            aria-busy={exporting || undefined}
            onClick={onExport}
          >
            {REPORT_EXPORT_BUTTON}
          </Button>
        )}
      </div>

      {answer === null && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải báo cáo đơn thư…
          </p>
          <Skeleton className="h-96 w-full" />
        </>
      )}
      {answer !== null && !answer.ok && (
        <Card className="bg-white">
          <EmptyState
            icon={CloudOff}
            tone="neutral"
            title="Chưa tải được báo cáo đơn thư"
            description={
              <span className="text-danger-600" role="alert">
                {answer.thongBao}
              </span>
            }
            action={
              onReload !== undefined ? (
                <Button type="button" variant="secondary" icon={<Glyph icon={RefreshCw} />} onClick={onReload}>
                  Tải lại
                </Button>
              ) : undefined
            }
          />
        </Card>
      )}
      {answer !== null && answer.ok && <ReportBody report={answer.duLieu} units={units} />}
    </section>
  );
}

function ReportBody({ report, units }: { report: documents_letterReportOut; units: BangTraDanhMuc }) {
  const peak = Math.max(1, ...report.by_month.map((m) => Math.max(m.received, m.resolved)));
  return (
    <>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <ReportCard label="Tiếp nhận trong năm" value={report.received} />
        <ReportCard label="Đã giải quyết" value={report.resolved} tone="leaf" />
        <ReportCard label="Đang xử lý" hint="Gồm cả đơn tồn từ năm trước" value={report.in_progress} />
        <ReportCard label="Quá hạn" value={report.overdue} tone={report.overdue > 0 ? "danger" : undefined} />
        <ReportCard label="Giải quyết đúng hạn" value={report.on_time_percent} suffix="%" />
        <ReportCard label="Số ngày xử lý trung bình" value={report.average_days} suffix=" ngày" />
      </div>

      <ReportPanel title="Theo loại đơn">
        <table className="w-full border-collapse text-[12.5px]">
          <thead>
            <tr className={HEAD_ROW}>
              <th scope="col" className="py-2 pr-3 font-semibold">
                Loại đơn
              </th>
              {["Tổng số", "Đã giải quyết", "Đang xử lý", "Quá hạn"].map((c) => (
                <th key={c} scope="col" className="py-2 pr-3 text-right font-semibold last:pr-0">
                  {c}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {report.by_type.length === 0 ? (
              <tr>
                <td colSpan={5} className="py-6 text-center text-ink-muted">
                  {REPORT_NO_TYPE_ROWS}
                </td>
              </tr>
            ) : (
              byTotalDescending(report.by_type).map((row) => (
                <tr key={row.letter_type} className={BODY_ROW}>
                  <td className="py-2 pr-3 text-navy">{letterTypeLabel(row.letter_type)}</td>
                  <td className="py-2 pr-3 text-right tabular-nums">{row.total}</td>
                  <td className="py-2 pr-3 text-right text-leaf tabular-nums">{row.resolved}</td>
                  <td className="py-2 pr-3 text-right tabular-nums">{row.in_progress}</td>
                  <td className={cn("py-2 text-right tabular-nums", row.overdue > 0 && "font-semibold text-danger")}>
                    {row.overdue}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </ReportPanel>

      <ReportPanel title="Tiến độ xử lý theo đơn vị">
        <table className="w-full border-collapse text-[12.5px]">
          <thead>
            <tr className={HEAD_ROW}>
              <th scope="col" className="py-2 pr-3 font-semibold">
                Bộ phận
              </th>
              {["Tổng số", "Đang xử lý", "Đã giải quyết", "Quá hạn", "Đúng hạn"].map((c) => (
                <th key={c} scope="col" className="py-2 pr-3 text-right font-semibold last:pr-0">
                  {c}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {report.by_unit.length === 0 ? (
              <tr>
                <td colSpan={6} className="py-6 text-center text-ink-muted">
                  {REPORT_NO_UNIT_ROWS}
                </td>
              </tr>
            ) : (
              report.by_unit.map((row) => (
                <tr key={row.unit_id ?? "chua-phan-cong"} className={BODY_ROW}>
                  <td className="py-2 pr-3 text-navy">{unitLabel(units, row.unit_id)}</td>
                  <td className="py-2 pr-3 text-right tabular-nums">{row.total}</td>
                  <td className="py-2 pr-3 text-right tabular-nums">{row.in_progress}</td>
                  <td className="py-2 pr-3 text-right text-leaf tabular-nums">{row.resolved}</td>
                  <td className={cn("py-2 pr-3 text-right tabular-nums", row.overdue > 0 && "font-semibold text-danger")}>
                    {row.overdue}
                  </td>
                  <td className="py-2 text-right font-semibold tabular-nums">{reportFigure(row.on_time_percent, "%")}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </ReportPanel>

      {/* Bars drawn with divs, as the prototype does — no chart library for two series of twelve. */}
      <ReportPanel title="Tiếp nhận và giải quyết theo tháng">
        <div className="flex items-end gap-2" role="img" aria-label={monthsLabel(report)}>
          {report.by_month.map((m) => (
            <div key={m.month} className="min-w-0 flex-1">
              <div className="flex h-28 items-end justify-center gap-0.5">
                <span
                  title={`Tiếp nhận: ${m.received}`}
                  className="w-2.5 rounded-t bg-brand"
                  style={{ height: `${(m.received / peak) * 100}%` }}
                />
                <span
                  title={`Đã giải quyết: ${m.resolved}`}
                  className="w-2.5 rounded-t bg-leaf"
                  style={{ height: `${(m.resolved / peak) * 100}%` }}
                />
              </div>
              <p className="m-0 mt-1 text-center text-[10.5px] text-ink-muted">T{m.month}</p>
            </div>
          ))}
        </div>
        <div className="mt-2 flex gap-4 text-[11.5px] text-ink-muted">
          <span className="flex items-center gap-1.5">
            <span aria-hidden="true" className="size-2.5 rounded-sm bg-brand" />
            Tiếp nhận
          </span>
          <span className="flex items-center gap-1.5">
            <span aria-hidden="true" className="size-2.5 rounded-sm bg-leaf" />
            Đã giải quyết
          </span>
        </div>
      </ReportPanel>
    </>
  );
}

const HEAD_ROW = "border-0 border-b border-solid border-line text-left text-[10.5px] text-ink-muted uppercase";
const BODY_ROW = "border-0 border-b border-solid border-line last:border-b-0";

/** The twelve months as one sentence for a screen reader — the bars alone say nothing to it. */
function monthsLabel(report: documents_letterReportOut): string {
  return report.by_month.map((m) => `Tháng ${m.month}: tiếp nhận ${m.received}, đã giải quyết ${m.resolved}`).join("; ");
}

/** The unit's name from the org chart; its id when the chart does not know it (or failed to load). */
function unitLabel(units: BangTraDanhMuc, id: string | null): string {
  if (id === null || id === "") return REPORT_NO_UNIT;
  const found = traTen(units, id);
  if (found.loai === "coTen") return found.ten;
  if (found.loai === "dangDoc") return "Đang tải…";
  return id;
}

/** The prototype's figure card (`PetitionReportPanel.tsx:233-264`): the number on top, the label under it. */
function ReportCard({
  label,
  value,
  hint,
  suffix = "",
  tone,
}: {
  label: string;
  value: number | null;
  hint?: string;
  suffix?: string;
  tone?: "leaf" | "danger";
}) {
  return (
    <Card className="bg-white p-3.5">
      <p
        className={cn(
          "m-0 text-[24px] font-bold tabular-nums",
          tone === "leaf" && "text-leaf",
          tone === "danger" && "text-danger",
          tone === undefined && "text-navy",
        )}
      >
        {reportFigure(value, suffix)}
      </p>
      <p className="m-0 mt-0.5 text-[12px] text-ink-muted">{label}</p>
      {hint !== undefined && <p className="m-0 mt-0.5 text-[11px] text-ink-muted/80">{hint}</p>}
    </Card>
  );
}

function ReportPanel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card as="section" className="bg-white p-4">
      <h3 className="m-0 mb-3 text-[11.5px] font-bold tracking-wide text-ink-muted uppercase">{title}</h3>
      {children}
    </Card>
  );
}
