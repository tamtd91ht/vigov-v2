"use client";

import { closestCenter, DndContext, useDraggable, useDroppable } from "@dnd-kit/core";
import {
  AlignLeft,
  ArrowDown,
  ArrowUp,
  Calendar,
  CircleDot,
  GripVertical,
  Hash,
  ListChecks,
  MousePointerClick,
  Pencil,
  Plus,
  Trash2,
  Type,
  type LucideIcon,
} from "lucide-react";
import { useCallback, useEffect, useRef, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Segmented } from "@/components/ui/segmented";
import { CitizenAttributeInputs } from "@/features/citizen-directory/citizen-attribute-inputs";
import {
  CANCEL_BUTTON,
  CONTACT_ADD_TITLE,
  CONTACT_DIALOG_DESCRIPTION,
  FIELD_ACTIVE,
  FIELD_ADDRESS,
  FIELD_EMAIL,
  FIELD_FULL_NAME,
  FIELD_GROUPS,
  FIELD_NOTE,
  FIELD_PHONE,
  FIELD_TAGS,
  FIELD_UNIT,
  TAGS_HINT,
  TAGS_WARNING,
  UNIT_UNKNOWN,
} from "@/features/citizen-directory/citizen-directory-words";
import { layoutWidthOf, type CitizenField, type CitizenFieldLayoutWidth, type CitizenFieldType } from "@/lib/api/citizen-fields";
import { cn } from "@/lib/cn";

import { DRAG_INSTRUCTIONS, orderAnnouncements, useOrderSensors } from "./citizen-field-dnd";
import {
  ADD_FIELD_BUTTON,
  ADD_FIELD_TITLE,
  DELETE_FIELD_BUTTON,
  EDIT_FIELD_TITLE,
  EMPTY_SLOT_HINT,
  LAYOUT_DYNAMIC_TITLE,
  LAYOUT_HINT,
  LAYOUT_NARROW_NOTE,
  LAYOUT_STATIC_NOTE,
  LAYOUT_TITLE,
  LAYOUT_WIDTHS,
  NEW_FIELD_LABEL,
  PANEL_EMPTY,
  fieldTypeLabel,
  insertBeforeLabel,
  loneHalfCodes,
  positionText,
  previewField,
  type CitizenFieldDraft,
} from "./citizen-field-form";

const HINT = "text-ink-muted m-0 text-[12px]";
const SECTION_LABEL = "text-ink-muted m-0 text-[11.5px] font-semibold tracking-wide uppercase";

const TYPE_ICONS: Record<CitizenFieldType, LucideIcon> = {
  singleText: Type,
  multiText: AlignLeft,
  combobox: ListChecks,
  singleBox: CircleDot,
  number: Hash,
  dateTime: Calendar,
};

/** The width segments' options, typed for `Segmented` (which emits the value it was given, nothing else). */
const WIDTH_OPTIONS = LAYOUT_WIDTHS.map((w) => ({ value: w.value, label: w.label }));

/** What the side panel is open on: a new field to go at index `at`, or a stored field. */
export type DesignerSelection = { kind: "add"; at: number } | { kind: "edit"; code: string } | null;

/** One cell of the designer's grid, in the order the contact form draws them. */
type Cell =
  | { kind: "field"; field: CitizenField; index: number; draft: boolean }
  | { kind: "new"; field: CitizenField };

/**
 * "Biểu mẫu" — the Thêm người dân form ITSELF, as the officer sees it, made editable where the commune may
 * change it (user, 17/10/2026; ADR 0101 #3, #3c):
 *
 *   - the header and the static fields are the real dialog's words and grid, drawn `inert`: they are the
 *     prototype's (#3c "ô tĩnh giữ bố cục prototype, đứng trên"), nothing here moves them, and a field can
 *     never be put among them;
 *   - the live fields follow in the dialog's own 2-column grid, each drawn by `CitizenAttributeInputs` — the
 *     very control the officer gets, not a sketch — `inert` too, so it can be looked at but not typed into;
 *   - a "+" before every field, on the empty right slot a lone half leaves, and at the end: the new field's
 *     draft card appears THERE at once, and the side panel's form redraws it as it is typed;
 *   - each field: Sửa (opens the panel), Xoá (the reasoned soft delete), Lên / Xuống and a drag handle, and
 *     "Nửa hàng / Cả hàng".
 *
 * Side panel: next to the form from `lg` (sticky, so it stays in view along a long form); under it below
 * that, scrolled into view when it opens.
 */
