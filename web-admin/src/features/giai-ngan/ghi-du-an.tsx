"use client";

import { ChevronDown, ChevronRight, Loader2, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { controlClass, Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import { BUSY_DELETING, BusyLabel } from "@/features/danh-ba/busy-label";
import { listFundingSources } from "@/lib/api/funding-sources";
import { suaDuAn, themDuAn, xoaDuAn } from "@/lib/api/giai-ngan";
import type {
  finance_duAnRa,
  finance_fundingSourceOut,
  finance_hangMucRa,
  finance_projectAllocationOut,
  identity_boPhanRa,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { nhanTien, shortDongLabel } from "./nhan-du-an";
import {
  allocationBlocksSave,
  docSoTien,
  FORM_DU_AN_TRONG,
  IMPLEMENTING_UNIT_MAX,
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
import {
  implementingUnitOptions,
  KNOWN_UNITS_LOADING,
  OFFICER_NOT_LISTED,
  OFFICER_PLACEHOLDER,
  PEOPLE_LOADING,
  UNIT_MANUAL,
  UNIT_NOT_LISTED,
  UNIT_ORG_PREFIX,
  UNIT_PLACEHOLDER,
  UNIT_TEXT_PREFIX,
  useImplementingUnits,
  useProjectPeople,
  type KnownUnits,
  type PeopleCatalogue,
  type ProjectPeople,
} from "./project-people";
import { Glyph } from "./project-ui";
import { CHECKBOX_CLASS, CHECKBOX_LABEL_CLASS } from "./spec-classes";

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
  people = PEOPLE_LOADING,
  knownUnits = KNOWN_UNITS_LOADING,
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
  /**
   * Org units and staff for the two §9 selects, loaded by the dialog / panel around the form. Absent =
   * still loading: both selects stay disabled on their saved value, so nothing about them is sent.
   */
  people?: ProjectPeople;
  /** The year's typed units for the `Đơn vị thực hiện` select. Absent = loading: fewer options, no block. */
  knownUnits?: KnownUnits;
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
      {thieuDanhMuc && (
        <Notice tone="info">
          Danh mục hạng mục kế hoạch vốn của xã đang rỗng, mà mỗi dự án phải thuộc đúng một hạng
          mục. Cần khai hạng mục ở màn Cấu hình (quyền “Quản lý danh mục”) trước khi thêm dự án.
        </Notice>
      )}

      {/* Category, code, name and amount in one two-column grid from md (prototype `:297`). */}
      <div className="grid min-w-0 gap-3.5 md:grid-cols-2">
        <Field
          label="Hạng mục"
          required
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
            {/* Input and `Tự sinh mã` on one row (prototype `:320-338`). Checked (the default): the box
                is disabled and empty, and no `code` leaves the browser — the server issues the next
                DA-number of this commune (9f0a0187). What was typed before ticking is kept, not sent. */}
            <div className="flex min-w-0 items-end gap-3">
              <Field
                label="Mã dự án"
                htmlFor="ma-du-an"
                grow="auto"
                className="min-w-0 flex-1"
              >
                <input
                  id="ma-du-an"
                  name="ma-du-an"
                  value={gt.autoCode ? "" : gt.ma}
                  disabled={gt.autoCode}
                  maxLength={MA_DU_AN_TOI_DA}
                  autoComplete="off"
                  placeholder={gt.autoCode ? "Hệ thống sẽ tự sinh" : "DA01"}
                  onChange={(e) => datGT({ ...gt, ma: e.target.value })}
                />
              </Field>
              <label htmlFor="tu-sinh-ma-du-an" className={cn(CHECKBOX_LABEL_CLASS, "h-8 shrink-0")}>
                <input
                  id="tu-sinh-ma-du-an"
                  type="checkbox"
                  className={CHECKBOX_CLASS}
                  checked={gt.autoCode}
                  onChange={(e) => datGT({ ...gt, autoCode: e.target.checked })}
                />
                Tự sinh mã
              </label>
            </div>
            {/* MÃ ĐÃ CẤP THÌ KHÔNG CẤP LẠI, KỂ CẢ KHI DỰ ÁN MANG MÃ ẤY ĐÃ RÚT KHỎI DANH SÁCH — nói
                trước, vì nếu không thì câu 409 của máy chủ đọc như một lỗi trước mặt người vừa xem
                hết danh sách và không thấy mã ấy ở đâu. Spec §9 wording. */}
            <p className="text-ink-muted m-0 text-[12px]">
              Tự sinh sẽ cấp số tiếp theo trong dãy DA01, DA02… Mã tự nhập phải chưa từng được dùng, kể cả
              bởi dự án đã rút khỏi danh sách.
            </p>
          </div>
        )}

        <Field label="Tên dự án" required htmlFor="ten-du-an" grow="auto" className="min-w-0">
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
          label={`Số tiền bố trí năm ${budgetYear} (đồng)`}
          required
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

      {/* ADR 0081 #2: the prototype stores `at_risk` but has nowhere to set it; the owner put the tick
          here, in the EDIT form only (a project is judged at risk once it exists). Gated by the form's
          own key, `budget.update`; the server checks it again on the PATCH and audits before/after. */}
      {isEdit && (
        <label htmlFor="nguy-co-du-an" className={cn(CHECKBOX_LABEL_CLASS, "w-fit")}>
          <input
            id="nguy-co-du-an"
            type="checkbox"
            className={CHECKBOX_CLASS}
            checked={gt.atRisk}
            onChange={(e) => datGT({ ...gt, atRisk: e.target.checked })}
          />
          Nguy cơ không giải ngân hết
        </label>
      )}

      <button
        type="button"
        aria-expanded={showMore}
        aria-controls="thong-tin-them-du-an"
        onClick={() => setShowMore((v) => !v)}
        className="text-ink-muted hover:text-navy flex w-fit cursor-pointer items-center gap-1 border-0 bg-transparent p-0 [font-family:inherit] text-[12.5px]"
      >
        <Glyph icon={showMore ? ChevronDown : ChevronRight} className="size-4" />
        Thông tin thêm (không bắt buộc)
      </button>

      <div
        id="thong-tin-them-du-an"
        hidden={!showMore}
        className="border-line flex min-w-0 flex-col gap-3 rounded-[10px] border border-solid p-3"
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
          <ImplementingUnitSelect
            catalogue={people.units}
            known={knownUnits}
            orgUnitId={gt.orgUnitId}
            implementingUnit={gt.implementingUnit}
            onChange={(unit) => datGT({ ...gt, ...unit })}
          />
          <ReferenceSelect
            id="can-bo-du-an"
            label="Cán bộ phụ trách"
            what="danh bạ cán bộ"
            placeholder={OFFICER_PLACEHOLDER}
            notListed={OFFICER_NOT_LISTED}
            catalogue={people.staff}
            optionsOf={(staff) =>
              // Spec 04 / prototype: the officer's name only (row F14).
              staff.map((c) => ({ value: c.code, label: c.full_name }))
            }
            value={gt.assigneeId}
            onChange={(assigneeId) => datGT({ ...gt, assigneeId })}
          />
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
            "xong tháng 3 vẫn có thể phải giải ngân trước 31/12."
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
        <Button type="button" variant="outline" disabled={dangGui} onClick={huy}>
          Huỷ
        </Button>
        <Button
          type="submit"
          variant="primary"
          icon={dangGui ? <Loader2 aria-hidden="true" className="size-4 animate-spin" /> : undefined}
          disabled={dangGui || thieuDanhMuc || allocationBlocked}
          aria-busy={dangGui || undefined}
        >
          {isEdit ? "Lưu dự án" : "Thêm dự án"}
        </Button>
      </div>
    </form>
  );
}

