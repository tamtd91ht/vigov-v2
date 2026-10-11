"use client";

import { Download, Layers, List, Map as MapIcon, MapPinned, Maximize2, Minimize2, Plus, RotateCcw, Upload } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";

import { useCauHinhXa } from "@/components/cau-hinh-xa"; // vi-name-ok: existing export (rule 12 inv 3)
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { inertOutside } from "@/features/dashboard/presentation";
import { OverlayDialog } from "@/features/noi-dung/overlay-dialog";
import { cn } from "@/lib/cn";
// vi-name-ok: existing module and exports (rule 12 inv 3) — /phan-anh's own sentences, one copy (rule 9)
import { HEAT_MAP_EMPTY_TITLE, HEAT_MAP_LOAD_FAILED, HEAT_MAP_LOADING } from "@/features/phan-anh/nhan-phieu";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layLoaiTaiNguyenBanDo } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import {
  NO_FILTER,
  getMapAsset,
  getMapAssetSummary,
  listMapAssetPoints,
  listMapAssets,
  seedMapAssetTypeDefaults,
  setMapAssetConfirmation,
  type MapAssetFilter,
} from "@/lib/api/map-assets";
import { listMapFields } from "@/lib/api/map-field-schemas";
import { getMapFrame } from "@/lib/api/map-frame";
import { listCitizenReportPoints, type PointsResult } from "@/lib/api/phieu-phan-anh"; // vi-name-ok: existing module (rule 12 inv 3)
import type {
  comms_danhSachLoaiTaiNguyenRa,
  comms_mapAssetOut,
  comms_mapAssetRowOut,
  comms_mapAssetSummaryOut,
  comms_mapFieldSchemaOut,
  comms_mapFrameOut,
  identity_thonToDanPhoRa,
} from "@/lib/api/schema.gen";
import { layDanhSachThonToDanPho } from "@/lib/api/thon-to-dan-pho";
import { BASEMAP_MISSING_SENTENCE } from "@/lib/basemap/assets";
import { coQuyen, QUYEN_QUAN_LY_DANH_MUC, QUYEN_XEM_PHAN_ANH } from "@/lib/quyen"; // vi-name-ok: existing exports (rule 12 inv 3)

import { AssetDialog, type AssetDialogMode } from "./asset-dialog";
import { DeleteAssetDialog, DetailPanel } from "./detail-panel";
import { EconomicMap, type FlyRequest } from "./economic-map";
import { FLOATING_BUTTON, MapExpandToggle } from "./map-expand";
import { FrameForm, ResetFrameDialog } from "./frame-form";
import {
  ADD_BUTTON,
  GROUP_SELECT_LABEL,
  PRESENTATION_BUTTON,
  PRESENTATION_KPIS,
  PRESENTATION_TITLE,
  SUMMARY_LOADING,
  ASSET_UPDATE_PERMISSION,
  FRAME_ASK_ADMIN,
  FRAME_CHANGE_BUTTON,
  FRAME_LOAD_FAILED,
  FRAME_NOT_SET,
  FRAME_RESET_BUTTON,
  FRAME_RESET_DONE,
  FRAME_RESET_DONE_NO_DEFAULT,
  FRAME_SAVED,
  HEAT_NEEDS_FEEDBACK_READ,
  HEAT_NEEDS_MAP,
  MAP_LOAD_FAILED,
  NO_BASEMAP_DETAIL,
  NO_GROUPS,
  NO_GROUPS_ASK_ADMIN,
  OUTSIDE_FRAME,
  PAGE_TITLE,
  SEED_DEFAULTS_BUTTON,
  VIEW_MAP,
  VIEW_REGISTER,
  CENTRE_FALLBACK_LABEL,
  pendingPart,
} from "./labels";
import {
  EMPTY_COLLECTION,
  frameDefaultFromApi,
  frameFromApi,
  frameHintsFromApi,
  insideFrame,
  meanCentre,
  summaryLine,
  toEconomicCollection,
  type EconomicCollection,
} from "./map-logic";
import { RegisterTable } from "./register-table";
import { DensityPending, FilterPanel, LayerPanel, type HeatSwitchState } from "./side-panel";

type View = "map" | "register";

type Dialog =
  | { readonly kind: "asset"; readonly mode: AssetDialogMode }
  | { readonly kind: "delete"; readonly asset: { readonly id: string; readonly name: string } }
  | { readonly kind: "frame" }
  | { readonly kind: "reset" };

type Points = { readonly ok: true; readonly collection: EconomicCollection } | { readonly ok: false; readonly message: string };

type Register = { readonly rows: readonly comms_mapAssetRowOut[]; readonly cursor: string; readonly hasMore: boolean };

const REGISTER_PAGE = 100;
const SEARCH_RESULTS = 10;
const SEARCH_DEBOUNCE_MS = 300;

