/**
 * HAI BÀI MỚI NHẤT TRÊN TRANG TIN CỦA ViHAT GROUP — **ẢNH CHỤP**, KHÔNG PHẢI MỘT LỜI GỌI MẠNG.
 *
 * ⚠ ĐÂY LÀ CHỖ DỄ VIẾT SAI NHẤT TRONG CẢ TÍNH NĂNG NÀY, NÊN NÓ ĐƯỢC NÓI RA TRƯỚC MỌI THỨ KHÁC:
 *
 *   Cách "đúng" theo phản xạ là gọi `vihatgroup.com/wp-json/wp/v2/posts` lúc chạy để tin luôn
 *   mới. Ứng dụng này KHÔNG được làm thế, và lý do thứ hai mới là lý do thật:
 *
 *   1. Giai đoạn 1 có ĐÚNG MỘT đích mạng — tuyến phát hành phiên đăng nhập — và dây bẫy trong
 *      `phase1-collects-nothing.test.ts` miễn lệnh cấm `fetch` cho ĐÚNG MỘT tệp
 *      (`features/dang-nhap/goi-may-chu.ts`). Thêm một lời gọi là nới đúng dây bẫy ấy.
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
  /** Đoạn trích NGUYÊN VĂN, kết thúc bằng "…" vì nó là một đoạn bị cắt. */
  trich: string;
  /** Đường dẫn bài viết. Mở qua `moRaNgoai("tin-tuc", …)`, xem `content/dich-ra-ngoai.ts`. */
  duong_dan: string;
};

/** Ngày đọc trang tin. Hiện LÊN MÀN HÌNH — người đọc phải tự biết chỗ này cũ hay mới. */
export const NGAY_CHUP_TIN = "21/09/2026";

export const TIN_VIHAT: readonly BaiViet[] = [
  {
    ma: "ha-noi-8-nam",
    ngay: "2026-07-20",
    tieu_de: "ViHAT Hà Nội mừng 8 năm thành lập – Tri ân khách hàng với loạt ưu đãi đặc biệt",
    trich:
      "Nhân dịp kỷ niệm 8 năm thành lập, ViHAT Hà Nội triển khai chương trình khuyến mãi đặc biệt dành cho khách hàng đăng ký, gia hạn hoặc nạp tiền sử dụng các sản phẩm, giải pháp do Chi nhánh ViHAT Hà Nội…",
    duong_dan:
      "https://vihatgroup.com/tin-uu-dai/vihat-ha-noi-mung-8-nam-thanh-lap-tri-an-khach-hang-voi-loat-uu-dai-dac-biet/",
  },
  {
    ma: "open-days",
    ngay: "2026-06-02",
    tieu_de: "OPEN DAYS – KHI CHÚNG TA KHÔNG CHỈ LÀ ĐỒNG NGHIỆP",
    trich:
      "Với định hướng xây dựng một môi trường làm việc gắn kết, tích cực và lấy con người làm trung tâm, ViHAT Group chính thức triển khai chương trình nội bộ Open Days trên toàn hệ thống từ tháng 6/2026…",
    duong_dan: "https://vihatgroup.com/hoat-dong/open-days-khi-chung-ta-khong-chi-la-dong-nghiep/",
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
