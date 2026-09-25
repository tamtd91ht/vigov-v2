import { NextResponse, type NextRequest } from "next/server";

import { SESSION_COOKIE } from "@/lib/session";

/**
 * Chặn đường ở **máy chủ**, trước khi bất kỳ trang nào được dựng.
 *
 * TÊN TỆP: đây chính là "middleware" mà README của ứng dụng và `skills/nextjs-multi-tenant`
 * nói tới. Next.js 16 đổi tên quy ước tệp `middleware.ts` thành `proxy.ts` và báo lỗi thời cho
 * tên cũ; chỉ có cái tên đổi, vị trí trong vòng đời yêu cầu thì không. Dựng tiếp trên một quy
 * ước đã bị khai tử là nợ có hạn.
 *
 * VÌ SAO Ở ĐÂY CHỨ KHÔNG PHẢI Ở NÚT BẤM: ẩn một mục menu là trải nghiệm người dùng, không
 * phải an ninh — mã client sửa được (luật 5, cấm #1). Middleware chạy trước mọi thứ và không
 * có bản sao nào của nó trong trình duyệt.
 *
 * ĐIỀU MIDDLEWARE NÀY CỐ Ý **KHÔNG** LÀM:
 *
 *   1. Nó không xác thực token. Nó chỉ thấy cookie có mặt hay không. Chữ ký, hạn dùng, `sid`
 *      còn trong sổ phiên hay đã bị thu hồi, `tenant_id` trong token có khớp `Host` không —
 *      tất cả do dịch vụ identity kiểm trên từng yêu cầu thật. Một cookie hết hạn lọt qua đây
 *      chỉ đổi lấy một 401 ở lời gọi kế tiếp, không đổi lấy dữ liệu.
 *   2. Nó không gắn thêm header xã nào. Xã suy từ `Host` ở rìa ngoài cùng; reverse proxy xoá
 *      sạch mọi header tenant từ ngoài vào, và ứng dụng web không phải nơi sinh ra header ấy
 *      (luật 1, cấm #2).
 *   3. Nó không đọc nội dung cookie. Cookie là httpOnly và do máy chủ identity đặt; ở đây chỉ
 *      hỏi "có hay không".
 */

/** Những đường công khai. Danh sách trắng, không phải danh sách đen: thiếu khai báo là chặn. */
const CONG_KHAI = new Set<string>(["/dang-nhap"]);

export default function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl;

  if (CONG_KHAI.has(pathname)) return NextResponse.next();

  if (request.cookies.has(SESSION_COOKIE)) return NextResponse.next();

  // `tiep-tuc` luôn là đường dẫn nội bộ, dựng từ chính yêu cầu này — không bao giờ từ tham số
  // người dùng gửi lên. Phía nhận vẫn kiểm lại (features/auth/form-dang-nhap.tsx).
  const toi = request.nextUrl.clone();
  toi.pathname = "/dang-nhap";
  toi.search = "";
  toi.searchParams.set("tiep-tuc", `${pathname}${search}`);
  return NextResponse.redirect(toi);
}

export const config = {
  // Mọi đường trừ tài nguyên tĩnh của Next, favicon, `/api/` và `/healthz`.
  //
  // `/api/` ĐI QUA ỨNG DỤNG NÀY (app/api/v1/[...duong]/route.ts chuyển tiếp tới dịch vụ Go sở
  // hữu tuyến) nhưng CỐ Ý không đi qua middleware: dịch vụ Go tự xác thực và tự kiểm quyền trên
  // TỪNG lời gọi, nên ở đây không có gì để chặn. Chuyển hướng một lời gọi API thiếu cookie về
  // `/dang-nhap` là sai — máy khách chờ một 401 JSON, và chính cú 307 ấy từng làm
  // `resolveTenant` ném lỗi, kéo `/dang-nhap` thành 500. Loại ở `matcher` chứ không `return`
  // sớm trong hàm, vì đường nào khớp middleware thì Next chép thân yêu cầu và CẮT nó ở
  // `proxyClientMaxBodySize`, mặc định 10MB (next/dist/server/body-streams.js:93-105) — tệp
  // đính kèm tới 25MB sẽ tới dịch vụ Go thiếu đuôi.
  //
  // `/healthz` là đầu dò k8s, gọi bằng IP pod: không cookie, không xã nào.
  matcher: ["/((?!api/|healthz$|_next/static|_next/image|favicon.ico).*)"],
};
