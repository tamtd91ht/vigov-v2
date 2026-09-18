/**
 * CHÍNH SÁCH QUYỀN RIÊNG TƯ — mọi câu chữ, một tệp. Theo Nghị định 13/2023/NĐ-CP.
 *
 * ⚠ NGUYÊN TẮC CHI PHỐI TOÀN BỘ TỆP NÀY:
 *
 *   Văn bản này công bố dưới tên một PHÁP NHÂN CÓ THẬT. Một câu mô tả hành vi mà mã không có là
 *   một tuyên bố sai dưới tên ấy, và có người phải trả lời cho nó. Ngược lại, giấu một hành vi
 *   mà mã CÓ là vi phạm chính Nghị định 13. Nên mỗi câu ở đây phải đối chiếu được với mã.
 *
 *   Cụ thể, ba câu dưới đây đúng vì `src/phase1-collects-nothing.test.ts` CẤM điều ngược lại
 *   trên toàn cây mã — không phải vì ai đó hứa:
 *
 *     "không gửi đi đâu"   <- dây bẫy cấm fetch / XHR / WebSocket / EventSource / axios
 *     "không lưu lại"      <- dây bẫy cấm localStorage / sessionStorage / cookie / indexedDB
 *     "không có máy chủ"   <- hệ quả của cả hai
 *
 *   Ai nới một trong hai dây bẫy ấy phải sửa tệp này TRƯỚC. Nếu không, chính sách thành sai mà
 *   không có gì đỏ lên.
 *
 * VÌ SAO MỤC VỀ BA QUYỀN NAY NẰM THẲNG TRONG DANH SÁCH NÀY:
 *
 *   Trước đây nó nằm sau cửa `bien-the/quyen`, vì có một biến thể bản dựng KHÔNG xin quyền nào
 *   và một chính sách nhắc tới số điện thoại trong bản ấy là mô tả sai đúng bản đang được duyệt.
 *   Biến thể ấy không còn: ba quyền nay thuộc về chính ứng dụng sản phẩm, có mặt trong MỌI bản
 *   dựng. Giữ lại cơ chế tách đôi danh sách khi không còn gì để tách là giữ lại một cái bẫy —
 *   người sau sẽ thêm một mục vào nửa sai và không hiểu vì sao thứ tự đọc ra lộn xộn.
 *
 * VÌ SAO THIẾU MÃ SỐ THUẾ VÀ NGƯỜI ĐẠI DIỆN: không có nguồn. `content/company-profile.ts` chỉ
 * có những gì đã công bố trên vihatsoftware.com và vihatgroup.com. Bịa hai trường ấy trong một
 * văn bản pháp lý là thứ không sửa lại được sau khi nộp. → README §"Còn thiếu"
 */

import { DOAN_CHINH_SACH_TINH_NANG } from "../features/tinh-nang/noi-dung";

export type MucChinhSach = {
  /** Dùng làm khoá React và làm mỏ neo cho test. Không hiện ra. */
  ma: string;
  tieu_de: string;
  /** Mỗi phần tử là một đoạn. Danh sách, không phải một chuỗi có `\n` — JSX vẽ từng đoạn. */
  doan: readonly string[];
};

/**
 * Phiên bản và ngày hiệu lực — HẰNG CÓ TÊN, không rải chuỗi trong component.
 *
 * Một chính sách không ghi phiên bản là một chính sách không chứng minh được nó đã nói gì vào
 * lúc người dùng bấm đồng ý.
 *
 * LÊN `1.1` VÌ NỘI DUNG ĐÃ ĐỔI, KHÔNG PHẢI VÌ ĐỔI CÂU CHỮ CHO ĐẸP: ba quyền nay gắn với ba tính
 * năng của ứng dụng sản phẩm, và mục "Chuyển dữ liệu cho bên thứ ba" nay nói ra việc ứng dụng mở
 * trang bản đồ và trang web của công ty. Hai thay đổi ấy là thay đổi về HÀNH VI, nên số phiên
 * bản phải đổi theo — nếu không thì "phiên bản 1.0" chỉ tên hai văn bản khác nhau.
 */
export const PHIEN_BAN_CHINH_SACH = "1.1";
export const NGAY_HIEU_LUC = "18/09/2026";

export const TIEU_DE_CHINH_SACH = "Chính sách quyền riêng tư";

/**
 * CÂU ĐỨNG ĐẦU, và nó đúng với CẢ HAI biến thể bản dựng.
 *
 * Cố ý không viết "chúng tôi có thể thu thập…" — lối viết phòng thủ ấy sẽ là một câu SAI ở đây,
 * và nó vứt đi điều mạnh nhất app này có để nói.
 */
export const CAU_DAU = "Ứng dụng này không lưu trữ và không gửi đi bất kỳ dữ liệu nào của bạn.";

/**
 * MỘT DANH SÁCH PHẲNG, ĐỌC TỪ TRÊN XUỐNG.
 *
 * ĐÁNH SỐ DO LÚC VẼ QUYẾT ĐỊNH, KHÔNG VIẾT CỨNG VÀO TIÊU ĐỀ: một con số viết cứng sẽ lệch ngay
 * lần thêm hoặc bớt mục kế tiếp — lệch trong một văn bản pháp lý, mà không có gì đỏ lên. Cũng vì
 * thế không câu nào trong tệp này tham chiếu tới "mục số N".
 */
