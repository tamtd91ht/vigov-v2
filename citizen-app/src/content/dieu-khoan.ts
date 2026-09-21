/**
 * ĐIỀU KHOẢN SỬ DỤNG — mọi câu chữ, một tệp. Cùng khuôn với `chinh-sach-rieng-tu.ts`.
 *
 * ⚠ VÌ SAO TỆP NÀY RA ĐỜI, VÀ ĐÓ LÀ MỘT LỖI ĐÃ ĐO ĐƯỢC CHỨ KHÔNG PHẢI MỘT SỞ THÍCH:
 *
 *   Điều khoản sử dụng trước đây chỉ tồn tại dưới dạng một tệp `.txt` chép tay trong
 *   `tmp/xin-quyen-zalo/`. Ngày 18/09/2026 nó khai VihatSoftware là bên phát hành; ngày
 *   21/09/2026 bên phát hành đổi thành Tập đoàn ViHAT Group (ADR 0031) và tệp ấy ĐỨNG YÊN — năm
 *   chỗ vẫn nói một pháp nhân không còn phát hành app này là bên phát hành. Ba ngày, không một
 *   thứ gì đỏ lên, vì một tệp chép tay không có gì đối chiếu nó với mã nguồn.
 *
 *   Nên nguồn sống nằm ở đây, và bản `.txt` nộp Zalo được SINH RA từ tệp này
 *   (`scripts/ho-so-zalo.mjs`). Một sự thật, một nơi giữ (luật 9).
 *
 * ⚠ HAI VẾ PHẢI TÁCH BẠCH, Y HỆT TRONG CHÍNH SÁCH QUYỀN RIÊNG TƯ — và trộn chúng là hỏng theo
 *   hai kiểu khác nhau:
 *
 *   | Vế | Nói gì | Tệp này làm gì |
 *   |---|---|---|
 *   | **AI PHÁT HÀNH / AI CHỊU TRÁCH NHIỆM** | ai đứng tên app, ai nhận khiếu nại | **Tập đoàn ViHAT Group** |
 *   | **AI NHẬN DỮ LIỆU** | hai mã đăng nhập đi tới máy chủ nào | **KHÔNG khai ở đây** — chỉ sang Chính sách quyền riêng tư |
 *
 *   Vế thứ hai là câu hỏi mở #28 (`kb/00-foundation/open-questions.json`): ai vận hành
 *   `vihat-miniapp` sau khi chuyển quyền sở hữu app thì CHƯA AI TRẢ LỜI. Văn bản này vì thế
 *   không tự viết ra một lời khai nơi nhận dữ liệu — nó trỏ sang văn bản đã có chủ của lời khai
 *   ấy. Đó cũng đúng nguyên tắc mục "Quyền riêng tư" bên dưới tự đặt ra: không chép lại, để hai
 *   văn bản không bao giờ nói hai điều khác nhau về cùng một việc.
 *
 * ⚠ VihatSoftware VẪN CÒN TRONG VĂN BẢN NÀY, VÀ ĐÓ LÀ CHỦ ĐÍCH: nó vẫn là một đơn vị thành viên
 *   có thật của Tập đoàn, và mục "Những việc bạn không được làm" cùng mục "Quyền sở hữu trí tuệ"
 *   nói về TÊN và TÀI SẢN TRÍ TUỆ của cả hai pháp nhân. Một lượt tìm-thay "VihatSoftware" ->
 *   "ViHAT Group" trên cả tệp xoá mất một pháp nhân khỏi hai điều khoản về sở hữu — sai theo
 *   chiều ngược với cái sai vừa sửa. `ket-xuat-ho-so.test.ts` canh CẢ HAI chiều.
 */

import type { MucChinhSach } from "./chinh-sach-rieng-tu";
import { COMPANY, CONTACT, OFFICES } from "./company-profile";

/**
 * MỘT HÌNH DẠNG MỤC CHO CẢ HAI VĂN BẢN, khai bằng một bí danh chứ không phải một kiểu thứ hai.
 *
 * Hai kiểu giống hệt nhau là hai kiểu sẽ lệch nhau, và hàm kết xuất dùng chung sẽ phải nhận cả
 * hai. Bí danh thì hai văn bản in ra bằng CÙNG một hàm, và một thay đổi về hình dạng không thể
 * chỉ đi vào một nửa.
 */
