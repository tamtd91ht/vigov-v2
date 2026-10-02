import { ContactRound, ShieldCheck } from "lucide-react";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { PageHeader } from "@/components/ui/page-header";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { DanhBaLienHe } from "@/features/danh-ba/danh-ba-lien-he";
import { CAU_THIEU_QUYEN, MO_TA_TRANG, TIEU_DE_TRANG } from "@/features/danh-ba/nhan-danh-ba";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { QUYEN_QUAN_LY_NGUOI_DUNG } from "@/lib/quyen";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/danh-ba` — Danh bạ cán bộ (`docs/ui-ux/12-danh-ba-can-bo.md`), đúng đường dẫn đặc tả ghi ở
 * đầu chương.
 *
 * MỘT MÀN RIÊNG, KHÔNG PHẢI MỘT TAB THỨ SÁU CỦA `/cau-hinh` — và sự tách ấy là của đặc tả (§1):
 * tab `Cấu hình → Người dùng` quản lý **tài khoản đăng nhập** (vai trò, khoá, mật khẩu tạm), màn
 * này quản lý **thông tin liên hệ** (gọi ai, ở khối nào, số nào). Hai màn đọc chung một bảng
 * `nguoi_dung` và chung một tuyến `GET /api/v1/staff`; khác nhau ở bộ trường hiển thị và ở bộ
 * thao tác ghi mở ra.
 *
 * TIÊU ĐỀ THẺ KHÔNG MANG TÊN XÃ. Tên xã đọc lúc chạy từ `Host` và hiện trên đầu trang
 * (`DauTrang`); một tên xã trong hằng số là một xã trong bundle, và một bundle không mang nổi tên
 * của 300 xã (luật 1, bất biến 10).
 *
 * BẢO VỆ ĐƯỜNG: `src/proxy.ts` chặn ở phía máy chủ trước khi trang này được dựng — chưa có cookie
 * phiên thì chuyển sang `/dang-nhap`. Nó cố ý KHÔNG kiểm quyền: `admin.user` do dịch vụ identity
 * kiểm trên TỪNG lời gọi `/api/v1/staff`, còn `CongQuyen` dưới đây chỉ là lớp tiện dụng để cán bộ
 * khỏi bấm vào thứ chắc chắn trả 403 (luật 5, cấm #1).
 *
 * CÙNG KHOÁ `admin.user` VỚI TAB NGƯỜI DÙNG, KHÔNG PHẢI MỘT KHOÁ RIÊNG. Đặc tả §9.4 đề nghị
 * `content.read` / `content.update` cho màn này, nhưng hai tuyến màn này thật sự gọi —
 * `GET /api/v1/staff` và `PATCH /api/v1/staff/{id}` — đều khai `admin.user` ở máy chủ
 * (`x-vigov-permission` trong `kb/20-contracts/openapi.json`). Dựng cổng theo `content.read` sẽ ẩn
 * màn với đúng những tài khoản máy chủ đang phục vụ bình thường, và hiện màn cho những tài khoản
 * sẽ nhận 403 ở lời gọi đầu tiên. Hợp đồng thắng đặc tả (luật 2, bất biến 7).
 */
export const dynamic = "force-dynamic";

export const metadata = {
  title: "Danh bạ cán bộ · ViGov",
};

export default async function TrangDanhBa() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
        <ThanhBen />
        <DauTrang />
        <main className="than-trang">
          {/* `relative`: the screen lifts its count badge and "Công khai nhiều người" button into the
              right of this header row from `lg` up (`danh-ba-lien-he.tsx`); `lg:pr-*` keeps the
              title and subtitle clear of them. The `<h1>` stays here, outside the gate. */}
          <div className="relative">
            <PageHeader
              icon={ContactRound}
              title={TIEU_DE_TRANG}
              className="lg:pr-[25rem]"
              subtitle={
                <span className="flex items-start gap-1.5">
                  <ShieldCheck aria-hidden="true" focusable="false" className="mt-0.5 text-success-600" />
                  <span className="mo-ta-trang m-0 max-w-none text-[13px] text-ink-500">{MO_TA_TRANG}</span>
                </span>
              }
            />
            <CongQuyen khoa={QUYEN_QUAN_LY_NGUOI_DUNG} cauThieuQuyen={CAU_THIEU_QUYEN}>
              <DanhBaLienHe />
            </CongQuyen>
          </div>
        </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
