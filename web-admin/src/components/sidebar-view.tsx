import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import Link from "next/link";

import { PendingMarker } from "@/components/ui/pending-feature";
import { Tooltip } from "@/components/ui/tooltip";
import type { CauHinhXaHienThi } from "@/lib/cau-hinh-xa-hien-thi";

import { CommuneIdentity } from "./commune-identity";
import { menuIcon } from "./menu-icons";
import { dangChon, PENDING_SCREENS, type NhomMenu } from "./muc-menu";

/**
 * What the sidebar DRAWS, given what `ThanhBen` decided — spec §5, owner decisions 02/10/2026.
 *
 * SPLIT FROM `thanh-ben.tsx` FOR THE SAME REASON `muc-menu.ts` IS: the collapsed state is read
 * from `localStorage`, which the server render never sees (it always draws the expanded menu).
 * As a pure function of its props, the collapsed branch renders to a string in a plain Node test.
 *
 * Nothing here filters or orders items: `groups` arrives already filtered by `locMenu`.
 */

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
        {groups.map((n, i) => (
          <div key={n.ten === "" ? `untitled-${i}` : n.ten} className="thanh-ben-nhom">
            {/* A group with no heading (Tổng quan) draws no empty label line. */}
            {n.ten !== "" && <p className="thanh-ben-nhan-nhom">{n.ten}</p>}
            <ul>
              {n.muc.map((m) => {
                const Icon = menuIcon(m.nhan);
                if (m.duong === null) {
                  // KHÔNG PHẢI LIÊN KẾT, và không phải `<button disabled>`: cả hai đều nhận được
                  // tiêu điểm bàn phím rồi không làm gì, tức bắt người dùng bàn phím đi qua những
                  // chặng chết để tới mục dùng được. Một `<span>` mang `aria-disabled` nằm ngoài
                  // thứ tự tiêu điểm và vẫn được trình đọc màn hình đọc đúng.
                  //
                  // The ONE focusable thing is the "?" (ADR 0068 §14): it opens what the screen is and
                  // why it is not built, and its accessible name already names the item. Collapsed on
                  // a wide screen the label is hidden from the eye, so the "?" moves to the icon's
                  // corner and its tooltip names the item too. Under 1024px the bar is a strip with
                  // labels shown whatever was saved (`globals.css`), so it stays in the row there.
                  const info = PENDING_SCREENS[m.nhan];
                  return (
                    <li key={m.nhan} className="thanh-ben-muc chua-co">
                      <span>
                        {/* `aria-disabled` on an INNER span, never on the row: ARIA applies it to
                            every focusable descendant too, and the "?" would be announced disabled. */}
                        <span
                          aria-disabled="true"
                          className={collapsed ? "flex min-w-0 flex-1 items-center gap-3 lg:flex-none" : "flex min-w-0 flex-1 items-center gap-3"}
                        >
                          <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />
                          <span className="thanh-ben-nhan">{m.nhan}</span>
                        </span>
                        {info !== undefined && (
                          <PendingMarker
                            info={info}
                            side="right"
                            nameInHover={collapsed}
                            className={collapsed ? "ml-auto lg:absolute lg:top-0 lg:right-0" : "ml-auto"}
                          />
                        )}
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
