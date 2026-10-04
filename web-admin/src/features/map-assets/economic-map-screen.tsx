"use client";

import { Download, Expand, Layers, List, Map as MapIcon, MapPinned, Plus, RotateCcw, Upload } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useCauHinhXa } from "@/components/cau-hinh-xa"; // vi-name-ok: existing export (rule 12 inv 3)
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { PageHeader } from "@/components/ui/page-header";
import { PendingButton } from "@/components/ui/pending-feature";
import { Segmented } from "@/components/ui/segmented";
import { SkeletonRows } from "@/components/ui/skeleton";
import { OverlayDialog } from "@/features/noi-dung/overlay-dialog";
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
import { coQuyen, QUYEN_QUAN_LY_DANH_MUC } from "@/lib/quyen";

import { AssetDialog, type AssetDialogMode } from "./asset-dialog";
import { DeleteAssetDialog, DetailPanel } from "./detail-panel";
import { EconomicMap, type FlyRequest } from "./economic-map";
import { MapExpandToggle } from "./map-expand";
import { FrameForm, ResetFrameDialog } from "./frame-form";
import {
  ADD_BUTTON,
  ASSET_UPDATE_PERMISSION,
  FRAME_ASK_ADMIN,
  FRAME_CHANGE_BUTTON,
  FRAME_LOAD_FAILED,
  FRAME_NOT_SET,
  FRAME_RESET_BUTTON,
  FRAME_RESET_DONE,
  FRAME_RESET_DONE_NO_DEFAULT,
  FRAME_SAVED,
  MAP_LOAD_FAILED,
  NO_BASEMAP,
  NO_BASEMAP_DETAIL,
  NO_GROUPS,
  NO_GROUPS_ASK_ADMIN,
  OUTSIDE_FRAME,
  PAGE_SUBTITLE,
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
import { DensityPending, FilterPanel, LayerPanel } from "./side-panel";

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
 *   2. a basemap style URL is configured (server-side `MAP_STYLE_URL`, passed in as `styleUrl`) — H1;
 *   3. the style loaded.
 * The register ("Sổ địa điểm") works in every one of those cases.
 *
 * PERMISSIONS ARE CONVENIENCE HERE (rule 5, #1): `asset.read` gates the whole screen (`page.tsx`),
 * `asset.update` shows the write controls, `admin.lookup` the frame form, "Về mặc định" and the
 * default-groups button. The server checks each on every request.
 */
/** The region "Mở rộng bản đồ" grows into the page overlay (`aria-controls` of the toggle). */
const MAP_REGION_ID = "economic-map-region";

export function EconomicMapScreen({ styleUrl }: { styleUrl: string | null }) {
  const phien = usePhien();
  const permissions = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const canUpdate = coQuyen(permissions, ASSET_UPDATE_PERMISSION);
  const canAdminLookup = coQuyen(permissions, QUYEN_QUAN_LY_DANH_MUC);

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
  const [notice, setNotice] = useState("");
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState(false);
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const [mapFailed, setMapFailed] = useState(false);
  const [chosenGroup, setChosenGroup] = useState("");
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
  const mapAvailable = frame !== null && styleUrl !== null && !mapFailed;
  // Expanded only while a map is actually on screen: switching to Sổ địa điểm, or losing the map,
  // returns to the normal layout by itself — derived, so it cannot get stuck open.
  const mapExpanded = mapExpandedState && view === "map" && mapAvailable;

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
      setNotice("");
      const r = await getMapAsset(id);
      if (seq !== detailSeq.current) return;
      setDetail(r);
      if (!r.ok) return;
      listMapFields(r.duLieu.asset_type_code).then((f) => seq === detailSeq.current && setDetailFields(f.ok ? f.duLieu.items : []));
      if (!fly) return;
      // Never move the camera outside the frame (security review 04/10/2026, #2).
      if (frame === null || !insideFrame(frame.bounds, r.duLieu.lng, r.duLieu.lat)) return setNotice(OUTSIDE_FRAME);
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
    setNotice(r.duLieu.verified ? `Đã xác minh ${r.duLieu.name}.` : `Đã bỏ xác minh ${r.duLieu.name}.`);
    setReload((n) => n + 1);
  }

  async function seedDefaults() {
    setBusy(true);
    setActionError("");
    const r = await seedMapAssetTypeDefaults();
    setBusy(false);
    if (!r.ok) return setActionError(r.thongBao);
    setNotice(`Đã nạp ${r.duLieu.created_count} nhóm tài nguyên mặc định.`);
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

  const headerActions = (
    <>
      {activeTypes.length > 0 && (
        <span className="inline-flex">
          <label htmlFor="header-group" className="an-thi-giac">
            Nhóm tài nguyên đang thao tác
          </label>
          <select
            id="header-group"
            className="h-10 rounded-control border border-line-strong bg-surface px-3 text-sm"
            value={currentGroup}
            onChange={(e) => setChosenGroup(e.target.value)}
          >
            {activeTypes.map((t) => (
              <option key={t.id} value={t.code}>
                {t.label}
              </option>
            ))}
          </select>
        </span>
      )}
      <PendingButton info={pendingPart("Mẫu Excel")} side="bottom" icon={<Download aria-hidden="true" />} />
      <PendingButton info={pendingPart("Nhập Excel")} side="bottom" icon={<Upload aria-hidden="true" />} />
      {canUpdate && activeTypes.length > 0 && (
        <Button
          type="button"
          variant="primary"
          icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
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
    </>
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
        <>
          <p role="status" className="an-thi-giac">
            Đang tải khung bản đồ…
          </p>
          <SkeletonRows rows={4} className="rounded-xl border border-line" />
        </>
      );
    }
    if (!frameRes.ok) {
      return <ErrorState role="alert" title={FRAME_LOAD_FAILED} message={frameRes.thongBao} onRetry={() => setFrameReload((n) => n + 1)} />;
    }
    if (frame === null) {
      return (
        <div className="flex flex-col gap-4 rounded-xl border border-line bg-surface p-4" data-frame-unset="">
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
                setNotice(FRAME_SAVED);
              }}
              onReloaded={adoptFrame}
            />
          ) : (
            <p className="m-0 text-[13px] text-ink-500">{FRAME_ASK_ADMIN}</p>
          )}
        </div>
      );
    }
    if (styleUrl === null) {
      return (
        <Notice tone="neutral" title={NO_BASEMAP}>
          {NO_BASEMAP_DETAIL}
        </Notice>
      );
    }
    if (mapFailed) return <ErrorState role="alert" title={MAP_LOAD_FAILED} onRetry={() => setMapFailed(false)} />;
    return (
      <>
        <div className="flex justify-end">
          <MapExpandToggle expanded={mapExpanded} onToggle={() => setMapExpanded((v) => !v)} controls={MAP_REGION_ID} />
        </div>
        <EconomicMap
          key={frame.bounds.join(",")}
          styleUrl={styleUrl}
          frame={frame}
          centreLabel={centreLabel}
          expanded={mapExpanded}
          collection={collection}
          visible={visible}
          selectedId={selectedId}
          flyTo={flyTo}
          onPointClick={(id) => void openAsset(id, false)}
          onLoadError={() => setMapFailed(true)}
        />
      </>
    );
  }

  function registerArea() {
    if (register === null) {
      return (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải sổ địa điểm…
          </p>
          <SkeletonRows rows={5} className="rounded-xl border border-line" />
        </>
      );
    }
    if (!register.ok) return <ErrorState role="alert" title="Chưa tải được sổ địa điểm" message={register.thongBao} />;
    return (
      <div className="flex flex-col gap-3">
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
    <div className="economic-map-screen flex min-w-0 flex-col gap-4">
      <PageHeader
        icon={MapIcon}
        title={PAGE_TITLE}
        subtitle={
          summary !== null && summary.ok ? (
            <span data-summary-line="">{summaryLine(summary.duLieu.total, summary.duLieu.verified_ratio)}</span>
          ) : (
            PAGE_SUBTITLE
          )
        }
        actions={headerActions}
        className="mb-0"
      />

      <div className="flex flex-wrap items-center gap-2">
        <Segmented
          legend="Chế độ xem"
          name="map-view"
          mode="buttons"
          value={view}
          onChange={(v) => setView(v === "register" ? "register" : "map")}
          options={[
            { value: "map", label: VIEW_MAP, icon: MapIcon },
            { value: "register", label: VIEW_REGISTER, icon: List },
          ]}
        />
        <PendingButton info={pendingPart("Trình chiếu")} side="bottom" size="sm" icon={<Expand aria-hidden="true" />} />
        {canAdminLookup && frame !== null && (
          <Button type="button" variant="secondary" size="sm" icon={<MapPinned aria-hidden="true" />} onClick={() => setDialog({ kind: "frame" })}>
            {FRAME_CHANGE_BUTTON}
          </Button>
        )}
        {canAdminLookup && frame !== null && frame.source === "commune" && (
          <Button type="button" variant="secondary" size="sm" icon={<RotateCcw aria-hidden="true" />} onClick={() => setDialog({ kind: "reset" })}>
            {FRAME_RESET_BUTTON}
          </Button>
        )}
      </div>

      {notice !== "" && (
        <p role="status" className="m-0 text-sm font-medium text-ink-700" data-notice="">
          {notice}
        </p>
      )}
      {detail === null && actionError !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {actionError}
        </p>
      )}

      {types !== null && !types.ok && <ErrorState role="alert" title="Chưa tải được nhóm tài nguyên" message={types.thongBao} />}
      {types !== null && types.ok && typeItems.length === 0 && (
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
      )}

      <div className="flex min-w-0 flex-col gap-4 lg:flex-row">
        <aside className="flex w-full min-w-0 flex-col gap-5 lg:w-[280px] lg:shrink-0" aria-label="Lớp và bộ lọc">
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
            onHideAll={() => setHidden(new Set(typeItems.map((t) => t.code)))}
            onShowAll={() => setHidden(new Set())}
            centreIsDefault={frame?.source === "default"}
          />
          <FilterPanel
            value={{ search, industryCode: filter.industryCode, residentialUnitId: filter.residentialUnitId, status: filter.status }}
            units={units}
            results={results}
            onSearchChange={setSearch}
            onChange={(patch) => setFilter((f) => ({ ...f, ...patch }))}
            onChooseResult={(row) => {
              setView("map");
              void openAsset(row.id, true);
            }}
          />
          <DensityPending />
        </aside>

        <div
          id={MAP_REGION_ID}
          className={
            mapExpanded
              ? // Above the sticky topbar (z 30), below menus/tooltips (z 60). The detail panel stays INSIDE,
                // so a click on a point still opens it next to the map.
                "fixed inset-0 z-40 flex min-w-0 flex-col gap-3 bg-surface p-3 md:flex-row"
              : "flex min-w-0 flex-1 flex-col gap-3 md:flex-row"
          }
          data-map-expanded={mapExpanded ? "" : undefined}
        >
          <div className={mapExpanded ? "flex min-h-0 min-w-0 flex-1 flex-col gap-3" : "flex min-w-0 flex-1 flex-col gap-3"}>
            {points !== null && !points.ok && (
              <Notice tone="neutral" title="Không thể tải dữ liệu kinh tế.">
                {points.message}
              </Notice>
            )}
            {view === "map" ? mapArea() : registerArea()}
          </div>
          {detailNode}
        </div>
      </div>

      {dialog !== null && dialog.kind === "asset" && (
        <AssetDialog
          mode={dialog.mode}
          types={typeItems}
          units={units}
          frame={frame}
          styleUrl={mapAvailable ? styleUrl : null}
          onCancel={() => setDialog(null)}
          onSaved={(saved, created) => {
            setDialog(null);
            setNotice(created ? `Đã thêm ${saved.name} lên bản đồ.` : `Đã lưu ${saved.name}.`);
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
            setNotice(`Đã xoá ${name} khỏi bản đồ.`);
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
              setNotice(FRAME_SAVED);
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
            setNotice(out.configured ? FRAME_RESET_DONE : FRAME_RESET_DONE_NO_DEFAULT);
          }}
        />
      )}
    </div>
  );
}
