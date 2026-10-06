"use client";

import { ChevronDown, ChevronRight, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import { BUSY_DELETING, BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { listFundingSources } from "@/lib/api/funding-sources";
import { suaDuAn, themDuAn, xoaDuAn } from "@/lib/api/giai-ngan";
import type {
  finance_duAnRa,
  finance_fundingSourceOut,
  finance_hangMucRa,
  finance_projectAllocationOut,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";
import { compactDong } from "@/lib/compact-dong";

import { nhanTien } from "./nhan-du-an";
import {
  allocationBlocksSave,
  docSoTien,
  FORM_DU_AN_TRONG,
  MA_DU_AN_TOI_DA,
  MO_TA_DU_AN_TOI_DA,
  summarizeAllocations,
  TEN_DU_AN_TOI_DA,
  thanSuaDuAn,
  thanThemDuAn,
  type AllocationRow,
  type AllocationSummary,
  type GiaTriFormDuAn,
} from "./nhan-ghi-giai-ngan";
import { AutoCodePending, UnitAndOfficerPending } from "./pending-parts";
import { Glyph } from "./project-ui";

/**
 * Ba tuyến GHI của dự án đầu tư — thêm, sửa, xoá mềm kèm lý do — in the prototype's composition
 * (ADR 0068 lần 5): `+ Thêm dự án` opens a centred dialog (`BudgetItemForm.tsx:695-729`, 64rem),
 * `Sửa dự án` opens the SAME form inline above the project card (`BudgetItemDetail.tsx:131-145`).
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * HAI KHOÁ, VÀ CHÚNG KHÔNG PHẢI MỘT: `budget.update` cho THÊM và SỬA, `budget.confirm` cho XOÁ.
 * Bảng phân quyền của hợp đồng chia đôi đúng như thế, và đó là cách xã tách người nhập liệu khỏi
 * người chịu trách nhiệm (§8.2, `06-giai-ngan.md:202`). Một tài khoản chỉ có `budget.update` thấy
 * nút Sửa và KHÔNG thấy nút Gỡ — nếu hai nút cùng hiện ra thì cổng đã gắn nhầm, và không có gì
 * khác trên màn hình nói ra điều đó vì máy chủ chỉ trả 403 vào đúng lúc bấm.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * GỠ DỰ ÁN STAYS ON THE DETAIL PAGE, ONE PROJECT AT A TIME, WITH A REASON. The prototype removes
 * projects by bulk selection on the list with a plain confirm; the server requires `reason` on each
 * removal (rule 7: `deleted_at` · `deleted_by` · `delete_reason`), and one reason typed once for N
 * archival records is not N reasons. The removal confirm is a dialog, as the prototype's is.
 */

/** The formatted amount under a money field (prototype: `formatDong(digits)` as the hint). */
function amountHint(typed: string): string | undefined {
  const read = docSoTien(typed);
  return read.loai === "so" ? nhanTien(read.dong) : undefined;
}

/**
 * Biểu mẫu dự án, dùng cho cả THÊM và SỬA — prototype field order: the three required fields and the
 * code in a two-column grid, the funding list, then "Thông tin thêm (không bắt buộc)" folded.
 *
 * `khoaChongTrung` SINH LÚC MỞ BIỂU MẪU, không lúc gửi: bấm lại sau một lỗi mạng phải dùng LẠI đúng
 * khoá ấy, vì lần gửi đầu có thể đã tới máy chủ và đã cấp một mã dự án. Biểu mẫu SỬA không cần khoá
 * (hợp đồng không đòi), nhưng vẫn nhận cùng chữ ký để hai lối gọi không rẽ nhánh ở đây.
 *
 * THE FOLDED PART STAYS IN THE DOM (`hidden`), so what was typed there survives folding it — the
 * prototype keeps that state too — and a validation sentence about a date still has its field.
 */
export function FormDuAn({
  tieuDeForm,
  budgetYear,
  giaTriDau,
  maChiDoc,
  danhMuc,
  fundingCatalogue,
  currentAllocations,
  dangGui,
  loi,
  huy,
  luu,
}: {
  /** The form's accessible name; the visible title belongs to the dialog / panel around it. */
  tieuDeForm: string;
  /** Year in the amount label (`Số tiền bố trí năm 2026`). */
  budgetYear: number;
  giaTriDau: GiaTriFormDuAn;
  /** Mã dự án khi đang SỬA. Hiện để ĐỌC: mã đã cấp thì không đánh lại (luật 7, cấm #4). */
  maChiDoc?: string;
  danhMuc: readonly finance_hangMucRa[];
  /** The commune's funding sources for `budgetYear`, loaded by the dialog / panel around the form. */
  fundingCatalogue: FundingCatalogue;
  /** When editing: the project's allocations as read, so a line keeps its name whatever the catalogue holds. */
  currentAllocations?: readonly finance_projectAllocationOut[];
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (gt: GiaTriFormDuAn, khoaChongTrung: string) => void;
}) {
  const [gt, datGT] = useState<GiaTriFormDuAn>(giaTriDau);
  const [khoaChongTrung] = useState(khoaChongTrungMoi);
  const isEdit = maChiDoc !== undefined;
  // Prototype: folded when adding, open when editing (`BudgetItemForm.tsx:128,166`).
  const [showMore, setShowMore] = useState(isEdit);

  // Danh mục hạng mục ship RỖNG cho tới khi có bước khởi tạo xã, và `category_id` là bắt buộc — một
  // biểu mẫu gửi đi lúc ấy chỉ nhận 404 "không tìm thấy hạng mục kế hoạch vốn này trong xã". Nói
  // trước, thay vì để cán bộ gõ xong cả biểu mẫu rồi mới biết.
  const thieuDanhMuc = danhMuc.length === 0;

  // Over-allocation blocks saving HERE so the clerk sees why before pressing; the server refuses the
  // same total with 409 `allocation_exceeds_plan` and stays the real check.
  const allocationSummary = summarizeAllocations(gt.keHoachVon, gt.allocations);
  const allocationBlocked = allocationBlocksSave(allocationSummary);

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (thieuDanhMuc || allocationBlocked) return;
    luu(gt, khoaChongTrung);
  }

  return (
    <form onSubmit={guiNgay} aria-label={tieuDeForm} className="flex min-w-0 flex-col gap-3.5">
      {isEdit && (
        <p className="m-0 text-[13px] text-ink-500">
          Mã dự án: <span className="ma-muc text-ink-700">{maChiDoc}</span> — mã đã cấp thì không
          đánh lại, nên ô này chỉ để đọc.
        </p>
      )}

      {thieuDanhMuc && (
        <Notice tone="info">
          Danh mục hạng mục kế hoạch vốn của xã đang rỗng, mà mỗi dự án phải thuộc đúng một hạng
          mục. Cần khai hạng mục ở màn Cấu hình (quyền “Quản lý danh mục”) trước khi thêm dự án.
        </Notice>
      )}

      {/* Category, code, name and amount in one two-column grid from md (prototype `:297`). */}
      <div className="grid min-w-0 gap-3.5 md:grid-cols-2">
        <Field
          label="Hạng mục *"
          htmlFor="hang-muc-du-an"
          kind="select"
          grow="auto"
          className="min-w-0"
          hint="Báo cáo tiến độ cộng dồn theo hạng mục, nên mỗi dự án thuộc đúng một hạng mục."
        >
          <select
            id="hang-muc-du-an"
            value={gt.hangMucID}
            disabled={thieuDanhMuc}
            onChange={(e) => datGT({ ...gt, hangMucID: e.target.value })}
          >
            <option value="">— Chọn hạng mục —</option>
            {danhMuc.map((h) => (
              <option key={h.id} value={h.id}>
                {h.label}
              </option>
            ))}
          </select>
        </Field>

        {!isEdit && (
          <div className="flex min-w-0 flex-col gap-1.5">
            {/* Input and `Tự sinh mã` on one row (prototype `:320-338`). The checkbox is a disabled
                "?" placeholder: no code is generated until its format is agreed (rule 7). */}
            <div className="flex min-w-0 items-end gap-3">
              <Field label="Mã dự án *" htmlFor="ma-du-an" grow="auto" className="min-w-0 flex-1">
                <input
                  id="ma-du-an"
                  name="ma-du-an"
                  value={gt.ma}
                  maxLength={MA_DU_AN_TOI_DA}
                  autoComplete="off"
                  placeholder="DA01"
                  onChange={(e) => datGT({ ...gt, ma: e.target.value })}
                />
              </Field>
              <AutoCodePending />
            </div>
            {/* MÃ ĐÃ CẤP THÌ KHÔNG CẤP LẠI, KỂ CẢ KHI DỰ ÁN MANG MÃ ẤY ĐÃ RÚT KHỎI DANH SÁCH — nói
                trước, vì nếu không thì câu 409 của máy chủ đọc như một lỗi trước mặt người vừa xem
                hết danh sách và không thấy mã ấy ở đâu. */}
            <p className="m-0 text-xs text-ink-500">
              Chỉ gồm chữ cái, chữ số và dấu gạch nối. Hệ thống chưa tự sinh mã. Mã đã cấp thì không
              cấp lại, kể cả khi dự án mang mã đó đã rút khỏi danh sách.
            </p>
          </div>
        )}

        <Field label="Tên dự án *" htmlFor="ten-du-an" grow="auto" className="min-w-0">
          <input
            id="ten-du-an"
            name="ten-du-an"
            value={gt.ten}
            maxLength={TEN_DU_AN_TOI_DA}
            autoComplete="off"
            placeholder="Bê tông hoá đường trục chính thôn Hà Lam"
            onChange={(e) => datGT({ ...gt, ten: e.target.value })}
          />
        </Field>

        <Field
          label={`Số tiền bố trí năm ${budgetYear} (đồng) *`}
          htmlFor="ke-hoach-von-du-an"
          grow="auto"
          className="min-w-0"
          hint={amountHint(gt.keHoachVon)}
        >
          {/* Ô CHỮ, KHÔNG PHẢI `type=number`: một ô số của trình duyệt nhận cả `1e9` và cả dấu
              phẩy thập phân theo vùng miền, rồi đưa xuống một giá trị không phải thứ người ta gõ.
              `docSoTien` đọc đúng một khuôn và từ chối phần còn lại bằng một câu. */}
          <input
            id="ke-hoach-von-du-an"
            name="ke-hoach-von-du-an"
            value={gt.keHoachVon}
            inputMode="numeric"
            autoComplete="off"
            placeholder="7.500.000.000"
            className="tabular-nums"
            onChange={(e) => datGT({ ...gt, keHoachVon: e.target.value })}
          />
        </Field>
      </div>

      <FundingAllocationList
        year={budgetYear}
        catalogue={fundingCatalogue}
        rows={gt.allocations}
        summary={allocationSummary}
        currentAllocations={currentAllocations}
        onChange={(allocations) => datGT({ ...gt, allocations })}
      />

      <button
        type="button"
        aria-expanded={showMore}
        aria-controls="thong-tin-them-du-an"
        onClick={() => setShowMore((v) => !v)}
        className="flex w-fit cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[13px] text-ink-500 hover:text-ink-900"
      >
        <Glyph icon={showMore ? ChevronDown : ChevronRight} className="size-4" />
        Thông tin thêm (không bắt buộc)
      </button>

      <div
        id="thong-tin-them-du-an"
        hidden={!showMore}
        className="flex min-w-0 flex-col gap-3 rounded-xl border border-line p-3"
      >
        <Field
          label="Tổng mức được duyệt cả dự án (đồng)"
          htmlFor="tong-muc-du-an"
          grow="auto"
          className="min-w-0"
          hint={amountHint(gt.tongMucDuyet) ?? "Để trống thì lấy bằng số tiền bố trí năm nay."}
        >
          <input
            id="tong-muc-du-an"
            name="tong-muc-du-an"
            value={gt.tongMucDuyet}
            inputMode="numeric"
            autoComplete="off"
            placeholder="11.500.000.000"
            className="tabular-nums"
            onChange={(e) => datGT({ ...gt, tongMucDuyet: e.target.value })}
          />
        </Field>

        <div className="grid min-w-0 gap-3 sm:grid-cols-2">
          <UnitAndOfficerPending />
        </div>

        <div className="grid min-w-0 gap-3 sm:grid-cols-2">
          <Field label="Ngày khởi công" htmlFor="ngay-khoi-cong" grow="auto" className="min-w-0">
            <input
              id="ngay-khoi-cong"
              name="ngay-khoi-cong"
              type="date"
              value={gt.ngayKhoiCong}
              onChange={(e) => datGT({ ...gt, ngayKhoiCong: e.target.value })}
            />
          </Field>

          <Field label="Ngày hoàn thành" htmlFor="ngay-hoan-thanh" grow="auto" className="min-w-0">
            <input
              id="ngay-hoan-thanh"
              name="ngay-hoan-thanh"
              type="date"
              value={gt.ngayHoanThanh}
              onChange={(e) => datGT({ ...gt, ngayHoanThanh: e.target.value })}
            />
          </Field>
        </div>

        {/* HAI MỐC KHÁC NHAU — thời hạn giải ngân không phải ngày hoàn thành công trình. */}
        <Field
          label="Thời hạn giải ngân"
          htmlFor="thoi-han-giai-ngan"
          grow="auto"
          className="min-w-0"
          hint={
            "Mốc phải tiêu hết phần vốn của năm. Khác với ngày hoàn thành công trình: công trình " +
            "xong tháng 3 vẫn có thể phải giải ngân trước 31/12. Để trống thì lấy mặc định 31/12."
          }
        >
          <input
            id="thoi-han-giai-ngan"
            name="thoi-han-giai-ngan"
            type="date"
            value={gt.thoiHanGiaiNgan}
            onChange={(e) => datGT({ ...gt, thoiHanGiaiNgan: e.target.value })}
          />
        </Field>

        <Field label="Mô tả" htmlFor="mo-ta-du-an" grow="auto" className="min-w-0">
          <textarea
            id="mo-ta-du-an"
            name="mo-ta-du-an"
            rows={2}
            value={gt.moTa}
            maxLength={MO_TA_DU_AN_TOI_DA}
            className="py-2"
            onChange={(e) => datGT({ ...gt, moTa: e.target.value })}
          />
        </Field>
      </div>

      {loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}

      <div className="flex justify-end gap-2 pt-1">
        <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
          Huỷ
        </Button>
        <Button
          type="submit"
          variant="primary"
          disabled={dangGui || thieuDanhMuc || allocationBlocked}
          aria-busy={dangGui || undefined}
        >
          <BusyLabel busy={dangGui} label={isEdit ? "Lưu dự án" : "Thêm dự án"} busyText={BUSY_SAVING} />
        </Button>
      </div>
    </form>
  );
}

/** The year's funding-source catalogue as the form sees it. */
export type FundingCatalogue =
  | { readonly phase: "loading" }
  | { readonly phase: "error"; readonly message: string }
  | { readonly phase: "ready"; readonly items: readonly finance_fundingSourceOut[] };

/**
 * GET /api/v1/funding-sources?year= for the form's selects and per-row hints. Loaded by the dialog /
 * panel, not by `FormDuAn`, so the form renders without the network. Kept with the year that produced
 * it: another year's "còn X chưa phân bổ" under this year's form is a wrong figure that looks right.
 */
function useFundingCatalogue(year: number): FundingCatalogue {
  const [loaded, setLoaded] = useState<{ year: number; catalogue: FundingCatalogue } | null>(null);
  useEffect(() => {
    let dropped = false;
    listFundingSources(year).then((result) => {
      if (dropped) return;
      setLoaded({
        year,
        catalogue: result.ok
          ? { phase: "ready", items: result.duLieu.items }
          : { phase: "error", message: result.thongBao },
      });
    });
    return () => {
      dropped = true;
    };
  }, [year]);
  return loaded !== null && loaded.year === year ? loaded.catalogue : { phase: "loading" };
}

/** Form rows from the project as read: raw amounts, for the same reason as `keHoachVon`. */
function allocationRowsOf(allocations: readonly finance_projectAllocationOut[] | undefined): AllocationRow[] {
  return (allocations ?? []).map((a, i) => ({ key: i, sourceId: a.funding_source_id, amount: String(a.amount) }));
}

/**
 * §9 `Nguồn vốn`: one row per source (select + amount), each source selectable once, the live
 * "Đã phân bổ A / số tiền bố trí B" line (prototype `BudgetItemForm.tsx:375-521`).
 *
 * EVERY SOURCE FIGURE IS THE SERVER'S (`granted_amount`, `unallocated_amount`); the only sum here is
 * the rows the clerk is typing, against the plan typed above — a live check of input, never a report
 * figure. When editing, a source's "còn … chưa phân bổ" already counts THIS project's saved line.
 *
 * NO CATALOGUE, NO ROWS: an empty catalogue for the year leaves nothing to choose, so the block says
 * where sources are declared; a catalogue that failed to load leaves the saved allocations untouched
 * (no row can change, so a PATCH omits the field).
 */
export function FundingAllocationList({
  year,
  catalogue,
  rows,
  summary,
  currentAllocations,
  onChange,
}: {
  year: number;
  catalogue: FundingCatalogue;
  rows: readonly AllocationRow[];
  summary: AllocationSummary;
  currentAllocations?: readonly finance_projectAllocationOut[];
  onChange: (rows: AllocationRow[]) => void;
}) {
  const legend = <legend className="mb-1.5 p-0 text-xs leading-tight font-medium text-ink-700">Nguồn vốn</legend>;
  const fieldset = (body: ReactNode) => (
    <fieldset className="m-0 flex min-w-0 flex-col gap-2 border-0 p-0" data-funding-allocations="">
      {legend}
      {body}
    </fieldset>
  );

  if (catalogue.phase === "loading") {
    return fieldset(
      <p role="status" className="m-0 text-[13px] text-ink-500">
        Đang tải danh mục nguồn vốn…
      </p>,
    );
  }
  const savedNames = (currentAllocations ?? []).map((a) => a.name).join(", ");
  if (catalogue.phase === "error") {
    return fieldset(
      <Notice tone="neutral">
        Chưa tải được danh mục nguồn vốn: {catalogue.message} Phần nguồn vốn của dự án được giữ nguyên
        {savedNames !== "" ? ` (${savedNames})` : ""}; các mục khác vẫn lưu được.
      </Notice>,
    );
  }

  const sources = catalogue.items;
  if (sources.length === 0 && rows.length === 0) {
    return fieldset(
      <p className="m-0 text-[13px] text-ink-500">
        Xã chưa khai báo nguồn vốn nào nên chưa gắn được nguồn cho dự án. Khai nguồn vốn ở khối “Tiến độ
        theo nguồn vốn” của trang Theo dõi giải ngân (nút Quản lý nguồn vốn), rồi sửa dự án để gắn nguồn.
      </p>,
    );
  }

  const byId = new Map(sources.map((s) => [s.id, s]));
  const used = new Set(rows.map((r) => r.sourceId).filter((id) => id !== ""));
  // A saved line whose source is not in this year's list keeps its name instead of vanishing from the
  // select (the select would otherwise show "— Chọn nguồn vốn —" over a line that exists).
  const nameOf = (id: string): string =>
    byId.get(id)?.name ??
    currentAllocations?.find((a) => a.funding_source_id === id)?.name ??
    "Nguồn vốn không có trong danh mục năm này";
  const nextKey = rows.reduce((max, r) => Math.max(max, r.key), -1) + 1;
  const replace = (key: number, change: Partial<AllocationRow>) =>
    onChange(rows.map((r) => (r.key === key ? { ...r, ...change } : r)));

  return fieldset(
    <>
      {rows.length === 0 ? (
        <p className="m-0 text-[13px] text-ink-500">
          Chưa gắn nguồn nào. Xã theo dõi kế hoạch vốn theo hạng mục thì để trống cũng được.
        </p>
      ) : (
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {rows.map((row, index) => {
            const source = byId.get(row.sourceId);
            const outside = row.sourceId !== "" && source === undefined;
            const label = row.sourceId === "" ? `dòng ${index + 1}` : nameOf(row.sourceId);
            return (
              <li
                key={row.key}
                className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-start gap-2 rounded-[10px] border border-line p-2 sm:grid-cols-[minmax(0,1fr)_11rem_auto]"
              >
                <Field
                  label={`Nguồn vốn ${label}`}
                  hideLabel
                  htmlFor={`nguon-von-du-an-${row.key}`}
                  kind="select"
                  grow="auto"
                  className="min-w-0"
                  hint={source !== undefined ? <SourceHint source={source} year={year} /> : undefined}
                >
                  <select
                    id={`nguon-von-du-an-${row.key}`}
                    value={row.sourceId}
                    onChange={(e) => replace(row.key, { sourceId: e.target.value })}
                  >
                    <option value="">— Chọn nguồn vốn —</option>
                    {outside && <option value={row.sourceId}>{nameOf(row.sourceId)}</option>}
                    {sources
                      .filter((s) => s.id === row.sourceId || !used.has(s.id))
                      .map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name}
                        </option>
                      ))}
                  </select>
                </Field>
                <Button
                  type="button"
                  variant="icon"
                  size="md"
                  aria-label={`Bỏ nguồn vốn ${label}`}
                  className="sm:col-start-3 sm:row-start-1"
                  onClick={() => onChange(rows.filter((r) => r.key !== row.key))}
                >
                  <Glyph icon={Trash2} className="size-4" />
                </Button>
                <Field
                  label={`Số tiền từ nguồn vốn ${label} (đồng)`}
                  hideLabel
                  htmlFor={`so-tien-nguon-von-${row.key}`}
                  grow="auto"
                  className="col-span-2 min-w-0 sm:col-span-1 sm:col-start-2 sm:row-start-1"
                  hint={amountHint(row.amount)}
                >
                  <input
                    id={`so-tien-nguon-von-${row.key}`}
                    value={row.amount}
                    inputMode="numeric"
                    autoComplete="off"
                    placeholder="Số tiền"
                    className="tabular-nums"
                    onChange={(e) => replace(row.key, { amount: e.target.value })}
                  />
                </Field>
              </li>
            );
          })}
        </ul>
      )}

      <Button
        type="button"
        variant="secondary"
        size="sm"
        className="self-start"
        icon={<Glyph icon={Plus} />}
        // Each row needs a source of its own: with as many rows as sources there is nothing left.
        disabled={rows.length >= sources.length}
        onClick={() => onChange([...rows, { key: nextKey, sourceId: "", amount: "" }])}
      >
        Thêm nguồn vốn
      </Button>

      <AllocationSummaryLine summary={summary} />
    </>,
  );
}

