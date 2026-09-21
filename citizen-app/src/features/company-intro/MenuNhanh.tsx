import type { ComponentType } from "react";

import { CONTACT } from "../../content/company-profile";

import type { DiemDen } from "./dieu-huong";
import {
  MOC_QUAN_LY_QUYEN,
  MOC_QUET_MA_QR,
  MOC_SO_HOA_THIEP,
  MOC_TIM_VAN_PHONG,
} from "./dieu-huong";
import { GridGlyph, NamecardGlyph, PhoneGlyph, PinGlyph, QrGlyph, ShieldGlyph } from "./icons";

/**
 * MENU NHANH TRÊN MÀN CHỦ — thẻ trắng ĐÈ LÊN ĐÁY HERO, sáu mục, mỗi mục dẫn tới một chỗ CÓ THẬT.
 *
 * ⚠ BẢN MẪU CÓ TÁM Ô TRONG LƯỚI 4x2. Ở ĐÂY CÓ SÁU Ô TRONG LƯỚI 3x2, VÀ CẢ HAI KHÁC BIỆT ĐỀU LÀ
 * QUYẾT ĐỊNH, KHÔNG PHẢI THIẾU SÓT:
 *
 *   | Mục bản mẫu | Ở đây |
 *   |---|---|
 *   | Giải pháp · Quét QR Lead · Chụp danh thiếp · Quyền ứng dụng | dựng nguyên nhãn bản mẫu |
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
 * ⚠ HAI MỤC DANH THIẾP DẪN TỚI HAI MỎ NEO KHÁC NHAU của cùng một màn, vì từ lượt này màn ấy không
 * còn tab riêng (`features/tinh-nang/index.ts`). Dùng chung một mốc thì mục "Chụp danh thiếp" mở
 * ra máy quét — một nút nói dối.
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
    ma: "qr",
    nhan: "Quét QR Lead",
    phu: "Mở máy quét",
    glyph: QrGlyph,
    kieu: "man",
    di: { man: "danh-thiep", moc: MOC_QUET_MA_QR },
  },
  {
    ma: "danh-thiep",
    nhan: "Chụp danh thiếp",
    phu: "Thiếp giấy",
    glyph: NamecardGlyph,
    kieu: "man",
    di: { man: "danh-thiep", moc: MOC_SO_HOA_THIEP },
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
    ma: "quyen",
    nhan: "Quyền ứng dụng",
    phu: "App xin gì",
    glyph: ShieldGlyph,
    kieu: "man",
    di: { man: "contact", moc: MOC_QUAN_LY_QUYEN },
  },
];

export function MenuNhanh({ onDi }: { onDi: (diem: DiemDen) => void }) {
  return (
    /* THẺ TRẮNG ĐÈ LÊN ĐÁY HERO (bản mẫu). Nó là một lớp CHỒNG, nên lề âm của nó và đệm đáy của
       hero là MỘT CẶP: đệm đáy phải lớn hơn, nếu không thẻ trùm lên hàng chỉ số trong hero.
       `accessibility.test.ts` đọc hai con số ấy ra khỏi CSS và so, thay vì tin vào mắt. */
    <div className="menu-the">
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
