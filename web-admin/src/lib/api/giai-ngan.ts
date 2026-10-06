/**
 * Chín tuyến GHI của phân hệ Giải ngân (`docs/ui-ux/06-giai-ngan.md` §7 · §8 · §8.2 · §9), đúng bộ
 * tuyến `service-finance/internal/http/routes.go` khai — không nhiều hơn, không ít hơn.
 *
 *   POST   /api/v1/disbursements                    budget.update   + Idempotency-Key BẮT BUỘC
 *   PATCH  /api/v1/disbursements/{id}                budget.update
 *   DELETE /api/v1/disbursements/{id}                budget.confirm  (lý do BẮT BUỘC)
 *   POST   /api/v1/disbursements/{id}/confirmation   budget.confirm  (không thân)
 *   POST   /api/v1/disbursements/{id}/lockout        budget.confirm  (không thân)
 *   DELETE /api/v1/disbursements/{id}/lockout        budget.confirm  (lý do BẮT BUỘC)
 *   POST   /api/v1/investment-projects               budget.update   + Idempotency-Key BẮT BUỘC
 *   PATCH  /api/v1/investment-projects/{id}          budget.update
 *   DELETE /api/v1/investment-projects/{id}          budget.confirm  (lý do BẮT BUỘC) — đang đổi:
 *                                                     budget.update, lý do tuỳ chọn (06/10/2026, `xoaDuAn`)
 *
 * ⚠ HAI KHOÁ, KHÔNG MỘT. Bảng trên chia đôi đúng theo trục nguy hiểm mà đặc tả §8.2 vạch ra
 * (`06-giai-ngan.md:202`): **sửa** thì `budget.update`, còn **xoá · xác nhận · khoá/mở khoá** thì
 * `budget.confirm`. Đó là cách xã tách người NHẬP LIỆU khỏi người CHỊU TRÁCH NHIỆM. Gộp hai khoá ở
 * giao diện là mở một thao tác xác nhận cho người chỉ được nhập số — và không có gì trên màn hình
 * nói ra điều đó, vì máy chủ vẫn trả 403 đúng lúc bấm, tức là sau khi người ta đã tin là mình làm
 * được.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: mọi hình dạng thân và phản hồi đến từ `schema.gen.ts` (sinh
 * từ `kb/20-contracts/openapi.json`). Không tệp nào trong ứng dụng này mô tả lại một chứng từ giải
 * ngân hay một dự án đầu tư (luật 9, cấm #2).
 *
 * TUYẾN ĐỌC NẰM Ở `du-an.ts`, KHÔNG Ở ĐÂY, và sự tách ấy giữ đúng một điều: `du-an.ts` vẫn là nơi
 * duy nhất dựng đường dẫn ĐỌC. ⚠ Khối chú thích đầu `du-an.ts` còn câu *"KHÔNG CÓ HÀM GHI NÀO …
 * Máy chủ cũng không có tuyến ghi"* — câu ấy ĐÃ SAI từ lượt này; tệp ấy nằm ngoài ranh giới ghi của
 * lượt này nên nó được báo về chứ không sửa lén.
 *
 * THE VOUCHER LIST IS READ PER PROJECT since db94b35c — `getProjectVouchers` in `du-an.ts`
 * (`GET /api/v1/investment-projects/{id}/disbursements`). The §8.2 tab re-reads it after every write
 * below; the rows these routes return are not kept on screen as a second copy.
 *
 * KHÔNG CÓ `tenant_id` Ở BẤT KỲ ĐÂU — không thân, không query, không header. Xã suy từ `Host` ở rìa
 * ngoài cùng; client tự khai xã là client tự cấp quyền (luật 1, cấm #2).
 *
 * KHÔNG GHI LOG GÌ: thân yêu cầu mang số tiền công quỹ, nội dung chi, tên đối tác và LÝ DO gỡ —
 * chữ tự do do cán bộ gõ. Không dòng nào ở đây đưa chúng vào log, vào tên tệp, vào URL hay vào khoá
 * đệm (luật 3, cấm #1 và #4).
 */

