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
 *     "ảnh không rời khỏi máy"      <- dây bẫy cấm `serverUploadUrl` trên toàn cây mã. ⚠ TỪ 02/10/2026
 *                                      CÂU NÀY CHỈ NÓI VỀ ẢNH DANH THIẾP (tính năng của app chung).
 *                                      Ảnh hiện trường của APP RIÊNG CỦA XÃ có rời máy — không qua
 *                                      `serverUploadUrl` (lệnh cấm vẫn nguyên), mà qua lời gọi của nửa
 *                                      phản ánh (`cong-dan/api/goi-vigov.ts`) sau khi phiếu đã tạo. Mục
 *                                      `anh-hien-truong` khai việc ấy; mọi câu "ảnh không rời máy" còn
 *                                      lại phải nêu tên tính năng danh thiếp — `chinh-sach.test.ts` ghim.
 *     "gửi đi ở đúng HAI chỗ"       <- dây bẫy cấm fetch / XHR / WebSocket / EventSource /
 *                                      axios ở MỌI tệp, miễn cho ĐÚNG HAI TỆP ĐƯỢC KÊ TÊN:
 *                                      `features/dang-nhap/goi-may-chu.ts` (đăng nhập) và
 *                                      `api/goi-may-chu.ts` (yêu cầu tư vấn)
 *
 * ⚠ 22/09/2026 — ỨNG DỤNG BẮT ĐẦU THU THẬP DỮ LIỆU BÁN HÀNG, VÀ ĐÓ LÀ THAY ĐỔI LỚN NHẤT MÀ VĂN
 * BẢN NÀY TỪNG PHẢI GHI:
 *
 *   Tới hôm qua, thứ duy nhất rời khỏi máy là hai mã đăng nhập. Từ hôm nay có một bề mặt "Tư vấn
 *   & báo giá" gửi đi: dòng giải pháp quan tâm · quy mô nhân sự · **một ô ghi chú tự do** · mã
 *   chiến dịch · loại yêu cầu. Ô ghi chú là ô NHẬP đầu tiên trong cả ứng dụng, và nó là thứ đáng
 *   khai nhất trong danh sách ấy: chúng tôi không kiểm soát được người dùng gõ gì vào đó.
 *
 *   ⚠ DANH SÁCH ẤY KHÔNG ĐƯỢC GÕ TAY Ở ĐÂY. Nó được DỰNG RA từ `TRUONG_GUI_DI` trong
 *   `api/hop-dong-yeu-cau.ts` — đúng cùng một cơ chế với câu "Có N chỗ mở trang ngoài", và vì
 *   đúng cùng một lý do. `content/chinh-sach.test.ts` khoá hai chiều: mọi khoá `thanYeuCau()` sinh
 *   ra phải có một dòng khai, và mọi dòng khai phải có mặt nguyên văn trong văn bản này. Thêm một
 *   trường mà quên khai là ĐỎ; dọn một mục khỏi văn bản trong khi mã vẫn gửi trường ấy cũng ĐỎ.
 *
 *   HẠN LƯU CỦA DỮ LIỆU ẤY LÀ **24 THÁNG, RỒI ẨN DANH HOÁ** — không phải xoá. Đọc từ
 *   `vihat-miniapp/migrations/0003_yeu_cau.sql`, hàm `an_danh_yeu_cau_qua_han(interval '24 months')`:
 *   nó đặt `ghi_chu = NULL` và ghi `an_danh_luc`, GIỮ NGUYÊN hàng. Nghĩa là ô ghi chú mất đi, còn
 *   sự kiện "có một yêu cầu loại này, quan tâm sản phẩm này, ngày này" thì ở lại. Viết "sau 24
 *   tháng chúng tôi xoá dữ liệu của bạn" là mô tả một cơ chế không tồn tại, và sai về phía hứa
 *   nhiều hơn thứ mã làm.
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
 * ⚠ CHUYỂN QUYỀN SỞ HỮU APP, 21/09/2026 — VÀ CÂU HỎI TỪNG TREO Ở ĐÂY ĐÃ CÓ LỜI ĐÁP 25/09/2026:
 *
 *   Bên phát hành ứng dụng đổi từ **VihatSoftware** sang **Tập đoàn ViHAT Group**. Văn bản này
 *   vì thế có hai vế phải tách bạch, và trộn chúng là hỏng theo hai kiểu khác nhau:
 *
 *   | Vế | Nói gì | Đã làm gì |
 *   |---|---|---|
 *   | **AI CHỊU TRÁCH NHIỆM** | ai phát hành app, ai chịu trách nhiệm về chính sách | đổi sang **ViHAT Group** (21/09) |
 *   | **AI NHẬN DỮ LIỆU** | dữ liệu đăng nhập đi tới máy chủ `vihat-miniapp` | đổi sang **"máy chủ của Tập đoàn ViHAT Group"** (25/09, câu mở #28) |
 *
 *   Vế thứ hai là một KHẲNG ĐỊNH SỰ THẬT về nơi nhận dữ liệu theo Nghị định 13/2023, nên nó chỉ
 *   được đổi khi có người xác nhận — và từ 21/09 tới 25/09 nó cố ý đứng yên ở "VihatSoftware" vì
 *   chưa ai xác nhận ai vận hành `vihat-miniapp`. Chủ dự án xác nhận ngày 25/09/2026 (câu mở #28,
 *   ADR 0044 câu 2): bên vận hành máy chủ ấy và bên nhận dữ liệu là **Tập đoàn ViHAT Group**,
 *   cùng pháp nhân với bên phát hành. Từ đó chuỗi cũ "máy chủ của VihatSoftware" (kể cả lối viết
 *   "— đơn vị thành viên của Tập đoàn ViHAT Group") là một lời khai SAI nơi nhận: nó vẫn nêu
 *   VihatSoftware là nơi nhận. `chinh-sach.test.ts` và `ket-xuat-ho-so.test.ts` ghim chuỗi mới.
 *
 *   VihatSoftware VẪN xuất hiện ở vai trò ĐƠN VỊ THÀNH VIÊN (danh sách đơn vị, mốc lịch sử, điều
 *   khoản sở hữu trí tuệ) — đó không phải lời khai bên nhận, và không được gộp vào việc sửa này.
 *
 *   Hai câu cũ "là bên phát hành ứng dụng này, không phải một bên thứ ba" đã bị gỡ ngày 21/09 vì
 *   khi ấy vế trước SAI. Chúng KHÔNG được thêm lại trong lượt 25/09: lượt ấy chỉ đổi tên bên nhận
 *   theo đúng câu chữ chủ dự án chọn; viết lại một kết luận pháp lý về "bên thứ ba" là việc khác.
 *
 *   `PHIEN_BAN_CHINH_SACH` GIỮ `1.0` cả sau lượt 25/09, dù đổi bên nhận dữ liệu đúng là loại thay
 *   đổi phải lên số: bản này chưa từng tới tay một người dùng nào (sổ tiến độ `nop-zalo-duyet` vẫn
 *   `chua_lam`), nên đây vẫn là cùng một lượt soạn thảo trước lần công bố đầu tiên — xem khối về
 *   số phiên bản bên dưới. Không ai từng đồng ý ở bản khai "VihatSoftware" để mà phải báo lại.
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

import { cauKhaiTruongGuiDi } from "../api/hop-dong-yeu-cau";

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
 * THE "CHỜ DUYỆT" MARK — at the START of every paragraph whose wording the project owner has not approved
 * (ADR 0047, row "Ảnh hiện trường khi gửi phản ánh"; G9). Written INTO each string, never concatenated at run
 * time, because `bundle-for-zalo.test.ts` requires some sections verbatim in the bundle. The mark shows on screen
 * and in the generated dossier on purpose: a text still carrying it is a text that must NOT be submitted, and
 * anybody reading the dossier sees that at once.
 *
 * ⚠ REMOVE ONLY ON THE OWNER'S APPROVAL, from EVERY paragraph at once, together with the "dấu chờ duyệt" case in
 * `chinh-sach.test.ts` (it turns red when the mark disappears — that is the point). Removing it without an
 * approval publishes, under a real legal entity's name, a sentence nobody has answered for.
 * `PHIEN_BAN_CHINH_SACH` stays 1.0 meanwhile (version block above): no user has ever received this text.
 */
export const PENDING_APPROVAL_MARK = "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt]";


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
  "Ứng dụng này không lưu bất kỳ dữ liệu nào của bạn xuống máy, và chỉ gửi đi ở đúng HAI việc, cả hai đều do chính bạn bấm: khi bạn đăng nhập, nó gửi hai mã dùng một lần do Zalo cấp; và khi bạn bấm gửi một yêu cầu tư vấn, nó gửi những gì bạn vừa chọn và vừa gõ trong màn ấy. Cả hai đi tới máy chủ của Tập đoàn ViHAT Group.";

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
      // ⚠ CÂU NÀY ĐÃ PHẢI SỬA HAI LẦN, VÀ LẦN THỨ HAI (22/09/2026) LÀ LẦN NẶNG NHẤT.
      //
      //   Bản đầu viết "không có ô đăng nhập"; ứng dụng CÓ đăng nhập, chỉ là không có ô nào để gõ.
      //   Bản thứ hai viết "không có biểu mẫu, không yêu cầu bạn nhập bất kỳ thông tin nào" — và
      //   câu ấy thành SAI vào đúng ngày màn "Tư vấn và báo giá" có một ô ghi chú.
      //
      //   Đây chính là chế độ hỏng mà cả tệp này sinh ra để chặn: hành vi đổi, câu chữ ở lại. Nên
      //   câu mới nói ĐỦ BA VẾ, và không vế nào được bỏ: có đúng một ô nhập · nó không bắt buộc ·
      //   việc đăng nhập vẫn không cần gõ gì.
      "Ứng dụng có ĐÚNG MỘT ô để bạn gõ chữ: phần ghi chú trong màn 'Tư vấn và báo giá', và nó không bắt buộc. Ngoài ô ấy, không màn nào yêu cầu bạn nhập gì — không có ô nhập số điện thoại, không có mã sáu số nào phải gõ. Việc đăng nhập là một lần chạm, bằng chính số Zalo bạn đang dùng; mục Đăng nhập bên dưới nói rõ.",
      "Ứng dụng không đọc danh bạ, không đọc tin nhắn và không theo dõi hành vi sử dụng của bạn.",
      // CÂU NÀY ĐÃ PHẢI SỬA, VÀ VIỆC SỬA NÓ LÀ BẮT BUỘC. Bản trước viết "không
      // đọc thư viện ảnh"; nay ứng dụng mở cửa sổ chọn ảnh của Zalo, nên câu ấy đã thành SAI.
      // Một chính sách mô tả sai bản dựng nó nằm trong là thứ Nghị định 13 nhắm tới, và là thứ
      // không sửa lại được sau khi đã nộp duyệt.
      "Ứng dụng không tự đọc thư viện ảnh của bạn. Nó chỉ nhận đúng tấm ảnh bạn tự chọn trong cửa sổ chọn ảnh của Zalo, và chỉ khi chính bạn bấm nút chọn ảnh.",
      // DRAFT 02/10/2026 (PENDING_APPROVAL_MARK): the sentence above stays true, but since 02/10 the commune
      // app also takes a photo with Zalo's camera, and that photo leaves the phone — say so here and point on.
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Trong ứng dụng riêng của một xã, bạn còn có thể tự bấm chụp ảnh bằng máy ảnh của Zalo, hoặc chọn ảnh có sẵn, để gửi kèm một phản ánh. Những ảnh ấy được gửi đi; mục 'Ảnh hiện trường gửi kèm phản ánh' bên dưới nói rõ chúng đi đâu và ai xem được.",
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
      // có mục "Bên xử lý dữ liệu" nói điều ngược lại.
      // ĐỔI BÊN NHẬN 25/09/2026 (câu mở #28, ADR 0044 câu 2): máy chủ `vihat-miniapp` do Tập đoàn
      // ViHAT Group vận hành — cùng pháp nhân với bên phát hành. Câu cũ "máy chủ của
      // VihatSoftware — đơn vị thành viên của Tập đoàn ViHAT Group" vì thế thành khai SAI nơi
      // nhận dữ liệu cá nhân (Nghị định 13/2023), dù nghe như đã nhắc tới tập đoàn: chủ ngữ của
      // "máy chủ của" vẫn là VihatSoftware. Câu chữ dưới đây là lối viết chủ dự án chọn 27/09.
      "Khi bạn bấm đăng nhập, ứng dụng gửi hai mã ấy tới máy chủ của Tập đoàn ViHAT Group. Máy chủ đổi mã tại Zalo bằng một khoá bí mật mà ứng dụng trên máy bạn không có và không được có.",
      "MÁY CHỦ LƯU SỐ ĐIỆN THOẠI CỦA BẠN, và lưu để làm đúng hai việc: làm tên đăng nhập cho những lần bạn mở lại ứng dụng, và làm nơi nhận thông báo ZNS. Ngoài số điện thoại, ứng dụng không gửi thông tin nào khác của bạn đi.",
      "Máy chủ trả về một phiếu phiên, và giữ lại bản ghi của phiên ấy: thời điểm tạo, thời điểm hết hạn, và một bản mã hoá một chiều của chính phiếu — không phải phiếu. Phiên hết hạn sau 7 ngày. Ứng dụng trên máy bạn thì chỉ giữ phiếu trong bộ nhớ: không ghi xuống máy, không hiện ra màn hình, và mất đi khi bạn đóng ứng dụng.",
      // VIẾT LẠI CHO ĐÚNG SAU KHI ĐỌC LƯỢC ĐỒ THẬT
      // (`vihat-miniapp/migrations/0001_init.sql`). Bản nháp viết "mỗi lần bạn đăng nhập" — người
      // đọc hiểu là lần THÀNH CÔNG, và hiểu như vậy là hiểu sai một nửa sự thật.
      "MÁY CHỦ GHI MỘT DÒNG NHẬT KÝ CHO MỖI LƯỢT ĐĂNG NHẬP, KỂ CẢ LƯỢT KHÔNG THÀNH CÔNG. Nghĩa là: nếu bạn bấm đăng nhập rồi việc ấy hỏng giữa chừng — mã hết hạn, Zalo không trả lời, hoặc bạn thử quá nhiều lần — thì địa chỉ IP của bạn vẫn được ghi lại, dù bạn chưa từng đăng nhập thành công lần nào và chưa từng có tài khoản ở đây.",
      "Mỗi dòng nhật ký gồm: thời điểm, địa chỉ IP, lượt ấy thành công hay không, một mã lý do ngắn do chúng tôi đặt, và mã định danh nội bộ của bạn NẾU lượt ấy thành công. Nhật ký KHÔNG chứa số điện thoại của bạn, và cũng không chứa tên, thiết bị hay vị trí.",
      "Nhật ký này dùng để phát hiện lạm dụng — ví dụ một máy thử đăng nhập hàng loạt. Nó là loại dữ liệu CHỈ GHI THÊM: không sửa được và không xoá được, kể cả bởi chính chúng tôi, vì một nhật ký sửa được thì không chứng minh được gì khi có tranh chấp.",
      // ⚠ BẢN NHÁP 27/09/2026 — CHỜ CHỦ DỰ ÁN DUYỆT, chưa công bố. Hai câu dưới khai thân gửi đi ở bước
      // xác nhận xã (`features/dang-nhap/hop-dong.ts` `thanYeuCauCauViGov`). Câu đầu CHỨA NGUYÊN VĂN
      // từng `trong_chinh_sach` của `TRUONG_GUI_DI_CAU_VIGOV`, và `chinh-sach.test.ts` khoá điều đó:
      // đổi một câu khai ở bảng mà không đổi ở đây là ĐỎ. Viết sẵn, không ghép lúc chạy, vì
      // `bundle-for-zalo.test.ts` đòi từng đoạn của mục này có mặt nguyên văn trong bundle.
      // `PHIEN_BAN_CHINH_SACH` giữ 1.0: văn bản chưa từng tới tay người dùng nào (khối số phiên bản ở trên).
      "Khi bạn bấm xác nhận làm việc với một xã, ứng dụng gửi tới máy chủ của Tập đoàn ViHAT Group đúng những thứ sau, và không gì khác: mã phiên Zalo của bạn, để máy chủ mở phiên làm việc với xã ấy — mã này KHÔNG chứa tên hay ảnh đại diện của bạn; tên miền của xã ấy, lấy từ mã QR hoặc đường liên kết bạn đã dùng để mở ứng dụng; và việc bạn đã bấm xác nhận đúng xã — máy chủ không mở phiên với xã nếu bạn chưa xác nhận.",
      "Bước xác nhận xã không gửi mã số điện thoại của bạn, và không chạy nếu bạn không bấm xác nhận.",
      "Bạn có thể dùng ứng dụng mà KHÔNG đăng nhập: toàn bộ phần giới thiệu, danh thiếp, văn phòng và các tính năng khác vẫn dùng được, và hotline cùng email nằm ngay dưới nút đăng nhập.",
    ],
  },
  {
    /**
     * MỤC RIÊNG CHO BỀ MẶT YÊU CẦU — 22/09/2026, và nó có dòng tiêu đề riêng vì đúng lý do mục
     * `dang-nhap` và mục `ghi-tep` có:
     *
     *   Người đọc chính sách để biết "ứng dụng này làm gì với những gì tôi vừa gõ" phải tìm thấy
     *   câu trả lời bằng một dòng tiêu đề, không phải bằng cách đọc hết. Và đây là bề mặt DUY
     *   NHẤT trong cả ứng dụng nhận chữ người dùng tự gõ.
     *
     * ⚠ ĐOẠN LIỆT KÊ DỮ LIỆU ĐƯỢC DỰNG RA TỪ `TRUONG_GUI_DI`, KHÔNG GÕ TAY. Xem khối đầu tệp.
     */
    ma: "yeu-cau-tu-van",
    tieu_de: "Yêu cầu tư vấn và đề nghị gọi lại",
    doan: [
      "Ứng dụng có một màn 'Tư vấn và báo giá'. Nó chỉ mở ra khi bạn tự vào, và chỉ gửi đi khi chính bạn bấm nút gửi. Bạn dùng được toàn bộ phần còn lại của ứng dụng mà không cần chạm tới màn ấy.",
      "Bạn phải đăng nhập trước khi gửi, vì chúng tôi cần biết liên hệ lại với ai. Nếu bạn chưa đăng nhập, màn ấy không hiện biểu mẫu nào — nó mời bạn đăng nhập, hoặc gọi thẳng hotline.",
      // ⚠ KHÔNG GÕ TAY. Dựng từ `TRUONG_GUI_DI` trong `api/hop-dong-yeu-cau.ts`, khoá hai chiều
      // với chính thân yêu cầu — xem khối chú thích đầu tệp này.
      cauKhaiTruongGuiDi(),
      "PHẦN GHI CHÚ LÀ Ô DUY NHẤT TRONG CẢ ỨNG DỤNG NHẬN CHỮ BẠN TỰ GÕ, và chúng tôi không kiểm soát được bạn viết gì vào đó. Bạn không bắt buộc phải điền nó, và chúng tôi khuyên bạn đừng ghi vào đó số căn cước, thông tin tài khoản ngân hàng, hay dữ liệu cá nhân của một người khác.",
      "Ứng dụng KHÔNG gửi kèm tên, địa chỉ, vị trí, danh bạ, ảnh hay thông tin thiết bị của bạn. Máy chủ biết yêu cầu ấy là của ai vì bạn đang đăng nhập — ứng dụng không gửi kèm một trường nào nói 'tôi là ai', và tuyến ấy cũng không nhận một trường như thế.",
      "SAU KHI BẠN GỬI, CHÚNG TÔI CÓ THỂ GỬI MỘT TIN ZNS XÁC NHẬN tới số Zalo bạn đã đăng nhập. Tin ấy có giới hạn số lượng và có thể tạm ngưng, nên không nhận được tin KHÔNG có nghĩa là yêu cầu của bạn chưa tới: bạn xem lại yêu cầu ở màn 'Yêu cầu của tôi' bất cứ lúc nào.",
      "NẾU BẠN BẤM 'ĐỀ NGHỊ GỌI LẠI', một nhân viên của chúng tôi sẽ gọi vào số Zalo bạn đã đăng nhập. Bạn đề nghị gọi lại được tối đa 3 lần trong 24 giờ; quá mức ấy, màn hình nói rõ và mời bạn gọi hotline nếu cần gấp. Chúng tôi không cam kết một mốc thời gian cụ thể cho cuộc gọi ấy.",
      "MÀN 'YÊU CẦU CỦA TÔI' CHỈ HIỆN YÊU CẦU CỦA CHÍNH BẠN. Máy chủ nhận ra bạn từ phiên đăng nhập, không từ một tham số nào ứng dụng gửi lên, nên không có cách nào đổi một con số để đọc yêu cầu của người khác.",
      "THỜI GIAN LƯU YÊU CẦU: 24 THÁNG, RỒI ẨN DANH HOÁ — không phải xoá. Sau 24 tháng kể từ ngày bạn gửi, chúng tôi XOÁ PHẦN GHI CHÚ bạn đã gõ và đánh dấu hàng ấy là đã ẩn danh. Những gì ở lại là sự kiện đã xảy ra: có một yêu cầu loại này, quan tâm dòng giải pháp này, đến từ chiến dịch này, vào ngày này — để thống kê hiệu quả không vỡ khi dữ liệu tới hạn.",
      "Trước mốc 24 tháng, bạn vẫn yêu cầu xoá được bất cứ lúc nào theo cách nói ở mục 'Cách xử lý và thời gian lưu' bên dưới.",
    ],
  },
  {
    /**
     * SCENE PHOTOS — DRAFT 02/10/2026, EVERY PARAGRAPH CARRIES `PENDING_APPROVAL_MARK` (ADR 0047 row "Ảnh hiện
     * trường khi gửi phản ánh"; the owner approves the wording before the Zalo submission).
     *
     *   Its own heading for the reason `ghi-tep` and `dang-nhap` have one: this is the first time a citizen's
     *   PICTURE leaves the phone, and a reader asking "where do my photos go" must find it by a heading. It
     *   names the COMMUNE APP in the heading because the shared app has no photo button (ADR 0047), and the
     *   business-card sentences elsewhere ("ảnh không rời khỏi máy") stay true of THAT feature.
     *
     *   Every claim is read from built code: optional, at most 5, JPEG/PNG/WebP, camera or picker only on the
     *   citizen's tap, explained before Zalo's dialog (`cong-dan/man/scene-photos.tsx`, `zalo-api.ts`); no
     *   `serverUploadUrl`; upload after the petition exists (`cong-dan/api/goi-vigov.ts`); malware scan, re-encode
     *   to JPEG, all EXIF dropped, private bucket, signed links ≤ 15 min, staff views audited (service-petitions,
     *   ADR 0047 G3, ADR 0052 §12); not in the draft (`noi-dung.ts` `not_in_draft`).
     *
     *   ⚠ THE THREE "KHI …" PARAGRAPHS CONTAIN, VERBATIM, every `trong_chinh_sach` of `SCENE_PHOTO_SLOT_FIELDS`,
     *   `SCENE_PHOTO_STORAGE_FIELDS`, `SCENE_PHOTO_COMPLETION_FIELDS` (`ket-xuat-ho-so.ts`). `chinh-sach.test.ts`
     *   locks it: change a declaration there without changing it here and the case is red.
     *
     *   ⚠ RETENTION IS A PLACEHOLDER (ADR 0052 §6: 24 months after closing, still owed to the customer) and no
     *   purge job exists, so the text says "chưa chốt", names the value as provisional, and promises only what the
     *   code does today: the photo stays with the petition.
     *
     *   ⚠ RECIPIENT: written as "máy chủ của Tập đoàn ViHAT Group" — ADR 0044 (02/10/2026: one shared policy;
     *   data recipient ViHAT Group, #28). The owner must confirm this naming for ViGov's storage when approving.
     */
    ma: "anh-hien-truong",
    tieu_de: "Ảnh hiện trường gửi kèm phản ánh (chỉ trong ứng dụng của xã)",
    doan: [
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Mục này chỉ nói về ứng dụng riêng của một xã. Ứng dụng chung của Tập đoàn ViHAT Group không có nút gửi ảnh nào. Những câu ở các mục khác nói rằng ảnh không rời khỏi máy là nói về tính năng số hoá danh thiếp giấy, và vẫn đúng cho tính năng ấy.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Khi gửi một phản ánh, bạn CÓ THỂ đính kèm tối đa 5 ảnh hiện trường. Việc này KHÔNG BẮT BUỘC: không có ảnh, phản ánh vẫn gửi được như thường. Ứng dụng chỉ nhận ảnh JPEG, PNG hoặc WebP, không nhận video.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Ảnh chỉ đến từ hai nút, và chỉ khi chính bạn bấm: 'Chụp ảnh' mở máy ảnh của Zalo — Zalo hỏi bạn có cho phép dùng máy ảnh hay không; 'Chọn ảnh có sẵn' mở cửa sổ chọn ảnh của Zalo. Ứng dụng nói rõ ảnh dùng để làm gì trước khi hộp thoại của Zalo hiện ra. Ứng dụng chỉ nhận những ảnh bạn đã chụp hoặc chọn, không xem các ảnh khác trong máy.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Ảnh chỉ được gửi đi SAU KHI phản ánh của bạn đã được ghi nhận, và đi thẳng vào kho lưu tệp riêng, không công khai, của hệ thống tiếp nhận phản ánh của xã (hệ thống ViGov), trên máy chủ của Tập đoàn ViHAT Group. Zalo không tải ảnh lên hộ: ứng dụng không giao cho Zalo một địa chỉ tải lên nào. Ảnh nào tải chưa được thì không làm hỏng phản ánh đã gửi; bạn tải lại ảnh ấy, hoặc chụp, chọn thêm ảnh, ở 'Phản ánh của tôi' trong lúc phản ánh còn ở bước 'Đã tiếp nhận'.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] KHI XIN CHỖ TẢI MỘT ẢNH, ứng dụng gửi đúng những thứ sau, và không gì khác: mã tra cứu của phản ánh bạn đã gửi, do chính hệ thống của xã cấp, nằm trên đường dẫn; nó chỉ mở được phản ánh của chính bạn · loại ảnh (JPEG, PNG hoặc WebP), đọc từ chính ảnh · dung lượng ảnh tính bằng byte.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] KHI TẢI ẢNH LÊN, ứng dụng gửi tới kho lưu tệp, qua một địa chỉ có chữ ký dùng được trong 15 phút, đúng những thứ sau, và không gì khác: các trường của biểu mẫu tải lên mà chính ViGov cấp ở bước xin chỗ tải (khoá, hạn, chữ ký), gửi lại nguyên văn; không có thông tin nào của bạn · ảnh bạn đã chụp hoặc chọn, đúng loại đã khai; hệ thống của xã bỏ toàn bộ thông tin kèm theo ảnh (như vị trí chụp, thiết bị) trước khi lưu.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] KHI BÁO ĐÃ TẢI XONG, ứng dụng gửi lại mã tra cứu ấy cùng mã của ảnh, do chính hệ thống của xã cấp ở bước xin chỗ tải. Khi bạn mở một phản ánh của mình để xem lại ảnh đã gửi, ứng dụng chỉ gửi mã tra cứu ấy.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] TRƯỚC KHI LƯU, hệ thống quét mã độc trong ảnh, rồi giải mã và mã hoá lại ảnh thành JPEG. Việc mã hoá lại BỎ TOÀN BỘ thông tin kèm theo ảnh (EXIF), kể cả vị trí chụp và thông tin thiết bị. Chỉ bản đã làm sạch được giữ lại; bản bạn gửi lên không được giữ.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] AI XEM ĐƯỢC ẢNH: chính bạn, và cán bộ của đúng xã ấy có quyền xem phản ánh. Ảnh không bao giờ được công khai. Mỗi lần xem là qua một đường dẫn có chữ ký, dùng được tối đa 15 phút. Mỗi lần cán bộ xem ảnh đều được ghi lại trong nhật ký của hệ thống: ai xem, và xem lúc nào.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Ảnh không được giữ trên máy bạn: ảnh không nằm trong bản nháp phản ánh, và ứng dụng không lưu ảnh vào thư viện ảnh của máy. Nếu bạn đóng ứng dụng trước khi gửi, bạn chọn lại ảnh.",
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] THỜI HẠN LƯU ẢNH HIỆN TRƯỜNG: CHƯA CHỐT. [CHỜ DUYỆT — GIÁ TRỊ TẠM] Mức đang dự kiến là 24 tháng sau khi phản ánh được đóng. Cho tới khi thời hạn được chốt, ảnh được giữ cùng phản ánh; thời hạn chính thức sẽ được công bố trong một bản cập nhật của chính sách này trước khi áp dụng.",
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
      // DRAFT 02/10/2026 (PENDING_APPROVAL_MARK): the business-card sentence above stays true and stays put; this
      // one keeps a reader from carrying it over to the commune app's scene photos, which DO leave the phone.
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Ảnh hiện trường bạn tự gửi kèm một phản ánh trong ứng dụng riêng của một xã thì khác ảnh danh thiếp: ảnh ấy được tải lên và lưu ở hệ thống của xã, như mục 'Ảnh hiện trường gửi kèm phản ánh' bên trên nói rõ.",
      "Kiểu kết nối mạng đọc được cũng chỉ hiện lên màn hình rồi mất đi. Ứng dụng không ghi lại lịch sử bạn đã kiểm tra những lần nào.",
      "Phiếu phiên nhận được sau khi bạn đăng nhập cũng chỉ nằm trong bộ nhớ ấy: ứng dụng không ghi nó xuống máy bạn, nên đóng ứng dụng là nó mất đi và lần sau bạn đăng nhập lại bằng một lần chạm.",
      "Trên máy bạn không có việc lưu trữ, nên không có bản sao lưu nào trên máy bạn chứa dữ liệu của bạn.",
      // ⚠ "BỐN THỨ" THÀNH "NĂM THỨ" NGÀY 22/09/2026, và việc sửa con số này là bắt buộc: một câu
      // liệt kê đủ rồi chốt bằng "ngoài bốn thứ ấy, máy chủ KHÔNG lưu gì khác" là một lời khai
      // TRỌN VẸN — thiếu một mục thì cả câu thành sai, không chỉ thiếu.
      "Ở máy chủ, năm thứ được lưu: số điện thoại dùng làm tên đăng nhập của bạn; bản ghi của từng phiên đăng nhập (thời điểm tạo, thời điểm hết hạn, và bản mã hoá một chiều của phiếu phiên); nhật ký đăng nhập; những yêu cầu tư vấn hoặc đề nghị gọi lại bạn đã gửi, nói ở mục 'Yêu cầu tư vấn và đề nghị gọi lại' bên trên; và — chỉ khi bạn từng yêu cầu xoá — một dòng bằng chứng của chính lần xoá ấy, nói ở cuối mục này. Mỗi bản ghi định danh còn mang thời điểm nó được tạo và lần gần nhất được cập nhật.",
      "Ngoài năm thứ ấy, máy chủ KHÔNG lưu gì khác của bạn: không tên, không email, không vị trí, không thông tin thiết bị, không danh bạ, không ảnh. Dịch vụ này cũng không cài công cụ đo hành vi nào và không nhúng bộ công cụ của bên thứ ba nào.",
      // DRAFT 02/10/2026 (PENDING_APPROVAL_MARK): "không ảnh" above is a COMPLETE claim about the login and
      // consultation server; left unscoped it now reads as "no photo is stored anywhere", which the commune
      // app's scene photos make false. Scoped here, without editing the approved sentence.
      "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Năm thứ và câu 'không ảnh' ở trên nói về phần máy chủ phục vụ đăng nhập và yêu cầu tư vấn. Riêng về ảnh: ảnh hiện trường bạn tự gửi kèm một phản ánh trong ứng dụng riêng của một xã được lưu ở hệ thống tiếp nhận phản ánh của xã, như mục 'Ảnh hiện trường gửi kèm phản ánh' bên trên nói rõ.",
      "THỜI HẠN LƯU YÊU CẦU TƯ VẤN: 24 THÁNG kể từ ngày gửi, rồi phần ghi chú bạn tự gõ bị xoá và hàng ấy được đánh dấu là đã ẩn danh. Chi tiết ở mục 'Yêu cầu tư vấn và đề nghị gọi lại' bên trên.",
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
      //      mất: bên nhận và bên phát hành app là MỘT. Từ 21/09 tới 25/09 ai vận hành
      //      `vihat-miniapp` sau chuyển giao chưa ai trả lời, nên câu chỉ nói ĐÚNG THỨ ĐO ĐƯỢC:
      //      có đúng một nơi nhận, và không còn nơi nào khác.
      // ⚠ CÂU NÀY SỬA LẦN THỨ BA (22/09/2026): nay có HAI bước gửi đi, không còn một. Bên NHẬN
      // thì KHÔNG ĐỔI — vẫn đúng một nơi — và việc phân biệt "mấy bước gửi" với "mấy nơi nhận" là
      // toàn bộ giá trị của câu này.
      // ⚠ SỬA LẦN THỨ TƯ (25/09/2026, câu chữ chọn 27/09): câu mở #28 ĐÃ QUYẾT — bên vận hành
      // `vihat-miniapp` và bên nhận dữ liệu là Tập đoàn ViHAT Group. Nơi nhận vẫn là MỘT, chỉ tên
      // đúng của nó đổi. Vế "bên thứ ba" không được thêm lại ở lượt này: lượt ấy chỉ sửa tên bên
      // nhận, không viết một kết luận pháp lý mới.
      "Nơi duy nhất nhận gì đó từ ứng dụng là máy chủ của Tập đoàn ViHAT Group. Có hai bước gửi tới nơi ấy, và cả hai đều do chính bạn bấm: bước đăng nhập nói tại mục Đăng nhập, và bước gửi một yêu cầu tư vấn nói tại mục Yêu cầu tư vấn. Ngoài nơi ấy, ứng dụng không gửi dữ liệu của bạn đi đâu khác, trong nước hay ngoài nước.",
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
      // ADDED 01/10/2026 (owner's answer to ADR 0067 Còn mở #6): links inside a commune's articles. TRUE BY
      // CONSTRUCTION, not by promise: the tap goes to `cong-dan/man/leave-app.tsx`, which asks, then hands the URL
      // to the platform's `openWebview` through `moRaNgoai` — no log line, no counter, no call to any server
      // (`phase1-collects-nothing.test.ts` and `dich-ra-ngoai.test.ts` hold the two doors shut). "Trình duyệt
      // của Zalo" and not "trình duyệt của máy": `openWebview` opens Zalo's in-app browser (`zalo-api.ts`
      // `moTrangWeb`), and naming the phone's browser would describe a step the app does not take.
      "Liên kết trong bài viết của xã mở ra ngoài ứng dụng, trong trình duyệt của Zalo, sau khi bạn xác nhận; ứng dụng không theo dõi việc bạn bấm liên kết nào hay xem gì ở trang ấy.",
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
      "Trên máy bạn không có dữ liệu nào được lưu lại. Với những gì đã gửi tới máy chủ của chúng tôi — ở bước đăng nhập, và ở những yêu cầu tư vấn bạn đã gửi — bạn thực hiện các quyền trên bằng cách liên hệ theo các đầu mối ở màn Liên hệ; chúng tôi trả lời trong thời hạn Nghị định 13/2023/NĐ-CP quy định.",
    ],
  },
  {
    ma: "rui-ro",
    tieu_de: "Rủi ro có thể xảy ra",
    doan: [
      // ⚠ CÂU NÀY ĐÃ PHẢI SỬA 22/09/2026, VÀ ĐÓ LÀ MỘT LẦN BỚT MỘT LỜI TRẤN AN. Bản trước nói
      // "thứ duy nhất nó gửi đi là hai mã đăng nhập" — câu ấy thành SAI từ ngày có bề mặt yêu
      // cầu, và nó sai đúng về phía làm người đọc yên tâm hơn thực tế. Ô ghi chú là văn bản tự
      // do: nó có thể chứa bất cứ thứ gì người dùng gõ vào, nên nó phải có tên trong mục rủi ro.
      "Ứng dụng không lưu dữ liệu nào trên máy bạn. Thứ nó gửi đi là hai mã đăng nhập dùng một lần — số điện thoại của bạn không nằm trong hai mã ấy — và, nếu bạn tự gửi một yêu cầu tư vấn, những gì bạn đã chọn cùng phần ghi chú bạn tự gõ.",
      "RỦI RO ĐÁNG KỂ NHẤT NẰM Ở CHÍNH Ô GHI CHÚ ẤY: nó là văn bản tự do, nên nó chứa đúng những gì bạn viết vào. Bạn hãy cân nhắc trước khi ghi vào đó số căn cước, thông tin tài khoản ngân hàng, hay dữ liệu cá nhân của một người khác — chúng tôi không có cách nào biết trước để ngăn.",
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
