import { describe, expect, it } from "vitest";

import { thanYeuCau, TRUONG_GUI_DI } from "../api/hop-dong-yeu-cau";
import { CLIENT_ERROR_FIELDS, clientErrorBody } from "../features/dang-nhap/hop-dong";
import { introPermissionRows } from "../features/company-intro/QuanLyQuyenScreen";
import {
  DOAN_CHINH_SACH_TINH_NANG,
  DOAN_CHINH_SACH_TUNG_QUYEN,
} from "../features/tinh-nang/noi-dung";
import { TRUONG_THIEP_CUA_CHUNG_TOI } from "../features/tinh-nang/vcard";
import { KHAI_BAO_LOI_GOI } from "../features/tinh-nang/zalo-api";

import {
  CAU_DAU,
  MUC_CHINH_SACH,
  NGAY_HIEU_LUC,
  PHIEN_BAN_CHINH_SACH,
} from "./chinh-sach-rieng-tu";
import { COMPANY } from "./company-profile";
import { cauKhaiDichRaNgoai, DICH_MO_RA_NGOAI, INTRO_DESTINATIONS } from "./dich-ra-ngoai";

/**
 * CHÍNH SÁCH QUYỀN RIÊNG TƯ — PHÉP KIỂM CỦA MỘT VĂN BẢN PHÁP LÝ, KHÔNG PHẢI CỦA MỘT MÀN HÌNH.
 *
 * Nguyên tắc chi phối `chinh-sach-rieng-tu.ts` là: **chính sách phải mô tả ĐÚNG bản dựng nó nằm
 * trong**. Nguyên tắc ấy không tự giữ được — nó hỏng theo đúng một kiểu, và kiểu ấy im lặng:
 * hành vi của mã đổi, câu chữ ở lại, và không có gì đỏ lên. Đã xảy ra hai lần trong kho này
 * ("không đọc thư viện ảnh", rồi "không có ô đăng nhập"), cả hai lần đều được
 * phát hiện bằng mắt người chứ không bằng phép kiểm. Tệp này là phép kiểm ấy.
 *
 * ⚠ BA CA DƯỚI ĐÂY ĐỀU ĐỎ KHI HÀNH VI ĐỔI, và đó là chủ đích — cái đỏ là cuộc trò chuyện phải
 * xảy ra TRƯỚC khi hành vi mới ra người dùng, không phải sau.
 */

const toanVan = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");

/** Mục "Cách xử lý và thời gian lưu" — hai describe bên dưới đều đọc nó, nên nó ở cấp mô-đun. */
const cachThuc = () => {
  const muc = MUC_CHINH_SACH.find((m) => m.ma === "cach-thuc");
  expect(muc, "chính sách không còn mục nào về cách xử lý và thời gian lưu").toBeDefined();
  return muc!.doan.join("\n");
};

