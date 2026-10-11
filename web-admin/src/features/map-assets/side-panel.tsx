"use client";

import { Eye, EyeOff, Flame, HelpCircle, Search, X } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import type { PendingFeatureInfo } from "@/components/ui/pending-feature";
import { Switch } from "@/components/ui/switch";
import { Tooltip } from "@/components/ui/tooltip";
import { cn } from "@/lib/cn";
import type { comms_loaiTaiNguyenRa, comms_mapAssetRowOut, identity_thonToDanPhoRa } from "@/lib/api/schema.gen";

import {
  CLEAR_FILTERS,
  DENSITY_TITLE,
  FILTERS_TITLE,
  HEAT_SWITCH_LABEL,
  HIDE_ALL,
  INDUSTRIES,
  LAYERS_TITLE,
  ONLY_LAYER,
  ONLY_LAYER_TITLE,
  SEARCH_PLACEHOLDER,
  SHOW_ALL,
  STATUS_OPTIONS,
  UNVERIFIED_LEGEND,
  CENTRE_LEGEND,
  CENTRE_LEGEND_DEFAULT,
  pendingPart,
} from "./labels";
import { groupSwatchClass } from "./map-logic";

export type HeatSwitchState = {
  readonly checked: boolean;
  /** `null` = usable; otherwise the sentence shown as `title` on the disabled switch. */
  readonly disabledReason: string | null;
  /** One visible sentence under the switch (loading, refusal, error, nothing to draw), or `null`. */
  readonly note: { readonly tone: "info" | "error"; readonly text: string } | null;
  readonly onChange: (on: boolean) => void;
};

/** Section heading of the left column — spec 04: `text-ink-muted text-[11.5px] font-bold tracking-wide uppercase`. */
const SECTION_TITLE = "m-0 text-[11.5px] font-bold tracking-wide uppercase text-ink-muted";

/** Filter label — shadcn `Label` look at 11.5px (spec 04 §2). */
const FILTER_LABEL = "text-[11.5px] leading-none font-medium text-ink-900";

/** Filter select — spec 04 §2, verbatim classes. */
const FILTER_SELECT =
  "mt-1 h-9 w-full rounded-md border border-line bg-white px-2.5 text-[12.5px] text-ink-900 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50";

/**
 * The "?" of spec 08 B, kept beside a label for a part the server cannot serve yet: a HelpCircle with a
 * tooltip. A BUTTON, so a keyboard reaches it (the disabled control next to it takes no focus), and its
 * `aria-label` holds the whole reason — touch and screen readers get no tooltip (`tooltip.tsx`).
 */
function HelpHint({ info }: { info: PendingFeatureInfo }) {
  return (
    <Tooltip content={info.viSao}>
      <button
        type="button"
        data-help-hint=""
        aria-label={`${info.ten}: ${info.viSao}`}
        className="relative inline-grid shrink-0 cursor-help place-items-center border-0 bg-transparent p-0 text-ink-muted/70 before:absolute before:-inset-2 before:content-[''] hover:text-ink-muted focus-visible:outline-2 focus-visible:outline-brand-500"
      >
        <HelpCircle aria-hidden="true" focusable="false" className="size-3.5" />
      </button>
    </Tooltip>
  );
}

/** One filter: label above, control below (spec 04 §2 `Field`). `hint` sits on the label line. */
function FilterField({ id, label, hint, children }: { id: string; label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <div className="flex items-center gap-1.5">
        <label htmlFor={id} className={FILTER_LABEL}>
          {label}
        </label>
        {hint}
      </div>
      {children}
    </div>
  );
}

/**
 * "LỚP BẢN ĐỒ" (spec §4.1, prototype `LayerPanel.tsx`): one row per group — swatch, name, count, and
 * "chỉ lớp này" on hover/focus. Pressing the row shows / hides the group ON THE MAP ONLY (a layer
 * filter; no request). `aria-pressed` says the state in words; the swatch is never the only signal.
 * The count stays visible while a layer is off: hiding is not removing from the figures.
 */
