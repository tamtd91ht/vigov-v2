/**
 * Sáu tuyến của màn "Nội dung Mini App" (`docs/ui-ux/11-noi-dung-mini-app.md`), đúng bộ tuyến
 * `service-comms/internal/http/routes.go` khai — không nhiều hơn, không ít hơn.
 *
 *   GET   /api/v1/content-items         content.read
 *   GET   /api/v1/content-items/{id}    content.read
 *   POST  /api/v1/content-items         content.update  + Idempotency-Key BẮT BUỘC
 *   PATCH /api/v1/content-items/{id}    content.update  (không cần khoá chống trùng)
 *   GET   /api/v1/content-categories    content.read
 *   POST  /api/v1/content-categories    content.update  + Idempotency-Key BẮT BUỘC
 *
 * KHÔNG CÓ `DELETE`, VÀ SỰ VẮNG MẶT ẤY LÀ MỘT CÂU TRẢ LỜI. §9 đề xuất một tuyến xoá; §6 chỉ vẽ
 * `✎`. Gỡ một bài khỏi Mini App là `PATCH` với `publish: false` — đúng như §7 tự mô tả ô tích của
 * nó. Không hàm nào ở đây dựng một đường `DELETE`: một hàm gọi vào tuyến không tồn tại là một hàm
 * biên dịch được, kiểm được bằng `fetch` giả, và 404 ở lần chạy thật.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `comms_noiDungRa`, `comms_themNoiDungVao`,
 * `comms_suaNoiDungVao`, `comms_danhMucRa`, `comms_danhSachDanhMucRa`, `comms_themDanhMucVao`,
 * `page_Result_comms_noiDungRa` đều đến từ `schema.gen.ts` (sinh từ `kb/20-contracts/openapi.json`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 * ⚠ HỢP ĐỒNG KHÔNG KHAI BA THAM SỐ LỌC CỦA §6, VÀ MÁY CHỦ THÌ ĐỌC ĐỦ CẢ BA.
 *
 * `comms_get_content_items["truyVan"]` sinh ra đúng bốn khoá phân trang (`limit` · `cursor` ·
 * `sort` · `order`), trong khi handler đọc thêm `type` · `category` · `q`
 * (`service-comms/internal/http/noi_dung_mini_app.go:296-317`). Nguyên nhân đã đo được và nằm ở
 * bộ sinh, không ở máy chủ: `tools/apidoc/truyvan.go` đọc được tên tham số ở một CLOSURE CỤC BỘ
 * (`lay("hamlet")`) nhưng không đọc được ở một HÀM CẤP GÓI — mà `thamSoLoc(q, "type")` chính là
 * hàm cấp gói ấy, và nó được đưa vào để tránh hai hook báo động nhầm (:250-266). Cái giá không ai
 * thấy lúc viết là ba tham số biến mất khỏi hợp đồng.
 *
 * Hệ quả nói thẳng chứ không giấu: **ba tên ấy không có kiểu nào của hợp đồng canh giúp**. Gõ
 * `loai` thay `type` thì `tsc` im lặng, máy chủ bỏ qua trong im lặng và trả cả quyển sổ trong khi
 * cán bộ tin mình đang xem một lát cắt. Vì thế ba cái tên nằm trong ĐÚNG MỘT hàm
 * (`themLocVaoTruyVan`) và `noi-dung.test.ts` đọc THẲNG tệp Go để so lại từng tên. Ngày hợp đồng
 * khai đủ `parameters`, đây là chỗ duy nhất phải sửa. Đã báo về.
 * ─────────────────────────────────────────────────────────────────────────────────────────────
 *
 * BỐN ĐIỀU MÁY CHỦ ĐÃ QUYẾT, VÀ MÔ-ĐUN NÀY KHÔNG VẼ KHÁC:
 *
 *   1. `body` VẮNG ở danh sách, CÓ ở chi tiết. Hợp đồng sinh ra `body?: string | null` đúng vì
 *      thế: `""` sẽ mang hai nghĩa — "bài này không có thân" và "bạn vừa hỏi một TRANG, mà trang
 *      thì không mang thân". Muốn toàn văn thì gọi `layMotNoiDung`, không suy từ danh sách.
 *   2. Không có `DELETE` — xem trên.
 *   3. `status` DO MÁY CHỦ QUYẾT từ ô tích `publish` của §7. Không thân nào ở đây mang `status`:
 *      một client tự khai trạng thái là một client đăng vượt qua bước duyệt mà §10.2 dành cho
 *      lượt đồng bộ.
 *   4. `view_count` chỉ để HIỆN. Không đường ghi nào ở đây đụng tới nó — thứ duy nhất được phép
 *      tăng nó là một cư dân mở bài, và tuyến công khai của §9 chưa dựng.
 *
 * KHÔNG CÓ `tenant_id` Ở BẤT KỲ ĐÂU — không thân, không query, không header. Xã suy từ `Host` ở
 * rìa ngoài cùng; client tự khai xã là client tự cấp quyền (luật 1, cấm #2).
 *
 * KHÔNG GHI LOG GÌ: `title`, `summary` và `body` là tin bài của xã, thường xuyên nhắc tên và hoàn
 * cảnh của một công dân cụ thể ("Trao quà cho gia đình ông …"). Chúng không được vào log, vào tên
 * tệp, vào URL hay vào khoá đệm (luật 3, cấm #1 và #4).
 */

