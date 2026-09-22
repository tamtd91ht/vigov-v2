/**
 * DẢI CHIẾN DỊCH TRÊN MÀN CHỦ — bảng tra mã, và KHÔNG có một đường nào đi vòng qua bảng ấy.
 *
 * Một mã chiến dịch đi vào app qua tham số `src` của đường mở (`lib/launch-params.ts`): QR dán ở
 * quầy lễ tân, mã in trên danh thiếp, đường liên kết trên trang Zalo. Nó nói với app rằng người
 * vừa mở tới từ đâu, và app chào lại bằng một câu hợp cảnh.
 *
 * ⚠ THAM SỐ ẤY LÀ DỮ LIỆU CLIENT TỰ ĐẶT. AI CŨNG SỬA ĐƯỢC ĐƯỜNG LIÊN KẾT TRƯỚC KHI GỬI ĐI.
 *
 *   Nên bảng này không phải một tiện nghi, nó là RANH GIỚI: mã khớp một dòng CÓ THẬT ở đây thì
 *   màn hình hiện chữ CỦA DÒNG ẤY — chữ nằm sẵn trong bundle, đã có người đọc. Mã lạ thì không
 *   hiện gì cả, im lặng, không một câu lỗi.
 *
 *   In nguyên văn chuỗi tham số ra màn hình — dù chỉ để "cho biết đã nhận" — là cho người ngoài
 *   viết chữ lên một màn hình đứng tên ViHAT Group: đưa ai đó một đường liên kết có `src=<câu
 *   muốn viết>` là đủ. Đó là lý do ở đây không có nhánh nào nhận `ma` rồi trả về chính nó, và
 *   `chien-dich.test.tsx` cho hàm này ăn đúng hình dạng tấn công ấy.
 *
 * ⚠ TRA BẰNG `hasOwnProperty`, KHÔNG BẰNG `CHIEN_DICH[ma] ?? null`. `src=constructor` (hoặc
 *   `toString`, `valueOf`) đọc thẳng vào nguyên mẫu của `Object` và trả về một thứ KHÁC `undefined`
 *   — tức là một mã lạ vẫn "khớp", và thứ hiện ra là một mảnh nội bộ của JavaScript. Fail closed
 *   nghĩa là chỉ những khoá tệp này tự khai mới được tính.
 *
 * KHÔNG GHI GÌ, KHÔNG GỬI GÌ. Giai đoạn A không có đích dữ liệu nào: mã chiến dịch chỉ dẫn GIAO
 * DIỆN, đúng như tham số `t` của lớp khám phá (ADR 0005). Nó không mở khoá gì, không được lưu, và
 * mất đi khi đóng app.
 */

export type ChienDich = {
  /** Dòng đầu của dải. Ngắn, nói ra người đọc tới từ đâu. */
  tieu_de: string;
  /** Câu chào, một câu. Không hứa hẹn gì app chưa làm được. */
  cau_chao: string;
};

/**
 * Khoá tham số mang mã chiến dịch. MỘT HẰNG, vì cả nơi đọc lẫn ca kiểm đều gọi tên nó — gõ
 * `"src"` ở hai chỗ là hai chỗ sẽ lệch.
 */
export const KHOA_CHIEN_DICH = "src";

/**
 * BA MÃ, VÀ CẢ BA MÔ TẢ MỘT KÊNH PHÁT HÀNH, KHÔNG MÔ TẢ MỘT SỰ KIỆN.
 *
 * Một dòng kiểu "Chào bạn từ hội nghị X ngày Y" là một khẳng định về một sự kiện có thật, đứng
 * tên một pháp nhân có thật, mà chưa ai trong kho này duyệt. Ba mã dưới chỉ nói lại đúng cái việc
 * người dùng vừa làm — quét mã ở quầy, quét mã trên danh thiếp, bấm liên kết trên trang Zalo —
 * nên chúng đúng bất kể chiến dịch nào đang chạy.
 *
 * ⚠ KHÔNG DÙNG `qr` VÀ `zns` LÀM MÃ Ở ĐÂY: hai chuỗi ấy đã là giá trị `src` của lớp khám phá
 * (`features/kham-pha/goi-y.ts`), và một chuỗi mang hai nghĩa trong cùng một tham số là chỗ người
 * đọc mã sau này tự kết luận sai.
 */
export const CHIEN_DICH: Readonly<Record<string, ChienDich>> = {
  "vp-quay": {
    tieu_de: "Bạn vừa quét mã tại quầy",
    cau_chao:
      "Cảm ơn bạn đã ghé văn phòng ViHAT Group. Xem giải pháp ngay tại đây, hoặc gọi hotline để có người trả lời trực tiếp.",
  },
  "thiep-nhan-vien": {
    tieu_de: "Từ tấm danh thiếp bạn vừa nhận",
    cau_chao:
      "Đây là ứng dụng của ViHAT Group. Bạn có thể xem các dòng giải pháp, hoặc gọi lại cho chúng tôi bất cứ lúc nào.",
  },
  "trang-zalo": {
    tieu_de: "Từ trang Zalo của ViHAT Group",
    cau_chao:
      "Bạn đang xem ứng dụng giới thiệu giải pháp của ViHAT Group. Ba bước gợi ý bên dưới giúp bạn tìm đúng dòng sản phẩm cần hỏi.",
  },
};

/**
 * Tra một mã, fail closed.
 *
 * `null` là câu trả lời cho MỌI thứ không phải một khoá tệp này tự khai — chuỗi rỗng, mã lạ, tên
 * một thuộc tính của nguyên mẫu. Nơi gọi chỉ có một việc với `null`: không vẽ gì.
 */
/**
 * Mã chiến dịch ĐƯỢC PHÉP GỬI LÊN MÁY CHỦ, hoặc chuỗi rỗng.
 *
 * ⚠ CÙNG MỘT LUẬT VỚI DẢI CHÀO, CHO MỘT HẬU QUẢ KHÁC. Dải chào chỉ vẽ chữ khi mã khớp một dòng
 * có thật; trường `source` của một yêu cầu cũng chỉ được mang mã khi mã ấy khớp một dòng có thật.
 *
 *   Không có cửa này thì một chuỗi người ngoài tự đặt đi thẳng vào CSDL của máy chủ và vào báo cáo
 *   hiệu quả chiến dịch — và máy chủ sẽ trả 400 cho bất cứ chuỗi nào lệch khuôn `^[a-z0-9]…`,
 *   nghĩa là một đường liên kết có `src=Xin chào` làm NÚT GỬI của người dùng hỏng, vì một thứ
 *   không liên quan gì tới họ. Lọc ở đây thì mã lạ chỉ đơn giản là không được gửi.
 *
 * Trả chuỗi rỗng chứ không `null`: `source` là một trường tuỳ chọn trên dây, và rỗng là cách hợp
 * đồng diễn đạt "không có".
 */
export function maChienDichHopLe(ma: string): string {
  return chienDichCua(ma) === null ? "" : ma;
}

export function chienDichCua(ma: string): ChienDich | null {
  // `Object.prototype.hasOwnProperty.call` chứ không `Object.hasOwn`: `Object.hasOwn` là ES2022 và
  // vắng mặt trên WebView của những máy Android cũ — đúng những máy mà app này nhận là người dùng
  // của mình. Một `TypeError` ở đây giết cả cây React, tức màn hình trắng, vì một dải trang trí.
  if (ma === "" || !Object.prototype.hasOwnProperty.call(CHIEN_DICH, ma)) return null;
  return CHIEN_DICH[ma] ?? null;
}
