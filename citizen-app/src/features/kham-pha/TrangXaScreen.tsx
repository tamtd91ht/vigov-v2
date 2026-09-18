/**
 * TRANG XÃ — thứ công dân thấy sau khi đã xác nhận xã.
 *
 * VÌ SAO MỖI XÃ MỘT NỘI DUNG, VÀ VÌ SAO ĐÓ KHÔNG PHẢI CHUYỆN TRANG TRÍ:
 *
 *   Một trang xã giống hệt trang xã bên cạnh thì tên xã trên header là bằng chứng DUY NHẤT rằng
 *   công dân đang ở đúng chỗ — và một bằng chứng duy nhất, đọc lướt, nằm ở góc trên cùng thì
 *   không ai đọc. Dịch vụ, giờ làm việc và số trực khác nhau là thứ làm cho "vào nhầm xã" nhìn
 *   ra được ngay, trước khi nó thành một hồ sơ gửi nhầm cơ quan (bất di dịch #2 và #5).
 *
 * ⚠ CÁC MỤC DỊCH VỤ CHƯA DẪN ĐI ĐÂU, VÀ MÀN NÀY NÓI RA ĐIỀU ĐÓ THAY VÌ GIẢ VỜ:
 *
 *   Phía sau chưa có màn nào, và cũng chưa có máy chủ nào. Dựng một màn giả để bấm vào cho "đủ
 *   luồng" là dạy công dân rằng kênh này nhận được việc — rồi không có việc nào được nhận. Nên
 *   mỗi mục mang chữ "Chưa mở" (bằng CHỮ, không bằng màu — README §Non-negotiables #6), và bấm
 *   vào thì nói rõ đang xây dựng kèm việc cần làm bây giờ: gọi số trực của xã (§Error message
 *   shape — nói việc cần làm, không nói mã lỗi).
 *
 * ⚠ SỐ TRỰC KHÔNG PHẢI MỘT LIÊN KẾT `tel:` — CỐ Ý.
 *
 *   Số trong danh mục này là số GIẢ của bản trình diễn (`demo-danh-muc-xa.ts`). Một liên kết
 *   bấm được là một liên kết sẽ có người bấm, và gọi vào một số giả dưới tên một cơ quan nhà
 *   nước thì người dân mất một cuộc gọi và mất lòng tin vào cả kênh. Khi số thật về từ cấu hình
 *   của xã, dòng này thành `tel:` — và lúc ấy nó đúng.
 */
import { type ComponentType, useState } from "react";

import {
  DEMO_GHI_CHU_TRANG_XA,
  DEMO_TEN_DICH_VU,
  type LoaiDichVu,
  type XaDemo,
} from "./demo-danh-muc-xa";

type Props = { xa: XaDemo };

/**
 * Dấu cách để ĐỌC, không phải để lưu. Kho giữ mười chữ số liền (một cách viết cho một số); mắt
 * người đọc một số điện thoại theo cụm, và đây là màn hình một người lớn tuổi đọc rồi bấm máy.
 */
function nhomSo(so: string): string {
  return so.replace(/^(\d{4})(\d{3})(\d{3})$/u, "$1 $2 $3");
}

/** Biểu tượng vẽ trong bundle, không phải tệp ảnh — xem `features/company-intro/icons.tsx`. */
const NET = {
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: "1.8",
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  "aria-hidden": true as const,
};

function GlyphPhanAnh({ className }: { className?: string }) {
  return (
    <svg className={className} {...NET}>
      <path d="M4 10v4h3l5,3V7l-5,3H4Z" />
      <path d="M16 9c1.2,1.2 1.2,4.8 0,6" />
    </svg>
  );
}

function GlyphHoSo({ className }: { className?: string }) {
  return (
    <svg className={className} {...NET}>
      <path d="M4 7h5l2,2h9v10H4V7Z" />
      <path d="M8 13h8" />
    </svg>
  );
}

function GlyphLich({ className }: { className?: string }) {
  return (
    <svg className={className} {...NET}>
      <path d="M4 6h16v14H4V6Z" />
      <path d="M4 10h16" />
      <path d="M9 4v4" />
      <path d="M15 4v4" />
    </svg>
  );
}

function GlyphThongBao({ className }: { className?: string }) {
  return (
    <svg className={className} {...NET}>
      <path d="M6 10a6,6 0 0,1 12,0c0,4 2,5 2,5H4s2,-1 2,-5Z" />
      <path d="M10 19a2,2 0 0,0 4,0" />
    </svg>
  );
}

function GlyphChungThuc({ className }: { className?: string }) {
  return (
    <svg className={className} {...NET}>
      <path d="M7 4h7l4,4v12H7V4Z" />
      <path d="M14 4v4h4" />
      <path d="M10 14l2,2 3,-4" />
    </svg>
  );
}

function GlyphDienThoai({ className }: { className?: string }) {
  return (
    <svg className={className} {...NET}>
      <path d="M6 4h3l2,5-2,1c1,2 2,3 4,4l1,-2 5,2v3c0,1 -1,2 -2,2C11,19 5,13 5,6c0,-1 1,-2 1,-2Z" />
    </svg>
  );
}

