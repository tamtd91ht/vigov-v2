/**
 * Năm tuyến của **bảng thời hạn xử lý** một xã — `GET /api/v1/sla`, `PATCH /api/v1/sla/{id}`,
 * `POST /api/v1/sla/defaults`, `POST /api/v1/sla`, `DELETE /api/v1/sla/{id}` — plus the three
 * field lists a row's `field` code comes from.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BẢNG NÀY RỖNG THÌ XÃ KHÔNG VÀO SỔ ĐƯỢC VĂN BẢN ĐẾN VÀ KHÔNG NHẬN ĐƯỢC PHẢN ÁNH. Không phải
 * "màn hình thiếu số liệu": `identity.ResolveDeadlines` từ chối bằng `FAILED_PRECONDITION`, và
 * `service-documents` biến nó thành 409 `sla_chua_cau_hinh` ngay ở tuyến tiếp nhận. Đó là lý do
 * màn hình gọi vào đây phải NÓI RA hậu quả chứ không chỉ hiện một bảng rỗng.
 *
 * VÀ KHÔNG MỘT PHÉP TÍNH HẠN NÀO Ở PHÍA WEB. `identity` sở hữu bảng này và sở hữu luôn phép cộng
 * giờ làm việc (ADR 0007, luật 10 bất biến 4). Con số ở đây chỉ để HIỆN và để SỬA.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CẢ NĂM TUYẾN `/api/v1/sla*` ĐÒI `admin.sla`, KỂ CẢ TUYẾN ĐỌC — khác hẳn ba tuyến đọc lịch làm việc
 * (`lich-lam-viec.ts`), vốn là `any-authenticated`. Sự khác nhau đến từ máy chủ và có lý do ghi
 * thẳng trên mã (`service-identity/internal/http/sla.go`): lịch làm việc được đọc ĐỂ VẼ mọi hạn
 * hiện trên màn hình, còn bảng này được đọc ĐỂ CẤU HÌNH — không màn hình nghiệp vụ nào vẽ
 * `gio_xu_ly_xong`, vì hạn đã được CHỐT và lưu trên chính hồ sơ (luật 10, bất biến 2).
 *
 * Hệ quả cho giao diện: tài khoản không có khoá nhận 403 ngay ở lượt ĐỌC, và câu từ chối là câu
 * của máy chủ. Không dựng một cổng quyền thứ hai ở client để đoán trước điều đó.
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: ba quy tắc
 * ấy đúng cho MỌI tuyến và được nói một lần ở `goi.ts`.
 */

import { layLoaiVanBan, layMucUuTienNhiemVu } from "./danh-muc-nghiep-vu";
import { docJSON, docThanKetQua, docThanLoiGoi, goiGhi, type KetQua } from "./goi";
import type {
  identity_addSLAFieldRowIn,
  identity_danhSachSLARa,
  identity_delete_sla_by_id,
  identity_dongSLARa,
  identity_get_sla,
  identity_gieoSLARa,
  identity_patch_sla_by_id,
  identity_post_sla,
  identity_post_sla_defaults,
  identity_removeSLAFieldRowIn,
  identity_suaSLAVao,
  petitions_get_citizen_report_fields,
  petitions_petitionFieldListOut,
} from "./schema.gen";

/**
 * GET /api/v1/sla — cả bảng, kèm `problems`.
 *
 * `problems` LÀ SAI SÓT CỦA DỮ LIỆU XÃ, do máy chủ suy ra từ chính những dòng nó vừa trả về
 * (bảng rỗng, một loại việc có dòng riêng mà thiếu dòng mặc định). Tuyến vẫn trả 200 có chủ ý:
 * đây là màn hình để SỬA, nên nó phải mở được kể cả khi cấu hình đang hỏng. Màn hình hiện NGUYÊN
 * câu máy chủ viết.
 *
 * KHÔNG PHÂN TRANG, và hợp đồng không có tham số nào để phân trang: trả lời "lĩnh vực này hạn bao
 * lâu" cần dòng của lĩnh vực ấy VÀ dòng mặc định nó rơi về, mà một trang không hứa được là giữ cả
 * hai.
 */
