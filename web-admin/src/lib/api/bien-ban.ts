/**
 * Bốn tuyến của màn "Biên bản và kết luận họp" (`docs/ui-ux/04-bien-ban-hop.md`).
 *
 *   GET  /api/v1/meetings                        `task.read`    — §2 danh sách thẻ
 *   POST /api/v1/meetings                        `task.create`  — §4 modal Nhập biên bản
 *   POST /api/v1/meetings/{id}/conclusions       `task.create`  — §2 hàng thêm kết luận
 *   POST …/conclusions/{stt}/task                `task.create`  — §3 Tách thành nhiệm vụ
 *
 * Tuyến thứ tư ĐẾN 24/09/2026, sau ba tuyến kia. Khối này trước viết rằng nó "cố ý không có hàm
 * nào ở đây", vì nó dùng lại nguyên biểu mẫu "Giao việc mới" của `02-nhiem-vu.md` §7 và biểu mẫu
 * ấy khi đó đang được dựng ở `features/nhiem-vu/`. Biểu mẫu ấy nay đã có và ĐÃ XUẤT RA
 * (`features/nhiem-vu/so-nhiem-vu.tsx` → `FormGiaoViec`), nên chỗ này là một lần import chứ không
 * phải một bản thứ hai (luật 9, cấm #2). Phần §3 CÒN THIẾU — điền sẵn nội dung kết luận vào ô tiêu
 * đề — vẫn ở `PHAN_CHUA_DUNG` của `features/bien-ban/nhan-bien-ban.ts`, kèm lý do.
 *
 * ══════════════════════════════════════════════════════════════════════════════════════════
 * BẢN CHÉP TAY ĐÃ XOÁ 23/09/2026 — và khối này ở lại để nói vì sao nó từng tồn tại.
 *
 * Hai kiểu thân yêu cầu ở đây TỪNG là bản chép tay, vì `schema.gen.ts` lệch khỏi
 * `kb/20-contracts/openapi.json`: hợp đồng đã có `petitions.taoBienBanVao` và
 * `petitions.themKetLuanVao`, tệp sinh thì chưa — `npm run check:api` đỏ từ TRƯỚC lượt dựng màn
 * này, tức không phải nợ của người dựng màn. Lượt ấy có ba agent chạy song song nên không ai
 * được ghi vào `schema.gen.ts`; chạy bộ sinh giữa lúc agent đang viết là đo một cây đang động.
 *
 * `npm run gen:api` đã chạy, `check:api` đã xanh, và hai kiểu dưới nay là BÍ DANH của kiểu sinh
 * ra — một hình dạng, một nguồn.
 *
 * ĐIỀU ĐÁNG GIỮ LẠI: bản chép tay ấy không trôi được trong im lặng, vì `bien-ban.test.ts` ĐỌC
 * THẲNG `kb/20-contracts/openapi.json` và so từng tên trường thật sự đi trên dây với lược đồ
 * trong hợp đồng. Bài kiểm ấy KHÔNG bị xoá cùng bản chép tay: nó canh một thứ khác và vẫn còn
 * đúng — bí danh chỉ bảo đảm hai kiểu khớp nhau, còn nó bảo đảm thứ hàm này THẬT SỰ GỬI ĐI khớp
 * với hợp đồng.
 * ══════════════════════════════════════════════════════════════════════════════════════════
 */

