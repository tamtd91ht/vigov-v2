/**
 * MẢNH GIAO DIỆN DÙNG CHUNG CỦA APP RIÊNG MỘT XÃ — theo bản mẫu `vi-gov/zalo-miniapp`
 * (`components/common.tsx`, `components/DataState.tsx`), lớp CSS `.xa-*` trong `styles.css`.
 *
 * Khác bản mẫu ở chỗ bắt buộc (`skills/accessibility-elderly`, `accessibility.test.ts`): chữ không dưới
 * 16px, đích chạm không dưới 48px, chữ phụ dùng `--ink-muted` (bản mẫu dùng một màu xám chỉ 3,2:1 trên
 * nền trắng), và không vòng quay động — "đang tải" là CHỮ.
 */
import type { ReactNode } from "react";

import { BACK, COMMUNE_APP_UI } from "./copy";
import { Icon, type IconName } from "./Icon";

/** Header của màn con: nút quay lại + tiêu đề. Nút Back của Zalo đóng cả app, nên đây là đường về. */
export function SubScreenHeader({ title, onBack }: { title: string; onBack: () => void }) {
  return (
    <div className="xa-dau-con">
      <button type="button" className="xa-dau-con__lui" onClick={onBack}>
        <Icon name="back" size={22} />
        <span>{BACK}</span>
      </button>
      <h1 className="xa-dau-con__tieu-de">{title}</h1>
    </div>
  );
}

/** Tiêu đề một khối trên trang chủ + "Xem tất cả". */
export function SectionHeader({ title, onViewAll }: { title: string; onViewAll?: () => void }) {
  return (
    <div className="xa-dau-khoi">
      <h2 className="xa-dau-khoi__tieu-de">{title}</h2>
      {onViewAll && (
        <button type="button" className="xa-dau-khoi__them" onClick={onViewAll}>
          {COMMUNE_APP_UI.view_all}
        </button>
      )}
    </div>
  );
}

/** Ô biểu tượng tròn nền nhạt; màu là tên một lớp `xa-mau--*` đã đo trong `styles.css`. */
export function IconTile({ name, color }: { name: IconName; color: "hong" | "xanh" | "luc" | "cam" | "navy" | "tim" }) {
  return (
    <span className={`xa-o-bt xa-mau--${color}`}>
      <Icon name={name} size={24} />
    </span>
  );
}

/** Khối trạng thái: đang tải · rỗng · lỗi (kèm nút). Luôn là chữ đọc được, không chỉ biểu tượng. */
export function StatusBlock(props: {
  icon: IconName;
  text: string;
  error?: boolean;
  loading?: boolean;
  button?: { label: string; onPress: () => void };
}) {
  return (
    <div className={`xa-trang-thai${props.error ? " xa-trang-thai--loi" : ""}`}>
      <Icon name={props.icon} size={36} />
      <p role={props.error ? "alert" : props.loading ? "status" : undefined}>{props.text}</p>
      {props.button && (
        <button type="button" className="xa-nut" onClick={props.button.onPress}>
          {props.button.label}
        </button>
      )}
    </div>
  );
}

/**
 * Chỗ của một việc cần đăng nhập (gửi, xem, tra cứu phản ánh) khi app riêng chưa có phiên. Đường đăng
 * nhập theo App ID của app xã CHƯA DỰNG (ADR 0047 §6); nói thật điều ấy, và chỉ người dân tới xã.
 */
export function NotLoggedIn({ text }: { text: string }) {
  return (
    <div className="xa-the xa-the--dem xa-ghi-chu">
      <Icon name="info" size={22} />
      <div>
        <p className="xa-ghi-chu__tieu-de">{COMMUNE_APP_UI.not_logged_in_title}</p>
        <p>{text}</p>
      </div>
    </div>
  );
}

export function SubPage({ children }: { children: ReactNode }) {
  return <div className="xa-trang xa-trang--con">{children}</div>;
}
