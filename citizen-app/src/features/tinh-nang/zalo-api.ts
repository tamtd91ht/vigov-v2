/**
 * LỚP DUY NHẤT CHẠM VÀO `zmp-sdk` — và nó cố tình nhỏ đúng bằng những lời gọi ba tính năng cần.
 *
 * Tệp này là chỗ duy nhất trong cả kho được phép nhắc tên mô-đun `zmp-sdk` và tên các lời gọi
 * nền tảng — `getPhoneNumber` · `getLocation` · `scanQRCode` · `getAccessToken` và sáu tên còn
 * lại. `phase1-collects-nothing.test.ts` cấm chúng ở MỌI tệp khác — lệnh cấm không bị xoá, nó được thu hẹp phạm vi, và có một ca chứng minh rằng
 * ngoài `src/features/tinh-nang/` thì nó vẫn bắt.
 *
 * ⚠ NHẬP ĐỘNG, TRONG HÀM, BỌC TRY/CATCH — BA ĐIỀU KIỆN, KHÔNG PHẢI KHẨU VỊ:
 *
 *   `zmp-sdk` chạm `window` NGAY LÚC NẠP MÔ-ĐUN. Một `import` tĩnh ở đầu tệp làm sập mọi test
 *   chạy dưới Node (không có `window`) trước khi một phép kiểm nào kịp chạy, và làm sập ngay cả
 *   những test không liên quan gì tới ba tính năng này. Nhập động bên trong hàm thì mô-đun chỉ
 *   được nạp khi người dùng đã bấm nút.
 *
 *   Ngoài Zalo, lời nhập ấy hỏng. Đó KHÔNG phải một sự cố cần giấu: hàm trả về `ngoai-zalo` và
 *   màn hình nói ra bằng tiếng Việt. Một `catch {}` im lặng ở đây là một màn hình trắng trên máy
 *   người duyệt, và một màn hình trắng là câu trả lời tệ nhất cho một vòng xét duyệt.
 *
 * ⚠ ĐỌC ĐÚNG `token`, KHÔNG ĐỌC `number` / `latitude` / `longitude`.
 *
 *   Đo từ `node_modules/zmp-sdk/index.d.ts` (2.53.0), không phải từ tài liệu web:
 *
 *     GetPhoneNumberReturns = { number?: @deprecated; token?: string }
 *     GetLocationReturns    = { latitude?/longitude?/timestamp?/provider?: @deprecated; token?: string }
 *     ScanQRCodeReturns     = { content: string }
 *
 *   Nền tảng vừa bỏ đường đưa số điện thoại và toạ độ về thiết bị. Đọc lại mấy trường ấy là tự
 *   rước dữ liệu cá nhân về máy đúng lúc nó đã hết cần thiết — và dữ liệu cá nhân nằm trên thiết
 *   bị là thứ luật 3 nói tới, không phải một chi tiết kỹ thuật.
 *
 *   HỆ QUẢ THẲNG THẮN, VÀ MÃ NÀY KHÔNG GIẢ VỜ VƯỢT QUA NÓ: chỉ có token nghĩa là **không thể xếp
 *   văn phòng theo khoảng cách ngay trên máy**. Đổi token thành toạ độ là một bước ở máy chủ, có
 *   app secret. Nên ở đây không có một dòng nào tính khoảng cách, và màn hình nói ra điều đó
 *   bằng tiếng Việt thay vì vẽ một danh sách đã "sắp xếp" mà thật ra không sắp xếp gì.
 *
 * ⚠ KHÔNG `console.log`, KHÔNG `fetch`, KHÔNG CHỖ LƯU NÀO. Nội dung mã QR có thể là bất cứ thứ
 * gì, kể cả dữ liệu cá nhân của NGƯỜI KHÁC (một tấm danh thiếp là dữ liệu cá nhân của chủ nhân
 * nó). Hiện lên màn hình rồi thôi; ghi nó ra log là đưa nó vào một nơi không ai gỡ lại được.
 */

/* ==============================================================================================
   KHAI BÁO MỤC ĐÍCH — MỖI LỜI GỌI NỀN TẢNG KHAI TẠI CHỖ NÓ ĐƯỢC VIẾT RA.

   VÌ SAO KHAI BÁO NẰM NGAY TRONG TỆP GỌI, KHÔNG NẰM Ở MÀN "QUẢN LÝ QUYỀN":

     Quyền của Zalo cấp theo **App ID**, không theo màn hình và không theo nửa ứng dụng. Nghĩa là
     ngày kênh công dân (nửa nhà nước) có tệp đầu tiên, nó **thừa hưởng nguyên vẹn** mọi quyền mà
     nửa thương mại đã xin được — không ai phải cấp lại, và không có gì chặn lại. Một danh sách
     quyền chép tay ở màn hình thì không mô tả được điều đó: nó mô tả cái người viết danh sách
     NHỚ, và nó đứng yên trong lúc mã nguồn đi tiếp.

     Nên màn "Quản lý quyền" ĐỌC TỪ BẢNG NÀY. Thêm một lời gọi mà quên khai là một ca đỏ
     (`ranh-gioi-hai-nua.test.ts` đối chiếu bảng này với chính mã nguồn của tệp này), chứ không
     phải một dòng thiếu trên một màn hình không ai soát.

   ⚠ CỘT `nua` KHÔNG PHẢI MỘT RÀO CHẮN. Nó là một lời KHAI: "hôm nay nửa nào cần lời gọi này".
     Rào chắn thật nằm ở ranh giới nhập mô-đun — nửa thương mại không nhập được tệp của nửa nhà
     nước và ngược lại. Cột này tồn tại để việc "nửa nhà nước thừa hưởng quyền của nửa thương mại"
     là thứ NHÌN THẤY ĐƯỢC trên màn hình, thay vì một hệ quả nền tảng không ai nói ra.
   ============================================================================================== */

/**
 * Hai nửa của MỘT Mini App, cộng một giá trị cho lời gọi cả hai nửa đều dùng.
 *
 * `nha-nuoc` chưa có lời gọi nào — kênh công dân chưa có tệp nào (`src/cong-dan/`). Giá trị vẫn
 * được khai từ bây giờ: bảng dưới là chỗ lời gọi đầu tiên của nửa ấy phải khai mình, và một kiểu
 * chưa có giá trị nào là một kiểu người sau sẽ thêm sai.
 */
