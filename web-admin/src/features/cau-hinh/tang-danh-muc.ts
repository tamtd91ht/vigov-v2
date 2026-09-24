/**
 * BA TẦNG CỦA MỘT MỤC DANH MỤC → NHỮNG NÚT ĐƯỢC PHÉP VẼ. Hàm thuần, không gọi mạng, không dựng
 * DOM — vì đây là quyết định dễ nói sai nhất của màn hình, và nó phải kiểm được mà không cần
 * một trình duyệt.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY LÀ LỚP TIỆN DỤNG, KHÔNG PHẢI LỚP CHẶN — và câu này không phải lời rào đón. Cái chặn thật
 * nằm ở CSDL: một trigger `BEFORE UPDATE OR DELETE` trên từng bảng danh mục
 * (`service-documents/migrations/0003_danh_muc_loai_van_ban.sql:138`), và nó giữ trước mọi
 * người ghi — dịch vụ này, một dòng `psql`, một tác vụ nhập liệu sau này. Tầng dịch vụ từ chối
 * trước đó một nhịp để câu trả lời là tiếng Việt chứ không phải một ngoại lệ PostgreSQL.
 *
 * Vậy tệp này để làm gì: để cán bộ KHÔNG BẤM vào một nút chắc chắn trả 409. Một nút `Xoá` vẽ ra
 * ở dòng của hệ thống là một lời mời tới một thông báo lỗi — và người bấm sẽ kết luận phần mềm
 * hỏng, chứ không kết luận quy tắc đang làm đúng việc của nó. Nếu hàm dưới đây có ngày trả lời
 * rộng hơn quy tắc, hậu quả là một câu 409 hiện trên màn hình; nó không bao giờ là một dòng
 * danh mục bị xoá.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * `tier` LÀ SUY RA, KHÔNG LƯU: máy chủ tính từ `nguon` và `ma_nguon_re_nhanh` mỗi lần trả lời
 * (`service-documents/internal/domain/danh_muc_ba_tang.go:64`). Màn hình vì vậy chỉ ĐỌC nó và
 * không bao giờ gửi lại — xem `lib/api/danh-muc.ts`, phần hai.
 */