export function LayerPanel({
  types,
  counts,
  hidden,
  onToggle,
  onOnly,
  onHideAll,
  onShowAll,
  centreIsDefault = false,
  heat,
}: {
  types: readonly comms_loaiTaiNguyenRa[];
  counts: ReadonlyMap<string, number>;
  hidden: ReadonlySet<string>;
  onToggle: (code: string) => void;
  /** Show this group alone. */
  onOnly: (code: string) => void;
  onHideAll: () => void;
  onShowAll: () => void;
  /** The frame in effect is the platform default (ADR 0072 K3): the legend says the centre is the default one. */
  centreIsDefault?: boolean;
  /** "Bản đồ nhiệt phản ánh" — state owned by the screen (it fetches the points). */
  heat: HeatSwitchState;
}) {
  // Prototype: "Ẩn hết" only while EVERY layer is on; any layer off offers "Hiện hết".
  const allOn = types.every((t) => !hidden.has(t.code));
  return (
    <section aria-labelledby="layers-title" className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <h2 id="layers-title" className={SECTION_TITLE}>
          {LAYERS_TITLE}
        </h2>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 px-2"
          icon={allOn ? <EyeOff aria-hidden="true" className="size-3.5" /> : <Eye aria-hidden="true" className="size-3.5" />}
          onClick={allOn ? onHideAll : onShowAll}
        >
          {allOn ? HIDE_ALL : SHOW_ALL}
        </Button>
      </div>
      <ul className="m-0 flex list-none flex-col gap-0.5 p-0">
        {types.map((t) => {
          const on = !hidden.has(t.code);
          return (
            <li key={t.id}>
              <div className={cn("group flex min-w-0 items-center gap-2 rounded-[8px] px-2 py-1.5 hover:bg-canvas", !on && "opacity-50")}>
                <button
                  type="button"
                  aria-pressed={on}
                  data-layer-toggle={t.code}
                  onClick={() => onToggle(t.code)}
                  title={t.label}
                  className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 border-0 bg-transparent p-0 text-left text-[12.5px] text-navy focus-visible:outline-2 focus-visible:outline-brand-500"
                >
                  <span aria-hidden="true" className={cn("inline-block size-2.5 shrink-0 rounded-full", groupSwatchClass(t.code))} />
                  <span className="min-w-0 truncate">{t.label}</span>
                </button>
                <span className="shrink-0 text-[11.5px] tabular-nums text-ink-muted">{counts.get(t.code) ?? 0}</span>
                <button
                  type="button"
                  data-layer-only={t.code}
                  aria-label={`${ONLY_LAYER_TITLE}: ${t.label}`}
                  title={ONLY_LAYER_TITLE}
                  onClick={() => onOnly(t.code)}
                  className="shrink-0 cursor-pointer border-0 bg-transparent p-0 text-[10.5px] text-brand opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-2 focus-visible:outline-brand-500"
                >
                  {ONLY_LAYER}
                </button>
              </div>
            </li>
          );
        })}
      </ul>
      {/* Prototype `LayerPanel.tsx` / spec 04: a bordered row, flame, label, Switch on the right. The whole
          row is the label, so it toggles from anywhere on it. Disabled (reason in `title`) without
          `feedback.read` or without a map — hiding it would leave staff guessing; the server checks
          `feedback.read` on the points route regardless (rule 5, #1). */}
      <label
        htmlFor="heatmap-toggle"
        data-heat-switch=""
        title={heat.disabledReason ?? undefined}
        className={cn(
          "flex min-w-0 items-center gap-2 rounded-[9px] border border-line p-2.5",
          heat.disabledReason === null ? "cursor-pointer" : "cursor-not-allowed",
        )}
      >
        <Flame aria-hidden="true" className="size-4 shrink-0 text-tangerine" />
        <span className="min-w-0 flex-1 text-[12.5px] text-navy">{HEAT_SWITCH_LABEL}</span>
        <Switch
          id="heatmap-toggle"
          checked={heat.checked}
          disabled={heat.disabledReason !== null}
          title={heat.disabledReason ?? undefined}
          onCheckedChange={heat.onChange}
        />
      </label>
      {heat.note !== null && (
        <p
          className={cn("m-0 text-xs", heat.note.tone === "error" ? "text-danger-600" : "text-ink-500")}
          role={heat.note.tone === "error" ? "alert" : "status"}
          data-heat-switch-note=""
        >
          {heat.note.text}
        </p>
      )}
      {/* Spec 08 B: the legend under the heat switch, each line with its dot. */}
      <ul className="m-0 list-none space-y-1 p-0 text-[11px] leading-snug text-ink-muted">
        <li className="flex items-center gap-2">
          <span aria-hidden="true" className="inline-block size-2.5 shrink-0 rounded-full bg-brand opacity-55" />
          <span className="min-w-0">{UNVERIFIED_LEGEND}</span>
        </li>
        <li className="flex items-center gap-2">
          <span aria-hidden="true" className="inline-block size-2.5 shrink-0 rounded-full bg-[#E5484D] ring-2 ring-[#E5484D]/30" />
          <span className="min-w-0" data-centre-legend="">
            {centreIsDefault ? CENTRE_LEGEND_DEFAULT : CENTRE_LEGEND}
          </span>
        </li>
      </ul>
    </section>
  );
}

export type FilterDraft = {
  readonly search: string;
  readonly industryCode: string;
  readonly residentialUnitId: string;
  readonly status: string;
};

/**
 * "BỘ LỌC" (spec §4.2, prototype `AssetFilters.tsx`). Search, industry, hamlet and status go to the
 * SERVER (they narrow the points and the register — a refetch). "Quy mô lao động" is a disabled "?"
 * control: the API has no such filter. Typing in search also lists matching places; choosing one flies
 * the map there (spec / guide §27). "Xoá lọc" appears only while a filter is set.
 */