import { docThanLoiGoi, goiGhi, type KetQua } from "./goi";
import type {
  finance_chungTuRa,
  finance_delete_disbursements_by_id,
  finance_delete_disbursements_by_id_lockout,
  finance_delete_investment_projects_by_id,
  finance_duAnGhiRa,
  finance_goChungTuVao,
  finance_moKhoaVao,
  finance_patch_disbursements_by_id,
  finance_patch_investment_projects_by_id,
  finance_post_disbursements,
  finance_post_disbursements_by_id_confirmation,
  finance_post_disbursements_by_id_lockout,
  finance_post_investment_projects,
  finance_suaChungTuVao,
  finance_suaDuAnVao,
  finance_themChungTuVao,
  finance_themDuAnVao,
  finance_xoaDuAnVao,
} from "./schema.gen";

/** Đường dẫn của một bản ghi cụ thể. `encodeURIComponent` vì id đi vào ĐƯỜNG DẪN, không vào thân. */
export function duongDanMot(mau: string, id: string): string {
  return mau.replace("{id}", encodeURIComponent(id));
}

/* ── Chứng từ giải ngân §8.2 ───────────────────────────────────────────────────────────────── */

const DUONG_DAN_CHUNG_TU: finance_post_disbursements["duongDan"] = "/api/v1/disbursements";
const MAU_MOT_CHUNG_TU: finance_patch_disbursements_by_id["duongDan"] = "/api/v1/disbursements/{id}";
const MAU_GO_CHUNG_TU: finance_delete_disbursements_by_id["duongDan"] = "/api/v1/disbursements/{id}";
const MAU_XAC_NHAN: finance_post_disbursements_by_id_confirmation["duongDan"] =
  "/api/v1/disbursements/{id}/confirmation";
const MAU_KHOA: finance_post_disbursements_by_id_lockout["duongDan"] =
  "/api/v1/disbursements/{id}/lockout";
const MAU_MO_KHOA: finance_delete_disbursements_by_id_lockout["duongDan"] =
  "/api/v1/disbursements/{id}/lockout";

/**
 * Thân của `POST /api/v1/disbursements` — §8.2 `+ Ghi nhận khoản chi`. TRỪ `status`.
 *
 * `status` BỊ LOẠI Ở TẦNG KIỂU, và nó có mặt trong hợp đồng CHỈ để bị từ chối: máy chủ trả **400**
 * khi thấy nó, vì "vòng đời đi qua các tuyến xác nhận / khoá / mở khoá, mỗi bước một vết kiểm toán"
 * (`domain.ErrTrangThaiDoTuClient`). Một client tự khai trạng thái là một client vừa tạo ra một
 * chứng từ `Đã khoá` mà không ai xác nhận — một con số đóng băng, không sửa được, đang cộng vào
 * tổng đã giải ngân của xã. `Omit` ở đây làm lời gọi ấy KHÔNG BIÊN DỊCH ĐƯỢC, thay vì hỏng lúc chạy.
 *
 * KHÔNG CÓ `entered_by`, và hợp đồng cũng không có: người nhập là chủ thể của phiên (luật 6, bất
 * biến 8). Một trường ở đây là một client khai người khác làm tác giả một bản ghi tài chính.
 */
export type ThemChungTuVao = Omit<finance_themChungTuVao, "status">;

