"use client";

import { closestCenter, DndContext, useDraggable, useDroppable } from "@dnd-kit/core";
import {
  AlignLeft,
  ArrowDown,
  ArrowUp,
  Calendar,
  ChevronDown,
  CircleDot,
  GripVertical,
  Hash,
  ListChecks,
  Type,
  type LucideIcon,
} from "lucide-react";
import { useCallback } from "react";

import { Button } from "@/components/ui/button";
import { Segmented } from "@/components/ui/segmented";
import {
  displayModeOf,
  layoutWidthOf,
  type CitizenField,
  type CitizenFieldLayoutWidth,
  type CitizenFieldType,
} from "@/lib/api/citizen-fields";
import { cn } from "@/lib/cn";

import { DRAG_INSTRUCTIONS, orderAnnouncements, useOrderSensors } from "./citizen-field-dnd";
import {
  EMPTY_SLOT_HINT,
  LAYOUT_NARROW_NOTE,
  LAYOUT_STATIC_NOTE,
  LAYOUT_TITLE,
  LAYOUT_WIDTHS,
  fieldTypeLabel,
  loneHalfCodes,
  positionText,
} from "./citizen-field-form";

const HINT = "text-ink-muted m-0 text-[12px]";
/** The mock control: the contact form's 32px box, drawn inert. */
const MOCK_BOX = "border-line text-ink-muted flex h-8 min-w-0 items-center justify-between gap-2 rounded-md border border-solid bg-white px-2.5 text-[12.5px]";
/** How many options a "full" choice field shows before "+N" — the preview is a sketch, not the form. */
const MOCK_OPTIONS_MAX = 4;

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

/**
 * "Bố cục biểu mẫu" (ADR 0101 #3c): the dynamic part of the Thêm / Sửa người dân dialog as that dialog draws
 * it — a 2-column grid, `half` = one column, `full` = `sm:col-span-2`, auto-flowing in sort order. The same
 * classes as `CitizenAttributeInputs`, so what is arranged here is what the officer sees; below 640px both
 * fall to one column.
 *
 * A half field left alone on its row gets a dashed "Ô bên phải còn trống" cell in the slot it leaves — the
 * owner asked for left / right pairs, and an empty slot is easy to miss in a list. It is hidden in one column,
 * where no slot exists.
 *
 * Reorder: drag (mouse, touch, keyboard — the list's sensors) or Lên / Xuống. Width: "Nửa hàng / Cả hàng",
 * handed to the tab, which saves it.
 */
export function FieldLayoutPreview({
  fields,
  busy,
  widthSaving,
  onMove,
  onDrop,
  onWidth,
}: {
  fields: readonly CitizenField[];
  /** A reorder is being saved: every move control waits. */
  busy: boolean;
  /** Codes whose width is being saved. */
  widthSaving: readonly string[];
  onMove: (from: number, to: number) => void;
  onDrop: (active: string, over: string | null) => void;
  onWidth: (f: CitizenField, width: CitizenFieldLayoutWidth) => void;
}) {
  const sensors = useOrderSensors();
  const labelOf = (code: string) => fields.find((f) => f.code === code)?.label ?? code;
  const lone = loneHalfCodes(fields);

  return (
    <section className="mt-4 flex min-w-0 flex-col gap-2" aria-labelledby="citizen-field-layout-title" data-field-layout="">
      <h4 id="citizen-field-layout-title" className="text-navy m-0 text-[13px] font-bold">
        {LAYOUT_TITLE}
      </h4>
      <p className={HINT} data-layout-static-note="">
        {LAYOUT_STATIC_NOTE}
      </p>
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
        <ol
          className="bg-surface-muted border-line m-0 grid list-none grid-cols-1 gap-3 rounded-[10px] border border-solid p-3 sm:grid-cols-2"
          aria-label="Bố cục thuộc tính trên biểu mẫu"
          data-layout-grid=""
        >
          {fields.map((f, i) => (
            <LayoutCardWithSlot
              key={f.code}
              field={f}
              index={i}
              total={fields.length}
              busy={busy}
              savingWidth={widthSaving.includes(f.code)}
              lone={lone.has(f.code)}
              onUp={() => onMove(i, i - 1)}
              onDown={() => onMove(i, i + 1)}
              onWidth={(w) => onWidth(f, w)}
            />
          ))}
        </ol>
      </DndContext>
    </section>
  );
}

function LayoutCardWithSlot(props: CardProps & { lone: boolean }) {
  const { lone, ...card } = props;
  return (
    <>
      <LayoutCard {...card} />
      {lone && (
        <li
          className="border-line text-ink-muted hidden min-h-16 items-center justify-center rounded-[10px] border border-dashed px-3 text-center text-[12px] sm:flex"
          data-empty-slot={props.field.code}
        >
          {EMPTY_SLOT_HINT}
        </li>
      )}
    </>
  );
}

type CardProps = {
  field: CitizenField;
  index: number;
  total: number;
  busy: boolean;
  savingWidth: boolean;
  onUp: () => void;
  onDown: () => void;
  onWidth: (w: CitizenFieldLayoutWidth) => void;
};

