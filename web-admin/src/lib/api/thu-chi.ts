/**
 * Tám tuyến của màn "Thu - Chi ngân sách" (`docs/ui-ux/07-thu-chi-ngan-sach.md`), đúng bộ tuyến
 * `service-finance/internal/http/routes.go:747-946` khai — không nhiều hơn, không ít hơn.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: mọi hình dạng thân và phản hồi đến từ `schema.gen.ts`.
 * Không tệp nào trong ứng dụng này mô tả lại một bảng ngân sách (luật 9, cấm #2).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BA CHỖ HỢP ĐỒNG SINH RA LỆCH KHỎI HANDLER THẬT — ĐÃ ĐO, VÀ THEO HANDLER.
 *
 *  1. `GET /api/v1/budget-sheets` khai `year` và `kind` là `required: false`
 *     (`finance_get_budget_sheets["truyVan"]` sinh ra hai trường `?`), trong khi handler trả
 *     **400 khi thiếu bất kỳ cái nào** (`thu_chi_ngan_sach.go:379` `namVaLoai`, và `:396`). Nên
 *     `LocBang` dưới đây bắt buộc CẢ HAI: `tsc` chặn lời gọi thiếu năm hay thiếu loại ngay lúc
 *     dựng, thay vì để nó thành một 400 lúc chạy mà cán bộ đọc ra là "không tải được".
 *
 *  2. `GET /api/v1/budget-indicators` **không bao giờ 404** khi xã chưa có bảng: nó trả 200 kèm
 *     `unavailable_reason` cho từng chỉ số (`thu_chi_ngan_sach.go:415-475`). Đó là lý do thẻ KPI
 *     ở màn hình không được biến mất và **không được đọc `null` thành `0`** — xem `nhan-thu-chi.ts`.
 *
 *  3. Ba trường `method` · `level` · `is_headline` CÓ trong kiểu thân sinh ra, và chúng ở đó
 *     *"present only so it can be refused"* (`thu_chi_ngan_sach.go:324`): gửi lên là **400**.
 *     Nên chúng bị `Omit` khỏi kiểu tham số của hai hàm ghi dưới đây — một lời gọi gửi chúng
 *     không biên dịch được, thay vì hỏng lúc chạy.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ TUYẾN NẠP EXCEL VÀ KHÔNG CÓ TUYẾN ĐỢT THU CHI, và hai chỗ trống ấy là **sự thật của
 * hợp đồng**, không phải việc còn lại của tệp này: `POST /api/ngan-sach/nap-excel` (§6) và
 * `GET/POST /api/khoan-muc/:id/dot` (§5) không tồn tại ở bất kỳ đâu trong `openapi.json`, và
 * `dot_thu_chi` không có bảng. Màn hình nói thẳng điều đó ra chứ không giấu trong chú thích.
 */

import { LOI_KHONG_RO, docJSON, goiGhi, type KetQua } from "./goi";
import type {
  finance_bangDayDuRa,
  finance_bangRa,
  finance_chiSoNamRa,
  finance_delete_budget_lines_by_id,
  finance_delete_budget_sheets_by_id,
  finance_dongRa,
  finance_get_budget_indicators,
  finance_get_budget_sheets,
  finance_goVao,
  finance_patch_budget_lines_by_id,
  finance_post_budget_lines,
  finance_post_budget_lines_by_id_headline,
  finance_post_budget_sheets,
  finance_suaDongVao,
  finance_taoBangVao,
  finance_themDongVao,
} from "./schema.gen";

/**
 * Đọc thân của một lần ghi thành công.
 *
 * ⚠ ĐÂY LÀ BẢN THỨ BA CỦA HÀM NÀY (`van-ban.ts:62`, `can-bo.ts:202` là hai bản kia), và nó được
 * viết lại thay vì gộp về `goi.ts` vì `goi.ts` nằm NGOÀI ranh giới ghi của lượt này. Ghi ra đây
 * như một phát hiện chứ không im lặng: điều kiện `goi.ts` tự đặt cho mình — *"màn hình ghi thứ
 * hai xuất hiện là lúc hàm này chuyển sang goi.ts"* — nay đã thoả lần thứ hai.
 */
