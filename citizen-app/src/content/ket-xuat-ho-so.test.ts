import { describe, expect, it } from "vitest";

import { thanYeuCau, TRUONG_GUI_DI } from "../api/hop-dong-yeu-cau";
// ⚠ Tệp TEST nhập client ViGov — được, vì lượt quét ranh giới (`ranh-gioi-hai-nua.test.ts`) bỏ mọi
// tệp `.test.`. Tệp SẢN XUẤT `ket-xuat-ho-so.ts` thì không được, nên nó chép, và ca §5 khoá bản chép.
import {
  COMMUNE_PROFILES_PATH,
  communeProfilesAddress,
  diaChiBaiTin,
  diaChiDanhBa,
  diaChiTinXa,
  diaChiTraXa,
  DUONG_DAN_DANH_BA,
  DUONG_DAN_TIN_XA,
  DUONG_DAN_XA,
  NEWS_CATEGORIES_PATH,
  newsCategoriesAddress,
} from "../cong-dan/api/hop-dong-cong-khai";
import { diaChiViGov } from "../cong-dan/api/dia-chi-vigov";
import {
  CITIZEN_FIELDS_PATH,
  citizenFieldsAddress,
  PHOTO_UPLOAD_FIELDS,
  photoCompletionAddress,
  photosAddress,
  photoUploadBody,
  STORAGE_FILE_FIELD,
} from "../cong-dan/api/hop-dong-phan-anh";
import { CUA_TOI, DANH_BA, GUI, KHAN_CAP, RATING, TIN_XA, TRA_CUU, XA_GIAO_DIEN, XA_TN } from "../cong-dan/man/noi-dung";
import { TIEU_DE_XAC_NHAN_XA } from "../features/kham-pha/goi-y";
import {
  BRIDGE_FIELDS_WITH_PHONE,
  bridgeBodyWithPhone,
  COMMUNE_APP_LOCATION_FIELDS,
  COMMUNE_APP_SESSION_FIELDS,
  COMMUNE_APP_SESSION_PATH,
  communeAppSessionAddress,
  communeAppSessionBody,
  LOCATION_FIELDS,
  LOCATION_PATH,
  locationBody,
  thanYeuCau as thanYeuCauPhien,
  thanYeuCauCauViGov,
  TRUONG_GUI_DI_CAU_VIGOV,
  TRUONG_GUI_DI_PHIEN,
} from "../features/dang-nhap/hop-dong";
import { KHAI_BAO_LOI_GOI, type KhaiBaoLoiGoi } from "../features/tinh-nang/zalo-api";

import type { MucChinhSach } from "./chinh-sach-rieng-tu";
import {
  COMMUNE_TERMS_EFFECTIVE_DATE,
  COMMUNE_TERMS_EMERGENCY,
  COMMUNE_TERMS_VERSION,
  communeTerms,
  communeTermsValuesFor,
  type CommuneTermsValues,
} from "./commune-terms";
import { COMPANY } from "./company-profile";
import { MUC_DIEU_KHOAN, NGAY_HIEU_LUC_DIEU_KHOAN, PHIEN_BAN_DIEU_KHOAN } from "./dieu-khoan";
import {
  canhBaoVanXuoi,
  communeTermsDocument,
  COMMUNE_APP_ROUTES_OWED,
  COMMUNE_APP_SESSION_SCREENS,
  communeAppCalls,
  communeAppRoutes,
  DUONG_CONG_KHAI,
  DUONG_ROI_KHOI_MAY,
  PHONE_VERIFICATION_SCREENS,
  SCENE_PHOTO_COMPLETION_FIELDS,
  SCENE_PHOTO_COMPLETION_PATH,
  SCENE_PHOTO_LIST_FIELDS,
  SCENE_PHOTO_SCREENS,
  SCENE_PHOTO_SLOT_FIELDS,
  SCENE_PHOTO_STORAGE_FIELDS,
  SCENE_PHOTO_STORAGE_TARGET,
  SCENE_PHOTOS_PATH,
  SEND_SCREEN_NAME,
  TEN_MAN_CONG_KHAI,
  khoiRoiKhoiMay,
  MOC_BAT_DAU,
  MOC_BAT_DAU_ROI_MAY,
  MOC_KET_THUC,
  MOC_KET_THUC_ROI_MAY,
  VAN_BAN_CHINH_SACH,
  VAN_BAN_DIEU_KHOAN,
  bangQuyen,
  banSinhRa,
  ketXuatVanBan,
  thayKhoiSinhRa,
  type VanBanPhapLy,
} from "./ket-xuat-ho-so";

/**
 * HỒ SƠ NỘP ZALO — PHÉP KIỂM CHỐNG TRÔI.
 *
 * ⚠ LOẠI LỖI TỆP NÀY TỒN TẠI VÌ NÓ, VÀ NÓ ĐÃ XẢY RA THẬT — ba ngày, không một thứ gì đỏ lên:
 *
 *   `tmp/xin-quyen-zalo/` có ba tài liệu soạn 18/09/2026. Ngày 21/09 bên phát hành app đổi sang
 *   Tập đoàn ViHAT Group (ADR 0031) và giao diện được dựng lại. Ba tệp ấy ĐỨNG YÊN: bốn chỗ
 *   trong bản chính sách và năm chỗ trong bản điều khoản vẫn khai một pháp nhân không còn phát
 *   hành app này là bên phát hành, và bảng tính năng vẫn kể một tính năng đã bị gỡ. Chúng là bản
 *   CHÉP TAY của những thứ sống ở nơi khác — đúng hình dạng luật 9 cấm.
 *
 * ⚠ CANH PHẦN THUẦN, KHÔNG CANH TỆP TRÊN ĐĨA — và đây là một quyết định, không phải tiện tay:
 *
 *   `tmp/` nằm trong `.gitignore`. Một ca đòi `tmp/xin-quyen-zalo/dieu-khoan-su-dung.txt` tồn
 *   tại sẽ ĐỎ trên mọi bản sao mới của kho, tức đỏ vì một lý do không liên quan gì tới nội dung
 *   — và một ca đỏ vì lý do sai là một ca sắp bị tắt. Ở đây mọi thứ được kiểm trên CHUỖI văn bản
 *   mà hàm kết xuất trả về; script chỉ còn việc ghi đúng chuỗi ấy xuống tệp.
 *
 * ⚠ MỖI HÀM KIỂM NHẬN **BẢN KẾT XUẤT** VÀ **DANH SÁCH NGUỒN** LÀ HAI THAM SỐ RỜI, có chủ đích:
 *   chỉ khi ấy mới cho nó ăn được một vi phạm dựng sẵn. Một hàm tự dựng lấy bản kết xuất từ
 *   chính danh sách nó đang kiểm thì không bao giờ đỏ được — nó so một thứ với chính nó, và đó
 *   là dạng "xanh vì lý do sai" mà kho này đã gặp nhiều lần.
 */

/* =============================================================================================
   1 — MỌI MỤC CỦA HAI VĂN BẢN ĐỀU CÓ MẶT TRONG BẢN KẾT XUẤT
   ============================================================================================= */

/**
 * Mục nào của văn bản nguồn không tìm thấy trong bản kết xuất.
 *
 * Kiểm CẢ tiêu đề lẫn TỪNG đoạn, không chỉ đếm số mục: một vòng lặp quên `muc.doan` vẫn in ra đủ
 * mười ba tiêu đề và mất trắng phần nội dung — bản nộp khi ấy là một mục lục, và nó trông hợp lý
 * cho tới lúc có người đọc.
 */
function mucBiBoQuen(ban_ket_xuat: string, muc: readonly MucChinhSach[]): string[] {
  const thieu: string[] = [];
  for (const mot of muc) {
    if (!ban_ket_xuat.includes(mot.tieu_de.toUpperCase())) thieu.push(`${mot.ma} (tiêu đề)`);
    mot.doan.forEach((doan, i) => {
      if (!ban_ket_xuat.includes(doan)) thieu.push(`${mot.ma} (đoạn ${i + 1})`);
    });
  }
  return thieu;
}