export type MucDieuKhoan = MucChinhSach;

/**
 * Phiên bản và ngày hiệu lực — cùng lý do với `PHIEN_BAN_CHINH_SACH`: một văn bản không ghi
 * phiên bản là một văn bản không chứng minh được nó đã nói gì vào lúc người dùng bấm đồng ý.
 *
 * ⚠ GIỮ `1.0`, VÀ ĐỔI NGÀY HIỆU LỰC TỪ 18/09 SANG 21/09 — hai quyết định khác nhau, hai lý do:
 *
 *   • SỐ giữ `1.0` vì chưa một người dùng nào đọc một bản nào (cùng bằng chứng với chính sách
 *     quyền riêng tư: sổ tiến độ `nop-zalo-duyet` = `chua_lam`, chưa có lần phát hành nào). Lên
 *     số để kể lại quá trình soạn thảo là làm người đọc tưởng đã có hai đợt công bố.
 *   • NGÀY đổi vì bản chép tay ghi 18/09 trong khi nội dung của nó chỉ đúng từ 21/09 — ngày bên
 *     phát hành đổi. Một văn bản pháp lý tự khai hiệu lực từ trước ngày điều nó khẳng định trở
 *     thành sự thật là một văn bản khai sai chính mốc của mình.
 *
 * Từ lần phát hành đầu tiên trở đi, quy tắc đảo ngược: mỗi thay đổi về QUYỀN và NGHĨA VỤ của
 * người dùng phải lên một số mới, không được gộp.
 */
export const PHIEN_BAN_DIEU_KHOAN = "1.0";
export const NGAY_HIEU_LUC_DIEU_KHOAN = "21/09/2026";

export const TIEU_DE_DIEU_KHOAN = "Điều khoản sử dụng";

/**
 * CÂU ĐỨNG ĐẦU.
 *
 * ⚠ VẾ "KHÔNG CÓ TÀI KHOẢN" ĐÃ BỊ GỠ, VÀ VIỆC GỠ LÀ BẮT BUỘC. Bản 18/09 mở đầu bằng *"Không có
 * tài khoản, không có thanh toán"* — đúng vào ngày ấy. Nay ứng dụng CÓ đăng nhập một chạm bằng
 * số Zalo (ADR 0020) và máy chủ giữ một bản ghi định danh, nên vế ấy đã thành sai ở đúng câu
 * đầu tiên người đọc đọc. Vế "không có thanh toán" thì vẫn đúng và ở lại.
 */
export const CAU_DAU_DIEU_KHOAN =
  "Ứng dụng này là kênh giới thiệu và một vài công cụ tiện ích. Bạn có thể đăng nhập bằng chính số Zalo của mình, nhưng không bắt buộc; không có thanh toán, và việc dùng ứng dụng không tạo ra hợp đồng dịch vụ nào.";

/**
 * ĐẦU MỐI LIÊN HỆ — ĐỌC TỪ `company-profile.ts`, KHÔNG CHÉP LẠI.
 *
 * Địa chỉ, hotline, email và website là những thứ đổi mà không ai nhớ có một tệp điều khoản đang
 * chép lại chúng. Đọc từ nguồn thì một lần đổi địa chỉ là một lần sửa, không phải hai.
 *
 * ⚠ THIẾU NGUỒN THÌ BỎ DÒNG, KHÔNG BỊA MỘT GIÁ TRỊ THAY THẾ. `COMPANY.website` là trường tuỳ
 * chọn và trụ sở chính là một mục tra theo tên: không có thì dòng ấy biến mất khỏi văn bản, chứ
 * không in ra một địa chỉ mặc định nào. Một dòng biến mất là thứ ca kiểm bắt được
 * (`ket-xuat-ho-so.test.ts` đòi mục này có hotline, email và trụ sở chính); một địa chỉ bịa ra
 * thì không ai bắt được, và nó nằm trong một văn bản pháp lý.
 */
const TRU_SO_CHINH = OFFICES.find((mot) => mot.name === "Trụ sở chính");

const DAU_MOI_LIEN_HE: readonly string[] = [
  "Tập đoàn ViHAT Group",
  ...(TRU_SO_CHINH ? [`Trụ sở chính: ${TRU_SO_CHINH.address}`] : []),
  `Hotline: ${CONTACT.hotlineLabel}`,
  `Email: ${CONTACT.email}`,
  ...(COMPANY.website ? [`Website: ${COMPANY.website}`] : []),
];

