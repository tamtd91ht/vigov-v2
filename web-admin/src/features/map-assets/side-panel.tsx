"use client";

import { Eye, EyeOff, Flame, Search, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { PendingField, PendingMarker } from "@/components/ui/pending-feature";
import { cn } from "@/lib/cn";
import type { comms_loaiTaiNguyenRa, comms_mapAssetRowOut, identity_thonToDanPhoRa } from "@/lib/api/schema.gen";

import {
  CLEAR_FILTERS,
  DENSITY_TITLE,
  FILTERS_TITLE,
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

/** Section heading of the left column — the prototype's small uppercase label (`LayerPanel.tsx`). */
const SECTION_TITLE = "m-0 text-xs font-semibold tracking-wide text-ink-500";

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
              <div className={cn("group flex min-w-0 items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-surface-subtle", !on && "opacity-50")}>
                <button
                  type="button"
                  aria-pressed={on}
                  data-layer-toggle={t.code}
                  onClick={() => onToggle(t.code)}
                  title={t.label}
                  className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 border-0 bg-transparent p-0 text-left text-[13px] text-ink-900 focus-visible:outline-2 focus-visible:outline-brand-500"
                >
                  <span aria-hidden="true" className={cn("inline-block size-2.5 shrink-0 rounded-full", groupSwatchClass(t.code))} />
                  <span className="min-w-0 truncate">{t.label}</span>
                </button>
                <span className="shrink-0 text-xs tabular-nums text-ink-500">{counts.get(t.code) ?? 0}</span>
                <button
                  type="button"
                  data-layer-only={t.code}
                  aria-label={`${ONLY_LAYER_TITLE}: ${t.label}`}
                  title={ONLY_LAYER_TITLE}
                  onClick={() => onOnly(t.code)}
                  className="shrink-0 cursor-pointer border-0 bg-transparent p-0 text-[11px] text-brand-700 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-2 focus-visible:outline-brand-500"
                >
                  {ONLY_LAYER}
                </button>
              </div>
            </li>
          );
        })}
      </ul>
      {/* Prototype: a bordered row with a flame and a switch. Disabled with "?" — ADR 0072 §4, H2. */}
      <div className="flex min-w-0 items-center gap-2 rounded-lg border border-line p-2.5" data-pending="">
        <Flame aria-hidden="true" className="size-4 shrink-0 text-ink-400" />
        <label htmlFor="heatmap-toggle" className="min-w-0 flex-1 text-[13px] text-ink-500">
          Bản đồ nhiệt phản ánh
        </label>
        <PendingMarker info={pendingPart("Bản đồ nhiệt phản ánh")} />
        <input id="heatmap-toggle" type="checkbox" role="switch" disabled className="size-4 shrink-0 cursor-not-allowed" />
      </div>
      <p className="m-0 text-xs text-ink-500">{UNVERIFIED_LEGEND}</p>
      <p className="m-0 flex items-center gap-2 text-xs text-ink-500">
        <span aria-hidden="true" className="inline-block size-2.5 shrink-0 rounded-full border-2 border-white bg-[#dc2626] ring-2 ring-[#dc2626]/30" />
        <span className="min-w-0" data-centre-legend="">
          {centreIsDefault ? CENTRE_LEGEND_DEFAULT : CENTRE_LEGEND}
        </span>
      </p>
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
      <Field label="Tìm kiếm" hideLabel htmlFor="map-search" icon={Search} grow="auto" className="w-full">
        <input
          id="map-search"
          type="search"
          placeholder={SEARCH_PLACEHOLDER}
          value={value.search}
          autoComplete="off"
          onChange={(e) => onSearchChange(e.target.value)}
        />
      </Field>
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
      <Field label="Ngành nghề" htmlFor="map-industry" kind="select" grow="auto" className="w-full">
        <select id="map-industry" value={value.industryCode} onChange={(e) => onChange({ industryCode: e.target.value })}>
          <option value="">Mọi ngành nghề</option>
          {INDUSTRIES.map((i) => (
            <option key={i.code} value={i.code}>
              {`${i.code} — ${i.label}`}
            </option>
          ))}
        </select>
      </Field>
      <Field label="Thôn / Tổ dân phố" htmlFor="map-unit" kind="select" grow="auto" className="w-full">
        <select id="map-unit" value={value.residentialUnitId} onChange={(e) => onChange({ residentialUnitId: e.target.value })}>
          <option value="">Toàn xã</option>
          {units.map((u) => (
            <option key={u.id} value={u.id}>
              {u.active ? u.name : `${u.name} (ngừng dùng)`}
            </option>
          ))}
        </select>
      </Field>
      <PendingField
        info={pendingPart("Quy mô lao động")}
        id="map-labour"
        kind="select"
        placeholder="Mọi quy mô"
        className="w-full max-w-none min-w-0 flex-none"
      />
      <Field label="Trạng thái" htmlFor="map-status" kind="select" grow="auto" className="w-full">
        <select id="map-status" value={value.status} onChange={(e) => onChange({ status: e.target.value })}>
          <option value="">Mọi trạng thái</option>
          {STATUS_OPTIONS.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </select>
      </Field>
    </section>
  );
}

/**
 * "MẬT ĐỘ THEO THÔN" (spec §4.3, prototype `HamletDensityPanel.tsx`) — not built (ADR 0072 §4): the
 * prototype's plain heading at its position, with the "?" and one sentence, no figures.
 */
export function DensityPending() {
  return (
    <section aria-labelledby="density-title" className="flex flex-col gap-2" data-pending="">
      <div className="flex items-center gap-1.5">
        <h2 id="density-title" className={SECTION_TITLE}>
          {DENSITY_TITLE}
        </h2>
        <PendingMarker info={pendingPart("Mật độ theo thôn")} />
      </div>
      <p className="m-0 text-xs text-ink-500">Số cơ sở và số phản ánh theo từng thôn — tính năng đang phát triển.</p>
    </section>
  );
}
