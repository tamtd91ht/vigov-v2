/**
 * Màn XÁC NHẬN XÃ — thứ hiện ra khi đường liên kết đủ tin (`src=qr`, `src=zns`) VÀ máy chủ đã tra
 * ra tên xã. Quy tắc đọc tham số nằm trong `goi-y.ts` cùng thư mục, kèm bảng ba lớp của ADR 0005.
 *
 * ĐÂY KHÔNG PHẢI "ĐÃ CHỌN XÃ", VÀ KHÁC BIỆT ẤY LÀ TOÀN BỘ LÝ DO MÀN NÀY TỒN TẠI.
 *
 * ⚠ HAI ĐƯỜNG ĐI, KHÔNG CÓ ĐƯỜNG THỨ BA (ADR 0044 câu 4 · ADR 0047): xác nhận, hoặc "không phải xã
 * này" — về phần giới thiệu. Không có "Chọn xã khác": không còn danh mục xã nào để chọn, và một
 * phiên chỉ có một xã. Người cần xã khác quét QR của xã ấy.
 *
 * ⚠ XÁC NHẬN Ở ĐÂY **CHƯA PHẢI MỘT PHIÊN**. Bấm "Đúng, tiếp tục" chỉ báo lên `App.tsx`; xã của
 * phiên do MÁY CHỦ ghi sau hành vi xác nhận ấy (ADR 0005 · 0022), không do màn hình này nhớ.
 * Hôm nay chưa có nguồn máy chủ nào nối vào (`goi-y.ts`), nên màn này không bao giờ được vẽ ra.
 */
import { nhanNguon, type XaGoiY } from "./goi-y";

type Props = {
  /** Xã máy chủ tra ra. Màn này không bao giờ hiện một mã trần, và không bao giờ một tên đoán ra. */
  xa: XaGoiY;
  /** `qr` · `zns`. Chỉ dùng để nói ra bằng chữ vì sao xã này được chọn sẵn. */
  nguon: string;
  onXacNhan: () => void;
  /** "Không phải xã này" — về phần giới thiệu. Lối thoát bắt buộc: màn này giấu thanh tab. */
  onKhongPhai: () => void;
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

export function GoiYXaScreen({ xa, nguon, onXacNhan, onKhongPhai }: Props) {
  return (
    <section className="goi-y" aria-labelledby="goi-y-tieu-de">
      <p className="goi-y__nhan">{nhanNguon(nguon)}</p>
      <h1 className="goi-y__tieu-de" id="goi-y-tieu-de">
        Bạn cần liên hệ với xã này?
      </h1>

      {/* Tên xã to và đứng một mình. Công dân xác nhận theo TÊN — một màn xác nhận hiện mã là một
          màn xác nhận không ai xác nhận được. */}
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
      <button type="button" className="goi-y__nut" onClick={onKhongPhai}>
        Không phải xã này
      </button>

      {/* Lời hứa bất di dịch #2, nói ra trước khi công dân bấm: tên xã sẽ theo suốt mọi màn hình. */}
      <p className="goi-y__tiep">Tên xã sẽ hiện ở đầu mỗi màn hình.</p>
    </section>
  );
}
