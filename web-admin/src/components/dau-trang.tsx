"use client";

import { Search } from "lucide-react";

import { NutDangXuat } from "@/features/auth/nut-dang-xuat";
import { khoiNguoiDung } from "@/features/phien/khoi-nguoi-dung";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { useCauHinhXa } from "./cau-hinh-xa";
import { CommuneIdentity } from "./commune-identity";
import { NotificationBell } from "./notification-bell";
import { PendingMarker, type PendingFeatureInfo } from "./ui/pending-feature";
import { RolePill, sessionRoleName } from "./role-pill";
import { userInitials } from "./user-initials";

/**
 * Đầu trang — spec giao diện 02/10/2026 §5 (thay `15-phu-luc-giao-dien-chung` §3 về hình thức).
 *
 * TÊN CƠ QUAN ĐÃ CHUYỂN SANG GÓC TRÁI CỦA THANH BÊN (chủ dự án 02/10/2026) — `CommuneIdentity`.
 * Đầu trang chỉ in lại nó khi trang KHÔNG có thanh bên (`withCommune`: đổi mật khẩu, chi tiết dự
 * án), để không trang đã đăng nhập nào thiếu tên cơ quan; in ở cả hai chỗ là lặp. Tên xã đọc lúc
 * chạy từ cấu hình xã, không bao giờ là hằng số hay giá trị dự phòng.
 *
 * Bên phải là khối người dùng — họ tên và chức vụ của chính người đang đăng nhập, đọc từ
 * `GET /api/v1/sessions/current` qua `PhienProvider`. Không đọc được thì khối ấy BIẾN MẤT chứ
 * không có tên dự phòng nào: xem `khoi-nguoi-dung.ts`. Ô chữ tắt (avatar) đi cùng khối ấy, nên nó
 * cũng biến mất cùng.
 *
 * Chuông thông báo (§3.2, `08-thong-bao §8`) có ở đây từ khi `service-comms` mở hộp chuông của
 * chính cán bộ (`/api/v1/notifications`). Nó chỉ vẽ khi đã đọc được phiên: không có phiên thì
 * không có hộp thư của ai để đếm, và bốn tuyến ấy sẽ chỉ trả 401.
 *
 * Ô tìm kiếm toàn hệ thống (§3.1) chưa dựng — giai đoạn 2 (`ROADMAP_PHASE2.md` mục 1). Theo ADR 0068
 * §14 nó đứng ĐÚNG CHỖ của nó, giữa đầu trang, dưới dạng ô nhập bị vô hiệu kèm dấu "?": không tìm,
 * không gọi máy chủ, không lưu gì (`SystemSearchPlaceholder`).
 */
export function DauTrang({ withCommune = false }: { withCommune?: boolean }) {
  const xa = useCauHinhXa();
  const phien = usePhien();
  const nguoi = khoiNguoiDung(phien);
  const roleName = sessionRoleName(phien);

  // Left: commune (only with no sidebar) + role pill. Middle: the Phase-2 system-wide search, as a
  // disabled field with "?". Right: bell, person, sign-out.
  return (
    <header className="dau-trang">
      <div className="khoi-co-quan">
        {withCommune && <CommuneIdentity commune={xa} />}
        {roleName !== null && <RolePill roleName={roleName} />}
      </div>
      <SystemSearchPlaceholder />
      <div className="khoi-nguoi-dung">
        {nguoi.hien && <NotificationBell />}
        {nguoi.hien ? (
          <div className="topbar-user">
            {userInitials(nguoi.hoTen) !== "" && (
              <span className="topbar-avatar" aria-hidden="true">
                {userInitials(nguoi.hoTen)}
              </span>
            )}
            <div className="topbar-user-text">
              <p className="ho-ten">{nguoi.hoTen}</p>
              <p className="chuc-vu">{nguoi.chucVu}</p>
            </div>
          </div>
        ) : null}
        <NutDangXuat />
      </div>
    </header>
  );
}

/**
 * Description behind the search box's "?". A constant HERE, not a `PHAN_CHUA_DUNG` entry: the shell is
 * no screen of its own — `tools/tien_do_san_pham.py` counts `PHAN_CHUA_DUNG` per `features/<dir>` and
 * maps each page to the dirs it imports, so an entry in any feature would be counted under that one
 * screen, and an entry in every feature would count it eleven times. Like the menu's `PENDING_SCREENS`
 * (`muc-menu.ts`), the shell keeps its own one sentence.
 *
 * TRUE TODAY: the contract has staff search (`/api/v1/staff/searches`) and per-register filters, but
 * no route that searches across modules (tasks, documents, citizen reports) for one query.
 */
export const SYSTEM_SEARCH_PENDING: PendingFeatureInfo = {
  ten: "Tìm kiếm toàn hệ thống",
  viSao:
    "Một ô tìm được cùng lúc nhiệm vụ, văn bản và phản ánh, kết quả lọc theo quyền của từng người. " +
    "Hệ thống chưa tìm chung được trên các phân hệ; hiện mỗi sổ có bộ lọc riêng trên màn của nó.",
};

/**
 * `docs/ui-ux/00` §3.1: placeholder `Tìm nhiệm vụ, văn bản, phản ánh…`, aria-label `Tìm kiếm toàn hệ
 * thống`. A DISABLED native input, never a `<form>` or `role="search"`: there is nothing to submit,
 * and a search landmark that searches nothing misleads a screen-reader user. The "?" sits inside the
 * right padding, like `PendingButton`.
 *
 * Below 768px it takes its own row under the commune/role block (`order-last basis-full`), so the
 * person block keeps its place at 320px.
 */
function SystemSearchPlaceholder() {
  return (
    <div
      className="relative order-last flex min-w-0 basis-full items-center md:order-none md:max-w-md md:flex-1 md:basis-auto"
      data-pending=""
    >
      <Search
        aria-hidden="true"
        focusable="false"
        strokeWidth={1.8}
        className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-400"
      />
      <input
        type="search"
        disabled
        aria-label="Tìm kiếm toàn hệ thống"
        placeholder="Tìm nhiệm vụ, văn bản, phản ánh…"
        className="h-10 w-full min-w-0 cursor-not-allowed rounded-control border border-line bg-[#f6f8fb] pr-9 pl-9 text-sm text-ink-500 placeholder:text-ink-400"
      />
      <PendingMarker info={SYSTEM_SEARCH_PENDING} phase2 placement="end" side="bottom" />
    </div>
  );
}