describe("1 — bản kết xuất mang đủ mọi mục của văn bản nguồn", () => {
  it("chính sách quyền riêng tư: không mục nào, không đoạn nào bị bỏ lại", () => {
    expect(
      mucBiBoQuen(ketXuatVanBan(VAN_BAN_CHINH_SACH), VAN_BAN_CHINH_SACH.muc),
      "một mục của chính sách không có trong bản gửi Zalo. Bản app hiện và bản Zalo đọc phải là " +
        "MỘT văn bản — lệch nhau một mục là công bố hai chính sách khác nhau cho cùng một ứng dụng.",
    ).toEqual([]);
    // Và lượt quét phải có gì để quét: một danh sách rỗng cũng "không thiếu mục nào".
    expect(VAN_BAN_CHINH_SACH.muc.length).toBeGreaterThanOrEqual(10);
  });

  it("điều khoản sử dụng: không mục nào, không đoạn nào bị bỏ lại", () => {
    expect(
      mucBiBoQuen(ketXuatVanBan(VAN_BAN_DIEU_KHOAN), VAN_BAN_DIEU_KHOAN.muc),
      "một mục của điều khoản không có trong bản gửi Zalo",
    ).toEqual([]);
    expect(VAN_BAN_DIEU_KHOAN.muc.length).toBeGreaterThanOrEqual(10);
  });

  it("phần đầu nói đủ: đây là bản sinh ra · tiêu đề · phiên bản · ngày hiệu lực · câu mở đầu", () => {
    for (const vb of [VAN_BAN_CHINH_SACH, VAN_BAN_DIEU_KHOAN, THANG_BINH_TERMS()]) {
      const chu = ketXuatVanBan(vb);
      // DÒNG ĐẦU TIÊN, không phải "có ở đâu đó": một tệp sinh ra mà trông như tệp viết tay là
      // tệp sẽ có người sửa tay, và bản sửa ấy biến mất không báo trước ở lần sinh lại.
      expect(chu.split("\n")[0]).toBe(banSinhRa(vb.nguon));
      expect(chu, "dòng đầu không chỉ ra tệp nguồn").toContain(vb.nguon);
      expect(chu, "dòng đầu không nói lệnh sinh lại").toContain("npm run ho-so");
      expect(chu).toContain(vb.tieu_de.toUpperCase());
      expect(chu).toContain(`Phiên bản ${vb.phien_ban}`);
      expect(chu).toContain(vb.ngay_hieu_luc);
      expect(chu).toContain(vb.cau_dau);
      // The app line is the document's own, right under the title (banner · blank · title · app line).
      expect(chu.split("\n")[3]).toBe(vb.appLine);
    }
  });

  it("commune app terms: no section, no paragraph left out", () => {
    const doc = THANG_BINH_TERMS();
    expect(mucBiBoQuen(ketXuatVanBan(doc), doc.muc)).toEqual([]);
    expect(doc.muc.length).toBeGreaterThanOrEqual(10);
  });

  it("bắt được một mục bị bỏ quên — kể cả khi chỉ MẤT PHẦN NỘI DUNG", () => {
    const chu = ketXuatVanBan(VAN_BAN_CHINH_SACH);
    const dau = VAN_BAN_CHINH_SACH.muc[0]!;

    // ĐỘT BIẾN 1 — thêm một mục vào tệp nội dung mà hàm kết xuất không đi qua nó.
    const muc_moi: MucChinhSach = {
      ma: "muc-chua-in",
      tieu_de: "Một mục chưa từng được in",
      doan: ["Một câu chưa từng được in."],
    };
    expect(mucBiBoQuen(chu, [muc_moi])).toEqual([
      "muc-chua-in (tiêu đề)",
      "muc-chua-in (đoạn 1)",
    ]);

    // ĐỘT BIẾN 2 — tiêu đề vẫn in, ĐOẠN thì không. Đây là hình dạng hỏng dễ lọt nhất: bản nộp
    // trông đủ mục, và chỉ người đọc hết mới thấy nó rỗng.
    const mat_doan: MucChinhSach = {
      ma: dau.ma,
      tieu_de: dau.tieu_de,
      doan: [...dau.doan, "Một đoạn chưa từng được in."],
    };
    expect(mucBiBoQuen(chu, [mat_doan])).toEqual([`${dau.ma} (đoạn ${dau.doan.length + 1})`]);

    // ĐỘT BIẾN 3 — bản kết xuất rỗng. Một hàm kết xuất trả về chuỗi rỗng phải đỏ, không xanh.
    expect(mucBiBoQuen("", VAN_BAN_DIEU_KHOAN.muc).length).toBeGreaterThan(0);

    // Và KHÔNG kêu oan trên văn bản thật — một dây bẫy kêu sai chỗ bị tắt nhanh y như dây bẫy câm.
    expect(mucBiBoQuen(ketXuatVanBan(VAN_BAN_DIEU_KHOAN), VAN_BAN_DIEU_KHOAN.muc)).toEqual([]);
  });
});

/* =============================================================================================
   2 — MỌI LỜI GỌI NỀN TẢNG ĐỀU CÓ MẶT TRONG BẢNG QUYỀN SINH RA
   ============================================================================================= */

/**
 * Phần tử nào của bảng khai không tìm thấy trong bảng quyền của hồ sơ.
 *
 * Kiểm ĐỦ NĂM TRƯỜNG người duyệt đọc, không chỉ tên API: một bảng in đúng mười hai cái tên mà
 * mất cột "rời khỏi máy" là một hồ sơ im lặng về đúng thứ vòng duyệt hỏi tới.
 */
function khaiBiBoQuen(bang: string, khai: readonly KhaiBaoLoiGoi[]): string[] {
  const thieu: string[] = [];
  for (const mot of khai) {
    if (!bang.includes(mot.api)) thieu.push(`${mot.api} (tên lời gọi)`);
    if (!bang.includes(mot.man)) thieu.push(`${mot.api} (màn)`);
    if (!bang.includes(mot.tinh_nang)) thieu.push(`${mot.api} (tính năng)`);
    if (!bang.includes(mot.de_lam_gi)) thieu.push(`${mot.api} (để làm gì)`);
    if (mot.roi_khoi_may !== "" && !bang.includes(mot.roi_khoi_may)) {
      thieu.push(`${mot.api} (rời khỏi máy)`);
    }
  }
  return thieu;
}

describe("2 — bảng quyền của hồ sơ đọc từ mã nguồn, không chép tay", () => {
  it("mọi lời gọi trong `KHAI_BAO_LOI_GOI` đều có mặt, đủ năm trường", () => {
    expect(
      khaiBiBoQuen(bangQuyen(KHAI_BAO_LOI_GOI), KHAI_BAO_LOI_GOI),
      "một lời gọi nền tảng không có trong bảng quyền gửi Zalo. Đây đúng là cách hồ sơ 18/09 " +
        "hỏng: bảng chép tay đứng yên trong lúc mã nguồn đi tiếp.",
    ).toEqual([]);
    expect(
      KHAI_BAO_LOI_GOI.length,
      "bảng khai rỗng thì phép kiểm trên xanh vì lý do sai",
    ).toBeGreaterThanOrEqual(12);
  });

  it("bảng nói ra nửa nào dùng, Zalo có hỏi không, và chính nó là bản sinh ra", () => {
    const bang = bangQuyen(KHAI_BAO_LOI_GOI);
    expect(bang).toContain("Thương mại");
    expect(bang).toContain("Cả hai");
    // Hai giá trị của cột "Zalo hỏi bạn?" — mất một trong hai nghĩa là cột ấy đang in một hằng.
    expect(bang).toContain("| Có |");
    expect(bang).toContain("| Không |");
    expect(bang, "khối không tự nói nó là bản sinh ra").toContain("bản sinh ra");
    expect(bang).toContain("npm run ho-so");
  });

  it("bắt được một lời gọi vừa được khai mà bảng chưa in ra", () => {
    // ĐỘT BIẾN DỰNG SẴN: một lời gọi thứ mười ba của nửa nhà nước — đúng thứ sẽ xảy ra khi
    // `src/cong-dan/` có tệp nghiệp vụ đầu tiên.
    const moi: KhaiBaoLoiGoi = {
      api: "openChat",
      nua: "nha-nuoc",
      man: "Liên hệ",
      tinh_nang: "Nhắn cho cán bộ xã",
      de_lam_gi: "Mở cửa sổ chat của Zalo để công dân nhắn thẳng cho cán bộ tiếp nhận hồ sơ.",
      hoi_nguoi_dung: true,
      roi_khoi_may: "Nội dung tin nhắn đi tới Zalo.",
    };
    const bang_cu = bangQuyen(KHAI_BAO_LOI_GOI);
    expect(khaiBiBoQuen(bang_cu, [moi])).toEqual([
      "openChat (tên lời gọi)",
      "openChat (tính năng)",
      "openChat (để làm gì)",
      "openChat (rời khỏi máy)",
    ]);

    // ĐỘT BIẾN 2 — một bảng chỉ in tên API, mất bốn cột còn lại. Trông vẫn là một bảng quyền.
    const chi_ten = KHAI_BAO_LOI_GOI.map((mot) => mot.api).join("\n");
    expect(khaiBiBoQuen(chi_ten, KHAI_BAO_LOI_GOI).length).toBeGreaterThan(0);

    // Và không kêu oan: bảng thật chứa đủ mọi trường của mọi lời gọi thật.
    expect(khaiBiBoQuen(bangQuyen(KHAI_BAO_LOI_GOI), KHAI_BAO_LOI_GOI)).toEqual([]);
  });
});

