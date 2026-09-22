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

import {
  DUONG_DAN_YEU_CAU,
  TRUONG_GUI_DI,
  type TruongGuiDi,
} from "../api/hop-dong-yeu-cau";
import { DUONG_DAN_PHIEN, TRUONG_GUI_DI_PHIEN } from "../features/dang-nhap/hop-dong";
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
 *
 * HAI MỐC NHẬN QUA THAM SỐ TỪ 22/09/2026 — README nay có HAI khối sinh ra, và hàm này thay từng
 * khối một. Giá trị mặc định giữ nguyên cặp mốc của bảng quyền, nên mọi nơi gọi cũ không phải đổi.
 */
export function thayKhoiSinhRa(
  readme: string,
  khoi: string,
  moc_bat_dau: string = MOC_BAT_DAU,
  moc_ket_thuc: string = MOC_KET_THUC,
): string {
  const dau = readme.indexOf(moc_bat_dau);
  const cuoi = readme.indexOf(moc_ket_thuc);
  if (dau === -1 || cuoi === -1 || cuoi < dau) {
    throw new Error(
      `README của hồ sơ không còn đủ hai mốc đánh dấu.\nThêm lại hai dòng này vào đúng chỗ khối sinh ra:\n${moc_bat_dau}\n${moc_ket_thuc}`,
    );
  }
  const truoc = readme.slice(0, dau + moc_bat_dau.length);
  const sau = readme.slice(cuoi);
  return `${truoc}\n\n${khoi}\n\n${sau}`;
}

/* =============================================================================================
   KHỐI "NHỮNG GÌ RỜI KHỎI MÁY" — KHỐI SINH RA THỨ HAI CỦA README HỒ SƠ
   ============================================================================================= */

/**
 * ⚠ KHỐI NÀY RA ĐỜI 22/09/2026 ĐỂ VÁ MỘT LỖ ĐÃ ĐỂ LỌT MỘT LỜI KHAI SAI VÀO HỒ SƠ PHÁP LÝ.
 *
 *   `bangQuyen()` sinh từ `KHAI_BAO_LOI_GOI`, và bảng ấy **chỉ biết lời gọi `zmp-sdk`**. Một lời
 *   gọi mạng thuần (`fetch`) hoàn toàn vô hình với nó. Đó là lý do hai câu trong phần văn xuôi —
 *   *"Đây là chức năng DUY NHẤT của ứng dụng có dữ liệu rời khỏi máy"* và *"Đúng MỘT chức năng có
 *   dữ liệu rời khỏi máy: đăng nhập"* — vẫn đứng nguyên sau khi giai đoạn B mở một tuyến thứ hai
 *   gửi đi chữ người dùng TỰ GÕ. Không một phép kiểm nào đỏ lên, vì không phép kiểm nào biết nhìn
 *   vào đâu.
 *
 *   Nên câu trả lời cho "thứ gì rời khỏi máy" nay được SINH RA từ hai hợp đồng, và phần văn xuôi
 *   không được nhắc một con số nào nữa — nó trỏ sang khối này. Một con số gõ tay trong văn xuôi là
 *   con số sẽ sai lần thứ hai.
 */
export const MOC_BAT_DAU_ROI_MAY = "<!-- BẮT ĐẦU KHỐI RỜI KHỎI MÁY — đừng sửa tay trong khối này -->";
export const MOC_KET_THUC_ROI_MAY = "<!-- KẾT THÚC KHỐI RỜI KHỎI MÁY -->";

/** Một đường dữ liệu rời khỏi máy: tuyến nào, xảy ra khi nào, và mang theo những gì. */
export type DuongRoiKhoiMay = {
  /** Tên tuyến, đúng chữ trên dây. Người duyệt đối chiếu nó với Chính sách quyền riêng tư. */
  tuyen: string;
  /** Hành động của NGƯỜI DÙNG làm tuyến này chạy. Không tuyến nào tự chạy lúc mở app. */
  khi_nao: string;
  /** Màn hình nơi hành động ấy xảy ra, đúng tiêu đề trên màn. */
  man: string;
  truong: readonly TruongGuiDi[];
};

