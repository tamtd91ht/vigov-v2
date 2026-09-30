/**
 * Chữ và phép quyết định của màn "Biên bản và kết luận họp" — `docs/ui-ux/04-bien-ban-hop.md`.
 *
 * TÁCH KHỎI COMPONENT ĐỂ KIỂM ĐƯỢC TỪNG PHÉP MỘT. Mọi chuỗi tiếng Việt lấy NGUYÊN VĂN từ đặc tả:
 * một chữ khác đi trên màn của cơ quan nhà nước là một chữ có người phải trả lời.
 */

import type { SuaBienBanVao, TaoBienBanVao, ThongBaoVao } from "@/lib/api/bien-ban";
import type {
  identity_canBoChonNguoiRa,
  petitions_bienBanRa,
  petitions_ketLuanRa,
  petitions_thongBaoKetLuanRa,
} from "@/lib/api/schema.gen";
import { nhanLuaChonCanBo, type DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";

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
/** `domain.LyDoXoaBienBanToiDa` — dùng cho cả gỡ biên bản lẫn gỡ kết luận (cùng `xoaBienBanVao`). */
export const LY_DO_XOA_TOI_DA = 500;
/** Số, ký hiệu Thông báo kết luận — cùng trần với số hiệu biên bản (`ErrSoThongBaoQuaDai`). */
export const SO_THONG_BAO_TOI_DA = 64;

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
 * CON SỐ CHÍNH của thẻ: `{x}/{y} kết luận hoàn thành` (quyết định người dùng 25/09/2026 — đo tiến
 * độ một cuộc họp bằng KẾT LUẬN, không bằng nhiệm vụ; một kết luận "không phát sinh nhiệm vụ" tính
 * là hoàn thành).
 *
 * HAI CON SỐ DO MÁY CHỦ ĐẾM (`conclusion_done_count`, `conclusion_count`). Đếm lại ở đây từ mảng
 * `conclusions` là dựng con số thứ hai của cùng một sự thật — thứ lệch lặng lẽ vào ngày máy chủ đổi
 * cách suy trạng thái, và con số lệch ấy là con số lãnh đạo đọc (luật 9, cấm #2).
 */
export function nhanTienDoBienBan(bb: petitions_bienBanRa): string {
  return `${bb.conclusion_done_count}/${bb.conclusion_count} kết luận hoàn thành`;
}

/**
 * Con số PHỤ của thẻ: `{x}/{y} nhiệm vụ xong` (§2, §7.5) — tổng trên mọi kết luận của biên bản.
 *
 * Cũng do MÁY CHỦ cộng (`task_done_count`, `task_count`), cùng lý do với `nhanTienDoBienBan`.
 */
export function nhanBadge(bb: petitions_bienBanRa): string {
  return `${bb.task_done_count}/${bb.task_count} nhiệm vụ xong`;
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
 * VÒNG ĐỜI: DỰ THẢO → ĐÃ KÝ (migration 0012, quyết định người dùng 25/09/2026)
 *
 * MỌI PHÉP ẨN/TẮT DƯỚI ĐÂY LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP. Máy chủ từ chối thật (409 kèm câu nói
 * cách làm đúng) trên TỪNG lời gọi; các phép này chỉ để cán bộ khỏi bấm vào một thứ chắc chắn bị
 * từ chối. Và chúng FAIL CLOSED theo trạng thái: một mã trạng thái lạ không phải "dự thảo", nên
 * không mở nút sửa/gỡ nào — cũng không phải "đã ký", nên không mở nút ghi Thông báo nào.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const TRANG_THAI_DU_THAO = "du-thao";
export const TRANG_THAI_DA_KY = "da-ky";

export function laDuThao(bb: petitions_bienBanRa): boolean {
  return bb.status === TRANG_THAI_DU_THAO;
}

export function laDaKy(bb: petitions_bienBanRa): boolean {
  return bb.status === TRANG_THAI_DA_KY;
}

/** Chip trạng thái của thẻ. Mã lạ hiện NGUYÊN VĂN — đoán một nhãn là nói sai tình trạng hồ sơ. */
export function nhanTrangThaiBienBan(bb: petitions_bienBanRa): string {
  if (laDuThao(bb)) return "Dự thảo";
  if (laDaKy(bb)) return "Đã ký";
  return bb.status;
}

export function lopChipBienBan(bb: petitions_bienBanRa): string {
  return laDaKy(bb) ? "chip chip-hoat-dong" : "chip chip-ngung";
}

/**
 * Nhãn trạng thái một kết luận — TỪ MÁY CHỦ, không suy ở đây.
 *
 * `status` là trạng thái máy chủ SUY RA từ các nhiệm vụ của kết luận (chưa giao · đang thực hiện ·
 * quá hạn · hoàn thành). Suy lại ở client từ `task_count`/`task_done_count` là dựng bản thứ hai
 * của một phép so với hạn — và "quá hạn" là thứ luật 10 cấm tính ở hai nơi.
 *
 * `no_task` ĐI TRƯỚC: kết luận đánh dấu "không phát sinh nhiệm vụ" được máy chủ tính là hoàn thành,
 * nhưng cán bộ cần đọc ra LÝ DO nó hoàn thành — một chữ "Hoàn thành" không kèm nhiệm vụ nào trông
 * như một con số đếm sai.
 */
export function nhanTrangThaiKetLuan(kl: petitions_ketLuanRa): string {
  if (kl.no_task) return "Không phát sinh nhiệm vụ";
  switch (kl.status) {
    case "chua-giao":
      return "Chưa giao";
    case "dang-thuc-hien":
      return "Đang thực hiện";
    case "qua-han":
      return "Quá hạn";
    case "hoan-thanh":
      return "Hoàn thành";
    default:
      return kl.status;
  }
}

/** Quá hạn là chip ĐỎ — và chữ đã nói rõ, màu không bao giờ là tín hiệu duy nhất (a11y §8). */
export function lopChipKetLuan(kl: petitions_ketLuanRa): string {
  if (kl.no_task || kl.status === "hoan-thanh") return "chip chip-hoat-dong";
  if (kl.status === "qua-han") return "chip chip-cham";
  return "chip chip-ngung";
}

/** Một nút: hiện hay không, và nếu hiện mà tắt thì VÌ SAO — câu ấy nằm ngay dưới nút. */
export type QuyTacNut = {
  readonly hien: boolean;
  /** `null` = bấm được. Chuỗi = tắt, kèm câu giải thích cho cán bộ. */
  readonly viSaoTat: string | null;
};

const AN: QuyTacNut = { hien: false, viSaoTat: null };
const MO: QuyTacNut = { hien: true, viSaoTat: null };

/** Câu giải thích khi một kết luận đã có nhiệm vụ — đọc theo câu `ErrKetLuanDaCoNhiemVu` của máy chủ. */
export const VI_SAO_KET_LUAN_KHOA =
  "Kết luận đã được tách thành nhiệm vụ — nội dung bị khoá, không gỡ được. Hãy xử lý các nhiệm vụ đó trước.";

/** Câu giải thích khi biên bản còn nhiệm vụ trỏ về một kết luận của nó. */
export const VI_SAO_BIEN_BAN_CON_NHIEM_VU =
  "Biên bản còn nhiệm vụ đang trỏ về kết luận — không gỡ được. Hãy xử lý các nhiệm vụ đó trước.";

export type QuyTacKetLuan = {
  readonly sua: QuyTacNut;
  readonly go: QuyTacNut;
  readonly danhDau: QuyTacNut;
  readonly boDau: QuyTacNut;
  readonly tach: QuyTacNut;
};

/**
 * Nút nào hiện trên MỘT dòng kết luận.
 *
 *   Sửa · Gỡ         chỉ biên bản dự thảo; TẮT kèm lý do khi kết luận đã có nhiệm vụ (nhiệm vụ
 *                    đang trích đúng câu ấy — sửa lời là để chúng trỏ vào một câu không ai giao)
 *   Đánh dấu         chỉ dự thảo, chỉ khi CHƯA có nhiệm vụ nào và chưa đánh dấu
 *   Bỏ dấu           chỉ dự thảo, chỉ khi đang đánh dấu
 *   Tách             cả dự thảo LẪN đã ký — ký khoá LỜI biên bản, không khoá việc giao nhiệm vụ từ
 *                    nó; ẨN khi đang đánh dấu "không phát sinh" (máy chủ trả 409 cho lần tách ấy)
 */
export function quyTacKetLuan(bb: petitions_bienBanRa, kl: petitions_ketLuanRa): QuyTacKetLuan {
  const nhap = laDuThao(bb);
  const coNhiemVu = kl.task_count > 0;
  const khoaVi = coNhiemVu ? { hien: true, viSaoTat: VI_SAO_KET_LUAN_KHOA } : MO;

  return {
    sua: nhap ? khoaVi : AN,
    go: nhap ? khoaVi : AN,
    danhDau: nhap && !coNhiemVu && !kl.no_task ? MO : AN,
    boDau: nhap && kl.no_task ? MO : AN,
    tach: kl.no_task ? AN : MO,
  };
}

export type QuyTacBienBan = {
  readonly sua: QuyTacNut;
  readonly xoa: QuyTacNut;
  readonly ky: QuyTacNut;
  readonly ghiThongBao: QuyTacNut;
  readonly boSung: QuyTacNut;
  readonly themKetLuan: QuyTacNut;
};

/**
 * Nút nào hiện trên MỘT thẻ biên bản.
 *
 *   Sửa · Thêm kết luận   chỉ dự thảo — biên bản đã ký không nhận kết luận mới, không đổi một chữ
 *   Gỡ biên bản           chỉ dự thảo; TẮT kèm lý do khi còn nhiệm vụ trỏ về
 *   Ký                    chỉ dự thảo, chỉ khi phiên có `task.approve` (cosmetic — xem `QUYEN_KY_BIEN_BAN`)
 *   Ghi Thông báo         chỉ đã ký VÀ chưa có Thông báo — máy chủ chỉ nhận MỘT lần
 *   Lập bổ sung           chỉ đã ký — bản nháp thì sửa trực tiếp (máy chủ trả 400 cho bổ sung trỏ về nháp)
 */
export function quyTacBienBan(bb: petitions_bienBanRa, coQuyenKy: boolean): QuyTacBienBan {
  const nhap = laDuThao(bb);
  const daKy = laDaKy(bb);
  const chuaCoThongBao = bb.notice === undefined || bb.notice === null;

  return {
    sua: nhap ? MO : AN,
    xoa: nhap ? (bb.task_count > 0 ? { hien: true, viSaoTat: VI_SAO_BIEN_BAN_CON_NHIEM_VU } : MO) : AN,
    ky: nhap && coQuyenKy ? MO : AN,
    ghiThongBao: daKy && chuaCoThongBao ? MO : AN,
    boSung: daKy ? MO : AN,
    themKetLuan: nhap ? MO : AN,
  };
}

/* ── Nhãn hành động ─────────────────────────────────────────────────────────────────────────── */

export const NHAN_NUT_XEM_BIEN_BAN = "Xem biên bản";
export const NHAN_NUT_DONG_BIEN_BAN = "Đóng biên bản";
export const NHAN_NUT_SUA_BIEN_BAN = "Sửa biên bản";
export const NHAN_NUT_XOA_BIEN_BAN = "Gỡ biên bản";
export const NHAN_NUT_KY = "Ký biên bản";
export const NHAN_NUT_XAC_NHAN_KY = "Xác nhận ký";
export const NHAN_NUT_GHI_THONG_BAO = "Ghi số Thông báo kết luận";
export const NHAN_NUT_BO_SUNG = "Lập biên bản bổ sung";
export const NHAN_NUT_SUA_KL = "Sửa";
export const NHAN_NUT_GO_KL = "Gỡ";
export const NHAN_NUT_DANH_DAU = "Đánh dấu “không phát sinh nhiệm vụ”";
export const NHAN_NUT_BO_DAU = "Bỏ dấu “không phát sinh nhiệm vụ”";
export const NHAN_NUT_LUU_SUA = "Lưu thay đổi";

/**
 * Câu xác nhận của hộp Ký. Nói ĐÚNG hệ quả mà máy chủ sẽ áp — câu `ErrBienBanDaKy`: nội dung, kết
 * luận, chủ trì, thư ký bị khoá; sai sót sau đó đi đường biên bản bổ sung.
 */
export const CAU_XAC_NHAN_KY =
  "Sau khi ký, nội dung, kết luận, chủ trì và thư ký của biên bản bị khoá và biên bản không gỡ được. " +
  "Sai sót phát hiện sau khi ký được đính chính bằng một biên bản bổ sung trỏ về biên bản này.";

/** Câu dưới biểu mẫu Thông báo khi biên bản đã ký: máy chủ chỉ nhận MỘT lần. */
export const CAU_THONG_BAO_MOT_LAN =
  "Số và ngày Thông báo kết luận chỉ ghi được một lần sau khi ký — kiểm tra kỹ trước khi lưu.";

/**
 * Dòng cảnh báo trên biểu mẫu nhập/sửa biên bản — ĐÃ QUYẾT (25/09/2026): nội dung mật không bao
 * giờ vào ViGov, nên không có cờ "mật" nào. Dòng này là cách duy nhất màn hình nói điều ấy.
 */
export const CANH_BAO_BI_MAT = "Không nhập nội dung thuộc bí mật nhà nước vào hệ thống.";

/** Lựa chọn rỗng của ô chọn Chủ trì / Thư ký — cả hai trường đều tuỳ chọn (§4). */
export const KHONG_GHI_CAN_BO = "— Không ghi —";

/**
 * Nút mở/đóng danh sách nhiệm vụ đã tách từ một kết luận (§3). Mang con số máy chủ đếm, để cán bộ
 * biết trước mình sắp mở bao nhiêu dòng.
 */
export function nhanNutXemNhiemVu(kl: petitions_ketLuanRa, dangMo: boolean): string {
  return dangMo
    ? `Ẩn ${kl.task_count} nhiệm vụ đã tách`
    : `Xem ${kl.task_count} nhiệm vụ đã tách`;
}

/* ── Đọc ra các trường của biên bản ─────────────────────────────────────────────────────────── */

/**
 * Một mã cán bộ (chủ trì, thư ký, người ký, người tạo) đọc ra thành chữ.
 *
 *   rỗng                         "Không ghi"
 *   danh bạ chưa tải / hỏng      MÃ — thứ duy nhất màn hình biết chắc
 *   có trong danh bạ             `Họ tên · Chức vụ`
 *   không có trong danh bạ       MÃ kèm câu trung tính: danh bạ chỉ gồm tài khoản đang hoạt động,
 *                                và biên bản cũ của người đã nghỉ vẫn phải đọc được
 */
export function nhanCanBo(ma: string | undefined, danhBa: DanhBaTheoMa | null): string {
  if (ma === undefined || ma === "") return "Không ghi";
  if (danhBa === null) return ma;
  const cb = danhBa.get(ma);
  return cb === undefined
    ? `${ma} (không có trong danh bạ cán bộ đang hoạt động)`
    : nhanLuaChonCanBo(cb);
}

/** Một dòng thành phần tham dự: mã có trong danh bạ thì ra họ tên, dòng chữ tự do thì nguyên văn. */
export function nhanThanhPhan(dong: string, danhBa: DanhBaTheoMa | null): string {
  const cb = danhBa?.get(dong);
  return cb === undefined ? dong : nhanLuaChonCanBo(cb);
}

/** `Số 12/TB-UBND, ngày 7/8/2026`, hoặc "Chưa ghi". Ngày là NGÀY LỊCH — đi qua `nhanNgayHop`. */
export function nhanThongBao(tb: petitions_thongBaoKetLuanRa | null | undefined): string {
  if (tb === null || tb === undefined) return "Chưa ghi";
  return `Số ${tb.reference_no}, ngày ${nhanNgayHop(tb.issued_on)}`;
}

/* ── Neo và đường dẫn tới một biên bản ─────────────────────────────────────────────────────────
 *
 * `id` biên bản là ULID mờ — không phải dữ liệu cá nhân, không mang nghĩa — nên nó đi được vào
 * URL. Neo `#bien-ban-{id}` chứ không phải `?id=`: phần sau `#` KHÔNG BAO GIỜ rời trình duyệt, nên
 * không vào log truy cập của proxy nào, và trang không cần đọc tham số truy vấn phía máy chủ.
 */

const TIEN_TO_NEO = "bien-ban-";

/** `id` của phần tử HTML bọc một thẻ biên bản. */
export function neoBienBan(id: string): string {
  return `${TIEN_TO_NEO}${id}`;
}

/** Đường dẫn TỪ MÀN KHÁC (drawer nhiệm vụ) tới đúng biên bản ấy. */
export function duongDanBienBan(id: string): string {
  return `/nhiem-vu/bien-ban#${neoBienBan(id)}`;
}

/**
 * `#bien-ban-01JBB…` → `01JBB…`; mọi thứ khác → `null`. Chỉ nhận chữ và số: neo là thứ ai cũng gõ
 * được vào thanh địa chỉ, và nó đi thẳng vào đường dẫn một lời gọi API.
 */
export function idTuNeo(hash: string): string | null {
  const khop = /^#bien-ban-([0-9A-Za-z]{1,64})$/.exec(hash);
  return khop === null ? null : (khop[1] ?? null);
}

/* ── Biểu mẫu nhập / sửa / bổ sung ──────────────────────────────────────────────────────────── */

/** Mọi ô của biểu mẫu, dạng CHUỖI như ô nhập giữ — chuyển sang thân yêu cầu bằng hai hàm dưới. */
export type GiaTriBieuMau = {
  readonly ten: string;
  /** `2026-08-05` từ `<input type="date">` — NGÀY LỊCH, không bao giờ đi qua `new Date`. */
  readonly ngay: string;
  readonly soHieu: string;
  readonly diaDiem: string;
  /** MÃ cán bộ chủ trì, hoặc rỗng. */
  readonly chuTri: string;
  /** MÃ cán bộ thư ký, hoặc rỗng. */
  readonly thuKy: string;
  /** MÃ cán bộ đã chọn từ danh bạ, theo thứ tự chọn. */
  readonly thanhPhanCanBo: readonly string[];
  /** Ô chữ tự do "mỗi dòng một người" — khách mời, đại diện thôn, người không có tài khoản. */
  readonly thanhPhanKhac: string;
  readonly noiDung: string;
  /** Chỉ có ở biểu mẫu NHẬP MỚI — kết luận của bản nháp sửa từng dòng trên thẻ. */
  readonly ketLuan: string;
};

export const BIEU_MAU_TRONG: GiaTriBieuMau = {
  ten: "",
  ngay: "",
  soHieu: "",
  diaDiem: "",
  chuTri: "",
  thuKy: "",
  thanhPhanCanBo: [],
  thanhPhanKhac: "",
  noiDung: "",
  ketLuan: "",
};

/**
 * Điền biểu mẫu SỬA từ một biên bản đã đọc qua tuyến chi tiết.
 *
 * THÀNH PHẦN TÁCH HAI NGĂN THEO DANH BẠ: dòng nào là mã có trong danh bạ thì vào ngăn "đã chọn",
 * còn lại vào ô chữ tự do NGUYÊN VĂN. Danh bạ chưa tải thì mọi dòng vào ô chữ — vẫn gửi lại đúng
 * chuỗi ấy, nên lần lưu không đổi thành phần của ai.
 */
export function giaTriTuBienBan(
  bb: petitions_bienBanRa,
  danhBa: DanhBaTheoMa | null,
): GiaTriBieuMau {
  const canBo: string[] = [];
  const khac: string[] = [];
  for (const dong of bb.attendees ?? []) {
    if (danhBa?.has(dong) === true) canBo.push(dong);
    else khac.push(dong);
  }
  return {
    ten: bb.title,
    ngay: bb.held_on,
    soHieu: bb.reference_no,
    diaDiem: bb.location,
    chuTri: bb.chaired_by,
    thuKy: bb.minutes_taker ?? "",
    thanhPhanCanBo: canBo,
    thanhPhanKhac: khac.join("\n"),
    noiDung: bb.content ?? "",
    ketLuan: "",
  };
}

/**
 * Danh sách thành phần GỬI ĐI: cán bộ đã chọn trước, rồi các dòng chữ tự do; dòng trùng bị bỏ (một
 * người không dự họp hai lần).
 */
export function thanhPhanTuBieuMau(gt: GiaTriBieuMau): string[] {
  const ra: string[] = [];
  for (const d of [...gt.thanhPhanCanBo, ...tachThanhPhan(gt.thanhPhanKhac)]) {
    if (!ra.includes(d)) ra.push(d);
  }
  return ra;
}

function rongThanhVang(s: string): string | undefined {
  const g = s.trim();
  return g === "" ? undefined : g;
}

/**
 * Thân `POST /api/v1/meetings` từ biểu mẫu. Trường rỗng thì VẮNG (`omitempty` ở máy chủ), trừ
 * `chaired_by` — hợp đồng khai nó là `string` thường, rỗng là trạng thái máy chủ nhận.
 *
 * `boSungChoID` là id biên bản ĐÃ KÝ mà biên bản này bổ sung, hoặc `null`.
 */
export function thanTaoTuBieuMau(gt: GiaTriBieuMau, boSungChoID: string | null): TaoBienBanVao {
  const thanhPhan = thanhPhanTuBieuMau(gt);
  const ketLuan = tachKetLuan(gt.ketLuan);
  return {
    title: gt.ten.trim(),
    held_on: gt.ngay,
    reference_no: rongThanhVang(gt.soHieu),
    location: rongThanhVang(gt.diaDiem),
    chaired_by: gt.chuTri,
    minutes_taker: rongThanhVang(gt.thuKy),
    content: gt.noiDung.trim() === "" ? undefined : gt.noiDung,
    attendees: thanhPhan.length === 0 ? undefined : thanhPhan,
    conclusions: ketLuan.length === 0 ? undefined : ketLuan,
    supplements_id: boSungChoID ?? undefined,
  };
}

function haiDanhSachBang(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && a.every((x, i) => x === b[i]);
}

/**
 * Thân `PATCH /api/v1/meetings/{id}` — CHỈ NHỮNG TRƯỜNG ĐÃ ĐỔI so với bản đã đọc.
 *
 * VÌ SAO KHÔNG GỬI CẢ BIỂU MẪU: PATCH đọc "có mặt" là "đặt thành", và mỗi trường gửi đi là một dòng
 * trước/sau trong vết kiểm toán của một hồ sơ lưu trữ. Gửi lại mười trường không đổi là mười dòng
 * nói rằng ai đó đã sửa chúng.
 *
 * TRƯỜNG BỊ XOÁ TRẮNG THÌ GỬI CHUỖI RỖNG (đặt thành rỗng), không bỏ vắng (không đổi). Ngày họp và
 * tên không bao giờ rỗng — biểu mẫu tắt nút Lưu trước đó.
 *
 * Trả `null` khi KHÔNG CÓ GÌ ĐỔI: một PATCH rỗng không có gì để ghi.
 */
export function thanSuaTuBieuMau(gt: GiaTriBieuMau, ban: petitions_bienBanRa): SuaBienBanVao | null {
  const than: { -readonly [K in keyof SuaBienBanVao]: SuaBienBanVao[K] } = {};
  const ten = gt.ten.trim();
  if (ten !== ban.title) than.title = ten;
  if (gt.ngay !== ban.held_on) than.held_on = gt.ngay;
  if (gt.soHieu.trim() !== ban.reference_no) than.reference_no = gt.soHieu.trim();
  if (gt.diaDiem.trim() !== ban.location) than.location = gt.diaDiem.trim();
  if (gt.chuTri !== ban.chaired_by) than.chaired_by = gt.chuTri;
  if (gt.thuKy !== (ban.minutes_taker ?? "")) than.minutes_taker = gt.thuKy;
  const thanhPhan = thanhPhanTuBieuMau(gt);
  if (!haiDanhSachBang(thanhPhan, ban.attendees ?? [])) than.attendees = thanhPhan;
  const noiDung = gt.noiDung.trim() === "" ? "" : gt.noiDung;
  if (noiDung !== (ban.content ?? "")) than.content = noiDung;
  return Object.keys(than).length === 0 ? null : than;
}

/**
 * Cặp Thông báo kết luận từ hai ô. Cả hai trống → `null` (không ghi). Còn lại gửi CẢ CẶP, kể cả khi
 * một nửa trống: máy chủ trả đúng câu nói nửa nào thiếu, và dựng lại phép kiểm ấy ở đây là bản sao
 * thứ hai của một quy tắc.
 */
export function thanThongBao(so: string, ngay: string): ThongBaoVao | null {
  if (so.trim() === "" && ngay === "") return null;
  return { reference_no: so.trim(), issued_on: ngay };
}

/**
 * Danh sách chọn của ô Chủ trì / Thư ký: cả danh bạ, CỘNG giá trị đang lưu nếu nó không còn trong
 * danh bạ (người đã khoá tài khoản). Thiếu dòng ấy thì `<select>` lặng lẽ hiện "Không ghi", và lần
 * lưu kế tiếp XOÁ chủ trì của một biên bản mà không ai bấm gì.
 */
export function luaChonCanBo(
  danhBa: readonly identity_canBoChonNguoiRa[],
  dangLuu: string,
): { readonly ma: string; readonly nhan: string }[] {
  const ds = danhBa.map((cb) => ({ ma: cb.code, nhan: nhanLuaChonCanBo(cb) }));
  if (dangLuu !== "" && !danhBa.some((cb) => cb.code === dangLuu)) {
    ds.unshift({ ma: dangLuu, nhan: `${dangLuu} (không có trong danh bạ cán bộ đang hoạt động)` });
  }
  return ds;
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
    ten: "Tệp đính kèm — bản scan biên bản (§4)",
    viSao:
      "Kho chưa có nơi lưu tệp và hợp đồng không có tuyến tải lên nào; cột `dinh_kem` giữ nguyên " +
      "mặc định `'[]'` (migration 0007). Vẽ một vùng kéo thả không có chỗ lưu là mời cán bộ thả " +
      "một bản scan vào rồi mất.",
  },
  {
    ten: "Cổng quyền `task.read` / `task.create` ở phía giao diện — chỉ nút Ký có cổng",
    viSao:
      "Nút `Ký biên bản` ẩn với phiên thiếu `task.approve` (`QUYEN_KY_BIEN_BAN`) — tiện dụng, " +
      "không phải biện pháp. Mọi nút khác (nhập, sửa, gỡ, đánh dấu, tách) KHÔNG có cổng theo " +
      "`task.create` ở client, đúng khuôn màn Văn bản: dịch vụ `petitions` kiểm quyền trên TỪNG " +
      "lời gọi, và tài khoản thiếu khoá nhận nguyên câu 403 của máy chủ ra màn hình. Ẩn một nút " +
      "chưa bao giờ là biện pháp (luật 5, cấm #1); thiếu nó ở đây chỉ tốn một lần bấm.",
  },
  {
    ten: "Lớp CSS riêng cho thẻ biên bản và ô tròn số thứ tự (§2)",
    viSao:
      "`globals.css` chưa có lớp nào cho danh sách thẻ, cho ô tròn xanh nhạt của số thứ tự, hay " +
      "cho một lớp phủ modal, và lượt này không được thêm CSS. Màn dùng lại các lớp sẵn có " +
      "(`khoi-chi-tiet`, `chip`), còn các biểu mẫu — Nhập/Sửa biên bản §4, Giao việc §3, hộp Ký, " +
      "Thông báo kết luận, Gỡ — và khung Xem biên bản đều dựng NỐI TIẾP trong trang thay vì làm lớp " +
      "phủ như chữ “modal” của đặc tả. " +
      "Mượn một lớp của thứ khác cho đúng hình hôm nay sẽ lệch hẳn vào ngày lớp ấy đổi vì cái nó " +
      "thật sự phục vụ; tên lớp cần thêm đã báo về.",
  },
];