function LayoutCard({ field: f, index, total, busy, savingWidth, onUp, onDown, onWidth }: CardProps) {
  const width = layoutWidthOf(f);
  const { setNodeRef: setDropRef, isOver } = useDroppable({ id: f.code });
  const drag = useDraggable({ id: f.code, disabled: busy });
  const { setNodeRef: setDragNode, setActivatorNodeRef } = drag;
  // Stable callbacks, so a re-render during the drag does not detach the nodes dnd-kit measures.
  const setCardRef = useCallback((node: HTMLElement | null) => setDropRef(node), [setDropRef]);
  const setBodyRef = useCallback((node: HTMLElement | null) => setDragNode(node), [setDragNode]);
  const setHandleRef = useCallback((node: HTMLElement | null) => setActivatorNodeRef(node), [setActivatorNodeRef]);
  const t = drag.transform;
  const Icon = TYPE_ICONS[f.field_type] ?? Type;

  return (
    <li
      ref={setCardRef}
      data-layout-card={f.code}
      data-width={width}
      className={cn(
        "border-line flex min-w-0 flex-col gap-2 rounded-[10px] border border-solid bg-white p-3",
        width === "full" && "sm:col-span-2",
        isOver && !drag.isDragging && "border-brand bg-brand/5",
      )}
    >
      <div
        ref={setBodyRef}
        style={t === null ? undefined : { transform: `translate3d(${t.x}px, ${t.y}px, 0)` }}
        className={cn("flex min-w-0 flex-col gap-2", drag.isDragging && "relative z-10 opacity-80")}
      >
        <div className="flex min-w-0 items-center gap-2">
          <button
            ref={setHandleRef}
            type="button"
            className="text-ink-muted flex size-8 shrink-0 cursor-grab touch-none items-center justify-center rounded-md border-0 bg-transparent p-0 disabled:cursor-not-allowed"
            aria-label={positionText(f.label, index, total)}
            disabled={busy}
            {...drag.listeners}
            {...drag.attributes}
          >
            <GripVertical aria-hidden="true" focusable="false" className="size-4" />
          </button>
          <span className="text-navy min-w-0 flex-1 text-[13px] font-semibold break-words">
            {f.label}
            {f.required && <span className="text-danger ml-1">*</span>}
          </span>
          <span className="text-ink-muted inline-flex shrink-0 items-center gap-1 text-[12px]" data-card-type="">
            <Icon aria-hidden="true" focusable="false" className="size-3.5" />
            {fieldTypeLabel(f.field_type)}
          </span>
        </div>
        <MockControl field={f} />
      </div>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Segmented
          mode="buttons"
          legend={`Độ rộng của ${f.label}`}
          name={`citizen-field-width-${f.code}`}
          value={width}
          options={WIDTH_OPTIONS}
          disabled={savingWidth}
          onChange={(v) => onWidth(v === "full" ? "full" : "half")}
        />
        <div className="flex items-center gap-1.5 [&>button]:min-h-0">
          <Button type="button" variant="outline" size="sm" aria-label={`Đưa ${f.label} lên`} disabled={busy || index === 0} onClick={onUp}>
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
        </div>
      </div>
    </li>
  );
}

/**
 * A sketch of the control the contact form draws for this field — the same choice as
 * `CitizenAttributeInputs` (type × display_mode), but not an input: nothing here can be typed into, so it
 * is hidden from assistive technology, and the card's label and type say what it is.
 */
function MockControl({ field: f }: { field: CitizenField }) {
  const live = f.options.filter((o) => !o.retired);
  let body;
  if (f.field_type === "combobox" || f.field_type === "singleBox") {
    if (displayModeOf(f) === "dropdown") {
      body = (
        <div className={MOCK_BOX}>
          <span className="truncate">Chưa chọn</span>
          <ChevronDown className="size-4 shrink-0" />
        </div>
      );
    } else {
      const shown = live.slice(0, MOCK_OPTIONS_MAX);
      body = (
        <div className="text-ink flex flex-wrap gap-x-4 gap-y-1.5 text-[12.5px]">
          {shown.map((o) => (
            <span key={o.value} className="inline-flex items-center gap-1.5">
              <span className={cn("border-line inline-block size-3.5 shrink-0 border border-solid bg-white", f.field_type === "singleBox" ? "rounded-full" : "rounded-[3px]")} />
              {o.label}
            </span>
          ))}
          {live.length > shown.length && <span className="text-ink-muted">+{live.length - shown.length} lựa chọn</span>}
        </div>
      );
    }
  } else if (f.field_type === "multiText") {
    body = <div className={cn(MOCK_BOX, "h-16 items-start py-1.5")}>{f.placeholder}</div>;
  } else if (f.field_type === "dateTime") {
    body = (
      <div className={MOCK_BOX}>
        <span className="truncate">{f.date_pattern}</span>
        <Calendar className="size-4 shrink-0" />
      </div>
    );
  } else if (f.field_type === "number") {
    body = <div className={cn(MOCK_BOX, "tabular-nums")}>{f.number_kind === "decimal" ? "0,0" : "0"}</div>;
  } else {
    body = (
      <div className={MOCK_BOX}>
        <span className="truncate">{f.placeholder}</span>
      </div>
    );
  }
  return (
    <div aria-hidden="true" className="pointer-events-none" data-mock-control={f.field_type}>
      {body}
    </div>
  );
}
