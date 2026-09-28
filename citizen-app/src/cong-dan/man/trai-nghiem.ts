/**
 * BẢN TRẢI NGHIỆM CỦA APP RIÊNG MỘT XÃ — phiếu phản ánh CHỈ SỐNG TRONG BỘ NHỚ; họ tên lấy từ Zalo bằng
 * một hành động xin quyền (`LayTenZalo`), không đăng nhập, không người dùng giả lập (bỏ 28/09/2026).
 *
 * PHIẾU TRẢI NGHIỆM MANG ĐÚNG KIỂU `PhieuCuaToi` CỦA HỢP ĐỒNG THẬT (`api/hop-dong-phan-anh.ts`): năm ô
 * gửi đi (nội dung · nơi xảy ra · họ tên · điện thoại · ẩn danh), trạng thái trong bảng `TRANG_THAI`,
 * lĩnh vực dân chọn lúc gửi (ADR 0050 #1; bản trải nghiệm chưa có mã danh mục nên `linh_vuc` và
 * `nhan_linh_vuc` cùng mang tên — máy chủ trả mã và nhãn riêng), hai mốc hạn, kết quả, lý do, cơ quan nhận. Ngày nối máy
 * chủ, màn hình giữ nguyên — chỉ nguồn dữ liệu đổi.
 *
 * PHIẾU TRẢI NGHIỆM KHÔNG ĐI VÀO HỆ THỐNG CỦA XÃ. Một phiếu mang danh tính giả mà vào sổ phản ánh thật
 * của một cơ quan nhà nước là một hồ sơ lưu trữ không ai đứng tên (luật 4, luật 6). Phiếu nằm trong
 * `useState`, mất khi đóng app, và MỌI màn hiện nó đều gắn nhãn "Bản trải nghiệm". Hai mốc hạn để
 * `null`: hạn đếm bằng GIỜ LÀM VIỆC theo lịch từng xã, chỉ `identity` đếm được (ADR 0007) — bản trải
 * nghiệm không bịa một ngày.

 */
import { DO_DAI_TOI_DA, type PhieuCuaToi } from "../api/hop-dong-phan-anh";

import { groupOf, STATUS_GROUP_LABEL, type StatusFilter, type StatusGroup, STEP_LABEL } from "./status-groups";

/**
 * HỌ TÊN TỪ ZALO — HÀNH ĐỘNG XIN QUYỀN DUY NHẤT (chủ dự án, 28/09/2026: "không còn đăng nhập nữa, chỉ cần
 * xin quyền để lấy được name, phone number"). Lớp vỏ tiêm hàm thật (`getUserInfo`); nửa này chỉ khai kiểu.
 *
 * SỐ ĐIỆN THOẠI KHÔNG CÓ Ở ĐÂY, và đó là ranh giới của nền tảng chứ không phải sơ suất: `getPhoneNumber`
 * chỉ trả MÃ, đổi ra số cần máy chủ có app secret của app xã (tài liệu zmp-sdk, bước 2–3). Xin quyền số
 * điện thoại mà không dùng được là làm phiền người dân và là lý do Zalo trả hồ sơ duyệt — nên app không
 * xin, người dân tự gõ số nếu muốn xã gọi lại.
 */
export type KetQuaLayTen =
  | { readonly kieu: "xong"; readonly ho_ten: string }
  | { readonly kieu: "tu-choi" }
  | { readonly kieu: "ngoai-zalo" }
  | { readonly kieu: "khong-lay-duoc" };
export type LayTenZalo = () => Promise<KetQuaLayTen>;

/**
 * MÃ VỊ TRÍ — kết quả của nút "Lấy vị trí hiện tại" ở màn gửi phản ánh. `getLocation` của zmp-sdk CHỈ
 * trả một token (toạ độ đã `@deprecated`); đổi token ra toạ độ cần máy chủ có app secret — chưa có. Nên
 * màn hình chỉ báo "đã nhận mã", không bao giờ vẽ một điểm hay một địa chỉ đoán ra. Token KHÔNG rời
 * máy và không được giữ lại: bản trải nghiệm không có chỗ nào gửi nó đi.
 */
export type KetQuaViTri = "da-nhan-ma" | "tu-choi" | "ngoai-zalo" | "khong-lay-duoc";
export type LayMaViTri = () => Promise<KetQuaViTri>;

/** Che số điện thoại khi hiện ra (luật 3 bất biến 3): giữ 2 số đầu, 4 số cuối — cùng khuôn máy chủ. */
export function cheSoDienThoai(so: string): string {
  const chu_so = so.replace(/\D/g, "");
  if (chu_so.length < 7) return "••••";
  return `${chu_so.slice(0, 2)}****${chu_so.slice(-4)}`;
}