export type NuaUngDung = "thuong-mai" | "nha-nuoc" | "ca-hai";

export type KhaiBaoLoiGoi = {
  /** Tên API của nền tảng, đúng chữ `zmp-sdk` khai. Đối chiếu ngược với mã nguồn tệp này. */
  api: string;
  /** Nửa nào của ứng dụng đang cần lời gọi này, HÔM NAY. Xem cảnh báo ở khối trên. */
  nua: NuaUngDung;
  /** Màn hình người dùng đang đứng khi lời gọi chạy. Nhãn tab, đúng chữ trên thanh tab. */
  man: string;
  /** Tính năng cụ thể trong màn ấy. */
  tinh_nang: string;
  /** Để làm gì — viết cho người dùng đọc, không phải cho lập trình viên. */
  de_lam_gi: string;
  /**
   * Zalo có hỏi người dùng trước khi lời gọi này chạy không.
   *
   * KHÔNG PHẢI CÙNG MỘT CÂU VỚI "có phải xin quyền ở Developer Console không". `getAccessToken`
   * cần quyền nhưng từ SDK 2.35.0 KHÔNG hỏi người dùng (`index.d.ts` dòng 3009), và nói ngược
   * lại trên một màn hình giải thích quyền là nói sai với đúng người đang cần biết.
   */
  hoi_nguoi_dung: boolean;
  /**
   * Thứ rời khỏi máy vì lời gọi này. Chuỗi rỗng nghĩa là KHÔNG CÓ GÌ rời khỏi máy.
   *
   * Đây là cột đắt nhất của bảng và là cột người duyệt đọc kỹ nhất: nó phải khớp với chính sách
   * quyền riêng tư, và `ranh-gioi-hai-nua.test.ts` buộc mỗi lời gọi có khai cột này.
   */
  roi_khoi_may: string;
  /**
   * How the COMMUNE'S OWN APP uses a call both halves use — required when `nua` is "ca-hai"
   * (`ket-xuat-ho-so.test.ts` §7). The commune app is a separate App ID with its own Zalo submission, and its
   * dossier (`tmp/xin-quyen-zalo/README.md`) must describe what THAT app does. The shared sentences above name
   * screens the commune app does not have (Liên hệ, Tư vấn và báo giá) and a route it no longer takes (ViHAT's
   * server for the login, ADR 0066); a reviewer who reads them looks for a button that is not there.
   */
  commune_app?: { man: string; tinh_nang: string; de_lam_gi: string; roi_khoi_may: string };
};

/**
 * MƯỜI HAI LỜI GỌI, MỘT BẢNG. Thứ tự theo màn hình, để màn "Quản lý quyền" đọc xuôi.
 *
 * Chín quyền phải xin ở Developer Console; `getAccessToken` là lời gọi thứ mười cần quyền nhưng
 * không hỏi người dùng; `openPhone` và `openWebview` là `@zaloOnly` và không nằm trong số quyền
 * phải xin — chúng vẫn ở đây, vì màn "Quản lý quyền" trả lời câu "app này gọi những gì của nền
 * tảng", và một lời gọi bị bỏ khỏi bảng vì "nó không phải quyền" là đúng cái lỗ hổng khai báo mà
 * bảng này tồn tại để đóng.
 */