describe("chính sách mô tả đúng thứ ứng dụng thật sự làm", () => {
  it("KHÔNG còn câu 'không gửi đi bất kỳ dữ liệu nào' — ứng dụng nay gửi hai mã đăng nhập", () => {
    // CÂU NÀY TỪNG ĐÚNG, VÀ LÀ CÂU MẠNH NHẤT ỨNG DỤNG CÓ ĐỂ NÓI. Nó thành sai vào 20/09/2026,
    // ngày bản nộp bắt đầu gọi máy chủ thật. Một câu đúng ở bản trước mà không ai sửa khi hành
    // vi đổi là loại lỗi nặng nhất một văn bản như thế này mắc phải.
    for (const cau of [CAU_DAU, toanVan]) {
      expect(cau, "chính sách vẫn tuyên bố không gửi gì đi").not.toMatch(
        /không gửi đi bất kỳ dữ liệu nào/,
      );
    }
  });

  it("nói ra ĐỦ BỐN điều của bước đăng nhập: gửi gì · ai nhận · lưu gì · vì sao", () => {
    const muc = MUC_CHINH_SACH.find((m) => m.ma === "dang-nhap");
    expect(muc, "chính sách không còn mục nào về đăng nhập").toBeDefined();
    const chu = muc!.doan.join("\n");

    expect(chu, "không nói GỬI GÌ").toMatch(/hai mã/);
    expect(chu, "không nói AI NHẬN").toMatch(/máy chủ của Tập đoàn ViHAT Group\./);
    expect(chu, "không nói MÁY CHỦ LƯU SỐ ĐIỆN THOẠI").toMatch(/LƯU SỐ ĐIỆN THOẠI/);
    expect(chu, "không nói VÌ SAO — đăng nhập và thông báo ZNS").toMatch(/tên đăng nhập/);
    expect(chu).toMatch(/ZNS/);
    // Và vẫn nói ra điều đúng: số điện thoại KHÔNG nằm trong hai mã gửi đi.
    expect(chu).toMatch(/không nằm trong hai mã/);
  });

  /**
   * ⚠ KHOÁ HAI CHIỀU GIỮA THÂN YÊU CẦU VÀ VĂN BẢN CHÍNH SÁCH — DÂY BẪY ĐẮT NHẤT CỦA LƯỢT 22/09.
   *
   *   Từ 22/09/2026 ứng dụng thu thập DỮ LIỆU BÁN HÀNG, không chỉ định danh. Chế độ hỏng ở đây im
   *   lặng theo đúng kiểu đã xảy ra HAI lần trong kho này ("không đọc thư viện ảnh", rồi "không có
   *   ô đăng nhập"): mã gửi thêm một trường, câu chữ ở lại, và không có gì đỏ lên. Khai thiếu một
   *   loại dữ liệu đang được thu thập là đúng thứ Nghị định 13 nhắm tới, và không sửa lại được sau
   *   khi công bố.
   *
   *   HAI CA DƯỚI ĐÂY LÀ HAI CHIỀU CỦA CÙNG MỘT KHOÁ, và thiếu một chiều là mất cả cơ chế:
   *     • chiều "mã đi trước" — thêm một khoá vào `thanYeuCau` mà quên khai;
   *     • chiều "văn bản đi sau" — dọn một mục khỏi chính sách trong khi mã vẫn gửi trường ấy.
   *   Chiều thứ hai mới là chiều người ta dọn văn bản cho gọn, và nó là chiều không ai nghĩ tới.
   */
  it("MỌI trường thân yêu cầu gửi đi đều có một dòng khai trong chính sách", () => {
    // 07/10/2026: the body's keys depend on the kind — `displayName` rides on `chat` only. So "what the body
    // sends" is the UNION over the shapes it can take: a consult, and a chat carrying a name. Measuring one kind
    // alone would call `displayName` "declared but never sent", or miss it entirely.
    const khoa_gui_di = [
      ...new Set(
        [
          thanYeuCau({ loai: "consult", quan_tam: ["messaging"], quy_mo: "10-50", ghi_chu: "x", nguon: "" }),
          thanYeuCau({ loai: "chat", quan_tam: [], quy_mo: "", ghi_chu: "", nguon: "", display_name: "Nguyễn Văn Thử" }),
        ].flatMap((than) => Object.keys(JSON.parse(than) as Record<string, unknown>)),
      ),
    ];
    expect(khoa_gui_di.length, "thân yêu cầu không còn trường nào để đo").toBeGreaterThan(0);

    const da_khai = new Set(TRUONG_GUI_DI.map((t) => t.khoa));
    for (const khoa of khoa_gui_di) {
      expect(
        da_khai,
        `thân yêu cầu gửi đi trường "${khoa}" mà chính sách quyền riêng tư KHÔNG khai. Thêm một ` +
          "dòng vào TRUONG_GUI_DI (api/hop-dong-yeu-cau.ts) — đừng sửa ca này.",
      ).toContain(khoa);
    }
    // Và chiều ngược của chính vế này: không khai thừa một trường mã KHÔNG gửi. Khai thừa là mô
    // tả sai theo hướng khác, và nó làm người đọc tưởng mình đã cho đi nhiều hơn thực tế.
    for (const truong of TRUONG_GUI_DI) {
      expect(
        khoa_gui_di,
        `chính sách khai trường "${truong.khoa}" mà thân yêu cầu không hề gửi`,
      ).toContain(truong.khoa);
    }
  });

  it("MỌI dòng khai ấy có mặt NGUYÊN VĂN trong văn bản chính sách", () => {
    // Chiều thứ hai của khoá. Không có ca này thì gỡ hẳn mục "Yêu cầu tư vấn" khỏi
    // `MUC_CHINH_SACH` vẫn xanh: bảng khai còn nguyên, chỉ là không ai đọc nó ra nữa.
    expect(TRUONG_GUI_DI.length, "bảng khai rỗng — ca này sẽ xanh vì không tìm thấy gì").toBeGreaterThan(0);
    for (const truong of TRUONG_GUI_DI) {
      expect(
        toanVan,
        `văn bản chính sách không còn nói tới: ${truong.trong_chinh_sach.slice(0, 40)}…`,
      ).toContain(truong.trong_chinh_sach);
    }
  });

  it("mục về bề mặt yêu cầu nói đủ: phải đăng nhập · ZNS không chắc chắn · trần gọi lại · 24 tháng", () => {
    const muc = MUC_CHINH_SACH.find((m) => m.ma === "yeu-cau-tu-van");
    expect(muc, "chính sách không còn mục nào về bề mặt yêu cầu").toBeDefined();
    const chu = muc!.doan.join("\n");

    expect(chu, "không nói phải đăng nhập trước").toMatch(/đăng nhập trước/);
    expect(chu, "không khai việc gửi tin ZNS xác nhận").toMatch(/ZNS/);
    // ⚠ ZNS KHÔNG ĐƯỢC HỨA CHẮC. Nó có trần (`zns_da_gui.ket_qua` có giá trị `bo_qua_vuot_tran`)
    // và có thể tắt. Một câu "Bạn sẽ nhận được tin" là một lời hứa mã không giữ được, và người
    // phát hiện ra là người ngồi chờ một tin không tới.
    expect(chu, "hứa chắc rằng sẽ có tin ZNS").not.toMatch(/[Bb]ạn sẽ nhận được (một )?tin/);
    expect(chu, "không nói ra rằng không nhận được tin vẫn là bình thường").toMatch(
      /không nhận được tin/,
    );
    expect(chu, "không nói trần gọi lại").toMatch(/3 lần trong 24 giờ/);
    expect(chu, "không nói hạn lưu 24 tháng").toMatch(/24 THÁNG|24 tháng/);
    // ⚠ 24 THÁNG LÀ ẨN DANH HOÁ, KHÔNG PHẢI XOÁ HÀNG. `an_danh_yeu_cau_qua_han()` đặt `ghi_chu`
    // thành NULL và GIỮ hàng. Viết "chúng tôi xoá dữ liệu của bạn" là mô tả một cơ chế không tồn
    // tại, và sai về phía hứa nhiều hơn thứ mã làm — đúng loại lỗi mục `cach-thuc` đã mắc một lần.
    expect(chu, "không nói ra rằng sau 24 tháng là ẩn danh hoá chứ không xoá hàng").toMatch(
      /ẩn danh/,
    );
    // KHÔNG MỘT CAM KẾT THỜI GIAN NÀO cho cuộc gọi lại.
    expect(chu, "chính sách in một cam kết thời gian cho cuộc gọi lại").not.toMatch(
      /trong \d+ (giây|phút)/,
    );
  });

  it("câu mở đầu nói ngay việc gửi đi, không giấu xuống dưới", () => {
    // Người đọc một chính sách quyền riêng tư đọc câu đầu rồi thôi. Giấu hành vi gửi dữ liệu
    // xuống mục thứ tư là đúng thứ Nghị định 13 nhắm tới.
    expect(CAU_DAU).toMatch(/gửi/);
    expect(CAU_DAU).toMatch(/đăng nhập/);
  });

  it("MỘT số phiên bản — văn bản này chưa từng tới tay một người dùng nào", () => {
    // ĐÃ KIỂM 20/09/2026, BẰNG CHỨNG CHỨ KHÔNG PHẢI SUY ĐOÁN: `git log -- citizen-app` không có
    // một lần phát hành nào · sổ tiến độ mục `nop-zalo-duyet` = `chua_lam` · README §"Còn thiếu"
    // #1 ghi ảnh chụp màn hình và mô tả store CHƯA CÓ, mà Zalo bắt buộc phải có mới xét duyệt ·
    // README §"Hai thứ đã kiểm bằng cách chạy thật" ghi rằng đường công khai trả "ứng dụng đang
    // trong giai đoạn phát triển", tức chưa phát hành; chỉ có bản THỬ NGHIỆM Version 6–7.
    //
    // Trong một ngày soạn thảo văn bản đã đi 1.0 → 1.5. Lý do tồn tại của việc lên số là "một
    // người đã bấm đồng ý ở bản cũ không biết về thứ mới" — không có người ấy thì sáu số chỉ kể
    // lại quá trình soạn thảo trong một văn bản pháp lý, và làm người đọc tưởng đã có sáu đợt
    // thay đổi được công bố. Quá trình soạn thảo nằm trong `git log`, đúng chỗ của nó.
    //
    // ⚠ TỪ LẦN PHÁT HÀNH ĐẦU TIÊN TRỞ ĐI, QUY TẮC ĐẢO NGƯỢC: mỗi thay đổi hành vi xử lý dữ liệu
    // phải lên một số mới và KHÔNG được gộp lại nữa. Ca này khi ấy đổi thành "không lùi số".
    expect(PHIEN_BAN_CHINH_SACH).toBe("1.0");
    // 12/10/2026: the day the text became what it says (consultation photos, nearest office) — still 1.0.
    expect(NGAY_HIEU_LUC).toBe("12/10/2026");
  });

  it("mỗi mục có mã riêng và không mục nào rỗng", () => {
    const ma = MUC_CHINH_SACH.map((m) => m.ma);
    expect(new Set(ma).size, "hai mục trùng mã — React sẽ vẽ sai khoá").toBe(ma.length);
    for (const m of MUC_CHINH_SACH) {
      expect(m.tieu_de.trim().length, `mục ${m.ma} không có tiêu đề`).toBeGreaterThan(0);
      expect(m.doan.length, `mục ${m.ma} rỗng`).toBeGreaterThan(0);
    }
  });
});

