"use client";

import { Pencil, Trash2 } from "lucide-react";
import {
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useState,
  type KeyboardEvent,
} from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { letterTypeLabel } from "@/features/van-ban/letter-display";
import {
  createCitizenLetterDeadlineRule,
  readCitizenLetterDeadlineRules,
  removeCitizenLetterDeadlineRule,
  updateCitizenLetterDeadlineRule,
} from "@/lib/api/citizen-letter-deadline-rules";
// vi-name-ok: imports the existing type of goi.ts unchanged (rule 12 invariant 3)
import type { KetQua } from "@/lib/api/goi";
import type { identity_citizenLetterDeadlineRulesOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  AMOUNT_LABEL,
  BLOCK_HELP,
  BLOCK_TITLE,
  COLUMN_DEADLINE,
  COLUMN_DEADLINE_KIND,
  COLUMN_LETTER_TYPE,
  EDIT_DEADLINE_TITLE,
  LOADING_LABEL,
  LOAD_ERROR_TITLE,
  NOT_SET,
  REMOVED_DEADLINE,
  REMOVE_DEADLINE_TITLE,
  composeSave,
  deadlineKindLabel,
  deadlineRows,
  unitLabel,
  type DeadlineRow,
} from "./citizen-letter-deadline";
import { ConfigLoading, ConfigTable, RowActions } from "./config-ui";
// vi-name-ok: imports the existing exports of nhan-thoi-han.ts / sua-thoi-han.ts unchanged (rule 12 invariant 3)
import { DA_LUU_THOI_HAN, NUT_HUY, NUT_LUU } from "./nhan-thoi-han";
import { RemoveStep } from "./remove-reason-step";
import { composeRemoveReason } from "./sua-thoi-han";
import type { InlineRemove } from "./tab-thoi-han-xu-ly";

/**
 * Block "Thời hạn giải quyết đơn thư" of tab "Thời hạn xử lý" (ADR 0084 #3, ADR 0085 B, câu 2–4).
 *
 * NO PROTOTYPE EQUIVALENT: `../vigov-require/apps/admin/src/components/admin/SlaTable.tsx` has no
 * citizen-letter rows. So this block borrows the neighbouring SLA table's pieces as they are —
 * `ConfigTable`, `RowActions`, edit IN PLACE with Lưu/Huỷ, the reason step of `RemoveStep`, toasts,
 * both error slots under the row — and adds no visual language of its own.
 *
 * SIX ROWS ALWAYS, configured or not: a pair without a rule reads "Không đặt hạn", which is what letters
 * of that type get at booking (ADR 0085 B3, câu 4 — nothing is seeded). Saving a "Không đặt hạn" row
 * creates the rule; saving a set row updates it; clearing asks a reason, then soft-deletes (rule 7).
 *
 * THE UNIT IS NEVER A CHOICE: the server locks one unit per pair (`required_unit`); the screen shows it.
 *
 * HIDDEN WITHOUT `admin.sla` (and while the session is unread). That is UX: all four routes declare
 * `RequirePermission("admin.sla")` and check every request (rule 5, forbidden #1).
 */
export function CitizenLetterDeadlineBlock({
  canWrite,
}: {
  canWrite: boolean;
}) {
  if (!canWrite) return null;
  return <CitizenLetterDeadlineLoader />;
}

/** The open in-place edit of one row; `idempotencyKey` is kept across retries of one create. */
type EditState = {
  readonly key: string;
  readonly text: string;
  readonly idempotencyKey: string;
};