/**
 * Bảng "những gì rời khỏi máy", sinh từ HAI hợp đồng.
 *
 * KHÔNG ĐỌC `KHAI_BAO_LOI_GOI`: hai bảng trả lời hai câu khác nhau. Bảng kia trả lời "app gọi
 * những gì của NỀN TẢNG"; bảng này trả lời "dữ liệu của tôi đi đâu". Một lời gọi nền tảng có thể
 * không đưa gì ra khỏi máy (`scanQRCode`), và một đường dữ liệu có thể không dùng lời gọi nền tảng
 * nào (`fetch` tới tuyến yêu cầu). Gộp chúng lại là mất đúng nửa mà lượt này vừa vá.
 */
export function khoiRoiKhoiMay(duong: readonly DuongRoiKhoiMay[]): string {
  const dong: string[] = [
    "> Khối này là **bản sinh ra** từ `TRUONG_GUI_DI_PHIEN`",
    "> (`citizen-app/src/features/dang-nhap/hop-dong.ts`) và `TRUONG_GUI_DI`",
    `> (\`citizen-app/src/api/hop-dong-yeu-cau.ts\`). Sinh lại: \`${LENH_SINH_LAI}\`.`,
    "",
    "> **Đây là câu trả lời đầy đủ cho \"dữ liệu của tôi đi đâu\".** Phần văn xuôi của hồ sơ không",
    "> nhắc con số nào về việc này, có chủ đích: một con số gõ tay là con số sẽ sai ở lần đổi sau.",
    "",
    `Ứng dụng có **${duong.length} đường** đưa dữ liệu ra khỏi máy. Cả ${duong.length} chỉ chạy khi chính người dùng bấm, và không đường nào chạy lúc mở ứng dụng.`,
  ];

  duong.forEach((mot, thu_tu) => {
    dong.push(
      "",
      `### ${thu_tu + 1}. \`${mot.tuyen}\``,
      "",
      `- **Chạy khi:** ${mot.khi_nao}`,
      `- **Màn hình:** ${mot.man}`,
      `- **Mang theo ${mot.truong.length} thứ, và không gì khác:**`,
    );
    for (const t of mot.truong) {
      dong.push(`  - \`${t.khoa}\` — ${t.trong_chinh_sach}`);
    }
  });

  dong.push(
    "",
    "Không đường nào ở trên mang theo một trường nói \"tôi là ai\": máy chủ lấy người dùng từ phiên",
    "đăng nhập trong tiêu đề `Authorization`, không từ thân yêu cầu.",
  );

  return dong.join("\n");
}

/**
 * HAI ĐƯỜNG DỮ LIỆU RỜI KHỎI MÁY, THEO ĐÚNG THỨ TỰ NGƯỜI DÙNG GẶP CHÚNG.
 *
 * Đăng nhập trước, vì không đăng nhập thì không gửi được yêu cầu nào — thứ tự ấy cũng là thứ tự
 * người duyệt đọc: cửa vào, rồi thứ đi qua cửa.
 *
 * ⚠ THÊM MỘT DÒNG Ở ĐÂY LÀ MỘT THAY ĐỔI VỀ HÀNH VI XỬ LÝ DỮ LIỆU, không phải một dòng tài liệu:
 * nó phải đi kèm một mục trong Chính sách quyền riêng tư và một lần lên số phiên bản (xem bảng
 * lên số trong `chinh-sach-rieng-tu.ts`).
 */
export const DUONG_ROI_KHOI_MAY: readonly DuongRoiKhoiMay[] = [
  {
    tuyen: DUONG_DAN_PHIEN,
    khi_nao: "người dùng tự bấm nút đăng nhập và đồng ý chia sẻ số Zalo trên hộp thoại của Zalo",
    man: "Liên hệ — khối “Đăng nhập bằng số Zalo”",
    truong: TRUONG_GUI_DI_PHIEN,
  },
  {
    tuyen: DUONG_DAN_YEU_CAU,
    khi_nao: "người dùng tự bấm “Gửi yêu cầu tư vấn” hoặc “Đề nghị gọi lại cho tôi”",
    man: "Tư vấn và báo giá",
    truong: TRUONG_GUI_DI,
  },
];

/* =============================================================================================
   RÀO CHẶN CHO PHẦN VĂN XUÔI GIỮ TAY
   ============================================================================================= */

