"use client";

import { Banknote, Landmark, ListChecks, Mail, MessageSquareWarning, Star, type LucideIcon } from "lucide-react";

import { useCauHinhXa } from "@/components/cau-hinh-xa";

/**
 * Cột thương hiệu của màn đăng nhập — spec giao diện 02/10/2026 §8.4: logo, khẩu hiệu ngắn, bốn ô
 * phân hệ, tên xã.
 *
 * KHÔNG CÓ TÊN SẢN PHẨM (ADR 0068 §13, chủ dự án chốt 02/10/2026): chỗ trước đây in `ViGov` nay in
 * tên xã, phụ đề là "Hệ thống điều hành số". Tên xã vì thế chỉ in MỘT lần, ở đầu cột; khối dưới
 * cùng chỉ còn cơ quan cấp trên (ẩn khi xã không khai, như `components/commune-identity.tsx`).
 *
 * Phụ đề, khẩu hiệu và bốn tên phân hệ là hằng số của SẢN PHẨM — viết thẳng được, giống nhau ở
 * mọi xã. Tên xã và cơ quan cấp trên thì KHÔNG: chúng đi xuống qua ngữ cảnh do máy chủ dựng, đọc
 * lúc chạy từ `Host`. Đây chính là chỗ một `NEXT_PUBLIC_TEN_XA` sẽ len vào nếu không ai để ý — và
 * một bundle không mang nổi tên của 300 xã, nên nó sẽ kéo theo mỗi xã một bản dựng riêng.
 *
 * Tên xã in NGUYÊN VĂN, không viết HOA, không ghép tiền tố: tên đúng của đơn vị hành chính là dữ
 * liệu do xã khai.
 *
 * Bốn ô phân hệ chỉ là hình minh hoạ, KHÔNG phải liên kết: người chưa đăng nhập không vào được
 * phân hệ nào, và một ô trông như bấm được mà không dẫn đi đâu là một lời hứa suông.
 */
const MODULES: readonly { label: string; Icon: LucideIcon; tone: string }[] = [
  { label: "Nhiệm vụ", Icon: ListChecks, tone: "tone-brand" },
  { label: "Văn bản & Đơn thư", Icon: Mail, tone: "tone-amber" },
  { label: "Ngân sách", Icon: Banknote, tone: "tone-green" },
  { label: "Phản ánh người dân", Icon: MessageSquareWarning, tone: "tone-red" },
];

export function KhoiThuongHieu() {
  const xa = useCauHinhXa();

  return (
    <section className="cot-thuong-hieu">
      <div className="login-brand-head">
        <div className="logo-vigov" aria-hidden="true">
          <Star focusable="false" />
        </div>
        <div>
          <p className="ten-san-pham">{xa.displayName}</p>
          <p className="phu-de-san-pham">Hệ thống điều hành số</p>
        </div>
      </div>

      <p className="login-slogan">
        Điều hành <span>thông suốt</span>, phục vụ người dân <span>kịp thời</span>
      </p>

      <ul className="login-modules" aria-label="Các phân hệ">
        {MODULES.map(({ label, Icon, tone }) => (
          <li key={label}>
            <span className={`login-module-icon ${tone}`} aria-hidden="true">
              <Icon focusable="false" strokeWidth={1.8} />
            </span>
            {label}
          </li>
        ))}
      </ul>

      {xa.parentAuthority !== "" && (
        <div className="login-commune">
          <span className="login-commune-icon" aria-hidden="true">
            <Landmark focusable="false" strokeWidth={1.8} />
          </span>
          <div>
            <p className="co-quan-cap-tren">{xa.parentAuthority}</p>
          </div>
        </div>
      )}
    </section>
  );
}
