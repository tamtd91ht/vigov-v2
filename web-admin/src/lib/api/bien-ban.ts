/**
 * Các tuyến của màn "Biên bản và kết luận họp" (`docs/ui-ux/04-bien-ban-hop.md`).
 *
 *   GET    /api/v1/meetings                          `task.read`    — §2 danh sách thẻ
 *   GET    /api/v1/meetings/{id}                     `task.read`    — Xem biên bản (toàn văn)
 *   POST   /api/v1/meetings                          `task.create`  — §4 Nhập biên bản / bổ sung
 *   PATCH  /api/v1/meetings/{id}                     `task.create`  — sửa nháp · ghi Thông báo
 *   DELETE /api/v1/meetings/{id}                     `task.create`  — gỡ nháp, kèm lý do
 *   POST   /api/v1/meetings/{id}/signature           `task.approve` — ký biên bản
 *   POST   /api/v1/meetings/{id}/conclusions         `task.create`  — §2 hàng thêm kết luận
 *   PATCH  …/conclusions/{stt}                       `task.create`  — sửa lời kết luận
 *   DELETE …/conclusions/{stt}                       `task.create`  — gỡ kết luận, kèm lý do
 *   PUT    …/conclusions/{stt}/no-task-marker        `task.create`  — đánh dấu "không phát sinh"
 *   DELETE …/conclusions/{stt}/no-task-marker        `task.create`  — bỏ dấu
 *   GET    …/conclusions/{stt}/tasks                 `task.read`    — §3 nhiệm vụ đã tách
 *   POST   …/conclusions/{stt}/task                  `task.create`  — §3 Tách thành nhiệm vụ
 *
 * MỌI TUYẾN CÓ `{stt}` NHẬN CẢ DÒNG KẾT LUẬN, KHÔNG NHẬN MỘT CON SỐ — lý do ở
 * `tachKetLuanThanhNhiemVu`, và nó đúng cho sửa/gỡ/đánh dấu y như cho tách: gửi vị trí trong mảng
 * thay cho số đã cấp là sửa, gỡ hay khoá MỘT KẾT LUẬN KHÁC.
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
  petitions_get_meetings_by_id,
  petitions_ketLuanRa,
  petitions_kyBienBanVao,
  petitions_nhiemVuKetLuanRa,
  petitions_nhiemVuRa,
  petitions_suaBienBanVao,
  petitions_suaKetLuanVao,
  petitions_tachKetLuanVao,
  petitions_taoBienBanVao,
  petitions_themKetLuanVao,
  petitions_thongBaoVao,
  petitions_xoaBienBanVao,
} from "./schema.gen";

/**
 * Phân trang của quyển sổ. Tuyến KHÔNG nhận bộ lọc nào — §2 vẽ một danh sách dọc, không có ô
 * tìm, không có tab, không có bộ lọc — nên ở đây cũng không có chỗ nào để truyền một bộ lọc vào.
 *
 * KHÔNG CÓ `sort`/`order`: hợp đồng khai mặc định `sort=held_on`, `order=desc` — NGÀY HỌP mới nhất
 * ở trên, đúng "mới nhất ở trên" của §2. Màn vẽ đúng thứ tự máy chủ trả, không sắp lại; gửi lại
 * đúng giá trị mặc định chỉ thêm một chỗ có thể lệch.
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
    // Thư ký: MÃ cán bộ, như `chaired_by`. Vắng khi không ghi (`omitempty` ở máy chủ).
    minutes_taker: than.minutes_taker,
    // Biên bản ĐÃ KÝ mà biên bản này bổ sung. Máy chủ trả 404 khi id lạ, 400 khi nó còn nháp —
    // client không kiểm trước điều ấy: trạng thái thẻ đang vẽ có thể đã cũ.
    supplements_id: than.supplements_id,
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

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * VÒNG ĐỜI BIÊN BẢN — dự thảo → đã ký (migration 0012, quyết định người dùng 25/09/2026)
 *
 * Mọi câu từ chối 400/409 của các tuyến dưới là câu viết cho cán bộ đọc ("biên bản họp đã ký —
 * … lập biên bản bổ sung"), nên màn hình hiện NGUYÊN VĂN chúng — không dựng lại một quy tắc nào ở
 * client. Các điều kiện ẩn/tắt nút ở `features/bien-ban/nhan-bien-ban.ts` chỉ để cán bộ khỏi bấm
 * vào một thứ chắc chắn bị từ chối; máy chủ vẫn là nơi từ chối thật.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

const GOC_BIEN_BAN: petitions_get_meetings["duongDan"] = "/api/v1/meetings";

/** `/api/v1/meetings/{id}` — id mã hoá vào đường dẫn thay vì ghép thẳng. */
function duongDanMotBienBan(bienBanID: string): string {
  return `${GOC_BIEN_BAN}/${encodeURIComponent(bienBanID)}`;
}

