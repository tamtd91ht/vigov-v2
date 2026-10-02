"use client";

import { NutDangXuat } from "@/features/auth/nut-dang-xuat";
import { khoiNguoiDung } from "@/features/phien/khoi-nguoi-dung";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { useCauHinhXa } from "./cau-hinh-xa";
import { CommuneIdentity } from "./commune-identity";
import { NotificationBell } from "./notification-bell";
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
 * Ô tìm kiếm toàn hệ thống (§3.1) vẫn chưa có: chưa có route nào trong hợp đồng phục vụ nó. Vẽ ra
 * một ô tìm kiếm không tìm được gì là hứa với cán bộ một chức năng không tồn tại. Khoảng trống bên
 * trái chừa cho nó (`ROADMAP_PHASE2.md`, ADR 0068 §11).
 */
export function DauTrang({ withCommune = false }: { withCommune?: boolean }) {
  const xa = useCauHinhXa();
  const phien = usePhien();
  const nguoi = khoiNguoiDung(phien);
  const roleName = sessionRoleName(phien);

  // Left: commune (only with no sidebar) + role pill. The MIDDLE stays empty on purpose — room for
  // the future system-wide search (ROADMAP_PHASE2 "chừa sẵn"); `justify-content: space-between`
  // keeps it open.
  return (
    <header className="dau-trang">
      <div className="khoi-co-quan">
        {withCommune && <CommuneIdentity commune={xa} />}
        {roleName !== null && <RolePill roleName={roleName} />}
      </div>
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
