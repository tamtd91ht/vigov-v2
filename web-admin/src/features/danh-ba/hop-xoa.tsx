"use client";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  CAU_CO_TAI_KHOAN,
  GIAI_THICH_XOA,
  MO_TA_LY_DO_XOA,
  NUT_XAC_NHAN_XOA,
  O_LY_DO_XOA,
  tieuDeXoa,
} from "./xoa-dong";

/**
 * Hộp xác nhận xoá MỘT dòng danh bạ nhập trùng.
 *
 * THUẦN TRÌNH BÀY, cùng khuôn `HopCongKhai`: giá trị vào qua `lyDo`, thay đổi ra qua `datLyDo`, lời
 * gọi mạng ở chỗ gọi. Đặt trên bảng, không lồng trong dòng — cùng lý do ở 320px.
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
    <form
      className="form-danh-muc"
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <h4>{tieuDe}</h4>
      <p className="ghi-chu">Mã cán bộ: {canBo.code}</p>

      <p className="canh-bao-pham-vi">{GIAI_THICH_XOA}</p>

      {coTaiKhoan ? (
        <p className="thong-bao-loi" role="alert">
          {CAU_CO_TAI_KHOAN}
        </p>
      ) : (
        <div className="o-nhap">
          <label htmlFor="o-ly-do-xoa-can-bo">{O_LY_DO_XOA}</label>
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
          />
          <p className="ghi-chu" id="o-ly-do-xoa-can-bo-mo-ta">
            {MO_TA_LY_DO_XOA}
          </p>
        </div>
      )}

      {/* Câu máy chủ hiện NGUYÊN VĂN — 409 staff_has_account / last_admin, 403 self_target_forbidden,
          404, 400 — không rẽ nhánh theo `code`, không viết lại. */}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiMayChu}
        </p>
      )}

      <div className="cum-nut">
        {!coTaiKhoan && (
          <button type="submit" className="nut-phu nut-xoa" disabled={dangGui}>
            {NUT_XAC_NHAN_XOA}
          </button>
        )}
        <button type="button" className="nut-phu" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </button>
      </div>
    </form>
  );
}
