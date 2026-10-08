"use client";

import { CircleCheck, ListChecks, LockKeyhole } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  FormGiaoViec, // vi-name-ok: existing export of so-nhiem-vu.tsx, imported not declared (rule 12 inv 3)
  type DanhMucNhiemVu, // vi-name-ok: existing type of so-nhiem-vu.tsx, imported not declared (rule 12 inv 3)
} from "@/features/nhiem-vu/so-nhiem-vu";
import { taskDetailHref } from "@/features/nhiem-vu/task-link";
import { createTaskFromLetter, letterTaskBody } from "@/lib/api/citizen-letter-task";
import {
  layDanhBaChonNguoi, // vi-name-ok: existing export, imported not declared (rule 12 inv 3)
} from "@/lib/api/danh-ba-chon-nguoi";
import {
  layDanhMucBoPhan, // vi-name-ok: existing export, imported not declared (rule 12 inv 3)
} from "@/lib/api/danh-muc";
import {
  layKhoiNhiemVu, // vi-name-ok: existing export, imported not declared (rule 12 inv 3)
  layLoaiNhiemVu, // vi-name-ok: existing export, imported not declared (rule 12 inv 3)
  layMucUuTienNhiemVu, // vi-name-ok: existing export, imported not declared (rule 12 inv 3)
} from "@/lib/api/danh-muc-nghiep-vu";
import type {
  KetQua, // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
} from "@/lib/api/goi";
import type {
  documents_citizenLetterOut,
  identity_danhBaChonNguoiRa, // vi-name-ok: generated contract type, imported not declared (rule 12 inv 3)
} from "@/lib/api/schema.gen";
import {
  QUYEN_DUYET_GIA_HAN, // vi-name-ok: existing constant of quyen.ts, imported not declared (rule 12 inv 3)
} from "@/lib/quyen";

import { Glyph } from "./document-ui";
import { dueDateText, drawerDueAt, letterNumber, NO_DEADLINE } from "./letter-display";

/** The drawer button and its sub-line (`PetitionDetailDrawer.tsx:598-602`), verbatim. */
export const RAISE_TASK_BUTTON = "Chuyển thành nhiệm vụ";
export const RAISE_TASK_HINT = "Nhiệm vụ kế thừa hạn xử lý của đơn, để hai bên không lệch nhau.";

/**
 * The prototype's `TaskFromRecordDialog` words (`:77-81`, `:162`). Its description's last clause ("sửa ở
 * đây thì hai bên bắt đầu lệch") is NOT kept: here the deadline cannot be edited — the server sets it
 * (ADR 0085 A6) — so the clause would describe a control that does not exist.
 */
export const RAISE_TASK_DIALOG_TITLE = "Tạo nhiệm vụ từ hồ sơ này";
export const RAISE_TASK_DIALOG_DESCRIPTION = "Xem lại rồi mới tạo. Hạn xử lý kế thừa từ hồ sơ để hai bên không lệch nhau.";
export const RAISE_TASK_SUBMIT = "Tạo nhiệm vụ";

/** The read-only deadline box: the prototype's label `Hạn xử lý` (`:135`), the owner's `Kế thừa hạn của đơn`. */
export const INHERITED_DUE_LABEL = "Hạn xử lý";
export const INHERITED_DUE_NOTE = "Kế thừa hạn của đơn";
export const INHERITED_NO_DUE_NOTE = "Đơn không đặt hạn nên nhiệm vụ cũng không có hạn.";

/**
 * The deadline the task WILL carry, as words: the date of the letter's current deadline at 17:00 — the
 * server's rule (`citizen_letter_task.go`: "ngày hạn, 17:00"), read from the same stored instant the
 * drawer's figure shows (`drawerDueAt` = documents' `CurrentStageDueAt`). Shown, never sent.
 */
export function inheritedDueText(
  letter: Pick<documents_citizenLetterOut, "accepted_at" | "processing_due_at" | "resolution_due_at">,
): string {
  const due = drawerDueAt(letter);
  return due === null || due === "" ? NO_DEADLINE : `17:00 ngày ${dueDateText(due)}`;
}

/** Where the source is said, as on Phản ánh's `Tạo nhiệm vụ` — nothing to choose, the server binds it. */
export function letterTaskSourceNote(number: number, year: number): string {
  return `Nguồn giao: Từ đơn thư số ${letterNumber(number, year)} — máy chủ gắn theo đơn, không sửa được.`;
}

/** After a 201: the code the server issued, which nobody could know beforehand. */
export function letterTaskCreated(taskCode: string): string {
  return `Đã tạo nhiệm vụ ${taskCode}.`;
}
export const OPEN_TASK_LINK_LABEL = "Mở nhiệm vụ";

const EMPTY_CATALOGUE: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };

/**
 * "Chuyển thành nhiệm vụ" of the citizen-letter drawer (prototype `PetitionDetailDrawer.tsx:590-627`) —
 * POST /api/v1/citizen-letter-tasks (ADR 0085 A).
 *
 * THE SAME FORM AS `+ Giao việc mới`, as Phản ánh's `Tạo nhiệm vụ` (`features/phan-anh/petition-task.tsx`)
 * reuses it for the same prototype dialog: `FormGiaoViec` with the prototype's title/description/submit,
 * the title pre-filled from the summary (ADR 0085 A7: typed, confirmed by a person), unit and assignee
 * from the letter's holding unit and assignee — all editable — and the deadline READ-ONLY (`inheritedDue`):
 * the server sets it and refuses a `due_at` (A6).
 *
 * THE DIALOG IS REBUILT ON EVERY OPENING (`open` mounts it): its values come from the letter, which may
 * have moved since; and `FormGiaoViec` mints its Idempotency-Key at mount, so each opening is one attempt
 * whose retries replay.
 *
 * The outcome is said INSIDE the drawer (it is a native modal; a toast would sit under it): the server's
 * refusal verbatim in the form, the new task's code with a link to `/nhiem-vu?task=<code>` on success.
 *
 * Drawn only when `showsRaiseTask` (keys, and never a denunciation); the route checks again.
 */
