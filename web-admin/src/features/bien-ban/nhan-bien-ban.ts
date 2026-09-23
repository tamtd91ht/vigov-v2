/**
 * Chữ và phép quyết định của màn "Biên bản và kết luận họp" — `docs/ui-ux/04-bien-ban-hop.md`.
 *
 * TÁCH KHỎI COMPONENT ĐỂ KIỂM ĐƯỢC TỪNG PHÉP MỘT. Mọi chuỗi tiếng Việt lấy NGUYÊN VĂN từ đặc tả:
 * một chữ khác đi trên màn của cơ quan nhà nước là một chữ có người phải trả lời.
 */

import type { petitions_bienBanRa, petitions_ketLuanRa } from "@/lib/api/schema.gen";

/* ── Trần độ dài, đúng bằng trần máy chủ ────────────────────────────────────────────────────
 *
 * Chép từ `service-petitions/internal/domain/bien_ban_hop_ghi.go:34-67` và CHỈ để ô nhập dừng
 * lại đúng chỗ máy chủ sẽ dừng — không phải để thay phép kiểm ấy. Máy chủ vẫn là nơi từ chối
 * thật; `maxLength` ở ô nhập chỉ để cán bộ thấy giới hạn thay vì thấy một câu 400 sau khi đã gõ
 * xong cả trang.
 */
export const TEN_CUOC_HOP_TOI_DA = 300;
export const SO_HIEU_TOI_DA = 64;
export const DIA_DIEM_TOI_DA = 300;
export const NOI_DUNG_BIEN_BAN_TOI_DA = 50000;
export const NOI_DUNG_KET_LUAN_TOI_DA = 5000;
/** Bao nhiêu kết luận MỘT LẦN NHẬP mang được. Không phải trần của cả biên bản — thêm tiếp bằng
 *  hàng `+ Thêm kết luận` ở cuối thẻ, và tuyến ấy không bị đếm vào đây. */
export const KET_LUAN_MOI_LAN_TOI_DA = 50;
export const THANH_PHAN_MOT_DONG_TOI_DA = 200;
export const THANH_PHAN_TOI_DA = 200;

/* ── Chữ trên màn ──────────────────────────────────────────────────────────────────────────── */

export const TIEU_DE_MAN = "Biên bản và kết luận họp";

/** §1 — nguyên văn câu mô tả trong giao diện. */
export const MO_TA_MAN =
  "Nhập một biên bản, tách thành nhiều nhiệm vụ. Mỗi nhiệm vụ giữ liên kết ngược về kết luận gốc " +
  "để truy vết được về sau.";

export const NHAN_NUT_NHAP_BIEN_BAN = "+ Nhập biên bản";
export const NHAN_NUT_THEM_KET_LUAN = "+ Thêm kết luận";
/** §2 — nguyên văn placeholder của hàng thêm kết luận. */
export const PLACEHOLDER_KET_LUAN = "Nhập một kết luận của cuộc họp…";

export const NHAN_NUT_HUY = "Huỷ";
export const NHAN_NUT_LUU = "Lưu biên bản";

/**
 * Sổ rỗng. §6 của phụ lục KHÔNG có dòng nào cho danh sách biên bản — bảng ấy chỉ liệt kê Kanban,
 * Sổ tay, Thông báo, đợt thu chi, nhật ký và "Chưa tách thành nhiệm vụ nào". Câu này vì thế là
 * câu viết mới, cùng giọng với các câu đã có, và nó được nói ra ở đây thay vì để người sau tưởng
 * mình đọc lại một câu của đặc tả.
 */
export const SO_RONG = "Chưa có biên bản họp nào.";

export const DANG_TAI_SO = "Đang tải danh sách biên bản…";

/** §6 — nguyên văn, và nó KHÁC "0/3 nhiệm vụ đã hoàn thành". Xem `nhanTienDoKetLuan`. */
export const CHUA_TACH_NHIEM_VU = "Chưa tách thành nhiệm vụ nào";

/**
 * Nút `✂ Tách thành nhiệm vụ` (§2, §3) — nguyên văn nhãn đặc tả vẽ.
 *
 * TỪ 24/09/2026 ĐÂY LÀ MỘT NÚT THẬT. Trước đó chỗ này là một dòng chữ `CHO_NUT_TACH` nói rằng màn
 * chưa dựng, vì biểu mẫu "Giao việc mới" của `02-nhiem-vu.md` §7 chưa có; nay nó có và được dùng
 * lại nguyên bản. Phần §3 CÒN THIẾU là điền sẵn — xem `PHAN_CHUA_DUNG`.
 */
