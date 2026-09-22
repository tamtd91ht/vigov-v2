"use client";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { LOI_KHONG_RO } from "@/lib/api/goi";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

/**
 * Phần QUẢN TRỊ VIÊN của thông tin đăng nhập cán bộ — cấp tài khoản và đặt lại mật khẩu hộ,
 * `docs/ui-ux/14-cau-hinh.md §3`. Hai tuyến, một ô hiện mật khẩu tạm dùng chung.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * MẬT KHẨU TẠM SỐNG ĐÚNG MỘT LẦN, VÀ TỆP NÀY LÀ BỀ MẶT DUY NHẤT HIỆN NÓ. Máy chủ không dựng lại
 * được nó: CSDL chỉ giữ chuỗi băm argon2id, không tuyến GET nào trả nó, và `core/idem` CỐ Ý không
 * lưu thân câu trả lời nào — nên kể cả một lần phát lại theo khoá chống trùng cũng không mang nó
 * về (`core/idem/idem.go:421`, `service-identity/internal/http/routes.go:806`). Mất là mất.
 *
 * Ba điều dưới đây là ĐIỀU KIỆN để tệp này đúng, không phải lời khuyên:
 *
 *   1. KHÔNG `console.*` ở bất kỳ đâu chạm tới giá trị này. Console của trình duyệt đi thẳng vào
 *      ảnh chụp màn hình mà cán bộ gửi cho hỗ trợ (luật 3, cấm #1).
 *   2. KHÔNG `localStorage`, `sessionStorage`, không biến ở mức module. Giá trị chỉ đi qua đúng
 *      một prop, vào đúng một nút văn bản, và chết cùng lần render khi ô đóng.
 *   3. KHÔNG đưa vào URL, `title`, `aria-label` hay tên tệp (luật 3, cấm #4). Nhãn của ô giá trị
 *      là một `<dt>` đứng cạnh, không phải một thuộc tính mang chính giá trị ấy.
 *
 * HIỆN DẠNG CHỮ ĐỌC ĐƯỢC, KHÔNG `type="password"`, KHÔNG DẤU SAO. Che một giá trị mà mục đích duy
 * nhất của nó là được đọc to cho một người khác là vô nghĩa — và tệ hơn là có hại: quản trị viên
 * sẽ chép nó sang một chỗ khác để đọc cho dễ, và chỗ khác ấy không ai kiểm soát.
 *
 * Ô KHÔNG TỰ ĐÓNG, KHÔNG ĐẾM NGƯỢC. Một ô biến mất sau 30 giây là ô biến mất đúng lúc cán bộ lớn
 * tuổi ở đầu dây bên kia chưa nghe rõ và đang hỏi lại. Chỉ một hành động rõ ràng — bấm
 * `NUT_DA_GHI_LAI` — mới đóng nó.
 *
 * KHÔNG CÓ NÚT SAO CHÉP, và đó là một lựa chọn: `navigator.clipboard` không có ở môi trường kết
 * xuất phía máy chủ, nên một nút vẽ theo điều kiện ấy sẽ khác nhau giữa hai lần render; và mỗi bề
 * mặt thêm chạm vào giá trị này là một bề mặt nữa phải chứng minh nó không rò. Giá trị này sinh
 * ra để đọc to, không để dán.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/* ---- hai tuyến, một kiểu ------------------------------------------------------------------- */

/**
 * Hai việc, KHÔNG phải một việc có hai trạng thái.
 *
 * Máy chủ tách chúng ngay trong mệnh đề WHERE: `POST /staff/{id}/account` mang `AND NOT
 * co_tai_khoan`, `PUT /staff/{id}/password` chỉ có nghĩa khi tài khoản đã tồn tại. Không tuyến nào
 * làm được việc của tuyến kia, nên giao diện cũng không được gộp chúng vào một nút đổi nhãn.
 */
export type KieuCapTaiKhoan = "cap" | "datLai";