export function FormDesigner({
  fields,
  busy,
  full,
  widthSaving,
  selection,
  draft,
  panel,
  onInsert,
  onEdit,
  onDelete,
  onMove,
  onDrop,
  onWidth,
}: {
  fields: readonly CitizenField[];
  /** A reorder is being saved: every move control waits. */
  busy: boolean;
  /** The commune's limit is reached: no "+" works. */
  full: boolean;
  /** Codes whose width is being saved. */
  widthSaving: readonly string[];
  selection: DesignerSelection;
  /** The open form's draft, `null` until it reports one. */
  draft: CitizenFieldDraft | null;
  /** The add / edit form, drawn in the side panel. */
  panel: ReactNode;
  onInsert: (at: number) => void;
  onEdit: (f: CitizenField) => void;
  onDelete: (f: CitizenField) => void;
  onMove: (from: number, to: number) => void;
  onDrop: (active: string, over: string | null) => void;
  onWidth: (f: CitizenField, width: CitizenFieldLayoutWidth) => void;
}) {
  const sensors = useOrderSensors();
  const labelOf = (code: string) => fields.find((f) => f.code === code)?.label ?? code;
  const panelRef = useRef<HTMLElement>(null);
  const selectionKey = selection === null ? "" : selection.kind === "add" ? `add:${selection.at}` : `edit:${selection.code}`;

  useEffect(() => {
    // One column (below lg) puts the panel under a possibly long form: bring it into view when it opens.
    if (selectionKey === "" || typeof window.matchMedia !== "function") return;
    if (!window.matchMedia("(max-width: 1023px)").matches) return;
    panelRef.current?.scrollIntoView?.({ behavior: "smooth", block: "start" });
  }, [selectionKey]);

  const cells: Cell[] = fields.map((f, index) => {
    const editingThis = selection?.kind === "edit" && selection.code === f.code && draft !== null;
    return { kind: "field", field: editingThis ? previewField(draft, f, f.label) : f, index, draft: editingThis };
  });
  if (selection?.kind === "add") {
    const shown = previewField(draft ?? EMPTY_DRAFT, null, NEW_FIELD_LABEL);
    cells.splice(Math.min(selection.at, cells.length), 0, { kind: "new", field: { ...shown, code: shown.code || "moi" } });
  }
  const lone = loneHalfCodes(cells.map((c) => c.field));

  return (
    <section className="mt-4 flex min-w-0 flex-col gap-3" aria-labelledby="citizen-field-layout-title" data-field-layout="">
      <div>
        <h4 id="citizen-field-layout-title" className="text-navy m-0 text-[13px] font-bold">
          {LAYOUT_TITLE}
        </h4>
        <p className={cn(HINT, "mt-1 max-w-3xl")}>{LAYOUT_HINT}</p>
      </div>

      <div className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(19rem,24rem)] lg:items-start">
        {/* The form, framed as the dialog is: same width cap (42rem), same header, same grids. */}
        <div className="border-line shadow-card mx-auto flex w-full max-w-[42rem] min-w-0 flex-col gap-4 rounded-[14px] border border-solid bg-white p-5" data-form-canvas="">
          <div>
            <p className="text-navy m-0 text-[16px] font-bold">{CONTACT_ADD_TITLE}</p>
            <p className="text-ink-muted m-0 mt-1 text-[12.5px]">{CONTACT_DIALOG_DESCRIPTION}</p>
          </div>

          <div className="flex flex-col gap-2">
            <p className={SECTION_LABEL} data-layout-static-note="">
              {LAYOUT_STATIC_NOTE}
            </p>
            <StaticFields />
          </div>

          <div className="flex flex-col gap-2">
            <p className={SECTION_LABEL}>{LAYOUT_DYNAMIC_TITLE}</p>
            <p className={cn(HINT, "sm:hidden")} data-layout-narrow-note="">
              {LAYOUT_NARROW_NOTE}
            </p>
            <DndContext
              id="citizen-field-layout"
              sensors={sensors}
              collisionDetection={closestCenter}
              accessibility={{ announcements: orderAnnouncements(labelOf), screenReaderInstructions: DRAG_INSTRUCTIONS }}
              onDragEnd={(e) => {
                if (busy) return;
                onDrop(String(e.active.id), e.over === null ? null : String(e.over.id));
              }}
            >
              <ol className="m-0 grid list-none grid-cols-1 gap-3 p-0 sm:grid-cols-2" aria-label="Thuộc tính trên biểu mẫu" data-layout-grid="">
                {cells.map((c) => {
                  const slot = lone.has(c.field.code) && (
                    <li key={`slot-${c.field.code}`} className="hidden sm:flex" data-empty-slot={c.field.code}>
                      {/* A lone half's right slot: the next field index is where a new one fills it. */}
                      <button
                        type="button"
                        className={SLOT_BUTTON}
                        disabled={full || c.kind === "new"}
                        onClick={() => onInsert(c.kind === "field" ? c.index + 1 : 0)}
                      >
                        <Plus aria-hidden="true" focusable="false" className="size-4" />
                        {EMPTY_SLOT_HINT}
                      </button>
                    </li>
                  );
                  if (c.kind === "new") {
                    return [<NewFieldCard key="new" field={c.field} />, slot];
                  }
                  return [
                    <FieldCard
                      key={c.field.code}
                      field={c.field}
                      stored={fields[c.index]!}
                      index={c.index}
                      total={fields.length}
                      busy={busy}
                      full={full}
                      selected={c.draft || (selection?.kind === "edit" && selection.code === c.field.code)}
                      savingWidth={widthSaving.includes(c.field.code)}
                      onInsert={() => onInsert(c.index)}
                      onEdit={() => onEdit(fields[c.index]!)}
                      onDelete={() => onDelete(fields[c.index]!)}
                      onUp={() => onMove(c.index, c.index - 1)}
                      onDown={() => onMove(c.index, c.index + 1)}
                      onWidth={(w) => onWidth(fields[c.index]!, w)}
                    />,
                    slot,
                  ];
                })}
                <li className="sm:col-span-2">
                  <button
                    type="button"
                    className={cn(SLOT_BUTTON, "min-h-14")}
                    disabled={full || selection?.kind === "add"}
                    data-add-at-end=""
                    onClick={() => onInsert(fields.length)}
                  >
                    <Plus aria-hidden="true" focusable="false" className="size-4" />
                    {ADD_FIELD_BUTTON}
                  </button>
                </li>
              </ol>
            </DndContext>
          </div>

          {/* The dialog's footer, as a picture of where the form ends. */}
          <div className="border-line flex justify-end gap-2 border-0 border-t border-solid pt-3" inert aria-hidden="true">
            <span className="border-line text-ink-muted rounded-md border border-solid px-3 py-1.5 text-[12.5px]">{CANCEL_BUTTON}</span>
            <span className="bg-brand rounded-md px-3 py-1.5 text-[12.5px] font-semibold text-white">Lưu</span>
          </div>
        </div>

        <aside
          ref={panelRef}
          className="border-line shadow-card min-w-0 scroll-mt-4 rounded-[14px] border border-solid bg-white p-4 lg:sticky lg:top-4"
          aria-label={selection === null ? "Thuộc tính đang chọn" : selection.kind === "add" ? ADD_FIELD_TITLE : EDIT_FIELD_TITLE}
          data-field-panel=""
        >
          {panel !== null ? (
            <div className="flex flex-col gap-3">
              <p className="text-navy m-0 text-[14px] font-bold">{selection?.kind === "edit" ? EDIT_FIELD_TITLE : ADD_FIELD_TITLE}</p>
              {panel}
            </div>
          ) : (
            <div className="text-ink-muted flex flex-col items-center gap-2 px-2 py-8 text-center text-[12.5px]" data-panel-empty="">
              <MousePointerClick aria-hidden="true" focusable="false" className="size-6" />
              <p className="m-0 max-w-64">{PANEL_EMPTY}</p>
            </div>
          )}
        </aside>
      </div>
    </section>
  );
}

