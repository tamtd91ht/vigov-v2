"use client";

import { LockKeyholeOpen, Pencil, Plus, Trash2, TriangleAlert } from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import { ModalDialog } from "@/components/ui/modal-dialog";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { PendingMarker } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import type { KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";
import {
  goChungTu,
  khoaChungTu,
  moKhoaChungTu,
  suaChungTu,
  themChungTu,
  xacNhanChungTu,
} from "@/lib/api/giai-ngan";
import { getProjectVouchers } from "@/lib/api/du-an";
import type {
  finance_chungTuRa,
  finance_projectAllocationOut,
  finance_projectVouchersOut,
} from "@/lib/api/schema.gen";

import { FormLyDo } from "./form-ly-do";
import { nhanNgay, nhanTien } from "./nhan-du-an";
import {
  allocationOptionLabel,
  CANH_BAO_KHOA,
  CANH_BAO_SUA_VE_NHAP,
  CHUNG_TU_DA_XAC_NHAN,
  CHUNG_TU_KE_TOAN_NHAP,
  DAU_GACH,
  docSoTien,
  DOI_TAC_TOI_DA,
  FORM_CHUNG_TU_TRONG,
  initialFundingSource,
  NOI_DUNG_CHUNG_TU_TOI_DA,
  nhanTrangThaiChungTu,
  pendingPart,
  SO_CHUNG_TU_TOI_DA,
  thaoTacChungTu,
  thanSuaChungTu,
  thanThemChungTu,
  type GiaTriFormChungTu,
} from "./nhan-ghi-giai-ngan";
import { Glyph, VoucherStatusBadge } from "./project-ui";
import { SUB_TABLE_HEAD_ROW_CLASS } from "./spec-classes";

/** The spec 07 behaviour the server lacks (ADR 0068 lần 6 #9): `Khoá` on a draft = confirm + lock at once. */
const LOCK_DRAFT = "Khoá khoản chi chưa xác nhận";

/** Spec 07 entry-box field: 11.5px label, `mt-1 h-9 bg-white text-[12.5px]` control, no `*`. */
const VOUCHER_CONTROL = cn(controlClass, "mt-1 h-9 bg-white text-[12.5px] md:text-[12.5px]");

function VoucherField({
  id,
  label,
  hint,
  className,
  children,
}: {
  id: string;
  label: string;
  hint?: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("min-w-0", className)}>
      <label htmlFor={id} className="text-navy block text-[11.5px] font-semibold">
        {label}
      </label>
      {children}
      {hint !== undefined && <p className="text-ink-muted m-0 mt-1 text-[11px] tabular-nums">{hint}</p>}
    </div>
  );
}

/**
 * Tab "Chứng từ" của §8.2 — sáu tuyến ghi của vòng đời chứng từ giải ngân, in the prototype's
 * composition (`BudgetItemDetail.tsx:391-519`, `DisbursementForm.tsx`, ADR 0068 lần 5): the
 * `+ Ghi nhận khoản chi` button that unfolds the entry box in place, the edit box with the accent
 * bar, then the table whose status cell carries the row's actions.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * THE TABLE IS THE SERVER'S LIST, RE-READ AFTER EVERY WRITE (db94b35c,
 * `GET /api/v1/investment-projects/{id}/disbursements`): every live voucher of the project, newest
 * payment date first, with its source's name. The rows the six write routes return are NOT patched
 * into the table — a second copy kept in the browser is what drifts from the server's order, status
 * and source name, and it is what a reload used to empty.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * HAI KHOÁ, CHIA THEO TRỤC NGUY HIỂM (§8.2): `budget.update` NHẬP và SỬA; `budget.confirm` XÁC
 * NHẬN, KHOÁ, MỞ KHOÁ và GỠ. Đây là cách xã tách người nhập liệu khỏi người chịu trách nhiệm, nên
 * bốn nút dưới không bao giờ được đi chung một cổng với hai nút trên.
 *
 * KHÔNG GHI LOG GÌ: nội dung chi, đối tác và lý do gỡ là chữ cán bộ vừa gõ về tiền công quỹ.
 */

