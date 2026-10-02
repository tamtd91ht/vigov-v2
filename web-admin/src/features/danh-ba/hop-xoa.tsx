"use client";

import { Trash2 } from "lucide-react";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BUSY_DELETING, BusyLabel } from "./busy-label";
import {
  CAU_CO_TAI_KHOAN,
  GIAI_THICH_XOA,
  MO_TA_LY_DO_XOA,
  NUT_XAC_NHAN_XOA,
  O_LY_DO_XOA,
  deleteQuestion,
  tieuDeXoa,
} from "./xoa-dong";

/**
 * Hộp xác nhận xoá MỘT dòng danh bạ nhập trùng.
 *
 * THUẦN TRÌNH BÀY, cùng khuôn `HopCongKhai`: giá trị vào qua `lyDo`, thay đổi ra qua `datLyDo`, lời
 * gọi mạng ở chỗ gọi. Đặt trên bảng, không lồng trong dòng — cùng lý do ở 320px. `ConfirmDialog`
 * chỉ là khung: biểu mẫu, ô lý do, hai nút và handler vẫn là của hộp này, y như trước.
 *
 * DÒNG CÓ TÀI KHOẢN: hộp vẫn mở (để người bấm đọc được VÌ SAO), nhưng KHÔNG có ô lý do và KHÔNG có
 * nút xác nhận — chỉ câu giải thích và nút Huỷ. Một nút gửi mà máy chủ chắc chắn từ chối là một nút
 * mời bấm cho biết.
 */
export function HopXoa({
  canBo,
  lyDo,
  datLyDo,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  canBo: identity_canBoTomTat;
  lyDo: string;
  datLyDo: (s: string) => void;
  loiMayChu: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const tieuDe = tieuDeXoa(canBo.full_name);
  const coTaiKhoan = canBo.has_account;

  return (
    <ConfirmDialog
      as="form"
      tone="danger"
      titleAs="h4"
      title={deleteQuestion(canBo.full_name)}
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
      actions={
        <>
          <Button type="button" variant="secondary" onClick={onHuy} disabled={dangGui}>
            {NUT_HUY}
          </Button>
          {!coTaiKhoan && (
            <Button
              type="submit"
              variant="danger"
              icon={dangGui ? undefined : <Trash2 aria-hidden="true" />}
              disabled={dangGui}
              aria-busy={dangGui}
            >
              <BusyLabel busy={dangGui} label={NUT_XAC_NHAN_XOA} busyText={BUSY_DELETING} />
            </Button>
          )}
        </>
      }
    >
      <p className="m-0 text-xs text-ink-500">Mã cán bộ: {canBo.code}</p>

      <p className="m-0">{GIAI_THICH_XOA}</p>

      {coTaiKhoan ? (
        <p className="thong-bao-loi m-0" role="alert">
          {CAU_CO_TAI_KHOAN}
        </p>
      ) : (
        <div className="flex flex-col gap-1.5">
          <label htmlFor="o-ly-do-xoa-can-bo" className="text-xs font-semibold text-ink-700">
            {O_LY_DO_XOA}
          </label>
          {/* `required` là lớp NHẮC của trình duyệt; phép kiểm thật là `yeuCauXoa` (cắt khoảng trắng,
              đếm ký tự) rồi máy chủ. Không `maxLength`: nó đếm đơn vị UTF-16, không đếm ký tự. */}
          <textarea
            id="o-ly-do-xoa-can-bo"
            name="o-ly-do-xoa-can-bo"
            required
            rows={3}
            value={lyDo}
            aria-describedby="o-ly-do-xoa-can-bo-mo-ta"
            onChange={(e) => datLyDo(e.target.value)}
            className="w-full min-w-0 rounded-control border border-line-strong bg-surface px-3 py-2 [font-family:inherit] text-base text-ink-900 focus-visible:border-brand-500 focus-visible:shadow-[0_0_0_3px_var(--brand-100)] focus-visible:outline-none md:text-sm"
          />
          <p className="m-0 text-xs text-ink-500" id="o-ly-do-xoa-can-bo-mo-ta">
            {MO_TA_LY_DO_XOA}
          </p>
        </div>
      )}

      {/* Câu máy chủ hiện NGUYÊN VĂN — 409 staff_has_account / last_admin, 403 self_target_forbidden,
          404, 400 — không rẽ nhánh theo `code`, không viết lại. */}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {loiMayChu}
        </p>
      )}
    </ConfirmDialog>
  );
}
