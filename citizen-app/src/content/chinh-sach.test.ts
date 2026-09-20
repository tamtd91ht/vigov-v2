import { describe, expect, it } from "vitest";

import {
  CAU_DAU,
  MUC_CHINH_SACH,
  NGAY_HIEU_LUC,
  PHIEN_BAN_CHINH_SACH,
} from "./chinh-sach-rieng-tu";

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
    expect(chu, "không nói AI NHẬN").toMatch(/máy chủ của VihatSoftware/);
    expect(chu, "không nói MÁY CHỦ LƯU SỐ ĐIỆN THOẠI").toMatch(/LƯU SỐ ĐIỆN THOẠI/);
    expect(chu, "không nói VÌ SAO — đăng nhập và thông báo ZNS").toMatch(/tên đăng nhập/);
    expect(chu).toMatch(/ZNS/);
    // Và vẫn nói ra điều đúng: số điện thoại KHÔNG nằm trong hai mã gửi đi.
    expect(chu).toMatch(/không nằm trong hai mã/);
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
    expect(NGAY_HIEU_LUC).toMatch(/^\d{2}\/\d{2}\/\d{4}$/);
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
  it("thời hạn lưu nhật ký là 90 ngày, và nói rõ nó áp cho CẢ dòng của lượt thất bại", () => {
    const chu = cachThuc();
    expect(chu).toMatch(/THỜI HẠN LƯU NHẬT KÝ ĐĂNG NHẬP: 90 ngày/);
    expect(chu, "không nói 90 ngày áp cho MỌI dòng").toMatch(/áp cho MỌI dòng/);
    expect(chu, "không nói ra vế lượt thất bại").toMatch(
      /kể cả những dòng của một lượt đăng nhập không thành công/,
    );
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

  it("KHÔNG để lại một câu đọc ra thành 'địa chỉ IP không bao giờ bị chạm tới'", () => {
    // HAI CÂU ĐỀU ĐÚNG NHƯNG KHÁC CHỦ NGỮ, VÀ ĐỌC LIỀN NHAU THÌ DẪN TỚI MỘT KẾT LUẬN SAI:
    //
    //   mục "Các quyền" : "Ứng dụng không nhận địa chỉ IP"        (đúng — `getNetworkType`)
    //   mục "Đăng nhập" : "máy chủ ghi lại … địa chỉ IP"           (đúng — máy chủ, không phải app)
    //
    // Người đọc không có nghĩa vụ đối chiếu hai mục cách nhau nửa trang rồi tự suy ra chủ ngữ.
    // Nên câu thứ nhất phải tự dẫn sang mục Đăng nhập. Ca này giữ cho vế dẫn ấy không bị ai
    // "dọn cho gọn" — mất nó là văn bản tự mời một hiểu lầm về đúng thứ nó đang khai.
    const cac_quyen = MUC_CHINH_SACH.find((m) => m.ma === "cac-quyen");
    expect(cac_quyen).toBeDefined();
    const noi_khong_nhan_ip = cac_quyen!.doan.filter((d) => d.includes("không nhận địa chỉ IP"));
    expect(noi_khong_nhan_ip.length, "không còn câu nào về địa chỉ IP trong mục các quyền").toBe(1);
    expect(
      noi_khong_nhan_ip[0],
      'câu "ứng dụng không nhận địa chỉ IP" đứng một mình, không dẫn sang mục Đăng nhập',
    ).toMatch(/máy chủ nhìn thấy địa chỉ IP/);
  });
});
