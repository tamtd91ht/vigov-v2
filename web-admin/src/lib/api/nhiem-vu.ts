/**
 * Tám tuyến của sổ Nhiệm vụ (`docs/ui-ux/02-nhiem-vu.md`), đúng bộ tuyến
 * `service-petitions/internal/http/routes.go:773-982` khai — không nhiều hơn, không ít hơn.
 *
 *   GET    /api/v1/tasks                                       task.read
 *   GET    /api/v1/tasks/{ma}                                  task.read
 *   POST   /api/v1/tasks                                       task.create  + Idempotency-Key
 *   PATCH  /api/v1/tasks/{ma}                                  task.update
 *   POST   /api/v1/tasks/{ma}/status                           task.update  (+ task.approve ở tầng
 *                                                              nghiệp vụ cho bước `hoan-thanh`)
 *   DELETE /api/v1/tasks/{ma}                                  task.delete
 *   POST   /api/v1/tasks/{ma}/extensions                       task.update  ← KHÔNG phải task.extend
 *   POST   /api/v1/tasks/{ma}/extensions/{deNghiID}/decision   task.extend  + ADR 0038 lớp hai
 *
 * HAI DÒNG CUỐI KHÔNG ĐƯỢC GỘP, và đó là toàn bộ ADR 0038: `task.extend` nhãn là **"Duyệt gia
 * hạn"** — quyền QUYẾT ĐỊNH. Gắn nó lên tuyến ĐỀ NGHỊ sẽ thành "chỉ người duyệt được mới xin
 * được", đúng điều ngược lại với §5.8, nơi ô đề nghị nằm trên drawer của người đang làm việc.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `petitions_nhiemVuRa`, `petitions_taoNhiemVuVao`,
 * `petitions_suaNhiemVuVao`, `petitions_doiTrangThaiVao`, `petitions_xoaNhiemVuVao`,
 * `petitions_deNghiLuiHanVao`, `petitions_quyetDinhLuiHanVao`, `petitions_deNghiLuiHanRa`,
 * `page_Result_petitions_nhiemVuRa` đều đến từ `schema.gen.ts`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * THAM SỐ TRUY VẤN CỦA TUYẾN DANH SÁCH NAY ĐÃ ĐƯỢC HỢP ĐỒNG KHAI: `petitions_get_tasks["truyVan"]`
 * (`schema.gen.ts`) có `status` · `source` · `type` · `bloc` · `priority` · `unit` · `assignee` ·
 * `q` · `late` · `scope` · `soon` cùng `limit` · `cursor` · `sort` · `order`. Khối này từng viết
 * rằng đối tượng ấy RỖNG — đúng lúc viết, hết đúng khi tuyến có chú thích `@query`.
 *
 * ⚠ ĐIỀU VẪN CÒN ĐÚNG: `themLocVaoTruyVan` ghép tên tham số bằng CHUỖI TRẦN vào `URLSearchParams`,
 * nên `tsc` CHƯA đối chiếu những cái tên ấy với kiểu `truyVan` — gõ sai một tên thì máy chủ bỏ
 * qua nó và trả cả quyển sổ trong khi cán bộ tin mình đang xem một lát cắt. Vì thế mười cái tên
 * nằm trong ĐÚNG MỘT hàm và có bài kiểm đọc lại từng tên; nối chúng vào kiểu `truyVan` là việc
 * nên làm tiếp, ở đúng hàm ấy.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * HAI BỘ LỌC CỦA §3 KHÔNG CÓ MẶT Ở ĐÂY VÀ SỰ VẮNG MẶT LÀ CHỦ Ý — máy chủ **TỪ CHỐI** chúng bằng
 * 400 kèm lý do chứ không bỏ qua: `scope=related` cần "bộ phận tôi đang giữ" mà phiên cán bộ
 * không mang, `soon=` cần `sla.gio_sap_den_han` của TỪNG XÃ mà chưa có đường đọc. Gửi lên là đổi
 * một ô lọc thành một trang lỗi; nên chúng ra tới màn hình qua `PHAN_CHUA_DUNG`.
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./goi";
import type {
  page_Result_petitions_nhiemVuRa,
  petitions_deNghiLuiHanRa,
  petitions_deNghiLuiHanVao,
  petitions_delete_tasks_by_ma,
  petitions_doiTrangThaiVao,
  petitions_get_tasks,
  petitions_get_tasks_by_ma,
  petitions_nhiemVuRa,
  petitions_patch_tasks_by_ma,
  petitions_post_tasks,
  petitions_post_tasks_by_ma_extensions,
  petitions_post_tasks_by_ma_extensions_by_deNghiID_decision,
  petitions_post_tasks_by_ma_status,
  petitions_quyetDinhLuiHanVao,
  petitions_suaNhiemVuVao,
  petitions_taoNhiemVuVao,
  petitions_vanBanNhiemVuVao,
  petitions_xoaNhiemVuVao,
} from "./schema.gen";

/**
 * Điền mã nhiệm vụ vào khuôn đường dẫn CỦA HỢP ĐỒNG.
 *
 * Nhận khuôn (`/api/v1/tasks/{ma}/status`) chứ không tự ghép chuỗi: khuôn ấy lấy từ kiểu sinh ra,
 * nên đổi đường dẫn ở máy chủ là `tsc` đỏ tại chỗ gọi thay vì 404 lúc chạy.
 *
 * `encodeURIComponent` vì `ma` là **mã sổ của xã** (`NV19`) do người gõ — §7.1 cho phép tự nhập —
 * nên một dấu `/` hay một khoảng trắng lọt vào sẽ đổi hẳn tuyến được gọi.
 */