/** "Nguồn này còn X chưa phân bổ trên tổng Y được giao năm N." — the server's figures for the year. */
function SourceHint({ source, year }: { source: finance_fundingSourceOut; year: number }) {
  if (source.granted_amount <= 0) return <>Nguồn này chưa nhập số vốn được giao năm {year}.</>;
  if (source.overallocated_amount > 0) {
    return (
      <>
        Nguồn này đã phân bổ vượt số được giao{" "}
        <b className="font-semibold text-danger-600" title={nhanTien(source.overallocated_amount)}>
          {compactDong(source.overallocated_amount)}
        </b>{" "}
        (tổng được giao <span title={nhanTien(source.granted_amount)}>{compactDong(source.granted_amount)}</span>).
      </>
    );
  }
  return (
    <>
      Nguồn này còn{" "}
      <b className="font-semibold text-warning-600" title={nhanTien(source.unallocated_amount)}>
        {compactDong(source.unallocated_amount)}
      </b>{" "}
      chưa phân bổ trên tổng <span title={nhanTien(source.granted_amount)}>{compactDong(source.granted_amount)}</span>{" "}
      được giao năm {year}.
    </>
  );
}

/**
 * The live line under the list (prototype `:499-518`): matched, short, or over — over in the danger
 * colour and saying it cannot be saved. Exact đồng, because this is the figure the clerk reconciles.
 */
