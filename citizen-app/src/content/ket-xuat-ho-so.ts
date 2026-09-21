/**
 * KẾT XUẤT HỒ SƠ NỘP ZALO — PHẦN THUẦN. Không đọc đĩa, không ghi đĩa, không biết `tmp/` tồn tại.
 *
 * VÌ SAO TÁCH KHỎI `scripts/ho-so-zalo.mjs`:
 *
 *   Thư mục kết quả (`tmp/xin-quyen-zalo/`) nằm trong `.gitignore`. Một ca kiểm mở tệp trên đĩa
 *   ra đọc sẽ ĐỎ trên mọi bản sao mới của kho — và cách người ta chữa một ca đỏ vì lý do sai là
 *   tắt nó đi. Nên ca kiểm đứng ở đây, trên chuỗi văn bản, nơi nó xanh hay đỏ vì đúng thứ nó
 *   canh: nội dung có đủ không. Script chỉ còn việc ghi chuỗi ấy xuống tệp.
 *
 * ⚠ MỘT TỆP SINH RA MÀ TRÔNG NHƯ TỆP VIẾT TAY LÀ TỆP SẼ CÓ NGƯỜI SỬA TAY — và bản sửa tay ấy
 * biến mất không báo trước ở lần sinh lại. Đó là lý do mọi thứ hàm này in ra đều mở đầu bằng
 * `banSinhRa()`, kèm lệnh sinh lại, kèm câu dặn bỏ dòng ấy đi khi dán vào hồ sơ Zalo.
 */

import type { KhaiBaoLoiGoi } from "../features/tinh-nang/zalo-api";

import {
  CAU_DAU,
  MUC_CHINH_SACH,
  NGAY_HIEU_LUC,
  PHIEN_BAN_CHINH_SACH,
  TIEU_DE_CHINH_SACH,
  type MucChinhSach,
} from "./chinh-sach-rieng-tu";
import { COMPANY } from "./company-profile";
import {
  CAU_DAU_DIEU_KHOAN,
  MUC_DIEU_KHOAN,
  NGAY_HIEU_LUC_DIEU_KHOAN,
  PHIEN_BAN_DIEU_KHOAN,
  TIEU_DE_DIEU_KHOAN,
} from "./dieu-khoan";

/** Một văn bản pháp lý của ứng dụng, đủ để in ra bản nộp. Hai văn bản, một hình dạng. */
export type VanBanPhapLy = {
  tieu_de: string;
  phien_ban: string;
  ngay_hieu_luc: string;
  cau_dau: string;
  muc: readonly MucChinhSach[];
  /** Đường dẫn tệp NGUỒN, in vào dòng đầu để người mở tệp biết sửa ở đâu. */
  nguon: string;
};

/** Lệnh sinh lại — một chuỗi, để ba chỗ in ra không bao giờ nói ba lệnh khác nhau. */
export const LENH_SINH_LAI = "cd citizen-app && npm run ho-so";

/**
 * Dòng đầu của MỌI tệp kết quả.
 *
 * Vế "khi dán vào hồ sơ Zalo thì bỏ dòng này" là bắt buộc, không phải lịch sự: hai tệp `.txt`
 * được dán thẳng vào ô giải trình trên Developer Console, và một dòng kỹ thuật lọt vào đầu một
 * chính sách quyền riêng tư là thứ người duyệt đọc trước tiên.
 */
export function banSinhRa(nguon: string): string {
  return `[BẢN SINH RA TỰ ĐỘNG — ĐỪNG SỬA TỆP NÀY. Nguồn: ${nguon} · Sinh lại: ${LENH_SINH_LAI} · Khi dán vào hồ sơ Zalo thì BỎ dòng này.]`;
}

/**
 * In một văn bản pháp lý ra chữ thuần.
 *
 * ĐÁNH SỐ MỤC Ở ĐÂY, KHÔNG Ở TỆP NỘI DUNG — cùng lý do `MUC_CHINH_SACH` nêu: một con số viết
 * cứng trong tiêu đề lệch ngay lần thêm hoặc bớt mục kế tiếp, và lệch trong một văn bản pháp lý
 * thì không có gì đỏ lên.
 */
export function ketXuatVanBan(vb: VanBanPhapLy): string {
  const dong: string[] = [
    banSinhRa(vb.nguon),
    "",
    vb.tieu_de.toUpperCase(),
    `Mini App ${COMPANY.name}`,
    `Phiên bản ${vb.phien_ban} — Hiệu lực từ ngày ${vb.ngay_hieu_luc}`,
    "",
    vb.cau_dau,
  ];

  vb.muc.forEach((muc, thu_tu) => {
    dong.push("", `${thu_tu + 1}. ${muc.tieu_de.toUpperCase()}`, "");
    dong.push(muc.doan.join("\n\n"));
  });

  return `${dong.join("\n")}\n`;
}

export const VAN_BAN_CHINH_SACH: VanBanPhapLy = {
  tieu_de: TIEU_DE_CHINH_SACH,
  phien_ban: PHIEN_BAN_CHINH_SACH,
  ngay_hieu_luc: NGAY_HIEU_LUC,
  cau_dau: CAU_DAU,
  muc: MUC_CHINH_SACH,
  nguon: "citizen-app/src/content/chinh-sach-rieng-tu.ts",
};

