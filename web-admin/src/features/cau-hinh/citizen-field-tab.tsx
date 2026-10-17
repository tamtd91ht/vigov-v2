"use client";

import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  MouseSensor,
  TouchSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type Announcements,
} from "@dnd-kit/core";
import { ArrowDown, ArrowUp, Ban, ChevronDown, ChevronRight, GripVertical, Pencil, Plus, RotateCcw, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { NoAccess } from "@/components/ui/no-access";
import { Skeleton } from "@/components/ui/skeleton";
import { usePhien } from "@/features/phien/phien-hien-tai";
import {
  CITIZEN_FIELD_CODE_TAKEN,
  CITIZEN_FIELD_LIMIT,
  createCitizenField,
  listCitizenFields,
  reorderCitizenFields,
  retireCitizenField,
  updateCitizenField,
  type CitizenField,
} from "@/lib/api/citizen-fields";
// vi-name-ok: the existing result type and sentence of lib/api/goi.ts, imported, not a new name
import { LOI_KHONG_RO, type KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";

import {
  ADD_FIELD_BUTTON,
  ADD_FIELD_TITLE,
  CITIZEN_FIELD_DESCRIPTION,
  CODE_EDIT_BUTTON,
  CODE_HINT,
  CODE_LOCKED_HINT,
  CODE_REDRAW,
  CODE_REDRAWN_AFTER_CONFLICT,
  CITIZEN_FIELD_TAB_TITLE,
  DATE_PATTERNS,
  EDIT_FIELD_TITLE,
  FIELD_ADDED_TOAST,
  FIELD_RETIRED_TOAST,
  FIELD_SAVED_TOAST,
  FIELD_TYPES,
  NO_FIELDS,
  NO_PERMISSION,
  NUMBER_KINDS,
  OPTIONS_HINT,
  ORDER_FAILED_TOAST,
  ORDER_SAVED_TOAST,
  RETIRE_CONSEQUENCE,
  RETIRE_REASON_LABEL,
  RETIRE_REASON_MAX,
  RETIRED_SECTION,
  addOption,
  buildCreateBody,
  buildUpdateBody,
  checkRetireReason,
  draftFromField,
  fieldCountText,
  fieldTypeLabel,
  freeRandomFieldCode,
  hasErrors,
  hasOptions,
  hasPlaceholder,
  isCodeCollision,
  limitReachedText,
  manualCodeError,
  moveCode,
  newFieldDraft,
  orderAfterDrop,
  positionText,
  restoreOption,
  retireOrRemoveOption,
  setFieldType,
  setOptionLabel,
  setOptionValue,
  splitFields,
  validateFieldDraft,
  type CitizenFieldDraft,
  type DatePattern,
  type FieldFormErrors,
} from "./citizen-field-form";
import { ConfigDialog } from "./config-dialog";
import { citizenFieldTabDecision } from "./quyen-tab";

export const SAVE_FAILED = "Chưa lưu được. Vui lòng thử lại.";


// Same card as Kênh SMS / Kênh Zalo.
const SECTION = "border-line shadow-card rounded-card m-0 min-w-0 border border-solid bg-white p-5";
const TITLE = "text-navy m-0 text-[14px] font-bold";
const DESCRIPTION = "text-ink-muted m-0 mt-1 max-w-2xl text-[12.5px]";
const FIELD_LABEL = "text-navy m-0 block text-[13px] leading-5 font-semibold";
const HINT = "text-ink-muted m-0 text-[12px]";
const ERROR = "text-danger m-0 text-[12px] font-medium";
const DANGER_OUTLINE = "border-danger/30 text-danger hover:not-disabled:bg-danger/5 bg-white";
const inputCls = cn(controlClass, "bg-white");
const selectCls = cn(controlClass, "bg-white pr-8");

/** The drag instructions a screen reader hears on a handle (dnd-kit's own are English). */
const DRAG_INSTRUCTIONS = {
  draggable:
    "Nhấn Space hoặc Enter để nhấc thuộc tính, dùng mũi tên lên / xuống để di chuyển, nhấn Space hoặc Enter lần nữa để thả, Esc để huỷ. Cũng có thể dùng nút Lên / Xuống ở mỗi dòng.",
};

function announcements(labelOf: (code: string) => string): Announcements {
  return {
    onDragStart: ({ active }) => `Đã nhấc thuộc tính ${labelOf(String(active.id))}.`,
    onDragOver: ({ active, over }) =>
      over === null ? `Thuộc tính ${labelOf(String(active.id))} không ở trên dòng nào.` : `Đang ở vị trí của ${labelOf(String(over.id))}.`,
    onDragEnd: ({ active, over }) =>
      over === null ? `Đã thả ${labelOf(String(active.id))}, thứ tự không đổi.` : `Đã thả ${labelOf(String(active.id))} vào vị trí của ${labelOf(String(over.id))}.`,
    onDragCancel: ({ active }) => `Đã huỷ kéo ${labelOf(String(active.id))}, thứ tự không đổi.`,
  };
}

type DialogState = { mode: "add"; idempotencyKey: string } | { mode: "edit"; field: CitizenField };

/**
 * "Cấu hình → Thuộc tính danh bạ" (ADR 0101 #1–#5): the commune's own fields of the citizen directory.
 * One key, `admin.citizen_field`, opens the tab (convenience: identity checks it on every write, rule 5
 * forbidden #1). NO PROTOTYPE: the card, the dialog and the reasoned "Ngừng dùng" follow Kênh SMS; the
 * options editor follows Trường bản đồ.
 *
 * THE ORDER IS THE FORM'S ORDER (#3). A drag (mouse, touch, keyboard via dnd-kit) or the Lên / Xuống buttons
 * move a row; the new order shows at once and `PUT` saves the whole live list. A refusal puts the old
 * order back and says so — the screen never shows an order the server does not hold.
 *
 * RETIRED FIELDS STAY LISTED, collapsed under "Đã ngừng dùng": their values are still on every person who
 * had them (#4, rule 7), and their codes stay taken.
 */
export function CitizenFieldTab() {
  const phien = usePhien();
  const decision = phien === null ? null : citizenFieldTabDecision(phien);
  const allowed = decision !== null && decision.hien;

  const [fields, setFields] = useState<KetQua<CitizenField[]> | null>(null);
  /** The live order while a reorder is being saved — shown at once, dropped on success or refusal. */
  const [pendingOrder, setPendingOrder] = useState<readonly string[] | null>(null);
  const [dialog, setDialog] = useState<DialogState | null>(null);
  const [retiring, setRetiring] = useState<CitizenField | null>(null);
  const [retiredOpen, setRetiredOpen] = useState(false);

  const reload = useCallback(async () => {
    const r = await listCitizenFields(true);
    setFields(r.ok ? { ok: true, duLieu: r.duLieu.items } : r);
  }, []);

  useEffect(() => {
    if (!allowed) return;
    let gone = false;
    listCitizenFields(true).then((r) => {
      if (!gone) setFields(r.ok ? { ok: true, duLieu: r.duLieu.items } : r);
    });
    return () => {
      gone = true;
    };
  }, [allowed]);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p className={cn(ERROR, "text-[12.5px]")} role="alert">
        {decision.thongBao}
      </p>
    ) : (
      <div className="flex min-w-0 flex-col items-center pb-10">
        <NoAccess className="pb-4" />
        <p className="text-ink-muted m-0 max-w-md px-4 text-center text-[13px]" data-citizen-field-denied="">
          {NO_PERMISSION}
        </p>
      </div>
    );
  }
  if (fields === null) {
    return (
      <>
        <p role="status" className="an-thi-giac">
          Đang tải thuộc tính danh bạ…
        </p>
        <Skeleton className="h-48 w-full" />
      </>
    );
  }
  if (!fields.ok) return <ErrorState role="alert" title="Chưa tải được thuộc tính danh bạ" message={fields.thongBao} />;

  const { live: serverLive, retired } = splitFields(fields.duLieu);
  const byCode = new Map(fields.duLieu.map((f) => [f.code, f]));
  const live =
    pendingOrder === null ? serverLive : pendingOrder.map((c) => byCode.get(c)).filter((f): f is CitizenField => f !== undefined);
  const codes = live.map((f) => f.code);
  const full = live.length >= CITIZEN_FIELD_LIMIT;

  async function saveOrder(next: readonly string[]) {
    if (pendingOrder !== null || next === codes) return;
    setPendingOrder(next);
    const r = await reorderCitizenFields(next);
    if (!r.ok) {
      setPendingOrder(null);
      toast.error(r.thongBao === LOI_KHONG_RO ? ORDER_FAILED_TOAST : `${ORDER_FAILED_TOAST} ${r.thongBao}`);
      return;
    }
    toast.success(ORDER_SAVED_TOAST);
    await reload();
    setPendingOrder(null);
  }

  const addButton = (
    <Button
      type="button"
      variant="primary"
      disabled={full}
      icon={<Plus aria-hidden="true" focusable="false" />}
      onClick={() => setDialog({ mode: "add", idempotencyKey: crypto.randomUUID() })}
    >
      {ADD_FIELD_BUTTON}
    </Button>
  );

  return (
    <div className="flex min-w-0 flex-col gap-4" data-citizen-fields="">
      <section className={SECTION} aria-labelledby="citizen-field-title">
        <div className="flex flex-wrap items-start gap-3">
          <div className="min-w-0 flex-1">
            <h3 id="citizen-field-title" className={TITLE}>
              {CITIZEN_FIELD_TAB_TITLE}
            </h3>
            <p className={DESCRIPTION}>{CITIZEN_FIELD_DESCRIPTION}</p>
            <p className={cn(HINT, "mt-2")} data-field-count="">
              {fieldCountText(live.length, CITIZEN_FIELD_LIMIT)}
            </p>
          </div>
          {addButton}
        </div>
        {full && (
          <p className={cn(HINT, "mt-2")} data-limit-reached="">
            {limitReachedText(CITIZEN_FIELD_LIMIT)}
          </p>
        )}

        {live.length === 0 ? (
          <p className="text-ink-muted m-0 mt-4 text-[12.5px]" data-fields-empty="">
            {NO_FIELDS}
          </p>
        ) : (
          <LiveFieldList
            fields={live}
            busy={pendingOrder !== null}
            onMove={(from, to) => void saveOrder(moveCode(codes, from, to))}
            onDrop={(active, over) => void saveOrder(orderAfterDrop(codes, active, over))}
            onEdit={(f) => setDialog({ mode: "edit", field: f })}
            onRetire={setRetiring}
          />
        )}
      </section>

      {retired.length > 0 && (
        <section className={SECTION} aria-label={RETIRED_SECTION}>
          <button
            type="button"
            className="text-navy m-0 flex cursor-pointer items-center gap-1.5 border-0 bg-transparent p-0 text-[13px] font-semibold"
            aria-expanded={retiredOpen}
            aria-controls="citizen-fields-retired"
            onClick={() => setRetiredOpen(!retiredOpen)}
          >
            {retiredOpen ? (
              <ChevronDown aria-hidden="true" focusable="false" className="size-4" />
            ) : (
              <ChevronRight aria-hidden="true" focusable="false" className="size-4" />
            )}
            {`${RETIRED_SECTION} (${retired.length})`}
          </button>
          {retiredOpen && (
            <ul id="citizen-fields-retired" className="m-0 mt-3 flex list-none flex-col gap-2 p-0" data-retired-fields="">
              {retired.map((f) => (
                <li key={f.code} className="border-line flex flex-wrap items-center gap-2 rounded-[10px] border border-solid px-3 py-2 opacity-70">
                  <span className="text-navy text-[13px] font-semibold">{f.label}</span>
                  <code className="text-ink-muted text-[11.5px]">{f.code}</code>
                  <span className="text-ink-muted text-[12px]">{fieldTypeLabel(f.field_type)}</span>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}

      {dialog !== null && (
        <CitizenFieldDialog
          key={dialog.mode === "edit" ? dialog.field.code : dialog.idempotencyKey}
          state={dialog}
          takenCodes={fields.duLieu.map((f) => f.code)}
          retiredCodes={retired.map((f) => f.code)}
          onClose={(saved) => {
            setDialog(null);
            if (saved) void reload();
          }}
        />
      )}

      {retiring !== null && (
        <RetireFieldDialog
          field={retiring}
          onClose={(done) => {
            setRetiring(null);
            if (done) void reload();
          }}
        />
      )}
    </div>
  );
}

/* ---- the live list ---------------------------------------------------------------------------------- */

/**
 * The live fields as an ordered list — not a table, so each row wraps at 320px instead of scrolling. Each
 * row is a drop target; its grip is the drag handle (dnd-kit's keyboard sensor makes it reachable by Tab,
 * Space, arrows). The Lên / Xuống buttons do the same move in one press, for anyone a drag does not suit.
 */
export function LiveFieldList({
  fields,
  busy,
  onMove,
  onDrop,
  onEdit,
  onRetire,
}: {
  fields: readonly CitizenField[];
  /** A reorder is being saved: every move control waits, so two orders never race. */
  busy: boolean;
  onMove: (from: number, to: number) => void;
  onDrop: (active: string, over: string | null) => void;
  onEdit: (f: CitizenField) => void;
  onRetire: (f: CitizenField) => void;
}) {
  const sensors = useSensors(
    // Mouse, not Pointer: a pointer sensor would race the touch sensor for one finger (`so-nhiem-vu.tsx`).
    useSensor(MouseSensor, { activationConstraint: { distance: 6 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 6 } }),
    useSensor(KeyboardSensor),
  );
  const labelOf = (code: string) => fields.find((f) => f.code === code)?.label ?? code;

  return (
    <DndContext
      // A fixed id keeps dnd-kit's `aria-describedby` the same on the server and in the browser.
      id="citizen-field-order"
      sensors={sensors}
      collisionDetection={closestCenter}
      accessibility={{ announcements: announcements(labelOf), screenReaderInstructions: DRAG_INSTRUCTIONS }}
      onDragEnd={(e) => {
        if (busy) return;
        onDrop(String(e.active.id), e.over === null ? null : String(e.over.id));
      }}
    >
      <ol className="m-0 mt-4 flex list-none flex-col gap-2 p-0" aria-label="Thứ tự thuộc tính trên biểu mẫu" data-live-fields="">
        {fields.map((f, i) => (
          <FieldRow
            key={f.code}
            field={f}
            index={i}
            total={fields.length}
            busy={busy}
            onUp={() => onMove(i, i - 1)}
            onDown={() => onMove(i, i + 1)}
            onEdit={() => onEdit(f)}
            onRetire={() => onRetire(f)}
          />
        ))}
      </ol>
    </DndContext>
  );
}

function FieldRow({
  field: f,
  index,
  total,
  busy,
  onUp,
  onDown,
  onEdit,
  onRetire,
}: {
  field: CitizenField;
  index: number;
  total: number;
  busy: boolean;
  onUp: () => void;
  onDown: () => void;
  onEdit: () => void;
  onRetire: () => void;
}) {
  const { setNodeRef: setDropRef, isOver } = useDroppable({ id: f.code });
  const drag = useDraggable({ id: f.code, disabled: busy });
  const { setNodeRef: setDragNode, setActivatorNodeRef } = drag;
  // Stable callbacks, so a re-render during the drag does not detach the nodes dnd-kit measures
  // (same shape as the Kanban card, `so-nhiem-vu.tsx`).
  const setRowRef = useCallback((node: HTMLElement | null) => setDropRef(node), [setDropRef]);
  const setBodyRef = useCallback((node: HTMLElement | null) => setDragNode(node), [setDragNode]);
  const setHandleRef = useCallback((node: HTMLElement | null) => setActivatorNodeRef(node), [setActivatorNodeRef]);
  const t = drag.transform;
  return (
    <li
      ref={setRowRef}
      data-field-row={f.code}
      className={cn(
        "border-line flex flex-wrap items-center gap-2 rounded-[10px] border border-solid bg-white px-3 py-2",
        isOver && !drag.isDragging && "border-brand bg-brand/5",
      )}
    >
      <div
        ref={setBodyRef}
        style={t === null ? undefined : { transform: `translate3d(${t.x}px, ${t.y}px, 0)` }}
        className={cn("flex min-w-0 flex-1 flex-wrap items-center gap-2", drag.isDragging && "relative z-10 opacity-80")}
      >
        <button
          ref={setHandleRef}
          type="button"
          className="text-ink-muted flex size-7 shrink-0 cursor-grab touch-none items-center justify-center rounded-md border-0 bg-transparent p-0 disabled:cursor-not-allowed"
          aria-label={positionText(f.label, index, total)}
          disabled={busy}
          {...drag.listeners}
          {...drag.attributes}
        >
          <GripVertical aria-hidden="true" focusable="false" className="size-4" />
        </button>
        <span className="text-navy min-w-0 text-[13px] font-semibold break-words">{f.label}</span>
        <code className="text-ink-muted text-[11.5px]">{f.code}</code>
        <span className="text-ink-muted text-[12px]">{fieldTypeLabel(f.field_type)}</span>
        {f.required && (
          <Badge tone="info" icon={null}>
            Bắt buộc
          </Badge>
        )}
        {f.show_in_list && (
          <Badge tone="neutral" icon={null}>
            Hiện ở danh sách
          </Badge>
        )}
      </div>
      <div className="flex items-center gap-1.5 [&>button]:min-h-0">
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={`Đưa ${f.label} lên`}
          disabled={busy || index === 0}
          onClick={onUp}
        >
          <ArrowUp aria-hidden="true" focusable="false" className="size-3.5" />
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={`Đưa ${f.label} xuống`}
          disabled={busy || index === total - 1}
          onClick={onDown}
        >
          <ArrowDown aria-hidden="true" focusable="false" className="size-3.5" />
        </Button>
        <Button type="button" variant="outline" size="sm" aria-label={`Sửa thuộc tính ${f.label}`} onClick={onEdit}>
          <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
        </Button>
        <Button type="button" variant="outline" size="sm" className={DANGER_OUTLINE} aria-label={`Ngừng dùng thuộc tính ${f.label}`} onClick={onRetire}>
          <Ban aria-hidden="true" focusable="false" className="size-3.5" />
        </Button>
      </div>
    </li>
  );
}

/* ---- add / edit dialog ------------------------------------------------------------------------------ */

/**
 * "Thêm thuộc tính" / "Sửa thuộc tính". On edit, Mã and Loại are drawn read-only (#1: neither changes).
 * On add, Mã is a RANDOM code drawn when the dialog opens, never one the commune holds, live or retired
 * (owner, 17/10/2026: the code is the variable ZNS / SMS templates map, so it says nothing about the label
 * and the label never touches it). "Sửa" switches to a hand-typed code checked live; "Sinh mã khác" draws
 * a new random one.
 * The Idempotency-Key of the POST is made when the dialog opens and kept across a retry inside it.
 */
export function CitizenFieldDialog({
  state,
  takenCodes,
  retiredCodes,
  onClose,
}: {
  state: DialogState;
  /** Every code the commune holds, retired ones included (the DB key covers both). */
  takenCodes: readonly string[];
  /** The retired subset of `takenCodes` — only changes the sentence. */
  retiredCodes: readonly string[];
  onClose: (saved: boolean) => void;
}) {
  const editing = state.mode === "edit";
  // Add only, drawn once at open. `null` (twenty draws all taken — a broken random source) leaves Mã empty
  // and opens it for typing instead of proposing a code known to be taken.
  const [firstCode] = useState(() => (state.mode === "edit" ? null : freeRandomFieldCode(new Set(takenCodes))));
  const [draft, setDraft] = useState<CitizenFieldDraft>(() =>
    state.mode === "edit" ? draftFromField(state.field) : { ...newFieldDraft(), code: firstCode ?? "" },
  );
  const [errors, setErrors] = useState<FieldFormErrors>({});
  const [formError, setFormError] = useState("");
  const [busy, setBusy] = useState(false);
  // Add only. "random": drawn by the screen, read-only; "manual": typed by hand, checked live.
  const [codeMode, setCodeMode] = useState<"random" | "manual">(firstCode === null ? "manual" : "random");
  // Codes the server refused with 409 while this dialog was open — another staff member took them meanwhile.
  const [serverTaken, setServerTaken] = useState<readonly string[]>([]);
  const codeInput = useRef<HTMLInputElement>(null);
  const focusCode = useRef(false);
  const id = (s: string) => `citizen-field-${s}`;

  const allTaken = [...takenCodes, ...serverTaken];
  const retiredSet = new Set(retiredCodes);
  const liveSet = new Set(allTaken.filter((c) => !retiredSet.has(c)));
  const drawCode = (extra: readonly string[] = []) => freeRandomFieldCode(new Set([...allTaken, ...extra]));
  const manualError = !editing && codeMode === "manual" ? manualCodeError(draft.code, liveSet, retiredSet) : undefined;
  const codeError = errors.code ?? manualError;

  useEffect(() => {
    if (codeMode === "manual" && focusCode.current) {
      focusCode.current = false;
      codeInput.current?.focus();
    }
  }, [codeMode]);

  function redraw() {
    const next = drawCode();
    setCodeMode(next === null ? "manual" : "random");
    setDraft({ ...draft, code: next ?? "" });
    setErrors({ ...errors, code: undefined });
  }

  async function submit() {
    if (busy) return;
    const found = validateFieldDraft(draft, !editing, allTaken, retiredCodes);
    setErrors(found);
    setFormError("");
    if (hasErrors(found)) return;
    let r: KetQua<null>;
    if (state.mode === "add") {
      setBusy(true);
      r = await createCitizenField(buildCreateBody(draft), state.idempotencyKey);
    } else {
      const body = buildUpdateBody(state.field, draft);
      if (body === null) {
        onClose(false);
        return;
      }
      setBusy(true);
      r = await updateCitizenField(state.field.code, body);
    }
    setBusy(false);
    if (!r.ok) {
      const text = r.thongBao === LOI_KHONG_RO ? SAVE_FAILED : r.thongBao;
      // A taken code is drawn under "Mã"; every other refusal (`citizen_field_limit` included) at the bottom.
      // The server's key is the final guard: another staff member may have saved the same code meanwhile.
      // A random code is redrawn at once and said; the person presses Lưu again (the Idempotency-Key was
      // released on the 4xx, so the retry under it is a fresh attempt). A typed code keeps the server's words.
      if (r.code === CITIZEN_FIELD_CODE_TAKEN && state.mode === "add") {
        const refused = draft.code.trim();
        setServerTaken([...serverTaken, refused]);
        const next = codeMode === "random" ? drawCode([refused]) : null;
        if (next !== null) {
          setDraft({ ...draft, code: next });
          setErrors({ code: CODE_REDRAWN_AFTER_CONFLICT });
        } else setErrors({ code: text });
      } else if (r.code === CITIZEN_FIELD_CODE_TAKEN) setErrors({ code: text });
      else setFormError(text);
      return;
    }
    toast.success(editing ? FIELD_SAVED_TOAST : FIELD_ADDED_TOAST);
    onClose(true);
  }

  const t = draft.fieldType;
  return (
    <ConfigDialog
      title={editing ? EDIT_FIELD_TITLE : ADD_FIELD_TITLE}
      description={editing ? state.field.label : undefined}
      size="lg"
      onDismiss={() => {
        if (!busy) onClose(false);
      }}
    >
      <form
        aria-label={editing ? EDIT_FIELD_TITLE : ADD_FIELD_TITLE}
        className="m-0 flex flex-col gap-4"
        autoComplete="off"
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <fieldset disabled={busy} className="m-0 flex min-w-0 flex-col gap-4 border-0 p-0">
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              id={id("code")}
              label="Mã"
              required={!editing}
              error={codeError}
              hint={editing ? CODE_LOCKED_HINT : CODE_HINT}
            >
              {/* Edit: read-only, no Sửa — every value typed on every person is keyed by this code (ADR 0101 #1). */}
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <input
                  ref={codeInput}
                  id={id("code")}
                  type="text"
                  spellCheck={false}
                  className={cn(inputCls, "min-w-0 flex-1 font-mono", (editing || codeMode === "random") && "bg-surface-muted")}
                  maxLength={12}
                  value={draft.code}
                  readOnly={editing || codeMode === "random"}
                  aria-readonly={editing || codeMode === "random" || undefined}
                  aria-invalid={codeError !== undefined}
                  data-code-mode={editing ? "locked" : codeMode}
                  onChange={(e) => {
                    setDraft({ ...draft, code: e.target.value.toLowerCase() });
                    setErrors({ ...errors, code: undefined });
                  }}
                />
                {!editing && codeMode === "random" && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="min-h-0 shrink-0"
                    aria-label="Sửa mã thuộc tính"
                    icon={<Pencil aria-hidden="true" focusable="false" className="size-3.5" />}
                    onClick={() => {
                      focusCode.current = true;
                      setCodeMode("manual");
                    }}
                  >
                    {CODE_EDIT_BUTTON}
                  </Button>
                )}
                {!editing && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="min-h-0 shrink-0"
                    icon={<RotateCcw aria-hidden="true" focusable="false" className="size-3.5" />}
                    onClick={redraw}
                  >
                    {CODE_REDRAW}
                  </Button>
                )}
              </div>
            </FormField>
            <FormField id={id("label")} label="Tên hiển thị" required error={errors.label}>
              <input
                id={id("label")}
                type="text"
                className={inputCls}
                value={draft.label}
                aria-invalid={errors.label !== undefined}
                onChange={(e) => setDraft({ ...draft, label: e.target.value })}
              />
            </FormField>
            <FormField id={id("type")} label="Loại" required={!editing} hint={editing ? "Loại không đổi sau khi tạo." : undefined}>
              {editing ? (
                <input id={id("type")} type="text" className={inputCls} value={fieldTypeLabel(t)} readOnly aria-readonly="true" />
              ) : (
                <select id={id("type")} className={selectCls} value={t} onChange={(e) => setDraft(setFieldType(draft, e.target.value as CitizenFieldDraft["fieldType"]))}>
                  {FIELD_TYPES.map((x) => (
                    <option key={x.value} value={x.value}>
                      {x.label}
                    </option>
                  ))}
                </select>
              )}
            </FormField>

            {hasPlaceholder(t) && (
              <FormField id={id("placeholder")} label="Gợi ý nhập (placeholder)" error={errors.placeholder}>
                <input
                  id={id("placeholder")}
                  type="text"
                  className={inputCls}
                  value={draft.placeholder}
                  aria-invalid={errors.placeholder !== undefined}
                  onChange={(e) => setDraft({ ...draft, placeholder: e.target.value })}
                />
              </FormField>
            )}
            {t === "number" && (
              <FormField id={id("number-kind")} label="Kiểu số" hint={editing ? "Kiểu số không đổi sau khi tạo." : undefined}>
                {editing ? (
                  <input
                    id={id("number-kind")}
                    type="text"
                    className={inputCls}
                    value={NUMBER_KINDS.find((k) => k.value === draft.numberKind)?.label ?? draft.numberKind}
                    readOnly
                    aria-readonly="true"
                  />
                ) : (
                  <select
                    id={id("number-kind")}
                    className={selectCls}
                    value={draft.numberKind}
                    onChange={(e) => setDraft({ ...draft, numberKind: e.target.value as CitizenFieldDraft["numberKind"] })}
                  >
                    {NUMBER_KINDS.map((k) => (
                      <option key={k.value} value={k.value}>
                        {k.label}
                      </option>
                    ))}
                  </select>
                )}
              </FormField>
            )}
            {t === "dateTime" && (
              <FormField id={id("date-pattern")} label="Định dạng ngày">
                <select
                  id={id("date-pattern")}
                  className={selectCls}
                  value={draft.datePattern}
                  onChange={(e) => setDraft({ ...draft, datePattern: e.target.value as DatePattern })}
                >
                  {DATE_PATTERNS.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
              </FormField>
            )}
          </div>

          {hasOptions(t) && <OptionsEditor draft={draft} setDraft={setDraft} errors={errors} idPrefix={id("option")} />}

          <div className="flex flex-wrap gap-x-6 gap-y-2">
            <label className="text-ink m-0 flex cursor-pointer items-center gap-2 text-[13px] font-normal">
              <input
                type="checkbox"
                className="accent-brand m-0 size-4 shrink-0"
                checked={draft.required}
                onChange={(e) => setDraft({ ...draft, required: e.target.checked })}
              />
              Bắt buộc
            </label>
            <label className="text-ink m-0 flex cursor-pointer items-center gap-2 text-[13px] font-normal">
              <input
                type="checkbox"
                className="accent-brand m-0 size-4 shrink-0"
                checked={draft.showInList}
                onChange={(e) => setDraft({ ...draft, showInList: e.target.checked })}
              />
              Hiện ở danh sách
            </label>
          </div>
        </fieldset>
        {formError !== "" && (
          <p className={ERROR} role="alert" data-form-error="">
            {formError}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="submit" variant="primary" disabled={busy || isCodeCollision(manualError)} aria-busy={busy}>
            Lưu
          </Button>
          <Button type="button" variant="outline" disabled={busy} onClick={() => onClose(false)}>
            {NUT_HUY}
          </Button>
        </div>
      </form>
    </ConfigDialog>
  );
}

/**
 * The options of a `combobox` / `singleBox`: Mã giá trị (kebab-case, locked once saved) and Nhãn. A saved
 * option is retired, never deleted (#1); one typed in this dialog and never saved is simply removed.
 */
export function OptionsEditor({
  draft,
  setDraft,
  errors,
  idPrefix,
}: {
  draft: CitizenFieldDraft;
  setDraft: (d: CitizenFieldDraft) => void;
  errors: FieldFormErrors;
  idPrefix: string;
}) {
  const hasSaved = draft.options.some((o) => o.saved);
  return (
    <fieldset className="m-0 flex min-w-0 flex-col gap-2 border-0 p-0" data-options-editor="">
      <legend className={cn(FIELD_LABEL, "p-0")}>
        Lựa chọn<span className="text-danger ml-1">*</span>
      </legend>
      {hasSaved && <p className={HINT}>{OPTIONS_HINT}</p>}
      {draft.options.map((o, i) => {
        const rowError = errors.optionRows?.[i];
        return (
          <div key={i} className="flex flex-col gap-1" data-option-row={i}>
            <div className={cn("grid grid-cols-1 items-center gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]", o.retired && "opacity-60")}>
              <label htmlFor={`${idPrefix}-value-${i}`} className="an-thi-giac">{`Mã giá trị ${i + 1}`}</label>
              <input
                id={`${idPrefix}-value-${i}`}
                type="text"
                spellCheck={false}
                className={cn(inputCls, "font-mono")}
                placeholder="Mã giá trị, ví dụ: ho-ngheo"
                value={o.value}
                readOnly={o.saved}
                aria-readonly={o.saved || undefined}
                aria-invalid={rowError !== undefined}
                onChange={(e) => setDraft(setOptionValue(draft, i, e.target.value.toLowerCase()))}
              />
              <label htmlFor={`${idPrefix}-label-${i}`} className="an-thi-giac">{`Nhãn ${i + 1}`}</label>
              <input
                id={`${idPrefix}-label-${i}`}
                type="text"
                className={inputCls}
                placeholder="Nhãn, ví dụ: Hộ nghèo"
                value={o.label}
                aria-invalid={rowError !== undefined}
                onChange={(e) => setDraft(setOptionLabel(draft, i, e.target.value))}
              />
              {o.saved && o.retired ? (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="min-h-0"
                  aria-label={`Dùng lại lựa chọn ${o.label || i + 1}`}
                  icon={<RotateCcw aria-hidden="true" focusable="false" className="size-3.5" />}
                  onClick={() => setDraft(restoreOption(draft, i))}
                >
                  Dùng lại
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="min-h-0"
                  aria-label={o.saved ? `Ngừng dùng lựa chọn ${o.label || i + 1}` : `Bỏ lựa chọn ${i + 1}`}
                  icon={o.saved ? <Ban aria-hidden="true" focusable="false" className="size-3.5" /> : <X aria-hidden="true" focusable="false" className="size-3.5" />}
                  onClick={() => setDraft(retireOrRemoveOption(draft, i))}
                >
                  {o.saved ? "Ngừng dùng" : "Bỏ"}
                </Button>
              )}
            </div>
            {o.retired && <p className={HINT}>Đã ngừng dùng — không còn hiện để chọn mới.</p>}
            {rowError !== undefined && (
              <p className={ERROR} role="alert">
                {rowError}
              </p>
            )}
          </div>
        );
      })}
      {errors.options !== undefined && (
        <p className={ERROR} role="alert" data-options-error="">
          {errors.options}
        </p>
      )}
      <div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="min-h-0"
          icon={<Plus aria-hidden="true" focusable="false" className="size-3.5" />}
          onClick={() => setDraft(addOption(draft))}
        >
          Thêm lựa chọn
        </Button>
      </div>
    </fieldset>
  );
}

function FormField({
  id,
  label,
  required = false,
  hint,
  error,
  children,
}: {
  id: string;
  label: string;
  required?: boolean;
  hint?: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <label htmlFor={id} className={FIELD_LABEL}>
        {label}
        {required && <span className="text-danger ml-1">*</span>}
      </label>
      {children}
      {hint !== undefined && error === undefined && <p className={HINT}>{hint}</p>}
      {error !== undefined && (
        <p className={ERROR} role="alert" data-field-error="">
          {error}
        </p>
      )}
    </div>
  );
}

/* ---- retire ----------------------------------------------------------------------------------------- */

/** "Ngừng dùng": a required reason, then DELETE — the field hides, every value stays (ADR 0101 #4, rule 7). */
export function RetireFieldDialog({ field, onClose }: { field: CitizenField; onClose: (done: boolean) => void }) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const inputId = `citizen-field-${field.code}-retire-reason`;

  async function retire() {
    const checked = checkRetireReason(reason);
    if (!checked.ok) {
      setError(checked.text);
      return;
    }
    setError(null);
    setBusy(true);
    const r = await retireCitizenField(field.code, checked.reason);
    setBusy(false);
    if (!r.ok) {
      setError(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED : r.thongBao);
      return;
    }
    toast.success(FIELD_RETIRED_TOAST);
    onClose(true);
  }

  return (
    <ConfigDialog
      title="Ngừng dùng thuộc tính?"
      hideHeader
      onDismiss={() => {
        if (!busy) onClose(false);
      }}
    >
      <ConfirmDialog
        as="form"
        aria-label="Ngừng dùng thuộc tính"
        icon={Ban}
        tone="danger"
        title={`Ngừng dùng thuộc tính “${field.label}”?`}
        className="m-0 p-0 shadow-none"
        onSubmit={(e) => {
          e.preventDefault();
          if (!busy) void retire();
        }}
        actions={
          <>
            <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
              Ngừng dùng
            </Button>
            <Button type="button" variant="outline" disabled={busy} onClick={() => onClose(false)}>
              {NUT_HUY}
            </Button>
          </>
        }
      >
        <p className="m-0" data-retire-consequence="">
          {RETIRE_CONSEQUENCE}
        </p>
        <div className="mt-3 flex flex-col gap-1.5">
          <label htmlFor={inputId} className={FIELD_LABEL}>
            {RETIRE_REASON_LABEL}
          </label>
          <input
            id={inputId}
            type="text"
            className={inputCls}
            maxLength={RETIRE_REASON_MAX}
            value={reason}
            disabled={busy}
            aria-invalid={error !== null}
            onChange={(e) => setReason(e.target.value)}
          />
          {error !== null && (
            <p className={ERROR} role="alert">
              {error}
            </p>
          )}
        </div>
      </ConfirmDialog>
    </ConfigDialog>
  );
}