/**
 * One §9 reference select (`Đơn vị thực hiện` / `Cán bộ phụ trách`) over an identity catalogue.
 *
 * A SAVED VALUE THE CATALOGUE DOES NOT LIST KEEPS AN OPTION OF ITS OWN, named for what it is — the select
 * would otherwise show the placeholder over a project that IS assigned, and a later save of some other
 * field would look like it cleared it. The raw reference is never printed.
 *
 * Catalogue loading or failed: the select is disabled on its current value (so a PATCH leaves the field
 * out) and a failure says so in the server's words; every other field still saves.
 */
function ReferenceSelect<T>({
  id,
  label,
  what,
  placeholder,
  notListed,
  catalogue,
  optionsOf,
  value,
  onChange,
}: {
  id: string;
  label: string;
  /** "danh mục bộ phận" — named in the loading / failure hint. */
  what: string;
  placeholder: string;
  notListed: string;
  catalogue: PeopleCatalogue<T>;
  optionsOf: (items: readonly T[]) => { value: string; label: string }[];
  value: string;
  onChange: (value: string) => void;
}) {
  const ready = catalogue.phase === "ready";
  const options = ready ? optionsOf(catalogue.items) : [];
  const stray = value !== "" && !options.some((o) => o.value === value);
  return (
    <Field label={label} htmlFor={id} kind="select" grow="auto" className="min-w-0" hint={catalogueHint(catalogue, what)}>
      <select id={id} value={value} disabled={!ready} onChange={(e) => onChange(e.target.value)}>
        <option value="">{placeholder}</option>
        {stray && <option value={value}>{ready ? notListed : "…"}</option>}
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </Field>
  );
}