export const KHAI_BAO_LOI_GOI: readonly KhaiBaoLoiGoi[] = [
  {
    api: "getNetworkType",
    nua: "thuong-mai",
    man: "Giải pháp",
    tinh_nang: "Kiểm tra đường truyền",
    de_lam_gi:
      "Đọc kiểu kết nối hiện tại (Wi-Fi hay di động) để nói trước chất lượng một cuộc gọi tổng đài trên đường mạng bạn đang dùng.",
    hoi_nguoi_dung: false,
    roi_khoi_may: "",
  },
  {
    api: "scanQRCode",
    nua: "thuong-mai",
    man: "Danh thiếp",
    tinh_nang: "Quét danh thiếp",
    de_lam_gi: "Mở máy ảnh để đọc mã QR trên một tấm danh thiếp và hiện nội dung lên màn hình.",
    hoi_nguoi_dung: true,
    roi_khoi_may: "",
  },
  {
    api: "openPhone",
    nua: "thuong-mai",
    man: "Danh thiếp",
    tinh_nang: "Quét danh thiếp",
    de_lam_gi: "Chuyển số điện thoại vừa quét sang màn hình gọi của điện thoại.",
    hoi_nguoi_dung: false,
    roi_khoi_may: "",
  },
  {
    // ⚠ DÒNG NÀY RỘNG RA NGÀY 21/09/2026, VÀ VIỆC SỬA NÓ LÀ BẮT BUỘC, KHÔNG PHẢI LỊCH SỰ: nút
    // Chat nổi có mặt trên MỌI màn, và khối "Tin ViHAT" trên màn chủ mở bài viết bằng cùng lời
    // gọi này. Một dòng khai kể ít hơn thứ app thật sự làm là một màn "Quản lý quyền" nói thiếu —
    // đúng lỗ hổng khai báo mà bảng này tồn tại để đóng.
    //
    // DANH SÁCH ĐÍCH ĐẾN KHÔNG NẰM Ở ĐÂY mà ở `content/dich-ra-ngoai.ts`, vì chính sách quyền
    // riêng tư đếm từ đó. Hai danh sách cho một sự thật thì một trong hai sẽ cũ.
    //
    // `nua` BECAME "ca-hai" ON 01/10/2026: the "Xem video" button of a news item in the commune's own app
    // (`cong-dan/man/TinTucAppXa.tsx`) opens the commune's posted link with this same call. The state half
    // does not import it — `App.tsx` injects `moRaNgoai("video", …)` — but a declaration by IMPORT GRAPH
    // instead of by WHO USES THE CALL would be the under-declaration this table exists to close.
    // 01/10/2026 (ADR 0067 §1, §5): also a link inside a commune article's body and a home banner pointing at an
    // https page — commune app only, each after the "Bạn sắp rời ứng dụng để mở <host>" question (`leave-app.tsx`).
    api: "openWebview",
    nua: "ca-hai",
    man: "Mọi màn · Tin tức của xã và Trang chủ (ứng dụng của xã)",
    tinh_nang:
      "Chat với Official Account · Quét danh thiếp · Tìm văn phòng · Tin ViHAT · Website · Xem video của xã · Liên kết trong bài và ảnh trang chủ của xã",
    de_lam_gi:
      "Mở một trang bên ngoài ngay trong Zalo, và chỉ khi chính bạn bấm: cửa sổ trò chuyện với Official Account của chúng tôi, bản đồ chỉ đường tới một văn phòng, trang web ghi trên mã QR bạn vừa quét, một bài trên trang tin của chúng tôi, trang web chính thức của chúng tôi, hoặc — trong ứng dụng của xã — video mà xã đăng kèm một tin, đúng địa chỉ xã đã đăng, và trang web mà xã gắn vào một bài viết hay một ảnh trên trang chủ, sau khi bạn xác nhận rời ứng dụng.",
    hoi_nguoi_dung: false,
    roi_khoi_may: "Địa chỉ trang được mở đi tới trình duyệt trong Zalo.",
    commune_app: {
      man: "Tin tức – Sự kiện · Video · Trang chủ",
      tinh_nang: "Xem video của xã · Liên kết trong bài và ảnh trang chủ của xã",
      de_lam_gi:
        "Mở video mà xã đăng kèm một tin khi chính bạn bấm “Xem video”, và mở trang web mà xã gắn vào một bài viết hay một ảnh trên trang chủ khi chính bạn bấm vào đó rồi chọn “Mở trang” ở câu hỏi “Bạn sắp rời ứng dụng để mở …”. Trang mở ngay trong Zalo; ứng dụng mở đúng địa chỉ xã đã đăng.",
      roi_khoi_may: "Địa chỉ trang xã đã đăng đi tới trình duyệt trong Zalo.",
    },
  },
  {
    api: "keepScreen",
    nua: "thuong-mai",
    man: "Danh thiếp",
    tinh_nang: "Danh thiếp của chúng tôi",
    de_lam_gi:
      "Giữ màn hình sáng trong lúc bạn chìa mã QR danh thiếp ra cho người khác quét. Tắt ngay khi bạn rời màn hình đó.",
    hoi_nguoi_dung: false,
    roi_khoi_may: "",
  },
  {
    api: "downloadFile",
    nua: "thuong-mai",
    man: "Danh thiếp",
    tinh_nang: "Danh thiếp của chúng tôi",
    de_lam_gi:
      "Lưu danh thiếp của ViHAT Group xuống máy bạn dưới dạng tệp .vcf, để thêm thẳng vào danh bạ.",
    hoi_nguoi_dung: true,
    roi_khoi_may: "",
  },
  {
    api: "requestCameraPermission",
    nua: "thuong-mai",
    man: "Danh thiếp",
    tinh_nang: "Số hoá thiếp giấy",
    de_lam_gi: "Hỏi bạn có cho phép dùng máy ảnh hay không, trước khi mở cửa sổ chọn ảnh.",
    hoi_nguoi_dung: true,
    roi_khoi_may: "",
  },
  {
    api: "openMediaPicker",
    nua: "thuong-mai",
    man: "Danh thiếp",
    tinh_nang: "Số hoá thiếp giấy",
    de_lam_gi:
      "Mở cửa sổ chọn ảnh để bạn chụp hoặc chọn một tấm thiếp giấy. Ảnh chỉ hiện lên màn hình này.",
    hoi_nguoi_dung: true,
    roi_khoi_may: "",
  },
  {
    api: "vibrate",
    nua: "thuong-mai",
    man: "Giải pháp · Danh thiếp",
    tinh_nang: "Phản hồi khi một việc chạy xong",
    de_lam_gi: "Rung một nhịp ngắn để báo việc bạn vừa bấm đã xong.",
    hoi_nguoi_dung: false,
    roi_khoi_may: "",
  },
  {
    /**
     * ⚠ MỤC ĐÍCH CỦA HAI DÒNG ĐĂNG NHẬP ĐÃ RỘNG RA — 22/09/2026, giai đoạn B. ĐÂY LÀ LẦN THỨ HAI.
     *
     *   Lần thứ nhất (20/09): "gọi lại tư vấn" → "định danh + nhận ZNS" (ADR 0020).
     *   Lần thứ hai (22/09): phiên đăng nhập nay còn là CỬA VÀO hai màn mới — "Tư vấn và báo giá"
     *   và "Yêu cầu của tôi". Không đăng nhập thì không gửi được yêu cầu nào và không đọc được
     *   yêu cầu nào.
     *
     *   Người đã đồng ý cho việc thứ nhất CHƯA đồng ý cho việc thứ hai, và câu `de_lam_gi` là câu
     *   chính ứng dụng nói với họ — nó phải kể đủ, chứ không phải kể phần dễ nghe. Đây cũng là câu
     *   người duyệt của Zalo đối chiếu với màn hình thật.
     *
     * `man` GIỮ "Liên hệ": chỉ `KhoiDangNhap` gọi hai API này, và khối ấy vẫn nằm trên màn Liên
     * hệ. Hai màn mới KHÔNG gọi chúng — chúng chỉ ĐỌC phiếu phiên đã phát hành. Ghi "Liên hệ ·
     * Tư vấn" ở đây là khai sai chỗ lời gọi chạy, và người duyệt sẽ đi tìm một nút không có ở đó.
     */
    api: "getAccessToken",
    nua: "ca-hai",
    // "Xác nhận xã" THÊM 27/09/2026: nút "Đúng, tiếp tục" trên màn xác nhận xã (sau khi quét QR của
    // xã) gọi lại chính lời gọi này để mở phiên làm việc với xã ấy. Cùng một lời gọi, một mục đích
    // mới — và câu `de_lam_gi` phải kể cả hai, vì đây là câu màn Quản lý quyền đọc cho người dùng.
    // "Gửi phản ánh · Phản ánh của tôi · Tra cứu phiếu của tôi" THÊM 28/09/2026: lần mở lại phiên kèm số
    // điện thoại (dòng `getPhoneNumber` ngay dưới) lấy lại mã phiên này trước.
    // "Vị trí nơi xảy ra sự việc" THÊM 29/09/2026: the location exchange (`vihat-miniapp`
    // `POST /api/v1/location`) needs this token beside the `getLocation` one — same call, one more
    // place the code goes, so the sentence tells it.
    man: "Liên hệ · Xác nhận xã · Gửi phản ánh · Phản ánh của tôi · Tra cứu phiếu của tôi",
    tinh_nang: "Đăng nhập bằng số Zalo · Mở phiên làm việc với xã · Vị trí nơi xảy ra sự việc",
    de_lam_gi:
      "Lấy mã phiên Zalo của bạn. Mã này không chứa tên hay số điện thoại; chỉ máy chủ đổi được nó thành định danh người dùng. Phiên mở ra từ mã này là thứ cho bạn gửi yêu cầu tư vấn và xem lại những yêu cầu của chính mình, và — sau khi bạn quét mã QR của xã và bấm xác nhận — là thứ mở phiên làm việc với đúng xã ấy. Khi xã cần xác nhận số điện thoại của bạn, ứng dụng lấy lại mã này để mở lại phiên ấy kèm số điện thoại bạn đồng ý chia sẻ. Khi bạn bấm “Lấy vị trí hiện tại” lúc gửi phản ánh, mã này đi cùng mã vị trí để máy chủ đổi mã vị trí thành toạ độ.",
    hoi_nguoi_dung: false,
    roi_khoi_may:
      "Mã phiên được gửi tới máy chủ để phát hành phiên đăng nhập, để mở phiên làm việc với xã, hoặc — khi bạn bấm lấy vị trí — để đổi mã vị trí thành toạ độ.",
    // ADR 0066: the commune app's login goes straight to ViGov identity; only the location exchange still
    // goes to `vihat-miniapp` (decision 5 moves it later — change this sentence in that card).
    commune_app: {
      man: "Gửi phản ánh · Phản ánh của tôi · Tra cứu phiếu của tôi · Đánh giá kết quả xử lý",
      tinh_nang: "Xác nhận số điện thoại với xã · Vị trí nơi xảy ra sự việc",
      de_lam_gi:
        "Lấy mã phiên Zalo của bạn. Mã này không chứa tên hay số điện thoại. Lần đầu bạn gửi, xem, tra cứu hoặc đánh giá phản ánh, mã này đi cùng mã số điện thoại bạn đồng ý chia sẻ tới hệ thống của xã, để mở phiên làm việc với xã. Khi bạn bấm “Lấy vị trí hiện tại” lúc gửi phản ánh, mã này đi cùng mã vị trí để máy chủ đổi mã vị trí thành toạ độ.",
      roi_khoi_may:
        "Mã phiên được gửi thẳng tới hệ thống của xã để mở phiên làm việc với xã, hoặc — khi bạn bấm lấy vị trí — tới máy chủ của Tập đoàn ViHAT Group để đổi mã vị trí thành toạ độ.",
    },
  },
  {
    // "Gửi phản ánh · Phản ánh của tôi · Tra cứu phiếu của tôi" THÊM 28/09/2026 (quyết định của người
    // dùng): khi ViGov trả 403 `chua_xac_thuc_so`, ba màn ấy hỏi công dân rồi — chỉ sau cú bấm đồng ý —
    // gọi lại lời gọi này qua `xinMaDangNhap` (`features/dang-nhap/cau-vigov.ts`
    // `reopenCitizenSessionWithPhone`). Cùng lời gọi, thêm một nơi mã đi tới — nên câu phải kể thêm.
    api: "getPhoneNumber",
    nua: "ca-hai",
    man: "Liên hệ · Gửi phản ánh · Phản ánh của tôi · Tra cứu phiếu của tôi",
    tinh_nang: "Đăng nhập bằng số Zalo · Xác nhận số điện thoại với xã",
    de_lam_gi:
      "Lấy mã số điện thoại sau khi bạn đồng ý chia sẻ. Số điện thoại KHÔNG nằm trong mã; chỉ máy chủ đổi được mã thành số. Số ấy làm tên đăng nhập của bạn, là nơi nhận thông báo ZNS, và là số chúng tôi gọi lại nếu bạn tự đề nghị gọi lại ở màn Tư vấn và báo giá. Khi xã cần xác nhận số điện thoại của bạn để gửi hoặc xem phản ánh, ứng dụng nói rõ vì sao và hỏi bạn trước; chỉ khi bạn bấm đồng ý, mã mới được gửi qua máy chủ của chúng tôi tới hệ thống của xã.",
    hoi_nguoi_dung: true,
    roi_khoi_may:
      "Mã số điện thoại được gửi tới máy chủ để phát hành phiên đăng nhập, hoặc để mở lại phiên làm việc với xã kèm số điện thoại đã xác nhận.",
    commune_app: {
      man: "Gửi phản ánh · Phản ánh của tôi · Tra cứu phiếu của tôi · Đánh giá kết quả xử lý",
      tinh_nang: "Xác nhận số điện thoại với xã",
      de_lam_gi:
        "Lấy mã số điện thoại ở lần đầu bạn gửi, xem, tra cứu hoặc đánh giá phản ánh — sau khi ứng dụng nói rõ vì sao và bạn bấm đồng ý. Số điện thoại KHÔNG nằm trong mã; chỉ hệ thống của xã đổi được mã thành số. Nhờ số này, xã biết phản ánh là của ai, và chỉ bạn xem được phản ánh của bạn.",
      roi_khoi_may: "Mã số điện thoại được gửi thẳng tới hệ thống của xã để mở phiên làm việc với xã.",
    },
  },
  {
    // THÊM 28/09/2026 — lời gọi ĐẦU TIÊN của riêng nửa nhà nước. Chủ dự án: "không còn đăng nhập nữa, chỉ
    // cần xin quyền để lấy được name, phone number". Tên thì `getUserInfo` trả thẳng sau khi người dân
    // đồng ý (NĐ 13/2023 — `autoRequestPermission`). Số điện thoại KHÔNG: `getPhoneNumber` chỉ trả mã,
    // máy chủ mới đổi được. Từ 29/09/2026 app riêng gọi nó CHỈ để mở phiên ở việc cá nhân đầu tiên (dòng
    // `getPhoneNumber` dưới, `cau-vigov.ts` `openCommuneAppSessionWithPhone`); số không về máy, nên ô số
    // điện thoại của biểu mẫu vẫn do người dân tự gõ.
    // 29/09/2026 (user decision): asked ONCE, when the commune app opens — no longer from a button on
    // "Gửi phản ánh" / "Cá nhân". Those two screens now only SHOW the name taken at entry.
    api: "getUserInfo",
    nua: "nha-nuoc",
    man: "Mở ứng dụng của xã (tên hiện ở Trang chủ, Cá nhân và ô họ tên của Gửi phản ánh)",
    tinh_nang: "Điền sẵn họ tên khi gửi phản ánh",
    de_lam_gi:
      "Lấy tên hiển thị Zalo của bạn một lần, khi bạn mở ứng dụng của xã và đồng ý, để điền sẵn ô họ tên khi gửi phản ánh tới xã. Tên chỉ nằm trên điện thoại này; ứng dụng không gửi nó đi đâu cho tới khi bạn tự bấm gửi phản ánh.",
    hoi_nguoi_dung: true,
    roi_khoi_may: "",
  },
  {
    api: "getLocation",
    nua: "ca-hai",
    // "Gửi phản ánh" THÊM 28/09/2026: ứng dụng riêng của một xã (`--vao-thang`) có nút "Lấy vị trí hiện
    // tại" để cán bộ tìm đúng nơi xảy ra sự việc. Cùng một lời gọi, cùng ranh giới: chỉ nhận MÃ.
    // 29/09/2026: "Gửi phản ánh" (both apps) now EXCHANGES the code — `vihat-miniapp` `POST
    // /api/v1/location` (0dada0f) turns it into coordinates, which come back to the phone, are shown to
    // the citizen, and go to the commune only inside a petition the citizen sends. "Tìm văn phòng" is
    // unchanged: it still only takes the code and exchanges nothing.
    man: "Liên hệ · Gửi phản ánh",
    tinh_nang: "Tìm văn phòng gần bạn · Vị trí nơi xảy ra sự việc",
    de_lam_gi:
      "Lấy mã vị trí sau khi bạn đồng ý chia sẻ. Ở màn Liên hệ, mã chỉ ở lại trên máy: bản dựng này chưa xếp được văn phòng theo khoảng cách và nói thẳng điều đó trên màn hình. Ở màn Gửi phản ánh, khi bạn bấm “Lấy vị trí hiện tại”, mã được gửi tới máy chủ của Tập đoàn ViHAT Group để đổi thành toạ độ; toạ độ hiện lên màn hình và chỉ tới xã nếu bạn bấm gửi phản ánh.",
    hoi_nguoi_dung: true,
    roi_khoi_may:
      "Ở màn Gửi phản ánh: mã vị trí được gửi tới máy chủ để đổi thành toạ độ, và toạ độ đi cùng phản ánh tới xã khi bạn gửi. Ở màn Liên hệ: không có gì rời khỏi máy.",
    commune_app: {
      man: "Gửi phản ánh",
      tinh_nang: "Vị trí nơi xảy ra sự việc",
      de_lam_gi:
        "Lấy mã vị trí khi bạn bấm “Lấy vị trí hiện tại” và đồng ý chia sẻ. Mã được gửi tới máy chủ của Tập đoàn ViHAT Group để đổi thành toạ độ; toạ độ hiện lên màn hình và chỉ tới xã nếu bạn bấm gửi phản ánh.",
      roi_khoi_may:
        "Mã vị trí được gửi tới máy chủ để đổi thành toạ độ, và toạ độ đi cùng phản ánh tới xã khi bạn gửi.",
    },
  },
];