export function LetterTaskBlock({
  letter,
  staff,
  onCreated,
}: {
  letter: documents_citizenLetterOut;
  /** The register's whole-commune staff directory — the same answer, not a second read. `null` = loading. */
  staff: KetQua<identity_danhBaChonNguoiRa> | null;
  /** After a 201: the drawer re-reads the letter and its log. */
  onCreated: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [catalogue, setCatalogue] = useState<DanhMucNhiemVu>(EMPTY_CATALOGUE);
  const [leaders, setLeaders] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [createdCode, setCreatedCode] = useState<string | null>(null);

  // Catalogues and the leader directory on FIRST OPEN (most drawers never open this dialog).
  useEffect(() => {
    if (!open || loaded) return;
    let dropped = false;
    Promise.all([layLoaiNhiemVu(), layMucUuTienNhiemVu(), layKhoiNhiemVu(), layDanhMucBoPhan()]).then(
      ([types, priorities, blocs, units]) => {
        if (dropped) return;
        setCatalogue({
          loai: types.ok ? types.duLieu.items : [],
          mucUuTien: priorities.ok ? priorities.duLieu.items : [],
          khoi: blocs.ok ? blocs.duLieu.items : [],
          boPhan: units.ok ? units.duLieu.items : [],
        });
        setLoaded(true);
      },
    );
    // `Lãnh đạo giao việc` offers only `task.extend` holders (ADR 0038), never the whole commune.
    layDanhBaChonNguoi(undefined, QUYEN_DUYET_GIA_HAN).then((k) => {
      if (!dropped) setLeaders(k);
    });
    return () => {
      dropped = true;
    };
  }, [open, loaded]);

  const noDue = drawerDueAt(letter) === null || drawerDueAt(letter) === "";

  return (
    <div className="mt-4">
      <Button
        type="button"
        variant="primary"
        icon={<Glyph icon={ListChecks} />}
        aria-haspopup="dialog"
        disabled={sending}
        onClick={() => {
          setError(null);
          setCreatedCode(null);
          setOpen(true);
        }}
      >
        {RAISE_TASK_BUTTON}
      </Button>
      <p className="m-0 mt-1.5 text-[11px] text-ink-muted">{RAISE_TASK_HINT}</p>

      {createdCode !== null && (
        <p role="status" className="m-0 mt-2 flex flex-wrap items-center gap-1.5 text-[12.5px] font-medium text-success-600">
          <Glyph icon={CircleCheck} className="size-4 shrink-0" />
          {letterTaskCreated(createdCode)}
          <a href={taskDetailHref(createdCode)} className="text-brand underline">
            {OPEN_TASK_LINK_LABEL}
          </a>
        </p>
      )}

      {open && (
        <FormGiaoViec
          dialog
          dialogTitle={RAISE_TASK_DIALOG_TITLE}
          dialogDescription={RAISE_TASK_DIALOG_DESCRIPTION}
          submitLabel={RAISE_TASK_SUBMIT}
          lead={
            <p className="m-0 inline-flex items-center gap-1.5 text-[12px] text-ink-muted">
              <Glyph icon={LockKeyhole} className="size-3.5 shrink-0" />
              {letterTaskSourceNote(letter.number, letter.year)}
            </p>
          }
          danhMuc={catalogue}
          danhBa={staff}
          danhBaLanhDao={leaders}
          // The route takes `documents` and `note` (`petitions_citizenLetterTaskIn`): the lists are drawn.
          coDanhSachVanBan
          dangGui={sending}
          loi={error}
          huy={() => {
            if (sending) return;
            setOpen(false);
            setError(null);
          }}
          tieuDeCoSan={letter.summary ?? ""}
          initialUnit={letter.holding_unit_id ?? ""}
          initialAssignee={letter.assignee_code ?? ""}
          inheritedDue={
            <div>
              <span className="mb-1.5 block text-[12.5px] font-semibold text-ink">{INHERITED_DUE_LABEL}</span>
              <p className="m-0 text-[13px] font-semibold text-navy tabular-nums">{inheritedDueText(letter)}</p>
              <p className="m-0 mt-1 text-[11.5px] text-ink-muted">{noDue ? INHERITED_NO_DUE_NOTE : INHERITED_DUE_NOTE}</p>
            </div>
          }
          giaoViec={(form, key) => {
            setSending(true);
            setError(null);
            // `letterTaskBody` copies the shared keys only: no `due_at`, `source`, `source_id`.
            void createTaskFromLetter(letterTaskBody(letter.id, form), key).then((k) => {
              setSending(false);
              if (!k.ok) {
                // VERBATIM: 404, 422 (`denunciation_no_task`, `assignment_required`), 503 — nothing written.
                setError(k.thongBao);
                return;
              }
              setCreatedCode(k.duLieu.code);
              setOpen(false);
              onCreated();
            });
          }}
        />
      )}
    </div>
  );
}