/**
 * THỜI GIAN LƯU — CHỖ TRỐNG ĐÃ ĐƯỢC LẤP, VÀ CA CANH ĐỔI THEO (20/09/2026).
 *
 * Bản 1.3 không nêu thời hạn nào vì chưa ai chốt, và ca ở đây canh cho chỗ trống ấy không bị
 * lấp bằng một con số tự nghĩ ra. Khách đã chốt: **không có hạn tự động, giữ tới khi người dùng
 * yêu cầu xoá** — nên ca cũ hết việc và được thay bằng ca canh chính CAM KẾT mới.
 *
 * ⚠ "GIỮ TỚI KHI BẠN YÊU CẦU XOÁ" LÀ MỘT CAM KẾT, KHÔNG PHẢI MỘT CÁCH NÉ CON SỐ — và một cam
 * kết không có cửa thực hiện là một lời hứa suông. Ba vế dưới đây là ba vế Nghị định 13 đòi, và
 * mỗi vế có một ca riêng để mất một vế thì đỏ đúng chỗ, không đỏ chung chung.
 */
describe("thời gian lưu ở máy chủ — cam kết, và cửa để thực hiện nó", () => {
  it("nói THẲNG rằng không có hạn tự động, và giữ tới khi người dùng yêu cầu xoá", () => {
    // Im lặng về thời hạn và "giữ tới khi bạn yêu cầu xoá" là HAI THỨ KHÁC NHAU với người đọc.
    // Một văn bản im lặng để người ta tự đoán; văn bản này nói ra, nên nó phải nói ra thật.
    const chu = cachThuc();
    expect(chu, "không nói ra rằng KHÔNG có hạn tự động").toMatch(/KHÔNG có hạn tự động/);
    expect(chu, "không nói ra mốc kết thúc việc lưu").toMatch(/tới khi chính bạn yêu cầu xoá/);
  });

  it("chỉ ra CỬA để yêu cầu xoá — hotline và email đã có sẵn, không phải một địa chỉ mới", () => {
    // Hứa một quyền mà không chỉ ra cửa thực hiện là hứa suông, và đó đúng là thứ Nghị định 13
    // đòi phải có. Cửa ấy cũng phải là cửa CÓ THẬT trong ứng dụng: hotline và email nằm ngay
    // trên màn Liên hệ, không phải một hòm thư dựng riêng cho một dòng trong chính sách.
    const chu = cachThuc();
    expect(chu).toMatch(/hotline/);
    expect(chu).toMatch(/email/);
    expect(chu).toMatch(/màn Liên hệ/);
    // Không đặt thêm rào: không bắt đăng nhập, không bắt nêu lý do.
    expect(chu).toMatch(/không cần đăng nhập/);
  });

  it("nói XOÁ THÌ XOÁ CÁI GÌ — và KHÔNG hứa xoá thứ lược đồ không cho xoá", () => {
    // ĐÃ SỬA Ở 1.5 SAU KHI ĐỌC LƯỢC ĐỒ THẬT. Bản 1.4 hứa xoá "số điện thoại VÀ BẢN GHI ĐỊNH
    // DANH". Lược đồ (vihat-miniapp/migrations/0001_init.sql): nhật ký đăng nhập tham chiếu bản
    // ghi định danh và là bảng CHỈ GHI THÊM (trigger chặn UPDATE/DELETE), nên CSDL TỪ CHỐI xoá
    // hàng định danh của bất cứ ai từng đăng nhập thành công. Lời hứa ấy không giữ được.
    //
    // Ca này canh theo CHIỀU NGƯỢC: văn bản phải nói xoá SỐ ĐIỆN THOẠI, và không được quay lại
    // hứa xoá cả bản ghi. Một lời hứa mạnh hơn thứ mã làm được là một tuyên bố sai dưới tên một
    // pháp nhân — nghe hay hơn, và sai.
    const chu = cachThuc();
    expect(chu).toMatch(/KHI XOÁ, CHÚNG TÔI XOÁ SỐ ĐIỆN THOẠI CỦA BẠN/);
    expect(chu, "hứa lại việc xoá bản ghi định danh — lược đồ không cho").not.toMatch(
      /xoá số điện thoại của bạn và bản ghi định danh/,
    );
    expect(chu, "không nói ra rằng nhật ký cũ vẫn còn").toMatch(/dòng nhật ký cũ vẫn trỏ/);
  });

  it("nói THẲNG trường hợp người CHƯA TỪNG đăng nhập thành công", () => {
    // Người ấy không có bản ghi định danh nào để yêu cầu xoá, nhưng IP của họ VẪN nằm trong
    // nhật ký. Gói tình huống này vào một câu chung là để họ tự suy ra — và suy ra sai theo
    // hướng có lợi cho chúng tôi. Nói thẳng cả ba vế: không có gì để xoá · nhật ký không xoá
    // được · chúng tôi cũng không biết dòng nào là của họ.
    const chu = cachThuc();
    expect(chu).toMatch(/NẾU BẠN CHƯA TỪNG ĐĂNG NHẬP THÀNH CÔNG/);
    expect(chu).toMatch(/không có bản ghi định danh nào của bạn để mà xoá/);
    expect(chu).toMatch(/không xoá riêng những dòng ấy theo yêu cầu được/);
  });
});

/**
 * NHẬT KÝ ĐĂNG NHẬP — MỘT HÀNH VI CỦA MÁY CHỦ MÀ BẢN NHÁP ĐẦU HOÀN TOÀN KHÔNG KHAI.
 *
 * Địa chỉ IP là thứ máy chủ THẤY từ chính lời gọi, không phải thứ ứng dụng gửi lên — nên câu mở
 * đầu *"gửi đi đúng MỘT việc"* vẫn đúng từng chữ. Nhưng im lặng về việc nó được GHI LẠI là giấu
 * một hành vi mà hệ thống CÓ, và đó là vi phạm chính Nghị định 13, không phải một thiếu sót nhỏ.
 *
 * Ca này là thứ giữ cho một lần "dọn câu chữ" sau này không bỏ mất dòng ấy.
 */
