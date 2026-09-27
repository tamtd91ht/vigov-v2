/**
 * BẢN TRẢI NGHIỆM CỦA APP RIÊNG MỘT XÃ — người dùng và phiếu phản ánh CHỈ SỐNG TRONG BỘ NHỚ.
 *
 * Chủ dự án, 28/09/2026: "Nếu phải xin quyền thì hãy fake tạm 1 cái tên và 1 số điện thoại… tôi muốn
 * nó phải là 1 hành động. Nếu lấy được ngay bây giờ thì trả về và lưu xuống hệ thống; nếu không lấy
 * được thì giả lập 1 thông tin cố định để trải nghiệm trước."
 *
 * HÔM NAY KHÔNG LẤY THẬT ĐƯỢC:
 *   · tên + ảnh: `getUserInfo` của zmp-sdk bật hộp xin quyền (NĐ 13/2023) — đúng điều kiện "phải xin
 *     quyền" chủ dự án đặt ra cho việc giả lập;
 *   · số điện thoại: `getPhoneNumber` chỉ trả một TOKEN; đổi ra số cần app secret của app xã ở máy chủ;
 *   · "lưu xuống hệ thống": cần đường đăng nhập app riêng → phiên ViGov, CHƯA DỰNG (ADR 0047 §6).
 * Nên `LayNguoiDung` — MỘT hành động, gắn vào nút "Tiếp tục với tài khoản Zalo" — hôm nay trả người
 * dùng giả lập cố định. Ngày có quyền, lớp vỏ (`App.tsx`) tiêm hàm thật vào ĐÚNG chỗ ấy.
 *
 * PHIẾU TRẢI NGHIỆM MANG ĐÚNG KIỂU `PhieuCuaToi` CỦA HỢP ĐỒNG THẬT (`api/hop-dong-phan-anh.ts`): năm ô
 * gửi đi (nội dung · nơi xảy ra · họ tên · điện thoại · ẩn danh), trạng thái trong bảng `TRANG_THAI`,
 * lĩnh vực do cán bộ chốt (rỗng khi mới gửi), hai mốc hạn, kết quả, lý do, cơ quan nhận. Ngày nối máy
 * chủ, màn hình giữ nguyên — chỉ nguồn dữ liệu đổi.
 *
 * PHIẾU TRẢI NGHIỆM KHÔNG ĐI VÀO HỆ THỐNG CỦA XÃ. Một phiếu mang danh tính giả mà vào sổ phản ánh thật
 * của một cơ quan nhà nước là một hồ sơ lưu trữ không ai đứng tên (luật 4, luật 6). Phiếu nằm trong
 * `useState`, mất khi đóng app, và MỌI màn hiện nó đều gắn nhãn "Bản trải nghiệm". Hai mốc hạn để
 * `null`: hạn đếm bằng GIỜ LÀM VIỆC theo lịch từng xã, chỉ `identity` đếm được (ADR 0007) — bản trải
 * nghiệm không bịa một ngày.
 *
 * Số điện thoại giả là `0900000000` — dải số giả đã thoả thuận (luật 3 bất biến 5).
 */
import { DO_DAI_TOI_DA, type PhieuCuaToi } from "../api/hop-dong-phan-anh";

export type NguoiDungApp = {
  readonly ho_ten: string;
  readonly so_dien_thoai: string;
  /** `gia-lap`: thông tin mẫu cố định. `zalo`: lấy thật từ Zalo (chưa có đường nào trả giá trị này). */
  readonly nguon: "gia-lap" | "zalo";
};

/** Hành động DUY NHẤT lấy người dùng. Hôm nay: `layNguoiDungGiaLap`. */
export type LayNguoiDung = () => Promise<NguoiDungApp>;

export const NGUOI_DUNG_GIA_LAP: NguoiDungApp = {
  ho_ten: "Nguyễn Văn An",
  so_dien_thoai: "0900000000",
  nguon: "gia-lap",
};

export const layNguoiDungGiaLap: LayNguoiDung = async () => NGUOI_DUNG_GIA_LAP;

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

