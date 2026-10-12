/**
 * HỢP ĐỒNG TUYẾN YÊU CẦU — MỘT TỆP, VÀ LÀ TỆP DUY NHẤT BIẾT `/api/v1/requests`.
 *
 *   POST <api-host>/api/v1/requests   Authorization: Bearer <phiếu phiên>
 *     gửi : { "kind": "consult" | "callback" | "chat" | "sms_promo" | "sms_optout",
 *             "interests": [<mã>…], "scale": "<mã>", "source": "<mã chiến dịch>",
 *             "note": "<tối đa 2000 KÝ TỰ>", "displayName"?: "<≤100 KÝ TỰ, CHỈ với kind chat>" }
 *     nhận: 201 { "requestId", "kind", "status": "moi", "createdAt" }
 *
 *   GET  <api-host>/api/v1/requests   Authorization: Bearer <phiếu phiên>
 *     nhận: 200 { "items": [ { "requestId", "kind", "status", "createdAt", "photoCount" } ] }
 *
 *   PHOTOS (owner 12/10/2026, `vihat-miniapp` README §"Ảnh đính kèm yêu cầu"), `consult` requests only:
 *   POST <api-host>/api/v1/requests/{requestId}/photos   multipart, EXACTLY one part "photo", one photo per call
 *     nhận: 201 { "photoId", "requestId", "position": 1..3, "contentType", "size", "createdAt" }
 *   GET  <api-host>/api/v1/requests/{requestId}/photos              → 200 { "items": [<as above>] }
 *   GET  <api-host>/api/v1/requests/{requestId}/photos/{photoId}    → 200 the image bytes
 *     lỗi : { "code", "message" } — `REQUEST_PHOTO_ERROR`; 401 and 429 by status only
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

/**
 * `chat` · `sms_promo` · `sms_optout` JOINED 07/10/2026 (owner decision): "Chat với chuyên viên" on Liên hệ and
 * "Nhận ưu đãi qua SMS" on the home screen. Each is one tap that records WHO asked (from the session) and
 * nothing more; the wire names are `vihat-miniapp`'s, fixed by its contract.
 */
export type LoaiYeuCau = "consult" | "callback" | "chat" | "sms_promo" | "sms_optout";

/**
 * Every kind the client knows. `docDanhSach` keeps only these: a row of a kind this build has no label for is
 * dropped rather than shown under a wrong label (the same reasoning as an unknown status, below).
 */
export const REQUEST_KINDS: readonly LoaiYeuCau[] = ["consult", "callback", "chat", "sms_promo", "sms_optout"];

/**
 * Server cap on `displayName`, in RUNES (`vihat-miniapp` counts with `utf8.RuneCountInString`, like the note).
 * A Zalo display name longer than this is cut here rather than sent whole: a 400 would cost the citizen the
 * whole chat request for a name they never typed.
 */
export const DISPLAY_NAME_MAX_RUNES = 100;

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
  /**
   * The Zalo display name, ONLY for `chat`, and only when the citizen allowed Zalo to share it. The server
   * answers 400 to this field on any other kind, so `thanYeuCau` drops it for them — a caller that sets it on
   * `sms_promo` changes nothing on the wire.
   */
  display_name?: string;
};