export function AllocationSummaryLine({ summary }: { summary: AllocationSummary }) {
  if (summary.state === "none") return null;
  let text: string;
  let tone: string;
  switch (summary.state) {
    case "noPlan":
      text = `Đã phân bổ ${nhanTien(summary.allocated)} · nhập số tiền bố trí ở trên để đối chiếu`;
      tone = "bg-surface-subtle text-ink-700";
      break;
    case "unsafe":
      text = "Tổng phân bổ quá lớn để trình duyệt giữ chính xác từng đồng, không lưu được";
      tone = "bg-danger-50 font-semibold text-danger-600";
      break;
    case "short":
      text =
        `Đã phân bổ ${nhanTien(summary.allocated)} / số tiền bố trí ${nhanTien(summary.planned)}` +
        ` · còn thiếu ${nhanTien(summary.gap)} chưa gắn nguồn`;
      tone = "bg-warning-50 text-warning-600";
      break;
    case "over":
      text =
        `Đã phân bổ ${nhanTien(summary.allocated)} / số tiền bố trí ${nhanTien(summary.planned)}` +
        ` · vượt ${nhanTien(summary.gap)}, không lưu được`;
      tone = "bg-danger-50 font-semibold text-danger-600";
      break;
    case "match":
      text = `Đã phân bổ ${nhanTien(summary.allocated)} / số tiền bố trí ${nhanTien(summary.planned)} · khớp`;
      tone = "bg-success-50 text-success-600";
      break;
  }
  return (
    <p
      className={cn("m-0 rounded-lg px-2.5 py-1.5 text-[13px] tabular-nums", tone)}
      aria-live="polite"
      data-allocation-state={summary.state}
    >
      {text}
    </p>
  );
}