/**
 * Kết quả một lần gọi nền tảng. TỪ CHỐI LÀ MỘT NHÁNH RIÊNG, NGANG HÀNG VỚI THÀNH CÔNG — không
 * phải một lỗi: người dùng có quyền nói không, và một app coi đó là lỗi sẽ hiện một câu trách móc.
 */
export type KetQuaXin<T> =
  | { kieu: "xong"; du_lieu: T }
  | { kieu: "tu-choi" }
  | { kieu: "ngoai-zalo" }
  | {
      kieu: "khong-lay-duoc";
      /**
       * What the platform actually answered — present whenever the SDK threw an error carrying a
       * numeric `code`; absent when nothing was measured (a throw without a code, or a result built
       * by hand in a test). See `SdkFailure`.
       */
      failure?: SdkFailure;
    };

/**
 * WHICH CAPABILITY a failed call was asking for — named by what it gives the app, not by the SDK
 * function (the SDK names may appear in THIS file only: `phase1-collects-nothing.test.ts`).
 *
 * WHY IT IS CARRIED AT ALL: a Zalo permission not yet granted to the App ID (Developer Console) fails
 * the call like any other error. Without the capability and the code, every such failure reached the
 * screen as the same "thử lại" sentence, and a tester could not see WHICH permission was missing. One
 * sign-in asks for two things (session code, then phone), so the capability is the step that failed,
 * not the function that was called.
 */
