/**
 * Danh sách thôn / tổ dân phố của xã — `GET /api/v1/residential-units`.
 *
 * TỆP RIÊNG, KHÔNG NẰM CHUNG VỚI BẢY DANH MỤC Ở `danh-muc-nghiep-vu.ts`, vì đây KHÔNG phải một
 * danh mục. Hợp đồng nói ra sự khác nhau ấy bằng chính tên trường: một mục danh mục có `label`
 * — một nhãn đặt lên một mã — còn một thôn có `name`, vì nó là một thứ CÓ TÊN, một địa bàn có
 * thật với số hộ và nhân khẩu (`service-identity/internal/http/thon_to_dan_pho.go`). Đặc tả
 * cũng tách đôi: thôn/tổ dân phố là tab §2, danh mục là tab §5.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY. Quy tắc chung cho mọi lời gọi nằm ở `goi.ts`; quy tắc
 * "không đệm ở mức module" và lý do của nó nằm ở `danh-muc-nghiep-vu.ts`.
 *
 * KHÔNG CÓ HÀM GHI NÀO. Lập một thôn, nhập hai thôn làm một, hay cho một thôn ngừng hoạt động
 * là những hành vi hành chính tác động lên một bản ghi mà dữ liệu nghiệp vụ của xã đang trỏ
 * tới; ai được làm, và phản ánh cùng hồ sơ hộ đang trỏ vào một địa bàn biến mất thì ra sao, là
 * câu chưa ai trả lời (chú thích ngay trên tuyến, cùng tệp Go nêu trên).
 */

import { docJSON, type KetQua } from "./goi";
import type {
  identity_danhSachThonToDanPhoRa,
  identity_get_residential_units,
} from "./schema.gen";

/**
 * GET /api/v1/residential-units — nguyên danh sách địa bàn của xã, kèm nhãn loại đơn vị.
 *
 * KHÔNG PHÂN TRANG: tuyến trả cả danh sách hoặc hỏng. Khi danh sách vượt trần, máy chủ TỪ CHỐI
 * thay vì cắt bớt — một danh sách ngắn đi lặng lẽ sẽ khiến phản ánh bị lập cho sai địa bàn
 * trong khi màn hình trông hoàn toàn bình thường.
 *
 * NHÃN LOẠI VỀ SẴN TRONG CÙNG PHẢN HỒI (`type_code` + `type_label`), nên màn hình này KHÔNG
 * ghép tay với `GET /api/v1/residential-unit-types`: máy chủ đã nối trong một câu truy vấn. Ghép
 * lại ở đây là dựng câu trả lời thứ hai cho cùng một câu hỏi, và hai câu ấy sẽ lệch nhau.
 */
export function layDanhSachThonToDanPho(): Promise<KetQua<identity_danhSachThonToDanPhoRa>> {
  const duongDan: identity_get_residential_units["duongDan"] = "/api/v1/residential-units";
  return docJSON<identity_danhSachThonToDanPhoRa>(duongDan);
}
