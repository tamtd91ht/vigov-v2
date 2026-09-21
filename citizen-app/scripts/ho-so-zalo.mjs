/**
 * SINH CẢ BỘ HỒ SƠ NỘP ZALO TỪ NGUỒN SỐNG — `npm run ho-so`.
 *
 * | Tệp kết quả | Sinh từ |
 * |---|---|
 * | `tmp/xin-quyen-zalo/chinh-sach-quyen-rieng-tu.txt` | `src/content/chinh-sach-rieng-tu.ts` |
 * | `tmp/xin-quyen-zalo/dieu-khoan-su-dung.txt` | `src/content/dieu-khoan.ts` |
 * | `tmp/xin-quyen-zalo/README.md`, phần giữa hai mốc | `KHAI_BAO_LOI_GOI` trong `src/features/tinh-nang/zalo-api.ts` |
 *
 * ⚠ VÌ SAO TỆP NÀY TỒN TẠI — MỘT LỖI ĐÃ ĐO ĐƯỢC, KHÔNG PHẢI MỘT Ý THÍCH:
 *
 *   Ba tài liệu ấy từng là bản CHÉP TAY của những thứ sống trong mã nguồn. Ngày 21/09/2026 bên
 *   phát hành app đổi sang Tập đoàn ViHAT Group (ADR 0031) và giao diện được dựng lại; ba tệp
 *   soạn ngày 18/09 đứng yên, và không một phép kiểm nào đỏ lên — vì không có gì đối chiếu một
 *   tệp `.txt` với mã nguồn. Đúng ba ngày là đủ để chúng nói sai (luật 9).
 *
 * ⚠ TOÀN BỘ PHẦN QUYẾT ĐỊNH NỘI DUNG NẰM Ở `src/content/ket-xuat-ho-so.ts`, KHÔNG Ở ĐÂY.
 *
 *   `tmp/` nằm trong `.gitignore`, nên một ca kiểm mở tệp kết quả ra đọc sẽ đỏ trên mọi bản sao
 *   mới của kho — đỏ vì một lý do không liên quan gì tới nội dung, và một ca đỏ vì lý do sai là
 *   một ca sắp bị tắt. Nên phần thuần (kết xuất, bảng quyền, thay khối) ở trong `src/` cùng ca
 *   kiểm của nó (`ket-xuat-ho-so.test.ts`); tệp này chỉ còn việc nạp, ghi, và báo lại.
 *
 * ⚠ NẠP `.ts` BẰNG VITE, KHÔNG THÊM PHỤ THUỘC MỚI. `vite` đã là devDependency và đã biết đọc
 *   TypeScript; `createServer({ server: { middlewareMode: true } })` + `ssrLoadModule` cho phép
 *   nhập thẳng mô-đun nguồn. `configFile: false` có chủ đích: tệp này không dựng bundle, nên nó
 *   không cần — và không nên phụ thuộc vào — biến thể bản dựng hay địa chỉ máy chủ.
 *
 * Gọi qua `process.execPath` (xem `dung.mjs`): kho này dựng trên Windows, nơi cú pháp
 * `BIEN=x lệnh` không đặt biến môi trường mà là một lỗi cú pháp.
 */

