"use client";

import { useRef, useState, type KeyboardEvent, type ReactNode } from "react";

import { Tab, TabList } from "@/components/ui/tabs";
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
import { WorkingCalendarTab } from "./working-calendar-tab";
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
  "lich-lam-viec": () => <WorkingCalendarTab />,
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

      {/* The prototype's shadcn `TabsList` (spec 02, ADR 0079): the `muted` strip (#edf3f8), 3px inset,
          segments `px-1.5 text-sm font-medium` at 60% foreground, each 25px tall (the prototype's
          `h-[calc(100%-1px)]` of a 32px strip); the open one a WHITE raised segment (`bg-surface` here —
          this app's `bg-background` is the page colour, not white).
          WRAPS, NEVER SCROLLS (owner, VALIDATE 08/10/2026): the prototype has nine tabs, this screen
          twelve, and at 1440px a sideways scroller hid "Nhận diện xã" past the right edge with nothing
          saying it was there. So the strip is `flex-wrap` with `h-auto`: when a row is full the next tabs
          go onto a second row and the muted strip grows to hold both. */}
      {coThanh && (
        <TabList aria-label="Các phần cấu hình" className="h-auto flex-wrap overflow-visible">
          {hien.map((t, i) => (
            <Tab
              key={t.ma}
              ref={(el: HTMLButtonElement | null) => {
                nutTab.current[t.ma] = el;
              }}
              id={idTab(t.ma)}
              selected={t.ma === chon}
              aria-controls={idPanel(t.ma)}
              tabIndex={t.ma === chon ? 0 : -1}
              onClick={() => datDangChon(t.ma)}
              onKeyDown={(e) => xuLyPhim(e, i)}
              className={cn("h-[25px]", t.ma === chon && "bg-surface")}
            >
              {t.nhan}
            </Tab>
          ))}
        </TabList>
      )}

      {/* Cùng một danh sách có `key` ở cả hai trạng thái (có thanh / không thanh), nên khi thanh
          xuất hiện React giữ nguyên các panel đã dựng — không đọc lại, không mất gì đang nhập.
          28px from the strip to every tab's content (spec 02: shadcn Tabs `gap-2` + TabsContent `mt-5`);
          the first child's own margin is zeroed so no tab can add to it. */}
      {hien.map((t) => (
        <div
          key={t.ma}
          id={idPanel(t.ma)}
          className={cn("min-w-0 [&>:first-child]:mt-0", coThanh && "mt-7")}
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
