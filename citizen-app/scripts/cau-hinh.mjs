/**
 * CẤU HÌNH LÚC DỰNG — MỘT NGUỒN SỰ THẬT, DÙNG CHUNG CHO `vite.config.ts` VÀ `scripts/deploy.mjs`.
 *
 * VÌ SAO TỆP NÀY TỒN TẠI: chủ sản phẩm đẩy bản lên Zalo từ MÁY LOCAL, nên địa chỉ máy chủ phải
 * nằm trong một tệp chứ không phải gõ vào shell mỗi lần. Nhưng hai chỗ cần đọc nó — bước dựng
 * (Vite) và bước đẩy (`deploy.mjs`, chạy NGOÀI Vite) — và chép logic đọc ra hai nơi là để chúng
 * lệch nhau đúng vào ngày một bên thấy biến còn bên kia không.
 *
 * ⚠ VÌ SAO KHÔNG CHỈ `process.env`: Vite **không** tự nạp `.env.local` vào `process.env`; chỉ
 * `loadEnv()` mới đọc các tệp `.env*`. Đặt tệp xuống rồi đọc bằng `process.env.X` thì nó im
 * lặng trả `undefined`, và người dùng kết luận "tệp không được đọc" mà không biết vì sao.
 *
 * ⚠ VÀ THAM SỐ THỨ BA PHẢI LÀ `""`: mặc định `loadEnv` chỉ lấy biến có tiền tố `VITE_`. Đây là
 * chỗ thứ hai người ta vấp, và nó cũng hỏng trong im lặng.
 *
 * ⚠ BIẾN SHELL THẮNG TỆP — ĐÃ ĐO, KHÔNG PHẢI SUY ĐOÁN (20/09/2026, `loadEnv` của Vite 8):
 * đặt `VIGOV_API_HOST` ở cả `.env.local` lẫn shell thì giá trị shell được dùng. Thứ tự ấy là
 * thứ tự phải có: CI và một lần đẩy tay phải **đè được** tệp local, nếu không thì một tệp cũ
 * trên máy ai đó quyết định thứ đi lên Zalo.
 */
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { loadEnv } from "vite";

/** Gốc gói — `scripts/` nằm ngay dưới nó. */
export const GOC_GOI = fileURLToPath(new URL("..", import.meta.url));

/**
 * DANH SÁCH TRẮNG — HAI TÊN, KHÔNG HƠN. Thêm một tên vào đây là một quyết định, không phải một
 * chỉnh sửa: xem khối cảnh báo ngay dưới.
 */
export const TEN_BIEN_CHO_PHEP = ["VIGOV_BIEN_THE", "VIGOV_API_HOST"];

/** Tệp cấu hình local của bước DỰNG. `.env` là tệp KHÁC, của `zmp-cli` — xem `kiemTenBien`. */
export const TEP_LOCAL = ".env.local";

/**
 * ⚠ `.env.local` CỦA MỘT APP VITE KHÔNG PHẢI CHỖ ĐỂ BÍ MẬT, VÀ ĐÂY LÀ RÀO CHẶN ĐIỀU ĐÓ.
 *
 *   Mọi thứ đi qua `define:` được **nung thẳng vào bundle**, và bundle ấy đi lên Zalo rồi tải
 *   về máy người dùng. `__VIGOV_API_HOST__` đang đi đúng con đường đó — một địa chỉ máy chủ thì
 *   công khai được, một khoá bí mật thì không, và không có gì trong cú pháp `KEY=value` phân
 *   biệt hai thứ ấy. Một người quen `.env` phía máy chủ sẽ đặt secret key vào đây theo phản xạ
 *   (luật 8, bất biến 4 — cùng lý do với `NEXT_PUBLIC_*`).
 *
 *   Cảnh báo bằng chữ thì người ta đọc một lần. Cái rào thì nó đứng đó mãi: gặp một tên ngoài
 *   danh sách trắng, bước dựng **DỪNG** kèm câu giải thích, thay vì lặng lẽ bỏ qua.
 *
 * ⚠ CHỈ KIỂM `.env.local`, KHÔNG KIỂM `.env` — và khác biệt ấy là có chủ đích, không phải sót:
 *
 *   `citizen-app/.env` đã tồn tại từ trước và thuộc về `zmp-cli` (`APP_ID`, `ZMP_TOKEN` — token
 *   đăng nhập Zalo, một bí mật thật). Nó nằm đúng chỗ của nó: `zmp-cli` là một công cụ chạy
 *   trên máy, không phải mã đi vào bundle. Bắt nó theo danh sách trắng của bước DỰNG là làm
 *   hỏng một thiết lập đang chạy đúng.
 *
 *   Thứ đóng đường rò cho CẢ HAI tệp là `docCauHinh` bên dưới: nó **CHỌN ĐÚNG HAI KHOÁ** rồi
 *   trả về, không bao giờ trả cả môi trường. Nên `ZMP_TOKEN` không có đường nào đi vào `define`
 *   kể cả khi `loadEnv` đọc thấy nó — và có một ca kiểm ghim đúng điều đó.
 */
