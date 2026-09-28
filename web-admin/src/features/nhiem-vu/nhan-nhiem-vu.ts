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

import { danhBaTheoMa, nhanThoiDiem, type DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import type { ChieuSapXepNhiemVu, CotSapXepNhiemVu } from "@/lib/api/nhiem-vu";
import {
  QUYEN_CAP_NHAT_NHIEM_VU,
  QUYEN_DUYET_GIA_HAN,
  QUYEN_DUYET_HOAN_THANH_NHIEM_VU,
  QUYEN_TAO_NHIEM_VU,
  QUYEN_XOA_NHIEM_VU,
  coQuyen,
} from "@/lib/quyen";
import type {
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
 * (`service-petitions/internal/domain/nhiem_vu.go:83-91`), và cái giá của nó được nói ra ở đây
 * chứ không để người sau tự phát hiện.
 *
 * `cho-duyet` → `dang-thuc-hien` là MŨI TÊN NGƯỢC DUY NHẤT — "Trả lại để làm tiếp", quyết định của
 * chủ đầu tư 27/09/2026 (`nhiem_vu.go:77-82`), không có trên sơ đồ §6. Bản đồ chỉ nói HÌNH DẠNG ấy có;
 * ai được đi bước ấy (`task.approve`) và nó phải mang gì (lý do) là của `laBuocTraLai` ở dưới, và của
 * máy chủ (`app/nhiem_vu.go:1062-1070`).
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
  "cho-duyet": ["hoan-thanh", "dang-thuc-hien", "tam-dung", "chuyen-tiep"],
  "tam-dung": ["moi-giao", "da-tiep-nhan", "dang-thuc-hien", "cho-duyet"],
  "hoan-thanh": [],
  "chuyen-tiep": [],
};

/**
 * Vòng đời CÓ bước này hay không — câu hỏi về HÌNH DẠNG, không phải về quyền.
 *
 * ⚠ LỎNG CÓ CHỦ Ý Ở `tam-dung`, đúng như miền nghiệp vụ: từ `tam-dung` thì bốn trạng thái chính
 * đều là hình dạng hợp lệ, nhưng chỉ ĐÚNG MỘT trong bốn là hợp lệ thật — trạng thái ngay trước
 * lúc tạm dừng. Máy chủ tìm nó trong nhật ký (`TrangThaiTruocTamDung`); màn hình **chưa tìm**:
 * tuyến đọc nhật ký nay có, nhưng dòng ngay trước lúc dừng có thể nằm ở trang bất kỳ, và việc lần
 * theo nó để thu hẹp bốn nút chưa dựng. Xem `PHAN_CHUA_DUNG`.
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
 * Ô ghi tay `Đã làm được gì, còn vướng gì…` KHÔNG có ở đây: tuyến ghi chưa dựng, chờ luật "ai
 * đang giữ việc thì được ghi". Xem `PHAN_CHUA_DUNG`.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Tiêu đề khối — nguyên văn §5.9. */
export const TIEU_DE_NHAT_KY_NHIEM_VU = "Nhật ký & Trao đổi";

/** Đang đọc trang đầu. `role="status"`, không phải `alert`. */
export const DANG_TAI_NHAT_KY_NHIEM_VU = "Đang tải nhật ký…";

export const NHAN_XEM_THEM_NHAT_KY_NHIEM_VU = "Xem thêm";

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
  const cb = danhBa?.get(ma);
  if (cb === undefined || cb.full_name === "") return ma;
  return `${cb.full_name} (${ma})`;
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
  /** Bước `Chuyển sang Hoàn thành` — `task.update` VÀ `task.approve`. */
  readonly duyetHoanThanh: boolean;
  /** Xoá khỏi sổ — `task.delete`. */
  readonly xoa: boolean;
  /** LỚP MỘT của duyệt lùi hạn — `task.extend`. Lớp hai (ADR 0038) vẫn là `quyetDinhDuyetLuiHan`. */
  readonly duyetGiaHan: boolean;
};

