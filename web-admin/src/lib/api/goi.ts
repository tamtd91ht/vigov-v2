/**
 * Cách gọi API dùng chung cho mọi tuyến — hình dạng yêu cầu và cách đọc thân lỗi.
 *
 * VÌ SAO TÁCH RA KHỎI `phien.ts`: hai quy tắc dưới đây phải giống nhau ở MỌI tuyến, và mỗi bản
 * sao của chúng là một bản sao sẽ trôi. Bản sao thứ hai không làm đỏ test nào: nó chỉ làm một
 * màn hình nào đó bắt đầu rẽ nhánh theo `code`, hoặc gửi `credentials: "include"`, ở đúng một
 * chỗ mà không ai đọc lại.
 *
 *   1. ĐƯỜNG DẪN TƯƠNG ĐỐI, KHÔNG BAO GIỜ MỘT HOST. API phục vụ trên chính host của xã, nên
 *      yêu cầu mang đúng `Host` ấy và cookie phiên (host-only) đi kèm được. Không một host nào
 *      bị nung vào bundle, và không ai *có thể* trỏ nhầm sang host của xã khác (luật 1, bất
 *      biến 10).
 *   2. KHÔNG CÓ `tenant_id` Ở BẤT KỲ ĐÂU — không thân, không query, không header. Client tự
 *      khai xã là client tự cấp quyền (luật 1, cấm #2).
 *
 * KHÔNG ĐỤNG COOKIE. Cookie phiên do dịch vụ identity đặt bằng Set-Cookie (httpOnly, secure,
 * SameSite=Lax, KHÔNG có thuộc tính Domain). Mã ở đây chỉ bật `credentials` để nó được gửi
 * kèm; không đọc, không ghi, không xoá.
 */

import type { httpx_Error } from "./schema.gen";

/**
 * Kết quả một lời gọi: hoặc dữ liệu, hoặc **một** thông báo cho người dùng đọc.
 *
 * Cố ý KHÔNG mang theo mã lỗi ra tới giao diện. Giao diện chỉ có đúng một chuỗi để hiển thị,
 * nên không có chỗ nào để rẽ nhánh theo `code` — xem `thongBaoLoi`.
 */
export type KetQua<T> = { ok: true; duLieu: T } | { ok: false; thongBao: string };

/** Câu trả lời khi không đọc nổi thân lỗi của máy chủ. Không bao giờ lộ chi tiết kỹ thuật. */
export const LOI_KHONG_RO = "Không kết nối được máy chủ. Vui lòng thử lại.";

/**
 * Lấy câu thông báo do máy chủ viết, nguyên văn.
 *
 * MỌI mã lỗi đều trả về cùng một hình dạng `httpx.Error` (`code` · `message` · `trace_id`) —
 * kể cả 401, 403 và 404. Nên ở đây chỉ cần đọc `message`, và **chỉ được** đọc `message`:
 *
 *   - KHÔNG rẽ nhánh giao diện theo `code` để đoán chuyện gì đã xảy ra. `code` là định danh
 *     cho máy, và ở màn đăng nhập máy chủ CỐ Ý trả cùng một `code` cho email sai và mật khẩu
 *     sai — dựng lại sự phân biệt ấy ở client là dựng lại đúng thứ máy chủ vừa giấu đi.
 *   - KHÔNG hiện `trace_id`. Nó là mốc để tra log, không phải mã lỗi nghiệp vụ; đặt nó cạnh
 *     một câu thông báo là mời cán bộ đọc nó thành "mã lỗi" rồi đọc lại qua điện thoại.
 */
export async function thongBaoLoi(phanHoi: Response): Promise<string> {
  try {
    const than = (await phanHoi.json()) as httpx_Error;
    return typeof than?.message === "string" && than.message !== "" ? than.message : LOI_KHONG_RO;
  } catch {
    return LOI_KHONG_RO;
  }
}