/**
 * POST /api/v1/disbursements — ghi một khoản chi cho một dự án. 201, trả về nguyên chứng từ.
 *
 * `Idempotency-Key` BẮT BUỘC (`x-vigov-idempotency.required` trong hợp đồng), và cái giá đi kèm
 * được nói ra: chế độ là `DongKhiHong`, nên Redis không tới được thì tuyến trả **503** và xã không
 * ghi được chứng từ. Nó được chọn như vậy vì không có khoá duy nhất nào phân biệt được một lần bấm
 * hai lần với hai khoản chi thật cùng ngày, cùng số tiền, cùng nội dung — và hai lần ấy có thật
 * (thanh toán làm nhiều đợt). Một lần trùng là một khoản tiền được đếm hai lần trong tổng đã giải
 * ngân, và tổng ấy đi lên báo cáo.
 *
 * `khoaChongTrung` LÀ THAM SỐ, SINH LÚC MỞ BIỂU MẪU, không sinh tại đây: sinh trong hàm thì mỗi lần
 * bấm lại sau một lỗi mạng là một khoá mới — tức là không chống được gì, mà lần gửi đầu CÓ THỂ đã
 * tới máy chủ.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG `...than`: một phép trải là đường để `status` — hay một trường ai đó đọc
 * được từ một chứng từ đã có — đi lên máy chủ vào ngày có người truyền vào một đối tượng khác.
 */
export function themChungTu(
  than: ThemChungTuVao,
  khoaChongTrung: string,
): Promise<KetQua<finance_chungTuRa>> {
  const thanGui: ThemChungTuVao = {
    project_id: than.project_id,
    payment_date: than.payment_date,
    amount: than.amount,
    description: than.description,
    counterparty: than.counterparty,
    voucher_no: than.voucher_no,
    funding_source_id: than.funding_source_id,
  };

  return docThanLoiGoi<finance_chungTuRa>(
    goiGhi(DUONG_DAN_CHUNG_TU, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }),
  );
}

/**
 * Thân của `PATCH /api/v1/disbursements/{id}` — §8.2 `✎ Sửa`. TRỪ `status` và `project_id`.
 *
 * HAI TRƯỜNG BỊ LOẠI VÌ HAI LÝ DO KHÁC NHAU, và cả hai là **400** ở máy chủ:
 *
 *   `status`      vòng đời không do client đặt — xem `ThemChungTuVao`.
 *   `project_id`  chuyển chứng từ sang dự án khác là chuyển tiền giữa hai con số đã báo cáo mà
 *                 không màn hình nào nói ra. Cách làm đúng là GỠ kèm lý do rồi nhập lại ở dự án
 *                 đúng — hai sự kiện, cả hai có vết (`domain.ErrDuAnBatBien`).
 *
 * ⚠ MỖI TRƯỜNG LÀ MỘT CON TRỎ Ở MÁY CHỦ, VÀ ĐÓ LÀ TOÀN BỘ LÝ DO ĐÂY LÀ `PATCH` CHỨ KHÔNG `PUT`.
 * VẮNG nghĩa là "để nguyên"; `""` ở `counterparty` · `voucher_no` · `funding_source_id` nghĩa là
 * **XOÁ TRẮNG** ô ấy. Riêng `funding_source_id` mang ba nghĩa và nghĩa giữa là thứ không diễn đạt
 * được bằng cách nào khác: `""` **gỡ** chứng từ khỏi nguồn vốn, đưa nó về đúng trạng thái §13 quy
 * tắc 6 ("đã chi nhưng chưa ghi rút từ nguồn nào").
 *
 * `null` ĐỌC LÀ "ĐỂ NGUYÊN", KHÔNG PHẢI "XOÁ" — đó là điều `*string` của Go làm, và máy chủ ghi rõ
 * quy ước ấy. Không hàm nào ở tệp này sinh ra `null`: `undefined` rụng khỏi thân khi
 * `JSON.stringify`, và đó là cách duy nhất nói "để nguyên" mà không nhập nhằng.
 */
export type SuaChungTuVao = Omit<finance_suaChungTuVao, "status" | "project_id">;

