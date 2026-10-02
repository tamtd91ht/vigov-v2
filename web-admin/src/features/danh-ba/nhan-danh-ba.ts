/**
 * Câu chữ và phép quyết định của màn **Danh bạ cán bộ** (`docs/ui-ux/12-danh-ba-can-bo.md`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * TÁCH KHỎI COMPONENT VÌ MỘT LÝ DO ĐÃ ĐO ĐƯỢC, không phải vì gọn: một quyết định nằm trong
 * module thuần kiểm được bằng một phép so chuỗi, còn cùng quyết định ấy viết thẳng trong JSX thì
 * chỉ kiểm được bằng cách kết xuất cả cây. Cùng khuôn với `components/danh-ba/nhan-ghi-danh-ba.ts`.
 *
 * TỪNG PHẦN ĐẶC TẢ CHƯA DỰNG ĐỀU CÓ LÝ DO Ở ĐÂY — `PHAN_CHUA_DUNG` là mô tả sau dấu "?" của
 * phần ấy, đặt ngay trên màn hình chứ không giấu trong chú thích. Một cán bộ mở `/danh-ba` và
 * không thấy một nút mà đặc tả vẽ sẽ kết luận hệ thống hỏng, rồi gọi lên tỉnh; thứ thật sự thiếu
 * thường là một tuyến API hoặc một quyết định của khách.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { KetQua } from "@/lib/api/goi";

/* ---- đầu trang ------------------------------------------------------------------------------ */

/** Tiêu đề trang — nguyên văn `docs/ui-ux/12-danh-ba-can-bo.md:3`. */
export const TIEU_DE_TRANG = "Danh bạ cán bộ";

/**
 * Câu mô tả dưới tiêu đề — MỘT DÒNG (đặc tả giao diện 02/10/2026 §5: dòng phụ ≤ ~80 ký tự).
 *
 * NỬA ĐẦU LÀ NGUYÊN VĂN ĐẶC TẢ, NỬA SAU THÌ KHÔNG — VÀ ĐÓ LÀ CHỦ Ý. Đặc tả (§1, §2) viết:
 * *"Toàn bộ cán bộ của xã. Chọn người cần công khai rồi bấm 'Thêm vào danh bạ Mini App' để bà con
 * gọi được."* Câu dưới nhấn vào điều #12 giữ nguyên kể cả ở khung "Công khai nhiều người" (chốt
 * 30/09/2026): sự đồng ý là của TỪNG người, sau khi hỏi ý — chọn nhiều người không thay được việc ấy.
 * Rút gọn khi làm mới giao diện (ADR 0068) nhưng giữ đủ ba ý ấy: hỏi ý, từng người, đồng ý.
 */
export const MO_TA_TRANG =
  "Toàn bộ cán bộ của xã. Chỉ công khai số lên Mini App khi đã hỏi ý và từng người đồng ý.";

/**
 * Câu hiện khi tài khoản thiếu `admin.user`.
 *
 * NÓI RA TÊN KHOÁ. "Bạn không có quyền" trống trơn là câu khiến cán bộ gọi lên tỉnh hỏi mình
 * thiếu quyền gì; tên khoá là thứ quản trị viên của xã tìm được ngay trên màn Phân quyền.
 */
export const CAU_THIEU_QUYEN =
  "Tài khoản của bạn không có quyền quản lý người dùng (admin.user), nên danh bạ cán bộ không " +
  "hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

/* ---- thẻ KPI -------------------------------------------------------------------------------- */

/** Nhãn thẻ KPI duy nhất dựng được — đặc tả §2 viết `SỐ KHỐI / ĐƠN VỊ`. */
export const NHAN_SO_KHOI = "Số khối / đơn vị";

/**
 * Trạng thái của con số "số khối / đơn vị". BA pha, không hai.
 *
 * "Chưa đọc xong" và "đọc xong, được 0" là hai sự thật khác nhau về một xã: cái sau nghĩa là sơ
 * đồ tổ chức còn trống và người quản trị phải làm gì đó.
 */