describe("nhật ký đăng nhập — khai ra, kèm mục đích", () => {
  it("khai rằng máy chủ ghi thời điểm và địa chỉ IP, và nói để làm gì", () => {
    const chu = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");
    expect(chu, "không khai việc ghi địa chỉ IP").toMatch(/địa chỉ IP/);
    expect(chu, "không khai việc ghi thời điểm đăng nhập").toMatch(/thời điểm/);
    expect(chu, "khai việc ghi nhưng không nói MỤC ĐÍCH").toMatch(/lạm dụng/);
  });

  it("nói VẾ NẶNG NHẤT thành một đoạn riêng: lượt THẤT BẠI cũng bị ghi IP", () => {
    // ĐÂY LÀ VẾ DỄ BỊ GÓI VÀO MỘT MỆNH ĐỀ PHỤ NHẤT, VÀ LÀ VẾ NẶNG NHẤT: nó có nghĩa là IP của
    // một người CHƯA TỪNG có tài khoản vẫn nằm trong cơ sở dữ liệu của chúng tôi. Người đọc câu
    // "mỗi lần bạn đăng nhập" hiểu là lần THÀNH CÔNG — hiểu sai đúng nửa quan trọng.
    const chu = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");
    expect(chu).toMatch(/KỂ CẢ LƯỢT KHÔNG THÀNH CÔNG/);
    expect(chu, "không nói ra hệ quả: chưa từng có tài khoản vẫn bị ghi IP").toMatch(
      /chưa từng đăng nhập thành công lần nào và chưa từng có tài khoản/,
    );
  });

  it("liệt kê ĐỦ những gì một dòng nhật ký chứa, gồm cả kết quả và mã lý do", () => {
    // Khai "có ghi nhật ký" mà không nói nó chứa gì là khai một nửa. Bốn thứ dưới đây đọc thẳng
    // từ lược đồ: tao_luc · dia_chi_ip · ket_qua · ly_do (+ nguoi_dung_id khi thành công).
    const chu = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");
    expect(chu).toMatch(/lượt ấy thành công hay không/);
    expect(chu).toMatch(/mã lý do ngắn/);
    expect(chu).toMatch(/mã định danh nội bộ của bạn NẾU lượt ấy thành công/);
    expect(chu, "không nói nhật ký là loại chỉ ghi thêm").toMatch(/CHỈ GHI THÊM/);
  });

  it("khai các mốc thời gian và bản băm phiếu phiên — không bỏ sót một cột nào của lược đồ", () => {
    // Văn bản này đang liệt kê từng thứ máy chủ giữ. Bỏ đúng một mục ra khỏi một danh sách đầy
    // đủ là mời câu hỏi "còn bỏ gì nữa" ở đúng vòng duyệt.
    const chu = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");
    expect(chu).toMatch(/thời điểm nó được tạo và lần gần nhất được cập nhật/);
    expect(chu).toMatch(/bản mã hoá một chiều của chính phiếu/);
    expect(chu, "không nói phiên hết hạn sau bao lâu").toMatch(/hết hạn sau 7 ngày/);
  });

  it("nói ra cả những thứ máy chủ KHÔNG lưu — phần người đọc lo nhất", () => {
    const chu = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");
    for (const khong of [
      "không tên",
      "không email",
      "không vị trí",
      "không thông tin thiết bị",
      "không danh bạ",
      "không ảnh",
    ]) {
      expect(chu, `thiếu vế "${khong}"`).toContain(khong);
    }
    expect(chu, "không nói ra rằng không có công cụ đo hành vi / SDK bên thứ ba").toMatch(
      /không cài công cụ đo hành vi nào và không nhúng bộ công cụ của bên thứ ba/,
    );
  });

  /**
   * THỜI HẠN LƯU NHẬT KÝ — ĐÃ CHỐT: 90 NGÀY. Chỗ trống thứ hai được lấp, hằng canh nó biến mất,
   * và ca canh đổi thành ca canh chính CAM KẾT — đúng bốn việc mà cơ chế ấy đòi.
   *
   * ⚠ VẾ "KỂ CẢ DÒNG CỦA LƯỢT THẤT BẠI" LÀ VẾ PHẢI CÓ, không phải một chi tiết thêm vào: nó là
   * câu trả lời DUY NHẤT cho người chưa từng có tài khoản. Họ không yêu cầu xoá được (không có
   * gì để đối chiếu, và nhật ký chỉ ghi thêm), nên nếu 90 ngày không áp cho dòng của họ thì IP
   * ấy nằm lại vĩnh viễn — và văn bản sẽ đang hứa một thứ không đúng với chính người dễ tổn
   * thương nhất trong ba nhóm người đọc nó.
   */
  it("90 ngày là TRẦN, không phải một cái mốc đúng ngày — và văn bản nói ra cả cận dưới", () => {
    // CÂU NÀY TỪNG SAI, VÀ SAI VỀ PHÍA GIỮ LÂU HƠN — loại sai tệ nhất trong một cam kết lưu trữ.
    // Cơ chế dọn thật (`vihat-miniapp/migrations/0002_…sql`, `nhat_ky_don_qua_han`) là DROP cả
    // một phân mảnh TUẦN khi đầu khoảng của nó quá hạn (`d <= nguong`), nên mỗi dòng sống tối
    // đa 90 ngày và tối thiểu 83. Viết "sau 90 ngày, dòng nhật ký được xoá" là mô tả một cơ chế
    // xoá-đúng-ngày không hề tồn tại.
    //
    // Ba vế, ba `expect`, để mất vế nào thì đỏ đúng vế ấy.
    const chu = cachThuc();
    expect(chu, "không nói 90 ngày là mức TRẦN").toMatch(/CHẬM NHẤT 90 NGÀY/);
    expect(chu, "không nói cơ chế dọn theo lô tuần").toMatch(/theo từng lô mỗi tuần/);
    expect(chu, "không nói cận dưới — một dòng có thể bị xoá sớm hơn").toMatch(
      /SỚM HƠN — sớm nhất là ngày thứ 83/,
    );
    expect(chu, "không khẳng định cái trần").toMatch(/Không dòng nào sống quá 90 ngày/);
  });

  it("KHÔNG quay lại cách viết 'đúng 90 ngày' — cách viết ấy mô tả một cơ chế không tồn tại", () => {
    // Ca canh theo CHIỀU NGƯỢC, vì câu sai cũ đọc trôi hơn câu đúng: một lượt "biên tập cho gọn"
    // sẽ rút về "sau 90 ngày, dòng nhật ký được xoá" mà không ai thấy mình vừa đổi một cam kết.
    const chu = cachThuc();
    expect(chu, "đã quay lại cách viết xoá-đúng-ngày").not.toMatch(
      /[Ss]au 90 ngày, dòng nhật ký được xoá/,
    );
  });

  it("thời hạn ấy áp cho CẢ dòng của lượt thất bại", () => {
    const chu = cachThuc();
    expect(chu, "không nói 90 ngày áp cho MỌI dòng").toMatch(/áp cho MỌI dòng/);
    expect(chu, "không nói ra vế lượt thất bại").toMatch(
      /kể cả những dòng của một lượt đăng nhập không thành công/,
    );
  });

  /**
   * DÒNG BẰNG CHỨNG CỦA MỘT LẦN XOÁ — `nhat_ky_an_danh`.
   *
   * Đây là dữ liệu DUY NHẤT về một người còn ở lại **sau khi** họ đã yêu cầu xoá, và là chỗ dễ
   * im lặng nhất của cả văn bản: nói ra thì phải giải thích, không nói thì không ai biết. Người
   * đọc không nên phát hiện ra nó qua một đường khác — nên nó có ca kiểm riêng, và ca ấy đòi cả
   * bốn vế: có bảng đó · nó chứa gì · KHÔNG chứa số điện thoại · giữ vô thời hạn VÀ vì sao.
   */
  it("khai dòng bằng chứng của một lần xoá, đủ bốn vế", () => {
    const chu = cachThuc();
    expect(chu, "không khai dòng bằng chứng").toMatch(/CHÚNG TÔI GIỮ LẠI MỘT DÒNG BẰNG CHỨNG/);
    expect(chu, "không nói nó chứa gì").toMatch(/hotline hay qua email/);
    expect(chu).toMatch(/tên người tiếp nhận và thực hiện/);
    expect(chu, "không nói nó KHÔNG chứa số điện thoại").toMatch(
      /KHÔNG chứa số điện thoại của bạn/,
    );
    expect(chu, "không nói nó giữ vô thời hạn").toMatch(/giữ VÔ THỜI HẠN, nằm ngoài quy tắc 90 ngày/);
    expect(chu, "giữ vô thời hạn mà không nói VÌ SAO").toMatch(
      /bằng chứng tự huỷ sau một thời gian thì không còn là bằng chứng/,
    );
  });

  it("nói ra rằng yêu cầu xoá làm các máy đang đăng nhập bị đăng xuất", () => {
    // Lệnh `an-danh` thu hồi mọi phiên còn hiệu lực trong CÙNG một giao dịch với việc ghi đè số
    // điện thoại (`internal/store/an_danh.go`). Người dùng sẽ thấy mình bị đăng xuất; biết
    // trước thì đó là hệ quả đã báo, không biết thì đó là một sự cố họ tự đoán nguyên nhân.
    const chu = cachThuc();
    expect(chu).toMatch(/mọi phiên đăng nhập còn hiệu lực của bạn bị thu hồi/);
    expect(chu).toMatch(/sẽ bị đăng xuất/);
  });

  it("người chưa từng đăng nhập thành công được chỉ sang đúng lối 90 ngày", () => {
    // Đoạn ấy nói "không xoá riêng theo yêu cầu được". Nếu dừng ở đó thì người đọc kết luận IP
    // của họ nằm lại mãi mãi — một kết luận sai, và sai theo hướng làm họ mất lòng tin. Hai vế
    // phải đi liền nhau trong cùng một đoạn.
    const chu = cachThuc();
    expect(chu).toMatch(/không xoá riêng những dòng ấy theo yêu cầu được/);
    expect(chu).toMatch(/tự hết hạn và bị xoá sau 90 ngày/);
  });

  it("nói rõ nhật ký ấy KHÔNG chứa số điện thoại — đó là điều kiện để nó được ở lại sau khi xoá", () => {
    const chu = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");
    expect(chu).toMatch(/Nhật ký KHÔNG chứa số điện thoại của bạn/);
  });

  it("no sentence about network type is left — the feature was removed 11/10/2026", () => {
    // The one sentence that said "ứng dụng không nhận địa chỉ IP" was about `getNetworkType`; with the call gone the
    // sentence must go too, or it describes a read the app no longer makes.
    const chu = [CAU_DAU, ...MUC_CHINH_SACH.flatMap((m) => m.doan)].join("\n");
    expect(chu).not.toMatch(/getNetworkType|[Kk]iểu kết nối|KIỂU kết nối/);
  });
});

