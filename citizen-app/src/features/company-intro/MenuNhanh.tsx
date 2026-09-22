import type { ComponentType, ReactNode } from "react";

import { CONTACT } from "../../content/company-profile";

import type { DiemDen } from "./dieu-huong";
import { MOC_TIM_VAN_PHONG } from "./dieu-huong";
import {
  CompassGlyph,
  GridGlyph,
  HandshakeGlyph,
  NamecardGlyph,
  PhoneGlyph,
  PinGlyph,
} from "./icons";

/**
 * MENU NHANH TRÊN MÀN CHỦ — thẻ trắng ĐÈ LÊN ĐÁY HERO, sáu mục, mỗi mục dẫn tới một chỗ CÓ THẬT.
 *
 * ⚠ BẢN MẪU CÓ TÁM Ô TRONG LƯỚI 4x2. Ở ĐÂY CÓ SÁU Ô TRONG LƯỚI 3x2, VÀ CẢ HAI KHÁC BIỆT ĐỀU LÀ
 * QUYẾT ĐỊNH, KHÔNG PHẢI THIẾU SÓT:
 *
 *   | Mục bản mẫu | Ở đây |
 *   |---|---|
 *   | Giải pháp | dựng nguyên nhãn bản mẫu |
 *   | **Quét QR Lead** + **Chụp danh thiếp** | **GỘP LÀM MỘT**: "Danh thiếp". Xem khối riêng dưới |
 *   | **Quyền ứng dụng** | **KHÔNG CÒN Ở ĐÂY** (22/09/2026). Xem khối riêng dưới |
 *   | **Văn phòng gần tôi** | dựng, nhưng nhãn rút thành "Văn phòng" — tính năng chưa xếp theo khoảng cách, xem dưới |
 *   | **Đặt lịch tư vấn** | **KHÔNG DỰNG** — là một biểu mẫu thu dữ liệu (xem dưới). Thay bằng một mục "Tư vấn" gọi hotline |
 *   | **Tải brochure** | **KHÔNG DỰNG** — trong kho không có một tệp PDF nào để tải |
 *   | **Hỗ trợ 24/7** | **KHÔNG DỰNG** — xem dưới |
 *
 *   Một nút không dẫn đi đâu, hoặc dẫn tới một thứ chưa tồn tại, là thứ người duyệt của Zalo bấm
 *   vào đầu tiên — và là thứ người dùng lớn tuổi bấm rồi kết luận rằng app hỏng. Ba cái tên ấy
 *   quay lại được bất cứ lúc nào sau khi có người nói chúng mở ra cái gì.
 *
 * ⚠ "HỖ TRỢ 24/7" GỘP VÀO "TƯ VẤN", VÀ KHÔNG ĐƯỢC ĐỔI TÊN THÀNH "24/7" — HAI PHẦN, HAI LÝ DO.
 *
 *   Đích trung thực duy nhất sau cái tên ấy là hotline, mà hotline đã có một ô rồi. Hai ô cạnh
 *   nhau cùng quay một số là hai ô mà người dùng phải đoán xem chúng khác gì nhau.
 *
 *   Và ô ấy KHÔNG mang chữ "24/7": không trang nào của ViHAT mà kho này đã đọc công bố rằng
 *   hotline trực 24/7. In "24/7" lên màn là một cam kết về giờ phục vụ, đứng tên một pháp nhân
 *   có thật, mà chưa ai cam kết — và người phát hiện ra là người gọi lúc 11 giờ đêm.
 *
 * ⚠ "TƯ VẤN" LÀ MỘT CUỘC GỌI, KHÔNG PHẢI MỘT BIỂU MẪU — và nó nói ra điều đó NGAY TRÊN NÚT.
 *
 *   Bản mẫu có một mục "Đặt lịch tư vấn". Nó không được dựng, theo yêu cầu, và không ai quyết một
 *   biểu mẫu sẽ gửi dữ liệu đi đâu; hơn nữa `phase1-collects-nothing.test.ts` cấm
 *   `<form|input|textarea|select>` ở mọi tệp, nên một ô nhập ở đây là một dây bẫy bị nới.
 *
 *   Thứ CÓ THẬT và chạy được ngay là hotline. Nên mục này là một neo `tel:` — đúng cách hai màn
 *   khác của app đã gọi cùng số ấy, chạy được cả ngoài Zalo, và KHÔNG cần khai thêm một lời gọi
 *   nền tảng nào (`openPhone` đang khai cho việc gọi một số vừa QUÉT được ở màn Danh thiếp; mượn
 *   nó cho việc này là làm sai lệch chính bảng khai mà màn Quản lý quyền đọc ra).
 *
 *   Dòng phụ "Gọi hotline" là phần bắt buộc: một nút tên "Tư vấn" mà mở thẳng màn quay số là một
 *   bất ngờ, và với người lớn tuổi thì một bất ngờ là một lần bấm nhầm.
 *
 * ⚠ BA CỘT CHỨ KHÔNG PHẢI BỐN — và đó là một phép đo, không phải một khẩu vị.
 *
 *   Bản mẫu bày 4 cột vì nó có 8 ô. Sáu ô trong lưới 4 cột để lại hàng hai chỉ có hai ô lẻ, lệch
 *   hẳn về trái — chính cái "ô lẻ loi ở hàng hai" phải tránh. 3x2 thì hai hàng đều đầy.
 *
 *   Bề rộng cũng chỉ đủ cho ba: ở 320px, thẻ này rộng 320 − 32 (lề trang) = 288, trừ 28 đệm thẻ
 *   còn 260. Chia ba, trừ khe 10: ~80px mỗi ô. Chia bốn thì còn ~58px, mà từ dài nhất trong sáu
 *   nhãn ("phòng", "thiếp", "dụng" …) đã cần ~50px ở cỡ 16px — hết chỗ, và nhãn bị cắt đuôi.
 *
 * ⚠ MỘT MỤC "DANH THIẾP", KHÔNG PHẢI HAI — 22/09/2026, yêu cầu của chủ dự án.
 *
 *   Bản mẫu có "Quét QR Lead" và "Chụp danh thiếp": hai ô cho hai QUYỀN máy ảnh, không phải hai
 *   việc khách cần. Thứ có thật đằng sau cả hai là MỘT DÒNG SẢN PHẨM ĐÃ CÔNG BỐ — `SOLUTIONS` mục
 *   `namecard`, "giải pháp networking và quản lý danh thiếp số". Nên ở đây là một ô mang đúng tên
 *   dòng sản phẩm ấy, dẫn tới ĐẦU màn Danh thiếp (không mốc), nơi ba việc quanh tấm thiếp nằm
 *   cạnh nhau và người dùng chọn.
 *
 *   Và chữ "Lead" biến mất cùng lúc, vì hai lý do độc lập: app này KHÔNG tạo ra một lead nào —
 *   không có tuyến nào nhận, giai đoạn A không thu thập gì — nên một nhãn hứa "lead" là một nhãn
 *   hứa thứ chưa có; và nó là tiếng lóng bán hàng, thứ một người lớn tuổi đọc xong không biết ô
 *   ấy mở ra cái gì.
 *
 * ⚠ KHÔNG CÒN Ô "QUYỀN ỨNG DỤNG" Ở ĐÂY — và đường tới màn ấy KHÔNG mất.
 *
 *   `HomeScreen` đã có sẵn một nút khối "Quản lý quyền" ở gần cuối màn, dẫn tới đúng cùng một chỗ.
 *   Hai đường tới một màn, đứng trên cùng một màn chủ, là hai chỗ người dùng phải đoán xem chúng
 *   khác gì nhau — và ô menu là bản trùng, không phải bản gốc. Màn `QuanLyQuyenScreen` giữ nguyên:
 *   nó là thứ vòng duyệt của Zalo đọc.
 *
 *   Chỗ trống ấy, cộng chỗ trống của hai ô danh thiếp gộp lại, thành hai ô mới: "Gợi ý" (bộ chọn
 *   ba bước, chạy hoàn toàn trên máy) và "Đăng nhập" (dẫn thẳng tới khối đăng nhập một chạm trên
 *   màn Liên hệ). Vẫn đúng sáu ô, vẫn lưới 3x2 — phép đo 320px ở dưới không bị động tới.
 */

