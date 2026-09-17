"use client";

import { useCauHinhXa } from "@/components/cau-hinh-xa";

/**
 * Nền trái của màn đăng nhập: khối thương hiệu ViGov + tên xã (§1).
 *
 * `ViGov` và `ĐIỀU HÀNH SỐ CẤP XÃ` là hằng số của sản phẩm — viết thẳng được. Tên xã và cơ quan
 * cấp trên thì KHÔNG: chúng đi xuống qua ngữ cảnh do máy chủ dựng, đọc lúc chạy từ `Host`. Đây
 * chính là chỗ một `NEXT_PUBLIC_TEN_XA` sẽ len vào nếu không ai để ý — và một bundle không mang
 * nổi tên của 300 xã, nên nó sẽ kéo theo mỗi xã một bản dựng riêng.
 *
 * Tên xã viết HOA bằng CSS (`text-transform`), không viết HOA trong dữ liệu: chữ hoa là cách
 * trình bày, còn tên đúng của đơn vị hành chính là dữ liệu — và còn dùng ở chỗ khác.
 */
export function KhoiThuongHieu() {
  const xa = useCauHinhXa();

  return (
    <section className="cot-thuong-hieu">
      <div className="logo-vigov" aria-hidden="true">
        VG
      </div>
      <p className="ten-san-pham">ViGov</p>
      <p className="phu-de-san-pham">ĐIỀU HÀNH SỐ CẤP XÃ</p>
      <hr className="gach-ngang" />
      <p className="ten-xa">{xa.displayName}</p>
      <p className="co-quan-cap-tren">{xa.parentAuthority}</p>
    </section>
  );
}