export type SoKhoi =
  | { pha: "dangDoc" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; so: number };

/**
 * Đếm số khối / đơn vị từ kết quả đọc `GET /api/v1/org-units`.
 *
 * ĐẾM ĐƯỢC VÌ TUYẾN ẤY TRẢ NGUYÊN DANH SÁCH, KHÔNG PHÂN TRANG (`lib/api/danh-muc.ts`). Hai thẻ KPI
 * kia thì KHÔNG đếm được như thế — xem `PHAN_CHUA_DUNG`.
 */
export function demSoKhoi(kq: KetQua<{ items: readonly unknown[] }> | null): SoKhoi {
  if (kq === null) return { pha: "dangDoc" };
  if (!kq.ok) return { pha: "loi", thongBao: kq.thongBao };
  return { pha: "xong", so: kq.duLieu.items.length };
}

/**
 * Chữ hiện trong thẻ KPI.
 *
 * KHÔNG BAO GIỜ TRẢ CHUỖI RỖNG, và không bao giờ trả `0` cho ca chưa đọc xong: một thẻ hiện số 0
 * trong lúc còn đang đọc là một câu khẳng định sai về sơ đồ tổ chức của một cơ quan nhà nước.
 */
export function nhanSoKhoi(so: SoKhoi): string {
  switch (so.pha) {
    case "dangDoc":
      return "đang đếm…";
    case "loi":
      return "chưa đọc được";
    case "xong":
      return String(so.so);
  }
}

/**
 * Chữ của huy hiệu đếm khối / đơn vị ở đầu trang (đặc tả giao diện §8.1: "N khối / đơn vị").
 *
 * CHỈ GHÉP CON SỐ KHI ĐÃ ĐẾM XONG. Khi còn đọc hay đọc hỏng, huy hiệu nói rõ nhãn và trạng thái
 * (`nhanSoKhoi`) — "đang đếm… khối / đơn vị" là một câu sai ngữ pháp, còn "0 khối / đơn vị" lúc chưa
 * đọc xong là một câu sai sự thật.
 */
export function unitCountText(so: SoKhoi): string {
  return so.pha === "xong" ? `${so.so} khối / đơn vị` : `${NHAN_SO_KHOI}: ${nhanSoKhoi(so)}`;
}

/* ---- trạng thái của vùng danh sách (đặc tả giao diện v2 §8b) --------------------------------- */

/** Tiêu đề khi đọc danh sách hỏng; câu bên dưới là câu NGUYÊN VĂN của máy chủ. */
export const LOAD_FAILED_TITLE = "Chưa tải được danh sách";
/** Nút đọc lại đúng trang đang xem, cùng bộ lọc — cơ chế đọc lại sau mỗi lần ghi. */
export const RELOAD = "Tải lại";
/** Tiêu đề khi đang tìm / lọc mà không ai khớp; câu gợi ý bên dưới là `KHONG_KHOP_LOC`. */
export const NO_MATCH_TITLE = "Không có cán bộ phù hợp";

/** Hai nút phân trang (đặc tả giao diện §8.1: "‹ Trước / Sau ›"); vùng `nav` đã nói là phân trang. */
export const PAGE_PREVIOUS = "Trước";
export const PAGE_NEXT = "Sau";

/* ---- bảng ----------------------------------------------------------------------------------- */

/**
 * Nhãn cột — mỗi nhãn là đúng chữ của đặc tả §4, TRỪ cột điện thoại.
 *
 * ĐẶC TẢ CÓ MỘT CỘT `Di động`; Ở ĐÂY CÓ HAI, VÀ NHÃN NÓI RÕ LOẠI SỐ. Câu mở #16 (chốt 22/09/2026):
 * máy bàn cơ quan là THÔNG TIN CÔNG VỤ, di động cá nhân là DỮ LIỆU CÁ NHÂN theo Nghị định 13. Hai
 * địa vị pháp lý khác nhau nghĩa là hai luật che, hai luật xuất Excel, hai luật công khai ra Mini
 * App — và một nhãn trung tính là chỗ người sắp bấm nút xuất không biết mình đang đụng loại nào.
 */