/**
 * `/ban-do` — "Bản đồ phát triển kinh tế số" (`docs/ui-ux/10-ban-do-kinh-te-so.md`, ADR 0072).
 *
 * WHAT DECIDES WHETHER A MAP IS DRAWN, in order — each "no" draws NO MAP, never a fallback view:
 *   1. the frame has loaded and is configured (`GET /api/v1/map-frame`) — ADR 0072 H3;
 *   2. the SELF-HOSTED basemap is configured (`BASEMAP_URL`, decided on the server page and passed in as
 *      `basemapAvailable`) — the same basemap and the same sentence as `/phan-anh` (owner 11/10/2026);
 *   3. the style loaded.
 * The register ("Sổ địa điểm") works in every one of those cases.
 *
 * PERMISSIONS ARE CONVENIENCE HERE (rule 5, #1): `asset.read` gates the whole screen (`page.tsx`),
 * `asset.update` shows the write controls, `admin.lookup` the frame form, "Về mặc định" and the
 * default-groups button, `feedback.read` the heat-layer switch. The server checks each on every request.
 *
 * THE HEAT LAYER reads citizens' petition coordinates (`GET /api/v1/citizen-report-points`, the route
 * `/phan-anh` draws from). They are asked for only while the switch is on AND a map is on screen, held in
 * state for the map and nowhere else — never logged, never in a URL (rule 3). A refusal (422: more than
 * the cap) or an error draws NOTHING: a heat map of part of the points looks like the commune's and is not.
 */
/** The region "Mở rộng bản đồ" grows into the page overlay (`aria-controls` of the toggle). */
const MAP_REGION_ID = "economic-map-region";