/** Ô của biểu mẫu, đổ từ một chứng từ máy chủ vừa trả về. */
export function giaTriTuChungTu(ct: finance_chungTuRa): GiaTriFormChungTu {
  return {
    ngayChi: ct.payment_date,
    // SỐ THÔ, KHÔNG ĐỊNH DẠNG — xem lý lẽ ở `giaTriTuDuAn`: ô nhập phải chứa đúng con số máy chủ
    // giữ, để phép so "có đổi không" không thấy khác ở một lần Lưu không sửa gì. Một `PATCH` thừa ở
    // tuyến này bóc mất chữ xác nhận của lãnh đạo (ADR 0036).
    soTien: String(ct.amount),
    noiDung: ct.description,
    doiTac: ct.counterparty ?? "",
    soChungTu: ct.voucher_no ?? "",
    fundingSourceId: ct.funding_source_id ?? "",
  };
}

/**
 * Biểu mẫu `+ Ghi nhận khoản chi` §8.2, dùng cho cả THÊM và SỬA — prototype fields in a two-column
 * grid: `Rút từ nguồn vốn` (full width) · Ngày chi · Số tiền · Nội dung chi (full width) · Đơn vị thụ
 * hưởng · Số chứng từ; the buttons below, left-aligned, save first.
 *
 * `Rút từ nguồn vốn` EXISTS ONLY WHEN THE PROJECT HAS ALLOCATION LINES (prototype
 * `DisbursementForm.tsx:158`), and its options are THOSE lines — not the commune's catalogue: the
 * server refuses any other source (409 `source_not_allocated`, decision 06/10/2026).
 *
 * ⚠ KHỐI CẢNH BÁO ADR 0036 HIỆN NGAY TRÊN CÁC Ô, KHÔNG PHẢI SAU KHI LƯU. Sửa một chứng từ đang
 * `Đã xác nhận` sẽ kéo nó VỀ `Kế toán nhập` và xoá dấu người xác nhận. Máy chủ làm việc ấy trong im
 * lặng và trả 200; người phát hiện ra sau là người đi tìm chữ ký ấy lúc quyết toán.
 *
 * KHÔNG CÓ Ô `Trạng thái`: vòng đời đi qua các tuyến xác nhận / khoá / mở khoá, mỗi bước một vết
 * kiểm toán. Máy chủ trả 400 cho một thân mang `status`, và kiểu `ThemChungTuVao` đã loại nó ra ở
 * tầng biên dịch.
 */
