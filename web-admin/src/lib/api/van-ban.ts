/**
 * Chín tuyến của **sổ văn bản đến** và **sổ văn bản đi** — `service-documents`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * SỐ ĐẾN VÀ SỐ ĐI LÀ THỨ KHÔNG LẤY LẠI ĐƯỢC. Máy chủ cấp số dưới một khoá dòng và KHÔNG BAO GIỜ
 * cấp lại một số đã cấp, kể cả sau khi bản ghi bị gỡ khỏi sổ (luật 7, bất biến 3). Hệ quả cho
 * tệp này, và nó quyết định hai chi tiết bên dưới chứ không phải một lời rào đón:
 *
 *   1. `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TRONG HÀM — ở cả hai tuyến `POST`. Sinh khoá mới
 *      mỗi lần gọi thì một lần bấm lại sau lỗi mạng là một khoá MỚI, tức đúng cái mà khoá chống
 *      trùng sinh ra để chặn: lần gửi đầu CÓ THỂ đã tới máy chủ và đã tiêu một số. Hai lần bấm
 *      là hai số biến khỏi dãy, và không thủ tục nào lấy chúng về.
 *   2. KHÔNG TUYẾN NÀO Ở ĐÂY GỬI `number` LÊN, và kiểu `Omit<…, "number">` chặn ngay lúc biên
 *      dịch. Máy chủ trả 400 nếu thân NHẮC TỚI số — một client đặt được số là một client ghi
 *      được số đã nằm trên một văn bản đã đóng dấu gửi đi (`van_ban_di.go`, `capSoVanBanDiVao`).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ `status`, KHÔNG CÓ `due_at` TRONG BẤT KỲ THÂN NÀO, cùng một lẽ: trạng thái là hằng số
 * trong câu INSERT, còn hạn xử lý do `identity.ResolveDeadlines` tính theo bảng thời hạn và lịch
 * làm việc CỦA XÃ ẤY (ADR 0007). Cả ba trường có mặt trong hợp đồng ĐỂ BỊ TỪ CHỐI, nên chúng bị
 * loại khỏi kiểu đầu vào ở đây — `tsc` đỏ trước khi ai kịp gõ chúng vào một biểu mẫu.
 *
 * VÀ KHÔNG CÓ TRƯỜNG `overdue` Ở BẤT KỲ ĐÂU — không trong hợp đồng, không trong thân gửi lên,
 * không trong một biến nào của màn hình. Quá hạn là phép SO SÁNH `due_at` với hiện tại, suy ra
 * lúc vẽ (luật 10, bất biến 3). Một giá trị boolean đóng băng lúc dựng phản hồi, còn màn hình thì
 * đứng trên máy cán bộ hàng giờ sau đó.
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: ba quy tắc
 * ấy đúng cho MỌI tuyến và được nói một lần ở `goi.ts`. Đọc tệp ấy trước khi sửa gì ở đây.
 */

import { LOI_KHONG_RO, docJSON, goiGhi, type KetQua } from "./goi";
import type {
  documents_capSoVanBanDiVao,
  documents_chuyenVanBanVao,
  documents_delete_incoming_documents_by_id,
  documents_delete_outgoing_documents_by_id,
  documents_get_incoming_documents,
  documents_get_outgoing_documents,
  documents_goVanBanVao,
  documents_patch_incoming_documents_by_id,
  documents_patch_outgoing_documents_by_id,
  documents_post_incoming_documents,
  documents_post_incoming_documents_by_id_routings,
  documents_post_outgoing_documents,
  documents_suaVanBanDenVao,
  documents_suaVanBanDiVao,
  documents_themVanBanDenVao,
  documents_vanBanDenRa,
  documents_vanBanDiRa,
  page_Result_documents_vanBanDenRa,
  page_Result_documents_vanBanDiRa,
} from "./schema.gen";