async function docThanRa<T>(goi: Promise<KetQua<Response>>): Promise<KetQua<T>> {
  const kq = await goi;
  if (!kq.ok) return kq;
  try {
    return { ok: true, duLieu: (await kq.duLieu.json()) as T };
  } catch {
    // Đúng mã mong đợi mà thân không phải JSON là máy chủ hoặc proxy đang trả thứ khác. Với cán
    // bộ thì đó vẫn là "không đọc được", không phải một trạng thái nghiệp vụ.
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/** Đường dẫn của một bản ghi cụ thể. `encodeURIComponent` vì id đi vào ĐƯỜNG DẪN, không vào thân. */
function duongDanMot(mau: string, id: string): string {
  return mau.replace("{id}", encodeURIComponent(id));
}

/**
 * Hai tab của màn hình, đúng hai giá trị máy chủ nhận (`domain.LoaiBang`). Không có giá trị thứ
 * ba, và một chuỗi tự do ở đây sẽ thành 400 ở lượt đọc đầu tiên.
 */
export type LoaiBang = "thu" | "chi";

/** Bảng nào: một NĂM và một TAB. Cả hai bắt buộc — xem khối chú thích đầu tệp, điểm 1. */
export type LocBang = {
  nam: number;
  loai: LoaiBang;
};

/**
 * Dựng đường dẫn đọc bảng. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`.
 *
 * KHÔNG CÓ `tenant_id` Ở ĐÂY, và không được thêm: xã suy từ `Host` ở rìa ngoài cùng, còn client
 * tự khai xã là client tự cấp quyền (luật 1, cấm #2).
 */
export function duongDanBang(loc: LocBang): string {
  const duongDan: finance_get_budget_sheets["duongDan"] = "/api/v1/budget-sheets";
  const truyVan = new URLSearchParams();
  truyVan.set("year", String(loc.nam));
  truyVan.set("kind", loc.loai);
  return `${duongDan}?${truyVan.toString()}`;
}

/**
 * GET /api/v1/budget-sheets — cả một bảng: cột, cây khoản mục, giá trị, và ô tóm tắt.
 *
 * **404 Ở TUYẾN NÀY THƯỜNG KHÔNG PHẢI LỖI**: nó là câu *"Không tìm thấy bảng ngân sách này."* của
 * một xã chưa lập bảng cho năm và loại ấy (`thu_chi_ngan_sach.go:735`). Hàm này KHÔNG phân biệt
 * nó với các mã lỗi khác, có chủ ý — `KetQua` cố ý không mang mã lỗi ra tới giao diện, và rẽ
 * nhánh theo `code` hay theo câu chữ là dựng lại đúng thứ `goi.ts` vừa giấu đi. Màn hình xử lý
 * bằng cách khác: xem `bang-thu-chi.tsx`, khối "chưa đọc được bảng".
 */
export function layBang(loc: LocBang): Promise<KetQua<finance_bangDayDuRa>> {
  return docJSON<finance_bangDayDuRa>(duongDanBang(loc));
}

/** Dựng đường dẫn đọc ba chỉ số của một năm. */
export function duongDanChiSo(nam: number): string {
  const duongDan: finance_get_budget_indicators["duongDan"] = "/api/v1/budget-indicators";
  const truyVan = new URLSearchParams();
  truyVan.set("year", String(nam));
  return `${duongDan}?${truyVan.toString()}`;
}

/**
 * GET /api/v1/budget-indicators — ba chỉ số §9 quy tắc 6 đẩy lên `/tong-quan` và `/bao-cao`.
 *
 * MỘT TUYẾN RIÊNG VÌ `Cân đối thu - chi` CẦN CẢ HAI BẢNG (ADR 0035 #32): nó là `Thu xã hưởng` của
 * bảng thu trừ `Chi ngân sách` của bảng chi, nên không nằm được trong phản hồi của bảng nào.
 */
export function layChiSoNganSach(nam: number): Promise<KetQua<finance_chiSoNamRa>> {
  return docJSON<finance_chiSoNamRa>(duongDanChiSo(nam));
}

/**
 * Thân tạo bảng, TRỪ `code`.
 *
 * `code` bị loại ở tầng kiểu chứ không ở tầng chạy: mã bảng do hệ thống cấp và
 * `UNIQUE (tenant_id, ma)` đếm cả dòng đã xoá mềm, nên một client đặt được mã là một client **đốt
 * vĩnh viễn** một mã của xã (`thu_chi_ngan_sach.go:297`). Máy chủ trả 400 khi thấy nó; ở đây nó
 * không gõ được.
 */
export type TaoBangVao = Omit<finance_taoBangVao, "code">;

/**
 * POST /api/v1/budget-sheets — lập bảng thu hoặc chi của một năm, kèm bộ cột. 201.
 *
 * `Idempotency-Key` BẮT BUỘC (`idem.Required(idem.DongKhiHong)`, `routes.go:795`), và cái giá đi
 * kèm được nói ra: Redis hỏng thì tuyến trả **503** và xã không lập được bảng. Nó được chọn như
 * vậy vì không có khoá duy nhất nào phân biệt được một lần bấm hai lần với một lần lập bảng thứ
 * hai có chủ ý — `lan` làm bảng thứ hai thành hợp lệ theo thiết kế.
 *
 * `khoaChongTrung` LÀ THAM SỐ, sinh ở chỗ MỞ biểu mẫu chứ không ở chỗ gửi: sinh tại đây thì mỗi
 * lần bấm lại là một khoá mới, tức là không chống được gì.
 */
export function taoBang(
  than: TaoBangVao,
  khoaChongTrung: string,
): Promise<KetQua<finance_bangRa>> {
  const duongDan: finance_post_budget_sheets["duongDan"] = "/api/v1/budget-sheets";

  // DỰNG TỪNG TRƯỜNG, KHÔNG `...than`: một phép trải là đường để `code` — hay một trường ai đó
  // đọc được từ một bảng đã có — đi lên máy chủ vào ngày có người truyền vào một đối tượng khác.
  const thanGui: TaoBangVao = {
    year: than.year,
    kind: than.kind,
    title: than.title,
    unit: than.unit,
    cumulative_to: than.cumulative_to,
    columns: than.columns.map((c) => ({
      name: c.name,
      order: c.order,
      type: c.type,
      formula: c.formula,
      role: c.role,
    })),
  };

  return docThanRa<finance_bangRa>(
    goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }),
  );
}