/**
 * ⚠ PHẦN VĂN XUÔI CỦA HỒ SƠ LÀ NỬA KHÔNG MỘT PHÉP KIỂM NÀO NHÌN THẤY — VÀ ĐÓ LÀ NƠI CÂU SAI ĐÃ
 * SỐNG SÓT.
 *
 *   `tmp/` nằm trong `.gitignore`, nên phần văn xuôi ấy **không nằm trong kho**: không diff, không
 *   review, không test. Một ca kiểm mở tệp trên đĩa ra đọc sẽ đỏ trên mọi bản sao mới (xem khối
 *   đầu tệp này), nên cách ấy không dùng được.
 *
 *   Thứ CHẠY ĐƯỢC ở đúng chỗ văn xuôi sống là `npm run ho-so`: nó đọc README thật, mỗi lần sinh
 *   lại. Nên hàm này là một phép quét THUẦN (kiểm được ở đây, trên chuỗi), và `scripts/ho-so-zalo.mjs`
 *   gọi nó rồi DỪNG với mã thoát khác 0 nếu có cảnh báo. Một hồ sơ không sinh được là một hồ sơ
 *   không ai nộp nhầm.
 *
 * ⚠ CHỈ QUÉT PHẦN NGOÀI CÁC KHỐI SINH RA. Khối sinh ra có quyền nói "2 đường", và bắt chính nó là
 * làm rào này kêu oan ngay lần đầu — một rào kêu oan là một rào sắp bị tắt.
 */
export type CanhBaoVanXuoi = { cau: string; vi_sao: string };

const RAO_VAN_XUOI: readonly { mau: RegExp; vi_sao: string }[] = [
  {
    mau: /duy nhất[^.\n]{0,80}rời khỏi máy/gi,
    vi_sao:
      'Văn xuôi lại khẳng định có một đường dữ liệu DUY NHẤT. Câu đúng hình dạng này đã SAI một lần (giai đoạn B mở tuyến thứ hai). Bỏ câu ấy đi và trỏ người đọc sang khối "Những gì rời khỏi máy".',
  },
  {
    mau: /(đúng|chỉ)\s+(một|1|hai|2)\s+(chức năng|thứ|mã|việc|đường)[^.\n]{0,80}(rời khỏi máy|gửi đi)/gi,
    vi_sao:
      'Văn xuôi đếm bằng tay số thứ rời khỏi máy. Con số ấy sống trong hai hợp đồng và được SINH RA; gõ lại nó ở đây là dựng chỗ thứ hai sẽ lệch.',
  },
  {
    mau: /gửi\s+(đúng\s+)?(một|hai|ba|bốn|\d+)\s+(thứ|mã|trường)/gi,
    vi_sao:
      'Văn xuôi đếm bằng tay số trường gửi đi. Danh sách ấy được SINH RA từ `thanYeuCau` của từng tuyến — trỏ sang khối sinh ra thay vì chép số.',
  },
];

/** Bỏ mọi khối sinh ra khỏi chuỗi, để chỉ còn phần văn xuôi giữ tay. */
function chiVanXuoi(readme: string): string {
  let con_lai = readme;
  for (const [dau, cuoi] of [
    [MOC_BAT_DAU, MOC_KET_THUC],
    [MOC_BAT_DAU_ROI_MAY, MOC_KET_THUC_ROI_MAY],
  ] as const) {
    const i = con_lai.indexOf(dau);
    const j = con_lai.indexOf(cuoi);
    if (i !== -1 && j !== -1 && j > i) {
      con_lai = con_lai.slice(0, i) + con_lai.slice(j + cuoi.length);
    }
  }
  return con_lai;
}

/**
 * Quét phần văn xuôi giữ tay, trả về những câu phải sửa. Mảng rỗng nghĩa là sạch.
 *
 * TRẢ VỀ CÂU TÌM ĐƯỢC, KHÔNG CHỈ TRẢ "CÓ LỖI": người chạy lệnh phải thấy đúng chuỗi phải xoá, ở
 * một tệp 260 dòng mà họ không viết.
 */
export function canhBaoVanXuoi(readme: string): CanhBaoVanXuoi[] {
  const van_xuoi = chiVanXuoi(readme);
  const ra: CanhBaoVanXuoi[] = [];
  for (const rao of RAO_VAN_XUOI) {
    for (const khop of van_xuoi.matchAll(rao.mau)) {
      ra.push({ cau: khop[0].replace(/\s+/g, " ").trim(), vi_sao: rao.vi_sao });
    }
  }
  return ra;
}