const SLOT_BUTTON =
  "border-line text-ink-muted hover:not-disabled:border-brand hover:not-disabled:text-brand hover:not-disabled:bg-brand/5 flex min-h-16 w-full cursor-pointer items-center justify-center gap-1.5 rounded-[10px] border border-dashed bg-transparent px-3 text-center [font-family:inherit] text-[12.5px] font-medium disabled:cursor-not-allowed disabled:opacity-50";

/** A draft with nothing typed yet: the panel reports the real one on its first render. */
const EMPTY_DRAFT: CitizenFieldDraft = {
  code: "",
  label: "",
  fieldType: "singleText",
  placeholder: "",
  options: [],
  numberKind: "integer",
  datePattern: "dd/MM/yyyy",
  required: false,
  showInList: false,
  displayMode: "full",
  layoutWidth: null,
};

/**
 * The dialog's static section, in its order and spans (`citizen-contact-dialog.tsx`), with its own words.
 * `inert`: it is the prototype's part of the form, shown so the commune sees where its fields land.
 * KEEP IN STEP with that dialog: a field added or moved there is added or moved here.
 */
function StaticFields() {
  const id = (s: string) => `citizen-field-static-${s}`;
  return (
    <div className="bg-surface-muted/60 grid min-w-0 gap-4 rounded-[10px] p-3 sm:grid-cols-2" inert data-static-fields="">
      <Field label={FIELD_FULL_NAME} htmlFor={id("name")} required grow="auto">
        <input id={id("name")} readOnly tabIndex={-1} />
      </Field>
      <Field label={FIELD_PHONE} htmlFor={id("phone")} required grow="auto">
        <input id={id("phone")} readOnly tabIndex={-1} />
      </Field>
      <Field label={FIELD_EMAIL} htmlFor={id("email")} grow="auto" className="sm:col-span-2">
        <input id={id("email")} readOnly tabIndex={-1} />
      </Field>
      <Field label={FIELD_UNIT} htmlFor={id("unit")} kind="select" grow="auto">
        <select id={id("unit")} tabIndex={-1} defaultValue="">
          <option value="">{UNIT_UNKNOWN}</option>
        </select>
      </Field>
      <div className="flex flex-col justify-end">
        <span className="text-navy flex h-8 items-center gap-2 text-sm">
          <input type="checkbox" className="accent-brand size-4" defaultChecked tabIndex={-1} aria-label={FIELD_ACTIVE} />
          {FIELD_ACTIVE}
        </span>
      </div>
      <Field label={FIELD_ADDRESS} htmlFor={id("address")} grow="auto" className="sm:col-span-2">
        <input id={id("address")} readOnly tabIndex={-1} />
      </Field>
      <Field label={FIELD_TAGS} htmlFor={id("tags")} grow="auto" className="sm:col-span-2" hint={TAGS_HINT}>
        <input id={id("tags")} readOnly tabIndex={-1} />
      </Field>
      <p className="text-danger m-0 -mt-2 text-[12px] font-medium sm:col-span-2">{TAGS_WARNING}</p>
      <div className="flex flex-col gap-1.5 sm:col-span-2">
        <p className="text-navy m-0 block text-[13px] leading-tight font-semibold">{FIELD_GROUPS}</p>
        {/* The commune's groups are chips here; no group is named, the designer reads none. */}
        <div className="flex flex-wrap gap-1.5">
          <span className="border-line text-ink-muted rounded-full border border-dashed bg-white px-3 py-1.5 text-[12px] font-semibold">Các nhóm của xã</span>
        </div>
      </div>
      <Field label={FIELD_NOTE} htmlFor={id("note")} grow="auto" className="sm:col-span-2">
        <textarea id={id("note")} rows={2} readOnly tabIndex={-1} />
      </Field>
    </div>
  );
}

