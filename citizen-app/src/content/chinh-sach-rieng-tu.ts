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
 *     "không lưu gì xuống máy bạn"  <- dây bẫy cấm localStorage / sessionStorage / cookie /
 *                                      indexedDB, KHÔNG miễn cho tệp nào
 *     "ảnh không rời khỏi máy"      <- dây bẫy cấm `serverUploadUrl` trên toàn cây mã
 *     "chỉ gửi đi ở bước đăng nhập" <- dây bẫy cấm fetch / XHR / WebSocket / EventSource /
 *                                      axios ở MỌI tệp, miễn cho ĐÚNG MỘT:
 *                                      `features/dang-nhap/goi-may-chu.ts`
 *
 * ⚠ MỘT VĂN BẢN, ĐÚNG CHO CẢ HAI BIẾN THỂ — 20/09/2026, và đây là một quyết định ĐÃ ĐẢO NGƯỢC:
 *
 *   Bản trước của tệp này tách câu mở đầu theo biến thể, vì bản nộp khi ấy không gọi mạng. Tiền
 *   đề đó không còn: **cả hai biến thể nay gọi máy chủ thật** ở bước đăng nhập. Không còn gì để
 *   tách, nên cơ chế tách bị gỡ — giữ lại một cơ chế tách đôi khi hai nửa nói y hệt nhau là
 *   giữ lại đúng cái bẫy mà `MUC_TRUOC_QUYEN` và biến thể `quyen` đã để lại, mỗi thứ một lần.
 *
 *   Và câu mở đầu cũ — *"không lưu trữ và không gửi đi bất kỳ dữ liệu nào của bạn"* — phải BIẾN
 *   MẤT KHỎI CẢ BẢN NỘP, không chỉ khỏi bản đầy đủ. Nó từng là câu mạnh nhất app này có để nói;
 *   hôm nay nó là một tuyên bố sai dưới tên một pháp nhân có thật.
 *
 * ⚠ CHUYỂN QUYỀN SỞ HỮU APP, 21/09/2026 — VÀ MỘT CÂU HỎI CHƯA AI TRẢ LỜI ĐANG TREO Ở ĐÂY:
 *
 *   Bên phát hành ứng dụng đổi từ **VihatSoftware** sang **Tập đoàn ViHAT Group**. Văn bản này
 *   vì thế có hai vế phải tách bạch, và trộn chúng là hỏng theo hai kiểu khác nhau:
 *
 *   | Vế | Nói gì | Đã làm gì |
 *   |---|---|---|
 *   | **AI CHỊU TRÁCH NHIỆM** | ai phát hành app, ai chịu trách nhiệm về chính sách | đổi sang **ViHAT Group** |
 *   | **AI NHẬN DỮ LIỆU** | dữ liệu đăng nhập đi tới "máy chủ của VihatSoftware" (`vihat-miniapp`) | **GIỮ NGUYÊN** |
 *
 *   Vế thứ hai giữ nguyên vì nó là một KHẲNG ĐỊNH SỰ THẬT về nơi nhận dữ liệu theo Nghị định
 *   13/2023: máy chủ ấy là kho `vihat-miniapp`, và **ai vận hành nó sau khi chuyển quyền sở hữu
 *   app thì chưa ai trả lời**. Lật nó sang ViHAT Group là khai sai nơi nhận dữ liệu cá nhân —
 *   đúng thứ Nghị định 13 nhắm tới, và là thứ không sửa lại được sau khi công bố.
 *
 *   ⚠ HỆ QUẢ ĐÃ PHẢI XỬ LÝ NGAY, không chờ được: hai câu cũ nói nơi nhận ấy "là bên phát hành
 *   ứng dụng này, không phải một bên thứ ba". Vế trước nay SAI; vế sau là một kết luận pháp lý
 *   dựa trên vế trước. Cả hai đã được gỡ khỏi hai câu (mục Đăng nhập và mục Chuyển dữ liệu cho
 *   bên thứ ba), giữ lại đúng phần đo được. **CHỦ DỰ ÁN PHẢI QUYẾT** ai vận hành `vihat-miniapp`
 *   trước lần công bố đầu tiên; nếu là một pháp nhân khác bên phát hành thì văn bản này còn nợ
 *   một mục khai chuyển dữ liệu cho bên thứ ba, và mục ấy không ai được tự viết.
 *
 *   `PHIEN_BAN_CHINH_SACH` GIỮ `1.0`: bản này chưa từng tới tay một người dùng nào, nên đây vẫn
 *   là cùng một lượt soạn thảo trước lần công bố đầu tiên — xem khối về số phiên bản bên dưới.
 *
 * ⚠ HAI THỜI HẠN, HAI CÂU TRẢ LỜI KHÁC NHAU, VÀ CẢ HAI ĐỀU ĐÃ CHỐT:
 *
 *   | Lưu gì | Bao lâu | Vì sao không giống nhau |
 *   |---|---|---|
 *   | Số điện thoại | **không có hạn tự động**, tới khi người dùng yêu cầu xoá | nó là danh tính: hết nó là hết tài khoản, nên chủ của nó quyết |
 *   | Nhật ký đăng nhập | **chậm nhất 90 ngày** (thực tế 83–90) | nó chứa IP của cả những người **chưa từng có tài khoản** — họ không có gì để yêu cầu xoá, nên một hạn tự động là cách duy nhất thứ ấy mất đi |
 *   | Dòng bằng chứng của một lần xoá | **vô thời hạn** | một bằng chứng tự huỷ thì không còn là bằng chứng |
 *
 *   ⚠ "90 NGÀY" LÀ TRẦN, KHÔNG PHẢI MỘT CÁI MỐC ĐÚNG NGÀY — và câu chữ phải nói ra đúng như cơ
 *   chế chạy. Backend dọn theo LÔ TUẦN: `nhat_ky_don_qua_han()` DROP cả một phân mảnh tuần khi
 *   ĐẦU khoảng của nó đã quá hạn (`migrations/0002_…sql`, điều kiện `d <= nguong`). Mỗi dòng vì
 *   thế sống **tối đa 90 ngày, tối thiểu 83**. Bản đầu của hàm ấy DROP khi ĐUÔI khoảng quá hạn
 *   — nghe "không xoá sớm của ai", nhưng đẩy dòng cũ nhất lên 97 ngày, tức **hệ thống vượt qua
 *   chính cái trần đã hứa**. Lệch về phía xoá sớm thì người đọc không mất gì.
 *
 *   Hai chỗ trống ngày trước (`THOI_GIAN_LUU_CHUA_CHOT`, `THOI_HAN_LUU_NHAT_KY_CHUA_CHOT`) đã
 *   được lấp và hai hằng ấy biến mất. Cơ chế canh chỗ-trống đã làm đúng việc của nó hai lần:
 *   không ai bịa một con số, và ngày khách chốt thì ca kiểm đỏ lên bắt đi trọn bốn việc.
 *
 *   "Giữ tới khi bạn yêu cầu xoá" KHÔNG phải một cách né con số — nó là một cam kết, và một cam
 *   kết thì phải kèm CỬA THỰC HIỆN. Nên mục ấy nói đủ năm điều, thiếu một là hứa suông: không
 *   có hạn tự động · yêu cầu xoá bằng đường nào (hotline và email trên màn Liên hệ, không cần
 *   đăng nhập, không phải nêu lý do) · xoá thì xoá cái gì (SỐ ĐIỆN THOẠI — không hứa xoá cả bản
 *   ghi, vì lược đồ không cho) · mọi phiên còn hiệu lực bị thu hồi nên các máy đang đăng nhập
 *   sẽ bị đăng xuất · và một DÒNG BẰNG CHỨNG của chính lần xoá ấy ở lại vô thời hạn.
 *
 *   ⚠ DÒNG BẰNG CHỨNG LÀ CHỖ DỄ IM LẶNG NHẤT CỦA CẢ VĂN BẢN: nó là dữ liệu DUY NHẤT về một
 *   người còn ở lại **sau khi** họ đã yêu cầu xoá. Người đọc không nên phát hiện ra nó qua một
 *   đường khác. Đọc từ `vihat-miniapp/migrations/0002_…sql`, bảng `nhat_ky_an_danh`: mã định
 *   danh nội bộ · `hotline` hay `email` · người tiếp nhận (CHECK: không được rỗng) · ghi chú ·
 *   thời điểm. Không cột nào chứa số điện thoại, và cũng không thể có — lúc dòng ấy được ghi
 *   thì số đã bị ghi đè trong cùng một giao dịch.
 *
 * ⚠ MỘT CÂU TRONG VĂN BẢN NÀY KHÔNG CÒN ĐƯỢC GÕ TAY — 21/09/2026:
 *
 *   Câu *"Có <N> chỗ ứng dụng mở một trang bên ngoài…"* trong mục "Chuyển dữ liệu cho bên thứ ba"
 *   nay được DỰNG RA từ `content/dich-ra-ngoai.ts` (`cauKhaiDichRaNgoai`). Con số ấy đã phải sửa
 *   BỐN lần trong hai ngày, lần nào cũng do một người đọc lại văn bản mà phát hiện — và một con
 *   số đếm bằng mắt trong một văn bản pháp lý sắp nộp là con số sẽ có lần không ai đếm.
 *
 *   Nửa còn lại của cơ chế nằm ở `content/dich-ra-ngoai.test.ts`: mã nguồn không có đường nào mở
 *   một trang ngoài mà không gọi tên một đích đã khai. Thêm một lối ra mà quên khai là một ca ĐỎ,
 *   không phải một câu sai trong hồ sơ.
 *
 *   PHIÊN BẢN VẪN LÀ 1.0 dù bề mặt "dữ liệu của bạn có thể tới đâu" vừa rộng ra thật sự (nút Chat
 *   với Official Account, hai liên kết bài viết): văn bản này CHƯA từng tới tay một người dùng
 *   nào, nên đây vẫn là cùng một lượt soạn thảo trước lần công bố đầu tiên — xem khối về số phiên
 *   bản bên dưới. Từ lần công bố đầu trở đi, đúng thay đổi này sẽ phải lên một số mới.
 *
 * VÌ SAO MỤC VỀ CÁC QUYỀN NAY NẰM THẲNG TRONG DANH SÁCH NÀY:
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

