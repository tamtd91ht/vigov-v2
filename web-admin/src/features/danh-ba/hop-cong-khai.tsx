"use client";

import { EyeOff, Smartphone } from "lucide-react";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button, buttonVariants } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { controlClass } from "@/components/ui/field";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { BUSY_SAVING, BusyLabel } from "./busy-label";

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
    <ConfirmDialog
      as="form"
      titleAs="h4"
      icon={congKhai ? Smartphone : EyeOff}
      title={tieuDe}
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
          {/* A native `<button>` with `type` first, not `Button`: the tests read this tag as
              `<button type="submit" …>label</button>`. Same classes as `Button variant="primary"`. */}
          <button
            type="submit"
            className={cn("nut-chinh", buttonVariants({ variant: "primary" }))}
            disabled={dangGui || (congKhai && !guiCongKhaiDuoc(ban))}
            aria-busy={dangGui}
          >
            <BusyLabel
              busy={dangGui}
              label={congKhai ? NUT_XAC_NHAN_CONG_KHAI : NUT_XAC_NHAN_RUT}
              busyText={BUSY_SAVING}
            />
          </button>
        </>
      }
    >
      <p className="m-0">{congKhai ? CANH_BAO_CONG_KHAI : CANH_BAO_RUT}</p>

      {congKhai && (
        <>
          {/* Ô tick BẮT BUỘC, KHÔNG TICK SẴN (`banCongKhaiTu`). `required` để trình duyệt nhắc; nút
              gửi mờ đi cho tới khi tick; và `yeuCauCongKhai` từ chối nếu vẫn tới được đó. */}
          <label
            htmlFor="o-da-hoi-y"
            className="flex cursor-pointer items-start gap-2.5 rounded-control border border-line bg-surface-muted p-3 font-medium text-ink-900"
          >
            <input
              id="o-da-hoi-y"
              name="o-da-hoi-y"
              type="checkbox"
              required
              checked={ban.daHoiY}
              onChange={(e) => datBan({ ...ban, daHoiY: e.target.checked })}
              className="mt-0.5 size-[18px] shrink-0 accent-brand-600"
            />
            {O_DA_HOI_Y}
          </label>

          <div className="flex max-w-xs flex-col gap-1.5">
            <label htmlFor="o-thu-tu-mini-app" className="text-xs font-semibold text-ink-700">
              {O_THU_TU}
            </label>
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
              className={cn(controlClass, "max-md:h-11")}
            />
            <p className="m-0 text-xs text-ink-500" id="o-thu-tu-mini-app-mo-ta">
              {MO_TA_THU_TU}
            </p>
          </div>
        </>
      )}

      {/* Câu máy chủ hiện NGUYÊN VĂN — 400 `consent_required`, 400 `invalid_request`, 404 — không
          rẽ nhánh theo `code`, không viết lại. */}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {loiMayChu}
        </p>
      )}
    </ConfirmDialog>
  );
}