const ADD_TITLE_ID = "tieu-de-them-du-an";

/**
 * `+ Thêm dự án` — the page header's solid button and the dialog it opens.
 *
 * CỔNG `budget.update`. Thiếu khoá thì KHÔNG có nút — the prototype draws nothing either
 * (`canRecord`); the empty list's sentence names the permission. The server checks the key on the
 * POST regardless (rule 5).
 */
export function KhoiThemDuAn({
  nam,
  danhMuc,
  coGhi,
  daGhiXong,
}: {
  nam: number;
  danhMuc: readonly finance_hangMucRa[];
  coGhi: boolean;
  daGhiXong: () => void;
}) {
  const [dangMo, datDangMo] = useState(false);
  if (!coGhi) return null;
  return (
    <>
      <Button
        type="button"
        variant="primary"
        icon={<Glyph icon={Plus} />}
        aria-haspopup="dialog"
        onClick={() => datDangMo(true)}
      >
        Thêm dự án
      </Button>
      {dangMo && (
        <AddProjectDialog nam={nam} danhMuc={danhMuc} onClose={() => datDangMo(false)} onSaved={daGhiXong} />
      )}
    </>
  );
}

/** The prototype's 64rem centred dialog (`BudgetItemForm.tsx:714-727`). Mounted = open. */
function AddProjectDialog({
  nam,
  danhMuc,
  onClose,
  onSaved,
}: {
  nam: number;
  danhMuc: readonly finance_hangMucRa[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [dangGui, datDangGui] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const fundingCatalogue = useFundingCatalogue(nam);

  function them(gt: GiaTriFormDuAn, khoa: string): void {
    const than = thanThemDuAn(nam, gt);
    if (!than.ok) {
      datLoi(than.cau);
      return;
    }
    datDangGui(true);
    themDuAn(than.than, khoa).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ. 409 của tuyến này nói rõ vì sao một mã không thấy trên màn hình
        // vẫn bị coi là đã dùng, và 404 của nó gọi đúng tên trường sai. A failed POST keeps the
        // dialog, the typed values AND the idempotency key (`khoaChongTrung`) for the retry.
        datLoi(kq.thongBao);
        return;
      }
      // ĐỌC LẠI CẢ DANH SÁCH, KHÔNG VÁ HÀNG MỚI VÀO: phản hồi của tuyến thêm cố ý không mang các
      // con số suy ra (`disbursed_amount`, `delay_score`…) — `bang-du-an.tsx`.
      onClose();
      onSaved();
    });
  }

  return (
    <ModalDialog
      titleId={ADD_TITLE_ID}
      size="lg"
      className="max-w-[64rem]"
      // Closing while the POST is in flight would hide whether a project code was issued.
      onDismiss={() => {
        if (!dangGui) onClose();
      }}
    >
      <ModalDialogHeader
        titleId={ADD_TITLE_ID}
        title="Thêm dự án"
        description={`Chọn hạng mục, đặt tên dự án và nhập số tiền bố trí cho năm ${nam}. Những mục còn lại bổ sung sau lúc nào cũng được.`}
      />
      <div className="min-h-0 overflow-y-auto">
        <FormDuAn
          tieuDeForm={`Thêm dự án — năm ngân sách ${nam}`}
          budgetYear={nam}
          giaTriDau={FORM_DU_AN_TRONG}
          danhMuc={danhMuc}
          fundingCatalogue={fundingCatalogue}
          dangGui={dangGui}
          loi={loi}
          huy={onClose}
          luu={them}
        />
      </div>
    </ModalDialog>
  );
}

