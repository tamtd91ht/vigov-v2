"use client";

import {
  BellRing,
  CalendarClock,
  FileClock,
  Home,
  ImageIcon,
  KeyRound,
  ListTree,
  Mail,
  MapPinned,
  MessageSquareText,
  Network,
  UsersRound,
  type LucideIcon,
} from "lucide-react";
import { useRef, useState, type KeyboardEvent, type ReactNode } from "react";

import { Tab, TabList } from "@/components/ui/tabs";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { AuditLogTab } from "./audit-log-tab";
import { AutomationTab } from "./automation-tab";
import { CommuneBrandingTab } from "./commune-branding-tab";
import { MailServerTab } from "./mail-server-tab";
import { MapFieldTab } from "./map-field-tab";
import { SystemMessagesTab } from "./system-messages-tab";
import { TabDanhMuc } from "./tab-danh-muc";
import { TabNguoiDung } from "./tab-nguoi-dung";
import { TabPhanQuyen } from "./tab-phan-quyen";
import { TabSoDoToChuc } from "./tab-so-do-to-chuc";
import { TabThoiHanXuLy } from "./tab-thoi-han-xu-ly";
import { TabThonToDanPho } from "./tab-thon-to-dan-pho";
import {
  cacTabHien,
  coThanhTab,
  TAB_CAU_HINH,
  tabKeTheoPhim,
  type MaTabCauHinh,
} from "./thanh-tab-cau-hinh";

/**
 * `active` is whether the panel is the one on display. Only a tab that shows catalogues another tab can
 * change needs it: "Người dùng" re-reads units and roles when it comes back (ND-01/ND-02).
 */
const NOI_DUNG: Record<MaTabCauHinh, (active: boolean) => ReactNode> = {
  "so-do-to-chuc": () => <TabSoDoToChuc />,
  "thon-to-dan-pho": () => <TabThonToDanPho />,
  "nguoi-dung": (active) => <TabNguoiDung active={active} />,
  "phan-quyen": () => <TabPhanQuyen />,
  "danh-muc": () => <TabDanhMuc />,
  "truong-ban-do": () => <MapFieldTab />,
  "loi-he-thong": () => <SystemMessagesTab />,
  "thoi-han-xu-ly": () => <TabThoiHanXuLy />,
  "tu-dong-hoa": () => <AutomationTab />,
  "may-chu-thu": () => <MailServerTab />,
  "nhat-ky-he-thong": () => <AuditLogTab />,
  "nhan-dien-xa": () => <CommuneBrandingTab />,
};

/** Icon of each tab (spec §7 "Tab: chữ 14/500 + icon"). Decorative: the tab's word carries the meaning. */
const ICON_TAB: Record<MaTabCauHinh, LucideIcon> = {
  "so-do-to-chuc": Network,
  "thon-to-dan-pho": Home,
  "nguoi-dung": UsersRound,
  "phan-quyen": KeyRound,
  "danh-muc": ListTree,
  "truong-ban-do": MapPinned,
  "loi-he-thong": MessageSquareText,
  "thoi-han-xu-ly": CalendarClock,
  "tu-dong-hoa": BellRing,
  "may-chu-thu": Mail,
  "nhat-ky-he-thong": FileClock,
  "nhan-dien-xa": ImageIcon,
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

      {coThanh && (
        <TabList aria-label="Các phần cấu hình" className="mb-4">
          {hien.map((t, i) => (
            <Tab
              key={t.ma}
              ref={(el) => {
                nutTab.current[t.ma] = el;
              }}
              type="button"
              icon={ICON_TAB[t.ma]}
              id={idTab(t.ma)}
              selected={t.ma === chon}
              aria-controls={idPanel(t.ma)}
              tabIndex={t.ma === chon ? 0 : -1}
              onClick={() => datDangChon(t.ma)}
              onKeyDown={(e) => xuLyPhim(e, i)}
            >
              {t.nhan}
            </Tab>
          ))}
        </TabList>
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