/** The real contact-form control of one field, drawn `inert` — filling its card, whatever its width. */
function LiveControl({ field }: { field: CitizenField }) {
  return (
    <div inert className="min-w-0" data-live-control={field.field_type}>
      <CitizenAttributeInputs fields={[{ ...field, layout_width: "full" }]} inputs={{}} stored={undefined} errors={{}} onChange={noop} />
    </div>
  );
}

function noop() {}

function TypeTag({ type }: { type: CitizenFieldType }) {
  const Icon = TYPE_ICONS[type] ?? Type;
  return (
    <span className="text-ink-muted inline-flex shrink-0 items-center gap-1 text-[11.5px]" data-card-type="">
      <Icon aria-hidden="true" focusable="false" className="size-3.5" />
      {fieldTypeLabel(type)}
    </span>
  );
}

/** The draft of the field being added, where it will land; the panel's form redraws it as it is typed. */
function NewFieldCard({ field }: { field: CitizenField }) {
  return (
    <li
      className={cn(
        "border-brand bg-brand/5 ring-brand/20 flex min-w-0 flex-col gap-2 rounded-[10px] border border-dashed p-3 ring-2",
        layoutWidthOf(field) === "full" && "sm:col-span-2",
      )}
      data-new-field-card=""
      data-width={layoutWidthOf(field)}
    >
      <div className="flex min-w-0 items-center gap-2">
        <span className="bg-brand rounded-full px-2 py-0.5 text-[11px] font-semibold text-white">{NEW_FIELD_LABEL}</span>
        <span className="flex-1" />
        <TypeTag type={field.field_type} />
      </div>
      <LiveControl field={field} />
    </li>
  );
}

