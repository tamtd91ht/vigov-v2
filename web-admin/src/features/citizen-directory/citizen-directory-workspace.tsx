"use client";

import { Plus, Upload } from "lucide-react";
import { useCallback, useEffect, useState, type KeyboardEvent } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Tab, TabList } from "@/components/ui/tabs";
import {
  TRANG_DAU, // vi-name-ok: existing export of ngan-xep-con-tro.ts, imported not declared (rule 12 inv 3)
  coTrangTruoc, // vi-name-ok: existing export of ngan-xep-con-tro.ts, imported not declared (rule 12 inv 3)
  sangTrangSau, // vi-name-ok: existing export of ngan-xep-con-tro.ts, imported not declared (rule 12 inv 3)
  veTrangTruoc, // vi-name-ok: existing export of ngan-xep-con-tro.ts, imported not declared (rule 12 inv 3)
  type NganXepConTro, // vi-name-ok: existing type of ngan-xep-con-tro.ts, imported not declared (rule 12 inv 3)
} from "@/features/cau-hinh/ngan-xep-con-tro";
import {
  NO_CITIZEN_CONTACT_FILTER,
  countCitizenContacts,
  deleteCitizenContact,
  listCitizenContactTags,
  readCitizenContactPage,
  type CitizenContact,
  type CitizenContactFilter,
  type CitizenContactTag,
} from "@/lib/api/citizen-contacts";
import { listCitizenGroups, type CitizenGroup } from "@/lib/api/citizen-groups";
import type { identity_thonToDanPhoRa } from "@/lib/api/schema.gen";
import { layDanhSachThonToDanPho } from "@/lib/api/thon-to-dan-pho"; // vi-name-ok: existing export of thon-to-dan-pho.ts, imported not declared (rule 12 inv 3)

import { CitizenContactDeleteDialog } from "./citizen-contact-delete-dialog";
import { CitizenContactDialog, type ContactDialogMode } from "./citizen-contact-dialog";
import { CitizenContactImportDialog } from "./citizen-contact-import-dialog";
import { CitizenContactList, type ContactPageState } from "./citizen-contact-list";
import {
  ADD_CONTACT_BUTTON,
  CITIZEN_DIRECTORY_SUBTITLE,
  CITIZEN_DIRECTORY_TITLE,
  IMPORT_OPEN_BUTTON,
  REMOVE_FAILED_TOAST,
  TAB_CONTACTS,
  TAB_GROUPS,
  deleteReasonProblem,
  removedToast,
  tabLabel,
} from "./citizen-directory-words";
import { CitizenGroupTab } from "./citizen-group-tab";

export const CITIZEN_PANEL_ID = "khung-tab-nguoi-dan"; // vi-name-ok: DOM id, same convention as `khung-tab-van-ban`

const TABS = [
  { id: "contacts", label: TAB_CONTACTS },
  { id: "groups", label: TAB_GROUPS },
] as const;
type TabId = (typeof TABS)[number]["id"];

export function citizenTabDomId(id: TabId): string {
  return `tab-nguoi-dan-${id}`;
}

/** The filter in force and the cursor stack — ONE state, so a filter change cannot forget to return to page 1. */
type ListQuery = { readonly filter: CitizenContactFilter; readonly stack: NganXepConTro };

/**
 * The screen behind the gate — the prototype's `CitizenWorkspace` frame (chapter 16 §2): the page
 * header with its two buttons (citizen.update only), the two tabs with their figures, the active tab's
 * body, the dialogs. `canEdit` is the session's `citizen.update`; every hidden control is convenience —
 * the sixteen routes check the key on every call (rule 5, forbidden #1).
 *
 * WHAT IS READ, AND WHEN:
 *   groups      on open and after every write (the filter's "(n)" and the Nhóm tab)
 *   tags        on open and after every contact write (the strip)
 *   units       on open, once, only for an editor (the dialog's select)
 *   page        on every change of filter / cursor, and after every write
 *   count       on every change of filter, and after every write — `POST …/count-queries`, the SAME
 *               filter as the page (D8), so "(n)" and the rows describe one set
 *
 * `reads` bumps after a write and re-runs every read that depends on it; nothing is spliced into a
 * loaded page by hand.
 */