/* =============================================================================================
   3 — HAI CHIỀU CỦA MỘT CÂU: AI PHÁT HÀNH, VÀ AI NHẬN DỮ LIỆU
   ============================================================================================= */

/**
 * BỐN MẪU, CANH ĐÚNG MỘT VIỆC: một câu khai VihatSoftware là BÊN PHÁT HÀNH / BÊN CHỊU TRÁCH
 * NHIỆM. Đó là câu đã sai từ 21/09/2026 (ADR 0031), và là câu còn sót bốn lần trong bản chính
 * sách chép tay, năm lần trong bản điều khoản chép tay.
 *
 * ⚠ MẪU NEO VÀO CỤM "BÊN PHÁT HÀNH" / "CHỊU TRÁCH NHIỆM", KHÔNG NEO VÀO CHỮ `VihatSoftware`
 * TRẦN. Đã thử một mẫu rộng hơn (`ứng dụng này … của VihatSoftware`) và nó kêu oan ngay trên một
 * câu khi ấy ĐÚNG: câu mở đầu của chính sách — *"Ứng dụng này không lưu … tới máy chủ của
 * VihatSoftware"* — nằm gọn trong một câu, không có dấu chấm nào ở giữa. Một dây bẫy kêu oan ở
 * đúng câu quan trọng nhất là một dây bẫy sẽ bị tắt trong lần dọn tiếp theo. (Từ 25/09/2026 câu
 * ấy nói "máy chủ của Tập đoàn ViHAT Group" — câu mở #28 — nhưng VihatSoftware vẫn đứng hợp lệ
 * trong câu dài ở vai trò đơn vị thành viên / sở hữu trí tuệ, nên lý do neo hẹp vẫn còn nguyên.)
 *
 * Khoảng cách chặn ở 80 ký tự: đủ cho lối viết chèn vế "— đơn vị thành viên của Tập đoàn ViHAT
 * Group —" vào giữa, không đủ để nối hai mệnh đề chẳng liên quan trong cùng một câu dài.
 *
 * ⚠ CỜ `i` KHÔNG PHẢI TRANG TRÍ — ĐÃ ĐO: bản đầu của bộ mẫu này phân biệt hoa thường và KHÔNG
 * bắt được câu *"Bên phát hành ứng dụng này là VihatSoftware."*, vì "Bên" viết hoa khi nó đứng
 * đầu câu. Một cách viết hoàn toàn bình thường lọt qua cả bốn mẫu, và nó lọt trong im lặng.
 */
const KHAI_SAI_BEN_PHAT_HANH: readonly RegExp[] = [
  /VihatSoftware[^.]{0,80}bên phát hành/i,
  /bên phát hành[^.]{0,80}VihatSoftware/i,
  /VihatSoftware[^.]{0,80}chịu trách nhiệm/i,
  /Mini App[^.]{0,40}của VihatSoftware/i,
];

function cauKhaiSaiBenPhatHanh(chu: string): string[] {
  return KHAI_SAI_BEN_PHAT_HANH.flatMap((mau) => chu.match(mau) ?? []);
}

describe("3 — bên phát hành và bên nhận dữ liệu là HAI câu khác nhau", () => {
  const caHai = () => `${ketXuatVanBan(VAN_BAN_CHINH_SACH)}\n${ketXuatVanBan(VAN_BAN_DIEU_KHOAN)}`;

  it("KHÔNG bản kết xuất nào còn khai VihatSoftware là bên phát hành", () => {
    expect(
      cauKhaiSaiBenPhatHanh(caHai()),
      "hồ sơ gửi Zalo vẫn khai một pháp nhân không còn phát hành app này là bên phát hành. " +
        "ADR 0031: bên phát hành là Tập đoàn ViHAT Group.",
    ).toEqual([]);
  });

  it("bản kết xuất khai ViHAT Group là bên phát hành và bên chịu trách nhiệm", () => {
    // Canh cả chiều THIẾU: gỡ câu sai mà không có câu đúng thay chỗ thì hồ sơ không khai ai chịu
    // trách nhiệm cả — với một văn bản theo Nghị định 13 thì đó là thiếu một phần bắt buộc.
    expect(ketXuatVanBan(VAN_BAN_CHINH_SACH)).toMatch(
      /Tập đoàn ViHAT Group là bên phát hành ứng dụng này và là bên chịu trách nhiệm/,
    );
    expect(ketXuatVanBan(VAN_BAN_DIEU_KHOAN)).toMatch(
      /Mini App giới thiệu của Tập đoàn ViHAT Group/,
    );
  });

  it("khai nơi nhận dữ liệu là máy chủ của Tập đoàn ViHAT Group, không còn VihatSoftware (#28)", () => {
    // Câu hỏi mở #28 ĐÃ QUYẾT 25/09/2026 (ADR 0044 câu 2): bên vận hành `vihat-miniapp` và bên
    // nhận dữ liệu là Tập đoàn ViHAT Group. Từ 21/09 tới 25/09 ca này ghim chiều ngược lại —
    // "máy chủ của VihatSoftware" — vì khi ấy chưa ai xác nhận. Nay chuỗi cũ trong bản gửi Zalo là
    // lời khai SAI nơi nhận dữ liệu cá nhân (Nghị định 13/2023), kể cả lối viết kèm "— đơn vị
    // thành viên của Tập đoàn ViHAT Group". Mẫu cấm là `code_signals` của chính câu #28.
    const chu = ketXuatVanBan(VAN_BAN_CHINH_SACH);
    expect(
      chu,
      "lời khai nơi nhận dữ liệu bị xoá khỏi bản gửi Zalo hoặc chưa đổi sang ViHAT Group",
    ).toContain("máy chủ của Tập đoàn ViHAT Group");
    expect(chu, "bản gửi Zalo còn khai VihatSoftware là nơi nhận dữ liệu").not.toMatch(
      /m[áa]y ch[ủu] c[ủu]a\s+vihat\s*software/i,
    );
  });

  it("điều khoản KHÔNG tự viết ra một lời khai nơi nhận dữ liệu — nó chỉ sang chính sách", () => {
    // Hai văn bản nói hai nơi nhận khác nhau là một mâu thuẫn ngay trong bộ hồ sơ. Lời khai ấy
    // có đúng một chủ (`chinh-sach-rieng-tu.ts`), và điều khoản chỉ đường tới đó.
    const chu = ketXuatVanBan(VAN_BAN_DIEU_KHOAN);
    expect(chu).toContain("Chính sách quyền riêng tư");
    expect(chu, "điều khoản đang tự khai một nơi nhận dữ liệu").not.toMatch(/máy chủ của \S/);
  });

  it("KHÔNG quay lại câu 'không có máy chủ nào của chúng tôi nhận dữ liệu'", () => {
    // Câu ấy đúng tới 19/09/2026 và SAI từ ngày bước đăng nhập gọi máy chủ thật — ở cả bản nộp.
    // Nó đọc trôi hơn câu đúng, nên một lượt "biên tập cho gọn" sẽ mời nó quay lại.
    expect(caHai()).not.toMatch(/không có máy chủ nào[^.]{0,60}nhận dữ liệu/);
  });

  it("bắt được câu khai sai — cả hai chiều viết — và không kêu oan ở câu đúng", () => {
    const PHAI_BAT = [
      "VihatSoftware — đơn vị thành viên của Tập đoàn ViHAT Group — là bên phát hành ứng dụng này và là bên chịu trách nhiệm về chính sách này.",
      "Đây là Mini App giới thiệu của VihatSoftware, cung cấp giải pháp chuyển đổi số.",
      "Bên phát hành ứng dụng này là VihatSoftware.",
      "VihatSoftware là bên chịu trách nhiệm về nội dung này.",
    ];
    for (const cau of PHAI_BAT) {
      expect(
        cauKhaiSaiBenPhatHanh(cau).length,
        `không bắt được câu khai sai: ${cau}`,
      ).toBeGreaterThan(0);
    }

    const KHONG_DUOC_KEU = [
      "Ứng dụng này không lưu bất kỳ dữ liệu nào của bạn xuống máy, và chỉ gửi đi ở đúng HAI việc, cả hai đều do chính bạn bấm: khi bạn đăng nhập, nó gửi hai mã dùng một lần do Zalo cấp. Cả hai đi tới máy chủ của Tập đoàn ViHAT Group.",
      "Khi bạn bấm đăng nhập, ứng dụng gửi hai mã ấy tới máy chủ của Tập đoàn ViHAT Group.",
      // VihatSoftware ở vai trò SỞ HỮU TRÍ TUỆ, cùng một câu với "ứng dụng" — câu đúng, phải lọt.
      "Tên gọi, logo, nội dung giới thiệu, hình ảnh và mã nguồn của ứng dụng thuộc quyền của VihatSoftware và Tập đoàn ViHAT Group.",
      "Không dùng tên, logo hoặc nội dung của VihatSoftware và ViHAT Group cho mục đích thương mại khác.",
      "Tập đoàn ViHAT Group là bên phát hành ứng dụng này và là bên chịu trách nhiệm về chính sách này.",
    ];
    for (const cau of KHONG_DUOC_KEU) {
      expect(cauKhaiSaiBenPhatHanh(cau), `kêu oan ở câu đúng: ${cau}`).toEqual([]);
    }
  });
});