import {
  DOAN_CHINH_SACH_TINH_NANG,
  DOAN_CHINH_SACH_TUNG_QUYEN,
} from "../features/tinh-nang/noi-dung";

import { cauKhaiDichRaNgoai } from "./dich-ra-ngoai";

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
 * ⚠ MỘT CÂU ĐÃ THÀNH SAI GIỮA CHỪNG, GHI LẠI VÌ ĐÓ LÀ LOẠI LỖI NẶNG NHẤT TỆP NÀY MẮC PHẢI:
 * *"không có ô đăng nhập"*. Nay ứng dụng CÓ đăng nhập — bằng một lần chạm, không ô nhập nào.
 * Câu ấy được thay bằng một câu nói đúng cả hai vế. Một câu đúng ở bản trước mà không ai sửa
 * khi hành vi đổi là loại lỗi **không có gì đỏ lên** để báo.
 *
 * ⚠ MỘT SỐ PHIÊN BẢN, KHÔNG PHẢI SÁU — VÀ ĐÂY LÀ QUYẾT ĐỊNH DỰA TRÊN BẰNG CHỨNG, KHÔNG PHẢI
 * DỌN CHO GỌN.
 *
 *   Trong một ngày soạn thảo, văn bản này đã đi 1.0 → 1.1 → 1.2 → 1.3 → 1.4 → 1.5, mỗi lần vì
 *   một lý do đúng: hành vi xử lý dữ liệu đổi thì số phải đổi. Nhưng LÝ DO TỒN TẠI của việc lên
 *   số là *"một người đã bấm đồng ý ở bản cũ không biết về thứ mới"* — và nó chỉ có nghĩa khi
 *   CÓ một người như thế.
 *
 *   ĐÃ KIỂM, 20/09/2026, và đây là bằng chứng chứ không phải suy đoán:
 *
 *     • `git log -- citizen-app` — không một commit nào nói tới một lần phát hành;
 *     • sổ tiến độ, mục `nop-zalo-duyet` — `chua_lam`;
 *     • README §"Còn thiếu" #1 — ảnh chụp màn hình và mô tả store **chưa có**, mà Zalo bắt buộc
 *       phải có mới xét duyệt được: chưa nộp được, nói gì tới phát hành;
 *     • README §"Hai thứ đã kiểm bằng cách chạy thật" — phép đo 18/09 ghi rằng đường công khai
 *       trả *"ứng dụng đang trong giai đoạn phát triển"*, tức app CHƯA phát hành; chỉ có bản
 *       THỬ NGHIỆM (`env=TESTING`, Version 6–7), thứ chỉ tài khoản người dựng mở được.
 *
 *   KHÔNG MỘT NGƯỜI DÙNG NÀO TỪNG ĐỌC MỘT BẢN NÀO CỦA VĂN BẢN NÀY. Sáu số trong một ngày vì thế
 *   không bảo vệ ai cả: chúng kể lại quá trình soạn thảo trong một văn bản pháp lý, và làm người
 *   đọc tưởng đã có sáu đợt thay đổi được công bố. Nên bản đầu tiên ra ngoài mang số **`1.0`**.
 *
 *   QUÁ TRÌNH SOẠN THẢO KHÔNG BỊ XOÁ — nó nằm trong `git log` của chính tệp này, đúng nơi lịch
 *   sử soạn thảo thuộc về. Thứ bị bỏ chỉ là việc kể lại nó dưới dạng "lịch sử phiên bản đã công
 *   bố", một điều không có thật.
 *
 * ⚠ TỪ LẦN PHÁT HÀNH ĐẦU TIÊN TRỞ ĐI, QUY TẮC ĐẢO NGƯỢC: mỗi thay đổi về HÀNH VI XỬ LÝ DỮ LIỆU
 * phải lên một số mới, kể cả khi cách nhau vài giờ, và **không được gộp**. Ba ví dụ thật, giữ
 * lại vì chúng nói rõ ranh giới hơn bất kỳ định nghĩa nào:
 *
 *   | Thay đổi | Lên số? |
 *   |---|---|
 *   | `getPhoneNumber` đổi mục đích: "gọi lại tư vấn" → **định danh + thông báo ZNS** | **CÓ** — người đã đồng ý cho việc này chưa đồng ý cho việc kia |
 *   | Khai thêm rằng máy chủ ghi **địa chỉ IP** mỗi lượt đăng nhập | **CÓ** — người đọc bản trước không biết |
 *   | Khai thêm một bảng máy chủ giữ (ví dụ dòng bằng chứng của một lần xoá) | **CÓ** |
 *   | Thêm một quyền nền tảng mới | **CÓ** — bề mặt quyền riêng tư mở rộng thật sự |
 *   | Sửa một con số cho khớp cơ chế đã chạy (90 → "chậm nhất 90, sớm nhất 83") | **CÓ** — người đọc bản trước tưởng mình được giữ đủ 90 |
 *   | Sửa một câu cho dễ đọc, không đổi hành vi nào | KHÔNG |
 */