export const NHAN_NUT_TACH = "✂ Tách thành nhiệm vụ";

/**
 * Tên đọc được của nút Tách, mang SỐ THỨ TỰ CỦA KẾT LUẬN.
 *
 * KHÔNG PHẢI MỘT CHI TIẾT TRỢ NĂNG. Một thẻ biên bản có nhiều dòng kết luận, và không có con số
 * này thì ba nút liền nhau mang cùng một tên — trình đọc màn hình đọc "Tách thành nhiệm vụ" ba
 * lần, và người dùng nó không có cách nào biết mình đang bấm vào kết luận nào.
 *
 * Nó còn là chỗ DUY NHẤT con số ấy ra tới trang: `{stt}` thật đi trên đường dẫn của lời gọi, mà
 * một lời gọi thì không kiểm được bằng `renderToStaticMarkup`. Hai chỗ cùng đọc `ketLuan.ordinal`,
 * nên vẽ sai số ở đây là dấu hiệu gửi sai số ở kia.
 *
 * MỞ ĐẦU BẰNG ĐÚNG CHỮ HIỆN TRÊN NÚT, không phải một câu viết lại: tên đọc được của một nút phải
 * CHỨA nhãn nhìn thấy được, nếu không thì người điều khiển bằng giọng nói đọc đúng chữ trên màn mà
 * không bấm được nút ấy (WCAG 2.5.3). Chỉ dấu kéo `✂` là trang trí nên không vào tên.
 */
export function nhanNutTach(kl: petitions_ketLuanRa): string {
  return `Tách thành nhiệm vụ — kết luận số ${soThuTuKetLuan(kl)}`;
}

/**
 * Dòng "Nguồn giao" KHOÁ của §3.
 *
 * §3 vẽ nó là một ô khoá, không sửa. Ở đây nó là một CÂU chứ không phải một ô bị vô hiệu hoá, vì
 * không có gì để chọn: hợp đồng không có trường `source`/`source_id` trên tuyến này, máy chủ tự
 * điền cặp ấy từ kết luận nêu trong đường dẫn. Một ô `select` bị `disabled` sẽ nói rằng có một giá
 * trị được gửi đi — không có.
 */
export const NGUON_GIAO_KHOA = "Nguồn giao: Từ kết luận họp — khoá, không sửa được";

/** Câu báo đã tách xong. Mang SỐ SỔ máy chủ vừa cấp — thứ cán bộ không thể biết trước. */
export function cauDaTach(maNhiemVu: string): string {
  return `Đã tách thành nhiệm vụ ${maNhiemVu}.`;
}

/* ── Phép định dạng ────────────────────────────────────────────────────────────────────────── */

/** Chuỗi hiện thay cho một giá trị chưa tính được. Cùng dấu gạch mọi màn khác đang dùng. */
export const DAU_GACH = "—";

/**
 * `2026-08-05` → `5/8/2026` (§2 vẽ ngày họp dạng `d/M/yyyy`).
 *
 * ⚠ KHÔNG ĐI QUA `new Date`. `held_on` là một NGÀY LỊCH, không phải một mốc thời gian: đưa nó qua
 * `new Date("2026-08-05")` là ép về nửa đêm UTC, và một trình duyệt ở múi giờ âm sẽ vẽ ngày 4/8
 * lên tấm thẻ mà toàn bộ nội dung là cái ngày ấy. Cắt chuỗi thì không có múi giờ nào can thiệp
 * được. Máy chủ cũng định dạng đúng một chỗ, bằng đúng một hàm (`ngayHopRa`) — vì cùng lý do.
 *
 * Chuỗi không đúng khuôn thì trả dấu gạch: bịa ra một ngày từ một chuỗi không đọc được là vẽ lên
 * thẻ một ngày họp không có thật.
 */
export function nhanNgayHop(hopNgay: string): string {
  const khop = /^(\d{4})-(\d{2})-(\d{2})$/.exec(hopNgay);
  if (khop === null) return DAU_GACH;

  const nam = khop[1] ?? "";
  const thang = Number(khop[2]);
  const ngay = Number(khop[3]);
  if (thang < 1 || thang > 12 || ngay < 1 || ngay > 31) return DAU_GACH;

  return `${ngay}/${thang}/${nam}`;
}