/**
 * Một lần mở biểu mẫu xác nhận.
 *
 * `khoaChongTrung` CHỈ CÓ Ở NHÁNH `datLai`, và sự bất đối xứng ấy đến từ máy chủ chứ không từ đây:
 * tuyến cấp tài khoản khai `idem.KhongCan` vì chính tài khoản là khoá tự nhiên (lần gửi thứ hai
 * trả 409, không cấp thêm), còn tuyến đặt lại khai `idem.Required` vì nó không có khoá nào để dựa
 * và mỗi lần gọi sinh một mật khẩu KHÁC (`routes.go:799` và `:837`).
 *
 * Khoá sinh ở lúc MỞ biểu mẫu và sống bằng đời một lần mở — xem `themCanBo` trong `lib/api/can-bo.ts`
 * cho tiền lệ, và `datLaiMatKhau` trong `lib/api/tai-khoan.ts` cho lý do ở đây đắt hơn.
 */
export type DangMoTaiKhoan =
  | { kieu: "cap"; canBo: identity_canBoTomTat }
  | { kieu: "datLai"; canBo: identity_canBoTomTat; khoaChongTrung: string };

/**
 * Giá trị đang hiện trên màn hình.
 *
 * MANG SẴN `hoTen` VÀ `maCanBo` CHỨ KHÔNG GIỮ CẢ DÒNG DANH BẠ: ô này chỉ cần gọi đúng tên người
 * vừa được cấp, và một bản chụp cả dòng nằm cạnh bảng đã đọc lại là hai câu trả lời cho cùng một
 * người. Hai trường này lấy từ DÒNG NGƯỜI DÙNG VỪA BẤM, không lấy từ thân câu trả lời — xem
 * `guiTaiKhoan` trong `danh-ba-can-bo.tsx`.
 */
export type MatKhauTamHienRa = {
  kieu: KieuCapTaiKhoan;
  maCanBo: string;
  hoTen: string;
  matKhau: string;
};

/* ---- nhãn các nút -------------------------------------------------------------------------- */

export const NUT_CAP_TAI_KHOAN = "Cấp tài khoản";
export const NUT_DAT_LAI_MAT_KHAU = "Đặt lại mật khẩu";
export const NUT_XAC_NHAN_CAP = "Xác nhận cấp tài khoản";
export const NUT_XAC_NHAN_DAT_LAI = "Xác nhận đặt lại mật khẩu";

/**
 * Nút đóng ô mật khẩu tạm. CHỮ NÀY NÓI RA MỘT HÀNH ĐỘNG ĐÃ LÀM, không phải "Đóng" hay "OK".
 *
 * "Đóng" mời người ta bấm theo phản xạ dọn màn hình. "Tôi đã ghi lại" là một lời khẳng định, nên
 * nó buộc dừng lại nửa giây — đúng nửa giây cần thiết trước khi một giá trị không lấy lại được
 * biến mất (`skills/accessibility-elderly`, REQUIRED #7).
 */
export const NUT_DA_GHI_LAI = "Tôi đã ghi lại";

/* ---- câu chữ ------------------------------------------------------------------------------- */

export function tieuDeXacNhan(dangMo: DangMoTaiKhoan): string {
  return dangMo.kieu === "cap"
    ? `${NUT_CAP_TAI_KHOAN}: ${dangMo.canBo.full_name}`
    : `${NUT_DAT_LAI_MAT_KHAU}: ${dangMo.canBo.full_name}`;
}

/**
 * Hậu quả nói ra TRƯỚC khi bấm, vì cả hai nhánh đều có một hậu quả không lùi lại được.
 *
 * Nhánh `datLai` phải nói ra nửa thứ hai — mật khẩu đang dùng hết hiệu lực ngay — vì đó là nửa
 * người bấm không nghĩ tới: họ đang định giúp một người quên mật khẩu, không định cắt đường đăng
 * nhập của một người đang làm việc bình thường.
 *
 * VÀ NÓ NÓI CẢ PHẦN THU HỒI PHIÊN, đã ĐỐI CHIẾU VỚI MÃ MÁY CHỦ chứ không suy ra từ tên tuyến:
 * `service-identity/internal/app/tai_khoan_can_bo.go:295-297` gọi `ThuHoiCuaCanBo` với lý do
 * `"quản trị viên đặt lại mật khẩu"` trong CÙNG giao dịch. Nên người bị đặt lại bị đăng xuất khỏi
 * mọi phiên đang mở, kể cả phiên họ đang gõ dở.
 *
 * Bỏ vế ấy đi thì câu cảnh báo vẫn đúng nhưng không đủ, và cái thiếu rơi đúng vào tình huống tệ
 * nhất: quản trị viên đặt lại hộ một người ĐANG NGỒI LÀM VIỆC, người ấy mất việc đang dở giữa
 * chừng, và không ai trong hai người biết vì sao.
 */
