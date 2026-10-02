import { KeyRound, LockKeyhole } from "lucide-react";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { PageHeader } from "@/components/ui/page-header";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { FormDoiMatKhau } from "@/features/mat-khau/form-doi-mat-khau";
import { NhacBatDoiMatKhau } from "@/features/mat-khau/nhac-bat-doi";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/doi-mat-khau` — cán bộ tự đổi mật khẩu của chính mình (`15-phu-luc-giao-dien-chung` §1,
 * tuyến `PUT /api/v1/staff/current/password`).
 *
 * ĐÂY KHÔNG PHẢI `/quen-mat-khau` HAY `/dat-lai-mat-khau/:token` MÀ §1 VẼ SẴN, và khác biệt ấy
 * là một quyết định của khách chứ không phải một lần đặt tên khác đi. Câu mở #17, chốt
 * 22/09/2026: KHÔNG có tự phục vụ qua thư điện tử — quản trị viên xã đặt lại hộ. Lý do là một
 * vòng lặp chết: máy chủ thư là cấu hình THEO XÃ và có thể chưa khai, nên đúng lúc người ta cần
 * nó nhất — không đăng nhập được — thì nó là thứ duy nhất không dùng được. Dựng hai route ấy
 * theo đặc tả sẽ là dựng một luồng bỏ rơi đúng xã mới onboard.
 *
 * TRANG NÀY LÀ ĐƯỜNG CÓ BẢO VỆ, KHÔNG PHẢI ĐƯỜNG CÔNG KHAI: `src/proxy.ts` chặn ở phía máy chủ
 * trước khi nó được dựng — chưa có cookie phiên thì chuyển sang `/dang-nhap`. Người đang bị bắt
 * đổi mật khẩu VẪN có phiên hợp lệ (máy chủ trả 403, không phải 401), nên họ đi qua được.
 *
 * `PhienProvider` CÓ Ở ĐÂY, và nó gọi đúng một trong ba tuyến mà máy chủ còn cho phép khi cờ bắt
 * đổi đang bật (`GET /api/v1/sessions/current`). Nó phục vụ hai thứ: đầu trang hiện họ tên và
 * nút Đăng xuất — tuyến thứ ba được phép, và là lối ra cho người ngồi nhầm máy — còn
 * `NhacBatDoiMatKhau` đọc cờ để giải thích vì sao họ đang ở đây.
 *
 * `dynamic = "force-dynamic"`: trang phụ thuộc `Host`. Một trang tĩnh mang tên xã A phục vụ cho
 * host xã B là hình dạng rõ nhất của rò rỉ giữa hai cơ quan, và nó không làm test nào đỏ.
 */
export const dynamic = "force-dynamic";

export const metadata = {
  // Hằng số của sản phẩm, không mang tên xã: tên xã đọc lúc chạy từ `Host` và hiện trên đầu
  // trang (luật 1, bất biến 10).
  title: "Đổi mật khẩu · ViGov",
};

export default async function TrangDoiMatKhau() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <DauTrang withCommune />
        <main className="than-trang">
          {/* One task on one screen: a narrow, centred column inside the form-page width (spec v2
              §5 `.page--form`), the same shape at 320px as on a wide screen. */}
          <div className="page--form mx-auto w-full">
            <div className="mx-auto flex w-full max-w-[28rem] flex-col [&>.nhac-bat-doi]:mt-0">
              <PageHeader
                icon={KeyRound}
                title="Đổi mật khẩu"
                subtitle={
                  <span className="inline-flex items-center gap-1.5">
                    <LockKeyhole aria-hidden="true" focusable="false" strokeWidth={1.8} />
                    Mật khẩu của tài khoản đang đăng nhập trên trình duyệt này.
                  </span>
                }
              />
              <NhacBatDoiMatKhau />
              <FormDoiMatKhau />
            </div>
          </div>
        </main>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