/** Các ô của biểu mẫu SỬA, đổ từ dự án máy chủ vừa trả về. */
export function giaTriTuDuAn(duAn: finance_duAnRa): GiaTriFormDuAn {
  return {
    ma: duAn.code,
    hangMucID: duAn.category_id,
    ten: duAn.name,
    moTa: duAn.description ?? "",
    // SỐ THÔ, KHÔNG ĐỊNH DẠNG: ô nhập phải chứa đúng con số máy chủ giữ, để một lần Lưu không đổi
    // gì thật sự KHÔNG đổi gì. Đưa `100.000.000` vào ô rồi đọc lại vẫn ra đúng số ấy nhờ
    // `docSoTien` bỏ dấu chấm, nhưng phép so "có đổi không" sẽ thấy khác và gửi đi một `PATCH`
    // thừa — thứ có thể bóc chữ xác nhận của một chứng từ ở tuyến bên cạnh, và ở đây là một lần
    // ghi vào hồ sơ lưu trữ không ai yêu cầu.
    keHoachVon: String(duAn.planned_amount),
    tongMucDuyet: String(duAn.approved_amount),
    ngayKhoiCong: duAn.start_date ?? "",
    ngayHoanThanh: duAn.completion_date ?? "",
    thoiHanGiaiNgan: duAn.disbursement_deadline,
    allocations: allocationRowsOf(duAn.funding_allocations),
  };
}

