/**
 * CHÍNH SÁCH QUYỀN RIÊNG TƯ — mọi câu chữ, một tệp. Theo Nghị định 13/2023/NĐ-CP.
 *
 * ⚠ NGUYÊN TẮC CHI PHỐI TOÀN BỘ TỆP NÀY:
 *
 *   Văn bản này công bố dưới tên một PHÁP NHÂN CÓ THẬT. Một câu mô tả hành vi mà mã không có là
 *   một tuyên bố sai dưới tên ấy, và có người phải trả lời cho nó. Ngược lại, giấu một hành vi
 *   mà mã CÓ là vi phạm chính Nghị định 13. Nên mỗi câu ở đây phải đối chiếu được với mã.
 *
 *   Cụ thể, hai câu dưới đây đúng vì các dây bẫy CẤM điều ngược lại trên toàn cây mã — không phải
 *   vì ai đó hứa:
 *
 *     "không lưu gì xuống máy bạn"  <- dây bẫy cấm localStorage / sessionStorage / cookie /
 *                                      indexedDB; miễn đúng MỘT tệp, bản nháp phản ánh của giao
 *                                      diện xã (`ranh-gioi-hai-nua.test.ts` §3b) — không phải
 *                                      ứng dụng này, và không đường nào từ ứng dụng này tới nó
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
 * ⚠ 11/10/2026 — TWO APPS TO THE USER, TWO POLICIES (owner decisions of 11/10/2026):
 *
 *   This text is the ViHAT Group app's — the company intro and its tools — and nothing else. The commune page a
 *   commune QR opens (`App.tsx` `QrCommuneApp`), and a commune's own app, are the COMMUNE'S app to the user; their
 *   data flows (petitions, scene photos, the on-phone draft, the location exchange, the session with the commune)
 *   moved to `content/commune-privacy.ts`, published in the name of the commune's People's Committee. No sentence
 *   here names a commune, a petition or ViGov (`chinh-sach.test.ts` pins it).
 *
 *   The owner approved the wording the same day: the "[CHỜ DUYỆT …]" marks and their machinery are gone. The
 *   intro features removed that day (paper-card photo, network type, location for offices) left the text with
 *   their code.
 *
 *   ZNS about a petition's progress is the commune's, not mentioned here.
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

import { cauKhaiDichRaNgoai, INTRO_DESTINATIONS } from "./dich-ra-ngoai";

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
/**
 * 11/10/2026: the day the text became what it now says — the commune sections left for the commune's own policy
 * (`commune-privacy.ts`), the removed intro features left with them, and the owner approved the wording (decision
 * of 11/10/2026). A legal text dated before the facts it states misstates its own date. Still `1.0`: never published.
 * Moved to 12/10/2026: that day added the consultation photos and "Tìm văn phòng gần tôi" (owner, App ViHAT upgrade
 * for the Zalo resubmission) — facts the 11/10 date would predate.
 */
export const NGAY_HIEU_LUC = "12/10/2026";

export const TIEU_DE_CHINH_SACH = "Chính sách quyền riêng tư";

/**
 * CÂU ĐỨNG ĐẦU — MỘT CÂU.
 *
 * Cố ý không viết "chúng tôi có thể thu thập…" — lối viết phòng thủ ấy mơ hồ đúng ở chỗ phải rõ
 * nhất, và người đọc một chính sách quyền riêng tư đọc đúng câu đầu tiên rồi thôi.
 *
 * Nên câu này nói NGAY ba điều quyết định: ứng dụng gửi gì, khi nào, và tới đâu.
 *
 * 11/10/2026 (owner): the "hai cách dùng" framing is gone. To the user this is ViHAT Group's app and nothing else;
 * the commune page a commune QR opens is the commune's app, with the commune's own policy (`commune-privacy.ts`).
 */