function CitizenLetterDeadlineLoader() {
  const [readCount, setReadCount] = useState(0);
  const [rules, setRules] =
    useState<KetQua<identity_citizenLetterDeadlineRulesOut> | null>(null);
  const [busy, setBusy] = useState(false);

  const [editing, setEditing] = useState<EditState | null>(null);
  const [editLocalError, setEditLocalError] = useState("");
  const [editServerError, setEditServerError] = useState("");

  const [removing, setRemoving] = useState<{
    key: string;
    reason: string;
  } | null>(null);
  const [removeLocalError, setRemoveLocalError] = useState("");
  const [removeServerError, setRemoveServerError] = useState("");

  useEffect(() => {
    let dropped = false;
    void readCitizenLetterDeadlineRules().then((result) => {
      if (!dropped) setRules(result);
    });
    return () => {
      dropped = true;
    };
  }, [readCount]);

  const rows = useMemo(
    () => (rules !== null && rules.ok ? deadlineRows(rules.duLieu.items) : []),
    [rules],
  );

  const cancelEdit = useCallback(() => {
    setEditing(null);
    setEditLocalError("");
    setEditServerError("");
  }, []);

  const cancelRemove = useCallback(() => {
    setRemoving(null);
    setRemoveLocalError("");
    setRemoveServerError("");
  }, []);

  const startEdit = useCallback(
    (row: DeadlineRow) => {
      // One row in one mode at a time, as in the SLA table.
      cancelRemove();
      setEditing({
        key: row.key,
        text: row.rule !== null ? String(row.rule.amount) : "",
        idempotencyKey: crypto.randomUUID(),
      });
      setEditLocalError("");
      setEditServerError("");
    },
    [cancelRemove],
  );

  const startRemove = useCallback(
    (row: DeadlineRow) => {
      cancelEdit();
      setRemoving({ key: row.key, reason: "" });
      setRemoveLocalError("");
      setRemoveServerError("");
    },
    [cancelEdit],
  );

  const save = useCallback(() => {
    if (editing === null || busy) return;
    const row = rows.find((r) => r.key === editing.key);
    if (row === undefined) return;
    setEditServerError("");
    const composed = composeSave(row, editing.text);
    if (!composed.ok) {
      setEditLocalError(composed.error);
      return;
    }
    setEditLocalError("");
    setBusy(true);
    const sent =
      composed.action === "create"
        ? createCitizenLetterDeadlineRule(composed.body, editing.idempotencyKey)
        : updateCitizenLetterDeadlineRule(composed.id, composed.body);
    void sent.then((result) => {
      setBusy(false);
      if (!result.ok) {
        // The server's sentence, verbatim — the 1..365 ceiling and the unit lock included.
        setEditServerError(result.thongBao);
        return;
      }
      setEditing(null);
      toast.success(DA_LUU_THOI_HAN);
      setReadCount((n) => n + 1);
    });
  }, [busy, editing, rows]);

  const confirmRemove = useCallback(() => {
    if (removing === null || busy) return;
    const row = rows.find((r) => r.key === removing.key);
    if (row === undefined || row.rule === null) return;
    setRemoveServerError("");
    const composed = composeRemoveReason(removing.reason);
    if (!composed.ok) {
      setRemoveLocalError(composed.error);
      return;
    }
    setRemoveLocalError("");
    setBusy(true);
    void removeCitizenLetterDeadlineRule(row.rule.id, composed.reason).then(
      (result) => {
        setBusy(false);
        if (!result.ok) {
          setRemoveServerError(result.thongBao);
          return;
        }
        setRemoving(null);
        toast.success(REMOVED_DEADLINE);
        setReadCount((n) => n + 1);
      },
    );
  }, [busy, removing, rows]);

  return (
    <CitizenLetterDeadlineView
      rules={rules}
      busy={busy}
      actions={{ startEdit, startRemove }}
      edit={
        editing === null
          ? null
          : {
              rowKey: editing.key,
              text: editing.text,
              localError: editLocalError,
              serverError: editServerError,
              busy,
              setText: (text) =>
                setEditing((open) =>
                  open === null ? null : { ...open, text },
                ),
              onSave: save,
              onCancel: cancelEdit,
            }
      }
      remove={
        removing === null
          ? null
          : {
              rowId: removing.key,
              reason: removing.reason,
              localError: removeLocalError,
              serverError: removeServerError,
              busy,
              setReason: (reason) =>
                setRemoving((open) =>
                  open === null ? null : { ...open, reason },
                ),
              onConfirm: confirmRemove,
              onCancel: cancelRemove,
            }
      }
    />
  );
}