/**
 * CHUYỂN QUYỀN SỞ HỮU APP (21/09/2026) — HAI VẾ PHẢI TÁCH BẠCH, VÀ CHÚNG HỎNG THEO HAI KIỂU:
 *
 *   | Vế | Hỏng kiểu gì nếu sai |
 *   |---|---|
 *   | AI CHỊU TRÁCH NHIỆM về chính sách | văn bản pháp lý đứng tên sai pháp nhân |
 *   | AI NHẬN dữ liệu đăng nhập | khai sai nơi nhận dữ liệu cá nhân — đúng thứ Nghị định 13 nhắm tới |
 *
 *   Hai vế vẫn là hai câu, dù từ 25/09/2026 chúng mang CÙNG một tên: câu mở #28 đã quyết bên
 *   vận hành `vihat-miniapp` và bên nhận dữ liệu là Tập đoàn ViHAT Group (ADR 0044 câu 2). Từ
 *   21/09 tới 25/09 ca cuối ghim "máy chủ của VihatSoftware" vì khi ấy chưa ai xác nhận; nay nó
 *   ghim chuỗi mới Ở TỪNG CHỖ khai nơi nhận, và cấm chuỗi cũ — kể cả lối viết "máy chủ của
 *   VihatSoftware — đơn vị thành viên của Tập đoàn ViHAT Group", vì câu ấy vẫn nêu VihatSoftware
 *   là nơi nhận. Tên VihatSoftware ở vai trò đơn vị thành viên (điều khoản, danh sách đơn vị)
 *   không thuộc ca này.
 */
describe("bên phát hành và bên nhận dữ liệu là HAI câu khác nhau", () => {
  it("nói bên chịu trách nhiệm về chính sách là ViHAT Group", () => {
    const muc = MUC_CHINH_SACH.find((m) => m.ma === "ben-xu-ly");
    expect(muc, "chính sách không còn mục nào về bên xử lý dữ liệu").toBeDefined();
    const chu = muc!.doan.join("\n");
    expect(chu).toMatch(/ViHAT Group/);
    expect(chu).toMatch(/bên phát hành ứng dụng này và là bên chịu trách nhiệm/);
  });

  it("KHÔNG còn câu nào nói VihatSoftware là bên phát hành ứng dụng", () => {
    // Câu ấy đúng tới 20/09/2026 và sai từ 21/09. Nó nằm trong CÙNG một văn bản với mục trên,
    // nên để lại là để văn bản tự mâu thuẫn — chỗ người duyệt đọc ra ngay.
    const chu = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");
    expect(chu, "văn bản vẫn nói VihatSoftware phát hành ứng dụng").not.toMatch(
      /VihatSoftware[^.]*bên phát hành/,
    );
  });

  it("khai nơi nhận dữ liệu là máy chủ của Tập đoàn ViHAT Group — ở ĐỦ mọi chỗ khai (#28)", () => {
    // ⚠ GHIM TỪNG CHỖ, KHÔNG GHIM "CÓ Ở ĐÂU ĐÓ": văn bản khai nơi nhận ở câu đầu, mục Đăng nhập
    // và mục Chuyển dữ liệu cho bên thứ ba. Sửa nửa vời — một chỗ mới, một chỗ cũ — là hai nơi nhận
    // khác nhau trong cùng thứ Nghị định 13 bắt khai, và một ca "toMatch trên toàn văn" vẫn xanh.
    const muc = (ma: string) => MUC_CHINH_SACH.find((m) => m.ma === ma)!.doan.join("\n");
    const choKhai: Record<string, string> = {
      "câu đầu": CAU_DAU,
      "mục Đăng nhập": muc("dang-nhap"),
      "mục Chuyển dữ liệu cho bên thứ ba": muc("ben-thu-ba"),
    };
    for (const [cho, chu] of Object.entries(choKhai)) {
      expect(chu, `${cho}: không khai nơi nhận là ViHAT Group`).toMatch(
        /máy chủ của Tập đoàn ViHAT Group/,
      );
    }
    // Mẫu của chính câu mở #28 (`code_signals`): chuỗi cũ còn sót ở BẤT KỲ đâu là một lời khai
    // sai nơi nhận — kể cả khi nó kèm "đơn vị thành viên của Tập đoàn ViHAT Group".
    const toanBo = [
      CAU_DAU,
      ...MUC_CHINH_SACH.flatMap((m) => m.doan),
    ].join("\n");
    expect(toanBo, "còn sót lời khai nơi nhận cũ").not.toMatch(
      /m[áa]y ch[ủu] c[ủu]a\s+vihat\s*software/i,
    );
  });
});

