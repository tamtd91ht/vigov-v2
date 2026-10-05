/**
 * Câu chữ, vòng đời và các phép QUYẾT ĐỊNH của màn "Quản lý nhiệm vụ"
 * (`docs/ui-ux/02-nhiem-vu.md`). Hàm thuần: không gọi mạng, không dựng DOM, không đọc đồng hồ —
 * thời điểm "bây giờ" luôn là một tham số, để ca "trễ 87 ngày" kiểm được mà không cần giả lập
 * đồng hồ hệ thống.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO TỆP NÀY TÁCH KHỎI `lib/api/nhiem-vu.ts` VÀ KHỎI MÀN HÌNH
 *
 * Tệp này được viết lúc `src/lib/api/schema.gen.ts` CHƯA có các kiểu Nhiệm vụ, nên nó chỉ nhận
 * giá trị vô hướng (chuỗi, số, `Date`). Các kiểu ấy NAY ĐÃ CÓ trong tệp sinh (`petitions_nhiemVuRa`,
 * `petitions_nhiemVuVanBanRa`, `page_Result_petitions_nhiemVuRa`…) và `lib/api/nhiem-vu.ts` dùng
 * thẳng chúng. Tệp này vẫn tách riêng vì một lý do khác: câu chữ và phép quyết định thuần kiểm
 * được mà không dựng DOM.
 *
 * Chỗ nào ở đây cần hình dạng một bản ghi thì `import type` từ tệp sinh — KHÔNG gõ tay lại. Gõ
 * tay là dựng đúng cái hỏng mà `scripts/gen-api-types.mjs` được viết ra để chặn: nhiều bản chép
 * của một hình dạng, trôi dần, và bản sai là bản chạy thật (luật 9, cấm #2; bất biến 6 của agent
 * này).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import {
  danhBaTheoMa,
  nhanThoiDiem,
  staffNameWithCode,
  type DanhBaTheoMa,
} from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import type { ChieuSapXepNhiemVu, CotSapXepNhiemVu } from "@/lib/api/nhiem-vu";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_TAO_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
  TASK_ASSIGN_PERMISSION,
  coQuyen,
} from "@/lib/quyen";
import type {
  identity_caLamViecRa,
  identity_canBoChonNguoiRa,
  identity_danhBaChonNguoiRa,
  petitions_mucUuTienRa,
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_deNghiChoDuyetRa,
  petitions_nhatKyNhiemVuRa,
  petitions_nhiemVuRa,
  petitions_nhiemVuVanBanRa,
  petitions_suaNhiemVuVao,
  petitions_taoNhiemVuVao,
  petitions_taskStatusCountOut,
  petitions_vanBanNhiemVuVao,
} from "@/lib/api/schema.gen";

/** Ô rỗng. Chưa có gì để tính thì **dấu gạch**, không bao giờ `0` và không bao giờ `0%`. */
export const O_TRONG = "—";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BẢY TRẠNG THÁI — §6
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Bảy mã trạng thái của §6. **DANH SÁCH ĐÓNG** — câu hỏi mở #21 đã chốt ngày 22/09/2026
 * (ADR 0035 §C): xã đổi được NHÃN và THỨ TỰ, không đổi được DANH SÁCH MÃ.
 *
 * Chính vì danh sách đóng mà gõ thẳng bảy mã ấy vào mã nguồn trình duyệt là hợp lệ. Với một danh
 * sách xã sửa được thì nó sẽ là một giá trị riêng của xã bị nung vào bundle (luật 1, bất biến 10)
 * — điều tệp này không được phép làm.
 *
 * Cùng bảy chuỗi mà lược đồ cưỡng chế bằng `CHECK`
 * (`service-petitions/migrations/0006_nhiem_vu.sql:341`) và miền nghiệp vụ khai bằng hằng
 * (`service-petitions/internal/domain/nhiem_vu.go:40-47`).
 */
export type TrangThaiNhiemVu =
  | "moi-giao"
  | "da-tiep-nhan"
  | "dang-thuc-hien"
  | "cho-duyet"
  | "hoan-thanh"
  | "tam-dung"
  | "chuyen-tiep";

/**
 * Năm trạng thái CHÍNH — đúng năm cột Kanban của §4.1, đúng thứ tự dải bước của §5.2.
 *
 * Hai trạng thái rẽ nhánh KHÔNG có cột riêng (§4.1), nên chúng không nằm trong mảng này.
 */
export const TRANG_THAI_CHINH: readonly TrangThaiNhiemVu[] = [
  "moi-giao",
  "da-tiep-nhan",
  "dang-thuc-hien",
  "cho-duyet",
  "hoan-thanh",
];

/** Hai trạng thái rẽ nhánh — hàng `Rẽ nhánh:` dưới dải bước (§5.2, phụ lục 15 §5.2). */
export const TRANG_THAI_RE_NHANH: readonly TrangThaiNhiemVu[] = ["tam-dung", "chuyen-tiep"];

/** Bảy mã, dùng cho ô chọn và cho phép kiểm `laTrangThaiNhiemVu`. */
export const MOI_TRANG_THAI: readonly TrangThaiNhiemVu[] = [
  ...TRANG_THAI_CHINH,
  ...TRANG_THAI_RE_NHANH,
];

/** Mã có phải một trong bảy hay không. Dùng để đọc `status` — hợp đồng khai nó là `string` trơn. */
export function laTrangThaiNhiemVu(ma: string): ma is TrangThaiNhiemVu {
  return (MOI_TRANG_THAI as readonly string[]).includes(ma);
}

/* ── NHÃN VÀ THỨ TỰ CỦA XÃ — `GET /api/v1/task-statuses` ─────────────────────────────────────
 *
 * MỘT NGUỒN, KHÔNG HAI (quyết định #21, 24/09/2026). Nhãn và thứ tự bảy trạng thái là của TỪNG XÃ
 * và máy chủ trả về ĐỦ BẢY, đã gộp chữ riêng của xã với chữ mặc định của phần mềm
 * (`service-petitions/internal/domain/nhan_trang_thai_nhiem_vu.go:50-72`). Màn hình vì vậy không
 * giữ bản nhãn nào của riêng nó — trừ ĐƯỜNG LUI dưới đây, dùng khi tuyến đọc hỏng.
 *
 * VÌ SAO KHÔNG CÒN NHÃN KANBAN RIÊNG ("Chưa thực hiện"): máy chủ cố ý giao `moi-giao` = "Mới giao"
 * làm mặc định, và coi chữ "Chưa thực hiện" của bảng Kanban là đúng loại chữ riêng mà một xã tự
 * đặt qua tab Danh mục (chú thích ở `nhan_trang_thai_nhiem_vu.go:57-59`). Giữ một bản nhãn Kanban
 * thứ hai ở đây là đặt lại đúng bản sao mà quyết định #21 bỏ đi: xã đổi nhãn `moi-giao` xong, cột
 * Kanban vẫn hiện chữ cũ.
 */

/** Nhãn và thứ tự bảy trạng thái mà màn hình đang dùng. */
export type BangNhanTrangThai = {
  readonly nhan: Readonly<Record<TrangThaiNhiemVu, string>>;
  /** Đủ bảy mã, theo thứ tự HIỆU LỰC của xã. */
  readonly thuTu: readonly TrangThaiNhiemVu[];
};

/**
 * ĐƯỜNG LUI — chỉ dùng khi chưa đọc xong hoặc đọc hỏng `GET /api/v1/task-statuses`.
 *
 * Chép đúng bảng mặc định của máy chủ (`nhan_trang_thai_nhiem_vu.go:64-72`), cả chữ lẫn thứ tự, để
 * một xã CHƯA đổi gì thấy cùng một màn hình dù tuyến đọc có trả lời hay không. Đây là bản sao duy
 * nhất còn lại, và nó chỉ lên màn KÈM câu `CANH_BAO_NHAN_MAC_DINH` — không bao giờ lặng lẽ.
 */
export const BANG_NHAN_MAC_DINH: BangNhanTrangThai = {
  nhan: {
    "moi-giao": "Mới giao",
    "da-tiep-nhan": "Đã tiếp nhận",
    "dang-thuc-hien": "Đang thực hiện",
    "cho-duyet": "Chờ duyệt",
    "hoan-thanh": "Hoàn thành",
    "tam-dung": "Tạm dừng",
    "chuyen-tiep": "Chuyển tiếp",
  },
  thuTu: MOI_TRANG_THAI,
};

/**
 * Câu hiện khi màn hình đang chạy bằng đường lui. Câu máy chủ đi kèm phía sau, nguyên văn.
 *
 * NÓI RA, KHÔNG GIẢ VỜ: một xã đã đổi "Mới giao" thành "Chưa thực hiện" mà màn hình lặng lẽ hiện
 * "Mới giao" thì cán bộ đọc hai chữ khác nhau cho cùng một việc ở hai màn, và không ai biết vì sao.
 */
export const CANH_BAO_NHAN_MAC_DINH =
  "Không đọc được nhãn trạng thái của xã, nên màn này đang hiện nhãn và thứ tự mặc định của phần " +
  "mềm. Nhãn xã đã đổi (nếu có) chưa hiện ra.";

/** Kết quả đọc nhãn → bảng để vẽ, và câu cảnh báo khi phải dùng đường lui. */
export type NhanTrangThaiDaDoc = {
  readonly bang: BangNhanTrangThai;
  /** `null` khi đang dùng nhãn của máy chủ, hoặc khi còn đang đọc. */
  readonly canhBao: string | null;
};

/**
 * `null` (chưa đọc xong) → đường lui, KHÔNG cảnh báo: cả màn hình còn đang tải, chưa có gì sai.
 * Đọc hỏng → đường lui KÈM cảnh báo và câu máy chủ.
 * Đọc được mà THIẾU một trong bảy mã → đường lui kèm cảnh báo: máy chủ hứa đủ bảy, nên thiếu là hợp
 * đồng vỡ; ghép nửa chữ xã nửa chữ mặc định sẽ cho ra một bảng không ai đặt ra.
 * Mã ngoài bảy bị BỎ QUA: danh sách mã là đóng (ADR 0035 §C), vòng đời chỉ biết bảy mã ấy.
 *
 * NHÃN VẼ NGUYÊN VĂN chữ máy chủ trả; THỨ TỰ là thứ tự của `items` — máy chủ đã sắp theo thứ tự
 * hiệu lực, hoà thì theo thứ tự mặc định. Không sắp lại ở đây: phép sắp thứ hai là một bản sao của
 * quy tắc hoà, và nó trôi.
 */
export function docBangNhanTrangThai(
  kq: KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null,
): NhanTrangThaiDaDoc {
  if (kq === null) return { bang: BANG_NHAN_MAC_DINH, canhBao: null };
  if (!kq.ok) {
    return { bang: BANG_NHAN_MAC_DINH, canhBao: `${CANH_BAO_NHAN_MAC_DINH} ${kq.thongBao}` };
  }
  const nhan: Partial<Record<TrangThaiNhiemVu, string>> = {};
  const thuTu: TrangThaiNhiemVu[] = [];
  for (const d of kq.duLieu.items) {
    if (!laTrangThaiNhiemVu(d.code) || nhan[d.code] !== undefined) continue;
    nhan[d.code] = d.label;
    thuTu.push(d.code);
  }
  if (thuTu.length !== MOI_TRANG_THAI.length) {
    return { bang: BANG_NHAN_MAC_DINH, canhBao: CANH_BAO_NHAN_MAC_DINH };
  }
  return { bang: { nhan: nhan as Record<TrangThaiNhiemVu, string>, thuTu }, canhBao: null };
}

/**
 * Nhãn để hiện, kể cả khi máy chủ gửi một mã màn hình chưa biết.
 *
 * `bang` BẮT BUỘC, KHÔNG CÓ GIÁ TRỊ MẶC ĐỊNH: một tham số mặc định là đường để một chỗ gọi quên
 * truyền bảng của xã và lặng lẽ hiện chữ mặc định — đúng lỗi "màn hình bỏ qua nhãn xã đã đổi".
 *
 * MÃ LẠ HIỆN NGUYÊN VĂN, KHÔNG HIỆN DẤU GẠCH và không im lặng bỏ qua: một trạng thái mới ở máy
 * chủ mà màn hình vẽ thành `—` là một hồ sơ trông như chưa có trạng thái.
 */
export function nhanTrangThai(bang: BangNhanTrangThai, ma: string): string {
  return laTrangThaiNhiemVu(ma) ? bang.nhan[ma] : ma;
}

/**
 * Xếp một tập mã theo thứ tự hiệu lực của xã. Dùng cho cột Kanban và ô lọc.
 *
 * CỘT KANBAN THEO THỨ TỰ CỦA XÃ (quyết định của lượt này): §4.1 cố định NĂM cột chính, còn thứ tự
 * năm cột ấy lấy theo `order` xã đặt — hợp đồng khai tuyến đọc phục vụ "cột Kanban" và thứ tự là
 * thứ duy nhất xã đổi được ngoài nhãn. Xã chưa đổi gì thì ra đúng thứ tự vòng đời §6.
 */
export function theoThuTuXa<T extends string>(
  ds: readonly T[],
  bang: BangNhanTrangThai,
): T[] {
  const vi = (ma: string) => {
    const i = (bang.thuTu as readonly string[]).indexOf(ma);
    return i < 0 ? Number.MAX_SAFE_INTEGER : i;
  };
  return [...ds].sort((a, b) => vi(a) - vi(b));
}

/**
 * THE LIFECYCLE IS THE SERVER'S, READ FROM THE ROW — `allowed_transitions` (3b2330b).
 *
 * ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: this file used to hold `CHUYEN_DUOC`, a hand-copied second map of
 * `chuyenDuocSangNhiemVu` (`service-petitions/internal/domain/nhiem_vu.go:97-105`). It drifted the day
 * the owner adopted require's table (reopen out of `hoan-thanh`, `dang-thuc-hien` → `hoan-thanh`
 * direct, `cho-duyet` no longer pausable, no history rule on resuming) — with every test green, because
 * the tests pinned the copy. Every task reply now carries the list the write path enforces
 * (`AllowedTransitions`, `nhiem_vu.go:132-145`), so there is nothing left to copy.
 *
 * ⚠ IT IS THE SHAPE, NOT THE CALLER'S RIGHTS (`nhiem_vu.go:136-138`). A move needing `task.approve`
 * is listed for everybody; who may take it is `mayTakeTransition` below.
 */
export function hasTransition(task: Pick<petitions_nhiemVuRa, "allowed_transitions">, target: string): boolean {
  return task.allowed_transitions.includes(target);
}

/**
 * Finished — `hoan-thanh`, BY NAME.
 *
 * ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: this was "the lifecycle has no way out", which made `hoan-thanh` and
 * `chuyen-tiep` terminal. Neither is any more: `hoan-thanh` reopens and a legacy `chuyen-tiep` row
 * moves on. The server says callers meaning "finished" must ask for `hoan-thanh` by name
 * (`nhiem_vu.go:78-83`, `CheckAssignable` in `task_assignment.go:121-126`) — and so does this.
 */
export function ketThuc(ma: string): boolean {
  return ma === "hoan-thanh";
}

/**
 * Câu giải thích dưới dải bước (§5.2: *"Dưới stepper là câu giải thích trạng thái hiện tại"*).
 *
 * Câu của `moi-giao` lấy NGUYÊN VĂN ví dụ đặc tả đưa ra. Sáu câu còn lại viết theo cùng một
 * khuôn: nói ai đang giữ việc và điều gì đang được chờ — không nói lại tên trạng thái.
 */