export function EconomicMapScreen({ basemapAvailable }: { basemapAvailable: boolean }) {
  const phien = usePhien();
  const permissions = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const canUpdate = coQuyen(permissions, ASSET_UPDATE_PERMISSION);
  const canAdminLookup = coQuyen(permissions, QUYEN_QUAN_LY_DANH_MUC);
  const canReadFeedback = coQuyen(permissions, QUYEN_XEM_PHAN_ANH);

  const [types, setTypes] = useState<KetQua<comms_danhSachLoaiTaiNguyenRa> | null>(null);
  const [typesReload, setTypesReload] = useState(0);
  const [frameRes, setFrameRes] = useState<KetQua<comms_mapFrameOut> | null>(null);
  // Commune name for the centre landmark — the same display name the header shows (from Host, ADR 0069).
  const centreLabel = useCauHinhXa().displayName.trim() || CENTRE_FALLBACK_LABEL;
  // "Mở rộng bản đồ": the map region (map + detail panel) grows into a page overlay; Esc or "Thu gọn"
  // brings it back. A page state, not the Fullscreen API (map-expand.tsx says why).
  const [mapExpandedState, setMapExpanded] = useState(false);
  const [frameReload, setFrameReload] = useState(0);
  const [summary, setSummary] = useState<KetQua<comms_mapAssetSummaryOut> | null>(null);
  const [units, setUnits] = useState<readonly identity_thonToDanPhoRa[]>([]);
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<MapAssetFilter>(NO_FILTER);
  const [points, setPoints] = useState<Points | null>(null);
  const [reload, setReload] = useState(0);
  const [hidden, setHidden] = useState<ReadonlySet<string>>(new Set());
  const [view, setView] = useState<View>("map");
  const [register, setRegister] = useState<KetQua<Register> | null>(null);
  const [results, setResults] = useState<readonly comms_mapAssetRowOut[] | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<KetQua<comms_mapAssetOut> | "loading" | null>(null);
  const [detailFields, setDetailFields] = useState<readonly comms_mapFieldSchemaOut[]>([]);
  const [flyTo, setFlyTo] = useState<FlyRequest | null>(null);
  // "Trình chiếu" (spec 03): the whole screen as a full-window overlay for a meeting room. A page
  // state, not the Fullscreen API and not in the URL — the same reasoning as Tổng quan's mode
  // (`features/dashboard/presentation.tsx`).
  const [presenting, setPresenting] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState(false);
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const [mapFailed, setMapFailed] = useState(false);
  const [chosenGroup, setChosenGroup] = useState("");
  const [heatOn, setHeatOn] = useState(false);
  /** Tagged with the activation it answers, so a reply from an earlier "on" is never shown as the current one. */
  const [heatResult, setHeatResult] = useState<{ readonly seq: number; readonly result: PointsResult } | null>(null);
  const [heatSeq, setHeatSeq] = useState(0);
  const detailSeq = useRef(0);
  const skipLogged = useRef(false);

  /* ---- reads ---------------------------------------------------------------------------------- */

  useEffect(() => {
    let gone = false;
    layLoaiTaiNguyenBanDo().then((r) => !gone && setTypes(r));
    return () => {
      gone = true;
    };
  }, [typesReload]);

  useEffect(() => {
    let gone = false;
    getMapFrame().then((r) => !gone && setFrameRes(r));
    return () => {
      gone = true;
    };
  }, [frameReload]);

  useEffect(() => {
    let gone = false;
    getMapAssetSummary().then((r) => !gone && setSummary(r));
    return () => {
      gone = true;
    };
  }, [reload, typesReload]);

  useEffect(() => {
    let gone = false;
    layDanhSachThonToDanPho().then((r) => !gone && r.ok && setUnits(r.duLieu.items));
    return () => {
      gone = true;
    };
  }, []);

  // Server filters → refetch the points. Layer toggles never come through here.
  useEffect(() => {
    let gone = false;
    listMapAssetPoints(filter).then((r) => {
      if (gone) return;
      if (!r.ok) return setPoints({ ok: false, message: r.thongBao });
      const { collection, skipped } = toEconomicCollection(r.duLieu);
      if (skipped > 0 && !skipLogged.current) {
        skipLogged.current = true;
        // A COUNT only — never a name or id (rule 3).
        console.warn(`Bản đồ kinh tế số: bỏ qua ${skipped} điểm có toạ độ không hợp lệ.`);
      }
      setPoints({ ok: true, collection });
    });
    return () => {
      gone = true;
    };
  }, [filter, reload]);

  useEffect(() => {
    const t = setTimeout(() => setFilter((f) => (f.q === search ? f : { ...f, q: search })), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    let gone = false;
    if (filter.q.trim() === "") {
      const t = setTimeout(() => !gone && setResults(null), 0);
      return () => {
        gone = true;
        clearTimeout(t);
      };
    }
    listMapAssets(filter, "", SEARCH_RESULTS).then((r) => !gone && setResults(r.ok ? r.duLieu.items : []));
    return () => {
      gone = true;
    };
  }, [filter, reload]);

  useEffect(() => {
    if (view !== "register") return;
    let gone = false;
    listMapAssets(filter, "", REGISTER_PAGE).then((r) => {
      if (gone) return;
      setRegister(
        r.ok ? { ok: true, duLieu: { rows: r.duLieu.items, cursor: r.duLieu.next_cursor, hasMore: r.duLieu.has_more } } : r,
      );
    });
    return () => {
      gone = true;
    };
  }, [view, filter, reload]);

  /* ---- derived -------------------------------------------------------------------------------- */

  const typeItems = useMemo(
    () => (types !== null && types.ok ? [...types.duLieu.items].sort((a, b) => a.order - b.order) : []),
    [types],
  );
  const frameOut = frameRes !== null && frameRes.ok ? frameRes.duLieu : null;
  const frame = useMemo(() => (frameOut === null ? null : frameFromApi(frameOut)), [frameOut]);
  const defaultFrame = useMemo(() => (frameOut === null ? null : frameDefaultFromApi(frameOut)), [frameOut]);
  const hints = useMemo(() => (frameOut === null ? null : frameHintsFromApi(frameOut)), [frameOut]);
  const collection = points !== null && points.ok ? points.collection : EMPTY_COLLECTION;
  const visible = useMemo(() => {
    const codes = new Set<string>(typeItems.map((t) => t.code));
    for (const f of collection.features) codes.add(f.properties.asset_type_code);
    for (const h of hidden) codes.delete(h);
    return codes;
  }, [typeItems, collection, hidden]);
  const counts = useMemo(
    () => new Map((summary !== null && summary.ok ? summary.duLieu.by_type : []).map((b) => [b.asset_type_code, b.count])),
    [summary],
  );
  const groupLabel = useCallback((code: string) => typeItems.find((t) => t.code === code)?.label ?? code, [typeItems]);
  const unitName = useCallback((id: string) => units.find((u) => u.id === id)?.name ?? "—", [units]);
  const activeTypes = typeItems.filter((t) => t.active);
  const currentGroup = activeTypes.some((t) => t.code === chosenGroup) ? chosenGroup : (activeTypes[0]?.code ?? "");
  const mapAvailable = frame !== null && basemapAvailable && !mapFailed;
  // "Trình chiếu" always shows the map, even from Sổ địa điểm (spec 03); leaving it returns to that view.
  const mapShown = view === "map" || presenting;
  // Expanded only while a map is actually on screen: switching to Sổ địa điểm, or losing the map,
  // returns to the normal layout by itself — derived, so it cannot get stuck open. "Trình chiếu" is
  // already full-window, so expanding means nothing there.
  const mapExpanded = mapExpandedState && mapShown && mapAvailable && !presenting;

  // The heat layer is LIVE only while it can be drawn: no map → no citizen coordinate is asked for.
  const heatLive = heatOn && canReadFeedback && mapAvailable && mapShown;
  useEffect(() => {
    if (!heatLive) return;
    let gone = false;
    listCitizenReportPoints().then((result) => !gone && setHeatResult({ seq: heatSeq, result }));
    return () => {
      gone = true;
    };
  }, [heatLive, heatSeq]);
  const heatCurrent = heatLive && heatResult !== null && heatResult.seq === heatSeq ? heatResult.result : null;
  const heatPoints = heatCurrent !== null && heatCurrent.ok ? heatCurrent.data.items : null;
  const heatDisabledReason = !canReadFeedback ? HEAT_NEEDS_FEEDBACK_READ : !mapAvailable ? HEAT_NEEDS_MAP : null;
  const heat: HeatSwitchState = {
    checked: heatOn && heatDisabledReason === null,
    disabledReason: heatDisabledReason,
    note: !heatLive
      ? null
      : heatCurrent === null
        ? { tone: "info", text: HEAT_MAP_LOADING }
        : !heatCurrent.ok
          ? heatCurrent.tooManyPoints
            ? // The server's sentence verbatim: the way out is narrowing, not retrying.
              { tone: "info", text: heatCurrent.message }
            : { tone: "error", text: `${HEAT_MAP_LOAD_FAILED}: ${heatCurrent.message}` }
          : heatCurrent.data.items.length === 0
            ? { tone: "info", text: HEAT_MAP_EMPTY_TITLE }
            : null,
    onChange: (on) => {
      setHeatOn(on);
      // Each "on" is a fresh read: turning it off and on again is the retry after an error.
      if (on) setHeatSeq((n) => n + 1);
    },
  };

  // "Trình chiếu": lock the page behind, make the shell (sidebar, topbar) inert so Tab stays in the
  // overlay, Esc leaves — unless a dialog or popover owns Esc first. Focus stays on the toggle: it is
  // the same element in both states (the header keeps its position in the tree).
  useEffect(() => {
    const root = rootRef.current;
    if (!presenting || root === null) return;
    const overflowBefore = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const madeInert = inertOutside(root);
    if (!root.contains(document.activeElement)) root.focus({ preventScroll: true });
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape" || e.defaultPrevented) return;
      if (document.querySelector('[role="dialog"], dialog[open]') !== null) return;
      setPresenting(false);
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = overflowBefore;
      for (const el of madeInert) el.removeAttribute("inert");
      root.querySelector<HTMLElement>("[data-map-presentation-toggle]")?.focus({ preventScroll: true });
    };
  }, [presenting]);

  useEffect(() => {
    if (!mapExpanded) return;
    // The page behind the overlay must not scroll under it.
    const before = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKey = (e: KeyboardEvent) => {
      // Esc belongs to an open dialog first (it closes the dialog); only then to the overlay.
      if (e.key === "Escape" && !e.defaultPrevented && dialog === null) setMapExpanded(false);
    };
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = before;
      window.removeEventListener("keydown", onKey);
    };
  }, [mapExpanded, dialog]);

  /* ---- actions -------------------------------------------------------------------------------- */

  /** A frame reply from a write or a reload becomes the frame in effect; a failed map gets a new chance. */
  const adoptFrame = useCallback((out: comms_mapFrameOut) => {
    setFrameRes({ ok: true, duLieu: out });
    setMapFailed(false);
  }, []);

  const openAsset = useCallback(
    async (id: string, fly: boolean) => {
      const seq = ++detailSeq.current;
      setSelectedId(id);
      setDetail("loading");
      setActionError("");
      const r = await getMapAsset(id);
      if (seq !== detailSeq.current) return;
      setDetail(r);
      if (!r.ok) return;
      listMapFields(r.duLieu.asset_type_code).then((f) => seq === detailSeq.current && setDetailFields(f.ok ? f.duLieu.items : []));
      if (!fly) return;
      // Never move the camera outside the frame (security review 04/10/2026, #2).
      if (frame === null || !insideFrame(frame.bounds, r.duLieu.lng, r.duLieu.lat)) return void toast.info(OUTSIDE_FRAME);
      setFlyTo((prev) => ({ lng: r.duLieu.lng, lat: r.duLieu.lat, seq: (prev?.seq ?? 0) + 1 }));
    },
    [frame],
  );

  const closeDetail = useCallback(() => {
    detailSeq.current += 1;
    setSelectedId(null);
    setDetail(null);
    setActionError("");
  }, []);

  async function editAsset(id: string) {
    setActionError("");
    const r = await getMapAsset(id);
    if (!r.ok) return setActionError(r.thongBao);
    setDialog({ kind: "asset", mode: { kind: "edit", asset: r.duLieu } });
  }

  async function toggleVerified(a: comms_mapAssetOut) {
    setBusy(true);
    setActionError("");
    const r = await setMapAssetConfirmation(a.id, !a.verified);
    setBusy(false);
    if (!r.ok) return setActionError(r.thongBao);
    setDetail(r);
    toast.success(r.duLieu.verified ? `Đã xác minh ${r.duLieu.name}.` : `Đã bỏ xác minh ${r.duLieu.name}.`);
    setReload((n) => n + 1);
  }

  async function seedDefaults() {
    setBusy(true);
    setActionError("");
    const r = await seedMapAssetTypeDefaults();
    setBusy(false);
    if (!r.ok) return setActionError(r.thongBao);
    toast.success(`Đã nạp ${r.duLieu.created_count} nhóm tài nguyên mặc định.`);
    setTypesReload((n) => n + 1);
  }

  async function loadMore() {
    if (register === null || !register.ok || !register.duLieu.hasMore) return;
    const before = register.duLieu;
    const r = await listMapAssets(filter, before.cursor, REGISTER_PAGE);
    if (!r.ok) return setActionError(r.thongBao);
    setRegister({
      ok: true,
      duLieu: { rows: [...before.rows, ...r.duLieu.items], cursor: r.duLieu.next_cursor, hasMore: r.duLieu.has_more },
    });
  }

  /* ---- parts ---------------------------------------------------------------------------------- */

  /** The reason a header button is disabled — the `PHAN_CHUA_DUNG` sentence (one copy, rule 9). */
  const excelButton = (name: "Mẫu Excel" | "Nhập Excel", Icon: typeof Download) => {
    const reason = pendingPart(name).viSao;
    // Spec 08 A4: an outline button, DISABLED (no backend route), the reason in `title` — no "?".
    // The wrapper carries the title too: a disabled button takes no pointer events, so its own
    // `title` never shows on hover.
    return (
      <span title={reason} className="inline-flex">
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled
          title={reason}
          icon={<Icon aria-hidden="true" focusable="false" className="size-3.5" />}
        >
          {name}
        </Button>
      </span>
    );
  };

  const summaryText =
    summary === null ? SUMMARY_LOADING : summary.ok ? summaryLine(summary.duLieu.total, summary.duLieu.verified_ratio) : summary.thongBao;

  // Prototype `MapWorkspace.tsx` / spec 03: ONE white bar — title and figures on the left, the toolbar on
  // the right of the same row. In "Trình chiếu" only the 26px title and the toggle remain. The toggle
  // keeps its position in the tree in both states, so the element that had focus is the one still there.
  const header = (
    <header className="flex shrink-0 flex-wrap items-center gap-3 border-b border-line bg-white px-5 py-3">
      <div className="min-w-0">
        <h1 className={cn("m-0 leading-tight font-bold text-navy", presenting ? "text-[26px]" : "text-[18px]")}>{PAGE_TITLE}</h1>
        {!presenting && (
          <p className="m-0 mt-0.5 text-[12px] text-ink-muted" data-summary-line="">
            {summaryText}
          </p>
        )}
      </div>
      <div className="ml-auto flex flex-wrap items-center gap-2">
        {!presenting && (
          <>
            {/* Also the default group of "Thêm đối tượng". Hidden while the commune has no active group. */}
            {activeTypes.length > 0 && (
              <select
                aria-label={GROUP_SELECT_LABEL}
                className="h-9 max-w-56 min-w-0 rounded-md border border-line bg-white px-2.5 text-[12.5px] text-ink-900"
                value={currentGroup}
                onChange={(e) => setChosenGroup(e.target.value)}
              >
                {activeTypes.map((t) => (
                  <option key={t.id} value={t.code}>
                    {t.label}
                  </option>
                ))}
              </select>
            )}
            {excelButton("Mẫu Excel", Download)}
            {canUpdate && excelButton("Nhập Excel", Upload)}
            {canUpdate && activeTypes.length > 0 && (
              <Button
                type="button"
                variant="primary"
                size="sm"
                icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
                onClick={() =>
                  setDialog({
                    kind: "asset",
                    mode: { kind: "create", assetTypeCode: currentGroup, idempotencyKey: crypto.randomUUID() },
                  })
                }
              >
                {ADD_BUTTON}
              </Button>
            )}
            {/* The map answers "what is where"; the register answers "what does the commune manage" — the
                question data-entry staff ask more often (prototype `MapWorkspace.tsx`). Toggle buttons with
                `aria-pressed`, reached and pressed with Tab / Enter / Space like any button. */}
            <div role="group" aria-label="Chế độ xem" className="flex gap-1 rounded-[10px] border border-line bg-canvas p-1">
              {(
                [
                  ["map", VIEW_MAP, MapIcon],
                  ["register", VIEW_REGISTER, List],
                ] as const
              ).map(([key, label, Icon]) => (
                <button
                  key={key}
                  type="button"
                  aria-pressed={view === key}
                  onClick={() => setView(key)}
                  className={cn(
                    "flex cursor-pointer items-center gap-1.5 rounded-[8px] border-0 px-3 py-1.5 text-[12.5px] font-semibold [font-family:inherit]",
                    "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
                    view === key ? "bg-white text-navy shadow-card" : "bg-transparent text-ink-muted",
                  )}
                >
                  <Icon aria-hidden="true" focusable="false" className="size-3.5" />
                  {label}
                </button>
              ))}
            </div>
          </>
        )}
        <Button
          type="button"
          size="sm"
          variant={presenting ? "primary" : "outline"}
          title={PRESENTATION_TITLE}
          aria-pressed={presenting}
          data-map-presentation-toggle=""
          icon={
            presenting ? (
              <Minimize2 aria-hidden="true" focusable="false" className="size-4" />
            ) : (
              <Maximize2 aria-hidden="true" focusable="false" className="size-4" />
            )
          }
          onClick={() => setPresenting((v) => !v)}
        >
          {PRESENTATION_BUTTON}
        </Button>
      </div>
    </header>
  );

  // "Trình chiếu" KPI row (spec 03). Two figures the summary counts; the other two have no source on the
  // server, so "—" with the reason — a 0 would be a false figure on a projector in front of leadership.
  const kpiRow = (
    <div className="grid shrink-0 grid-cols-2 gap-4 px-6 py-4 md:grid-cols-4">
      {PRESENTATION_KPIS.map((k) => {
        const reason = "reason" in k ? k.reason : null;
        const value =
          "typeCode" in k && summary !== null && summary.ok ? (counts.get(k.typeCode) ?? 0).toLocaleString("vi-VN") : "—";
        return (
          <div
            key={k.key}
            data-kpi={k.key}
            title={reason ?? undefined}
            className="min-w-0 rounded-[12px] border border-line bg-white px-5 py-4 shadow-card"
          >
            <div data-kpi-value="" className="text-[40px] leading-none font-bold tabular-nums text-navy">
              {value}
            </div>
            <div data-kpi-label="" className="mt-1.5 text-[15px] text-ink-muted">
              {k.label}
            </div>
            {reason !== null && <span className="an-thi-giac">{reason}</span>}
          </div>
        );
      })}
    </div>
  );
  const detailNode =
    detail === null ? null : (
      <DetailPanel
        detail={detail}
        groupLabel={groupLabel}
        unitName={unitName}
        fields={detailFields}
        canUpdate={canUpdate}
        busy={busy}
        actionError={actionError}
        onClose={closeDetail}
        onEdit={() => detail !== "loading" && detail.ok && setDialog({ kind: "asset", mode: { kind: "edit", asset: detail.duLieu } })}
        onToggleVerified={() => detail !== "loading" && detail.ok && void toggleVerified(detail.duLieu)}
        onDelete={() =>
          detail !== "loading" && detail.ok && setDialog({ kind: "delete", asset: { id: detail.duLieu.id, name: detail.duLieu.name } })
        }
      />
    );

  function mapArea() {
    if (frameRes === null) {
      return (
        <div className="p-4">
          <p role="status" className="an-thi-giac">
            Đang tải khung bản đồ…
          </p>
          <SkeletonRows rows={4} className="rounded-xl border border-line" />
        </div>
      );
    }
    if (!frameRes.ok) {
      return (
        <div className="p-4">
          <ErrorState role="alert" title={FRAME_LOAD_FAILED} message={frameRes.thongBao} onRetry={() => setFrameReload((n) => n + 1)} />
        </div>
      );
    }
    if (frame === null) {
      return (
        <div className="flex min-h-0 flex-col gap-4 overflow-y-auto p-4" data-frame-unset="">
          <p className="m-0 text-[15px] font-semibold text-ink-900">{FRAME_NOT_SET}</p>
          {canAdminLookup && hints !== null ? (
            <FrameForm
              current={null}
              defaultFrame={defaultFrame}
              hints={hints}
              suggestedCentre={meanCentre(collection.features)}
              titleId="frame-form-inline"
              onSaved={(out) => {
                adoptFrame(out);
                toast.success(FRAME_SAVED);
              }}
              onReloaded={adoptFrame}
            />
          ) : (
            <p className="m-0 text-[13px] text-ink-500">{FRAME_ASK_ADMIN}</p>
          )}
        </div>
      );
    }
    // The frame's own controls (ADR 0072; the prototype has none — spec 08 B keeps them): floating on the
    // map's top-left corner, with the icon-only "Mở rộng bản đồ". Without a map (no basemap, map failed)
    // the same buttons sit in a plain row above the message. None of them in "Trình chiếu".
    const frameButtons = (floating: boolean) =>
      presenting ? null : (
        <div className={floating ? "absolute top-3 left-3 z-10 flex gap-2" : "flex flex-wrap gap-2 px-4 pt-4"}>
          {canAdminLookup && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className={FLOATING_BUTTON}
              icon={<MapPinned aria-hidden="true" focusable="false" className="size-3.5" />}
              onClick={() => setDialog({ kind: "frame" })}
            >
              {FRAME_CHANGE_BUTTON}
            </Button>
          )}
          {canAdminLookup && frame.source === "commune" && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className={FLOATING_BUTTON}
              icon={<RotateCcw aria-hidden="true" focusable="false" className="size-3.5" />}
              onClick={() => setDialog({ kind: "reset" })}
            >
              {FRAME_RESET_BUTTON}
            </Button>
          )}
          {floating && (
            <MapExpandToggle floating expanded={mapExpanded} onToggle={() => setMapExpanded((v) => !v)} controls={MAP_REGION_ID} />
          )}
        </div>
      );
    if (!basemapAvailable || mapFailed) {
      return (
        <>
          {canAdminLookup && frameButtons(false)}
          <div className="p-4">
            {!basemapAvailable ? (
              <Notice tone="neutral" title={BASEMAP_MISSING_SENTENCE}>
                {NO_BASEMAP_DETAIL}
              </Notice>
            ) : (
              <ErrorState role="alert" title={MAP_LOAD_FAILED} onRetry={() => setMapFailed(false)} />
            )}
          </div>
        </>
      );
    }
    return (
      <div className="relative flex min-h-0 min-w-0 flex-1 flex-col">
        {frameButtons(true)}
        <EconomicMap
          key={frame.bounds.join(",")}
          frame={frame}
          centreLabel={centreLabel}
          expanded={mapExpanded}
          presentation={presenting}
          collection={collection}
          visible={visible}
          flyTo={flyTo}
          heatPoints={heatPoints}
          onPointClick={(id) => void openAsset(id, false)}
          onLoadError={() => setMapFailed(true)}
        />
      </div>
    );
  }

  function registerArea() {
    if (register === null) {
      return (
        <div className="p-4">
          <p role="status" className="an-thi-giac">
            Đang tải sổ địa điểm…
          </p>
          <SkeletonRows rows={5} className="rounded-xl border border-line" />
        </div>
      );
    }
    if (!register.ok) {
      return (
        <div className="p-4">
          <ErrorState role="alert" title="Chưa tải được sổ địa điểm" message={register.thongBao} />
        </div>
      );
    }
    return (
      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-auto p-4">
        <RegisterTable
          rows={register.duLieu.rows}
          hasMore={register.duLieu.hasMore}
          groupLabel={groupLabel}
          unitName={unitName}
          canUpdate={canUpdate}
          canLocate={mapAvailable}
          onLocate={(row) => {
            setView("map");
            void openAsset(row.id, true);
          }}
          onEdit={(row) => void editAsset(row.id)}
          onDelete={(row) => setDialog({ kind: "delete", asset: { id: row.id, name: row.name } })}
        />
        {register.duLieu.hasMore && (
          <p className="m-0">
            <Button type="button" variant="secondary" onClick={() => void loadMore()}>
              Tải thêm
            </Button>
          </p>
        )}
      </div>
    );
  }

  return (
    // Prototype `MapWorkspace.tsx` / spec 03: FULL-BLEED — no page padding (removed on this route only,
    // `app/ban-do/page.tsx`), no card; from lg the screen is exactly the window under the 4rem topbar, so
    // the left column scrolls inside and the map runs to the right and bottom edges. Stacked below lg.
    // "Trình chiếu" turns THIS element into the overlay (same element, so the map is never rebuilt).
    // `data-map-presentation`, never `data-presentation`: that one switches on Tổng quan's dark tokens.
    <div
      ref={rootRef}
      className={
        presenting
          ? // Above the topbar (30) and the expanded map (40), below tooltips and popovers (60).
            "economic-map-screen fixed inset-0 z-50 flex min-w-0 flex-col bg-canvas outline-none"
          : "economic-map-screen flex min-w-0 flex-col lg:h-[calc(100vh-4rem)]"
      }
      data-map-presentation={presenting ? "" : undefined}
      tabIndex={presenting ? -1 : undefined}
    >
      {header}
      {presenting && kpiRow}

      {!presenting && detail === null && actionError !== "" && (
        <p className="thong-bao-loi m-0 px-5 pt-3" role="alert">
          {actionError}
        </p>
      )}

      {!presenting && types !== null && !types.ok && (
        <div className="p-4">
          <ErrorState role="alert" title="Chưa tải được nhóm tài nguyên" message={types.thongBao} />
        </div>
      )}
      {!presenting && types !== null && types.ok && typeItems.length === 0 && (
        <div className="p-4">
          <EmptyState
            icon={Layers}
            title={NO_GROUPS}
            description={canAdminLookup ? undefined : NO_GROUPS_ASK_ADMIN}
            action={
              canAdminLookup ? (
                <Button type="button" variant="primary" disabled={busy} onClick={() => void seedDefaults()}>
                  {SEED_DEFAULTS_BUTTON}
                </Button>
              ) : undefined
            }
          />
        </div>
      )}

      {/* ONE workspace — left column (layers · filters · density, divided by rules), the map or the
          register in the middle, the detail panel on the right. */}
      <div className="economic-workspace flex min-h-0 min-w-0 flex-1 flex-col bg-white lg:flex-row">
        {!presenting && (
          <aside
            className="flex w-full min-w-0 flex-col gap-4 border-b border-line bg-white p-4 lg:w-72 lg:shrink-0 lg:overflow-y-auto lg:border-r lg:border-b-0"
            aria-label="Lớp và bộ lọc"
          >
            <LayerPanel
              types={typeItems}
              counts={counts}
              hidden={hidden}
              onToggle={(code) =>
                setHidden((h) => {
                  const next = new Set(h);
                  if (next.has(code)) next.delete(code);
                  else next.add(code);
                  return next;
                })
              }
              onOnly={(code) => setHidden(new Set(typeItems.map((t) => t.code).filter((c) => c !== code)))}
              onHideAll={() => setHidden(new Set(typeItems.map((t) => t.code)))}
              onShowAll={() => setHidden(new Set())}
              centreIsDefault={frame?.source === "default"}
              heat={heat}
            />
            <div className="border-t border-line pt-4">
              <FilterPanel
                value={{ search, industryCode: filter.industryCode, residentialUnitId: filter.residentialUnitId, status: filter.status }}
                units={units}
                results={results}
                onSearchChange={setSearch}
                onChange={(patch) => setFilter((f) => ({ ...f, ...patch }))}
                onClear={() => {
                  setSearch("");
                  setFilter(NO_FILTER);
                }}
                onChooseResult={(row) => {
                  setView("map");
                  void openAsset(row.id, true);
                }}
              />
            </div>
            <div className="border-t border-line pt-4">
              <DensityPending />
            </div>
          </aside>
        )}

        <div
          id={MAP_REGION_ID}
          className={
            mapExpanded
              ? // Above the sticky topbar (z 30), below menus/tooltips (z 60). The detail panel stays INSIDE,
                // so a click on a point still opens it next to the map.
                "fixed inset-0 z-40 flex min-w-0 flex-col bg-surface xl:flex-row"
              : "flex min-h-0 min-w-0 flex-1 flex-col xl:flex-row"
          }
          data-map-expanded={mapExpanded ? "" : undefined}
        >
          <div className="flex min-h-0 min-w-0 flex-1 flex-col">
            {points !== null && !points.ok && (
              <div className="p-3 pb-0">
                <Notice tone="neutral" title="Không thể tải dữ liệu kinh tế.">
                  {points.message}
                </Notice>
              </div>
            )}
            {mapShown ? mapArea() : registerArea()}
          </div>
          {!presenting && detailNode}
        </div>
      </div>

      {dialog !== null && dialog.kind === "asset" && (
        <AssetDialog
          mode={dialog.mode}
          types={typeItems}
          units={units}
          frame={frame}
          mapAvailable={mapAvailable}
          onCancel={() => setDialog(null)}
          onSaved={(saved, created) => {
            setDialog(null);
            toast.success(created ? `Đã thêm ${saved.name} lên bản đồ.` : `Đã lưu ${saved.name}.`);
            setReload((n) => n + 1);
            detailSeq.current += 1;
            setSelectedId(saved.id);
            setDetail({ ok: true, duLieu: saved });
          }}
        />
      )}
      {dialog !== null && dialog.kind === "delete" && (
        <DeleteAssetDialog
          asset={dialog.asset}
          onCancel={() => setDialog(null)}
          onDeleted={() => {
            const name = dialog.asset.name;
            if (selectedId === dialog.asset.id) closeDetail();
            setDialog(null);
            toast.success(`Đã xoá ${name} khỏi bản đồ.`);
            setReload((n) => n + 1);
          }}
        />
      )}
      {dialog !== null && dialog.kind === "frame" && hints !== null && (
        <OverlayDialog titleId="frame-form-dialog" onDismiss={() => setDialog(null)}>
          <FrameForm
            current={frame}
            defaultFrame={defaultFrame}
            hints={hints}
            suggestedCentre={meanCentre(collection.features)}
            titleId="frame-form-dialog"
            onCancel={() => setDialog(null)}
            onSaved={(out) => {
              setDialog(null);
              adoptFrame(out);
              toast.success(FRAME_SAVED);
            }}
            onReloaded={adoptFrame}
          />
        </OverlayDialog>
      )}
      {dialog !== null && dialog.kind === "reset" && hints !== null && (
        <ResetFrameDialog
          defaultFrame={defaultFrame}
          hints={hints}
          onCancel={() => setDialog(null)}
          onReloaded={adoptFrame}
          onDone={(out) => {
            setDialog(null);
            adoptFrame(out);
            toast.success(out.configured ? FRAME_RESET_DONE : FRAME_RESET_DONE_NO_DEFAULT);
          }}
        />
      )}
    </div>
  );
}