export function CitizenDirectoryWorkspace({ canEdit }: { canEdit: boolean }) {
  const [tab, setTab] = useState<TabId>("contacts");
  const [query, setQuery] = useState<ListQuery>({ filter: NO_CITIZEN_CONTACT_FILTER, stack: TRANG_DAU });
  const [page, setPage] = useState<ContactPageState>({ phase: "loading" });
  const [count, setCount] = useState<number | null>(null);
  const [groups, setGroups] = useState<readonly CitizenGroup[] | null>(null);
  const [tags, setTags] = useState<readonly CitizenContactTag[]>([]);
  const [units, setUnits] = useState<readonly identity_thonToDanPhoRa[]>([]);
  const [reads, setReads] = useState(0);

  const [contactDialog, setContactDialog] = useState<ContactDialogMode | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [deleting, setDeleting] = useState<CitizenContact | null>(null);
  const [deleteReason, setDeleteReason] = useState("");
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deleteServerError, setDeleteServerError] = useState("");
  const [deleteBusy, setDeleteBusy] = useState(false);

  const reread = useCallback(() => setReads((n) => n + 1), []);

  useEffect(() => {
    let dropped = false;
    listCitizenGroups().then((kq) => {
      if (!dropped) setGroups(kq.ok ? kq.duLieu.items : null);
    });
    return () => {
      dropped = true;
    };
  }, [reads]);

  useEffect(() => {
    let dropped = false;
    listCitizenContactTags().then((kq) => {
      if (!dropped && kq.ok) setTags(kq.duLieu.items);
    });
    return () => {
      dropped = true;
    };
  }, [reads]);

  useEffect(() => {
    if (!canEdit) return;
    let dropped = false;
    layDanhSachThonToDanPho().then((kq) => {
      if (!dropped && kq.ok) setUnits(kq.duLieu.items);
    });
    return () => {
      dropped = true;
    };
  }, [canEdit]);

  useEffect(() => {
    let dropped = false;
    readCitizenContactPage(query.filter, query.stack.hienTai).then((kq) => {
      if (dropped) return;
      setPage(kq.ok ? { phase: "ready", page: kq.duLieu } : { phase: "error", message: kq.thongBao });
    });
    return () => {
      dropped = true;
    };
  }, [query, reads]);

  useEffect(() => {
    let dropped = false;
    countCitizenContacts(query.filter).then((kq) => {
      // A refusal leaves the bracket off — a 0 nobody counted would be a figure.
      if (!dropped) setCount(kq.ok ? kq.duLieu : null);
    });
    return () => {
      dropped = true;
    };
  }, [query.filter, reads]);

  /** `loading` is set HERE, in the event, not in the effect (React's lint refuses a sync setState there). */
  const changeQuery = useCallback((next: (cur: ListQuery) => ListQuery) => {
    setPage({ phase: "loading" });
    setDeleting(null);
    setQuery(next);
  }, []);

  const onFilterChange = useCallback(
    (change: Partial<CitizenContactFilter>) => {
      setCount(null);
      changeQuery((cur) => ({ filter: { ...cur.filter, ...change }, stack: TRANG_DAU }));
    },
    [changeQuery],
  );

  function openDelete(row: CitizenContact) {
    setDeleting(row);
    setDeleteReason("");
    setDeleteError(null);
    setDeleteServerError("");
  }

  async function confirmDelete() {
    if (deleting === null || deleteBusy) return;
    const problem = deleteReasonProblem(deleteReason);
    if (problem !== null) {
      setDeleteError(problem);
      return;
    }
    setDeleteError(null);
    setDeleteBusy(true);
    const kq = await deleteCitizenContact(deleting.id, deleteReason.trim());
    setDeleteBusy(false);
    if (!kq.ok) {
      setDeleteServerError(kq.thongBao);
      toast.error(REMOVE_FAILED_TOAST);
      return;
    }
    toast.success(removedToast(deleting.full_name));
    setDeleting(null);
    reread();
  }

  function onTabKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const at = TABS.findIndex((t) => t.id === tab);
    const last = TABS.length - 1;
    const next =
      e.key === "ArrowRight" ? (at === last ? 0 : at + 1)
      : e.key === "ArrowLeft" ? (at === 0 ? last : at - 1)
      : e.key === "Home" ? 0
      : e.key === "End" ? last
      : null;
    if (next === null) return;
    e.preventDefault();
    setTab(TABS[next]!.id);
    document.getElementById(citizenTabDomId(TABS[next]!.id))?.focus();
  }

  const counts: Record<TabId, number | null> = { contacts: count, groups: groups === null ? null : groups.length };

  return (
    <div className="min-w-0">
      <PageHeader
        title={CITIZEN_DIRECTORY_TITLE}
        subtitle={CITIZEN_DIRECTORY_SUBTITLE}
        className="mb-5"
        actions={
          canEdit ? (
            <>
              <Button type="button" variant="outline" icon={<Upload aria-hidden="true" />} onClick={() => setImportOpen(true)}>
                {IMPORT_OPEN_BUTTON}
              </Button>
              <Button
                type="button"
                variant="primary"
                icon={<Plus aria-hidden="true" />}
                onClick={() => setContactDialog({ kind: "add", idempotencyKey: crypto.randomUUID() })}
              >
                {ADD_CONTACT_BUTTON}
              </Button>
            </>
          ) : undefined
        }
      />

      <TabList aria-label="Phần của màn Danh bạ người dân" onKeyDown={onTabKeyDown}>
        {TABS.map((t) => (
          <Tab
            key={t.id}
            id={citizenTabDomId(t.id)}
            selected={t.id === tab}
            aria-controls={CITIZEN_PANEL_ID}
            tabIndex={t.id === tab ? 0 : -1}
            onClick={() => setTab(t.id)}
          >
            {tabLabel(t.label, counts[t.id])}
          </Tab>
        ))}
      </TabList>

      <div role="tabpanel" id={CITIZEN_PANEL_ID} aria-labelledby={citizenTabDomId(tab)} className="mt-4 min-w-0">
        {tab === "contacts" ? (
          <CitizenContactList
            filter={query.filter}
            groups={groups ?? []}
            tags={tags}
            state={page}
            pageNumber={query.stack.daQua.length + 1}
            hasPrev={coTrangTruoc(query.stack)}
            canEdit={canEdit}
            onFilterChange={onFilterChange}
            onPrev={() => changeQuery((cur) => ({ ...cur, stack: veTrangTruoc(cur.stack) }))}
            onNext={() => {
              if (page.phase !== "ready" || !page.page.has_more || page.page.next_cursor === "") return;
              const cursor = page.page.next_cursor;
              changeQuery((cur) => ({ ...cur, stack: sangTrangSau(cur.stack, cursor) }));
            }}
            onEdit={(row) => setContactDialog({ kind: "edit", contact: row })}
            onDelete={openDelete}
            confirm={
              deleting !== null ? (
                <CitizenContactDeleteDialog
                  contact={deleting}
                  reason={deleteReason}
                  setReason={(v) => {
                    setDeleteReason(v);
                    if (deleteError !== null && v.trim() !== "") setDeleteError(null);
                  }}
                  error={deleteError}
                  serverError={deleteServerError}
                  busy={deleteBusy}
                  onConfirm={() => void confirmDelete()}
                  onCancel={() => setDeleting(null)}
                />
              ) : undefined
            }
          />
        ) : (
          <CitizenGroupTab groups={groups} canEdit={canEdit} onChanged={reread} />
        )}
      </div>

      {contactDialog !== null && (
        <CitizenContactDialog
          // Keyed by the row: reopening the same person starts masked again (a new audited reveal).
          key={contactDialog.kind === "edit" ? contactDialog.contact.id : contactDialog.idempotencyKey}
          mode={contactDialog}
          groups={groups ?? []}
          units={units}
          canReveal={canEdit}
          onClose={() => setContactDialog(null)}
          onSaved={reread}
        />
      )}
      {importOpen && <CitizenContactImportDialog onClose={() => setImportOpen(false)} onImported={reread} />}
    </div>
  );
}
