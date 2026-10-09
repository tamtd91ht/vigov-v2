/**
 * Câu chữ của màn tra cứu phiếu phản ánh (`docs/ui-ux/09-phan-anh-nguoi-dan.md §8`). Hàm thuần:
 * không gọi mạng, không dựng DOM. Thời điểm hiện tại được TRUYỀN VÀO, không đọc từ đồng hồ ở
 * đây — một hàm tự đọc đồng hồ là một hàm không kiểm được ở đúng lúc quan trọng nhất: ngay trước
 * và ngay sau hạn.
 */

import type {
  identity_canBoChonNguoiRa,
  petitions_phieuPhanAnhRa,
} from "@/lib/api/schema.gen";
import { coQuyen, QUYEN_TAO_NHIEM_VU, QUYEN_XEM_PHAN_ANH } from "@/lib/quyen";

/**
 * Giờ Việt Nam, GHIM chứ không theo cài đặt của máy cán bộ.
 *
 * Một hạn xử lý là CAM KẾT của một cơ quan nhà nước với người dân. Máy đặt múi giờ khác sẽ hiện
 * cùng một mốc lệch đi vài giờ, và ở một hạn đếm bằng giờ làm việc thì vài giờ là cả buổi làm —
 * đủ để hai cán bộ đọc cùng một phiếu ra hai hạn khác nhau.
 */