/* =============================================================================================
   4 — KHỐI SINH RA TRONG README CỦA HỒ SƠ
   ============================================================================================= */

describe("4 — khối sinh ra chỉ ăn phần giữa hai mốc", () => {
  const README_GIA = [
    "# Hồ sơ",
    "",
    "Văn xuôi giữ tay ở trên.",
    "",
    MOC_BAT_DAU,
    "",
    "bảng cũ, sẽ bị thay",
    "",
    MOC_KET_THUC,
    "",
    "Văn xuôi giữ tay ở dưới.",
  ].join("\n");

  it("thay đúng phần giữa, giữ nguyên văn xuôi hai đầu", () => {
    const ra = thayKhoiSinhRa(README_GIA, "BẢNG MỚI");
    expect(ra).toContain("Văn xuôi giữ tay ở trên.");
    expect(ra).toContain("Văn xuôi giữ tay ở dưới.");
    expect(ra).toContain("BẢNG MỚI");
    expect(ra, "bảng cũ vẫn còn — README sẽ có hai bảng quyền").not.toContain("bảng cũ");
    // Sinh hai lần liên tiếp cho cùng một kết quả: một hàm thay khối mà mỗi lần chạy lại nở thêm
    // một khối là một tệp lớn dần sau mỗi lần sinh.
    expect(thayKhoiSinhRa(ra, "BẢNG MỚI")).toBe(ra);
  });

  it("MẤT MỐC thì dừng lại và nói ra, không phụ thêm vào cuối tệp", () => {
    // Phụ thêm vào cuối thì README có hai bảng — bảng chép tay cũ ở trên, bảng sinh ra ở dưới —
    // và người duyệt đọc phải bảng nào là chuyện may rủi.
    expect(() => thayKhoiSinhRa("# Không còn mốc nào", "BẢNG MỚI")).toThrow(/mốc đánh dấu/);
    expect(() => thayKhoiSinhRa(`${MOC_KET_THUC}\n${MOC_BAT_DAU}`, "X")).toThrow(/mốc đánh dấu/);
  });

  it("thay được khối THỨ HAI mà không đụng khối thứ nhất", () => {
    // README nay có hai khối sinh ra. Một hàm chỉ biết một cặp mốc thì khối thứ hai phải được
    // thay bằng tay — tức là không bao giờ được thay.
    const hai_khoi = [
      MOC_BAT_DAU,
      "bảng quyền cũ",
      MOC_KET_THUC,
      "văn xuôi ở giữa",
      MOC_BAT_DAU_ROI_MAY,
      "khối rời khỏi máy cũ",
      MOC_KET_THUC_ROI_MAY,
    ].join("\n");

    const ra = thayKhoiSinhRa(hai_khoi, "RỜI MÁY MỚI", MOC_BAT_DAU_ROI_MAY, MOC_KET_THUC_ROI_MAY);
    expect(ra).toContain("RỜI MÁY MỚI");
    expect(ra, "khối rời khỏi máy cũ vẫn còn").not.toContain("khối rời khỏi máy cũ");
    expect(ra, "thay khối thứ hai lại nuốt mất khối thứ nhất").toContain("bảng quyền cũ");
    expect(ra).toContain("văn xuôi ở giữa");
  });
});

/* =============================================================================================
   5 — KHỐI "NHỮNG GÌ RỜI KHỎI MÁY": KHOÁ HAI CHIỀU VỚI HAI HỢP ĐỒNG
   ============================================================================================= */

/**
 * ⚠ KHỐI NÀY RA ĐỜI VÌ MỘT LỜI KHAI SAI ĐÃ SỐNG TRONG HỒ SƠ PHÁP LÝ, VÀ KHÔNG GÌ ĐỎ LÊN.
 *
 *   `bangQuyen()` đọc `KHAI_BAO_LOI_GOI`, mà bảng ấy chỉ biết lời gọi `zmp-sdk`. Khi giai đoạn B
 *   mở `POST /api/v1/requests` — một `fetch` thuần, gửi đi chữ người dùng TỰ GÕ — hai câu trong
 *   văn xuôi vẫn khai rằng đăng nhập là đường DUY NHẤT có dữ liệu rời khỏi máy.
 *
 *   Ba ca dưới là hai chiều của một cái khoá, cộng một ca chống "xanh vì không tìm thấy gì".
 */
