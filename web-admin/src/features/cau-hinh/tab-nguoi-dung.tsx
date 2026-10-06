"use client";

import { Upload, UsersRound } from "lucide-react";
import { useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { NoAccess } from "@/components/ui/no-access";
import { PageHeader } from "@/components/ui/page-header";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { DanhBaCanBo } from "./danh-ba-can-bo";
import { IMPORT_BUTTON } from "./excel-import-flow";
import { quyetDinhTabNguoiDung, type QuyetDinhTab } from "./quyen-tab";

/**
 * Cổng của màn "Người dùng" — `/nguoi-dung` từ 05/10/2026, trước đó là một tab của `/cau-hinh`
 * (`docs/ui-ux/14-cau-hinh.md §12.8`: "Tab nào thiếu quyền thì ẩn tab đó"). Tên `TabNguoiDung` giữ
 * nguyên (luật 12, bất biến 3); quyết định vẫn là `quyetDinhTabNguoiDung`, không có bản thứ hai.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY LÀ LỚP TIỆN DỤNG, KHÔNG PHẢI LỚP CHẶN. Hai lớp, và chỉ lớp thứ hai là biện pháp:
 *
 *   1. (ở đây) `permissions` từ `GET /api/v1/sessions/current` không có `admin.user` → không
 *      dựng bảng, không gọi tuyến danh sách. Chỉ để cán bộ không phải bấm vào một thứ chắc
 *      chắn trả 403.
 *   2. (ở máy chủ) `GET /api/v1/staff` khai `RequirePermission("admin.user")` và kiểm trên
 *      TỪNG yêu cầu, theo `(tenant_id, role, permission)`. Gọi thẳng bằng cookie phiên của
 *      chính mình vẫn 403 — không cần sửa một dòng JavaScript nào để thử.
 *
 * Bỏ lớp 1 thì màn hình xấu. Bỏ lớp 2 thì danh bạ cán bộ của một cơ quan nhà nước mở cho mọi
 * tài khoản đã đăng nhập (luật 5, cấm #1: kiểm quyền ở giao diện thay cho tầng dịch vụ là
 * không kiểm gì cả).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * VÌ SAO ĐỌC QUYỀN Ở TRÌNH DUYỆT CHỨ KHÔNG Ở MÁY CHỦ: gọi ở phía máy chủ thì phải chuyển tiếp
 * cookie phiên từ yêu cầu vào lời gọi API — thêm một chỗ cầm cookie phiên, và là đúng chỗ dễ
 * chuyển tiếp sang sai host. Ở đây lời gọi là đường dẫn tương đối trên chính host của xã,
 * trình duyệt tự gửi cookie host-only, và ứng dụng web không bao giờ chạm vào nó
 * (`lib/api/goi.ts`).
 */

/** The prototype's page header words (`AccountWorkspace.tsx`, `UserWorkspace`), verbatim. */
export const USERS_PAGE_TITLE = "Người dùng";
export const USERS_PAGE_SUBTITLE =
  "Tài khoản cán bộ của đơn vị: bộ phận công tác, vai trò được gán và trạng thái hoạt động.";

/**
 * THE PAGE HEADER LIVES HERE, not in `app/nguoi-dung/page.tsx`, for one reason: the prototype puts
 * `Nhập từ Excel` in the header's action slot, and that button opens a dialog whose state belongs to the
 * list below it. The header is drawn in EVERY branch (checking, unreadable, denied, allowed) so the
 * `<h1>` never depends on the session; only the allowed branch gets the action.
 */
function UsersPageHeader({ actions }: { actions?: ReactNode }) {
  return <PageHeader icon={UsersRound} title={USERS_PAGE_TITLE} subtitle={USERS_PAGE_SUBTITLE} actions={actions} />;
}

/** `active` — whether this tab is on display; passed through so the staff screen re-reads its catalogues. */
export function TabNguoiDung({ active = true }: { active?: boolean } = {}) {
  // Phiên đọc MỘT LẦN cho cả trang, ở `PhienProvider`. Trước đây component này tự gọi, nên
  // mở một màn hình là hai lời gọi cùng một tuyến — và tệ hơn: hai câu trả lời có thể khác
  // nhau, cho ra một màn hình vừa hiện tên cán bộ trên đầu trang vừa báo phiên đã hết hạn ở
  // thân trang.
  const phien = usePhien();

  /**
   * Whether the staff Excel import dialog is open. HELD HERE because the button sits in the page header
   * (prototype) while the dialog belongs to the list. The dialog holds the import attempt — including,
   * after a 201, the temporary passwords — and only its own close sets this back to `false`.
   */
  const [importOpen, setImportOpen] = useState(false);

  // `null` là "chưa đọc xong", không phải "không có quyền". Ba trạng thái, không hai.
  const quyetDinh: QuyetDinhTab | null = phien === null ? null : quyetDinhTabNguoiDung(phien);

  if (quyetDinh === null) {
    return (
      <>
        <UsersPageHeader />
        <p role="status">Đang kiểm tra quyền truy cập…</p>
      </>
    );
  }

  // KHÔNG ĐOÁN KHI KHÔNG ĐỌC ĐƯỢC QUYỀN: không dựng bảng, và hiện đúng câu của máy chủ (thường
  // là "Phiên làm việc đã hết hạn" — nhưng chữ ấy do máy chủ viết, không do đây đoán).
  if (!quyetDinh.hien && quyetDinh.vi === "khong-doc-duoc") {
    return (
      <>
        <UsersPageHeader />
        <p className="thong-bao-loi" role="alert">
          {quyetDinh.thongBao}
        </p>
      </>
    );
  }

  if (!quyetDinh.hien) {
    // Shared `NoAccess` (spec v2 §8b) + this tab's own sentence, verbatim, as its caption.
    return (
      <>
        <UsersPageHeader />
        <div className="khung-thieu-quyen flex min-w-0 flex-col items-center pb-10 [&>.trang-thai-rong]:m-0 [&>.trang-thai-rong]:max-w-md [&>.trang-thai-rong]:border-0 [&>.trang-thai-rong]:bg-transparent [&>.trang-thai-rong]:px-4 [&>.trang-thai-rong]:py-0 [&>.trang-thai-rong]:text-center [&>.trang-thai-rong]:text-[13px] [&>.trang-thai-rong]:text-ink-500">
          <NoAccess className="pb-4" />
          <p className="trang-thai-rong">
            Tài khoản của bạn không có quyền quản lý người dùng, nên màn này không hiển thị. Liên hệ
            quản trị viên của đơn vị nếu bạn cần quyền này.
          </p>
        </div>
      </>
    );
  }

  return (
    <>
      {/* Same key as every write of this screen (`admin.user`), so no second gate; the role column of
          the sheet additionally needs `admin.role`, which the server checks on the import itself. */}
      <UsersPageHeader
        actions={
          <Button
            type="button"
            variant="outline"
            size="sm"
            icon={<Upload aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={() => setImportOpen(true)}
            disabled={importOpen}
          >
            {IMPORT_BUTTON}
          </Button>
        }
      />
      <DanhBaCanBo active={active} importOpen={importOpen} onImportClose={() => setImportOpen(false)} />
    </>
  );
}