/**
 * MỘT DANH SÁCH PHẲNG, ĐỌC TỪ TRÊN XUỐNG — cùng lý do với `MUC_CHINH_SACH`: số thứ tự do lúc
 * kết xuất quyết định, không viết cứng vào tiêu đề, và không câu nào tham chiếu tới "mục số N".
 */
export const MUC_DIEU_KHOAN: readonly MucDieuKhoan[] = [
  {
    ma: "ung-dung-la-gi",
    tieu_de: "Ứng dụng này là gì",
    doan: [
      // ĐÃ SỬA 21/09/2026: bản chép tay ghi "Mini App giới thiệu của VihatSoftware — đơn vị thành
      // viên của Tập đoàn ViHAT Group". Đây là vế BÊN PHÁT HÀNH, và nó đã đổi (ADR 0031).
      "Đây là Mini App giới thiệu của Tập đoàn ViHAT Group, cung cấp giải pháp chuyển đổi số cho doanh nghiệp: tổng đài đám mây, Contact Center tích hợp CRM, ứng dụng AI và danh thiếp số.",
      // ĐÃ SỬA: danh sách cũ kết thúc bằng "đăng ký nhận tư vấn" — tính năng ấy KHÔNG CÒN. Khối
      // `getPhoneNumber` trên màn Liên hệ nay là KHỐI ĐĂNG NHẬP (`features/tinh-nang/noi-dung.ts`,
      // mã tính năng `dang-nhap`). Một điều khoản kể một tính năng không có trên màn hình là chỗ
      // người duyệt đối chiếu ra trước tiên.
      "Ngoài phần giới thiệu, ứng dụng có một vài công cụ tiện ích: quét danh thiếp số, hiển thị và tải danh thiếp của chúng tôi, số hoá danh thiếp giấy, kiểm tra kiểu kết nối mạng, tìm văn phòng và đăng nhập bằng số Zalo.",
      "Ứng dụng này không phải là sản phẩm tổng đài, CRM hay AI của chúng tôi. Đó là những sản phẩm riêng, có hợp đồng riêng.",
    ],
  },
  {
    ma: "ai-dung-duoc",
    tieu_de: "Ai dùng được, và dùng thế nào",
    doan: [
      "Ứng dụng mở cho mọi người dùng Zalo. Không cần đăng ký, không cần cung cấp thông tin gì trước khi dùng.",
      "Mỗi tính năng cần quyền của nền tảng đều chỉ hỏi quyền đúng vào lúc bạn tự bấm nút của tính năng đó. Bạn có thể từ chối, và ứng dụng vẫn dùng được.",
    ],
  },
  {
    ma: "khong-co-gi",
    tieu_de: "Những gì ứng dụng không có",
    doan: [
      // ĐÃ SỬA: bản chép tay viết "Không có tài khoản người dùng, không có mật khẩu, không có hồ
      // sơ cá nhân". Vế "không có tài khoản" nay SAI — có đăng nhập, và máy chủ giữ một bản ghi
      // định danh gắn với số điện thoại. Hai vế còn lại vẫn đúng và được giữ nguyên nghĩa.
      "Không có mật khẩu, và không có biểu mẫu hồ sơ nào để bạn khai. Việc đăng nhập là một lần chạm bằng chính số Zalo của bạn; bạn xem được toàn bộ ứng dụng mà không cần đăng nhập.",
      "Không có thanh toán, không có gói cước, không có giao dịch tài chính nào.",
      // ĐÃ SỬA, VÀ ĐÂY LÀ CÂU SAI NẶNG NHẤT CỦA BẢN CHÉP TAY: "không có máy chủ nào của chúng tôi
      // nhận dữ liệu từ ứng dụng này". Từ 20/09/2026 bước đăng nhập gọi máy chủ thật, ở CẢ BẢN
      // NỘP (`features/dang-nhap/goi-may-chu.ts`). Câu ấy còn mâu thuẫn thẳng với chính sách quyền
      // riêng tư nằm cùng bộ hồ sơ — người duyệt đọc cả hai.
      //
      // ⚠ CÂU THAY THẾ CỐ Ý KHÔNG NÓI NƠI NHẬN LÀ AI: đó là câu hỏi mở #28. Nó chỉ sang văn bản
      // có chủ của lời khai ấy, đúng nguyên tắc mục "Quyền riêng tư" bên dưới tự đặt ra.
      "Không có biểu mẫu nào để bạn nhập dữ liệu: không có ô nhập số điện thoại, không có mã sáu số nào phải gõ. Thứ duy nhất rời khỏi máy bạn là hai mã dùng một lần ở bước đăng nhập — Chính sách quyền riêng tư nói rõ chúng đi tới đâu, máy chủ lưu gì và trong bao lâu.",
    ],
  },
  {
    ma: "thong-tin-tham-khao",
    tieu_de: "Thông tin trong ứng dụng là để tham khảo",
    doan: [
      "Phần giới thiệu công ty, danh sách giải pháp và các số liệu của Tập đoàn là thông tin giới thiệu. Chúng không phải là cam kết hợp đồng, báo giá hay đề nghị giao kết.",
      "Điều khoản của một dịch vụ cụ thể nằm trong hợp đồng ký riêng cho dịch vụ đó, không nằm ở đây.",
      "Chúng tôi có thể cập nhật nội dung giới thiệu bất cứ lúc nào mà không báo trước.",
    ],
  },
  {
    ma: "ma-qr-ben-thu-ba",
    tieu_de: "Mã QR bạn quét là nội dung của bên thứ ba",
    doan: [
      "Khi bạn quét một mã QR, ứng dụng đọc nội dung trong mã và hiện ra cho bạn xem. Nội dung đó do người tạo mã đặt vào, không phải do chúng tôi.",
      "Chúng tôi không kiểm soát và không chịu trách nhiệm về nội dung, liên kết hay số điện thoại nằm trong một mã do người khác tạo. Một mã QR có thể chứa liên kết dẫn tới trang web bất kỳ.",
      "Ứng dụng luôn hiện nội dung ra trước và để bạn quyết định có mở liên kết hay gọi hay không. Bạn hãy cân nhắc trước khi mở một liên kết từ mã của người lạ.",
    ],
  },
  {
    ma: "danh-thiep-nguoi-khac",
    tieu_de: "Danh thiếp của người khác là dữ liệu cá nhân của họ",
    doan: [
      "Khi bạn chụp hoặc chọn ảnh một tấm danh thiếp giấy, tấm thiếp đó chứa dữ liệu cá nhân của người đã đưa nó cho bạn.",
      "Ứng dụng chỉ hiện ảnh lên màn hình: ảnh không rời khỏi máy bạn và biến mất khi bạn rời màn hình.",
      "Việc bạn lưu giữ, sử dụng hay chia sẻ thông tin của người khác sau đó thuộc trách nhiệm của bạn theo Nghị định 13/2023/NĐ-CP về bảo vệ dữ liệu cá nhân.",
    ],
  },
  {
    ma: "phu-thuoc-zalo",
    tieu_de: "Các công cụ phụ thuộc vào Zalo và vào thiết bị của bạn",
    doan: [
      "Quét mã, chọn ảnh, giữ màn hình sáng, tải tệp, lấy vị trí và lấy số điện thoại đều là tính năng do nền tảng Zalo cung cấp. Chúng chỉ chạy được bên trong ứng dụng Zalo trên điện thoại.",
      "Vì vậy chúng tôi không cam kết các công cụ này luôn sẵn sàng hoặc luôn cho kết quả: chúng phụ thuộc vào phiên bản Zalo, vào hệ điều hành và vào quyền bạn đã cấp trên thiết bị.",
      "Phần giới thiệu công ty vẫn đọc được bình thường kể cả khi các công cụ không chạy.",
    ],
  },
  {
    // ⚠ HAI TÊN PHÁP NHÂN Ở LẠI NGUYÊN VẸN TRONG MỤC NÀY VÀ MỤC DƯỚI. Chúng nói về TÊN, LOGO và
    // TÀI SẢN TRÍ TUỆ, không nói ai phát hành app — VihatSoftware vẫn là một đơn vị thành viên có
    // thật (`company-profile.ts`, danh sách đơn vị thành viên), nên tên và logo của nó vẫn được
    // bảo hộ. Bỏ nó đi là thu hẹp một điều khoản bảo hộ, không phải sửa một lỗi.
    ma: "khong-duoc-lam",
    tieu_de: "Những việc bạn không được làm",
    doan: [
      "Không dùng ứng dụng để thu thập thông tin của người khác trái với quy định pháp luật.",
      "Không can thiệp, dịch ngược, hay tìm cách thay đổi cách ứng dụng hoạt động.",
      "Không dùng tên, logo hoặc nội dung của VihatSoftware và ViHAT Group cho mục đích thương mại khác khi chưa có văn bản đồng ý của chúng tôi.",
    ],
  },
  {
    ma: "so-huu-tri-tue",
    tieu_de: "Quyền sở hữu trí tuệ",
    doan: [
      // GIỮ NGUYÊN CẢ HAI PHÁP NHÂN, CÓ CHỦ ĐÍCH. Chuyển quyền sở hữu Mini App (ADR 0031) là một
      // việc; ai đang giữ quyền đối với tên, logo, nội dung giới thiệu và mã nguồn sau lần chuyển
      // ấy là một câu hỏi pháp lý KHÔNG ai trong kho này trả lời được. Bớt một bên khỏi câu này là
      // tự ra một quyết định về sở hữu trí tuệ — đắt hơn hẳn việc để nguyên một câu vẫn đúng với
      // tài sản của cả hai. Chủ dự án xác nhận trước khi nộp.
      "Tên gọi, logo, nội dung giới thiệu, hình ảnh và mã nguồn của ứng dụng thuộc quyền của VihatSoftware và Tập đoàn ViHAT Group.",
      "Bạn được xem và sử dụng ứng dụng cho mục đích cá nhân hoặc cho công việc của doanh nghiệp mình. Mọi hình thức sao chép để phân phối lại đều cần văn bản đồng ý.",
    ],
  },
  {
    ma: "gioi-han-trach-nhiem",
    tieu_de: "Giới hạn trách nhiệm",
    doan: [
      "Ứng dụng được cung cấp miễn phí và theo hiện trạng. Chúng tôi không cam kết ứng dụng chạy liên tục, không lỗi, hay phù hợp với một mục đích cụ thể nào của bạn.",
      "Chúng tôi không chịu trách nhiệm về thiệt hại phát sinh từ việc bạn dựa vào thông tin giới thiệu trong ứng dụng để ra quyết định, hoặc từ việc bạn mở một liên kết nằm trong mã QR do người khác tạo.",
      "Giới hạn này không loại trừ những trách nhiệm mà pháp luật Việt Nam không cho phép loại trừ.",
    ],
  },
  {
    ma: "quyen-rieng-tu",
    tieu_de: "Quyền riêng tư",
    doan: [
      'Ứng dụng xử lý dữ liệu thế nào, xin quyền gì và vì sao — tất cả nằm trong Chính sách quyền riêng tư của ứng dụng, đọc được ngay trong Mini App ở cuối màn hình "Liên hệ".',
      "Nội dung ấy không chép lại vào đây, để hai văn bản không bao giờ nói hai điều khác nhau về cùng một việc.",
    ],
  },
  {
    ma: "thay-doi-dieu-khoan",
    tieu_de: "Thay đổi điều khoản",
    doan: [
      `Điều khoản này có hiệu lực từ ngày ${NGAY_HIEU_LUC_DIEU_KHOAN}, phiên bản ${PHIEN_BAN_DIEU_KHOAN}.`,
      "Khi ứng dụng có thêm chức năng làm thay đổi những gì viết ở trên, chúng tôi sẽ cập nhật và công bố bản mới TRƯỚC khi chức năng đó bắt đầu chạy.",
    ],
  },
  {
    ma: "luat-va-lien-he",
    tieu_de: "Luật áp dụng và liên hệ",
    doan: [
      "Điều khoản này chịu sự điều chỉnh của pháp luật Việt Nam.",
      "Mọi câu hỏi hoặc khiếu nại liên quan tới ứng dụng, xin liên hệ:",
      ...DAU_MOI_LIEN_HE,
      // ĐÃ BỎ MỘT CÂU: "(Hotline và email là đầu mối liên hệ chung của Tập đoàn ViHAT Group.)".
      // Câu ấy tồn tại để giải thích vì sao một đơn vị thành viên lại in số của Tập đoàn. Nay
      // chính Tập đoàn đứng tên khối liên hệ này, nên nó không còn giải thích gì — nó chỉ làm
      // người đọc đi tìm một pháp nhân thứ hai không có trong khối.
    ],
  },
];