const DINH_DANG_THOI_DIEM = new Intl.DateTimeFormat("vi-VN", {
  timeZone: "Asia/Ho_Chi_Minh",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

/** Mốc thời gian ISO-8601 của hợp đồng → `14:20 09/09/2026` theo giờ Việt Nam. */
export function nhanThoiDiem(iso: string): string {
  const moc = new Date(iso);
  // Chuỗi không đọc được là hợp đồng hỏng, không phải một trạng thái nghiệp vụ: hiện nguyên văn
  // chuỗi máy chủ gửi thay vì chữ "Invalid Date", để người báo lỗi có thứ để đọc qua điện thoại.
  if (Number.isNaN(moc.getTime())) return iso;
  return DINH_DANG_THOI_DIEM.format(moc);
}

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BẢNG NHÃN TRẠNG THÁI VÀ BẢNG NHÃN KÊNH LÀ HAI BẢN CHÉP TAY, VÀ ĐÓ LÀ MỘT LỖ HỔNG CỦA HỢP ĐỒNG
 * CHỨ KHÔNG PHẢI MỘT LỰA CHỌN Ở ĐÂY.
 *
 * `openapi.json` khai cả hai trường là `string` trơn, không kèm `enum` — dù bộ sinh kiểu CÓ dịch
 * `enum` chuỗi thành hợp của chuỗi hằng và đã làm đúng thế cho `sort` của `GET /api/v1/staff`
 * (`scripts/gen-api-types.mjs`). Danh sách đóng thì có thật và nằm trong mã Go
 * (`service-petitions/internal/domain/phieu_phan_anh.go:37` và `:115`).
 *
 * HỆ QUẢ: thêm một trạng thái ở máy chủ thì màn hình này KHÔNG đỏ ở `tsc` — nó chỉ lặng lẽ rơi
 * xuống nhánh dự phòng. Nên nhánh dự phòng được viết để NÓI RA điều đó chứ không để giấu đi: nó
 * hiện nguyên mã và nói rõ mã ấy chưa có nhãn. Đã báo lên để `tools/apidoc` phát `enum`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
const NHAN_TRANG_THAI: Readonly<Record<string, string>> = {
  "da-tiep-nhan": "Đã tiếp nhận",
  "dang-phan-loai": "Đang phân loại",
  "da-chuyen-xu-ly": "Đã chuyển xử lý",
  "dang-xu-ly": "Đang xử lý",
  "da-xu-ly": "Đã xử lý",
  "cho-dan-xac-nhan": "Chờ dân xác nhận",
  "da-dong": "Đã đóng",
  "khong-tiep-nhan": "Không tiếp nhận",
  "chuyen-cap-tren": "Chuyển cấp trên",
};

const NHAN_KENH: Readonly<Record<string, string>> = {
  "zalo-mini-app": "Zalo Mini App",
  "zalo-oa": "Zalo OA",
  // Prototype `feedback-display.ts:61` (`CHANNEL_LABEL.web`), verbatim.
  "web-xa": "Web của xã",
  "can-bo-nhap-ho": "Cán bộ nhập hộ",
};

function traNhan(bang: Readonly<Record<string, string>>, ma: string, loai: string): string {
  const nhan = bang[ma];
  if (nhan !== undefined) return nhan;
  if (ma === "") return `Không có ${loai}`;
  return `${ma} (mã ${loai} chưa có nhãn trên màn hình này)`;
}

export function nhanTrangThai(ma: string): string {
  return traNhan(NHAN_TRANG_THAI, ma, "trạng thái");
}

export function nhanKenh(ma: string): string {
  return traNhan(NHAN_KENH, ma, "kênh");
}

/**
 * Lĩnh vực phản ánh — BA ca, cùng hình dạng với cột Loại của tab Thôn/Tổ dân phố.
 *
 * `field_label` RỖNG LÀ CA THÔNG THƯỜNG, không phải lỗi: nhãn chỉ có khi xã đã đặt lại tên cho
 * mã ấy, còn mã gốc thuộc bộ danh mục tầng 1 do dịch vụ `platform` giữ và `petitions` chưa có
 * đường đọc (`service-petitions/internal/http/phieu_phan_anh.go`, chú thích trên `FieldLabel`).
 * `field` RỖNG lại là chuyện khác hẳn: phiếu CHƯA ĐƯỢC PHÂN LOẠI — và đó chính là lý do phiếu
 * ấy chưa có hạn xử lý xong.
 */
export type LinhVucPhanAnh =
  | { loai: "chuaPhanLoai" }
  | { loai: "coNhan"; nhan: string }
  | { loai: "chiCoMa"; ma: string };

export function linhVucPhanAnh(ma: string, nhan: string): LinhVucPhanAnh {
  if (ma === "") return { loai: "chuaPhanLoai" };
  if (nhan === "") return { loai: "chiCoMa", ma };
  return { loai: "coNhan", nhan };
}

export function nhanLinhVuc(l: LinhVucPhanAnh): string {
  switch (l.loai) {
    case "chuaPhanLoai":
      return "Chưa phân loại";
    case "coNhan":
      return l.nhan;
    case "chiCoMa":
      // Hiện MÃ, không kèm lời trách móc: xã chưa đặt lại tên cho mã là chuyện bình thường.
      return l.ma;
  }
}

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * NGƯỜI GỬI — MÀN HÌNH HIỆN ĐÚNG THỨ MÁY CHỦ GỬI, KHÔNG GHÉP LẠI VÀ KHÔNG "LÀM ĐẸP".
 *
 * `reporter_name` về dạng `Nguyễn V. A.` và `reporter_phone` về dạng `09****5678` — TRỪ MỘT CA:
 * `GET /api/v1/citizen-reports/{maTraCuu}` trả bản ĐẦY ĐỦ cho tài khoản có `feedback.unmask` (khoá
 * gieo 20/09/2026), và mỗi lần đọc như thế máy chủ ghi một dòng vết kiểm toán
 * (`service-petitions/internal/http/phieu_phan_anh.go:240-251`). Tuyến danh sách và bốn tuyến ghi
 * thì LUÔN che, kể cả với khoá ấy (`xu_ly_phan_anh.go:156`, `:469`). Hàm này không biết và không cần
 * biết mình đang cầm bản nào: nó hiện đúng thứ máy chủ gửi, và quyết định che hay không nằm ở máy chủ.
 *
 * ẨN DANH THÌ CẢ HAI TRƯỜNG RỖNG, và màn hình KHÔNG được bù vào bằng gì cả: một cái tên đã che
 * vẫn là một cái tên — `Nguyễn V. A.` trong một xã vài nghìn người vẫn chỉ ra một người, và
 * đúng điều cờ ẩn danh bảo vệ là cán bộ đang xử lý không biết ai gửi.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function nhanNguoiGui(phieu: petitions_phieuPhanAnhRa): string {
  if (phieu.anonymous) return "Người gửi ẩn danh";
  return senderPair(phieu);
}

/**
 * The sender on a LIST CARD — the prototype's words (`FeedbackCard.tsx`, spec 03): "Gửi ẩn danh" where
 * the drawer says "Người gửi ẩn danh". Same masked pair otherwise; nothing is put back together.
 */
export function cardSenderLabel(phieu: petitions_phieuPhanAnhRa): string {
  if (phieu.anonymous) return "Gửi ẩn danh";
  return senderPair(phieu);
}

/** Prototype's fallback (`FeedbackDetailDrawer.tsx:203`) when the server sent neither half. */
export const SENDER_UNKNOWN = "Không rõ người gửi";

function senderPair(phieu: petitions_phieuPhanAnhRa): string {
  const parts = [phieu.reporter_name, phieu.reporter_phone].filter((p) => p !== "");
  if (parts.length === 0) return SENDER_UNKNOWN;
  return parts.join(" · ");
}

/**
 * The account key that opens the `can-bo` field (seeded `service-identity/migrations/0001_init.sql:301`).
 * Read ONLY to decide whether the restricted-flow note is drawn — UX; the server filters the petitions.
 */
export const RESTRICTED_FEEDBACK_PERMISSION = "feedback.restricted";

/**
 * Prototype `FeedbackWorkspace.tsx:125-131`, FIRST SENTENCE ONLY (owner decision D3, 09/10/2026): the
 * key can be granted to others than the chairman, so "Chỉ Chủ tịch Uỷ ban đọc được." would be false.
 */
export const RESTRICTED_FLOW_NOTE = "Phản ánh về tác phong cán bộ đi luồng riêng và không hiển thị ở đây.";

/**
 * `?id=` of `/phan-anh` → the lookup code to open, or `null`. Only a single plain token is accepted
 * (letters, digits, `-`, `_`, ≤ 64): anything else is not a lookup code and is ignored, never sent.
 * The detail route still decides whether this account in this commune may read it.
 */
export function petitionDeepLinkCode(raw: string | readonly string[] | undefined): string | null {
  if (typeof raw !== "string") return null;
  const code = raw.trim();
  return /^[A-Za-z0-9_-]{1,64}$/.test(code) ? code : null;
}

const DATE_FORMAT = new Intl.DateTimeFormat("vi-VN", {
  timeZone: "Asia/Ho_Chi_Minh",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
});
const TIME_FORMAT = new Intl.DateTimeFormat("vi-VN", {
  timeZone: "Asia/Ho_Chi_Minh",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

/**
 * `09/09/2026 14:21` — the prototype's `formatDateTime` order (date first), for the drawer's header
 * line. Same pinned zone as `nhanThoiDiem`; an unreadable string is shown as sent.
 */
export function dateTimeLabel(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return iso;
  return `${DATE_FORMAT.format(at)} ${TIME_FORMAT.format(at)}`;
}

/**
 * SELF-DECLARED CONTACT (ADR 0080 decision 5). The petition came from a Mini App session Zalo gave no
 * phone number to: the name and phone above were typed by the sender, never verified, and no
 * notification reaches them (ADR 0080 decision 4) — so the officer must call if the case needs it.
 *
 * READ FROM THE SERVER'S EXPLICIT FIELD ONLY, never derived from `channel` + `has_citizen`: a staff
 * intake also has no citizen, and so will a future channel. A UI inference would be a second copy of
 * "does this petition carry an identity" that drifts the day a third channel exists (rule 9).
 * `=== true`: absent or `null` means the server did not say so, and the label is not drawn.
 */
export function isContactUnverified(phieu: petitions_phieuPhanAnhRa): boolean {
  return phieu.contact_unverified === true;
}

export const UNVERIFIED_CONTACT_LABEL = "Số tự khai — chưa xác thực";
export const UNVERIFIED_CONTACT_NOTE =
  "Người gửi tự nhập, chưa xác nhận qua Zalo; công dân không nhận thông báo — liên hệ qua số này nếu cần.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * SCENE LOCATION (§8.4) — requirement `FeedbackDetailDrawer.tsx:394-406`
 *
 * TEXT ONLY, NO MAP. The requirement draws a mini-map; drawing one means asking a tile service for
 * the tiles around the point, which sends a citizen's scene coordinates (personal data, rule 3) to an
 * outside party for the first time — a rule 3 stop condition the owner has not decided — and would
 * need a new CSP origin (rule 13). For the same reason there is no "open in maps" link: it would put
 * the coordinates into a third-party URL. The gap is listed in `PHAN_CHUA_DUNG`.
 *
 * SHOWN ON ANONYMOUS PETITIONS TOO, like `address`: the server returns both under `feedback.read`
 * regardless of the flag, because the officer cannot deal with a scene they cannot find. What the
 * flag hides is the reporter (`nhanNguoiGui`), not the place.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** The location row of the lookup view (`tra-cuu-phieu.tsx`). */
export const SCENE_LOCATION_LABEL = "Vị trí hiện trường";
/** The drawer's section title — prototype `FeedbackDetailDrawer.tsx:395`, owner decision D2. */
export const LOCATION_SECTION_TITLE = "Vị trí";
/** Requirement `FeedbackDetailDrawer.tsx:402`, verbatim. */
export const SCENE_NO_ADDRESS = "Không có địa chỉ ghi kèm";
export const SCENE_NO_COORDINATES = "Người dân không gửi toạ độ";
export const SCENE_COORDINATES_NOTE = "Toạ độ do người dân gửi kèm từ ứng dụng";

/**
 * `"21.028511, 105.804817"` — six decimals, the precision the server stores (~0.1 m), latitude first.
 *
 * `null` unless BOTH are finite numbers: half a coordinate is no location, and printing `NaN` or a
 * lone latitude would look like a place when there is none.
 */
export function sceneCoordinates(
  petition: Pick<petitions_phieuPhanAnhRa, "lat" | "lng">,
): string | null {
  const { lat, lng } = petition;
  if (typeof lat !== "number" || typeof lng !== "number") return null;
  if (!Number.isFinite(lat) || !Number.isFinite(lng)) return null;
  return `${lat.toFixed(6)}, ${lng.toFixed(6)}`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * SCENE PHOTOS (§8.4 `Ảnh trước và sau khi xử lý`) — owner, 02/10/2026 (ADR 0047, row "Ảnh hiện
 * trường khi gửi phản ánh"): staff with `feedback.read` see the photos the citizen attached.
 *
 * THE "AFTER" HALF (owner, 02/10/2026 — ADR 0047 row "Ảnh 'sau xử lý' của cán bộ — THAY G8"): staff
 * with `feedback.resolve` upload verification photos; anyone with `feedback.read` sees them. The empty
 * column does NOT print the spec's red "Bắt buộc phải có trước khi đóng phiếu": whether a photo is
 * required is the commune's `bat_buoc_anh_nghiem_thu` switch (ADR 0008 decision 3), which no route
 * lets this screen read — a commune with it off would be told something false. The obligation is said
 * where it is KNOWN: the closure's 409 `after_photo_required`, in the commune's own words.
 *
 * A PHOTOGRAPH IS PERSONAL DATA (rule 3) that cannot be masked. Its alt text is a position ("Ảnh hiện
 * trường 2/3"), never the reporter's name or anything else from the petition.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const SCENE_PHOTOS_TITLE = "Ảnh trước và sau khi xử lý";
export const SCENE_PHOTOS_BEFORE = "Trước khi xử lý";
export const SCENE_PHOTOS_AFTER = "Sau khi xử lý";
export const SCENE_PHOTOS_LOADING = "Đang tải ảnh người dân gửi kèm phiếu…";
/** Prototype `FeedbackDetailDrawer.tsx:354`, verbatim. */
export const SCENE_PHOTOS_EMPTY = "Người dân chưa gửi ảnh.";
/** 503 `storage_not_configured`. Says the rest of the petition still works — it does. */
export const SCENE_PHOTOS_UNAVAILABLE =
  "Kho ảnh tạm thời chưa sẵn sàng nên chưa xem được ảnh hiện trường. Các thông tin khác của phiếu vẫn dùng bình thường.";
export const SCENE_PHOTOS_RETRY = "Tải lại ảnh";
export const SCENE_PHOTOS_REFRESHING = "Đang lấy liên kết xem ảnh mới…";
export const SCENE_PHOTO_BROKEN =
  "Không hiện được ảnh này. Bấm “Tải lại ảnh” để lấy liên kết xem mới.";
export const SCENE_PHOTO_GONE = "Ảnh này không còn trong danh sách ảnh của phiếu.";
// Not "Ảnh trước" / "Ảnh sau": on this block those read as "before / after processing".
export const SCENE_PHOTO_PREVIOUS = "Xem ảnh liền trước";
export const SCENE_PHOTO_NEXT = "Xem ảnh tiếp theo";
export const SCENE_PHOTO_CLOSE = "Đóng ảnh";

/* ── "Sau khi xử lý" — verification photos ─────────────────────────────────────────────── */

export const AFTER_PHOTOS_LOADING = "Đang tải ảnh sau xử lý…";
export const AFTER_PHOTOS_EMPTY = "Chưa có ảnh sau xử lý.";
/** 503 `storage_not_configured` — same shape as the "before" column's sentence. */
export const AFTER_PHOTOS_UNAVAILABLE =
  "Kho ảnh tạm thời chưa sẵn sàng nên chưa xem được ảnh sau xử lý. Các thông tin khác của phiếu vẫn dùng bình thường.";
/** Spec §8.4 `⬆ Tải ảnh sau xử lý`; the arrow is a lucide icon beside the words (ADR 0068). */
export const AFTER_PHOTO_UPLOAD_BUTTON = "Tải ảnh sau xử lý";
export const AFTER_PHOTO_INPUT_LABEL = "Chọn ảnh sau xử lý (JPG, PNG, WebP)";
export const AFTER_PHOTO_NOTE =
  "Mỗi ảnh được quét mã độc và lưu lại không kèm thông tin của máy chụp. Người dân gửi phiếu từ ứng " +
  "dụng xem được ảnh này trong phiếu của mình. Số ảnh tối đa do hệ thống quy định.";
/** The pre-check's list — the server's (`petition_staff_file.go`: JPEG, PNG, WebP). */
export const AFTER_PHOTO_TYPES: readonly string[] = ["image/jpeg", "image/png", "image/webp"];
export const AFTER_PHOTO_ACCEPT = "image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp";
export const AFTER_PHOTO_TYPE_REFUSED = "Chỉ tải được ảnh JPG, PNG hoặc WebP.";
export const AFTER_PHOTO_EMPTY_REFUSED = "Tệp rỗng — không có ảnh nào để tải lên.";
/** Shown on the three statuses where the server refuses an upload (409 `petition_state`). */
export const AFTER_PHOTO_CLOSED = "Phiếu đã kết thúc nên không thêm ảnh sau xử lý được nữa.";
export const AFTER_PHOTO_UPLOAD_DENIED =
  "Tài khoản của bạn không có quyền kết thúc xử lý phản ánh (feedback.resolve), nên không tải được " +
  "ảnh sau xử lý.";

/**
 * The statuses where the server REFUSES a verification photo (`domain.VerificationPhotoUploadOpen`:
 * every status but the three endings). A DENY list, mirroring the server's own: a status code the
 * screen does not know keeps the button, and the server's 409 sentence is the answer.
 */
const AFTER_PHOTO_CLOSED_STATUSES: readonly string[] = ["da-dong", "khong-tiep-nhan", "chuyen-cap-tren"];

export function afterPhotoUploadOpen(status: string): boolean {
  return !AFTER_PHOTO_CLOSED_STATUSES.includes(status);
}

/** `Ảnh sau xử lý 2/3` — alt text and button label; never anything from the petition. */
export function afterPhotoAlt(index: number, total: number): string {
  return `Ảnh sau xử lý ${index + 1}/${total}`;
}

/** The type to DECLARE, or a refusal. The server sniffs the bytes and refuses a mismatch. */
export function afterPhotoType(file: { readonly name: string; readonly type: string; readonly size: number }):
  | { readonly ok: true; readonly contentType: string }
  | { readonly ok: false; readonly message: string } {
  if (file.size <= 0) return { ok: false, message: AFTER_PHOTO_EMPTY_REFUSED };
  if (file.type !== "") {
    return AFTER_PHOTO_TYPES.includes(file.type)
      ? { ok: true, contentType: file.type }
      : { ok: false, message: AFTER_PHOTO_TYPE_REFUSED };
  }
  const ext = file.name.toLowerCase().split(".").pop() ?? "";
  const byExt: Readonly<Record<string, string>> = {
    jpg: "image/jpeg",
    jpeg: "image/jpeg",
    png: "image/png",
    webp: "image/webp",
  };
  const t = byExt[ext];
  return t === undefined ? { ok: false, message: AFTER_PHOTO_TYPE_REFUSED } : { ok: true, contentType: t };
}

/**
 * The close block's line under the commune's refusal (409 `after_photo_required`): WHERE to go next.
 * The refusal itself is the server's sentence, shown above this one, verbatim.
 */
export const AFTER_PHOTO_REQUIRED_HINT =
  "Tải ảnh ở mục “Sau khi xử lý” trong khối Ảnh trước và sau khi xử lý, rồi bấm Đóng phiếu lại.";
export const AFTER_PHOTO_GO_TO = "Đến mục Sau khi xử lý";

/** `Ảnh hiện trường 2/3` — 1-based position. Alt text, dialog title and button label all use it. */
export function scenePhotoAlt(index: number, total: number): string {
  return `Ảnh hiện trường ${index + 1}/${total}`;
}

/** Screen-reader label of a thumbnail button. */
export function scenePhotoOpenLabel(index: number, total: number): string {
  return `Xem cỡ lớn ${scenePhotoAlt(index, total).toLowerCase()}`;
}

/**
 * How long before `url_expires_at` a link already counts as expired. An image request takes time to
 * reach the store; a link opened 5 s before its end can arrive after it and be refused.
 */
export const PHOTO_LINK_MARGIN_MS = 30_000;

/**
 * Is this signed link still safe to put in an `<img src>` now?
 *
 * FAIL CLOSED: an `url_expires_at` that does not parse counts as expired, so the screen asks for a fresh
 * list instead of using a link of unknown age (the server's contract says ≤ 15 minutes; the screen does
 * not trust a link past what it was told).
 */
export function photoLinkUsable(
  photo: { url_expires_at: string },
  now: Date,
  marginMs: number = PHOTO_LINK_MARGIN_MS,
): boolean {
  const expires = new Date(photo.url_expires_at).getTime();
  if (Number.isNaN(expires)) return false;
  return expires - now.getTime() > marginMs;
}

/**
 * THE ONLY `src` A SCENE PHOTO GETS: the server's signed `url`, and only when it parses as an absolute
 * http(s) URL — same rule as `coverPreviewSrc` (`features/noi-dung/cover-image.ts`).
 */
export function scenePhotoSrc(photo: { url: string }): string | null {
  if (photo.url === "") return null;
  let u: URL;
  try {
    u = new URL(photo.url);
  } catch {
    return null;
  }
  return u.protocol === "https:" || u.protocol === "http:" ? u.href : null;
}

/**
 * Trạng thái của MỘT hạn xử lý.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * QUÁ HẠN LÀ SUY RA, KHÔNG PHẢI MỘT TRƯỜNG ĐƯỢC GỬI VỀ (luật 10, bất biến 3) — và hợp đồng CỐ Ý
 * không có trường `overdue` vì đúng lý do ấy: một giá trị boolean đóng băng lúc dựng phản hồi
 * còn màn hình thì đứng trên máy cán bộ hàng giờ sau đó.
 *
 * NHƯNG Ở ĐÂY CHỈ SUY RA "ĐÃ QUA MỐC HAY CHƯA", KHÔNG SUY RA "QUÁ HẠN MẤY NGÀY". So hai mốc thời
 * gian tuyệt đối là một phép so sánh và nó đúng ở mọi lịch. Còn "quá hạn 3 ngày" mà đặc tả §8.3
 * vẽ là một khoảng đếm bằng GIỜ LÀM VIỆC — nó cần lịch làm việc, ngày nghỉ lễ và ngày làm bù của
 * chính xã ấy, ba bảng do `identity` sở hữu cùng với phép cộng giờ làm việc (ADR 0007). Đếm bằng
 * giờ đồng hồ ở trình duyệt sẽ ra một con số khác con số của máy chủ vào đúng dịp lễ, và con số
 * hiện trên màn hình cán bộ là con số được báo cáo lên trên.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export type TrangThaiHan =
  /** `acknowledge_due = null`: phiếu do cán bộ nhập hộ — cán bộ CHÍNH LÀ người đọc, nên khoảng ấy không tồn tại. */
  | { loai: "khongApDung" }
  /** `resolve_due = null`: phiếu chưa được phân loại nên chưa có cam kết nào được ấn định. */
  | { loai: "chuaCo" }
  | { loai: "conHan"; moc: string }
  | { loai: "quaHan"; moc: string };

/**
 * `hanISO` là `null` với HAI nghĩa khác nhau tuỳ trường, nên nghĩa ấy được TRUYỀN VÀO chứ không
 * đoán ở đây: `khongApDung` cho hạn tiếp nhận, `chuaCo` cho hạn xử lý xong. Gộp hai cái làm một
 * là để màn hình nói "chưa có hạn" về một phiếu mà khoảng ấy không bao giờ tồn tại.
 */
export function trangThaiHan(
  hanISO: string | null,
  khiRong: "khongApDung" | "chuaCo",
  bayGio: Date,
): TrangThaiHan {
  if (hanISO === null) return { loai: khiRong };

  const moc = new Date(hanISO);
  if (Number.isNaN(moc.getTime())) return { loai: "conHan", moc: hanISO };

  return moc.getTime() < bayGio.getTime()
    ? { loai: "quaHan", moc: nhanThoiDiem(hanISO) }
    : { loai: "conHan", moc: nhanThoiDiem(hanISO) };
}

export function nhanHan(h: TrangThaiHan): string {
  switch (h.loai) {
    case "khongApDung":
      // KHÔNG hiện thành `0` và không hiện ô trống: một số không là một mẫu hợp lệ, và một xã
      // nhập hộ nhiều phiếu sẽ báo cáo thời gian tiếp nhận trung bình gần bằng không (ADR 0028).
      return "Không áp dụng";
    case "chuaCo":
      return "Chưa ấn định — phiếu chưa được phân loại";
    case "conHan":
      return `Hạn cuối ${h.moc}`;
    case "quaHan":
      return `Quá hạn · hạn cuối ${h.moc}`;
  }
}

export function lopHan(h: TrangThaiHan): string | undefined {
  return h.loai === "quaHan" ? "nhan-lech" : undefined;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * "HIỂN THỊ VỚI NGƯỜI DÂN" (§8.3, §14.4) — publication_status, ADR 0050 point 8
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** The field of staff-conduct petitions. Never public — the server refuses with 409 `never_public`. */
export const STAFF_CONDUCT_FIELD = "can-bo";

/**
 * The three stored values, and what the box says for each. Labels and hints are the requirement's
 * wording (`vigov-require/apps/admin/src/lib/feedback-display.ts:75-94`, spec §8.3), not ours.
 */
const PUBLICATION_VIEW: Readonly<Record<string, { label: string; hint: string }>> = {
  "cho-duyet": {
    label: "Chưa cho hiện công khai",
    hint: "Chỉ cán bộ trong xã xem được. Người gửi vẫn tra cứu được phiếu của mình.",
  },
  "cong-khai": {
    label: "Đang hiện công khai",
    hint: "Mọi người dân đều xem được phiếu này trên Mini App của xã.",
  },
  an: {
    label: "Không cho hiện công khai",
    hint: "Đã chặn hiện ra ngoài. Chỉ cán bộ trong xã xem được.",
  },
};

/**
 * The box's hint on a staff-conduct petition — a UI hint shown BEFORE any action, explaining why no
 * publish button is offered. It is NOT the refusal: the refusal is the 409 `never_public` sentence,
 * which is a system message the commune may reword (`feedback.never_public`, Lời hệ thống tab) and
 * which the drawer shows verbatim from the server.
 *
 * Worded as the software's DEFAULT of that message (`service-petitions/internal/domain/
 * system_message.go`) so the two read alike on a commune that has not reworded it. A commune that
 * has sees its own sentence on refusal and this hint in the box — the hint describes the rule, the
 * refusal is the commune's voice.
 */
export const NEVER_PUBLIC_HINT =
  "Phản ánh về thái độ, tác phong cán bộ không được hiển thị công khai.";

export type PublicationView = {
  /** The stored value, or the one derived from `public` when the server predates the field. */
  readonly status: string;
  readonly label: string;
  readonly hint: string;
  /** Whether `Cho hiện công khai` has any meaning on this petition. */
  readonly canPublish: boolean;
  /** Whether `Ẩn khỏi trang công khai` has any meaning on this petition. */
  readonly canHide: boolean;
};

/**
 * The state of the `Hiển thị với người dân` box.
 *
 * `publication_status` ABSENT means the server predates the field (it is set on every response now),
 * so the state is derived from `public`: `true` is `cong-khai`; `false` cannot tell "waiting" from
 * "hidden", and `cho-duyet`'s label — "Chưa cho hiện công khai" — is the one sentence true of both.
 *
 * AN UNKNOWN VALUE shows the raw code and says it has no label, and offers NO button: guessing what an
 * unknown state means is guessing what a click on it would do.
 *
 * The buttons follow the requirement drawer (`FeedbackDetailDrawer.tsx:270,292`): publish unless already
 * public, hide unless already hidden. A `can-bo` petition never gets the publish button — the server
 * would refuse it with 409 — but keeps the hide button when it is somehow not hidden.
 */
export function publicationView(p: petitions_phieuPhanAnhRa): PublicationView {
  const status =
    p.publication_status !== undefined && p.publication_status !== ""
      ? p.publication_status
      : p.public
        ? "cong-khai"
        : "cho-duyet";
  const view = PUBLICATION_VIEW[status];
  if (view === undefined) {
    return {
      status,
      label: traNhan({}, status, "trạng thái công khai"),
      hint: "",
      canPublish: false,
      canHide: false,
    };
  }
  const staffConduct = p.field === STAFF_CONDUCT_FIELD;
  return {
    status,
    label: view.label,
    hint: staffConduct ? NEVER_PUBLIC_HINT : view.hint,
    canPublish: status !== "cong-khai" && !staffConduct,
    canHide: status !== "an",
  };
}

/**
 * Button labels — spec §8.3's words. The spec's emoji (👁, 🚫) are now lucide icons drawn beside the
 * words (`Eye`, `EyeOff` in `PublicationBox`, ADR 0068 §2 "bỏ emoji làm icon").
 */
export const PUBLISH_BUTTON_LABEL = "Cho hiện công khai";
export const HIDE_BUTTON_LABEL = "Ẩn khỏi trang công khai";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * THE CITIZEN'S RATING (§8.3, §14.3) — rating, rating_comment, rated_at, reopen_count; ADR 0050 pt 2
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const RATING_MAX_STARS = 5;

/**
 * At or below this, the rating is LOW: the spec's filter "phiếu 1–2 sao" (§4) and the server's fixed
 * reopen threshold "1 hoặc 2 sao" (ADR 0050 point 2 — a fixed number, not configuration).
 */
export const LOW_RATING_MAX = 2;

export const RATING_TITLE = "Đánh giá của người dân";
export const NOT_RATED = "Người dân chưa đánh giá";
/** The requirement's sentence (`FeedbackDetailDrawer.tsx:477`). */
export const LOW_RATING_REOPENED = "Đánh giá thấp — phiếu đã tự mở lại để xử lý tiếp.";
export const LOW_RATING_FILTER_LABEL = "Bị đánh giá thấp";

/** `★★☆☆☆` — filled first. Out-of-range values are clamped: the stars never claim more than five. */
export function ratingStars(rating: number): string {
  const filled = Math.max(0, Math.min(RATING_MAX_STARS, Math.round(rating)));
  return "★".repeat(filled) + "☆".repeat(RATING_MAX_STARS - filled);
}

export type RatingView =
  | { readonly kind: "none" }
  | {
      readonly kind: "rated";
      readonly stars: string;
      /** `4/5` */
      readonly score: string;
      /** Vietnam time, or `null` when the server sent no time. */
      readonly at: string | null;
      /** The citizen's own words — shown like `content`, as text, never as HTML. `""` when none. */
      readonly comment: string;
      readonly low: boolean;
      /** Show `LOW_RATING_REOPENED`: a low rating AND the server counts at least one reopening. */
      readonly reopened: boolean;
    };

export function ratingView(p: petitions_phieuPhanAnhRa): RatingView {
  if (p.rating === undefined || p.rating === null) return { kind: "none" };
  const low = p.rating <= LOW_RATING_MAX;
  return {
    kind: "rated",
    stars: ratingStars(p.rating),
    score: `${p.rating}/${RATING_MAX_STARS}`,
    at: p.rated_at === undefined || p.rated_at === null ? null : nhanThoiDiem(p.rated_at),
    comment: p.rating_comment ?? "",
    low,
    reopened: low && (p.reopen_count ?? 0) > 0,
  };
}

/**
 * The line under the deadlines (`FeedbackDetailDrawer.tsx:242-246`). `null` at zero, and when the
 * server predates the field — absent is "unknown", never "reopened 0 times" said out loud.
 */
export function reopenLine(reopenCount: number | null | undefined): string | null {
  if (reopenCount === undefined || reopenCount === null || reopenCount <= 0) return null;
  return `Đã mở lại ${reopenCount} lần do người dân chấm điểm thấp`;
}

/** Câu dẫn của ô nhập mã. Màn tra cứu nay đứng CẠNH quyển sổ, không thay cho nó. */
export const HUONG_DAN_TRA_CUU =
  "Nhập mã tra cứu đã trả cho người dân để mở thẳng đúng một phiếu, kể cả phiếu không nằm trong " +
  "bộ lọc đang chọn ở quyển sổ bên trên.";

/** Chưa gõ mã nào. Trạng thái BÌNH THƯỜNG lúc mở màn, không phải lỗi. */
export const CHUA_TRA_CUU = "Chưa tra phiếu nào. Nhập mã tra cứu rồi bấm Tra cứu.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * DANH MỤC LĨNH VỰC PHẢN ÁNH — 12 mục, §5
 *
 * ⚠ ĐÂY LÀ BẢN CHÉP TAY THỨ BA CỦA TỆP NÀY, cùng loại với `NHAN_TRANG_THAI` và `NHAN_KENH` bên
 * trên, và nó là LỖ HỔNG CỦA HỢP ĐỒNG chứ không phải một lựa chọn ở đây: `openapi.json` không có
 * tuyến nào trả về danh mục lĩnh vực phản ánh. Bộ mã tầng 1 do dịch vụ `platform` giữ và
 * `service-petitions` cũng không đọc được nó (ADR 0026, điều kiện dừng #2) — chính vì thế handler
 * **không kiểm** `field` tồn tại hay không (`domain.KiemLinhVuc`).
 *
 * HAI HỆ QUẢ ĐƯỢC NÓI RA THAY VÌ GIẤU:
 *
 *   1. Xã đặt lại tên cho một mã thì NHÃN TRÊN THẺ vẫn đúng — nó lấy `field_label` máy chủ gửi.
 *      Chỉ ô CHỌN dưới đây là dùng nhãn chép tay, vì lúc chọn thì chưa có phiếu nào để hỏi.
 *   2. Khách chốt thêm mã thứ mười ba thì màn hình này KHÔNG đỏ ở `tsc` — ô chọn chỉ thiếu một
 *      dòng, lặng lẽ. Đã báo về.
 *
 * `can-bo` CÓ TRONG DANH SÁCH NÀY, VÀ ĐÓ KHÔNG PHẢI SƠ SUẤT. Nó là lĩnh vực hạn chế ở đường ĐỌC
 * (máy chủ trả 404 cho tài khoản thiếu `feedback.restricted`, và loại hẳn khỏi trang), nhưng chốt
 * lĩnh vực VÀO `can-bo` là hành vi được phép với người có `feedback.classify`
 * (`app.duocChamPhieuHanChe`) — đúng tình huống một cán bộ đọc phiếu rồi nhận ra nó nói về một
 * đồng nghiệp. Bỏ mục ấy khỏi ô chọn là bịt đường phân loại đúng.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type MucChon = { readonly ma: string; readonly nhan: string };

export const LINH_VUC_PHAN_ANH: readonly MucChon[] = [
  { ma: "rac-thai", nhan: "Rác thải – Vệ sinh môi trường" },
  { ma: "giao-thong", nhan: "Hạ tầng giao thông" },
  { ma: "cap-thoat-nuoc", nhan: "Cấp thoát nước" },
  { ma: "dien", nhan: "Điện" },
  { ma: "trat-tu-do-thi", nhan: "Trật tự đô thị – lấn chiếm vỉa hè" },
  { ma: "an-ninh", nhan: "An ninh trật tự" },
  { ma: "xay-dung", nhan: "Xây dựng không phép" },
  { ma: "o-nhiem", nhan: "Ô nhiễm (tiếng ồn, khí thải, nước thải)" },
  { ma: "y-te-giao-duc", nhan: "Y tế – Giáo dục" },
  { ma: "can-bo", nhan: "Thái độ / tác phong cán bộ" },
  { ma: "an-toan-thuc-pham", nhan: "An toàn thực phẩm" },
  { ma: "khac", nhan: "Khác" },
];

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CHÍN TRẠNG THÁI — danh sách ĐÓNG (ADR 0027; khách duyệt NGUYÊN VĂN từng ký tự 20/09/2026)
 *
 * Đổi một trong chín chuỗi mã là **di trú hồ sơ lưu trữ** (luật 7), không phải đổi tên. Bảy mã
 * luồng chính đi theo đúng thứ tự vòng đời; hai mã còn lại là rẽ nhánh.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const LUONG_CHINH: readonly string[] = [
  "da-tiep-nhan",
  "dang-phan-loai",
  "da-chuyen-xu-ly",
  "dang-xu-ly",
  "da-xu-ly",
  "cho-dan-xac-nhan",
  "da-dong",
];

export const RE_NHANH: readonly string[] = ["khong-tiep-nhan", "chuyen-cap-tren"];

/** Chín mã, đúng thứ tự ô chọn `Tất cả trạng thái` của §4. */
export const MOI_TRANG_THAI: readonly string[] = [...LUONG_CHINH, ...RE_NHANH];

/** Bốn kênh tiếp nhận (§7, §12), cho ô lọc. */
export const MOI_KENH: readonly string[] = ["zalo-mini-app", "zalo-oa", "web-xa", "can-bo-nhap-ho"];

/** Một ô của StatusStepper §8.2. */
export type OBuoc = {
  readonly ma: string;
  readonly nhan: string;
  readonly vaiTro: "daQua" | "dangODay" | "chuaToi";
};

/**
 * Bảy ô luồng chính, với ô hiện tại đánh dấu `đang ở đây` (§8.2).
 *
 * PHIẾU ĐANG Ở MỘT RẼ NHÁNH (`khong-tiep-nhan`, `chuyen-cap-tren`) THÌ KHÔNG Ô NÀO LÀ "đang ở
 * đây", và bảy ô đều `chuaToi`. Không suy bừa một vị trí trên luồng chính cho một phiếu đã rời
 * khỏi luồng ấy: một ô tô màu sai nói với cán bộ rằng phiếu còn đang chạy.
 *
 * MỘT MÃ LẠ (hợp đồng mọc thêm trạng thái) cũng rơi vào đúng nhánh trên — không ô nào sáng — và
 * chuỗi trạng thái hiện nguyên mã ở chỗ khác. Không nhánh dự phòng nào ở đây tô ô đầu tiên.
 */
export function buocLuongChinh(trangThai: string): readonly OBuoc[] {
  const viTri = LUONG_CHINH.indexOf(trangThai);
  return LUONG_CHINH.map((ma, i) => ({
    ma,
    nhan: nhanTrangThai(ma),
    vaiTro: viTri < 0 ? "chuaToi" : i < viTri ? "daQua" : i === viTri ? "dangODay" : "chuaToi",
  }));
}

/**
 * Hai ô RẼ NHÁNH của §8.2 (`Không tiếp nhận`, `Chuyển cấp trên`), sáng theo TRẠNG THÁI của phiếu.
 *
 * Which of them is PRESSABLE, and which act it opens, is `statusStrip`'s (owner decision D1,
 * 09/10/2026): pressing one opens its reason form (`BieuMauReNhanh`) in the strip's composer.
 *
 * Không có `daQua`: hai nhánh là trạng thái cuối, không phiếu nào đi QUA chúng.
 */
export function buocReNhanh(trangThai: string): readonly OBuoc[] {
  return RE_NHANH.map((ma) => ({
    ma,
    nhan: nhanTrangThai(ma),
    vaiTro: ma === trangThai ? "dangODay" : "chuaToi",
  }));
}

/**
 * One explanation sentence per status (§8.2, the line under the stepper) — the PROTOTYPE'S NINE,
 * VERBATIM (owner, ADR 0027 Bổ sung 2026-10-02 row 1: *"Dùng nguyên câu bản mẫu"*):
 * `../vigov-require` `apps/admin/src/lib/feedback-display.ts:136-146` (`FEEDBACK_STATUS_HINT`).
 *
 * Eight are character-for-character the citizen app's table (`citizen-app/src/cong-dan/man/noi-dung.ts`
 * `TRANG_THAI`, row 4: one table for both apps). The NINTH differs on purpose: the citizen reads only
 * "Phiếu đã đóng." (row 5), while staff read the prototype's full sentence — the closing rule is a rule
 * for STAFF, shown only here.
 *
 * ⚠ The `da-dong` sentence is true only where the commune's `bat_buoc_anh_nghiem_thu` switch is on
 * (ADR 0008 decision 3, default on); the contract exposes no read of that switch, so the screen cannot
 * tell. The owner chose the verbatim sentence knowing this (ADR 0027 Bổ sung, "Hai chỗ chưa khớp").
 *
 * The words are the software's, not the commune's: a commune does not reword them. An unknown code
 * returns `null` — no line, never a sentence the web made up.
 */
const STATUS_EXPLANATION: Readonly<Record<string, string>> = {
  "da-tiep-nhan": "Phiếu vừa vào sổ, chưa phân cho ai.",
  "dang-phan-loai": "Đang xem phiếu thuộc lĩnh vực nào, có tiếp nhận không.",
  "da-chuyen-xu-ly": "Đã giao cho bộ phận, chưa bắt tay làm.",
  "dang-xu-ly": "Bộ phận đang xử lý tại hiện trường.",
  "da-xu-ly": "Đã làm xong, chờ báo lại cho người dân.",
  "cho-dan-xac-nhan": "Đã báo người dân, chờ họ xác nhận và chấm điểm.",
  "da-dong": "Phiếu đã đóng. Phải có ảnh sau xử lý mới đóng được.",
  "khong-tiep-nhan": "Không thuộc thẩm quyền hoặc không đủ căn cứ. Đã ghi lý do.",
  "chuyen-cap-tren": "Vượt thẩm quyền của xã, đã chuyển lên cấp trên.",
};

export function cauGiaiThichTrangThai(trangThai: string): string | null {
  // `hasOwn`, not a bare index: `constructor` is a key of the prototype, not a status.
  return Object.hasOwn(STATUS_EXPLANATION, trangThai) ? (STATUS_EXPLANATION[trangThai] ?? null) : null;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BỐN THAO TÁC, VÀ **HAI CỔNG KHÁC NHAU** — điểm chính của màn hình này
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Nút nào ĐƯỢC VẼ cho bộ quyền của phiên hiện tại. */
export type CongThaoTac = {
  readonly phanLoai: boolean;
  readonly phanCong: boolean;
  /**
   * Nút `Đóng phiếu`. `feedback.resolve` và **chỉ** khoá ấy — xem `QUYEN_DONG_PHAN_ANH` ở
   * `lib/quyen.ts`.
   */
  readonly dongPhieu: boolean;
  /**
   * The two public-page buttons (PUT …/publication). The route's key is `feedback.assign` — the same
   * key as `phanCong`, so it is derived from the same argument rather than read twice. UX only: the
   * server enforces the key, and its 403 reaches the screen verbatim.
   */
  readonly moderate: boolean;
};

/**
 * Bốn nút, và chỉ BA trong bốn có cổng ở giao diện.
 *
 * ⚠ `tienTrangThai` KHÔNG CÓ TRONG KIỂU NÀY, VÀ SỰ VẮNG MẶT ẤY LÀ NỘI DUNG CHÍNH CHỨ KHÔNG PHẢI
 * MỘT CHỖ CÒN THIẾU. Điều kiện thật của `…/status` là `feedback.resolve` **HOẶC** chính là cán bộ
 * được phân công phiếu ấy (LUẬT NẮM GIỮ). `assignee` của phiếu và `staff.code` của phiên đều là
 * MÃ CÁN BỘ (`CB-00123`) nên về kỹ thuật so được — nhưng giao diện CỐ Ý không so: luật nắm giữ là
 * của máy chủ (`service-petitions/internal/app/xu_ly_phan_anh.go:184-212`), và một bản sao ở đây
 * sẽ ẩn nút đúng vào ngày luật ấy được nới mà màn hình chưa biết.
 *
 * Nên nút tiến trạng thái HIỆN VỚI MỌI NGƯỜI XEM ĐƯỢC SỔ, và câu 403 của máy chủ
 * (*"Phiếu này không được phân công cho bạn…"*) ra thẳng màn hình. Gắn nó sau `feedback.resolve`
 * cho "gọn" là lấy mất đúng điều luật nắm giữ mở ra cho trưởng thôn.
 */
export function congThaoTac(coPhanLoai: boolean, coPhanCong: boolean, coDong: boolean): CongThaoTac {
  return { phanLoai: coPhanLoai, phanCong: coPhanCong, dongPhieu: coDong, moderate: coPhanCong };
}

/**
 * Nút `Phân loại` chỉ có nghĩa khi phiếu CHƯA phân loại.
 *
 * Máy chủ chốt lĩnh vực bằng một câu `UPDATE` mang `trang_thai = 'da-tiep-nhan'`, nên lần thứ hai
 * trả **409** (`routes.go`, khối `PHÂN LOẠI`). Ẩn nút ở trạng thái khác là nói ra điều ấy trước,
 * chứ không phải dựng thêm một luật.
 */
export function phanLoaiDuoc(trangThai: string): boolean {
  return trangThai === "da-tiep-nhan";
}

/**
 * The first value of the classify select: the petition's CURRENT field when it is one of the
 * catalogue's options, else empty. A staff-booked petition arrives with its field already chosen at
 * intake (PA-03); an empty select made the clerk pick it a second time. A code the catalogue lacks
 * stays empty — a select whose value matches no option shows one thing and submits another.
 */
export function initialClassifyField(field: string | undefined | null): string {
  if (field === undefined || field === null) return "";
  return LINH_VUC_PHAN_ANH.some((l) => l.ma === field) ? field : "";
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * HAI NHÁNH RẼ — `Không tiếp nhận`, `Chuyển cấp trên` (POST …/rejection, …/referral)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Hai nhánh chỉ rời được từ `dang-phan-loai` — bước một người đọc phiếu lần đầu và quyết xã có
 * nhận việc này không (`domain.KetThucNhanhDuoc`). Nơi khác máy chủ trả 409; ẩn biểu mẫu là nói
 * ra điều ấy trước, không phải dựng thêm một luật.
 */
export function reNhanhDuoc(trangThai: string): boolean {
  return trangThai === "dang-phan-loai";
}

/** Giới hạn của máy chủ (`service-petitions`, 400 `invalid_request` khi vượt). */
export const LY_DO_TOI_THIEU = 10;
export const LY_DO_TOI_DA = 2000;
export const CO_QUAN_TOI_DA = 200;

/**
 * Đếm KÝ TỰ như máy chủ đếm: số điểm mã Unicode (rune của Go) của chuỗi ĐÃ CẮT khoảng trắng.
 *
 * KHÔNG dùng `.length`: `.length` đếm đơn vị UTF-16, nên một ký tự ngoài mặt phẳng cơ bản (biểu
 * tượng cảm xúc cán bộ dán từ Zalo) đếm thành hai, và bộ đếm trên màn hình lệch khỏi con số máy
 * chủ dùng để từ chối. Chữ Việt dựng sẵn (`ệ`, `ữ`) là một điểm mã, đúng như người đọc thấy.
 */
export function demKyTu(s: string): number {
  return Array.from(s.trim()).length;
}

/** Câu lỗi của ô lý do, hoặc `null` khi hợp lệ. */
export function loiLyDo(lyDo: string): string | null {
  const n = demKyTu(lyDo);
  if (n < LY_DO_TOI_THIEU) {
    return `Lý do cần ít nhất ${LY_DO_TOI_THIEU} ký tự (hiện có ${n}).`;
  }
  if (n > LY_DO_TOI_DA) {
    return `Lý do không được quá ${LY_DO_TOI_DA} ký tự (hiện có ${n}).`;
  }
  return null;
}

/** Câu lỗi của ô cơ quan tiếp nhận, hoặc `null` khi hợp lệ. */
export function loiCoQuan(coQuan: string): string | null {
  const n = demKyTu(coQuan);
  if (n === 0) return "Cần ghi tên cơ quan tiếp nhận.";
  if (n > CO_QUAN_TOI_DA) {
    return `Tên cơ quan tiếp nhận không được quá ${CO_QUAN_TOI_DA} ký tự (hiện có ${n}).`;
  }
  return null;
}

export const NHAN_KHONG_TIEP_NHAN = "Không tiếp nhận";
export const NHAN_CHUYEN_CAP_TREN = "Chuyển cấp trên";
/** Prototype `FeedbackStatusPipeline.tsx:211-213`, verbatim. */
export const NHAN_O_LY_DO = "Lý do (bắt buộc, trả lời cho người dân)";
export const NHAN_O_CO_QUAN = "Cơ quan tiếp nhận";
export const GOI_Y_CO_QUAN = "Ví dụ: Công ty điện lực, Công an xã, Sở Xây dựng…";

export const CANH_BAO_RE_NHANH =
  "Thao tác này kết thúc xử lý phiếu tại xã và không hoàn tác được. Người dân được thông báo và " +
  "đọc được lý do này khi tra cứu phiếu của mình.";

/**
 * Phiếu có tài khoản công dân nào đứng sau để XÁC NHẬN không.
 *
 * `has_citizen` (thêm 26/09/2026) là đúng điều kiện máy chủ dùng — `cong_dan_id` khác rỗng — nói
 * ra dưới dạng một cờ, không mang danh tính. Có cờ thì cờ QUYẾT ĐỊNH, kể cả khi nó trái với kênh:
 * một phiếu `zalo-mini-app` mà không có tài khoản là chuyện có thật, và máy chủ cho đóng nó ở
 * `da-xu-ly`.
 *
 * CỜ VẮNG (`undefined`/`null` — máy chủ cũ, hay tuyến chưa trả) THÌ QUAY VỀ LUẬT KÊNH CŨ: kênh
 * `can-bo-nhap-ho` là phiếu cán bộ vào sổ thay dân, nên không có công dân. Sai lệch của luật ấy chỉ
 * theo một chiều an toàn: thiếu nút mà máy chủ vẫn cho, không bao giờ có nút máy chủ từ chối.
 */
export function coCongDanXacNhan(phieu: petitions_phieuPhanAnhRa): boolean {
  if (typeof phieu.has_citizen === "boolean") return phieu.has_citizen;
  return phieu.channel !== "can-bo-nhap-ho";
}

/**
 * Nút `Đóng phiếu` có nghĩa ở phiếu này không — HAI ĐIỂM, cùng hai điểm `domain.DongDuoc` của máy
 * chủ:
 *
 *   cho-dan-xac-nhan                                 luồng thường
 *   da-xu-ly  VÀ  không có công dân để xác nhận       xem `coCongDanXacNhan`
 */
export function dongDuocTrenManHinh(phieu: petitions_phieuPhanAnhRa): boolean {
  if (phieu.status === "cho-dan-xac-nhan") return true;
  return phieu.status === "da-xu-ly" && !coCongDanXacNhan(phieu);
}

/** Phiếu đã đóng hoặc đã rẽ nhánh thì không còn bước kế tiếp trên luồng chính. */
export function conBuocKeTiep(trangThai: string): boolean {
  const viTri = LUONG_CHINH.indexOf(trangThai);
  return viTri >= 0 && viTri < LUONG_CHINH.length - 1;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * THE CLICKABLE STATUS STRIP — owner decision D1, 09/10/2026
 *
 * A chip is clickable ONLY for a move our server performs for staff, and clicking it opens the ONE
 * existing act that performs it — its own route, its own key. The prototype's `TRANSITIONS` table is
 * NOT adopted: no lifecycle change, no notification change.
 *
 * `STAFF_MOVES` is the server's lifecycle map (`service-petitions/internal/domain/phieu_phan_anh.go:80-90`,
 * `chuyenDuocSang`) narrowed to the moves a STAFF act reaches, each paired with that act:
 *
 *   da-tiep-nhan    → dang-phan-loai    classify  (domain/xu_ly_phan_anh.go:288)
 *   dang-phan-loai  → da-chuyen-xu-ly   assign    (SauKhiPhanCong, :347-358)
 *   dang-phan-loai  → khong-tiep-nhan   reject    (KetThucNhanhDuoc — branches only from here)
 *   dang-phan-loai  → chuyen-cap-tren   refer
 *   da-chuyen-xu-ly → dang-xu-ly        advance   (tienTrinhChinh, :303-307)
 *   dang-xu-ly      → da-xu-ly          advance
 *   da-xu-ly        → cho-dan-xac-nhan  advance
 *   da-xu-ly        → da-dong           close     only with NO citizen to confirm (DongDuoc)
 *   cho-dan-xac-nhan → da-dong          close
 *
 * LEFT OUT ON PURPOSE: every reopening edge (`… → dang-xu-ly` after `da-xu-ly`) — only the citizen's
 * 1–2 star rating reaches it (ADR 0050 point 2); a staff chip there would be a button whose only answer
 * is a refusal. A copy of a server map can drift; when it does, the server's 409 sentence still reaches
 * the screen verbatim, and the drift is only ever a missing chip, never a move the server lacks.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** The existing act behind a move. */
export type StripAct = "classify" | "assign" | "reject" | "refer" | "advance" | "close";

const STAFF_MOVES: Readonly<Record<string, readonly { readonly to: string; readonly act: StripAct }[]>> = {
  "da-tiep-nhan": [{ to: "dang-phan-loai", act: "classify" }],
  "dang-phan-loai": [
    { to: "da-chuyen-xu-ly", act: "assign" },
    { to: "khong-tiep-nhan", act: "reject" },
    { to: "chuyen-cap-tren", act: "refer" },
  ],
  "da-chuyen-xu-ly": [{ to: "dang-xu-ly", act: "advance" }],
  "dang-xu-ly": [{ to: "da-xu-ly", act: "advance" }],
  "da-xu-ly": [
    { to: "cho-dan-xac-nhan", act: "advance" },
    { to: "da-dong", act: "close" },
  ],
  "cho-dan-xac-nhan": [{ to: "da-dong", act: "close" }],
};

/** Reasons a chip is disabled — the `title` / screen-reader text of the chip. */
export const STRIP_NOT_A_STEP = "Không chuyển thẳng sang bước này được theo quy trình.";
export const STRIP_NEEDS_CLASSIFY = "Cần quyền chốt lĩnh vực phản ánh (feedback.classify).";
export const STRIP_NEEDS_ASSIGN = "Cần quyền chuyển xử lý phản ánh (feedback.assign).";
export const STRIP_NEEDS_RESOLVE = "Cần quyền kết thúc xử lý phản ánh (feedback.resolve).";
export const STRIP_WAIT_CITIZEN = "Phiếu có người dân đứng sau: chờ người dân xác nhận rồi mới đóng.";
/** Prototype `FeedbackStatusPipeline.tsx:329-331`, verbatim — no move is open to this account. */
export const STRIP_NO_PERMISSION = "Bạn không có quyền đổi trạng thái phiếu này.";

/** One chip of the strip. */
export type StripStep = {
  readonly code: string;
  readonly label: string;
  /** `current` = where the petition is; `next` = clickable; `blocked` = not clickable now. */
  readonly role: "current" | "next" | "blocked";
  /** The act a `next` chip opens. */
  readonly act?: StripAct;
  /** Why a `blocked` chip cannot be pressed; the status explanation on `current`. */
  readonly reason: string;
  /** A lifecycle move from the current status (pressable or not) — the branch row shows only these. */
  readonly isMove: boolean;
};

export type StatusStripView = {
  readonly main: readonly StripStep[];
  /** Only the branch the petition is on, or a branch it can move to (prototype `:156-158`). */
  readonly branches: readonly StripStep[];
  /** There is a move from here, and this account may press none of them. */
  readonly noPermission: boolean;
};

function gateReason(act: StripAct, cong: CongThaoTac, petition: petitions_phieuPhanAnhRa): string | null {
  switch (act) {
    case "classify":
    case "reject":
    case "refer":
      return cong.phanLoai ? null : STRIP_NEEDS_CLASSIFY;
    case "assign":
      return cong.phanCong ? null : STRIP_NEEDS_ASSIGN;
    case "advance":
      // NO UI GATE: the holder rule is the server's (`congThaoTac`); its 403 reaches the screen.
      return null;
    case "close":
      if (!dongDuocTrenManHinh(petition)) return STRIP_WAIT_CITIZEN;
      return cong.dongPhieu ? null : STRIP_NEEDS_RESOLVE;
  }
}

/** The whole strip for one petition and one account's gates. */
export function statusStrip(petition: petitions_phieuPhanAnhRa, cong: CongThaoTac): StatusStripView {
  const moves = Object.hasOwn(STAFF_MOVES, petition.status) ? (STAFF_MOVES[petition.status] ?? []) : [];
  const step = (code: string): StripStep => {
    const label = nhanTrangThai(code);
    if (code === petition.status) {
      return { code, label, role: "current", reason: cauGiaiThichTrangThai(code) ?? "", isMove: false };
    }
    const move = moves.find((m) => m.to === code);
    if (move === undefined) return { code, label, role: "blocked", reason: STRIP_NOT_A_STEP, isMove: false };
    const refused = gateReason(move.act, cong, petition);
    return refused === null
      ? { code, label, role: "next", act: move.act, reason: `Chuyển sang ${label}`, isMove: true }
      : { code, label, role: "blocked", reason: refused, isMove: true };
  };
  const main = LUONG_CHINH.map(step);
  const branches = RE_NHANH.map(step).filter((s) => s.role === "current" || s.isMove);
  const all = [...main, ...branches];
  return {
    main,
    branches,
    noPermission: moves.length > 0 && !all.some((s) => s.role === "next"),
  };
}

/** The composer's title — prototype `FeedbackStatusPipeline.tsx:187-189`. */
export function composerTitle(statusCode: string): string {
  return `Chuyển sang “${nhanTrangThai(statusCode)}”`;
}

/** Success toast of a status act — prototype `FeedbackStatusPipeline.tsx:96`. */
export function movedToast(statusCode: string): string {
  return `Đã chuyển sang “${nhanTrangThai(statusCode)}”.`;
}

/** Success toasts of the other acts — prototype `FeedbackDetailDrawer.tsx:174,277,299,183`. */
export const ASSIGNED_TOAST = "Đã chuyển xử lý.";
export const PUBLISHED_TOAST = "Phiếu đã hiện công khai.";
export const HIDDEN_TOAST = "Đã ẩn khỏi trang công khai.";
export const AFTER_PHOTO_TOAST = "Đã tải ảnh sau xử lý.";
export const LOG_WRITTEN_TOAST = "Đã ghi vào nhật ký.";

/** Prototype `HandoverFields` warning, verbatim (row 52-60). */
export const UNIT_WITHOUT_STAFF = "Bộ phận này chưa có cán bộ nào đang hoạt động.";

/** Fact cell `Đang giao cho` fallbacks — prototype `FeedbackDetailDrawer.tsx:251,256`. */
export const NO_UNIT_YET = "Chưa giao bộ phận nào";
export const NO_ASSIGNEE_YET = "Chưa chỉ định cán bộ xử lý";

/** The branch box's label — prototype `FeedbackDetailDrawer.tsx:338-340`. */
export function branchReasonLabel(statusCode: string): string {
  return `Lý do ${nhanTrangThai(statusCode).toLowerCase()}`;
}

/** Câu dưới nút `Đóng phiếu` khi tài khoản thiếu `feedback.resolve`. Nói ĐÚNG tên khoá. */
export const CAU_THIEU_QUYEN_DONG =
  "Tài khoản của bạn không có quyền kết thúc xử lý phản ánh (feedback.resolve), nên không có nút " +
  "Đóng phiếu. Bạn vẫn tiến được trạng thái của phiếu được phân công cho mình.";

export const CAU_THIEU_QUYEN_PHAN_LOAI =
  "Tài khoản của bạn không có quyền chốt lĩnh vực phản ánh (feedback.classify), nên không có ô " +
  "phân loại và không có hai thao tác Không tiếp nhận, Chuyển cấp trên. Chốt lĩnh vực là hành vi " +
  "ấn định hạn xử lý xong của xã.";

export const CAU_THIEU_QUYEN_PHAN_CONG =
  "Tài khoản của bạn không có quyền chuyển xử lý phản ánh (feedback.assign), nên không có khối " +
  "Chuyển xử lý.";

/** Under the `Hiển thị với người dân` box when the account lacks `feedback.assign`. Names the key. */
export const PUBLICATION_DENIED =
  "Tài khoản của bạn không có quyền chuyển xử lý phản ánh (feedback.assign), nên không đổi được " +
  "việc hiển thị phiếu này trên trang công khai.";

/** Nhãn nút tiến trạng thái. Không nêu tên bước kế tiếp: máy chủ giữ bản đồ, không phải màn này. */
export const NHAN_TIEN_TRANG_THAI = "Chuyển sang bước kế tiếp";

// Plain administrative wording (tester report 05/10/2026, PA-07): a clerk reads what the button does
// and who may press it — not how the server is built.
export const GHI_CHU_TIEN_TRANG_THAI =
  "Chuyển phiếu sang bước xử lý tiếp theo. Cán bộ được phân công phiếu này hoặc cán bộ có quyền " +
  "xử lý phản ánh của xã đều thực hiện được.";

/**
 * Shown INSTEAD of the `Chuyển xử lý` form while the petition is still classifiable (`phanLoaiDuoc`).
 * The server refuses an assignment from `da-tiep-nhan` with 409 (ADR 0027); drawing a live form there
 * is drawing a button whose only answer is a refusal (tester report 05/10/2026, PA-03).
 */
export const ASSIGN_NEEDS_CLASSIFICATION = "Cần phân loại phiếu trước khi chuyển xử lý.";

/**
 * Prototype `FeedbackStatusPipeline.tsx:196-203`, verbatim. The prototype asks it at `resolved`; our
 * server takes the citizen-readable result at CLOSING (rule 10, invariant 6), so it sits in the close act.
 */
export const NHAN_O_KET_QUA = "Kết quả xử lý — người dân sẽ đọc câu này";
export const RESULT_PLACEHOLDER = "Đã thu gom toàn bộ rác, dọn vệ sinh khu vực.";

export const GHI_CHU_O_KET_QUA =
  "Bắt buộc. Ghi rõ kết quả xử lý; người dân sẽ đọc được nội dung này khi tra cứu phiếu.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * QUYỂN SỔ — câu chữ của danh sách và bộ lọc (§2, §4)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const SO_RONG =
  "Không có phiếu phản ánh nào khớp bộ lọc đang chọn. Bỏ bớt điều kiện lọc để xem rộng hơn.";

export const DANG_TAI_SO = "Đang tải sổ phản ánh…";

/**
 * The register with no filter and nothing to show — the prototype's two lines, word for word
 * (`FeedbackWorkspace.tsx:236-242`). Since owner decision D3 (09/10/2026) an account without
 * `feedback.restricted` also reads, above, that staff-conduct petitions are not shown here, so "Chưa có
 * phản ánh nào" is read as "none on this screen" and says nothing about any restricted petition.
 */
export const LIST_EMPTY_TITLE = "Chưa có phản ánh nào";
export const LIST_EMPTY_HINT = "Phiếu gửi từ Zalo Mini App sẽ hiện ở đây ngay khi người dân bấm gửi.";

/** A card whose petition carries no address (the prototype's fallback, verbatim). */
export const CARD_NO_LOCATION = "Chưa rõ vị trí";

/** The page header (prototype `FeedbackWorkspace.tsx:75-81`, word for word). */
export const PETITION_PAGE_TITLE = "Phản ánh của người dân";
export const PETITION_PAGE_SUBTITLE =
  "Tiếp nhận từ Zalo Mini App và các kênh khác, theo dõi thời hạn, đối chiếu ảnh trước và sau khi xử lý.";

/** Nhãn "mọi giá trị" của từng ô lọc — nguyên văn §4. */
export const MOI_TRANG_THAI_NHAN = "Tất cả trạng thái";
export const MOI_LINH_VUC_NHAN = "Tất cả lĩnh vực";
export const MOI_DIA_BAN_NHAN = "Tất cả địa bàn";
export const MOI_BO_PHAN_NHAN = "Tất cả bộ phận";
export const MOI_KENH_NHAN = "Tất cả kênh tiếp nhận";
export const CHI_TRE_HAN_NHAN = "Chỉ phiếu trễ hạn";
export const TIM_PLACEHOLDER = "Tìm theo nội dung, mã phiếu, địa chỉ…";

/**
 * Bộ phận đang giữ phiếu, đọc ra thành chữ.
 *
 * `unit` LÀ ULID, KHÔNG PHẢI TÊN. Tra sang tên bằng danh mục `GET /api/v1/org-units` — tuyến
 * `any-authenticated`, nên mọi tài khoản xem được sổ đều tra được. Một id không có trong danh mục
 * (bộ phận vừa bị đổi, danh mục đọc hỏng) thì hiện câu nói ra điều đó, KHÔNG hiện id: một chuỗi
 * `01JB…` trên màn hình cán bộ là một chuỗi không ai làm gì được.
 */
export function nhanBoPhan(id: string, tenTheoID: ReadonlyMap<string, string>): string {
  if (id === "") return "Chưa chuyển bộ phận";
  return tenTheoID.get(id) ?? "Bộ phận không còn trong danh mục";
}

/** Danh bạ chọn người, tra theo MÃ CÁN BỘ (`code`). */
export type DanhBaTheoMa = ReadonlyMap<string, identity_canBoChonNguoiRa>;

/** Dựng bảng tra từ `items` của `GET /api/v1/staff-directory`. */
export function danhBaTheoMa(items: readonly identity_canBoChonNguoiRa[]): DanhBaTheoMa {
  return new Map(items.map((cb) => [cb.code, cb]));
}

/**
 * A staff business code read out on a record's history line: `Full name (CB-…)` when the directory
 * knows the code, the bare code otherwise (directory not loaded, failed, or the account is no longer
 * active — a retired officer's rows must still name exactly one person).
 *
 * THE CODE STAYS ON THE LINE even with a name: history is re-read during complaints, and two officers
 * can share a full name; the code is what still identifies one person years later (rule 6, inv. 8).
 * Shared by the petition log, the incoming-document panel and the task log (`nhanNguoiNhatKy`).
 */
export function staffNameWithCode(code: string, directory: DanhBaTheoMa | null): string {
  const cb = directory?.get(code);
  if (cb === undefined || cb.full_name === "") return code;
  return `${cb.full_name} (${code})`;
}

/**
 * Ô `ĐANG GIAO CHO` của §8.3 — phần CÁN BỘ.
 *
 * `assignee` LÀ MÃ CÁN BỘ (`CB-00123`), và họ tên tra bằng danh bạ chọn người
 * (`GET /api/v1/staff-directory`, mọi cán bộ đăng nhập đọc được). BỐN CA:
 *
 *   rỗng                         phiếu giao cho bộ phận, bộ phận tự phân công
 *   có trong danh bạ             họ tên (họ tên rỗng thì hiện mã — không bao giờ một ô trống)
 *   danh bạ chưa tải / tải hỏng  hiện MÃ: mã là thứ duy nhất màn hình biết chắc
 *   không có trong danh bạ       hiện MÃ kèm một câu trung tính. Danh bạ chỉ gồm người có tài khoản
 *                                đang hoạt động, nên người đã khoá tài khoản (nghỉ hưu, chuyển công
 *                                tác) rơi vào ca này — và phiếu cũ của họ vẫn phải đọc được
 *
 * KHÔNG CÓ EMAIL như đặc tả vẽ (`Họ tên — email`): danh bạ chọn người cố ý không trả email.
 */
export function nhanCanBoXuLy(assignee: string, danhBa: DanhBaTheoMa | null): string {
  if (assignee === "") return "Chưa phân công cán bộ cụ thể";
  if (danhBa === null) return assignee;
  const cb = danhBa.get(assignee);
  if (cb === undefined) return `${assignee} (không có trong danh bạ cán bộ đang hoạt động)`;
  return cb.full_name === "" ? assignee : cb.full_name;
}

/**
 * Một dòng của ô chọn `Cán bộ xử lý` (§8.5). Đặc tả vẽ `Họ tên — email · Chức danh`; danh bạ không
 * có email nên dòng là `Họ tên · Chức danh`, hoặc chỉ họ tên khi chưa có chức danh.
 */
export function nhanLuaChonCanBo(cb: identity_canBoChonNguoiRa): string {
  const ten = cb.full_name === "" ? cb.code : cb.full_name;
  return cb.position === "" ? ten : `${ten} · ${cb.position}`;
}

/** Nhãn và lựa chọn mặc định của ô chọn cán bộ — nguyên văn §8.5. */
export const NHAN_CHON_CAN_BO = "Cán bộ xử lý";
export const DE_BO_PHAN_PHAN_CONG = "— Để bộ phận phân công —";

/**
 * Ba tab phạm vi của §4. Tab thứ ba — `Liên quan đến tôi` — là chỗ giữ vô hiệu có dấu "?" (ADR 0068
 * §14), xem `PHAN_CHUA_DUNG` mục `scopeRelated`.
 */
export const PHAM_VI_TOAN_XA = "Toàn xã";
export const PHAM_VI_GIAO_CHO_TOI = "Giao cho tôi";
export const SCOPE_RELATED_LABEL = "Liên quan đến tôi";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHẬT KÝ XỬ LÝ (§8.7) và GHI CHÚ NỘI BỘ của sáu thao tác
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Mười mã thao tác của một dòng nhật ký — danh sách ĐÓNG của `service-petitions`.
 *
 * Hợp đồng khai `action` là `string` trơn (không `enum`), cùng lỗ hổng với `NHAN_TRANG_THAI` ở trên.
 * `Record<MaThaoTacNhatKy, string>` bên dưới vì thế là chỗ canh ở mức KIỂU: thêm một mã vào hợp này
 * mà quên nhãn là `tsc` đỏ. Mã lạ từ máy chủ thì rơi xuống nhánh dự phòng, nói rõ mã chưa có nhãn.
 */
export type MaThaoTacNhatKy =
  | "phan-loai"
  | "phan-cong"
  | "chuyen-trang-thai"
  | "dong-phieu"
  | "khong-tiep-nhan"
  | "chuyen-cap-tren"
  | "ghi-chu"
  // The citizen's rating (ADR 0050 point 2, `domain.LogActionCitizenRating` /
  // `LogActionReopenByRating`): rated with nothing moved, and rated 1–2 stars so the petition came back.
  | "danh-gia"
  | "mo-lai-theo-danh-gia"
  // A task booked from this petition (`domain.LogActionTaskCreated`); the row's note holds the task's
  // register number only.
  | "tao-nhiem-vu"
  // The staff intake booked this petition (ADR 0028 Bổ sung 2026-10-02 row 6, "cán bộ X nhập hộ").
  | "nhap-ho";

/** Nhãn nguyên văn do chuyên gia nghiệp vụ chốt. `phan-cong` hiện là "Chuyển xử lý", như nút §8.5. */
export const NHAN_THAO_TAC_NHAT_KY: Readonly<Record<MaThaoTacNhatKy, string>> = {
  "phan-loai": "Phân loại",
  "phan-cong": "Chuyển xử lý",
  "chuyen-trang-thai": "Chuyển trạng thái",
  "dong-phieu": "Đóng phiếu",
  "khong-tiep-nhan": "Không tiếp nhận",
  "chuyen-cap-tren": "Chuyển cấp trên",
  "ghi-chu": "Ghi chú",
  // The two sentences the server writes as the note carry the stars ("Người dân đánh giá n sao[ —
  // phiếu được mở lại]"); these labels only name the act, so the row does not repeat the number.
  "danh-gia": "Người dân đánh giá",
  "mo-lai-theo-danh-gia": "Mở lại do đánh giá thấp",
  "tao-nhiem-vu": "Tạo nhiệm vụ",
  "nhap-ho": "Nhập hộ phản ánh",
};

export function nhanThaoTacNhatKy(ma: string): string {
  return traNhan(NHAN_THAO_TAC_NHAT_KY, ma, "thao tác");
}

/** The fixed actor marker of rows a CITIZEN caused (`domain.CitizenLogActor`). */
export const CITIZEN_LOG_ACTOR = "cong-dan";

/**
 * The `Người thực hiện` cell of a log row.
 *
 * `cong-dan` IS A MARKER, NOT A STAFF CODE, and it is never looked up in the staff directory: the server
 * writes it instead of the citizen's id on purpose, so an anonymous citizen's reports cannot be linked
 * on a staff screen (ADR 0008). Every other value is a staff business code, read out as
 * `Full name (CB-…)` through the screen's one directory read (`staffNameWithCode`) — the bare code when
 * the directory does not know it (rule 6, invariant 8).
 */
export function logActorLabel(actorCode: string, directory: DanhBaTheoMa | null): string {
  if (actorCode === "") return "—";
  if (actorCode === CITIZEN_LOG_ACTOR) return "Người dân";
  return staffNameWithCode(actorCode, directory);
}

/** Chỉ dòng `phan-cong` mang bộ phận và người phụ trách; dòng khác hai trường ấy rỗng. */
export function laDongPhanCong(ma: string): boolean {
  return ma === "phan-cong";
}

/** Giới hạn của máy chủ cho một ghi chú nhật ký, và cho `note` của sáu thao tác. */
export const GHI_CHU_TOI_DA = 2000;

/** Câu lỗi của ô ghi nhật ký (BẮT BUỘC có chữ), hoặc `null` khi hợp lệ. */
export function loiGhiChuNhatKy(ghiChu: string): string | null {
  const n = demKyTu(ghiChu);
  if (n === 0) return "Cần ghi nội dung nhật ký.";
  if (n > GHI_CHU_TOI_DA) return `Nhật ký không được quá ${GHI_CHU_TOI_DA} ký tự (hiện có ${n}).`;
  return null;
}

/** Câu lỗi của ô ghi chú nội bộ TUỲ CHỌN của sáu thao tác, hoặc `null` (trống là hợp lệ). */
export function loiGhiChuNoiBo(ghiChu: string): string | null {
  const n = demKyTu(ghiChu);
  return n > GHI_CHU_TOI_DA
    ? `Ghi chú nội bộ không được quá ${GHI_CHU_TOI_DA} ký tự (hiện có ${n}).`
    : null;
}

export const TIEU_DE_NHAT_KY = "Nhật ký xử lý";
export const DANG_TAI_NHAT_KY = "Đang tải nhật ký xử lý…";
/**
 * No row — prototype `FeedbackActivityPanel.tsx:237-239`, verbatim. (A petition booked before
 * 26/09/2026 has none by design: the log started that day.)
 */
export const NHAT_KY_RONG = "Chưa có ghi chép nào.";
export const NHAN_XEM_THEM_NHAT_KY = "Xem thêm";
export const NHAN_NUT_GHI_NHAT_KY = "Ghi nhật ký";
export const NHAN_O_GHI_NHAT_KY = "Nội dung nhật ký";
export const GOI_Y_GHI_NHAT_KY = "Đã làm gì, ai làm, còn vướng gì…";
/** Luật 3: nhật ký là bản ghi lưu trữ, không phải chỗ chép số điện thoại hay số CCCD. */
export const NHAC_DU_LIEU_CA_NHAN =
  "Không ghi số điện thoại, số CCCD của người dân vào nhật ký.";
export const NHAN_BO_PHAN_PHU_TRACH = "Bộ phận / Phụ trách";
export const NHAN_NGUOI_THUC_HIEN = "Người thực hiện";

/**
 * The optional note of every act — prototype `FeedbackStatusPipeline.tsx:236-241`, verbatim. It goes
 * into the processing log only; the line under the box says it is not sent to the citizen.
 */
export const NHAN_O_GHI_CHU_NOI_BO = "Nội dung cập nhật";
export const UPDATE_NOTE_PLACEHOLDER = "Đã làm gì, ai làm, còn vướng gì… (không bắt buộc)";
export const UPDATE_NOTE_INTERNAL = "không gửi người dân";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * "TẠO NHIỆM VỤ" FROM A PETITION (§13) — POST …/tasks
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * The statuses the button is OFFERED on — the four where the petition has been classified and handed
 * to a unit, the server's rule (`domain.PetitionAcceptsTask`). An ALLOW list, so a status code the
 * screen does not know hides the button (fail closed, as the server does).
 *
 * The server refuses the rest by name: `da-tiep-nhan` / `dang-phan-loai` → 409 `petition_not_classified`,
 * the three terminal statuses → 409 `petition_state`. Its sentence reaches the screen verbatim should
 * the two lists ever disagree.
 */
const PETITION_TASK_STATUSES: readonly string[] = [
  "da-chuyen-xu-ly",
  "dang-xu-ly",
  "da-xu-ly",
  "cho-dan-xac-nhan",
];

/**
 * Whether the drawer OFFERS `Tạo nhiệm vụ`. UX ONLY — the route checks `task.create` AND
 * `feedback.read` and the petition's state on every call (rule 5, forbidden #1).
 *
 * TWO KEYS, BOTH REQUIRED, neither implied by the other (rule 5, invariant 3b): reading petitions is
 * not booking tasks, and booking tasks is not reading petitions.
 *
 * `can-bo` IS HIDDEN: a staff-conduct report is handled by the leadership on the petition itself, and
 * the server refuses it with 409 `restricted_field_no_task`. A holder of `feedback.restricted` is the
 * only one who ever sees such a petition here, so hiding the button reveals nothing.
 */
export function petitionTaskOffered(
  permissions: readonly string[],
  petition: Pick<petitions_phieuPhanAnhRa, "status" | "field">,
): boolean {
  return (
    coQuyen(permissions, QUYEN_TAO_NHIEM_VU) &&
    coQuyen(permissions, QUYEN_XEM_PHAN_ANH) &&
    PETITION_TASK_STATUSES.includes(petition.status) &&
    // No settled field is refused by the server too (`petition_not_classified`).
    petition.field !== "" &&
    petition.field !== STAFF_CONDUCT_FIELD
  );
}

export const PETITION_TASK_BUTTON = "Tạo nhiệm vụ";

/**
 * The prototype's `TaskFromRecordDialog` heading and submit words. Its description ("Hạn xử lý kế thừa
 * từ hồ sơ…") is NOT copied: this route does not inherit the petition's deadline, and the line would
 * promise something the server does not do. Only its first sentence is kept.
 */
export const PETITION_TASK_DIALOG_TITLE = "Tạo nhiệm vụ từ hồ sơ này";
export const PETITION_TASK_DIALOG_DESCRIPTION = "Xem lại rồi mới tạo.";

/**
 * The title the form opens with. ONLY the lookup code — never the petition's content or the reporter:
 * a task title is read by every officer who can read tasks, and petition content is personal data
 * (rule 3). The clerk rewrites it into a sentence of work.
 */
export function petitionTaskTitle(lookupCode: string): string {
  return `Xử lý phản ánh ${lookupCode}`;
}

/** A sentence, not a disabled select: nothing is sent — the server takes the petition in the path. */
export function petitionTaskSourceNote(lookupCode: string): string {
  return `Nguồn giao: Từ phản ánh ${lookupCode} — máy chủ gắn theo phiếu, không sửa được.`;
}

/** Carries the register number the server just issued — something the clerk cannot know beforehand. */
export function petitionTaskCreated(taskCode: string): string {
  return `Đã tạo nhiệm vụ ${taskCode}.`;
}

/**
 * The task register. There is no deep link to one task by code (`/nhiem-vu` reads no such parameter),
 * so the link opens the register and the sentence beside it names the code.
 */
export const TASK_REGISTER_HREF = "/nhiem-vu";
export const TASK_REGISTER_LINK_LABEL = "Mở sổ Nhiệm vụ";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * "NHẬP HỘ PHẢN ÁNH" (§11) — POST /api/v1/citizen-reports, `feedback.create`
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** "Tiếp nhận phản ánh" — seeded at `service-identity/migrations/0001_init.sql:298`, spec §14.6. */
export const PETITION_INTAKE_PERMISSION = "feedback.create";

export const INTAKE_BUTTON = "Nhập hộ phản ánh";
export const INTAKE_TITLE = "Nhập hộ phản ánh của người dân";
/** Spec §11, approved by the owner 30/09/2026 (spec 09:248-249) — verbatim. */
export const INTAKE_DESCRIPTION =
  "Dùng khi người dân gọi điện, ghé trụ sở, hoặc gặp trưởng thôn ngoài địa bàn. Phiếu nhập ở đây đi " +
  "cùng quy trình với phiếu gửi từ Zalo. Vì đã biết lĩnh vực ngay, hạn xử lý được ấn định luôn — hãy " +
  "ghi đúng thời điểm người dân phản ánh.";
export const INTAKE_FIELD_LABEL = "Lĩnh vực";
export const INTAKE_FIELD_PLACEHOLDER = "— Chọn lĩnh vực —";
export const INTAKE_FIELDS_LOADING = "Đang tải danh sách lĩnh vực…";
export const INTAKE_FIELDS_EMPTY =
  "Xã chưa bật lĩnh vực phản ánh nào, nên chưa nhập hộ được. Hãy bật lĩnh vực ở màn hình Cấu hình.";
export const INTAKE_CLOCK_LABEL = "Dân phản ánh lúc";
export const INTAKE_CLOCK_HINT =
  "Để trống thì lấy lúc vào sổ. Không được sớm hơn 7 ngày trước lúc vào sổ, và không được muộn hơn lúc vào sổ.";
export const INTAKE_CONTENT_LABEL = "Nội dung phản ánh";
export const INTAKE_CONTENT_PLACEHOLDER = "Ghi lại lời người dân: sự việc gì, ở đâu, từ khi nào.";
export const INTAKE_ADDRESS_LABEL = "Địa chỉ, vị trí";
export const INTAKE_ADDRESS_PLACEHOLDER = "Đầu ngõ thôn Hà Lam";
export const INTAKE_NAME_LABEL = "Người gửi";
export const INTAKE_PHONE_LABEL = "Số điện thoại";
export const INTAKE_ANONYMOUS_LABEL = "Người dân đề nghị gửi ẩn danh";
export const INTAKE_SUBMIT = "Vào sổ phản ánh";
export const INTAKE_CANCEL = "Huỷ";
export const INTAKE_CHANNEL_NOTE = "Kênh tiếp nhận: Cán bộ, trưởng thôn nhập hộ.";

export const INTAKE_DONE_TITLE = "Đã vào sổ phản ánh";
export const INTAKE_DONE_CODE_LABEL = "Mã tra cứu";
/**
 * ADR 0028 Bổ sung 2026-10-02 rows 1–3: the petition is linked to NO citizen account, so it never
 * appears in "Phản ánh của tôi" — the code the officer hands over is the citizen's only handle. Says
 * nothing about WHERE the citizen can use it: lookup without a session is rule 4 stop condition #2,
 * not decided (same ADR, "Hệ quả — chưa chốt").
 */
export const INTAKE_DONE_SENTENCE =
  "Hãy đọc hoặc đưa mã tra cứu này cho người dân và dặn họ giữ lại. Phiếu nhập hộ không gắn với tài " +
  "khoản Zalo nào, nên không hiện trong mục “Phản ánh của tôi” trên ứng dụng của người dân.";
export const INTAKE_DONE_CLOSE = "Đóng";
export const INTAKE_DONE_ANOTHER = "Nhập phiếu khác";

/** The content is required, and the server refuses a blank one — say so before sending. */
export function intakeContentError(content: string): string | null {
  return demKyTu(content) === 0 ? "Cần ghi nội dung phản ánh." : null;
}

/**
 * The commune's administrative clock — a PLATFORM constant, not per commune: Vietnam has one zone and
 * no daylight saving (same pin as `nhanThoiDiem`, `lib/drill-down.ts`).
 */
const ADMINISTRATIVE_OFFSET = "+07:00";

/**
 * `<input type="datetime-local">` (`2026-10-02T08:30`) → RFC 3339 WITH a zone
 * (`2026-10-02T08:30:00+07:00`), or `""` when blank.
 *
 * THE VALUE IS READ AS VIETNAM TIME, NOT AS THE BROWSER'S ZONE: the officer types the hour the citizen
 * called, on the commune's clock. A laptop set to another zone would otherwise move the deadline's
 * starting point by hours — and the deadline is a commitment counted from that point (rule 10). A
 * string that is not the input's shape returns `null`: refused here, never guessed.
 */
export function clockFromRfc3339(local: string): string | null {
  const v = local.trim();
  if (v === "") return "";
  const m = /^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(v);
  if (m === null) return null;
  return `${m[1]}T${m[2]}:${m[3]}:${m[4] ?? "00"}${ADMINISTRATIVE_OFFSET}`;
}

const LOCAL_INPUT_PARTS = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Asia/Ho_Chi_Minh",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

/** An instant as a `datetime-local` value on the commune's clock — for the input's `min`/`max`. */
export function toLocalInputValue(at: Date): string {
  const parts = LOCAL_INPUT_PARTS.formatToParts(at);
  const get = (t: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === t)?.value ?? "";
  // `en-GB` writes midnight as `24` in some engines; the input wants `00`.
  const hour = get("hour") === "24" ? "00" : get("hour");
  return `${get("year")}-${get("month")}-${get("day")}T${hour}:${get("minute")}`;
}

/**
 * The input's `min` / `max` — a HINT to the picker only. The server decides against the BOOKING
 * instant, which is a few seconds after this; its 400 `clock_from_out_of_range` sentence is the answer.
 */
export function clockFromBounds(now: Date): { readonly min: string; readonly max: string } {
  const sevenDays = 7 * 24 * 60 * 60 * 1000;
  return { min: toLocalInputValue(new Date(now.getTime() - sevenDays)), max: toLocalInputValue(now) };
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * THE FOUR KPI CARDS (§3) — GET /api/v1/citizen-report-summary (`feedback.read` AND `report.read`)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Spec §3 card 1: "TỔNG PHẢN ÁNH 90 NGÀY". Cards 1 and 2 read the same window. */
export const KPI_WINDOW_DAYS = 90;

export const KPI_TOTAL_LABEL = "Tổng phản ánh 90 ngày";
export const KPI_ON_TIME_LABEL = "Đúng hạn / trễ hạn";
export const KPI_RATING_LABEL = "Điểm hài lòng trung bình";
export const KPI_PENDING_LABEL = "Chờ kiểm duyệt";
/** Spec §3 card 4's line, verbatim. */
export const KPI_PENDING_CAPTION = "Kiểm duyệt trước khi hiển thị công khai";
export const KPI_LOADING = "Đang tải số liệu phản ánh…";
export const KPI_NO_RATING = "Chưa có phiếu nào được người dân chấm điểm";
export const KPI_NO_DEADLINE_SAMPLE = "Chưa có phiếu nào có hạn trong 90 ngày qua";

const ONE_DECIMAL = new Intl.NumberFormat("vi-VN", { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const COUNT = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 0 });

export function kpiCount(n: number): string {
  return COUNT.format(n);
}

/**
 * The average rating, `sum / sample`, one decimal (`4,3/5`), or `null` for NO VALUE.
 *
 * `null` WHEN THE SAMPLE IS 0, NEVER `0,0/5`: a commune nobody has rated yet is not a commune rated
 * zero, and a 0 printed here is the figure that travels upward. `null` too when the server predates the
 * fields (absent / null) — unknown is not zero either. The division is the client's by the server's own
 * contract (`summary_metrics.go`: "divided by the client like every ratio here").
 */
export function ratingAverage(sum: number | null | undefined, sample: number | null | undefined): string | null {
  if (typeof sum !== "number" || typeof sample !== "number" || !(sample > 0)) return null;
  return `${ONE_DECIMAL.format(sum / sample)}/${RATING_MAX_STARS}`;
}

/** `on_time / on_time_sample` as `33,3%`, or `null` when the sample is 0 — never 0%. */
export function onTimePercent(onTime: number, sample: number): string | null {
  if (!(sample > 0)) return null;
  return `${ONE_DECIMAL.format((onTime / sample) * 100)}%`;
}

export function lowRatingCaption(n: number): string {
  return `${kpiCount(n)} phiếu bị đánh giá thấp`;
}

export function inProgressCaption(n: number): string {
  return `${kpiCount(n)} phiếu đang xử lý`;
}

/** Accessible name of a KPI link — the dashboard's wording (`drillLabel`, spec 01 §4/§9). */
export function kpiLinkLabel(label: string): string {
  return `Xem danh sách đằng sau: ${label}`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA ĐẶC TẢ **CHƯA DỰNG ĐƯỢC** — mô tả sau dấu "?" (ADR 0068 §14)
 *
 * Mỗi phần được vẽ ĐÚNG CHỖ đặc tả đặt nó, đúng loại control, bị vô hiệu, kèm dấu "?"; bấm "?" là
 * đọc `ten` + `viSao` của mục ấy, nguyên văn. Vì thế `viSao` viết cho CÁN BỘ đọc: ngắn, không tên
 * tuyến, không tên bảng. Lý do kỹ thuật đầy đủ nằm ở chú thích ngay trên từng mục.
 *
 * `id` là khoá để màn lấy đúng mục (`petitionPendingPart`), không lấy theo vị trí trong mảng.
 *
 * Mục KHÔNG có chỗ giữ trên màn nhưng vẫn ở đây, vì `tools/tien_do_san_pham.py` đếm mảng này cho
 * báo cáo tiến độ: ghi nhận đánh giá thay người dân (ĐÃ QUYẾT KHÔNG LÀM, ADR 0062 — câu của mục nói
 * rõ điều ấy, nên báo cáo không đọc nó thành việc còn nợ), email cán bộ và số ngày quá hạn (chưa nằm
 * trong bảng vị trí đã duyệt, ADR 0068 §14).
 *
 * MỖI MỤC PHẢI ĐÚNG VÀO NGÀY NÓ CÒN Ở ĐÂY. Dựng xong phần nào thì xoá mục ấy trong cùng lượt.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type PhanChuaDung = {
  /** English key the screen looks the entry up by — see `petitionPendingPart`. */
  readonly id: string;
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  // §8.4 mini map + hamlet beside the address. The basemap question (rule 3, stop condition 2: a
  // citizen's coordinates must not go to an outside tile provider) is DECIDED — ADR 0072 §Sửa đổi
  // 09/10/2026: self-hosted PMTiles, served by the system itself. Still missing: the basemap file being
  // built and deployed. Until then no tile is loaded and no "open in map" link exists. The petition
  // response carries no hamlet (`petitions_phieuPhanAnhRa`, re-read 02/10/2026).
  {
    id: "sceneMap",
    ten: "Bản đồ hiện trường và tên thôn",
    viSao:
      "Đã chọn dùng nền bản đồ lưu ngay trên máy chủ của hệ thống, không gửi vị trí của người dân ra " +
      "ngoài. Bản đồ nhỏ đang chờ dựng tệp nền bản đồ nên chưa hiện. Phiếu cũng chưa mang tên thôn.",
  },
  // §9 heat map tab: same self-hosted basemap (ADR 0072 §Sửa đổi 09/10/2026), plus a route returning
  // EVERY located petition of the commune — the paged list holds one page only, and a heat map of one
  // page would show leadership a false density.
  {
    id: "heatMapTab",
    ten: "Bản đồ nhiệt",
    viSao:
      "Đã chọn dùng nền bản đồ lưu ngay trên máy chủ của hệ thống. Tab này đang chờ dựng tệp nền bản " +
      "đồ và phần lấy toàn bộ điểm phản ánh của xã, nên chưa mở được.",
  },
  // §10 report tab. The only counting route is `GET /api/v1/citizen-report-summary` (the four KPI
  // cards, §3); none counts by field, unit or hamlet (service-petitions routes, re-read 02/10/2026).
  // Counting the page in view would be the figure of ONE PAGE, not of the commune — and it is the
  // figure leadership reads and reports upward.
  {
    id: "reportTab",
    ten: "Báo cáo",
    viSao:
      "Hệ thống chưa đếm được phản ánh theo lĩnh vực, theo bộ phận hay theo thôn cho cả xã. Bốn thẻ " +
      "số liệu đầu màn là số của cả xã và đã dùng được.",
  },
  // The prototype's drawer section `Có thể trùng với phiếu khác` (FeedbackDetailDrawer.tsx:408-439)
  // and the card's "N phiếu trùng". No petitions route detects or merges duplicates
  // (service-petitions/internal/http/routes.go, re-read 06/10/2026), and the list carries no count.
  {
    id: "duplicates",
    ten: "Có thể trùng với phiếu khác",
    viSao:
      "Hệ thống chưa dò được phiếu trùng (cùng nơi, cùng sự việc) và chưa gộp được phiếu, nên khối " +
      "này chưa dùng được.",
  },
  // §4 scope tab. The petition list answers 400 to `scope=related`
  // (service-petitions/internal/http/xu_ly_phan_anh.go, errPhamViLienQuanChuaCo): who counts as
  // "related" and what a related officer may do is undecided with the customer.
  {
    id: "scopeRelated",
    ten: "Liên quan đến tôi",
    viSao:
      "Chưa chốt thế nào là phiếu “liên quan đến tôi” và người liên quan được làm gì với phiếu, nên " +
      "bộ lọc này chưa dùng được. “Toàn xã” và “Giao cho tôi” đã dùng được.",
  },
  {
    id: "staffRecordedRating",
    ten: "Biểu mẫu `Ghi nhận đánh giá của người dân` (§8.6)",
    viSao:
      "Không dựng theo quyết định của chủ dự án: cán bộ không ghi đánh giá thay người dân. Điểm " +
      "đánh giá chỉ đến từ chính người dân trên Mini App (ADR 0050 điểm 2), và màn này hiện điểm ấy " +
      "ở khối `Đánh giá của người dân`.",
  },
  {
    id: "staffEmail",
    ten: "Email của cán bộ trong ô `Đang giao cho` và ô chọn cán bộ (§8.3, §8.5)",
    viSao:
      "Danh bạ chọn người (`GET /api/v1/staff-directory`) cố ý chỉ trả mã, tên, chức vụ và bộ " +
      "phận — không email, không số điện thoại — để mọi cán bộ đăng nhập đọc được nó. Màn hình vì " +
      "thế hiện `Họ tên · Chức danh` thay cho `Họ tên — email · Chức danh`.",
  },
  // §11 hamlet select of the intake modal. The intake route answers 400 to `hamlet` / `thon_id`
  // (service-petitions/internal/http/staff_intake.go): no identity RPC checks that a hamlet an officer
  // picks is one of this commune's. The channel select is NOT a missing part: an intake is always
  // `can-bo-nhap-ho` (ADR 0028, Bổ sung 02/10/2026).
  {
    id: "intakeHamlet",
    ten: "Thôn, tổ dân phố",
    viSao:
      "Phiếu nhập hộ chưa ghi được thôn do cán bộ chọn: hệ thống chưa kiểm được thôn ấy có đúng là " +
      "thôn của xã hay không. Hãy ghi vị trí vào ô “Địa chỉ, vị trí”.",
  },
  // §11 scene photos of the intake modal. The intake body has no attachment, and the scene-photo
  // upload is bound to the CITIZEN's session (Mini App), not to an officer's.
  {
    id: "intakePhotos",
    ten: "Đính ảnh hiện trường",
    viSao:
      "Phiếu nhập hộ chưa đính được ảnh hiện trường: hiện chỉ người dân gửi ảnh được, từ Zalo Mini " +
      "App.",
  },
  // The composer's `Đính kèm ảnh, tệp` (FeedbackStatusPipeline.tsx:287-294). None of the six act routes
  // (classification, assignment, status, closure, rejection, referral) takes a file; files go with a
  // log entry only (`…/log-attachments`).
  {
    id: "composerAttachment",
    ten: "Đính kèm ảnh, tệp khi chuyển trạng thái",
    viSao:
      "Lúc chuyển trạng thái chưa đính kèm được ảnh hay tệp. Hãy đính kèm khi ghi nhật ký ở cột bên " +
      "phải, ngay sau khi chuyển.",
  },
  // The log row's `Nội bộ` pill (FeedbackActivityPanel.tsx:268-272). A log row carries no
  // "shown to the citizen" flag (`petitions_nhatKyPhieuRa`); every staff log row is internal today.
  {
    id: "logVisibility",
    ten: "Nội bộ",
    viSao:
      "Chưa đánh dấu được dòng nhật ký nào người dân xem được và dòng nào chỉ nội bộ. Hiện mọi dòng " +
      "nhật ký chỉ cán bộ trong xã xem được.",
  },
  // The log file row's remove button (FeedbackActivityPanel.tsx:365-374). No route removes a log
  // attachment: a file on a log entry is part of an archival record (rule 7).
  {
    id: "logFileRemoval",
    ten: "Gỡ tệp đính kèm",
    viSao:
      "Chưa gỡ được tệp đã đính kèm vào nhật ký. Tệp đã ghi vào nhật ký là một phần hồ sơ của phiếu.",
  },
  {
    id: "overdueDays",
    ten: "`⚠ Quá hạn 3 ngày` — số ngày trễ (§8.3, §7)",
    viSao:
      "Hạn đếm bằng **giờ làm việc** của chính xã, cần lịch làm việc, ngày nghỉ lễ và ngày làm bù " +
      "— ba bảng do `identity` sở hữu (ADR 0007). Đếm bằng giờ treo tường ở trình duyệt sẽ ra một " +
      "con số khác con số của máy chủ vào đúng dịp lễ. Màn hình vì thế nói `Quá hạn` kèm mốc hạn " +
      "cuối, không nói mấy ngày.",
  },
];

/**
 * The entry behind one "?" on this screen. THROWS on an unknown `id`: a renamed entry must turn the
 * screen's tests red, never open an empty description in front of an officer.
 */
export function petitionPendingPart(id: string): PhanChuaDung {
  const entry = PHAN_CHUA_DUNG.find((p) => p.id === id);
  if (entry === undefined) throw new Error(`nhan-phieu: PHAN_CHUA_DUNG has no entry "${id}"`);
  return entry;
}