/**
 * PATCH /api/v1/disbursements/{id} — sửa một chứng từ CHƯA KHOÁ. 200, trả về hàng SAU khi sửa.
 *
 * ⚠ CHỨNG TỪ ĐANG `Đã xác nhận` MÀ SỬA THÌ **VỀ `Kế toán nhập`**, và dấu người xác nhận bị xoá
 * (ADR 0036, `app/chung_tu_giai_ngan.go:423-441`). Lý do là của khách: *"Lãnh đạo xác nhận những
 * con số kia, không phải những con số này."* Màn hình phải nói trước điều đó ở chỗ người ta bấm
 * Sửa — phản hồi mang `status` mới về, nhưng một cán bộ phát hiện sau khi lưu là một cán bộ vừa
 * làm mất chữ ký của lãnh đạo mà không biết.
 *
 * MỘT LẦN SỬA KHÔNG ĐỔI GÌ THÌ MÁY CHỦ KHÔNG GHI GÌ, và nhánh ấy có thật chứ không phải tối ưu:
 * nó là thứ giữ cho một lần bấm Lưu lặp lại không bóc mất chữ xác nhận của một chứng từ không ai
 * sửa. Giao diện vẫn nên tự chặn thân rỗng — xem `thanSuaChungTu`.
 *
 * KHÔNG MANG `Idempotency-Key`: hợp đồng không đòi, và đặt lại đúng những giá trị ấy hai lần cho
 * ra cùng một hàng.
 */
export function suaChungTu(id: string, than: SuaChungTuVao): Promise<KetQua<finance_chungTuRa>> {
  const thanGui: SuaChungTuVao = {
    payment_date: than.payment_date,
    amount: than.amount,
    description: than.description,
    counterparty: than.counterparty,
    voucher_no: than.voucher_no,
    funding_source_id: than.funding_source_id,
  };

  return docThanLoiGoi<finance_chungTuRa>(goiGhi(duongDanMot(MAU_MOT_CHUNG_TU, id), "PATCH", thanGui, 200));
}

/**
 * DELETE /api/v1/disbursements/{id} — §8.2 `🗑 Gỡ`. 204 KHÔNG THÂN.
 *
 * ĐỨNG SAU `budget.confirm`, KHÔNG SAU `budget.update`, và đó là chỗ dễ gắn cổng nhầm nhất của cả
 * màn: gỡ một chứng từ lấy một khoản tiền ra khỏi tổng đã giải ngân mà lãnh đạo đã đọc.
 *
 * LÝ DO LÀ BẮT BUỘC, và đặc tả không biết điều đó — §8.2 chỉ vẽ một cái nút. Máy chủ đòi `reason`
 * trong THÂN (luật 7 bất biến 1 kể tên `delete_reason`), nên hộp xác nhận phải có MỘT Ô LÝ DO chứ
 * không chỉ một nút Đồng ý. Lý do đi trong thân chứ không trong chuỗi truy vấn: chữ tự do về chi
 * tiêu của một cơ quan nhà nước mà nằm trong URL là chữ nằm lại trong mọi nhật ký truy cập.
 *
 * XOÁ MỀM: hàng còn nguyên kèm `deleted_at`, `deleted_by`, `delete_reason` (luật 7, bất biến 1).
 */
