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
 * ⚠ CHỖ TRỐNG VỀ THỜI GIAN LƯU ĐÃ ĐƯỢC LẤP — 20/09/2026, bản `1.4`. Khách chốt: **không có hạn
 * tự động; số điện thoại được lưu tới khi chính người dùng yêu cầu xoá.** Hằng
 * `THOI_GIAN_LUU_CHUA_CHOT` vì thế đã biến mất, và ca kiểm canh chỗ trống ấy được thay bằng ca
 * canh chính CAM KẾT mới (`chinh-sach.test.ts`).
 *
 *   "Giữ tới khi bạn yêu cầu xoá" KHÔNG phải một cách né con số — nó là một cam kết, và một cam
 *   kết thì phải kèm CỬA THỰC HIỆN. Nên mục ấy nói đủ ba điều, thiếu một là hứa suông: không có
 *   hạn tự động · yêu cầu xoá bằng đường nào (hotline và email trên màn Liên hệ) · xoá thì xoá
 *   cái gì (số điện thoại và bản ghi định danh; nhật ký đăng nhập ở lại được vì nó không chứa
 *   số điện thoại).
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
 *
 * LÊN `1.2` VÌ BỀ MẶT QUYỀN RIÊNG TƯ MỞ RỘNG THẬT SỰ — bốn thứ mới, mỗi thứ một dòng phải khai:
 *
 *   | Mới | Ứng dụng làm gì | Khai ở mục |
 *   |---|---|---|
 *   | **Máy ảnh** | Hỏi quyền để chụp lại một tấm danh thiếp giấy | `cac-quyen` |
 *   | **Thư viện ảnh** | Mở cửa sổ chọn ảnh của Zalo; ảnh **không rời khỏi máy** | `du-lieu` · `cac-quyen` |
 *   | **Thông tin mạng** | Chỉ **kiểu** kết nối — không IP, không tên mạng | `cac-quyen` |
 *   | **Ghi một tệp xuống máy** | Tệp danh thiếp **của chúng tôi**, không phải dữ liệu của người dùng | `ghi-tep` |
 *
 * Ba thứ đầu là dữ liệu ĐI VÀO ứng dụng; thứ tư là thứ ứng dụng VIẾT RA, và đó là một loại hành
 * vi mà bản 1.1 hoàn toàn không có. Giấu nó đi vì "chỉ là tệp của chính mình" là đúng thứ Nghị
 * định 13 buộc phải nói ra: người dùng có quyền biết ứng dụng ghi gì lên thiết bị của họ.
 *
 * LÊN `1.3` VÌ MỤC ĐÍCH CỦA MỘT QUYỀN ĐÃ ĐỔI — và đổi mục đích là thay đổi nặng nhất một chính
 * sách quyền riêng tư có thể mang, nặng hơn cả thêm một quyền mới:
 *
 *   | Bản 1.2 | Bản 1.3 |
 *   |---|---|
 *   | `getPhoneNumber` — "để đội kinh doanh gọi lại tư vấn" | `getPhoneNumber` — **để đăng nhập / định danh**, và để gửi **thông báo ZNS** |
 *   | không nhắc `getAccessToken` | khai `getAccessToken`: mã cho biết bạn là người dùng Zalo nào đối với riêng ứng dụng này |
 *   | không có hành vi gửi đi nào | bản dựng CÓ bước máy chủ gửi hai mã ấy đi khi bạn bấm đăng nhập |
 *
 *   Một người đã đồng ý cho "gọi lại tư vấn" KHÔNG phải đã đồng ý cho "định danh và gửi thông
 *   báo". Cùng một quyền, cùng một nút, nhưng là hai sự đồng ý khác nhau — nên phải có một số
 *   phiên bản mới để chỉ đúng văn bản họ đã đọc lúc bấm.
 *
 * ⚠ VÀ MỘT CÂU CỦA BẢN 1.2 ĐÃ THÀNH SAI: *"không có ô đăng nhập"*. Nay ứng dụng CÓ đăng nhập —
 * bằng một lần chạm, không ô nhập nào. Câu ấy được thay bằng một câu nói đúng cả hai vế: không
 * có ô để gõ, nhưng có việc đăng nhập. Một câu đúng ở bản trước mà không ai sửa khi hành vi đổi
 * là loại lỗi nặng nhất một văn bản như thế này mắc phải, và là loại không có gì đỏ lên.
 *
 * LÊN `1.4` VÌ HAI ĐIỀU KHOẢN ĐỔI NỘI DUNG — cùng ngày với 1.3, và vẫn phải là một số mới:
 *
 *   | Mới ở 1.4 | Nội dung |
 *   |---|---|
 *   | **Thời gian lưu** | Bản 1.3 **không nêu** thời hạn nào (chưa ai chốt). Nay chốt: **không có hạn tự động**, giữ tới khi người dùng yêu cầu xoá — kèm cửa thực hiện và phạm vi xoá |
 *   | **Nhật ký đăng nhập** | Máy chủ ghi **thời điểm và địa chỉ IP** mỗi lần đăng nhập để phát hiện lạm dụng. Bản 1.3 hoàn toàn không khai điều này |
 *
 *   Cả hai đều là HÀNH VI XỬ LÝ DỮ LIỆU, không phải câu chữ. Một người đọc bản 1.3 rồi bấm đồng
 *   ý **không** biết địa chỉ IP của mình được ghi lại — nên bản họ đã đọc và bản hôm nay phải
 *   mang hai số khác nhau, kể cả khi cách nhau vài giờ.
 *
 *   ⚠ Địa chỉ IP là thứ MÁY CHỦ THẤY TỪ CHÍNH LỜI GỌI, không phải thứ ứng dụng gửi lên. Câu mở
 *   đầu *"gửi đi đúng MỘT việc"* vì thế vẫn đúng từng chữ — nhưng im lặng về việc máy chủ ghi
 *   lại nó thì là giấu một hành vi mà hệ thống CÓ, đúng thứ Nghị định 13 nhắm tới.
 */