export function canhBaoTruocKhiGui(kieu: KieuCapTaiKhoan): string {
  return kieu === "cap"
    ? "Hệ thống sẽ sinh một mật khẩu tạm và hiện ra ĐÚNG MỘT LẦN ngay sau khi cấp. Hãy chuẩn bị " +
        "sẵn giấy bút, hoặc gọi được cho cán bộ, trước khi bấm."
    : "Mật khẩu cán bộ này đang dùng sẽ HẾT HIỆU LỰC ngay khi đặt lại, và MỌI PHIÊN LÀM VIỆC " +
        "ĐANG MỞ của họ bị đăng xuất — kể cả phiên họ đang dùng lúc này. Mật khẩu tạm mới hiện ra " +
        "ĐÚNG MỘT LẦN — hãy chuẩn bị sẵn giấy bút, hoặc gọi được cho cán bộ, trước khi bấm.";
}

/**
 * Câu "chỉ hiện một lần" — câu đắt nhất của cả màn hình này.
 *
 * NÓ PHẢI NÓI RA ĐƯỢC RẰNG BẤM ĐẶT LẠI SINH MỘT GIÁ TRỊ KHÁC. Không có vế ấy, quản trị viên đánh
 * mất giá trị sẽ đi tìm một nút "xem lại" không tồn tại, rồi bấm Đặt lại và đọc cho cán bộ đúng
 * cái mật khẩu vừa bị chính lần bấm ấy vô hiệu hoá.
 */
export const CAU_CHI_HIEN_MOT_LAN =
  "Mật khẩu tạm này CHỈ HIỆN MỘT LẦN. Hệ thống không lưu bản đọc được và không có cách nào hiện " +
  "lại. Nếu để mất, phải bấm Đặt lại mật khẩu — lần đặt lại sinh một mật khẩu KHÁC, và mật khẩu " +
  "đang hiện ở đây sẽ hết hiệu lực.";

/** Việc phải làm ngay, đặt cạnh giá trị. Bắt đổi ở lần đăng nhập đầu là ràng buộc của máy chủ. */
export const CAU_VIEC_CAN_LAM =
  "Đọc lại cho cán bộ, hoặc ghi ra giấy giao tận tay. Cán bộ phải đổi mật khẩu này ngay ở lần " +
  "đăng nhập đầu tiên.";

export function cauDaGhiXong(m: MatKhauTamHienRa): string {
  return m.kieu === "cap"
    ? `Đã cấp tài khoản cho ${m.hoTen} (${m.maCanBo}).`
    : `Đã đặt lại mật khẩu cho ${m.hoTen} (${m.maCanBo}).`;
}

/**
 * Máy chủ không trả lời được — và đây là ca TỆ NHẤT của cả luồng, nên câu chữ không được cụt lủn.
 *
 * `LOI_KHONG_RO` phủ cả hai đường: mạng đứt trước khi yêu cầu tới nơi, và mạng đứt SAU khi máy chủ
 * đã ghi (`docCapTaiKhoan` trong `lib/api/tai-khoan.ts`). Client không phân biệt được hai đường ấy
 * và cũng không được phép đoán. Một câu "Thất bại, vui lòng thử lại" ở đây là câu SAI ở đúng nửa
 * nguy hiểm: tài khoản đã cấp rồi, mật khẩu đã mất rồi, và quản trị viên đang ngồi chờ một thứ sẽ
 * không bao giờ hiện ra.
 *
 * Nên câu này chỉ làm một việc: mời KIỂM TRA LẠI, rồi ĐẶT LẠI nếu cần.
 */
