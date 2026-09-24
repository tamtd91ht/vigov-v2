/**
 * Các tuyến của màn "Thu - Chi ngân sách" (`docs/ui-ux/07-thu-chi-ngan-sach.md`), đúng bộ tuyến
 * `service-finance/internal/http/routes.go` khai — không nhiều hơn, không ít hơn.
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
 *  3. Nhiều trường CÓ trong kiểu thân sinh ra chỉ để bị từ chối: `method` · `level` ·
 *     `is_headline` khi thêm dòng, `year` · `kind` · `code` · `columns` khi sửa bảng. Gửi lên là
 *     **400**. Nên chúng bị loại khỏi kiểu tham số của các hàm ghi dưới đây — một lời gọi gửi
 *     chúng không biên dịch được, thay vì hỏng lúc chạy. `method` chỉ đi lên qua MỘT hàm riêng
 *     (`doiCachTinh`) với đúng hai giá trị máy chủ nhận.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * TIỀN TRÊN DÂY LUÔN LÀ SỐ NGUYÊN ĐỒNG (`int64`), cả chiều đọc lẫn chiều ghi. Đơn vị tính của
 * bảng (`unit`) chỉ đổi cách HIỂN THỊ; phép quy đổi nằm ở `nhan-thu-chi.ts`, không ở tệp này.
 *
 * KHÔNG CÓ TUYẾN NẠP EXCEL, và chỗ trống ấy là **sự thật của hợp đồng**: không có tuyến nhận tệp
 * nào trong `openapi.json` (§6). Màn hình nói thẳng điều đó ra chứ không giấu trong chú thích.
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./goi";
import type {
  finance_bangDayDuRa,
  finance_bangRa,
  finance_chiSoNamRa,
  finance_danhSachDotRa,
  finance_delete_budget_entries_by_id,
  finance_delete_budget_lines_by_id,
  finance_delete_budget_sheets_by_id,
  finance_dongRa,
  finance_dotRa,
  finance_get_budget_indicators,
  finance_get_budget_lines_by_id_entries,
  finance_get_budget_sheets,
  finance_ghiDotVao,
  finance_goVao,
  finance_patch_budget_lines_by_id,
  finance_patch_budget_sheets_by_id,
  finance_post_budget_lines,
  finance_post_budget_lines_by_id_entries,
  finance_post_budget_lines_by_id_headline,
  finance_post_budget_sheets,
  finance_suaBangVao,
  finance_suaDongVao,
  finance_taoBangVao,
  finance_themDongVao,
} from "./schema.gen";

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

  return docThanLoiGoi<finance_bangRa>(
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
 * `method` KHÔNG GỬI LÚC THÊM: một khoản mục mới luôn là dòng lá `manual`
 * (`ErrCachTinhDoTuClient`). Đổi sang `entries` là một lần PATCH riêng — `doiCachTinh`.
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

  return docThanLoiGoi<finance_dongRa>(
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

  return docThanLoiGoi<finance_dongRa>(goiGhi(duongDanMot(mau, id), "PATCH", thanGui, 200));
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
  return docThanLoiGoi<finance_dongRa>(goiGhi(duongDanMot(mau, id), "POST", undefined, 200));
}

/**
 * Thân sửa bảng — ĐÚNG BA trường máy chủ nhận.
 *
 * `finance_suaBangVao` còn khai `year` · `kind` · `code` · `columns`, và máy chủ trả **400** cho
 * cả bốn: đổi năm hay loại là đổi bảng này thành một bảng khác, đổi cột là đổi nghĩa của mọi con
 * số đã nhập. Chúng không có trong kiểu này, nên không gõ được.
 *
 * `cumulative_to`: `"YYYY-MM-DD"` đặt mốc, `""` BỎ mốc, vắng mặt thì để nguyên.
 */
export type SuaBangVao = Pick<finance_suaBangVao, "title" | "cumulative_to" | "unit">;

/**
 * PATCH /api/v1/budget-sheets/{id} — sửa tiêu đề, mốc luỹ kế, đơn vị tính hiển thị. 200.
 *
 * ĐỔI ĐƠN VỊ KHÔNG ĐỔI CON SỐ NÀO: số liệu lưu bằng đồng, `unit` chỉ nói màn hình chia cho bao
 * nhiêu khi vẽ. Không cần `Idempotency-Key` (`idem.KhongCan`, `routes.go:1036`): gửi hai lần cùng
 * một thân để lại đúng một trạng thái.
 */
