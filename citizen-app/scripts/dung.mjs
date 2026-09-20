/**
 * Dựng MỘT biến thể của bản nộp.
 *
 * | Biến thể | Nội dung | Dùng để |
 * |---|---|---|
 * | `goc` | Ứng dụng sản phẩm đầy đủ, gồm sáu tính năng dùng chín quyền nền tảng | BẢN NỘP |
 * | `day-du` | Thêm lớp khám phá + danh mục xã mẫu + bảng chẩn đoán | Thử nghiệm, demo |
 *
 * VÌ SAO PHẢI LÀ MỘT TỆP SCRIPT CHỨ KHÔNG PHẢI `VIGOV_BIEN_THE=goc vite build` TRONG package.json:
 *
 *   Trên Windows `npm run` chạy qua `cmd`, nơi cú pháp `BIEN=x lệnh` không đặt biến môi trường
 *   mà là một lỗi cú pháp. Node đặt biến giống nhau ở mọi máy. Đây cũng là lý do `deploy.mjs`
 *   tồn tại — cùng một ràng buộc, xem chú thích ở đó.
 *
 * Gọi `vite` bằng `process.execPath` chứ không qua shell: không có chuỗi nào phải trích dẫn, nên
 * không có đường dẫn nào chứa dấu cách làm hỏng lệnh.
 *
 * ⚠ CÒN MỘT BIẾN LÚC DỰNG NỮA — `VIGOV_API_HOST`, địa chỉ máy chủ của khối đăng nhập. Tệp này
 * KHÔNG chặn khi nó rỗng (dựng thử và chạy test phải được), nhưng `deploy.mjs` thì CHẶN: đẩy
 * một bản chưa khai địa chỉ là nộp một nút đăng nhập không đăng nhập nổi.
 */
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const BIEN_THE = ["goc", "day-du"];

export const MO_TA_BIEN_THE = {
  goc: "ứng dụng sản phẩm đầy đủ (quét danh thiếp · tìm văn phòng · đăng nhập bằng số Zalo, CÓ gọi máy chủ) — BẢN NỘP, không lớp khám phá, không danh mục xã, không bảng chẩn đoán",
  "day-du": "bản gốc + lớp khám phá + danh mục xã mẫu + bảng chẩn đoán",
};

/**
 * Dựng, trả về mã thoát.
 *
 * SAI TÊN BIẾN THỂ THÌ DỪNG. Một cái tên gõ nhầm mà vẫn dựng tiếp nghĩa là dựng bản ĐẦY ĐỦ rồi
 * đem nộp duyệt dưới nhãn "bản gốc" — hỏng trong im lặng, đúng chỗ đắt nhất. `vite.config.ts`
 * cũng kiểm lại một lần nữa, vì nó có thể được gọi thẳng chứ không qua tệp này.
 */
export function dung(bien_the) {
  if (!BIEN_THE.includes(bien_the)) {
    console.error(`Biến thể "${bien_the}" không có. Chỉ nhận: ${BIEN_THE.join(" · ")}`);
    return 2;
  }

  const vite = fileURLToPath(new URL("../node_modules/vite/bin/vite.js", import.meta.url));
  const goc_du_an = fileURLToPath(new URL("..", import.meta.url));

  const ket_qua = spawnSync(process.execPath, [vite, "build"], {
    cwd: goc_du_an,
    stdio: "inherit",
    env: { ...process.env, VIGOV_BIEN_THE: bien_the },
  });
  return ket_qua.status ?? 1;
}

// Chạy thẳng (`node scripts/dung.mjs goc`) thì dựng; được `deploy.mjs` nhập thì chỉ xuất hàm.
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exit(dung(process.argv[2] ?? "day-du"));
}
