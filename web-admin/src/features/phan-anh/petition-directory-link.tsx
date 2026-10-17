"use client";

import { BookUser, UserPlus } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { CitizenContactDialog, type ContactDialogMode } from "@/features/citizen-directory/citizen-contact-dialog";
import type { CitizenField } from "@/lib/api/citizen-fields";
import { listCitizenFields } from "@/lib/api/citizen-fields";
import type { CitizenGroup } from "@/lib/api/citizen-groups";
import { listCitizenGroups } from "@/lib/api/citizen-groups";
import { readDirectoryMatch, type DirectoryMatch } from "@/lib/api/petition-directory-link";
import { layPhieuPhanAnh } from "@/lib/api/phieu-phan-anh"; // vi-name-ok: existing export of phieu-phan-anh.ts, imported not declared (rule 12 inv 3)
import type { identity_thonToDanPhoRa, petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";
import {
  CITIZEN_READ_PERMISSION,
  CITIZEN_REPORT_UNMASK_PERMISSION,
  CITIZEN_UPDATE_PERMISSION,
  coQuyen, // vi-name-ok: existing export of quyen.ts, imported not declared (rule 12 inv 3)
  QUYEN_XEM_PHAN_ANH, // vi-name-ok: existing export of quyen.ts, imported not declared (rule 12 inv 3)
} from "@/lib/quyen";

import { buttonClass, Glyph } from "./petition-ui";

export const DIRECTORY_SYNC_LABEL = "Đồng bộ danh bạ";
export const VIEW_IN_DIRECTORY = "Xem trong danh bạ";
export const ADD_TO_DIRECTORY = "Thêm vào danh bạ";
export const DIRECTORY_MATCH_FAILED = "Chưa đối chiếu được với danh bạ.";
export const DIRECTORY_MATCH_LOADING = "Đang đối chiếu với danh bạ…";

/** `/nguoi-dan?id=<row id>` — the directory opens that row's edit dialog (`citizenContactDeepLinkId`). Never a number. */
export function directoryContactHref(contactId: string): string {
  return `/nguoi-dan?id=${encodeURIComponent(contactId)}`;
}

/**
 * The four keys the link needs, ALL of them (ADR 0102 #5): the match route checks `feedback.read` +
 * `citizen.read`; both of its actions need `feedback.unmask` (the full number reaches the browser) and
 * `citizen.update` (the edit dialog, the add). Without any one nothing is drawn AND nothing is requested.
 * Convenience only — every route checks its own keys again (rule 5, forbidden #1).
 */
export function mayLinkDirectory(permissions: readonly string[]): boolean {
  return (
    coQuyen(permissions, QUYEN_XEM_PHAN_ANH) &&
    coQuyen(permissions, CITIZEN_READ_PERMISSION) &&
    coQuyen(permissions, CITIZEN_REPORT_UNMASK_PERMISSION) &&
    coQuyen(permissions, CITIZEN_UPDATE_PERMISSION)
  );
}

/** A full number only: a masked one (`09****5678`) would be refused on save, so the field is left empty instead. */
function unmaskedPhone(phone: string): string {
  return /^[0-9+][0-9 .+-]*$/.test(phone.trim()) ? phone.trim() : "";
}

type Opening = { phase: "idle" } | { phase: "preparing" } | { phase: "refused"; message: string };

/**
 * "Đồng bộ danh bạ" in the petition drawer (ADR 0102). A CHILD COMPONENT ON PURPOSE: it holds state and
 * reads the network, and `ChiTietPhieu` must not gain a hook (`chon-can-bo.test.tsx` calls it as a function).
 *
 *   found           "Xem trong danh bạ" → `/nguoi-dan?id=<row id>`
 *   not_found       "Thêm vào danh bạ"  → the directory's own add dialog, name + number prefilled
 *   not_applicable  nothing (anonymous, or no number) — ADR 0102 #4
 *   error / 503     one muted sentence; NEVER read as not_found (that would offer to add somebody who is there)
 *
 * THE PREFILL COMES FROM THE PETITION'S DETAIL ROUTE, on the press: that route is the one path that returns
 * the sender unmasked to a `feedback.unmask` holder, and it writes the audit entry for it (ADR 0030). The
 * drawer's own copy is usually the LIST row, which is always masked. One press = one audited read; nothing
 * is fetched on render. Hamlet and address are NOT carried over: on a petition they are the place of the
 * incident (ADR 0102 #7). The number lives in the dialog's state only — never a URL, never a log.
 */
export function PetitionDirectoryLink({
  petition,
  permissions,
  units = [],
}: {
  petition: petitions_phieuPhanAnhRa;
  permissions: readonly string[];
  /** The commune's residential units — the add dialog's select (left unselected). */
  units?: readonly identity_thonToDanPhoRa[];
}) {
  const allowed = mayLinkDirectory(permissions);
  const code = petition.code;
  const [reads, setReads] = useState(0);
  const [answer, setAnswer] = useState<{ key: string; match: DirectoryMatch } | null>(null);
  const [opening, setOpening] = useState<Opening>({ phase: "idle" });
  const [dialog, setDialog] = useState<{
    mode: ContactDialogMode;
    groups: readonly CitizenGroup[];
    fields: readonly CitizenField[];
  } | null>(null);

  const key = `${code}|${reads}`;
  useEffect(() => {
    if (!allowed) return;
    let dropped = false;
    readDirectoryMatch(code).then((match) => {
      if (!dropped) setAnswer({ key: `${code}|${reads}`, match });
    });
    return () => {
      dropped = true;
    };
  }, [allowed, code, reads]);

  if (!allowed) return null;
  const match = answer !== null && answer.key === key ? answer.match : null;

  async function openAdd(phoneUnverified: boolean) {
    if (opening.phase === "preparing") return;
    setOpening({ phase: "preparing" });
    const [detail, groups, fields] = await Promise.all([layPhieuPhanAnh(code), listCitizenGroups(), listCitizenFields()]);
    if (!detail.ok) {
      setOpening({ phase: "refused", message: detail.thongBao });
      return;
    }
    setOpening({ phase: "idle" });
    setDialog({
      mode: {
        kind: "add",
        idempotencyKey: crypto.randomUUID(),
        prefill: { fullName: detail.duLieu.reporter_name, phone: unmaskedPhone(detail.duLieu.reporter_phone) },
        phoneUnverified,
      },
      // A refusal of either catalogue leaves it empty, as on the directory screen: the person can still be added.
      groups: groups.ok ? groups.duLieu.items : [],
      fields: fields.ok
        ? [...fields.duLieu.items].filter((f) => f.retired_at === null).sort((a, b) => a.sort_order - b.sort_order)
        : [],
    });
  }

  return (
    <div className="mt-1.5 flex flex-wrap items-center gap-2" role="group" aria-label={DIRECTORY_SYNC_LABEL} data-directory-link="">
      {match === null && (
        <p className="an-thi-giac m-0" role="status">
          {DIRECTORY_MATCH_LOADING}
        </p>
      )}
      {match?.kind === "error" && <p className="m-0 text-[11px] text-ink-muted">{DIRECTORY_MATCH_FAILED}</p>}
      {match?.kind === "found" && (
        <Link className={buttonClass("secondary", "sm")} href={directoryContactHref(match.contactId)}>
          <Glyph icon={BookUser} />
          {VIEW_IN_DIRECTORY}
        </Link>
      )}
      {match?.kind === "not_found" && (
        <Button
          type="button"
          variant="secondary"
          size="sm"
          icon={<Glyph icon={UserPlus} />}
          disabled={opening.phase === "preparing"}
          aria-busy={opening.phase === "preparing" || undefined}
          onClick={() => void openAdd(match.phoneUnverified)}
        >
          {ADD_TO_DIRECTORY}
        </Button>
      )}
      {opening.phase === "refused" && (
        <p className="thong-bao-loi m-0 text-[11px]" role="alert">
          {opening.message}
        </p>
      )}
      {dialog !== null && (
        <CitizenContactDialog
          mode={dialog.mode}
          groups={dialog.groups}
          fields={dialog.fields}
          units={units}
          canReveal={false}
          onClose={() => setDialog(null)}
          // A 201: ask again, so the button turns into "Xem trong danh bạ" from the server's answer, not a guess.
          onSaved={() => setReads((n) => n + 1)}
        />
      )}
    </div>
  );
}
