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

import type { KetQua } from "@/lib/api/goi";
import type {
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_nhiemVuRa,
  petitions_nhiemVuVanBanRa,
  petitions_suaNhiemVuVao,
  petitions_taoNhiemVuVao,
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
 * Vòng đời §6 — **BẢN THỨ HAI** của `chuyenDuocSangNhiemVu`
 * (`service-petitions/internal/domain/nhiem_vu.go:77-85`), và cái giá của nó được nói ra ở đây
 * chứ không để người sau tự phát hiện.
 *
 * VÌ SAO VẪN CHẤP NHẬN ĐƯỢC: hợp đồng không có tuyến nào phát ra vòng đời, mà §5.2 đòi dải bước
 * biết ô nào **bấm được**. Bản sao này chỉ quyết định MỘT NÚT CÓ HIỆN HAY KHÔNG; nó không quyết
 * định lần ghi nào thành công. `POST /api/v1/tasks/{ma}/status` kiểm lại toàn bộ bằng
 * `ChuyenTrangThaiDuoc` và trả `ErrChuyenTrangThaiNhiemVuSaiLuc` cho bước nó không có.
 *
 * Nên bản sao này trôi theo hai chiều, cả hai đều HỎNG AN TOÀN:
 *   - rộng hơn máy chủ ⇒ một nút hiện ra rồi nhận câu từ chối nguyên văn của máy chủ;
 *   - hẹp hơn máy chủ ⇒ một nút thiếu, cán bộ báo ngay vì họ đang cần bấm nó.
 * Không chiều nào cho ra một lần GHI SAI, và đó là điều kiện để chấp nhận một bản sao.
 */
const CHUYEN_DUOC: Readonly<Record<TrangThaiNhiemVu, readonly TrangThaiNhiemVu[]>> = {
  "moi-giao": ["da-tiep-nhan", "tam-dung", "chuyen-tiep"],
  "da-tiep-nhan": ["dang-thuc-hien", "tam-dung", "chuyen-tiep"],
  "dang-thuc-hien": ["cho-duyet", "tam-dung", "chuyen-tiep"],
  "cho-duyet": ["hoan-thanh", "tam-dung", "chuyen-tiep"],
  "tam-dung": ["moi-giao", "da-tiep-nhan", "dang-thuc-hien", "cho-duyet"],
  "hoan-thanh": [],
  "chuyen-tiep": [],
};

/**
 * Vòng đời CÓ bước này hay không — câu hỏi về HÌNH DẠNG, không phải về quyền.
 *
 * ⚠ LỎNG CÓ CHỦ Ý Ở `tam-dung`, đúng như miền nghiệp vụ: từ `tam-dung` thì bốn trạng thái chính
 * đều là hình dạng hợp lệ, nhưng chỉ ĐÚNG MỘT trong bốn là hợp lệ thật — trạng thái ngay trước
 * lúc tạm dừng. Máy chủ tìm nó trong nhật ký (`TrangThaiTruocTamDung`); màn hình **không tìm
 * được**, vì hợp đồng không có tuyến nhật ký nào. Xem `PHAN_CHUA_DUNG`.
 */
export function chuyenSangDuoc(hienTai: string, moi: string): boolean {
  if (!laTrangThaiNhiemVu(hienTai) || !laTrangThaiNhiemVu(moi)) return false;
  return CHUYEN_DUOC[hienTai].includes(moi);
}

/** Vòng đời có lối ra khỏi trạng thái này không (`hoan-thanh` và `chuyen-tiep` là hai ngõ cụt). */
export function ketThuc(ma: string): boolean {
  return laTrangThaiNhiemVu(ma) && CHUYEN_DUOC[ma].length === 0;
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
      return "Đã duyệt hoàn thành. Nhiệm vụ khép lại ở đây.";
    case "tam-dung":
      return "Đang tạm dừng. Tiếp tục thì việc quay về đúng trạng thái trước lúc dừng.";
    case "chuyen-tiep":
      return "Đã chuyển cho bộ phận khác. Nhiệm vụ này khép lại tại đây.";
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
  "truc-tiep": "Giao trực tiếp",
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

export function oHan(hanISO: string | null, bayGio: Date): OHan {
  const tt = tinhTrangHan(hanISO, bayGio);
  if (tt.loai === "khong-han") return { ngay: O_TRONG, phanTre: "" };
  if (tt.loai === "tre") {
    return {
      ngay: nhanNgay(hanISO),
      phanTre: tt.soNgay < 1 ? "(trễ dưới 1 ngày)" : `(trễ ${tt.soNgay} ngày)`,
    };
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

export const CAU_CHUA_GHI_LANH_DAO_GIAO_VIEC =
  "Nhiệm vụ này chưa ghi lãnh đạo giao việc nên chưa ai duyệt được đề nghị lùi hạn — hãy bổ sung " +
  "lãnh đạo giao việc cho nhiệm vụ.";

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

/** §5.3 — ô `ĐANG GIAO CHO` khi nhiệm vụ chưa về bộ phận nào. Không phải dấu gạch: một trạng thái thật. */
export const CHUA_GIAO_BO_PHAN = "Chưa giao bộ phận nào";

/** Lựa chọn mặc định của hai ô chọn ở §5.7 và §7.1 — nguyên văn đặc tả. */
export const CHUA_XAC_DINH = "— Chưa xác định —";

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
 * Câu nói ra rằng hạn CHỈ ĐẶT ĐƯỢC MỘT LẦN, đặt ngay cạnh ô ngày ở form tạo.
 *
 * Không phải một lời nhắc lịch sự: `han_ban_dau` lấy cùng mốc với `han_xu_ly` lúc INSERT và
 * trigger `nhiem_vu_bat_bien` từ chối mọi lần ghi lại. Một nhiệm vụ tạo ra không có hạn thì
 * **không bao giờ** có hạn nữa — kể cả qua đường đề nghị lùi hạn, vì không có gì để lùi.
 */
export const CANH_BAO_HAN_MOT_LAN =
  "Hạn hoàn thành chỉ đặt được một lần, ngay lúc tạo. Bỏ trống thì nhiệm vụ này sẽ không có hạn " +
  "và cũng không đặt được về sau — kể cả qua đề nghị lùi hạn.";

/** §5.10 — việc con có hạn RIÊNG (ADR 0037 quyết định 2), không thừa kế hạn cha. */
export const GHI_CHU_HAN_VIEC_CON =
  "Việc con có hạn riêng, không lấy theo hạn của việc cha.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * Ô NGÀY ↔ MỐC CỦA HỢP ĐỒNG
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * `<input type="date">` phát ra `YYYY-MM-DD`; hợp đồng đòi một **mốc `date-time`** (`due_at`,
 * `new_due_at` là `time.Time` ở máy chủ, nên `"2026-12-20"` trần là **400**).
 *
 * ⚠ GIỜ TRONG NGÀY LÀ MỘT GIẢ ĐỊNH ĐƯỢC NÓI RA, KHÔNG PHẢI MỘT CHI TIẾT KỸ THUẬT. Đặc tả chỉ cho
 * một ô NGÀY (§7.1, §5.6), nên phải chọn một mốc trong ngày ấy, và hai lựa chọn không tương đương:
 *
 *   00:00  ⇒ nhiệm vụ "hạn 20/6" ĐÃ QUÁ HẠN suốt cả ngày 20/6. Đọc lên là sai.
 *   23:59  ⇒ đúng nghĩa thông thường của "hạn ngày 20/6", và là mốc dùng ở đây.
 *
 * KHÔNG DÙNG GIỜ TAN LÀM VIỆC (17:00 hay bất kỳ số nào): đó là `ca_lam_viec` của TỪNG XÃ, và một
 * con số như thế nung vào bundle là đúng thứ luật 1 bất biến 10 cấm. Nếu khách muốn hạn rơi vào
 * cuối giờ làm việc thì đó là một phép tính của `identity` (nơi giữ lịch làm việc, `ngay_nghi_le`
 * và `ngay_lam_bu`), không phải một hằng ở trình duyệt. Đã báo về như một giả định chờ khách chốt.
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
 * Ô `Ghi chú` của §7.2 KHÔNG có ở form tạo: `petitions.taoNhiemVuVao` không nhận `note`. Vẽ ô ấy
 * ra thì chữ cán bộ gõ vào sẽ mất lặng lẽ; tạo rồi `PATCH` thêm là hai hành vi ghi cho một lần bấm.
 * Câu này đứng trong form để người tìm ô ấy biết nó ở đâu.
 */
export const GHI_CHU_KHONG_CO_O_GHI_CHU =
  "Ô Ghi chú (§7.2) không có ở đây: yêu cầu tạo nhiệm vụ không nhận ghi chú. Giao việc xong, mở " +
  "nhiệm vụ và bấm ✎ Sửa ở khối Sổ theo dõi văn bản chỉ đạo để nhập ghi chú.";

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
  readonly coQuanChuTri: string;
  readonly chuyenVien: string;
  /** `YYYY-MM-DD` hoặc rỗng. */
  readonly han: string;
  readonly vanBan: readonly DongVanBanNhap[];
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
 * TRƯỜNG ĐANG ẨN KHÔNG ĐƯỢC GỬI — CẮT LÚC GỬI, KHÔNG XOÁ LÚC ĐỔI LOẠI. Cán bộ gõ `Cơ quan chủ trì`
 * rồi đổi sang `Nhiệm vụ cơ bản`: ô ấy biến khỏi màn, và một giá trị không còn nhìn thấy mà vẫn lên
 * dây là một bản ghi mang thứ người giao việc tin là đã bỏ. Cắt ở ĐÂY — một chỗ, có bài kiểm — thay
 * vì dọn state trong trình xử lý đổi loại, nơi mỗi ô thêm sau này là một lần phải nhớ dọn. Đổi lại
 * về `Theo văn bản` thì chữ đã gõ hiện lại, và lúc ấy nó ĐANG NHÌN THẤY nên gửi là đúng.
 *
 * `coDanhSachVanBan` LÀ QUYẾT ĐỊNH CỦA BÊN GỌI, KHÔNG PHẢI CỦA LOẠI: màn Biên bản dùng lại form này
 * để gửi `…/conclusions/{stt}/task`, và `petitions.tachKetLuanVao` KHÔNG có `documents`. Ở đó dù
 * loại là `Theo văn bản` cũng không có `documents`.
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
  if (theoVanBan) {
    if (f.coQuanChuTri !== "") than.lead_unit = f.coQuanChuTri;
    if (f.chuyenVien.trim() !== "") than.monitor = f.chuyenVien.trim();
  }
  // HẠN: ô ngày → mốc cuối ngày theo giờ Việt Nam. Bỏ trống thì trường VẮNG MẶT HẲN, không gửi
  // chuỗi rỗng — `due_at` là con trỏ ở máy chủ và "không có hạn" là một trạng thái thật (§4.1 vẽ
  // nó thành `Hạn —`).
  if (f.han !== "") than.due_at = mocCuoiNgay(f.han);
  if (tuyChon.maCha !== undefined && tuyChon.maCha !== "") than.parent = tuyChon.maCha;

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
 * `petitions_suaNhiemVuVao` nhận. Ra năm ô và ba danh sách: `title` · `result_summary` · `note` ·
 * `leader_approved` · `superior_acknowledged` · `documents`. Bốn trường §5.4 còn lại HIỆN mà
 * KHÔNG SỬA ĐƯỢC, mỗi trường một lý do ra tới màn:
 *
 *   Mã nhiệm vụ          mã đã cấp — trigger `nhiem_vu_bat_bien`, luật 7 bất biến 3
 *   Hạn xử lý            PATCH không có `due_at` — hạn chỉ dịch qua đề nghị lùi hạn (§5.8)
 *   Cơ quan chủ trì      PATCH không có `lead_unit`
 *   Chuyên viên          PATCH không có `monitor`
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

/** Mã sổ là mã ĐÃ CẤP: không đổi, không cấp lại (luật 7, bất biến 3; trigger `nhiem_vu_bat_bien`). */
export const LY_DO_KHONG_SUA_MA =
  "Mã nhiệm vụ đã cấp thì giữ nguyên suốt đời hồ sơ, không sửa và không cấp lại.";

/**
 * Vì sao `Hạn xử lý` không sửa được. MỘT câu, dùng ở HAI chỗ — mục `PHAN_CHUA_DUNG` và dòng chỉ
 * đọc trong form `✎ Sửa` — để hai chỗ không trôi khỏi nhau (luật 9, cấm #2).
 */
export const LY_DO_KHONG_SUA_HAN =
  "`PATCH /api/v1/tasks/{ma}` không nhận `due_at`. Hạn ấn định MỘT LẦN lúc tạo việc và sau đó " +
  "chỉ dịch được qua đường đề nghị lùi hạn có người duyệt (§5.8) — cố ý, vì hạn là cam kết đã " +
  "đưa ra, và `han_ban_dau` bị trigger `nhiem_vu_bat_bien` cấm ghi lại sau khi tạo. Một ô ngày " +
  "sửa trực tiếp ở form sẽ là đường vòng qua đúng vòng duyệt ấy.";

/** Vì sao `Cơ quan chủ trì` và `Chuyên viên` không sửa được. Cùng quy tắc một câu hai chỗ. */
export const LY_DO_KHONG_SUA_CHU_TRI =
  "`PATCH /api/v1/tasks/{ma}` không nhận `lead_unit` và `monitor`, nên cơ quan chủ trì tham mưu " +
  "và chuyên viên theo dõi chỉ đặt được lúc giao việc. Sửa được chúng cần máy chủ mở thêm hai " +
  "trường ấy trên tuyến sửa.";

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
 * DỰNG TỪNG TRƯỜNG, KHÔNG TRẢI FORM HAY BẢN GHI. Không bao giờ có `code`, `due_at`, `assigner`,
 * `position` — trường thứ nhất là mã đã cấp, hai trường sau hợp đồng cố ý không nhận, trường cuối
 * do sổ cấp.
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

  return Object.keys(than).length === 0 ? null : than;
}

/**
 * Câu chặn nút `Lưu`, hoặc `null`. Cùng phép kiểm dòng văn bản với form tạo (`canhBaoVanBan`),
 * cộng hai điều chỉ form sửa gặp: tiêu đề bị xoá trắng, và một ngày văn bản sai khuôn đọc từ máy chủ.
 */
export function canhBaoSua(f: FormSuaNhiemVu): string | null {
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

/** Hai chế độ xem DỰNG ĐƯỢC. Chế độ thứ ba (`Sổ theo dõi` §4.3) — xem `PHAN_CHUA_DUNG`. */
export const NHAN_CHE_DO_KANBAN = "▦ Kanban";
export const NHAN_CHE_DO_DANH_SACH = "☰ Danh sách";

/** Đặt cạnh cụm chọn chế độ xem, để cán bộ không đi tìm cái nút thứ ba của đặc tả. */
export const GHI_CHU_THIEU_SO_THEO_DOI =
  "Đặc tả có chế độ xem thứ ba — Sổ theo dõi (§4.3) — và nó chưa dựng được. Lý do nằm ở phần " +
  "chưa dựng được đầu màn.";

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
    `${nhanTrangThai(bang, "chuyen-tiep")} — không có cột riêng trên Kanban (§4.1), nên việc đang ` +
    "ở hai trạng thái ấy không hiện ở bảng này. Xem chúng ở chế độ Danh sách."
  );
}

/**
 * Con số trên đầu cột — **SỐ THẺ ĐÃ TẢI VỀ**, không phải tổng số việc của cột.
 *
 * `page.Result` của tuyến đọc sổ chỉ mang `items` · `next_cursor` · `has_more`: **KHÔNG CÓ TỔNG
 * SỐ**. Nên một con số trần ở đây sẽ đọc ra là "cột này có 20 việc" trong khi sự thật là "cột này
 * có ít nhất 20 việc", và con số ấy đi thẳng vào một câu báo cáo miệng với lãnh đạo. Dấu `+` là
 * toàn bộ phần trung thực của nhãn này; `GHI_CHU_DEM_COT` nói nốt phần còn lại.
 */
export function nhanDemCot(soThe: number, conNua: boolean): string {
  return conNua ? `${soThe}+` : String(soThe);
}

export const GHI_CHU_DEM_COT =
  "Số trên đầu mỗi cột là số thẻ đã tải về cột ấy; dấu + nghĩa là còn nữa. Tuyến đọc sổ trả về " +
  "từng trang và không trả tổng số, nên đây không phải tổng số việc của cột.";

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
  {
    ten: "Ô đổi bộ phận / người thực hiện ở form sửa (§5.7)",
    viSao:
      "`task.assign` chưa có tuyến, và `PATCH /api/v1/tasks/{ma}` CỐ Ý không đổi `bo_phan_id` hay " +
      "`nguoi_thuc_hien_ma`. Gộp việc giao người vào form sửa sẽ trao quyền giao việc cho mọi tài " +
      "khoản cầm `task.update` — một khoá khác hẳn, và bảng `quyen` có sẵn `task.assign` để tách " +
      "hai việc ấy ra.",
  },
  {
    ten: "`Chuyển tiếp` không nối hai nhiệm vụ (§6)",
    viSao:
      "§6 viết chuyển tiếp `sinh bản ghi liên kết`, nhưng KHÔNG CỘT NÀO nối hai nhiệm vụ với " +
      "nhau. Hệ quả có thật, và nó hiện ngay trên màn: một nhiệm vụ cha còn việc con đã " +
      "`chuyen-tiep` thì KHÔNG hoàn thành được — máy chủ chỉ tính `hoan-thanh` là xong, vì nó " +
      "không có đường nào lần theo để biết việc đã chuyển đi có xong ở nơi khác hay chưa.",
  },
  {
    ten: "Bộ lọc `Liên quan đến tôi` (§3)",
    viSao:
      "Máy chủ TỪ CHỐI `?scope=related` kèm lý do: bộ lọc này cần `bộ phận tôi đang giữ`, mà hợp " +
      "đồng phiên cán bộ không trả về bộ phận. Trả lời bằng ba vế còn lại sẽ tệ hơn từ chối — cán " +
      "bộ có việc do bộ phận mình đang giữ sẽ KHÔNG thấy nó trên đúng cái tab mang tên ấy.",
  },
  {
    ten: "Ô tick `Sắp đến hạn` (§3)",
    viSao:
      "Máy chủ TỪ CHỐI `?soon=`: ngưỡng là `sla.gio_sap_den_han`, số giờ của TỪNG XÃ, và chưa có " +
      "đường đọc số ấy. Con số `72 giờ` ở đặc tả là MẶC ĐỊNH xã ghi đè được, nên nung nó vào màn " +
      "hình là dựng nguồn thứ hai cho một con số mà mọi chỗ `sắp đến hạn` phải đọc từ một cột duy nhất.",
  },
  {
    ten: "Nhật ký & Trao đổi (§5.9) và `Tiếp tục` sau khi tạm dừng (§6)",
    viSao:
      "Hợp đồng không có tuyến nhật ký nào — tám tuyến Nhiệm vụ không gồm `POST .../nhat-ky` hay " +
      "tuyến đọc dòng thời gian. Kéo theo một chỗ nữa: trạng thái trước lúc tạm dừng nằm TRONG " +
      "nhật ký, nên màn hình không biết việc đang tạm dừng phải quay về đâu; nó hiện cả bốn lối và " +
      "để máy chủ từ chối ba lối sai.",
  },
  {
    ten: "Chip `{n} việc con` trên thẻ (§4.1) và khối Nhiệm vụ con (§5.10)",
    viSao:
      "`petitions.nhiemVuRa` có `parent` (mã việc cha) nhưng KHÔNG có số việc con, và không có " +
      "tuyến liệt kê việc con của một mã. Đếm trong trang đang mở sẽ ra một con số phụ thuộc vào " +
      "trang — `2 việc con` ở trang này và `3 việc con` ở trang sau, cho cùng một nhiệm vụ.",
  },
  {
    ten: "Ô `Ghi chú` ở form `Giao việc mới` (§7.2)",
    viSao:
      "`petitions.taoNhiemVuVao` không nhận `note`, nên form tạo không có ô Ghi chú. Ghi chú nhập " +
      "được ngay sau khi giao việc, bằng nút `✎ Sửa` của khối SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO trong " +
      "khung chi tiết. Tạo rồi tự gửi thêm một lần sửa sau lưng cán bộ là hai hành vi ghi cho một " +
      "lần bấm, nên màn này không làm thế.",
  },
  {
    ten: "Sửa `Cơ quan chủ trì tham mưu` và `Chuyên viên theo dõi` ở form sửa (§5.4)",
    viSao: LY_DO_KHONG_SUA_CHU_TRI,
  },
  {
    ten: "Sửa `Hạn hoàn thành` ở form sửa (§5.4, §5.6)",
    viSao: LY_DO_KHONG_SUA_HAN,
  },
  {
    ten: "⬆ Nhập từ Excel (§8) · 🗑 Xoá đã chọn (§2) · Xuất Sổ theo dõi (§4.3)",
    viSao:
      "Ba tuyến §10 đề xuất — `nhap-excel`, `xoa-nhieu`, `xuat-so-theo-doi` — không có trong hợp " +
      "đồng. Xoá từng nhiệm vụ thì có (`DELETE /api/v1/tasks/{ma}`, kèm lý do bắt buộc), nên thao " +
      "tác hàng loạt là bấm từng dòng chứ không phải một nút gom.",
  },
  {
    ten: "THÊM VIỆC CON (§5.10) và chuyển việc sang cha khác (§5.4)",
    viSao:
      "`petitions.taoNhiemVuVao.parent` và `petitions.suaNhiemVuVao.parent` nhận **id nội bộ** " +
      "của việc cha, nhưng `petitions.nhiemVuRa` KHÔNG phát ra `id` của chính nó — chỉ có `code` " +
      "(`NV19`) và `parent` (id của cha). Nên một màn hình đang mở NV19 biết cha của NV19 là ai mà " +
      "không bao giờ biết id của NV19, tức không gửi nổi một yêu cầu tạo việc con. Vẽ ô ấy ra rồi " +
      "gửi `NV19` vào `parent` sẽ là 409 `task_tree` ở mọi lần bấm. Cần thêm `id` vào phản hồi, " +
      "hoặc cho `parent` nhận mã sổ.",
  },
  {
    ten: "Khối `Duyệt / Từ chối` đề nghị lùi hạn của người khác (§5.8)",
    viSao:
      "Tuyến quyết định cần `deNghiID`, mà hợp đồng KHÔNG có tuyến nào liệt kê đề nghị đang chờ " +
      "của một nhiệm vụ, và `petitions.nhiemVuRa` cũng không mang đề nghị nào. Màn hình chỉ cầm " +
      "được id của đúng đề nghị VỪA GỬI trong cùng lượt mở — trong khi người duyệt theo ADR 0038 " +
      "là một người KHÁC, mở drawer sau đó. Phép kiểm hai lớp của ADR 0038 vẫn chạy và vẫn hiện " +
      "câu từ chối, nhưng lãnh đạo không có đề nghị nào để bấm.",
  },
  {
    ten: "Ô chọn `Người thực hiện` · `Lãnh đạo giao việc` · `Chuyên viên theo dõi` (§3, §7)",
    viSao:
      "Ba ô ấy cần danh bạ cán bộ, mà `GET /api/v1/staff` đứng sau `admin.user` — khoá quản trị " +
      "danh bạ, không phải khoá nhiệm vụ. Một cán bộ có `task.create` mà không có `admin.user` sẽ " +
      "nhận 403 khi màn hình đi đổ ba ô chọn ấy. Nên ba ô là **ô gõ mã cán bộ** (`CB-…`) chứ không " +
      "phải ô chọn, và chúng nhận đúng loại giá trị ba cột kia giữ (luật 6, bất biến 8).",
  },
  {
    ten: "CỔNG QUYỀN Ở GIAO DIỆN cho bảy khoá `task.*`",
    viSao:
      "Bảy khoá `task.read` · `task.create` · `task.update` · `task.approve` · `task.extend` · " +
      "`task.delete` · `task.assign` ĐỀU CÓ trong bảng `quyen` và đều được tuyến khai, nhưng " +
      "`lib/quyen.ts` chưa có hằng nào cho chúng và lượt này không được sửa tệp ấy. Hệ quả: màn " +
      "hình vẽ đủ nút cho mọi tài khoản đọc được sổ, và câu 403 của máy chủ ra thẳng màn hình — " +
      "cùng khuôn `document.read` đã chọn. Lớp chặn thật không đổi (máy chủ kiểm từng yêu cầu); " +
      "thứ thiếu là sự tiện dụng. RIÊNG lớp hai của ADR 0038 vẫn chạy: nút duyệt vẫn ẩn với người " +
      "không phải lãnh đạo giao việc ghi trên bản ghi.",
  },
  {
    ten: "KÉO-THẢ thẻ giữa các cột Kanban (§4.1)",
    viSao:
      "Không dựng, và lý do là KHẢ NĂNG TIẾP CẬN chứ không phải công sức: kéo-thả HTML5 không có " +
      "lối bàn phím tương đương, nên một bảng chỉ đổi được trạng thái bằng cách kéo là một bảng " +
      "cán bộ dùng bàn phím — hoặc dùng chuột không vững — không thao tác được " +
      "(`skills/accessibility-elderly`). Đổi trạng thái trên Kanban đi qua drawer: bấm `Mở NV…` " +
      "trên thẻ, dùng khối `Chuyển trạng thái` §6. ĐÓ LÀ CÙNG MỘT TUYẾN " +
      "(`POST /api/v1/tasks/{ma}/status`) và cùng chỗ in NGUYÊN VĂN câu từ chối của máy chủ — kể " +
      "cả câu liệt kê mã việc con còn lại khi hoàn thành việc cha.",
  },
  {
    ten: "SỐ LƯỢNG THẬT của mỗi cột Kanban (§4.1)",
    viSao:
      "`page.Result` chỉ mang `items` · `next_cursor` · `has_more` — KHÔNG có tổng số. Nên đầu cột " +
      "hiện số thẻ đã tải kèm dấu `+` khi còn nữa, chứ không hiện một con số trần: `20` đọc ra là " +
      "`cột này có 20 việc`, trong khi sự thật là `ít nhất 20`, và con số ấy đi thẳng vào một câu " +
      "báo cáo với lãnh đạo. Kanban cũng vì thế không có phân trang từng cột — xem tiếp ở Danh sách.",
  },
  {
    ten: "Chế độ xem `Sổ theo dõi` (§4.3)",
    viSao:
      "Cụm chọn chế độ xem có HAI nút chứ không phải ba. Bảng §4.3 lấy quá nửa số cột từ ba nhóm " +
      "văn bản chỉ đạo, mà tuyến đọc sổ `GET /api/v1/tasks` CỐ Ý không trả `documents` — trên sổ, " +
      "trường ấy vắng mặt nghĩa là `không phục vụ ở đây`, không phải `không có văn bản`. Đọc chi " +
      "tiết từng dòng để ghép bảng là một lời gọi cho mỗi dòng của mỗi trang. Nút xuất Excel giữ " +
      "đúng thứ tự cột cũng chưa có tuyến (`xuat-so-theo-doi` không có trong hợp đồng). Cả hai là " +
      "phần việc của máy chủ.",
  },
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