/** The two acts a row's buttons ask for. */
export type CitizenLetterDeadlineActions = {
  readonly startEdit: (row: DeadlineRow) => void;
  readonly startRemove: (row: DeadlineRow) => void;
};

/** The row whose amount is being typed. Two error slots, never merged (as `InlineEdit`). */
export type AmountEdit = {
  readonly rowKey: string;
  readonly text: string;
  readonly localError: string;
  readonly serverError: string;
  readonly busy: boolean;
  readonly setText: (text: string) => void;
  readonly onSave: () => void;
  readonly onCancel: () => void;
};

/**
 * The block, PURE PRESENTATION — exported so a test renders it without a network. `remove.rowId` is the
 * row's `key` (the rule id: only a set row can be cleared).
 */
export function CitizenLetterDeadlineView({
  rules,
  busy,
  actions,
  edit = null,
  remove = null,
}: {
  rules: KetQua<identity_citizenLetterDeadlineRulesOut> | null;
  busy: boolean;
  actions: CitizenLetterDeadlineActions;
  edit?: AmountEdit | null;
  remove?: InlineRemove | null;
}) {
  return (
    <section
      className="space-y-3"
      aria-labelledby="citizen-letter-deadline-title"
    >
      <div>
        <h3
          id="citizen-letter-deadline-title"
          className="text-navy m-0 text-[13px] font-bold"
        >
          {BLOCK_TITLE}
        </h3>
        <p className="text-ink-muted m-0 mt-0.5 text-[12px]">{BLOCK_HELP}</p>
      </div>

      {rules === null && <ConfigLoading label={LOADING_LABEL} />}
      {rules !== null && !rules.ok && (
        <ErrorState
          role="alert"
          title={LOAD_ERROR_TITLE}
          message={rules.thongBao}
          className="py-6"
        />
      )}
      {rules !== null && rules.ok && (
        <DeadlineTable
          rows={deadlineRows(rules.duLieu.items)}
          busy={busy}
          actions={actions}
          edit={edit}
          remove={remove}
        />
      )}
    </section>
  );
}

const COLUMN_COUNT = 4;