/**
 * Đọc thân của một lần ghi thành công.
 *
 * `goiGhi` TRẢ `Response` THÔ, và đó là lý do lớp mỏng này tồn tại: hai tuyến `DELETE` thành công
 * bằng **204 không thân**, nên một hàm dùng chung luôn gọi `.json()` sẽ biến một lần gỡ thành
 * công thành "không đọc được". Tuyến nào biết mình trả gì thì tự đọc lấy.
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

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BỘ LỌC VÀ PHÂN TRANG
 *
 * ⚠ HAI TUYẾN ĐỌC KHÔNG KHAI THAM SỐ TRUY VẤN NÀO TRONG HỢP ĐỒNG, và đó là một PHÁT HIỆN chứ
 * không phải một chỗ để bịa. `kb/20-contracts/openapi.json` không có `parameters` cho
 * `GET /api/v1/incoming-documents` lẫn `GET /api/v1/outgoing-documents`, nên `schema.gen.ts` sinh
 * ra `truyVan: {}` — trong khi handler THẬT SỰ đọc năm bộ lọc và bốn tham số trang
 * (`service-documents/internal/http/van_ban_den.go:450` `locVanBanDenTuQuery`, `:400` `page.Parse`;
 * `van_ban_di.go:260` `locVanBanDiTuQuery`). Nguyên nhân: hai tuyến ấy thiếu chú thích `@page`
 * mà `tools/apidoc` đọc — `GET /api/v1/staff` có nó nên phát đủ `limit/cursor/sort/order`.
 *
 * Hệ quả, và nó được nói ra chứ không giấu đi: tên năm bộ lọc dưới đây LẤY TỪ MÃ NGUỒN TUYẾN,
 * không từ hợp đồng, nên `tsc` KHÔNG đỏ vào ngày máy chủ đổi tên một tham số. Đã ghi vào báo cáo
 * để lượt backend bổ sung chú thích; khi có, kiểu ở đây rút về `…["truyVan"]` như `can-bo.ts`.
 *
 * KHÔNG GỬI `sort` VÀ `order`. Danh sách trắng khoá sắp xếp (`docstore.SapXepVanBanDen`) cũng
 * không có trong hợp đồng, nên một khoá gõ tay ở đây là đúng bản chép mà `can-bo.ts` đã phải gỡ
 * bỏ một lần: thêm hay bớt một cột ở máy chủ thì không bài kiểm nào đỏ, chỉ có một màn hình nhận
 * 400 mà cán bộ đọc ra là "không tải được". Không gửi thì máy chủ áp mặc định của nó — số vào sổ
 * giảm dần, đúng thứ tự `05-van-ban-don-thu §3.1` vẽ.
 * ══════════════════════════════════════════════════════════════════════════════════════════
 */

/** Một yêu cầu trang. `cursor` là chuỗi MỜ ĐỤC máy chủ phát ra, client chỉ chuyền lại nguyên văn. */
export type ThamSoTrangVanBan = {
  limit?: number;
  cursor?: string | null;
};

/** Năm bộ lọc của sổ văn bản đến — `van_ban_den.go:450`. Trường vắng mặt nghĩa là KHÔNG lọc. */
export type LocVanBanDen = ThamSoTrangVanBan & {
  /** `year` — máy chủ từ chối ngoài khoảng 2000–2200. */
  nam?: number;
  /** `status` — một trong sáu mã của `05-van-ban-don-thu §3.2`. Mã lạ bị TỪ CHỐI, không bỏ qua. */
  trangThai?: string;
  /** `document_type` — MÃ trong danh mục Loại văn bản của xã, không phải nhãn. */
  loaiVanBan?: string;
  /** `holding_unit` — id bộ phận đang giữ hồ sơ. */
  boPhanDangGiu?: string;
  /** `q` — tìm chữ; máy chủ từ chối chuỗi dài quá 200 ký tự. */
  tim?: string;
};

/** Ba bộ lọc của sổ văn bản đi — `van_ban_di.go:260`. Sổ đi KHÔNG có trạng thái và không có bộ phận. */
export type LocVanBanDi = ThamSoTrangVanBan & {
  nam?: number;
  loaiVanBan?: string;
  tim?: string;
};

/**
 * Thêm phần chung của hai bộ lọc vào chuỗi truy vấn.
 *
 * Tham số nào không đặt thì KHÔNG xuất hiện trong URL — để máy chủ áp mặc định của nó, thay vì
 * ứng dụng web giữ một bản sao thứ hai của các mặc định ấy rồi trôi khỏi bản của máy chủ.
 */
function themPhanChung(truyVan: URLSearchParams, loc: LocVanBanDi): void {
  if (loc.nam !== undefined) truyVan.set("year", String(loc.nam));
  if (loc.loaiVanBan !== undefined && loc.loaiVanBan !== "") {
    truyVan.set("document_type", loc.loaiVanBan);
  }
  if (loc.tim !== undefined && loc.tim !== "") truyVan.set("q", loc.tim);
  if (loc.limit !== undefined) truyVan.set("limit", String(loc.limit));
  // Con trỏ rỗng nghĩa là trang đầu. Gửi `cursor=` rỗng thì máy chủ trả 400 "con trỏ không hợp
  // lệ" — đúng vào lần mở màn hình đầu tiên.
  if (loc.cursor !== undefined && loc.cursor !== null && loc.cursor !== "") {
    truyVan.set("cursor", loc.cursor);
  }
}

