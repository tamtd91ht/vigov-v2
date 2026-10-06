"use client";

import { ChevronDown, ChevronRight, Pencil, Plus, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { suaDuAn, themDuAn, xoaDuAn } from "@/lib/api/giai-ngan";
import type { finance_duAnRa, finance_hangMucRa } from "@/lib/api/schema.gen";

import { FormLyDo } from "./form-ly-do";
import { nhanTien } from "./nhan-du-an";
import {
  docSoTien,
  FORM_DU_AN_TRONG,
  MA_DU_AN_TOI_DA,
  MO_TA_DU_AN_TOI_DA,
  TEN_DU_AN_TOI_DA,
  thanSuaDuAn,
  thanThemDuAn,
  type GiaTriFormDuAn,
} from "./nhan-ghi-giai-ngan";
import { AutoCodePending, FundingListPending, UnitAndOfficerPending } from "./pending-parts";
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

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (thieuDanhMuc) return;
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

      <FundingListPending />

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
          disabled={dangGui || thieuDanhMuc}
          aria-busy={dangGui || undefined}
        >
          <BusyLabel busy={dangGui} label={isEdit ? "Lưu dự án" : "Thêm dự án"} busyText={BUSY_SAVING} />
        </Button>
      </div>
    </form>
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
  };
}

/**
 * The detail card header's buttons: `Sửa dự án` (`budget.update`) and `Gỡ dự án` (`budget.confirm`).
 *
 * HAI CỔNG RIÊNG, KHÔNG MỘT. Đây là chỗ dễ gắn nhầm nhất của phân hệ: gộp hai khoá lại thì người
 * chỉ được nhập liệu bỗng xoá được cả một dự án khỏi mọi con số tổng của năm. Hiding is UX only —
 * the server checks each key on its route (rule 5).
 *
 * While the edit panel is open the edit button reads `Đang sửa` in the solid look and closes it
 * again (prototype `BudgetItemDetail.tsx:264-273`).
 */
export function ProjectHeaderActions({
  coGhi,
  coXacNhan,
  editing,
  onToggleEdit,
  onRemove,
}: {
  coGhi: boolean;
  coXacNhan: boolean;
  editing: boolean;
  onToggleEdit: () => void;
  onRemove: () => void;
}) {
  if (!coGhi && !coXacNhan) return null;
  return (
    <div className="flex shrink-0 flex-wrap items-center gap-2">
      {coGhi && (
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
      )}
      {/* Removal is the danger look, never the solid colour, and keeps its words (spec v2 §7). */}
      {coXacNhan && (
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
      )}
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
        dangGui={dangGui}
        loi={loi}
        huy={onClose}
        luu={(gt) => sua(gt)}
      />
    </section>
  );
}

const REMOVE_FORM_ID = "form-go-du-an";

/**
 * `Gỡ dự án` — the reason form in a centred dialog (the prototype confirms removals in a dialog).
 * Xoá MỀM kèm lý do (luật 7); the server refuses while vouchers remain and says so verbatim.
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

  function xoa(lyDo: string): void {
    datDangGui(true);
    xoaDuAn(duAn.id, lyDo).then((kq) => {
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
      <FormLyDo
        formId={REMOVE_FORM_ID}
        className="shadow-none"
        tieuDe={`Gỡ dự án “${duAn.name}”?`}
        busyText="Đang gỡ…"
        moTa={
          "Dự án được xoá MỀM: hàng vẫn còn kèm người xoá và lý do, và mã dự án KHÔNG quay lại " +
          "dãy — nhập lại phải chọn mã khác. Dự án còn chứng từ giải ngân thì máy chủ từ chối, " +
          "phải gỡ từng chứng từ kèm lý do trước."
        }
        nhanNut="Gỡ dự án"
        dangGui={dangGui}
        loi={loi}
        huy={onClose}
        xacNhan={xoa}
      />
    </ModalDialog>
  );
}
