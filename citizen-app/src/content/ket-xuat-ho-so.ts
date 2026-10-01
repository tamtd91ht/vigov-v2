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
import {
  BRIDGE_FIELDS_WITH_PHONE,
  COMMUNE_APP_LOCATION_FIELDS,
  COMMUNE_APP_SESSION_FIELDS,
  COMMUNE_APP_SESSION_PATH,
  DUONG_DAN_PHIEN,
  LOCATION_FIELDS,
  LOCATION_PATH,
  TRUONG_GUI_DI_CAU_VIGOV,
  TRUONG_GUI_DI_PHIEN,
} from "../features/dang-nhap/hop-dong";
import { TIEU_DE_XAC_NHAN_XA } from "../features/kham-pha/goi-y";
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
export function bangQuyen(khai: readonly KhaiBaoLoiGoi[], options: { communeApp?: boolean } = {}): string {
  // The commune app's dossier has no "Nửa" column: that app has one half, and a column saying "Cả hai"
  // to its reviewer points at a commercial half the submitted app does not contain.
  const communeApp = options.communeApp === true;
  const dong: string[] = [
    communeApp
      ? "> Khối này là **bản sinh ra** từ `KHAI_BAO_LOI_GOI` (mọi dòng không thuộc riêng nửa thương mại, theo phần `commune_app` của dòng ấy) trong"
      : `> Khối này là **bản sinh ra** từ \`KHAI_BAO_LOI_GOI\` trong`,
    `> \`citizen-app/src/features/tinh-nang/zalo-api.ts\`. Sinh lại: \`${LENH_SINH_LAI}\`.`,
    "",
    communeApp
      ? "| # | Lời gọi nền tảng | Màn | Tính năng | Zalo hỏi bạn? | Rời khỏi máy |"
      : "| # | Lời gọi nền tảng | Nửa | Màn (tab) | Tính năng | Zalo hỏi bạn? | Rời khỏi máy |",
    communeApp ? "|---|---|---|---|---|---|" : "|---|---|---|---|---|---|---|",
  ];

  khai.forEach((mot, thu_tu) => {
    const roi = mot.roi_khoi_may.trim() === "" ? "Không có gì" : oBang(mot.roi_khoi_may);
    const nua = communeApp ? "" : ` ${NHAN_NUA[mot.nua] ?? oBang(mot.nua)} |`;
    dong.push(
      `| ${thu_tu + 1} | \`${mot.api}\` |${nua} ${oBang(mot.man)} | ${oBang(mot.tinh_nang)} | ${
        mot.hoi_nguoi_dung ? "Có" : "Không"
      } | ${roi} |`,
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

/** Một đường dữ liệu rời khỏi máy: tuyến nào, tới máy chủ nào, xảy ra khi nào, và mang theo những gì. */
export type DuongRoiKhoiMay = {
  /** Tên tuyến, đúng chữ trên dây. Người duyệt đối chiếu nó với Chính sách quyền riêng tư. */
  tuyen: string;
  /**
   * Máy chủ nhận — MÔ TẢ KỸ THUẬT (hệ thống nào, dịch vụ nào), không phải lời khai pháp nhân. Có từ
   * 27/09/2026 vì ba tuyến công khai đi tới ViGov, không tới `vihat-miniapp`: một khối kể bảy tuyến mà
   * không nói tuyến nào đi đâu để người đọc tưởng cả bảy cùng một nơi nhận.
   */
  may_chu: string;
  /** Việc làm tuyến này chạy. */
  khi_nao: string;
  /**
   * `true` = chỉ chạy khi chính người dùng bấm. `false` = chạy KHÔNG cần một cú bấm — hôm nay đúng một
   * tuyến: tra tên xã ngay lúc app mở bằng mã QR của xã. Câu đầu khối đếm từ cột này, không gõ tay.
   */
  nguoi_dung_bam: boolean;
  /** Màn hình nơi việc ấy xảy ra, đúng tiêu đề trên màn. */
  man: string;
  truong: readonly TruongGuiDi[];
  /**
   * Which Mini App runs this route: the shared ViHAT Group app, the commune's own app, or both. Each app is a
   * separate App ID and a separate Zalo submission, so each dossier lists only its own routes
   * (`communeAppRoutes`).
   */
  app: "shared" | "commune" | "both";
  /**
   * The commune app's view of a `both` row — required there (`ket-xuat-ho-so.test.ts` §7). The shared row's
   * sentences say "sau khi đã xác nhận xã"; the commune app has no confirmation step, and it loads some of
   * these routes at open, without a tap.
   */
  commune_app?: { man: string; khi_nao: string; nguoi_dung_bam: boolean };
};

/**
 * Bảng "những gì rời khỏi máy", sinh từ HAI hợp đồng.
 *
 * KHÔNG ĐỌC `KHAI_BAO_LOI_GOI`: hai bảng trả lời hai câu khác nhau. Bảng kia trả lời "app gọi
 * những gì của NỀN TẢNG"; bảng này trả lời "dữ liệu của tôi đi đâu". Một lời gọi nền tảng có thể
 * không đưa gì ra khỏi máy (`scanQRCode`), và một đường dữ liệu có thể không dùng lời gọi nền tảng
 * nào (`fetch` tới tuyến yêu cầu). Gộp chúng lại là mất đúng nửa mà lượt này vừa vá.
 */
export function khoiRoiKhoiMay(duong: readonly DuongRoiKhoiMay[], options: { communeApp?: boolean } = {}): string {
  const tu_chay = duong.filter((d) => !d.nguoi_dung_bam).length;
  // ⚠ CÂU NÀY TỪNG GÕ CỨNG "không đường nào chạy lúc mở ứng dụng" — và thành SAI ngày 27/09/2026, khi
  // màn xác nhận xã tra tên xã ngay lúc app mở bằng mã QR. Nay nó đếm từ cột `nguoi_dung_bam`.
  const cau_khi_chay =
    tu_chay === 0
      ? `Cả ${duong.length} chỉ chạy khi chính người dùng bấm, và không đường nào chạy lúc mở ứng dụng.`
      : `${duong.length - tu_chay} đường chạy khi chính người dùng bấm; ${tu_chay} đường chạy mà không cần một cú bấm — xem mục "Chạy khi" của từng đường.`;
  // The commune app's header names ITS sources and says the block is NOT yet complete: "đầy đủ" printed over
  // a list that still lacks the petition routes would be the false declaration this block exists to prevent.
  const header =
    options.communeApp === true
      ? [
          "> Khối này là **bản sinh ra** từ `DUONG_ROI_KHOI_MAY` — chỉ các dòng **ứng dụng riêng của xã** chạy (cột `app`),",
          "> theo phần `commune_app` của từng dòng — trong `citizen-app/src/content/ket-xuat-ho-so.ts`, cùng",
          "> `COMMUNE_APP_SESSION_FIELDS` / `COMMUNE_APP_LOCATION_FIELDS` (`citizen-app/src/features/dang-nhap/hop-dong.ts`).",
          `> Sinh lại: \`${LENH_SINH_LAI}\`.`,
          "",
          COMMUNE_APP_ROUTES_OWED,
        ]
      : [
          "> Khối này là **bản sinh ra** từ `TRUONG_GUI_DI_PHIEN`, `TRUONG_GUI_DI_CAU_VIGOV`, `BRIDGE_FIELDS_WITH_PHONE` và `COMMUNE_APP_SESSION_FIELDS`",
          "> (`citizen-app/src/features/dang-nhap/hop-dong.ts`), `TRUONG_GUI_DI`",
          "> (`citizen-app/src/api/hop-dong-yeu-cau.ts`) và bảng ba tuyến công khai trong",
          `> \`citizen-app/src/content/ket-xuat-ho-so.ts\`. Sinh lại: \`${LENH_SINH_LAI}\`.`,
          "",
          "> **Đây là câu trả lời đầy đủ cho \"dữ liệu của tôi đi đâu\".** Phần văn xuôi của hồ sơ không",
          "> nhắc con số nào về việc này, có chủ đích: một con số gõ tay là con số sẽ sai ở lần đổi sau.",
        ];
  const dong: string[] = [
    ...header,
    "",
    `Ứng dụng có **${duong.length} đường** đưa dữ liệu ra khỏi máy. ${cau_khi_chay}`,
  ];

  duong.forEach((mot, thu_tu) => {
    dong.push(
      "",
      `### ${thu_tu + 1}. \`${mot.tuyen}\``,
      "",
      `- **Máy chủ nhận:** ${mot.may_chu}`,
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

/* ---------------------------------------------------------------------------------------------
   BA TUYẾN CÔNG KHAI CỦA ViGov (tra xã · danh bạ · tin tức) — khai ở đây, KHÔNG nhập từ hợp đồng
   ---------------------------------------------------------------------------------------------

   ⚠ BẢN CHÉP, VÀ LÝ DO LÀ MỘT RANH GIỚI, KHÔNG PHẢI SỰ TIỆN TAY. Đường dẫn và tham số của ba tuyến
   sống ở `cong-dan/api/hop-dong-cong-khai.ts` — client ViGov của nửa nhà nước. Tệp này thuộc nửa
   thương mại, và `ranh-gioi-hai-nua.test.ts` §3a cấm MỌI tệp ngoài `./cong-dan/` nhập client ấy. Nên
   chuỗi được chép, và `ket-xuat-ho-so.test.ts` (tệp test, nằm ngoài lượt quét ranh giới) khoá bản chép
   HAI CHIỀU với địa chỉ thật mà client dựng ra: mỗi tham số địa chỉ ấy mang phải có một dòng khai, mỗi
   dòng khai phải là một tham số có thật, và mỗi đường dẫn phải đúng từng chữ. Lệch một chữ là ĐỎ.

   Không tuyến nào mang `Authorization`, số điện thoại hay một mã nào của Zalo: chúng chỉ trả thứ xã
   đã công bố, theo tên miền. */

const HOST_CONG_KHAI: TruongGuiDi = {
  khoa: "host",
  trong_chinh_sach:
    "tên miền của xã — lấy từ mã QR hoặc đường liên kết đã mở ứng dụng, hoặc do phiên làm việc với xã trả về; không kèm số điện thoại, mã Zalo hay thông tin nào khác của bạn",
};

/**
 * The same `host` in the commune's own app: there the domain is fixed when the app is built
 * (`deploy.mjs --domain=… --vao-thang`), never read from a QR code or link — saying "lấy từ mã QR" to that
 * app's reviewer describes a step the app does not have.
 */
const COMMUNE_APP_HOST: TruongGuiDi = {
  khoa: "host",
  trong_chinh_sach:
    "tên miền của xã — gắn sẵn trong ứng dụng của xã từ lúc dựng; không kèm số điện thoại, mã Zalo hay thông tin nào khác của bạn",
};

/** Ba màn phản ánh nơi xã có thể cần xác nhận số điện thoại — chép, khoá như `TEN_MAN_CONG_KHAI`. */
export const PHONE_VERIFICATION_SCREENS = "Gửi phản ánh · Phản ánh của tôi · Tra cứu phiếu của tôi";

/**
 * The four personal acts of a commune's own app that open a session (`cong-dan/man/commune-session.ts`) —
 * copied from `cong-dan/man/noi-dung.ts` for the boundary reason above; the test pins each word.
 */
export const COMMUNE_APP_SESSION_SCREENS = 
  "Ứng dụng của xã: Gửi phản ánh · Phản ánh của tôi · Tra cứu phiếu của tôi · Đánh giá kết quả xử lý";

/** The send screen, where "Lấy vị trí hiện tại" sits — copied like the line above; the test pins it. */
export const SEND_SCREEN_NAME = "Gửi phản ánh";

/** Ba tên màn, chép từ `cong-dan/man/noi-dung.ts` cùng lý do ranh giới — test khoá từng chữ. */
export const TEN_MAN_CONG_KHAI = {
  danh_ba: "Danh bạ cán bộ xã",
  tin_xa: "Tin tức của xã",
  /** The commune app's home tab label (`XA_GIAO_DIEN.tab_trang_chu`), prefixed like the other commune rows. */
  trang_chu_xa: "Ứng dụng của xã: Trang chủ",
  /** The commune app's news tab header (`XA_TN.news_tab_title`), prefixed the same way. */
  commune_news_tab: "Ứng dụng của xã: Tin tức – Sự kiện",
  /** The commune app's directory screen (`ManDanhBa` titles it `DANH_BA.tieu_de`), prefixed the same way. */
  commune_directory: "Ứng dụng của xã: Danh bạ cán bộ xã",
} as const;

/**
 * The sentence the commune app's "Những gì rời khỏi máy" block carries until the petition routes are
 * declared (see the header of `DUONG_ROI_KHOI_MAY`). Printed INSIDE the generated block, so a dossier built
 * from this list cannot be pasted without the gap being on the page.
 */
export const COMMUNE_APP_ROUTES_OWED =
  "> ⚠ **CHƯA ĐỦ — CÁC TUYẾN PHẢN ÁNH CHƯA ĐƯỢC KHAI Ở ĐÂY:** gửi phản ánh, *Phản ánh của tôi*, tra cứu phiếu, đánh giá kết quả xử lý. Chúng mang nội dung phản ánh, họ tên, số điện thoại người dân tự gõ và toạ độ (nếu người dân đã lấy vị trí), tới ViGov — dịch vụ `petitions`. Khai chúng ở `citizen-app/src/content/ket-xuat-ho-so.ts` trước khi nộp.";

export const DUONG_CONG_KHAI: readonly DuongRoiKhoiMay[] = [
  {
    tuyen: "/api/v1/communes",
    may_chu: "ViGov — dịch vụ `identity`",
    khi_nao:
      "ứng dụng được mở bằng mã QR hoặc đường liên kết của một xã — chạy NGAY LÚC MỞ, trước khi người dùng bấm gì, để lấy tên và tỉnh của xã cho người dùng xác nhận",
    nguoi_dung_bam: false,
    man: TIEU_DE_XAC_NHAN_XA,
    truong: [HOST_CONG_KHAI],
    app: "both",
    // `TrangXa` looks the commune up at open, for the name in the header — the domain is the build's.
    commune_app: {
      man: TEN_MAN_CONG_KHAI.trang_chu_xa,
      khi_nao:
        "ứng dụng riêng của một xã được mở — chạy NGAY LÚC MỞ, trước khi người dùng bấm gì, để lấy tên xã hiện ở đầu màn hình",
      nguoi_dung_bam: false,
    },
  },
  {
    // 29/09/2026 (identity cdbf276): the commune's declared office — address, hotline, office hours — shown
    // on the home screen of the commune's own app. Runs at open, like the lookup above, and carries only
    // the domain: no session, no Zalo code, nothing of the citizen.
    tuyen: "/api/v1/commune-profiles",
    may_chu: "ViGov — dịch vụ `identity`",
    khi_nao:
      "ứng dụng riêng của một xã được mở — chạy NGAY LÚC MỞ, trước khi người dùng bấm gì, để hiện trụ sở, đường dây nóng và giờ làm việc mà xã đã công bố",
    nguoi_dung_bam: false,
    man: TEN_MAN_CONG_KHAI.trang_chu_xa,
    truong: [HOST_CONG_KHAI],
    app: "commune",
  },
  {
    tuyen: "/api/v1/commune-staff",
    may_chu: "ViGov — dịch vụ `identity`",
    khi_nao: `người dùng tự bấm “${TEN_MAN_CONG_KHAI.danh_ba}”, sau khi đã xác nhận xã`,
    nguoi_dung_bam: true,
    man: TEN_MAN_CONG_KHAI.danh_ba,
    truong: [HOST_CONG_KHAI],
    app: "both",
    commune_app: {
      man: TEN_MAN_CONG_KHAI.commune_directory,
      khi_nao: "người dùng tự bấm ô “Danh bạ” hoặc “Xem tất cả” của nhóm “Chính quyền số” trên trang chủ",
      nguoi_dung_bam: true,
    },
  },
  {
    tuyen: "/api/v1/commune-news",
    may_chu: "ViGov — dịch vụ `comms`",
    khi_nao: `người dùng tự bấm “${TEN_MAN_CONG_KHAI.tin_xa}” hoặc “Xem thêm tin”, sau khi đã xác nhận xã`,
    nguoi_dung_bam: true,
    man: TEN_MAN_CONG_KHAI.tin_xa,
    truong: [
      HOST_CONG_KHAI,
      {
        khoa: "cursor",
        trong_chinh_sach:
          "con trỏ trang tin do chính máy chủ trả ở trang trước, chỉ khi người dùng bấm “Xem thêm tin”",
      },
      {
        // 01/10/2026 (ADR 0067 §5): the commune app also sends `type=banner` AT OPEN, for the home picture strip —
        // the same key on the same route, so one line, saying both.
        khoa: "type",
        trong_chinh_sach:
          "loại tin người dùng chọn xem (tin tức, sự kiện, thông báo), chỉ khi người dùng bấm một nút lọc hoặc ô “Sự kiện”; trong ứng dụng riêng của xã còn là `banner` ngay lúc mở, để hiện dải ảnh xã đăng trên trang chủ",
      },
      {
        // 30/09/2026 (comms 58abea4c, card D2): the chip the citizen tapped. The id is the server's own,
        // from the categories route below — nothing typed, nothing about the citizen.
        khoa: "category",
        trong_chinh_sach:
          "mã chuyên mục tin người dùng chọn xem, do chính máy chủ trả trong danh sách chuyên mục, chỉ khi người dùng bấm một chuyên mục",
      },
    ],
    app: "both",
    // The commune app's home screen shows the latest news (`AppCuaXa` → `useTinXa`), so the list loads at open.
    commune_app: {
      man: `${TEN_MAN_CONG_KHAI.trang_chu_xa} · ${TEN_MAN_CONG_KHAI.commune_news_tab}`,
      khi_nao:
        "ứng dụng riêng của một xã được mở — chạy NGAY LÚC MỞ, trước khi người dùng bấm gì, để hiện tin mới và dải ảnh xã đăng trên trang chủ; và khi người dùng mở tab “Tin tức”, bấm một ô Truyền thanh · Video · Sự kiện, chọn một loại tin hoặc một chuyên mục, hoặc bấm “Xem thêm tin”",
      nguoi_dung_bam: false,
    },
  },
  {
    // 30/09/2026 (comms 58abea4c, card D2): the category chip rows above the news list. Runs when the
    // citizen opens the news tab or taps a type tab — a tap, like the list itself; carries only the domain
    // and the chosen type. COMMUNE APP ONLY: its one caller is `TinTucAppXa.tsx` (the shared app's
    // `TinTucXaScreen` has no chips).
    tuyen: "/api/v1/commune-news/categories",
    may_chu: "ViGov — dịch vụ `comms`",
    khi_nao: "trong ứng dụng riêng của một xã, người dùng tự mở tab “Tin tức” hoặc chọn một loại tin — để hiện các chuyên mục có tin",
    nguoi_dung_bam: true,
    man: TEN_MAN_CONG_KHAI.commune_news_tab,
    truong: [
      HOST_CONG_KHAI,
      {
        khoa: "type",
        trong_chinh_sach: "loại tin người dùng đang xem (tin tức, sự kiện, thông báo), để chỉ hiện chuyên mục có tin thuộc loại ấy",
      },
    ],
    app: "commune",
  },
  {
    tuyen: "/api/v1/commune-news/{id}",
    may_chu: "ViGov — dịch vụ `comms`",
    khi_nao: "người dùng tự bấm “Đọc tin” trên một tin",
    nguoi_dung_bam: true,
    man: TEN_MAN_CONG_KHAI.tin_xa,
    truong: [
      HOST_CONG_KHAI,
      { khoa: "id", trong_chinh_sach: "mã của tin người dùng bấm đọc, do chính máy chủ trả trong danh sách tin" },
    ],
    app: "both",
    // Articles open from the home screen, the news tab, and the Truyền thanh · Video · Sự kiện tiles. 01/10/2026
    // (ADR 0067 §4): the same route, the same two keys, is read AGAIN when a broadcast's short-lived audio link has
    // expired as the citizen taps "Nghe" (`cong-dan/man/broadcast-player.tsx`) — once per tap, so it is said here.
    commune_app: {
      man: `${TEN_MAN_CONG_KHAI.trang_chu_xa} · ${TEN_MAN_CONG_KHAI.commune_news_tab}`,
      khi_nao:
        "người dùng tự bấm vào một tin để đọc — trên trang chủ, ở tab “Tin tức”, hoặc trong các ô Truyền thanh · Video · Sự kiện; và đọc lại đúng tin ấy một lần khi người dùng bấm “Nghe” một bản tin truyền thanh mà đường dẫn âm thanh đã hết hạn, để lấy đường dẫn mới",
      nguoi_dung_bam: true,
    },
  },
];

/**
 * CÁC ĐƯỜNG DỮ LIỆU RỜI KHỎI MÁY, THEO ĐÚNG THỨ TỰ NGƯỜI DÙNG GẶP CHÚNG.
 *
 * Đăng nhập trước, vì không đăng nhập thì không gửi được yêu cầu nào — thứ tự ấy cũng là thứ tự
 * người duyệt đọc: cửa vào, rồi thứ đi qua cửa. Rồi tới lớp xã: tra tên xã, xác nhận xã, hai màn
 * công khai.
 *
 * ⚠ THÊM MỘT DÒNG Ở ĐÂY LÀ MỘT THAY ĐỔI VỀ HÀNH VI XỬ LÝ DỮ LIỆU, không phải một dòng tài liệu:
 * nó phải đi kèm một mục trong Chính sách quyền riêng tư và một lần lên số phiên bản (xem bảng
 * lên số trong `chinh-sach-rieng-tu.ts`). Năm dòng thêm ngày 27/09/2026 KHÔNG lên số: văn bản chưa
 * từng được công bố, cùng lý do khối số phiên bản ở đó nêu.
 *
 * ⚠ CHƯA KHAI Ở ĐÂY: các tuyến phản ánh của ViGov (`petitions`) — gửi phản ánh, "Phản ánh của tôi",
 * tra cứu. Chúng mang nội dung phản ánh, họ tên và số điện thoại — và từ 29/09/2026 cả toạ độ `lat`/`lng`
 * khi công dân đã bấm lấy vị trí. Việc khai chúng chưa thuộc lượt nào.
 */
export const DUONG_ROI_KHOI_MAY: readonly DuongRoiKhoiMay[] = [
  {
    tuyen: DUONG_DAN_PHIEN,
    may_chu: "`vihat-miniapp` — máy chủ của Tập đoàn ViHAT Group",
    khi_nao: "người dùng tự bấm nút đăng nhập và đồng ý chia sẻ số Zalo trên hộp thoại của Zalo",
    nguoi_dung_bam: true,
    man: "Liên hệ — khối “Đăng nhập bằng số Zalo”",
    truong: TRUONG_GUI_DI_PHIEN,
    app: "shared",
  },
  {
    tuyen: DUONG_DAN_YEU_CAU,
    may_chu: "`vihat-miniapp` — máy chủ của Tập đoàn ViHAT Group",
    khi_nao: "người dùng tự bấm “Gửi yêu cầu tư vấn” hoặc “Đề nghị gọi lại cho tôi”",
    nguoi_dung_bam: true,
    man: "Tư vấn và báo giá",
    truong: TRUONG_GUI_DI,
    app: "shared",
  },
  DUONG_CONG_KHAI[0]!,
  {
    // CÙNG TUYẾN với dòng đăng nhập, thân KHÁC: không `phoneToken`, thêm tên miền xã và cú xác nhận.
    tuyen: DUONG_DAN_PHIEN,
    may_chu:
      "`vihat-miniapp` — máy chủ của Tập đoàn ViHAT Group; máy chủ ấy chuyển tiếp sang ViGov — dịch vụ `identity` để mở phiên với xã",
    khi_nao: "người dùng tự bấm “Đúng, tiếp tục” trên màn xác nhận xã",
    nguoi_dung_bam: true,
    man: TIEU_DE_XAC_NHAN_XA,
    truong: TRUONG_GUI_DI_CAU_VIGOV,
    app: "shared",
  },
  {
    // CÙNG TUYẾN, THÂN THỨ BA (28/09/2026): mở lại phiên với xã KÈM `phoneToken`, khi ViGov đòi số điện
    // thoại đã xác thực để gửi hoặc xem phản ánh. Tên ba màn chép từ `cong-dan/man/noi-dung.ts` (tệp này
    // không được nhập nửa nhà nước — cùng lý do `TEN_MAN_CONG_KHAI`); test khoá từng chữ.
    tuyen: DUONG_DAN_PHIEN,
    may_chu:
      "`vihat-miniapp` — máy chủ của Tập đoàn ViHAT Group, không lưu số điện thoại ở lượt này; máy chủ ấy chuyển tiếp sang ViGov — dịch vụ `identity` để mở lại phiên với xã kèm số điện thoại đã xác thực",
    khi_nao:
      "xã cần xác nhận số điện thoại để gửi hoặc xem phản ánh, người dùng tự bấm “Đồng ý chia sẻ số điện thoại” và đồng ý trên hộp thoại của Zalo",
    nguoi_dung_bam: true,
    man: PHONE_VERIFICATION_SCREENS,
    truong: BRIDGE_FIELDS_WITH_PHONE,
    app: "shared",
  },
  {
    // FOURTH BODY (29/09/2026): login from a commune's OWN app — `appId` instead of a commune domain, and
    // ALWAYS `phoneToken` (the server verifies the App ID by exchanging it). Runs only after the citizen reads
    // why and taps "Đồng ý chia sẻ số điện thoại", at the first personal act — never when the app opens
    // (ADR 0047:251).
    //
    // ⚠ OWN ROUTE SINCE ADR 0066 (01/10/2026): ViGov identity `POST /api/v1/citizen-sessions`, DIRECTLY — no
    //   longer through `vihat-miniapp`. Same body, so the same field table.
    //
    // ⚠ THE PRIVACY-POLICY SENTENCE FOR `appId` IS STILL OWED — legal wording is the project owner's, same
    //   stance as `bridgeBodyWithPhone`. `chinh-sach.test.ts` pins the gap. So is the policy's statement of
    //   WHERE this login goes now (ViGov, not ViHAT's server): also the owner's wording.
    tuyen: COMMUNE_APP_SESSION_PATH,
    may_chu:
      "ViGov — dịch vụ `identity`, trực tiếp, không qua máy chủ của Tập đoàn ViHAT Group; dịch vụ ấy đổi hai mã Zalo bằng khoá bí mật của ứng dụng xã để mở phiên với xã của ứng dụng",
    khi_nao:
      "trong ứng dụng riêng của một xã, người dùng làm việc cá nhân đầu tiên (gửi, xem, tra cứu hoặc đánh giá phản ánh), đọc lời giải thích, tự bấm “Đồng ý chia sẻ số điện thoại” và đồng ý trên hộp thoại của Zalo",
    nguoi_dung_bam: true,
    man: COMMUNE_APP_SESSION_SCREENS,
    truong: COMMUNE_APP_SESSION_FIELDS,
    app: "commune",
  },
  {
    // 29/09/2026 — the location exchange (`vihat-miniapp` 0dada0f). A SECOND route to the same server,
    // run only on the citizen's tap on "Lấy vị trí hiện tại" in the send screen of either app. The screen
    // name is copied from `cong-dan/man/noi-dung.ts` `GUI.tieu_de` (this file may not import the state
    // half — same reason as `TEN_MAN_CONG_KHAI`); the test pins it word for word.
    //
    // ⚠ THE PRIVACY-POLICY SECTION THIS ROW NEEDS IS STILL OWED — legal wording is the project owner's,
    //   not written here (same stance as `bridgeBodyWithPhone`). `chinh-sach.test.ts` pins the gap.
    tuyen: LOCATION_PATH,
    may_chu: "`vihat-miniapp` — máy chủ của Tập đoàn ViHAT Group, không lưu mã vị trí lẫn toạ độ",
    khi_nao:
      "người dùng tự bấm “Lấy vị trí hiện tại” khi gửi phản ánh và đồng ý chia sẻ vị trí trên hộp thoại của Zalo",
    nguoi_dung_bam: true,
    man: SEND_SCREEN_NAME,
    truong: LOCATION_FIELDS,
    app: "shared",
  },
  {
    // SAME ROUTE, the commune's OWN app (29/09/2026, `vihat-miniapp` 4114f00): the body adds `appId` so
    // the server exchanges the token with that app's secret. Same tap, same screen, one more key.
    // ⚠ Policy sentence still owed (same stance as the row above); `chinh-sach.test.ts` pins it.
    // ⚠ Still `vihat-miniapp` on 01/10/2026 although the commune app's LOGIN moved to identity: ADR 0066
    //   decision 5 moves this exchange too, in a later card. Change this row in that card, not before.
    //   Not offered at all in the `--demo` build (`App.tsx` `AppRieng`).
    tuyen: LOCATION_PATH,
    may_chu: "`vihat-miniapp` — máy chủ của Tập đoàn ViHAT Group, không lưu mã vị trí lẫn toạ độ",
    khi_nao:
      "trong ứng dụng riêng của một xã, người dùng tự bấm “Lấy vị trí hiện tại” khi gửi phản ánh và đồng ý chia sẻ vị trí trên hộp thoại của Zalo",
    nguoi_dung_bam: true,
    man: `Ứng dụng của xã: ${SEND_SCREEN_NAME}`,
    truong: COMMUNE_APP_LOCATION_FIELDS,
    app: "commune",
  },
  {
    // 29/09/2026 (service-petitions af3fff0): the commune's field list for step 1 of "Gửi phản ánh". The
    // FIRST `petitions` route declared here (the send/read routes, which carry the petition itself, are
    // still owed — see the header of this table): it carries NO field at all — no query, no body — only the
    // session header, from which the server takes the commune. Path copied from
    // `cong-dan/api/hop-dong-phan-anh.ts` `CITIZEN_FIELDS_PATH` (boundary); the test pins it.
    // BOTH APPS since 2c158913 (01/10/2026): the shared app's `GuiPhanAnhScreen` loads it too, once a session
    // with the confirmed commune exists.
    tuyen: "/api/v1/my-citizen-report-fields",
    may_chu: "ViGov — dịch vụ `petitions`",
    khi_nao:
      "người dùng mở “Gửi phản ánh” khi đã có phiên làm việc với xã đã xác nhận, hoặc bấm “Thử lại” khi danh sách lĩnh vực chưa tải được",
    nguoi_dung_bam: true,
    man: SEND_SCREEN_NAME,
    truong: [],
    app: "both",
    commune_app: {
      man: `Ứng dụng của xã: ${SEND_SCREEN_NAME}`,
      khi_nao:
        "trong ứng dụng riêng của một xã, người dùng mở “Gửi phản ánh” sau khi đã đồng ý chia sẻ số điện thoại (phiên làm việc với xã đã mở), hoặc bấm “Thử lại” khi danh sách lĩnh vực chưa tải được",
      nguoi_dung_bam: true,
    },
  },
  ...DUONG_CONG_KHAI.slice(1),
];

/* =============================================================================================
   THE COMMUNE APP'S DOSSIER — the same declarations, seen from the app being submitted
   ============================================================================================= */

/**
 * The calls the commune's own app makes: every row that is not commercial-only, in its `commune_app` words.
 *
 * DERIVED, NOT A SECOND TABLE. A second list of calls would be a second place to forget a call; this one
 * follows `KHAI_BAO_LOI_GOI`, and a `ca-hai` row without a commune view is refused here rather than printed
 * in the shared app's words to the commune app's reviewer.
 */
export function communeAppCalls(khai: readonly KhaiBaoLoiGoi[]): KhaiBaoLoiGoi[] {
  return khai
    .filter((row) => row.nua !== "thuong-mai")
    .map((row) => {
      if (row.nua === "nha-nuoc") return row;
      if (row.commune_app === undefined) {
        throw new Error(`\`${row.api}\` is used by both halves but declares no \`commune_app\` view (zalo-api.ts).`);
      }
      return { ...row, ...row.commune_app };
    });
}

/**
 * The routes the commune's own app runs, `both` rows in their `commune_app` words. Same stance as above.
 * Every `host` field becomes `COMMUNE_APP_HOST`: in that app the domain is ALWAYS the build's, so the
 * sentence is a fact of the app, not of the row.
 */
export function communeAppRoutes(routes: readonly DuongRoiKhoiMay[]): DuongRoiKhoiMay[] {
  return routes
    .filter((row) => row.app !== "shared")
    .map((row) => {
      if (row.app === "both" && row.commune_app === undefined) {
        throw new Error(`\`${row.tuyen}\` runs in both apps but declares no \`commune_app\` view (ket-xuat-ho-so.ts).`);
      }
      const truong = row.truong.map((t) => (t.khoa === HOST_CONG_KHAI.khoa ? COMMUNE_APP_HOST : t));
      return { ...row, ...(row.app === "both" ? row.commune_app : {}), truong };
    });
}

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