/** Che họ tên như máy chủ che cho người gửi: "Nguyễn Văn An" → "Nguyễn V. A.". */
export function cheHoTen(ho_ten: string): string {
  const tu = ho_ten.trim().split(/\s+/).filter(Boolean);
  if (tu.length === 0) return "";
  if (tu.length === 1) return tu[0]!;
  return [tu[0]!, ...tu.slice(1).map((t) => `${t[0]!.toUpperCase()}.`)].join(" ");
}

/** Chữ cái đầu của TÊN GỌI (từ cuối) — quy ước của bản mẫu. */
export function chuCaiDau(ho_ten: string): string {
  const cuoi = ho_ten.trim().split(/\s+/).pop() ?? "";
  return (cuoi[0] ?? "C").toUpperCase();
}

/** Lời chào theo giờ, như bản mẫu. Giờ Việt Nam do lớp gọi truyền vào — hàm không đọc đồng hồ. */
export function loiChao(gio: number): string {
  if (gio < 11) return "Chào buổi sáng";
  if (gio < 18) return "Chào buổi chiều";
  return "Chào buổi tối";
}

/* ═══════════════════════════════ PHIẾU TRẢI NGHIỆM ═══════════════════════════════ */

/**
 * THỨ TỰ VÒNG ĐỜI cho dòng thời gian — đúng bảy trạng thái của nhánh chính trong `TRANG_THAI`
 * (`noi-dung.ts`). Hai nhánh kết thúc (`khong-tiep-nhan`, `chuyen-cap-tren`) không nằm trên đường này:
 * phiếu ở hai nhánh ấy dừng sau "Đang phân loại".
 */
export const VONG_DOI: readonly string[] = [
  "da-tiep-nhan",
  "dang-phan-loai",
  "da-chuyen-xu-ly",
  "dang-xu-ly",
  "da-xu-ly",
  "cho-dan-xac-nhan",
  "da-dong",
];

/**
 * DANH MỤC LĨNH VỰC TẠM — mười hai tên của SRS kho yêu cầu (`../vigov-require/docs/SRS.md:310`; mock của prototype chỉ có 8), cho bước
 * "chọn lĩnh vực gần đúng nhất". Theo ADR 0050, lĩnh vực dân chọn LÀ lĩnh vực của phiếu và máy chủ đặt hạn
 * từ nó lúc tạo phiếu. KHÔNG KÈM SỐ GIỜ NÀO: SLA là cấu hình từng xã, và chỉ `identity` đếm hạn (luật 10 cấm
 * #2, #3). Gỡ danh sách này khi có tuyến đọc danh mục lĩnh vực của xã.
 */
/**
 * The staff-conduct field — SRS M4.3.8 + R-05: a separate route, by default only the Party Secretary and
 * the Chairman see it, NEVER public (spec 05-nghiep-vu.md:204), and each commune can switch it off. A
 * constant so the send screen can tell the citizen exactly that; once commune config exists, a commune that
 * switched it off must not show this field at all.
 */
export const STAFF_CONDUCT_FIELD = "Thái độ / tác phong cán bộ";

export const LINH_VUC_TAM: readonly string[] = [
  "Rác thải – Vệ sinh môi trường",
  "Hạ tầng giao thông",
  "Cấp thoát nước",
  "Điện",
  "Trật tự đô thị – lấn chiếm vỉa hè",
  "An ninh trật tự",
  "Xây dựng không phép",
  "Ô nhiễm (tiếng ồn, khí thải, nước thải)",
  "Y tế – Giáo dục",
  STAFF_CONDUCT_FIELD,
  "An toàn thực phẩm",
  "Khác",
];

/**
 * Đánh giá của dân sau khi xử lý (ADR 0050 điểm 2). `sau_lan_mo_lai` ghi số lần mở lại LÚC CHẤM: phiếu bị
 * mở lại rồi xử lý xong lần nữa thì được chấm lại, và lần chấm mới thay lần cũ (`service.py:813`).
 */
export type DanhGia = { readonly sao: number; readonly nhan_xet: string; readonly sau_lan_mo_lai: number };

/** Phiếu trải nghiệm = phiếu của hợp đồng thật + đánh giá (khi có) + số lần mở lại. */
export type PhieuTN = PhieuCuaToi & { readonly danh_gia: DanhGia | null; readonly so_lan_mo_lai: number };

/** Ngưỡng mở lại: chấm từ số sao này trở xuống thì phiếu mở lại (`service.py` POOR_RATING, SRS M4.3.7). */
export const SAO_MO_LAI = 2;

