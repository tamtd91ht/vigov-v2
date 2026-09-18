/**
 * LỚP DUY NHẤT CHẠM VÀO `zmp-sdk` — và nó cố tình nhỏ đúng bằng những lời gọi ba tính năng cần.
 *
 * Tệp này là chỗ duy nhất trong cả kho được phép nhắc tên mô-đun `zmp-sdk` và tên ba hàm
 * `getPhoneNumber` · `getLocation` · `scanQRCode`. `phase1-collects-nothing.test.ts` cấm chúng ở
 * MỌI tệp khác — lệnh cấm không bị xoá, nó được thu hẹp phạm vi, và có một ca chứng minh rằng
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

/**
 * Kết quả một lần gọi nền tảng. TỪ CHỐI LÀ MỘT NHÁNH RIÊNG, NGANG HÀNG VỚI THÀNH CÔNG — không
 * phải một lỗi: người dùng có quyền nói không, và một app coi đó là lỗi sẽ hiện một câu trách móc.
 */
export type KetQuaXin<T> =
  | { kieu: "xong"; du_lieu: T }
  | { kieu: "tu-choi" }
  | { kieu: "ngoai-zalo" }
  | { kieu: "khong-lay-duoc" };

/**
 * Mã lỗi người dùng từ chối. Lấy từ chính ví dụ trong `zmp-sdk/index.d.ts` (`if (code === -201)`
 * — "Người dùng đã từ chối cấp quyền"), không phải từ trí nhớ.
 *
 * Không khớp mã này thì rơi vào `khong-lay-duoc`, và câu hiện ra vẫn nói việc cần làm tiếp. Đoán
 * sai theo hướng ấy thì tệ nhất là một câu chung chung; đoán sai theo hướng ngược lại là mắng
 * người vừa bấm "Từ chối" rằng họ gặp lỗi.
 */
const MA_TU_CHOI = -201;

/** Lỗi SDK ném ra: `{ code: number; message?: string }`. Chỉ đọc `code` — `message` là chữ kỹ
 *  thuật, và README §Error message shape cấm đưa nó ra cho người dùng. */
function laTuChoi(loi: unknown): boolean {
  return typeof loi === "object" && loi !== null && (loi as { code?: unknown }).code === MA_TU_CHOI;
}

/**
 * Gọi một API của Zalo và quy mọi đường về bốn nhánh trên.
 *
 * `nap` tách khỏi `goi` vì hai lời hứa khác nhau: nhập mô-đun hỏng nghĩa là KHÔNG Ở TRONG ZALO
 * (nói ra được, sửa được bằng cách mở trong Zalo), còn lời gọi hỏng nghĩa là người dùng từ chối
 * hoặc nền tảng không trả lời. Gộp hai thứ vào một `catch` thì màn hình nói sai một trong hai.
 */
async function xin<T>(goi: (sdk: typeof import("zmp-sdk")) => Promise<T>): Promise<KetQuaXin<T>> {
  let sdk: typeof import("zmp-sdk");
  try {
    sdk = await import("zmp-sdk");
  } catch {
    return { kieu: "ngoai-zalo" };
  }

  try {
    return { kieu: "xong", du_lieu: await goi(sdk) };
  } catch (loi) {
    return laTuChoi(loi) ? { kieu: "tu-choi" } : { kieu: "khong-lay-duoc" };
  }
}

/** Token số điện thoại. Chuỗi rỗng là câu trả lời thật của nền tảng ở môi trường phát triển. */
export function xinTokenSoDienThoai(): Promise<KetQuaXin<string>> {
  return xin(async (sdk) => (await sdk.getPhoneNumber()).token ?? "");
}

/** Token vị trí. Không đọc `latitude`/`longitude` — xem khối chú thích đầu tệp. */
export function xinTokenViTri(): Promise<KetQuaXin<string>> {
  return xin(async (sdk) => (await sdk.getLocation()).token ?? "");
}

/** Nội dung mã QR — API DUY NHẤT ở đây trả về dữ liệu thật, không phải token. */
export function quetMaQR(): Promise<KetQuaXin<string>> {
  return xin(async (sdk) => (await sdk.scanQRCode()).content);
}

/**
 * Mở màn hình gọi với một số điện thoại — `openPhone`, đã đối chiếu `zmp-sdk/index.d.ts` dòng
 * 4141: `openPhone(args: … & { phoneNumber: string }): Promise<void>`, `@zaloOnly`.
 *
 * Số truyền vào là số vừa quét được từ danh thiếp của NGƯỜI KHÁC. Nó đi từ màn hình sang màn
 * quay số của hệ điều hành và không đi đâu khác: không log, không lưu, không gửi.
 */
export function moCuocGoi(so: string): Promise<KetQuaXin<void>> {
  return xin(async (sdk) => sdk.openPhone({ phoneNumber: so }));
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
  return xin(async (sdk) => {
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
  return xin(async (sdk) => String((await sdk.getNetworkType()).networkType));
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
  await xin(async (sdk) => {
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
  return xin(async (sdk) => {
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
  return xin(async (sdk) => (await sdk.requestCameraPermission()).userAllow);
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
  return xin(async (sdk) => {
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
  return xin(async (sdk) => {
    await sdk.downloadFile({ fileBase64Data: du_lieu_base64 });
  });
}