function duongDanNhiemVu(mau: string, ma: string): string {
  return mau.replace("{ma}", encodeURIComponent(ma));
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ĐỌC
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Bộ lọc của sổ nhiệm vụ — ĐÚNG mười tham số `locNhiemVuTuQuery` đọc, cộng phân trang.
 *
 * KHÔNG CÓ `related` VÀ KHÔNG CÓ `soon`. Cả hai bị máy chủ TỪ CHỐI bằng 400 kèm lý do, nên một ô
 * lọc gửi chúng lên không phải "lọc không ăn" mà là "màn hình hỏng". Xem khối đầu tệp.
 *
 * KHÔNG CÓ `che_do_xem`: Kanban / Danh sách / Sổ theo dõi chọn CÁCH BÀY cùng một tập dòng, nên
 * máy chủ cố ý không đọc nó (`nhiem_vu.go:409`). Gửi lên là dựng một tham số không ai đọc.
 */
export type LocNhiemVu = {
  /**
   * §3 tab Phạm vi. `mine` = "Giao cho tôi", và MÁY CHỦ tự điền mã cán bộ từ PHIÊN —
   * `?scope=mine` **không** mang theo danh tính nào (`nhiem_vu.go:273-288`). Đó là điều kiện để
   * nó không phải một lời client tự khai mình là ai (luật 1, cấm #2).
   */
  phamVi?: "all" | "mine";
  /** Một trong bảy mã trạng thái §6. Sai mã là **400**, không phải bị bỏ qua. */
  trangThai?: string;
  /** Một trong bốn mã nguồn giao §3. Sai mã là **400**. */
  nguonGiao?: string;
  /** Mã danh mục `loai_nhiem_vu` (`code`, không phải `id`). KHÔNG kiểm ở máy chủ: mã lạ ⇒ trang rỗng. */
  loai?: string;
  /** Mã danh mục khối của `identity` (`code`). Không kiểm ở máy chủ. */
  khoi?: string;
  /** Mã danh mục `muc_uu_tien_nhiem_vu` (`code`). Không kiểm ở máy chủ. */
  mucUuTien?: string;
  /** ULID bộ phận thực hiện. */
  boPhanID?: string;
  /**
   * MÃ NGHIỆP VỤ CÁN BỘ (`CB-…`), không phải ULID — cột `nguoi_thuc_hien_ma` giữ mã cán bộ
   * (luật 6, bất biến 8). Một ULID ở đây trả về trang rỗng và không có gì đỏ ở đâu cả.
   */
  nguoiThucHienMa?: string;
  /** Tìm trong mã + tiêu đề. Tối đa 200 ký tự, dài hơn là 400. */
  tim?: string;
  /** Chỉ việc đã quá hạn. Máy chủ chỉ nhận đúng chuỗi `true`. */
  chiTreHan?: boolean;
  limit?: number;
  cursor?: string | null;
};

/**
 * Gom TÊN tham số truy vấn về đúng một chỗ.
 *
 * Mười cái tên dưới đây là mười chuỗi KHÔNG có kiểu nào của hợp đồng canh giúp (xem khối chú
 * thích đầu tệp). Gõ sai một cái thì máy chủ bỏ qua nó và trả về cả quyển sổ, và màn hình trông
 * hoàn toàn bình thường — nên chúng đứng một chỗ và có bài kiểm đọc lại từng tên.
 */
function themLocVaoTruyVan(truyVan: URLSearchParams, loc: LocNhiemVu): void {
  // `scope=all` là mặc định của máy chủ và không cần predicate nào, nên tab "Toàn xã" gửi tham
  // số vắng mặt hẳn — ít một tham số là ít một chỗ có thể gõ sai.
  if (loc.phamVi === "mine") truyVan.set("scope", "mine");

  if (loc.trangThai !== undefined && loc.trangThai !== "") truyVan.set("status", loc.trangThai);
  if (loc.nguonGiao !== undefined && loc.nguonGiao !== "") truyVan.set("source", loc.nguonGiao);
  if (loc.loai !== undefined && loc.loai !== "") truyVan.set("type", loc.loai);
  if (loc.khoi !== undefined && loc.khoi !== "") truyVan.set("bloc", loc.khoi);
  if (loc.mucUuTien !== undefined && loc.mucUuTien !== "") truyVan.set("priority", loc.mucUuTien);
  if (loc.boPhanID !== undefined && loc.boPhanID !== "") truyVan.set("unit", loc.boPhanID);
  if (loc.nguoiThucHienMa !== undefined && loc.nguoiThucHienMa !== "") {
    truyVan.set("assignee", loc.nguoiThucHienMa);
  }
  if (loc.tim !== undefined && loc.tim !== "") truyVan.set("q", loc.tim);

  // `late` CHỈ NHẬN ĐÚNG CHUỖI `true`. Gửi `false` là **400**, không phải "không lọc" — nên ô
  // chưa tích thì tham số vắng mặt hẳn (`errLocTreHanNhiemVuKhongHopLe`).
  if (loc.chiTreHan === true) truyVan.set("late", "true");

  if (loc.limit !== undefined) truyVan.set("limit", String(loc.limit));
  // Con trỏ rỗng nghĩa là trang đầu. Gửi `cursor=` rỗng thì máy chủ trả 400 "con trỏ không hợp
  // lệ" — đúng vào lần mở màn hình đầu tiên.
  if (loc.cursor !== undefined && loc.cursor !== null && loc.cursor !== "") {
    truyVan.set("cursor", loc.cursor);
  }
}

/** Dựng đường dẫn đọc sổ. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`. */
export function duongDanSoNhiemVu(loc: LocNhiemVu = {}): string {
  const duongDan: petitions_get_tasks["duongDan"] = "/api/v1/tasks";
  const truyVan = new URLSearchParams();
  themLocVaoTruyVan(truyVan, loc);
  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/** GET /api/v1/tasks — một trang của sổ nhiệm vụ. */
export function laySoNhiemVu(
  loc: LocNhiemVu = {},
): Promise<KetQua<page_Result_petitions_nhiemVuRa>> {
  return docJSON<page_Result_petitions_nhiemVuRa>(duongDanSoNhiemVu(loc));
}

/**
 * GET /api/v1/tasks/{ma}.
 *
 * MỘT MÃ 404 DUY NHẤT CHO BA TÌNH HUỐNG, và giao diện không được dựng lại sự phân biệt ấy: mã
 * không tồn tại · mã của xã khác · nhiệm vụ đã xoá mềm. Phân biệt được chúng là nói cho người
 * đang thử mã biết những số nào đang tồn tại trong một quyển sổ họ không đọc được.
 */
export function layNhiemVu(ma: string): Promise<KetQua<petitions_nhiemVuRa>> {
  const mau: petitions_get_tasks_by_ma["duongDan"] = "/api/v1/tasks/{ma}";
  return docJSON<petitions_nhiemVuRa>(duongDanNhiemVu(mau, ma));
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * SÁU THAO TÁC GHI
 *
 * 409 LÀ CÂU TRẢ LỜI BÌNH THƯỜNG CỦA NĂM TRONG SÁU TUYẾN, không phải sự cố. Hai lần từ chối nặng
 * nhất mang toàn bộ thông tin TRONG CÂU CHỮ, không trong mã lỗi:
 *
 *   hoàn thành cha còn con   "còn 3 việc con (NV20, NV21, NV22) — hoàn thành hết việc con rồi mới
 *                            hoàn thành việc cha"
 *   xoá cha còn con          "còn 3 việc con chưa xoá — xử lý hoặc xoá các việc con trước"
 *
 * DANH SÁCH MÃ VÀ CON SỐ LÀ TOÀN BỘ PHẦN CÓ ÍCH. Nuốt chúng thành "có lỗi xảy ra" để lại cho cán
 * bộ đúng thông tin bằng không: họ biết mình không được phép, và không biết còn vướng ở đâu. Nên
 * không hàm nào dưới đây viết lại một câu từ chối; `thongBao` của `goi.ts` đã mang nguyên văn
 * `message` máy chủ viết (luật 9, cấm #2).
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * POST /api/v1/tasks — §7 "Giao việc mới". 201, trả về nhiệm vụ vừa tạo.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ. Hợp đồng đòi `Idempotency-Key` bắt buộc. Sinh
 * khoá bên trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới — tức đúng cái khoá
 * chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ đã tới máy chủ và đã cấp một số sổ. Khoá do
 * biểu mẫu giữ, sống bằng đời một lần mở form, và lần bấm lại dùng lại chính nó.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG `...than`. Một phép trải ở đây là đường để một trường lạ — `status`,
 * `original_due_at`, `created_by` — đi lên máy chủ vào ngày ai đó truyền vào một dòng vừa đọc
 * được. Cả ba đều là những thứ hợp đồng CỐ Ý không nhận.
 */
export function taoNhiemVu(
  than: petitions_taoNhiemVuVao,
  khoaChongTrung: string,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const duongDan: petitions_post_tasks["duongDan"] = "/api/v1/tasks";

  const thanGui: petitions_taoNhiemVuVao = {
    auto_code: than.auto_code,
    type: than.type,
    title: than.title,
  };
  // Trường tuỳ chọn chỉ có mặt khi có giá trị: gửi chuỗi rỗng là gửi một lựa chọn, còn vắng mặt
  // là "không khai" — hai điều khác nhau ở một cột nullable.
  if (than.code !== undefined && than.code !== "") thanGui.code = than.code;
  if (than.bloc !== undefined && than.bloc !== "") thanGui.bloc = than.bloc;
  if (than.description !== undefined && than.description !== "") {
    thanGui.description = than.description;
  }
  if (than.priority !== undefined && than.priority !== "") thanGui.priority = than.priority;
  if (than.source !== undefined && than.source !== "") thanGui.source = than.source;
  if (than.source_id !== undefined && than.source_id !== "") thanGui.source_id = than.source_id;
  if (than.unit !== undefined && than.unit !== "") thanGui.unit = than.unit;
  if (than.assignee !== undefined && than.assignee !== "") thanGui.assignee = than.assignee;
  if (than.assigner !== undefined && than.assigner !== "") thanGui.assigner = than.assigner;
  if (than.lead_unit !== undefined && than.lead_unit !== "") thanGui.lead_unit = than.lead_unit;
  if (than.monitor !== undefined && than.monitor !== "") thanGui.monitor = than.monitor;
  // `due_at` LÀ MỘT LẦN DUY NHẤT TRONG ĐỜI NHIỆM VỤ: `han_ban_dau` lấy cùng mốc và trigger
  // `nhiem_vu_bat_bien` từ chối mọi lần ghi sau. Việc tạo mà bỏ trống hạn thì KHÔNG bao giờ đặt
  // được hạn nữa — hệ quả có thật, và nó ra tới màn hình chứ không nằm ở đây.
  if (than.due_at !== undefined && than.due_at !== null && than.due_at !== "") {
    thanGui.due_at = than.due_at;
  }
  if (than.parent !== undefined && than.parent !== "") thanGui.parent = than.parent;
  // BA NHÓM VĂN BẢN §7.2 — cũng dựng từng trường, từng dòng. Chỉ bốn trường hợp đồng nhận cho một
  // dòng MỚI: không `id` (lúc tạo mọi dòng đều mới; một `id` gửi kèm là một dòng máy chủ không
  // tìm thấy), không `position` (số thứ tự do sổ cấp). Mảng rỗng thì VẮNG MẶT: lúc tạo, rỗng và vắng cùng nghĩa, và vắng thì thân
  // của loại `Nhiệm vụ cơ bản` không mang một khoá thuộc §7.2.
  if (than.documents !== undefined && than.documents.length > 0) {
    thanGui.documents = than.documents.map((d) => {
      const dong: petitions_vanBanNhiemVuVao = { group: d.group, summary: d.summary };
      if (d.reference !== undefined && d.reference !== "") dong.reference = d.reference;
      if (d.date !== undefined && d.date !== "") dong.date = d.date;
      return dong;
    });
  }

  return docThanLoiGoi<petitions_nhiemVuRa>(
    goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }),
  );
}

/**
 * PATCH /api/v1/tasks/{ma} — §5.4 nút `✎ Sửa`. 200, trả về nhiệm vụ sau khi sửa.
 *
 * `null` NGHĨA LÀ "KHÔNG ĐỔI", không phải "xoá trắng" — mọi trường là con trỏ ở máy chủ. Chuỗi
 * rỗng thì KHÁC HẲN: nó là "xoá nội dung ô này", một việc hợp lệ với ghi chú và tóm tắt kết quả.
 *
 * KHÔNG CÓ `due_at` VÀ KHÔNG CÓ `assigner`, và cả hai sự vắng mặt là TỪ CHỐI chứ không phải bỏ
 * sót: hạn dịch được đúng một đường — đề nghị lùi hạn có người duyệt (§5.8); còn `assigner`
 * CHÍNH LÀ người duyệt theo ADR 0038, nên một tài khoản `task.update` sửa được cột ấy là một tài
 * khoản tự đặt mình làm người duyệt đề nghị của chính mình.
 */
export function suaNhiemVu(
  ma: string,
  than: petitions_suaNhiemVuVao,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const mau: petitions_patch_tasks_by_ma["duongDan"] = "/api/v1/tasks/{ma}";
  return docThanLoiGoi<petitions_nhiemVuRa>(
    goiGhi(duongDanNhiemVu(mau, ma), "PATCH", than, 200),
  );
}

/**
 * POST /api/v1/tasks/{ma}/status — §6 chuyển trạng thái. 200, trả về nhiệm vụ sau khi chuyển.
 *
 * TRẠNG THÁI ĐÍCH ĐI TRÊN DÂY, khác hẳn tuyến `…/status` của phiếu phản ánh — vì vòng đời §6 RẼ
 * NHÁNH ở mọi bước (`tam-dung` và `chuyen-tiep` rời được cả bốn trạng thái làm việc), nên không
 * có một "bước kế tiếp" nào để máy chủ tự chọn. Cái máy chủ vẫn giữ là TẤM BẢN ĐỒ: một bước sơ
 * đồ không vẽ thì trả 409.
 *
 * ⚠ BƯỚC `hoan-thanh` ĐI QUA HAI PHÉP KIỂM NỮA Ở MÁY CHỦ, và cả hai đều trả câu chữ mang thông
 * tin: `task.approve` (khoá hẹp hơn khoá cổng của tuyến), và **mọi việc con phải xong** — câu từ
 * chối LIỆT KÊ MÃ các việc con còn lại. Màn hình vẽ thẳng câu ấy.
 */
export function doiTrangThaiNhiemVu(
  ma: string,
  trangThai: string,
  ghiChu?: string,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const mau: petitions_post_tasks_by_ma_status["duongDan"] = "/api/v1/tasks/{ma}/status";
  const than: petitions_doiTrangThaiVao =
    ghiChu !== undefined && ghiChu !== ""
      ? { status: trangThai, note: ghiChu }
      : { status: trangThai };
  return docThanLoiGoi<petitions_nhiemVuRa>(
    goiGhi(duongDanNhiemVu(mau, ma), "POST", than, 200),
  );
}

/**
 * DELETE /api/v1/tasks/{ma} — xoá MỀM, kèm lý do bắt buộc. **204 không thân.**
 *
 * TRẢ VỀ `KetQua<void>` CHỨ KHÔNG ĐỌC THÂN: một hàm luôn gọi `.json()` sẽ biến lần xoá thành
 * công thành "không đọc được".
 *
 * ⚠ 409 KHI CÒN VIỆC CON CHƯA XOÁ (ADR 0037 quyết định 3), và câu từ chối MANG SỐ VIỆC CON. Con
 * số ấy là toàn bộ phần có ích: từ chối mà không nói còn vướng gì là bắt cán bộ đi tìm.
 */
export function xoaNhiemVu(ma: string, lyDo: string): Promise<KetQua<void>> {
  const mau: petitions_delete_tasks_by_ma["duongDan"] = "/api/v1/tasks/{ma}";
  const than: petitions_xoaNhiemVuVao = { reason: lyDo };
  return goiGhi(duongDanNhiemVu(mau, ma), "DELETE", than, 204).then((kq) =>
    kq.ok ? ({ ok: true, duLieu: undefined } as KetQua<void>) : kq,
  );
}

/**
 * POST /api/v1/tasks/{ma}/extensions — §5.8 gửi đề nghị lùi hạn. 201, trả về đề nghị.
 *
 * ⚠ TUYẾN NÀY ĐỨNG SAU `task.update`, **KHÔNG** SAU `task.extend` (ADR 0038). Người ĐỀ NGHỊ là
 * người đang làm việc; người DUYỆT là lãnh đạo giao việc. Gộp hai khoá là biến ô đề nghị thành
 * thứ chỉ người duyệt mới dùng được.
 *
 * `new_due_at` PHẢI MUỘN HƠN HẠN ĐANG CÓ — máy chủ kiểm, và một "đề nghị lùi hạn" về phía trước
 * là một lần rút ngắn cam kết qua đường vòng không ai duyệt.
 *
 * Mỗi nhiệm vụ chỉ có MỘT đề nghị đang chờ (`UNIQUE (tenant_id, nhiem_vu_id, moc_cho_duyet)`),
 * nên lần gửi thứ hai là 409 chứ không phải một đề nghị thứ hai lãnh đạo duyệt được hai lần.
 */
export function deNghiLuiHan(
  ma: string,
  hanMoiISO: string,
  lyDo: string,
): Promise<KetQua<petitions_deNghiLuiHanRa>> {
  const mau: petitions_post_tasks_by_ma_extensions["duongDan"] = "/api/v1/tasks/{ma}/extensions";
  const than: petitions_deNghiLuiHanVao = { new_due_at: hanMoiISO, reason: lyDo };
  return docThanLoiGoi<petitions_deNghiLuiHanRa>(
    goiGhi(duongDanNhiemVu(mau, ma), "POST", than, 201),
  );
}

/**
 * POST /api/v1/tasks/{ma}/extensions/{deNghiID}/decision — §5.8, ADR 0038. 200.
 *
 * HAI GIÁ TRỊ VÀ ĐÚNG HAI: `approve` · `reject`. Máy chủ TỪ CHỐI mọi giá trị khác thay vì đọc
 * thành "từ chối" — một lỗi gõ im lặng bác một đề nghị lùi hạn là một cam kết với dân bị giữ
 * nguyên mà không ai biết vì sao.
 *
 * ⚠ HAI LỚP KIỂM Ở MÁY CHỦ, KHÔNG THAY NHAU: `task.extend` ở cổng, rồi
 * `Principal.Ma == lanh_dao_giao_viec_ma` trong giao dịch. Nhiệm vụ CHƯA GHI lãnh đạo giao việc
 * thì **không ai duyệt được** — hỏng theo chiều đóng, và câu từ chối chỉ thẳng thứ phải bổ sung.
 */
export function quyetDinhLuiHan(
  ma: string,
  deNghiID: string,
  duyet: boolean,
  ghiChu?: string,
): Promise<KetQua<petitions_deNghiLuiHanRa>> {
  const mau: petitions_post_tasks_by_ma_extensions_by_deNghiID_decision["duongDan"] =
    "/api/v1/tasks/{ma}/extensions/{deNghiID}/decision";
  const duongDan = duongDanNhiemVu(mau, ma).replace(
    "{deNghiID}",
    encodeURIComponent(deNghiID),
  );
  const than: petitions_quyetDinhLuiHanVao =
    ghiChu !== undefined && ghiChu !== ""
      ? { decision: duyet ? "approve" : "reject", note: ghiChu }
      : { decision: duyet ? "approve" : "reject" };
  return docThanLoiGoi<petitions_deNghiLuiHanRa>(goiGhi(duongDan, "POST", than, 200));
}