export function FormChungTu({
  tieuDeForm,
  giaTriDau,
  allocations,
  currentSourceName,
  trangThaiHienTai,
  dangGui,
  loi,
  huy,
  luu,
}: {
  tieuDeForm: string;
  giaTriDau: GiaTriFormChungTu;
  /** The project's allocation lines (`funding_allocations`). Empty → no source select at all. */
  allocations: readonly finance_projectAllocationOut[];
  /** EDIT only: the voucher's current source name, for a source the project no longer allocates. */
  currentSourceName?: string;
  /** Trạng thái của chứng từ đang SỬA. `undefined` khi đang THÊM. */
  trangThaiHienTai?: string;
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (gt: GiaTriFormChungTu, khoaChongTrung: string) => void;
}) {
  const [gt, datGT] = useState<GiaTriFormChungTu>(giaTriDau);
  const [khoaChongTrung] = useState(khoaChongTrungMoi);
  const isEdit = trangThaiHienTai !== undefined;
  const amount = docSoTien(gt.soTien);
  // A voucher filed on a source the project has since stopped allocating still shows THAT source
  // pre-filled — a select silently showing another line would read as the voucher's real source.
  const sourceOutsideLines =
    gt.fundingSourceId !== "" && !allocations.some((a) => a.funding_source_id === gt.fundingSourceId);

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    luu(gt, khoaChongTrung);
  }

  return (
    <form
      onSubmit={guiNgay}
      aria-label={tieuDeForm}
      className={cn(
        "flex min-w-0 flex-col gap-3 rounded-[10px] border border-solid p-3",
        // Spec 07: the entry box sits on the page grey; the edit box is white with the accent bar.
        isEdit ? "border-brand/35 border-l-brand border-l-4 bg-white" : "border-line bg-canvas",
      )}
    >
      {isEdit && <p className="text-navy m-0 text-[12.5px] font-semibold">{tieuDeForm}</p>}

      {trangThaiHienTai === CHUNG_TU_DA_XAC_NHAN && (
        <Notice tone="legal" icon={TriangleAlert}>
          {CANH_BAO_SUA_VE_NHAP}
        </Notice>
      )}

      <div className="grid min-w-0 grid-cols-2 gap-3">
        {allocations.length > 0 && (
          <VoucherField id="nguon-von-chung-tu" label="Rút từ nguồn vốn" className="col-span-2">
            <select
              id="nguon-von-chung-tu"
              name="nguon-von-chung-tu"
              className={VOUCHER_CONTROL}
              value={gt.fundingSourceId}
              onChange={(e) => datGT({ ...gt, fundingSourceId: e.target.value })}
            >
              <option value="">— Chọn nguồn vốn —</option>
              {sourceOutsideLines && (
                <option value={gt.fundingSourceId}>
                  {currentSourceName ?? "Nguồn vốn hiện tại"} — không còn phân bổ cho dự án
                </option>
              )}
              {allocations.map((a) => (
                <option
                  key={a.funding_source_id}
                  value={a.funding_source_id}
                  title={nhanTien(a.amount - a.disbursed_amount)}
                >
                  {allocationOptionLabel(a)}
                </option>
              ))}
            </select>
          </VoucherField>
        )}

        {/* HAI NGÀY KHÁC NHAU, và xã nhập chứng từ tuần trước vào sáng thứ Hai là chuyện thường —
            mọi biểu đồ luỹ kế của §4 xếp theo ngày CHI, không theo ngày gõ. */}
        <VoucherField id="ngay-chi-chung-tu" label="Ngày chi">
          <input
            id="ngay-chi-chung-tu"
            name="ngay-chi-chung-tu"
            type="date"
            className={VOUCHER_CONTROL}
            value={gt.ngayChi}
            onChange={(e) => datGT({ ...gt, ngayChi: e.target.value })}
          />
        </VoucherField>

        <VoucherField
          id="so-tien-chung-tu"
          label="Số tiền (đồng)"
          hint={amount.loai === "so" ? nhanTien(amount.dong) : DAU_GACH}
        >
          <input
            id="so-tien-chung-tu"
            name="so-tien-chung-tu"
            value={gt.soTien}
            inputMode="numeric"
            autoComplete="off"
            placeholder="250.000.000"
            className={cn(VOUCHER_CONTROL, "tabular-nums")}
            onChange={(e) => datGT({ ...gt, soTien: e.target.value })}
          />
        </VoucherField>

        <VoucherField id="noi-dung-chung-tu" label="Nội dung chi" className="col-span-2">
          <input
            id="noi-dung-chung-tu"
            name="noi-dung-chung-tu"
            value={gt.noiDung}
            maxLength={NOI_DUNG_CHUNG_TU_TOI_DA}
            autoComplete="off"
            placeholder="Thanh toán khối lượng đợt 1"
            className={VOUCHER_CONTROL}
            onChange={(e) => datGT({ ...gt, noiDung: e.target.value })}
          />
        </VoucherField>

        <VoucherField id="doi-tac-chung-tu" label="Đơn vị thụ hưởng">
          <input
            id="doi-tac-chung-tu"
            name="doi-tac-chung-tu"
            value={gt.doiTac}
            maxLength={DOI_TAC_TOI_DA}
            autoComplete="off"
            className={VOUCHER_CONTROL}
            onChange={(e) => datGT({ ...gt, doiTac: e.target.value })}
          />
        </VoucherField>

        <VoucherField id="so-chung-tu" label="Số chứng từ">
          <input
            id="so-chung-tu"
            name="so-chung-tu"
            value={gt.soChungTu}
            maxLength={SO_CHUNG_TU_TOI_DA}
            autoComplete="off"
            className={VOUCHER_CONTROL}
            onChange={(e) => datGT({ ...gt, soChungTu: e.target.value })}
          />
        </VoucherField>
      </div>

      {loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}

      <div className="flex gap-2">
        <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui || undefined}>
          {isEdit ? "Lưu thay đổi" : "Lưu khoản chi"}
        </Button>
        <Button type="button" variant="outline" disabled={dangGui} onClick={huy}>
          Huỷ
        </Button>
      </div>
    </form>
  );
}

