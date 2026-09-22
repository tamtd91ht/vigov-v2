/**
 * HỢP ĐỒNG TUYẾN YÊU CẦU — MỘT TỆP, VÀ LÀ TỆP DUY NHẤT BIẾT `/api/v1/requests`.
 *
 *   POST <api-host>/api/v1/requests   Authorization: Bearer <phiếu phiên>
 *     gửi : { "kind": "consult" | "callback", "interests": [<mã>…], "scale": "<mã>",
 *             "source": "<mã chiến dịch>", "note": "<tối đa 2000 KÝ TỰ>" }
 *     nhận: 201 { "requestId", "kind", "status": "moi", "createdAt" }
 *
 *   GET  <api-host>/api/v1/requests   Authorization: Bearer <phiếu phiên>
 *     nhận: 200 { "items": [ { "requestId", "kind", "status", "createdAt" } ] }
 *
 *   lỗi : 400 sai khuôn hoặc ghi chú quá 2000 ký tự · 401 chưa đăng nhập / phiên hết hạn ·
 *         429 vượt trần (theo IP, HOẶC trần gọi lại 3 lượt/24 giờ) · 503 gọi lại chưa cấu hình
 *
 * ⚠ HAI HỢP ĐỒNG, HAI TỆP — VÀ CHÚNG KHÔNG ĐƯỢC GỘP.
 *
 *   `features/dang-nhap/hop-dong.ts` là tệp duy nhất biết `/api/v1/sessions`; tệp này là tệp duy
 *   nhất biết `/api/v1/requests`. Gộp lại thì một tệp biết hai bề mặt, và ngày một bề mặt đổi thì
 *   người sửa phải đọc cả hai để biết mình không làm gãy cái kia. Thứ DÙNG CHUNG chỉ có địa chỉ
 *   máy chủ, và nó ở `api/dia-chi.ts`.
 *
 * ⚠ THÂN YÊU CẦU KHÔNG CÓ, VÀ KHÔNG BAO GIỜ ĐƯỢC CÓ, MỘT TRƯỜNG NÓI "TÔI LÀ AI".
 *
 *   Máy chủ lấy người dùng TỪ PHIÊN (rule 4, bất biến 2: định danh đến từ phiên, không từ tham
 *   số). Bên kia dây có ca kiểm nhồi `nguoi_dung_id` / `userId` / `phone` vào thân để chứng minh
 *   chúng bị bỏ qua; bên này có `yeu-cau.test.ts` khẳng định `thanYeuCau` KHÔNG sinh ra chúng.
 *   Hai nửa của cùng một bất biến, mỗi nửa canh phần của mình.
 *
 * ⚠ MỌI MÃ GỬI ĐI PHẢI KHỚP `MA_HOP_LE`. Máy chủ trả 400 nếu lệch, và câu 400 ấy tới tay người
 *   dùng như một lỗi không ai hiểu. Nhãn tiếng Việt hiện trên màn là việc của client; thứ gửi đi
 *   là MÃ — ASCII thường, không dấu. `yeu-cau.test.ts` cho mọi mã CÓ THẬT trong ứng dụng
 *   (`SOLUTIONS`, quy mô, chiến dịch) ăn chính cái regex này.
 *
 * ⚠ `status` TRẢ VỀ LÀ MÃ, KHÔNG PHẢI CÂU CHỮ. Máy chủ cố ý không trả nhãn tiếng Việt, để đổi một
 *   nhãn không phải phát hành lại máy chủ. Ánh xạ mã → nhãn nằm ở ĐÚNG MỘT chỗ:
 *   `features/yeu-cau/trang-thai.ts`.
 *
 * ⚠ TÊN TRƯỜNG VIẾT `camelCase` TIẾNG ANH — khuôn của bên kia dây, giống `/api/v1/sessions`.
 *   Việt hoá nửa bề mặt cho "đồng bộ với mã nguồn" là tạo ra một bản dịch hai bên phải nhớ.
 */
import { diaChiApi } from "./dia-chi";

/**
 * Đường dẫn tuyến. Không chứa gì của người dùng (luật 3, cấm #4).
 *
 * Xuất ra để `bundle-for-zalo.test.ts` khẳng định nó có mặt ĐÚNG MỘT LẦN trong cả hai bản dựng —
 * không phải để tệp nào khác dùng. Phép kiểm ấy đọc hằng này thay vì gõ lại đường dẫn: gõ lại là
 * tạo bản sao thứ hai, và ngày hợp đồng đổi, bản sao ấy làm phép kiểm xanh vì KHÔNG TÌM THẤY GÌ.
 */