/**
 * DELETE /api/v1/budget-sheets/{id} — §6's `🗑 Gỡ`. 204 KHÔNG THÂN.
 *
 * LÝ DO LÀ BẮT BUỘC, VÀ ĐẶC TẢ KHÔNG BIẾT ĐIỀU ĐÓ. §6 chỉ viết *"phải có hộp xác nhận"*; máy chủ
 * đòi `reason` trong THÂN (luật 7 bất biến 1 kể tên `delete_reason`), nên hộp xác nhận ở màn hình
 * phải có một ô lý do chứ không chỉ một nút Đồng ý. Lý do đi trong thân chứ không trong chuỗi
 * truy vấn: chữ tự do về ngân sách một cơ quan nhà nước mà nằm trong URL là chữ nằm lại trong mọi
 * nhật ký truy cập và mọi bộ đệm trung gian.
 *
 * XOÁ MỀM: cả bảng lẫn cây khoản mục còn nguyên kèm `deleted_at`, `deleted_by`, `delete_reason`,
 * và mã bảng KHÔNG quay lại dãy — lần lập lại của năm ấy là `lan` kế tiếp.
 */
export async function goBang(id: string, lyDo: string): Promise<KetQua<null>> {
  const mau: finance_delete_budget_sheets_by_id["duongDan"] = "/api/v1/budget-sheets/{id}";
  const thanGui: finance_goVao = { reason: lyDo };

  const kq = await goiGhi(duongDanMot(mau, id), "DELETE", thanGui, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/**
 * Thân thêm khoản mục, TRỪ ba trường máy chủ từ chối.
 *
 * `method` KHÔNG PHẢI LỰA CHỌN CỦA NGƯỜI DÙNG — máy suy nó từ CÂY (`CachTinhTheoCay(coCon)`): có
 * con thì cộng con, không con thì nhập tay. §4.2 vẽ một ô chọn ba giá trị; hai trong ba điều ấy
 * đã sai (chỉ còn HAI chế độ, và không ai chọn).
 */
export type ThemDongVao = Omit<finance_themDongVao, "method" | "level" | "is_headline">;

/**
 * POST /api/v1/budget-lines — `＋ Thêm khoản mục con` và `⊞ Thêm khoản mục cấp cao nhất`. 201.
 *
 * `Idempotency-Key` BẮT BUỘC (`routes.go:849`): hai dòng cùng tên dưới một cha là chuyện CÓ THẬT
 * trong biểu mẫu này, nên không ràng buộc duy nhất nào phân biệt được một lần bấm hai lần với hai
 * dòng thật. Một lần trùng là một con số bị cộng hai lần vào tổng của cha, và tổng ấy đi lên báo cáo.
 */
export function themKhoanMuc(
  than: ThemDongVao,
  khoaChongTrung: string,
): Promise<KetQua<finance_dongRa>> {
  const duongDan: finance_post_budget_lines["duongDan"] = "/api/v1/budget-lines";

  const thanGui: ThemDongVao = {
    sheet_id: than.sheet_id,
    parent_id: than.parent_id,
    no: than.no,
    name: than.name,
    order: than.order,
  };

  return docThanRa<finance_dongRa>(
    goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }),
  );
}

/**
 * Thân sửa khoản mục, TRỪ năm trường máy chủ từ chối.
 *
 * `sheet_id` và `parent_id` bị TỪ CHỐI chứ không bị bỏ qua, và đó là một quyết định nghiệp vụ:
 * dời một dòng là dời con số của nó giữa hai cái tổng mà người ta đã đọc trên màn hình. Cách làm
 * đúng cho một dòng đặt sai chỗ là GỠ kèm lý do rồi nhập lại — hai sự kiện, cả hai có vết.
 */