/**
 * Ghi đánh giá — đúng luật của kho yêu cầu (`service.py:804-840`): 1–2 sao đưa phiếu về "đang xử lý" và
 * tăng số lần mở lại; 3 sao trở lên ghi nhận, trạng thái giữ nguyên. THUẦN.
 */
export function apDanhGia(p: PhieuTN, sao: number, nhan_xet: string): PhieuTN {
  const s = Math.max(1, Math.min(5, Math.round(sao)));
  const mo_lai = s <= SAO_MO_LAI;
  return {
    ...p,
    danh_gia: { sao: s, nhan_xet: nhan_xet.trim(), sau_lan_mo_lai: p.so_lan_mo_lai },
    trang_thai: mo_lai ? "dang-xu-ly" : p.trang_thai,
    so_lan_mo_lai: p.so_lan_mo_lai + (mo_lai ? 1 : 0),
  };
}

/**
 * Được chấm khi phiếu đã xử lý xong và chưa chấm KỂ TỪ LẦN MỞ LẠI GẦN NHẤT (prototype: `canRate = status
 * === "resolved"`; máy chủ ghi đè đánh giá mỗi lần chấm).
 */
export function duocDanhGia(p: PhieuTN): boolean {
  if (nhomCua(p.trang_thai) !== "da-xu-ly-xong") return false;
  return p.danh_gia === null || p.danh_gia.sau_lan_mo_lai < p.so_lan_mo_lai;
}

/**
 * BỐN NHÓM NGƯỜI DÂN THẤY (ADR 0050 điểm 5) — bảng gộp và hai bảng nhãn nay nằm ở `status-groups.ts`, dùng
 * chung cho màn công dân của CẢ app chung lẫn app riêng (chủ dự án 28/09/2026). Ba tên cũ dưới đây giữ lại
 * cho các tệp đang nhập chúng; chúng trỏ về đúng một bảng, không phải một bản chép.
 */
export type NhomLoc = StatusFilter;

/**
 * Nhóm của app riêng — exactly `groupOf`, as in the shared app: an unknown code is `null`, NEVER a guessed
 * group. Falling back to "Đã đóng" told the citizen a ticket still open was closed; the screens now show the
 * shared neutral sentence (`nhanTrangThai`) and the ticket appears only under "Tất cả", never under a group.
 */
export function nhomCua(trang_thai: string): StatusGroup | null {
  return groupOf(trang_thai);
}

export const NHAN_NHOM = STATUS_GROUP_LABEL;

export const NHAN_BUOC = STEP_LABEL;

/**
 * Năm ô của hợp đồng thật (`TRUONG_DUOC_NHAN`) cộng lĩnh vực dân chọn (ADR 0050 — máy chủ CHƯA nhận
 * trường này). `an_danh` do công tắc "Gửi ẩn danh" của bà con đặt (SRS M4.2, ADR 0050 #3).
 */
export type NhapPhieu = {
  readonly linh_vuc: string;
  readonly noi_dung: string;
  readonly dia_chi: string;
  readonly ho_ten: string;
  readonly dien_thoai: string;
  readonly an_danh: boolean;
};

/**
 * DRAFT OF A FEEDBACK BEING WRITTEN — commune's own app ONLY (ADR 0050 #7; owner, 28/09/2026: "3 điểm còn
 * lại cũng theo require nhé", prototype `store/draft.ts`). The shell (`App.tsx`, `AppRieng` only) injects
 * it, exactly like `LayTenZalo`: this half never touches a storage API itself (`ranh-gioi-hai-nua.test.ts`
 * §3b), and the shared ViHAT app never receives one, so its "không lưu gì xuống máy" promise still holds.
 *
 * `load` returns `null` for "no draft", a malformed one, or storage that is unavailable; `save`/`clear`
 * never throw. The draft never leaves the phone.
 */
export type FeedbackDraftStore = {
  readonly load: () => NhapPhieu | null;
  readonly save: (draft: NhapPhieu) => void;
  readonly clear: () => void;
};

export type LoiNhapPhieu =Partial<Record<"noi_dung" | "dia_chi" | "ho_ten" | "dien_thoai", string>>;

const soKyTu = (s: string): number => [...s].length;

/**
 * Kiểm lúc bấm gửi, cùng giới hạn máy chủ (`DO_DAI_TOI_DA`). THUẦN.
 *
 * Bắt buộc theo SRS M4.2 (chủ dự án 28/09/2026: "theo require"): mô tả, và NGƯỜI GỬI khi không ẩn danh —
 * họ tên; gửi ẩn danh thì không. Lĩnh vực bắt buộc do bước 1 (không qua được nếu chưa chọn). Ảnh/video và
 * vị trí trên bản đồ cũng bắt buộc theo SRS, nhưng ứng dụng CHƯA có hai thứ ấy — chặn nút gửi vì chúng thì
 * bản trải nghiệm không gửi được phiếu nào; màn hình nói rõ là "bắt buộc — sắp có" (ADR 0050).
 */
