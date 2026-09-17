"use client";

import { NutDangXuat } from "@/features/auth/nut-dang-xuat";
import { khoiNguoiDung } from "@/features/phien/khoi-nguoi-dung";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { useCauHinhXa } from "./cau-hinh-xa";

/**
 * Đầu trang — `15-phu-luc-giao-dien-chung` §3.
 *
 * Bên trái là tên cơ quan và cơ quan cấp trên, đọc lúc chạy từ cấu hình xã. Một tên xã sai trên
 * đầu trang của một cơ quan nhà nước là sự cố có người phải trả lời, nên nó không bao giờ là
 * hằng số trong mã và không bao giờ là giá trị dự phòng.
 *
 * Bên phải là khối người dùng — họ tên và chức vụ của chính người đang đăng nhập, đọc từ
 * `GET /api/v1/sessions/current` qua `PhienProvider`. Không đọc được thì khối ấy BIẾN MẤT chứ
 * không có tên dự phòng nào: xem `khoi-nguoi-dung.ts`.
 *
 * Ô tìm kiếm toàn hệ thống (§3.1) và chuông thông báo (§3.2) chưa có ở đây: chưa có route nào
 * trong hợp đồng phục vụ chúng. Vẽ ra một ô tìm kiếm không tìm được gì là hứa với cán bộ một
 * chức năng không tồn tại.
 */
export function DauTrang() {
  const xa = useCauHinhXa();
  const nguoi = khoiNguoiDung(usePhien());

  return (
    <header className="dau-trang">
      <div className="khoi-co-quan">
        <p className="ten-co-quan">{xa.displayName}</p>
        <p className="co-quan-cap-tren">{xa.parentAuthority}</p>
      </div>
      <div className="khoi-nguoi-dung">
        {nguoi.hien ? (
          <>
            <p className="ho-ten">{nguoi.hoTen}</p>
            <p className="chuc-vu">{nguoi.chucVu}</p>
          </>
        ) : null}
        <NutDangXuat />
      </div>
    </header>
  );
}