export function layThoiHanXuLy(): Promise<KetQua<identity_danhSachSLARa>> {
  const duongDan: identity_get_sla["duongDan"] = "/api/v1/sla";
  return docJSON<identity_danhSachSLARa>(duongDan);
}

/**
 * The kinds of work that take a field's own row (ADR 0079 lô 2 Q4). `don-thu` is absent ON PURPOSE:
 * it has only its default row (lô 3), and the server refuses a field row for it
 * (`domain.CheckSLAKindTakesFieldRows`). The contract types `work_kind` as a bare string, so this list
 * is the one place the three are named for the add form; the server still decides.
 */
export const FIELD_ROW_KINDS = ["phan-anh", "van-ban-den", "nhiem-vu"] as const;
export type FieldRowKind = (typeof FIELD_ROW_KINDS)[number];

/**
 * One entry of the list a kind's field codes come from, as the SLA tab needs it: the code stored on
 * the row, the commune's label, and whether the owner still accepts it on a write.
 */
export type SlaFieldOption = { readonly code: string; readonly label: string; readonly active: boolean };

/**
 * Per kind, the read of its field list: `null` = not read (yet, or not readable by this account).
 * Labels for the "Lĩnh vực" column AND the options of the add form come from the same read.
 */
export type SlaFieldCatalogues = Readonly<Record<FieldRowKind, KetQua<readonly SlaFieldOption[]> | null>>;

function toOptions<T extends { code: string; label: string; active: boolean }>(
  kq: KetQua<{ items: T[] }>,
): KetQua<readonly SlaFieldOption[]> {
  if (!kq.ok) return kq;
  return { ok: true, duLieu: kq.duLieu.items.map((i) => ({ code: i.code, label: i.label, active: i.active })) };
}

/**
 * GET /api/v1/citizen-report-fields (owned by `petitions`, `admin.lookup`) — the field list of
 * `phan-anh` rows. `active` is the tier-1 flag the server checks on the write (`CheckForIntake`).
 *
 * Every code is kept, retired ones included: an SLA row may point at a field since retired, and it
 * still deserves its name in the table. Only the add form filters to `active`. No module-level cache:
 * the catalogue is one commune's (`danh-muc.ts`).
 */
export async function readPetitionFieldOptions(): Promise<KetQua<readonly SlaFieldOption[]>> {
  const path: petitions_get_citizen_report_fields["duongDan"] = "/api/v1/citizen-report-fields";
  return toOptions(await docJSON<petitions_petitionFieldListOut>(path));
}

/** GET /api/v1/document-types — the field list of `van-ban-den` rows. Reuses the catalogue reader. */
export async function readDocumentTypeOptions(): Promise<KetQua<readonly SlaFieldOption[]>> {
  return toOptions(await layLoaiVanBan());
}

/** GET /api/v1/task-priorities — the field list of `nhiem-vu` rows. Order kept: it is the scale. */
export async function readTaskPriorityOptions(): Promise<KetQua<readonly SlaFieldOption[]>> {
  return toOptions(await layMucUuTienNhiemVu());
}

/**
 * POST /api/v1/sla — a field's own row. All six figures travel: the contract requires five, and the
 * screen fills those it does not show from the kind's default row (`sua-thoi-han.ts`).
 *
 * `Idempotency-Key` REQUIRED (`idem.Required`), held by the open form so a retry replays the 201
 * instead of meeting `sla_rule_exists`; a refusal releases it server-side, so the same key may carry
 * the corrected request. Every refusal — 409 `sla_rule_exists`, 400 `sla_field_unknown`, 503 when the
 * owner of the list cannot be asked — is the server's sentence, verbatim.
 */
