/**
 * `src/cong-dan/` — NỬA NHÀ NƯỚC của Mini App. Cửa duy nhất, và hôm nay còn rỗng, có chủ đích.
 *
 * Một App ID, một bundle, một origin — HAI NỬA NGHIỆP VỤ:
 *
 *   Thương mại   `src/content/` · `features/company-intro/` · `features/tinh-nang/`
 *                `features/dang-nhap/`            -> khách hàng doanh nghiệp của ViHAT Group
 *   Nhà nước     `src/cong-dan/` (thư mục này)    -> công dân làm việc với xã/phường
 *
 * Thư mục này được dựng TRƯỚC khi có tệp đầu tiên, vì ba ràng buộc dưới đây chỉ viết được một
 * lần: sau khi nửa này có mã, mỗi ràng buộc trở thành một cuộc dọn dẹp thay vì một dòng khai báo.
 *
 * VÌ SAO MỘT TỆP GIỮ CHỖ CHỨ KHÔNG PHẢI MỘT THƯ MỤC RỖNG:
 *
 *   Ba ràng buộc ấy (`src/ranh-gioi-hai-nua.test.ts`) quét theo ĐƯỜNG DẪN. Một thư mục không có
 *   tệp `.ts` nào thì lượt quét không đọc tới nó, và "được canh" với "không tồn tại" là một —
 *   đúng chế độ hỏng mà `phase1-collects-nothing.test.ts` đã ghi lại hai lần. Tệp này làm nửa nhà
 *   nước có mặt thật trong lượt quét từ trước khi nó có nghiệp vụ.
 *
 * BA RÀNG BUỘC, MỖI CÁI LÀ MỘT CA TEST — không phải một quy ước trong tài liệu, vì một quy ước
 * không ai đo được là một quy ước sẽ trôi:
 *
 *   1. RANH GIỚI MỘT CHIỀU, HAI CHIỀU ĐỀU CẤM. Nửa thương mại không nhập tệp của nửa nhà nước;
 *      nửa nhà nước không nhập tệp của nửa thương mại. Và nặng nhất: nửa thương mại KHÔNG ĐƯỢC
 *      NHẬP CLIENT API CỦA VIGOV — phiên công dân không bao giờ được với tới từ nửa thương mại.
 *
 *   2. KHÔNG LƯU TRỮ ĐỊNH DANH. Một bundle là một origin, nên `localStorage` · `sessionStorage` ·
 *      `IndexedDB` là CHUNG giữa hai nửa theo đúng cấu tạo. Không phiên, không số điện thoại,
 *      không xã đã chọn được ghi xuống máy — ở cả hai nửa.
 *
 *   3. MỖI LỜI GỌI SDK KHAI MỤC ĐÍCH TẠI CHỖ (`KHAI_BAO_LOI_GOI` trong
 *      `features/tinh-nang/zalo-api.ts`). Quyền cấp theo App ID, nên nửa này THỪA HƯỞNG mọi quyền
 *      nửa thương mại xin được; khai tại chỗ làm điều đó nhìn thấy được, thay vì là một hệ quả
 *      nền tảng không ai nói ra.
 *
 * TỆP ĐẦU TIÊN ĐẶT VÀO ĐÂY PHẢI TRẢ LỜI TRƯỚC — luật 4 (cách ly công dân) và luật 1 (cách ly xã):
 *
 *   · Xã được xác định bằng đường nào (deep link · hồ sơ · GPS GỢI Ý · chọn tay), và GPS KHÔNG
 *     BAO GIỜ QUYẾT.
 *   · Danh tính lấy TỪ PHIÊN DO MÁY CHỦ PHÁT HÀNH, không từ tham số của client.
 *   · Tên xã hiện trên MỌI màn, và được xác nhận lại ở BƯỚC CUỐI TRƯỚC KHI GỬI.
 *
 * ⚠ KHÔNG TỆP NÀO NHẬP TỆP NÀY, VÀ ĐÓ LÀ ĐÚNG. Nửa thương mại bị CẤM nhập nó (ràng buộc 1), nên
 * nó không đi vào bundle gửi Zalo duyệt. Ngày kênh công dân có màn hình đầu tiên, `App.tsx` —
 * lớp vỏ, không thuộc nửa nào — là chỗ duy nhất được nối hai nửa lại.
 */

/**
 * Nửa nhà nước đã có nghiệp vụ chưa.
 *
 * `false` hôm nay. Khai thành một hằng có kiểu chứ không để thư mục im lặng: một hằng đọc được là
 * thứ một ca test hỏi được, còn một thư mục rỗng thì không trả lời được câu nào.
 */
export const NUA_NHA_NUOC_DA_CO_NGHIEP_VU = false;
