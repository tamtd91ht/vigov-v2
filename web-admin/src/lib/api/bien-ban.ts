/**
 * Ba tuyến của màn "Biên bản và kết luận họp" (`docs/ui-ux/04-bien-ban-hop.md`).
 *
 *   GET  /api/v1/meetings                        `task.read`    — §2 danh sách thẻ
 *   POST /api/v1/meetings                        `task.create`  — §4 modal Nhập biên bản
 *   POST /api/v1/meetings/{id}/conclusions       `task.create`  — §2 hàng thêm kết luận
 *
 * Tuyến thứ tư của chương — `POST …/conclusions/{stt}/task` (§3 "Tách thành nhiệm vụ") — CỐ Ý
 * không có hàm nào ở đây. Nó dùng lại nguyên biểu mẫu "Giao việc mới" của `02-nhiem-vu.md` §7,
 * biểu mẫu ấy đang được dựng ở `features/nhiem-vu/`, và một bản thứ hai của nó là một bản sao sẽ
 * trôi (luật 9, cấm #2). Xem `PHAN_CHUA_DUNG` trong `features/bien-ban/nhan-bien-ban.ts`.
 *
 * ══════════════════════════════════════════════════════════════════════════════════════════
 * HAI KIỂU THÂN YÊU CẦU Ở TỆP NÀY LÀ BẢN CHÉP TAY TẠM THỜI, VÀ ĐÓ LÀ MỘT KHIẾM KHUYẾT ĐÃ BÁO
 * LÊN chứ không phải một lựa chọn thiết kế.
 *
 * `src/lib/api/schema.gen.ts` ĐANG LỆCH khỏi `kb/20-contracts/openapi.json`: hợp đồng CÓ
 * `petitions.taoBienBanVao` và `petitions.themKetLuanVao`, tệp sinh thì KHÔNG — nó mới chỉ có
 * `petitions_bienBanRa`, `petitions_ketLuanRa` và tuyến GET. `npm run check:api` đỏ từ trước
 * lượt làm này, tức `make web` cũng đang đỏ vì đúng lý do ấy.
 *
 * Sửa đúng là MỘT LỆNH: `npm run gen:api`. Lượt này không được ghi vào `schema.gen.ts` (ba agent
 * chạy song song), nên hai kiểu dưới đây đứng tạm ở đây. NGÀY LỆNH ẤY CHẠY THÌ XOÁ CHÚNG và
 * `import type { petitions_taoBienBanVao, petitions_themKetLuanVao }` — không phải "để đó cho
 * gọn": một hình dạng hợp đồng chép tay là hình dạng sẽ trôi mà không bài kiểm nào đỏ.
 *
 * ĐỂ SỰ TRÔI ẤY KHÔNG IM LẶNG, `bien-ban.test.ts` ĐỌC THẲNG `kb/20-contracts/openapi.json` và
 * so từng tên trường của hai kiểu này với lược đồ trong hợp đồng. Hợp đồng đổi một trường thì
 * bài kiểm ấy đỏ ngay, thay vì đợi tới lúc máy chủ trả 400 trên máy của một xã.
 * ══════════════════════════════════════════════════════════════════════════════════════════
 */

import { docJSON, goiGhi, LOI_KHONG_RO, type KetQua } from "./goi";
import type {
  page_Result_petitions_bienBanRa,
  petitions_bienBanRa,
  petitions_get_meetings,
  petitions_ketLuanRa,
} from "./schema.gen";

/**
 * Phân trang của quyển sổ. Tuyến KHÔNG nhận bộ lọc nào — §2 vẽ một danh sách dọc, không có ô
 * tìm, không có tab, không có bộ lọc — nên ở đây cũng không có chỗ nào để truyền một bộ lọc vào.
 *
 * KHÔNG CÓ `sort`: máy chủ chỉ cho `created_at` và mặc định đã là giảm dần
 * (`service-petitions/internal/store/bien_ban_hop.go:68`), tức "mới nhất ở trên" của §2. Gửi
 * lại đúng giá trị mặc định chỉ thêm một chỗ có thể lệch.
 */
export type TrangBienBan = {
  limit?: number;
  /** `null` là trang đầu — xem `features/cau-hinh/ngan-xep-con-tro.ts`. */
  cursor?: string | null;
};

/**
 * Dựng đường dẫn danh sách. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`.
 *
 * KHÔNG CÓ `tenant_id` Ở BẤT KỲ ĐÂU, và không được thêm: xã suy từ `Host` ở rìa ngoài cùng, còn
 * client tự khai xã là client tự cấp quyền (luật 1, cấm #2).
 */