/** Dựng đường dẫn sổ văn bản đến. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`. */
export function duongDanSoVanBanDen(loc: LocVanBanDen = {}): string {
  const duongDan: documents_get_incoming_documents["duongDan"] = "/api/v1/incoming-documents";
  const truyVan = new URLSearchParams();

  themPhanChung(truyVan, loc);
  if (loc.trangThai !== undefined && loc.trangThai !== "") truyVan.set("status", loc.trangThai);
  if (loc.boPhanDangGiu !== undefined && loc.boPhanDangGiu !== "") {
    truyVan.set("holding_unit", loc.boPhanDangGiu);
  }

  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/** Dựng đường dẫn sổ văn bản đi. */
export function duongDanSoVanBanDi(loc: LocVanBanDi = {}): string {
  const duongDan: documents_get_outgoing_documents["duongDan"] = "/api/v1/outgoing-documents";
  const truyVan = new URLSearchParams();

  themPhanChung(truyVan, loc);

  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/* ---- sổ văn bản đến -------------------------------------------------------------------- */

/**
 * GET /api/v1/incoming-documents — một trang của sổ văn bản đến.
 *
 * Máy chủ đòi `document.read` VÀ KHÔNG CHE BỚT CỘT NÀO cho người có khoá ấy: `summary` và
 * `issuing_body` là chữ tự do có thể nhắc tên một công dân (luật 3), và chúng ra nguyên văn cho
 * cán bộ CỦA CHÍNH XÃ ẤY — đó là công dụng của quyển sổ. Việc của màn hình là không bao giờ ghi
 * hai giá trị ấy vào log, vào tên tệp hay vào một URL.
 */
export function laySoVanBanDen(
  loc: LocVanBanDen = {},
): Promise<KetQua<page_Result_documents_vanBanDenRa>> {
  return docJSON<page_Result_documents_vanBanDenRa>(duongDanSoVanBanDen(loc));
}

/**
 * Thân của một lần vào sổ — BA TRƯỜNG CỦA HỢP ĐỒNG BỊ LOẠI BỎ.
 *
 * `number`, `status`, `due_at` có mặt trong `themVanBanDenVao` **chỉ để máy chủ từ chối chúng**
 * (`van_ban_den.go:135`). Gửi bất kỳ trường nào trong ba là 400. Loại chúng khỏi kiểu đầu vào ở
 * đây nghĩa là một biểu mẫu vẽ ô "Số đến" sẽ đỏ ở `tsc`, chứ không đỏ ở tay cán bộ.
 */
export type VaoSoVanBanDenVao = Omit<documents_themVanBanDenVao, "number" | "status" | "due_at">;

/**
 * POST /api/v1/incoming-documents — vào sổ một văn bản đến. 201, trả về dòng vừa vào sổ.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ — xem khối chú thích đầu tệp. Hợp đồng khai
 * `Idempotency-Key` BẮT BUỘC trên tuyến này (`idem.Required(idem.DongKhiHong)`).
 *
 * 409 Ở TUYẾN NÀY THƯỜNG KHÔNG PHẢI LỖI NHẬP LIỆU. Nó là câu `sla_chua_cau_hinh`: xã chưa khai
 * bảng Thời hạn xử lý hoặc Giờ làm việc, nên `identity.ResolveDeadlines` không ấn định được hạn
 * và máy chủ TỪ CHỐI thay vì đặt một hạn mặc định (luật 10, cấm #3). Câu của máy chủ chỉ thẳng
 * màn hình sửa được nó, và việc của giao diện là đưa câu ấy ra trang NGUYÊN VĂN.
 */
export function vaoSoVanBanDen(
  than: VaoSoVanBanDenVao,
  khoaChongTrung: string,
): Promise<KetQua<documents_vanBanDenRa>> {
  const duongDan: documents_post_incoming_documents["duongDan"] = "/api/v1/incoming-documents";

  // DỰNG TỪNG TRƯỜNG, KHÔNG `...than`. Một phép trải ở đây là đường để một trường lạ — `number`,
  // `status`, `due_at`, hay một trường `overdue` ai đó dựng ở client — đi lên máy chủ vào ngày có
  // người truyền vào một dòng vừa đọc được. Kiểu chặn lúc biên dịch, phép dựng này chặn lúc chạy.
  const thanGui: VaoSoVanBanDenVao = {
    received_date: than.received_date,
    reference_no: than.reference_no,
    document_date: than.document_date,
    issuing_body: than.issuing_body,
    document_type: than.document_type,
    summary: than.summary,
    urgency: than.urgency,
  };

  return docThanRa<documents_vanBanDenRa>(
    goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }),
  );
}

