import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import Link from "next/link";

import { Tooltip } from "@/components/ui/tooltip";
import type { CauHinhXaHienThi } from "@/lib/cau-hinh-xa-hien-thi";

import { CommuneIdentity } from "./commune-identity";
import { menuIcon } from "./menu-icons";
import { CHUA_CO_MAN, dangChon, type NhomMenu } from "./muc-menu";

/**
 * What the sidebar DRAWS, given what `ThanhBen` decided — spec §5, owner decisions 02/10/2026.
 *
 * SPLIT FROM `thanh-ben.tsx` FOR THE SAME REASON `muc-menu.ts` IS: the collapsed state is read
 * from `localStorage`, which the server render never sees (it always draws the expanded menu).
 * As a pure function of its props, the collapsed branch renders to a string in a plain Node test.
 *
 * Nothing here filters or orders items: `groups` arrives already filtered by `locMenu`.
 */

/**
 * Badge on an item with no screen yet. "Chưa có", NEVER "Sắp có": a public authority's menu saying
 * "coming soon" is a promise with a date nobody set (owner decision 02/10/2026). The full sentence
 * `CHUA_CO_MAN` ("Chưa có màn hình") stays the item's `title`.
 */
export const NOT_BUILT_BADGE = "Chưa có";

export const COLLAPSE_LABEL = "Thu gọn menu";
export const EXPAND_LABEL = "Mở rộng menu";

export type SidebarViewProps = {
  commune: CauHinhXaHienThi;
  groups: readonly NhomMenu[];
  pathname: string;
  collapsed: boolean;
  onToggleCollapsed: () => void;
};

export function SidebarView({ commune, groups, pathname, collapsed, onToggleCollapsed }: SidebarViewProps) {
  const toggleLabel = collapsed ? EXPAND_LABEL : COLLAPSE_LABEL;
  const ToggleIcon = collapsed ? PanelLeftOpen : PanelLeftClose;

  return (
    <nav className={collapsed ? "thanh-ben thu-gon" : "thanh-ben"} aria-label="Điều hướng chính">
      <div className="thanh-ben-dinh">
        <CommuneIdentity commune={commune} />
      </div>

      <div className="thanh-ben-cuon">
        {groups.map((n) => (
          <div key={n.ten} className="thanh-ben-nhom">
            <p className="thanh-ben-nhan-nhom">{n.ten}</p>
            <ul>
              {n.muc.map((m) => {
                const Icon = menuIcon(m.nhan);
                if (m.duong === null) {
                  // KHÔNG PHẢI LIÊN KẾT, và không phải `<button disabled>`: cả hai đều nhận được
                  // tiêu điểm bàn phím rồi không làm gì, tức bắt người dùng bàn phím đi qua những
                  // chặng chết để tới mục dùng được. Một `<span>` mang `aria-disabled` nằm ngoài
                  // thứ tự tiêu điểm và vẫn được trình đọc màn hình đọc đúng.
                  //
                  // Collapsed, the label is hidden from the eye, so the native tooltip names the
                  // item as well as saying it has no screen.
                  return (
                    <li key={m.nhan} className="thanh-ben-muc chua-co">
                      <span aria-disabled="true" title={collapsed ? `${m.nhan} — ${CHUA_CO_MAN}` : CHUA_CO_MAN}>
                        <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />
                        <span className="thanh-ben-nhan">{m.nhan}</span>
                        <span className="thanh-ben-dau-chua-co">{NOT_BUILT_BADGE}</span>
                      </span>
                    </li>
                  );
                }
                const active = dangChon(m.duong, pathname);
                return (
                  <li key={m.nhan} className="thanh-ben-muc">
                    <Tooltip content={m.nhan} side="right" enabled={collapsed}>
                      <Link
                        href={m.duong}
                        aria-current={active ? "page" : undefined}
                        className={active ? "dang-chon" : undefined}
                      >
                        <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />
                        <span className="thanh-ben-nhan">{m.nhan}</span>
                      </Link>
                    </Tooltip>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </div>

      <div className="thanh-ben-chan">
        <Tooltip content={toggleLabel} side="right" enabled={collapsed}>
          <button
            type="button"
            className="thanh-ben-nut-thu-gon"
            onClick={onToggleCollapsed}
            aria-expanded={!collapsed}
            // Nhãn nói HÀNH ĐỘNG SẼ XẢY RA, không nói trạng thái hiện tại: người dùng trình đọc
            // màn hình nghe "Thu gọn menu" thì biết bấm vào sẽ thu gọn.
            aria-label={toggleLabel}
          >
            <ToggleIcon aria-hidden="true" focusable="false" strokeWidth={1.8} />
            <span className="thanh-ben-nhan">{toggleLabel}</span>
          </button>
        </Tooltip>
      </div>
    </nav>
  );
}
