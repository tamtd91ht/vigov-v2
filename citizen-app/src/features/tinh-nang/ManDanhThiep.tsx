/**
 * BA MÀN QUYỀN — mỗi màn tự giải thích được cho người duyệt của Zalo.
 *
 * VÌ SAO MỖI MÀN PHẢI NÓI VÌ SAO NÓ CẦN QUYỀN ẤY:
 *
 *   Zalo chỉ cấp `getPhoneNumber` · `getLocation` · `scanQRCode` khi bản nộp CÓ chỗ dùng chúng
 *   nhìn thấy được, và chính sách Mini App (điều 3.3.4, trích ngay trên `getPhoneNumber` trong
 *   `zmp-sdk/index.d.ts`) nói thẳng: *"chúng tôi sẽ từ chối xét duyệt cho những Mini App có luồng
 *   xin cấp quyền chưa rõ ràng, không nêu được mục đích xin quyền đến người dùng"*. Một nút trần
 *   không kèm lời giải thích là một vòng duyệt trượt.
 *
 *   Và lời giải thích ấy không chỉ để qua vòng duyệt: người bấm "Đồng ý" mà không biết mình đồng
 *   ý cho việc gì là người sẽ gỡ app ngay lần đầu thấy lạ.
 *
 * ⚠ TỪ CHỐI LÀ ĐƯỜNG ĐI BÌNH THƯỜNG, KHÔNG PHẢI LỖI. Ba màn đều có nhánh riêng cho nó, và câu
 * hiện ra nói rõ vẫn dùng được ứng dụng và bấm lại lúc nào cũng được. Người dân có quyền nói
 * không với một cơ quan nhà nước; một màn hình báo đỏ ở đây là một màn hình mắng họ vì điều đó.
 *
 * ⚠ BA MÀN NÀY LẤY DỮ LIỆU RỒI HIỆN LÊN MÀN HÌNH, HẾT. Không `fetch`, không `localStorage`,
 * không `console.log` — các dây bẫy trong `phase1-collects-nothing.test.ts` giữ nguyên, và chúng
 * là thứ biến câu "không gửi đi đâu" thành một ràng buộc kiểm được thay vì một lời hứa.
 */
import { type ReactElement, type ReactNode, useState } from "react";

import {
  CHI_HIEN_LEN_MAN_HINH,
  DAN_NHAP_QUYEN,
  MA_RONG,
  type MaQuyen,
  noiDung,
  NOI_DUNG_QUYEN,
  QR_RONG,
  TOKEN_KHONG_CHUA_GI,
} from "./noi-dung";
import { type KetQuaXin, quetMaQR, xinTokenSoDienThoai, xinTokenViTri } from "./zalo-api";

/**
 * Che token, chỉ để vài ký tự đầu.
 *
 * Token KHÔNG chứa số điện thoại hay toạ độ (xem `zalo-api.ts`), nên đây không phải chuyện dữ
 * liệu cá nhân — nó là một CHỨNG TỪ đổi được dữ liệu ở máy chủ. Hiện trọn vẹn lên màn hình là
 * mời người đứng cạnh chụp lại trong hai phút nó còn sống. Vài ký tự đầu đủ để người duyệt thấy
 * "đã nhận được một chuỗi thật", và không đủ để dùng lại.
 */
export function cheToken(token: string): string {
  return token.length <= 6 ? "…" : `${token.slice(0, 6)}…`;
}

export type TrangThai = { kieu: "chua-goi" } | { kieu: "dang-cho" } | KetQuaXin<string>;

/**
 * Một màn quyền, THUẦN — nhận cả trạng thái qua tham số, không giữ gì.
 *
 * Tách ra vì cùng lý do `KhungApp` được tách khỏi `App` và `DichVuDong` khỏi `TrangXaScreen`: bộ
 * test ở đây dựng bằng `react-dom/server` và không có DOM để bấm. Bốn nhánh kết quả — nhất là
 * `tu-choi` và `ngoai-zalo`, hai nhánh người dùng gặp nhiều nhất — chỉ kiểm được khi dựng thẳng
 * được chúng. Một câu nói với người vừa từ chối mà không ai kiểm là một câu sẽ trôi thành "Lỗi".
 */
export function ManQuyenThuan({
  ma,
  trang_thai,
  onBam,
  veKetQua,
}: {
  ma: MaQuyen;
  trang_thai: TrangThai;
  onBam: () => void;
  veKetQua: (du_lieu: string) => ReactNode;
}) {
  const nd = noiDung(ma);
  const dang_cho = trang_thai.kieu === "dang-cho";

  return (
    <section className="quyen" aria-labelledby={`quyen-${ma}`}>
      <h1 className="quyen__tieu-de" id={`quyen-${ma}`}>
        {nd.tieu_de}
      </h1>
      <p className="quyen__vi-sao">{nd.vi_sao}</p>

      <button
        type="button"
        className="quyen__nut"
        onClick={onBam}
        disabled={dang_cho}
        aria-busy={dang_cho}
      >
        {/* Trạng thái "đang chờ" nói bằng CHỮ trên chính cái nút, không bằng riêng màu nền. */}
        {dang_cho ? nd.dang_cho : nd.nut}
      </button>

      {/* `role="status"` để trình đọc màn hình đọc kết quả ra khi nó hiện. Không có nó thì người
          khiếm thị bấm nút xong không biết có gì xảy ra hay không. */}
      <div className="quyen__ket-qua" role="status">
        {trang_thai.kieu === "xong" && veKetQua(trang_thai.du_lieu)}
        {trang_thai.kieu === "tu-choi" && <p className="quyen__loi">{nd.tu_choi}</p>}
        {trang_thai.kieu === "ngoai-zalo" && <p className="quyen__loi">{nd.ngoai_zalo}</p>}
        {trang_thai.kieu === "khong-lay-duoc" && <p className="quyen__loi">{nd.khong_lay_duoc}</p>}
      </div>

      <p className="quyen__loi-hua">{CHI_HIEN_LEN_MAN_HINH}</p>
    </section>
  );
}