import { docJSON, docThanKetQua, goiGhi, type KetQua } from "./goi";
import type {
  comms_danhMucRa,
  comms_danhSachDanhMucRa,
  comms_get_content_categories,
  comms_get_content_items,
  comms_get_content_items_by_id,
  comms_noiDungRa,
  comms_suaNoiDungVao,
  comms_themDanhMucVao,
  comms_themNoiDungVao,
  page_Result_comms_noiDungRa,
} from "./schema.gen";

/* ── Bộ lọc §6 ─────────────────────────────────────────────────────────────────────────────── */

/**
 * Ba bộ lọc của §6 cộng phân trang.
 *
 * TÊN THAM SỐ TRÊN DÂY LÀ TIẾNG ANH (`type` · `category` · `q`), không phải `loai`/`danh_muc` như
 * §9 phác: bề mặt hợp đồng dùng tiếng Anh (ADR 0011), và handler đọc đúng ba tên ấy.
 */
export type BoLocNoiDung = {
  /** Một trong sáu mã của §5. Rỗng/không có nghĩa là mọi loại. Mã lạ bị máy chủ trả 400. */
  loai?: string;
  /** ULID của danh mục. Rỗng nghĩa là `Tất cả danh mục`. */
  danhMucID?: string;
  /** Ô `🔍 Tìm theo tiêu đề…`. */
  tim?: string;
  limit?: number;
  /** `null` là trang đầu — xem `features/cau-hinh/ngan-xep-con-tro.ts`. */
  cursor?: string | null;
};

/**
 * Trần độ dài từ khoá tìm, đúng bằng trần máy chủ (`commsstore.TuKhoaTimToiDa`).
 *
 * Chép ở đây CHỈ để ô nhập dừng lại đúng chỗ máy chủ sẽ dừng — máy chủ vẫn là nơi từ chối thật,
 * và nó từ chối bằng một câu CỐ Ý không nhắc lại chữ người ta vừa gõ.
 */
export const TU_KHOA_TIM_TOI_DA = 200;

/**
 * BA CÁI TÊN CỦA §6 NẰM Ở ĐÚNG MỘT CHỖ — đây.
 *
 * Xem khối ⚠ ở đầu tệp: hợp đồng không khai chúng, nên không có kiểu nào canh. Gom vào một hàm để
 * ngày hợp đồng khai đủ thì chỉ phải sửa một chỗ, và để bài kiểm có đúng một nơi để đọc lại.
 */
function themLocVaoTruyVan(truyVan: URLSearchParams, loc: BoLocNoiDung): void {
  if (loc.loai !== undefined && loc.loai !== "") truyVan.set("type", loc.loai);
  if (loc.danhMucID !== undefined && loc.danhMucID !== "") truyVan.set("category", loc.danhMucID);
  if (loc.tim !== undefined && loc.tim !== "") truyVan.set("q", loc.tim);

  if (loc.limit !== undefined) truyVan.set("limit", String(loc.limit));
  // Con trỏ rỗng nghĩa là trang đầu. Gửi `cursor=` rỗng thì `page.Parse` trả 400 "con trỏ không
  // hợp lệ", nên trang đầu phải VẮNG tham số chứ không mang một tham số rỗng.
  if (loc.cursor !== undefined && loc.cursor !== null && loc.cursor !== "") {
    truyVan.set("cursor", loc.cursor);
  }
}

/**
 * Dựng đường dẫn sổ nội dung. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`.
 *
 * KHÔNG GỬI `sort` VÀ `order`: máy chủ chỉ cho `created_at` và mặc định đã là giảm dần. Gửi lại
 * đúng giá trị mặc định chỉ thêm một chỗ có thể lệch.
 */