type CardProps = {
  /** What is drawn — the stored field, or the edit draft over it. */
  field: CitizenField;
  /** The stored field: its label names the buttons, so they do not change while the label is typed. */
  stored: CitizenField;
  index: number;
  total: number;
  busy: boolean;
  full: boolean;
  selected: boolean;
  savingWidth: boolean;
  onInsert: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onUp: () => void;
  onDown: () => void;
  onWidth: (w: CitizenFieldLayoutWidth) => void;
};

function FieldCard({ field: f, stored, index, total, busy, full, selected, savingWidth, onInsert, onEdit, onDelete, onUp, onDown, onWidth }: CardProps) {
  const width = layoutWidthOf(f);
  const name = stored.label;
  const { setNodeRef: setDropRef, isOver } = useDroppable({ id: stored.code });
  const drag = useDraggable({ id: stored.code, disabled: busy });
  const { setNodeRef: setDragNode, setActivatorNodeRef } = drag;
  // Stable callbacks, so a re-render during the drag does not detach the nodes dnd-kit measures.
  const setCardRef = useCallback((node: HTMLElement | null) => setDropRef(node), [setDropRef]);
  const setBodyRef = useCallback((node: HTMLElement | null) => setDragNode(node), [setDragNode]);
  const setHandleRef = useCallback((node: HTMLElement | null) => setActivatorNodeRef(node), [setActivatorNodeRef]);
  const t = drag.transform;

  return (
    <li
      ref={setCardRef}
      data-layout-card={stored.code}
      data-width={width}
      data-selected={selected || undefined}
      className={cn(
        "group border-line flex min-w-0 flex-col gap-2 rounded-[10px] border border-solid bg-white p-3",
        width === "full" && "sm:col-span-2",
        selected && "border-brand ring-brand/20 ring-2",
        isOver && !drag.isDragging && "border-brand bg-brand/5",
      )}
    >
      <div
        ref={setBodyRef}
        style={t === null ? undefined : { transform: `translate3d(${t.x}px, ${t.y}px, 0)` }}
        className={cn("flex min-w-0 flex-col gap-2", drag.isDragging && "relative z-10 opacity-80")}
      >
        <div className="flex min-w-0 items-center gap-1">
          <button
            ref={setHandleRef}
            type="button"
            className="text-ink-muted flex size-7 shrink-0 cursor-grab touch-none items-center justify-center rounded-md border-0 bg-transparent p-0 disabled:cursor-not-allowed"
            aria-label={positionText(name, index, total)}
            disabled={busy}
            {...drag.listeners}
            {...drag.attributes}
          >
            <GripVertical aria-hidden="true" focusable="false" className="size-4" />
          </button>
          <TypeTag type={f.field_type} />
          <span className="flex-1" />
          <div className="flex items-center gap-1 [&>button]:min-h-0">
            <Button type="button" variant="icon" size="sm" aria-label={insertBeforeLabel(name)} title="Chèn thuộc tính trước ô này" disabled={full} onClick={onInsert}>
              <Plus aria-hidden="true" focusable="false" className="size-3.5" />
            </Button>
            <Button type="button" variant="icon" size="sm" aria-label={`Sửa thuộc tính ${name}`} title="Sửa" aria-pressed={selected} onClick={onEdit}>
              <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
            </Button>
            <Button
              type="button"
              variant="icon"
              size="sm"
              className="text-danger"
              aria-label={`${DELETE_FIELD_BUTTON} ${name}`}
              title={DELETE_FIELD_BUTTON}
              onClick={onDelete}
            >
              <Trash2 aria-hidden="true" focusable="false" className="size-3.5" />
            </Button>
          </div>
        </div>
        {/* A click on the drawing opens it in the panel, the way a form builder does. */}
        <div className="cursor-pointer" onClick={onEdit} data-card-body="">
          <LiveControl field={f} />
        </div>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Segmented
          mode="buttons"
          legend={`Độ rộng của ${name}`}
          name={`citizen-field-width-${stored.code}`}
          value={width}
          options={WIDTH_OPTIONS}
          disabled={savingWidth || selected}
          onChange={(v) => onWidth(v === "full" ? "full" : "half")}
        />
        <div className="flex items-center gap-1.5 [&>button]:min-h-0">
          <Button type="button" variant="outline" size="sm" aria-label={`Đưa ${name} lên`} disabled={busy || index === 0} onClick={onUp}>
            <ArrowUp aria-hidden="true" focusable="false" className="size-3.5" />
          </Button>
          <Button type="button" variant="outline" size="sm" aria-label={`Đưa ${name} xuống`} disabled={busy || index === total - 1} onClick={onDown}>
            <ArrowDown aria-hidden="true" focusable="false" className="size-3.5" />
          </Button>
        </div>
      </div>
    </li>
  );
}