export const PHIEN_BAN_CHINH_SACH = "1.4";
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
      "VihatSoftware — đơn vị thành viên của Tập đoàn ViHAT Group — là bên phát hành ứng dụng này và là bên chịu trách nhiệm về chính sách này.",
      "Địa chỉ trụ sở chính và các đầu mối liên hệ được ghi ở màn Liên hệ của ứng dụng.",
    ],
  },
  {
    ma: "du-lieu",
    tieu_de: "Dữ liệu ứng dụng xử lý",
    doan: [
      // CÂU NÀY ĐÃ ĐƯỢC SỬA Ở PHIÊN BẢN 1.3. Bản 1.2 viết "không có ô đăng nhập"; nay ứng dụng
      // CÓ đăng nhập, chỉ là không có ô nào để gõ. Giữ nguyên câu cũ là mô tả sai bản dựng.
      "Ứng dụng không yêu cầu bạn nhập bất kỳ thông tin nào: không có biểu mẫu, không có ô nhập số điện thoại, không có mã sáu số nào phải gõ. Việc đăng nhập là một lần chạm, bằng chính số Zalo bạn đang dùng — mục Đăng nhập bên dưới nói rõ.",
      "Ứng dụng không đọc danh bạ, không đọc tin nhắn và không theo dõi hành vi sử dụng của bạn.",
      // CÂU NÀY ĐÃ ĐƯỢC SỬA Ở PHIÊN BẢN 1.2, VÀ VIỆC SỬA NÓ LÀ BẮT BUỘC. Bản 1.1 viết "không
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
     *   thấy câu trả lời bằng một dòng tiêu đề, không phải bằng cách đọc hết. Và từ bản 1.3,
     *   `getPhoneNumber` không còn là "để gọi lại tư vấn" mà là ĐỊNH DANH — thứ quyết định
     *   người dùng thấy gì khi mở lại ứng dụng.
     */
    ma: "dang-nhap",
    tieu_de: "Đăng nhập bằng số điện thoại Zalo",
    doan: [
      "Ứng dụng xin số điện thoại Zalo của bạn để bạn đăng nhập bằng một lần chạm, và để gửi thông báo ZNS tới đúng số ấy khi bạn cần được báo kết quả. Không có ô nhập số điện thoại, và không có mã sáu số nào phải gõ.",
      "Zalo không trả số điện thoại của bạn về máy: ứng dụng chỉ nhận hai mã dùng được một lần, hết hạn sau hai phút. Số điện thoại của bạn không nằm trong hai mã ấy.",
      "Khi bạn bấm đăng nhập, ứng dụng gửi hai mã ấy tới máy chủ của VihatSoftware — bên phát hành ứng dụng này, không phải một bên thứ ba. Máy chủ đổi mã tại Zalo bằng một khoá bí mật mà ứng dụng trên máy bạn không có và không được có.",
      "MÁY CHỦ LƯU SỐ ĐIỆN THOẠI CỦA BẠN, và lưu để làm đúng hai việc: làm tên đăng nhập cho những lần bạn mở lại ứng dụng, và làm nơi nhận thông báo ZNS. Ngoài số điện thoại, ứng dụng không gửi thông tin nào khác của bạn đi.",
      "Máy chủ trả về một phiếu phiên. Ứng dụng giữ phiếu ấy trong bộ nhớ, không ghi xuống máy bạn và không hiện nó ra màn hình; bạn đóng ứng dụng là phiếu mất đi và lần sau đăng nhập lại bằng một lần chạm.",
      // KHAI Ở BẢN 1.4. Địa chỉ IP là thứ máy chủ THẤY từ chính lời gọi, không phải thứ ứng dụng
      // gửi lên — nhưng im lặng về việc nó được GHI LẠI là giấu một hành vi mà hệ thống có.
      "Mỗi lần bạn đăng nhập, máy chủ ghi lại thời điểm và địa chỉ IP của lần đăng nhập đó, để phát hiện việc lạm dụng (ví dụ một máy thử đăng nhập hàng loạt). Nhật ký này KHÔNG chứa số điện thoại của bạn.",
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
      "Ứng dụng ghi đúng MỘT loại tệp xuống máy bạn, và chỉ khi chính bạn bấm nút tải: tệp danh thiếp của VihatSoftware, ở định dạng vCard (.vcf).",
      "Tệp ấy chứa tên công ty, hotline, email và trang web của chúng tôi. Nó KHÔNG chứa bất kỳ thông tin nào của bạn, vì ứng dụng không có thông tin nào của bạn để đưa vào.",
      "Nội dung tệp được dựng ngay trên máy bạn và đưa thẳng cho Zalo ghi hộ. Không có một lời gọi mạng nào, không có máy chủ nào tham gia, và Zalo là bên quyết định tệp nằm ở thư mục nào.",
      "Ngoài tệp ấy, ứng dụng không đọc, không sửa, không xoá và không tạo bất kỳ tệp nào khác trên máy bạn.",
    ],
  },
  {
    /**
     * THỜI GIAN LƯU — ĐÃ CHỐT Ở BẢN 1.4: **không có hạn tự động, giữ tới khi người dùng yêu cầu
     * xoá.** Bản 1.3 để trống chỗ này vì chưa ai chốt, và chỗ trống ấy có một ca kiểm canh.
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
      "Ở máy chủ, hai thứ được lưu: số điện thoại dùng làm tên đăng nhập của bạn, và nhật ký đăng nhập gồm thời điểm cùng địa chỉ IP của mỗi lần đăng nhập.",
      "THỜI GIAN LƯU: số điện thoại của bạn KHÔNG có hạn tự động. Chúng tôi giữ nó chừng nào bạn còn dùng ứng dụng, và giữ tới khi chính bạn yêu cầu xoá — không có mốc nào tự động xoá, và cũng không có mốc nào tự động giữ thêm.",
      "CÁCH YÊU CẦU XOÁ: gọi hotline hoặc gửi email cho chúng tôi theo hai đầu mối ở màn Liên hệ của ứng dụng. Bạn không cần đăng nhập để yêu cầu, và không phải nêu lý do.",
      "KHI XOÁ, CHÚNG TÔI XOÁ: số điện thoại của bạn và bản ghi định danh gắn với nó — nghĩa là bạn trở lại như chưa từng đăng nhập. Nhật ký đăng nhập được giữ lại, và lý do là nó KHÔNG chứa số điện thoại của bạn: sau khi xoá, không còn đường nào nối những dòng nhật ký ấy về với bạn.",
    ],
  },
  {
    ma: "ben-thu-ba",
    tieu_de: "Chuyển dữ liệu cho bên thứ ba",
    doan: [
      // SỬA Ở BẢN 1.3: câu cũ ("nó không có đường gửi dữ liệu đi đâu cả") chỉ còn đúng ở bản
      // nộp. Vế BÊN THỨ BA thì đúng ở cả hai bản và là vế mục này nói tới — máy chủ nhận hai mã
      // đăng nhập là máy chủ của chính bên phát hành ứng dụng, không phải một bên thứ ba.
      "Ứng dụng không gửi dữ liệu của bạn cho bất kỳ bên thứ ba nào, trong nước hay ngoài nước. Nơi duy nhất nhận gì đó từ ứng dụng là máy chủ của chính VihatSoftware, ở bước đăng nhập nói tại mục Đăng nhập bên trên.",
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
      // SỬA Ở BẢN 1.3, cùng lý do với mục "Chuyển dữ liệu cho bên thứ ba": câu cũ ("không có
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