export function duongDanSoBienBan(trang: TrangBienBan): string {
  const duongDan: petitions_get_meetings["duongDan"] = "/api/v1/meetings";
  const truyVan = new URLSearchParams();

  if (trang.limit !== undefined) truyVan.set("limit", String(trang.limit));
  // Con trỏ rỗng nghĩa là trang đầu. Gửi `cursor=` rỗng thì `page.Parse` trả 400 "con trỏ không
  // hợp lệ", nên trang đầu phải VẮNG tham số chứ không mang một tham số rỗng.
  if (trang.cursor !== undefined && trang.cursor !== null && trang.cursor !== "") {
    truyVan.set("cursor", trang.cursor);
  }

  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/** GET /api/v1/meetings — một trang biên bản, mỗi biên bản kèm kết luận và bộ đếm nhiệm vụ. */
export function laySoBienBan(
  trang: TrangBienBan,
): Promise<KetQua<page_Result_petitions_bienBanRa>> {
  return docJSON<page_Result_petitions_bienBanRa>(duongDanSoBienBan(trang));
}

/**
 * Thân của `POST /api/v1/meetings` — §4. **Bản chép tay tạm thời**, xem khối đầu tệp.
 *
 * `chaired_by` KHÔNG TUỲ CHỌN TRONG HỢP ĐỒNG dù §4 để "Chủ trì" là trường tuỳ chọn: máy chủ khai
 * nó là `string` thường (không `omitempty`), nên khoá luôn có mặt trên dây và giá trị rỗng là
 * trạng thái bình thường — `domain.KiemChuTri` chỉ chặn độ dài, không chặn rỗng.
 *
 * KHÔNG CÓ `created_by` VÀ KHÔNG CÓ `attachments`, và cả hai là TỪ CHỐI chứ không phải bỏ sót:
 * người tạo là chủ thể của phiên (một yêu cầu tự khai tác giả là một yêu cầu giả được vết kiểm
 * toán), còn kho chưa có nơi lưu tệp nên một danh sách đính kèm trên dây là lời hứa không ai giữ.
 */
export type TaoBienBanVao = {
  title: string;
  /** NGÀY LỊCH `2026-08-05`, không phải một mốc thời gian. Xem `nhanNgayHop`. */
  held_on: string;
  reference_no?: string;
  location?: string;
  /** MÃ NGHIỆP VỤ của cán bộ (`CB-2026-7K3M9Q`), không phải id nội bộ, không phải họ tên. */
  chaired_by: string;
  content?: string;
  attendees?: string[];
  conclusions?: string[];
};

/** Thân của `POST …/conclusions`. **Bản chép tay tạm thời**, xem khối đầu tệp. */
export type ThemKetLuanVao = {
  content: string;
};

/**
 * Đọc thân của một lần ghi thành kiểu của tuyến.
 *
 * `goiGhi` trả `Response` thô chứ không phân giải sẵn, nên lớp mỏng này tồn tại — cùng lý do
 * `goiGhiCanBo` tồn tại trong `can-bo.ts`: có tuyến thành công bằng 204 không thân, và một hàm
 * dùng chung luôn gọi `.json()` sẽ biến lần ấy thành "không đọc được".
 *
 * KHÔNG GHI LOG GÌ KHI THÂN HỎNG: nội dung biên bản và nội dung kết luận là chữ của một cuộc
 * họp có thể nhắc tới hồ sơ của công dân (luật 3, cấm #1).
 */
async function docThanGhi<T>(ketQua: KetQua<Response>): Promise<KetQua<T>> {
  if (!ketQua.ok) return ketQua;
  try {
    return { ok: true, duLieu: (await ketQua.duLieu.json()) as T };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/**
 * POST /api/v1/meetings — nhập một biên bản kèm các kết luận đã gõ trên biểu mẫu. 201, trả về cả
 * tấm thẻ.
 *
 * TRẢ VỀ CẢ THẺ VÌ ID CỦA TỪNG KẾT LUẬN LÀ THỨ CLIENT KHÔNG THỂ BIẾT TRƯỚC — luồng tách nhiệm vụ
 * của §3 cần đúng những id ấy.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ. Hợp đồng đòi `Idempotency-Key` bắt buộc. Sinh
 * khoá bên trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới — tức đúng cái khoá
 * chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ đã tới máy chủ và đã ghi một biên bản vào sổ
 * lưu trữ. Khoá do biểu mẫu giữ, sống bằng đời một lần mở form.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG `...than`: một phép trải ở đây là đường để một trường lạ đi lên máy chủ
 * vào ngày ai đó truyền vào một đối tượng vừa đọc được từ nơi khác.
 */
export function taoBienBan(
  than: TaoBienBanVao,
  khoaChongTrung: string,
): Promise<KetQua<petitions_bienBanRa>> {
  const duongDan: petitions_get_meetings["duongDan"] = "/api/v1/meetings";

  const thanGui: TaoBienBanVao = {
    title: than.title,
    held_on: than.held_on,
    reference_no: than.reference_no,
    location: than.location,
    chaired_by: than.chaired_by,
    content: than.content,
    attendees: than.attendees,
    conclusions: than.conclusions,
  };

  return goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }).then(
    docThanGhi<petitions_bienBanRa>,
  );
}

/**
 * POST /api/v1/meetings/{id}/conclusions — thêm một kết luận vào biên bản đã có. 201, trả về
 * đúng kết luận vừa thêm.
 *
 * THỨ CLIENT KHÔNG THỂ BIẾT TRƯỚC LÀ `ordinal`: §7.2 nối tiếp số ĐÃ CẤP, lấy từ mốc cao nhất
 * dưới khoá của biên bản, nên một client đoán `số dòng đang thấy + 1` sẽ sai ở mọi biên bản từng
 * có một kết luận bị gỡ (luật 7 — số đã cấp không cấp lại). Màn hình vì thế VẼ LẠI theo con số
 * máy chủ trả, không tự đánh số.
 *
 * `id` mã hoá vào đường dẫn thay vì ghép thẳng.
 */
export function themKetLuan(
  bienBanID: string,
  than: ThemKetLuanVao,
  khoaChongTrung: string,
): Promise<KetQua<petitions_ketLuanRa>> {
  const goc: petitions_get_meetings["duongDan"] = "/api/v1/meetings";
  const duongDan = `${goc}/${encodeURIComponent(bienBanID)}/conclusions`;

  const thanGui: ThemKetLuanVao = { content: than.content };

  return goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }).then(
    docThanGhi<petitions_ketLuanRa>,
  );
}