export const CHUNG: RequestInit = {
  // same-origin: cookie phiên đi kèm vì API nằm trên chính host của xã. Không dùng "include" —
  // "include" chỉ cần thiết khi gửi sang origin khác, mà gửi phiên sang origin khác là đúng
  // điều không được phép xảy ra ở đây.
  credentials: "same-origin",
  // Không cache lời gọi nào ở đây, ở bất kỳ tầng nào: cả phiên làm việc lẫn danh bạ cán bộ đều
  // là dữ liệu của một xã cụ thể, và một bản đệm sống lâu hơn yêu cầu là đúng hình dạng của
  // một lần dữ liệu xã này hiện trên màn hình xã khác.
  cache: "no-store",
};

/**
 * GET một tuyến, trả về `KetQua`. Chỉ 200 là thành công; mọi mã khác thành một câu thông báo.
 *
 * Kiểu `T` do bên gọi truyền vào TỪ `schema.gen.ts` — hàm này không biết hình dạng nào cả, nên
 * không có chỗ nào ở đây gõ tay lại một hình dạng của hợp đồng.
 */
export async function docJSON<T>(duongDan: string): Promise<KetQua<T>> {
  let phanHoi: Response;
  try {
    phanHoi = await fetch(duongDan, { ...CHUNG, method: "GET" });
  } catch {
    // Mạng hỏng. Không ghi log gì: phản hồi của các tuyến này chứa dữ liệu cá nhân (luật 3).
    return { ok: false, thongBao: LOI_KHONG_RO };
  }

  if (phanHoi.status !== 200) return { ok: false, thongBao: await thongBaoLoi(phanHoi) };

  try {
    return { ok: true, duLieu: (await phanHoi.json()) as T };
  } catch {
    // 200 mà thân không phải JSON là máy chủ hoặc proxy đang trả thứ khác. Với người dùng thì
    // đó vẫn là "không đọc được", không phải một trạng thái nghiệp vụ.
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/**
 * Trả về hàm đặt MỘT tham số truy vấn, tên và kiểu giá trị lấy từ hợp đồng `T`
 * (`<tuyen>["truyVan"]` trong `schema.gen.ts`). Máy chủ đổi tên một tham số thì `tsc` đỏ ở đúng
 * dòng gửi nó, thay vì máy chủ lặng lẽ bỏ qua một tham số lạ và trả CẢ danh sách trong khi cán bộ
 * tin mình đang xem một lát cắt.
 *
 * HAI TẦNG HÀM, KHÔNG MỘT: TypeScript không suy được `K` khi `T` đã ghi tay, nên một hàm phẳng
 * `datThamSo<T>(…, "sort", x)` chỉ kiểm TÊN mà để lọt mọi chuỗi vào `sort`. Tầng trong suy `K` từ
 * tên, nên giá trị bị đối chiếu với đúng trường ấy.
 *
 * Vắng mặt, `null` hay chuỗi rỗng thì KHÔNG đặt: chuỗi rỗng nghĩa là "không lọc", và máy chủ từ
 * chối một `status=` hay `cursor=` rỗng thay vì bỏ qua (400 ngay lần mở màn hình đầu tiên).
 *
 * Ở `goi.ts` vì đã có hai nơi dùng — sổ văn bản (`van-ban.ts`, nơi nó ra đời) và danh bạ cán bộ
 * (`can-bo.ts`). Bản sao thứ hai của phép "bỏ qua chuỗi rỗng" là bản sẽ quên nhánh ấy.
 */
export function thamSoTheoHopDong<T extends object>(truyVan: URLSearchParams) {
  return <K extends Extract<keyof T, string>>(
    ten: K,
    giaTri: NonNullable<T[K]> | null | undefined,
  ): void => {
    if (giaTri === undefined || giaTri === null || giaTri === "") return;
    truyVan.set(ten, String(giaTri));
  };
}

/**
 * GHI một tuyến, trả về phản hồi thô — bên gọi tự đọc kiểu của mình.
 *
 * NÓ ĐẾN ĐÂY VÌ ĐÃ CÓ HAI BẢN SAO, không vì một nguyên tắc. `lib/api/danh-muc.ts` viết bản đầu
 * kèm đúng một điều kiện: *"màn hình ghi thứ hai xuất hiện là lúc hàm này chuyển sang `goi.ts`"*.
 * Màn hình ấy là danh bạ cán bộ, và nó đến kèm một bản thứ hai — nên điều kiện đã thoả và hai bản
 * gộp lại thành một. Bản sao thứ hai của cách gọi ghi là đúng thứ `goi.ts` được tách ra để chặn.
 *
 * TRẢ `Response` CHỨ KHÔNG PHẢI `T` ĐÃ PHÂN GIẢI, khác `docJSON` ngay bên trên. Không phải để
 * tổng quát hơn: `DELETE` của danh mục thành công bằng **204 không thân**, nên một hàm luôn gọi
 * `.json()` sẽ biến lần xoá thành công thành "không đọc được". Bên gọi biết tuyến của mình trả gì.
 *
 * `than === undefined` THÌ KHÔNG CÓ THÂN VÀ KHÔNG CÓ `Content-Type`. Hai tuyến khoá/mở khoá cán bộ
 * cố ý không nhận thân nào (`can_bo_ghi.go`: lược đồ không có cột nào giữ lý do khoá, nên một ô lý
 * do sẽ rơi vào vết kiểm toán mà không màn hình nào đọc lại được). Gửi `{}` kèm `Content-Type` là
 * tuyên bố có một thân — thứ sẽ mời người sau điền vào.
 *
 * KHÔNG RẼ NHÁNH THEO `code`, KHÔNG HIỆN `trace_id`, KHÔNG HIỆN SỐ HIỆU HTTP. `thongBaoLoi` đọc
 * đúng `message` máy chủ viết, và với các tuyến ghi thì đó là toàn bộ điểm: một lần từ chối theo
 * tầng trả 409 kèm nguyên câu *"Mục do hệ thống cấp thì không xoá được…"*, và một lần chạm ràng
 * buộc #13 trả nguyên câu về người quản trị cuối cùng. Viết lại chúng ở client là dựng bản sao thứ
 * hai của một quy tắc nghiệp vụ, và bản sao ấy trôi mà không ai thấy.
 *
 * KHÔNG GHI LOG GÌ KHI MẠNG HỎNG: thân yêu cầu mang chữ cán bộ vừa gõ — họ tên, số điện thoại, lý
 * do xoá (luật 3).
 */
export async function goiGhi(
  duongDan: string,
  phuongThuc: "POST" | "PATCH" | "PUT" | "DELETE",
  than: unknown | undefined,
  maMongDoi: number,
  headerThem?: Readonly<Record<string, string>>,
): Promise<KetQua<Response>> {
  let phanHoi: Response;
  try {
    phanHoi = await fetch(duongDan, {
      ...CHUNG,
      method: phuongThuc,
      headers:
        than === undefined
          ? { ...headerThem }
          : { "Content-Type": "application/json", ...headerThem },
      body: than === undefined ? undefined : JSON.stringify(than),
    });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }

  if (phanHoi.status !== maMongDoi) return { ok: false, thongBao: await thongBaoLoi(phanHoi) };
  return { ok: true, duLieu: phanHoi };
}

/**
 * Đọc thân JSON của một lần GHI đã thành công, thành kiểu của tuyến.
 *
 * ĐÂY LÀ CHỖ MƯỜI BA BẢN SAO GẶP NHAU (gom 24/09/2026). `goiGhi` ngay trên trả `Response` THÔ và
 * nêu lý do: `DELETE` của danh mục thành công bằng **204 không thân**, nên một hàm gọi `.json()`
 * vô điều kiện sẽ biến lần xoá thành công thành "không đọc được". Hệ quả không được lường trước là
 * mọi tệp có tuyến ghi TRẢ thân đều phải tự viết lấy lớp mỏng này — mười bản có TÊN dưới ba chữ ký,
 * một bản hợp nhất trong `goiGhiCanBo` của `can-bo.ts`, và hai bản viết THẲNG trong thân hàm ở
 * `thoi-han-xu-ly.ts`. Hai bản cuối là chỗ đáng sợ nhất: không mang tên nào thì không đếm được, nên
 * lần kiểm nào cũng bỏ sót chúng. Điều kiện `goi.ts` tự đặt cho mình từ bản đầu — *"màn hình ghi
 * thứ hai xuất hiện là lúc hàm này chuyển sang `goi.ts`"* — đã thoả từ rất lâu trước khi có ai gom.
 *
 * HAI HÀM, KHÔNG MỘT VÀ CŨNG KHÔNG BA. Ba chữ ký cũ khác nhau đúng ở chỗ bên gọi đang cầm gì, chứ
 * không ở việc chúng làm. Bản này nhận một `KetQua` ĐÃ CÓ; `docThanLoiGoi` ngay dưới nhận LỜI GỌI
 * chưa xong. Chữ ký thứ ba — nhận thẳng `Response` — KHÔNG còn: mọi chỗ gọi nó viết
 * `kq.ok ? doc(kq.duLieu) : kq`, tức là dựng lại bằng tay đúng nhánh `!ok` của hàm này. Ép cả hai
 * bản còn lại vào một chữ ký thì hàm phải hỏi lúc CHẠY "đối số này là promise hay chưa" — một câu
 * hỏi kiểu đã trả lời được, và một nhánh `if` mà không ca kiểm nào đi qua đủ cả hai chiều.
 *
 * KHÔNG CÓ NHÁNH NÀO CHO 204, CÓ CHỦ Ý. Tuyến thành công không thân thì KHÔNG đi qua đây: nó đọc
 * `kq.ok` rồi trả thẳng `duLieu: null` (xem `xoaMuc` trong `danh-muc.ts`, `xoaDong` trong
 * `lich-ghi.ts`). Một tham số "có thân hay không" ở đây là chỗ để ai đó truyền sai, và lần truyền
 * sai ấy biến một lần xoá THÀNH CÔNG thành một câu báo lỗi trên màn hình.
 *
 * KHÔNG GHI LOG GÌ KHI THÂN HỎNG, và điều đó đúng cho MỌI tuyến đi qua đây chứ không riêng tuyến
 * nào: thân của chúng mang họ tên và số điện thoại cán bộ, nội dung biên bản họp có thể nhắc tới
 * hồ sơ của một công dân, số tiền công quỹ và lý do gỡ do người gõ (luật 3, cấm #1). Một dòng log
 * thêm vào đây để gỡ lỗi sẽ đẩy cả bốn thứ ấy đi cùng một lúc.
 */
export async function docThanKetQua<T>(ketQua: KetQua<Response>): Promise<KetQua<T>> {
  if (!ketQua.ok) return ketQua;
  try {
    return { ok: true, duLieu: (await ketQua.duLieu.json()) as T };
  } catch {
    // Đúng mã mong đợi mà thân không phải JSON là máy chủ hoặc proxy đang trả thứ khác. Với cán bộ
    // thì đó vẫn là "không đọc được", không phải một trạng thái nghiệp vụ.
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/**
 * Như `docThanKetQua`, nhưng nhận LỜI GỌI chưa xong: `docThanLoiGoi<T>(goiGhi(...))`.
 *
 * KHÔNG CÓ LOGIC RIÊNG — một phép `then` rồi giao lại, đúng để không có bản thứ hai của phép phân
 * giải thân. Nó tồn tại vì phần lớn chỗ gọi dựng đối số của `goiGhi` trên nhiều dòng, và một
 * `.then(...)` treo sau dấu ngoặc đóng ấy đọc ngược thứ tự việc.
 */
export function docThanLoiGoi<T>(loiGoi: Promise<KetQua<Response>>): Promise<KetQua<T>> {
  return loiGoi.then(docThanKetQua<T>);
}