type MucMenu = {
  /** Khoá React và mỏ neo cho test. Không hiện ra. */
  ma: string;
  nhan: string;
  /** Dòng phụ nói mục này dẫn tới ĐÂU. Bắt buộc: một nhãn một từ không nói được nó làm gì. */
  phu: string;
  glyph: ComponentType<{ className?: string }>;
} & (
  | { kieu: "man"; di: DiemDen }
  /** `tel:` — không phải một lời gọi nền tảng, xem khối chú thích trên. */
  | { kieu: "goi"; so: string }
);

export const MUC_MENU_NHANH: readonly MucMenu[] = [
  {
    ma: "giai-phap",
    nhan: "Giải pháp",
    phu: "Hệ sinh thái",
    glyph: GridGlyph,
    kieu: "man",
    di: { man: "solutions" },
  },
  {
    // MỘT DÒNG SẢN PHẨM ĐÃ CÔNG BỐ, MỘT Ô. Không `moc`: ô này thả người dùng xuống ĐẦU màn Danh
    // thiếp, nơi ba việc quanh tấm thiếp nằm cạnh nhau. Xem khối chú thích đầu tệp.
    ma: "danh-thiep",
    nhan: "Danh thiếp",
    phu: "Thiếp số ViHAT",
    glyph: NamecardGlyph,
    kieu: "man",
    di: { man: "danh-thiep" },
  },
  {
    // BỘ CHỌN BA BƯỚC, CHẠY HOÀN TOÀN TRÊN MÁY. Dòng phụ nói ra số bước, vì một nhãn "Gợi ý" trần
    // không cho biết sau cú bấm là một câu hỏi hay một danh sách.
    ma: "goi-y",
    nhan: "Gợi ý",
    phu: "Chọn 3 bước",
    glyph: CompassGlyph,
    kieu: "man",
    di: { man: "goi-y" },
  },
  {
    ma: "tu-van",
    nhan: "Tư vấn",
    phu: "Gọi hotline",
    glyph: PhoneGlyph,
    kieu: "goi",
    so: CONTACT.hotlineDialable,
  },
  {
    // ⚠ NHÃN BẢN MẪU LÀ "Văn phòng gần tôi", VÀ Ở ĐÂY NÓ KHÔNG ĐƯỢC DÙNG.
    //
    //   Khối ấy xin được mã vị trí của Zalo rồi nói thẳng rằng nó CHƯA xếp được ba văn phòng theo
    //   khoảng cách (`VAN_PHONG.chua_xep_duoc`) — vì toạ độ thật phải đổi ở máy chủ, thứ chưa có.
    //   Một nhãn hứa "gần tôi" đứng trên một tính năng liệt kê đủ ba địa chỉ theo thứ tự cố định
    //   là một lời hứa app không giữ, và nó hứa đúng vào lúc đang xin quyền vị trí.
    ma: "van-phong",
    nhan: "Văn phòng",
    phu: "Ba địa chỉ",
    glyph: PinGlyph,
    kieu: "man",
    di: { man: "contact", moc: MOC_TIM_VAN_PHONG },
  },
  {
    // ⚠ Ô NÀY LÀ "YÊU CẦU", KHÔNG CÒN LÀ "ĐĂNG NHẬP" — ĐỔI 22/09/2026 (giai đoạn B).
    //
    //   Đăng nhập là một BƯỚC, không phải một việc khách cần — đúng cùng một lỗi mà cả lượt thiết
    //   kế lại này đang sửa: màn chủ bày thứ app cần, không bày thứ khách cần. Thứ khách cần là
    //   xem lại những yêu cầu họ đã gửi.
    //
    //   ĐƯỜNG ĐĂNG NHẬP KHÔNG MẤT: màn "Yêu cầu của tôi" tự lo ca chưa đăng nhập bằng một lời mời
    //   đăng nhập dẫn thẳng tới khối `getPhoneNumber` trên màn Liên hệ (`MOC_DANG_NHAP` vẫn còn và
    //   vẫn được dùng, ở đó). Một ô menu tên "Đăng nhập" trên màn chủ là một ô chỉ có nghĩa với
    //   người CHƯA đăng nhập, và không có nghĩa gì với người đã đăng nhập — tức là sai một nửa
    //   thời gian.
    ma: "yeu-cau",
    nhan: "Yêu cầu",
    phu: "Tình trạng",
    glyph: HandshakeGlyph,
    kieu: "man",
    di: { man: "yeu-cau" },
  },
];