/**
 * The detail card header's buttons: `Sửa dự án` and `Gỡ dự án`, BOTH under `budget.update`.
 *
 * ONE KEY FOR BOTH, BY USER DECISION 06/10/2026 (follow the prototype, where `canRecord` gates edit
 * and delete alike). This reverses the earlier split that put project deletion under `budget.confirm`;
 * the route follows in a server card, and until it lands a `budget.update`-only account gets the
 * server's 403 sentence in the dialog. Voucher confirm / lock / unlock / remove stay `budget.confirm`.
 * Hiding is UX only — the server checks the key on its route (rule 5).
 *
 * While the edit panel is open the edit button reads `Đang sửa` in the solid look and closes it
 * again (prototype `BudgetItemDetail.tsx:264-273`).
 */
export function ProjectHeaderActions({
  coGhi,
  editing,
  onToggleEdit,
  onRemove,
}: {
  coGhi: boolean;
  editing: boolean;
  onToggleEdit: () => void;
  onRemove: () => void;
}) {
  if (!coGhi) return null;
  return (
    <div className="flex shrink-0 flex-wrap items-center gap-2">
      <Button
        type="button"
        variant={editing ? "primary" : "secondary"}
        size="sm"
        icon={<Glyph icon={Pencil} />}
        aria-expanded={editing}
        aria-controls="sua-du-an"
        onClick={onToggleEdit}
      >
        {editing ? "Đang sửa" : "Sửa dự án"}
      </Button>
      {/* Removal is the danger look, never the solid colour, and keeps its words (spec v2 §7). */}
      <Button
        type="button"
        variant="danger"
        size="sm"
        icon={<Glyph icon={Trash2} />}
        aria-haspopup="dialog"
        onClick={onRemove}
      >
        Gỡ dự án
      </Button>
    </div>
  );
}