export type ZaloCapability = "access-token" | "phone" | "location" | "name" | "other";

export type SdkFailure = {
  readonly capability: ZaloCapability;
  /** The SDK's `code`, verbatim. The code IS the measurement — no table here guesses what it means. */
  readonly code: number;
  /** `true` only for the codes the SDK itself answers with "Please try again later" (`TRANSIENT_CODES`). */
  readonly transient: boolean;
};

/**
 * Mã lỗi người dùng từ chối. Lấy từ chính ví dụ trong `zmp-sdk/index.d.ts` (`if (code === -201)`
 * — "Người dùng đã từ chối cấp quyền"), không phải từ trí nhớ.
 *
 * Không khớp mã này thì rơi vào `khong-lay-duoc`, và câu hiện ra vẫn nói việc cần làm tiếp. Đoán
 * sai theo hướng ấy thì tệ nhất là một câu chung chung; đoán sai theo hướng ngược lại là mắng
 * người vừa bấm "Từ chối" rằng họ gặp lỗi.
 */
const MA_TU_CHOI = -201;

/**
 * THE ONLY CODES CALLED "TRY AGAIN" — read from `node_modules/zmp-sdk/apis/constants.js` (2.53.0), where
 * `RESPONSE_CODE` is `UNKNOWN_ERROR:-2e3 · TIME_OUT:-1408 · FAIL_LIMIT:-1409 · DUPLICATE_REQUEST:-1410`
 * and `MESSAGES` gives exactly those four the text "... Please try again later." Every other code is
 * shown as NOT a passing glitch: pressing again will meet the same answer (a permission the App ID lacks
 * does not appear by retrying). A code outside this set is never mapped to a meaning here.
 */