// 12/10/2026 (owner): "Tìm văn phòng gần tôi" joined the list — the one-time location exchange is a send too.
export const CAU_DAU =
  "Ứng dụng này là ứng dụng của Tập đoàn ViHAT Group trên Zalo. Ứng dụng không lưu dữ liệu nào của bạn xuống máy, và chỉ gửi dữ liệu đi khi chính bạn bấm đăng nhập, gửi một yêu cầu tư vấn, bấm 'Chat với chuyên viên', đăng ký hay huỷ nhận ưu đãi qua SMS, hoặc bấm 'Tìm văn phòng gần tôi' rồi cho phép lấy vị trí — tới máy chủ của Tập đoàn ViHAT Group.";

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
      //   Nên câu mới nói ĐỦ BA VẾ, và không vế nào được bỏ: có đúng một ô nhập · nó không bắt buộc ·
      //   việc đăng nhập vẫn không cần gõ gì.
      "Ứng dụng có ĐÚNG MỘT ô để bạn gõ chữ: phần ghi chú trong màn 'Tư vấn và báo giá', và nó không bắt buộc. Ngoài ô ấy, ứng dụng không yêu cầu bạn nhập gì — không có ô nhập số điện thoại, không có mã sáu số nào phải gõ. Việc đăng nhập là một lần chạm, bằng chính số Zalo bạn đang dùng; mục Đăng nhập bên dưới nói rõ.",
      "Ứng dụng không đọc danh bạ, không đọc tin nhắn và không theo dõi hành vi sử dụng của bạn.",
      // 11/10/2026: the paper-card photo block is gone (owner), so the camera served the QR scan only.
      // 12/10/2026 (owner): "Tư vấn và báo giá" attaches up to 3 photos — the camera and the picker are back, for
      // that one screen, and the sentence says both uses and that nothing is kept on the phone.
      "Ứng dụng dùng máy ảnh ở hai chỗ, và chỉ khi chính bạn bấm: để quét mã QR khi bạn bấm 'Quét mã QR'; và để chụp ảnh gửi kèm một yêu cầu tư vấn khi bạn bấm 'Chụp ảnh' ở màn 'Tư vấn và báo giá'. Ứng dụng chỉ mở thư viện ảnh khi bạn bấm 'Chọn ảnh' ở cùng màn ấy, và chỉ nhận những ảnh bạn chọn — không xem các ảnh khác trong máy. Ứng dụng không lưu ảnh nào trên máy bạn.",
    ],
  },
  {
    /**
     * MỤC RIÊNG CHO VIỆC ĐĂNG NHẬP — nó có một dòng tiêu đề riêng vì đúng lý do mục `ghi-tep` có:
     *
     *   Người đọc chính sách để biết "ứng dụng này làm gì với số điện thoại của tôi" phải tìm
     *   thấy câu trả lời bằng một dòng tiêu đề, không phải bằng cách đọc hết. Và `getPhoneNumber`
     *   không phải "để gọi lại tư vấn" mà là ĐỊNH DANH — thứ quyết định người dùng thấy gì khi mở
     *   lại ứng dụng.
     *
     * Every paragraph is a literal string: `bundle-for-zalo.test.ts` wants each of them verbatim in the bundle.
     */
    ma: "dang-nhap",
    tieu_de: "Đăng nhập bằng số điện thoại Zalo",
    doan: [
      "Ứng dụng xin số điện thoại Zalo của bạn để bạn đăng nhập bằng một lần chạm, và để gửi thông báo ZNS tới đúng số ấy khi bạn cần được báo kết quả. Không có ô nhập số điện thoại, và không có mã sáu số nào phải gõ.",
      "Zalo không trả số điện thoại của bạn về máy: ứng dụng chỉ nhận hai mã dùng được một lần, hết hạn sau hai phút. Số điện thoại của bạn không nằm trong hai mã ấy.",
      // ĐỔI BÊN NHẬN 25/09/2026 (câu mở #28, ADR 0044 câu 2): máy chủ `vihat-miniapp` do Tập đoàn
      // ViHAT Group vận hành — cùng pháp nhân với bên phát hành. Câu chữ là lối viết chủ dự án chọn 27/09.
      "Khi bạn bấm đăng nhập, ứng dụng gửi hai mã ấy tới máy chủ của Tập đoàn ViHAT Group. Máy chủ đổi mã tại Zalo bằng một khoá bí mật mà ứng dụng trên máy bạn không có và không được có.",
      // 07/10/2026: "đúng hai việc" was false once the number also became the one a specialist sees on "Chat với
      // chuyên viên" and the one promotional SMS go to on "Đăng ký nhận" — each only on the citizen's own tap.
      "MÁY CHỦ LƯU SỐ ĐIỆN THOẠI CỦA BẠN, và lưu để làm những việc sau: làm tên đăng nhập cho những lần bạn mở lại ứng dụng; làm nơi nhận thông báo ZNS; và — chỉ khi chính bạn bấm — làm số chúng tôi gọi lại khi bạn đề nghị gọi lại, số chuyên viên thấy khi bạn bấm 'Chat với chuyên viên', và số nhận tin nhắn SMS ưu đãi khi bạn bấm 'Đăng ký nhận', cho tới khi bạn bấm 'Huỷ nhận ưu đãi SMS'. Ở bước đăng nhập này, ngoài hai mã ấy, ứng dụng không gửi thông tin nào khác của bạn đi.",
      "Máy chủ trả về một phiếu phiên, và giữ lại bản ghi của phiên ấy: thời điểm tạo, thời điểm hết hạn, và một bản mã hoá một chiều của chính phiếu — không phải phiếu. Phiên hết hạn sau 7 ngày. Ứng dụng trên máy bạn thì chỉ giữ phiếu trong bộ nhớ: không ghi xuống máy, không hiện ra màn hình, và mất đi khi bạn đóng ứng dụng.",
      // VIẾT LẠI CHO ĐÚNG SAU KHI ĐỌC LƯỢC ĐỒ THẬT (`vihat-miniapp/migrations/0001_init.sql`). Bản nháp viết
      // "mỗi lần bạn đăng nhập" — người đọc hiểu là lần THÀNH CÔNG, và hiểu như vậy là hiểu sai một nửa sự thật.
      "MÁY CHỦ GHI MỘT DÒNG NHẬT KÝ CHO MỖI LƯỢT ĐĂNG NHẬP, KỂ CẢ LƯỢT KHÔNG THÀNH CÔNG. Nghĩa là: nếu bạn bấm đăng nhập rồi việc ấy hỏng giữa chừng — mã hết hạn, Zalo không trả lời, hoặc bạn thử quá nhiều lần — thì địa chỉ IP của bạn vẫn được ghi lại, dù bạn chưa từng đăng nhập thành công lần nào và chưa từng có tài khoản ở đây.",
      "Mỗi dòng nhật ký gồm: thời điểm, địa chỉ IP, lượt ấy thành công hay không, một mã lý do ngắn do chúng tôi đặt, và mã định danh nội bộ của bạn NẾU lượt ấy thành công. Nhật ký KHÔNG chứa số điện thoại của bạn, và cũng không chứa tên, thiết bị hay vị trí.",
      "Nhật ký này dùng để phát hiện lạm dụng — ví dụ một máy thử đăng nhập hàng loạt. Nó là loại dữ liệu CHỈ GHI THÊM: không sửa được và không xoá được, kể cả bởi chính chúng tôi, vì một nhật ký sửa được thì không chứng minh được gì khi có tranh chấp.",
      "Bạn có thể dùng ứng dụng mà KHÔNG đăng nhập: thông tin về Tập đoàn, giải pháp, tin tức, danh thiếp, văn phòng và các tính năng khác vẫn dùng được, và hotline cùng email nằm ngay dưới nút đăng nhập.",
    ],
  },
  {
    /**
     * MỤC RIÊNG CHO BỀ MẶT YÊU CẦU — 22/09/2026, và nó có dòng tiêu đề riêng vì đúng lý do mục
     * `dang-nhap` và mục `ghi-tep` có: "ứng dụng này làm gì với những gì tôi vừa gõ" phải tìm thấy
     * bằng một dòng tiêu đề.
     *
     * ⚠ ĐOẠN LIỆT KÊ DỮ LIỆU ĐƯỢC DỰNG RA TỪ `TRUONG_GUI_DI`, KHÔNG GÕ TAY. Xem khối đầu tệp.
     */
    ma: "yeu-cau-tu-van",
    tieu_de: "Yêu cầu tư vấn và đề nghị gọi lại",
    doan: [
      "Ứng dụng có một màn 'Tư vấn và báo giá'. Nó chỉ mở ra khi bạn tự vào, và chỉ gửi đi khi chính bạn bấm nút gửi. Bạn dùng được toàn bộ phần còn lại của ứng dụng mà không cần chạm tới màn ấy.",
      "Bạn phải đăng nhập trước khi gửi, vì chúng tôi cần biết liên hệ lại với ai. Nếu bạn chưa đăng nhập, màn ấy không hiện biểu mẫu nào — nó mời bạn đăng nhập, hoặc gọi thẳng hotline.",
      // ⚠ KHÔNG GÕ TAY. Dựng từ `TRUONG_GUI_DI` trong `api/hop-dong-yeu-cau.ts`, khoá hai chiều với chính thân
      // yêu cầu. Joined at run time is fine here: no test needs this section verbatim in the bundle (only its
      // heading, `bundle-for-zalo.test.ts`).
      cauKhaiTruongGuiDi(),
      "PHẦN GHI CHÚ LÀ Ô DUY NHẤT TRONG ỨNG DỤNG NHẬN CHỮ BẠN TỰ GÕ, và chúng tôi không kiểm soát được bạn viết gì vào đó. Bạn không bắt buộc phải điền nó, và chúng tôi khuyên bạn đừng ghi vào đó số căn cước, thông tin tài khoản ngân hàng, hay dữ liệu cá nhân của một người khác.",
      // 12/10/2026 (owner): THE PHOTOS — what is collected, where it goes, who sees it, what the server strips, how
      // long, how it is erased. Read from `vihat-miniapp` README §"Ảnh đính kèm yêu cầu" (d9592b71 + 2cf31a30):
      // `consult` only, ≤ 3 × 5 MB, EXIF/XMP removed, `bytea` kept as long as the request (24 months), bytes + hash
      // deleted by the 24-month job and by an erasure (`make an-danh`), specialists read them through the request.
      "ẢNH GỬI KÈM YÊU CẦU TƯ VẤN: ở màn 'Tư vấn và báo giá', bạn có thể tự chụp hoặc chọn tối đa 3 ảnh, mỗi ảnh không quá 5 MB, và việc này không bắt buộc. Ứng dụng chỉ nhận đúng những ảnh bạn chụp hoặc chọn. Sau khi yêu cầu tư vấn đã được ghi nhận, từng ảnh được gửi tới máy chủ của Tập đoàn ViHAT Group — cùng nơi lưu yêu cầu — để đính vào chính yêu cầu ấy; ảnh không gửi kèm đề nghị gọi lại. Máy chủ bỏ thông tin vị trí chụp và các thông tin kèm theo khác trong ảnh trước khi lưu. Mỗi ảnh lưu xong, máy chủ báo cho hệ thống tiếp nhận yêu cầu của Tập đoàn ViHAT Group kèm một đường tải ảnh chỉ dùng được trong 24 giờ; chuyên viên của Tập đoàn ViHAT Group mở ảnh qua đường ấy để tư vấn cho bạn. Lời báo không kèm số điện thoại, tên hay ghi chú của bạn. Ảnh được lưu bằng thời hạn của yêu cầu — 24 tháng —, rồi bị xoá; nếu bạn yêu cầu xoá dữ liệu, ảnh bị xoá cùng lúc với phần ghi chú của yêu cầu. Ứng dụng không lưu ảnh nào trên máy bạn, và chúng tôi khuyên bạn đừng gửi ảnh giấy tờ tuỳ thân hay ảnh có dữ liệu cá nhân của người khác.",
      // 07/10/2026: "KHÔNG gửi kèm tên" became false for one kind — the chat request carries the Zalo name.
      // 12/10/2026: and "KHÔNG gửi kèm … ảnh" became false for the consultation request — the photos you choose.
      "Trừ tên hiển thị Zalo đi kèm một yêu cầu trò chuyện với chuyên viên — chỉ khi bạn đồng ý chia sẻ, mục 'Chat với chuyên viên và nhận ưu đãi qua SMS' nói rõ — và những ảnh bạn tự chụp hoặc chọn gửi kèm một yêu cầu tư vấn, ứng dụng KHÔNG gửi kèm tên, địa chỉ, vị trí, danh bạ, ảnh nào khác hay thông tin thiết bị của bạn. Máy chủ biết yêu cầu ấy là của ai vì bạn đang đăng nhập — ứng dụng không gửi kèm một trường nào nói 'tôi là ai', và tuyến ấy cũng không nhận một trường như thế.",
      "SAU KHI BẠN GỬI, CHÚNG TÔI CÓ THỂ GỬI MỘT TIN ZNS XÁC NHẬN tới số Zalo bạn đã đăng nhập. Tin ấy có giới hạn số lượng và có thể tạm ngưng, nên không nhận được tin KHÔNG có nghĩa là yêu cầu của bạn chưa tới: bạn xem lại yêu cầu ở màn 'Yêu cầu của tôi' bất cứ lúc nào.",
      "NẾU BẠN BẤM 'ĐỀ NGHỊ GỌI LẠI', một nhân viên của chúng tôi sẽ gọi vào số Zalo bạn đã đăng nhập. Bạn đề nghị gọi lại được tối đa 3 lần trong 24 giờ; quá mức ấy, màn hình nói rõ và mời bạn gọi hotline nếu cần gấp. Chúng tôi không cam kết một mốc thời gian cụ thể cho cuộc gọi ấy.",
      "MÀN 'YÊU CẦU CỦA TÔI' CHỈ HIỆN YÊU CẦU CỦA CHÍNH BẠN. Máy chủ nhận ra bạn từ phiên đăng nhập, không từ một tham số nào ứng dụng gửi lên, nên không có cách nào đổi một con số để đọc yêu cầu của người khác.",
      "THỜI GIAN LƯU YÊU CẦU: 24 THÁNG, RỒI ẨN DANH HOÁ — không phải xoá. Sau 24 tháng kể từ ngày bạn gửi, chúng tôi XOÁ PHẦN GHI CHÚ bạn đã gõ và đánh dấu hàng ấy là đã ẩn danh. Những gì ở lại là sự kiện đã xảy ra: có một yêu cầu loại này, quan tâm dòng giải pháp này, đến từ chiến dịch này, vào ngày này — để thống kê hiệu quả không vỡ khi dữ liệu tới hạn.",
      "Trước mốc 24 tháng, bạn vẫn yêu cầu xoá được bất cứ lúc nào theo cách nói ở mục 'Cách xử lý và thời gian lưu' bên dưới.",
    ],
  },
  {
    /**
     * THE TWO ONE-TAP REQUESTS (owner decision 07/10/2026; wording approved 11/10/2026).
     *
     *   What each sentence is read from:
     *     · flow — `features/yeu-cau/quick-request.ts` (`chatWithSpecialist`, `sendSmsChoice`), login through
     *       `features/dang-nhap/ensure-session.ts`, name through `zalo-api.ts` `layTenZalo(true)`, chat through
     *       the declared `chat-oa` destination;
     *     · body — `api/hop-dong-yeu-cau.ts` `thanYeuCau`: `displayName` on `chat` only, no phone in any body;
     *     · subscription — derived from the citizen's own list, in memory (`deriveSmsSubscription`).
     *
     *   Storage and use, owner decision 5 of 11/10/2026: kept on ViHAT Group's server, read by ViHAT staff to
     *   contact the user back or stop sending. Retention: the SMS choices until the user asks for deletion; the
     *   Zalo name the same, CAPPED by the code at 24 months — `vihat-miniapp` `migrations/0004_yeu_cau_chat_sms.sql`
     *   §4 (`an_danh_yeu_cau_qua_han` clears `ten_hien_thi` together with the note), and
     *   `internal/store/an_danh.go:82-89` clears both on an erasure request. "Until deletion" alone would promise
     *   to keep the name longer than the code does.
     *
     *   ⚠ NOT WRITTEN, either way: whether requests are forwarded elsewhere. `vihat-miniapp` has an OPTIONAL signed
     *   webhook (`internal/webhook/client.go`, env `REQUEST_WEBHOOK_URL`) that posts each new request — phone, name,
     *   note — to a configured receiver. Whether it is on, and who receives, is a deployment fact this file cannot
     *   read; the owner states it before publication.
     */
    ma: "chat-sms",
    tieu_de: "Chat với chuyên viên và nhận ưu đãi qua SMS",
    doan: [
      "Ở màn Liên hệ có nút 'Chat với chuyên viên'. Khi bạn bấm, nếu bạn chưa đăng nhập, ứng dụng đăng nhập cho bạn bằng một lần chạm như mục Đăng nhập nói rõ; rồi xin Zalo cho dùng tên hiển thị Zalo của bạn; rồi gửi tới máy chủ của Tập đoàn ViHAT Group một yêu cầu trò chuyện, kèm tên ấy nếu bạn đồng ý chia sẻ; rồi mở cửa sổ trò chuyện với Official Account của chúng tôi. Máy chủ biết yêu cầu ấy là của ai, và số điện thoại nào, từ phiên đăng nhập — nhờ đó chuyên viên biết ai đang nhắn và gọi lại cho bạn nếu cuộc trò chuyện bị gián đoạn.",
      "Bạn có quyền từ chối. Từ chối chia sẻ số điện thoại thì không có gì được gửi đi, và nút này không mở cửa sổ trò chuyện; bạn vẫn nhắn được ngay bằng nút 'Chat Zalo' trên màn hình — nút ấy không gửi thông tin nào của bạn. Từ chối chia sẻ tên thì yêu cầu vẫn được gửi, chỉ là không có tên. Nếu yêu cầu không gửi được, cửa sổ trò chuyện vẫn mở và màn hình nói rõ chuyên viên chưa nhận được thông tin của bạn.",
      "Cũng trong thẻ 'Kết nối với ViHAT' ở màn Liên hệ có khối 'Nhận ưu đãi qua SMS'. Khi bạn bấm 'Đăng ký nhận', ứng dụng đăng nhập cho bạn nếu cần, rồi gửi tới máy chủ của Tập đoàn ViHAT Group một yêu cầu đăng ký. Từ đó, SỐ ĐIỆN THOẠI ZALO CỦA BẠN — số máy chủ đã lưu khi bạn đăng nhập — ĐƯỢC DÙNG ĐỂ GỬI TIN NHẮN SMS QUẢNG CÁO: ưu đãi và chương trình khuyến mại của Tập đoàn ViHAT Group. Ứng dụng không gửi kèm số ấy trong yêu cầu; máy chủ biết số từ phiên đăng nhập.",
      "MUỐN DỪNG NHẬN, bạn bấm 'Huỷ nhận ưu đãi SMS' ngay trong khối ấy, bất cứ lúc nào; ứng dụng gửi một yêu cầu huỷ theo cùng cách. Ứng dụng không lưu việc bạn đã đăng ký xuống máy: khi bạn đang đăng nhập, nó đọc lại từ danh sách yêu cầu của chính bạn ở máy chủ, và yêu cầu mới nhất — đăng ký hay huỷ — là yêu cầu có hiệu lực.",
      "Các yêu cầu bạn gửi — tư vấn, đề nghị gọi lại, trò chuyện với chuyên viên, đăng ký hay huỷ nhận ưu đãi SMS — được lưu trên máy chủ của Tập đoàn ViHAT Group. Nhân viên của Tập đoàn ViHAT Group đọc chúng để liên hệ lại với bạn, hoặc để ngừng gửi tin nhắn ưu đãi cho bạn.",
      "THỜI GIAN LƯU: các yêu cầu đăng ký hay huỷ nhận ưu đãi SMS được lưu cho tới khi bạn yêu cầu xoá. Tên Zalo đi kèm một yêu cầu trò chuyện cũng được lưu cho tới khi bạn yêu cầu xoá, nhưng không quá 24 tháng kể từ ngày gửi: tới mốc ấy, tên bị xoá khỏi yêu cầu cùng lúc với phần ghi chú. Cách yêu cầu xoá nằm ở mục 'Cách xử lý và thời gian lưu' bên dưới.",
    ],
  },
  {
    /**
     * "TÌM VĂN PHÒNG GẦN TÔI" (owner decision 12/10/2026) — the intro's one use of the location permission.
     *
     *   What each clause is read from:
     *     · when / once — `features/tinh-nang/nearby-offices.ts` `createNearbyFlow`: Zalo is asked only after the
     *       app's own "Cho phép", one `getLocation` per tap;
     *     · where — `features/dang-nhap/current-location.ts` `getCurrentLocation` → `vihat-miniapp`
     *       `POST /api/v1/location`, body `accessToken` + `locationToken` (`hop-dong.ts` `LOCATION_FIELDS`);
     *     · not stored — `vihat-miniapp` `internal/httpapi/vi_tri.go:89-92,157`: nothing written, coordinates and
     *       tokens never logged, not even a success line;
     *     · on the phone only — `locateNearbyOffices` keeps distances, not the coordinates.
     */
    ma: "vi-tri-van-phong",
    tieu_de: "Vị trí khi bạn tìm văn phòng gần nhất",
    doan: [
      "Ở màn Liên hệ có nút 'Tìm văn phòng gần tôi'. Chỉ khi bạn bấm nút ấy, đọc câu giải thích của ứng dụng và bấm 'Cho phép', ứng dụng mới xin Zalo vị trí hiện tại của bạn — MỘT LẦN cho mỗi lần bấm. Zalo không trả toạ độ về máy mà chỉ trả một mã dùng một lần; ứng dụng gửi mã ấy, cùng mã phiên Zalo, tới máy chủ của Tập đoàn ViHAT Group để đổi ra toạ độ, và máy chủ trả toạ độ về máy bạn. MÁY CHỦ KHÔNG LƯU vị trí của bạn — không lưu mã, không lưu toạ độ, không ghi chúng vào nhật ký. Trên máy bạn, toạ độ chỉ dùng để tính khoảng cách tới từng văn phòng và xếp văn phòng gần nhất lên đầu; ứng dụng không giữ lại toạ độ, không lưu xuống máy, và không dùng hay gửi chúng cho việc gì khác. Bấm 'Không', hoặc từ chối trên hộp thoại của Zalo, thì không có gì được gửi đi và danh sách văn phòng giữ thứ tự sẵn có.",
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
      // `COMPANY` + `CONTACT` (`features/tinh-nang/vcard.ts`). `chinh-sach.test.ts` buộc hai bên khớp nhau
      // theo CẢ HAI CHIỀU.
      "Tệp ấy chứa tên công ty, hotline, email và trang web của chúng tôi. Nó KHÔNG chứa bất kỳ thông tin nào của bạn.",
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
     * định 13 đòi đúng cái cửa ấy. Nên các đoạn dưới nói đủ ba điều, thiếu một là hỏng:
     *
     *   1. không có hạn tự động — nói THẲNG;
     *   2. yêu cầu xoá bằng ĐƯỜNG NÀO — hotline và email đã có sẵn trên màn Liên hệ;
     *   3. xoá thì XOÁ CÁI GÌ — số điện thoại, và chữ người dùng tự gõ trên các yêu cầu (`vihat-miniapp`
     *      `internal/store/an_danh.go:82-89`: ghi chú và tên Zalo). Nhật ký đăng nhập ở lại được, và lý do
     *      phải nói ra: nó không chứa số điện thoại.
     */
    ma: "cach-thuc",
    tieu_de: "Cách xử lý và thời gian lưu",
    doan: [
      "Trên máy bạn, dữ liệu chỉ tồn tại trong bộ nhớ tạm của phiên làm việc và mất đi khi bạn rời màn hình hoặc đóng ứng dụng.",
      "Phiếu phiên nhận được sau khi bạn đăng nhập cũng chỉ nằm trong bộ nhớ ấy: ứng dụng không ghi nó xuống máy bạn, nên đóng ứng dụng là nó mất đi và lần sau bạn đăng nhập lại bằng một lần chạm.",
      "Trên máy bạn không có việc lưu trữ, nên không có bản sao lưu nào trên máy bạn chứa dữ liệu của bạn.",
      // ⚠ "BỐN THỨ" THÀNH "NĂM THỨ" NGÀY 22/09/2026: một câu liệt kê đủ rồi chốt bằng "ngoài … ấy, máy chủ
      // KHÔNG lưu gì khác" là một lời khai TRỌN VẸN — thiếu một mục thì cả câu thành sai, không chỉ thiếu.
      "Ở máy chủ, năm thứ được lưu: số điện thoại dùng làm tên đăng nhập của bạn; bản ghi của từng phiên đăng nhập (thời điểm tạo, thời điểm hết hạn, và bản mã hoá một chiều của phiếu phiên); nhật ký đăng nhập; những yêu cầu bạn đã gửi — tư vấn (kèm những ảnh bạn tự chụp hoặc chọn gửi kèm), đề nghị gọi lại, trò chuyện với chuyên viên (kèm tên Zalo nếu bạn đồng ý chia sẻ), đăng ký hay huỷ nhận ưu đãi SMS —, nói ở mục 'Yêu cầu tư vấn và đề nghị gọi lại' và mục 'Chat với chuyên viên và nhận ưu đãi qua SMS' bên trên; và — chỉ khi bạn từng yêu cầu xoá — một dòng bằng chứng của chính lần xoá ấy, nói ở cuối mục này. Mỗi bản ghi định danh còn mang thời điểm nó được tạo và lần gần nhất được cập nhật.",
      "Ngoài năm thứ ấy, máy chủ KHÔNG lưu gì khác của bạn: không tên — trừ tên Zalo đi kèm một yêu cầu trò chuyện với chuyên viên, khi bạn đồng ý chia sẻ —, không email, không vị trí, không thông tin thiết bị, không danh bạ, không ảnh — trừ những ảnh bạn tự gửi kèm một yêu cầu tư vấn, đã bỏ thông tin vị trí chụp. Dịch vụ này cũng không cài công cụ đo hành vi nào và không nhúng bộ công cụ của bên thứ ba nào.",
      // 11/10/2026: the SDK failure report (`zalo-api.ts` `reportSdkFailure`, owner 08/10/2026), declared in the
      // dossier since that day and owed here. Every `trong_chinh_sach` of `CLIENT_ERROR_FIELDS` the intro can send
      // sits VERBATIM in this paragraph (`chinh-sach.test.ts`); `host` is the commune interface's only — the intro
      // has no commune, so it is null and left out of the body. Nothing stored, no IP logged (`vihat-miniapp`
      // `internal/httpapi/client_errors.go:26-27`).
      "KHI MỘT CHỨC NĂNG CỦA ZALO MÀ BẠN VỪA DÙNG BÁO LỖI — ví dụ đăng nhập không thành công —, ứng dụng gửi tới máy chủ của Tập đoàn ViHAT Group đúng những thứ sau, và không gì khác: tên chức năng Zalo vừa gọi không thành công (ví dụ: mã phiên, số điện thoại); mã lỗi do Zalo trả về; câu báo lỗi kỹ thuật do Zalo trả về — không chứa tên hay số điện thoại của bạn; và mã của ứng dụng đang chạy. Máy chủ chỉ ghi chúng thành một dòng nhật ký kỹ thuật để tìm nguyên nhân lỗi — không kèm địa chỉ IP của bạn — và không lưu chúng vào cơ sở dữ liệu.",
      "THỜI HẠN LƯU YÊU CẦU TƯ VẤN: 24 THÁNG kể từ ngày gửi, rồi phần ghi chú bạn tự gõ bị xoá và hàng ấy được đánh dấu là đã ẩn danh. Chi tiết ở mục 'Yêu cầu tư vấn và đề nghị gọi lại' bên trên.",
      "THỜI HẠN LƯU NHẬT KÝ ĐĂNG NHẬP: CHẬM NHẤT 90 NGÀY. Đây là mức trần: chúng tôi dọn nhật ký theo từng lô mỗi tuần chứ không xoá từng dòng đúng vào ngày thứ 90, nên một dòng có thể bị xoá SỚM HƠN — sớm nhất là ngày thứ 83. Không dòng nào sống quá 90 ngày.",
      "Thời hạn ấy áp cho MỌI dòng, kể cả những dòng của một lượt đăng nhập không thành công. Nghĩa là nếu bạn chưa từng đăng nhập thành công, nên không có gì để yêu cầu xoá, thì địa chỉ IP trong những dòng ấy vẫn tự mất đi trong vòng 90 ngày mà bạn không phải làm gì cả.",
      "THỜI GIAN LƯU SỐ ĐIỆN THOẠI: KHÔNG có hạn tự động. Chúng tôi giữ nó chừng nào bạn còn dùng ứng dụng, và giữ tới khi chính bạn yêu cầu xoá — không có mốc nào tự động xoá, và cũng không có mốc nào tự động giữ thêm.",
      "CÁCH YÊU CẦU XOÁ: gọi hotline hoặc gửi email cho chúng tôi theo hai đầu mối ở màn Liên hệ của ứng dụng. Bạn không cần đăng nhập để yêu cầu, và không phải nêu lý do.",
      // CÂU NÀY ĐÃ PHẢI SỬA, VÀ ĐÓ LÀ SỬA MỘT LỜI HỨA KHÔNG GIỮ ĐƯỢC: nhật ký đăng nhập tham chiếu tới bản
      // ghi định danh và là bảng CHỈ GHI THÊM, nên CSDL TỪ CHỐI xoá hàng định danh của bất cứ ai từng đăng
      // nhập thành công (`vihat-miniapp/migrations/0001_init.sql`). Thứ làm được là XOÁ SỐ ĐIỆN THOẠI.
      "KHI XOÁ, CHÚNG TÔI XOÁ SỐ ĐIỆN THOẠI CỦA BẠN — thứ duy nhất trong hệ thống nhận ra bạn là ai. Bản ghi còn lại chỉ là một mã định danh nội bộ không gắn với số nào, và những dòng nhật ký cũ vẫn trỏ vào mã ấy; nhưng từ mã ấy không còn số điện thoại nào để tra về bạn nữa.",
      // 11/10/2026: what the erasure does to the requests, read from `vihat-miniapp` `internal/store/an_danh.go:82-89`
      // — said, because "until you ask for deletion" (section `chat-sms`) is a promise only if deletion reaches them.
      "Cùng lúc đó, chúng tôi xoá phần ghi chú bạn tự gõ và tên Zalo trong mọi yêu cầu bạn đã gửi, cùng mọi ảnh bạn đã gửi kèm yêu cầu tư vấn, và đánh dấu các yêu cầu ấy là đã ẩn danh; và mọi phiên đăng nhập còn hiệu lực của bạn bị thu hồi: nếu bạn đang đăng nhập trên một máy nào đó, máy ấy sẽ bị đăng xuất.",
      // KHAI RA MỘT BẢNG NỮA, VÀ ĐÂY LÀ CHỖ DỄ IM LẶNG NHẤT CỦA CẢ VĂN BẢN: nó là dòng dữ liệu DUY NHẤT về một
      // người còn ở lại SAU KHI họ đã yêu cầu xoá. `vihat-miniapp/migrations/0002_…sql`, bảng `nhat_ky_an_danh`.
      "CHÚNG TÔI GIỮ LẠI MỘT DÒNG BẰNG CHỨNG CHO CHÍNH VIỆC XOÁ ẤY, và nói ra ở đây để bạn không phát hiện nó qua một đường khác. Dòng ấy gồm: mã định danh nội bộ của bạn, yêu cầu tới qua hotline hay qua email, tên người tiếp nhận và thực hiện, một ghi chú ngắn (số phiếu hoặc tiêu đề email), và thời điểm. Nó KHÔNG chứa số điện thoại của bạn — lúc nó được ghi thì số đã bị xoá khỏi hệ thống rồi.",
      "Dòng bằng chứng ấy được giữ VÔ THỜI HẠN, nằm ngoài quy tắc 90 ngày, và lý do nằm ở chính công dụng của nó: nó là thứ chứng minh chúng tôi ĐÃ làm điều đã hứa với bạn, và trả lời được câu 'ai cho phép xoá dữ liệu của người này'. Một bằng chứng tự huỷ sau một thời gian thì không còn là bằng chứng.",
      "NẾU BẠN CHƯA TỪNG ĐĂNG NHẬP THÀNH CÔNG: chúng tôi không có bản ghi định danh nào của bạn để mà xoá — chỉ có những dòng nhật ký ghi lại thời điểm, địa chỉ IP và việc lượt ấy đã hỏng vì lý do gì. Chúng tôi không xoá riêng những dòng ấy theo yêu cầu được, vì nhật ký chỉ ghi thêm, và cũng không có cách nào biết dòng nào là của bạn: trong hệ thống không có gì khác của bạn để đối chiếu. Nhưng bạn không cần yêu cầu — chúng tự hết hạn và bị xoá sau 90 ngày, như mọi dòng nhật ký khác.",
    ],
  },
  {
    ma: "ben-thu-ba",
    tieu_de: "Chuyển dữ liệu cho bên thứ ba",
    doan: [
      // 11/10/2026: the receivers of THIS app only. No "và không nơi nào khác": `vihat-miniapp` can post each new
      // request to a configured webhook receiver (see the `chat-sms` block) — the owner states whether that is
      // on, and who receives, before publication.
      "Ứng dụng gửi dữ liệu của bạn tới máy chủ của Tập đoàn ViHAT Group: khi bạn đăng nhập, gửi một yêu cầu tư vấn, bấm 'Chat với chuyên viên', đăng ký hay huỷ nhận ưu đãi SMS, hoặc bấm 'Tìm văn phòng gần tôi' rồi cho phép lấy vị trí, và khi một chức năng của Zalo bạn vừa dùng báo lỗi. Từng bước gửi được nói ở mục Đăng nhập, mục 'Yêu cầu tư vấn và đề nghị gọi lại', mục 'Chat với chuyên viên và nhận ưu đãi qua SMS', mục 'Vị trí khi bạn tìm văn phòng gần nhất' và mục 'Cách xử lý và thời gian lưu'.",
      // ⚠ CÂU NÀY KHÔNG GÕ TAY — 21/09/2026. Dựng từ `INTRO_DESTINATIONS` (`content/dich-ra-ngoai.ts`): cách duy
      // nhất con số và danh sách không lệch nhau. Đếm theo LOẠI ĐÍCH ĐẾN, không theo số nút.
      cauKhaiDichRaNgoai(INTRO_DESTINATIONS),
      // 11/10/2026 the sentence ended "ứng dụng không hỏi vị trí của bạn" — false since 12/10/2026, when "Tìm văn
      // phòng gần tôi" asks it again. What stays true: the map link carries the office's address and nothing else.
      "Nút chỉ đường chỉ mang theo ĐỊA CHỈ VĂN PHÒNG của chúng tôi — không mang theo vị trí của bạn, kể cả khi bạn vừa cho phép lấy vị trí để tìm văn phòng gần nhất.",
      "Zalo là nền tảng ứng dụng chạy trên đó. Việc bạn dùng Zalo chịu sự điều chỉnh của chính sách quyền riêng tư của Zalo, nằm ngoài phạm vi văn bản này.",
    ],
  },
  {
    ma: "quyen-cua-ban",
    tieu_de: "Quyền của bạn theo Nghị định 13/2023/NĐ-CP",
    doan: [
      "Bạn có quyền được biết, quyền đồng ý và rút lại đồng ý, quyền truy cập, chỉnh sửa, xoá và hạn chế việc xử lý dữ liệu cá nhân của mình, quyền phản đối và quyền khiếu nại.",
      "Trong ứng dụng này, việc thực hiện các quyền ấy rất đơn giản: bạn rút lại đồng ý bằng cách tắt quyền tương ứng trong phần cài đặt của Zalo.",
      "Trên máy bạn, ứng dụng không lưu dữ liệu nào của bạn. Với những gì đã gửi tới máy chủ của Tập đoàn ViHAT Group — ở bước đăng nhập, và ở những yêu cầu bạn đã gửi, kể cả tên Zalo đi kèm yêu cầu trò chuyện — bạn thực hiện các quyền trên bằng cách liên hệ theo các đầu mối ở màn Liên hệ; riêng đồng ý nhận ưu đãi qua SMS, bạn rút lại bằng nút 'Huỷ nhận ưu đãi SMS' ở màn Liên hệ; chúng tôi trả lời trong thời hạn Nghị định 13/2023/NĐ-CP quy định.",
    ],
  },
  {
    ma: "rui-ro",
    tieu_de: "Rủi ro có thể xảy ra",
    doan: [
      // ⚠ CÂU NÀY ĐÃ PHẢI SỬA 22/09/2026, VÀ ĐÓ LÀ MỘT LẦN BỚT MỘT LỜI TRẤN AN: ô ghi chú là văn bản tự do, nên
      // nó phải có tên trong mục rủi ro.
      "Ứng dụng không lưu dữ liệu nào của bạn trên máy; thứ nó gửi đi là hai mã đăng nhập dùng một lần — số điện thoại của bạn không nằm trong hai mã ấy — và, nếu bạn tự gửi một yêu cầu tư vấn, những gì bạn đã chọn cùng phần ghi chú bạn tự gõ và những ảnh bạn tự chụp hoặc chọn gửi kèm; nếu bạn bấm 'Chat với chuyên viên' và đồng ý, tên Zalo của bạn; nếu bạn bấm 'Tìm văn phòng gần tôi' và cho phép, một mã vị trí dùng một lần, để đổi ra toạ độ mà máy chủ không lưu.",
      "RỦI RO ĐÁNG KỂ NHẤT NẰM Ở CHÍNH Ô GHI CHÚ ẤY: nó là văn bản tự do, nên nó chứa đúng những gì bạn viết vào. Bạn hãy cân nhắc trước khi ghi vào đó số căn cước, thông tin tài khoản ngân hàng, hay dữ liệu cá nhân của một người khác — chúng tôi không có cách nào biết trước để ngăn.",
      "Rủi ro còn lại nằm ở màn hình: nội dung hiển thị sau khi bạn dùng một tính năng có thể bị người đứng cạnh nhìn thấy. Điều này đáng lưu ý nhất khi bạn vừa quét một tấm danh thiếp — thứ hiện ra là dữ liệu cá nhân của người đã đưa nó cho bạn, và bạn là người đang giữ nó.",
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