export function duongDanSoNoiDung(loc: BoLocNoiDung): string {
  const duongDan: comms_get_content_items["duongDan"] = "/api/v1/content-items";
  const truyVan = new URLSearchParams();
  themLocVaoTruyVan(truyVan, loc);
  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/**
 * GET /api/v1/content-items — một trang của bảng §6.
 *
 * ⚠ CÁC HÀNG TRẢ VỀ KHÔNG MANG `body`. Đó là quyết định của máy chủ, không phải một thiếu sót:
 * một trăm bài ở trần thân bài là hai mươi triệu ký tự trong một phản hồi, còn §6 chỉ cần tiêu đề
 * và một dòng tóm tắt. Muốn toàn văn thì `layMotNoiDung`.
 */
export function laySoNoiDung(loc: BoLocNoiDung): Promise<KetQua<page_Result_comms_noiDungRa>> {
  return docJSON<page_Result_comms_noiDungRa>(duongDanSoNoiDung(loc));
}

/**
 * Đường dẫn một mục nội dung.
 *
 * `encodeURIComponent` CHỨ KHÔNG GHÉP THẲNG: id đến từ dữ liệu máy chủ trả, nhưng một id đi thẳng
 * vào đường dẫn là hình dạng mà ngày nào đó có người truyền vào một chuỗi do người dùng gõ.
 */
export function duongDanMotNoiDung(id: string): string {
  const mau: comms_get_content_items_by_id["duongDan"] = "/api/v1/content-items/{id}";
  return mau.replace("{id}", encodeURIComponent(id));
}

/** GET /api/v1/content-items/{id} — một mục KÈM toàn văn, cho modal sửa §7. */
export function layMotNoiDung(id: string): Promise<KetQua<comms_noiDungRa>> {
  return docJSON<comms_noiDungRa>(duongDanMotNoiDung(id));
}

/* ── Ghi ───────────────────────────────────────────────────────────────────────────────────── */

/**
 * Thân của `POST /api/v1/content-items` — §7, trường theo trường. Bí danh của kiểu SINH RA.
 *
 * KHÔNG CÓ `status`, `source`, `source_ref` HAY `author_code`, và cả bốn là TỪ CHỐI CỦA MÁY CHỦ
 * chứ không phải bỏ sót. Trạng thái suy từ ô tích `publish`; nguồn gốc quyết định lượt đồng bộ sau
 * được làm gì với hàng (§10.4); tác giả là chủ thể của phiên (luật 6, bất biến 8). Một trường ở
 * đây cho bất kỳ thứ nào trong bốn là một client tự quyết một điều không thuộc về nó.
 *
 * KHÔNG CÓ `published_on`: §7 không có ô ngày. Bài soạn tay mang ngày hôm nay, do máy chủ đặt.
 */
export type ThemNoiDungVao = comms_themNoiDungVao;

/**
 * POST /api/v1/content-items — soạn một mục nội dung. 201, trả về cả hàng KÈM thân bài.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ. Hợp đồng đòi `Idempotency-Key` bắt buộc. Sinh
 * khoá bên trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới — tức đúng cái khoá
 * chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ đã tới máy chủ và đã đăng một bài lên Mini
 * App của cả xã. Khoá do biểu mẫu giữ, sống bằng đời một lần mở form.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG `...than`: một phép trải ở đây là đường để một trường lạ đi lên máy chủ
 * vào ngày ai đó truyền vào một đối tượng vừa đọc được từ nơi khác — mà hàng đọc về CÓ mang
 * `status`, `source` và `view_count`.
 */
export function themNoiDung(
  than: ThemNoiDungVao,
  khoaChongTrung: string,
): Promise<KetQua<comms_noiDungRa>> {
  const duongDan: comms_get_content_items["duongDan"] = "/api/v1/content-items";

  const thanGui: ThemNoiDungVao = {
    type: than.type,
    title: than.title,
    category_id: than.category_id,
    summary: than.summary,
    body: than.body,
    image_url: than.image_url,
    publish: than.publish,
  };

  return goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }).then(
    docThanKetQua<comms_noiDungRa>,
  );
}