/**
 * TỆP `.vcf` — VĂN BẢN KHAI NÓ CHỨA GÌ, VÀ TẤM THIẾP THẬT SỰ CHỨA GÌ, LÀ MỘT.
 *
 *   Chính sách liệt kê từng trường của tệp ứng dụng ghi xuống máy người dùng. Tấm thiếp thì dựng
 *   từ `COMPANY` + `CONTACT`, và một trường không có nguồn (`URL`, từ 21/09/2026) bị bỏ hẳn. Hai
 *   nơi ấy lệch nhau một lần là văn bản khai một trường tệp không có — mô tả sai đúng thứ người
 *   dùng vừa tải về, và không có gì đỏ lên vì cả hai nửa đều "đúng" khi đọc riêng.
 */
describe("khai của tệp danh thiếp khớp với tấm thiếp thật", () => {
  const ghiTep = () => {
    const muc = MUC_CHINH_SACH.find((m) => m.ma === "ghi-tep");
    expect(muc, "chính sách không còn mục nào về tệp ghi xuống máy").toBeDefined();
    return muc!.doan.join("\n");
  };

  it("nói tệp là danh thiếp của ViHAT Group, đúng tên trên chính tấm thiếp", () => {
    expect(ghiTep()).toContain(COMPANY.name);
  });

  // HAI CÂU, KHÔNG PHẢI MỘT — và ca này chỉ canh một câu cho tới 21/09/2026.
  //
  // Cùng tấm thiếp được khai ở HAI chỗ: mục `ghi-tep` của chính sách, và đoạn "Với việc tải tệp"
  // trong `DOAN_CHINH_SACH_TINH_NANG`. Ca cũ chỉ hỏi mục `ghi-tep`, nên khi `COMPANY.website` được
  // điền lại thì câu trong chính sách đúng trở lại còn câu kia đứng yên — văn bản khai THIẾU một
  // trường mà tệp tải về CÓ, và không có gì đỏ.
  //
  // Nên phép kiểm hỏi CẢ HAI nguồn văn bản, cùng một câu hỏi. Thêm một chỗ khai thứ ba thì thêm
  // nó vào mảng này, đừng viết một ca thứ ba: một câu hỏi hỏi ở hai nơi là hai nơi có thể trả lời
  // khác nhau.
  const NOI_KHAI_TEP: readonly (readonly [string, () => string])[] = [
    ["chính sách, mục ghi-tep", ghiTep],
    ["đoạn tính năng 'Với việc tải tệp'", () =>
      DOAN_CHINH_SACH_TINH_NANG.filter((d) => d.includes("tải tệp")).join("\n")],
  ];

  it.each(NOI_KHAI_TEP)("%s — khai 'trang web' khi và chỉ khi tấm thiếp có dòng URL", (_ten, doc) => {
    const co_url = TRUONG_THIEP_CUA_CHUNG_TOI.some(
      (truong) => truong.ten === "URL" && truong.gia_tri !== "",
    );
    expect(co_url, "tấm thiếp có URL nhưng COMPANY.website trống, hoặc ngược lại").toBe(
      COMPANY.website !== undefined,
    );

    const van_ban = doc();
    expect(van_ban, "không tìm thấy câu khai tệp ở nguồn này — nó bị đổi chữ hay bị xoá?")
      .not.toBe("");

    if (co_url) {
      expect(van_ban, "tệp có trang web mà văn bản không khai").toMatch(/trang web/);
    } else {
      expect(van_ban, "văn bản khai một trang web mà tệp không chứa").not.toMatch(/trang web/);
    }
  });
});

/**
 * 11/10/2026 — TWO APPS TO THE USER, TWO POLICIES (owner). This text is the ViHAT Group app's: it names no commune,
 * no petition and no ViGov; it carries no "CHỜ DUYỆT" mark (the owner approved it); it says nothing about the
 * intro features removed that day. The commune interface's facts — and their fidelity locks — moved to
 * `commune-privacy.ts` / `commune-privacy.test.ts`.
 */
/** A whole word, case-insensitive, Unicode-aware (`` is ASCII-only and misses "xã" before a space). */
const word = (w: string) => new RegExp(`(?<!\\p{L})${w}(?!\\p{L})`, "iu");
const COMMUNE_WORDS: readonly RegExp[] = [
  word("xã"),
  word("phường"),
  /phản ánh/i,
  /ViGov/,
  /dịch vụ công/i,
  /hai cách dùng/,
];