export const COT_HO_TEN = "Họ và tên";
export const COT_CHUC_VU = "Chức vụ";
export const COT_KHOI = "Khối / đơn vị";
export const COT_MAY_BAN = "Máy bàn cơ quan";
export const COT_DI_DONG = "Di động cá nhân";

/**
 * Nhãn nút sửa — chứa NGUYÊN VĂN chuỗi `15-phu-luc-giao-dien-chung.md §8` yêu cầu giữ.
 *
 * `ariaSua` GẮN THÊM TÊN NGƯỜI vào sau chuỗi ấy chứ không thay nó. Hai mươi dòng cho ra hai mươi
 * nút đọc lên giống hệt nhau là danh sách mà người dùng trình đọc màn hình không chọn đúng được
 * dòng nào — và ở đây chọn nhầm dòng nghĩa là sửa hồ sơ của một cán bộ khác.
 */
export const NUT_SUA_THONG_TIN = "Sửa thông tin cán bộ";

export function ariaSua(hoTen: string): string {
  return `${NUT_SUA_THONG_TIN}: ${hoTen}`;
}

/**
 * Chữ HIỆN của nút sửa trên thẻ điện thoại. Nhãn trợ năng vẫn là `ariaSua` — và vì chữ hiện là
 * phần ĐẦU của nhãn ấy, người dùng điều khiển bằng giọng nói nói "Sửa" là trúng nút (WCAG 2.5.3).
 */
export const EDIT_SHORT = "Sửa";

/** Nhãn trợ năng (và `title`) của nút "⋯" trên một dòng — gọi tên người, cùng lý do với `ariaSua`. */
export function ariaMoreActions(hoTen: string): string {
  return `Thao tác khác: ${hoTen}`;
}

/**
 * Câu cho một danh bạ rỗng.
 *
 * TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI (`15-phu-luc §6`): một xã vừa onboard có danh bạ
 * rỗng thật, và máy chủ trả `items: []` chứ không bao giờ trả `null`.
 */
export const DANH_BA_RONG =
  "Đơn vị chưa có cán bộ nào trong danh bạ. Khi cán bộ được thêm vào, danh sách sẽ hiện ở đây.";

/**
 * Câu cho một danh sách rỗng KHI ĐANG TÌM HOẶC LỌC.
 *
 * KHÁC `DANH_BA_RONG`, VÀ PHẢI KHÁC: "đơn vị chưa có cán bộ nào" in ra dưới một ô tìm vừa gõ sai
 * chính tả là một câu khẳng định sai về cả cơ quan. Câu này nói đúng điều đã xảy ra — không ai khớp
 * điều kiện — và KHÔNG nhắc lại chữ đã tìm (thường là họ tên hoặc số điện thoại).
 */
export const KHONG_KHOP_LOC =
  "Không có cán bộ nào khớp điều kiện tìm kiếm hoặc bộ lọc đang chọn. Thử bỏ bớt điều kiện.";

/**
 * Câu nói rõ số di động ở màn này KHÔNG bị che, và vì sao.
 *
 * ĐÂY LÀ QUYẾT ĐỊNH #11, CHỐT 22/09/2026: không che trong nội bộ xã — cán bộ cùng xã cần gọi nhau
 * để làm việc, che thì họ truyền số qua kênh riêng và hệ thống mất cả vết lẫn quyền kiểm soát.
 * Phạm vi vẫn đóng chặt vì kho buộc `tenant_id`. Quyết định ấy CHỈ nói về màn hình nội bộ: bản
 * xuất Excel và mọi đường ra ngoài cơ quan VẪN CHE (luật 3, bất biến 4).
 *
 * Câu này phải có mặt trên màn hình vì người đọc cần biết mình đang nhìn dữ liệu cá nhân chưa che
 * của đồng nghiệp — không phải một bản đã che sẵn mà họ được phép chụp lại và gửi đi.
 */