const GLYPH: Readonly<Record<LoaiDichVu, ComponentType<{ className?: string }>>> = {
  "phan-anh": GlyphPhanAnh,
  "ho-so": GlyphHoSo,
  "tiep-cong-dan": GlyphLich,
  "thong-bao": GlyphThongBao,
  "chung-thuc": GlyphChungThuc,
};

/**
 * Câu nói ra khi công dân bấm vào một mục chưa mở.
 *
 * Nó nói VIỆC CẦN LÀM BÂY GIỜ, không nói mã lỗi và không nói "coming soon" (README §Error message
 * shape). Một người dân bấm vào "Phản ánh hiện trường" là một người dân đang có việc cần báo;
 * câu trả lời đúng là chỉ họ tới chỗ nhận được việc ấy hôm nay, chứ không phải mời họ quay lại.
 */
export const LOI_NHAN_DANG_LAM =
  "Mục này đang được xây dựng, chưa mở trên ứng dụng. Bạn hãy gọi số điện thoại trực của xã ở trên để được hướng dẫn.";

/**
 * Một dòng dịch vụ, THUẦN — nhận cả trạng thái mở/đóng qua tham số.
 *
 * Tách ra vì cùng lý do `KhungApp` được tách khỏi `App` (xem App.tsx): bộ test ở đây dựng bằng
 * `react-dom/server` và không có DOM để bấm, nên trạng thái "đã bấm" chỉ kiểm được khi dựng
 * được nó thẳng. Một lời nhắn không kiểm được là một lời nhắn sẽ trôi thành "Coming soon".
 */
export function DichVuDong({
  loai,
  mo,
  onBam,
}: {
  loai: LoaiDichVu;
  mo: boolean;
  onBam: () => void;
}) {
  const Glyph = GLYPH[loai];
  return (
    <li>
      <button type="button" className="dich-vu__nut" aria-expanded={mo} onClick={onBam}>
        <span className="tile tile--soft" aria-hidden="true">
          <Glyph className="tile__glyph" />
        </span>
        <span className="dich-vu__chu">
          <strong className="dich-vu__ten">{DEMO_TEN_DICH_VU[loai]}</strong>
          {/* Trạng thái bằng CHỮ. Một chấm màu ở đây là một trạng thái người mù màu và người
              dùng trình đọc màn hình không đọc được (README §Non-negotiables #6). */}
          <span className="dich-vu__trang-thai">Chưa mở</span>
        </span>
      </button>

      {mo && (
        // `role="status"` để trình đọc màn hình đọc câu này ra khi nó hiện, thay vì để người
        // dùng không thấy gì xảy ra sau khi bấm.
        <p className="dich-vu__dang-lam" role="status">
          {LOI_NHAN_DANG_LAM}
        </p>
      )}
    </li>
  );
}

export function TrangXaScreen({ xa }: Props) {
  /**
   * Mục vừa bấm — trạng thái giao diện thuần, không lưu xuống máy, không gửi đi đâu. Một mục tại
   * một thời điểm: hai lời nhắn mở cùng lúc là hai thứ để đọc trên một màn hình đáng lẽ nói một
   * việc (`skills/accessibility-elderly` #4).
   */
  const [daBam, setDaBam] = useState<LoaiDichVu | null>(null);

  return (
    <section className="trang-xa" aria-labelledby="trang-xa-tieu-de">
      {/* Tên xã to, đứng đầu trang, ngoài header. Header cuộn khỏi tầm mắt trên màn hình nhỏ;
          câu trả lời cho "tôi đang ở xã nào" thì không được cuộn mất. */}
      <h1 className="trang-xa__ten" id="trang-xa-tieu-de">
        {xa.ten}
      </h1>
      <p className="trang-xa__tinh">{xa.tinh}</p>
      <p className="trang-xa__gioi-thieu">{xa.gioi_thieu}</p>

      <div className="trang-xa__truc">
        <span className="tile tile--soft" aria-hidden="true">
          <GlyphDienThoai className="tile__glyph" />
        </span>
        <span className="trang-xa__truc-chu">
          <span className="trang-xa__nhan">Số điện thoại trực</span>
          {/* Chữ số, không phải liên kết gọi — xem khối chú thích đầu tệp. */}
          <strong className="trang-xa__so">{nhomSo(xa.dien_thoai_truc)}</strong>
          <span className="trang-xa__gio">{xa.gio_lam_viec}</span>
        </span>
      </div>

      <h2 className="section-title">Dịch vụ của xã</h2>

      <ul className="dich-vu">
        {xa.dich_vu.map((loai) => (
          <DichVuDong
            key={loai}
            loai={loai}
            mo={daBam === loai}
            onBam={() => setDaBam(daBam === loai ? null : loai)}
          />
        ))}
      </ul>

      <p className="demo-ghi-chu">{DEMO_GHI_CHU_TRANG_XA}</p>
    </section>
  );
}