export const VAN_BAN_DIEU_KHOAN: VanBanPhapLy = {
  tieu_de: TIEU_DE_DIEU_KHOAN,
  phien_ban: PHIEN_BAN_DIEU_KHOAN,
  ngay_hieu_luc: NGAY_HIEU_LUC_DIEU_KHOAN,
  cau_dau: CAU_DAU_DIEU_KHOAN,
  muc: MUC_DIEU_KHOAN,
  nguon: "citizen-app/src/content/dieu-khoan.ts",
};

/* =============================================================================================
   BẢNG TÓM TẮT QUYỀN TRONG README CỦA HỒ SƠ
   ============================================================================================= */

/**
 * HAI MỐC ĐÁNH DẤU, VÀ CHÚNG PHẢI LÀ CHÚ THÍCH HTML: README ấy được đọc như Markdown, nên hai
 * dòng này không hiện ra cho người đọc, nhưng vẫn nhìn thấy được trong bản thô của người sửa.
 *
 * Phần NGOÀI hai mốc là văn xuôi giữ tay — mô tả từng chức năng, ảnh chụp, việc còn phải làm.
 * Phần TRONG hai mốc đọc từ `KHAI_BAO_LOI_GOI`, tức từ chính mã nguồn gọi nền tảng.
 */
export const MOC_BAT_DAU = "<!-- BẮT ĐẦU BẢNG SINH RA — đừng sửa tay trong khối này -->";
export const MOC_KET_THUC = "<!-- KẾT THÚC BẢNG SINH RA -->";

/** `|` trong một ô sẽ cắt đôi cột. Chưa câu khai nào có, nhưng câu sau thì không ai kiểm. */
function oBang(chu: string): string {
  return chu.replace(/\|/g, "\\|");
}

const NHAN_NUA: Readonly<Record<string, string>> = {
  "thuong-mai": "Thương mại",
  "nha-nuoc": "Nhà nước",
  "ca-hai": "Cả hai",
};

/**
 * Bảng tóm tắt + danh sách mục đích, sinh từ `KHAI_BAO_LOI_GOI`.
 *
 * ⚠ BẢNG NÀY KHÔNG CÓ CỘT "TÊN QUYỀN TRÊN CONSOLE" VÀ KHÔNG CÓ CỘT "ẢNH", CÓ CHỦ ĐÍCH: cả hai
 * đều là sự thật về HỒ SƠ (Developer Console đặt tên quyền, thư mục `anh/` chứa ảnh), không phải
 * sự thật về mã nguồn. Sinh ra một cột mà nguồn của nó không nằm trong mã là bịa, và bịa đúng
 * vào chỗ người duyệt đối chiếu. Hai thứ ấy ở lại phần văn xuôi giữ tay.
 */
export function bangQuyen(khai: readonly KhaiBaoLoiGoi[]): string {
  const dong: string[] = [
    `> Khối này là **bản sinh ra** từ \`KHAI_BAO_LOI_GOI\` trong`,
    `> \`citizen-app/src/features/tinh-nang/zalo-api.ts\`. Sinh lại: \`${LENH_SINH_LAI}\`.`,
    "",
    "| # | Lời gọi nền tảng | Nửa | Màn (tab) | Tính năng | Zalo hỏi bạn? | Rời khỏi máy |",
    "|---|---|---|---|---|---|---|",
  ];

  khai.forEach((mot, thu_tu) => {
    const roi = mot.roi_khoi_may.trim() === "" ? "Không có gì" : oBang(mot.roi_khoi_may);
    dong.push(
      `| ${thu_tu + 1} | \`${mot.api}\` | ${NHAN_NUA[mot.nua] ?? oBang(mot.nua)} | ${oBang(
        mot.man,
      )} | ${oBang(mot.tinh_nang)} | ${mot.hoi_nguoi_dung ? "Có" : "Không"} | ${roi} |`,
    );
  });

  dong.push("", "**Mục đích từng lời gọi** — câu này là câu chính ứng dụng nói với người dùng:", "");
  for (const mot of khai) {
    dong.push(`- \`${mot.api}\` — ${mot.de_lam_gi}`);
  }

  return dong.join("\n");
}

/**
 * Thay phần giữa hai mốc, giữ nguyên mọi thứ ngoài chúng.
 *
 * ⚠ THIẾU MỐC THÌ NÉM LỖI, KHÔNG PHỤ THÊM VÀO CUỐI TỆP. Một README mất mốc mà vẫn được "sinh
 * tiếp" sẽ có hai bảng quyền: bảng cũ chép tay ở trên, bảng mới ở dưới, và người duyệt đọc phải
 * bảng nào là chuyện may rủi. Dừng lại và nói ra thì người sửa README biết mình vừa xoá cái gì.
 */
export function thayKhoiSinhRa(readme: string, khoi: string): string {
  const dau = readme.indexOf(MOC_BAT_DAU);
  const cuoi = readme.indexOf(MOC_KET_THUC);
  if (dau === -1 || cuoi === -1 || cuoi < dau) {
    throw new Error(
      `README của hồ sơ không còn đủ hai mốc đánh dấu.\nThêm lại hai dòng này vào đúng chỗ bảng quyền:\n${MOC_BAT_DAU}\n${MOC_KET_THUC}`,
    );
  }
  const truoc = readme.slice(0, dau + MOC_BAT_DAU.length);
  const sau = readme.slice(cuoi);
  return `${truoc}\n\n${khoi}\n\n${sau}`;
}