describe("5 — mọi thứ rời khỏi máy đều được khai, và mọi thứ khai đều thật sự rời khỏi máy", () => {
  it("quét đủ MƯỜI đường — một lượt quét thiếu đường sẽ xanh vì lý do sai", () => {
    // 2 của 22/09 (đăng nhập · yêu cầu) + 5 của 27/09: bước xác nhận xã (cùng tuyến đăng nhập, thân
    // khác) và bốn tuyến công khai của ViGov (tra xã · danh bạ · danh sách tin · một tin) + 1 của 28/09:
    // mở lại phiên với xã KÈM `phoneToken` (cùng tuyến đăng nhập, thân thứ ba) + 1 của 29/09: đổi mã vị
    // trí lấy toạ độ (`vihat-miniapp` `/api/v1/location`), chỉ sau cú bấm "Lấy vị trí hiện tại" + 1 cũng
    // của 29/09: đăng nhập từ app riêng của xã (cùng tuyến đăng nhập, thân thứ tư, `appId` + `phoneToken`).
    // + 1: the commune app's location exchange (same route, `appId` added). + 1: the commune's declared
    // office, read when the commune app opens (`/commune-profiles`). + 1: the commune's field list for
    // step 1 of "Gửi phản ánh" (`/my-citizen-report-fields`) — no field sent, only the session header.
    // + 1 (30/09, card D2): the news category chips (`/commune-news/categories`), on the citizen's tap.
    // + 4 (02/10/2026): the commune app's scene photos — slot, upload to the store, completion, own list.
    expect(DUONG_ROI_KHOI_MAY).toHaveLength(18);
    const fieldsRow = DUONG_ROI_KHOI_MAY.find((d) => d.tuyen === CITIZEN_FIELDS_PATH);
    expect(fieldsRow, "hồ sơ không khai tuyến danh mục lĩnh vực").toBeDefined();
    expect(fieldsRow!.truong).toEqual([]);
    expect(fieldsRow!.nguoi_dung_bam).toBe(true);
    // The client sends no query string and no body on that route: nothing more to declare.
    expect(citizenFieldsAddress()).not.toContain("?");
    const communeLocationRow = DUONG_ROI_KHOI_MAY.find((d) => d.truong === COMMUNE_APP_LOCATION_FIELDS);
    expect(communeLocationRow, "hồ sơ không khai đổi mã vị trí của app riêng").toBeDefined();
    expect(communeLocationRow!.tuyen).toBe(LOCATION_PATH);
    expect(communeLocationRow!.nguoi_dung_bam).toBe(true);
    const locationRow = DUONG_ROI_KHOI_MAY.find((d) => d.tuyen === LOCATION_PATH);
    expect(locationRow, "hồ sơ không khai tuyến đổi mã vị trí").toBeDefined();
    expect(locationRow!.nguoi_dung_bam).toBe(true);
    expect(locationRow!.truong).toBe(LOCATION_FIELDS);
    expect(locationRow!.man).toBe(SEND_SCREEN_NAME);
    const tuyen = DUONG_ROI_KHOI_MAY.map((d) => d.tuyen);
    // Three bodies of the SHARED app's `vihat-miniapp` login; the commune app's login left it (ADR 0066).
    expect(tuyen.filter((t) => t === "/api/v1/sessions")).toHaveLength(3);
    // The commune app's login prints ITS table — `appId` and `phoneToken`, no commune domain — after a tap.
    const communeAppRow = DUONG_ROI_KHOI_MAY.find((d) => d.truong === COMMUNE_APP_SESSION_FIELDS);
    expect(communeAppRow, "hồ sơ không khai đăng nhập từ app riêng của xã").toBeDefined();
    // ...and it says the truth about WHERE: ViGov identity's own route, directly (ADR 0066).
    expect(communeAppRow!.tuyen).toBe(COMMUNE_APP_SESSION_PATH);
    expect(communeAppRow!.tuyen).toBe("/api/v1/citizen-sessions");
    expect(communeAppRow!.may_chu).toMatch(/^ViGov — dịch vụ `identity`/);
    expect(communeAppRow!.may_chu).toContain("không qua máy chủ của Tập đoàn ViHAT Group");
    // The declared path is the one the real call reaches, on the host the state half's map gives.
    expect(new URL(communeAppSessionAddress(diaChiViGov("identity", ""))).pathname).toBe(communeAppRow!.tuyen);
    expect(tuyen.filter((t) => t === COMMUNE_APP_SESSION_PATH)).toHaveLength(1);
    expect(communeAppRow!.nguoi_dung_bam).toBe(true);
    expect(communeAppRow!.man).toBe(COMMUNE_APP_SESSION_SCREENS);
    expect(communeAppRow!.truong.map((t) => t.khoa)).not.toContain("communeHostHint");
    // Lần mở lại in ĐÚNG bảng của nó — bảng có `phoneToken` — và nó chỉ chạy sau một cú bấm.
    const reopenRow = DUONG_ROI_KHOI_MAY.find((d) => d.truong === BRIDGE_FIELDS_WITH_PHONE);
    expect(reopenRow, "hồ sơ không khai lần mở lại phiên kèm số điện thoại").toBeDefined();
    expect(reopenRow!.nguoi_dung_bam).toBe(true);
    expect(reopenRow!.truong.map((t) => t.khoa)).toContain("phoneToken");
    expect(reopenRow!.man).toBe(PHONE_VERIFICATION_SCREENS);
    for (const t of [
      "/api/v1/requests",
      DUONG_DAN_XA,
      DUONG_DAN_DANH_BA,
      DUONG_DAN_TIN_XA,
      `${DUONG_DAN_TIN_XA}/{id}`,
      NEWS_CATEGORIES_PATH,
    ]) {
      expect(tuyen, `hồ sơ không khai tuyến ${t}`).toContain(t);
    }
    // Bước xác nhận xã in ĐÚNG bảng của nó, không mượn bảng của khối đăng nhập.
    expect(DUONG_ROI_KHOI_MAY.map((d) => d.truong)).toContain(TRUONG_GUI_DI_CAU_VIGOV);
  });

  it("MỌI khoá ba thân yêu cầu gửi đi đều có một dòng khai", () => {
    // Chiều "mã đi trước": thêm một trường vào `thanYeuCau` mà quên khai là ĐỎ ở đây.
    for (const [ten, than, bang] of [
      ["phiên đăng nhập", thanYeuCauPhien({ ma_so_dien_thoai: "x", ma_truy_cap: "y" }), TRUONG_GUI_DI_PHIEN],
      // 07/10/2026: measured on a CHAT body carrying a name — the widest shape of this route (`displayName`
      // rides on `chat` only; every other kind sends a subset). Measured on a consult body, `displayName` would
      // read as "declared but never sent". `chinh-sach.test.ts` checks the union of shapes both ways.
      [
        "yêu cầu tư vấn",
        thanYeuCau({ loai: "chat", quan_tam: [], quy_mo: "", ghi_chu: "", nguon: "", display_name: "Nguyễn Văn Thử" }),
        TRUONG_GUI_DI,
      ],
      [
        "xác nhận xã",
        thanYeuCauCauViGov({ ma_truy_cap: "m", ten_mien_xa: "xa-vi-du.vigov.example" }),
        TRUONG_GUI_DI_CAU_VIGOV,
      ],
      [
        "mở lại phiên kèm số",
        bridgeBodyWithPhone({ ma_truy_cap: "m", ten_mien_xa: "xa-vi-du.vigov.example", ma_so_dien_thoai: "p" }),
        BRIDGE_FIELDS_WITH_PHONE,
      ],
      ["đổi mã vị trí", locationBody({ access_token: "m", location_token: "v" }), LOCATION_FIELDS],
      [
        "đổi mã vị trí của app riêng",
        locationBody({ access_token: "m", location_token: "v" }, "1234567890"),
        COMMUNE_APP_LOCATION_FIELDS,
      ],
      [
        "đăng nhập từ app riêng của xã",
        communeAppSessionBody({ ma_truy_cap: "m", ma_so_dien_thoai: "p", app_id: "1234567890" }),
        COMMUNE_APP_SESSION_FIELDS,
      ],
    ] as const) {
      const khoa = Object.keys(JSON.parse(than) as Record<string, unknown>);
      expect(khoa.length, `thân ${ten} không còn trường nào để đo`).toBeGreaterThan(0);
      const da_khai = bang.map((t) => t.khoa);
      for (const k of khoa) {
        expect(
          da_khai,
          `tuyến ${ten} gửi đi trường "${k}" mà hồ sơ Zalo KHÔNG khai. Thêm một dòng vào bảng ` +
            "TRUONG_GUI_DI tương ứng — đừng sửa ca này.",
        ).toContain(k);
      }
      // Chiều ngược: không khai thừa một trường mã KHÔNG gửi.
      for (const t of bang) {
        expect(khoa, `hồ sơ khai trường "${t.khoa}" mà tuyến ${ten} không hề gửi`).toContain(t.khoa);
      }
    }
  });

  it("khối sinh ra in ĐỦ mọi dòng khai của cả hai tuyến", () => {
    const khoi = khoiRoiKhoiMay(DUONG_ROI_KHOI_MAY);
    for (const duong of DUONG_ROI_KHOI_MAY) {
      expect(khoi, `khối thiếu tuyến ${duong.tuyen}`).toContain(duong.tuyen);
      expect(khoi, `khối không nói tuyến ${duong.tuyen} chạy khi nào`).toContain(duong.khi_nao);
      for (const t of duong.truong) {
        expect(khoi, `khối thiếu trường ${t.khoa}`).toContain(t.khoa);
        expect(khoi, `khối thiếu câu khai của ${t.khoa}`).toContain(t.trong_chinh_sach);
      }
    }
    // Và nó nói ra rằng không tuyến nào mang một trường định danh — câu người duyệt hỏi đầu tiên.
    expect(khoi).toContain("tôi là ai");
  });

  /**
   * BA TUYẾN CÔNG KHAI — BẢN CHÉP TRONG `ket-xuat-ho-so.ts`, KHOÁ VỚI ĐỊA CHỈ THẬT CLIENT DỰNG RA.
   *
   *   Tệp sản xuất không được nhập client ViGov (`ranh-gioi-hai-nua.test.ts` §3a), nên nó chép. Tệp test
   *   này thì được (lượt quét ranh giới bỏ `.test.`), nên nó dựng địa chỉ bằng CHÍNH hàm của client và
   *   đối chiếu hai chiều: tham số địa chỉ mang ⇄ dòng khai; đường dẫn đúng từng chữ.
   */
  it("mỗi tuyến công khai: đường dẫn đúng từng chữ, và tham số gửi đi ⇄ dòng khai, hai chiều", () => {
    const TEN_MIEN = "xa-vi-du.vigov.example";
    const tim = (t: string) => DUONG_CONG_KHAI.find((d) => d.tuyen === t);
    const CA: ReadonlyArray<readonly [string, string, readonly string[]]> = [
      // [tuyến khai, địa chỉ client dựng, tham số đường dẫn]
      [DUONG_DAN_XA, diaChiTraXa(TEN_MIEN), []],
      [COMMUNE_PROFILES_PATH, communeProfilesAddress(TEN_MIEN), []],
      [DUONG_DAN_DANH_BA, diaChiDanhBa(TEN_MIEN), []],
      // Every parameter the client can send: cursor, type AND category.
      [DUONG_DAN_TIN_XA, diaChiTinXa(TEN_MIEN, "con-tro-thu", "su-kien", "dm-thu"), []],
      // Card D2: the category chips' route — host and type.
      [NEWS_CATEGORIES_PATH, newsCategoriesAddress(TEN_MIEN, "su-kien"), []],
      // Every parameter the client can send: the audio re-read's `no_view` too (02/10/2026).
      [`${DUONG_DAN_TIN_XA}/{id}`, diaChiBaiTin(TEN_MIEN, "tin-thu", { noView: true }), ["id"]],
    ];
    expect(DUONG_CONG_KHAI).toHaveLength(CA.length);
    for (const [tuyen, dia_chi, tham_so_duong_dan] of CA) {
      const duong = tim(tuyen);
      expect(duong, `hồ sơ không khai tuyến ${tuyen}`).toBeDefined();
      expect(dia_chi, `client không dựng được địa chỉ cho ${tuyen} — ca này sẽ xanh vì rỗng`).not.toBe("");
      const url = new URL(dia_chi);
      // Đường dẫn: thay `{id}` bằng giá trị đã dựng rồi so từng chữ.
      expect(url.pathname).toBe(tuyen.replace("{id}", "tin-thu"));
      const gui_di = [...url.searchParams.keys(), ...tham_so_duong_dan].sort();
      expect(duong!.truong.map((t) => t.khoa).sort(), `tuyến ${tuyen}: khai ⇄ gửi đi lệch nhau`).toEqual(gui_di);
      // Không tuyến công khai nào tự nhận là "do người dùng bấm" khi nó chạy lúc mở app, và ngược lại.
      // Two run at open: the commune lookup (QR path) and the commune app's office profile.
      expect(duong!.nguoi_dung_bam, tuyen).toBe(tuyen !== DUONG_DAN_XA && tuyen !== COMMUNE_PROFILES_PATH);
    }
  });

  /**
   * SCENE PHOTOS (02/10/2026) — the four commune-app rows are COPIES (boundary, as the public routes), locked
   * here to the addresses and the body the state half really builds. A new key in `photoUploadBody`, a renamed
   * path, or a row moved into the shared app is red.
   */
  it("ảnh hiện trường: bốn đường của app riêng, đường dẫn và trường gửi đi khoá với hợp đồng thật", () => {
    const rows = DUONG_ROI_KHOI_MAY.filter((d) =>
      [SCENE_PHOTOS_PATH, SCENE_PHOTO_COMPLETION_PATH, SCENE_PHOTO_STORAGE_TARGET].includes(d.tuyen),
    );
    expect(rows).toHaveLength(4);
    for (const r of rows) {
      expect(r.app, r.tuyen).toBe("commune");
      expect(r.nguoi_dung_bam, r.tuyen).toBe(true);
      expect(r.man).toBe(SCENE_PHOTO_SCREENS);
    }
    const CODE = "PA7K2QX9M4TD";
    expect(new URL(photosAddress(CODE)).pathname).toBe(SCENE_PHOTOS_PATH.replace("{maTraCuu}", CODE));
    expect(new URL(photoCompletionAddress(CODE, "f-1")).pathname).toBe(
      SCENE_PHOTO_COMPLETION_PATH.replace("{maTraCuu}", CODE).replace("{id}", "f-1"),
    );
    // Slot: the path's code plus EXACTLY the body's keys, both ways.
    const body = Object.keys(JSON.parse(photoUploadBody("image/jpeg", 1)) as Record<string, unknown>).sort();
    expect(body).toEqual([...PHOTO_UPLOAD_FIELDS].sort());
    expect(SCENE_PHOTO_SLOT_FIELDS.map((t) => t.khoa).sort()).toEqual(["maTraCuu", ...body].sort());
    // Upload: the issued form, then the file, under the field name the client writes.
    expect(SCENE_PHOTO_STORAGE_FIELDS.map((t) => t.khoa)).toEqual(["fields", STORAGE_FILE_FIELD]);
    // Completion and list: path parameters only, no body.
    expect(SCENE_PHOTO_COMPLETION_FIELDS.map((t) => t.khoa)).toEqual(["maTraCuu", "id"]);
    expect(SCENE_PHOTO_LIST_FIELDS.map((t) => t.khoa)).toEqual(["maTraCuu"]);
    // The copied screen names are the real ones.
    expect(SCENE_PHOTO_SCREENS).toBe(`Ứng dụng của xã: ${[GUI.tieu_de, CUA_TOI.tieu_de, TRA_CUU.tieu_de].join(" · ")}`);
    // And the commune app's dossier prints them; the shared one does not carry the commune rows' screens.
    const communeBlock = khoiRoiKhoiMay(communeAppRoutes(DUONG_ROI_KHOI_MAY), { communeApp: true });
    for (const t of [...SCENE_PHOTO_SLOT_FIELDS, ...SCENE_PHOTO_STORAGE_FIELDS, ...SCENE_PHOTO_COMPLETION_FIELDS]) {
      expect(communeBlock, t.khoa).toContain(t.trong_chinh_sach);
    }
  });

  it("tên màn chép trong hồ sơ đúng từng chữ tên màn thật", () => {
    expect(TEN_MAN_CONG_KHAI.danh_ba).toBe(DANH_BA.tieu_de);
    expect(TEN_MAN_CONG_KHAI.tin_xa).toBe(TIN_XA.tieu_de);
    expect(TEN_MAN_CONG_KHAI.trang_chu_xa).toBe(`Ứng dụng của xã: ${XA_GIAO_DIEN.tab_trang_chu}`);
    expect(PHONE_VERIFICATION_SCREENS).toBe([GUI.tieu_de, CUA_TOI.tieu_de, TRA_CUU.tieu_de].join(" · "));
    expect(SEND_SCREEN_NAME).toBe(GUI.tieu_de);
    expect(COMMUNE_APP_SESSION_SCREENS).toBe(
      `Ứng dụng của xã: ${[GUI.tieu_de, CUA_TOI.tieu_de, TRA_CUU.tieu_de, RATING.title].join(" · ")}`,
    );
  });

  it("câu đầu khối KHÔNG còn nói 'không đường nào chạy lúc mở ứng dụng' — tra tên xã chạy lúc mở", () => {
    const khoi = khoiRoiKhoiMay(DUONG_ROI_KHOI_MAY);
    expect(khoi).not.toContain("không đường nào chạy lúc mở ứng dụng");
    expect(khoi).toContain("16 đường chạy khi chính người dùng bấm; 2 đường chạy mà không cần một cú bấm");
    // Và khi mọi đường đều chờ một cú bấm, câu cũ quay lại — cột ấy thật sự được đọc.
    const chi_bam = DUONG_ROI_KHOI_MAY.filter((d) => d.nguoi_dung_bam);
    expect(khoiRoiKhoiMay(chi_bam)).toContain("không đường nào chạy lúc mở ứng dụng");
    // Mỗi đường nói máy chủ nào nhận.
    for (const d of DUONG_ROI_KHOI_MAY) expect(khoi).toContain(d.may_chu);
  });

  it("bắt được một tuyến vừa khai mà khối chưa in ra", () => {
    // Ca về chính cơ chế: nếu `khoiRoiKhoiMay` bỏ sót một dòng, ba ca trên phải đỏ. Cho nó ăn một
    // tuyến dựng sẵn mà bản in KHÔNG chứa, và khẳng định phép so bắt được.
    const khoi = khoiRoiKhoiMay(DUONG_ROI_KHOI_MAY);
    expect(khoi).not.toContain("/api/v1/tuyen-chua-khai");
  });
});

