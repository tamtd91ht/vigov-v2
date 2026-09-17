import "server-only";

import { cache } from "react";

import { headers } from "next/headers";
import { notFound } from "next/navigation";

import { resolveTenant, type TenantConfig } from "./tenant-config";

/**
 * Suy ra xã cho yêu cầu đang phục vụ, **ở phía máy chủ**, từ `Host`.
 *
 * `import "server-only"` ở dòng đầu không phải trang trí: nó khiến việc lỡ tay nhập tệp này
 * vào một component client **hỏng lúc build** chứ không phải hỏng lúc chạy ở máy người dùng.
 * Cấu hình xã phải được quyết ở máy chủ; một bản sao quyết ở trình duyệt là một bản sao người
 * ngoài sửa được.
 *
 * `Host` không khớp xã nào → **404**, không phải 400, không rơi về xã mặc định, và không tiết
 * lộ xã nào tồn tại (luật 1, bất biến 3; `kb/00-foundation/multi-tenant-model.md` §Ranh giới
 * tin cậy).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐANG VƯỚNG HỢP ĐỒNG — đọc trước khi sửa:
 *
 *   `resolveTenant` trong `tenant-config.ts` còn là khung và ném lỗi. Nó chưa gọi được đi đâu
 *   cả: hợp đồng REST (`kb/20-contracts/openapi.json`) hiện chỉ có hai route phiên làm việc,
 *   KHÔNG có route nào trả cấu hình công khai của xã theo `Host`. Dịch vụ platform mới chỉ
 *   phơi ra gRPC (`platform/internal/grpc`), và route HTTP `GET /api/v1/communes`
 *   trong `platform/internal/http/routes.go` vẫn đang là chú thích "chưa chốt".
 *
 *   Ở đây CỐ Ý KHÔNG bịa một endpoint, KHÔNG nhét một danh bạ xã tạm vào mã, và KHÔNG dựng
 *   một xã mặc định để "chạy tạm". Cả ba đều làm màn hình trông như đã xong trong khi cách ly
 *   giữa hai cơ quan nhà nước chưa hề được kiểm. Ứng dụng từ chối phục vụ cho tới khi route
 *   ấy tồn tại — đó là "fail closed", không phải thiếu sót.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
// `cache` gói ở ngoài để nhiều component trong CÙNG một yêu cầu chỉ suy ra xã một lần. Đây là
// khử trùng lặp trong phạm vi một yêu cầu, không phải bộ nhớ đệm giữa các yêu cầu — một bộ đệm
// sống lâu hơn yêu cầu mà lỡ dùng chung giữa hai host là đúng hình dạng của một vụ rò dữ liệu
// giữa hai xã.
export const layCauHinhXa = cache(async (): Promise<TenantConfig> => {
  const host = (await headers()).get("host") ?? "";
  if (host === "") notFound();

  const cauHinh = await resolveTenant(host);
  if (cauHinh === null) notFound();

  // Xã đã sáp nhập/giải thể thì không phục vụ tiếp, nhưng dữ liệu vẫn còn nguyên (luật 7,
  // bất biến 6). Cùng một 404: ai gõ vào host này không cần biết xã ấy từng tồn tại.
  if (!cauHinh.active) notFound();

  return cauHinh;
});