/**
 * PATCH /api/v1/incoming-documents/{id} — sửa một văn bản đã vào sổ. 200, trả về dòng sau khi sửa.
 *
 * KHÔNG SỬA ĐƯỢC BỘ PHẬN ĐANG GIỮ VÀ CÁN BỘ XỬ LÝ, và đó không phải bỏ sót: chuyển một hồ sơ
 * giữa hai bộ phận là một hành vi CÓ LÝ DO và có một dòng lịch sử — tuyến `routings` làm việc ấy.
 * Một lần sửa mà dời được hồ sơ là dời hồ sơ không để lại vết ai dời đi đâu.
 */
export type SuaVanBanDenVao = Omit<documents_suaVanBanDenVao, "number" | "status" | "due_at">;

export function suaVanBanDen(
  id: string,
  than: SuaVanBanDenVao,
): Promise<KetQua<documents_vanBanDenRa>> {
  const mau: documents_patch_incoming_documents_by_id["duongDan"] =
    "/api/v1/incoming-documents/{id}";

  // `null` NGHĨA LÀ "KHÔNG NHẮC TỚI", chuỗi rỗng nghĩa là "xoá nội dung ô này" — hai điều khác
  // hẳn nhau, và máy chủ đọc đúng như vậy (`suaVanBanDenVao` toàn con trỏ). Lẫn hai thứ ấy là xoá
  // trắng số ký hiệu của cơ quan ban hành trên một hồ sơ lưu trữ mà không ai báo gì.
  const thanGui: SuaVanBanDenVao = {
    received_date: than.received_date,
    reference_no: than.reference_no,
    document_date: than.document_date,
    issuing_body: than.issuing_body,
    document_type: than.document_type,
    summary: than.summary,
    urgency: than.urgency,
  };

  return docThanRa<documents_vanBanDenRa>(goiGhi(duongDanMot(mau, id), "PATCH", thanGui, 200));
}

/**
 * DELETE /api/v1/incoming-documents/{id} — gỡ một văn bản khỏi sổ. 204 KHÔNG THÂN.
 *
 * XOÁ MỀM, VÀ SỐ ĐẾN VẪN BỊ TIÊU. Dòng còn nguyên trong CSDL kèm `deleted_at`, `deleted_by`,
 * `delete_reason`; số đã cấp KHÔNG quay về dãy, nên quyển sổ để lại một khoảng trống nhìn thấy
 * được — chính khoảng trống ấy là bằng chứng. Màn hình PHẢI nói câu đó ra trước khi cán bộ bấm.
 *
 * LÝ DO LÀ BẮT BUỘC: luật 7 bất biến 1 kể tên ba cột, và lý do đi trong THÂN chứ không trong
 * chuỗi truy vấn — chữ tự do về một hồ sơ nhà nước mà nằm trong URL là chữ nằm lại trong mọi
 * nhật ký truy cập và mọi bộ đệm trung gian.
 */