const TRANSIENT_CODES: ReadonlySet<number> = new Set([-2000, -1408, -1409, -1410]);

/** The numeric `code` of what the SDK threw (`{ code: number; message?: string }`), or `null`. Only
 *  `code` is read — `message` is technical text, and README §Error message shape keeps it off screens. */
function sdkErrorCode(loi: unknown): number | null {
  if (typeof loi !== "object" || loi === null) return null;
  const code = (loi as { code?: unknown }).code;
  return typeof code === "number" && Number.isFinite(code) ? code : null;
}

/**
 * Gọi một API của Zalo và quy mọi đường về bốn nhánh trên.
 *
 * `nap` tách khỏi `goi` vì hai lời hứa khác nhau: nhập mô-đun hỏng nghĩa là KHÔNG Ở TRONG ZALO
 * (nói ra được, sửa được bằng cách mở trong Zalo), còn lời gọi hỏng nghĩa là người dùng từ chối
 * hoặc nền tảng không trả lời. Gộp hai thứ vào một `catch` thì màn hình nói sai một trong hai.
 *
 * `capability` is the step the call starts with; a call made of two steps moves it with `step(...)`
 * right before the second, so a failure names the step that failed.
 */
async function xin<T>(
  capability: ZaloCapability,
  goi: (sdk: typeof import("zmp-sdk"), step: (next: ZaloCapability) => void) => Promise<T>,
): Promise<KetQuaXin<T>> {
  let sdk: typeof import("zmp-sdk");
  try {
    sdk = await import("zmp-sdk");
  } catch {
    return { kieu: "ngoai-zalo" };
  }

  let current = capability;
  try {
    return {
      kieu: "xong",
      du_lieu: await goi(sdk, (next) => {
        current = next;
      }),
    };
  } catch (loi) {
    const code = sdkErrorCode(loi);
    if (code === MA_TU_CHOI) return { kieu: "tu-choi" };
    if (code === null) return { kieu: "khong-lay-duoc" };
    return { kieu: "khong-lay-duoc", failure: { capability: current, code, transient: TRANSIENT_CODES.has(code) } };
  }
}

/** Token số điện thoại. Chuỗi rỗng là câu trả lời thật của nền tảng ở môi trường phát triển. */
export function xinTokenSoDienThoai(): Promise<KetQuaXin<string>> {
  return xin("phone", async (sdk) => (await sdk.getPhoneNumber()).token ?? "");
}

/**
 * HAI MÃ CỦA MỘT LẦN ĐĂNG NHẬP — `getAccessToken` + `getPhoneNumber`, đúng luồng ADR 0020.
 *
 * ⚠ CẢ HAI ĐỀU LÀ MÃ, KHÔNG PHẢI DỮ LIỆU. Số điện thoại không nằm trong mã nào, và cả hai chỉ
 * đổi được ở MÁY CHỦ bằng khoá bí mật của Mini App (ADR 0020, bất biến 2). Đổi mã ở client là
 * đưa khoá bí mật xuống máy người dùng — điều kiện dừng #1 của chính ADR ấy, và luật 8 cấm #5.
 *
 * `getAccessToken` GỌI TRƯỚC, CÓ CHỦ ĐÍCH: `index.d.ts` dòng 3009 ghi rằng từ SDK 2.35.0 lời
 * gọi này KHÔNG hỏi người dùng. Nên nếu nó hỏng thì hỏng trước khi ai bị làm phiền bằng một hộp
 * thoại; gọi sau thì người dùng vừa bấm "Đồng ý" chia sẻ số xong lại nhận một câu báo hỏng.
 *
 * KHÔNG CÓ OTP Ở ĐÂY, VÀ SẼ KHÔNG CÓ: ADR 0020 chọn một chạm chính vì màn nhập sáu số là rào
 * với người cao tuổi. Ai định thêm một bước "nhập mã" vào đây phải mở lại ADR ấy trước.
 */
export type MaDangNhap = {
  /** Token của `getPhoneNumber` — máy chủ đổi thành số điện thoại. */
  ma_so_dien_thoai: string;
  /** Access token của phiên Zalo — máy chủ đổi thành định danh người dùng Zalo. */
  ma_truy_cap: string;
};

export function xinMaDangNhap(): Promise<KetQuaXin<MaDangNhap>> {
  return xin("access-token", async (sdk, step) => {
    const ma_truy_cap = await sdk.getAccessToken();
    step("phone");
    const { token } = await sdk.getPhoneNumber();
    return { ma_so_dien_thoai: token ?? "", ma_truy_cap };
  });
}

/**
 * CHỈ access token — cho nút xác nhận xã (mở phiên công dân ViGov qua cầu, ADR 0045 · 0047).
 *
 * KHÔNG KÈM `getPhoneNumber`, CÓ CHỦ ĐÍCH: mở phiên sau khi xác nhận xã không xin số điện thoại (ADR
 * 0045 câu 2), và lời gọi này không hỏi người dùng (`index.d.ts` dòng 3009). Một hộp thoại xin số hiện
 * ra ngay sau cú bấm "Đúng, tiếp tục" là hỏi một thứ người dân chưa cần đưa.
 */
export function xinMaTruyCap(): Promise<KetQuaXin<string>> {
  return xin("access-token", async (sdk) => sdk.getAccessToken());
}

/**
 * THE APP ID OF THE MINI APP THAT IS RUNNING — `window.APP_ID`, or `null`.
 *
 * WHERE IT COMES FROM: the Zalo runtime sets `window.APP_ID` before our code runs, and `zmp-sdk` itself
 * reads exactly that global for its own App ID (`node_modules/zmp-sdk/apis/constants.js`: `a=window.APP_ID`,
 * exported as `APP_ID`). `getAppInfo` does NOT return the id (its `GetAppInfoReturns` has name, version,
 * urls — no id), so this global is the only runtime source. Not a build constant on purpose: one bundle
 * can be pushed to several App IDs, and a baked id would be right for exactly one of them.
 *
 * IT STEERS, IT GRANTS NOTHING: `vihat-miniapp` uses it only to choose which app secret exchanges the
 * phone token, and that exchange succeeding is what verifies it (`internal/httpapi/app_zalo.go`).
 *
 * DIGITS ONLY, else `null`: `vihat-miniapp` refuses any commune App ID that is not all digits
 * (README, `ZALO_MINIAPP_COMMUNE_APP_SECRETS`). Anything else — absent outside Zalo, a number, junk — is
 * "unknown", and the caller then asks Zalo for nothing (a login body without `appId` would be read by the
 * server as the SHARED app and issue a commercial ticket). Not logged: it is not personal, but there is
 * nothing a log line of it would fix.
 */
