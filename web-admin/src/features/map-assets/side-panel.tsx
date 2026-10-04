"use client";

import { Eye, EyeOff, Search } from "lucide-react";

import { Field } from "@/components/ui/field";
import { PendingField, PendingMarker, PendingSection } from "@/components/ui/pending-feature";
import { cn } from "@/lib/cn";
import type { comms_loaiTaiNguyenRa, comms_mapAssetRowOut, identity_thonToDanPhoRa } from "@/lib/api/schema.gen";

import {
  FILTERS_TITLE,
  HIDE_ALL,
  INDUSTRIES,
  LAYERS_TITLE,
  SEARCH_PLACEHOLDER,
  SHOW_ALL,
  STATUS_OPTIONS,
  UNVERIFIED_LEGEND,
  CENTRE_LEGEND,
  pendingPart,
} from "./labels";
import { groupSwatchClass } from "./map-logic";

/**
 * "LỚP BẢN ĐỒ" (spec §4.1): one row per group — swatch, name, count — pressing it shows / hides the
 * group ON THE MAP ONLY (a layer filter; no request). `aria-pressed` says the state in words for a
 * screen reader; the swatch is never the only signal.
 */
export function LayerPanel({
  types,
  counts,
  hidden,
  onToggle,
  onHideAll,
  onShowAll,
}: {
  types: readonly comms_loaiTaiNguyenRa[];
  counts: ReadonlyMap<string, number>;
  hidden: ReadonlySet<string>;
  onToggle: (code: string) => void;
  onHideAll: () => void;
  onShowAll: () => void;
}) {
  const allHidden = types.length > 0 && types.every((t) => hidden.has(t.code));
  return (
    <section aria-labelledby="layers-title" className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-2">
        <h2 id="layers-title" className="m-0 text-xs font-semibold tracking-wide text-ink-500">
          {LAYERS_TITLE}
        </h2>
        <button
          type="button"
          className="inline-flex min-h-8 items-center gap-1 rounded-lg border-0 bg-transparent px-2 text-xs font-semibold text-brand-700 hover:bg-brand-50"
          onClick={allHidden ? onShowAll : onHideAll}
        >
          {allHidden ? <Eye aria-hidden="true" className="size-4" /> : <EyeOff aria-hidden="true" className="size-4" />}
          {allHidden ? SHOW_ALL : HIDE_ALL}
        </button>
      </div>
      <ul className="m-0 flex list-none flex-col gap-0.5 p-0">
        {types.map((t) => {
          const on = !hidden.has(t.code);
          return (
            <li key={t.id}>
              <button
                type="button"
                aria-pressed={on}
                data-layer-toggle={t.code}
                onClick={() => onToggle(t.code)}
                className={cn(
                  "flex min-h-10 w-full items-center gap-2 rounded-lg border-0 bg-transparent px-2 text-left text-[13px] hover:bg-brand-50",
                  "focus-visible:outline-2 focus-visible:outline-brand-500",
                  on ? "text-ink-900" : "text-ink-400 line-through",
                )}
              >
                <span aria-hidden="true" className={cn("inline-block size-3 shrink-0 rounded-full", groupSwatchClass(t.code), !on && "opacity-30")} />
                <span className="min-w-0 flex-1">{t.label}</span>
                <span className="tabular-nums text-ink-500">{counts.get(t.code) ?? 0}</span>
              </button>
            </li>
          );
        })}
      </ul>
      <p className="m-0 text-xs text-ink-500">{UNVERIFIED_LEGEND}</p>
      <p className="m-0 flex items-center gap-2 text-xs text-ink-500">
        <span aria-hidden="true" className="inline-block size-2.5 shrink-0 rounded-full border-2 border-white bg-[#dc2626] ring-2 ring-[#dc2626]/30" />
        {CENTRE_LEGEND}
      </p>
      <div className="flex min-h-10 items-center gap-2" data-pending="">
        <input id="heatmap-toggle" type="checkbox" role="switch" disabled className="size-4 cursor-not-allowed" />
        <label htmlFor="heatmap-toggle" className="text-[13px] text-ink-500">
          Bản đồ nhiệt phản ánh
        </label>
        <PendingMarker info={pendingPart("Bản đồ nhiệt phản ánh")} />
      </div>
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
 * "BỘ LỌC" (spec §4.2). Search, industry, hamlet and status go to the SERVER (they narrow the points
 * and the register — a refetch). "Quy mô lao động" is a disabled "?" control: the API has no such filter.
 * Typing in search also lists matching places; choosing one flies the map there (spec / guide §27).
 */
export function FilterPanel({
  value,
  units,
  results,
  onSearchChange,
  onChange,
  onChooseResult,
}: {
  value: FilterDraft;
  units: readonly identity_thonToDanPhoRa[];
  /** Matches for the current search; `null` = no search running. */
  results: readonly comms_mapAssetRowOut[] | null;
  onSearchChange: (text: string) => void;
  onChange: (patch: Partial<Omit<FilterDraft, "search">>) => void;
  onChooseResult: (row: comms_mapAssetRowOut) => void;
}) {
  return (
    <section aria-labelledby="filters-title" className="flex flex-col gap-3">
      <h2 id="filters-title" className="m-0 text-xs font-semibold tracking-wide text-ink-500">
        {FILTERS_TITLE}
      </h2>
      <Field label="Tìm kiếm" htmlFor="map-search" icon={Search} grow="auto" className="w-full">
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
                className="flex min-h-10 w-full flex-col items-start justify-center rounded-md border-0 bg-transparent px-2 text-left hover:bg-brand-50"
                onClick={() => onChooseResult(r)}
              >
                <span className="text-[13px] font-semibold text-ink-900">{r.name}</span>
                {(r.address ?? "") !== "" && <span className="text-xs text-ink-500">{r.address}</span>}
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
              {`${i.code} · ${i.label}`}
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
        className="w-full max-w-none flex-none"
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

/** "MẬT ĐỘ THEO THÔN" (spec §4.3) — not built (ADR 0072 §4): a disabled "?" section at its position. */
export function DensityPending() {
  return <PendingSection info={pendingPart("Mật độ theo thôn")} title="MẬT ĐỘ THEO THÔN" titleAs="h2" />;
}
