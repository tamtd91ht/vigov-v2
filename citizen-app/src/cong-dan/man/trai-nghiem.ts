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
 *   · số điện thoại: `getPhoneNumber` chỉ trả một TOKEN; đổi ra số cần app secret của app xã ở máy chủ,
 *     và app được Zalo cấp quyền số điện thoại sau khi duyệt;
 *   · "lưu xuống hệ thống": cần đường đăng nhập app riêng → phiên ViGov, CHƯA DỰNG (ADR 0047 §6).
 * Nên `LayNguoiDung` — MỘT hành động, gắn vào nút "Tiếp tục với tài khoản Zalo" — hôm nay trả người
 * dùng giả lập cố định. Ngày có quyền, lớp vỏ (`App.tsx`) tiêm hàm thật vào ĐÚNG chỗ ấy; màn hình
 * không đổi.
 *
 * PHIẾU TRẢI NGHIỆM KHÔNG ĐI VÀO HỆ THỐNG CỦA XÃ. Một phiếu mang danh tính giả mà vào sổ phản ánh thật
 * của một cơ quan nhà nước là một hồ sơ lưu trữ không ai đứng tên (luật 4, luật 6). Phiếu nằm trong
 * `useState`, mất khi đóng app, và MỌI màn hiện nó đều gắn nhãn "Bản trải nghiệm".
 *
 * Số điện thoại giả là `0900000000` — dải số giả đã thoả thuận (luật 3 bất biến 5).
 */

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

/** Che số điện thoại khi hiện ra (luật 3 bất biến 3): giữ 3 số đầu, 3 số cuối. */
export function cheSoDienThoai(so: string): string {
  const chu_so = so.replace(/\D/g, "");
  if (chu_so.length < 7) return "•••";
  return `${chu_so.slice(0, 3)} •••• ${chu_so.slice(-3)}`;
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

export type TrangThaiTraiNghiem = "moi" | "dang-xu-ly" | "da-xu-ly";

export const NHAN_TRANG_THAI_TN: Readonly<Record<TrangThaiTraiNghiem, string>> = {
  moi: "Mới tiếp nhận",
  "dang-xu-ly": "Đang xử lý",
  "da-xu-ly": "Đã xử lý",
};

export type PhieuTraiNghiem = {
  readonly ma: string;
  readonly tieu_de: string;
  readonly noi_dung: string;
  readonly dia_chi: string;
  readonly luc_gui: string;
  readonly trang_thai: TrangThaiTraiNghiem;
};

/**
 * Mã phiếu trải nghiệm: tiền tố `TN-` (không lẫn với mã phiếu thật) + 8 ký tự ngẫu nhiên không đoán
 * được (luật 4 bất biến 4). `ngau_nhien` truyền vào để test cố định được.
 */
export function maPhieuTraiNghiem(ngau_nhien: () => number = Math.random): string {
  const BANG = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
  let ma = "";
  for (let i = 0; i < 8; i += 1) ma += BANG[Math.floor(ngau_nhien() * BANG.length) % BANG.length];
  return `TN-${ma}`;
}

export type NhapPhieu = { readonly tieu_de: string; readonly noi_dung: string; readonly dia_chi: string };

export const TOI_DA_TIEU_DE = 120;
export const TOI_DA_NOI_DUNG = 1000;
export const TOI_DA_DIA_CHI = 200;
export const TOI_THIEU_TIEU_DE = 5;

export type LoiNhapPhieu = Partial<Record<keyof NhapPhieu, string>>;

/** Kiểm nội dung trước bước xác nhận. Trả lỗi theo ô; rỗng là hợp lệ. THUẦN. */
export function kiemNhapPhieu(nhap: NhapPhieu): LoiNhapPhieu {
  const loi: LoiNhapPhieu = {};
  const tieu_de = nhap.tieu_de.trim();
  if (tieu_de === "") loi.tieu_de = "Vui lòng nhập tiêu đề phản ánh.";
  else if (tieu_de.length < TOI_THIEU_TIEU_DE) loi.tieu_de = `Tiêu đề cần ít nhất ${TOI_THIEU_TIEU_DE} ký tự.`;
  if (nhap.noi_dung.trim() === "") loi.noi_dung = "Vui lòng mô tả sự việc.";
  if (nhap.dia_chi.trim() === "") loi.dia_chi = "Vui lòng nhập nơi xảy ra sự việc.";
  return loi;
}

export function taoPhieuTraiNghiem(nhap: NhapPhieu, luc_gui: string, ma: string): PhieuTraiNghiem {
  return {
    ma,
    tieu_de: nhap.tieu_de.trim(),
    noi_dung: nhap.noi_dung.trim(),
    dia_chi: nhap.dia_chi.trim(),
    luc_gui,
    trang_thai: "moi",
  };
}
