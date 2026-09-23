"use client";

import { useState, type FormEvent } from "react";

import { CAU_THIEU_LY_DO, LY_DO_TOI_DA, lyDoDuDung } from "./nhan-ghi-giai-ngan";

/**
 * Hộp xác nhận KÈM Ô LÝ DO, dùng chung cho ba thao tác `budget.confirm` có thân:
 * gỡ chứng từ · mở khoá chứng từ · xoá dự án.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * VÌ SAO LÀ MỘT Ô NHẬP CHỨ KHÔNG PHẢI MỘT NÚT `Đồng ý`. Đặc tả không biết điều này — §8.2 chỉ vẽ
 * `🗑 Gỡ`, §7 không vẽ gì cho việc xoá dự án. Máy chủ thì đòi `reason` trong thân và từ chối chuỗi
 * rỗng bằng một câu riêng, vì luật 7 bất biến 1 kể tên ba cột `deleted_at` · `deleted_by` ·
 * `delete_reason`, và câu hỏi mở #29 chốt rằng lý do MỞ KHOÁ cũng bắt buộc: những lần mở khoá đã
 * xảy ra không dựng lại được từ bất kỳ nguồn nào sau đó.
 *
 * Nên một hộp xác nhận không có ô lý do ở đây không phải là "gọn hơn" — nó là một biểu mẫu chắc
 * chắn nhận 400 ở mọi lần bấm.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * KHÔNG CÓ GIÁ TRỊ MẶC ĐỊNH CHO Ô LÝ DO. Một câu gợi sẵn kiểu "Nhập sai" là câu sẽ nằm trong vết
 * kiểm toán của hàng trăm bản ghi, và một vết kiểm toán mà mọi dòng giống nhau thì không trả lời
 * được câu hỏi nó sinh ra để trả lời.
 */
export function FormLyDo({
  tieuDe,
  moTa,
  nhanNut,
  dangGui,
  loi,
  huy,
  xacNhan,
}: {
  tieuDe: string;
  /** Câu nói TRƯỚC hậu quả của thao tác. Nhận vào chứ không viết cứng: ba thao tác, ba hậu quả. */
  moTa: string;
  nhanNut: string;
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  xacNhan: (lyDo: string) => void;
}) {
  const [lyDo, datLyDo] = useState("");
  const duDieuKien = lyDoDuDung(lyDo);

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien) return;
    xacNhan(lyDo.trim());
  }

  return (
    <form className="form-danh-muc" onSubmit={guiNgay} aria-label={tieuDe}>
      <h4>{tieuDe}</h4>
      <p className="ghi-chu">{moTa}</p>

      <div className="o-nhap">
        <label htmlFor="ly-do-giai-ngan">Lý do *</label>
        <textarea
          id="ly-do-giai-ngan"
          name="ly-do-giai-ngan"
          rows={3}
          value={lyDo}
          maxLength={LY_DO_TOI_DA}
          onChange={(e) => datLyDo(e.target.value)}
        />
        {!duDieuKien && <p className="ghi-chu">{CAU_THIEU_LY_DO}</p>}
      </div>

      {/* NGUYÊN VĂN câu máy chủ: 409 của các tuyến này mang đúng quy tắc nghiệp vụ đã từ chối
          ("dự án còn chứng từ", "người vừa khoá không tự mở lại được"), và viết lại nó ở client là
          dựng bản sao thứ hai của một quy tắc rồi để nó trôi. */}
      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          Huỷ
        </button>
        <button type="submit" className="nut-xoa" disabled={dangGui || !duDieuKien}>
          {nhanNut}
        </button>
      </div>
    </form>
  );
}