/**
 * Thân của `PATCH /api/v1/content-items/{id}` — §6 `✎` mở lại modal §7. Bí danh của kiểu SINH RA.
 *
 * ⚠ MỖI TRƯỜNG LÀ MỘT CON TRỎ Ở MÁY CHỦ, VÀ ĐÓ LÀ TOÀN BỘ LÝ DO ĐÂY LÀ `PATCH` CHỨ KHÔNG PHẢI
 * `PUT`. Mọi trường đều có một giá trị rỗng CÓ NGHĨA: `""` ở tóm tắt là "không có tóm tắt", `""`
 * ở danh mục là `— Chưa xếp danh mục —`, và `publish: false` là "gỡ khỏi Mini App". Một trường
 * VẮNG nghĩa là "để nguyên". Nên một trường `undefined` ở đây phải đi khỏi thân — `JSON.stringify`
 * làm đúng việc ấy — còn gửi `null` là gửi "để nguyên" bằng một cách khác, không phải "xoá".
 */
export type SuaNoiDungVao = comms_suaNoiDungVao;

/**
 * PATCH /api/v1/content-items/{id} — sửa một phần. 200, trả về hàng SAU khi sửa.
 *
 * KHÔNG MANG `Idempotency-Key`: hợp đồng không đòi, và một `PATCH` đặt lại đúng những giá trị ấy
 * hai lần cho ra cùng một hàng.
 *
 * PHẢN HỒI MANG `hand_edited`, VÀ ĐÓ LÀ NỬA NHÌN THẤY ĐƯỢC CỦA §10.4: cán bộ vừa sửa một bài đồng
 * bộ từ Cổng cần thấy rằng lần sửa ấy nay được bảo vệ khỏi lượt đồng bộ sau. Đây là chỗ duy nhất
 * màn hình biết được điều đó.
 */
export function suaNoiDung(id: string, than: SuaNoiDungVao): Promise<KetQua<comms_noiDungRa>> {
  const thanGui: SuaNoiDungVao = {
    type: than.type,
    category_id: than.category_id,
    title: than.title,
    summary: than.summary,
    body: than.body,
    image_url: than.image_url,
    publish: than.publish,
  };

  return goiGhi(duongDanMotNoiDung(id), "PATCH", thanGui, 200, undefined).then(
    docThanKetQua<comms_noiDungRa>,
  );
}

/* ── Danh mục tin §6 ───────────────────────────────────────────────────────────────────────── */

/**
 * GET /api/v1/content-categories — cả cây danh mục của xã, PHẲNG.
 *
 * KHÔNG PHÂN TRANG, có chủ ý ở máy chủ: §7 vẽ một ô chọn và §3 vẽ một danh sách thụt đầu dòng, cả
 * hai cần trọn cây. Trần thay cho `limit` là `store.TranDanhMucMiniApp`.
 *
 * `parent_id` RỖNG LÀ GỐC. Cây trả về phẳng vì hai màn muốn hai hình dạng khác nhau của cùng một
 * bộ hàng — dựng cây là việc của bên hiển thị, không phải của tuyến.
 */
export function layDanhMucNoiDung(): Promise<KetQua<comms_danhSachDanhMucRa>> {
  const duongDan: comms_get_content_categories["duongDan"] = "/api/v1/content-categories";
  return docJSON<comms_danhSachDanhMucRa>(duongDan);
}

/** Thân của `POST /api/v1/content-categories`. Bí danh của kiểu SINH RA. */
export type ThemDanhMucVao = comms_themDanhMucVao;

/**
 * POST /api/v1/content-categories — thêm một danh mục. 201.
 *
 * `Idempotency-Key` BẮT BUỘC theo hợp đồng, cùng khuôn với `themNoiDung`: khoá do biểu mẫu giữ,
 * chỉ đổi khi đã ghi xong.
 *
 * MÁY CHỦ TRẢ 409 KHI SLUG ĐÃ DÙNG — KỂ CẢ KHI DANH MỤC MANG SLUG ẤY ĐÃ BỊ XOÁ. Câu 409 ấy nói rõ
 * lý do ("mã đã cấp thì không cấp lại"), và câu ấy đi thẳng ra màn hình chứ không được viết lại ở
 * đây: viết lại là dựng bản sao thứ hai của một quy tắc nghiệp vụ.
 */
export function themDanhMucNoiDung(
  than: ThemDanhMucVao,
  khoaChongTrung: string,
): Promise<KetQua<comms_danhMucRa>> {
  const duongDan: comms_get_content_categories["duongDan"] = "/api/v1/content-categories";

  const thanGui: ThemDanhMucVao = {
    name: than.name,
    slug: than.slug,
    parent_id: than.parent_id,
    order: than.order,
  };

  return goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }).then(
    docThanKetQua<comms_danhMucRa>,
  );
}
