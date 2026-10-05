"use client";

import { Download, FileSpreadsheet, ListChecks, Plus, ScanLine } from "lucide-react";
import type { KeyboardEvent, ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field } from "@/components/ui/field";
import { PendingButton, PendingMarker } from "@/components/ui/pending-feature";
import { Segmented } from "@/components/ui/segmented";
import { Tab, TabList } from "@/components/ui/tabs";

import { PHAN_CHUA_DUNG, type PendingPart } from "./nhan-van-ban";

/**
 * The parts of the Văn bản & Đơn thư screen that cannot be built yet, drawn at the PROTOTYPE's
 * position (`vigov-require/apps/admin/src/components/documents/DocumentWorkspace.tsx`, ADR 0068 lần 5)
 * as the control they will be, disabled, with a "?" (ADR 0068 §14). NOTHING HERE CALLS A SERVER:
 * there is no handler to pass, on purpose — and no figure, row or count is drawn where the data does
 * not exist (never fake data, lần 5 #5).
 *
 * Descriptions come from `PHAN_CHUA_DUNG` by name, never a second sentence written here: two copies
 * of one reason drift, and the stale one is what a staff member reads.
 */
function part(name: string): PendingPart {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === name);
  // A renamed entry must fail loudly in tests, not draw a "?" that explains nothing.
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry named "${name}"`);
  return found;
}

/* ---- the tab bar ------------------------------------------------------------------------------ */

/**
 * The prototype's four tabs, in its order: the registers first, then `Đơn thư công dân`, then
 * `Báo cáo`. The prototype HIDES its `Văn bản đến` tab because ITS demo commune asked (17/09/2026);
 * that request does not apply here, and our two working registers each get their tab. The prototype's
 * counts in brackets are not drawn: our registers are paged, so the count of the loaded page is not
 * the register's count, and the petition register does not exist.
 */
export const DOCUMENT_TABS = [
  { id: "incoming", label: "Văn bản đến" },
  { id: "outgoing", label: "Văn bản đi" },
  { id: "petitions", label: "Đơn thư công dân" },
  { id: "report", label: "Báo cáo" },
] as const;

export type DocumentTabId = (typeof DOCUMENT_TABS)[number]["id"];

export const DOCUMENT_PANEL_ID = "khung-tab-van-ban";

export function documentTabDomId(id: DocumentTabId): string {
  return `tab-van-ban-${id}`;
}

/**
 * The tab bar under the page header. ALL FOUR TABS ARE SELECTABLE — `Đơn thư công dân` and `Báo cáo`
 * open the prototype's layout as disabled controls with "?", so the staff member sees what the tab
 * will hold and why it is not there yet, instead of a dead tab. Roving focus: Left/Right/Home/End.
 */
export function DocumentTabBar({
  selected,
  onSelect,
}: {
  selected: DocumentTabId;
  onSelect: (id: DocumentTabId) => void;
}) {
  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const at = DOCUMENT_TABS.findIndex((t) => t.id === selected);
    const last = DOCUMENT_TABS.length - 1;
    const next =
      e.key === "ArrowRight" ? (at === last ? 0 : at + 1)
      : e.key === "ArrowLeft" ? (at === 0 ? last : at - 1)
      : e.key === "Home" ? 0
      : e.key === "End" ? last
      : null;
    if (next === null) return;
    e.preventDefault();
    // Focus follows selection; the page moves it once the selected tab is rendered.
    onSelect(DOCUMENT_TABS[next]!.id);
  }

  return (
    <TabList aria-label="Phần của màn Văn bản & đơn thư" onKeyDown={onKeyDown}>
      {DOCUMENT_TABS.map((t) => (
        <Tab
          key={t.id}
          id={documentTabDomId(t.id)}
          selected={t.id === selected}
          aria-controls={DOCUMENT_PANEL_ID}
          tabIndex={t.id === selected ? 0 : -1}
          onClick={() => onSelect(t.id)}
        >
          {t.label}
        </Tab>
      ))}
    </TabList>
  );
}

/* ---- header actions --------------------------------------------------------------------------- */

/** The prototype's "or" between two ways of entering one register. */
export function HeaderOr() {
  return <span className="text-[12px] text-ink-500">hoặc</span>;
}

/** Spec §2 / §3.4: the "Quét & OCR" button, disabled — sending scans out is the authority's call. */
export function ScanOcrButton() {
  return <PendingButton info={part("Quét & OCR")} icon={<ScanLine aria-hidden="true" />} />;
}

/**
 * The prototype's header pair of the petition register — `[+ Vào sổ đơn thư]  hoặc  [Nhập từ Excel]`
 * — both disabled with the register's one reason.
 */
export function PetitionHeaderActions() {
  const info = part("Đơn thư công dân");
  return (
    <>
      <PendingButton info={info} variant="primary" icon={<Plus aria-hidden="true" />}>
        Vào sổ đơn thư
      </PendingButton>
      <HeaderOr />
      <PendingButton info={info} variant="outline" icon={<FileSpreadsheet aria-hidden="true" />}>
        Nhập từ Excel
      </PendingButton>
    </>
  );
}

/* ---- pieces of the incoming register ---------------------------------------------------------- */

/**
 * The prototype's `ScopeFilter`, first in the incoming filter row: `Toàn xã` is what the register
 * shows today (selected, nothing to change), the two personal scopes are disabled with "?".
 */
export function IncomingScopeFilter() {
  const info = part("Lọc Giao cho tôi / Liên quan đến tôi");
  return (
    <Segmented
      mode="buttons"
      legend="Lọc nhanh theo người xử lý"
      name="pham-vi-van-ban-den"
      value=""
      onChange={() => {}}
      options={[
        { value: "", label: "Toàn xã" },
        { value: "assigned", label: "Giao cho tôi", pending: info },
        { value: "involved", label: "Liên quan đến tôi", pending: info },
      ]}
    />
  );
}

/** Right end of the incoming filter row: `Nhập hàng loạt từ Excel` and `Xuất sổ {năm}`, disabled. */
export function IncomingRowLinks({ year }: { year: number }) {
  return (
    <div className="ml-auto flex flex-wrap items-center gap-2">
      <PendingButton
        info={part("Nhập hàng loạt từ Excel")}
        variant="ghost"
        size="sm"
        icon={<FileSpreadsheet aria-hidden="true" />}
      />
      <PendingButton info={part("Xuất sổ văn bản đến")} variant="ghost" size="sm" icon={<Download aria-hidden="true" />}>
        Xuất sổ {year}
      </PendingButton>
    </div>
  );
}

/** The detail's `Chuyển thành nhiệm vụ` (prototype `DocumentDetailDrawer`), disabled. */
export function RaiseTaskButton() {
  return (
    <PendingButton info={part("Chuyển thành nhiệm vụ")} variant="primary" icon={<ListChecks aria-hidden="true" />} />
  );
}

/**
 * Target statuses of the incoming-document lifecycle the customer settled on 30/09/2026 (Nghị định
 * 30/2020: Đã vào sổ → Chờ trình/phân luồng → Đã chuyển xử lý → Đang xử lý → Hoàn thành) — the four
 * after "Đã vào sổ". NOT the petition-letter labels the spec drew (§3.5): that decision says incoming
 * documents do not borrow them. Labels only — no status code is sent anywhere from here.
 */
const STATUS_TARGETS = ["Chờ trình/phân luồng", "Đã chuyển xử lý", "Đang xử lý", "Hoàn thành"] as const;

/**
 * The prototype's status strip: one chip per step, `flex-1` with a 6.5rem floor, "chuyển sang" under
 * the label — all disabled, ONE "?" for the row (four identical markers would say the same sentence
 * four times and crowd the row at 320px).
 */
export function StatusChangeRow() {
  return (
    <div role="group" aria-labelledby="nhan-chuyen-trang-thai" className="flex min-w-0 flex-col gap-2" data-pending="">
      <div className="flex items-center gap-1.5">
        <span id="nhan-chuyen-trang-thai" className="text-xs font-semibold text-ink-500">
          Chuyển trạng thái
        </span>
        <PendingMarker info={part("Chuyển trạng thái văn bản đến")} />
      </div>
      <div className="flex min-w-0 flex-wrap items-stretch gap-1.5">
        {STATUS_TARGETS.map((s) => (
          // The prototype draws the status above "chuyển sang"; the name reads in speaking order.
          <Button
            key={s}
            type="button"
            variant="secondary"
            disabled
            aria-label={`Chuyển sang ${s}`}
            className="h-auto min-w-[6.5rem] flex-1 flex-col items-start gap-0.5 px-2.5 py-1.5 text-left"
          >
            <span className="text-[13px] font-semibold whitespace-normal">{s}</span>
            <span className="text-[11px] font-normal text-ink-500">chuyển sang</span>
          </Button>
        ))}
      </div>
    </div>
  );
}

/* ---- the petition tab ------------------------------------------------------------------------- */

/** The prototype's petition register columns (`PetitionTable.tsx`), in its order. */
export const PETITION_COLUMNS = [
  "Số",
  "Ngày nhận",
  "Người gửi",
  "Loại đơn",
  "Nội dung",
  "Đang giữ",
  "Số ngày xử lý",
  "Hạn giải quyết",
  "Trạng thái",
] as const;

/**
 * `Đơn thư công dân`: the prototype's tab body — scope filter + one line about the register, the
 * four-filter row, the table — every control disabled, and the table body is the ONE reason sentence,
 * never an empty-register line (there is no register to be empty).
 *
 * The prototype's second sentence ("Hệ thống nhắc khi một người gửi lại đơn có nội dung tương tự.")
 * is not drawn: it describes a duplicate check that does not exist.
 */
export function PendingPetitionRegister() {
  const info = part("Đơn thư công dân");
  return (
    <section aria-labelledby="tieu-de-don-thu" className="flex min-w-0 flex-col gap-3 [&>*]:my-0" data-pending="">
      <h2 id="tieu-de-don-thu" className="an-thi-giac">
        Sổ đơn thư công dân
      </h2>
      <div className="flex min-w-0 flex-wrap items-center gap-3">
        <Segmented
          mode="buttons"
          legend="Lọc nhanh theo người xử lý"
          name="pham-vi-don-thu"
          value=""
          disabled
          onChange={() => {}}
          options={[
            { value: "", label: "Toàn xã" },
            { value: "assigned", label: "Giao cho tôi" },
            { value: "involved", label: "Liên quan đến tôi" },
          ]}
        />
        <p className="m-0 max-w-lg text-[13px] text-ink-500">
          Sổ theo dõi đơn khiếu nại, tố cáo, kiến nghị và đề nghị của công dân.
        </p>
      </div>

      <div className="flex min-w-0 flex-wrap items-end gap-2.5">
        <Field label="Từ ngày" htmlFor="don-thu-tu-ngay" grow="auto">
          <input id="don-thu-tu-ngay" type="date" disabled />
        </Field>
        <Field label="Đến ngày" htmlFor="don-thu-den-ngay" grow="auto">
          <input id="don-thu-den-ngay" type="date" disabled />
        </Field>
        {(
          [
            ["don-thu-don-vi", "Lọc theo đơn vị chủ quản", "Tất cả đơn vị"],
            ["don-thu-can-bo", "Lọc theo cán bộ chủ quản", "Tất cả cán bộ"],
            ["don-thu-trang-thai", "Lọc theo trạng thái", "Tất cả trạng thái"],
          ] as const
        ).map(([id, label, first]) => (
          <Field key={id} label={label} htmlFor={id} kind="select" hideLabel grow="auto">
            <select id={id} disabled>
              <option>{first}</option>
            </select>
          </Field>
        ))}
        <span className="relative mb-2 inline-flex">
          <PendingMarker info={info} />
        </span>
      </div>

      <Card className="overflow-hidden">
        <div className="bang-cuon overflow-x-auto rounded-none border-0 shadow-none">
          <table className="bang-danh-muc min-w-[900px]">
            <caption className="an-thi-giac">Sổ đơn thư công dân</caption>
            <thead>
              <tr>
                {PETITION_COLUMNS.map((c) => (
                  <th key={c} scope="col">
                    {c}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              <tr>
                <td colSpan={PETITION_COLUMNS.length} className="p-6 text-center text-[13px] text-ink-500">
                  {info.viSao}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </Card>
    </section>
  );
}

/* ---- the report tab --------------------------------------------------------------------------- */

const REPORT_FIGURES = [
  { label: "Tiếp nhận trong năm" },
  { label: "Đã giải quyết" },
  { label: "Đang xử lý", hint: "Gồm cả đơn tồn từ năm trước" },
  { label: "Quá hạn" },
  { label: "Giải quyết đúng hạn" },
  { label: "Số ngày xử lý trung bình" },
] as const;

const REPORT_TABLES = [
  { title: "Theo loại đơn", columns: ["Loại đơn", "Tổng số", "Đã giải quyết", "Đang xử lý", "Quá hạn"] },
  {
    title: "Tiến độ xử lý theo đơn vị",
    columns: ["Bộ phận", "Tổng số", "Đang xử lý", "Đã giải quyết", "Quá hạn", "Đúng hạn"],
  },
] as const;

/**
 * `Báo cáo`: the prototype's `PetitionReportPanel` frame — heading + `Xuất Excel`, six figure cards,
 * two tables, the monthly panel. Every figure is "—" and every table body is the reason sentence:
 * a 0 would be a figure, and the commune would report it upward.
 */
export function PendingPetitionReport({ year }: { year: number }) {
  const info = part("Báo cáo");
  return (
    <section aria-labelledby="tieu-de-bao-cao-don-thu" className="flex min-w-0 flex-col gap-5 [&>*]:my-0" data-pending="">
      <div className="flex flex-wrap items-center gap-3">
        <h2 id="tieu-de-bao-cao-don-thu" className="m-0 text-[15px] font-bold text-ink-900">
          Tiến độ tiếp nhận và xử lý đơn thư năm {year}
        </h2>
        <PendingMarker info={info} />
        <PendingButton
          info={info}
          variant="outline"
          size="sm"
          icon={<Download aria-hidden="true" />}
          className="ml-auto"
        >
          Xuất Excel
        </PendingButton>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {REPORT_FIGURES.map((f) => (
          <Card key={f.label} className="p-3.5">
            <p className="m-0 text-[24px] font-bold text-ink-400 tabular-nums" aria-hidden="true">
              —
            </p>
            <p className="m-0 mt-0.5 text-[12px] text-ink-500">{f.label}</p>
            {"hint" in f && <p className="m-0 mt-0.5 text-[11px] text-ink-500">{f.hint}</p>}
          </Card>
        ))}
      </div>

      {REPORT_TABLES.map((t) => (
        <ReportPanel key={t.title} title={t.title}>
          <div className="overflow-x-auto">
            <table className="w-full border-collapse text-[13px]">
              <thead>
                <tr className="border-b border-line text-left text-[11px] text-ink-500 uppercase">
                  {t.columns.map((c, i) => (
                    <th key={c} scope="col" className={i === 0 ? "py-2 pr-3 font-semibold" : "py-2 pr-3 text-right font-semibold"}>
                      {c}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td colSpan={t.columns.length} className="py-6 text-center text-ink-500">
                    {info.viSao}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </ReportPanel>
      ))}

      <ReportPanel title="Tiếp nhận và giải quyết theo tháng">
        <p className="m-0 py-6 text-center text-[13px] text-ink-500">{info.viSao}</p>
      </ReportPanel>
    </section>
  );
}

function ReportPanel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card as="section" className="p-4">
      <h3 className="m-0 mb-3 text-[11.5px] font-bold tracking-wide text-ink-500 uppercase">{title}</h3>
      {children}
    </Card>
  );
}