/* =============================================================================================
   6 — RÀO CHO PHẦN VĂN XUÔI GIỮ TAY
   ============================================================================================= */

/**
 * ⚠ PHẦN VĂN XUÔI KHÔNG NẰM TRONG KHO (`tmp/` bị `.gitignore`), nên `npm test` không nhìn thấy
 * nó — và đó chính là nơi hai câu sai đã sống. Chỗ DUY NHẤT vừa chạy được vừa nhìn thấy nó là
 * `npm run ho-so`. Nên rào là một hàm THUẦN kiểm được ở đây, và script gọi nó rồi DỪNG.
 */
describe("6 — văn xuôi giữ tay không được tự đếm thứ rời khỏi máy", () => {
  it("bắt được đúng hai câu đã từng sai thật trong hồ sơ", () => {
    // Hai chuỗi này là NGUYÊN VĂN những gì đã nằm trong `tmp/xin-quyen-zalo/README.md` tới
    // 22/09/2026. Giữ nguyên văn để ca này chứng minh được nó bắt đúng cái đã xảy ra.
    const cu = [
      "**Đây là chức năng DUY NHẤT của ứng dụng có dữ liệu rời khỏi máy**, và nó gửi đúng hai thứ.",
      "4. **Đúng MỘT chức năng có dữ liệu rời khỏi máy: đăng nhập** (mục 9).",
    ].join("\n");
    const ra = canhBaoVanXuoi(cu);
    expect(ra.length, "rào không bắt được hai câu đã sai thật").toBeGreaterThanOrEqual(2);
    for (const c of ra) {
      expect(c.vi_sao, "cảnh báo không nói việc phải làm tiếp").not.toBe("");
      expect(c.cau, "cảnh báo không chỉ ra câu phải sửa").not.toBe("");
    }
  });

  it("KHÔNG kêu oan ở phần văn xuôi đúng, và KHÔNG ăn khối sinh ra", () => {
    // Một rào kêu oan bị tắt nhanh y như một rào câm. Khối sinh ra CÓ QUYỀN nói "2 đường" —
    // nó là bản sinh ra, và bắt chính nó là làm rào này đỏ ngay lần đầu chạy.
    const sach = [
      "Xem khối **Những gì rời khỏi máy** ở trên để biết dữ liệu của bạn đi đâu.",
      "Ứng dụng không ghi bất kỳ dữ liệu nào của người dùng xuống máy.",
      MOC_BAT_DAU_ROI_MAY,
      khoiRoiKhoiMay(DUONG_ROI_KHOI_MAY),
      MOC_KET_THUC_ROI_MAY,
    ].join("\n");
    expect(canhBaoVanXuoi(sach)).toEqual([]);
  });

  it("README thật của hồ sơ — nếu có trên máy này — phải sạch", () => {
    // ⚠ CA NÀY KHÔNG ĐỌC ĐĨA, có chủ đích (xem khối đầu tệp): `tmp/` bị `.gitignore`, và một ca
    // đòi một tệp không có trong kho là một ca đỏ vì lý do sai trên mọi bản sao mới. Thứ kiểm
    // được ở đây là chính bản sinh ra của khối — và nó phải không tự kích hoạt rào.
    expect(canhBaoVanXuoi(khoiRoiKhoiMay(DUONG_ROI_KHOI_MAY))).toEqual([]);
  });
});