/**
 * The inline edit panel above the project card (prototype `BudgetItemForm.tsx:678-692`): white, the
 * accent bar on the left, "✎ Sửa dự án". A dialog would cover the very figures needed to edit right.
 */
export function ProjectEditPanel({
  duAn,
  danhMuc,
  onClose,
  onSaved,
}: {
  duAn: finance_duAnRa;
  danhMuc: readonly finance_hangMucRa[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [dangGui, datDangGui] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const fundingCatalogue = useFundingCatalogue(duAn.year);

  function sua(gt: GiaTriFormDuAn): void {
    const than = thanSuaDuAn(giaTriTuDuAn(duAn), gt);
    if (!than.ok) {
      datLoi(than.cau);
      return;
    }
    datDangGui(true);
    suaDuAn(duAn.id, than.than).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoi(kq.thongBao);
        return;
      }
      // ĐỌC LẠI DỰ ÁN chứ không vá bằng phản hồi: `duAnGhiRa` cố ý không mang `disbursed_amount`,
      // `disbursed_ratio`, `delay_score` hay `is_delayed` — tuyến ghi không đọc chúng. Vá bằng nó
      // sẽ đặt bốn con số 0 lên màn hình, và chúng trông y hệt số thật.
      onClose();
      onSaved();
    });
  }

  return (
    <section
      id="sua-du-an"
      aria-labelledby="tieu-de-sua-du-an"
      className="mb-4 rounded-card border border-l-4 border-brand-100 border-l-brand-500 bg-surface p-5"
    >
      <h3 id="tieu-de-sua-du-an" className="m-0 mb-4 flex items-center gap-2 text-[15px] font-semibold text-ink-900">
        <Glyph icon={Pencil} className="size-4 text-brand-600" />
        Sửa dự án
      </h3>
      <FormDuAn
        tieuDeForm={`Sửa dự án: ${duAn.name}`}
        budgetYear={duAn.year}
        giaTriDau={giaTriTuDuAn(duAn)}
        maChiDoc={duAn.code}
        danhMuc={danhMuc}
        fundingCatalogue={fundingCatalogue}
        currentAllocations={duAn.funding_allocations}
        dangGui={dangGui}
        loi={loi}
        huy={onClose}
        luu={(gt) => sua(gt)}
      />
    </section>
  );
}