describe("chính sách của ứng dụng ViHAT — không một chữ nào về xã, không dấu chờ duyệt", () => {
  const all = () =>
    [CAU_DAU, ...MUC_CHINH_SACH.flatMap((m) => [m.tieu_de, ...m.doan]), ...DOAN_CHINH_SACH_TINH_NANG, ...DOAN_CHINH_SACH_TUNG_QUYEN].join("\n");

  it("no commune word: xã · phường · phản ánh · ViGov · dịch vụ công · 'trang của một xã'", () => {
    const t = all();
    for (const banned of COMMUNE_WORDS) {
      expect(t, `the ViHAT policy still says ${banned}`).not.toMatch(banned);
    }
    // Must-still-catch: `\b` does not see a word ending in "ã", so the word pattern is a Unicode one — and it bites.
    expect(COMMUNE_WORDS.filter((w) => w.test("Gửi tới xã, rồi Phường xem."))).toHaveLength(2);
  });

  it("no CHỜ DUYỆT anywhere in user-visible policy text — the owner approved it (11/10/2026)", () => {
    expect(all(), "a pending-approval mark is back").not.toMatch(/CHỜ DUYỆT/);
    expect(all()).not.toMatch(/GIÁ TRỊ TẠM|CẦN CHỦ DỰ ÁN QUYẾT|bản nháp 02\/10\/2026/);
  });

  // `/văn phòng gần/` left this list 12/10/2026: "Tìm văn phòng gần tôi" came back (owner), this time with the location
  // exchanged on the server and a section of its own (`vi-tri-van-phong`, pinned below).
  it("no removed feature, and no build talk: paper card, network type, 'sáu tính năng', 'bản dựng'", () => {
    const t = all();
    for (const gone of [/thiếp giấy/i, /số hoá/i, /kiểu kết nối/i, /sáu tính năng/i, /bản dựng/i, /đầy đủ chức năng/i]) {
      expect(t, `the ViHAT policy still says ${gone}`).not.toMatch(gone);
    }
  });

  // 12/10/2026 (owner): the camera is the QR scan AND the consultation photos; the library is opened for those only.
  it("the intro's camera: the QR scan and the consultation photos, each on a tap; the library only for 'Chọn ảnh'", () => {
    const text = MUC_CHINH_SACH.find((m) => m.ma === "du-lieu")!.doan.join("\n");
    expect(text).toMatch(/để quét mã QR khi bạn bấm 'Quét mã QR'/);
    expect(text).toMatch(/để chụp ảnh gửi kèm một yêu cầu tư vấn khi bạn bấm 'Chụp ảnh' ở màn 'Tư vấn và báo giá'/);
    expect(text).toMatch(/chỉ mở thư viện ảnh khi bạn bấm 'Chọn ảnh'/);
    expect(text).toMatch(/không lưu ảnh nào trên máy bạn/);
    // The old absolute sentence is gone — it became false the day photos could be attached.
    expect(text).not.toMatch(/không đọc thư viện ảnh của bạn/);
  });

  it("the consultation photos paragraph says what, where, who, the metadata removal, retention and erasure", () => {
    const section = MUC_CHINH_SACH.find((m) => m.ma === "yeu-cau-tu-van")!.doan.join("\n");
    const photos = section.split("\n").find((d) => d.startsWith("ẢNH GỬI KÈM YÊU CẦU TƯ VẤN"));
    expect(photos, "the request section lost its photo paragraph").toBeDefined();
    for (const said of [
      /tối đa 3 ảnh, mỗi ảnh không quá 5 MB/, // what, and the caps the server enforces
      /chỉ nhận đúng những ảnh bạn chụp hoặc chọn/,
      /tới máy chủ của Tập đoàn ViHAT Group — cùng nơi lưu yêu cầu/, // where
      /chuyên viên của Tập đoàn ViHAT Group mở ảnh/, // who
      /đường tải ảnh chỉ dùng được trong 24 giờ/, // how: the request.photo_added webhook's signed link (vihat-miniapp 2cf31a30)
      /không kèm số điện thoại, tên hay ghi chú/,
      /bỏ thông tin vị trí chụp/, // EXIF/XMP stripped server-side (vihat-miniapp `internal/photo`)
      /lưu bằng thời hạn của yêu cầu — 24 tháng/, // retention
      /ảnh bị xoá cùng lúc với phần ghi chú/, // erasure
      /không gửi kèm đề nghị gọi lại/,
    ]) {
      expect(photos!, String(said)).toMatch(said);
    }
    // And the erasure section says the photos go too.
    expect(MUC_CHINH_SACH.find((m) => m.ma === "cach-thuc")!.doan.join("\n")).toMatch(
      /cùng mọi ảnh bạn đã gửi kèm yêu cầu tư vấn/,
    );
  });

  it("the per-permission list names exactly the intro's Console permissions — locked to `introPermissionRows`", () => {
    // `openPhone` and `openWebview` are `@zaloOnly` and need no Console permission (`zalo-api.ts` table header).
    const NOT_A_PERMISSION = new Set(["openPhone", "openWebview"]);
    const shown = introPermissionRows(KHAI_BAO_LOI_GOI).map((r) => r.api).filter((api) => !NOT_A_PERMISSION.has(api));
    expect(shown.length, "the intro shows no permission — this case would pass on nothing").toBeGreaterThan(3);
    const listed = DOAN_CHINH_SACH_TUNG_QUYEN.map((d) => /\(([A-Za-z]+)\)/.exec(d)?.[1]);
    expect([...listed].sort()).toEqual([...shown].sort());
    // And nothing of the commune interface rides along.
    // (12/10/2026: the camera, the picker and `getLocation` are the intro's again — listed above through `shown`.
    // Same day `chooseImage` left: "Chụp ảnh" is Zalo's camera, the picker's own call.)
    expect(DOAN_CHINH_SACH_TUNG_QUYEN.join("\n")).not.toContain("getNetworkType");
    expect(DOAN_CHINH_SACH_TUNG_QUYEN.join("\n")).not.toContain("chooseImage");
    // The intro's location line names no commune use — that is the commune policy's (`commune-privacy.ts`).
    const location = DOAN_CHINH_SACH_TUNG_QUYEN.find((d) => d.includes("(getLocation)"))!;
    expect(location).toMatch(/'Tìm văn phòng gần tôi'/);
    expect(location).not.toMatch(/phản ánh|(?<!\p{L})xã(?!\p{L})/u);
  });

  it("the outside destinations are the intro's only, counted from `INTRO_DESTINATIONS`", () => {
    const thirdParty = MUC_CHINH_SACH.find((m) => m.ma === "ben-thu-ba")!.doan.join("\n");
    expect(thirdParty).toContain(cauKhaiDichRaNgoai(INTRO_DESTINATIONS));
    for (const d of DICH_MO_RA_NGOAI.filter((x) => x.app !== "intro")) {
      expect(thirdParty, `a commune destination is in the ViHAT text: ${d.ma}`).not.toContain(d.trong_chinh_sach);
    }
    // 12/10/2026: "ứng dụng không hỏi vị trí của bạn" became false ("Tìm văn phòng gần tôi"); what stays true is that
    // the map link carries the office's address only — and the old absolute sentence must not come back.
    expect(thirdParty).toMatch(/không mang theo vị trí của bạn, kể cả khi bạn vừa cho phép lấy vị trí/);
    expect(thirdParty).not.toMatch(/không hỏi vị trí/);
    expect(thirdParty).toMatch(/bấm 'Tìm văn phòng gần tôi' rồi cho phép lấy vị trí/);
  });

  /**
   * "TÌM VĂN PHÒNG GẦN TÔI" (owner decision 12/10/2026) — one paragraph, and each fact it states is the code's:
   * what (one location, on a tap, after "Cho phép"), where (ViHAT's server, to convert the Zalo code), not stored by
   * that server (`vihat-miniapp` `vi_tri.go:89-92,157`), distances on the phone (`nearby-offices.ts`), no other use.
   */
  it("the nearby-offices paragraph says what, when, where, not stored, on the phone, and no other use", () => {
    const section = MUC_CHINH_SACH.find((m) => m.ma === "vi-tri-van-phong");
    expect(section, "the policy lost its nearby-offices section").toBeDefined();
    expect(section!.doan).toHaveLength(1);
    const t = section!.doan[0]!;
    for (const said of [
      /nút 'Tìm văn phòng gần tôi'/, // the tap
      /bấm 'Cho phép'/, // the app's own question first
      /MỘT LẦN cho mỗi lần bấm/, // what: once
      /chỉ trả một mã dùng một lần/,
      /tới máy chủ của Tập đoàn ViHAT Group để đổi ra toạ độ/, // where, and why
      /MÁY CHỦ KHÔNG LƯU vị trí của bạn — không lưu mã, không lưu toạ độ, không ghi chúng vào nhật ký/, // not stored
      /toạ độ chỉ dùng để tính khoảng cách tới từng văn phòng/, // on the phone
      /không giữ lại toạ độ, không lưu xuống máy, và không dùng hay gửi chúng cho việc gì khác/, // no other use
      /thì không có gì được gửi đi và danh sách văn phòng giữ thứ tự sẵn có/, // refusal
    ]) {
      expect(t, String(said)).toMatch(said);
    }
    expect(CAU_DAU).toMatch(/bấm 'Tìm văn phòng gần tôi' rồi cho phép lấy vị trí/);
  });

  it("the SDK failure report: every field the intro can send is declared VERBATIM — and `host` is not (the intro has none)", () => {
    const t = MUC_CHINH_SACH.find((m) => m.ma === "cach-thuc")!.doan.join("\n");
    const introFields = CLIENT_ERROR_FIELDS.filter((f) => f.khoa !== "host");
    expect(introFields.length).toBe(4);
    for (const f of introFields) expect(t, `not declared: ${f.khoa}`).toContain(f.trong_chinh_sach);
    expect(t).not.toContain(CLIENT_ERROR_FIELDS.find((f) => f.khoa === "host")!.trong_chinh_sach);
    // The body the intro builds really has no `host`: `reportSdkFailure` passes null outside a commune.
    const body = JSON.parse(clientErrorBody({ capability: "phone", code: -1, message: "m", app_id: "1", host: null })) as Record<string, unknown>;
    expect(Object.keys(body).sort()).toEqual(introFields.map((f) => f.khoa).sort());
  });

  it("the opening sentence names the app, what leaves, when, and where", () => {
    expect(CAU_DAU).toMatch(/ứng dụng của Tập đoàn ViHAT Group trên Zalo/);
    expect(CAU_DAU).toMatch(/không lưu dữ liệu nào của bạn xuống máy/);
    expect(CAU_DAU).toMatch(/'Chat với chuyên viên'/);
  });
});