/* =============================================================================================
   7 — HỒ SƠ CỦA ỨNG DỤNG RIÊNG CỦA XÃ: chỉ những gì app ấy làm, bằng lời của app ấy
   ============================================================================================= */

describe("7 — the commune app's dossier lists that app only, in its own words", () => {
  const calls = communeAppCalls(KHAI_BAO_LOI_GOI);
  const routes = communeAppRoutes(DUONG_ROI_KHOI_MAY);

  it("every call both halves use declares a commune view; every commercial-only call is left out", () => {
    for (const row of KHAI_BAO_LOI_GOI) {
      if (row.nua === "ca-hai") expect(row.commune_app, row.api).toBeDefined();
    }
    expect(calls.map((c) => c.api)).toEqual(KHAI_BAO_LOI_GOI.filter((r) => r.nua !== "thuong-mai").map((r) => r.api));
    // Whether Zalo asks is a fact of the platform call, not of the app — never overridden.
    for (const c of calls) {
      expect(c.hoi_nguoi_dung, c.api).toBe(KHAI_BAO_LOI_GOI.find((r) => r.api === c.api)!.hoi_nguoi_dung);
    }
  });

  it("every route running in both apps declares a commune view; shared-only routes are left out", () => {
    for (const row of DUONG_ROI_KHOI_MAY) {
      if (row.app === "both") expect(row.commune_app, row.tuyen).toBeDefined();
    }
    expect(routes).toHaveLength(DUONG_ROI_KHOI_MAY.filter((r) => r.app !== "shared").length);
    // `communeAppRoutes` rebuilds each field list, so compare by route + keys, not by reference.
    const shape = (tuyen: string, truong: readonly { khoa: string }[]) => `${tuyen}|${truong.map((t) => t.khoa).join(",")}`;
    const printed = routes.map((r) => shape(r.tuyen, r.truong));
    for (const shared of DUONG_ROI_KHOI_MAY.filter((r) => r.app === "shared")) {
      expect(printed, shared.tuyen).not.toContain(shape(shared.tuyen, shared.truong));
    }
    expect(printed).toContain(shape(COMMUNE_APP_SESSION_PATH, COMMUNE_APP_SESSION_FIELDS));
    expect(printed).toContain(shape(LOCATION_PATH, COMMUNE_APP_LOCATION_FIELDS));
  });

  it("in the commune app, `host` is the build's domain — never 'from a QR code or link'", () => {
    for (const r of routes) {
      for (const t of r.truong.filter((t) => t.khoa === "host")) {
        expect(t.trong_chinh_sach, r.tuyen).toContain("gắn sẵn trong ứng dụng của xã");
        expect(t.trong_chinh_sach, r.tuyen).not.toContain("mã QR");
      }
    }
    // The shared dossier keeps its own sentence.
    expect(khoiRoiKhoiMay(DUONG_ROI_KHOI_MAY)).toContain("lấy từ mã QR");
  });

  it("the commune app's login is declared as going straight to ViGov, never through ViHAT's server (ADR 0066)", () => {
    const login = routes.find((r) => r.tuyen === COMMUNE_APP_SESSION_PATH)!;
    expect(login.truong.map((t) => t.khoa)).toEqual(COMMUNE_APP_SESSION_FIELDS.map((t) => t.khoa));
    expect(login.may_chu).toContain("không qua máy chủ của Tập đoàn ViHAT Group");
    for (const api of ["getPhoneNumber", "getAccessToken"]) {
      expect(calls.find((c) => c.api === api)!.roi_khoi_may, api).toContain("thẳng tới hệ thống của xã");
    }
  });

  it("no commune-app sentence names a screen of the shared app", () => {
    const tables = [bangQuyen(calls, { communeApp: true }), khoiRoiKhoiMay(routes, { communeApp: true })];
    for (const t of tables) {
      for (const sharedScreen of ["Liên hệ", "Tư vấn và báo giá", "Danh thiếp", TIEU_DE_XAC_NHAN_XA, "sau khi đã xác nhận xã"]) {
        expect(t, sharedScreen).not.toContain(sharedScreen);
      }
      expect(t).not.toContain("| Nửa |");
    }
  });

  it("what runs at open, the commune app says so: the lookup, the office profile and the home-screen news", () => {
    const atOpen = routes.filter((r) => !r.nguoi_dung_bam).map((r) => r.tuyen).sort();
    expect(atOpen).toEqual([COMMUNE_PROFILES_PATH, DUONG_DAN_TIN_XA, DUONG_DAN_XA].sort());
  });

  it("the gap is printed inside the block until the petition routes are declared", () => {
    const block = khoiRoiKhoiMay(routes, { communeApp: true });
    expect(block).toContain(COMMUNE_APP_ROUTES_OWED);
    // Not "đầy đủ" over a list that still lacks the petition routes; not the shared app's sources either.
    expect(block).not.toContain("câu trả lời đầy đủ");
    expect(block).not.toContain("TRUONG_GUI_DI_PHIEN");
    expect(khoiRoiKhoiMay(DUONG_ROI_KHOI_MAY)).not.toContain(COMMUNE_APP_ROUTES_OWED);
  });

  it("the copied commune screen names match the real screens word for word", () => {
    expect(TEN_MAN_CONG_KHAI.commune_news_tab).toBe(`Ứng dụng của xã: ${XA_TN.news_tab_title}`);
    expect(TEN_MAN_CONG_KHAI.commune_directory).toBe(`Ứng dụng của xã: ${DANH_BA.tieu_de}`);
  });

  it("a `both` row with no commune view is refused, not printed in the shared app's words", () => {
    const bare = { ...DUONG_ROI_KHOI_MAY[0]!, app: "both" as const, commune_app: undefined };
    expect(() => communeAppRoutes([bare])).toThrow();
    const bareCall = { ...KHAI_BAO_LOI_GOI.find((r) => r.nua === "ca-hai")!, commune_app: undefined };
    expect(() => communeAppCalls([bareCall])).toThrow();
  });

  it("the commune app's blocks do not trip the prose guard", () => {
    expect(canhBaoVanXuoi(khoiRoiKhoiMay(routes, { communeApp: true }))).toEqual([]);
  });
});

/* =============================================================================================
   8 — THE COMMUNE APP'S TERMS OF USE: one template, filled per commune, in the commune's name only
   ============================================================================================= */

/**
 * The REAL per-commune table, read through `import.meta.glob` rather than an `import`: the file is a `.mjs` under
 * `scripts/` with no type declarations, and a test file may read it (`dich-den.test.mjs` exempts `*.test.*`).
 * Reading the real row — not a copy typed here — is what makes "renders with Thăng Bình's values" mean the values
 * the generator will use.
 */
const DOMAIN_TABLE_MODULE = Object.values(
  import.meta.glob("../../scripts/ung-dung-theo-ten-mien.mjs", { eager: true }),
)[0] as {
  COMMUNE_TERMS_BY_DOMAIN: Record<string, Partial<CommuneTermsValues>>;
};

const THANG_BINH = "thangbinh-danang.vigov.vn";
const THANG_BINH_URL = "https://thangbinh.danang.gov.vn/gioi-thieu/gioi-thieu-chung";

function THANG_BINH_TERMS(): VanBanPhapLy {
  return communeTermsDocument(communeTermsValuesFor(DOMAIN_TABLE_MODULE.COMMUNE_TERMS_BY_DOMAIN, THANG_BINH), THANG_BINH);
}

