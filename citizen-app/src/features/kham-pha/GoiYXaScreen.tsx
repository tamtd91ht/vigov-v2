/**
 * Màn XÁC NHẬN XÃ — thứ hiện ra khi đường liên kết đủ tin để chọn sẵn một xã (`src=qr`, `src=zns`).
 *
 * ĐÂY KHÔNG PHẢI "ĐÃ CHỌN XÃ", VÀ KHÁC BIỆT ẤY LÀ TOÀN BỘ LÝ DO MÀN NÀY TỒN TẠI.
 * Quy tắc đọc tham số nằm trong `goi-y.ts` cùng thư mục, kèm bảng ba lớp của ADR 0005.
 *
 * ⚠ XÁC NHẬN Ở ĐÂY **CHƯA PHẢI MỘT PHIÊN**, VÀ NGƯỜI ĐỌC SAU PHẢI BIẾT ĐIỀU ĐÓ:
 *
 *   Bấm nút dưới đây chỉ ghi xã vào **trạng thái giao diện phía client** (`useState` trong
 *   `App.tsx`). Không có lệnh gọi máy chủ, không có phiên nào được cấp, không có gì được lưu lại
 *   trên máy. Kênh công dân phía máy chủ chưa tồn tại: `service-identity` mới có phía cán bộ, và
 *   `ListTenants` của service `platform` còn chưa có cài đặt.
 *
 *   Khi tuyến ấy sống, câu lệnh đúng là: gửi mã xã + xác nhận của công dân lên máy chủ, máy chủ
 *   ghi xã vào PHIÊN và trả về phiên đó. Xã của phiên do máy chủ nói, không do màn hình này nhớ.
 *   Ai đó đọc tệp này mà tưởng phiên đã được cấp sẽ dựng tiếp tính năng lên trên một nền không
 *   có — đúng cái lỗi mà ba lớp của ADR 0005 dựng ra để chặn.
 */
import { nhanNguon } from "./goi-y";
import type { XaDemo } from "./demo-danh-muc-xa";

type Props = {
  /** Xã do đường liên kết gợi ý. Đã tra được tên — màn này không bao giờ hiện một mã trần. */
  xa: XaDemo;
  /** `qr` · `zns`. Chỉ dùng để nói ra bằng chữ vì sao xã này được chọn sẵn. */
  nguon: string;
  onXacNhan: () => void;
  onChonXaKhac: () => void;
};

/** Trụ sở uỷ ban. Trang trí — câu chữ bên cạnh mới là nội dung, nên `aria-hidden`. */
function GlyphTruSo({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      aria-hidden="true"
    >
      <path d="M3 10.5 12 4l9 6.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M5 10.5V20h14v-9.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M9.5 20v-5h5v5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function GoiYXaScreen({ xa, nguon, onXacNhan, onChonXaKhac }: Props) {
  return (
    <section className="goi-y" aria-labelledby="goi-y-tieu-de">
      <p className="goi-y__nhan">{nhanNguon(nguon)}</p>
      <h1 className="goi-y__tieu-de" id="goi-y-tieu-de">
        Bạn cần liên hệ với xã này?
      </h1>

      {/* Tên xã to và đứng một mình. Công dân xác nhận theo TÊN — mã ULID không đọc được, và một
          màn xác nhận hiện mã là một màn xác nhận không ai xác nhận được. */}
      <div className="xa-the">
        <span className="tile xa-the__dau" aria-hidden="true">
          <GlyphTruSo className="tile__glyph" />
        </span>
        <span className="xa-the__chu">
          <strong className="xa-the__ten">{xa.ten}</strong>
          <span className="xa-the__tinh">{xa.tinh}</span>
        </span>
      </div>

      <button type="button" className="goi-y__nut goi-y__nut--chinh" onClick={onXacNhan}>
        Đúng, tiếp tục
      </button>
      <button type="button" className="goi-y__nut" onClick={onChonXaKhac}>
        Chọn xã khác
      </button>

      {/* Lời hứa bất di dịch #2, nói ra trước khi công dân bấm: tên xã sẽ theo suốt mọi màn hình. */}
      <p className="goi-y__tiep">
        Tên xã sẽ hiện ở đầu mỗi màn hình cho tới khi bạn đổi sang xã khác.
      </p>
    </section>
  );
}
