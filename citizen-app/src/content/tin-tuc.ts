/**
 * HAI BÀI MỚI NHẤT TRÊN TRANG TIN CỦA ViHAT GROUP — **ẢNH CHỤP**, KHÔNG PHẢI MỘT LỜI GỌI MẠNG.
 *
 * ⚠ ĐÂY LÀ CHỖ DỄ VIẾT SAI NHẤT TRONG CẢ TÍNH NĂNG NÀY, NÊN NÓ ĐƯỢC NÓI RA TRƯỚC MỌI THỨ KHÁC:
 *
 *   Cách "đúng" theo phản xạ là gọi `vihatgroup.com/wp-json/wp/v2/posts` lúc chạy để tin luôn
 *   mới. Ứng dụng này KHÔNG được làm thế, và lý do thứ hai mới là lý do thật:
 *
 *   1. Dây bẫy trong `phase1-collects-nothing.test.ts` chỉ miễn lệnh cấm `fetch` cho các tệp kê
 *      trong `TEP_GOI_MANG` — lời gọi tới máy chủ của app và của ViGov, không tới trang tin. Thêm
 *      một lời gọi tới trang tin là nới đúng dây bẫy ấy.
 *   2. **Thêm một đích mạng là SỬA MỘT LỜI KHAI TRONG CHÍNH SÁCH QUYỀN RIÊNG TƯ** — văn bản đang
 *      nói ứng dụng chỉ gửi đi đúng một việc, và một lời gọi tới máy chủ của trang tin để lộ địa
 *      chỉ IP của người dùng cho một bên nữa, ngay khi họ mở màn hình chủ, mà họ không bấm gì.
 *      Đó là một thay đổi pháp lý, không phải một tính năng.
 *
 *   Nên tin ở đây là chữ NẰM SẴN TRONG BUNDLE. Ứng dụng không biết trang tin có gì mới, và màn
 *   hình NÓI RA điều đó kèm ngày chụp, thay vì để người đọc tưởng đây là tin trực tiếp.
 *
 * ⚠ CÁCH CẬP NHẬT — ba bước, không có bước nào tự động:
 *
 *   1. Mở `https://vihatgroup.com/wp-json/wp/v2/posts` (hoặc chính trang tin) và đọc hai bài mới
 *      nhất: ngày đăng, tiêu đề NGUYÊN VĂN, một đoạn trích, đường dẫn.
 *   2. Thay hai mục trong `TIN_VIHAT` dưới đây và đổi `NGAY_CHUP_TIN` thành ngày bạn vừa đọc.
 *   3. `npm test` — có ca canh ngày chụp phải hiện trên màn hình, và canh mỗi bài có đủ bốn phần.
 *
 * ⚠ TIÊU ĐỀ VÀ ĐOẠN TRÍCH LÀ CHỮ CỦA NGƯỜI KHÁC, CHÉP NGUYÊN VĂN. Rút gọn hay "viết cho gọn" một
 * tiêu đề đã đăng là in ra một câu trang tin không hề đăng, dưới tên chính pháp nhân ấy.
 */

export type BaiViet = {
  /** Khoá React và mỏ neo cho test. Không hiện ra. */
  ma: string;
  /** Ngày đăng, dạng ISO `YYYY-MM-DD` — dạng SẮP XẾP được. Dạng đọc do `ngayDoc` in ra. */
  ngay: string;
  /** Tiêu đề NGUYÊN VĂN như trang tin đăng. */
  tieu_de: string;
  /** Đoạn trích NGUYÊN VĂN. Kết thúc bằng "…" khi trang tin cắt nó; trang tin đăng trọn câu thì giữ trọn câu. */
  trich: string;
  /** Đường dẫn bài viết. Mở qua `moRaNgoai("tin-tuc", …)`, xem `content/dich-ra-ngoai.ts`. */
  duong_dan: string;
};

/** Ngày đọc trang tin. Hiện LÊN MÀN HÌNH — người đọc phải tự biết chỗ này cũ hay mới. */
export const NGAY_CHUP_TIN = "14/10/2026";