/** `…/conclusions/{stt}` — `{stt}` là `ordinal` ĐỌC TỪ BẢN GHI, không phải vị trí trong mảng. */
function duongDanKetLuan(bienBanID: string, ketLuan: petitions_ketLuanRa): string {
  return `${duongDanMotBienBan(bienBanID)}/conclusions/${String(ketLuan.ordinal)}`;
}

/**
 * GET /api/v1/meetings/{id} — một biên bản ĐẦY ĐỦ: toàn văn, thành phần, thư ký, người ký, Thông
 * báo kết luận, và hai chiều bổ sung (`supplements_id`, `supplemented_by`). Danh sách cố ý không trả
 * `content`/`attendees`, nên "Xem biên bản" và "Sửa" đều đọc qua đây.
 */
export function layBienBan(bienBanID: string): Promise<KetQua<petitions_bienBanRa>> {
  // Khuôn lấy TỪ HỢP ĐỒNG: máy chủ đổi đường dẫn thì `tsc` đỏ ở đây.
  const khuon: petitions_get_meetings_by_id["duongDan"] = "/api/v1/meetings/{id}";
  return docJSON<petitions_bienBanRa>(khuon.replace("{id}", encodeURIComponent(bienBanID)));
}

/** Thân `PATCH /api/v1/meetings/{id}` — bí danh của kiểu sinh ra. */
export type SuaBienBanVao = petitions_suaBienBanVao;

/** Cặp số/ngày Thông báo kết luận — bí danh của kiểu sinh ra. */
export type ThongBaoVao = petitions_thongBaoVao;

/** Thân `POST …/signature` — bí danh của kiểu sinh ra. */
export type KyBienBanVao = petitions_kyBienBanVao;

/**
 * Chép một cặp Thông báo TỪNG TRƯỜNG. `null`/vắng thì vắng: "không ghi Thông báo" là trạng thái
 * bình thường, không phải một cặp chuỗi rỗng (máy chủ đọc cặp rỗng là THIẾU số, 400).
 */
function chepThongBao(tb: ThongBaoVao | null | undefined): ThongBaoVao | undefined {
  if (tb === null || tb === undefined) return undefined;
  return { reference_no: tb.reference_no, issued_on: tb.issued_on };
}

/**
 * PATCH /api/v1/meetings/{id} — 200, trả lại cả tấm thẻ.
 *
 * TRƯỜNG VẮNG LÀ "KHÔNG ĐỔI", TRƯỜNG CÓ MẶT LÀ "ĐẶT THÀNH" — kể cả chuỗi rỗng (xoá địa điểm). Vì
 * thế biểu mẫu chỉ đưa vào đây những trường cán bộ ĐÃ ĐỔI (`thanSuaTuBieuMau`), và hàm này dựng
 * từng trường, không `...than`.
 *
 * Nháp: sửa được mọi trường. Đã ký: CHỈ `notice`, và chỉ một lần — mọi thứ khác là 409 kèm câu nói
 * cách làm đúng (lập biên bản bổ sung).
 */
export function suaBienBan(
  bienBanID: string,
  than: SuaBienBanVao,
): Promise<KetQua<petitions_bienBanRa>> {
  const thanGui: SuaBienBanVao = {
    title: than.title,
    held_on: than.held_on,
    reference_no: than.reference_no,
    location: than.location,
    chaired_by: than.chaired_by,
    minutes_taker: than.minutes_taker,
    attendees: than.attendees,
    content: than.content,
    notice: chepThongBao(than.notice),
  };
  return goiGhi(duongDanMotBienBan(bienBanID), "PATCH", thanGui, 200).then(
    docThanKetQua<petitions_bienBanRa>,
  );
}

/**
 * DELETE /api/v1/meetings/{id} — XOÁ MỀM kèm lý do, 204. Chỉ biên bản nháp; 409 khi còn nhiệm vụ
 * trỏ về một kết luận của nó (câu của máy chủ mang số nhiệm vụ đang cản).
 *
 * `duLieu: null` khi thành công — 204 không có thân, và đi qua `docThanKetQua` là biến lần gỡ thành
 * công thành "không đọc được" (`goi.ts`).
 */
