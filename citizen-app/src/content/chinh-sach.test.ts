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
 * ("không đọc thư viện ảnh" ở bản 1.1, "không có ô đăng nhập" ở bản 1.2), cả hai lần đều được
 * phát hiện bằng mắt người chứ không bằng phép kiểm. Tệp này là phép kiểm ấy.
 *
 * ⚠ BA CA DƯỚI ĐÂY ĐỀU ĐỎ KHI HÀNH VI ĐỔI, và đó là chủ đích — cái đỏ là cuộc trò chuyện phải
 * xảy ra TRƯỚC khi hành vi mới ra người dùng, không phải sau.
 */

const toanVan = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");

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

  it("phiên bản và ngày hiệu lực đi cùng nhau, và không lùi về bản cũ", () => {
    // 1.4: thời gian lưu được chốt, và nhật ký đăng nhập (thời điểm + IP) được khai lần đầu.
    // Cả hai là thay đổi HÀNH VI XỬ LÝ DỮ LIỆU, nên phải có số mới — kể cả khi cách 1.3 vài giờ.
    // Một người đọc bản 1.3 rồi bấm đồng ý KHÔNG biết địa chỉ IP của mình được ghi lại.
    expect(PHIEN_BAN_CHINH_SACH).toBe("1.4");
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
 * THỜI GIAN LƯU — CHỖ TRỐNG ĐÃ ĐƯỢC LẤP, VÀ CA CANH ĐỔI THEO (bản 1.4, 20/09/2026).
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
  const cachThuc = () => {
    const muc = MUC_CHINH_SACH.find((m) => m.ma === "cach-thuc");
    expect(muc, "chính sách không còn mục nào về cách xử lý và thời gian lưu").toBeDefined();
    return muc!.doan.join("\n");
  };

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

  it("nói XOÁ THÌ XOÁ CÁI GÌ, và vì sao nhật ký đăng nhập ở lại được", () => {
    // Một lời hứa xoá mà không nói phạm vi là một lời hứa người dùng hiểu rộng hơn thứ sẽ xảy
    // ra. Và phần ở LẠI phải có lý do nói được thành lời — ở đây là: nhật ký không chứa số điện
    // thoại, nên sau khi xoá không còn đường nào nối nó về với người dùng.
    const chu = cachThuc();
    expect(chu).toMatch(/số điện thoại của bạn và bản ghi định danh/);
    expect(chu).toMatch(/Nhật ký đăng nhập được giữ lại/);
    expect(chu).toMatch(/KHÔNG chứa số điện thoại/);
  });
});

/**
 * NHẬT KÝ ĐĂNG NHẬP — MỘT HÀNH VI CỦA MÁY CHỦ MÀ BẢN 1.3 HOÀN TOÀN KHÔNG KHAI.
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

  it("nói rõ nhật ký ấy KHÔNG chứa số điện thoại — đó là điều kiện để nó được ở lại sau khi xoá", () => {
    const chu = MUC_CHINH_SACH.flatMap((m) => m.doan).join("\n");
    expect(chu).toMatch(/Nhật ký này KHÔNG chứa số điện thoại/);
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