export function MenuNhanh({
  onDi,
  dau,
}: {
  onDi: (diem: DiemDen) => void;
  /**
   * Khối đứng NGAY TRÊN lưới sáu ô, BÊN TRONG thẻ trắng — hôm nay là dải chiến dịch.
   *
   * ⚠ NÓ NẰM TRONG THẺ CHỨ KHÔNG ĐỨNG GIỮA HERO VÀ THẺ, VÀ ĐÓ LÀ MỘT QUYẾT ĐỊNH VỀ BỐ CỤC.
   *
   *   Thẻ này là một lớp CHỒNG: lề âm của nó (`.menu-the { margin: -56px … }`) ăn khớp với đệm
   *   đáy của hero (`.hero { padding: … 76px }`), và `accessibility.test.ts` đọc cả hai con số ra
   *   khỏi CSS rồi so. Chen một khối vào GIỮA hai thẻ ấy thì lề âm kia không còn trèo lên hero
   *   nữa mà trèo lên khối mới — thẻ trắng trùm mất chính dải vừa thêm, và phép kiểm vẫn xanh vì
   *   hai con số nó đọc không đổi. Một phép kiểm xanh trong lúc bố cục đã vỡ là phép kiểm tệ nhất
   *   trong tệp này.
   *
   *   Đặt vào trong thẻ thì cặp số ấy giữ nguyên nghĩa, và dải vẫn ở đúng chỗ mắt cần: ngay dưới
   *   hero, trên sáu ô.
   */
  dau?: ReactNode;
}) {
  return (
    /* THẺ TRẮNG ĐÈ LÊN ĐÁY HERO (bản mẫu). Nó là một lớp CHỒNG, nên lề âm của nó và đệm đáy của
       hero là MỘT CẶP: đệm đáy phải lớn hơn, nếu không thẻ trùm lên hàng chỉ số trong hero.
       `accessibility.test.ts` đọc hai con số ấy ra khỏi CSS và so, thay vì tin vào mắt. */
    <div className="menu-the">
      {dau}
      <ul className="menu-nhanh">
          {MUC_MENU_NHANH.map((muc) => {
          const Glyph = muc.glyph;
          const ben_trong = (
            <>
              {/* Dùng lại `.tile--soft` y như mọi ô biểu tượng khác — không có lớp riêng cho menu
                  này: một lớp CSS không quy định gì là một lớp người sau sẽ gắn một màu chưa ai đo
                  vào. Ô 44x44 cũng chính là thứ giữ cho ô menu cao hơn `--tap-min`. */}
              <span className="tile tile--soft" aria-hidden="true">
                <Glyph className="tile__glyph" />
              </span>
              <span className="menu-nhanh__nhan">{muc.nhan}</span>
              <span className="menu-nhanh__phu">{muc.phu}</span>
            </>
          );

          return (
            <li className="menu-nhanh__muc" key={muc.ma}>
              {muc.kieu === "goi" ? (
                <a className="menu-nhanh__nut" href={`tel:${muc.so}`}>
                  {ben_trong}
                </a>
              ) : (
                <button type="button" className="menu-nhanh__nut" onClick={() => onDi(muc.di)}>
                  {ben_trong}
                </button>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