export const DUONG_DAN_YEU_CAU = "/api/v1/requests";

/** Địa chỉ đầy đủ. Rỗng khi chưa khai host — tầng gọi mạng fail-closed ở đó. */
export function diaChiYeuCau(): string {
  return diaChiApi(DUONG_DAN_YEU_CAU);
}

/**
 * Khuôn của MỌI mã ứng dụng gửi đi (`interests`, `scale`, `source`).
 *
 * CHÉP ĐÚNG `maNgan` của máy chủ (`internal/httpapi/yeu_cau.go`). Đây là một trong số rất ít chỗ
 * một bản sao được chấp nhận, và lý do là: bên kia dây KHÔNG công bố regex này qua một tuyến nào,
 * nên cách duy nhất để client biết trước là chép — và chép rồi thì phải có một ca kiểm cho nó ăn
 * MỌI mã có thật trong ứng dụng. Không có ca ấy thì bản sao này vô dụng: nó chỉ lặp lại một luật
 * mà không ai đối chiếu.
 */
export const MA_HOP_LE = /^[a-z0-9][a-z0-9_-]{0,63}$/;

/** Số dòng sản phẩm tối đa một yêu cầu mang theo. Máy chủ trả 400 nếu vượt. */
export const SO_QUAN_TAM_TOI_DA = 8;

/**
 * Trần ghi chú, ĐẾM THEO KÝ TỰ (rune), KHÔNG THEO BYTE.
 *
 * ⚠ ĐÂY LÀ CHỖ SAI IM LẶNG NHẤT CỦA CẢ TUYẾN NÀY. Máy chủ đếm bằng `utf8.RuneCountInString`; một
 * ký tự tiếng Việt có dấu tốn tới 3 byte. Đếm theo byte ở client thì ô ghi chú báo "còn 0 ký tự"
 * khi người dùng mới gõ chừng 700 chữ — một cái trần thấp hơn trần thật ba lần, mà không có gì
 * đỏ lên, vì máy chủ vẫn nhận. Người phát hiện ra là người đang gõ dở một câu hỏi.
 *
 * `demKyTu` dùng `Array.from` chứ không `.length`: `.length` đếm ĐƠN VỊ MÃ UTF-16, nên một ký tự
 * ngoài mặt phẳng cơ bản (emoji) bị tính là hai.
 */
export const TRAN_GHI_CHU = 2000;

export function demKyTu(chu: string): number {
  return Array.from(chu).length;
}

/** Số ký tự còn lại cho ô ghi chú. Âm nghĩa là đã vượt trần — màn hình chặn nút gửi. */
export function conLaiGhiChu(ghi_chu: string): number {
  return TRAN_GHI_CHU - demKyTu(ghi_chu.trim());
}

export type LoaiYeuCau = "consult" | "callback";

/** Mã trạng thái máy chủ trả về. Union đóng: một mã lạ không lọt qua `tsc` vào bảng nhãn. */
export type MaTrangThai = "moi" | "dang_xu_ly" | "da_lien_he" | "dong";

export const MA_TRANG_THAI: readonly MaTrangThai[] = ["moi", "dang_xu_ly", "da_lien_he", "dong"];

/** Hình dạng của TA cho một yêu cầu sắp gửi. Không phải hình dạng của dây. */
export type YeuCauMoi = {
  loai: LoaiYeuCau;
  /** Mã dòng sản phẩm, lấy từ `SOLUTIONS` id. Không dựng danh sách sản phẩm thứ hai. */
  quan_tam: readonly string[];
  /** Mã quy mô, hoặc rỗng nếu người dùng chưa chọn. */
  quy_mo: string;
  /** Chữ người dùng tự gõ. Có thể rỗng. */
  ghi_chu: string;
  /** Mã chiến dịch của đường mở app, hoặc rỗng. */
  nguon: string;
};

/** Một yêu cầu đã gửi, đọc từ danh sách. Hình dạng của ta. */
export type YeuCauDaGui = {
  ma: string;
  loai: LoaiYeuCau;
  trang_thai: MaTrangThai;
  tao_luc: string;
};