export async function goVanBanDen(id: string, lyDo: string): Promise<KetQua<null>> {
  const mau: documents_delete_incoming_documents_by_id["duongDan"] =
    "/api/v1/incoming-documents/{id}";
  const thanGui: documents_goVanBanVao = { reason: lyDo };

  const kq = await goiGhi(duongDanMot(mau, id), "DELETE", thanGui, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/**
 * POST /api/v1/incoming-documents/{id}/routings — chuyển văn bản cho một bộ phận. 200.
 *
 * QUYỀN RIÊNG, `document.route`, VÀ NÓ KHÔNG SUY RA TỪ `document.create`: đây là hành vi quyết
 * định AI CHỊU TRÁCH NHIỆM, không phải hành vi gõ một văn bản vào sổ (đặc tả §7 quy tắc 4).
 *
 * `reason` BẮT BUỘC — máy chủ đòi, và lý do nặng hơn một phép kiểm dữ liệu: dòng lịch sử ấy
 * KHÔNG SỬA ĐƯỢC về sau (luật 7, cấm #5; trigger `lich_su_chuyen_chi_them`). Một lệnh chuyển
 * không kèm lý do là một chỉ đạo không ai truy được, và người ra lệnh đã chuyển công tác vào lúc
 * có người hỏi lại.
 *
 * KHÔNG CÓ `Idempotency-Key`, và hợp đồng cố ý không đòi: gửi lại là một lần chuyển NỮA, có thật,
 * và để lại dòng thứ hai. Một khoá chống trùng ở đây sẽ GIẤU hành vi thứ hai thay vì ghi nó.
 */
export function chuyenVanBanDen(
  id: string,
  than: documents_chuyenVanBanVao,
): Promise<KetQua<documents_vanBanDenRa>> {
  const mau: documents_post_incoming_documents_by_id_routings["duongDan"] =
    "/api/v1/incoming-documents/{id}/routings";

  const thanGui: documents_chuyenVanBanVao = {
    to_unit: than.to_unit,
    assignee: than.assignee,
    reason: than.reason,
  };

  return docThanRa<documents_vanBanDenRa>(
    goiGhi(duongDanMot(mau, id), "POST", thanGui, 200),
  );
}

/* ---- sổ văn bản đi --------------------------------------------------------------------- */

/**
 * GET /api/v1/outgoing-documents — một trang của sổ văn bản đi.
 *
 * ⚠ SỔ ĐI KHÔNG CÓ ĐẶC TẢ NÀO (`x-vigov-screen` của tuyến ghi rõ *"chưa có đặc tả"*). Trường của
 * nó do lượt backend đặt, và `vanBanDiRa` CỐ Ý không có `status`, không có `due_at`: một văn bản
 * đi không có vòng đời và không mang cam kết nào — phát hành CHÍNH LÀ hành vi ấy
 * (`van_ban_di.go:40`). Không tầng nào được bịa thêm một quy trình cho nó.
 */
export function laySoVanBanDi(
  loc: LocVanBanDi = {},
): Promise<KetQua<page_Result_documents_vanBanDiRa>> {
  return docJSON<page_Result_documents_vanBanDiRa>(duongDanSoVanBanDi(loc));
}

/** Thân cấp số văn bản đi — `number` bị loại: xem khối chú thích đầu tệp. */
export type CapSoVanBanDiVao = Omit<documents_capSoVanBanDiVao, "number">;

/**
 * POST /api/v1/outgoing-documents — cấp số và ghi vào sổ. 201, trả về dòng vừa cấp.
 *
 * LẦN GHI NẶNG NHẤT CỦA CẢ HAI QUYỂN SỔ: con số tuyến này cấp ra được in lên giấy, đóng dấu, và
 * rời khỏi trụ sở. Vì vậy `Idempotency-Key` là BẮT BUỘC và khoá phải do biểu mẫu giữ — một lần
 * bấm lại với khoá mới là một số thứ hai cấp cho một văn bản đã có số.
 */
export function capSoVanBanDi(
  than: CapSoVanBanDiVao,
  khoaChongTrung: string,
): Promise<KetQua<documents_vanBanDiRa>> {
  const duongDan: documents_post_outgoing_documents["duongDan"] = "/api/v1/outgoing-documents";

  const thanGui: CapSoVanBanDiVao = {
    document_date: than.document_date,
    document_type: than.document_type,
    summary: than.summary,
    recipient: than.recipient,
    signer: than.signer,
  };

  return docThanRa<documents_vanBanDiRa>(
    goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }),
  );
}

/** Thân sửa một văn bản đi đã cấp số. `number` và năm không sửa được — chúng đã nằm trên giấy. */
export type SuaVanBanDiVao = Omit<documents_suaVanBanDiVao, "number">;

/** PATCH /api/v1/outgoing-documents/{id} — sửa một dòng đã cấp số. 200. */
export function suaVanBanDi(
  id: string,
  than: SuaVanBanDiVao,
): Promise<KetQua<documents_vanBanDiRa>> {
  const mau: documents_patch_outgoing_documents_by_id["duongDan"] =
    "/api/v1/outgoing-documents/{id}";

  const thanGui: SuaVanBanDiVao = {
    document_date: than.document_date,
    document_type: than.document_type,
    summary: than.summary,
    recipient: than.recipient,
    signer: than.signer,
  };

  return docThanRa<documents_vanBanDiRa>(goiGhi(duongDanMot(mau, id), "PATCH", thanGui, 200));
}

/**
 * DELETE /api/v1/outgoing-documents/{id} — gỡ một dòng khỏi sổ đi. 204 KHÔNG THÂN.
 *
 * SỐ ĐI VẪN BỊ TIÊU, và ở quyển sổ này điều đó còn dứt khoát hơn: văn bản mang số ấy đã rời khỏi
 * xã. Gỡ dòng khỏi màn hình không gỡ được con số khỏi tờ giấy đang nằm ở một cơ quan khác.
 */
export async function goVanBanDi(id: string, lyDo: string): Promise<KetQua<null>> {
  const mau: documents_delete_outgoing_documents_by_id["duongDan"] =
    "/api/v1/outgoing-documents/{id}";
  const thanGui: documents_goVanBanVao = { reason: lyDo };

  const kq = await goiGhi(duongDanMot(mau, id), "DELETE", thanGui, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}
