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
 * TUYẾN ĐỌC CẤU HÌNH XÃ NAY ĐÃ CÓ: `GET /api/v1/commune` (`kb/20-contracts/openapi.json`),
 * công khai, trả tên xã và tỉnh/thành ứng với `Host`. `resolveTenant` gọi đúng tuyến ấy —
 * vẫn không có danh bạ xã nào nằm trong mã web, và vẫn không có xã mặc định nào.
 */
// `cache` gói ở ngoài để nhiều component trong CÙNG một yêu cầu chỉ suy ra xã một lần. Đây là
// khử trùng lặp trong phạm vi một yêu cầu, không phải bộ nhớ đệm giữa các yêu cầu — một bộ đệm
// sống lâu hơn yêu cầu mà lỡ dùng chung giữa hai host là đúng hình dạng của một vụ rò dữ liệu
// giữa hai xã.
export const layCauHinhXa = cache(async (): Promise<TenantConfig> => {
  const host = (await headers()).get("host") ?? "";
  if (host === "") notFound();

  // MỘT PHÉP KIỂM, KHÔNG PHẢI HAI. Trước đây ở đây còn một dòng `if (!cauHinh.active)
  // notFound()`, và nay nó biến mất cùng với trường `active`. KHÔNG phải vì "xã đã ngừng hoạt
  // động" thôi là một trạng thái thật — nó vẫn là (luật 7, bất biến 6: sáp nhập thì đánh dấu
  // ngừng, dữ liệu giữ nguyên) — mà vì phép kiểm ấy không còn chỗ nào để đứng:
  //
  //   Biên trả 404 cho xã đã ngừng hoạt động TRƯỚC khi bất kỳ handler nào chạy
  //   (`core/httpx/edge.go:25`), và trả **cùng một** 404 cho host chưa bao giờ là xã. Nên
  //   `resolveTenant` nhận đúng một câu "không" cho cả hai, trả `null`, và dòng dưới đây đã
  //   bao hết. Giữ thêm một cờ `active` ở phía web là giữ một nhánh KHÔNG BAO GIỜ CHẠY —
  //   nhìn như đã lo cho xã sáp nhập, trong khi chẳng lo gì cả, và không test nào đỏ.
  //
  // Muốn xã đã sáp nhập nhận một câu trả lời khác 404 trống thì phải sửa ở biên, chỗ mọi dịch
  // vụ dùng chung — không phải dựng lại ở đây một nhánh không có dữ liệu để chạy.
  const cauHinh = await resolveTenant(host);
  if (cauHinh === null) notFound();

  return cauHinh;
});
