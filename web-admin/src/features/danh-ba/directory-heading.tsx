import type { ReactNode } from "react";

import { MO_TA_TRANG, TIEU_DE_TRANG } from "./nhan-danh-ba";

/**
 * The tab's header — the prototype's `mb-4 flex flex-wrap items-end gap-4` row (`StaffDirectoryWorkspace
 * .tsx:146-171`): the title and one line on the left, the two actions on the right.
 *
 * Its own component because TWO places draw it: the screen (`DanhBaLienHe`, with its actions, which
 * open dialogs that screen owns) and the tab (`StaffDirectoryTab`, without actions, while the
 * permission gate is still reading the session or has refused) — so the tab keeps its title in every
 * state, and the buttons sit in the header row without any absolute positioning.
 *
 * No hook: a plain function, safe under the mocked React of `danh-ba-lien-he.luong.test.tsx`.
 */
export function DirectoryHeading({ actions }: { actions?: ReactNode }) {
  return (
    <div className="mb-4 flex flex-wrap items-end gap-4">
      <div className="min-w-0">
        <h1 className="text-navy m-0 text-[22px] leading-tight font-bold">{TIEU_DE_TRANG}</h1>
        <p className="text-ink-muted m-0 mt-1 text-[13px]">{MO_TA_TRANG}</p>
      </div>
      {actions !== undefined && <div className="ml-auto flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}
