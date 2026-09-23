/**
 * Hai tuyến của màn "Thông báo" (`docs/ui-ux/08-thong-bao.md`).
 *
 *   GET  /api/v1/announcements   `announcement.create`  — §2, §3 sổ thông báo
 *   POST /api/v1/announcements   `announcement.create`  — §5 phát hành
 *
 * HAI TUYẾN, KHÔNG PHẢI TÁM. §7 phác ra tám đường (`/:id`, `/phat-hanh`, `/go`, `/xac-nhan`,
 * `/da-mo`, `/chua-doc`); hợp đồng hôm nay chỉ có hai. Không hàm nào ở đây dựng sẵn một đường
 * chưa tồn tại — một hàm gọi vào tuyến không có là một hàm biên dịch được, kiểm được bằng
 * `fetch` giả, và 404 ở lần chạy thật. Từng thứ thiếu và lý do nằm ở `PHAN_CHUA_DUNG` trong
 * `features/thong-bao/nhan-thong-bao.ts`, và chúng hiện RA MÀN HÌNH.
 *
 * MỘT KHOÁ QUYỀN CHO CẢ ĐỌC LẪN GHI, và đó là quyết định của máy chủ chứ không phải một lần gộp
 * cho gọn ở đây: bảng `quyen` chỉ có `announcement.create` trong nhóm `THÔNG BÁO`, không có khoá
 * đọc nào (`service-comms/internal/http/thong_bao_noi_bo.go`, đầu tệp). Đó là cấp THIẾU chứ không
 * cấp THỪA, và nó là câu hỏi mở #27 — không phải một dòng `INSERT INTO quyen`.
 *
 * KHÔNG CHÉP TAY MỘT HÌNH DẠNG NÀO. Ba kiểu dưới đều là bí danh của kiểu SINH RA từ
 * `kb/20-contracts/openapi.json` (`npm run check:api` xanh ngày 23/09/2026). Bài kiểm kèm theo đọc
 * THẲNG tệp hợp đồng và so bộ khoá THẬT SỰ đi trên dây — bí danh chỉ bảo đảm hai kiểu khớp nhau,
 * còn bài kiểm ấy bảo đảm thứ hàm này gửi đi khớp với hợp đồng.
 */

import { docJSON, docThanKetQua, goiGhi, type KetQua } from "./goi";
import type {
  comms_get_announcements,
  comms_phatHanhThongBaoVao,
  comms_thongBaoRa,
  page_Result_comms_thongBaoRa,
} from "./schema.gen";

/**
 * Phân trang của quyển sổ. Tuyến KHÔNG nhận bộ lọc nào.
 *
 * KHÔNG CÓ `pham_vi`, và sự vắng mặt ấy là của MÁY CHỦ: §2 phác hai phạm vi
 * (`gui-cho-toi` · `ca-so`), còn tuyến cố ý không nhận tham số ấy thay vì nhận rồi lặng lẽ bỏ qua
 * một giá trị — một tham số được đọc mà không có tác dụng là cách một màn hình mang nhãn
 * "Gửi cho tôi" hiện ra cả sổ. Xem `PHAN_CHUA_DUNG`.
 *
 * KHÔNG CÓ `tenant_id` Ở BẤT KỲ ĐÂU, và không được thêm: xã suy từ `Host` ở rìa ngoài cùng, còn
 * client tự khai xã là client tự cấp quyền (luật 1, cấm #2).
 */
export type TrangThongBao = {
  limit?: number;
  /** `null` là trang đầu — xem `features/cau-hinh/ngan-xep-con-tro.ts`. */
  cursor?: string | null;
};

/**
 * Dựng đường dẫn danh sách. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`.
 *
 * KHÔNG GỬI `sort` VÀ `order`: máy chủ chỉ cho `created_at` và mặc định đã là giảm dần, tức
 * "mới nhất ở trên" của §3. Gửi lại đúng giá trị mặc định chỉ thêm một chỗ có thể lệch.
 */