export async function goChungTu(id: string, lyDo: string): Promise<KetQua<null>> {
  const thanGui: finance_goChungTuVao = { reason: lyDo };
  const kq = await goiGhi(duongDanMot(MAU_GO_CHUNG_TU, id), "DELETE", thanGui, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/**
 * POST /api/v1/disbursements/{id}/confirmation — `Kế toán nhập` → `Đã xác nhận`. 200.
 *
 * KHÔNG THÂN VÀ KHÔNG `Content-Type`: xác nhận không mang thông tin nào ngoài "chứng từ nào" và
 * "ai", mà cả hai đã nằm trong đường dẫn và trong phiên. Gửi `{}` là tuyên bố có một thân — thứ
 * mời người sau điền vào.
 *
 * 409 KHI CHỨNG TỪ ĐÃ XÁC NHẬN RỒI, không phải 403: người gọi CÓ quyền, cái bị từ chối là thao tác
 * này trên CHỨNG TỪ NÀY. Câu của máy chủ ra thẳng màn hình.
 */
export function xacNhanChungTu(id: string): Promise<KetQua<finance_chungTuRa>> {
  return docThanLoiGoi<finance_chungTuRa>(goiGhi(duongDanMot(MAU_XAC_NHAN, id), "POST", undefined, 200));
}

/**
 * POST /api/v1/disbursements/{id}/lockout — `Đã xác nhận` → `Đã khoá`. 200, KHÔNG THÂN gửi đi.
 *
 * KHOÁ NGHĨA LÀ GÌ TRÊN MÀN (§8.2 + `domain.ErrChungTuDaKhoa`): chứng từ đã khoá thì **không sửa,
 * không gỡ** — muốn sửa phải mở khoá trước. Đây là bước cuối của vòng đời, không phải một cái cờ
 * trang trí.
 *
 * CHƯA XÁC NHẬN THÌ KHÔNG KHOÁ ĐƯỢC: máy chủ trả 409 kèm nguyên câu vòng đời. Giao diện không dựng
 * lại luật ấy bằng cách tự ẩn nút theo trạng thái nó đoán — nó ẩn theo `status` máy chủ vừa trả.
 */
export function khoaChungTu(id: string): Promise<KetQua<finance_chungTuRa>> {
  return docThanLoiGoi<finance_chungTuRa>(goiGhi(duongDanMot(MAU_KHOA, id), "POST", undefined, 200));
}

/**
 * DELETE /api/v1/disbursements/{id}/lockout — mở khoá, về `Đã xác nhận`. 200 kèm chứng từ.
 *
 * LÝ DO BẮT BUỘC, và đây là chỗ nó là một QUYẾT ĐỊNH chứ không phải luật của người khác: câu hỏi mở
 * #29 được chốt theo hướng này ngày 22/09/2026 vì những lần mở khoá ĐÃ XẢY RA không dựng lại được
 * từ bất kỳ nguồn nào sau đó (`moKhoaVao`, `chung_tu_giai_ngan.go:207-216`).
 *
 * ⚠ NGƯỜI VỪA KHOÁ KHÔNG TỰ MỞ LẠI ĐƯỢC — 409, không phải 403, vì luật là về NGƯỜI chứ không về
 * quyền: cấp thêm khoá cũng không đổi được gì (`domain.ErrTuMoKhoaChungTuMinhVuaKhoa`). Giao diện
 * KHÔNG đoán trước điều đó: nó không biết mã cán bộ của phiên có bằng `locked_by` hay không mà
 * không đi so hai chuỗi định danh, nên nó để máy chủ từ chối và hiện NGUYÊN câu ấy — câu đã nói rõ
 * phải nhờ một cán bộ khác.
 */
export function moKhoaChungTu(id: string, lyDo: string): Promise<KetQua<finance_chungTuRa>> {
  const thanGui: finance_moKhoaVao = { reason: lyDo };
  return docThanLoiGoi<finance_chungTuRa>(
    goiGhi(duongDanMot(MAU_MO_KHOA, id), "DELETE", thanGui, 200),
  );
}

/* ── Dự án đầu tư §9 · §8 · §7 ─────────────────────────────────────────────────────────────── */

const DUONG_DAN_DU_AN: finance_post_investment_projects["duongDan"] = "/api/v1/investment-projects";
const MAU_SUA_DU_AN: finance_patch_investment_projects_by_id["duongDan"] =
  "/api/v1/investment-projects/{id}";
const MAU_XOA_DU_AN: finance_delete_investment_projects_by_id["duongDan"] =
  "/api/v1/investment-projects/{id}";

/**
 * Thân của `POST /api/v1/investment-projects` — §9 modal "Thêm dự án", trường theo trường.
 *
 * ⚠ `code` LÀ BẮT BUỘC, VÀ Ô `☑ Tự sinh mã` CỦA §9 CHƯA CÓ. Máy chủ nói thẳng: *"hệ thống chưa tự
 * sinh mã dự án, hãy nhập mã"* (`domain.ErrThieuMaDuAn`), vì đặc tả đưa ra hai khuôn mã mâu thuẫn
 * nhau và không nói dãy số chạy trong phạm vi nào — mà một mã dự án là MÃ ĐÃ CẤP, thứ luật 7 cấm
 * đánh lại. Đây là câu để HỎI khách, không phải để màn hình tự chế một dãy.
 *
 * KHÔNG CÓ `disbursed_amount` và hợp đồng cũng không có: số đã giải ngân là SUM trên chứng từ còn
 * sống, suy ra mỗi lần đọc. Một trường ở đây là một client tự khai con số xã báo cáo lên cấp trên.
 *
 * `funding_allocations` KHÔNG BẮT BUỘC và tổng của nó KHÔNG bị máy chủ ép khớp `planned_amount`:
 * §9 nói rõ đó là một **CẢNH BÁO**, không phải một lần từ chối — ép khớp là chặn cán bộ đúng lúc
 * họ còn đang tính.
 */
export type ThemDuAnVao = finance_themDuAnVao;

/**
 * POST /api/v1/investment-projects — thêm một dự án cho một năm ngân sách. 201.
 *
 * `Idempotency-Key` BẮT BUỘC, chế độ `MoKhiHong`: Redis không tới được thì tuyến **vẫn cho qua** và
 * ghi cảnh báo — KHÁC hẳn tuyến chứng từ ở trên, và sự khác nhau ấy là của máy chủ chứ không phải
 * một lựa chọn ở đây. Nó an toàn được vì dự án có `UNIQUE (tenant_id, ma)`: một lần gửi trùng đập
 * vào ràng buộc ấy và nhận 409 *"mã đã cấp thì không cấp lại"*, nên không có bản ghi đôi nào lọt.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG `...than` — cùng lý do với `themChungTu`. Danh sách phân bổ cũng dựng lại
 * từng dòng, không truyền thẳng mảng người gọi đưa vào.
 */
export function themDuAn(
  than: ThemDuAnVao,
  khoaChongTrung: string,
): Promise<KetQua<finance_duAnGhiRa>> {
  const thanGui: ThemDuAnVao = {
    code: than.code,
    year: than.year,
    category_id: than.category_id,
    name: than.name,
    description: than.description,
    planned_amount: than.planned_amount,
    approved_amount: than.approved_amount,
    org_unit_id: than.org_unit_id,
    assignee_id: than.assignee_id,
    start_date: than.start_date,
    completion_date: than.completion_date,
    disbursement_deadline: than.disbursement_deadline,
    funding_allocations: than.funding_allocations?.map((d) => ({
      funding_source_id: d.funding_source_id,
      amount: d.amount,
    })),
  };

  return docThanLoiGoi<finance_duAnGhiRa>(
    goiGhi(DUONG_DAN_DU_AN, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }),
  );
}

/**
 * Thân của `PATCH /api/v1/investment-projects/{id}` — §8 `[✎ Sửa dự án]`. TRỪ `code` và `year`.
 *
 * HAI TRƯỜNG BỊ LOẠI Ở TẦNG KIỂU VÌ HAI LÝ DO KHÁC NHAU, và máy chủ trả **400** cho cả hai:
 *
 *   `code`  luật 7, cấm #4 — mã đã cấp thì không đánh lại. Nó được in trên hàng §7.2, được trích
 *           trong các quyết định gắn với dự án, và là `subject` của mọi vết kiểm toán mà chứng từ
 *           của dự án đã ghi.
 *   `year`  §13 quy tắc 8 — mỗi năm ngân sách là một tập dự án riêng. Chuyển một dự án sang năm
 *           khác là bê cả kế hoạch vốn và mọi chứng từ của nó ra khỏi tổng của năm này.
 *
 * MỖI TRƯỜNG LÀ MỘT CON TRỎ: vắng là "để nguyên", `""` là **XOÁ TRẮNG** (mô tả về rỗng, cán bộ phụ
 * trách về `— Chưa phân công —`, thời hạn giải ngân về mặc định 31/12). Một thân giá trị thường
 * không phân biệt được hai điều ấy, và hộp thoại sửa mỗi cái tên sẽ lặng lẽ bỏ phân công của dự án.
 *
 * `funding_allocations` HAS THREE MEANINGS (8245698b): ABSENT = leave the allocations as they are,
 * `[]` = remove every line, a list = FULL REPLACEMENT. So the form sends it only when the clerk
 * changed the set (`thanSuaDuAn`) — sending the unchanged list back is a write nobody asked for, and
 * the server would refuse it with `source_has_disbursements` the day a line it repeats was dropped.
 * One source twice in one project is refused (409 `duplicate_source`).
 */
export type SuaDuAnVao = Omit<finance_suaDuAnVao, "code" | "year">;

/**
 * PATCH /api/v1/investment-projects/{id} — sửa một dự án. 200.
 *
 * PHẢN HỒI KHÔNG MANG SỐ SUY RA. `duAnGhiRa` cố ý không có `disbursed_amount`, `disbursed_ratio`,
 * `delay_score` hay `is_delayed`: tuyến ghi không đọc chúng và không giữ khoá trên chúng, nên trả
 * về sẽ là bịa ra những con số 0 trông y hệt số thật. Màn hình ĐỌC LẠI dự án sau khi sửa — một lời
 * gọi thêm, và đó là hình dạng thật thà.
 */
export function suaDuAn(id: string, than: SuaDuAnVao): Promise<KetQua<finance_duAnGhiRa>> {
  const thanGui: SuaDuAnVao = {
    category_id: than.category_id,
    name: than.name,
    description: than.description,
    planned_amount: than.planned_amount,
    approved_amount: than.approved_amount,
    org_unit_id: than.org_unit_id,
    assignee_id: than.assignee_id,
    start_date: than.start_date,
    completion_date: than.completion_date,
    disbursement_deadline: than.disbursement_deadline,
    // Rebuilt line by line, as on create; `undefined` drops out of the JSON and means "unchanged",
    // while `[]` survives and means "remove all" — `map` keeps that difference.
    funding_allocations: than.funding_allocations?.map((d) => ({
      funding_source_id: d.funding_source_id,
      amount: d.amount,
    })),
  };

  return docThanLoiGoi<finance_duAnGhiRa>(goiGhi(duongDanMot(MAU_SUA_DU_AN, id), "PATCH", thanGui, 200));
}

/**
 * DELETE /api/v1/investment-projects/{id} — xoá mềm một dự án. 204 KHÔNG THÂN.
 *
 * THE REASON IS OPTIONAL, BY USER DECISION 06/10/2026 (follow the prototype: deleting a project asks
 * no reason, and `budget.update` gates it). The server card that goes with it makes `reason` optional
 * and writes a fixed sentence into `delete_reason` when it is blank, so rule 7's three columns are
 * still filled — by the server, not by a default typed on screen. Until that card lands the route
 * still says `budget.confirm` + reason required, and its refusal is shown verbatim.
 *
 * `{}` IS SENT, NEVER AN EMPTY BODY: the handler decodes JSON before anything else, and an empty body
 * is an EOF → 400 "không phải JSON hợp lệ" (`service-finance/internal/http/danh_muc_ghi.go` `docThan`).
 * `Partial<…>` of the GENERATED type, so the field name still comes from the contract.
 *
 * DỰ ÁN MÀ MÁY CHỦ TỪ CHỐI (409 — còn chứng từ) thì câu của máy chủ ra thẳng màn hình, vì không có
 * nó thì màn hình trông như hỏng: dự án nằm ngay đó và nút Xoá thì không làm gì.
 *
 * MÃ DỰ ÁN KHÔNG QUAY LẠI DÃY sau khi xoá (luật 7, bất biến 3) — nhập lại phải chọn mã khác.
 */
export async function xoaDuAn(id: string, lyDo?: string): Promise<KetQua<null>> {
  const reason = lyDo?.trim() ?? "";
  const thanGui: Partial<finance_xoaDuAnVao> = reason === "" ? {} : { reason };
  const kq = await goiGhi(duongDanMot(MAU_XOA_DU_AN, id), "DELETE", thanGui, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}
