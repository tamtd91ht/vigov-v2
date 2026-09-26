import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { FormDangNhap } from "@/features/auth/form-dang-nhap";
import { layCauHinhXa } from "@/lib/tenant.server";

import { KhoiThuongHieu } from "./khoi-thuong-hieu";

/**
 * `/dang-nhap` — `15-phu-luc-giao-dien-chung` §1.
 *
 * Trang này là đường công khai duy nhất (`src/middleware.ts`), nhưng "công khai" không có nghĩa
 * là "không cần biết đang ở xã nào": nó vẫn suy ra xã từ `Host` ở máy chủ, và `Host` không khớp
 * xã nào thì 404 — trước khi vẽ bất cứ thứ gì. Một trang đăng nhập chịu phục vụ cho một host lạ
 * là một trang đăng nhập nhận mật khẩu cán bộ ở nơi không ai chịu trách nhiệm.
 *
 * `dynamic = "force-dynamic"`: trang phụ thuộc `Host`, nên không có bản dựng sẵn nào dùng lại
 * được. Một trang tĩnh mang tên xã A phục vụ cho host xã B là hình dạng rõ nhất của rò rỉ giữa
 * hai cơ quan — và nó sẽ không làm test nào đỏ.
 */
export const dynamic = "force-dynamic";

export default async function TrangDangNhap({
  searchParams,
}: {
  searchParams: Promise<{ [khoa: string]: string | string[] | undefined }>;
}) {
  const xa = await layCauHinhXa();
  const thamSo = await searchParams;

  const tho = thamSo["tiep-tuc"];
  const tiepTuc = typeof tho === "string" ? tho : null;

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <main className="trang-dang-nhap">
        <KhoiThuongHieu />
        <section className="cot-form">
          <FormDangNhap tiepTuc={tiepTuc} />
        </section>
      </main>
    </CauHinhXaProvider>
  );
}