export function cauGiaiThichTrangThai(ma: string): string {
  switch (ma) {
    case "moi-giao":
      return "Đã giao nhưng người nhận chưa bấm tiếp nhận.";
    case "da-tiep-nhan":
      return "Người nhận đã tiếp nhận nhưng chưa bắt tay vào làm.";
    case "dang-thuc-hien":
      return "Đang làm, chưa báo xong.";
    case "cho-duyet":
      return "Đã báo xong, đang chờ lãnh đạo duyệt hoàn thành.";
    case "hoan-thanh":
      // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: "khép lại ở đây" became false the day reopen existed.
      return "Đã duyệt hoàn thành. Người có quyền duyệt mở lại được, kèm lý do bắt buộc.";
    case "tam-dung":
      // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: the "back to the status before the pause" rule is gone from
      // the server; resuming offers the server's list, and the clerk picks.
      return "Đang tạm dừng. Tiếp tục thì chọn trạng thái việc quay về.";
    case "chuyen-tiep":
      // ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: a legacy `chuyen-tiep` row moves on (`nhiem_vu.go:71-72`).
      return "Đã chuyển cho bộ phận khác theo cách ghi cũ. Bộ phận nhận tiếp nhận hoặc làm tiếp được.";
    default:
      return "";
  }
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BỐN NGUỒN GIAO VIỆC — §3, §4.2
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Bốn mã nguồn giao, danh sách ĐÓNG: mỗi mã trỏ về một quyển sổ gốc KHÁC nhau, nên mã thứ năm là
 * một tích hợp mới chứ không phải một dòng danh mục (`domain.NguonGiao`). Máy chủ TỪ CHỐI — không
 * bỏ qua — một `source` ngoài bốn mã này (`nhiem_vu.go:352`).
 */
export type NguonGiao = "truc-tiep" | "ket-luan-hop" | "van-ban-den" | "phan-anh";

const NHAN_NGUON_GIAO: Readonly<Record<NguonGiao, string>> = {
  // NOT "Giao trực tiếp": testers read it as "assigned to a person" (report 05/10/2026, NV-11). The
  // code means the task was entered on the register itself, not split from a meeting, a document or
  // a petition — it says nothing about who holds it.
  "truc-tiep": "Tạo trên sổ nhiệm vụ",
  "ket-luan-hop": "Từ kết luận họp",
  "van-ban-den": "Từ văn bản đến",
  "phan-anh": "Từ phản ánh",
};

export const MOI_NGUON_GIAO: readonly NguonGiao[] = [
  "truc-tiep",
  "ket-luan-hop",
  "van-ban-den",
  "phan-anh",
];

export function laNguonGiao(ma: string): ma is NguonGiao {
  return (MOI_NGUON_GIAO as readonly string[]).includes(ma);
}

/** Dòng phụ nhỏ dưới tên việc ở bảng Danh sách (§4.2). Mã lạ hiện nguyên văn. */
export function nhanNguonGiao(ma: string): string {
  return laNguonGiao(ma) ? NHAN_NGUON_GIAO[ma] : ma;
}

/**
 * Dòng liên kết ngược về biên bản gốc trong drawer (`04-bien-ban-hop.md` §7.4): `Từ kết luận số 3 —
 * Giao ban tháng 8`. `null` khi nhiệm vụ không tách từ một kết luận.
 *
 * ĐỌC `meeting_id`, KHÔNG ĐỌC `source`: máy chủ chỉ điền bộ ba `meeting_*` khi nó THẬT SỰ nối được
 * về một biên bản còn đó. Một nhiệm vụ `source = ket-luan-hop` mà thiếu `meeting_id` là nhiệm vụ mà
 * một đường dẫn dựng từ `source_id` sẽ trỏ vào hư không.
 *
 * Số kết luận là số ĐÃ CẤP (`conclusion_no`), đúng con số in trên biên bản giấy. Thiếu nó thì câu
 * bỏ con số chứ không bịa một con số.
 */
export function cauTuKetLuan(nv: petitions_nhiemVuRa): string | null {
  if (nv.meeting_id === undefined || nv.meeting_id === "") return null;
  const ten = nv.meeting_title === undefined || nv.meeting_title === "" ? "biên bản họp" : nv.meeting_title;
  return nv.conclusion_no === undefined
    ? `Từ kết luận họp — ${ten}`
    : `Từ kết luận số ${nv.conclusion_no} — ${ten}`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * HẠN XỬ LÝ — và vì sao "quá hạn" KHÔNG BAO GIỜ là một cột
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Múi giờ GHIM cho mọi phép in ngày của màn này.
 *
 * ĐÂY LÀ HẰNG CỦA NỀN TẢNG, KHÔNG PHẢI GIÁ TRỊ CỦA MỘT XÃ (luật 8, bất biến 5): Việt Nam dùng
 * một múi giờ duy nhất trên toàn quốc, nên nó không khác nhau giữa 300 xã. Không ghim thì
 * `due_at` — một mốc `date-time` — in ra ngày khác nhau trên máy đặt múi giờ khác nhau, và một
 * hạn xử lý lệch một ngày là một cam kết với dân bị đọc sai.
 */
const MUI_GIO = "Asia/Ho_Chi_Minh";

/** `20/6/2026` — không đệm số 0, đúng như §5.3, §5.4 và §5.6 in ra. */
const DINH_DANG_NGAY = new Intl.DateTimeFormat("vi-VN", {
  timeZone: MUI_GIO,
  day: "numeric",
  month: "numeric",
  year: "numeric",
});

/**
 * Mốc `date-time` của hợp đồng → `20/6/2026`. `null` → `—`.
 *
 * CHUỖI KHÔNG ĐỌC ĐƯỢC HIỆN NGUYÊN VĂN, không hiện `Invalid Date` và không hiện dấu gạch: một
 * ngày đoán sai trông y hệt một ngày đúng, còn dấu gạch nói dối rằng máy chủ không gửi gì.
 */
export function nhanNgay(mocISO: string | null): string {
  if (mocISO === null || mocISO === "") return O_TRONG;
  const t = Date.parse(mocISO);
  if (Number.isNaN(t)) return mocISO;
  return DINH_DANG_NGAY.format(new Date(t));
}

/**
 * Tình trạng hạn — **SUY RA**, không đọc từ một cột.
 *
 * Luật 10, bất biến 3: quá hạn là phép so `han_xu_ly` với BÂY GIỜ, không bao giờ là một cột hay
 * một cờ được ghi. Một cột `is_overdue` do việc chạy đêm đặt sẽ sai ngay khi việc ấy chạy trễ,
 * khi đồng hồ lệch, hoặc khi thêm một ngày nghỉ lễ — và bản sai là bản đi lên báo cáo.
 *
 * ⚠ `soNgay` LÀ THỜI GIAN TRÔI QUA THEO LỊCH, KHÔNG PHẢI GIỜ LÀM VIỆC. §4.1 và §4.2 in đúng như
 * thế (`Trễ 87 ngày`, `10/9/2026 (trễ 6 ngày)`). Nó KHÔNG được dùng để tính ra một cái hạn: hạn
 * đếm bằng GIỜ LÀM VIỆC và chỉ `identity.AdvanceWorkingHours` tính được, vì chỉ dịch vụ ấy giữ
 * lịch làm việc, `ngay_nghi_le` và `ngay_lam_bu` (luật 10, bất biến 4 và cấm #2, ADR 0007).
 */
export type TinhTrangHan =
  /** Nhiệm vụ không có hạn. Hai cột hạn có hoặc không CÙNG NHAU, nên đây là một trạng thái thật. */
  | { readonly loai: "khong-han" }
  | { readonly loai: "tre"; readonly soNgay: number }
  | { readonly loai: "con-han"; readonly soNgay: number };

const MOT_NGAY_MS = 86_400_000;

/**
 * So hạn với bây giờ. `bayGio` là THAM SỐ chứ không phải `new Date()` đọc trong hàm — một hàm
 * đọc đồng hồ hệ thống là một hàm chỉ kiểm được bằng cách giả lập đồng hồ.
 *
 * KHÔNG CÓ NHÁNH "SẮP ĐẾN HẠN" Ở ĐÂY, có chủ ý. Ngưỡng ấy là `sla.gio_sap_den_han` — **số giờ
 * của TỪNG XÃ** (`service-identity/migrations/0008_sla.sql:178`) — nên nung số 72 vào đây là
 * đúng thứ luật 1 bất biến 10 cấm, và là nguồn thứ hai cho một con số mà mọi màn "sắp đến hạn"
 * phải đọc từ một cột duy nhất. Máy chủ cũng TỪ CHỐI `?soon=` vì đúng lý do ấy
 * (`nhiem_vu.go:386`). Xem `PHAN_CHUA_DUNG`.
 */
export function tinhTrangHan(hanISO: string | null, bayGio: Date): TinhTrangHan {
  if (hanISO === null || hanISO === "") return { loai: "khong-han" };
  const han = Date.parse(hanISO);
  if (Number.isNaN(han)) return { loai: "khong-han" };

  const lech = bayGio.getTime() - han;
  if (lech > 0) return { loai: "tre", soNgay: Math.floor(lech / MOT_NGAY_MS) };
  return { loai: "con-han", soNgay: Math.floor(-lech / MOT_NGAY_MS) };
}

/**
 * Dòng hạn trên thẻ Kanban (§4.1): `Trễ 87 ngày` · `Hạn 20/12/2026` · `Hạn —`.
 *
 * "TRỄ 0 NGÀY" KHÔNG ĐƯỢC IN RA. Một việc vừa quá hạn nửa tiếng vẫn là việc đã trễ, và dòng chữ
 * `Trễ 0 ngày` đọc ra là "chưa trễ" — đúng điều ngược lại.
 */
export function nhanHanThe(hanISO: string | null, bayGio: Date): string {
  const tt = tinhTrangHan(hanISO, bayGio);
  if (tt.loai === "khong-han") return `Hạn ${O_TRONG}`;
  if (tt.loai === "tre") {
    return tt.soNgay < 1 ? "Trễ dưới 1 ngày" : `Trễ ${tt.soNgay} ngày`;
  }
  return `Hạn ${nhanNgay(hanISO)}`;
}

/**
 * Ô hạn ở bảng Danh sách (§4.2): `10/9/2026 (trễ 6 ngày)`, phần trong ngoặc hiện đỏ; hoặc `—`.
 *
 * Trả về HAI MẢNH thay vì một chuỗi ghép sẵn, vì đặc tả tô đỏ riêng phần trễ. Ghép sẵn rồi cắt
 * lại bằng biểu thức chính quy ở tầng vẽ là dựng một phép phân tích trên chính chuỗi mình vừa tạo.
 */
export type OHan = {
  readonly ngay: string;
  /** Rỗng khi việc chưa trễ — tầng vẽ không phải đoán. */
  readonly phanTre: string;
};

/**
 * `trễ 6 ngày` · `trễ dưới 1 ngày` — the lateness words of the list cell (§4.2) and of the Sổ tay
 * row (`03-so-tay-lanh-dao.md` §3), ONE source so the two screens cannot print different figures for
 * the same task. Never `trễ 0 ngày` — see `nhanHanThe`.
 */
export function lateText(soNgay: number): string {
  return soNgay < 1 ? "trễ dưới 1 ngày" : `trễ ${soNgay} ngày`;
}

export function oHan(hanISO: string | null, bayGio: Date): OHan {
  const tt = tinhTrangHan(hanISO, bayGio);
  if (tt.loai === "khong-han") return { ngay: O_TRONG, phanTre: "" };
  if (tt.loai === "tre") {
    return { ngay: nhanNgay(hanISO), phanTre: `(${lateText(tt.soNgay)})` };
  }
  return { ngay: nhanNgay(hanISO), phanTre: "" };
}

/**
 * Chip xanh lá `Hoàn thành trễ hạn` (§4.1, §6).
 *
 * SO VỚI `original_due_at`, KHÔNG SO VỚI `due_at` — và đó là toàn bộ điểm của hai cột. §5.6 và
 * §11 quy tắc 3 nói thẳng: tỷ lệ đúng hạn tính trên `ngay_hoan_thanh ≤ han_ban_dau`, và
 * `HẠN BAN ĐẦU` **không đổi khi gia hạn**. So với `due_at` thì mọi việc được duyệt lùi hạn đều
 * thành "đúng hạn", tức là một lần lùi hạn tự xoá dấu vết của chính nó khỏi báo cáo.
 */
export function hoanThanhTreHan(hoanThanhLucISO: string | null, hanBanDauISO: string | null): boolean {
  if (!hoanThanhLucISO || !hanBanDauISO) return false;
  const xong = Date.parse(hoanThanhLucISO);
  const han = Date.parse(hanBanDauISO);
  if (Number.isNaN(xong) || Number.isNaN(han)) return false;
  return xong > han;
}

/**
 * Chữ của chip trên: `{nhãn hoan-thanh của xã} trễ hạn`.
 *
 * GHÉP TỪ NHÃN CỦA XÃ, KHÔNG GÕ CỨNG "Hoàn thành" (quyết định #21, 24/09/2026 — một nguồn): xã đổi
 * nhãn `hoan-thanh` ở tab Danh mục mà chip vẫn hiện chữ cũ là đúng lỗi im lặng mà bảng nhãn được
 * đọc từ máy chủ để chặn. Đường lui giữ nguyên của `nhanTrangThai`: đọc hỏng thì ra
 * `Hoàn thành trễ hạn`, đúng chữ trước đây.
 *
 * ⚠ Ghép chuỗi giả định nhãn xã đặt là một cụm động từ/tính từ ("Đã hoàn thành" → "Đã hoàn thành
 * trễ hạn"). Một nhãn dạng danh từ sẽ đọc gượng; chưa có quy tắc nào cho ca ấy và ở đây không bịa.
 */
export function nhanHoanThanhTreHan(bang: BangNhanTrangThai): string {
  return `${nhanTrangThai(bang, "hoan-thanh")} trễ hạn`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ADR 0038 — AI DUYỆT ĐƯỢC ĐỀ NGHỊ LÙI HẠN
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Chưa phân công người thực hiện (§4.1, §4.2). Không phải dấu gạch: đây là một trạng thái thật. */
export const CHUA_PHAN_CONG = "Chưa phân công";

/**
 * Nút `Duyệt` / `Từ chối` của §5.8 có hiện hay không — **HAI LỚP, KHÔNG THAY NHAU** (ADR 0038).
 *
 *   task.extend                    tài khoản này có được đụng tới gia hạn không   — CỔNG CỦA TUYẾN,
 *                                  `authz.RequirePermission` ở `routes.go`
 *   ma === maLanhDaoGiaoViec       đây có phải việc của CHÍNH NGƯỜI ẤY không      — ở đây
 *
 * Bỏ lớp một thì ai gọi tuyến cũng được. Bỏ lớp hai thì **mọi lãnh đạo cầm khoá duyệt được mọi
 * nhiệm vụ của cả xã**, kể cả của bộ phận họ không liên quan — vì luật 5 kiểm
 * `(tenant_id, role, permission)` và không có chiều "bản ghi nào". Máy chủ giữ cả hai
 * (`DuocDuyetLuiHan`, `nhiem_vu_ghi.go:676`); màn hình chỉ soi lại cùng hai câu hỏi ấy để cán bộ
 * không bấm vào một thứ chắc chắn bị từ chối.
 *
 * ẨN MỘT NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP (luật 5, cấm #1). Nếu hàm này có ngày trả `hien:
 * true` sai, hậu quả là một màn hình hiện câu từ chối của máy chủ — không phải một lần duyệt lọt.
 *
 * SO SÁNH HAI **MÃ NGHIỆP VỤ CÁN BỘ** (`CB-…`), và đó là toàn bộ tính đúng đắn của nó.
 * `lanh_dao_giao_viec_ma` giữ `CB-…` (migration 0006 đặt tên cột như vậy) và
 * `phien.staff.code` giữ đúng loại giá trị ấy. Đem một id nội bộ ULID ra so với cột này là so
 * HAI LOẠI ĐỊNH DANH KHÁC NHAU: không bao giờ khớp, mọi lãnh đạo rơi vào nhánh "không phải người
 * được ghi", tính năng đơn giản là không chạy — mà không có gì đỏ ở đâu cả, vì cả hai đều là
 * chuỗi khác rỗng trông rất hợp lý (luật 6, bất biến 8; đã đo trong chính kho này 22/09/2026).
 *
 * HAI CHUỖI RỖNG KHÔNG ĐƯỢC PHÉP KHỚP NHAU, và đây là chỗ một dòng thiếu gây hỏng rộng nhất:
 * không có hai phép kiểm rỗng bên dưới thì `"" === ""` là đúng, và **mọi** tài khoản cầm
 * `task.extend` duyệt được **mọi** đề nghị trên **mọi** nhiệm vụ chưa ghi lãnh đạo giao việc.
 */
export type QuyetDinhDuyetLuiHan =
  | { readonly hien: true }
  /** Tài khoản không có `task.extend`. Tuyến sẽ trả 403 trước khi chạm tới quy tắc ADR 0038. */
  | { readonly hien: false; readonly vi: "thieu-quyen" }
  /** Nhiệm vụ chưa ghi lãnh đạo giao việc — ADR 0038 để ngỏ, và câu trả lời là TỪ CHỐI. */
  | { readonly hien: false; readonly vi: "chua-ghi-lanh-dao-giao-viec"; readonly thongBao: string }
  /** Có khoá, nhưng không phải người được ghi trên bản ghi này. */
  | { readonly hien: false; readonly vi: "khong-phai-lanh-dao-giao-viec"; readonly thongBao: string };

/**
 * KHÔNG KHUYÊN "HÃY BỔ SUNG", VÀ ĐÓ LÀ LÝ DO CÂU NÀY ĐƯỢC VIẾT LẠI (27/09/2026). Câu cũ bảo cán bộ
 * bổ sung lãnh đạo giao việc — nhưng `PATCH /api/v1/tasks/{ma}` CỐ Ý không nhận `assigner`
 * (`petitions_suaNhiemVuVao` không có trường ấy; ADR 0038: cột này CHÍNH LÀ người duyệt, nên người
 * cầm `task.update` sửa được nó là tự đặt mình làm người duyệt). Một lời khuyên không làm theo được
 * là một lời khuyên sai, và cán bộ sẽ đi tìm một ô không tồn tại.
 */
export const CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC =
  "Nhiệm vụ này không ghi lãnh đạo giao việc, nên không ai duyệt được đề nghị lùi hạn. Lãnh đạo " +
  "giao việc chỉ ghi được lúc giao việc, không bổ sung được trên màn hình này.";

/**
 * Tài khoản chưa được cấp quyền `Duyệt gia hạn` (`task.extend`). Nói ra ở hàng chờ, nơi đề nghị
 * đã hiện sẵn: một dòng mất hai nút mà không một lời là cán bộ tưởng màn hình hỏng.
 */
export const CAU_THIEU_QUYEN_DUYET_GIA_HAN =
  "Tài khoản của bạn chưa được cấp quyền duyệt gia hạn, nên không duyệt hay từ chối được đề nghị này.";

export const CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC =
  "Chỉ lãnh đạo giao việc ghi trên nhiệm vụ này mới duyệt được đề nghị lùi hạn.";

export function quyetDinhDuyetLuiHan(
  /** `phien.staff.code` — mã nghiệp vụ của người đang đăng nhập (`CB-…`). */
  maNguoiDangNhap: string,
  /** `assigner` của nhiệm vụ — chính là `lanh_dao_giao_viec_ma`. */
  maLanhDaoGiaoViec: string,
  /** Tài khoản có khoá `task.extend` hay không — lớp CỔNG, do `lib/quyen.ts` trả lời. */
  coQuyenDuyetGiaHan: boolean,
): QuyetDinhDuyetLuiHan {
  // LỚP MỘT TRƯỚC. Thiếu khoá thì tuyến trả 403 bất kể người ấy có tên trên bản ghi hay không,
  // nên đây là câu trả lời đúng và cũng là câu trả lời rẻ nhất.
  if (!coQuyenDuyetGiaHan) return { hien: false, vi: "thieu-quyen" };

  if (maLanhDaoGiaoViec === "") {
    return {
      hien: false,
      vi: "chua-ghi-lanh-dao-giao-viec",
      thongBao: CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC,
    };
  }
  // Phiên không có mã cán bộ thì không so được với một cột giữ mã cán bộ. FAIL CLOSED.
  if (maNguoiDangNhap === "" || maNguoiDangNhap !== maLanhDaoGiaoViec) {
    return {
      hien: false,
      vi: "khong-phai-lanh-dao-giao-viec",
      thongBao: CAU_KHONG_PHAI_LANH_DAO_GIAO_VIEC,
    };
  }
  return { hien: true };
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ĐẾM VÀ CÂU CHỮ CHUNG
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Chân trang §2: `Hiển thị {N} nhiệm vụ.` */
export function nhanBoDem(n: number): string {
  return `Hiển thị ${n} nhiệm vụ.`;
}

/** Hàng lọc 2 khi có dòng được tick (§2): `Đã chọn {N} nhiệm vụ`. */
export function nhanDaChon(n: number): string {
  return `Đã chọn ${n} nhiệm vụ`;
}

/** Cột Kanban rỗng (§4.1, phụ lục 15 §6). */
export const COT_RONG = "Không có nhiệm vụ";

/** Nhật ký rỗng (§5.9, phụ lục 15 §6). */
export const NHAT_KY_RONG = "Chưa có ghi chép nào.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHẬT KÝ & TRAO ĐỔI §5.9 — NỬA ĐỌC (`GET /api/v1/tasks/{ma}/log-entries`)
 *
 * ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W2): ô ghi tay `Đã làm được gì, còn vướng gì…` NAY CÓ —
 * `POST /api/v1/tasks/{ma}/log-entries` (60011e8). `📎 Đính kèm` có từ A4 (ADR 0052, b37ec2d).
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Tiêu đề khối — nguyên văn §5.9. */
export const TIEU_DE_NHAT_KY_NHIEM_VU = "Nhật ký & Trao đổi";

/** Đang đọc trang đầu. `role="status"`, không phải `alert`. */
export const DANG_TAI_NHAT_KY_NHIEM_VU = "Đang tải nhật ký…";

export const NHAN_XEM_THEM_NHAT_KY_NHIEM_VU = "Xem thêm";

/* ── MANUAL ENTRY (§5.9) ─────────────────────────────────────────────────────────────────── */

/**
 * Placeholder and button — verbatim §5.9, except the `➤` glyph: the button draws a lucide icon
 * instead (ADR 0068 — no emoji as icons). The words are unchanged.
 */
export const LOG_ENTRY_PLACEHOLDER = "Đã làm được gì, còn vướng gì…";
export const LOG_ENTRY_BUTTON = "Ghi nhật ký";
/** A visible label: a placeholder alone vanishes on the first keystroke and is no label (a11y). */
export const LOG_ENTRY_LABEL = "Ghi vào nhật ký của nhiệm vụ";
/** The table is append-only (rule 7): said BEFORE the click, since nothing undoes it. */
export const LOG_ENTRY_NOTE = "Dòng đã ghi không sửa, không xoá được — đọc lại trước khi bấm ghi.";
export const LOG_ENTRY_DONE = "Đã ghi vào nhật ký.";

/**
 * May this account see the entry form on this task — the server's `TaskWorkRightFor`
 * (`service-petitions/internal/domain/task_participant.go:60-76`), read the same way: `task.update`;
 * else, with a non-empty session code, the assignee, the assigner or the creator. There is no
 * separate monitor since ADR 0065 NV5 (user decision 30/09/2026): "chuyên viên theo dõi" IS the
 * assignee, and the response no longer carries a `monitor` field.
 *
 * CONVENIENCE, NOT PROTECTION (rule 5, forbidden #1): the server decides again on the row read FOR
 * UPDATE, and its 403 shows verbatim. The empty-code check is the one that matters: without it
 * `"" === ""` would offer the form on every task with an unset assigner to every unread session.
 */
export function canWriteLogEntry(
  quyen: QuyenNhiemVu,
  task: Pick<petitions_nhiemVuRa, "assignee" | "assigner" | "created_by">,
  staffCode: string,
): boolean {
  if (quyen.capNhat) return true;
  if (staffCode === "") return false;
  return [task.assignee, task.assigner, task.created_by].includes(staffCode);
}

/** The text to send, or `null` (button disabled): trimmed, as the server trims before it checks. */
export function logEntryNote(text: string): string | null {
  const s = text.trim();
  return s === "" ? null : s;
}

/**
 * Người ghi một dòng: `Họ tên (CB-…)` khi danh bạ có họ tên, còn lại là MÃ.
 *
 * MÃ LUÔN CÒN TRÊN DÒNG, kể cả khi đã có họ tên: nhật ký được đọc lại lúc khiếu nại, và mã là thứ
 * còn chỉ ra được đúng một người nhiều năm sau — hai người trùng họ tên thì họ tên không làm được
 * (luật 6, bất biến 8). Danh bạ chỉ gồm tài khoản đang hoạt động, nên người đã nghỉ chỉ còn mã —
 * đó là một ca bình thường của nhật ký, không phải lỗi.
 *
 * Rỗng thì hiện gạch: máy chủ hứa luôn có mã, nên một ô trống là hợp đồng vỡ, và ô trống đọc ra là
 * "không ai làm".
 */
export function nhanNguoiNhatKy(ma: string, danhBa: DanhBaTheoMa | null): string {
  if (ma === "") return O_TRONG;
  return staffNameWithCode(ma, danhBa);
}

/**
 * Danh bạ màn hình đã đọc (TASK-05, một lần cho cả màn) → bảng tra theo mã cho nhật ký.
 *
 * `null` khi còn đang tải HOẶC tải hỏng: cả hai ca, dòng nhật ký hiện MÃ — thứ duy nhất màn hình
 * biết chắc. Câu lỗi của danh bạ đã hiện ở ô lọc và form giao việc; nhắc lại trong drawer là câu
 * thứ ba cho cùng một sự cố.
 */
export function danhBaChoNhatKy(kq: KetQua<identity_danhBaChonNguoiRa> | null): DanhBaTheoMa | null {
  const db = docDanhBaChonNguoi(kq);
  if (db.dangTai || db.loi !== null) return null;
  return danhBaTheoMa(db.ds);
}

/**
 * Tên một cán bộ trên thẻ Kanban và cột `Người thực hiện` của bảng §4.2 — HỌ TÊN khi danh bạ có,
 * còn lại là MÃ `CB-…`. Mã rỗng thì `rong` (`Chưa phân công` hay `—`, tuỳ ô).
 *
 * KHÔNG BAO GIỜ TRẢ CHUỖI RỖNG cho một mã có thật. Người đã nghỉ, người bị khoá và mọi lần danh bạ
 * đọc hỏng đều rơi vào nhánh "không có trong danh bạ" — và ô ấy vẫn phải chỉ ra được đúng một người,
 * tức là hiện mã. Một ô trống đọc ra là "chưa giao cho ai", đúng điều ngược lại.
 *
 * Ô CHẬT THÌ CHỈ HỌ TÊN; drawer thì `Họ tên (CB-…)` qua `nhanNguoiNhatKy` — xem `nhanCanBoDrawer`.
 */
export function nhanCanBoNgan(ma: string, danhBa: DanhBaTheoMa | null, rong: string): string {
  if (ma === "") return rong;
  const cb = danhBa?.get(ma);
  return cb === undefined || cb.full_name === "" ? ma : cb.full_name;
}

/**
 * Who holds a task, on a Kanban card: the assignee's name; with no assignee, the holding unit
 * `{unit} · Chưa phân công` (report 05/10/2026, NV-11) — "Chưa phân công" alone hid which unit
 * still owed the work. A unit missing from the catalogue shows its id, as in the timeline.
 */
export function cardHolderText(
  task: Pick<petitions_nhiemVuRa, "assignee" | "unit">,
  danhBa: DanhBaTheoMa | null,
  unitNames: ReadonlyMap<string, string>,
): string {
  if (task.assignee !== "" || task.unit === "") {
    return nhanCanBoNgan(task.assignee, danhBa, CHUA_PHAN_CONG);
  }
  return `${unitNames.get(task.unit) ?? task.unit} · ${CHUA_PHAN_CONG}`;
}

/**
 * Tên một cán bộ trong drawer §5 — `Họ tên (CB-…)`, hoặc mã khi danh bạ không có.
 *
 * MÃ CÒN LẠI TRÊN DÒNG vì drawer là chỗ đối chiếu hồ sơ: hai người trùng họ tên thì chỉ mã phân biệt
 * được (luật 6, bất biến 8). Cùng phép với nhật ký §5.9, nên gọi lại đúng hàm ấy.
 */
export function nhanCanBoDrawer(ma: string, danhBa: DanhBaTheoMa | null, rong: string): string {
  return ma === "" ? rong : nhanNguoiNhatKy(ma, danhBa);
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CỔNG NÚT THEO KHOÁ `task.*` — TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP
 *
 * Máy chủ kiểm từng khoá trên TỪNG yêu cầu (`authz.RequirePermission`, luật 5 cấm #1). Việc của
 * khối này chỉ là để cán bộ không bấm vào một nút chắc chắn trả 403. Nếu nó có ngày trả `true`
 * sai, hậu quả là một câu 403 nguyên văn trên màn — không phải một lần ghi lọt.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Tài khoản đang đăng nhập được bấm những nút nào của màn Nhiệm vụ. */
export type QuyenNhiemVu = {
  /** `+ Giao việc mới` — `task.create`. */
  readonly giaoViec: boolean;
  /** `✎ Sửa`, khối Chuyển trạng thái, ô GỬI đề nghị lùi hạn — `task.update`. */
  readonly capNhat: boolean;
  /**
   * `task.approve` — the moves `transitionNeedsApproval` names. ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: this
   * was `task.update` AND `task.approve`. Since ea55113 the status route's gate is `task.read` and the
   * row admits the ASSIGNEE without `task.update`, so an assignee holding `task.approve` may approve.
   * Who may move the row at all is `canMoveTask`, not this key.
   */
  readonly duyetHoanThanh: boolean;
  /** Xoá khỏi sổ — `task.delete`. */
  readonly xoa: boolean;
  /** LỚP MỘT của duyệt lùi hạn — `task.extend`. Lớp hai (ADR 0038) vẫn là `quyetDinhDuyetLuiHan`. */
  readonly duyetGiaHan: boolean;
  /**
   * The §5.7 block `Giao việc, chuyển việc` — `task.assign` ALONE. Not implied by `task.update`:
   * the assignment route declares `task.assign` and nothing else (`TASK_ASSIGN_PERMISSION`).
   */
  readonly reassign: boolean;
};

/**
 * Danh sách quyền của phiên → cổng từng nút. `null` = phiên chưa đọc xong hoặc đọc hỏng.
 *
 * FAIL CLOSED: không đọc được quyền thì MỌI cổng đóng. "Chưa rõ" không được hành xử như "có" (luật
 * 1, cấm #1). Mỗi khoá so CHÍNH XÁC qua `coQuyen` — không tiền tố, không `task.*` (luật 5, bất biến 3b).
 *
 * `duyetHoanThanh` is `task.approve` ALONE — see the field.
 */
export function quyenNhiemVu(dsQuyen: readonly string[] | null): QuyenNhiemVu {
  const ds = dsQuyen ?? [];
  return {
    giaoViec: coQuyen(ds, QUYEN_TAO_NHIEM_VU),
    capNhat: coQuyen(ds, QUYEN_CAP_NHAT_NHIEM_VU),
    duyetHoanThanh: coQuyen(ds, QUYEN_DUYET_HOAN_THANH_NHIEM_VU),
    xoa: coQuyen(ds, QUYEN_XOA_NHIEM_VU),
    duyetGiaHan: coQuyen(ds, QUYEN_DUYET_GIA_HAN),
    reassign: coQuyen(ds, TASK_ASSIGN_PERMISSION),
  };
}

/**
 * The row lists a move needing `task.approve` and the account lacks it. Said out loud: a button that
 * vanishes without a word reads as "the lifecycle lacks a step" — the clerk files a bug instead of
 * asking for the right.
 */
export const CAU_THIEU_QUYEN_DUYET_HOAN_THANH =
  "Duyệt hoàn thành hoặc trả lại việc đang chờ duyệt, và mở lại việc đã hoàn thành, cần quyền duyệt " +
  "hoàn thành. Tài khoản của bạn chưa được cấp quyền này.";

/* ── WHO MAY MOVE A ROW, AND WHICH MOVES ───────────────────────────────────────────────────────
 *
 * Every gate here is CONVENIENCE (rule 5, forbidden #1): `POST /api/v1/tasks/{ma}/status` decides
 * all of it again on the row under the lock, and its refusal shows verbatim.
 *
 * Two layers, both the server's (ea55113, 3b2330b):
 *   1. the ROW — the task's assignee, or a holder of `task.update` (`canMoveTask`);
 *   2. the MOVE — the sign-off `cho-duyet` → `hoan-thanh`, the reopen out of `hoan-thanh` and the
 *      return from review also need `task.approve` (`transitionNeedsApproval`, the server's
 *      `NeedsApproval`). The direct `dang-thuc-hien` → `hoan-thanh` does not (ADR 0065 NV1).
 *
 * `transitionNeedsApproval` IS A COPY of a server predicate — the one the reply does NOT carry
 * (`allowed_transitions` is deliberately not filtered by the caller's keys). It decides only whether a
 * button SHOWS: too wide ⇒ a button that earns a verbatim 403, too narrow ⇒ a missing button the clerk
 * reports. Neither is a wrong write.
 */

/** `domain.IsReopen` — signed-off work taken back into progress (`nhiem_vu.go:168`). */
export function isReopen(from: string, to: string): boolean {
  return from === "hoan-thanh" && to === "dang-thuc-hien";
}

/**
 * `domain.NeedsApproval` (`service-petitions/internal/domain/nhiem_vu.go:189-191`, 90a17153).
 *
 * THE KEY FOLLOWS THE SOURCE `cho-duyet`, NOT THE TARGET `hoan-thanh` (ADR 0065 NV1, 30/09/2026):
 * review is optional, so the assignee completes straight from `dang-thuc-hien` without
 * `task.approve`; work already sent up for review waits for a reviewer. Keying on the target again
 * would hide a completion the server allows — the clerk would read it as "I cannot finish my task".
 */
export function transitionNeedsApproval(from: string, to: string): boolean {
  return (from === "cho-duyet" && to === "hoan-thanh") || isReopen(from, to) || laBuocTraLai(from, to);
}

/**
 * Moves that carry a MANDATORY reason, so they are never a plain `Chuyển sang …` button (a click
 * would send an empty note): the return from review, and the reopen. The reopen's reason is required
 * by this screen already; the server is about to require it too (backend card in progress).
 */
export function transitionNeedsReason(from: string, to: string): boolean {
  return laBuocTraLai(from, to) || isReopen(from, to);
}

/**
 * May this account move this ROW at all — `task.update`, or it is the task's assignee.
 *
 * FAIL CLOSED on the comparison: an empty session code (session unread) matches nothing, including a
 * task with no assignee — `"" === ""` would otherwise hand every unassigned task to every account.
 * Both sides are business codes (`CB-…`), never ids (`staff.code`, `nhiem_vu.assignee`).
 */
export function canMoveTask(
  quyen: QuyenNhiemVu,
  task: Pick<petitions_nhiemVuRa, "assignee">,
  staffCode: string,
): boolean {
  return quyen.capNhat || (staffCode !== "" && staffCode === task.assignee);
}

/** One move, both layers: listed by the server, row allowed, approval held if needed. */
export function mayTakeTransition(
  quyen: QuyenNhiemVu,
  task: Pick<petitions_nhiemVuRa, "assignee" | "status" | "allowed_transitions">,
  staffCode: string,
  target: string,
): boolean {
  if (!hasTransition(task, target) || !canMoveTask(quyen, task, staffCode)) return false;
  return transitionNeedsApproval(task.status, target) ? quyen.duyetHoanThanh : true;
}

/**
 * The plain `Chuyển sang …` steps this account may take on this task — the server's list, minus the
 * steps needing a reason (their own form, `KhoiTraLai`), minus what `mayTakeTransition` refuses.
 *
 * ONE LIST FOR THE DRAWER AND THE KANBAN. Two filters would drift, and the drifted one would offer a
 * Kanban move the drawer hides — or hide one the drawer offers. Order is `MOI_TRANG_THAI`; a code the
 * screen does not know is left out (no label, no column) rather than drawn as a raw button.
 */
export function clickableTransitions(
  task: Pick<petitions_nhiemVuRa, "assignee" | "status" | "allowed_transitions">,
  quyen: QuyenNhiemVu,
  staffCode: string,
): TrangThaiNhiemVu[] {
  return MOI_TRANG_THAI.filter(
    (t) => !transitionNeedsReason(task.status, t) && mayTakeTransition(quyen, task, staffCode, t),
  );
}

/** Which reason form the drawer shows for this task and account, or `null`. */
export type ReasonMove = "return" | "reopen";

export function reasonMove(
  task: Pick<petitions_nhiemVuRa, "assignee" | "status" | "allowed_transitions">,
  quyen: QuyenNhiemVu,
  staffCode: string,
): ReasonMove | null {
  if (!mayTakeTransition(quyen, task, staffCode, "dang-thuc-hien")) return null;
  if (laBuocTraLai(task.status, "dang-thuc-hien")) return "return";
  if (isReopen(task.status, "dang-thuc-hien")) return "reopen";
  return null;
}

/**
 * The account may move the row, and the server lists an approval move it cannot take — the case
 * `CAU_THIEU_QUYEN_DUYET_HOAN_THANH` exists for.
 */
export function lacksApprovalFor(
  task: Pick<petitions_nhiemVuRa, "assignee" | "status" | "allowed_transitions">,
  quyen: QuyenNhiemVu,
  staffCode: string,
): boolean {
  return (
    canMoveTask(quyen, task, staffCode) &&
    !quyen.duyetHoanThanh &&
    task.allowed_transitions.some((t) => transitionNeedsApproval(task.status, t))
  );
}

/* ── MOVING A CARD ON THE KANBAN (§4.1) ─────────────────────────────────────────────────────
 *
 * Two paths, one route: drag a card onto a column, or the card's `Chuyển sang cột…` menu. Both
 * send `POST /api/v1/tasks/{ma}/status` — the drawer's route — and neither moves the card before
 * the server answers: a card that jumps and then silently jumps back is a change the clerk
 * believes happened.
 */

/** The card's menu button. Visible text; the accessible name adds the code (`kanbanMoveButtonName`). */
export const KANBAN_MOVE_BUTTON = "Chuyển sang cột…";

/** Accessible name: contains the visible text (WCAG 2.5.3) plus which card, among twenty. */
export function kanbanMoveButtonName(code: string): string {
  return `${KANBAN_MOVE_BUTTON} (${code})`;
}

/** A branch status has no column — say the card will leave the board, before it does. */
export function kanbanMoveItemLabel(bang: BangNhanTrangThai, target: string): string {
  const label = nhanTrangThai(bang, target);
  return (TRANG_THAI_CHINH as readonly string[]).includes(target)
    ? label
    : `${label} — thẻ sẽ rời bảng Kanban`;
}

export function kanbanMovePendingText(bang: BangNhanTrangThai, code: string, target: string): string {
  return `Đang chuyển ${code} sang “${nhanTrangThai(bang, target)}”… Thẻ ở nguyên cột cũ cho tới khi máy chủ trả lời.`;
}

export function kanbanMoveDoneText(bang: BangNhanTrangThai, code: string, target: string): string {
  return `Đã chuyển ${code} sang “${nhanTrangThai(bang, target)}”.`;
}

/**
 * Prefix of a refusal. The server's sentence follows VERBATIM in the same line — it is the only
 * place the list of unfinished child tasks reaches the screen.
 */
export function kanbanMoveRefusedPrefix(bang: BangNhanTrangThai, target: string): string {
  return `Chưa chuyển được sang “${nhanTrangThai(bang, target)}”.`;
}

export function kanbanDropHint(bang: BangNhanTrangThai, target: string): string {
  return `Thả vào đây để chuyển sang “${nhanTrangThai(bang, target)}”.`;
}

/** The return step is not in the menu; say where it is instead of letting it look missing. */
export const KANBAN_RETURN_NOTE =
  "Trả lại để làm tiếp cần ghi lý do — bấm “Mở” trên thẻ và dùng khối “Trả lại để làm tiếp” " +
  "trong phần chi tiết.";

/**
 * Cặp (từ, sang) này có phải bước trả lại không — cùng vị từ với `domain.LaTraLaiLamTiep`.
 *
 * `note` là LÝ DO, bắt buộc, cắt khoảng trắng trước khi kiểm (`KiemLyDoTraLai`) — người nhận lại việc
 * phải biết còn thiếu gì. `da-tiep-nhan` → `dang-thuc-hien` là bước THUẬN thường, nên câu hỏi là về
 * CẶP (từ, sang), không bao giờ chỉ về trạng thái đích.
 */
export function laBuocTraLai(tu: string, sang: string): boolean {
  return tu === "cho-duyet" && sang === "dang-thuc-hien";
}

/** Reopen form texts (`KhoiTraLai kind="reopen"`). */
export const REOPEN_BUTTON = "Mở lại để làm tiếp";
export const REOPEN_REASON_LABEL = "Lý do mở lại (bắt buộc)";

/**
 * Under the reopen title. The server clears the completion date on reopen and keeps the old one only
 * in the timeline row and the audit entry (`nhiem_vu.go:161-167`) — said, so nobody reads the empty
 * date as "never finished".
 */
export function reopenNote(bang: BangNhanTrangThai): string {
  return (
    `Nhiệm vụ đã hoàn thành quay về trạng thái “${nhanTrangThai(bang, "dang-thuc-hien")}”. Ngày hoàn ` +
    "thành cũ và lý do được ghi vào nhật ký."
  );
}

/**
 * Giới hạn của máy chủ: lý do CHÍNH LÀ dòng nhật ký mà bước ấy ghi, nên dùng chung
 * `NoiDungNhatKyToiDa` (`service-petitions/internal/domain/nhiem_vu_ghi.go:43`). Ô dừng đúng chỗ;
 * lệch khỏi máy chủ thì câu từ chối của máy chủ vẫn ra nguyên văn.
 */
export const LY_DO_TRA_LAI_TOI_DA = 5000;

/**
 * Server bound for one timeline entry — the SAME `NoiDungNhatKyToiDa` the return reason uses
 * (`LY_DO_TRA_LAI_TOI_DA`), because both are one row's `noi_dung`. One constant, not a second 5000. Declared AFTER it: a
 * `const` read before its line throws at module load.
 */
export const LOG_ENTRY_MAX = LY_DO_TRA_LAI_TOI_DA;

export const NHAN_NUT_TRA_LAI = "Trả lại để làm tiếp";
export const NHAN_LY_DO_TRA_LAI = "Lý do trả lại (bắt buộc)";
/**
 * Câu dưới tiêu đề ô trả lại. Tên trạng thái đích lấy từ BẢNG NHÃN CỦA XÃ (quyết định #21 — một
 * nguồn): xã đổi "Đang thực hiện" thành chữ khác mà câu này vẫn nói chữ cũ là cán bộ đọc hai tên cho
 * một trạng thái.
 */
export function ghiChuTraLai(bang: BangNhanTrangThai): string {
  return (
    `Nhiệm vụ quay về trạng thái “${nhanTrangThai(bang, "dang-thuc-hien")}”. Lý do được ghi vào ` +
    "nhật ký để người thực hiện biết còn thiếu gì."
  );
}

/**
 * Ô lý do → hai đối số của `doiTrangThaiNhiemVu`, hoặc `null` khi chưa có lý do — nút gửi khoá.
 *
 * CẮT Ở ĐÂY, CÙNG PHÉP VỚI MÁY CHỦ: `"   "` bị máy chủ từ chối y như `""`, nên để nút mở với một lý do
 * toàn khoảng trắng là mời cán bộ bấm vào một câu 400. Trạng thái đích nằm TRONG hàm này, không ở chỗ
 * gọi: ô trả lại chỉ có một đích, và một chỗ gọi gõ nhầm đích là một bước tiến gửi kèm lý do trả lại.
 */
export function yeuCauTraLai(
  chuoi: string,
): { readonly trangThai: "dang-thuc-hien"; readonly ghiChu: string } | null {
  const s = chuoi.trim();
  return s === "" ? null : { trangThai: "dang-thuc-hien", ghiChu: s };
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §7.1 — MỨC ƯU TIÊN MẶC ĐỊNH
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Mã mức ưu tiên xã đặt làm MẶC ĐỊNH, hoặc `""` (= `— Chưa xác định —`) khi xã chưa đặt dòng nào.
 *
 * KHÔNG GÕ CỨNG `Thường`. §7.1 viết `Thường (mặc định)`, nhưng thang ưu tiên là DANH MỤC CỦA XÃ
 * (`GET /api/v1/task-priorities`, cột `is_default`): xã đổi tên, đổi mức mặc định, hoặc không có mức
 * `Thường` nào. Nung chữ ấy vào bundle là một giá trị riêng của xã bị nung vào mã (luật 1, bất biến 10).
 *
 * CHỈ DÒNG ĐANG DÙNG. Một dòng mặc định đã ngừng dùng mà vẫn được chọn sẵn là chọn sẵn thứ xã đã bỏ.
 */
export function mucUuTienMacDinh(ds: readonly petitions_mucUuTienRa[]): string {
  return ds.find((m) => m.is_default && m.active)?.code ?? "";
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §3 — BỘ LỌC ĐỒNG BỘ VÀO ĐƯỜNG DẪN ("nên đồng bộ vào query string để chia sẻ link")
 *
 * CHÍN THAM SỐ, ĐÚNG CHÍN BỘ LỌC MÁY CHỦ NHẬN VÀ Ô LỌC ĐANG VẼ — cùng tên với tham số của
 * `GET /api/v1/tasks` (`petitions_get_tasks["truyVan"]`), để một đường dẫn chia sẻ đọc lên là đúng
 * câu hỏi gửi máy chủ. CỘNG `sort` VÀ `order` của bảng Danh sách §4.2: một đường dẫn chia sẻ "việc
 * xếp theo mã" phải mở ra đúng thứ tự ấy.
 *
 * `cursor` KHÔNG LÊN ĐƯỜNG DẪN: con trỏ sống vài giây giữa hai trang và gắn với đúng một cách sắp
 * (`core/page/page.go:456-457`); một đường dẫn chia sẻ mang nó sẽ là trang lỗi 400 cho người nhận
 * ngay khi họ đổi thứ tự.
 *
 * `q` (Ô TÌM) CỐ Ý KHÔNG LÊN ĐƯỜNG DẪN TRÌNH DUYỆT. Chữ tìm là chữ cán bộ gõ tự do, và tiêu đề
 * nhiệm vụ có thể mang tên người dân (một việc giao từ phản ánh). Thanh địa chỉ đi vào lịch sử
 * trình duyệt, vào dấu trang, vào đường dẫn dán sang Zalo — những nơi không ai kiểm soát (luật 3,
 * cấm #4). Cái giá: một đường dẫn chia sẻ không mang theo từ khoá tìm.
 *
 * ĐỌC VÀO THÌ LỌC HẸP, KHÔNG TIN ĐƯỜNG DẪN. Tham số lạ bị bỏ qua. Ba tham số mà máy chủ trả **400**
 * cho giá trị sai (`status`, `source`, `late`) và `scope` chỉ được nhận đúng giá trị hợp lệ — một
 * đường dẫn gõ sai không được biến quyển sổ thành một trang lỗi. `scope=related` và `soon` máy chủ
 * từ chối hẳn (xem `PHAN_CHUA_DUNG`), nên cũng bị bỏ. Năm tham số còn lại máy chủ không kiểm: mã lạ
 * ⇒ trang rỗng, đúng như ô lọc chọn một mã lạ.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Bộ lọc đồng bộ được — cùng hình với phần lọc của `LocNhiemVu`, TRỪ `tim` (xem khối trên). */
export type LocTrenDuongDan = {
  phamVi?: "mine" | "related";
  dueSoon?: true;
  trangThai?: string;
  nguonGiao?: string;
  loai?: string;
  khoi?: string;
  mucUuTien?: string;
  boPhanID?: string;
  nguoiThucHienMa?: string;
  chiTreHan?: true;
  sapXep?: CotSapXepNhiemVu;
  chieu?: ChieuSapXepNhiemVu;
};

/** Độ dài tối đa nhận từ đường dẫn cho một mã tự do. Dài hơn là đường dẫn hỏng, không phải một mã. */
const MA_TREN_DUONG_DAN_TOI_DA = 100;

function maTuDo(v: string | null): string | undefined {
  if (v === null) return undefined;
  const s = v.trim();
  return s === "" || s.length > MA_TREN_DUONG_DAN_TOI_DA ? undefined : s;
}

/** `?status=…&late=true…` → bộ lọc. Nhận chuỗi `location.search` (có hoặc không có `?`). */
export function locTuDuongDan(search: string): LocTrenDuongDan {
  const t = new URLSearchParams(search);
  const loc: LocTrenDuongDan = {};

  const scope = t.get("scope");
  if (scope === "mine" || scope === "related") loc.phamVi = scope;
  if (t.get("soon") === "true") loc.dueSoon = true;
  const status = t.get("status");
  if (status !== null && laTrangThaiNhiemVu(status)) loc.trangThai = status;
  const source = t.get("source");
  if (source !== null && laNguonGiao(source)) loc.nguonGiao = source;
  if (t.get("late") === "true") loc.chiTreHan = true;

  const loai = maTuDo(t.get("type"));
  if (loai !== undefined) loc.loai = loai;
  const khoi = maTuDo(t.get("bloc"));
  if (khoi !== undefined) loc.khoi = khoi;
  const uuTien = maTuDo(t.get("priority"));
  if (uuTien !== undefined) loc.mucUuTien = uuTien;
  const boPhan = maTuDo(t.get("unit"));
  if (boPhan !== undefined) loc.boPhanID = boPhan;
  const nguoi = maTuDo(t.get("assignee"));
  if (nguoi !== undefined) loc.nguoiThucHienMa = nguoi;

  // Hai tham số máy chủ trả 400 cho giá trị lạ (`page.New`, `core/page/page.go:336-355`) — nên chỉ
  // nhận đúng giá trị hợp lệ, cùng quy tắc với `status`.
  const sort = t.get("sort");
  if (sort !== null && laCotSapXep(sort)) loc.sapXep = sort;
  const order = t.get("order");
  if (order === "asc" || order === "desc") loc.chieu = order;

  return loc;
}

/**
 * Bộ lọc → chuỗi truy vấn cho thanh địa chỉ, `""` khi không lọc gì. KHÔNG BAO GIỜ mang `q`.
 *
 * Nhận nguyên bộ lọc của màn hình, kể cả `tim`: tham số ấy bị bỏ ở ĐÂY, tại đúng một chỗ, để không
 * nơi gọi nào phải nhớ bỏ nó.
 */
export function duongDanTuLoc(loc: {
  readonly phamVi?: string;
  readonly trangThai?: string;
  readonly nguonGiao?: string;
  readonly loai?: string;
  readonly khoi?: string;
  readonly mucUuTien?: string;
  readonly boPhanID?: string;
  readonly nguoiThucHienMa?: string;
  readonly chiTreHan?: boolean;
  readonly dueSoon?: boolean;
  readonly tim?: string;
  readonly sapXep?: CotSapXepNhiemVu;
  readonly chieu?: ChieuSapXepNhiemVu;
}): string {
  const t = new URLSearchParams();
  if (loc.phamVi === "mine" || loc.phamVi === "related") t.set("scope", loc.phamVi);
  if (loc.trangThai) t.set("status", loc.trangThai);
  if (loc.nguonGiao) t.set("source", loc.nguonGiao);
  if (loc.loai) t.set("type", loc.loai);
  if (loc.khoi) t.set("bloc", loc.khoi);
  if (loc.mucUuTien) t.set("priority", loc.mucUuTien);
  if (loc.boPhanID) t.set("unit", loc.boPhanID);
  if (loc.nguoiThucHienMa) t.set("assignee", loc.nguoiThucHienMa);
  if (loc.chiTreHan === true) t.set("late", "true");
  if (loc.dueSoon === true) t.set("soon", "true");
  if (loc.sapXep) t.set("sort", loc.sapXep);
  if (loc.chieu) t.set("order", loc.chieu);
  return t.toString();
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §4.2 — SẮP XẾP BẢNG DANH SÁCH ("cột có thể sắp xếp (icon ⇅)")
 *
 * NĂM CỘT, đúng năm cột máy chủ sắp được (`petitions_get_tasks["truyVan"]["sort"]`): `created_at`,
 * `code`, `due_at` (ad7f821), và — ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3b, backend P9) — `title` và
 * `priority`. `priority` xếp theo THỨ TỰ XÃ ĐẶT ở danh mục mức ưu tiên (`thu_tu`), không theo mã chữ
 * cái (`store/nhiem_vu.go:104-109`). Việc KHÔNG CÓ HẠN và việc KHÔNG CÓ MỨC ƯU TIÊN luôn nằm CUỐI ở
 * cả hai chiều (máy chủ quyết, hai câu `…_LAST_NOTE` nói ra). Không mũi tên nào sắp ở trình duyệt:
 * sắp trang đang mở — 20 dòng — trông như sắp cả sổ mà không phải.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

const MOI_COT_SAP_XEP: readonly CotSapXepNhiemVu[] = ["created_at", "code", "due_at", "title", "priority"];

function laCotSapXep(s: string): s is CotSapXepNhiemVu {
  return (MOI_COT_SAP_XEP as readonly string[]).includes(s);
}

/** Cách sắp đang áp dụng — cột và chiều, luôn đủ cả hai. */
export type SapXepSo = { readonly cot: CotSapXepNhiemVu; readonly chieu: ChieuSapXepNhiemVu };

/**
 * Mặc định của máy chủ khi yêu cầu không mang `sort`/`order`: `created_at`, GIẢM DẦN — và giảm dần
 * cho MỌI cột (`page.NewAllowlist(page.Desc, …)`, `store/nhiem_vu.go:92`).
 *
 * ⚠ BẢN SAO của một mặc định máy chủ, và cái giá được giới hạn thế này: bảng Danh sách LUÔN gửi cách
 * sắp nó đang vẽ (`sapXepDayDu`), nên mũi tên trên đầu cột là đúng câu hỏi đã gửi đi — không phải một
 * phỏng đoán về mặc định. Bản sao này chỉ quyết định màn hình gửi gì khi cán bộ chưa bấm cột nào.
 */
export const SAP_XEP_MAC_DINH: SapXepSo = { cot: "created_at", chieu: "desc" };

/** Bộ lọc → cách sắp đủ hai vế. Thiếu vế nào thì lấy vế ấy của máy chủ, đúng như máy chủ làm. */
export function sapXepDayDu(loc: {
  readonly sapXep?: CotSapXepNhiemVu;
  readonly chieu?: ChieuSapXepNhiemVu;
}): SapXepSo {
  return { cot: loc.sapXep ?? SAP_XEP_MAC_DINH.cot, chieu: loc.chieu ?? SAP_XEP_MAC_DINH.chieu };
}

/**
 * Bấm tiêu đề một cột. Cùng cột ⇒ đảo chiều. Cột khác ⇒ chiều TỰ NHIÊN của cột ấy:
 *   `code`        TĂNG DẦN — NV01, NV02… là thứ tự của chính quyển sổ (§4.3)
 *   `created_at`  GIẢM DẦN — việc mới giao lên đầu, như lúc mở màn
 *   `due_at`      TĂNG DẦN — hạn sớm nhất (kể cả hạn đã qua) lên đầu: câu hỏi "việc nào gấp nhất"
 *   `title`       TĂNG DẦN — A → Z, theo collation của cơ sở dữ liệu
 *   `priority`    TĂNG DẦN — đúng thứ tự xã xếp danh mục mức ưu tiên, mục đầu danh mục lên đầu
 *
 * ⚠ ĐỔI CÁCH SẮP LÀ VỀ TRANG ĐẦU, và đó không phải lựa chọn giao diện: con trỏ mang `sort`/`order`
 * bên trong và máy chủ trả 400 `invalid_cursor` cho con trỏ của một cách sắp khác
 * (`core/page/page.go:456-457`). Hàm này chỉ trả cách sắp mới; màn hình đưa nó qua đúng lối đổi bộ
 * lọc (`datLocMoi`), lối ấy đặt lại ngăn xếp con trỏ.
 */
export function bamCotSapXep(hienTai: SapXepSo, cot: CotSapXepNhiemVu): SapXepSo {
  if (hienTai.cot === cot) return { cot, chieu: hienTai.chieu === "asc" ? "desc" : "asc" };
  return { cot, chieu: cot === "created_at" ? "desc" : "asc" };
}

/** `aria-sort` của một tiêu đề cột — để trình đọc màn hình đọc đúng chiều. */
export function ariaSapXep(
  hienTai: SapXepSo,
  cot: CotSapXepNhiemVu,
): "ascending" | "descending" | "none" {
  if (hienTai.cot !== cot) return "none";
  return hienTai.chieu === "asc" ? "ascending" : "descending";
}

/** Nhãn ba cột sắp được — `Ngày giao` là `created_at`: lúc nhiệm vụ được giao cũng là lúc nó vào sổ. */
export const NHAN_COT_MA = "Mã";
export const NHAN_COT_NGAY_GIAO = "Ngày giao";
export const DUE_COLUMN_LABEL = "Hạn";
export const TITLE_COLUMN_LABEL = "Tên việc";
export const PRIORITY_COLUMN_LABEL = "Ưu tiên";

/**
 * Under the list, always visible. The server puts tasks WITHOUT a deadline last in BOTH directions;
 * unsaid, a clerk who flips `Hạn` to descending expects them first and concludes they are missing.
 */
export const NO_DEADLINE_LAST_NOTE =
  "Sắp theo Hạn: việc không có hạn luôn nằm cuối danh sách, dù sắp tăng hay giảm.";

/**
 * Same reason as `NO_DEADLINE_LAST_NOTE`, for `priority` — and it says WHAT the order is: the
 * commune's own catalogue order, which the clerk can check on the Danh mục screen. Without it,
 * ascending reads as "lowest first" to one clerk and "most urgent first" to another.
 */
export const NO_PRIORITY_LAST_NOTE =
  "Sắp theo Ưu tiên: theo đúng thứ tự xã xếp danh mục mức ưu tiên; việc chưa có mức ưu tiên luôn nằm " +
  "cuối danh sách, dù sắp tăng hay giảm.";

/** Một dòng nhật ký đã dịch sang chữ để vẽ. */
export type DongNhatKyHien = {
  readonly id: string;
  /** Nguyên văn `at` — cho thuộc tính `dateTime` của `<time>`. */
  readonly luc: string;
  /** `14:20 09/09/2026`, giờ Việt Nam. */
  readonly thoiDiem: string;
  readonly nguoi: string;
  /** Nhãn CỦA XÃ cho trạng thái SAU hành vi (một nguồn — `GET /api/v1/task-statuses`). */
  readonly trangThai: string;
  /** `Bộ phận · phụ trách` — CHỈ ở dòng đổi phân công; `null` ở mọi dòng khác. */
  readonly phanCong: string | null;
  readonly ghiChu: string;
};

/**
 * Dòng của máy chủ → chữ để vẽ.
 *
 * `unit`/`assignee` RỖNG Ở DÒNG KHÔNG ĐỔI PHÂN CÔNG (`nhat_ky_nhiem_vu.go:52-57`), nên dòng
 * `phanCong` chỉ hiện khi một trong hai có giá trị — §5.9 "thông tin bộ phận/phụ trách khi có thay
 * đổi phân công". Có một vế thì vế kia hiện câu trạng thái thật, không phải ô trống.
 */
export function hienDongNhatKy(
  d: petitions_nhatKyNhiemVuRa,
  nhanTT: BangNhanTrangThai,
  danhBa: DanhBaTheoMa | null,
  tenBoPhan: ReadonlyMap<string, string>,
): DongNhatKyHien {
  const coPhanCong = d.unit !== "" || d.assignee !== "";
  return {
    id: d.id,
    luc: d.at,
    thoiDiem: nhanThoiDiem(d.at),
    nguoi: nhanNguoiNhatKy(d.actor_code, danhBa),
    trangThai: nhanTrangThai(nhanTT, d.status),
    phanCong: coPhanCong
      ? `${d.unit === "" ? CHUA_GIAO_BO_PHAN : (tenBoPhan.get(d.unit) ?? d.unit)} · ${
          d.assignee === "" ? CHUA_PHAN_CONG : nhanNguoiNhatKy(d.assignee, danhBa)
        }`
      : null,
    ghiChu: d.note,
  };
}

/**
 * Nối trang sau vào những dòng đã có, BỎ dòng trùng `id`.
 *
 * Trùng xảy ra thật: giữa hai lần bấm `Xem thêm`, một thao tác mới ghi thêm dòng ở ĐẦU nhật ký,
 * và nếu kho phân trang theo vị trí thì dòng cuối trang trước trôi sang đầu trang sau. Hai dòng
 * cùng `id` còn là hai phần tử cùng `key` trong React — một trong hai sẽ không được vẽ lại đúng.
 * Giữ bản ĐÃ CÓ, theo thứ tự máy chủ trả.
 *
 * Chung kiểu theo `id` vì hàng chờ duyệt lùi hạn (`KhoiHangChoLuiHan`) cần đúng phép gộp này: một
 * đề nghị mới gửi giữa hai lần `Xem thêm` cũng làm trôi dòng như vậy.
 */
export function gopTrangNhatKy<T extends { readonly id: string }>(
  daCo: readonly T[],
  trangMoi: readonly T[],
): T[] {
  const daThay = new Set(daCo.map((d) => d.id));
  const ra = [...daCo];
  for (const d of trangMoi) {
    if (daThay.has(d.id)) continue;
    daThay.add(d.id);
    ra.push(d);
  }
  return ra;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * HÀNG CHỜ DUYỆT LÙI HẠN §5.8 — `GET /api/v1/task-extensions`
 *
 * HAI CHỖ QUYẾT ĐỊNH, MỘT CỔNG VÀ MỘT TUYẾN: mục `Đề nghị lùi hạn chờ duyệt` trên sổ, và — từ khi
 * tuyến nhận `task=NV19` (ad7f821) — khối đề nghị đang chờ trong drawer của chính nhiệm vụ ấy
 * (`task-extension-block.tsx`). Cả hai vẽ dòng qua `hienDongHangCho` và gọi `quyetDinhLuiHan` với
 * cùng ghi chú tuỳ chọn đã cắt khoảng trắng, nên không chỗ nào mời duyệt khi chỗ kia không mời.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Hai bộ lọc của hàng chờ. `cua-toi` là MẶC ĐỊNH — quyết định của người dùng 27/09/2026: web mở
 * hàng chờ ở `Chờ tôi duyệt`, có lối chuyển sang toàn xã.
 */
export type LocHangCho = "cua-toi" | "toan-xa";

export const LOC_HANG_CHO_MAC_DINH: LocHangCho = "cua-toi";

/**
 * Bộ lọc màn hình → tham số tuyến. `cua-toi` là `approver=me` — chuỗi `me`, KHÔNG phải mã cán bộ:
 * máy chủ tự lấy mã từ phiên. `toan-xa` là tham số VẮNG MẶT, không phải `approver=` rỗng.
 */
export function thamSoHangCho(loc: LocHangCho): { approver?: "me" } {
  return loc === "cua-toi" ? { approver: "me" } : {};
}

/** Tiêu đề mục — cũng là đích của liên kết trong drawer. */
export const TIEU_DE_HANG_CHO = "Đề nghị lùi hạn chờ duyệt";

/** `id` của mục, cho liên kết `#…` từ drawer. */
export const ID_HANG_CHO = "hang-cho-lui-han";

/** Nhãn cột `Chờ tôi duyệt` của Sổ tay lãnh đạo (`03-so-tay-lanh-dao.md`), nguyên văn. */
export const NHAN_LOC_CHO_TOI = "Chờ tôi duyệt";
export const NHAN_LOC_TOAN_XA = "Toàn xã";

export const DANG_TAI_HANG_CHO = "Đang tải đề nghị lùi hạn…";

export const NHAN_XEM_THEM_HANG_CHO = "Xem thêm";

/** Hàng chờ rỗng — HAI câu, vì "không có gì chờ bạn" và "xã không có gì chờ" là hai sự thật khác. */
export function cauHangChoRong(loc: LocHangCho): string {
  return loc === "cua-toi"
    ? "Không có đề nghị lùi hạn nào chờ bạn duyệt."
    : "Xã không có đề nghị lùi hạn nào đang chờ duyệt.";
}

/**
 * Dòng có `task_assigner` rỗng. Cùng một sự thật với `CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC` của drawer,
 * nói ngắn hơn vì đứng trên một dòng hàng chờ. Cả hai đều KHÔNG bảo "hãy bổ sung" — `PATCH` cố ý
 * không nhận `assigner` (ADR 0038), nên từ màn này không có đường nào bổ sung được.
 */
export const CAU_KHONG_AI_DUYET_DUOC =
  "Nhiệm vụ này không ghi lãnh đạo giao việc, nên không ai duyệt được đề nghị này.";

/* ── Drawer block: this task's pending extension requests (#11) ─────────────────────────── */

export const TASK_EXTENSIONS_TITLE = "Đề nghị lùi hạn đang chờ duyệt";
export const TASK_EXTENSIONS_LOADING = "Đang tải đề nghị lùi hạn của nhiệm vụ này…";
export const TASK_EXTENSIONS_EMPTY = "Nhiệm vụ này không có đề nghị lùi hạn nào đang chờ duyệt.";
/** Same label and optional-note rule as the queue on the register (`hang-cho-lui-han.tsx`). */
export const DECISION_NOTE_LABEL = "Ghi chú quyết định (không bắt buộc)";

/**
 * The one sentence the drawer block says when this account may not decide — or `null`.
 *
 * TASK-LEVEL, NOT PER ROW: every row of the block belongs to the same task, so `task_assigner` is
 * the same on all of them and the queue's per-row sentence would repeat itself. Missing
 * `task.extend` is only worth saying when there IS a request to decide (`coDeNghi`): the other two
 * reasons are facts about the task, true with or without one.
 */
export function extensionBlockNote(
  cong: QuyetDinhDuyetLuiHan,
  coDeNghi: boolean,
): string | null {
  if (cong.hien) return null;
  if (cong.vi === "thieu-quyen") return coDeNghi ? CAU_THIEU_QUYEN_DUYET_GIA_HAN : null;
  return cong.thongBao;
}

/** Một dòng hàng chờ đã dịch sang chữ để vẽ. */
export type DongHangChoHien = {
  readonly id: string;
  readonly maNhiemVu: string;
  readonly tieuDe: string;
  /** Hạn đang có — `20/6/2026`, hoặc `—` khi nhiệm vụ không có hạn. */
  readonly hanHienTai: string;
  readonly hanDeNghi: string;
  readonly lyDo: string;
  /** `Họ tên (CB-…)` khi danh bạ có, còn lại là mã. */
  readonly nguoiDeNghi: string;
  /** Nguyên văn `requested_at` — cho `dateTime` của `<time>`. */
  readonly lucDeNghiISO: string;
  readonly lucDeNghi: string;
  readonly lanhDao: string;
  /** `null` = hiện hai nút. Còn lại là câu nói vì sao không có nút. */
  readonly cauChan: string | null;
};

/**
 * Dòng của máy chủ → chữ để vẽ.
 *
 * `cauChan` ĐI QUA `quyetDinhDuyetLuiHan` — đúng phép so mã cán bộ drawer dùng (ADR 0038 lớp hai),
 * không phải một phép so thứ hai. Đó là tiện dụng, không phải biện pháp: máy chủ kiểm lại trong
 * giao dịch và câu 403/409 của nó ra nguyên văn. Lớp một (`task.extend`) là tham số cuối, đọc từ
 * danh sách quyền của PHIÊN (`quyenNhiemVu`) — phiên chưa đọc được thì `false`, FAIL CLOSED.
 */
export function hienDongHangCho(
  d: petitions_deNghiChoDuyetRa,
  danhBa: DanhBaTheoMa | null,
  maNguoiDangNhap: string,
  coQuyenDuyetGiaHan: boolean,
): DongHangChoHien {
  const cong = quyetDinhDuyetLuiHan(maNguoiDangNhap, d.task_assigner, coQuyenDuyetGiaHan);
  let cauChan: string | null = null;
  if (!cong.hien) {
    cauChan =
      cong.vi === "thieu-quyen"
        ? CAU_THIEU_QUYEN_DUYET_GIA_HAN
        : cong.vi === "chua-ghi-lanh-dao-giao-viec"
          ? CAU_KHONG_AI_DUYET_DUOC
          : cong.thongBao;
  }
  return {
    id: d.id,
    maNhiemVu: d.task_code,
    tieuDe: d.task_title,
    hanHienTai: nhanNgay(d.task_due_at),
    hanDeNghi: nhanNgay(d.new_due_at),
    lyDo: d.reason,
    nguoiDeNghi: nhanNguoiNhatKy(d.requested_by, danhBa),
    lucDeNghiISO: d.requested_at,
    lucDeNghi: nhanThoiDiem(d.requested_at),
    lanhDao: d.task_assigner === "" ? O_TRONG : nhanNguoiNhatKy(d.task_assigner, danhBa),
    cauChan,
  };
}

/** Bỏ đề nghị vừa quyết định khỏi những dòng đang hiện — nó không còn chờ nữa. */
export function boDeNghiDaQuyet<T extends { readonly id: string }>(
  ds: readonly T[],
  id: string,
): T[] {
  return ds.filter((d) => d.id !== id);
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CÂU CHỮ CỦA MÀN HÌNH — §2, §3, §5, §7
 *
 * Đặt ở đây chứ không rải trong `so-nhiem-vu.tsx` vì cùng một lý do `nhan-phieu.ts` làm thế: một
 * chuỗi giao diện nằm trong JSX là một chuỗi không bài kiểm nào so lại được với đặc tả mà không
 * phải kết xuất cả cây.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Quyển sổ rỗng. */
export const SO_RONG = "Chưa có nhiệm vụ nào khớp bộ lọc đang chọn.";

/** Đang đọc trang. `role="status"`, không phải `alert`. */
export const DANG_TAI_SO = "Đang tải sổ nhiệm vụ…";

/** Ô tìm §3 — nguyên văn placeholder đặc tả. */
export const TIM_PLACEHOLDER = "Tìm theo tên nhiệm vụ…";

/** Bảy nhãn "tất cả" của §3, nguyên văn. */
export const MOI_BO_PHAN_NHAN = "Tất cả bộ phận";
export const MOI_NGUOI_THUC_HIEN_NHAN = "Tất cả người thực hiện";
export const MOI_MUC_UU_TIEN_NHAN = "Mọi mức ưu tiên";
export const MOI_LOAI_NHAN = "Mọi loại nhiệm vụ";
export const MOI_KHOI_NHAN = "Mọi khối";
export const MOI_NGUON_GIAO_NHAN = "Mọi nguồn giao";
export const MOI_TRANG_THAI_NHAN = "Mọi trạng thái";
export const CHI_QUA_HAN_NHAN = "Chỉ việc quá hạn";

/** Hai tab phạm vi DỰNG ĐƯỢC. Tab thứ ba (`Liên quan đến tôi`) — xem `PHAN_CHUA_DUNG`. */
export const PHAM_VI_TOAN_XA = "Toàn xã";
export const PHAM_VI_CUA_TOI = "Giao cho tôi";
/** §3 — verbatim. What it covers is said by the tab's description, `SCOPE_RELATED_NOTE`. */
export const SCOPE_RELATED_LABEL = "Liên quan đến tôi";
/**
 * §3's definition, said while the tab is on so nobody reads "related" as "assigned to me":
 * the server's `relatedCondition` covers exactly these (require, 3a4e60f).
 */
export const SCOPE_RELATED_NOTE =
  "Liên quan đến tôi: việc tôi giao, tôi theo dõi, tôi đã xử lý, hoặc do bộ phận tôi đang giữ.";
/** §3 checkbox — verbatim. The threshold is the commune's (`Cấu hình → Thời hạn xử lý`), never shown as a number here. */
export const DUE_SOON_FILTER_LABEL = "Sắp đến hạn";

/** §5.3 — ô `ĐANG GIAO CHO` khi nhiệm vụ chưa về bộ phận nào. Không phải dấu gạch: một trạng thái thật. */
export const CHUA_GIAO_BO_PHAN = "Chưa giao bộ phận nào";

/** Lựa chọn mặc định của hai ô chọn ở §5.7 và §7.1 — nguyên văn đặc tả. */
export const CHUA_XAC_DINH = "— Chưa xác định —";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CÁC Ô CHỌN CÁN BỘ (§3 `Người thực hiện`; §7 `Người thực hiện` · `Lãnh đạo giao việc`) — đổ từ
 * DANH BẠ CHỌN NGƯỜI. `Chuyên viên theo dõi` đã gộp vào `Người thực hiện` (ADR 0065 NV5)
 *
 * Nguồn là `GET /api/v1/staff-directory` (`AnyAuthenticated`, chỉ mã · họ tên · chức vụ · bộ phận),
 * KHÔNG phải `GET /api/v1/staff` (đứng sau `admin.user`). Giá trị gửi đi vẫn là MÃ NGHIỆP VỤ
 * `CB-…` (`code`) — đúng loại hai trường `assignee` · `assigner` của hợp đồng giữ
 * (luật 6, bất biến 8). Một ULID ở đó được máy chủ nhận nhưng không khớp cán bộ nào, lặng lẽ. Ô chọn dùng chung: `components/o-chon-can-bo.tsx`.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** §7 — lựa chọn rỗng của ô `Người thực hiện`, nguyên văn đặc tả. */
export const DE_BO_PHAN_TU_PHAN_CONG = "— Để bộ phận tự phân công —";

/** Chữ của lựa chọn rỗng trong lúc danh bạ còn đang tải — ô khoá, và nói vì sao. */
export const DANG_TAI_DANH_BA = "Đang tải danh bạ cán bộ…";

/**
 * Danh bạ đọc ra cho ô chọn — BA pha, không hai.
 *
 * "Đang tải" và "tải hỏng" đều cho ra một mảng rỗng, nhưng nói hai câu khác hẳn: một ô rỗng không
 * lời giải thích đọc lên là "xã không có cán bộ nào", và cán bộ sẽ giao việc cho bộ phận vì tưởng
 * không còn cách nào khác.
 */
export type DanhBaChonNguoi = {
  readonly ds: readonly identity_canBoChonNguoiRa[];
  readonly dangTai: boolean;
  /** Câu máy chủ, NGUYÊN VĂN, khi đọc hỏng. */
  readonly loi: string | null;
};

export function docDanhBaChonNguoi(
  kq: KetQua<identity_danhBaChonNguoiRa> | null,
): DanhBaChonNguoi {
  if (kq === null) return { ds: [], dangTai: true, loi: null };
  if (!kq.ok) return { ds: [], dangTai: false, loi: kq.thongBao };
  return { ds: kq.duLieu.items, dangTai: false, loi: null };
}

/** Chữ của lựa chọn rỗng: đang tải thì nói đang tải, còn lại là ý nghĩa của "không chọn ai". */
export function nhanTrongOChonCanBo(db: DanhBaChonNguoi, macDinh: string): string {
  return db.dangTai ? DANG_TAI_DANH_BA : macDinh;
}

/** Câu dưới ô lọc `Người thực hiện` khi danh bạ đọc hỏng. */
export function cauLoiDanhBaLoc(thongBao: string): string {
  return (
    `Không tải được danh bạ cán bộ: ${thongBao} Ô lọc Người thực hiện tạm thời chỉ còn ` +
    `"${MOI_NGUOI_THUC_HIEN_NHAN}".`
  );
}

/**
 * Câu trong form `Giao việc mới` khi danh bạ đọc hỏng. NÓI RA HỆ QUẢ, không chỉ nói lỗi: bấm
 * `Giao việc` lúc này là tạo một nhiệm vụ không ghi cán bộ nào, và `Lãnh đạo giao việc` không sửa
 * lại được sau khi tạo (ADR 0038) — không ai duyệt được đề nghị lùi hạn của nó, vĩnh viễn.
 */
export function cauLoiDanhBaGiaoViec(thongBao: string): string {
  return (
    `Không tải được danh bạ cán bộ: ${thongBao} Các ô chọn cán bộ chỉ còn lựa chọn trống. Giao ` +
    "việc lúc này thì nhiệm vụ không ghi người thực hiện, lãnh đạo giao việc hay chuyên viên theo " +
    "dõi — và lãnh đạo giao việc không ghi lại được sau khi tạo. Đóng biểu mẫu và mở lại trang " +
    "để thử đọc lại danh bạ."
  );
}

/* ── Ô `Lãnh đạo giao việc` — CHỈ NGƯỜI CẦM `task.extend` ────────────────────────────────────
 *
 * Người ghi ở ô này là người DUYỆT đề nghị lùi hạn (ADR 0038), và cột ấy không sửa được sau khi tạo.
 * Nên ô đọc danh bạ RIÊNG, lọc `permission=task.extend` (`layDanhBaChonNguoi`): gợi một người không
 * cầm khoá ấy là tạo ra một nhiệm vụ không ai duyệt lùi hạn được, vĩnh viễn. Hai ô cán bộ còn lại đọc
 * danh bạ cả xã như cũ.
 *
 * ĐÂY LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP: máy chủ kiểm lại lúc tạo rằng người được chọn là cán bộ
 * đang hoạt động của xã (400 nguyên văn, commit 5af3d51), và kiểm `task.extend` lúc duyệt.
 */

/**
 * Danh bạ đã lọc đọc được mà RỖNG: không ai trong xã đang cầm quyền duyệt gia hạn. Một câu thay cho
 * một ô chọn rỗng — ô rỗng không lời đọc lên là "màn hình hỏng" — và nói hệ quả của việc giao lúc này.
 */
export const CAU_KHONG_AI_CO_QUYEN_DUYET_GIA_HAN =
  "Hiện trong xã chưa có cán bộ nào được cấp quyền duyệt gia hạn, nên chưa có ai để chọn làm lãnh " +
  "đạo giao việc. Giao việc lúc này thì nhiệm vụ không ghi lãnh đạo giao việc và không ai duyệt được " +
  "đề nghị lùi hạn của nó. Người quản trị cấp quyền này ở màn Phân quyền.";

/** Câu khi danh bạ lãnh đạo (đã lọc) đọc hỏng. Câu máy chủ đứng nguyên văn ở giữa. */
export function cauLoiDanhBaLanhDao(thongBao: string): string {
  return (
    `Không tải được danh sách người có quyền duyệt gia hạn: ${thongBao} Ô Lãnh đạo giao việc chỉ ` +
    "còn lựa chọn trống, và lãnh đạo giao việc không ghi lại được sau khi tạo. Đóng biểu mẫu và mở " +
    "lại trang để thử đọc lại."
  );
}

/** §5.8 — chú thích bắt buộc dưới ô đề nghị lùi hạn. */
export const GHI_CHU_LUI_HAN =
  "Hạn gốc vẫn được giữ lại để báo cáo đúng hạn không bị lùi theo.";

/** §7.1 — chú thích dưới ô `Lãnh đạo giao việc`. */
export const GHI_CHU_LANH_DAO_GIAO_VIEC =
  "Đề nghị lùi hạn sẽ gửi tới người này, qua chuông và qua thư.";

/** §7.1 — chú thích dưới ô `Tự sinh mã`. */
export const GHI_CHU_TU_SINH_MA =
  "Tự sinh sẽ cấp số tiếp theo trong dãy NV01, NV02… Nhập từ Excel cũng được đánh số tự động " +
  "theo dãy này.";

/** §5.4 — chú thích BẮT BUỘC hiển thị dưới hai ô tick phê duyệt. */
export const CHU_THICH_HAI_O_TICK =
  "Hai ô này đánh dấu bằng tay và không làm đổi trạng thái nhiệm vụ.";

/** §7 — mô tả dưới tiêu đề form `Giao việc mới`, nguyên văn. */
export const MO_TA_FORM_GIAO_VIEC =
  "Giao cho một bộ phận hoặc trực tiếp cho cán bộ. Giao cho bộ phận mà quá lâu chưa phân công " +
  "người thì hệ thống báo lên lãnh đạo.";

/**
 * Under the create form's deadline fields. Replaces `CANH_BAO_HAN_MOT_LAN` ("set once, never
 * later"), which stopped being true with ADR 0065 NV4: `PATCH /api/v1/tasks/{code}` sets or corrects
 * `due_at`, including on a task created without one (`service-petitions/internal/app/nhiem_vu.go`).
 * Keeping the old sentence pushed clerks to invent a deadline rather than leave it empty.
 */
export const NEW_TASK_DUE_LATER_NOTE =
  "Bỏ trống thì nhiệm vụ chưa có hạn. Người có quyền cập nhật nhiệm vụ đặt hoặc sửa hạn về sau " +
  "trong phần chi tiết nhiệm vụ.";

/** Drawer button that opens the deadline editor of a task whose type has no document block. */
export const DUE_EDIT_BUTTON = "Sửa hạn xử lý";
/** Same button on a task created without a deadline. */
export const DUE_SET_BUTTON = "Đặt hạn xử lý";

/** The extension block of a task without a deadline: nothing to extend, and where to set one. */
export const EXTENSION_NO_DUE =
  "Nhiệm vụ này chưa có hạn, nên chưa có gì để lùi. Người có quyền cập nhật nhiệm vụ đặt hạn ngay " +
  "trong phần chi tiết này.";

/**
 * Reason `Giao việc` is disabled while no task type is chosen (report 05/10/2026, NV-01). Without
 * it the button simply greyed out, and the select showed its first option while the value was `""`.
 */
export const TASK_TYPE_MISSING = "Chọn loại nhiệm vụ để giao việc.";
/** Empty first option of the type select while nothing is chosen. */
export const TASK_TYPE_PLACEHOLDER = "— Chọn loại —";

/**
 * The drawer's progress figure (ADR 0068 §14): no screen edits `progress` yet, so the field is
 * drawn disabled with a "?" instead of a bare `0%` that reads as measured.
 */
export const TASK_PROGRESS_PENDING = {
  ten: "Cập nhật tiến độ (%)",
  viSao:
    "Chưa có ô nhập phần trăm tiến độ trên màn hình này. Con số đang hiện là giá trị đang lưu của " +
    "nhiệm vụ; muốn theo dõi tiến độ, ghi vào nhật ký nhiệm vụ.",
} as const;

/** §5.10 — việc con có hạn RIÊNG (ADR 0037 quyết định 2), không thừa kế hạn cha. */
export const GHI_CHU_HAN_VIEC_CON =
  "Việc con có hạn riêng, không lấy theo hạn của việc cha.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * Ô NGÀY ↔ MỐC CỦA HỢP ĐỒNG
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * `<input type="date">` phát ra `YYYY-MM-DD`; hợp đồng đòi một **mốc `date-time`** (`new_due_at`
 * là `time.Time` ở máy chủ, nên `"2026-12-20"` trần là **400**).
 *
 * CHỈ CÒN Ô `Hạn mới` CỦA ĐỀ NGHỊ LÙI HẠN (§5.6) dùng hàm này — ô ấy chỉ có NGÀY, nên phải chọn
 * một mốc trong ngày: 00:00 làm việc "hạn 20/6" quá hạn suốt ngày 20/6, còn 23:59 đúng nghĩa
 * thông thường. Form TẠO nhiệm vụ không còn qua đây: nó có ô giờ, điền sẵn 17:00 theo ADR 0065 NV6
 * (`defaultNewTaskDue`), và ghép bằng `dueAtFromInputs`.
 *
 * MÚI GIỜ GHIM `+07:00`, không lấy múi giờ máy: một cán bộ mở màn hình trên máy đặt sai múi giờ
 * sẽ gửi lên một hạn lệch một ngày, và đó là một cam kết bị đọc sai.
 */
export function mocCuoiNgay(ngay: string): string {
  return `${ngay}T23:59:59+07:00`;
}

/**
 * Mốc của hợp đồng → `YYYY-MM-DD` để đổ vào `<input type="date">`.
 *
 * Cắt theo MÚI GIỜ VIỆT NAM chứ không `mocISO.slice(0, 10)`: một hạn `2026-06-20T23:59:59+07:00`
 * lưu ở dạng UTC là `2026-06-20T16:59:59Z`, cắt mười ký tự đầu vẫn ra `2026-06-20` — nhưng
 * `2026-06-20T00:30:00+07:00` là `2026-06-19T17:30:00Z`, và phép cắt ấy cho ra ngày **19**.
 */
export function ngayChoONhap(mocISO: string | null): string {
  if (mocISO === null || mocISO === "") return "";
  const t = Date.parse(mocISO);
  if (Number.isNaN(t)) return "";
  const phan = new Intl.DateTimeFormat("en-CA", {
    timeZone: MUI_GIO,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date(t));
  return phan;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * KHỐI "SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO" §5.4 — BA NHÓM VĂN BẢN, CHỈ ĐỌC
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Mã loại nhiệm vụ DUY NHẤT có khối §5.4 (*"chỉ với loại `Theo văn bản`"*).
 *
 * Gõ thẳng mã này vào trình duyệt là HỢP LỆ, khác hẳn nhãn của nó: `theo-van-ban` là mã TẦNG 3 —
 * mã nguồn rẽ nhánh trên đúng chuỗi ấy (`service-petitions/migrations/0003_danh_muc_nhiem_vu.sql:
 * 205-215`). Xã đổi được NHÃN `Theo văn bản`, không đổi được mã.
 */
export const LOAI_THEO_VAN_BAN = "theo-van-ban";

/** Loại nhiệm vụ này có khối §5.4 hay không. */
export function coKhoiVanBanChiDao(loai: string): boolean {
  return loai === LOAI_THEO_VAN_BAN;
}

/** Tiêu đề khối — chữ của §5.4, viết hoa đầu câu như mọi tiêu đề khối khác trong drawer. */
export const TIEU_DE_KHOI_VAN_BAN = "Sổ theo dõi văn bản chỉ đạo";

/**
 * Ba nhóm, DANH SÁCH ĐÓNG: đúng ba giá trị `CHECK nhiem_vu_van_ban_nhom_hop_le` cho phép
 * (`service-petitions/migrations/0009_nhiem_vu_van_ban.sql:266`). Thứ tự mảng là thứ tự §5.4 vẽ.
 */
export type NhomVanBan = "cap-tren-giao" | "chi-dao-dang-uy" | "san-pham-dau-ra";

export const MOI_NHOM_VAN_BAN: readonly NhomVanBan[] = [
  "cap-tren-giao",
  "chi-dao-dang-uy",
  "san-pham-dau-ra",
];

/** Nhãn nguyên văn §5.4. `uỷ` viết đúng như đặc tả. */
const NHAN_NHOM_VAN_BAN: Readonly<Record<NhomVanBan, string>> = {
  "cap-tren-giao": "Văn bản cấp trên giao",
  "chi-dao-dang-uy": "Văn bản chỉ đạo của Đảng uỷ",
  "san-pham-dau-ra": "Văn bản sản phẩm đầu ra",
};

function laNhomVanBan(ma: string): ma is NhomVanBan {
  return (MOI_NHOM_VAN_BAN as readonly string[]).includes(ma);
}

/** Nhãn nhóm. Mã lạ hiện NGUYÊN VĂN — cùng quy tắc `nhanTrangThai`. */
export function nhanNhomVanBan(ma: string): string {
  return laNhomVanBan(ma) ? NHAN_NHOM_VAN_BAN[ma] : ma;
}

/** Văn bản không ghi số, ký hiệu — chữ §4.3 dùng cho đúng tình huống này. */
export const KHONG_SO = "Không số";

/** Đang đọc chi tiết. `role="status"`. */
export const DANG_TAI_VAN_BAN = "Đang tải các văn bản chỉ đạo…";

/**
 * Đứng TRƯỚC câu lỗi của máy chủ khi không đọc được khối.
 *
 * Câu này tồn tại vì một khối trống ở đây đọc ra là "nhiệm vụ không có văn bản nào" — một điều
 * chưa ai ghi. Không đọc được thì phải NÓI là không đọc được.
 */
export const KHONG_DOC_DUOC_VAN_BAN =
  "Không đọc được các văn bản chỉ đạo của nhiệm vụ này — điều đó KHÔNG có nghĩa là nhiệm vụ " +
  "không có văn bản.";

/**
 * Tuyến chi tiết trả về nhiệm vụ mà THIẾU hẳn trường `documents`.
 *
 * Hợp đồng nói tuyến chi tiết LUÔN trả một mảng (có thể rỗng); vắng mặt chỉ đúng trên tuyến sổ.
 * Gặp vắng mặt ở đây là hợp đồng bị vi phạm, và câu trả lời an toàn là báo lỗi — đọc nó thành
 * "không có văn bản" là nói dối đúng điều `documents` được thiết kế để phân biệt.
 */
export const CHI_TIET_THIEU_VAN_BAN = "Máy chủ trả chi tiết nhiệm vụ nhưng không kèm khối văn bản.";

/**
 * Ngày văn bản `YYYY-MM-DD` → `9/6/2026`. Rỗng → rỗng.
 *
 * KHÔNG ĐỆM SỐ 0, cùng khuôn `nhanNgay` và đúng ví dụ §5.4 (`1742-CV/BTCTU · 9/6/2026`).
 *
 * TÁCH CHUỖI, KHÔNG `Date.parse`: đây là một NGÀY, không phải một mốc. `Date.parse("2026-06-09")`
 * đọc thành nửa đêm UTC, và định dạng theo một múi giờ phía tây UTC sẽ ra ngày 8 — một văn bản
 * ghi sai ngày ban hành. Chuỗi sai khuôn hiện NGUYÊN VĂN, cùng lý do `nhanNgay` làm thế.
 */
export function ngayVanBan(ngay: string): string {
  if (ngay === "") return "";
  const k = /^(\d{4})-(\d{2})-(\d{2})$/.exec(ngay);
  if (k === null) return ngay;
  return `${Number(k[3])}/${Number(k[2])}/${k[1]}`;
}

/**
 * Dòng đầu của một văn bản: `{số, ký hiệu} · {ngày}`.
 *
 * Số rỗng ⇒ `Không số` (hợp lệ ở máy chủ: văn bản không số có thật). Ngày rỗng ⇒ BỎ HẲN phần
 * ngày, không in `· —`: dấu gạch sau dấu chấm giữa đọc ra như một ngày bị xoá.
 */
export function dongVanBan(soKyHieu: string, ngay: string): string {
  const so = soKyHieu === "" ? KHONG_SO : soKyHieu;
  const n = ngayVanBan(ngay);
  return n === "" ? so : `${so} · ${n}`;
}

export type NhomVanBanDaChia = {
  readonly ma: string;
  readonly nhan: string;
  readonly vanBan: readonly petitions_nhiemVuVanBanRa[];
};

/**
 * Chia các dòng về ba nhóm §5.4 — luôn đủ BA nhóm, kể cả nhóm rỗng (để vẽ `—`).
 *
 * GIỮ NGUYÊN THỨ TỰ MÁY CHỦ GỬI trong mỗi nhóm; KHÔNG sắp lại theo `position`. `position` là số
 * thứ tự đã cấp, có thể có lỗ hổng hợp lệ, và máy chủ đã xếp sẵn (nhóm, rồi vị trí).
 *
 * MỘT MÃ NHÓM LẠ KHÔNG BỊ BỎ RƠI: nó thành một nhóm thứ tư mang nhãn nguyên văn, xếp sau ba nhóm
 * kia. Lọc im lặng thì một văn bản biến khỏi màn hình mà không ai biết nó từng có.
 */
export function chiaNhomVanBan(ds: readonly petitions_nhiemVuVanBanRa[]): readonly NhomVanBanDaChia[] {
  const maLa: string[] = [];
  for (const v of ds) {
    if (!laNhomVanBan(v.group) && !maLa.includes(v.group)) maLa.push(v.group);
  }
  return [...MOI_NHOM_VAN_BAN, ...maLa].map((ma) => ({
    ma,
    nhan: nhanNhomVanBan(ma),
    vanBan: ds.filter((v) => v.group === ma),
  }));
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * FORM `GIAO VIỆC MỚI` §7 — ĐỔI TRƯỜNG THEO LOẠI (§7.2 / §7.3) VÀ BA DANH SÁCH VĂN BẢN ĐỘNG
 *
 * Thân yêu cầu được dựng Ở ĐÂY, trong một hàm thuần, chứ không trong trình xử lý `onSubmit`: môi
 * trường kiểm là Node không DOM (`vitest.config.mts`), nên một quy tắc chỉ sống trong thành phần
 * là quy tắc không bài nào chạy tới — kể cả quy tắc đắt nhất của form này: trường đang ẨN thì
 * KHÔNG được gửi.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Nhãn ô tiêu đề theo loại — §7.2 và §7.3. Cả hai đều bắt buộc. */
export const NHAN_TIEU_DE_THEO_VAN_BAN = "Nội dung nhiệm vụ / Trích yếu văn bản";
export const NHAN_TIEU_DE_CO_BAN = "Tên nhiệm vụ";

export function nhanOTieuDe(loai: string): string {
  return coKhoiVanBanChiDao(loai) ? NHAN_TIEU_DE_THEO_VAN_BAN : NHAN_TIEU_DE_CO_BAN;
}

/** Placeholder từng nhóm — NGUYÊN VĂN §7.2 (`02-nhiem-vu.md:263-265`). */
const PLACEHOLDER_NHOM_VAN_BAN: Readonly<Record<NhomVanBan, string>> = {
  "cap-tren-giao": "Ví dụ: Thông báo số 90-TB/TU ngày 30/01/2026 về ý kiến chỉ đạo…",
  "chi-dao-dang-uy": "Ví dụ: Công văn số 416-CV/ĐU ngày 15/6/2026 về tham mưu báo cáo…",
  "san-pham-dau-ra": "Ví dụ: Báo cáo số 335-BC/ĐU ngày 29/6/2026",
};

export function placeholderNhomVanBan(nhom: NhomVanBan): string {
  return PLACEHOLDER_NHOM_VAN_BAN[nhom];
}

export const NHAN_THEM_VAN_BAN = "+ Thêm văn bản";

/**
 * Tên đọc được của nút `✕`. Mỗi nhóm có nhiều nút `✕` liền nhau; không có số thứ tự và tên nhóm
 * thì trình đọc màn hình đọc mười lần cùng một chữ "xoá", và cán bộ không biết mình vừa gỡ dòng nào.
 */
export function nhanNutGoVanBan(nhom: NhomVanBan, soThuTu: number): string {
  return `Gỡ văn bản thứ ${soThuTu} khỏi nhóm ${nhanNhomVanBan(nhom)}`;
}

/**
 * Ba giới hạn của máy chủ (`service-petitions/internal/domain/nhiem_vu_van_ban.go:123,127,136`).
 * Chép ở đây để ô nhập DỪNG ĐÚNG CHỖ thay vì để cán bộ gõ xong một nghìn chữ rồi nhận 400. Máy chủ
 * vẫn là nơi quyết; hai số này lệch khỏi máy chủ thì câu từ chối của máy chủ vẫn ra nguyên văn.
 */
export const SO_KY_HIEU_VAN_BAN_TOI_DA = 100;
export const TRICH_YEU_VAN_BAN_TOI_DA = 1000;
export const VAN_BAN_MOT_LAN_TOI_DA = 100;

/**
 * Một dòng văn bản ĐANG NHẬP. `khoa` chỉ để React giữ đúng ô khi một dòng giữa bị gỡ — KHÔNG lên
 * dây: dòng mới không có `id` (hợp đồng: `id` rỗng nghĩa là dòng mới).
 */
export type DongVanBanNhap = {
  readonly khoa: string;
  readonly nhom: NhomVanBan;
  readonly trichYeu: string;
  readonly soKyHieu: string;
  /** Giá trị của `<input type="date">`: `YYYY-MM-DD` hoặc rỗng. */
  readonly ngay: string;
};

/** Mọi ô của form, kể cả những ô ĐANG ẨN vì loại nhiệm vụ. */
export type FormGiaoViecNhap = {
  readonly tuSinhMa: boolean;
  readonly ma: string;
  readonly loai: string;
  readonly khoi: string;
  readonly tieuDe: string;
  readonly moTa: string;
  readonly mucUuTien: string;
  readonly boPhan: string;
  readonly nguoiThucHien: string;
  readonly lanhDaoGiaoViec: string;
  /** `YYYY-MM-DD` hoặc rỗng. */
  readonly han: string;
  /** `HH:MM` of the deadline, commune time zone. Read only when `han` is set. */
  readonly dueTime: string;
  readonly vanBan: readonly DongVanBanNhap[];
  /** Ô `Ghi chú` §7.2 — chỉ `Theo văn bản`, chỉ màn Nhiệm vụ (xem `thanGiaoViec`). */
  readonly ghiChu: string;
};

/**
 * Các dòng đang nhập của MỘT nhóm, đúng thứ tự trên màn — `+ Thêm văn bản` nối vào cuối.
 */
export function dongCuaNhom<T extends DongVanBanNhap>(ds: readonly T[], nhom: NhomVanBan): T[] {
  return ds.filter((d) => d.nhom === nhom);
}

/**
 * Câu chặn nút `Giao việc` vì ba danh sách văn bản, hoặc `null` khi không có gì chặn.
 *
 * DÒNG TRỐNG CHẶN CHỨ KHÔNG BỊ BỎ LẶNG LẼ: cán bộ đã bấm `+ Thêm văn bản`, tức đã nói có một văn
 * bản. Lọc bỏ dòng ấy lúc gửi là một văn bản biến mất khỏi bản ghi mà không ai được báo; máy chủ
 * cũng từ chối đúng ca này (`ErrThieuTrichYeuVanBan`).
 */
export function canhBaoVanBan(
  ds: readonly DongVanBanNhap[],
  /** Chủ ngữ của câu giới hạn: form tạo là một lần GIAO VIỆC, form `✎ Sửa` là một lần LƯU. */
  lanGui = "Một lần giao việc",
): string | null {
  if (ds.length > VAN_BAN_MOT_LAN_TOI_DA) {
    return `${lanGui} nhận tối đa ${VAN_BAN_MOT_LAN_TOI_DA} dòng văn bản — hiện có ${ds.length}.`;
  }
  for (const nhom of MOI_NHOM_VAN_BAN) {
    const dong = dongCuaNhom(ds, nhom);
    const i = dong.findIndex((d) => d.trichYeu.trim() === "");
    if (i >= 0) {
      return (
        `Văn bản thứ ${i + 1} của nhóm ${nhanNhomVanBan(nhom)} chưa có trích yếu — nhập trích yếu ` +
        "hoặc bấm ✕ để gỡ dòng ấy."
      );
    }
  }
  return null;
}

/**
 * Dựng thân `POST /api/v1/tasks` (và thân Tách kết luận của màn Biên bản) từ các ô của form.
 *
 * TRƯỜNG ĐANG ẨN KHÔNG ĐƯỢC GỬI — CẮT LÚC GỬI, KHÔNG XOÁ LÚC ĐỔI LOẠI. Cán bộ gõ `Ghi chú`
 * rồi đổi sang `Nhiệm vụ cơ bản`: ô ấy biến khỏi màn, và một giá trị không còn nhìn thấy mà vẫn lên
 * dây là một bản ghi mang thứ người giao việc tin là đã bỏ. Cắt ở ĐÂY — một chỗ, có bài kiểm — thay
 * vì dọn state trong trình xử lý đổi loại, nơi mỗi ô thêm sau này là một lần phải nhớ dọn. Đổi lại
 * về `Theo văn bản` thì chữ đã gõ hiện lại, và lúc ấy nó ĐANG NHÌN THẤY nên gửi là đúng.
 *
 * `coDanhSachVanBan` LÀ QUYẾT ĐỊNH CỦA BÊN GỌI, KHÔNG PHẢI CỦA LOẠI: màn Biên bản dùng lại form này
 * để gửi `…/conclusions/{stt}/task`, và `petitions.tachKetLuanVao` KHÔNG có `documents` lẫn `note`.
 * Ở đó dù loại là `Theo văn bản` cũng không có hai trường ấy.
 *
 * DÒNG VĂN BẢN: `summary` cắt khoảng trắng; `reference` và `date` VẮNG MẶT khi bỏ trống — không gửi
 * `""`. `date` đi NGUYÊN chuỗi `YYYY-MM-DD` của ô ngày, không bao giờ qua `Date`: máy chủ từ chối
 * RFC 3339 có chủ ý (`nhiem_vu_ghi.go:335-337`), vì múi giờ trình duyệt không được quyết văn bản ký
 * ngày nào. Thứ tự: theo nhóm §5.4, trong mỗi nhóm đúng thứ tự trên màn.
 */
export function thanGiaoViec(
  f: FormGiaoViecNhap,
  tuyChon: { readonly coDanhSachVanBan: boolean; readonly maCha?: string },
): petitions_taoNhiemVuVao {
  const theoVanBan = coKhoiVanBanChiDao(f.loai);
  const than: petitions_taoNhiemVuVao = {
    auto_code: f.tuSinhMa,
    type: f.loai,
    title: f.tieuDe.trim(),
  };
  if (!f.tuSinhMa && f.ma.trim() !== "") than.code = f.ma.trim();
  if (f.khoi !== "") than.bloc = f.khoi;
  if (f.moTa.trim() !== "") than.description = f.moTa.trim();
  if (f.mucUuTien !== "") than.priority = f.mucUuTien;
  if (f.boPhan !== "") than.unit = f.boPhan;
  if (f.nguoiThucHien.trim() !== "") than.assignee = f.nguoiThucHien.trim();
  if (f.lanhDaoGiaoViec.trim() !== "") than.assigner = f.lanhDaoGiaoViec.trim();
  // NO `lead_unit` / `monitor`, whatever the type (ADR 0065 NV5, user decision 30/09/2026): "cơ quan
  // chủ trì" IS `unit` and "chuyên viên theo dõi" IS `assignee`. The server answers 400 to either.
  // HẠN: ô ngày + ô giờ → một mốc `+07:00`. Bỏ trống ngày thì trường VẮNG MẶT HẲN, không gửi
  // chuỗi rỗng — `due_at` là con trỏ ở máy chủ và "không có hạn" là một trạng thái thật (§4.1 vẽ
  // nó thành `Hạn —`). Ngày có mà giờ sai khuôn thì form đã khoá nút gửi (`newTaskDueProblem`).
  if (f.han !== "") than.due_at = dueAtFromInputs(f.han, f.dueTime);
  if (tuyChon.maCha !== undefined && tuyChon.maCha !== "") than.parent = tuyChon.maCha;

  // GHI CHÚ: cùng cổng với ba danh sách văn bản, cùng hai lý do. §7.3 bỏ ô này ở loại khác `Theo
  // văn bản` (cắt lúc gửi); và `petitions.tachKetLuanVao` của màn Biên bản KHÔNG
  // có `note` — gửi nó ở đó là chữ cán bộ gõ vào mất lặng lẽ. Cắt khoảng trắng như thân `PATCH`;
  // rỗng thì VẮNG MẶT, không gửi `""`. Độ dài do máy chủ quyết (`GhiChuNhiemVuToiDa`), ô nhập chỉ
  // dừng sớm ở cùng `GHI_CHU_NHIEM_VU_TOI_DA` của form `✎ Sửa`.
  if (theoVanBan && tuyChon.coDanhSachVanBan && f.ghiChu.trim() !== "") {
    than.note = f.ghiChu.trim();
  }

  if (theoVanBan && tuyChon.coDanhSachVanBan && f.vanBan.length > 0) {
    than.documents = MOI_NHOM_VAN_BAN.flatMap((nhom) =>
      dongCuaNhom(f.vanBan, nhom).map((d) => {
        const dong: petitions_vanBanNhiemVuVao = { group: d.nhom, summary: d.trichYeu.trim() };
        if (d.soKyHieu.trim() !== "") dong.reference = d.soKyHieu.trim();
        if (d.ngay !== "") dong.date = d.ngay;
        return dong;
      }),
    );
  }
  return than;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NÚT `✎ SỬA` CỦA KHỐI §5.4 — THÂN `PATCH /api/v1/tasks/{ma}`
 *
 * NHỮNG Ô SỬA ĐƯỢC LÀ GIAO CỦA HAI TẬP: các trường §5.4 vẽ, và các trường
 * `petitions_suaNhiemVuVao` nhận: `due_at` · `title` · `result_summary` · `note` ·
 * `leader_approved` · `superior_acknowledged` · `documents`.
 *
 * `Mã nhiệm vụ` KHÔNG SỬA ĐƯỢC (ADR 0065 NV3, người dùng chốt 30/09/2026): mã đã cấp không đổi —
 * luật 7 bất biến 3; máy chủ trả 400 nếu thân mang `code`. Đường đổi mã 3c3525f đã gỡ.
 * `Hạn xử lý` (f27fd6e — SỬA hạn, không phải lùi hạn) sửa được. Mỗi lần lưu từ form này mang
 * `expected_updated_at` (d2ed15e): ai đó đã ghi nhiệm vụ kể từ lúc mở form thì máy chủ từ chối 409.
 *
 * `description`, `priority`, `bloc`, `progress`, `parent` hợp đồng CÓ nhận nhưng §5.4 KHÔNG vẽ, nên
 * không có ô ở đây: nút `✎ Sửa` nằm ở góc khối §5.4 và sửa đúng khối ấy.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Giới hạn của máy chủ (`service-petitions/internal/domain/nhiem_vu_ghi.go:30,46-47`). */
export const TIEU_DE_NHIEM_VU_TOI_DA = 500;
export const TOM_TAT_KET_QUA_TOI_DA = 5000;
export const GHI_CHU_NHIEM_VU_TOI_DA = 5000;

export const NHAN_NUT_SUA = "✎ Sửa";
export const NHAN_NUT_LUU = "Lưu";
export const NHAN_NUT_HUY = "Huỷ";

/**
 * Under the deadline fields. ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: this replaces `LY_DO_KHONG_SUA_HAN`
 * (the owner's earlier "only through an approved extension"), reversed by the owner's adoption of
 * require 93cff7f. What the server does (`nhiem_vu_ghi.go:220-225`): a CORRECTION, no approver;
 * `original_due_at` follows only while the task was never extended; the timeline says which.
 */
export const DUE_EDIT_NOTE =
  "Đây là SỬA hạn cho đúng, không phải lùi hạn: không cần lãnh đạo duyệt. Nhiệm vụ chưa từng được " +
  "duyệt lùi hạn thì hạn ban đầu đổi theo; đã từng được duyệt lùi hạn thì chỉ hạn xử lý đổi, hạn " +
  "ban đầu giữ nguyên. Nhật ký ghi rõ trường hợp nào đã xảy ra. Muốn xin thêm thời gian thì dùng " +
  "khối Đề nghị lùi hạn.";

/** The time field was left empty for one of these reasons — said under it, never a silent blank. */
export const DUE_TIME_NO_CALENDAR =
  "Chưa điền sẵn giờ: không đọc được lịch làm việc của xã. Nhập giờ của hạn.";
export const DUE_TIME_NO_SHIFT =
  "Chưa điền sẵn giờ: theo lịch làm việc hằng tuần, xã không có ca nào vào thứ này. Nhập giờ của hạn.";
export const DUE_TIME_LOADING = "Đang đọc lịch làm việc của xã để điền sẵn giờ…";
export function dueTimeFilledNote(time: string): string {
  return `Giờ ${time} điền sẵn theo giờ kết thúc ca cuối trong ngày của lịch làm việc của xã. Sửa được.`;
}

/** The commune's weekly calendar as the edit form holds it. */
export type CalendarLoad =
  | { readonly pha: "dangTai" }
  | { readonly pha: "loi" }
  | { readonly pha: "xong"; readonly shifts: readonly Pick<identity_caLamViecRa, "weekday" | "end">[] };

/**
 * The sentence under the time field, or `null`. DERIVED from the fields, not stored: "pre-filled"
 * is said while the time still equals the calendar default for the chosen date AND the deadline was
 * changed in this form — so it disappears the moment the clerk types another hour.
 */
export function dueTimeHint(
  cal: CalendarLoad,
  f: Pick<FormSuaNhiemVu, "dueDate" | "dueTime">,
  goc: Pick<petitions_nhiemVuRa, "due_at">,
): string | null {
  if (f.dueDate === "") return null;
  if (f.dueTime === "") {
    if (cal.pha === "dangTai") return DUE_TIME_LOADING;
    if (cal.pha === "loi") return DUE_TIME_NO_CALENDAR;
    return defaultDueTime(cal.shifts, f.dueDate) === "" ? DUE_TIME_NO_SHIFT : null;
  }
  if (cal.pha !== "xong") return null;
  const changed = f.dueDate !== ngayChoONhap(goc.due_at) || f.dueTime !== timeForInput(goc.due_at);
  return changed && f.dueTime === defaultDueTime(cal.shifts, f.dueDate)
    ? dueTimeFilledNote(f.dueTime)
    : null;
}

/**
 * Added under the server's refusal when a re-read after a failed save shows the task CHANGED since
 * the form opened (409 `task_changed`, d2ed15e). Detected from the data — `updated_at` of the fresh
 * read against the snapshot — not from the error `code`, which this screen never branches on
 * (`lib/api/goi.ts`). The form is NOT refreshed underneath the clerk: saving on top of a version
 * they have not seen is exactly what the lock exists to stop.
 */
export const TASK_CHANGED_NOTE =
  "Nhiệm vụ đã được đọc lại. Chưa lưu gì — bấm Huỷ rồi ✎ Sửa để sửa trên bản mới nhất.";

/** A fresh read that proves the task moved on since `snapshotUpdatedAt`. */
export function changedSince(
  fresh: KetQua<Pick<petitions_nhiemVuRa, "updated_at">>,
  snapshotUpdatedAt: string,
): boolean {
  return fresh.ok && fresh.duLieu.updated_at !== snapshotUpdatedAt;
}

/** `✎ Sửa` khoá khi khối văn bản còn đang tải. */
export const KHOA_SUA_DANG_TAI =
  "Chưa sửa được: các văn bản chỉ đạo còn đang tải. Sửa khi chưa thấy đủ văn bản sẽ gỡ mất những " +
  "dòng chưa hiện ra.";

/** `✎ Sửa` khoá khi không đọc được khối văn bản. */
export const KHOA_SUA_LOI =
  "Chưa sửa được: không đọc được các văn bản chỉ đạo. Sửa khi chưa thấy đủ văn bản sẽ gỡ mất những " +
  "dòng chưa hiện ra — đóng rồi mở lại nhiệm vụ để đọc lại.";

/** `✎ Sửa` khoá khi máy chủ gửi một mã nhóm màn hình không biết. */
export const KHOA_SUA_NHOM_LA =
  "Chưa sửa được: có văn bản thuộc một nhóm màn hình này không biết, và form sửa chỉ vẽ ba nhóm " +
  "§5.4 — lưu lại sẽ gỡ mất văn bản ấy.";

/**
 * Lý do `✎ Sửa` phải KHOÁ, hoặc `null` khi mở được.
 *
 * `documents` TRÊN PATCH LÀ THAY CẢ TẬP: dòng nào không gửi lại là dòng bị gỡ. Nên form sửa CHỈ được
 * bắt đầu từ một khối ĐÃ ĐỌC XONG — mở form khi khối đang tải hay đọc hỏng là mở từ một tập rỗng,
 * và lần `Lưu` đầu tiên sẽ xoá mềm mọi văn bản cán bộ chưa từng thấy. Một mã nhóm lạ cũng khoá, cùng
 * lý do: form không có chỗ vẽ dòng ấy, nên nó sẽ không được gửi lại.
 */
export function lyDoKhoaSua(
  tai:
    | { readonly pha: "dangTai" }
    | { readonly pha: "loi" }
    | { readonly pha: "xong"; readonly duLieu: readonly petitions_nhiemVuVanBanRa[] },
): string | null {
  if (tai.pha === "dangTai") return KHOA_SUA_DANG_TAI;
  if (tai.pha === "loi") return KHOA_SUA_LOI;
  if (tai.duLieu.some((v) => !laNhomVanBan(v.group))) return KHOA_SUA_NHOM_LA;
  return null;
}

/**
 * Một dòng văn bản ở form `✎ Sửa`: đúng dòng của form tạo, cộng `id`.
 *
 * `id` RỖNG NGHĨA LÀ DÒNG MỚI (`+ Thêm văn bản`) — đúng quy ước của hợp đồng. Dòng đã có mang `id`
 * của nó, và PHẢI mang nó lên dây: thiếu `id` thì máy chủ đọc dòng ấy là một dòng mới VÀ gỡ dòng
 * cũ — một văn bản bị xoá mềm rồi chép lại, nhật ký ghi hai hành vi cho một lần không ai đụng tới.
 */
export type DongVanBanSua = DongVanBanNhap & { readonly id: string };

/** Mọi ô sửa được của form `✎ Sửa`. */
export type FormSuaNhiemVu = {
  /** Deadline date `YYYY-MM-DD` and time `HH:MM`, both in the commune's time zone; `""` = empty. */
  readonly dueDate: string;
  readonly dueTime: string;
  readonly tieuDe: string;
  readonly tomTatKetQua: string;
  readonly ghiChu: string;
  readonly lanhDaoPheDuyet: boolean;
  readonly capTrenCongNhan: boolean;
  readonly vanBan: readonly DongVanBanSua[];
};

/**
 * Điền form `✎ Sửa` từ CHI TIẾT vừa đọc. Thứ tự dòng là thứ tự máy chủ gửi.
 *
 * `khoa` của dòng đã có dựng từ `id`, không từ chỉ số: gỡ một dòng giữa thì các dòng sau không đổi
 * khoá, và React không đem chữ của dòng này sang ô của dòng kia.
 */
export function formSuaTuChiTiet(
  n: petitions_nhiemVuRa,
  ds: readonly petitions_nhiemVuVanBanRa[],
): FormSuaNhiemVu {
  return {
    dueDate: ngayChoONhap(n.due_at),
    dueTime: timeForInput(n.due_at),
    tieuDe: n.title,
    tomTatKetQua: n.result_summary,
    ghiChu: n.note,
    lanhDaoPheDuyet: n.leader_approved,
    capTrenCongNhan: n.superior_acknowledged,
    vanBan: ds.flatMap((v) =>
      laNhomVanBan(v.group)
        ? [
            {
              khoa: `id-${v.id}`,
              id: v.id,
              nhom: v.group,
              trichYeu: v.summary,
              soKyHieu: v.reference,
              ngay: v.date,
            },
          ]
        : [],
    ),
  };
}

const KHUON_NGAY = /^\d{4}-\d{2}-\d{2}$/;
const TIME_PATTERN = /^\d{2}:\d{2}$/;

/**
 * Instant → `HH:MM` for `<input type="time">`, in the commune's time zone (same reason as
 * `ngayChoONhap`: slicing the ISO string reads UTC). Seconds are dropped — see `thanSuaNhiemVu` for
 * why an untouched field never re-sends them.
 */
export function timeForInput(iso: string | null): string {
  if (iso === null || iso === "") return "";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "";
  return new Intl.DateTimeFormat("en-GB", {
    timeZone: MUI_GIO,
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).format(new Date(t));
}

/** Date + time fields → the RFC 3339 instant PATCH takes. Same `+07:00` as `mocCuoiNgay`. */
export function dueAtFromInputs(date: string, time: string): string {
  return `${date}T${time}:00+07:00`;
}

/**
 * The default time of a deadline on `date` (`YYYY-MM-DD`): the END OF THE LAST SESSION that weekday
 * in the commune's weekly calendar (`GET /api/v1/working-hours`) — the afternoon shift's end on an
 * ordinary day (user decision 28/09/2026). `""` when that weekday has no session: the field stays
 * empty and required rather than inventing an hour.
 *
 * NEVER A HARD-CODED `17:00`: the calendar is per-commune configuration (rule 1, invariant 10).
 *
 * ⚠ WEEKLY CALENDAR ONLY. Public holidays and swap working days are not read: on a weekend swap day
 * the field stays empty (the clerk types the hour); on a holiday the default is that weekday's usual
 * hour. It is a pre-filled suggestion the clerk sees and can change, never a computed deadline —
 * computing deadlines in working hours stays `identity`'s (ADR 0007).
 */
export function defaultDueTime(
  shifts: readonly Pick<identity_caLamViecRa, "weekday" | "end">[],
  date: string,
): string {
  if (!KHUON_NGAY.test(date)) return "";
  const day = new Date(`${date}T00:00:00Z`).getUTCDay();
  // The calendar's weekday is ISO 8601: 1 = Monday … 7 = Sunday (`domain/lich_lam_viec.go:22`).
  const iso = day === 0 ? 7 : day;
  const ends = shifts
    .filter((c) => c.weekday === iso)
    .map((c) => c.end.slice(0, 5))
    .filter((e) => TIME_PATTERN.test(e))
    .sort();
  return ends.length === 0 ? "" : (ends[ends.length - 1] as string);
}

/** ADR 0065 NV6 (user decision 30/09/2026): a new task's deadline is pre-filled 7 days ahead… */
export const DEFAULT_DUE_DAYS = 7;
/** …at 17:00 — the same hour as tasks generated from đơn thư (decision C9). */
export const DEFAULT_DUE_TIME = "17:00";

/** Under the create form's deadline fields: says the value was pre-filled and can be changed. */
export const NEW_TASK_DUE_PREFILLED_NOTE = `Điền sẵn ${DEFAULT_DUE_DAYS} ngày nữa, lúc ${DEFAULT_DUE_TIME}. Sửa được.`;

/** Blocks `Giao việc` when a date is chosen but the time field is empty or malformed. */
export const NEW_TASK_DUE_TIME_MISSING = "Nhập giờ của hạn hoàn thành.";

/**
 * The pre-filled deadline of the `Giao việc mới` form (and every screen reusing it): today in the
 * commune's time zone + `DEFAULT_DUE_DAYS` CALENDAR days, at `DEFAULT_DUE_TIME`.
 *
 * CALENDAR DAYS, NOT WORKING DAYS, on purpose: the user said "+7 ngày", and counting working time
 * (weekends, `ngay_nghi_le`, `ngay_lam_bu`) belongs to `identity` (ADR 0007) — a browser copy of that
 * arithmetic would drift from the server's. This is a value the clerk sees and can change, never a
 * deadline the software fixes on its own. The fixed 17:00 is the user's decision, not read from the
 * commune's calendar (unlike `defaultDueTime` of the edit form).
 *
 * "Today" is read in `MUI_GIO`, not the machine's zone: at 23:30 on a machine set to UTC the local
 * date is still yesterday's, and the pre-fill would land a day early.
 */
export function defaultNewTaskDue(now: Date = new Date()): { readonly date: string; readonly time: string } {
  const today = ngayChoONhap(now.toISOString());
  const d = new Date(`${today}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + DEFAULT_DUE_DAYS);
  return { date: d.toISOString().slice(0, 10), time: DEFAULT_DUE_TIME };
}

/** Why the create form's deadline cannot be sent, or `null`. No date = no deadline, a real state. */
export function newTaskDueProblem(date: string, time: string): string | null {
  if (date === "") return null;
  return TIME_PATTERN.test(time) ? null : NEW_TASK_DUE_TIME_MISSING;
}

/** Ba danh sách có khác tập đã đọc hay không — so theo `id`, sau khi cắt khoảng trắng. */
function vanBanDaDoi(
  ds: readonly DongVanBanSua[],
  goc: readonly petitions_nhiemVuVanBanRa[],
): boolean {
  if (ds.length !== goc.length) return true;
  return ds.some((d) => {
    if (d.id === "") return true;
    const cu = goc.find((v) => v.id === d.id);
    return (
      cu === undefined ||
      d.trichYeu.trim() !== cu.summary ||
      d.soKyHieu.trim() !== cu.reference ||
      d.ngay !== cu.date
    );
  });
}

/**
 * Dựng thân `PATCH /api/v1/tasks/{ma}` — hoặc `null` khi KHÔNG CÓ GÌ ĐỔI (nút `Lưu` khoá).
 *
 * CHỈ GỬI TRƯỜNG CÁN BỘ ĐÃ ĐỔI, so với CHI TIẾT lúc mở form. Mọi trường là con trỏ ở máy chủ: vắng
 * mặt là "không đụng tới". Gửi lại nguyên giá trị cũ của một trường không ai sửa là ghi đè lên bất
 * cứ thay đổi nào người khác vừa lưu vào trường ấy.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG TRẢI FORM HAY BẢN GHI. Không bao giờ có `code`, `assigner`,
 * `position` — trường thứ nhất là mã đã cấp (ADR 0065 NV3: gửi thì 400), trường thứ hai hợp đồng cố
 * ý không nhận, trường cuối do sổ cấp.
 *
 * `documents` — THAY CẢ TẬP, ba trường hợp:
 *   không đổi   VẮNG MẶT, để máy chủ giữ nguyên khối
 *   gỡ hết      `[]` — phải diễn đạt được; gộp nó vào "vắng mặt" là giữ lại những dòng cán bộ đã gỡ
 *   còn lại     MỌI dòng còn giữ, mỗi dòng cũ KÈM `id`; dòng mới không có `id`; dòng đã gỡ thì
 *               không có mặt — đó là cách `✕` tới được máy chủ
 * `group` của dòng cũ là nhóm ĐÃ LƯU (form không cho đổi nhóm, máy chủ từ chối `ErrDoiNhomVanBan`).
 * `date` đi nguyên `YYYY-MM-DD`, không qua `Date` — xem `thanGiaoViec`.
 */
export function thanSuaNhiemVu(
  f: FormSuaNhiemVu,
  goc: petitions_nhiemVuRa,
  gocVanBan: readonly petitions_nhiemVuVanBanRa[],
): petitions_suaNhiemVuVao | null {
  const than: petitions_suaNhiemVuVao = {};
  // THE DEADLINE IS COMPARED FIELD BY FIELD WITH WHAT THE FORM OPENED WITH, not instant by instant:
  // a stored `23:59:59` shows as `23:59`, and re-composing it would send `23:59:00` — a one-second
  // "correction" nobody made, written to the timeline and, on a never-extended task, to the original
  // deadline §11.3 counts against.
  if (f.dueDate !== ngayChoONhap(goc.due_at) || f.dueTime !== timeForInput(goc.due_at)) {
    if (f.dueDate !== "" && f.dueTime !== "") than.due_at = dueAtFromInputs(f.dueDate, f.dueTime);
  }
  const tieuDe = f.tieuDe.trim();
  if (tieuDe !== goc.title) than.title = tieuDe;
  const tomTat = f.tomTatKetQua.trim();
  if (tomTat !== goc.result_summary) than.result_summary = tomTat;
  const ghiChu = f.ghiChu.trim();
  if (ghiChu !== goc.note) than.note = ghiChu;
  if (f.lanhDaoPheDuyet !== goc.leader_approved) than.leader_approved = f.lanhDaoPheDuyet;
  if (f.capTrenCongNhan !== goc.superior_acknowledged) {
    than.superior_acknowledged = f.capTrenCongNhan;
  }

  if (vanBanDaDoi(f.vanBan, gocVanBan)) {
    than.documents = MOI_NHOM_VAN_BAN.flatMap((nhom) =>
      dongCuaNhom(f.vanBan, nhom).map((d) => {
        const dong: petitions_vanBanNhiemVuVao = { group: d.nhom, summary: d.trichYeu.trim() };
        if (d.id !== "") dong.id = d.id;
        if (d.soKyHieu.trim() !== "") dong.reference = d.soKyHieu.trim();
        if (d.ngay !== "") dong.date = d.ngay;
        return dong;
      }),
    );
  }

  if (Object.keys(than).length === 0) return null;
  // EVERY PATCH FROM THIS FORM CARRIES THE TOKEN of the task as the form opened it (d2ed15e): if
  // anybody wrote the task since, the server refuses with 409 instead of this save overwriting it.
  if (goc.updated_at !== "") than.expected_updated_at = goc.updated_at;
  return than;
}

/**
 * Câu chặn nút `Lưu`, hoặc `null`. Cùng phép kiểm dòng văn bản với form tạo (`canhBaoVanBan`),
 * cộng hai điều chỉ form sửa gặp: tiêu đề bị xoá trắng, và một ngày văn bản sai khuôn đọc từ máy chủ.
 */
export function canhBaoSua(f: FormSuaNhiemVu, goc?: Pick<petitions_nhiemVuRa, "due_at">): string | null {
  if (f.dueDate === "" && f.dueTime === "") {
    // A deadline cannot be cleared (`due_at` null = "leave it"), so emptying both fields would look
    // saved while nothing changed. Refuse and say so.
    if (goc !== undefined && goc.due_at !== null && goc.due_at !== "") {
      return "Hạn xử lý không xoá được — chỉ sửa sang ngày giờ khác.";
    }
  } else if (f.dueDate === "" || !KHUON_NGAY.test(f.dueDate)) {
    return "Chọn ngày của hạn xử lý.";
  } else if (f.dueTime === "" || !TIME_PATTERN.test(f.dueTime)) {
    return "Nhập giờ của hạn xử lý.";
  }
  if (f.tieuDe.trim() === "") {
    return `Ô ${NHAN_TIEU_DE_THEO_VAN_BAN} không được để trống.`;
  }
  const i = f.vanBan.findIndex((d) => d.ngay !== "" && !KHUON_NGAY.test(d.ngay));
  if (i >= 0) {
    const d = f.vanBan[i] as DongVanBanSua;
    const thu = dongCuaNhom(f.vanBan, d.nhom).indexOf(d) + 1;
    return (
      `Ngày của văn bản thứ ${thu} nhóm ${nhanNhomVanBan(d.nhom)} không đúng khuôn ngày — chọn lại ` +
      "ngày hoặc xoá trống ô ấy."
    );
  }
  return canhBaoVanBan(f.vanBan, "Một lần lưu");
}

/* ── ĐỌC LẠI TRƯỚC KHI LƯU: chặn việc gỡ lặng lẽ văn bản người khác vừa thêm ──────────────────
 *
 * `documents` là THAY CẢ TẬP. Form gửi lại đúng những dòng cán bộ đã THẤY lúc mở; một dòng người khác
 * thêm trong lúc ấy không có trong thân, nên máy chủ xoá mềm nó — không ai được báo. Vì thế, NGAY
 * TRƯỚC khi gửi một thân CÓ `documents`, màn hình đọc lại chi tiết và so với bản chụp lúc mở form;
 * khác thì KHÔNG gửi, giữ nguyên chữ cán bộ đã gõ, và nói ra. Không tự gộp: gộp hai lần sửa của hai
 * người trên một hồ sơ hành chính là một quyết định của người, không phải của trình duyệt.
 *
 * ⚠ ĐIỀU NÀY THU HẸP KHE HỞ, KHÔNG ĐÓNG NÓ. Giữa lần đọc lại và lần PATCH vẫn còn một khoảng — ngắn,
 * nhưng có thật — trong đó một lần lưu khác lọt vào được. Hợp đồng không có phiên bản hay `etag`
 * (`petitions_nhiemVuRa`), nên trình duyệt không có gì để máy chủ đối chiếu. Đóng hẳn khe hở là việc
 * của MÁY CHỦ: một trường phiên bản trên phản hồi và trên thân PATCH, từ chối 409 khi lệch.
 *
 * Thân KHÔNG có `documents` bỏ qua bước này: trường vô hướng không xoá mềm thứ gì.
 */

/** Câu hiện khi khối văn bản đã đổi ở máy chủ kể từ lúc mở form. */
export const VAN_BAN_VUA_BI_DOI =
  "Văn bản chỉ đạo của nhiệm vụ này vừa được người khác thay đổi. Chưa lưu gì. Bấm Huỷ rồi mở lại " +
  "nhiệm vụ để xem bản mới trước khi sửa.";

/** Thân này có phải đọc lại chi tiết trước khi gửi không — đúng khi và chỉ khi nó mang `documents`. */
export function canDocLaiTruocKhiLuu(than: petitions_suaNhiemVuVao): boolean {
  return than.documents !== undefined && than.documents !== null;
}

/**
 * Khối văn bản ở máy chủ có khác bản chụp lúc mở form không. So năm trường của từng dòng: `id`,
 * `group`, `reference`, `date`, `summary`.
 *
 * SO THEO TẬP, KHÔNG THEO THỨ TỰ: chỉ khác thứ tự thì coi là GIỐNG. Thứ tự dòng do sổ quyết
 * (nhóm, rồi `position` — số đã cấp không đổi), nên một thứ tự khác không có nghĩa là có dòng nào
 * được thêm, gỡ hay sửa; và thân PATCH không mang thứ tự, nên nó không đổi được gì ở đây. Đọc thứ tự
 * lệch thành "người khác vừa sửa" là chặn cán bộ vì một điều không xảy ra.
 */
export function vanBanDaDoiOMayChu(
  bienChup: readonly petitions_nhiemVuVanBanRa[],
  hienTai: readonly petitions_nhiemVuVanBanRa[],
): boolean {
  if (bienChup.length !== hienTai.length) return true;
  return bienChup.some((cu) => {
    const moi = hienTai.find((v) => v.id === cu.id);
    return (
      moi === undefined ||
      moi.group !== cu.group ||
      moi.reference !== cu.reference ||
      moi.date !== cu.date ||
      moi.summary !== cu.summary
    );
  });
}

/**
 * Câu chặn lần lưu sau khi ĐỌC LẠI chi tiết, hoặc `null` khi được gửi PATCH.
 *
 * Tách khỏi `FormSuaKhoiVanBan` để ba nhánh từ chối kiểm được không cần DOM — mỗi nhánh là một lần
 * PATCH thay cả tập từ một tập máy chủ vừa KHÔNG xác nhận được:
 *   đọc hỏng           câu máy chủ nguyên văn
 *   thiếu `documents`   hợp đồng vỡ — đọc thành `[]` thì mọi dòng người khác vừa thêm đều "không đổi"
 *                       so với… không gì cả, và lần lưu gỡ chúng
 *   tập đã khác         `VAN_BAN_VUA_BI_DOI`
 */
export function loiSauKhiDocLai(
  moi: KetQua<petitions_nhiemVuRa>,
  bienChup: readonly petitions_nhiemVuVanBanRa[],
): string | null {
  if (!moi.ok) return moi.thongBao;
  const docs = moi.duLieu.documents;
  if (!Array.isArray(docs)) return CHI_TIET_THIEU_VAN_BAN;
  if (vanBanDaDoiOMayChu(bienChup, docs)) return VAN_BAN_VUA_BI_DOI;
  return null;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BẢNG KANBAN §4.1 — CÂU CHỮ VÀ HAI PHÉP QUYẾT ĐỊNH
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Chế độ xem Kanban và Danh sách; chế độ thứ ba (`Sổ theo dõi` §4.3) là `REGISTER_VIEW_LABEL`.
 * The `▦`/`☰` glyphs of the spec are drawn as lucide icons beside the words (ADR 0068).
 */
export const NHAN_CHE_DO_KANBAN = "Kanban";
export const NHAN_CHE_DO_DANH_SACH = "Danh sách";

/** Đặt cạnh cụm chọn chế độ xem, để cán bộ không đi tìm cái nút thứ ba của đặc tả. */

/**
 * Cột Kanban nào còn phải đọc, sau khi tính bộ lọc `Trạng thái` của §3.
 *
 * ĐÂY LÀ PHÉP AND CỦA MÁY CHỦ ĐƯỢC NÓI RA Ở MÀN HÌNH, không phải một sự tối ưu. Kanban đọc mỗi
 * cột bằng một lời gọi mang `status=<mã cột>`; nếu cán bộ đang lọc `status=cho-duyet` thì bốn cột
 * kia CHẮC CHẮN rỗng — gửi bốn lời gọi nữa chỉ để nhận về bốn trang rỗng là bốn lời gọi thừa, và
 * một cột rỗng vì bộ lọc trông y hệt một cột rỗng vì xã không có việc nào.
 *
 * MỘT TRẠNG THÁI RẼ NHÁNH HOẶC MỘT MÃ LẠ CHO RA MẢNG RỖNG, và đó là câu trả lời đúng: §4.1 cố ý
 * không cho `tam-dung` và `chuyen-tiep` một cột nào. Màn hình phải NÓI RA điều ấy
 * (`CAU_LOC_TRANG_THAI_KHONG_CO_COT`) chứ không vẽ năm cột rỗng — năm cột rỗng đọc lên là "xã
 * không có việc nào", đúng điều ngược lại với sự thật.
 */
export function cotPhaiDoc(trangThaiDangLoc: string | undefined): readonly TrangThaiNhiemVu[] {
  if (trangThaiDangLoc === undefined || trangThaiDangLoc === "") return TRANG_THAI_CHINH;
  return TRANG_THAI_CHINH.filter((ma) => ma === trangThaiDangLoc);
}

/** Hiện khi bộ lọc Trạng thái chọn một trạng thái mà Kanban không có cột. */
export const CAU_LOC_TRANG_THAI_KHONG_CO_COT =
  "Bộ lọc Trạng thái đang chọn một trạng thái không có cột trên Kanban. Chuyển sang chế độ Danh " +
  "sách để xem những nhiệm vụ ấy.";

/**
 * Câu đứng dưới bảng, và nó là chỗ một cán bộ tìm ra việc của mình khi việc ấy "biến mất".
 *
 * Một việc vừa chuyển sang `tam-dung` rời khỏi Kanban HOÀN TOÀN — không phải lỗi, mà là §4.1. Không
 * nói ra thì người giao việc kết luận nhiệm vụ đã bị xoá.
 *
 * HAI TÊN TRẠNG THÁI TRONG CÂU LẤY TỪ BẢNG NHÃN CỦA XÃ: xã đổi "Tạm dừng" thành "Tạm hoãn" mà câu
 * này vẫn nói "Tạm dừng" thì cán bộ đi tìm một trạng thái không có trên màn nào.
 */
export function ghiChuKanbanReNhanh(bang: BangNhanTrangThai): string {
  return (
    `Hai trạng thái rẽ nhánh — ${nhanTrangThai(bang, "tam-dung")} và ` +
    `${nhanTrangThai(bang, "chuyen-tiep")} — không có cột riêng trên Kanban, nên việc đang ` +
    "ở hai trạng thái ấy không hiện ở bảng này. Xem chúng ở chế độ Danh sách."
  );
}

/**
 * The number on a Kanban column header — the REAL total from `GET /api/v1/task-counts` (#15),
 * counted under the board's own filters (`appendTaskFilters`). `null` when the answer has no row
 * for that status: the server promises all seven, so a missing one is a broken contract, and the
 * header shows `—` rather than a `0` that reads as "no work here".
 */
export function kanbanColumnCount(
  counts: readonly petitions_taskStatusCountOut[],
  status: string,
): number | null {
  return counts.find((c) => c.status === status)?.count ?? null;
}

/**
 * Said under a column that shows fewer cards than it counts. The board reads 20 cards per column
 * and has no paging; without this line `57` above twenty cards reads as a board that lost 37.
 * `total === null` (counts unreadable) still says the column goes on, from `has_more`.
 */
export function kanbanPartialNote(shown: number, total: number | null): string {
  return total === null
    ? `Đang hiện ${shown} việc đầu của cột — xem đủ ở chế độ Danh sách.`
    : `Đang hiện ${shown} trong ${total} việc của cột — xem đủ ở chế độ Danh sách.`;
}

/**
 * The ONE sentence of a board whose every column was refused for the same reason — typically 409
 * `due_soon_not_configured` under `☐ Sắp đến hạn`, or a 503 from identity. Five copies of one
 * sentence (plus a sixth in the counts line) read as five problems; one reads as the one it is.
 * `null` when any column loaded, or when columns failed differently — then each says its own.
 */
export function kanbanSharedError(
  cot: readonly { readonly tai: { readonly pha: string; readonly thongBao?: string } }[],
): string | null {
  if (cot.length < 2) return null;
  const first = cot[0]?.tai;
  if (first === undefined || first.pha !== "loi" || first.thongBao === undefined) return null;
  return cot.every((c) => c.tai.pha === "loi" && c.tai.thongBao === first.thongBao)
    ? first.thongBao
    : null;
}

/** Prefix of the server's sentence when the counts cannot be read. The cards still show. */
export const KANBAN_COUNTS_ERROR = "Không đọc được số nhiệm vụ của từng cột:";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * VIỆC CHA — VIỆC CON §4.1, §5.10 (ADR 0037) — `parent` là MÃ SỔ, `child_count` do máy chủ đếm
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * The `{n} việc con` chip on a card and a list row, or `null` for a task with none.
 *
 * `child_count` IS COUNTED BY THE SERVER over the direct live children — never counted from the
 * page on screen, which would give `2` on this page and `3` on the next for the same task.
 */
export function childCountLabel(childCount: number): string | null {
  return childCount > 0 ? `${childCount} việc con` : null;
}

export const CHILD_TASKS_TITLE = "Nhiệm vụ con";
export const CHILD_TASKS_LOADING = "Đang tải các việc con…";
export const CHILD_TASKS_EMPTY = "Nhiệm vụ này chưa có việc con nào.";
export const ADD_CHILD_BUTTON = "+ Thêm việc con";

/**
 * Shown above the create form when it was opened from a drawer. Plain words only (report
 * 05/10/2026, NV-13): the server's checks on the parent surface as its own refusal if they fail.
 */
export function childFormNote(parentCode: string): string {
  return `Việc con của ${parentCode}.`;
}

export function childCreatedText(code: string): string {
  return `Đã giao việc con ${code}.`;
}

export const PARENT_TITLE = "Việc cha";
export const PARENT_NONE = "Không thuộc việc cha nào — đây là một việc gốc.";
export const PARENT_INPUT_LABEL = "Mã việc cha (mã sổ, ví dụ NV19)";
export const PARENT_SAVE_BUTTON = "Đặt làm việc cha";
export const PARENT_DETACH_BUTTON = "Gỡ khỏi việc cha";

/**
 * `PATCH` body that moves the task under another parent, or `null` (button disabled).
 *
 * TRIMMED, NOT CASE-FOLDED: a register code is whatever the commune issued (§7.1 allows typing
 * one), and guessing `nv19` means `NV19` would be a second rule the server does not have. Every
 * invalid parent — unknown, another commune's, deleted, a cycle, too deep — is the SERVER's 409
 * `task_tree`, shown verbatim; none of those checks is copied here.
 *
 * Empty is `null`, never `{ parent: "" }`: emptying the box is not a way to detach. Detaching is
 * its own button (`DETACH_PARENT_BODY`), so a cleared box cannot silently cut a task from its tree.
 */
export function parentPatchBody(input: string, current: string): petitions_suaNhiemVuVao | null {
  const code = input.trim();
  if (code === "" || code === current) return null;
  return { parent: code };
}

/** `PATCH parent: ""` detaches (ad7f821). */
export const DETACH_PARENT_BODY: petitions_suaNhiemVuVao = { parent: "" };

/**
 * Append the next page of children, dropping codes already shown — a child created between two
 * `Xem thêm` clicks shifts rows across the page boundary. Keyed by `code`: children are task rows,
 * which carry no `id`.
 */
export function mergeChildPages(
  shown: readonly petitions_nhiemVuRa[],
  next: readonly petitions_nhiemVuRa[],
): petitions_nhiemVuRa[] {
  const seen = new Set(shown.map((t) => t.code));
  return [...shown, ...next.filter((t) => !seen.has(t.code))];
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA ĐẶC TẢ **KHÔNG DỰNG ĐƯỢC**, VÀ CHÚNG PHẢI RA TỚI MÀN HÌNH
 *
 * Không giấu trong chú thích, không vẽ một nút chắc chắn hỏng. Cùng khuôn `PHAN_CHUA_DUNG` màn
 * Thu - Chi dùng (`features/thu-chi/nhan-thu-chi.ts`).
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
];

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * KHÔNG CÓ HÀM NÀO Ở ĐÂY VIẾT LẠI MỘT CÂU TỪ CHỐI CỦA MÁY CHỦ, VÀ SỰ VẮNG MẶT ẤY LÀ CHỦ Ý
 *
 * Hai lần từ chối nặng nhất của màn này mang thông tin NẰM TRONG CÂU CHỮ, không nằm trong mã lỗi:
 *
 *   hoàn thành cha còn con   "còn 3 việc con (NV20, NV21, NV22) — hoàn thành hết việc con rồi
 *                            mới hoàn thành việc cha"        (`LoiConChuaXong`, nhiem_vu_ghi.go:331)
 *   xoá cha còn con          "còn 3 việc con chưa xoá — xử lý hoặc xoá các việc con trước"
 *                                                            (`LoiConChuaXoa`, nhiem_vu_ghi.go:323)
 *
 * DANH SÁCH MÃ VÀ CON SỐ LÀ TOÀN BỘ PHẦN CÓ ÍCH. Nuốt chúng thành "có lỗi xảy ra" để lại cho cán
 * bộ đúng thông tin bằng không: họ biết mình không được phép, và không biết còn vướng ở đâu.
 *
 * Nên tầng gọi và màn hình vẽ THẲNG `KetQua.thongBao` của `lib/api/goi.ts` — nó đã mang nguyên
 * văn `message` máy chủ viết. Một hằng chuỗi ở đây sẽ là bản sao thứ hai của một quy tắc nghiệp
 * vụ, và bản sao ấy trôi mà không bài kiểm nào đỏ (luật 9, cấm #2).
 * ══════════════════════════════════════════════════════════════════════════════════════════ */