export const MUC_CHINH_SACH: readonly MucChinhSach[] = [
  {
    ma: "ben-xu-ly",
    tieu_de: "Bên xử lý dữ liệu",
    doan: [
      "VihatSoftware — đơn vị thành viên của Tập đoàn ViHAT Group — là bên phát hành ứng dụng này và là bên chịu trách nhiệm về chính sách này.",
      "Địa chỉ trụ sở chính và các đầu mối liên hệ được ghi ở màn Liên hệ của ứng dụng.",
    ],
  },
  {
    ma: "du-lieu",
    tieu_de: "Dữ liệu ứng dụng xử lý",
    doan: [
      "Ứng dụng không yêu cầu bạn nhập bất kỳ thông tin nào: không có biểu mẫu, không có ô đăng nhập, không có ô nhập số điện thoại.",
      "Ứng dụng không đọc danh bạ, không đọc tin nhắn, không đọc thư viện ảnh và không theo dõi hành vi sử dụng của bạn.",
    ],
  },
  {
    ma: "ba-quyen",
    tieu_de: "Ba quyền ứng dụng xin, và vì sao",
    doan: DOAN_CHINH_SACH_TINH_NANG,
  },
  {
    ma: "cach-thuc",
    tieu_de: "Cách xử lý và thời gian lưu",
    doan: [
      "Dữ liệu chỉ tồn tại trong bộ nhớ tạm của phiên làm việc và mất đi khi bạn rời màn hình hoặc đóng ứng dụng.",
      "Không có việc lưu trữ, nên không có thời hạn lưu trữ. Không có bản sao lưu nào chứa dữ liệu của bạn.",
    ],
  },
  {
    ma: "ben-thu-ba",
    tieu_de: "Chuyển dữ liệu cho bên thứ ba",
    doan: [
      "Ứng dụng không gửi dữ liệu của bạn cho bất kỳ bên thứ ba nào, trong nước hay ngoài nước. Nó không có đường gửi dữ liệu đi đâu cả.",
      "Có ba chỗ ứng dụng mở một trang bên ngoài, và cả ba đều chỉ mở khi chính bạn bấm: trang web của công ty, trang bản đồ để chỉ đường tới một văn phòng, và trang web ghi trên mã QR bạn vừa quét. Ứng dụng không gửi kèm thông tin nào của bạn khi mở chúng; từ lúc trang mở ra, việc bạn dùng trang ấy chịu sự điều chỉnh của chính sách bên sở hữu nó.",
      "Nút chỉ đường chỉ mang theo ĐỊA CHỈ VĂN PHÒNG của chúng tôi — không mang theo vị trí của bạn, vì ứng dụng không hề có vị trí của bạn.",
      "Zalo là nền tảng ứng dụng chạy trên đó. Việc bạn dùng Zalo chịu sự điều chỉnh của chính sách quyền riêng tư của Zalo, nằm ngoài phạm vi văn bản này.",
    ],
  },
  {
    ma: "quyen-cua-ban",
    tieu_de: "Quyền của bạn theo Nghị định 13/2023/NĐ-CP",
    doan: [
      "Bạn có quyền được biết, quyền đồng ý và rút lại đồng ý, quyền truy cập, chỉnh sửa, xoá và hạn chế việc xử lý dữ liệu cá nhân của mình, quyền phản đối và quyền khiếu nại.",
      "Trong ứng dụng này, việc thực hiện các quyền ấy rất đơn giản: bạn rút lại đồng ý bằng cách tắt quyền tương ứng trong phần cài đặt của Zalo.",
      "Không có dữ liệu nào để bạn yêu cầu truy cập hay yêu cầu xoá, vì ứng dụng không lưu dữ liệu nào. Nếu bạn muốn xác nhận điều này, hãy liên hệ với chúng tôi theo các đầu mối ở màn Liên hệ.",
    ],
  },
  {
    ma: "rui-ro",
    tieu_de: "Rủi ro có thể xảy ra",
    doan: [
      "Vì ứng dụng không lưu và không gửi dữ liệu đi đâu, không có rủi ro rò rỉ dữ liệu từ phía ứng dụng.",
      "Rủi ro còn lại nằm ở màn hình: nội dung hiển thị sau khi bạn dùng một tính năng có thể bị người đứng cạnh nhìn thấy. Điều này đáng lưu ý nhất khi bạn vừa quét một tấm danh thiếp — thông tin hiện ra là dữ liệu cá nhân của người đã đưa nó cho bạn.",
    ],
  },
  {
    ma: "lien-he",
    tieu_de: "Liên hệ về dữ liệu cá nhân",
    doan: [
      "Mọi câu hỏi, yêu cầu hoặc khiếu nại liên quan tới dữ liệu cá nhân, xin gửi tới các đầu mối ở màn Liên hệ của ứng dụng.",
    ],
  },
  {
    ma: "cam-ket-cap-nhat",
    tieu_de: "Hiệu lực và cam kết cập nhật",
    doan: [
      `Chính sách này có hiệu lực từ ngày ${NGAY_HIEU_LUC}, phiên bản ${PHIEN_BAN_CHINH_SACH}.`,
      "Chúng tôi cam kết cập nhật và công bố chính sách này TRƯỚC khi bắt đầu bất kỳ việc thu thập, lưu trữ hoặc truyền dữ liệu nào mà bản hiện tại chưa có.",
    ],
  },
];