/**
 * Dòng meta của thẻ: `{ngày họp} · {số hiệu} · {địa điểm}` — §2: *"phần nào thiếu thì bỏ, phân
 * tách bằng ` · `"*.
 *
 * SỐ HIỆU KHÔNG PHẢI MÃ ĐỊNH DANH, và nó nằm giữa dòng meta chính vì thế: nó gõ tay, tuỳ chọn,
 * và KHÔNG duy nhất — `31/BB-UBND` của năm nay và của năm sau là hai biên bản khác nhau mang
 * cùng một chuỗi. Biên bản họp không có mã nghiệp vụ nào cả (migration 0007), nên không chỗ nào
 * trên màn được vẽ nó như `PA-2026-0021` của quyển sổ phản ánh.
 */
export function dongMeta(bb: petitions_bienBanRa): string {
  const phan = [nhanNgayHop(bb.held_on), bb.reference_no, bb.location].filter(
    (s) => s !== "" && s !== DAU_GACH,
  );
  return phan.length === 0 ? DAU_GACH : phan.join(" · ");
}

/**
 * Badge góc phải của thẻ: `{n} kết luận · {x}/{y} nhiệm vụ xong` (§2, §7.5).
 *
 * `x/y` LÀ TỔNG TRÊN MỌI KẾT LUẬN CỦA BIÊN BẢN, và hai con số ấy do MÁY CHỦ cộng
 * (`bienBanRaNgoai` gọi `domain.TienDoNhiemVu`). Cộng lại ở đây từ mảng `conclusions` sẽ cho
 * đúng số trong hầu hết trường hợp và cho một con số thứ hai của cùng một sự thật — thứ sẽ lệch
 * vào ngày máy chủ đổi cách đếm, lệch lặng lẽ, và con số lệch ấy là con số lãnh đạo đọc (luật 9).
 */
export function nhanBadge(bb: petitions_bienBanRa): string {
  return `${bb.conclusions.length} kết luận · ${bb.task_done_count}/${bb.task_count} nhiệm vụ xong`;
}

/**
 * Dòng phụ của một kết luận: `{x}/{y} nhiệm vụ đã hoàn thành`, hoặc `Chưa tách thành nhiệm vụ nào`.
 *
 * HAI CÂU, KHÔNG MỘT. `0/0` và "chưa tách" là hai phát biểu khác nhau: cái sau nói rằng chưa ai
 * biến kết luận này thành việc của ai cả — đúng thứ §1 nói màn hình này sinh ra để làm. Máy chủ
 * cố ý không gửi một cờ boolean cho nó: client đã có cả hai con số (luật 9, cấm #2).
 */
export function nhanTienDoKetLuan(kl: petitions_ketLuanRa): string {
  if (kl.task_count === 0) return CHUA_TACH_NHIEM_VU;
  return `${kl.task_done_count}/${kl.task_count} nhiệm vụ đã hoàn thành`;
}

/**
 * Số trong ô tròn của một dòng kết luận.
 *
 * ⚠ LẤY `ordinal` CỦA MÁY CHỦ, TUYỆT ĐỐI KHÔNG LẤY VỊ TRÍ TRONG MẢNG. §7.2 nối tiếp số ĐÃ CẤP:
 * gỡ kết luận ② thì kết luận kế tiếp mang số ④, và khoảng trống ấy là ĐÚNG — số đã cấp thì không
 * cấp lại, kể cả sau xoá mềm (luật 7, bất biến 3). Đánh lại số theo vị trí sẽ làm mọi nhiệm vụ đã
 * tách từ "kết luận ③" trỏ về một con số khác với con số đang hiện trên màn, trong khi biên bản
 * giấy đã in và đã ký thì mang con số cũ.
 *
 * Hàm này tồn tại để chỗ ấy có MỘT tên gọi và MỘT bài kiểm, thay vì một `{i + 1}` nằm lẫn trong
 * JSX — nơi không ai đọc lại.
 *
 * TỪ 24/09/2026 CON SỐ ẤY CÒN ĐI MỘT ĐƯỜNG THỨ HAI: nó là `{stt}` trên đường dẫn của tuyến Tách
 * (`POST …/conclusions/{stt}/task`). Vẽ sai thì cán bộ thấy sai; GỬI sai thì không ai thấy gì cả —
 * lời gọi trả 201 và một nhiệm vụ gắn vào kết luận khác, giao cho người khác, trong một quyển sổ
 * không xoá được. Vì thế `tachKetLuanThanhNhiemVu` nhận cả DÒNG kết luận chứ không nhận một số.
 */
