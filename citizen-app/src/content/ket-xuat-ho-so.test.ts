import { describe, expect, it } from "vitest";

import { KHAI_BAO_LOI_GOI, type KhaiBaoLoiGoi } from "../features/tinh-nang/zalo-api";

import type { MucChinhSach } from "./chinh-sach-rieng-tu";
import {
  MOC_BAT_DAU,
  MOC_KET_THUC,
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
    for (const vb of [VAN_BAN_CHINH_SACH, VAN_BAN_DIEU_KHOAN]) {
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
    }
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
 * câu ĐÚNG: câu mở đầu của chính sách — *"Ứng dụng này không lưu … tới máy chủ của
 * VihatSoftware"* — nằm gọn trong một câu, không có dấu chấm nào ở giữa. Một dây bẫy kêu oan ở
 * đúng câu quan trọng nhất là một dây bẫy sẽ bị tắt trong lần dọn tiếp theo.
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

  it("VẪN GIỮ lời khai nơi nhận dữ liệu: máy chủ của VihatSoftware", () => {
    // ⚠ CHIỀU NGƯỢC VỚI CA TRÊN, VÀ ĐÓ LÀ CHỦ ĐÍCH — câu hỏi mở #28. Máy chủ đổi hai mã đăng
    // nhập là kho `vihat-miniapp`; ai vận hành nó sau khi chuyển quyền sở hữu app thì CHƯA AI
    // TRẢ LỜI. Một lượt tìm-thay "VihatSoftware" -> "ViHAT Group" trên bản kết xuất sẽ làm ca ở
    // trên xanh và biến văn bản thành lời khai SAI về nơi nhận dữ liệu cá nhân — đúng thứ Nghị
    // định 13/2023 nhắm tới, và là thứ không sửa lại được sau khi công bố.
    expect(
      ketXuatVanBan(VAN_BAN_CHINH_SACH),
      "lời khai nơi nhận dữ liệu đã bị đổi tên hoặc bị xoá khỏi bản gửi Zalo",
    ).toContain("máy chủ của VihatSoftware");
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
      "Ứng dụng này không lưu bất kỳ dữ liệu nào của bạn xuống máy, và chỉ gửi đi đúng MỘT việc: khi chính bạn bấm đăng nhập, nó gửi hai mã dùng một lần do Zalo cấp tới máy chủ của VihatSoftware.",
      "Khi bạn bấm đăng nhập, ứng dụng gửi hai mã ấy tới máy chủ của VihatSoftware — đơn vị thành viên của Tập đoàn ViHAT Group.",
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
});