export function suaBang(id: string, than: SuaBangVao): Promise<KetQua<finance_bangRa>> {
  const mau: finance_patch_budget_sheets_by_id["duongDan"] = "/api/v1/budget-sheets/{id}";

  // DỰNG TỪNG TRƯỜNG VÀ BỎ TRƯỜNG VẮNG: một phép trải là đường để `year` hay `columns` đi lên vào
  // ngày có người truyền vào nguyên một `bangRa`.
  const thanGui: SuaBangVao = {};
  if (than.title !== undefined) thanGui.title = than.title;
  if (than.cumulative_to !== undefined) thanGui.cumulative_to = than.cumulative_to;
  if (than.unit !== undefined) thanGui.unit = than.unit;

  return docThanLoiGoi<finance_bangRa>(goiGhi(duongDanMot(mau, id), "PATCH", thanGui, 200));
}

/** Hai chế độ một dòng LÁ được chọn (§4.2). `children` do cây quyết định — gửi lên là 400. */
export type CachTinhChon = "manual" | "entries";

/**
 * PATCH /api/v1/budget-lines/{id} với ĐÚNG MỘT trường `method`. 200.
 *
 * Tách khỏi `suaKhoanMuc` có chủ ý: biểu mẫu sửa dòng gửi `values`, và một thân mang cả `method`
 * lẫn `values` là hai quyết định trong một lần bấm. Dòng có con trả **409**.
 *
 * `entries → manual` thì máy chủ chép tổng các đợt vào ô số trong cùng giao dịch; `manual →
 * entries` thì số gõ tay được giữ nhưng không hiện. Màn hình cảnh báo cả hai TRƯỚC khi gửi.
 */
export function doiCachTinh(id: string, method: CachTinhChon): Promise<KetQua<finance_dongRa>> {
  const mau: finance_patch_budget_lines_by_id["duongDan"] = "/api/v1/budget-lines/{id}";
  const thanGui: Pick<finance_suaDongVao, "method"> = { method };
  return docThanLoiGoi<finance_dongRa>(goiGhi(duongDanMot(mau, id), "PATCH", thanGui, 200));
}

/** GET /api/v1/budget-lines/{id}/entries — các đợt của một khoản mục, mới nhất trước. */
export function layDot(khoanMucId: string): Promise<KetQua<finance_danhSachDotRa>> {
  const mau: finance_get_budget_lines_by_id_entries["duongDan"] =
    "/api/v1/budget-lines/{id}/entries";
  return docJSON<finance_danhSachDotRa>(duongDanMot(mau, khoanMucId));
}

/**
 * POST /api/v1/budget-lines/{id}/entries — `+ Ghi đợt`. 201.
 *
 * `Idempotency-Key` BẮT BUỘC (`routes.go:1220`): không có khoá duy nhất nào ngoài khoá chính (hai
 * đợt của cùng một khoản thu trong một ngày là chuyện có thật), nên một lần bấm hai lần là một
 * đợt bị cộng hai lần vào con số đi lên cấp trên. Khoá do BIỂU MẪU giữ: dùng lại khi gửi lại sau
 * lỗi, thay mới sau một lần thành công (`khoaSauLanGhi`). Máy chủ nhả khoá khi trả 4xx/5xx
 * (`core/idem/idem.go:378`), nên gửi lại một thân đã sửa bằng cùng khoá là hợp lệ.
 *
 * `counterparty` có thể là DỮ LIỆU CÁ NHÂN (tên người nộp): đi trong THÂN, không bao giờ trong URL.
 */
export function ghiDot(
  khoanMucId: string,
  than: finance_ghiDotVao,
  khoaChongTrung: string,
): Promise<KetQua<finance_dotRa>> {
  const mau: finance_post_budget_lines_by_id_entries["duongDan"] =
    "/api/v1/budget-lines/{id}/entries";

  const thanGui: finance_ghiDotVao = {
    date: than.date,
    content: than.content,
    values: { ...than.values },
  };
  if (than.counterparty !== undefined) thanGui.counterparty = than.counterparty;
  if (than.document_no !== undefined) thanGui.document_no = than.document_no;

  return docThanLoiGoi<finance_dotRa>(
    goiGhi(duongDanMot(mau, khoanMucId), "POST", thanGui, 201, {
      "Idempotency-Key": khoaChongTrung,
    }),
  );
}

/**
 * DELETE /api/v1/budget-entries/{id} — gỡ mềm một đợt. 204, `reason` BẮT BUỘC trong thân.
 *
 * Đứng sau `budget.confirm` (`routes.go:1242`): gỡ một đợt đổi con số của một dòng `entries`, con
 * số có thể đã được đọc trên màn hình và báo lên trên.
 */
export async function goDot(dotId: string, lyDo: string): Promise<KetQua<null>> {
  const mau: finance_delete_budget_entries_by_id["duongDan"] = "/api/v1/budget-entries/{id}";
  const thanGui: finance_goVao = { reason: lyDo };

  const kq = await goiGhi(duongDanMot(mau, dotId), "DELETE", thanGui, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}