const REMOVE_FORM_ID = "form-go-du-an";

/** The consequence sentence of `Gỡ dự án`. Exported for the test that pins "no reason asked". */
export const PROJECT_REMOVE_NOTE =
  "Dự án được xoá mềm: bản ghi vẫn còn trong hệ thống kèm người gỡ, và mã dự án không quay lại dãy — " +
  "nhập lại phải chọn mã khác. Nếu máy chủ từ chối (ví dụ dự án còn chứng từ giải ngân), câu trả lời " +
  "của máy chủ hiện ngay tại đây.";

/**
 * `Gỡ dự án` — a confirmation in a centred dialog (the prototype confirms removals in a dialog).
 *
 * NO REASON FIELD (user decision 06/10/2026, as the prototype): `xoaDuAn` sends none and the server
 * writes its fixed sentence into `delete_reason`, so the soft delete still names a reason (rule 7).
 * The confirmation stays: one click must not remove a project from every total of the year. A refusal
 * (409 vouchers, or 403 until the route takes `budget.update`) is the server's sentence, verbatim.
 */
export function ProjectRemoveDialog({
  duAn,
  onClose,
  onRemoved,
}: {
  duAn: finance_duAnRa;
  onClose: () => void;
  onRemoved: () => void;
}) {
  const [dangGui, datDangGui] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);

  function xoa(e: FormEvent<HTMLFormElement>): void {
    e.preventDefault();
    if (dangGui) return;
    datDangGui(true);
    datLoi(null);
    xoaDuAn(duAn.id).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // 409 "Dự án này còn chứng từ giải ngân nên chưa xoá được…" ra NGUYÊN VĂN: không có câu ấy
        // thì màn hình trông như hỏng — dự án nằm ngay đó và nút Gỡ không làm gì.
        datLoi(kq.thongBao);
        return;
      }
      onClose();
      onRemoved();
    });
  }

  return (
    <ModalDialog
      titleId={REMOVE_FORM_ID}
      onDismiss={() => {
        if (!dangGui) onClose();
      }}
      className="p-0"
    >
      <ConfirmDialog
        as="form"
        id={REMOVE_FORM_ID}
        className="shadow-none"
        tone="danger"
        icon={Trash2}
        title={`Gỡ dự án “${duAn.name}”?`}
        titleAs="h4"
        aria-label={`Gỡ dự án “${duAn.name}”?`}
        onSubmit={xoa}
        actions={
          <>
            <Button type="submit" variant="danger" disabled={dangGui} aria-busy={dangGui || undefined}>
              <BusyLabel busy={dangGui} label="Gỡ dự án" busyText={BUSY_DELETING} />
            </Button>
            <Button type="button" variant="secondary" disabled={dangGui} onClick={onClose}>
              Huỷ
            </Button>
          </>
        }
      >
        <p className="m-0">{PROJECT_REMOVE_NOTE}</p>
        {loi !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {loi}
          </p>
        )}
      </ConfirmDialog>
    </ModalDialog>
  );
}