/**
 * Bảng chứng từ §8.2 — prototype columns (`BudgetItemDetail.tsx:417-424`): Ngày chi · Số tiền · Nguồn
 * vốn · Nội dung · Chứng từ · Trạng thái, the row's actions UNDER the status pill (`:462-510`).
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * NÚT CỦA MỘT HÀNG LÀ GIAO CỦA HAI ĐIỀU, VÀ CẢ HAI ĐỀU CẦN:
 *
 *   1. TRẠNG THÁI cho phép gì (`thaoTacChungTu`) — vòng đời là một dây xích: chưa xác nhận thì
 *      chưa khoá được, đã khoá thì không sửa và không gỡ.
 *   2. TÀI KHOẢN có khoá nào — `budget.update` cho Sửa, `budget.confirm` cho bốn nút còn lại.
 *
 * Thiếu vế 1 thì màn hình vẽ những nút chắc chắn nhận 409. Thiếu vế 2 thì màn hình mở thao tác xác
 * nhận cho người chỉ được nhập liệu. Không vế nào thay được vế nào, và không vế nào là biện pháp:
 * máy chủ kiểm lại cả hai trên TỪNG yêu cầu (luật 5, cấm #1).
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */
export function BangChungTu({
  ds,
  coGhi,
  coXacNhan,
  dangGui,
  moSua,
  moGo,
  moMoKhoa,
  xacNhan,
  khoa,
}: {
  ds: readonly finance_chungTuRa[];
  coGhi: boolean;
  coXacNhan: boolean;
  dangGui: boolean;
  moSua: (id: string) => void;
  moGo: (id: string) => void;
  moMoKhoa: (id: string) => void;
  xacNhan: (id: string) => void;
  khoa: (id: string) => void;
}) {
  if (ds.length === 0) {
    // The server's list, so an empty table is a fact about the project, said as one (spec 07 words).
    return <p className="text-ink-muted m-0 text-[12.5px]">Chưa có khoản chi nào được ghi nhận.</p>;
  }

  const th = "py-2 pr-3 text-left font-semibold";
  const td = "py-2.5 pr-3 align-top";
  // Spec 07 row buttons: `size="sm" h-7 px-2 text-[11px]`.
  const rowButton = "h-7 px-2 text-[11px]";
  return (
    <div role="region" tabIndex={0} aria-label="Chứng từ giải ngân của dự án" className="overflow-x-auto">
      <table className="w-full min-w-[640px] border-collapse text-[12px]">
        <caption className="an-thi-giac">Chứng từ giải ngân của dự án, ngày chi mới nhất trước</caption>
        <thead>
          <tr className={SUB_TABLE_HEAD_ROW_CLASS}>
            <th scope="col" className={th}>
              Ngày chi
            </th>
            <th scope="col" className={cn(th, "text-right")}>
              Số tiền
            </th>
            <th scope="col" className={th}>
              Nguồn vốn
            </th>
            <th scope="col" className={th}>
              Nội dung
            </th>
            <th scope="col" className={th}>
              Chứng từ
            </th>
            <th scope="col" className={th}>
              Trạng thái
            </th>
          </tr>
        </thead>
        <tbody>
          {ds.map((ct) => {
            const cho = thaoTacChungTu(ct.status);
            const ngay = nhanNgay(ct.payment_date);
            const actions = [
              coXacNhan && cho.xacNhan && (
                <Button
                  key="xac-nhan"
                  type="button"
                  variant="outline"
                  size="sm"
                  className={rowButton}
                  disabled={dangGui}
                  onClick={() => xacNhan(ct.id)}
                  aria-label={`Xác nhận chứng từ ngày ${ngay}`}
                >
                  Xác nhận
                </Button>
              ),
              // Spec 07 draws `Khoá` on a draft too (confirm + lock in one call). The server cannot yet:
              // the control stands in the prototype's place, disabled, with its "?" (lần 6 #9).
              coXacNhan && ct.status === CHUNG_TU_KE_TOAN_NHAP && (
                <span key="khoa-nhap" className="relative inline-flex" data-pending="">
                  <Button type="button" variant="primary" size="sm" className={cn(rowButton, "pr-7")} disabled>
                    Khoá
                  </Button>
                  <PendingMarker info={pendingPart(LOCK_DRAFT)} placement="end" className="right-1" />
                </span>
              ),
              coXacNhan && cho.khoa && (
                <Button
                  key="khoa"
                  type="button"
                  variant="primary"
                  size="sm"
                  className={rowButton}
                  disabled={dangGui}
                  onClick={() => khoa(ct.id)}
                  aria-label={`Khoá chứng từ ngày ${ngay}`}
                  title={CANH_BAO_KHOA}
                >
                  Khoá
                </Button>
              ),
              coXacNhan && cho.moKhoa && (
                <Button
                  key="mo-khoa"
                  type="button"
                  variant="outline"
                  size="sm"
                  className={rowButton}
                  icon={<Glyph icon={LockKeyholeOpen} className="size-3" />}
                  disabled={dangGui}
                  onClick={() => moMoKhoa(ct.id)}
                  aria-label={`Mở khoá chứng từ ngày ${ngay}`}
                >
                  Mở khoá
                </Button>
              ),
              coGhi && cho.sua && (
                <Button
                  key="sua"
                  type="button"
                  variant="outline"
                  size="sm"
                  className={rowButton}
                  icon={<Glyph icon={Pencil} className="size-3" />}
                  disabled={dangGui}
                  onClick={() => moSua(ct.id)}
                  aria-label={`Sửa chứng từ ngày ${ngay}`}
                >
                  Sửa
                </Button>
              ),
              // Gỡ sits last: outline with the danger word (spec 07).
              coXacNhan && cho.go && (
                <Button
                  key="go"
                  type="button"
                  variant="outline"
                  size="sm"
                  className={cn(rowButton, "text-danger hover:not-disabled:text-danger")}
                  icon={<Glyph icon={Trash2} className="size-3" />}
                  disabled={dangGui}
                  onClick={() => moGo(ct.id)}
                  aria-label={`Gỡ chứng từ ngày ${ngay}`}
                >
                  Gỡ
                </Button>
              ),
            ].filter(Boolean);
            return (
              <tr key={ct.id} className="border-line border-b last:border-b-0" data-voucher={ct.status}>
                <td className={cn(td, "tabular-nums whitespace-nowrap")}>{ngay}</td>
                {/* IN ĐÚNG CON SỐ MÁY CHỦ TRẢ. Không đổi đơn vị, không làm tròn: một con số sai ở
                    đây đi thẳng vào báo cáo ngân sách. */}
                <td className={cn(td, "text-navy text-right font-semibold whitespace-nowrap tabular-nums")}>
                  {nhanTien(ct.amount)}
                </td>
                {/* The server's name for the source; "—" for a voucher drawn on none (§13 rule 6). */}
                <td className={cn(td, "text-ink-muted whitespace-normal")}>
                  {ct.funding_source_name === undefined || ct.funding_source_name === ""
                    ? DAU_GACH
                    : ct.funding_source_name}
                </td>
                <td className={cn(td, "whitespace-normal")}>
                  {ct.description}
                  {ct.counterparty !== undefined && ct.counterparty !== "" && (
                    <span className="text-ink-muted block text-[11px]">{ct.counterparty}</span>
                  )}
                </td>
                <td className={cn(td, "text-ink-muted whitespace-nowrap")}>
                  {ct.voucher_no === undefined || ct.voucher_no === "" ? DAU_GACH : ct.voucher_no}
                </td>
                <td className={td}>
                  <VoucherStatusBadge status={ct.status}>{nhanTrangThaiChungTu(ct.status)}</VoucherStatusBadge>
                  {actions.length > 0 && <span className="mt-1.5 flex flex-wrap gap-1.5">{actions}</span>}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

type DangMo =
  | { kieu: "them" }
  | { kieu: "sua"; ct: finance_chungTuRa }
  | { kieu: "go"; ct: finance_chungTuRa }
  | { kieu: "moKhoa"; ct: finance_chungTuRa }
  | null;

const REASON_FORM_ID = "form-ly-do-chung-tu";

/** The project's voucher list as the tab sees it. `count` is the server's, not `items.length`. */
export type ProjectVoucherList =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "ready"; items: readonly finance_chungTuRa[]; count: number };

/**
 * Reads `GET /api/v1/investment-projects/{id}/disbursements` once per `reloadKey`.
 *
 * THE CALLER PASSES THE SAME KEY THAT RE-READS THE PROJECT, so one write re-reads both: the voucher
 * list and the four figures derived from it (`disbursed_amount`, `remaining_amount`,
 * `disbursed_ratio`, `delay_score`) can never be from two different moments. "Loading" is DERIVED from
 * the stored key differing from the current one — the `chi-tiet-du-an.tsx` pattern, no `setState` in
 * the effect body.
 */
export function useProjectVouchers(projectId: string, reloadKey: string): ProjectVoucherList {
  const key = `${projectId}|${reloadKey}`;
  const [loaded, setLoaded] = useState<{ key: string; result: KetQua<finance_projectVouchersOut> } | null>(
    null,
  );

  useEffect(() => {
    let cancelled = false;
    getProjectVouchers(projectId).then((result) => {
      if (!cancelled) setLoaded({ key, result });
    });
    return () => {
      cancelled = true;
    };
  }, [projectId, key]);

  if (loaded === null || loaded.key !== key) return { phase: "loading" };
  if (!loaded.result.ok) return { phase: "error", message: loaded.result.thongBao };
  return { phase: "ready", items: loaded.result.duLieu.items, count: loaded.result.duLieu.count };
}

/**
 * Khối "Chứng từ" của trang chi tiết dự án — nối cả sáu tuyến ghi. Rendered as the tab panel's
 * content, not as a card of its own (the prototype's tab content sits inside the project card).
 *
 * GỠ AND MỞ KHOÁ ASK FOR THEIR REASON IN A DIALOG — the prototype confirms a removal in a dialog; the
 * server requires the reason for both (rule 7, open question #29).
 *
 * MỖI LẦN GHI XONG GỌI `daGhiXong`, và trang cha ĐỌC LẠI cả dự án lẫn danh sách chứng từ:
 * `disbursed_amount`, `remaining_amount`, `disbursed_ratio` và `delay_score` đều SUY RA từ chứng từ,
 * và thứ tự, trạng thái, tên nguồn của từng hàng là của máy chủ. Vá tại chỗ ở trình duyệt là dựng câu
 * trả lời thứ hai cho cùng một câu hỏi.
 */
export function KhoiChungTu({
  duAnID,
  allocations,
  vouchers,
  coGhi,
  coXacNhan,
  daGhiXong,
}: {
  duAnID: string;
  /** The project's `funding_allocations`: the source select's options and its "required" rule. */
  allocations: readonly finance_projectAllocationOut[];
  vouchers: ProjectVoucherList;
  coGhi: boolean;
  coXacNhan: boolean;
  /** After a successful write, and as the list's retry: the parent re-reads project AND vouchers. */
  daGhiXong: () => void;
}) {
  const ds: readonly finance_chungTuRa[] = vouchers.phase === "ready" ? vouchers.items : [];
  const allocatedSourceIds = allocations.map((a) => a.funding_source_id);
  const [dangMo, datDangMo] = useState<DangMo>(null);
  const [dangGui, datDangGui] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const [lanGhiXong, datLanGhiXong] = useState(0);

  /**
   * Kết thúc MỘT lần ghi, dùng chung cho cả sáu tuyến.
   *
   * MỘT HÀM CHO SÁU TUYẾN vì sáu bản sao của đoạn này sẽ trôi, và bản trôi là bản quên gọi
   * `daGhiXong` — tức bảng chứng từ và bốn con số suy ra của dự án ở ngay trên đều cũ, mà không có gì
   * nói ra rằng chúng cũ.
   */
  function ketThuc<T>(kq: KetQua<T>, done: string): void {
    datDangGui(false);
    if (!kq.ok) {
      // NGUYÊN VĂN câu máy chủ, as a toast (ADR 0068 lần 6 #4), kể cả 409 của vòng đời, 409 "người vừa
      // khoá không tự mở lại được" và 409 `source_required` / `source_not_allocated`. Những câu ấy mang
      // tên thao tác và mang đường ra — nuốt chúng thành "Có lỗi xảy ra" là lấy mất đúng thứ cán bộ cần.
      // The spec's own sentences for those codes cannot be chosen here: `KetQua` carries no code.
      datLoi(null);
      toast.error(kq.thongBao);
      return;
    }
    toast.success(done);
    datLoi(null);
    datDangMo(null);
    datLanGhiXong((n) => n + 1);
    daGhiXong();
  }

  function them(gt: GiaTriFormChungTu, khoa: string): void {
    const than = thanThemChungTu(duAnID, gt, allocatedSourceIds);
    if (!than.ok) {
      datLoi(than.cau);
      return;
    }
    datDangGui(true);
    themChungTu(than.than, khoa).then((kq) => ketThuc(kq, "Đã ghi nhận khoản chi. Chờ lãnh đạo xác nhận."));
  }

  function sua(ct: finance_chungTuRa, gt: GiaTriFormChungTu): void {
    const than = thanSuaChungTu(giaTriTuChungTu(ct), gt, allocatedSourceIds);
    if (!than.ok) {
      datLoi(than.cau);
      return;
    }
    datDangGui(true);
    suaChungTu(ct.id, than.than).then((kq) => ketThuc(kq, "Đã sửa khoản chi. Cần lãnh đạo xác nhận lại."));
  }

  function xacNhan(id: string): void {
    datDangGui(true);
    xacNhanChungTu(id).then((kq) => ketThuc(kq, "Đã xác nhận."));
  }

  function khoa(id: string): void {
    datDangGui(true);
    // Spec 07's sentence for a lock; true here too — the voucher is now confirmed AND locked.
    khoaChungTu(id).then((kq) => ketThuc(kq, "Đã xác nhận và khoá."));
  }

  function moKhoa(ct: finance_chungTuRa, lyDo: string): void {
    datDangGui(true);
    // No spec sentence (the spec has no unlock); the shortest true one.
    moKhoaChungTu(ct.id, lyDo).then((kq) => ketThuc(kq, "Đã mở khoá."));
  }

  function go(ct: finance_chungTuRa, lyDo: string): void {
    datDangGui(true);
    // 204 KHÔNG THÂN. Hàng vẫn còn ở CSDL kèm người gỡ và lý do (xoá mềm); danh sách đọc lại không
    // còn nó vì tuyến đọc chỉ trả chứng từ còn hiệu lực.
    goChungTu(ct.id, lyDo).then((kq) => ketThuc(kq, "Đã gỡ khoản chi."));
  }

  function timHang(id: string): finance_chungTuRa | undefined {
    return ds.find((c) => c.id === id);
  }

  function dong(): void {
    if (dangGui) return;
    datDangMo(null);
    datLoi(null);
  }

  return (
    <div className="flex min-w-0 flex-col gap-3">
      {/* The entry box unfolds IN PLACE of its button (prototype `DisbursementForm.tsx:134-141`). */}
      {coGhi && dangMo?.kieu !== "them" && (
        <div>
          <Button
            type="button"
            variant="outline"
            icon={<Glyph icon={Plus} />}
            onClick={() => {
              datLoi(null);
              datDangMo({ kieu: "them" });
            }}
          >
            Ghi nhận khoản chi
          </Button>
        </div>
      )}

      {dangMo?.kieu === "them" && coGhi && (
        <FormChungTu
          key={`them-chung-tu|${lanGhiXong}`}
          tieuDeForm="Ghi nhận khoản chi"
          giaTriDau={{ ...FORM_CHUNG_TU_TRONG, fundingSourceId: initialFundingSource(allocations) }}
          allocations={allocations}
          dangGui={dangGui}
          loi={loi}
          huy={dong}
          luu={them}
        />
      )}

      {dangMo?.kieu === "sua" && coGhi && (
        <FormChungTu
          key={`sua-chung-tu|${dangMo.ct.id}|${lanGhiXong}`}
          tieuDeForm={`Sửa khoản chi ngày ${nhanNgay(dangMo.ct.payment_date)}`}
          giaTriDau={giaTriTuChungTu(dangMo.ct)}
          allocations={allocations}
          currentSourceName={dangMo.ct.funding_source_name}
          trangThaiHienTai={dangMo.ct.status}
          dangGui={dangGui}
          loi={loi}
          huy={dong}
          luu={(gt) => {
            sua(dangMo.ct, gt);
          }}
        />
      )}

      {/* Spec 07: no lifecycle sentence, and buttons an account cannot use are simply absent — the
          server checks both keys on every call (rule 5). */}

      {vouchers.phase === "loading" && (
        <div>
          <p role="status" className="an-thi-giac">
            Đang tải chứng từ…
          </p>
          <div aria-hidden="true" className="flex flex-col gap-2">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
          </div>
        </div>
      )}

      {/* A failed read is NOT an empty table: "chưa có chứng từ nào" would be a claim about the
          commune's money that the screen has no ground for. */}
      {vouchers.phase === "error" && (
        <ErrorState
          title="Chưa tải được danh sách chứng từ"
          message={<span role="alert">{vouchers.message}</span>}
          onRetry={daGhiXong}
        />
      )}

      {vouchers.phase === "ready" && (
      <BangChungTu
        ds={ds}
        coGhi={coGhi}
        coXacNhan={coXacNhan}
        dangGui={dangGui}
        moSua={(id) => {
          const ct = timHang(id);
          if (ct === undefined) return;
          datLoi(null);
          datDangMo({ kieu: "sua", ct });
        }}
        moGo={(id) => {
          const ct = timHang(id);
          if (ct === undefined) return;
          datLoi(null);
          datDangMo({ kieu: "go", ct });
        }}
        moMoKhoa={(id) => {
          const ct = timHang(id);
          if (ct === undefined) return;
          datLoi(null);
          datDangMo({ kieu: "moKhoa", ct });
        }}
        xacNhan={xacNhan}
        khoa={khoa}
      />
      )}

      {dangMo?.kieu === "go" && coXacNhan && (
        <ModalDialog titleId={REASON_FORM_ID} onDismiss={dong} className="p-0">
          <FormLyDo
            formId={REASON_FORM_ID}
            className="shadow-none"
            tieuDe={`Gỡ chứng từ ngày ${nhanNgay(dangMo.ct.payment_date)}?`}
            busyText="Đang gỡ…"
            moTa={
              "Chứng từ được gỡ MỀM: hàng vẫn còn kèm người gỡ và lý do, nhưng nó thôi cộng vào số " +
              "đã giải ngân của dự án — tức là một con số đã báo cáo vừa thay đổi."
            }
            nhanNut="Gỡ chứng từ"
            dangGui={dangGui}
            loi={loi}
            huy={dong}
            xacNhan={(lyDo) => {
              go(dangMo.ct, lyDo);
            }}
          />
        </ModalDialog>
      )}

      {dangMo?.kieu === "moKhoa" && coXacNhan && (
        <ModalDialog titleId={REASON_FORM_ID} onDismiss={dong} className="p-0">
          <FormLyDo
            formId={REASON_FORM_ID}
            className="shadow-none"
            tieuDe={`Mở khoá chứng từ ngày ${nhanNgay(dangMo.ct.payment_date)}?`}
            tone="default"
            busyText="Đang mở khoá…"
            moTa={
              "Mở khoá đưa chứng từ về Đã xác nhận để sửa được. Lý do là bắt buộc, và NGƯỜI VỪA KHOÁ " +
              "không tự mở lại được — cần một cán bộ khác có quyền “Xác nhận, khoá khoản giải ngân”."
            }
            nhanNut="Mở khoá"
            dangGui={dangGui}
            loi={loi}
            huy={dong}
            xacNhan={(lyDo) => {
              moKhoa(dangMo.ct, lyDo);
            }}
          />
        </ModalDialog>
      )}
    </div>
  );
}