export function FilterPanel({
  value,
  units,
  results,
  onSearchChange,
  onChange,
  onClear,
  onChooseResult,
}: {
  value: FilterDraft;
  units: readonly identity_thonToDanPhoRa[];
  /** Matches for the current search; `null` = no search running. */
  results: readonly comms_mapAssetRowOut[] | null;
  onSearchChange: (text: string) => void;
  onChange: (patch: Partial<Omit<FilterDraft, "search">>) => void;
  onClear: () => void;
  onChooseResult: (row: comms_mapAssetRowOut) => void;
}) {
  const active = value.search !== "" || value.industryCode !== "" || value.residentialUnitId !== "" || value.status !== "";
  return (
    <section aria-labelledby="filters-title" className="flex flex-col gap-3">
      <div className="flex min-h-7 items-center justify-between gap-2">
        <h2 id="filters-title" className={SECTION_TITLE}>
          {FILTERS_TITLE}
        </h2>
        {active && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2"
            icon={<X aria-hidden="true" className="size-3.5" />}
            onClick={onClear}
          >
            {CLEAR_FILTERS}
          </Button>
        )}
      </div>
      <div className="relative">
        <label htmlFor="map-search" className="an-thi-giac">
          Tìm kiếm
        </label>
        <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-ink-muted" />
        <input
          id="map-search"
          type="search"
          placeholder={SEARCH_PLACEHOLDER}
          value={value.search}
          autoComplete="off"
          onChange={(e) => onSearchChange(e.target.value)}
          className="h-9 w-full min-w-0 rounded-md border border-line bg-white pr-2.5 pl-8 text-[12.5px] text-ink-900 outline-none placeholder:text-ink-muted focus-visible:ring-[3px] focus-visible:ring-ring/50"
        />
      </div>
      {results !== null && (
        <ul className="m-0 flex max-h-56 list-none flex-col gap-0.5 overflow-y-auto rounded-lg border border-line p-1" aria-label="Kết quả tìm kiếm">
          {results.length === 0 && <li className="px-2 py-1.5 text-[13px] text-ink-500">Không tìm thấy địa điểm nào.</li>}
          {results.map((r) => (
            <li key={r.id}>
              <button
                type="button"
                className="flex min-h-10 w-full min-w-0 flex-col items-start justify-center rounded-md border-0 bg-transparent px-2 text-left hover:bg-brand-50"
                onClick={() => onChooseResult(r)}
              >
                <span className="max-w-full break-words text-[13px] font-semibold text-ink-900">{r.name}</span>
                {(r.address ?? "") !== "" && <span className="max-w-full break-words text-xs text-ink-500">{r.address}</span>}
              </button>
            </li>
          ))}
        </ul>
      )}
      <FilterField id="map-industry" label="Ngành nghề">
        <select id="map-industry" className={FILTER_SELECT} value={value.industryCode} onChange={(e) => onChange({ industryCode: e.target.value })}>
          <option value="">Mọi ngành nghề</option>
          {INDUSTRIES.map((i) => (
            <option key={i.code} value={i.code}>
              {`${i.code} — ${i.label}`}
            </option>
          ))}
        </select>
      </FilterField>
      <FilterField id="map-unit" label="Thôn / Tổ dân phố">
        <select id="map-unit" className={FILTER_SELECT} value={value.residentialUnitId} onChange={(e) => onChange({ residentialUnitId: e.target.value })}>
          <option value="">Toàn xã</option>
          {units.map((u) => (
            <option key={u.id} value={u.id}>
              {u.active ? u.name : `${u.name} (ngừng dùng)`}
            </option>
          ))}
        </select>
      </FilterField>
      {/* No such filter on the server: disabled, the reason on the "?" beside the label (spec 08 B). */}
      <FilterField id="map-labour" label="Quy mô lao động" hint={<HelpHint info={pendingPart("Quy mô lao động")} />}>
        <select id="map-labour" className={FILTER_SELECT} disabled data-pending="">
          <option>Mọi quy mô</option>
        </select>
      </FilterField>
      <FilterField id="map-status" label="Trạng thái">
        <select id="map-status" className={FILTER_SELECT} value={value.status} onChange={(e) => onChange({ status: e.target.value })}>
          <option value="">Mọi trạng thái</option>
          {STATUS_OPTIONS.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </select>
      </FilterField>
    </section>
  );
}

/**
 * "MẬT ĐỘ THEO THÔN" (spec 04 §3, prototype `HamletDensityPanel.tsx`) — not built (ADR 0072 §4): the
 * heading at its position with the "?" help hint carrying the reason (spec 08 B), no figures.
 */
export function DensityPending() {
  return (
    <section aria-labelledby="density-title" className="flex flex-col gap-2" data-pending="">
      <div className="flex items-center gap-1.5">
        <h2 id="density-title" className={SECTION_TITLE}>
          {DENSITY_TITLE}
        </h2>
        <HelpHint info={pendingPart("Mật độ theo thôn")} />
      </div>
    </section>
  );
}