/**
 * TỪNG THỨ THÂN YÊU CẦU MANG ĐI — VÀ CÂU KHAI NÓ TRONG CHÍNH SÁCH QUYỀN RIÊNG TƯ.
 *
 * ⚠ BẢNG NÀY LÀ MỘT NỬA CỦA MỘT CÁI KHOÁ HAI CHIỀU, và nó là dây bẫy đắt nhất của lượt này.
 *
 *   Từ 22/09/2026 ứng dụng thu thập DỮ LIỆU BÁN HÀNG, không chỉ định danh. Chế độ hỏng ở đây im
 *   lặng theo đúng kiểu đã xảy ra hai lần trong kho này ("không đọc thư viện ảnh", "không có ô
 *   đăng nhập"): mã gửi thêm một trường, câu chữ trong chính sách ở lại như cũ, và không có gì
 *   đỏ lên. Khai thiếu một loại dữ liệu đang được thu thập là đúng thứ Nghị định 13/2023/NĐ-CP
 *   nhắm tới, và là thứ không sửa lại được sau khi công bố.
 *
 *   Nên hai vế khoá nhau, và `content/chinh-sach.test.ts` giữ cả hai:
 *     • MỌI khoá `thanYeuCau()` sinh ra phải có một dòng ở đây — thêm một trường mà quên khai là ĐỎ;
 *     • MỌI `trong_chinh_sach` ở đây phải có mặt nguyên văn trong văn bản chính sách — gỡ một mục
 *       khỏi chính sách trong khi mã vẫn gửi trường ấy cũng là ĐỎ.
 *
 *   Vế thứ nhất một mình không đủ: nó chỉ bắt được chiều "mã đi trước". Vế thứ hai bắt chiều còn
 *   lại, và chiều còn lại mới là chiều người ta dọn văn bản cho gọn.
 */
export type TruongGuiDi = {
  /** Khoá JSON đúng như nó lên dây. */
  khoa: string;
  /** Cụm từ in NGUYÊN VĂN vào câu khai của chính sách. Viết cho người dùng đọc. */
  trong_chinh_sach: string;
};

export const TRUONG_GUI_DI: readonly TruongGuiDi[] = [
  { khoa: "kind", trong_chinh_sach: "loại yêu cầu bạn chọn: tư vấn, hay đề nghị chúng tôi gọi lại" },
  { khoa: "interests", trong_chinh_sach: "những dòng giải pháp bạn đánh dấu là đang quan tâm" },
  { khoa: "scale", trong_chinh_sach: "quy mô nhân sự bạn chọn trong danh sách có sẵn" },
  {
    khoa: "note",
    trong_chinh_sach:
      "phần ghi chú bạn TỰ GÕ, nếu bạn có gõ — đây là ô duy nhất trong ứng dụng nhận chữ của bạn, và chúng tôi không kiểm soát được bạn viết gì vào đó",
  },
  {
    khoa: "source",
    trong_chinh_sach:
      "mã chiến dịch của đường liên kết hoặc mã QR bạn đã dùng để mở ứng dụng, nếu có",
  },
];

/**
 * CÂU KHAI TRONG CHÍNH SÁCH — DỰNG TỪ BẢNG, KHÔNG GÕ TAY.
 *
 * Cùng cơ chế với `cauKhaiDichRaNgoai` ở `content/dich-ra-ngoai.ts`, và cùng lý do: một danh sách
 * chép tay trong một văn bản pháp lý là một danh sách sẽ có lần không ai đối chiếu.
 */
export function cauKhaiTruongGuiDi(danh_sach: readonly TruongGuiDi[] = TRUONG_GUI_DI): string {
  const ke = danh_sach.map((t) => t.trong_chinh_sach);
  const cuoi = ke[ke.length - 1] ?? "";
  const ke_ra = ke.length <= 1 ? cuoi : `${ke.slice(0, -1).join("; ")}; và ${cuoi}`;
  return `Khi bạn bấm gửi, ứng dụng gửi đi đúng những thứ sau, và không gì khác: ${ke_ra}.`;
}