export function readRuntimeAppId(): string | null {
  try {
    const value: unknown = (globalThis as { APP_ID?: unknown }).APP_ID;
    return typeof value === "string" && /^[0-9]{1,32}$/.test(value) ? value : null;
  } catch {
    return null;
  }
}

/**
 * Tên hiển thị Zalo. Chỉ đọc `name`: `id` là định danh người dùng theo App ID và ảnh đại diện là dữ liệu
 * cá nhân không màn nào cần.
 *
 * `ask` false = CHECK ONLY: `autoRequestPermission: false` never opens Zalo's dialog — it returns the name
 * when the citizen already allowed it, and fails otherwise. That is what lets the commune app read the
 * name at entry without prompting a returning citizen on every open. `ask` true opens Zalo's own dialog,
 * and is only called after the entry card has said why the name is wanted (policy 3.3.4, `khung.tsx`).
 */
export function layTenZalo(ask: boolean): Promise<KetQuaXin<string>> {
  return xin("name", async (sdk) => (await sdk.getUserInfo({ autoRequestPermission: ask })).userInfo.name ?? "");
}

/** Token vị trí. Không đọc `latitude`/`longitude` — xem khối chú thích đầu tệp. */
export function xinTokenViTri(): Promise<KetQuaXin<string>> {
  return xin("location", async (sdk) => (await sdk.getLocation()).token ?? "");
}

/**
 * The two codes the location exchange needs (`vihat-miniapp` `POST /api/v1/location`): the Zalo
 * access token and the one-use `getLocation` token. Both are CODES, not data — the coordinates are
 * obtained only by the server, with the app secret (the same boundary as `MaDangNhap`).
 */
export type LocationCodes = {
  access_token: string;
  location_token: string;
};

/**
 * `getAccessToken` first, then `getLocation` — same order and same reason as `xinMaDangNhap`: the first
 * asks nobody (`index.d.ts` line 3009), so a failure there happens BEFORE the citizen is shown Zalo's
 * location dialog, not after they have agreed to it. The location token lives about two minutes and is
 * single-use, so the caller exchanges it immediately; nothing here keeps either code.
 */
export function requestLocationCodes(): Promise<KetQuaXin<LocationCodes>> {
  return xin("access-token", async (sdk, step) => {
    const access_token = await sdk.getAccessToken();
    step("location");
    const { token } = await sdk.getLocation();
    return { access_token, location_token: token ?? "" };
  });
}

/** Nội dung mã QR — API DUY NHẤT ở đây trả về dữ liệu thật, không phải token. */
export function quetMaQR(): Promise<KetQuaXin<string>> {
  return xin("other", async (sdk) => (await sdk.scanQRCode()).content);
}

/**
 * Mở màn hình gọi với một số điện thoại — `openPhone`, đã đối chiếu `zmp-sdk/index.d.ts` dòng
 * 4141: `openPhone(args: … & { phoneNumber: string }): Promise<void>`, `@zaloOnly`.
 *
 * Số truyền vào là số vừa quét được từ danh thiếp của NGƯỜI KHÁC. Nó đi từ màn hình sang màn
 * quay số của hệ điều hành và không đi đâu khác: không log, không lưu, không gửi.
 */
export function moCuocGoi(so: string): Promise<KetQuaXin<void>> {
  return xin("other", async (sdk) => sdk.openPhone({ phoneNumber: so }));
}

/**
 * Mở một trang web trong webview của Zalo — `openWebview`, `zmp-sdk/index.d.ts` dòng 4491,
 * `@zaloOnly`, tối thiểu 2.11.0.
 *
 * VÌ SAO KHÔNG PHẢI MỘT THẺ `<a target="_blank">`: bên trong Zalo, một liên kết ngoài mở ra
 * không có thanh điều hướng và không có đường quay lại app. `openWebview` là API nền tảng dựng
 * ra đúng cho việc này, và nó trả người dùng về đúng chỗ họ đang đứng.
 */
export function moTrangWeb(duong_dan: string): Promise<KetQuaXin<void>> {
  return xin("other", async (sdk) => {
    await sdk.openWebview({ url: duong_dan, config: { style: "normal" } });
  });
}

/* ==============================================================================================
   SÁU QUYỀN XIN THÊM — mỗi lời gọi đã đối chiếu chữ ký trong `node_modules/zmp-sdk/index.d.ts`,
   không lấy từ tài liệu web và không lấy từ trí nhớ.

     getNetworkType()              -> { networkType: "none"|"wifi"|"cellular"|"unknown" }  :1226, :3232
     vibrate({ type?, milliseconds? }) -> Promise<void>                                    :4400–4433
     keepScreen({ keepScreenOn })  -> Promise<void>                                        :4201–4231
     requestCameraPermission()     -> { userAllow: boolean; message: string }               :1293, :4002
     openMediaPicker({ type, … })  -> { data: string[] | string }                           :1403, :4804
     downloadFile({ url?, fileBase64Data? }) -> Promise<void>                               :6060–6116

   Cả sáu đều `@requirePermission` và `@zaloOnly` trong chính `index.d.ts`: chúng cần được cấp
   quyền cho App ID ở trang Quản lý ứng dụng, và ngoài Zalo thì không chạy. Nhánh `ngoai-zalo`
   của `xin` là thứ nói ra điều thứ hai bằng tiếng Việt thay vì để một nút im lặng.
   ============================================================================================ */

/**
 * Kiểu kết nối mạng hiện tại, dưới dạng chuỗi thô của nền tảng.
 *
 * TRẢ VỀ CHUỖI THÔ, KHÔNG PHẢI NHÃN: việc quy chuỗi ấy về một nhãn tiếng Việt thuộc về
 * `noi-dung.ts` (`kieuKetNoi`), nơi kiểm được mà không cần Zalo — kể cả với một giá trị thứ năm
 * mà bản SDK hôm nay chưa có. Ép kiểu về `string` ở đây là có chủ đích: `NetworkType` là một
 * `enum` của SDK, và tin rằng nền tảng sẽ mãi trả đúng bốn giá trị ấy là tin vào một lời hứa
 * không ai viết ra.
 */