function DeadlineTable({
  rows,
  busy,
  actions,
  edit,
  remove,
}: {
  rows: readonly DeadlineRow[];
  busy: boolean;
  actions: CitizenLetterDeadlineActions;
  edit: AmountEdit | null;
  remove: InlineRemove | null;
}) {
  return (
    <ConfigTable
      label={BLOCK_TITLE}
      caption="Số ngày của từng loại hạn theo loại đơn của đơn vị"
    >
      <thead>
        <tr>
          <th scope="col">{COLUMN_LETTER_TYPE}</th>
          <th scope="col">{COLUMN_DEADLINE_KIND}</th>
          <th scope="col">{COLUMN_DEADLINE}</th>
          <th scope="col">
            <span className="an-thi-giac">Thao tác</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => {
          const typeText = letterTypeLabel(row.letterType);
          const kindText = deadlineKindLabel(row.deadlineKind);
          const rowName = `${typeText} — ${kindText}`;
          const editing =
            edit !== null && edit.rowKey === row.key ? edit : null;
          // Only a set row can be cleared, even if a stale state names another.
          const removing =
            editing === null &&
            row.rule !== null &&
            remove !== null &&
            remove.rowId === row.key
              ? remove
              : null;
          const notes = [
            row.rule?.problem ?? "",
            ...(editing !== null
              ? [editing.localError, editing.serverError]
              : removing !== null
                ? [removing.localError, removing.serverError]
                : []),
          ].filter((e) => e !== "");
          return (
            <Fragment key={row.key}>
              <tr>
                <td className="text-navy font-medium">{typeText}</td>
                <td>{kindText}</td>
                <td>
                  {editing !== null ? (
                    <AmountInput
                      edit={editing}
                      unit={row.unit}
                      rowName={rowName}
                    />
                  ) : row.rule !== null ? (
                    <span className="font-semibold">{`${row.rule.amount} ${unitLabel(row.rule.unit)}`}</span>
                  ) : (
                    <span className="text-ink-muted">{NOT_SET}</span>
                  )}
                </td>
                <td>
                  <RowActions>
                    {editing !== null ? (
                      <>
                        <Button
                          type="button"
                          variant="primary"
                          size="sm"
                          disabled={editing.busy}
                          aria-busy={editing.busy}
                          onClick={editing.onSave}
                        >
                          <BusyLabel
                            busy={editing.busy}
                            label={NUT_LUU}
                            busyText={BUSY_SAVING}
                          />
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          disabled={editing.busy}
                          onClick={editing.onCancel}
                        >
                          {NUT_HUY}
                        </Button>
                      </>
                    ) : removing !== null ? (
                      <RemoveStep remove={removing} rowName={rowName} />
                    ) : (
                      <>
                        <Button
                          type="button"
                          variant="secondary"
                          size="sm"
                          title={EDIT_DEADLINE_TITLE}
                          aria-label={`${EDIT_DEADLINE_TITLE} ${rowName}`}
                          disabled={busy}
                          onClick={() => actions.startEdit(row)}
                        >
                          <Pencil
                            aria-hidden="true"
                            focusable="false"
                            className="size-3.5"
                          />
                        </Button>
                        {row.rule !== null && (
                          <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            className="text-danger"
                            title={REMOVE_DEADLINE_TITLE}
                            aria-label={`${REMOVE_DEADLINE_TITLE} ${rowName}`}
                            disabled={busy}
                            onClick={() => actions.startRemove(row)}
                          >
                            <Trash2
                              aria-hidden="true"
                              focusable="false"
                              className="size-3.5"
                            />
                          </Button>
                        )}
                      </>
                    )}
                  </RowActions>
                </td>
              </tr>
              {notes.length > 0 && (
                <tr>
                  <td colSpan={COLUMN_COUNT}>
                    <div className="space-y-1 whitespace-normal">
                      {notes.map((e, i) => (
                        <p
                          key={i}
                          role="alert"
                          className="text-danger m-0 text-[12.5px]"
                        >
                          {e}
                        </p>
                      ))}
                    </div>
                  </td>
                </tr>
              )}
            </Fragment>
          );
        })}
      </tbody>
    </ConfigTable>
  );
}

/** The amount box in edit mode, the locked unit beside it as text. Enter saves, Esc cancels. */
function AmountInput({
  edit,
  unit,
  rowName,
}: {
  edit: AmountEdit;
  unit: string;
  rowName: string;
}) {
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      edit.onSave();
    } else if (e.key === "Escape") {
      e.preventDefault();
      edit.onCancel();
    }
  };
  return (
    <span className="inline-flex items-center gap-2">
      <input
        type="number"
        min={1}
        step={1}
        name="amount"
        autoFocus
        aria-label={`${AMOUNT_LABEL} — ${rowName}`}
        aria-invalid={edit.localError !== ""}
        className={cn(controlClass, "h-8 w-20 text-[12.5px]")}
        value={edit.text}
        disabled={edit.busy}
        // A box the browser cannot parse reports "" — keep it a refusal, never "nothing typed".
        onChange={(e) =>
          edit.setText(
            e.currentTarget.validity.badInput ? "?" : e.currentTarget.value,
          )
        }
        onKeyDown={onKeyDown}
      />
      <span className="text-ink-muted text-[12.5px]">{unitLabel(unit)}</span>
    </span>
  );
}