export function xoaBienBan(bienBanID: string, lyDo: string): Promise<KetQua<null>> {
  const thanGui: petitions_xoaBienBanVao = { reason: lyDo };
  return goiGhi(duongDanMotBienBan(bienBanID), "DELETE", thanGui, 204).then((kq) =>
    kq.ok ? { ok: true, duLieu: null } : kq,
  );
}

/**
 * POST /api/v1/meetings/{id}/signature — ký, 200, trả lại biên bản đã ký. Lần ký thứ hai là 409,
 * không phải một chữ ký mới. Người ký và thời điểm ký là của PHIÊN và của máy chủ — thân không có
 * trường nào cho chúng.
 *
 * THÂN LUÔN LÀ MỘT ĐỐI TƯỢNG, kể cả khi không kèm Thông báo: hợp đồng khai thân yêu cầu là BẮT BUỘC.
 */
export function kyBienBan(
  bienBanID: string,
  than: KyBienBanVao,
): Promise<KetQua<petitions_bienBanRa>> {
  const thanGui: KyBienBanVao = { notice: chepThongBao(than.notice) };
  return goiGhi(`${duongDanMotBienBan(bienBanID)}/signature`, "POST", thanGui, 200).then(
    docThanKetQua<petitions_bienBanRa>,
  );
}

/**
 * PATCH …/conclusions/{stt} — sửa LỜI một kết luận, 200. Chỉ biên bản nháp; 409 khi kết luận đã
 * tách thành nhiệm vụ (nhiệm vụ đang trích đúng câu ấy).
 */
export function suaKetLuan(
  bienBanID: string,
  ketLuan: petitions_ketLuanRa,
  noiDung: string,
): Promise<KetQua<petitions_ketLuanRa>> {
  const thanGui: petitions_suaKetLuanVao = { content: noiDung };
  return goiGhi(duongDanKetLuan(bienBanID, ketLuan), "PATCH", thanGui, 200).then(
    docThanKetQua<petitions_ketLuanRa>,
  );
}

/**
 * DELETE …/conclusions/{stt} — xoá mềm một kết luận kèm lý do, 204. Số đã cấp KHÔNG cấp lại: gỡ ②
 * thì kết luận thêm sau mang số ④ (luật 7, bất biến 3).
 */
export function xoaKetLuan(
  bienBanID: string,
  ketLuan: petitions_ketLuanRa,
  lyDo: string,
): Promise<KetQua<null>> {
  const thanGui: petitions_xoaBienBanVao = { reason: lyDo };
  return goiGhi(duongDanKetLuan(bienBanID, ketLuan), "DELETE", thanGui, 204).then((kq) =>
    kq.ok ? { ok: true, duLieu: null } : kq,
  );
}

/**
 * PUT …/no-task-marker — đánh dấu "không phát sinh nhiệm vụ", 200. KHÔNG CÓ THÂN (hợp đồng không
 * khai thân yêu cầu nào): chỉ đường dẫn nói kết luận nào.
 */
export function danhDauKhongPhatSinh(
  bienBanID: string,
  ketLuan: petitions_ketLuanRa,
): Promise<KetQua<petitions_ketLuanRa>> {
  return goiGhi(`${duongDanKetLuan(bienBanID, ketLuan)}/no-task-marker`, "PUT", undefined, 200).then(
    docThanKetQua<petitions_ketLuanRa>,
  );
}

/** DELETE …/no-task-marker — bỏ dấu, 204, không thân. */
export function boDauKhongPhatSinh(
  bienBanID: string,
  ketLuan: petitions_ketLuanRa,
): Promise<KetQua<null>> {
  return goiGhi(`${duongDanKetLuan(bienBanID, ketLuan)}/no-task-marker`, "DELETE", undefined, 204).then(
    (kq) => (kq.ok ? { ok: true, duLieu: null } : kq),
  );
}

/**
 * GET …/conclusions/{stt}/tasks — các nhiệm vụ CÒN HIỆU LỰC tách từ một kết luận, theo thứ tự tách
 * (§3: "có thể mở rộng để xem danh sách nhiệm vụ đã sinh ra").
 */
export function layNhiemVuCuaKetLuan(
  bienBanID: string,
  ketLuan: petitions_ketLuanRa,
): Promise<KetQua<petitions_nhiemVuKetLuanRa>> {
  return docJSON<petitions_nhiemVuKetLuanRa>(`${duongDanKetLuan(bienBanID, ketLuan)}/tasks`);
}