export function docKieuKetNoi(): Promise<KetQuaXin<string>> {
  return xin("other", async (sdk) => String((await sdk.getNetworkType()).networkType));
}

/**
 * Rung một nhịp để báo "xong".
 *
 * KHÔNG TRẢ VỀ GÌ VÀ KHÔNG BAO GIỜ HỎNG RA NGOÀI: rung là phản hồi phụ, không phải kết quả.
 * Một máy không rung được, hoặc một người đã tắt rung trong cài đặt, không được phép làm hỏng
 * việc họ vừa bấm — nên mọi nhánh của `xin` đều bị nuốt ở đây, có chủ đích.
 *
 * Không truyền `milliseconds`: `index.d.ts` ghi tham số ấy CHỈ có tác dụng trên Android, và mặc
 * định là 500ms. Đặt một con số chỉ chạy trên một nửa số máy là một khác biệt không ai kiểm.
 */
export async function rungMotNhip(): Promise<void> {
  await xin("other", async (sdk) => {
    await sdk.vibrate({ type: "oneShot" });
  });
}

/**
 * Bật / tắt chế độ giữ màn hình sáng.
 *
 * ⚠ BẬT RỒI PHẢI TẮT. Chế độ này sống ở tầng hệ điều hành, không thuộc về màn hình React nào:
 * bật xong rồi bỏ đó là để màn hình một người sáng mãi cho tới khi họ đóng app, tức là lấy pin
 * của họ cho một tính năng họ đã rời khỏi. `hieuUngGiuManSang` trong `giu-man-sang.ts` là thứ
 * bảo đảm cặp bật/tắt luôn đủ đôi, và nó có phép kiểm riêng.
 *
 * `keepScreen` khai trả `Promise<void>` (dòng 4231) dù có kiểu `KeepScreenReturns` trong tệp:
 * nên ở đây KHÔNG đọc `.success` — đọc một trường không tồn tại rồi hiện nó ra là dựng một
 * trạng thái giả trên màn hình.
 */
export function giuManHinhSang(bat: boolean): Promise<KetQuaXin<boolean>> {
  return xin("other", async (sdk) => {
    await sdk.keepScreen({ keepScreenOn: bat });
    return bat;
  });
}

/**
 * Hỏi quyền dùng máy ảnh.
 *
 * ⚠ `userAllow === false` LÀ MỘT KẾT QUẢ THÀNH CÔNG, KHÔNG PHẢI MỘT LỖI — và khác biệt ấy nằm
 * ở chỗ API này trả về trạng thái chứ không ném: người dùng bấm "Không cho phép" thì lời gọi
 * vẫn thành công và mang về `false`. Quy nó vào nhánh `tu-choi` sẽ hiện một câu chung chung;
 * giữ nguyên `false` thì màn hình nói được đúng việc cần làm tiếp (`SO_HOA_THIEP.tu_choi_quyen`).
 *
 * `message` của nền tảng bị bỏ hẳn: đó là chữ kỹ thuật, và README §Error message shape cấm đưa
 * nó ra cho người dùng.
 */
export function xinQuyenMayAnh(): Promise<KetQuaXin<boolean>> {
  return xin("other", async (sdk) => (await sdk.requestCameraPermission()).userAllow);
}

/**
 * Mở cửa sổ chọn ảnh và nhận về đường dẫn tệp TẠM TRÊN MÁY.
 *
 * ⚠ KHÔNG TRUYỀN `serverUploadUrl`, VÀ ĐÓ LÀ TOÀN BỘ VẤN ĐỀ:
 *
 *   `index.d.ts` dòng 4721 ghi rõ *"Tham số serverUploadUrl không còn bắt buộc. Mặc định nếu
 *   không truyền, SDK sẽ trả về đường dẫn tạm thời (local cache path) của media mà không tự
 *   động upload lên server"*. Truyền nó vào là ảnh của người dùng — rất có thể là danh thiếp
 *   của một người thứ ba — được tải lên một máy chủ. Ứng dụng này không có máy chủ nào, và dây
 *   bẫy trong `phase1-collects-nothing.test.ts` cấm mọi đường gửi ra.
 *
 *   Bỏ nó đi thì câu "ảnh không rời khỏi máy" trong chính sách quyền riêng tư là một sự thật về
 *   cấu trúc, không phải một lời hứa. Có một phép kiểm quét toàn bộ mã nguồn để giữ điều đó.
 *
 * Chuẩn hoá `string | string[]` về mảng: kiểu trả về khai cả hai vì khi CÓ `serverUploadUrl` thì
 * `data` là nguyên văn phản hồi của máy chủ. Ta không đi đường ấy, nhưng một `typeof` ở đây rẻ
 * hơn một `.map` chạy trên một chuỗi rồi hiện ra từng ký tự một.
 */
export function chonAnhTuMay(): Promise<KetQuaXin<readonly string[]>> {
  return xin("other", async (sdk) => {
    const { data } = await sdk.openMediaPicker({ type: "photo" });
    return typeof data === "string" ? [data] : data;
  });
}

/**
 * Ghi một tệp xuống máy người dùng từ dữ liệu base64 có sẵn trong app.
 *
 * ⚠ DÙNG `fileBase64Data`, KHÔNG DÙNG `url` (`index.d.ts` dòng 6060–6061, cả hai đều tuỳ chọn).
 * `url` sẽ cần một địa chỉ `https://` có thật để tải về — tức một máy chủ, thứ ứng dụng này
 * không có và không được có. `fileBase64Data` thì nội dung đi thẳng từ bộ nhớ của app sang API
 * ghi tệp của nền tảng, không một lời gọi mạng nào.
 *
 * ĐÂY LÀ HÀNH VI DUY NHẤT ỨNG DỤNG VIẾT LÊN THIẾT BỊ, nên nó được khai riêng một đoạn trong
 * chính sách quyền riêng tư. Thứ được ghi là danh thiếp CỦA CHÚNG TÔI, không phải dữ liệu của
 * người dùng — xem `vcard.ts`.
 */
export function taiTepVeMay(du_lieu_base64: string): Promise<KetQuaXin<void>> {
  return xin("other", async (sdk) => {
    await sdk.downloadFile({ fileBase64Data: du_lieu_base64 });
  });
}