/** The loading / failure sentence under a select fed by an identity catalogue. */
function catalogueHint<T>(catalogue: PeopleCatalogue<T>, what: string): string | undefined {
  if (catalogue.phase === "loading") return `Đang tải ${what}…`;
  if (catalogue.phase === "error") {
    return `Chưa tải được ${what}: ${catalogue.message} Mục này được giữ nguyên; các mục khác vẫn lưu được.`;
  }
  return undefined;
}

/** Words of the manual-entry option (spec 04 / prototype `BudgetItemForm.tsx:588`). */
export const UNIT_MANUAL_LABEL = "— Đơn vị khác, nhập tay —";

/**
 * §9 `Đơn vị thực hiện` (prototype `BudgetItemForm.tsx:566-599`): one select listing the commune's org
 * units, then the typed units already used this year, then "— Đơn vị khác, nhập tay —", which opens a
 * text box under the select.
 *
 * WHAT IS SENT, EXACTLY ONE OF TWO FIELDS:
 *   an org unit             → `org_unit_id` = its id, `implementing_unit` cleared
 *   a known or typed unit   → `implementing_unit` = the text (trimmed), `org_unit_id` cleared
 *   `— Chưa xác định —`     → both cleared
 * "Cleared" is `""` in a PATCH (the server stores NULL) and ABSENT on create. A field that did not
 * change is not sent at all (`thanSuaDuAn`).
 *
 * A SAVED TYPED UNIT NOT IN THIS YEAR'S LIST opens with the text box filled (prototype `:155-160`), so
 * it never vanishes behind the placeholder. `manual` is only the user's last choice in this select;
 * before any choice it is derived from the saved value.
 *
 * Disabled while the org-unit catalogue is loading or failed, as before: the field is then left out
 * of the PATCH. A failed typed-unit read only means fewer suggestions.
 */
