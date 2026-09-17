"use client";

import { NutDangXuat } from "@/features/auth/nut-dang-xuat";

import { useCauHinhXa } from "./cau-hinh-xa";

/**
 * Đầu trang — `15-phu-luc-giao-dien-chung` §3.
 *
 * Bên trái là tên cơ quan và cơ quan cấp trên, đọc lúc chạy từ cấu hình xã. Một tên xã sai trên
 * đầu trang của một cơ quan nhà nước là sự cố có người phải trả lời, nên nó không bao giờ là
 * hằng số trong mã và không bao giờ là giá trị dự phòng.
 *
 * Ô tìm kiếm toàn hệ thống (§3.1) và chuông thông báo (§3.2) chưa có ở đây: chưa có route nào
 * trong hợp đồng phục vụ chúng. Vẽ ra một ô tìm kiếm không tìm được gì là hứa với cán bộ một
 * chức năng không tồn tại.
 */
export function DauTrang() {
  const xa = useCauHinhXa();

  return (
    <header className="dau-trang">
      <div className="khoi-co-quan">
        <p className="ten-co-quan">{xa.displayName}</p>
        <p className="co-quan-cap-tren">{xa.parentAuthority}</p>
      </div>
      <NutDangXuat />
    </header>
  );
}