/**
 * Danh sách quyền của phiên → cổng từng nút. `null` = phiên chưa đọc xong hoặc đọc hỏng.
 *
 * FAIL CLOSED: không đọc được quyền thì MỌI cổng đóng. "Chưa rõ" không được hành xử như "có" (luật
 * 1, cấm #1). Mỗi khoá so CHÍNH XÁC qua `coQuyen` — không tiền tố, không `task.*` (luật 5, bất biến 3b).
 *
 * `duyetHoanThanh` ĐÒI CẢ HAI KHOÁ vì tuyến `…/status` khai `task.update` ở cổng, rồi đòi thêm
 * `task.approve` cho riêng bước `hoan-thanh`: có `task.approve` mà thiếu `task.update` vẫn là 403 ở cổng.
 */
export function quyenNhiemVu(dsQuyen: readonly string[] | null): QuyenNhiemVu {
  const ds = dsQuyen ?? [];
  const capNhat = coQuyen(ds, QUYEN_CAP_NHAT_NHIEM_VU);
  return {
    giaoViec: coQuyen(ds, QUYEN_TAO_NHIEM_VU),
    capNhat,
    duyetHoanThanh: capNhat && coQuyen(ds, QUYEN_DUYET_HOAN_THANH_NHIEM_VU),
    xoa: coQuyen(ds, QUYEN_XOA_NHIEM_VU),
    duyetGiaHan: coQuyen(ds, QUYEN_DUYET_GIA_HAN),
  };
}

/**
 * Vòng đời có bước `hoan-thanh` nhưng tài khoản thiếu `task.approve`. Nói ra, vì một nút biến mất
 * không lời đọc lên là "vòng đời thiếu bước" — cán bộ báo lỗi phần mềm thay vì xin cấp quyền.
 */
export const CAU_THIEU_QUYEN_DUYET_HOAN_THANH =
  "Bước hoàn thành và bước trả lại để làm tiếp cần quyền duyệt hoàn thành. Tài khoản của bạn chưa " +
  "được cấp quyền này.";

/**
 * Bước chuyển trạng thái này có cần hiện nút THƯỜNG cho tài khoản này không — xem `quyenNhiemVu`.
 *
 * BƯỚC TRẢ LẠI (`laBuocTraLai`) KHÔNG ĐI QUA ĐÂY: nó không phải một nút `Chuyển sang …` mà là một ô lý
 * do bắt buộc (`KhoiTraLai`), và cổng của nó là `duocTraLai`. Bên gọi lọc nó ra trước.
 */
export function duocBamChuyen(quyen: QuyenNhiemVu, sangTrangThai: string): boolean {
  if (!quyen.capNhat) return false;
  return sangTrangThai === "hoan-thanh" ? quyen.duyetHoanThanh : true;
}

/* ── "TRẢ LẠI ĐỂ LÀM TIẾP" — `cho-duyet` → `dang-thuc-hien` ─────────────────────────────────────
 *
 * Cùng tuyến `POST /api/v1/tasks/{ma}/status`, cùng thân (`status` + `note`). Hai điều khiến nó khác
 * mọi bước khác, cả hai do MÁY CHỦ cưỡng chế (`app/nhiem_vu.go:1062-1070`):
 *   - chỉ người cầm `task.approve` — CÙNG khoá với duyệt hoàn thành (`ErrKhongDuocTraLai`, :193-204):
 *     trả lại là nửa kia của cùng một phán quyết;
 *   - `note` là LÝ DO, bắt buộc, cắt khoảng trắng trước khi kiểm (`KiemLyDoTraLai`,
 *     `domain/nhiem_vu_ghi.go:505-519`) — người nhận lại việc phải biết còn thiếu gì.
 * `da-tiep-nhan` → `dang-thuc-hien` là bước THUẬN thường, không cần khoá lẫn lý do; nên câu hỏi là
 * về CẶP (từ, sang), không bao giờ chỉ về trạng thái đích.
 */

/**
 * The plain `Chuyển sang …` steps this account may take from `status` — lifecycle shape
 * (`chuyenSangDuoc`) minus the return step (it needs a mandatory reason, `KhoiTraLai`) minus what
 * the session's keys do not allow (`duocBamChuyen`).
 *
 * ONE LIST FOR THE DRAWER AND THE KANBAN. Two filters would drift, and the drifted one would
 * offer a Kanban move the drawer hides — or hide one the drawer offers. Order is `MOI_TRANG_THAI`.
 */