export type SuaDongVao = Omit<
  finance_suaDongVao,
  "sheet_id" | "parent_id" | "method" | "level" | "is_headline"
>;

/**
 * PATCH /api/v1/budget-lines/{id} — sửa TT, tên, thứ tự và các ô số. 200.
 *
 * `values` LÀ MỘT BẢN ĐỒ BA TRẠNG THÁI, không phải hai: một mã cột kèm số thì GHI, kèm `null` thì
 * XOÁ TRẮNG ô ấy (§9 quy tắc 4 — ô trống là một trạng thái, vẽ `—`, và không phải số 0), còn vắng
 * mặt thì để nguyên. Lẫn "không nhắc tới" với "xoá trắng" là xoá một con số ngân sách mà không ai
 * báo gì.
 *
 * MỘT TUYẾN CHO CẢ BỐN THỨ, VÀ ĐẶC TẢ §8 ĐỀ XUẤT BA. Sửa tên, sửa TT, sửa ô số và đổi cha từng là
 * ba tuyến riêng trong bản vẽ; máy chủ gộp ba việc đầu vào đây và **không cho** việc thứ tư.
 *
 * GỬI SỐ VÀO MỘT DÒNG CÓ CON LÀ **409**, không phải một lần ghi lặng lẽ bị bỏ: cha luôn cộng con
 * (quyết định của khách 06/09/2026). Câu từ chối của máy chủ đi thẳng ra màn hình.
 */
export function suaKhoanMuc(id: string, than: SuaDongVao): Promise<KetQua<finance_dongRa>> {
  const mau: finance_patch_budget_lines_by_id["duongDan"] = "/api/v1/budget-lines/{id}";

  const thanGui: SuaDongVao = {
    no: than.no,
    name: than.name,
    order: than.order,
    values: than.values,
  };

  return docThanRa<finance_dongRa>(goiGhi(duongDanMot(mau, id), "PATCH", thanGui, 200));
}

/**
 * DELETE /api/v1/budget-lines/{id} — `🗑 Gỡ khoản mục`. 204 KHÔNG THÂN, `reason` BẮT BUỘC.
 *
 * GỠ MỘT DÒNG CÓ CON LÀ **409**, không phải một lần gỡ cả nhánh: một lần bấm không được phép lấy
 * cả một nhánh ra khỏi mọi con số tổng, vì những dòng này là hồ sơ lưu trữ — "hoàn tác" ở đây
 * không phải một nút, nó là nhập lại từng dòng.
 */
export async function goKhoanMuc(id: string, lyDo: string): Promise<KetQua<null>> {
  const mau: finance_delete_budget_lines_by_id["duongDan"] = "/api/v1/budget-lines/{id}";
  const thanGui: finance_goVao = { reason: lyDo };

  const kq = await goiGhi(duongDanMot(mau, id), "DELETE", thanGui, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/**
 * POST /api/v1/budget-lines/{id}/headline — ngôi sao `☆` của §4.1. 200, KHÔNG THÂN gửi đi.
 *
 * ĐÂY LÀ THAO TÁC NẶNG NHẤT MÀN HÌNH, và nó đứng sau `budget.confirm` chứ không `budget.update`:
 * nó không đổi một con số, nó đổi **dòng mà mọi ô tóm tắt và cả hai chỉ số được đọc từ đó** — tức
 * là những con số đi vào một văn bản gửi lên cấp trên. Đánh sao sai không nhìn thấy được: bảng thu
 * có hai dòng cấp cao lồng nhau, bảng chi có `Tổng số` đứng NGANG HÀNG A…E, nên một ngôi sao sai
 * cho ra một báo cáo đầy đủ, hợp lý, và cộng đôi (ADR 0035 §A).
 *
 * KHÔNG CÓ TUYẾN BỎ SAO, có chủ ý: §4.1 vẽ ngôi sao như thứ DI CHUYỂN. Bảng mất dòng tổng chỉ khi
 * dòng đang mang sao bị gỡ, và khi ấy thẻ tóm tắt nói ra bằng một câu thay vì hiện số 0.
 */
export function datDongTong(id: string): Promise<KetQua<finance_dongRa>> {
  const mau: finance_post_budget_lines_by_id_headline["duongDan"] =
    "/api/v1/budget-lines/{id}/headline";

  // KHÔNG THÂN và KHÔNG `Content-Type`: hành vi này không mang thông tin nào ngoài "dòng nào" và
  // "ai", mà cả hai đã nằm trong đường dẫn và trong phiên. Gửi `{}` là tuyên bố có một thân —
  // thứ mời người sau điền vào.
  return docThanRa<finance_dongRa>(goiGhi(duongDanMot(mau, id), "POST", undefined, 200));
}
