"use client";

import { useRef, useState, type KeyboardEvent, type ReactNode } from "react";

import { usePhien } from "@/features/phien/phien-hien-tai";
import { cn } from "@/lib/cn";

import { AuditLogTab } from "./audit-log-tab";
import { AutomationTab } from "./automation-tab";
import { CommuneBrandingTab } from "./commune-branding-tab";
import { MailServerTab } from "./mail-server-tab";
import { MapFieldTab } from "./map-field-tab";
import { SystemMessagesTab } from "./system-messages-tab";
import { TabDanhMuc } from "./tab-danh-muc";
import { TabSoDoToChuc } from "./tab-so-do-to-chuc";
import { TabThoiHanXuLy } from "./tab-thoi-han-xu-ly";
import { TabThonToDanPho } from "./tab-thon-to-dan-pho";
import { ZaloChannelTab } from "./zalo-channel-tab";
import {
  cacTabHien,
  coThanhTab,
  TAB_CAU_HINH,
  tabKeTheoPhim,
  type MaTabCauHinh,
} from "./thanh-tab-cau-hinh";

/**
 * `active` is whether the panel is the one on display. No tab uses it since "Người dùng" — the one
 * that re-read units and roles when it came back (ND-01/ND-02) — moved to `/nguoi-dung` (05/10/2026),
 * where it re-reads on mount instead. Kept so the next tab with that need does not rebuild the plumbing.
 */
const NOI_DUNG: Record<MaTabCauHinh, (active: boolean) => ReactNode> = {
  "so-do-to-chuc": () => <TabSoDoToChuc />,
  "thon-to-dan-pho": () => <TabThonToDanPho />,
  "danh-muc": () => <TabDanhMuc />,
  "truong-ban-do": () => <MapFieldTab />,
  "loi-he-thong": () => <SystemMessagesTab />,
  "thoi-han-xu-ly": () => <TabThoiHanXuLy />,
  "tu-dong-hoa": () => <AutomationTab />,
  "may-chu-thu": () => <MailServerTab />,
  "kenh-zalo": () => <ZaloChannelTab />,
  "nhat-ky-he-thong": () => <AuditLogTab />,
  "nhan-dien-xa": () => <CommuneBrandingTab />,
};

const idTab = (ma: MaTabCauHinh) => `tab-cau-hinh-${ma}`;
const idPanel = (ma: MaTabCauHinh) => `panel-cau-hinh-${ma}`;

/**
 * Thanh tab + các panel của màn Cấu hình. Quyết định tab nào hiện và có thanh hay không nằm ở
 * `thanh-tab-cau-hinh.ts` (có bài test riêng); ở đây chỉ dựng.
 *
 * PANEL KHÔNG ĐƯỢC CHỌN VẪN ĐƯỢC GIỮ (`hidden`), không gỡ khỏi cây: một cột Phân quyền đang sửa dở
 * hay một biểu mẫu nhập nửa chừng mà mất chỉ vì bấm sang tab khác là mất việc của cán bộ. Cái giá là
 * mọi tab được phép đều đọc dữ liệu của nó ngay khi mở màn — đúng như khi các phần còn dựng nối tiếp.
 *
 * Tab đang chọn chỉ nằm trong state của component: không ghi `localStorage`, không đồng bộ URL.
 */
export function KhungTabCauHinh() {
  const phien = usePhien();
  const hien = cacTabHien(TAB_CAU_HINH, phien);
  const coThanh = coThanhTab(phien, hien.length);

  const [dangChon, datDangChon] = useState<MaTabCauHinh>("so-do-to-chuc");
  const nutTab = useRef<Partial<Record<MaTabCauHinh, HTMLButtonElement | null>>>({});

  // Tab đã chọn mà nay không còn trong danh sách được hiện thì về tab đầu tiên được hiện — không
  // bao giờ để một panel bị ẩn mà không panel nào hiện thay.
  const chon: MaTabCauHinh | undefined = hien.some((t) => t.ma === dangChon)
    ? dangChon
    : hien[0]?.ma;

  function xuLyPhim(e: KeyboardEvent<HTMLButtonElement>, viTri: number) {
    const ke = tabKeTheoPhim(e.key, viTri, hien.length);
    const toi = ke === null ? undefined : hien[ke];
    if (toi === undefined) return;
    e.preventDefault();
    const ma = toi.ma;
    datDangChon(ma);
    nutTab.current[ma]?.focus();
  }

  return (
    <>
      {/* Phiên đọc hỏng thì các tab có cổng bị ẩn (đóng khi không chắc). Câu của máy chủ — thường là
          "phiên đã hết hạn" — phải còn ra tới màn hình, như khi các phần ấy tự hiện nó. */}
      {phien !== null && !phien.ok && (
        <p className="thong-bao-loi" role="alert">
          {phien.thongBao}
        </p>
      )}

      {/* The prototype's `TabsList` (ADR 0068 lần 5): one muted rounded strip, 3px inset, the open
          tab a white raised segment; words only, no icons. Eleven tabs do not fit 320px, so the
          strip scrolls sideways INSIDE its own box (`max-w-full overflow-x-auto`) and never widens
          the page. */}
      {coThanh && (
        <div className="mb-1 max-w-full min-w-0 overflow-x-auto">
          <div
            role="tablist"
            aria-label="Các phần cấu hình"
            className="inline-flex w-max items-center gap-0.5 rounded-lg bg-surface-subtle p-[3px]"
          >
            {hien.map((t, i) => (
              <button
                key={t.ma}
                ref={(el) => {
                  nutTab.current[t.ma] = el;
                }}
                type="button"
                role="tab"
                id={idTab(t.ma)}
                aria-selected={t.ma === chon}
                aria-controls={idPanel(t.ma)}
                tabIndex={t.ma === chon ? 0 : -1}
                onClick={() => datDangChon(t.ma)}
                onKeyDown={(e) => xuLyPhim(e, i)}
                className={cn(
                  "inline-flex h-8 shrink-0 cursor-pointer items-center rounded-md border border-solid border-transparent bg-transparent px-3 [font-family:inherit] text-[13px] font-medium whitespace-nowrap text-ink-500",
                  "transition-[color,background-color] duration-(--dur-fast) ease-(--ease) hover:text-ink-900",
                  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
                  t.ma === chon && "bg-surface text-ink-900 shadow-sm",
                )}
              >
                {t.nhan}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Cùng một danh sách có `key` ở cả hai trạng thái (có thanh / không thanh), nên khi thanh
          xuất hiện React giữ nguyên các panel đã dựng — không đọc lại, không mất gì đang nhập. */}
      {hien.map((t) => (
        <div
          key={t.ma}
          id={idPanel(t.ma)}
          className="panel-cau-hinh"
          hidden={t.ma !== chon}
          {...(coThanh
            ? { role: "tabpanel", "aria-labelledby": idTab(t.ma), tabIndex: 0 }
            : {})}
        >
          {NOI_DUNG[t.ma](t.ma === chon)}
        </div>
      ))}
    </>
  );
}