export function cauKhongRoKetQua(kieu: KieuCapTaiKhoan): string {
  return kieu === "cap"
    ? "Không nhận được trả lời của máy chủ, nên chưa biết tài khoản đã được cấp hay chưa. Nếu máy " +
        "chủ đã cấp thì mật khẩu tạm của lần ấy đã mất và không lấy lại được. Hãy đóng ô này, xem " +
        "lại cột Tài khoản ở dòng của cán bộ: nếu đã có tài khoản thì bấm Đặt lại mật khẩu — lần " +
        "đặt lại sinh một mật khẩu KHÁC."
    : "Không nhận được trả lời của máy chủ, nên chưa biết mật khẩu đã đổi hay chưa. Nếu máy chủ đã " +
        "đổi thì mật khẩu tạm của lần ấy đã mất và không lấy lại được, và mật khẩu cũ cũng không " +
        "còn dùng được. Hãy đóng ô này rồi bấm Đặt lại mật khẩu một lần nữa — lần ấy sinh một mật " +
        "khẩu KHÁC và là giá trị duy nhất còn hiệu lực.";
}

/**
 * Máy chủ trả ĐÚNG MÃ MONG ĐỢI nhưng thân không có mật khẩu — một ca THẬT, không phải phòng xa.
 *
 * `PUT /api/v1/staff/{id}/password` khai `idem.Required`, và một lần gửi lại cùng khoá chống trùng
 * được `core/idem` trả lời bằng `{"code":…,"replayed":true}` kèm ĐÚNG mã 200 của lần đầu —
 * `core/idem/idem.go:421` ghi rõ: thân câu trả lời đầu tiên KHÔNG BAO GIỜ được lưu, nên không dựng
 * lại được và không được phép dựng lại. Chính tuyến ấy cũng nói ra hệ quả (`routes.go:806`).
 *
 * Với TypeScript thì thân ấy vẫn ép kiểu sạch sang `capTaiKhoanRa`, nên không có gì đỏ: trường
 * `temporary_password` chỉ đơn giản là `undefined`, và nếu không ai kiểm thì màn hình đọc to hai
 * chữ "undefined" cho một cán bộ đang cầm bút. Vì vậy chỗ gọi PHẢI kiểm hình dạng thật của thân
 * trước khi mở ô — xem `guiTaiKhoan` trong `danh-ba-can-bo.tsx`.
 */
export const CAU_PHAT_LAI_KHONG_CO_MAT_KHAU =
  "Máy chủ cho biết yêu cầu này đã được xử lý trước đó và không gửi kèm mật khẩu tạm — hệ thống cố " +
  "ý không lưu lại câu trả lời cũ, kể cả để gửi lần thứ hai, nên mật khẩu của lần ấy đã mất. Hãy " +
  "đóng ô này rồi bấm Đặt lại mật khẩu một lần nữa: lần ấy sinh một mật khẩu KHÁC.";

/* ---- biểu mẫu xác nhận ---------------------------------------------------------------------- */

/**
 * Bước xác nhận, BẮT BUỘC, không phải một lần bấm thẳng từ bảng.
 *
 * Ở 320px trên tay một cán bộ lớn tuổi, các nút của một dòng nằm sát nhau (`globals.css`,
 * `.bang-can-bo .o-thao-tac`). Một lần bấm trượt vào "Đặt lại mật khẩu" mà thi hành ngay là cắt
 * đường đăng nhập của một người đang làm việc, không ai kịp ngăn. Bước này còn làm một việc thứ
 * hai mà một hộp xác nhận thông thường không làm: nó nói ra TRƯỚC rằng mật khẩu chỉ hiện một lần,
 * đúng lúc quản trị viên còn kịp đi lấy giấy bút.
 *
 * THUẦN TRÌNH BÀY — mọi lời gọi mạng nằm ở chỗ gọi, để hai nhánh không ai nhìn thấy trong lúc phát
 * triển ("máy chủ vừa từ chối", "đang gửi") kết xuất được bằng `react-dom/server`.
 */