/** Nhóm lọc ở danh sách — theo việc người dân muốn biết, không theo mã trạng thái. */
export type NhomLoc = "tat-ca" | "dang-cho" | "dang-xu-ly" | "da-xong";

export function nhomCua(trang_thai: string): Exclude<NhomLoc, "tat-ca"> {
  if (trang_thai === "da-tiep-nhan" || trang_thai === "dang-phan-loai") return "dang-cho";
  if (trang_thai === "da-chuyen-xu-ly" || trang_thai === "dang-xu-ly") return "dang-xu-ly";
  return "da-xong";
}

/** Năm ô người dân gõ — ĐÚNG năm trường máy chủ nhận (`TRUONG_DUOC_NHAN`). */
export type NhapPhieu = {
  readonly noi_dung: string;
  readonly dia_chi: string;
  readonly ho_ten: string;
  readonly dien_thoai: string;
  readonly an_danh: boolean;
};

export type LoiNhapPhieu = Partial<Record<"noi_dung" | "dia_chi" | "ho_ten" | "dien_thoai", string>>;

const soKyTu = (s: string): number => [...s].length;

/** Kiểm trước bước xác nhận, cùng giới hạn máy chủ (`DO_DAI_TOI_DA`). Rỗng là hợp lệ. THUẦN. */
export function kiemNhapPhieu(nhap: NhapPhieu, cau: { thieu: string; qua_dai: (toi_da: number) => string }): LoiNhapPhieu {
  const loi: LoiNhapPhieu = {};
  if (nhap.noi_dung.trim() === "") loi.noi_dung = cau.thieu;
  else if (soKyTu(nhap.noi_dung.trim()) > DO_DAI_TOI_DA.noi_dung) loi.noi_dung = cau.qua_dai(DO_DAI_TOI_DA.noi_dung);
  if (soKyTu(nhap.dia_chi.trim()) > DO_DAI_TOI_DA.dia_chi) loi.dia_chi = cau.qua_dai(DO_DAI_TOI_DA.dia_chi);
  if (!nhap.an_danh && soKyTu(nhap.ho_ten.trim()) > DO_DAI_TOI_DA.ho_ten) loi.ho_ten = cau.qua_dai(DO_DAI_TOI_DA.ho_ten);
  if (!nhap.an_danh && soKyTu(nhap.dien_thoai.trim()) > DO_DAI_TOI_DA.dien_thoai) {
    loi.dien_thoai = cau.qua_dai(DO_DAI_TOI_DA.dien_thoai);
  }
  return loi;
}

/**
 * Mã tra cứu trải nghiệm: tiền tố `TN-` (không lẫn với mã thật) + 8 ký tự không đoán được (luật 4 bất
 * biến 4). `ngau_nhien` truyền vào để test cố định được.
 */
export function maPhieuTraiNghiem(ngau_nhien: () => number = Math.random): string {
  const BANG = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
  let ma = "";
  for (let i = 0; i < 8; i += 1) ma += BANG[Math.floor(ngau_nhien() * BANG.length) % BANG.length];
  return `TN-${ma}`;
}

/**
 * Phiếu vừa gửi, đúng hình dạng máy chủ trả cho người gửi: trạng thái `da-tiep-nhan`, chưa phân loại,
 * họ tên và số điện thoại ĐÃ CHE, RỖNG khi ẩn danh (cùng lời hứa của `thanGuiPhanAnh`).
 */
export function taoPhieuTraiNghiem(nhap: NhapPhieu, luc_gui_iso: string, ma: string): PhieuCuaToi {
  return {
    ma_tra_cuu: ma,
    trang_thai: "da-tiep-nhan",
    linh_vuc: "",
    nhan_linh_vuc: "",
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
  };
}

/** Tra cứu trong các phiếu của lần mở này. Không phân biệt hoa thường, bỏ khoảng trắng. */
export function traPhieuTraiNghiem(ds: readonly PhieuCuaToi[], ma: string): PhieuCuaToi | null {
  const q = ma.trim().toUpperCase();
  return ds.find((p) => p.ma_tra_cuu === q) ?? null;
}