export const GHI_CHU_SO_DIEN_THOAI =
  "Số di động cá nhân hiện đầy đủ cho cán bộ trong cùng đơn vị để liên hệ công việc. Đây là dữ " +
  "liệu cá nhân theo Nghị định 13/2023/NĐ-CP: không sao chép ra ngoài cơ quan và không công khai " +
  "khi chưa có sự đồng ý của chính người đó.";

/* ---- những phần đặc tả vẽ mà màn này KHÔNG dựng --------------------------------------------- */

/**
 * Một mục của danh sách "đặc tả có, ở đây không". `viSao` phải nói cả CÁI GÌ MỞ KHOÁ nó.
 *
 * CÙNG TÊN HẰNG, CÙNG HAI KHOÁ `ten` / `viSao` như các màn khác, vì `tools/tien_do_san_pham.py` đếm
 * các dòng `ten: "` NẰM TRONG khối khai báo `PHAN_CHUA_DUNG` (tới dấu `];` đầu dòng). Đặt tên khác là
 * báo cáo tiến độ in "không khai" trong khi màn vẫn hiện đủ các dấu "?".
 */
export type PhanChuaDung = { readonly ten: string; readonly viSao: string };

/**
 * Những phần đặc tả vẽ mà hợp đồng chưa cho phép dựng.
 *
 * MỖI MỤC RA TỚI MÀN HÌNH Ở ĐÚNG CHỖ ĐẶC TẢ ĐẶT NÓ (ADR 0068 §14): hai thẻ KPI ở đầu màn
 * (`pending-staff-kpis.tsx`), cột Ảnh đại diện trong bảng (`bang-lien-he.tsx`) — mỗi chỗ là control nó
 * sẽ là, bị vô hiệu, mang dấu "?"; bấm "?" đọc đúng mục ở đây. Khối gập "N phần chưa mở" cuối màn đã
 * bỏ. Dựng xong một phần thì XOÁ dòng của nó ở đây — một dòng "chưa mở" cho thứ đã mở là một câu sai
 * trên màn hình.
 *
 * KHÔNG DÒNG NÀO Ở ĐÂY LÀ "CHƯA LÀM TỚI": mỗi dòng thiếu một tuyến API hoặc một trường trong hợp đồng.
 * "Nhập từ Excel" đã ra khỏi danh sách: nhập cán bộ từ Excel đã dựng ở Cấu hình (ADR 0059).
 */
export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  // The directory route is paginated and deliberately returns no total. Counting the rows of the open
  // page and calling it the total would report a figure nobody computed.
  {
    ten: "Tổng số cán bộ",
    viSao:
      "Danh bạ được tải theo từng trang và hệ thống chưa đếm tổng số cán bộ, nên chưa có con số " +
      "này. Lấy số dòng của trang đang mở làm tổng sẽ ra một con số sai.",
  },
  // No route returns how many staff are shown on the Mini App, commune-wide or per block — which is
  // also why the block filter shows block names only.
  {
    ten: "Đang hiện trên Mini App",
    viSao:
      "Hệ thống chưa đếm số cán bộ đang hiện trên Mini App, của cả xã lẫn của từng khối. Muốn biết " +
      "ai đang hiện, dùng bộ lọc “Đang hiện trên Mini App”.",
  },
  // The staff record has no photo field, and there is no upload purpose / route for a staff photo.
  {
    ten: "Ảnh đại diện",
    viSao:
      "Hồ sơ cán bộ chưa lưu được ảnh, nên bảng và biểu mẫu chưa có ảnh. Danh bạ hiện chữ cái đầu " +
      "của họ tên thay cho ảnh.",
  },
];

/** One entry by its `ten`. Throws on an unknown name — a "?" with no description is never drawn. */
export function pendingPart(ten: string): PhanChuaDung {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === ten);
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry "${ten}"`);
  return found;
}