export function duongDanSoThongBao(trang: TrangThongBao): string {
  const duongDan: comms_get_announcements["duongDan"] = "/api/v1/announcements";
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

/** GET /api/v1/announcements — một trang thẻ thông báo, kèm bộ đếm xác nhận. */
export function laySoThongBao(
  trang: TrangThongBao,
): Promise<KetQua<page_Result_comms_thongBaoRa>> {
  return docJSON<page_Result_comms_thongBaoRa>(duongDanSoThongBao(trang));
}

/**
 * Thân của `POST /api/v1/announcements` — §5. Bí danh của kiểu SINH RA từ hợp đồng.
 *
 * `recipient_codes` LÀ MÃ NGHIỆP VỤ CÁN BỘ (`CB-2026-7K3M9Q`), không phải id nội bộ và không phải
 * họ tên. Máy chủ so chính giá trị ấy với `.Ma` của phiên khi dựng bộ lọc "Gửi cho tôi", nên một
 * loại định danh thứ hai sẽ làm bộ lọc ấy không viết được.
 *
 * `org_unit_ids` CÓ TRONG HỢP ĐỒNG VÀ MÁY CHỦ TỪ CHỐI NÓ BẰNG **501**. Đó là từ chối CÓ CHỦ Ý:
 * nở một bộ phận thành danh sách cán bộ là dữ liệu của `identity`, và chưa có RPC nào làm việc
 * ấy (`service-comms/internal/app/thong_bao_noi_bo.go`, `ErrGuiTheoBoPhanChuaCo`). Trường vẫn nằm
 * trong kiểu vì hợp đồng khai nó — nhưng `phatHanhThongBao` KHÔNG BAO GIỜ gửi nó, xem dưới.
 *
 * KHÔNG CÓ `status` VÀ KHÔNG CÓ `author_code`, và cả hai là TỪ CHỐI của máy chủ chứ không phải bỏ
 * sót: một thân tự khai trạng thái là một thông báo `da-phat-hanh` không có người nhận nào, còn
 * một thân tự khai tác giả là một cán bộ phát thông báo dưới tên đồng nghiệp (luật 6, bất biến 8).
 */
export type PhatHanhThongBaoVao = comms_phatHanhThongBaoVao;

/**
 * POST /api/v1/announcements — phát hành một thông báo tới các cán bộ được chọn đích danh. 201,
 * trả về cả tấm thẻ.
 *
 * TRẢ VỀ CẢ THẺ VÌ `recipient_count` LÀ CON SỐ NGƯỜI SOẠN CẦN THẤY NHẤT và là thứ duy nhất họ
 * không tự kiểm được từ biểu mẫu vừa gửi.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ. Hợp đồng đòi `Idempotency-Key` bắt buộc. Sinh
 * khoá bên trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới — tức đúng cái khoá
 * chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ đã tới máy chủ và đã phát một thông báo đi
 * khắp xã. Khoá do biểu mẫu giữ, sống bằng đời một lần mở form.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG `...than`: một phép trải ở đây là đường để một trường lạ đi lên máy chủ
 * vào ngày ai đó truyền vào một đối tượng vừa đọc được từ nơi khác.
 *
 * ⚠ `org_unit_ids` KHÔNG CÓ TRONG THÂN GỬI ĐI, VÀ ĐÓ LÀ CHỖ QUAN TRỌNG NHẤT CỦA HÀM NÀY. Gửi nó
 * đi — kể cả một mảng RỖNG — là mời màn hình dựng một ô chọn bộ phận mà mọi lần bấm đều nhận 501.
 * Màn hình vì thế không có ô ấy, và tuyến này không có đường nào để một ô như thế đi qua. Ngày
 * `identity` có RPC nở bộ phận, chỗ này là một trường được thêm lại — không phải một khiếm khuyết
 * được sửa.
 */
export function phatHanhThongBao(
  than: PhatHanhThongBaoVao,
  khoaChongTrung: string,
): Promise<KetQua<comms_thongBaoRa>> {
  const duongDan: comms_get_announcements["duongDan"] = "/api/v1/announcements";

  const thanGui: PhatHanhThongBaoVao = {
    title: than.title,
    body: than.body,
    recipient_codes: than.recipient_codes,
    pinned: than.pinned,
    ack_required: than.ack_required,
    email_requested: than.email_requested,
  };

  return goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }).then(
    docThanKetQua<comms_thongBaoRa>,
  );
}