/** The text a reviewer reads: everything after the generated-file banner, which is stripped before pasting. */
const readerText = (doc: VanBanPhapLy) => ketXuatVanBan(doc).split("\n").slice(1).join("\n");

const VALID: CommuneTermsValues = {
  displayName: "Xã Thử",
  province: "Tỉnh Thử",
  introductionUrl: "https://xa-thu.example.gov.vn/gioi-thieu",
};

describe("8 — the commune app's terms of use", () => {
  it("the table holds the values the owner gave for Thăng Bình", () => {
    // App IDs left this file on 06/10/2026 (service-platform `mini_app` is the only source), so there is no
    // second table to pair this row with any more.
    expect(DOMAIN_TABLE_MODULE.COMMUNE_TERMS_BY_DOMAIN[THANG_BINH]).toEqual({
      displayName: "Xã Thăng Bình",
      province: "Thành phố Đà Nẵng",
      introductionUrl: THANG_BINH_URL,
    });
  });

  it("renders with Thăng Bình's values: name, province, People's Committee, version and date", () => {
    const text = readerText(THANG_BINH_TERMS());
    expect(text).toContain("ĐIỀU KHOẢN SỬ DỤNG");
    expect(text).toContain("Mini App Xã Thăng Bình, Thành phố Đà Nẵng");
    expect(text).toContain(
      "Ủy ban nhân dân xã Thăng Bình cung cấp ứng dụng này cho người dân và chịu trách nhiệm về ứng dụng.",
    );
    // `skills/administrative-language`: the unit is lower case after "Ủy ban nhân dân".
    expect(text).not.toContain("Ủy ban nhân dân Xã");
    expect(text).toContain(`Phiên bản ${COMMUNE_TERMS_VERSION} — Hiệu lực từ ngày ${COMMUNE_TERMS_EFFECTIVE_DATE}`);
    expect(COMMUNE_TERMS_VERSION).toBe("1.0");
    expect(COMMUNE_TERMS_EFFECTIVE_DATE).toBe("02/10/2026");
  });

  it("names no company anywhere — not ViHAT, not VihatSoftware, not the shared app's publisher (owner, 02/10/2026)", () => {
    // The whole rendered file, banner included.
    const whole = ketXuatVanBan(THANG_BINH_TERMS());
    expect(whole).not.toMatch(/vihat/i);
    expect(whole).not.toContain(COMPANY.name);
    expect(whole).not.toMatch(/tập đoàn|công ty|doanh nghiệp/i);
    // The measure measures: the shared terms DO carry the name.
    expect(ketXuatVanBan(VAN_BAN_DIEU_KHOAN)).toMatch(/ViHAT/);
  });

  it("the contact section gives the owner's sentence with the commune's introduction page", () => {
    const contact = THANG_BINH_TERMS().muc.find((m) => m.ma === "luat-va-lien-he")!;
    const sentence = `Mọi câu hỏi hoặc khiếu nại liên quan tới ứng dụng và nội dung của xã, xin liên hệ Ủy ban nhân dân xã Thăng Bình theo thông tin tại trang Giới thiệu của xã: ${THANG_BINH_URL}`;
    expect(contact.doan).toContain(sentence);
    // The URL ends its paragraph — no full stop to be copied with it.
    expect(contact.doan.at(-1)).toBe(sentence);
  });

  it("carries no phone number and no e-mail address — the only contact route is the introduction page", () => {
    const text = readerText(THANG_BINH_TERMS());
    const PHONE = /(?:\+84|\b0)\d(?:[ .-]?\d){7,9}\b/;
    const EMAIL = /[^\s@]+@[^\s@]+\.[^\s@]+/;
    expect(text).not.toMatch(PHONE);
    expect(text).not.toMatch(EMAIL);
    // The measure measures.
    expect("Gọi 0900000000").toMatch(PHONE);
    expect("Gọi 0900 000 000").toMatch(PHONE);
    expect("viết tới hop-thu@xa-thu.example").toMatch(EMAIL);
    // The three emergency numbers are short codes, not a contact line — and they are the send screen's sentence.
    expect(COMMUNE_TERMS_EMERGENCY).toBe(KHAN_CAP);
    expect(text).toContain(KHAN_CAP);
  });

  it("states no data recipient — it points to the privacy policy (open question #28)", () => {
    const text = readerText(THANG_BINH_TERMS());
    expect(text).toContain("Chính sách quyền riêng tư");
    expect(text).not.toMatch(/máy chủ của \S/);
  });

  it("says a petition here is not a complaint, a denunciation or an administrative procedure", () => {
    const send = THANG_BINH_TERMS().muc.find((m) => m.ma === "gui-phan-anh")!;
    expect(send.doan.join(" ")).toContain(
      "không phải là đơn khiếu nại hay đơn tố cáo, và không thay thế các thủ tục, hồ sơ hành chính theo quy định",
    );
  });

  it("cites no law but Decree 13/2023", () => {
    const text = readerText(THANG_BINH_TERMS());
    // `\w` would stop at "Đ" of "NĐ-CP": the citation runs to the next space, minus a sentence's full stop.
    const cited = [...text.matchAll(/(?:Nghị định|Luật|Thông tư|Quyết định)\s+(?:số\s+)?\d[^\s,;)]*/gu)].map((m) =>
      m[0].replace(/\.$/, ""),
    );
    expect(cited).toEqual(["Nghị định 13/2023/NĐ-CP"]);
  });

  it("a missing commune, or any missing value, FAILS — no default, no placeholder printed", () => {
    expect(() => communeTermsValuesFor({}, "xa-khac.vigov.example")).toThrow(
      /COMMUNE_TERMS_BY_DOMAIN\["xa-khac\.vigov\.example"\]/,
    );
    for (const key of ["displayName", "province", "introductionUrl"] as const) {
      const rest: Partial<CommuneTermsValues> = { ...VALID };
      delete rest[key];
      expect(() => communeTermsValuesFor({ x: rest }, "x"), `${key} absent`).toThrow(new RegExp(`${key}: chưa điền`));
      expect(() => communeTermsValuesFor({ x: { ...VALID, [key]: "  " } }, "x"), `${key} blank`).toThrow(
        new RegExp(key),
      );
      expect(() => communeTermsValuesFor({ x: { ...VALID, [key]: "<CHUA-CO>" } }, "x"), `${key} placeholder`).toThrow(
        /chỗ giữ chỗ/,
      );
      // The renderer refuses on its own too: a caller skipping the lookup still cannot print a hole.
      expect(() => communeTerms({ ...VALID, [key]: "" }), `${key} empty at render`).toThrow();
    }
    expect(() => communeTermsValuesFor({ x: { ...VALID, introductionUrl: "http://xa-thu.example.gov.vn" } }, "x")).toThrow(
      /https/,
    );
    expect(() => communeTermsValuesFor({ x: { ...VALID, introductionUrl: "xa-thu.example.gov.vn" } }, "x")).toThrow(/https/);
    expect(() => communeTermsValuesFor({ x: { ...VALID, displayName: "Thăng Bình" } }, "x")).toThrow(
      /Xã \/ Phường \/ Đặc khu/,
    );
    // And a valid row passes, for each unit type.
    for (const displayName of ["Xã Thử", "Phường Thử", "Đặc khu Thử"]) {
      expect(() => communeTermsValuesFor({ x: { ...VALID, displayName } }, "x")).not.toThrow();
    }
    expect(readerText(communeTermsDocument({ ...VALID, displayName: "Phường Thử" }, "x"))).toContain(
      "Ủy ban nhân dân phường Thử",
    );
  });

  it("the shared app's terms are unchanged: ViHAT Group's text, version and date, from dieu-khoan.ts", () => {
    expect(VAN_BAN_DIEU_KHOAN.muc).toBe(MUC_DIEU_KHOAN);
    expect(VAN_BAN_DIEU_KHOAN.phien_ban).toBe(PHIEN_BAN_DIEU_KHOAN);
    expect(VAN_BAN_DIEU_KHOAN.ngay_hieu_luc).toBe(NGAY_HIEU_LUC_DIEU_KHOAN);
    expect(VAN_BAN_DIEU_KHOAN.ngay_hieu_luc).toBe("21/09/2026");
    expect(VAN_BAN_DIEU_KHOAN.nguon).toBe("citizen-app/src/content/dieu-khoan.ts");
    expect(VAN_BAN_DIEU_KHOAN.appLine).toBe(`Mini App ${COMPANY.name}`);
    expect(VAN_BAN_CHINH_SACH.appLine).toBe(`Mini App ${COMPANY.name}`);
    expect(ketXuatVanBan(VAN_BAN_DIEU_KHOAN)).toMatch(/Mini App giới thiệu của Tập đoàn ViHAT Group/);
  });
});