export function kiemNhapPhieu(
  nhap: NhapPhieu,
  cau: { thieu: string; thieu_nguoi_gui: string; qua_dai: (toi_da: number) => string },
): LoiNhapPhieu {
  const loi: LoiNhapPhieu = {};
  if (nhap.noi_dung.trim() === "") loi.noi_dung = cau.thieu;
  else if (soKyTu(nhap.noi_dung.trim()) > DO_DAI_TOI_DA.noi_dung) loi.noi_dung = cau.qua_dai(DO_DAI_TOI_DA.noi_dung);
  if (soKyTu(nhap.dia_chi.trim()) > DO_DAI_TOI_DA.dia_chi) loi.dia_chi = cau.qua_dai(DO_DAI_TOI_DA.dia_chi);
  if (!nhap.an_danh && nhap.ho_ten.trim() === "") loi.ho_ten = cau.thieu_nguoi_gui;
  else if (!nhap.an_danh && soKyTu(nhap.ho_ten.trim()) > DO_DAI_TOI_DA.ho_ten) loi.ho_ten = cau.qua_dai(DO_DAI_TOI_DA.ho_ten);
  if (!nhap.an_danh && soKyTu(nhap.dien_thoai.trim()) > DO_DAI_TOI_DA.dien_thoai) {
    loi.dien_thoai = cau.qua_dai(DO_DAI_TOI_DA.dien_thoai);
  }
  return loi;
}

/**
 * CSPRNG bytes — same source as `api/lan-gui.ts` (`crypto.getRandomValues`). No `crypto` means THROW, never a
 * weaker fallback: a guessable lookup code is exactly what rule 4 invariant 4 and rule 13 forbid.
 */
function cryptoBytes(n: number): Uint8Array {
  const bytes = new Uint8Array(n);
  globalThis.crypto.getRandomValues(bytes);
  return bytes;
}

/**
 * Mã tra cứu trải nghiệm: tiền tố `TN-` (không lẫn với mã thật) + 8 ký tự không đoán được (luật 4 bất
 * biến 4). `randomBytes` is injectable so a test can pin the output; the default is the CSPRNG above.
 *
 * The alphabet has exactly 32 symbols, so `byte % 32` is unbiased (256 is a multiple of 32) — do not add or
 * remove a symbol without switching to rejection sampling.
 */
export function maPhieuTraiNghiem(randomBytes: (n: number) => Uint8Array = cryptoBytes): string {
  const BANG = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
  const bytes = randomBytes(8);
  let ma = "";
  for (let i = 0; i < 8; i += 1) ma += BANG[bytes[i]! % BANG.length];
  return `TN-${ma}`;
}

/**
 * Phiếu vừa gửi, đúng hình dạng máy chủ trả cho người gửi: trạng thái `da-tiep-nhan`, chưa phân loại,
 * họ tên và số điện thoại ĐÃ CHE, RỖNG khi ẩn danh (cùng lời hứa của `thanGuiPhanAnh`).
 */
export function taoPhieuTraiNghiem(nhap: NhapPhieu, luc_gui_iso: string, ma: string): PhieuTN {
  return {
    danh_gia: null,
    so_lan_mo_lai: 0,
    ma_tra_cuu: ma,
    trang_thai: "da-tiep-nhan",
    linh_vuc: nhap.linh_vuc,
    nhan_linh_vuc: nhap.linh_vuc,
    noi_dung: nhap.noi_dung.trim(),
    dia_chi: nhap.dia_chi.trim(),
    ho_ten_da_che: nhap.an_danh ? "" : cheHoTen(nhap.ho_ten),
    dien_thoai_da_che: nhap.an_danh || nhap.dien_thoai.trim() === "" ? "" : cheSoDienThoai(nhap.dien_thoai),
    an_danh: nhap.an_danh,
    goc_dem_han: luc_gui_iso,
    han_tiep_nhan: null,
    han_xu_ly_xong: null,
    ket_qua: "",
    ly_do: "",
    co_quan_nhan: "",
    // The experience keeps its rating in `danh_gia` (it needs the comment and the reopen count, which the
    // real response does not carry); these two fields of the real contract stay empty here.
    rating: null,
    rated_at: null,
  };
}

/** Tra cứu trong các phiếu của lần mở này. Không phân biệt hoa thường, bỏ khoảng trắng. */
export function traPhieuTraiNghiem(ds: readonly PhieuTN[], ma: string): PhieuTN | null {
  const q = ma.trim().toUpperCase();
  return ds.find((p) => p.ma_tra_cuu === q) ?? null;
}