/** Màn quyền có trạng thái: bấm → chờ → một trong bốn nhánh. Không giữ gì sau khi đóng app. */
export function ManMotQuyen({
  ma,
  xin,
  veKetQua,
}: {
  ma: MaQuyen;
  xin: () => Promise<KetQuaXin<string>>;
  veKetQua: (du_lieu: string) => ReactNode;
}) {
  const [trang_thai, datTrangThai] = useState<TrangThai>({ kieu: "chua-goi" });

  async function bam() {
    datTrangThai({ kieu: "dang-cho" });
    // `xin` không bao giờ ném — mọi đường đã quy về bốn nhánh trong `zalo-api.ts`. Không có
    // `catch` ở đây vì một `catch` thừa sẽ che mất lỗi lập trình thật của chính màn này.
    datTrangThai(await xin());
  }

  return (
    <ManQuyenThuan ma={ma} trang_thai={trang_thai} onBam={() => void bam()} veKetQua={veKetQua} />
  );
}

/** Kết quả của hai màn token: độ dài, vài ký tự đầu, và vì sao màn hình này không có gì để che. */
export function KetQuaToken({ ma, token }: { ma: "so-dien-thoai" | "vi-tri"; token: string }) {
  if (token === "") return <p className="quyen__loi">{MA_RONG}</p>;
  return (
    <>
      <p className="quyen__xong">Đã nhận được mã (token) từ Zalo.</p>
      <ul className="quyen__do">
        <li>Độ dài mã: {token.length} ký tự</li>
        <li>Vài ký tự đầu: {cheToken(token)}</li>
      </ul>
      <p className="quyen__giai-thich">{TOKEN_KHONG_CHUA_GI[ma]}</p>
    </>
  );
}

/**
 * Kết quả màn quét QR — API DUY NHẤT ở đây trả về dữ liệu THẬT, không phải token.
 *
 * Nội dung ấy có thể là bất cứ thứ gì, kể cả dữ liệu cá nhân của người khác. Hiện lên màn hình
 * là hết: không `console.log`, không lưu, không gửi (xem `zalo-api.ts`).
 */
export function KetQuaQR({ noi_dung_qr }: { noi_dung_qr: string }) {
  if (noi_dung_qr === "") return <p className="quyen__loi">{QR_RONG}</p>;
  return (
    <>
      <p className="quyen__xong">Nội dung quét được:</p>
      <p className="quyen__qr">{noi_dung_qr}</p>
    </>
  );
}

export function ManSoDienThoai() {
  return (
    <ManMotQuyen
      ma="so-dien-thoai"
      xin={xinTokenSoDienThoai}
      veKetQua={(token) => <KetQuaToken ma="so-dien-thoai" token={token} />}
    />
  );
}

export function ManViTri() {
  return (
    <ManMotQuyen
      ma="vi-tri"
      xin={xinTokenViTri}
      veKetQua={(token) => <KetQuaToken ma="vi-tri" token={token} />}
    />
  );
}

export function ManQuetQR() {
  return (
    <ManMotQuyen
      ma="quet-qr"
      xin={quetMaQR}
      veKetQua={(noi_dung_qr) => <KetQuaQR noi_dung_qr={noi_dung_qr} />}
    />
  );
}

const MAN: Readonly<Record<MaQuyen, () => ReactElement>> = {
  "so-dien-thoai": ManSoDienThoai,
  "vi-tri": ManViTri,
  "quet-qr": ManQuetQR,
};

/**
 * Khu vực ba màn quyền — một tab của app, và MỘT màn hiện tại một thời điểm.
 *
 * Ba màn xếp chồng trên một trang thì người duyệt phải cuộn qua ba khối giống nhau để tìm nút
 * mình cần, và mỗi màn mất đi thứ đắt nhất của nó: một việc, nói rõ một lý do
 * (`skills/accessibility-elderly` #4). Đổi màn thì màn cũ bị gỡ, nên kết quả cũ không nằm lại
 * dưới một tiêu đề mới.
 */
export function KhuQuyen() {
  const [dang_xem, datDangXem] = useState<MaQuyen>("so-dien-thoai");
  const ManDangXem = MAN[dang_xem];

  return (
    <div className="quyen-khu">
      <p className="quyen-khu__dan">{DAN_NHAP_QUYEN}</p>

      <nav className="quyen-khu__chon" aria-label="Chọn quyền để xem">
        {NOI_DUNG_QUYEN.map((mot) => (
          <button
            key={mot.ma}
            type="button"
            className="quyen-khu__nut"
            // Trạng thái "đang xem" nói bằng `aria-pressed` chứ không bằng riêng màu nền
            // (README §Non-negotiables #6).
            aria-pressed={mot.ma === dang_xem}
            onClick={() => datDangXem(mot.ma)}
          >
            {mot.nhan_chon}
          </button>
        ))}
      </nav>

      <ManDangXem />
    </div>
  );
}
