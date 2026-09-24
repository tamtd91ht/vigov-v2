"use client";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  CANH_BAO_CONG_KHAI,
  CANH_BAO_RUT,
  MO_TA_THU_TU,
  NUT_XAC_NHAN_CONG_KHAI,
  NUT_XAC_NHAN_RUT,
  O_DA_HOI_Y,
  O_THU_TU,
  guiCongKhaiDuoc,
  tieuDeCongKhai,
  tieuDeRut,
  type BanCongKhai,
} from "./cong-khai";

/** Hộp đang mở: công khai hay rút, cho ĐÚNG MỘT người. */
export type DangMoCongKhai =
  | { kieu: "congKhai"; canBo: identity_canBoTomTat }
  | { kieu: "rut"; canBo: identity_canBoTomTat };

/**
 * Hộp công khai / rút một cán bộ khỏi danh bạ Zalo Mini App (#12).
 *
 * THUẦN TRÌNH BÀY, cùng khuôn `BieuMauGhiCanBo`: giá trị vào qua `ban`, thay đổi ra qua `datBan`,
 * lời gọi mạng ở chỗ gọi. Nhờ vậy nhánh "nút còn mờ vì chưa tick" kết xuất được bằng
 * `react-dom/server` mà không cần trình duyệt giả lập.
 *
 * MỘT BIỂU MẪU, ĐẶT TRÊN BẢNG, KHÔNG LỒNG TRONG DÒNG — cùng lý do với biểu mẫu sửa: ở 320px bảng
 * cuộn ngang, một hộp nằm trong ô bảng có thể mở ra ngoài khung nhìn. Tiêu đề gọi tên người, nên
 * không có ca công khai nhầm số của một người khác.
 */
export function HopCongKhai({
  dangMo,
  ban,
  datBan,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  dangMo: DangMoCongKhai;
  ban: BanCongKhai;
  datBan: (b: BanCongKhai) => void;
  loiMayChu: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const congKhai = dangMo.kieu === "congKhai";
  const tieuDe = congKhai ? tieuDeCongKhai(dangMo.canBo.full_name) : tieuDeRut(dangMo.canBo.full_name);

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

      <p className="canh-bao-pham-vi">{congKhai ? CANH_BAO_CONG_KHAI : CANH_BAO_RUT}</p>

      {congKhai && (
        <>
          {/* Ô tick BẮT BUỘC, KHÔNG TICK SẴN (`banCongKhaiTu`). `required` để trình duyệt nhắc; nút
              gửi mờ đi cho tới khi tick; và `yeuCauCongKhai` từ chối nếu vẫn tới được đó. */}
          <div className="o-nhap">
            <label htmlFor="o-da-hoi-y">
              <input
                id="o-da-hoi-y"
                name="o-da-hoi-y"
                type="checkbox"
                required
                checked={ban.daHoiY}
                onChange={(e) => datBan({ ...ban, daHoiY: e.target.checked })}
              />{" "}
              {O_DA_HOI_Y}
            </label>
          </div>

          <div className="o-nhap">
            <label htmlFor="o-thu-tu-mini-app">{O_THU_TU}</label>
            <input
              id="o-thu-tu-mini-app"
              name="o-thu-tu-mini-app"
              type="number"
              inputMode="numeric"
              min={0}
              step={1}
              value={ban.thuTu}
              aria-describedby="o-thu-tu-mini-app-mo-ta"
              onChange={(e) => datBan({ ...ban, thuTu: e.target.value })}
            />
            <p className="ghi-chu" id="o-thu-tu-mini-app-mo-ta">
              {MO_TA_THU_TU}
            </p>
          </div>
        </>
      )}

      {/* Câu máy chủ hiện NGUYÊN VĂN — 400 `consent_required`, 400 `invalid_request`, 404 — không
          rẽ nhánh theo `code`, không viết lại. */}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiMayChu}
        </p>
      )}

      <div className="cum-nut">
        <button
          type="submit"
          className="nut-chinh"
          disabled={dangGui || (congKhai && !guiCongKhaiDuoc(ban))}
        >
          {congKhai ? NUT_XAC_NHAN_CONG_KHAI : NUT_XAC_NHAN_RUT}
        </button>
        <button type="button" className="nut-phu" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </button>
      </div>
    </form>
  );
}