export function soThuTuKetLuan(kl: petitions_ketLuanRa): number {
  return kl.ordinal;
}

/**
 * Tách ô "Thành phần tham dự" thành danh sách: mỗi dòng một người.
 *
 * Dòng trắng bị bỏ — gõ Enter hai lần là một thói quen gõ văn bản, không phải một người dự.
 */
export function tachThanhPhan(chu: string): string[] {
  return chu
    .split("\n")
    .map((d) => d.trim())
    .filter((d) => d !== "");
}

/**
 * Tách ô "Các kết luận" của biểu mẫu nhập: mỗi dòng một kết luận, đúng thứ tự người gõ — đó là
 * thứ tự trở thành ① ② ③ (§4, và `taoBienBanVao.Conclusions` ở máy chủ).
 *
 * Cùng phép tách với `tachThanhPhan` nhưng là HAI HÀM, vì hai ô ấy ràng buộc khác nhau và sẽ rẽ
 * khỏi nhau ngay khi một trong hai có ô nhập riêng cho từng dòng như §4 mô tả.
 */
export function tachKetLuan(chu: string): string[] {
  return chu
    .split("\n")
    .map((d) => d.trim())
    .filter((d) => d !== "");
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA ĐẶC TẢ **KHÔNG DỰNG ĐƯỢC**, VÀ CHÚNG PHẢI RA TỚI MÀN HÌNH
 *
 * Không giấu trong chú thích, không vẽ một nút chắc chắn hỏng. Cùng khuôn `PHAN_CHUA_DUNG` của
 * màn Thu - Chi ngân sách và màn Phản ánh.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  {
    ten: "HẠN GỢI Ý của biểu mẫu Tách (§3) — nút và điền sẵn thì đã dựng",
    viSao:
      "Nút `✂ Tách thành nhiệm vụ` đã có, mở đúng biểu mẫu “Giao việc mới” của `02-nhiem-vu.md` " +
      "§7 (dùng lại nguyên `FormGiaoViec`, không bản thứ hai), và ô “Nội dung nhiệm vụ” NAY ĐÃ " +
      "ĐIỀN SẴN nội dung kết luận. Thứ còn thiếu là HẠN GỢI Ý: §3 muốn đọc ngày từ chính câu kết " +
      "luận (`báo cáo trước ngày 20/8` → 20/8). Đây KHÔNG phải một việc chưa tới lượt mà là một " +
      "câu chờ khách chốt — máy chủ CỐ Ý không suy ngày, và " +
      "`service-petitions/internal/http/bien_ban_hop_ghi.go:130-135` ghi số đo của bản cài đặt " +
      "anh em: đúng ba trên bốn lần, lần thứ tư để lại một nhiệm vụ SAI HẠN trong một quyển sổ " +
      "không xoá được, chỉ thu hồi được. Dựng bộ đoán ngày ở client là dựng lại đúng thứ máy chủ " +
      "vừa từ chối, chỉ khác là ở chỗ không ai kiểm được. Hai ô “Nguồn giao” và `nguon_id` của " +
      "bảng §3 thì không thiếu gì cả: hợp đồng không có trường nào cho chúng, máy chủ suy cặp ấy " +
      "từ kết luận nêu trong đường dẫn.",
  },
  {
    ten: "Ô chọn `Chủ trì` (§4, combobox cán bộ)",
    viSao:
      "Trường `chaired_by` nhận MÃ NGHIỆP VỤ của cán bộ (`CB-2026-7K3M9Q`), và tuyến duy nhất tra " +
      "được danh sách ấy — `GET /api/v1/staff` — đòi `admin.user`, một khoá quản trị hệ thống " +
      "không liên quan tới việc gõ biên bản. Một ô text để cán bộ tự gõ mã là một ô sẽ được điền " +
      "bằng họ tên, và giá trị ấy vào thẳng cột mã trong sổ lưu trữ. Biên bản vì thế lưu với " +
      "`chaired_by` rỗng — máy chủ chấp nhận trạng thái ấy.",
  },
  {
    ten: "Tệp đính kèm — bản scan biên bản (§4)",
    viSao:
      "Kho chưa có nơi lưu tệp và hợp đồng không có tuyến tải lên nào; cột `dinh_kem` giữ nguyên " +
      "mặc định `'[]'` (migration 0007). Vẽ một vùng kéo thả không có chỗ lưu là mời cán bộ thả " +
      "một bản scan vào rồi mất.",
  },
  {
    ten: "Nội dung biên bản, thành phần tham dự KHI ĐỌC LẠI (§4)",
    viSao:
      "Hai trường ấy GỬI LÊN được nhưng KHÔNG ĐỌC VỀ được: `GET /api/v1/meetings` cố ý không trả " +
      "`content`, `attendees` và `dinh_kem` (§2 không vẽ chúng, và toàn văn mọi cuộc họp trên một " +
      "trang sẽ lấn hết phần còn lại), còn tuyến chi tiết một biên bản thì chưa có. Hệ quả phải " +
      "nói thẳng với cán bộ: gõ xong biên bản rồi thì màn này KHÔNG mở lại toàn văn ấy được.",
  },
  {
    ten: "Sửa, xoá biên bản và sửa kết luận (§6 của đặc tả)",
    viSao:
      "Hợp đồng không có `PATCH`/`DELETE` nào cho biên bản hay kết luận. Đó không phải thiếu sót " +
      "của lượt này: một kết luận đã có nhiệm vụ trỏ vào thì có được sửa lời hay không là câu hỏi " +
      "migration 0007 cố ý để ngỏ (§7.1 đòi “không xoá cứng, cảnh báo và giữ liên kết”).",
  },
  {
    ten: "Danh sách nhiệm vụ đã tách ra từ một kết luận (§3)",
    viSao:
      "§3 nói dòng kết luận “có thể mở rộng để xem danh sách nhiệm vụ đã sinh ra”. Hợp đồng chỉ " +
      "trả hai con số đếm, không trả danh sách; tuyến `GET /api/ket-luan/:id/nhiem-vu` mà §6 phác " +
      "ra chưa tồn tại. Hai con số thì hiện đúng hai con số.",
  },
  {
    ten: "Cổng quyền `task.read` / `task.create` ở phía giao diện",
    viSao:
      "`src/lib/quyen.ts` chưa có hằng cho hai khoá ấy và lượt này không được sửa tệp đó (đang có " +
      "agent khác chạy song song), còn gõ thẳng chuỗi `\"task.create\"` vào màn là dựng bản sao " +
      "thứ hai của một khoá phân quyền. Vì thế màn này KHÔNG có cổng ở client — đúng khuôn màn " +
      "Văn bản đang dùng cho `document.read`: dịch vụ `petitions` kiểm quyền trên TỪNG lời gọi, " +
      "và tài khoản thiếu khoá nhận nguyên câu 403 của máy chủ ra màn hình. Ẩn một nút chưa bao " +
      "giờ là biện pháp (luật 5, cấm #1); thiếu nó ở đây chỉ tốn một lần bấm.",
  },
  {
    ten: "Thứ tự thẻ theo NGÀY HỌP (§2: “mới nhất ở trên”)",
    viSao:
      "Máy chủ phân trang theo `tao_luc` (thời điểm NHẬP), không theo `ngay_hop`, và cố ý: cột " +
      "`ngay_hop` là DATE, không có chỉ mục, và con trỏ keyset trên một cột DATE chưa kiểm được " +
      "trong môi trường này (`store/bien_ban_hop.go:52-67`). Hệ quả nhìn thấy được: một biên bản " +
      "họp tháng 7 nhập muộn sẽ nằm TRÊN biên bản tháng 8 nhập sớm.",
  },
  {
    ten: "Lớp CSS riêng cho thẻ biên bản và ô tròn số thứ tự (§2)",
    viSao:
      "`globals.css` chưa có lớp nào cho danh sách thẻ, cho ô tròn xanh nhạt của số thứ tự, hay " +
      "cho một lớp phủ modal, và lượt này không được thêm CSS. Màn dùng lại các lớp sẵn có " +
      "(`khoi-chi-tiet`, `chip`), còn hai biểu mẫu — Nhập biên bản §4 và Giao việc §3 — dựng NỐI " +
      "TIẾP trong trang thay vì làm lớp phủ như hai chữ “modal” của đặc tả. " +
      "Mượn một lớp của thứ khác cho đúng hình hôm nay sẽ lệch hẳn vào ngày lớp ấy đổi vì cái nó " +
      "thật sự phục vụ; tên lớp cần thêm đã báo về.",
  },
];