import { docJSON, docThanKetQua, goiGhi, type KetQua } from "./goi";
import type {
  page_Result_petitions_bienBanRa,
  petitions_bienBanRa,
  petitions_get_meetings,
  petitions_ketLuanRa,
  petitions_nhiemVuRa,
  petitions_tachKetLuanVao,
  petitions_taoBienBanVao,
  petitions_themKetLuanVao,
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
 * Thân của `POST /api/v1/meetings` — §4. Bí danh của kiểu SINH RA từ hợp đồng, không phải một
 * hình dạng viết tay: `held_on` là NGÀY LỊCH `2026-08-05` chứ không phải một mốc thời gian (xem
 * `nhanNgayHop`), và `chaired_by` là MÃ NGHIỆP VỤ của cán bộ (`CB-2026-7K3M9Q`), không phải id
 * nội bộ, không phải họ tên.
 *
 * `chaired_by` KHÔNG TUỲ CHỌN TRONG HỢP ĐỒNG dù §4 để "Chủ trì" là trường tuỳ chọn: máy chủ khai
 * nó là `string` thường (không `omitempty`), nên khoá luôn có mặt trên dây và giá trị rỗng là
 * trạng thái bình thường — `domain.KiemChuTri` chỉ chặn độ dài, không chặn rỗng.
 *
 * KHÔNG CÓ `created_by` VÀ KHÔNG CÓ `attachments`, và cả hai là TỪ CHỐI chứ không phải bỏ sót:
 * người tạo là chủ thể của phiên (một yêu cầu tự khai tác giả là một yêu cầu giả được vết kiểm
 * toán), còn kho chưa có nơi lưu tệp nên một danh sách đính kèm trên dây là lời hứa không ai giữ.
 */
export type TaoBienBanVao = petitions_taoBienBanVao;

/** Thân của `POST …/conclusions`. */
export type ThemKetLuanVao = petitions_themKetLuanVao;

/**
 * Thân của `POST …/conclusions/{stt}/task` — §3. Bí danh của kiểu SINH RA từ hợp đồng.
 *
 * NÓ LÀ NGUYÊN BỘ TRƯỜNG CỦA BIỂU MẪU "Giao việc mới", TRỪ ĐÚNG HAI: `source` và `source_id`
 * KHÔNG có mặt, và sự vắng mặt ấy CHÍNH LÀ tính chất an ninh của tuyến. §3 vẽ "Nguồn giao = Từ
 * kết luận họp" là ô KHOÁ; máy chủ thể hiện điều đó bằng cách không nhận cặp ấy trên dây và tự
 * điền nó từ kết luận nêu trong ĐƯỜNG DẪN (`http/bien_ban_hop_ghi.go:137-143`). Một trường phải
 * đi kiểm là một trường có ngày ai đó quên kiểm — ở đây không có gì để quên, vì không có gì client
 * gửi lên mà trỏ được nhiệm vụ về một bản ghi nó tự chọn.
 */
export type TachKetLuanVao = petitions_tachKetLuanVao;

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
    docThanKetQua<petitions_bienBanRa>,
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
    docThanKetQua<petitions_ketLuanRa>,
  );
}

/**
 * POST /api/v1/meetings/{id}/conclusions/{stt}/task — §3. 201, trả về NHIỆM VỤ vừa lập, kèm số sổ
 * máy chủ vừa cấp (`NV12`) — thứ bên gọi không thể biết trước.
 *
 * ⚠ NHẬN CẢ DÒNG KẾT LUẬN, KHÔNG NHẬN MỘT CON SỐ, và đó là toàn bộ lý do hàm này có chữ ký như
 * vậy. `{stt}` trên đường dẫn phải là `ordinal` MÁY CHỦ TRẢ; §7.2 nối tiếp số ĐÃ CẤP, nên một biên
 * bản từng gỡ kết luận ② mang các số ①③④ và "vị trí thứ hai trong mảng" là số **3**, không phải 2.
 * Gửi vị trí thay cho số đã cấp thì lời gọi thành công, trả về 201, và lập một nhiệm vụ gắn vào
 * MỘT KẾT LUẬN KHÁC — giao cho người khác, về việc khác, trong một quyển sổ không xoá được.
 *
 * Một tham số `thuTu: number` sẽ nhận `viTri + 1` mà không có gì đỏ ở đâu. Nhận nguyên dòng kết
 * luận thì con số ấy đọc từ chính bản ghi máy chủ trả, và không còn chỗ nào để truyền một con số
 * khác vào.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG `...than`: nó là chỗ `source`/`source_id` bị chặn lại. Biểu mẫu dùng
 * chung khai kiểu `petitions_taoNhiemVuVao` — kiểu ấy CÓ hai trường ấy, và TypeScript cho gán sang
 * `TachKetLuanVao` vì nó chỉ thừa chứ không thiếu. Một phép trải ở đây đưa thẳng chúng lên dây.
 */
export function tachKetLuanThanhNhiemVu(
  bienBanID: string,
  ketLuan: petitions_ketLuanRa,
  than: TachKetLuanVao,
  khoaChongTrung: string,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const goc: petitions_get_meetings["duongDan"] = "/api/v1/meetings";
  const duongDan =
    `${goc}/${encodeURIComponent(bienBanID)}/conclusions/${String(ketLuan.ordinal)}/task`;

  const thanGui: TachKetLuanVao = {
    code: than.code,
    auto_code: than.auto_code,
    type: than.type,
    bloc: than.bloc,
    title: than.title,
    description: than.description,
    priority: than.priority,
    unit: than.unit,
    assignee: than.assignee,
    assigner: than.assigner,
    lead_unit: than.lead_unit,
    monitor: than.monitor,
    due_at: than.due_at,
    parent: than.parent,
  };

  return goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }).then(
    docThanKetQua<petitions_nhiemVuRa>,
  );
}