export function tenBienKhai(noi_dung) {
  return [...noi_dung.matchAll(/^[ \t]*(?:export[ \t]+)?([A-Za-z_][A-Za-z0-9_]*)[ \t]*=/gm)].map(
    (khop) => khop[1],
  );
}

/** Ném lỗi nếu `.env.local` khai một tên ngoài danh sách trắng. Thuần: nhận chữ, không đọc đĩa. */
export function kiemTenBien(noi_dung, duong_dan = TEP_LOCAL) {
  const la = tenBienKhai(noi_dung).filter((ten) => !TEN_BIEN_CHO_PHEP.includes(ten));
  if (la.length === 0) return;

  throw new Error(
    `${duong_dan} khai ${la.length} biến không nằm trong danh sách cho phép: ${la.join(", ")}.\n` +
      `\nTệp này chỉ nhận: ${TEN_BIEN_CHO_PHEP.join(" · ")}.\n` +
      "\nVÌ SAO DỪNG THAY VÌ BỎ QUA: mọi giá trị ở đây đi qua `define:` và được NUNG THẲNG vào " +
      "bundle gửi lên Zalo, rồi tải về máy người dùng. Một khoá bí mật đặt vào đây là một khoá " +
      "bí mật đã công bố — không thu hồi được bằng cách sửa tệp.\n" +
      "\nNếu đó là bí mật của công cụ dòng lệnh (ví dụ ZMP_TOKEN), nó thuộc về `.env`, không " +
      "phải tệp này. Nếu thật sự cần một biến dựng mới, thêm tên nó vào TEN_BIEN_CHO_PHEP " +
      "trong `scripts/cau-hinh.mjs` — và đọc lại khối chú thích ở đó trước khi thêm.",
  );
}

/**
 * Đọc cấu hình lúc dựng: `.env.local` (nếu có) + biến shell, shell thắng.
 *
 * TRẢ VỀ ĐÚNG HAI KHOÁ, KHÔNG BAO GIỜ TRẢ CẢ MÔI TRƯỜNG. `loadEnv(…, "")` gom toàn bộ
 * `process.env` — đã đo: 87 khoá trên máy dựng hôm nay. Trả nguyên đống ấy ra cho người gọi là
 * đặt sẵn một đường để `ZMP_TOKEN` hay `AWS_SECRET_ACCESS_KEY` đi vào `define` trong một lượt
 * sửa "cho tiện" nào đó về sau.
 */
export function docCauHinh(goc = GOC_GOI) {
  let noi_dung = null;
  try {
    noi_dung = readFileSync(new URL(TEP_LOCAL, `file://${goc.replace(/\\/g, "/")}`), "utf8");
  } catch {
    noi_dung = null; // không có `.env.local` là chuyện bình thường: CI chỉ dùng biến shell.
  }
  if (noi_dung !== null) kiemTenBien(noi_dung);

  // `mode` để rỗng: app này không có `.env.production` / `.env.development`, và thêm một trục
  // nữa vào một thứ đã có hai nguồn (tệp + shell) là thêm một chỗ để lệch.
  const tat_ca = loadEnv("", goc, "");

  const chon = {};
  for (const ten of TEN_BIEN_CHO_PHEP) chon[ten] = (tat_ca[ten] ?? "").trim();
  return chon;
}