/** Một yêu cầu đã gửi, đọc từ danh sách. Hình dạng của ta. */
export type YeuCauDaGui = {
  ma: string;
  loai: LoaiYeuCau;
  trang_thai: MaTrangThai;
  tao_luc: string;
  /** How many photos the server holds for this request (`photoCount`); 0 when absent or unreadable. */
  photo_count: number;
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
  // 07/10/2026: the two new surfaces send the same body, with their own kind — so this sentence names them.
  {
    khoa: "kind",
    trong_chinh_sach:
      "loại yêu cầu bạn chọn: tư vấn, đề nghị chúng tôi gọi lại, trò chuyện với chuyên viên, đăng ký hay huỷ nhận ưu đãi qua SMS",
  },
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
  // 07/10/2026 — the ONLY personal value this body carries, and only on a chat request (see `thanYeuCau`).
  {
    khoa: "displayName",
    trong_chinh_sach:
      "tên hiển thị Zalo của bạn — chỉ khi bạn bấm 'Chat với chuyên viên' và đồng ý cho Zalo chia sẻ tên",
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
 * ĐÂY LÀ CHỖ DUY NHẤT CÁC TÊN TRƯỜNG GỬI ĐI ĐƯỢC VIẾT RA (năm, cộng `displayName` cho `chat`). Màn hình chỉ chuyền `YeuCauMoi` — kiểu
 * của ta, đặt tên theo việc chứ không theo dây.
 *
 * ⚠ KHÔNG THÊM MỘT KHOÁ NÀO VÀO ĐÂY MÀ KHÔNG THÊM MỘT DÒNG VÀO `TRUONG_GUI_DI`. Đó không phải
 * một lời khuyên: `content/chinh-sach.test.ts` đọc thẳng các khoá của thân này ra và đối chiếu.
 */
export function thanYeuCau(yc: YeuCauMoi): string {
  const base = {
    kind: yc.loai,
    interests: yc.quan_tam,
    scale: yc.quy_mo,
    note: yc.ghi_chu.trim(),
    source: yc.nguon,
  };
  // `displayName` ONLY on `chat`, and only when there is a name: the server refuses the field on every other
  // kind (400), and an empty one says nothing. Trimmed and cut to the server's rune cap here, not there.
  const name =
    yc.loai === "chat"
      ? Array.from((yc.display_name ?? "").trim())
          .slice(0, DISPLAY_NAME_MAX_RUNES)
          .join("")
          .trim()
      : "";
  return JSON.stringify(name === "" ? base : { ...base, displayName: name });
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

function isRequestKind(v: unknown): v is LoaiYeuCau {
  return typeof v === "string" && (REQUEST_KINDS as readonly string[]).includes(v);
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
    const { requestId, kind, status, createdAt, photoCount } = mot as Record<string, unknown>;
    if (typeof requestId !== "string" || requestId === "") continue;
    if (!isRequestKind(kind)) continue;
    if (!laMaTrangThai(status)) continue;
    ra.push({
      ma: requestId,
      loai: kind,
      trang_thai: status,
      tao_luc: typeof createdAt === "string" ? createdAt : "",
      // An older server sends no `photoCount`: 0, so the row says nothing about photos rather than a wrong number.
      photo_count: isPhotoCount(photoCount) ? photoCount : 0,
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

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * PHOTOS OF A CONSULTATION REQUEST (owner 12/10/2026) — `vihat-miniapp` d9592b71 + 2cf31a30
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * The server's caps, copied because no route publishes them (as `MA_HOP_LE` above): at most 3 photos per request
 * (CHECK + UNIQUE in its database), at most 5 MB of uploaded bytes each. Checked here only to say so BEFORE a 5 MB
 * upload crosses a rural line to meet a 413; the server stays the authority.
 */
export const REQUEST_PHOTO_MAX = 3;
export const REQUEST_PHOTO_MAX_BYTES = 5 * 1024 * 1024;

/** The ONE multipart part the server accepts — a second part, or another name, is 400 `invalid_upload`. */
export const REQUEST_PHOTO_PART = "photo";

/**
 * `<api-host>/api/v1/requests/{requestId}/photos`, or "" when the host is not declared (the caller fails closed).
 * Built FROM `diaChiYeuCau` so the route's path is written once — `bundle-for-zalo.test.ts` counts it. The id is
 * the server's own UUID; it is encoded anyway, so a malformed one can only miss, never reach another path.
 */
export function requestPhotosAddress(requestId: string): string {
  const base = diaChiYeuCau();
  return base === "" ? "" : `${base}/${encodeURIComponent(requestId)}/photos`;
}

/** `<api-host>/api/v1/requests/{requestId}/photos/{photoId}` — the image bytes, behind the same bearer. */
export function requestPhotoAddress(requestId: string, photoId: string): string {
  const list = requestPhotosAddress(requestId);
  return list === "" ? "" : `${list}/${encodeURIComponent(photoId)}`;
}

/**
 * The upload body: exactly one part, named `photo`. No file name of the phone's: the server ignores it, and a
 * camera file name can carry a date or a person's name (rule 3, forbidden #4) — so a fixed one is sent.
 */
export function requestPhotoForm(photo: Blob): FormData {
  const form = new FormData();
  form.append(REQUEST_PHOTO_PART, photo, REQUEST_PHOTO_PART);
  return form;
}

/** One stored photo, in our shape. */
export type RequestPhoto = {
  id: string;
  request_id: string;
  position: number;
  content_type: string;
  size: number;
  created_at: string;
};

/** A 201 body or one list item → our shape, or `null` when a field the screen relies on is missing. */
export function readRequestPhoto(body: unknown): RequestPhoto | null {
  if (typeof body !== "object" || body === null) return null;
  const { photoId, requestId, position, contentType, size, createdAt } = body as Record<string, unknown>;
  if (typeof photoId !== "string" || photoId === "") return null;
  if (typeof requestId !== "string" || requestId === "") return null;
  if (typeof position !== "number" || !Number.isInteger(position) || position < 1 || position > REQUEST_PHOTO_MAX) {
    return null;
  }
  return {
    id: photoId,
    request_id: requestId,
    position,
    content_type: typeof contentType === "string" ? contentType : "",
    size: typeof size === "number" && Number.isFinite(size) ? size : 0,
    created_at: typeof createdAt === "string" ? createdAt : "",
  };
}

/** The list body → its readable items, in the server's order. Always an array. */
export function readRequestPhotoList(body: unknown): readonly RequestPhoto[] {
  if (typeof body !== "object" || body === null) return [];
  const { items } = body as Record<string, unknown>;
  if (!Array.isArray(items)) return [];
  return items.map(readRequestPhoto).filter((p): p is RequestPhoto => p !== null);
}

function isPhotoCount(v: unknown): v is number {
  return typeof v === "number" && Number.isInteger(v) && v >= 0 && v <= REQUEST_PHOTO_MAX;
}

/** The stable `code`s of the photo routes (`vihat-miniapp` README). Renamed never, only added to. */
export const REQUEST_PHOTO_ERROR = {
  invalidUpload: "invalid_upload",
  notFound: "not_found",
  limit: "photo_limit",
  notAllowed: "photo_not_allowed",
  tooLarge: "file_too_large",
  unsupported: "unsupported_photo_type",
  unreadable: "photo_unreadable",
  internal: "internal_error",
  unavailable: "photo_unavailable",
} as const;

/**
 * Why a photo did not go — one sentence each in `features/yeu-cau/noi-dung.ts` (`CONSULT_PHOTOS.failures`),
 * keyed by WHAT THE PERSON DOES NEXT, not by the HTTP status.
 *
 *   `session` · `rate-limited`            401 · 429 (no `code` on these, by status)
 *   `network` · `not-configured`          nothing reached the server
 *   the rest                              the server's `code`
 */
export type RequestPhotoFailure =
  | "invalid-upload"
  | "not-found"
  | "limit"
  | "not-allowed"
  | "too-large"
  | "unsupported"
  | "unreadable"
  | "server"
  | "unavailable"
  | "session"
  | "rate-limited"
  | "network"
  | "not-configured";

/** The error body's `code`, or `null`. */
export function readErrorCode(body: unknown): string | null {
  if (typeof body !== "object" || body === null) return null;
  const { code } = body as Record<string, unknown>;
  return typeof code === "string" && code !== "" ? code : null;
}

/**
 * A refused answer → its failure. The `code` decides; without one (a proxy answering 413/415 itself, an empty
 * body) the status does. An UNKNOWN code is a server fault, never "fix your photo": telling someone to choose
 * another picture for a reason that is not theirs sends them in a circle.
 */
export function photoFailureFrom(status: number, code: string | null): RequestPhotoFailure {
  if (status === 401) return "session";
  if (status === 429) return "rate-limited";
  switch (code) {
    case REQUEST_PHOTO_ERROR.invalidUpload:
      return "invalid-upload";
    case REQUEST_PHOTO_ERROR.notFound:
      return "not-found";
    case REQUEST_PHOTO_ERROR.limit:
      return "limit";
    case REQUEST_PHOTO_ERROR.notAllowed:
      return "not-allowed";
    case REQUEST_PHOTO_ERROR.tooLarge:
      return "too-large";
    case REQUEST_PHOTO_ERROR.unsupported:
      return "unsupported";
    case REQUEST_PHOTO_ERROR.unreadable:
      return "unreadable";
    case REQUEST_PHOTO_ERROR.unavailable:
      return "unavailable";
    case REQUEST_PHOTO_ERROR.internal:
      return "server";
  }
  if (code !== null) return "server";
  switch (status) {
    case 400:
      return "invalid-upload";
    case 404:
      return "not-found";
    case 413:
      return "too-large";
    case 415:
      return "unsupported";
    case 422:
      return "unreadable";
    case 503:
      return "unavailable";
    default:
      return "server";
  }
}

/** Failures a second press can change. The rest meet the same answer again, so no "Tải lại" is offered. */
export const RETRYABLE_PHOTO_FAILURES: ReadonlySet<RequestPhotoFailure> = new Set<RequestPhotoFailure>([
  "server",
  "unavailable",
  "rate-limited",
  "network",
  "not-configured",
]);

/**
 * The image types the server accepts, read from the FIRST BYTES (its own rule: the name and the declared type
 * are not trusted). Used to say "choose another photo" while the person can still choose — a HEIC from an
 * iPhone would otherwise be refused only after the request was sent. The server re-checks; this never decides.
 */
export function sniffRequestPhotoType(head: Uint8Array): "image/jpeg" | "image/png" | "image/webp" | null {
  const at = (i: number) => head[i] ?? -1;
  if (at(0) === 0xff && at(1) === 0xd8 && at(2) === 0xff) return "image/jpeg";
  if (at(0) === 0x89 && at(1) === 0x50 && at(2) === 0x4e && at(3) === 0x47) return "image/png";
  const ascii = (from: number, to: number) => String.fromCharCode(...Array.from(head.slice(from, to)));
  if (head.length >= 12 && ascii(0, 4) === "RIFF" && ascii(8, 12) === "WEBP") return "image/webp";
  return null;
}
