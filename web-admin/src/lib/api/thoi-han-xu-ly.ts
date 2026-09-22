/**
 * Ba tuyến của **bảng thời hạn xử lý** một xã — `GET /api/v1/sla`, `PATCH /api/v1/sla/{id}`,
 * `POST /api/v1/sla/defaults`.
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
 * CẢ BA TUYẾN ĐÒI `admin.sla`, KỂ CẢ TUYẾN ĐỌC — khác hẳn ba tuyến đọc lịch làm việc
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

import { LOI_KHONG_RO, docJSON, goiGhi, type KetQua } from "./goi";
import type {
  identity_danhSachSLARa,
  identity_dongSLARa,
  identity_get_sla,
  identity_gieoSLARa,
  identity_patch_sla_by_id,
  identity_post_sla_defaults,
  identity_suaSLAVao,
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
 * Năm con số của một dòng, dạng màn hình gửi lên: **vắng nghĩa là KHÔNG ĐỔI**.
 *
 * KHOÁ LẤY TỪ HỢP ĐỒNG (`keyof identity_suaSLAVao`), KHÔNG GÕ LẠI. Ngày hợp đồng có con số thứ
 * sáu, kiểu này mọc thêm khoá ấy và chỗ dựng thân yêu cầu bên dưới đỏ ngay — chứ không lặng lẽ bỏ
 * quên một ô.
 *
 * `number | undefined` CHỨ KHÔNG PHẢI `number | null`: `JSON.stringify` bỏ hẳn trường `undefined`
 * khỏi thân, còn `null` thì đi lên thật. Máy chủ đọc "không nhắc tới" là "giữ nguyên"
 * (`suaSLAVao` toàn con trỏ), nên gửi `null` là gửi một giá trị nó sẽ phải từ chối.
 */
export type SuaGioVao = Partial<Record<keyof identity_suaSLAVao, number>>;

/**
 * PATCH /api/v1/sla/{id} — sửa năm con số của một dòng.
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
 * và trả 200 sẽ nói với cán bộ rằng sửa đổi đã lưu.
 */
export async function suaThoiHanXuLy(
  id: string,
  than: SuaGioVao,
): Promise<KetQua<identity_dongSLARa>> {
  // DỰNG TỪNG TRƯỜNG, KHÔNG TRẢI TỪ ĐỐI TƯỢNG NGUỒN. Kiểu ánh xạ đòi ĐỦ năm khoá, nên bỏ quên
  // một ô là lỗi biên dịch; một `...than` thì nhận cả những khoá hợp đồng không có.
  const thanGui: { [K in keyof identity_suaSLAVao]: number | undefined } = {
    acknowledge_hours: than.acknowledge_hours,
    resolve_hours: than.resolve_hours,
    due_soon_hours: than.due_soon_hours,
    escalate_leader_hours: than.escalate_leader_hours,
    escalate_president_hours: than.escalate_president_hours,
  };

  const mau: identity_patch_sla_by_id["duongDan"] = "/api/v1/sla/{id}";
  const kq = await goiGhi(mau.replace("{id}", encodeURIComponent(id)), "PATCH", thanGui, 200);
  if (!kq.ok) return kq;
  try {
    return { ok: true, duLieu: (await kq.duLieu.json()) as identity_dongSLARa };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
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
  if (!kq.ok) return kq;
  try {
    return { ok: true, duLieu: (await kq.duLieu.json()) as identity_gieoSLARa };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}
