"use client";

import { useState } from "react";

import type { finance_bangRa } from "@/lib/api/schema.gen";
import type { SuaBangVao } from "@/lib/api/thu-chi";

import { DON_VI_TINH, dungThanSuaBang, GOI_Y_DOI_DON_VI, laMaDonVi } from "./nhan-thu-chi";

/**
 * Biểu mẫu **Sửa thông tin bảng**: tiêu đề, `Luỹ kế đến`, đơn vị tính (§6 "Luỹ kế … nhập tay").
 *
 * BA TRƯỜNG, KHÔNG HƠN: `PATCH /budget-sheets/{id}` từ chối năm, loại, mã và bộ cột bằng 400 — đổi
 * những thứ ấy là đổi bảng này thành một bảng khác. Biểu mẫu chỉ gửi trường THẬT SỰ đổi
 * (`dungThanSuaBang`).
 *
 * BẢNG MANG ĐƠN VỊ CŨ CHƯA ÁNH XẠ ĐƯỢC (`unit` rỗng) THÌ Ô CHỌN KHÔNG CHỌN SẴN GÌ: chọn thay cán bộ
 * là đoán hệ số chia cho mọi con số của bảng — đúng điều máy chủ đã từ chối đoán.
 */
export function FormSuaBang({
  bang,
  dangGui,
  huy,
  luu,
}: {
  bang: finance_bangRa;
  dangGui: boolean;
  huy: () => void;
  luu: (than: SuaBangVao) => void;
}) {
  const [loi, datLoi] = useState<string | null>(null);

  return (
    <form
      className="khoi-chi-tiet"
      onSubmit={(e) => {
        e.preventDefault();
        const fd = new FormData(e.currentTarget);
        const dung = dungThanSuaBang(bang, {
          tieuDe: String(fd.get("title") ?? ""),
          luyKe: String(fd.get("cumulative_to") ?? ""),
          donVi: String(fd.get("unit") ?? ""),
        });
        if (!dung.ok) {
          datLoi(dung.thongBao);
          return;
        }
        datLoi(null);
        luu(dung.than);
      }}
    >
      <div className="dau-khoi-chi-tiet">
        <h3>Sửa thông tin bảng</h3>
      </div>

      <p>
        <label htmlFor="sua-bang-title">Tiêu đề bảng (in trên đầu báo cáo)</label>{" "}
        <input
          id="sua-bang-title"
          name="title"
          className="o-nhap"
          type="text"
          required
          maxLength={300}
          defaultValue={bang.title}
        />
      </p>

      <p>
        <label htmlFor="sua-bang-cumulative">Luỹ kế đến</label>{" "}
        <input
          id="sua-bang-cumulative"
          name="cumulative_to"
          className="o-nhap"
          type="date"
          defaultValue={bang.cumulative_to ?? ""}
        />
      </p>
      <p className="ghi-chu">Để trống để bỏ mốc luỹ kế khỏi đầu báo cáo.</p>

      <p>
        <label htmlFor="sua-bang-unit">Đơn vị tính</label>{" "}
        <select
          id="sua-bang-unit"
          name="unit"
          required
          defaultValue={laMaDonVi(bang.unit) ? bang.unit : ""}
        >
          {!laMaDonVi(bang.unit) && <option value="">— Chọn đơn vị tính —</option>}
          {DON_VI_TINH.map((d) => (
            <option key={d.ma} value={d.ma}>
              {d.nhan}
            </option>
          ))}
        </select>
      </p>
      <p className="ghi-chu">{GOI_Y_DOI_DON_VI}</p>

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <button type="submit" className="nut-chinh" disabled={dangGui}>
        Lưu thông tin bảng
      </button>{" "}
      <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
        Huỷ
      </button>
    </form>
  );
}
