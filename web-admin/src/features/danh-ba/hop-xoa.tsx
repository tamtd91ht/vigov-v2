"use client";

import { Trash2 } from "lucide-react";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BUSY_DELETING, BusyLabel } from "./busy-label";
import { CAU_CO_TAI_KHOAN, GIAI_THICH_XOA, NUT_XAC_NHAN_XOA, deleteQuestion, tieuDeXoa } from "./xoa-dong";

/**
 * Hộp xác nhận xoá MỘT dòng danh bạ nhập trùng.
 *
 * THUẦN TRÌNH BÀY, cùng khuôn `HopCongKhai`: lời gọi mạng ở chỗ gọi. NO REASON BOX (bug sheet row 27,
 * owner decision 10/10/2026): the prototype's question, its sentence, Huỷ and the red button. Đặt trên bảng, không lồng trong dòng — cùng lý do ở 320px. `ConfirmDialog`
 * chỉ là khung: biểu mẫu, hai nút và handler vẫn là của hộp này, y như trước.
 *
 * DÒNG CÓ TÀI KHOẢN: hộp vẫn mở (để người bấm đọc được VÌ SAO), nhưng KHÔNG có nút xác nhận — chỉ câu giải thích và nút Huỷ. Một nút gửi mà máy chủ chắc chắn từ chối là một nút
 * mời bấm cho biết.
 */
export function HopXoa({
  canBo,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  canBo: identity_canBoTomTat;
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

      {coTaiKhoan && (
        <p className="thong-bao-loi m-0" role="alert">
          {CAU_CO_TAI_KHOAN}
        </p>
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
