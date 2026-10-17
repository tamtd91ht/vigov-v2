"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { EmptyState } from "@/components/ui/empty-state";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Pagination } from "@/components/ui/pagination";
import { SkeletonRows } from "@/components/ui/skeleton";
import { nhanTrangThai } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing export of nhan-phieu.ts, imported not declared (rule 12 inv 3)
import {
  CONTACT_PETITION_PAGE_SIZE,
  listContactPetitions,
  type petitions_contactPetitionPageOut,
} from "@/lib/api/petition-directory-link";

export const CONTACT_PETITIONS_TITLE_ID = "tieu-de-phan-anh-cua-nguoi-dan"; // vi-name-ok: DOM id, same convention as the app's other dialog ids

/** The button that opens this dialog — in the edit dialog and on a directory row (ADR 0102 #5). */
export const VIEW_PETITIONS_BUTTON = "Xem phản ánh";
export const NO_PETITIONS = "Người này chưa có phản ánh nào.";
export const PETITIONS_LOADING = "Đang tải phản ánh…";
export const CONTACT_PETITIONS_PAGINATION_LABEL = "Phân trang phản ánh của người dân";

export function contactPetitionsTitle(fullName: string): string {
  return fullName.trim() === "" ? "Phản ánh của người dân" : `Phản ánh của ${fullName.trim()}`;
}

export function viewPetitionsLabel(fullName: string): string {
  return `Xem phản ánh của ${fullName}`;
}

/** `/phan-anh?id=<lookup code>` — the register's existing deep link (`petitionDeepLinkCode`). */
export function petitionHref(lookupCode: string): string {
  return `/phan-anh?id=${encodeURIComponent(lookupCode)}`;
}

/** "Trang 2 / 3 · 25 phản ánh" — from the server's own total, never counted here. */
export function petitionPageLabel(page: number, total: number): string {
  const pages = Math.max(1, Math.ceil(total / CONTACT_PETITION_PAGE_SIZE));
  return `Trang ${page} / ${pages} · ${total} phản ánh`;
}

const RECEIVED_DATE = new Intl.DateTimeFormat("vi-VN", {
  timeZone: "Asia/Ho_Chi_Minh",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
});

/** The received date, Vietnam time, day only. An unreadable value is shown as sent (a broken contract, not a state). */
export function receivedDateLabel(iso: string): string {
  const at = new Date(iso);
  return Number.isNaN(at.getTime()) ? iso : RECEIVED_DATE.format(at);
}

type PageState =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "ready"; data: petitions_contactPetitionPageOut };

/**
 * "Xem phản ánh" of one directory row (ADR 0102 #5, #8): that person's petitions, ten a page, newest as the
 * server orders them. Each row is the lookup code, the title, the status label and the received date — no
 * reporter, no content; the row opens the petition in the register (`/phan-anh?id=`).
 *
 * THE PERSON IS NAMED BY THE ROW ID ONLY. The server resolves the number from it in memory; nothing here
 * ever holds the number, so nothing can put it on a URL (rule 3, forbidden #4).
 *
 * Which petitions appear is entirely the server's: anonymous ones never (ADR 0102 #4), `can-bo` ones only for
 * `feedback.restricted` — and the dialog does not say that any were left out (rule 4, forbidden #2).
 */
export function CitizenPetitionsDialog({
  contactId,
  fullName,
  onClose,
}: {
  contactId: string;
  fullName: string;
  onClose: () => void;
}) {
  const [page, setPage] = useState(1);
  const [state, setState] = useState<{ page: number; value: PageState }>({ page: 1, value: { phase: "loading" } });

  useEffect(() => {
    let dropped = false;
    listContactPetitions(contactId, page).then((kq) => {
      if (dropped) return;
      setState({ page, value: kq.ok ? { phase: "ready", data: kq.duLieu } : { phase: "error", message: kq.thongBao } });
    });
    return () => {
      dropped = true;
    };
  }, [contactId, page]);

  // A page whose answer has not arrived yet is loading — the previous page's rows are never shown under it.
  const view: PageState = state.page === page ? state.value : { phase: "loading" };
  const data = view.phase === "ready" ? view.data : null;
  const hasNext = data !== null && page * CONTACT_PETITION_PAGE_SIZE < data.total;

  return (
    <ModalDialog titleId={CONTACT_PETITIONS_TITLE_ID} className="max-w-[42rem]" onDismiss={onClose}>
      <ModalDialogHeader titleId={CONTACT_PETITIONS_TITLE_ID} title={contactPetitionsTitle(fullName)} />

      {/* The scroll region carries `-m-1 p-1` (ModalDialog LAYOUT CONTRACT): the row links' focus ring needs the room. */}
      <div className="-m-1 flex min-h-0 flex-col overflow-y-auto p-1">
        {view.phase === "loading" && (
          <>
            <p className="an-thi-giac" role="status">
              {PETITIONS_LOADING}
            </p>
            <SkeletonRows rows={4} columns={4} />
          </>
        )}
        {view.phase === "error" && (
          <p className="thong-bao-loi m-0" role="alert">
            {view.message}
          </p>
        )}
        {data !== null && data.items.length === 0 && <EmptyState tone="neutral" title={NO_PETITIONS} role="status" />}
        {data !== null && data.items.length > 0 && (
          <ul className="m-0 flex list-none flex-col gap-1.5 p-0" data-contact-petitions="">
            {data.items.map((p) => (
              <li key={p.lookup_code}>
                <Link
                  href={petitionHref(p.lookup_code)}
                  className="border-line hover:bg-surface flex min-w-0 flex-col gap-0.5 rounded-lg border border-solid bg-white px-3 py-2 text-[12.5px] no-underline sm:flex-row sm:items-center sm:gap-3"
                >
                  <span className="ma-muc text-ink-muted shrink-0 tabular-nums">{p.lookup_code}</span>
                  <span className="text-navy min-w-0 flex-1 truncate font-semibold">{p.title}</span>
                  <span className="text-ink shrink-0">{nhanTrangThai(p.status)}</span>
                  <span className="text-ink-muted shrink-0 tabular-nums">{receivedDateLabel(p.received_at)}</span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>

      {data !== null && (page > 1 || hasNext) && (
        <div className="text-ink flex flex-wrap items-center justify-end gap-3 text-[12.5px]">
          <span>{petitionPageLabel(page, data.total)}</span>
          <Pagination
            hasPrev={page > 1}
            hasNext={hasNext}
            onPrev={() => setPage((n) => Math.max(1, n - 1))}
            onNext={() => setPage((n) => n + 1)}
            label={CONTACT_PETITIONS_PAGINATION_LABEL}
          />
        </div>
      )}
    </ModalDialog>
  );
}