export const PHIEN_BAN_CHINH_SACH = "1.0";
export const NGAY_HIEU_LUC = "20/09/2026";

export const TIEU_DE_CHINH_SACH = "Chính sách quyền riêng tư";


/**
 * CÂU ĐỨNG ĐẦU — MỘT CÂU, ĐÚNG CHO CẢ HAI BIẾN THỂ.
 *
 * Cố ý không viết "chúng tôi có thể thu thập…" — lối viết phòng thủ ấy mơ hồ đúng ở chỗ phải rõ
 * nhất, và người đọc một chính sách quyền riêng tư đọc đúng câu đầu tiên rồi thôi.
 *
 * Nên câu này nói NGAY ba điều quyết định: ứng dụng gửi gì, khi nào, và cái gì thì không rời
 * khỏi máy. "Đúng một việc" là phần đắt nhất của câu — nó là lời hứa mà dây bẫy `fetch` (miễn
 * cho đúng một tệp) giữ hộ.
 */
export const CAU_DAU =
  "Ứng dụng này không lưu bất kỳ dữ liệu nào của bạn xuống máy, và chỉ gửi đi đúng MỘT việc: khi chính bạn bấm đăng nhập, nó gửi hai mã dùng một lần do Zalo cấp tới máy chủ của VihatSoftware.";

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
      "Tập đoàn ViHAT Group là bên phát hành ứng dụng này và là bên chịu trách nhiệm về chính sách này.",
      "Địa chỉ trụ sở chính và các đầu mối liên hệ được ghi ở màn Liên hệ của ứng dụng.",
    ],
  },
  {
    ma: "du-lieu",
    tieu_de: "Dữ liệu ứng dụng xử lý",
    doan: [
      // CÂU NÀY ĐÃ PHẢI SỬA GIỮA CHỪNG. Bản trước viết "không có ô đăng nhập"; nay ứng dụng
      // CÓ đăng nhập, chỉ là không có ô nào để gõ. Giữ nguyên câu cũ là mô tả sai bản dựng.
      "Ứng dụng không yêu cầu bạn nhập bất kỳ thông tin nào: không có biểu mẫu, không có ô nhập số điện thoại, không có mã sáu số nào phải gõ. Việc đăng nhập là một lần chạm, bằng chính số Zalo bạn đang dùng — mục Đăng nhập bên dưới nói rõ.",
      "Ứng dụng không đọc danh bạ, không đọc tin nhắn và không theo dõi hành vi sử dụng của bạn.",
      // CÂU NÀY ĐÃ PHẢI SỬA, VÀ VIỆC SỬA NÓ LÀ BẮT BUỘC. Bản trước viết "không
      // đọc thư viện ảnh"; nay ứng dụng mở cửa sổ chọn ảnh của Zalo, nên câu ấy đã thành SAI.
      // Một chính sách mô tả sai bản dựng nó nằm trong là thứ Nghị định 13 nhắm tới, và là thứ
      // không sửa lại được sau khi đã nộp duyệt.
      "Ứng dụng không tự đọc thư viện ảnh của bạn. Nó chỉ nhận đúng tấm ảnh bạn tự chọn trong cửa sổ chọn ảnh của Zalo, và chỉ khi chính bạn bấm nút chọn ảnh.",
    ],
  },
  {
    /**
     * MỤC RIÊNG CHO VIỆC ĐĂNG NHẬP — mục DUY NHẤT trong văn bản này nói khác nhau giữa hai biến
     * thể, và nó có một dòng tiêu đề riêng vì đúng lý do mục `ghi-tep` có:
     *
     *   Người đọc chính sách để biết "ứng dụng này làm gì với số điện thoại của tôi" phải tìm
     *   thấy câu trả lời bằng một dòng tiêu đề, không phải bằng cách đọc hết. Và nay
     *   `getPhoneNumber` không còn là "để gọi lại tư vấn" mà là ĐỊNH DANH — thứ quyết định
     *   người dùng thấy gì khi mở lại ứng dụng.
     */
    ma: "dang-nhap",
    tieu_de: "Đăng nhập bằng số điện thoại Zalo",
    doan: [
      "Ứng dụng xin số điện thoại Zalo của bạn để bạn đăng nhập bằng một lần chạm, và để gửi thông báo ZNS tới đúng số ấy khi bạn cần được báo kết quả. Không có ô nhập số điện thoại, và không có mã sáu số nào phải gõ.",
      "Zalo không trả số điện thoại của bạn về máy: ứng dụng chỉ nhận hai mã dùng được một lần, hết hạn sau hai phút. Số điện thoại của bạn không nằm trong hai mã ấy.",
      // ⚠ VẾ "bên phát hành ứng dụng này, không phải một bên thứ ba" ĐÃ BỊ GỠ KHỎI CÂU NÀY,
      // 21/09/2026, và việc gỡ là bắt buộc: từ ngày bên phát hành app là Tập đoàn ViHAT Group,
      // câu ấy khẳng định VihatSoftware là bên phát hành — một câu SAI, trong cùng một văn bản
      // có mục "Bên xử lý dữ liệu" nói điều ngược lại. Thứ CÒN GIỮ NGUYÊN là vế sự thật: dữ
      // liệu đi tới máy chủ của VihatSoftware (`vihat-miniapp`). AI VẬN HÀNH máy chủ ấy sau khi
      // chuyển quyền sở hữu app, và vì thế nơi nhận có phải "bên thứ ba" theo Nghị định 13 hay
      // không, là CÂU CHƯA AI TRẢ LỜI — nên văn bản này không khẳng định gì về nó. Khi chủ dự
      // án trả lời, vế ấy quay lại (hoặc thành một mục khai chuyển dữ liệu cho bên thứ ba).
      "Khi bạn bấm đăng nhập, ứng dụng gửi hai mã ấy tới máy chủ của VihatSoftware — đơn vị thành viên của Tập đoàn ViHAT Group. Máy chủ đổi mã tại Zalo bằng một khoá bí mật mà ứng dụng trên máy bạn không có và không được có.",
      "MÁY CHỦ LƯU SỐ ĐIỆN THOẠI CỦA BẠN, và lưu để làm đúng hai việc: làm tên đăng nhập cho những lần bạn mở lại ứng dụng, và làm nơi nhận thông báo ZNS. Ngoài số điện thoại, ứng dụng không gửi thông tin nào khác của bạn đi.",
      "Máy chủ trả về một phiếu phiên, và giữ lại bản ghi của phiên ấy: thời điểm tạo, thời điểm hết hạn, và một bản mã hoá một chiều của chính phiếu — không phải phiếu. Phiên hết hạn sau 7 ngày. Ứng dụng trên máy bạn thì chỉ giữ phiếu trong bộ nhớ: không ghi xuống máy, không hiện ra màn hình, và mất đi khi bạn đóng ứng dụng.",
      // VIẾT LẠI CHO ĐÚNG SAU KHI ĐỌC LƯỢC ĐỒ THẬT
      // (`vihat-miniapp/migrations/0001_init.sql`). Bản nháp viết "mỗi lần bạn đăng nhập" — người
      // đọc hiểu là lần THÀNH CÔNG, và hiểu như vậy là hiểu sai một nửa sự thật.
      "MÁY CHỦ GHI MỘT DÒNG NHẬT KÝ CHO MỖI LƯỢT ĐĂNG NHẬP, KỂ CẢ LƯỢT KHÔNG THÀNH CÔNG. Nghĩa là: nếu bạn bấm đăng nhập rồi việc ấy hỏng giữa chừng — mã hết hạn, Zalo không trả lời, hoặc bạn thử quá nhiều lần — thì địa chỉ IP của bạn vẫn được ghi lại, dù bạn chưa từng đăng nhập thành công lần nào và chưa từng có tài khoản ở đây.",
      "Mỗi dòng nhật ký gồm: thời điểm, địa chỉ IP, lượt ấy thành công hay không, một mã lý do ngắn do chúng tôi đặt, và mã định danh nội bộ của bạn NẾU lượt ấy thành công. Nhật ký KHÔNG chứa số điện thoại của bạn, và cũng không chứa tên, thiết bị hay vị trí.",
      "Nhật ký này dùng để phát hiện lạm dụng — ví dụ một máy thử đăng nhập hàng loạt. Nó là loại dữ liệu CHỈ GHI THÊM: không sửa được và không xoá được, kể cả bởi chính chúng tôi, vì một nhật ký sửa được thì không chứng minh được gì khi có tranh chấp.",
      "Bạn có thể dùng ứng dụng mà KHÔNG đăng nhập: toàn bộ phần giới thiệu, danh thiếp, văn phòng và các tính năng khác vẫn dùng được, và hotline cùng email nằm ngay dưới nút đăng nhập.",
    ],
  },
  {
    ma: "cac-quyen",
    tieu_de: "Các quyền ứng dụng xin, và vì sao",
    doan: DOAN_CHINH_SACH_TINH_NANG,
  },
  {
    // MỘT MỤC RIÊNG LIỆT KÊ TỪNG QUYỀN THEO ĐÚNG TÊN API. Mục trên kể theo TÍNH NĂNG — thứ
    // người dùng hiểu. Mục này kể theo QUYỀN — thứ Developer Console cấp và người duyệt đối
    // chiếu. Một tính năng dùng hai quyền, nên hai cách kể không thay thế được nhau.
    ma: "tung-quyen",
    tieu_de: "Danh sách từng quyền",
    doan: DOAN_CHINH_SACH_TUNG_QUYEN,
  },
  {
    // HÀNH VI DUY NHẤT ỨNG DỤNG VIẾT LÊN THIẾT BỊ, nên nó có mục riêng thay vì một câu lẫn
    // trong mục khác. Người đọc chính sách để biết "ứng dụng này làm gì với máy tôi" phải tìm
    // thấy nó bằng một dòng tiêu đề, không phải bằng cách đọc hết.
    ma: "ghi-tep",
    tieu_de: "Tệp ứng dụng ghi xuống máy bạn",
    doan: [
      "Ứng dụng ghi đúng MỘT loại tệp xuống máy bạn, và chỉ khi chính bạn bấm nút tải: tệp danh thiếp của ViHAT Group, ở định dạng vCard (.vcf).",
      // ⚠ CÂU NÀY LIỆT KÊ ĐÚNG NHỮNG TRƯỜNG TẤM THIẾP THẬT SỰ CHỨA, và tấm thiếp dựng từ
      // `COMPANY` + `CONTACT` (`features/tinh-nang/vcard.ts`). "trang web" ĐÃ QUAY LẠI danh sách
      // ngày 21/09/2026: `COMPANY.website` được cấp (`https://vihatgroup.com`, đọc từ
      // vihatgroup.com), nên `vcard.ts` sinh lại dòng `URL`. Khai thiếu một trường tệp CÓ, y như
      // khai thừa một trường tệp KHÔNG có, đều là mô tả sai chính thứ người dùng vừa tải về.
      // `chinh-sach.test.ts` buộc hai bên khớp nhau theo CẢ HAI CHIỀU.
      "Tệp ấy chứa tên công ty, hotline, email và trang web của chúng tôi. Nó KHÔNG chứa bất kỳ thông tin nào của bạn, vì ứng dụng không có thông tin nào của bạn để đưa vào.",
      "Nội dung tệp được dựng ngay trên máy bạn và đưa thẳng cho Zalo ghi hộ. Không có một lời gọi mạng nào, không có máy chủ nào tham gia, và Zalo là bên quyết định tệp nằm ở thư mục nào.",
      "Ngoài tệp ấy, ứng dụng không đọc, không sửa, không xoá và không tạo bất kỳ tệp nào khác trên máy bạn.",
    ],
  },
  {
    /**
     * THỜI GIAN LƯU — ĐÃ CHỐT: **không có hạn tự động, giữ tới khi người dùng yêu cầu
     * xoá.** Chỗ này từng để trống vì chưa ai chốt, và chỗ trống ấy có một ca kiểm canh.
     *
     * ⚠ MỘT CAM KẾT KIỂU NÀY PHẢI ĐI KÈM CỬA THỰC HIỆN, nếu không nó là hứa suông — và Nghị
     * định 13 đòi đúng cái cửa ấy. Nên ba đoạn dưới nói đủ ba điều, thiếu một là hỏng:
     *
     *   1. không có hạn tự động — nói THẲNG, vì im lặng về thời hạn và "giữ tới khi bạn yêu cầu
     *      xoá" là hai thứ khác hẳn nhau với người đọc;
     *   2. yêu cầu xoá bằng ĐƯỜNG NÀO — hotline và email đã có sẵn trên màn Liên hệ, không phải
     *      một địa chỉ mới phải dựng;
     *   3. xoá thì XOÁ CÁI GÌ — số điện thoại và bản ghi định danh. Nhật ký đăng nhập ở lại
     *      được, và lý do phải nói ra: nó không chứa số điện thoại.
     */
    ma: "cach-thuc",
    tieu_de: "Cách xử lý và thời gian lưu",
    doan: [
      "Trên máy bạn, dữ liệu chỉ tồn tại trong bộ nhớ tạm của phiên làm việc và mất đi khi bạn rời màn hình hoặc đóng ứng dụng.",
      "Tấm ảnh danh thiếp bạn chọn cũng vậy: ứng dụng chỉ giữ đường dẫn tạm của nó trong bộ nhớ để hiện lên màn hình, và buông ra khi bạn chọn ảnh khác hoặc rời màn hình. Ảnh không được sao chép đi đâu và không được tải lên máy chủ nào.",
      "Kiểu kết nối mạng đọc được cũng chỉ hiện lên màn hình rồi mất đi. Ứng dụng không ghi lại lịch sử bạn đã kiểm tra những lần nào.",
      "Phiếu phiên nhận được sau khi bạn đăng nhập cũng chỉ nằm trong bộ nhớ ấy: ứng dụng không ghi nó xuống máy bạn, nên đóng ứng dụng là nó mất đi và lần sau bạn đăng nhập lại bằng một lần chạm.",
      "Trên máy bạn không có việc lưu trữ, nên không có bản sao lưu nào trên máy bạn chứa dữ liệu của bạn.",
      "Ở máy chủ, bốn thứ được lưu: số điện thoại dùng làm tên đăng nhập của bạn; bản ghi của từng phiên đăng nhập (thời điểm tạo, thời điểm hết hạn, và bản mã hoá một chiều của phiếu phiên); nhật ký đăng nhập; và — chỉ khi bạn từng yêu cầu xoá — một dòng bằng chứng của chính lần xoá ấy, nói ở cuối mục này. Mỗi bản ghi định danh còn mang thời điểm nó được tạo và lần gần nhất được cập nhật.",
      "Ngoài bốn thứ ấy, máy chủ KHÔNG lưu gì khác của bạn: không tên, không email, không vị trí, không thông tin thiết bị, không danh bạ, không ảnh. Dịch vụ này cũng không cài công cụ đo hành vi nào và không nhúng bộ công cụ của bên thứ ba nào.",
      "THỜI HẠN LƯU NHẬT KÝ ĐĂNG NHẬP: CHẬM NHẤT 90 NGÀY. Đây là mức trần: chúng tôi dọn nhật ký theo từng lô mỗi tuần chứ không xoá từng dòng đúng vào ngày thứ 90, nên một dòng có thể bị xoá SỚM HƠN — sớm nhất là ngày thứ 83. Không dòng nào sống quá 90 ngày.",
      "Thời hạn ấy áp cho MỌI dòng, kể cả những dòng của một lượt đăng nhập không thành công. Nghĩa là nếu bạn chưa từng đăng nhập thành công, nên không có gì để yêu cầu xoá, thì địa chỉ IP trong những dòng ấy vẫn tự mất đi trong vòng 90 ngày mà bạn không phải làm gì cả.",
      "THỜI GIAN LƯU SỐ ĐIỆN THOẠI: KHÔNG có hạn tự động. Chúng tôi giữ nó chừng nào bạn còn dùng ứng dụng, và giữ tới khi chính bạn yêu cầu xoá — không có mốc nào tự động xoá, và cũng không có mốc nào tự động giữ thêm.",
      "CÁCH YÊU CẦU XOÁ: gọi hotline hoặc gửi email cho chúng tôi theo hai đầu mối ở màn Liên hệ của ứng dụng. Bạn không cần đăng nhập để yêu cầu, và không phải nêu lý do.",
      // CÂU NÀY ĐÃ PHẢI SỬA, VÀ ĐÓ LÀ SỬA MỘT LỜI HỨA KHÔNG GIỮ ĐƯỢC. Bản nháp viết
      // "chúng tôi xoá số điện thoại VÀ BẢN GHI ĐỊNH DANH". Đọc lược đồ thật
      // (`vihat-miniapp/migrations/0001_init.sql`) thì thấy không làm được: nhật ký đăng nhập
      // tham chiếu tới bản ghi định danh và là bảng CHỈ GHI THÊM (trigger chặn UPDATE/DELETE),
      // nên CSDL sẽ TỪ CHỐI xoá hàng định danh của bất cứ ai từng đăng nhập thành công. Thứ
      // làm được — và cũng là thứ Nghị định 13 gọi là xoá dữ liệu cá nhân — là XOÁ SỐ ĐIỆN
      // THOẠI khỏi bản ghi ấy. Hứa một hành vi mã không làm được là tuyên bố sai dưới tên một
      // pháp nhân, kể cả khi lời hứa nghe mạnh hơn.
      "KHI XOÁ, CHÚNG TÔI XOÁ SỐ ĐIỆN THOẠI CỦA BẠN — thứ duy nhất trong hệ thống nhận ra bạn là ai. Bản ghi còn lại chỉ là một mã định danh nội bộ không gắn với số nào, và những dòng nhật ký cũ vẫn trỏ vào mã ấy; nhưng từ mã ấy không còn số điện thoại nào để tra về bạn nữa.",
      "Cùng lúc đó, mọi phiên đăng nhập còn hiệu lực của bạn bị thu hồi: nếu bạn đang đăng nhập trên một máy nào đó, máy ấy sẽ bị đăng xuất.",
      // KHAI RA MỘT BẢNG NỮA, VÀ ĐÂY LÀ CHỖ DỄ IM LẶNG NHẤT CỦA CẢ VĂN BẢN: nó là dòng dữ liệu
      // DUY NHẤT về một người còn ở lại SAU KHI họ đã yêu cầu xoá. Người đọc không nên phát
      // hiện ra nó qua một đường khác. Đọc từ `vihat-miniapp/migrations/0002_…sql`, bảng
      // `nhat_ky_an_danh`: `nguoi_dung_id` · `nguon_yeu_cau` (CHECK: 'hotline' | 'email') ·
      // `nguoi_thuc_hien` (CHECK: không được rỗng) · `ghi_chu` · `tao_luc`. Không có cột nào
      // chứa số điện thoại, và cũng không thể có: lúc dòng này được ghi thì số đã bị ghi đè rồi.
      "CHÚNG TÔI GIỮ LẠI MỘT DÒNG BẰNG CHỨNG CHO CHÍNH VIỆC XOÁ ẤY, và nói ra ở đây để bạn không phát hiện nó qua một đường khác. Dòng ấy gồm: mã định danh nội bộ của bạn, yêu cầu tới qua hotline hay qua email, tên người tiếp nhận và thực hiện, một ghi chú ngắn (số phiếu hoặc tiêu đề email), và thời điểm. Nó KHÔNG chứa số điện thoại của bạn — lúc nó được ghi thì số đã bị xoá khỏi hệ thống rồi.",
      "Dòng bằng chứng ấy được giữ VÔ THỜI HẠN, nằm ngoài quy tắc 90 ngày, và lý do nằm ở chính công dụng của nó: nó là thứ chứng minh chúng tôi ĐÃ làm điều đã hứa với bạn, và trả lời được câu 'ai cho phép xoá dữ liệu của người này'. Một bằng chứng tự huỷ sau một thời gian thì không còn là bằng chứng.",
      "NẾU BẠN CHƯA TỪNG ĐĂNG NHẬP THÀNH CÔNG: chúng tôi không có bản ghi định danh nào của bạn để mà xoá — chỉ có những dòng nhật ký ghi lại thời điểm, địa chỉ IP và việc lượt ấy đã hỏng vì lý do gì. Chúng tôi không xoá riêng những dòng ấy theo yêu cầu được, vì nhật ký chỉ ghi thêm, và cũng không có cách nào biết dòng nào là của bạn: trong hệ thống không có gì khác của bạn để đối chiếu. Nhưng bạn không cần yêu cầu — chúng tự hết hạn và bị xoá sau 90 ngày, như mọi dòng nhật ký khác.",
    ],
  },
  {
    ma: "ben-thu-ba",
    tieu_de: "Chuyển dữ liệu cho bên thứ ba",
    doan: [
      // ĐÃ SỬA HAI LẦN, VÀ LẦN THỨ HAI LÀ LẦN BỚT MỘT LỜI KHẲNG ĐỊNH:
      //   1. (20/09) câu cũ "nó không có đường gửi dữ liệu đi đâu cả" chỉ còn đúng ở bản nộp.
      //   2. (21/09) câu "không gửi cho BẤT KỲ BÊN THỨ BA NÀO" đứng được là nhờ một tiền đề đã
      //      mất: bên nhận và bên phát hành app là MỘT. Từ khi app thuộc Tập đoàn ViHAT Group,
      //      việc máy chủ của VihatSoftware có phải "bên thứ ba" theo Nghị định 13 hay không
      //      phụ thuộc vào câu chưa ai trả lời — ai vận hành `vihat-miniapp` sau chuyển giao.
      //      Nên câu nay nói ĐÚNG THỨ ĐO ĐƯỢC: có đúng một nơi nhận, và không còn nơi nào khác.
      //      Khẳng định pháp lý về "bên thứ ba" chờ chủ dự án, không ai tự viết lại.
      "Nơi duy nhất nhận gì đó từ ứng dụng là máy chủ của VihatSoftware — đơn vị thành viên của Tập đoàn ViHAT Group — ở bước đăng nhập nói tại mục Đăng nhập bên trên. Ngoài nơi ấy, ứng dụng không gửi dữ liệu của bạn đi đâu khác, trong nước hay ngoài nước.",
      // ⚠ CÂU NÀY KHÔNG CÒN ĐƯỢC GÕ TAY — 21/09/2026. Nó được DỰNG RA từ `DICH_MO_RA_NGOAI`, và
      // đó là cách duy nhất con số và danh sách không lệch nhau được nữa.
      //
      // LỊCH SỬ CỦA ĐÚNG MỘT CON SỐ, giữ lại vì nó là toàn bộ lý lẽ:
      //
      //   20/09  ba: bản đồ · trang web trên mã QR · neo website chính thức.
      //   21/09  hai: `COMPANY.website` trống nên neo website không vẽ ra.
      //   21/09  ba trở lại: website được cấp, neo vẽ lại, và trang chi tiết giải pháp thêm một
      //          neo TỚI CÙNG ĐỊA CHỈ ẤY.
      //   21/09  năm: nút Chat với Official Account (mọi màn) và hai liên kết bài viết trên trang
      //          tin của chúng tôi (màn chủ).
      //
      // Bốn lần sửa trong hai ngày, lần nào cũng do một người ĐỌC LẠI văn bản mà phát hiện — không
      // lần nào do một phép kiểm. Một con số đếm bằng mắt trong một văn bản pháp lý sắp nộp là
      // con số sẽ có lần không ai đếm, và khai thiếu một nơi dữ liệu người dùng có thể đi tới là
      // đúng thứ Nghị định 13 nhắm tới.
      //
      // Cách đếm KHÔNG ĐỔI — theo LOẠI ĐÍCH ĐẾN, không theo số nút: hai neo website là một dòng.
      // Xem `content/dich-ra-ngoai.ts`, nơi cả danh sách lẫn quy ước đếm được ghi ra một lần.
      cauKhaiDichRaNgoai(),
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
      // ĐÃ SỬA, cùng lý do với mục "Chuyển dữ liệu cho bên thứ ba": câu cũ ("không có
      // dữ liệu nào để yêu cầu xoá") chỉ đúng khi ứng dụng không gửi gì đi. Vế "trên máy bạn"
      // thì đúng ở mọi bản dựng, và vế còn lại được chỉ sang đúng chỗ người đọc phải đi.
      "Trên máy bạn không có dữ liệu nào được lưu lại. Với những gì đã gửi tới máy chủ của chúng tôi ở bước đăng nhập, bạn thực hiện các quyền trên bằng cách liên hệ theo các đầu mối ở màn Liên hệ; chúng tôi trả lời trong thời hạn Nghị định 13/2023/NĐ-CP quy định.",
    ],
  },
  {
    ma: "rui-ro",
    tieu_de: "Rủi ro có thể xảy ra",
    doan: [
      "Ứng dụng không lưu dữ liệu nào trên máy bạn, và thứ duy nhất nó gửi đi là hai mã đăng nhập dùng một lần — số điện thoại của bạn không nằm trong hai mã ấy. Nên rủi ro rò rỉ dữ liệu từ phía ứng dụng là rất hẹp.",
      "Rủi ro còn lại nằm ở màn hình: nội dung hiển thị sau khi bạn dùng một tính năng có thể bị người đứng cạnh nhìn thấy. Điều này đáng lưu ý nhất khi bạn vừa quét một tấm danh thiếp, hoặc vừa chọn ảnh một tấm thiếp giấy — thứ hiện ra là dữ liệu cá nhân của người đã đưa nó cho bạn, và bạn là người đang giữ nó.",
      "Khi bạn bật chế độ giữ màn hình sáng để người khác quét mã, màn hình sẽ không tự tối đi. Ứng dụng tắt chế độ ấy ngay khi bạn rời màn hình danh thiếp, nhưng trong lúc đang bật, những gì trên màn hình nằm trong tầm nhìn của người xung quanh lâu hơn bình thường.",
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