export function XacNhanTaiKhoan({
  dangMo,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  dangMo: DangMoTaiKhoan;
  loiMayChu: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const tieuDe = tieuDeXacNhan(dangMo);
  return (
    <form
      className="form-danh-muc"
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <h4>{tieuDe}</h4>

      <p className="ghi-chu">Mã cán bộ: {dangMo.canBo.code}</p>

      <p className="canh-bao-pham-vi">{canhBaoTruocKhiGui(dangMo.kieu)}</p>

      {/*
        LỖI CỦA MÁY CHỦ HIỆN NGUYÊN VĂN. 409 của tuyến cấp nghĩa là người này ĐÃ có tài khoản, và
        máy chủ viết sẵn câu ấy bằng tiếng Việt; #14 viết sẵn câu từ chối thao tác lên chính mình.
        Không rẽ nhánh theo `code`, không hiện `trace_id`, không hiện số hiệu HTTP — viết lại chúng
        ở đây là dựng bản sao thứ hai của một quy tắc nghiệp vụ (luật 9, cấm #2).
      */}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiMayChu}
        </p>
      )}

      {/*
        NGOẠI LỆ DUY NHẤT, VÀ NÓ KHÔNG PHẢI MỘT LẦN DIỄN GIẢI LỖI CỦA MÁY CHỦ: `LOI_KHONG_RO` là
        câu client tự đặt cho ca "không có câu trả lời nào để hiện" (`lib/api/goi.ts`), nên so với
        chính hằng ấy là cách duy nhất biết được rằng máy chủ đã im lặng. Im lặng ở đây có thể có
        nghĩa là đã ghi xong, nên câu bổ sung mời kiểm tra lại thay vì tuyên bố thất bại.
      */}
      {loiMayChu === LOI_KHONG_RO && (
        <p className="canh-bao-pham-vi">{cauKhongRoKetQua(dangMo.kieu)}</p>
      )}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={dangGui}>
          {dangMo.kieu === "cap" ? NUT_XAC_NHAN_CAP : NUT_XAC_NHAN_DAT_LAI}
        </button>
        <button type="button" className="nut-phu" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </button>
      </div>
    </form>
  );
}

/* ---- ô hiện mật khẩu tạm -------------------------------------------------------------------- */

/**
 * Ô hiện mật khẩu tạm — MỘT LẦN, đóng bằng đúng một hành động rõ ràng của người dùng.
 *
 * `role="status"` ĐẶT TRÊN CÂU XÁC NHẬN, KHÔNG TRÊN CẢ KHỐI, và đó là một quyết định chứ không
 * phải chỗ đặt ngẫu nhiên: một vùng sống bao cả khối sẽ khiến trình đọc màn hình ĐỌC TO mật khẩu
 * qua loa, ở một phòng một cửa có người dân đang ngồi chờ. Câu xác nhận nói ra rằng đã xong và ô
 * đã mở; giá trị thì nằm trong một `<dl>` có nhãn để người dùng tự điều hướng tới khi cần.
 *
 * GIÁ TRỊ ĐỂ FONT ĐẲNG CHIỀU (`.ma-muc`) VÌ NÓ ĐƯỢC ĐỌC QUA ĐIỆN THOẠI: `l` với `1`, `O` với `0`
 * phải phân biệt được, đúng lý do lớp ấy đã có sẵn cho mã danh mục.
 */
export function OMatKhauTam({
  matKhauTam,
  onDong,
}: {
  matKhauTam: MatKhauTamHienRa;
  onDong: () => void;
}) {
  return (
    <section className="khoi-chi-tiet" aria-labelledby="tieu-de-mat-khau-tam">
      <div className="dau-khoi-chi-tiet">
        <h3 id="tieu-de-mat-khau-tam">Mật khẩu tạm — chỉ hiện một lần</h3>
      </div>

      <p role="status">{cauDaGhiXong(matKhauTam)}</p>

      <p className="canh-bao-pham-vi">{CAU_CHI_HIEN_MOT_LAN}</p>

      {/* `<dt>` LÀ NHÃN, GIÁ TRỊ NẰM TRONG `<dd>`. Không `aria-label`, không `title`, không `input`
          — ba chỗ ấy đều là chỗ giá trị này bị mang đi (luật 3, cấm #4), và một `input` còn kéo
          theo `name` với khả năng bị trình duyệt lưu vào bộ nhớ điền tự động. */}
      <dl className="danh-sach-truong">
        <dt>Mật khẩu tạm</dt>
        <dd className="mat-khau-tam ma-muc">{matKhauTam.matKhau}</dd>
      </dl>

      <p className="ghi-chu">{CAU_VIEC_CAN_LAM}</p>

      <div className="cum-nut">
        <button type="button" className="nut-chinh" onClick={onDong}>
          {NUT_DA_GHI_LAI}
        </button>
      </div>
    </section>
  );
}