import { mkdir, readFile, writeFile } from "node:fs/promises";
import { relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const GOC_APP = fileURLToPath(new URL("..", import.meta.url));
const GOC_KHO = fileURLToPath(new URL("../..", import.meta.url));
const THU_MUC_HO_SO = fileURLToPath(new URL("../../tmp/xin-quyen-zalo/", import.meta.url));

/**
 * Dòng đầu của README hồ sơ — tệp DUY NHẤT trong ba tệp kết quả vẫn còn phần viết tay.
 *
 * Nói đúng một nửa sự thật của nó: bảng quyền là bản sinh ra, văn xuôi còn lại thì không. Viết
 * "tệp này được sinh ra" trên một tệp phần lớn viết tay là một lời cảnh báo sai, và một cảnh báo
 * sai thì lần sau không ai đọc.
 */
const DONG_BAO_README =
  "<!-- Bảng tóm tắt quyền trong tệp này là BẢN SINH RA từ mã nguồn — sinh lại: cd citizen-app && npm run ho-so. Phần văn xuôi còn lại giữ tay. -->";

/** Đường dẫn để in ra màn hình — luôn tương đối với gốc kho, không in đường dẫn máy ai. */
function duongDanNgan(tuyet_doi) {
  return relative(GOC_KHO, tuyet_doi).replace(/\\/g, "/");
}

/**
 * Đặt dòng báo ở đầu README.
 *
 * Nếu dòng đầu đã là một chú thích HTML thì THAY nó, không chèn thêm: chạy hai lần liên tiếp
 * phải cho cùng một tệp, nếu không mỗi lần sinh lại tệp dài thêm một dòng.
 */
function datDongBao(readme) {
  const dong = readme.split("\n");
  if ((dong[0] ?? "").trimStart().startsWith("<!--")) {
    dong[0] = DONG_BAO_README;
    return dong.join("\n");
  }
  return `${DONG_BAO_README}\n${readme}`;
}

async function main() {
  const may_chu = await createServer({
    configFile: false,
    root: GOC_APP,
    logLevel: "warn",
    appType: "custom",
    server: { middlewareMode: true },
    // KHÔNG QUÉT PHỤ THUỘC. Không có tệp cấu hình thì cũng không có `resolve.alias`, nên lượt
    // quét mặc định bò từ `App.tsx` vào `bien-the/kham-pha` và in ra một cảnh báo đỏ "Are they
    // installed?" — sai, và sai đúng kiểu làm người chạy tưởng bản sinh ra hỏng. Tệp này chỉ
    // nạp ba mô-đun nội dung qua `ssrLoadModule`; nó không dựng bundle nên không cần lượt quét.
    optimizeDeps: { noDiscovery: true, include: [] },
  });

  try {
    const ket_xuat = await may_chu.ssrLoadModule("/src/content/ket-xuat-ho-so.ts");
    const zalo_api = await may_chu.ssrLoadModule("/src/features/tinh-nang/zalo-api.ts");

    await mkdir(THU_MUC_HO_SO, { recursive: true });

    const da_ghi = [];

    for (const [ten_tep, van_ban] of [
      ["chinh-sach-quyen-rieng-tu.txt", ket_xuat.VAN_BAN_CHINH_SACH],
      ["dieu-khoan-su-dung.txt", ket_xuat.VAN_BAN_DIEU_KHOAN],
    ]) {
      const duong_dan = resolve(THU_MUC_HO_SO, ten_tep);
      await writeFile(duong_dan, ket_xuat.ketXuatVanBan(van_ban), "utf8");
      da_ghi.push(`${duongDanNgan(duong_dan)}  <- ${van_ban.nguon}`);
    }

    // README: đọc bản đang có, thay ĐÚNG phần giữa hai mốc, giữ nguyên văn xuôi.
    //
    // KHÔNG TỰ DỰNG LẠI README TỪ ĐẦU, và đó là một ranh giới chứ không phải một sự lười: phần
    // văn xuôi mô tả từng chức năng cho người duyệt đọc, và không một dòng nào của nó suy ra được
    // từ mã nguồn. Sinh ra nó là bịa; xoá nó là làm hồ sơ mất phần người duyệt cần nhất.
    const duong_dan_readme = resolve(THU_MUC_HO_SO, "README.md");
    const readme_cu = await readFile(duong_dan_readme, "utf8");
    const readme_moi = datDongBao(
      ket_xuat.thayKhoiSinhRa(readme_cu, ket_xuat.bangQuyen(zalo_api.KHAI_BAO_LOI_GOI)),
    );
    await writeFile(duong_dan_readme, readme_moi, "utf8");
    da_ghi.push(
      `${duongDanNgan(duong_dan_readme)}  <- citizen-app/src/features/tinh-nang/zalo-api.ts (chỉ phần giữa hai mốc)`,
    );

    console.log("Đã sinh hồ sơ nộp Zalo:");
    for (const dong of da_ghi) console.log(`  ${dong}`);
    console.log(
      "\nBa tệp trên là BẢN SINH RA. Sửa nội dung thì sửa tệp nguồn rồi chạy lại `npm run ho-so`.",
    );
  } finally {
    await may_chu.close();
  }
}

main().catch((loi) => {
  // Nói VIỆC CẦN LÀM, không in một vết ngăn xếp: lỗi hay gặp nhất ở đây là README mất mốc đánh
  // dấu, và `thayKhoiSinhRa` đã viết sẵn hai dòng phải thêm lại vào thông báo của nó.
  console.error(`Không sinh được hồ sơ nộp Zalo.\n${loi instanceof Error ? loi.message : loi}`);
  process.exit(1);
});