export function clickableTransitions(status: string, quyen: QuyenNhiemVu): TrangThaiNhiemVu[] {
  return MOI_TRANG_THAI.filter(
    (t) => chuyenSangDuoc(status, t) && !laBuocTraLai(status, t) && duocBamChuyen(quyen, t),
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

/** Cặp (từ, sang) này có phải bước trả lại không — cùng vị từ với `domain.LaTraLaiLamTiep`. */
export function laBuocTraLai(tu: string, sang: string): boolean {
  return tu === "cho-duyet" && sang === "dang-thuc-hien";
}

/**
 * Tài khoản có được thấy ô trả lại không. `task.update` (cổng tuyến) VÀ `task.approve` — đúng
 * `duyetHoanThanh`, vì máy chủ đòi đúng hai khoá ấy. Ẩn là tiện dụng: máy chủ vẫn kiểm (luật 5, cấm #1).
 */
export function duocTraLai(quyen: QuyenNhiemVu): boolean {
  return quyen.duyetHoanThanh;
}

/**
 * Giới hạn của máy chủ: lý do CHÍNH LÀ dòng nhật ký mà bước ấy ghi, nên dùng chung
 * `NoiDungNhatKyToiDa` (`service-petitions/internal/domain/nhiem_vu_ghi.go:43`). Ô dừng đúng chỗ;
 * lệch khỏi máy chủ thì câu từ chối của máy chủ vẫn ra nguyên văn.
 */
export const LY_DO_TRA_LAI_TOI_DA = 5000;

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
  phamVi?: "mine";
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

  if (t.get("scope") === "mine") loc.phamVi = "mine";
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
  readonly tim?: string;
  readonly sapXep?: CotSapXepNhiemVu;
  readonly chieu?: ChieuSapXepNhiemVu;
}): string {
  const t = new URLSearchParams();
  if (loc.phamVi === "mine") t.set("scope", "mine");
  if (loc.trangThai) t.set("status", loc.trangThai);
  if (loc.nguonGiao) t.set("source", loc.nguonGiao);
  if (loc.loai) t.set("type", loc.loai);
  if (loc.khoi) t.set("bloc", loc.khoi);
  if (loc.mucUuTien) t.set("priority", loc.mucUuTien);
  if (loc.boPhanID) t.set("unit", loc.boPhanID);
  if (loc.nguoiThucHienMa) t.set("assignee", loc.nguoiThucHienMa);
  if (loc.chiTreHan === true) t.set("late", "true");
  if (loc.sapXep) t.set("sort", loc.sapXep);
  if (loc.chieu) t.set("order", loc.chieu);
  return t.toString();
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * §4.2 — SẮP XẾP BẢNG DANH SÁCH ("cột có thể sắp xếp (icon ⇅)")
 *
 * CHỈ HAI CỘT, vì máy chủ chỉ sắp được hai: `created_at` và `code` (`SapXepNhiemVu`,
 * `service-petitions/internal/store/nhiem_vu.go:92-95`). Vì sao các cột khác không có mũi tên nằm
 * ở `PHAN_CHUA_DUNG`. Một mũi tên sắp theo trang đang mở — chỉ 20 dòng — sẽ trông như sắp cả sổ
 * trong khi chỉ xếp lại một lát cắt tuỳ con trỏ, nên không có mũi tên nào sắp ở trình duyệt.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

const MOI_COT_SAP_XEP: readonly CotSapXepNhiemVu[] = ["created_at", "code"];

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
 *
 * ⚠ ĐỔI CÁCH SẮP LÀ VỀ TRANG ĐẦU, và đó không phải lựa chọn giao diện: con trỏ mang `sort`/`order`
 * bên trong và máy chủ trả 400 `invalid_cursor` cho con trỏ của một cách sắp khác
 * (`core/page/page.go:456-457`). Hàm này chỉ trả cách sắp mới; màn hình đưa nó qua đúng lối đổi bộ
 * lọc (`datLocMoi`), lối ấy đặt lại ngăn xếp con trỏ.
 */
export function bamCotSapXep(hienTai: SapXepSo, cot: CotSapXepNhiemVu): SapXepSo {
  if (hienTai.cot === cot) return { cot, chieu: hienTai.chieu === "asc" ? "desc" : "asc" };
  return { cot, chieu: cot === "code" ? "asc" : "desc" };
}

/** `aria-sort` của một tiêu đề cột — để trình đọc màn hình đọc đúng chiều. */
export function ariaSapXep(
  hienTai: SapXepSo,
  cot: CotSapXepNhiemVu,
): "ascending" | "descending" | "none" {
  if (hienTai.cot !== cot) return "none";
  return hienTai.chieu === "asc" ? "ascending" : "descending";
}

/** Nhãn hai cột sắp được — `Ngày giao` là `created_at`: lúc nhiệm vụ được giao cũng là lúc nó vào sổ. */
export const NHAN_COT_MA = "Mã";
export const NHAN_COT_NGAY_GIAO = "Ngày giao";

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
 * VÌ SAO LÀ MỘT MỤC TRÊN SỔ, KHÔNG PHẢI HAI NÚT TRONG DRAWER: §5.8 đặt ô ĐỀ NGHỊ trong drawer, và
 * không vẽ chỗ nào cho quyết định. Tuyến hàng chờ KHÔNG lọc được theo nhiệm vụ, nên đưa quyết định
 * vào drawer nghĩa là lật cả hàng chờ của xã — một lời gọi mỗi trang — chỉ để tìm một dòng. Nên
 * quyết định nằm ở mục `Đề nghị lùi hạn chờ duyệt` trên sổ, và drawer chỉ đường tới đó.
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

/** Liên kết trong drawer §5.8 tới hàng chờ. */
export const CAU_LIEN_KET_HANG_CHO =
  "Duyệt hoặc từ chối đề nghị lùi hạn ở mục “Đề nghị lùi hạn chờ duyệt” đầu sổ.";

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

/** §5.3 — ô `ĐANG GIAO CHO` khi nhiệm vụ chưa về bộ phận nào. Không phải dấu gạch: một trạng thái thật. */
export const CHUA_GIAO_BO_PHAN = "Chưa giao bộ phận nào";

/** Lựa chọn mặc định của hai ô chọn ở §5.7 và §7.1 — nguyên văn đặc tả. */
export const CHUA_XAC_DINH = "— Chưa xác định —";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BA Ô CHỌN CÁN BỘ (§3 `Người thực hiện`; §7 `Người thực hiện` · `Lãnh đạo giao việc` ·
 * `Chuyên viên theo dõi`) — đổ từ DANH BẠ CHỌN NGƯỜI
 *
 * Nguồn là `GET /api/v1/staff-directory` (`AnyAuthenticated`, chỉ mã · họ tên · chức vụ · bộ phận),
 * KHÔNG phải `GET /api/v1/staff` (đứng sau `admin.user`). Giá trị gửi đi vẫn là MÃ NGHIỆP VỤ
 * `CB-…` (`code`) — đúng loại ba trường `assignee` · `assigner` · `monitor` của hợp đồng giữ
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
 * TRƯỜNG ĐANG ẨN KHÔNG ĐƯỢC GỬI — CẮT LÚC GỬI, KHÔNG XOÁ LÚC ĐỔI LOẠI. Cán bộ gõ `Cơ quan chủ trì`
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
  if (theoVanBan) {
    if (f.coQuanChuTri !== "") than.lead_unit = f.coQuanChuTri;
    if (f.chuyenVien.trim() !== "") than.monitor = f.chuyenVien.trim();
  }
  // HẠN: ô ngày → mốc cuối ngày theo giờ Việt Nam. Bỏ trống thì trường VẮNG MẶT HẲN, không gửi
  // chuỗi rỗng — `due_at` là con trỏ ở máy chủ và "không có hạn" là một trạng thái thật (§4.1 vẽ
  // nó thành `Hạn —`).
  if (f.han !== "") than.due_at = mocCuoiNgay(f.han);
  if (tuyChon.maCha !== undefined && tuyChon.maCha !== "") than.parent = tuyChon.maCha;

  // GHI CHÚ: cùng cổng với ba danh sách văn bản, cùng hai lý do. §7.3 bỏ ô này ở loại khác `Theo
  // văn bản` (cắt lúc gửi, như `lead_unit`); và `petitions.tachKetLuanVao` của màn Biên bản KHÔNG
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
 *
 * Owner's decision of 28/09/2026: kept on purpose, not a gap waiting for server work.
 */
export const LY_DO_KHONG_SUA_HAN =
  "Giữ có chủ ý — chủ đầu tư quyết định ngày 28/09/2026: hạn hoàn thành chỉ dịch được qua đề " +
  "nghị lùi hạn có người duyệt (§5.8), không sửa trực tiếp. Sửa thẳng một ô ngày sẽ làm đổi tỷ " +
  "lệ đúng hạn báo cáo lên lãnh đạo, trong khi con số ấy phải đếm theo đúng cam kết đã đưa ra. " +
  "Vì vậy `PATCH /api/v1/tasks/{ma}` không nhận `due_at`.";

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
    ten: "`Chuyển tiếp` — giao tiếp cùng nhiệm vụ cho nơi khác (§6)",
    viSao:
      "Nghĩa của `Chuyển tiếp` đã được chủ đầu tư quyết định ngày 28/09/2026: vẫn là CÙNG nhiệm " +
      "vụ ấy, làm tiếp, được giao cho một đơn vị hoặc một người khác — không sinh nhiệm vụ mới. " +
      "Phần máy chủ cho nghĩa ấy đang làm. Cho tới khi xong, bước `chuyen-tiep` trên màn vẫn chạy " +
      "theo cách cũ: nhiệm vụ dừng ở trạng thái ấy, không có lối ra, và một nhiệm vụ cha còn việc " +
      "con đã `chuyen-tiep` thì chưa hoàn thành được.",
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
    ten: "Ô ghi tay `Ghi nhật ký` (§5.9) và `Tiếp tục` sau khi tạm dừng (§6)",
    viSao:
      "Khối Nhật ký & Trao đổi nay ĐỌC được (`GET /api/v1/tasks/{ma}/log-entries`), nhưng chưa " +
      "GHI được: tuyến `POST` một dòng ghi tay chưa dựng, vì còn chờ luật người đang giữ việc — ai " +
      "được ghi vào nhật ký của một nhiệm vụ. Mọi dòng hiện có là dòng máy chủ tự ghi ở mỗi thao tác. " +
      "`Tiếp tục` sau tạm dừng: trạng thái trước lúc dừng nằm trong nhật ký, nhưng có thể ở trang " +
      "bất kỳ, và màn hình chưa lần theo nó — nên hiện cả bốn lối và để máy chủ từ chối ba lối sai.",
  },
  {
    ten: "Chip `{n} việc con` trên thẻ (§4.1) và khối Nhiệm vụ con (§5.10)",
    viSao:
      "`petitions.nhiemVuRa` có `parent` (mã việc cha) nhưng KHÔNG có số việc con, và không có " +
      "tuyến liệt kê việc con của một mã. Đếm trong trang đang mở sẽ ra một con số phụ thuộc vào " +
      "trang — `2 việc con` ở trang này và `3 việc con` ở trang sau, cho cùng một nhiệm vụ.",
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
    ten: "`Duyệt / Từ chối` ngay trong drawer của nhiệm vụ (§5.8)",
    viSao:
      "Duyệt / Từ chối nay nằm ở mục `Đề nghị lùi hạn chờ duyệt` đầu sổ " +
      "(`GET /api/v1/task-extensions`, mở sẵn ở `Chờ tôi duyệt`). Drawer chỉ đường tới đó chứ không " +
      "hiện đề nghị đang chờ của chính nhiệm vụ ấy: tuyến hàng chờ KHÔNG lọc được theo nhiệm vụ, và " +
      "`petitions.nhiemVuRa` không mang đề nghị nào — tìm một dòng là lật cả hàng chờ của xã, một " +
      "lời gọi mỗi trang. Cần một bộ lọc `task` trên tuyến hàng chờ, hoặc đề nghị đang chờ trong " +
      "phản hồi chi tiết.",
  },
  {
    ten: "Sắp xếp theo `Hạn`, `Tên việc`, `Ưu tiên` và các cột khác của bảng Danh sách (§4.2)",
    viSao:
      "§4.2 cho mọi cột một mũi tên ⇅, nhưng máy chủ chỉ sắp được theo `Mã` và `Ngày giao` — hai " +
      "cột có mũi tên trên bảng. `Hạn` có thể để trống, và phân trang theo mốc làm mất hẳn mọi việc " +
      "không có hạn từ trang hai trở đi; cần việc trễ lên đầu thì dùng ô `Chỉ việc quá hạn`. `Tên " +
      "việc` hay trích lời phản ánh của người dân, nên không đưa được lên đường dẫn. `Ưu tiên` là mã " +
      "danh mục, xếp theo chữ cái sẽ ra một thứ tự không phải thứ tự của thang ưu tiên. Sắp lại " +
      "trong trình duyệt chỉ xếp được 20 dòng đang hiện, trông như xếp cả sổ mà không phải.",
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