function ImplementingUnitSelect({
  catalogue,
  known,
  orgUnitId,
  implementingUnit,
  onChange,
}: {
  catalogue: PeopleCatalogue<identity_boPhanRa>;
  known: KnownUnits;
  orgUnitId: string;
  implementingUnit: string;
  onChange: (unit: { orgUnitId: string; implementingUnit: string }) => void;
}) {
  const [manualChoice, setManualChoice] = useState<boolean | null>(null);
  const ready = catalogue.phase === "ready";
  const options = ready ? implementingUnitOptions(catalogue.items, known) : [];
  const typed = implementingUnit.trim();
  const typedListed = options.some((o) => o.value === UNIT_TEXT_PREFIX + typed);
  const manual = manualChoice ?? (typed !== "" && !typedListed);
  const orgValue = UNIT_ORG_PREFIX + orgUnitId;
  const strayOrg = orgUnitId !== "" && typed === "" && !options.some((o) => o.value === orgValue);

  const value = manual
    ? UNIT_MANUAL
    : typed !== ""
      ? UNIT_TEXT_PREFIX + typed
      : orgUnitId !== ""
        ? orgValue
        : "";

  function pick(picked: string): void {
    if (picked === UNIT_MANUAL) {
      setManualChoice(true);
      // Keep a text already typed (switching back and forth loses nothing); an org unit is replaced.
      onChange({ orgUnitId: "", implementingUnit: typed !== "" ? implementingUnit : "" });
      return;
    }
    setManualChoice(false);
    if (picked.startsWith(UNIT_ORG_PREFIX)) {
      onChange({ orgUnitId: picked.slice(UNIT_ORG_PREFIX.length), implementingUnit: "" });
    } else if (picked.startsWith(UNIT_TEXT_PREFIX)) {
      onChange({ orgUnitId: "", implementingUnit: picked.slice(UNIT_TEXT_PREFIX.length) });
    } else {
      onChange({ orgUnitId: "", implementingUnit: "" });
    }
  }

  return (
    // The text box sits OUTSIDE the Field: Field centres its chevron on everything it wraps.
    <div className="flex min-w-0 flex-col gap-2">
      <Field
        label="Đơn vị thực hiện"
        htmlFor="don-vi-du-an"
        kind="select"
        grow="auto"
        className="min-w-0"
        hint={catalogueHint(catalogue, "danh mục bộ phận")}
      >
        <select id="don-vi-du-an" value={value} disabled={!ready} onChange={(e) => pick(e.target.value)}>
          <option value="">{UNIT_PLACEHOLDER}</option>
          {strayOrg && <option value={orgValue}>{ready ? UNIT_NOT_LISTED : "…"}</option>}
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
          <option value={UNIT_MANUAL}>{UNIT_MANUAL_LABEL}</option>
        </select>
      </Field>
      {manual && ready && (
        <input
          id="don-vi-du-an-nhap-tay"
          aria-label="Tên đơn vị thực hiện (nhập tay)"
          className={controlClass}
          value={implementingUnit}
          maxLength={IMPLEMENTING_UNIT_MAX}
          autoComplete="off"
          placeholder="Công ty TNHH Xây dựng Trường Giang"
          onChange={(e) => onChange({ orgUnitId: "", implementingUnit: e.target.value })}
        />
      )}
    </div>
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
  // The Field label look (spec 00 §5): navy 13px semibold.
  const legend = <legend className="text-navy mb-1.5 p-0 text-[13px] leading-tight font-semibold">Nguồn vốn</legend>;
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
  // Spec 04: the block shows ONLY when the commune has sources (row F7). A saved line on a year with no
  // catalogue still shows, so it can be seen and removed.
  if (sources.length === 0 && rows.length === 0) return null;

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
        <p className="text-ink-muted m-0 text-[11.5px]">
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
                className="border-line grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-start gap-2 rounded-[10px] border border-solid bg-white p-2 sm:grid-cols-[minmax(0,1fr)_11rem_auto]"
              >
                <Field
                  label={`Nguồn vốn ${label}`}
                  hideLabel
                  htmlFor={`nguon-von-du-an-${row.key}`}
                  kind="select"
                  grow="auto"
                  className="min-w-0"
                  hint={source !== undefined ? <SourceHint source={source} /> : undefined}
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
                  variant="outline"
                  size="md"
                  aria-label={`Bỏ nguồn vốn ${label}`}
                  title="Bỏ nguồn vốn này"
                  className="size-8 px-0 sm:col-start-3 sm:row-start-1"
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
        variant="outline"
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

/**
 * Spec 04: "Nguồn này còn **{short}** chưa phân bổ trên tổng {short} được giao." — the server's figures
 * for the form's year. Two states the spec does not draw keep their own words: no figure entered for the
 * year, and already over-allocated (a "còn" there would be a negative amount).
 */
function SourceHint({ source }: { source: finance_fundingSourceOut }) {
  if (source.granted_amount <= 0) return <span className="text-[11px]">Nguồn này chưa nhập số vốn được giao.</span>;
  if (source.overallocated_amount > 0) {
    return (
      <span className="text-[11px]">
        Nguồn này đã phân bổ vượt số được giao{" "}
        <b className="text-danger font-semibold" title={nhanTien(source.overallocated_amount)}>
          {shortDongLabel(source.overallocated_amount)}
        </b>{" "}
        (tổng {shortDongLabel(source.granted_amount)} được giao).
      </span>
    );
  }
  return (
    <span className="text-[11px]">
      Nguồn này còn{" "}
      <b className="text-tangerine font-semibold" title={nhanTien(source.unallocated_amount)}>
        {shortDongLabel(source.unallocated_amount)}
      </b>{" "}
      chưa phân bổ trên tổng <span title={nhanTien(source.granted_amount)}>{shortDongLabel(source.granted_amount)}</span>{" "}
      được giao.
    </span>
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
      tone = "bg-canvas text-ink";
      break;
    case "unsafe":
      text = "Tổng phân bổ quá lớn để trình duyệt giữ chính xác từng đồng, không lưu được";
      tone = "bg-danger/10 font-semibold text-danger";
      break;
    case "short":
      text =
        `Đã phân bổ ${nhanTien(summary.allocated)} / số tiền bố trí ${nhanTien(summary.planned)}` +
        ` · còn thiếu ${nhanTien(summary.gap)} chưa gắn nguồn`;
      tone = "bg-tangerine/10 text-tangerine";
      break;
    case "over":
      text =
        `Đã phân bổ ${nhanTien(summary.allocated)} / số tiền bố trí ${nhanTien(summary.planned)}` +
        ` · vượt ${nhanTien(summary.gap)}, không lưu được`;
      tone = "bg-danger/10 font-semibold text-danger";
      break;
    case "match":
      text = `Đã phân bổ ${nhanTien(summary.allocated)} / số tiền bố trí ${nhanTien(summary.planned)} · khớp`;
      tone = "bg-leaf/10 text-leaf";
      break;
  }
  return (
    <p
      className={cn("m-0 rounded-[8px] px-2.5 py-1.5 text-[11.5px] tabular-nums", tone)}
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
        icon={<Glyph icon={Plus} className="size-4" />}
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
  const people = useProjectPeople();
  const knownUnits = useImplementingUnits(nam);

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
        // NGUYÊN VĂN câu máy chủ, as a toast (ADR 0068 lần 6 #4: outcomes are toasts; a form's own
        // check stays inline). 409 của tuyến này nói rõ vì sao một mã không thấy trên màn hình vẫn bị
        // coi là đã dùng. A failed POST keeps the dialog, the typed values AND the idempotency key.
        datLoi(null);
        toast.error(kq.thongBao);
        return;
      }
      toast.success("Đã thêm dự án.");
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
      className="max-h-[88vh] max-w-[64rem]"
      // The prototype opens with the focus on `Hạng mục` (brief §3.4). Disabled (empty catalogue) →
      // the title keeps it, and the notice above the form is what is read first.
      initialFocusId="hang-muc-du-an"
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
          people={people}
          knownUnits={knownUnits}
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
    // The code is already issued; the edit form shows it read-only and never sends it.
    autoCode: false,
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
    orgUnitId: duAn.org_unit_id ?? "",
    implementingUnit: duAn.implementing_unit ?? "",
    assigneeId: duAn.assignee_id ?? "",
    // Absent on an older reply reads as not flagged; the PATCH carries `at_risk` only if the box changes.
    atRisk: duAn.at_risk === true,
  };
}

/**
 * The detail card header's buttons: `Sửa dự án` and `Gỡ dự án`, BOTH under `budget.update`.
 *
 * ONE KEY FOR BOTH, BY USER DECISION 06/10/2026 (follow the prototype, where `canRecord` gates edit
 * and delete alike). This reverses the earlier split that put project deletion under `budget.confirm`;
 * the route follows in a server card, and until it lands a `budget.update`-only account gets the
 * server's 403 sentence in the dialog. Voucher confirm / lock / unlock stay `budget.confirm`; voucher
 * removal moved to `budget.update` too (356a5a9f).
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
        variant={editing ? "primary" : "outline"}
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
  people,
  onClose,
  onSaved,
}: {
  duAn: finance_duAnRa;
  danhMuc: readonly finance_hangMucRa[];
  /** The detail page's catalogues (already read for its figures) — not read a second time here. */
  people: ProjectPeople;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [dangGui, datDangGui] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const fundingCatalogue = useFundingCatalogue(duAn.year);
  const knownUnits = useImplementingUnits(duAn.year);

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
        datLoi(null);
        toast.error(kq.thongBao);
        return;
      }
      toast.success("Đã lưu dự án.");
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
      className="border-brand/35 shadow-card rounded-card border-l-brand mb-4 border border-l-4 border-solid bg-white p-5"
    >
      <h3 id="tieu-de-sua-du-an" className="text-navy m-0 mb-4 flex items-center gap-2 text-[13.5px] font-bold">
        <Glyph icon={Pencil} className="text-brand size-4" />
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
        people={people}
        knownUnits={knownUnits}
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