/**
 * Yêu cầu của ta → thân JSON đúng khuôn của máy chủ.
 *
 * ĐÂY LÀ CHỖ DUY NHẤT NĂM TÊN TRƯỜNG GỬI ĐI ĐƯỢC VIẾT RA. Màn hình chỉ chuyền `YeuCauMoi` — kiểu
 * của ta, đặt tên theo việc chứ không theo dây.
 *
 * ⚠ KHÔNG THÊM MỘT KHOÁ NÀO VÀO ĐÂY MÀ KHÔNG THÊM MỘT DÒNG VÀO `TRUONG_GUI_DI`. Đó không phải
 * một lời khuyên: `content/chinh-sach.test.ts` đọc thẳng các khoá của thân này ra và đối chiếu.
 */
export function thanYeuCau(yc: YeuCauMoi): string {
  return JSON.stringify({
    kind: yc.loai,
    interests: yc.quan_tam,
    scale: yc.quy_mo,
    note: yc.ghi_chu.trim(),
    source: yc.nguon,
  });
}

/**
 * Thân trả lời của POST → mã yêu cầu, hoặc `null` nếu không đúng khuôn.
 *
 * KIỂM TỪNG TRƯỜNG CHỨ KHÔNG ÉP KIỂU: một `as` sẽ cho `undefined` đi tiếp và hiện lên màn hình
 * thành chữ "undefined" ở đúng chỗ người dùng đang tìm mã yêu cầu của mình.
 */
export function docMaYeuCau(than: unknown): string | null {
  if (typeof than !== "object" || than === null) return null;
  const { requestId } = than as Record<string, unknown>;
  return typeof requestId === "string" && requestId !== "" ? requestId : null;
}

/** Chỉ bốn mã đã khai mới được coi là trạng thái. Mã lạ: xem `docDanhSach`. */
function laMaTrangThai(v: unknown): v is MaTrangThai {
  return typeof v === "string" && (MA_TRANG_THAI as readonly string[]).includes(v);
}

/**
 * Thân trả lời của GET → danh sách yêu cầu. Luôn trả một mảng, không bao giờ `null`.
 *
 * ⚠ MỘT HÀNG CÓ MÃ TRẠNG THÁI TA CHƯA BIẾT BỊ BỎ QUA, KHÔNG BỊ QUY VỀ `moi`.
 *
 *   Quy nó về `moi` là hiện một trạng thái SAI cho một yêu cầu có thật — người dùng đọc "Mới
 *   tiếp nhận" trong khi máy chủ đang nói điều khác. Bỏ qua thì họ thấy thiếu một dòng, và dòng
 *   thiếu là thứ họ hỏi lại được qua hotline; một trạng thái sai thì không ai hỏi, vì nó trông
 *   đúng. Cả hai đều dở, và cái thứ hai dở hơn.
 */
export function docDanhSach(than: unknown): readonly YeuCauDaGui[] {
  if (typeof than !== "object" || than === null) return [];
  const { items } = than as Record<string, unknown>;
  if (!Array.isArray(items)) return [];

  const ra: YeuCauDaGui[] = [];
  for (const mot of items) {
    if (typeof mot !== "object" || mot === null) continue;
    const { requestId, kind, status, createdAt } = mot as Record<string, unknown>;
    if (typeof requestId !== "string" || requestId === "") continue;
    if (kind !== "consult" && kind !== "callback") continue;
    if (!laMaTrangThai(status)) continue;
    ra.push({
      ma: requestId,
      loai: kind,
      trang_thai: status,
      tao_luc: typeof createdAt === "string" ? createdAt : "",
    });
  }
  return ra;
}

/**
 * Câu lỗi máy chủ trả về, hoặc `null`.
 *
 * ⚠ HIỆN NGUYÊN VĂN CÂU CỦA MÁY CHỦ, KHÔNG DỊCH LẠI. Máy chủ đã viết bằng tiếng Việt và đã nói
 * việc cần làm tiếp ("Vui lòng gọi hotline nếu cần gấp"). Viết lại ở client là dựng chỗ thứ hai
 * giữ cùng một câu — và ngày máy chủ đổi trần từ 3 lên 5 lượt, chỗ thứ hai vẫn nói "3".
 *
 * Chỉ khi máy chủ KHÔNG nói gì thì client mới có câu của riêng mình (`noi-dung.ts`).
 */
export function docCauLoi(than: unknown): string | null {
  if (typeof than !== "object" || than === null) return null;
  const { message } = than as Record<string, unknown>;
  return typeof message === "string" && message.trim() !== "" ? message : null;
}