import type { MucDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";
import type { MucDanhMucGhi } from "@/lib/api/danh-muc";

/** Đơn vị tự thêm: xoá mềm CÓ · tắt CÓ · đổi nhãn CÓ. */
export const TANG_DON_VI = 1;
/** Phần mềm cấp: xoá mềm KHÔNG · tắt CÓ · đổi nhãn CÓ. */
export const TANG_HE_THONG = 2;
/** Phần mềm cấp VÀ mã nguồn rẽ nhánh theo mã này: xoá KHÔNG · tắt KHÔNG · đổi nhãn CÓ. */
export const TANG_RE_NHANH = 3;

export type ThaoTacChoPhep = {
  /** Đổi nhãn hiển thị, thứ tự, hoặc đặt mặc định. */
  readonly doiNhan: boolean;
  /** Đưa mục ra khỏi danh sách chọn khi lập hồ sơ mới. */
  readonly tat: boolean;
  /** Xoá **mềm**, kèm lý do bắt buộc. */
  readonly xoa: boolean;
};

/** Không vẽ nút nào. Dùng cho mọi ca không đọc ra được tầng — xem `thaoTacTheoTang`. */
const KHONG_THAO_TAC: ThaoTacChoPhep = { doiNhan: false, tat: false, xoa: false };

/**
 * Một mục có phải mục của danh mục CÓ ĐƯỜNG GHI hay không.
 *
 * Từ 7aa0127 cả bảy danh mục đều phát ra `order`, `source`, `tier`. Phép thử vẫn hỏi đúng thứ nó
 * cần biết — "hợp đồng có nói tầng của dòng này không" — chứ không hỏi tên nhóm: một dòng không
 * nói được tầng của nó (hợp đồng trôi) thì không nút ghi nào được vẽ. Nhóm CÓ được vẽ nút hay
 * không là câu hỏi khác, do mô tả đường ghi của nhóm trả lời (`nhom-danh-muc.ts`, trường `ghi`).
 */
export function laMucGhi(m: MucDanhMuc): m is MucDanhMucGhi {
  return "tier" in m && "source" in m && "order" in m;
}

/**
 * Những thao tác được phép trên một tầng.
 *
 * ĐÓNG KHI KHÔNG CHẮC. Một số tầng ngoài 1·2·3 — `null` vì dòng không phát ra `tier`, hay một
 * tầng 4 mà một bản migration sau này đặt ra — trả về KHÔNG thao tác nào, chứ không rơi vào
 * nhánh rộng nhất. Chiều sai an toàn ở đây là thiếu một nút (cán bộ hỏi, người khác trả lời
 * được); chiều sai còn lại là mời người ta bấm vào một thao tác mà quy tắc ba tầng vừa dựng lên
 * để chặn (luật 1, cấm #1: không có giá trị mặc định trên đường cách ly).
 *
 * TẦNG 3 GIỮ ĐƯỢC `doiNhan`, VÀ ĐÓ LÀ CHỦ Ý CHỨ KHÔNG PHẢI SÓT: nhãn trên màn hình là chữ của
 * đơn vị, mã trong mã nguồn thì không. Đổi nhãn là thao tác DUY NHẤT còn lại ở tầng 3, và bỏ
 * nốt nó đi là lấy mất của đơn vị quyền gọi tên việc của chính họ
 * (`service-documents/internal/domain/danh_muc_ba_tang.go:51-54`).
 */
export function thaoTacTheoTang(tang: number | null): ThaoTacChoPhep {
  switch (tang) {
    case TANG_DON_VI:
      return { doiNhan: true, tat: true, xoa: true };
    case TANG_HE_THONG:
      return { doiNhan: true, tat: true, xoa: false };
    case TANG_RE_NHANH:
      return { doiNhan: true, tat: false, xoa: false };
    default:
      return KHONG_THAO_TAC;
  }
}

/** Thao tác được phép trên một mục. Mục không nói được tầng của nó thì không có thao tác nào. */
export function thaoTacCuaMuc(m: MucDanhMuc): ThaoTacChoPhep {
  return laMucGhi(m) ? thaoTacTheoTang(m.tier) : KHONG_THAO_TAC;
}

/**
 * BẬT LẠI MỘT MỤC ĐÃ TẮT KHÔNG PHẢI CÙNG MỘT CÂU HỎI VỚI TẮT NÓ ĐI, nên nó có hàm riêng.
 *
 * Trigger chỉ từ chối chiều `true -> false` ở tầng 3 — chiều ngược lại luôn được phép, và phải
 * được phép: một dòng tầng 3 vì lý do nào đó đang tắt thì không còn đường nào quay lại nếu màn
 * hình cũng giấu nốt nút bật (`danh_muc_ba_tang.go:114-121`). Vì vậy nút `Bật lại` vẽ ở MỌI
 * tầng, còn nút `Tắt` thì theo `thaoTacTheoTang`.
 */
export function choBatLai(m: MucDanhMuc): boolean {
  return laMucGhi(m) && !m.active;
}

/** Kết quả kiểm một ô nhập: giá trị đã chuẩn hoá, hoặc một câu cho người dùng đọc. */
export type KiemO = { ok: true; giaTri: string } | { ok: false; loi: string };

/**
 * Câu hiện ra khi bấm `Xoá` mà chưa nhập lý do.
 *
 * NÓI RA HẬU QUẢ, KHÔNG CHỈ NÓI "BẮT BUỘC". Lý do xoá không phải một ô cho đủ thủ tục: nó được
 * lưu cạnh dòng đã xoá (`delete_reason`) và là câu trả lời duy nhất còn lại vào ngày có người
 * hỏi vì sao một loại văn bản biến mất — mà dòng ấy vẫn còn đó, nên câu hỏi SẼ được hỏi (luật
 * 7, bất biến 1).
 */
export const LOI_THIEU_LY_DO_XOA =
  "Vui lòng nhập lý do xoá. Lý do được lưu cùng mục đã xoá để sau này trả lời được câu hỏi vì " +
  "sao mục này không còn trong danh mục.";

/**
 * Kiểm lý do xoá TRƯỚC KHI gửi đi.
 *
 * ĐÂY LÀ PHÉP KIỂM DUY NHẤT MÀN HÌNH TỰ LÀM, và sự tiết chế ấy có lý do. Máy chủ đã kiểm khuôn
 * mã, độ dài nhãn, khoảng thứ tự và độ dài lý do, mỗi thứ kèm một câu tiếng Việt nói rõ phải
 * sửa gì (`service-documents/internal/domain/danh_muc_ba_tang.go:126-240`). Chép những phép ấy
 * xuống client là dựng bản sao thứ hai của một bộ quy tắc nghiệp vụ, và bản sao ấy trôi mà
 * không bài test nào đỏ (luật 9, cấm #2).
 *
 * Lý do xoá là ngoại lệ vì nó là ô DUY NHẤT mà một lần gửi hỏng để lại hậu quả khó chịu hơn một
 * thông báo lỗi: cán bộ đã bấm `Xoá`, đã xác nhận, và nhận về một 400 — trong khi cái sai chỉ
 * là một ô trống mà màn hình biết thừa từ trước khi gửi.
 *
 * `trim()` chứ không phải `=== ""`: một ô chỉ có dấu cách qua được phép kiểm rỗng ngây thơ, và
 * máy chủ cũng cắt trắng rồi từ chối — tức là màn hình sẽ nói "đã gửi" cho một thao tác hỏng.
 */
export function kiemLyDoXoa(lyDo: string): KiemO {
  const sach = lyDo.trim();
  return sach === "" ? { ok: false, loi: LOI_THIEU_LY_DO_XOA } : { ok: true, giaTri: sach };
}
