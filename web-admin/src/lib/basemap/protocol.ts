import type * as MapLibre from "maplibre-gl";

/**
 * Teaches MapLibre the `pmtiles://` scheme the petition-map style uses (`./style.ts`). Call it — and
 * await it — BEFORE `new ml.Map(...)` with that style; a map created first fails its source silently.
 *
 * `pmtiles` is imported dynamically, like `maplibre-gl` in `features/map-assets`: neither belongs in the
 * bundle of a page that draws no map. The archive is read with `fetch` + `Range` against the same-origin
 * `/basemap/*` route, so the session cookie goes with it (`proxy.ts` gate) and nothing leaves the origin.
 *
 * Once per page: MapLibre keeps one handler per scheme, and one `Protocol` instance caches each
 * archive's header and directories — two instances would read them twice.
 */
let registered: Promise<void> | null = null;

export function registerPmtilesProtocol(ml: Pick<typeof MapLibre, "addProtocol">): Promise<void> {
  registered ??= import("pmtiles").then(
    ({ Protocol }) => {
      const protocol = new Protocol();
      ml.addProtocol("pmtiles", protocol.tile as Parameters<typeof ml.addProtocol>[1]);
    },
    (err: unknown) => {
      // A failed chunk load (network blip) must not be cached forever: the next map retries.
      registered = null;
      throw err;
    },
  );
  return registered;
}