/**
 * 07/10/2026 — "CHAT VỚI CHUYÊN VIÊN" AND "NHẬN ƯU ĐÃI QUA SMS"; storage, use and retention filled by owner decision 5
 * (11/10/2026), capped where the code caps it.
 */
describe("chat với chuyên viên và ưu đãi SMS — khai đủ, không câu cũ nào còn sai", () => {
  const section = () => {
    const m = MUC_CHINH_SACH.find((x) => x.ma === "chat-sms");
    expect(m, "the policy lost its chat / SMS section").toBeDefined();
    return m!.doan.join("\n");
  };

  it("the name leaves the phone ONLY with a chat request, and only when the citizen agrees", () => {
    const t = section();
    expect(t).toMatch(/kèm tên ấy nếu bạn đồng ý chia sẻ/);
    expect(t).toMatch(/Từ chối chia sẻ tên thì yêu cầu vẫn được gửi, chỉ là không có tên/);
    // The refusal path is said, and it names the button that shares nothing.
    expect(t).toMatch(/nút 'Chat Zalo' trên màn hình — nút ấy không gửi thông tin nào của bạn/);
  });

  it("the SMS consent names the number, says it is advertising, and says how to stop", () => {
    const t = section();
    expect(t).toMatch(/SỐ ĐIỆN THOẠI ZALO CỦA BẠN/);
    expect(t).toMatch(/SMS QUẢNG CÁO/);
    expect(t).toMatch(/MUỐN DỪNG NHẬN, bạn bấm 'Huỷ nhận ưu đãi SMS' ngay trong khối ấy, bất cứ lúc nào/);
    expect(t, "says the choice is stored on the phone").toMatch(/không lưu việc bạn đã đăng ký xuống máy/);
  });

  it("storage and use per decision 5: ViHAT's server, read by ViHAT staff; no open decision is left in the text", () => {
    const t = section();
    expect(t).toMatch(/được lưu trên máy chủ của Tập đoàn ViHAT Group\. Nhân viên của Tập đoàn ViHAT Group đọc chúng để liên hệ lại với bạn/);
    expect(t).not.toMatch(/CẦN CHỦ DỰ ÁN QUYẾT/);
  });

  it("retention per decision 5 — SMS choices until deletion; the name until deletion, capped at the code's 24 months", () => {
    const t = section();
    expect(t).toMatch(/các yêu cầu đăng ký hay huỷ nhận ưu đãi SMS được lưu cho tới khi bạn yêu cầu xoá/);
    // `vihat-miniapp` 0004 §4: the 24-month job clears `ten_hien_thi` with the note — so "until deletion" alone
    // would promise to keep the name longer than the code does.
    expect(t).toMatch(/Tên Zalo đi kèm một yêu cầu trò chuyện cũng được lưu cho tới khi bạn yêu cầu xoá, nhưng không quá 24 tháng/);
    // And the erasure really reaches the name (`internal/store/an_danh.go:82-89`) — said where erasure is.
    expect(cachThuc()).toMatch(/xoá phần ghi chú bạn tự gõ và tên Zalo trong mọi yêu cầu bạn đã gửi/);
  });

  it("no sentence still says the server keeps no name, or keeps the number for exactly two things", () => {
    const all = [CAU_DAU, ...MUC_CHINH_SACH.flatMap((m) => m.doan)].join("\n");
    expect(all, "'không tên' without its chat exception").not.toMatch(/không tên, không email/);
    expect(all).toMatch(/không tên — trừ tên Zalo đi kèm một yêu cầu trò chuyện với chuyên viên/);
    expect(all, "the number is said to serve exactly two purposes").not.toMatch(/lưu để làm đúng hai việc/);
    expect(all, "the request section still says no name is ever sent").not.toMatch(/^Ứng dụng KHÔNG gửi kèm tên/m);
    expect(CAU_DAU).toMatch(/'Chat với chuyên viên'/);
  });

  it("the Zalo-name permission line names the chat only", () => {
    const line = DOAN_CHINH_SACH_TUNG_QUYEN.find((d) => d.includes("Tên Zalo (getUserInfo)"));
    expect(line, "the per-permission list lost its Zalo-name line").toBeDefined();
    expect(line).toMatch(/Chat với chuyên viên/);
    expect(line).toMatch(/máy chủ của Tập đoàn ViHAT Group/);
  });
});