export async function addSlaFieldRow(
  body: identity_addSLAFieldRowIn,
  idempotencyKey: string,
): Promise<KetQua<identity_dongSLARa>> {
  // Built field by field, never spread: a key the contract lacks must not ride along.
  const sent: identity_addSLAFieldRowIn = {
    work_kind: body.work_kind,
    field: body.field,
    acknowledge_hours: body.acknowledge_hours,
    resolve_hours: body.resolve_hours,
    due_soon_hours: body.due_soon_hours,
    escalate_leader_hours: body.escalate_leader_hours,
    escalate_president_hours: body.escalate_president_hours,
    unassigned_hold_hours: body.unassigned_hold_hours,
  };
  const path: identity_post_sla["duongDan"] = "/api/v1/sla";
  return docThanLoiGoi<identity_dongSLARa>(
    goiGhi(path, "POST", sent, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/**
 * DELETE /api/v1/sla/{id} — soft delete of a field's own row, with the reason the trail keeps
 * (rule 7). 204, no body. No `Idempotency-Key` (`idem.KhongCan`): a second send is a 404.
 * 409 `default_sla_rule` is the default row; its sentence comes back verbatim.
 */
export async function removeSlaFieldRow(id: string, reason: string): Promise<KetQua<null>> {
  const body: identity_removeSLAFieldRowIn = { reason };
  const template: identity_delete_sla_by_id["duongDan"] = "/api/v1/sla/{id}";
  const kq = await goiGhi(template.replace("{id}", encodeURIComponent(id)), "DELETE", body, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/**
 * The sixth figure (migration 0016): working hours a unit may hold work with nobody assigned
 * before the reminder job reports it. `satisfies` against BOTH contract types, so a rename on
 * either side fails the build instead of silently sending a key the server ignores.
 */
export const UNASSIGNED_HOLD_KEY = "unassigned_hold_hours" as const satisfies keyof identity_suaSLAVao &
  keyof identity_dongSLARa;

/** The five figures that are always set — every key of the contract except the sixth. */
export type RequiredHoursKey = Exclude<keyof identity_suaSLAVao, typeof UNASSIGNED_HOLD_KEY>;

/**
 * Sáu con số của một dòng, dạng màn hình gửi lên: **vắng nghĩa là KHÔNG ĐỔI**.
 *
 * KHOÁ LẤY TỪ HỢP ĐỒNG (`keyof identity_suaSLAVao`), KHÔNG GÕ LẠI. Ngày hợp đồng có con số thứ
 * bảy, kiểu này mọc thêm khoá ấy và chỗ dựng thân yêu cầu bên dưới đỏ ngay — chứ không lặng lẽ bỏ
 * quên một ô.
 *
 * NĂM CON SỐ ĐẦU LÀ `number | undefined` CHỨ KHÔNG PHẢI `number | null`: `JSON.stringify` bỏ hẳn
 * trường `undefined` khỏi thân, còn `null` thì đi lên thật. Máy chủ đọc "không nhắc tới" là "giữ
 * nguyên" (`suaSLAVao` toàn con trỏ), nên gửi `null` cho năm cột ấy là gửi một giá trị nó từ chối.
 *
 * CON SỐ THỨ SÁU CÓ BA TRẠNG THÁI, và `null` ở đây là một ý định thật: vắng = giữ nguyên, `null` =
 * xoá ngưỡng ("không báo"), một số = đặt ngưỡng (`service-identity/internal/http/sla.go:286-288`).
 * Kiểu giá trị lấy từ phía ĐỌC của hợp đồng (`identity_dongSLARa`), vì bộ sinh kiểu ra
 * `identity_optionalHoursIn = Record<string, never>` cho phía ghi — nó không nhìn thấy
 * `UnmarshalJSON` tự viết của máy chủ. Dùng thẳng kiểu sinh ra ấy thì không gửi được con số nào.
 */
export type SuaGioVao = Partial<Record<RequiredHoursKey, number>> & {
  [UNASSIGNED_HOLD_KEY]?: identity_dongSLARa[typeof UNASSIGNED_HOLD_KEY];
};

/**
 * PATCH /api/v1/sla/{id} — sửa sáu con số của một dòng.
 *
 * ⚠ KHÔNG HỒI TỐ. Hạn của hồ sơ đã tiếp nhận nằm trên chính hồ sơ (`han_tiep_nhan`,
 * `han_xu_ly_xong` ở `service-petitions` và `service-documents`), được chốt MỘT lần tại hành vi
 * cố định nó và không bao giờ tính lại (luật 10, bất biến 2; ADR 0028). `service-identity` cũng
 * không có đường nào ghi sang hai lược đồ ấy. Màn hình PHẢI nói câu đó ra
 * (`docs/ui-ux/14-cau-hinh.md:289`) — nếu không, cán bộ sẽ tưởng vừa sửa xong là mọi hồ sơ đang
 * chạy đổi hạn theo.
 *
 * KHÔNG CÓ `work_kind`, KHÔNG CÓ `field` TRONG THÂN, và đó không phải sơ suất: dòng này áp cho
 * cái gì được cố định lúc tạo dòng. Một trường lĩnh vực ở đây sẽ cho phép màn hình lặng lẽ trỏ
 * lại một cam kết đang có sang lĩnh vực khác, và nó đúng là tham số cần đối chiếu bộ mã tầng 1
 * mà ADR 0026 điều kiện dừng #2 chặn.
 *
 * GỬI THÂN RỖNG BỊ TỪ CHỐI 400, có chủ ý: `{}` nghĩa là màn hình đọc hỏng biểu mẫu của chính nó,
 * và trả 200 sẽ nói với cán bộ rằng sửa đổi đã lưu. Cũng 400 khi số giờ Báo Chủ tịch nhỏ hơn số
 * giờ Báo lãnh đạo trực tiếp của DÒNG SAU KHI SỬA (quyết định 29/09/2026, ADR 0029 §Bổ sung 29/09);
 * câu từ chối là câu của máy chủ, trả về nguyên văn.
 */
export async function suaThoiHanXuLy(
  id: string,
  than: SuaGioVao,
): Promise<KetQua<identity_dongSLARa>> {
  // DỰNG TỪNG TRƯỜNG, KHÔNG TRẢI TỪ ĐỐI TƯỢNG NGUỒN. Kiểu ánh xạ đòi ĐỦ sáu khoá, nên bỏ quên
  // một ô là lỗi biên dịch; một `...than` thì nhận cả những khoá hợp đồng không có.
  const thanGui: {
    [K in keyof identity_suaSLAVao]: K extends typeof UNASSIGNED_HOLD_KEY
      ? number | null | undefined
      : number | undefined;
  } = {
    acknowledge_hours: than.acknowledge_hours,
    resolve_hours: than.resolve_hours,
    due_soon_hours: than.due_soon_hours,
    escalate_leader_hours: than.escalate_leader_hours,
    escalate_president_hours: than.escalate_president_hours,
    // `undefined` drops the key (unchanged); `null` travels and clears the threshold.
    unassigned_hold_hours: than.unassigned_hold_hours,
  };

  const mau: identity_patch_sla_by_id["duongDan"] = "/api/v1/sla/{id}";
  const kq = await goiGhi(mau.replace("{id}", encodeURIComponent(id)), "PATCH", thanGui, 200);
  return docThanKetQua<identity_dongSLARa>(kq);
}

/**
 * POST /api/v1/sla/defaults — gieo bộ thời hạn mặc định cho xã chưa cấu hình.
 *
 * KHÔNG GỬI THÂN VÀ KHÔNG KHAI `Content-Type`: hợp đồng khai `than: never`, và tuyến không đọc
 * thân nào. Gửi `{}` là tuyên bố có một thân — thứ sẽ mời người sau điền vào.
 *
 * 200 CHỨ KHÔNG PHẢI 201, kể cả ở lần gieo đầu: yêu cầu không tạo ra một tài nguyên có địa chỉ,
 * nó đưa bảng cấu hình của xã về một trạng thái đã biết.
 *
 * IDEMPOTENT: bấm lần thứ hai không ghi thêm dòng nào và KHÔNG ghi đè con số xã đã sửa — kết quả
 * trả về `{seeded: 0, kept: 15}`. Vì thế không có `Idempotency-Key` (hợp đồng cũng không đòi):
 * chống trùng đã nằm trong chính phép ghi.
 */
export async function gieoThoiHanMacDinh(): Promise<KetQua<identity_gieoSLARa>> {
  const duongDan: identity_post_sla_defaults["duongDan"] = "/api/v1/sla/defaults";
  const kq = await goiGhi(duongDan, "POST", undefined, 200);
  return docThanKetQua<identity_gieoSLARa>(kq);
}