/*
 * Bản chụp 14/10/2026: `GET https://vihatgroup.com/wp-json/wp/v2/posts?per_page=4` — hai bài mới nhất đăng
 * 08/10 và 07/10/2026. Tiêu đề và đoạn trích chép nguyên văn từ `title.rendered` / `excerpt.rendered`; trang
 * tin cắt đoạn trích của bài 07/10 bằng "[…]", ở đây là "…". Đoạn trích bài 08/10 trang tin đăng trọn câu, nên
 * không có "…".
 */
export const TIN_VIHAT: readonly BaiViet[] = [
  {
    ma: "omi-buber-connect-lao",
    ngay: "2026-10-08",
    tieu_de: "ViHAT Group mở rộng sang Lào: OMI JSC hợp tác Buber Connect, được VnExpress đưa tin",
    trich:
      "VnExpress đưa tin OMI JSC – thành viên ViHAT Group – hợp tác Buber Connect, đưa tổng đài đám mây OMICall đến Lào. Thêm một bước trên hành trình Đông Nam Á của hệ sinh thái ViHAT.",
    duong_dan: "https://vihatgroup.com/hoat-dong/vihat-group-omi-jsc-hop-tac-buber-connect-lao/",
  },
  {
    ma: "chuyen-doi-so-xa-thang-binh",
    ngay: "2026-10-07",
    tieu_de: "ViHAT Group đồng hành cùng xã Thăng Bình trên lộ trình chuyển đổi số cơ sở",
    trich:
      "Chiều ngày 21/08/2026, đoàn công tác ViHAT Group đã có buổi làm việc với lãnh đạo xã Thăng Bình, TP. Đà Nẵng, nhằm giới thiệu giải pháp công nghệ và khảo sát thực trạng chuyển đổi số tại địa phương. Buổi trao đổi tập trung vào hai trụ cột hành chính số và công dân…",
    duong_dan: "https://vihatgroup.com/hoat-dong/vihat-group-chuyen-doi-so-xa-thang-binh/",
  },
];

/**
 * Ngày ISO đọc thành ngày Việt.
 *
 * MỘT HÀM THUẦN TRÊN CHUỖI, KHÔNG QUA `new Date(...)`: `new Date("2026-07-20")` phân giải theo
 * UTC rồi in ra theo múi giờ của MÁY, nên trên một máy ở UTC-5 nó lùi thành 19/07. Ngày đăng của
 * một bài viết là một nhãn, không phải một thời điểm — không có gì phải đổi múi giờ.
 *
 * Chuỗi không đúng dạng thì trả nguyên văn: một nhãn ngày trông lạ vẫn đọc được, còn một chuỗi
 * rỗng hay một `Invalid Date` trên màn hình thì không nói được gì.
 */
export function ngayDoc(iso: string): string {
  const khop = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (khop === null) return iso;
  return `${khop[3]}/${khop[2]}/${khop[1]}`;
}

/**
 * Câu nói ra rằng đây là ảnh chụp, hiện ngay dưới hai bài.
 *
 * Nó không phải một lời xin lỗi vì "chưa làm kịp phần tự động": nó là lời khai giữ cho câu mở đầu
 * của chính sách quyền riêng tư — *"chỉ gửi đi đúng MỘT việc"* — vẫn đúng từng chữ khi màn hình
 * chủ có một khối tin tức trên đó.
 */
export const GHI_CHU_TIN = `Hai tin trên là bản chụp ngày ${NGAY_CHUP_TIN} từ trang tin của chúng tôi. Ứng dụng không tự tải tin mới: mở bài viết là mở trang tin trong Zalo, và đó là lúc duy nhất có một lời gọi ra ngoài.`;

/** Nhãn nút mở một bài. Đứng ở đây để `bundle-for-zalo.test.ts` đọc lại đúng chuỗi màn hình vẽ. */
export const NUT_MO_BAI = "Đọc bài này";

export const TIEU_DE_KHOI_TIN = "Tin ViHAT";
