#!/usr/bin/env bash
# Builds and uploads the self-hosted Vietnam basemap of the PETITION maps (ADR 0072 §Sửa đổi 09/10/2026,
# §Trả lời 09/10/2026). Runs on a machine with Internet access and an `mc` alias to ViGov's MinIO: the
# Jenkins job `vigov-deploy`, task `nen-ban-do` (owner 11/10/2026), or an operator by hand
# — never a pod. The whole procedure, the mandatory sovereignty check and the signed checklist:
# kb/40-runbooks/basemap-pmtiles.md. Read it first.
#
#   build   DUCKDB_IMAGE=<ref@sha256> TIPPECANOE_IMAGE=<local image> \
#           web-admin/scripts/build-basemap.sh build  --date 20261008 --overture 2026-09-23.1 --out /data/basemap
#   upload  web-admin/scripts/build-basemap.sh upload --date 20261008 --out /data/basemap \
#             --alias vigov --bucket vigov-prod-public
#   places-sql  web-admin/scripts/build-basemap.sh places-sql --overture 2026-09-23.1   (prints the SQL only)
#
# Two archives per dated directory: vn-mainland.pmtiles (Protomaps) and vn-places.pmtiles (Overture
# places, owner 12/10/2026). The two images are prepared, pinned by digest/commit, by
# deploy/basemap/basemap-task.sh (`basemap.py images`, deploy/basemap/tippecanoe.Dockerfile); a hand run
# prepares them the same way and passes them in the two variables above.
#
# Uploading is NOT going live: the files land under a NEW dated prefix `config/map/<date>/` of the bucket
# (prefix chosen by the owner 11/10/2026; the web-admin route /basemap/* is unrelated) that nothing reads until the
# ConfigMap key BASEMAP_URL is changed to point at it — and that change comes only after the signed check.
#
# WHAT IS FIXED HERE AND WHY (changing any of it is a stop condition of ADR 0072, not a flag):
#   BBOX     mainland + near-shore islands, decided by the owner 09/10/2026.
#   MINZOOM  8: at zoom <= 7 a tile touching the bbox's east edge spans to ~112.5°E and would carry the
#            archipelagos with OSM's names. At zoom 8 the easternmost tile stops at ~111.09°E, short of
#            Hoàng Sa (~111.2°E) and Trường Sa (~111.5°E). The maps never zoom out that far anyway (frame
#            radius <= 50 km, ADR 0072 K2), so nothing visible is lost. Builder's choice, stated in the runbook.
#   MAXZOOM  15: the deepest zoom of the Protomaps daily build.
#   Assets   fonts + sprites from protomaps/basemaps-assets at a PINNED commit — a branch name would
#            change what a refresh uploads without anyone changing this file.
#   Places   (owner 12/10/2026) Overture places of the release given, confidence >= 0.5, inside Overture's
#            own VN country area of the SAME release AND inside BBOX (ADR 0072: nothing east of BBOX's edge,
#            whatever any polygon says), names with CJK characters dropped. One layer `places`, properties
#            `name` + optional `kind` and NOTHING else — Overture also carries phones, e-mails, websites and
#            addresses of businesses, none of which a petition map needs. z13–15: every point is kept at z15;
#            z13–14 may thin the densest tiles. The web-admin style is built against exactly this contract.
set -euo pipefail

readonly BBOX="101.6,7.9,110.0,23.9"
readonly MINZOOM=8
readonly MAXZOOM=15
readonly ARCHIVE="vn-mainland.pmtiles"
# Tile schema the style package targets: @protomaps/basemaps 5.x reads v4 tiles (web-admin/package.json).
readonly TILE_SCHEMA_MAJOR=4
readonly ASSETS_COMMIT="028c18f713baecad011301ff7a69acc39bcc2ae7"   # protomaps/basemaps-assets, 2025-10-31
readonly ASSETS_RAW="https://raw.githubusercontent.com/protomaps/basemaps-assets/${ASSETS_COMMIT}"
# Must equal BASEMAP_FONTS / BASEMAP_SPRITE in web-admin/src/lib/basemap/assets.ts.
readonly FONTS=("Noto Sans Regular" "Noto Sans Medium" "Noto Sans Italic")
readonly SPRITE="light"
readonly PLACES_ARCHIVE="vn-places.pmtiles"
readonly PLACES_LAYER="places"
readonly PLACES_MINZOOM=13
readonly PLACES_MAXZOOM=15
readonly PLACES_MIN_CONFIDENCE=0.5
# A whole-country cut under this many points means a broken filter or a half-published release, not a
# sparse country (Thăng Bình alone has ~540 at confidence >= 0.5, release 2026-09-23.1).
readonly PLACES_MIN_FEATURES=1000
# Anonymous, regional endpoint of Overture's public bucket: one host for the firewall, no AWS account.
readonly OVERTURE_BUCKET="overturemaps-us-west-2"
readonly OVERTURE_REGION="us-west-2"
readonly OVERTURE_HTTPS="https://${OVERTURE_BUCKET}.s3.${OVERTURE_REGION}.amazonaws.com"

die() { echo "DỪNG: $*" >&2; exit 1; }

usage() {
  sed -n '2,20p' "$0" >&2
  exit 2
}

mode="${1:-}"; shift || true
date="" out="" alias="" bucket="" overture=""
while [ $# -gt 0 ]; do
  case "$1" in
    --date)     date="${2:-}"; shift 2 ;;
    --overture) overture="${2:-}"; shift 2 ;;
    --out)      out="${2:-}"; shift 2 ;;
    --alias)    alias="${2:-}"; shift 2 ;;
    --bucket)   bucket="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done

# The release goes into SQL and an S3 path: this pattern is the whole of what may reach them.
check_overture() {
  [[ "$overture" =~ ^20[0-9]{2}-[0-9]{2}-[0-9]{2}\.[0-9]+$ ]] \
    || die "--overture phải là mã bản phát hành Overture dạng 2026-09-23.1 (${OVERTURE_HTTPS}/?list-type=2&prefix=release/&delimiter=/)"
}

# Prints the DuckDB script of the places cut. Paths are the container's (/work). BBOX is split from
# the one constant above — the sovereignty cut has a single source.
places_sql() {
  local x0 y0 x1 y1 src="s3://${OVERTURE_BUCKET}/release/${overture}"
  IFS=, read -r x0 y0 x1 y1 <<<"$BBOX"
  cat <<SQL
-- vn-places: Overture ${overture}, generated by web-admin/scripts/build-basemap.sh (owner 12/10/2026, ADR 0072).
SET extension_directory = '/work/duckdb-ext';
SET temp_directory = '/work/tmp';
-- Core extensions from extensions.duckdb.org; DuckDB refuses an unsigned one unless told otherwise,
-- and nothing here tells it otherwise.
INSTALL httpfs; LOAD httpfs; INSTALL spatial; LOAD spatial;
SET s3_region = '${OVERTURE_REGION}';
SET s3_endpoint = 's3.${OVERTURE_REGION}.amazonaws.com';
-- The cut reads ~100 parquet parts over a few minutes; one failed lookup or reset among them used to
-- end the whole run ("Could not resolve hostname", prod 14/10/2026 at part 14). DuckDB's default is 3
-- retries over ~2 s; this rides out ~4 minutes (1+2+4+…+128 s) before giving up on one request.
SET http_retries = 8;
SET http_retry_wait_ms = 1000;
SET http_retry_backoff = 2;

-- Vietnam as the SAME release draws it (land + maritime rows of the country).
CREATE TABLE vn AS
  SELECT ST_Union_Agg(geometry) AS g
  FROM read_parquet('${src}/theme=divisions/type=division_area/*', hive_partitioning = false)
  WHERE subtype = 'country' AND country = 'VN';
SELECT CASE WHEN g IS NULL THEN error('Overture ${overture}: không có vùng quốc gia VN') END FROM vn;

-- The country is one polygon of ~70k vertices: testing a million points against it costs ~1 ms each.
-- Cut into 0.25° cells, the spatial join tests each point against one small piece (218 s → 0.2 s for
-- Hà Nội's 203k points, same count, measured 12/10/2026).
CREATE TABLE vn_cells AS
  WITH cells AS (
    SELECT ST_MakeEnvelope(gx.x, gy.y, gx.x + 0.25, gy.y + 0.25) AS cell
    FROM (SELECT ${x0} + i * 0.25 AS x FROM range(0, CAST(ceil((${x1} - ${x0}) / 0.25) AS BIGINT)) r(i)) gx,
         (SELECT ${y0} + j * 0.25 AS y FROM range(0, CAST(ceil((${y1} - ${y0}) / 0.25) AS BIGINT)) r(j)) gy)
  SELECT ST_Intersection(cells.cell, vn.g) AS g FROM cells, vn WHERE ST_Intersects(cells.cell, vn.g);

CREATE TABLE places AS
  SELECT p.names.primary AS name, NULLIF(p.basic_category, '') AS kind,
         ST_X(p.geometry) AS lon, ST_Y(p.geometry) AS lat
  FROM read_parquet('${src}/theme=places/type=place/*', hive_partitioning = false) AS p
  WHERE p.bbox.xmin >= ${x0} AND p.bbox.xmax <= ${x1} AND p.bbox.ymin >= ${y0} AND p.bbox.ymax <= ${y1}
    AND p.confidence >= ${PLACES_MIN_CONFIDENCE}
    AND ST_GeometryType(p.geometry) = 'POINT'
    AND ST_X(p.geometry) BETWEEN ${x0} AND ${x1} AND ST_Y(p.geometry) BETWEEN ${y0} AND ${y1}
    AND trim(coalesce(p.names.primary, '')) <> ''
    AND NOT regexp_matches(p.names.primary, '[\x{3400}-\x{4DBF}\x{4E00}-\x{9FFF}\x{F900}-\x{FAFF}]')
    AND EXISTS (SELECT 1 FROM vn_cells c WHERE ST_Intersects(c.g, p.geometry));

SELECT CASE WHEN count(*) < ${PLACES_MIN_FEATURES}
            THEN error('chỉ ' || count(*) || ' điểm sau lọc — bộ lọc hoặc bản phát hành hỏng') END FROM places;
SELECT CASE WHEN max(lon) > ${x1} OR min(lon) < ${x0} OR max(lat) > ${y1} OR min(lat) < ${y0}
            THEN error('có điểm ngoài BBOX ${BBOX}') END FROM places;

-- One GeoJSON Feature per line; JSON-typed columns are written as JSON, so each row IS a Feature.
-- kind is left out, not null, when Overture has none.
COPY (
  SELECT 'Feature' AS type,
         json_object('type', 'Point', 'coordinates', json_array(round(lon, 7), round(lat, 7))) AS geometry,
         CASE WHEN kind IS NULL THEN json_object('name', name)
              ELSE json_object('name', name, 'kind', kind) END AS properties
  FROM places ORDER BY lon, lat, name
) TO '/work/places.geojsonl' (FORMAT json);
-- Written last: its presence means every statement above ran.
COPY (SELECT count(*) FROM places) TO '/work/places.count' (FORMAT csv, HEADER false);
SQL
}

if [ "$mode" = "places-sql" ]; then
  check_overture
  places_sql
  exit 0
fi

[[ "$date" =~ ^20[0-9]{6}$ ]] || die "--date phải là YYYYMMDD của một bản dựng hằng ngày Protomaps"
[ -n "$out" ] || die "--out: thư mục làm việc (cần vài GB trống)"
dir="${out%/}/${date}"
# Intermediate files of the places cut: OUTSIDE $dir, which is uploaded and checksummed whole.
work="${out%/}/${date}.places-work"

# PMTiles v3 header: byte 99 tile type (1 = mvt), 100/101 min/max zoom, 102..117 bounds as int32 e7
# little-endian (x86 `od` reads native order; CentOS 7's od has no --endian).
check_places_header() {
  local f="$1" b z bounds
  b=($(od -An -tu1 -j99 -N3 "$f"))
  [ "${b[0]:-}" = 1 ] || die "$f: kiểu tile ${b[0]:-?}, cần 1 (vector)"
  [ "${b[1]:-}" = "$PLACES_MINZOOM" ] && [ "${b[2]:-}" = "$PLACES_MAXZOOM" ] \
    || die "$f: zoom ${b[1]:-?}–${b[2]:-?}, cần ${PLACES_MINZOOM}–${PLACES_MAXZOOM}"
  bounds=($(od -An -td4 -j102 -N16 "$f"))
  local x1e7
  x1e7=$(awk -v b="$BBOX" 'BEGIN { split(b, a, ","); printf "%d", a[3] * 10000000 }')
  [ "${bounds[2]:-9999999999}" -le "$x1e7" ] \
    || die "$f: biên đông ${bounds[2]:-?} (e7) vượt ${x1e7} — vượt BBOX, KHÔNG tải lên (ADR 0072)"
}

build_places() {
  local uid gid n
  uid="$(id -u)"; gid="$(id -g)"
  rm -rf "$work"; mkdir -p "$work/tmp"
  places_sql > "$work/places.sql"
  # The DuckDB container needs the network (S3 + signed extensions); tippecanoe never does.
  # WHOLE-CUT RETRY on top of DuckDB's per-request one (places_sql): an outage longer than those ~4
  # minutes, or an error DuckDB does not retry, reruns the cut from the start — the outputs are
  # removed first, so a half-written file of a failed attempt is never read as a result.
  local attempt
  for attempt in 1 2 3; do
    rm -f "$work/places.count" "$work/places.geojsonl"
    if docker run --rm -i --user "$uid:$gid" -e HOME=/work -v "$work:/work:z" "$DUCKDB_IMAGE" \
        /duckdb -bail < "$work/places.sql"; then
      break
    fi
    [ "$attempt" -lt 3 ] || die "DuckDB lỗi cả 3 lần (thường là mạng tới ${OVERTURE_HTTPS}) — xem lỗi ở trên"
    echo "DuckDB lỗi ở lần ${attempt}/3 — chờ 60 giây rồi chạy lại bước cắt địa điểm"
    sleep 60
  done
  [ -s "$work/places.count" ] && [ -s "$work/places.geojsonl" ] || die "DuckDB dừng giữa chừng — xem lỗi ở trên"
  n="$(tr -dc '0-9' < "$work/places.count")"
  [ "${n:-0}" -ge "$PLACES_MIN_FEATURES" ] || die "chỉ ${n:-0} điểm — dưới ngưỡng ${PLACES_MIN_FEATURES}"

  local tip=(docker run --rm --network none --user "$uid:$gid" -v "$work:/work:z" "$TIPPECANOE_IMAGE")
  # Two passes, then joined: z15 with every limit off is the only way tippecanoe GUARANTEES no point
  # is dropped there; z13–14 may thin the densest tiles to stay under the tile size limit.
  "${tip[@]}" tippecanoe -q -P -o /work/z15.pmtiles -l "$PLACES_LAYER" -Z"$PLACES_MAXZOOM" -z"$PLACES_MAXZOOM" \
    -r1 --no-feature-limit --no-tile-size-limit -y name -y kind -t /work/tmp /work/places.geojsonl
  "${tip[@]}" tippecanoe -q -P -o /work/z13.pmtiles -l "$PLACES_LAYER" -Z"$PLACES_MINZOOM" -z"$((PLACES_MAXZOOM - 1))" \
    -r1 --drop-densest-as-needed -y name -y kind -t /work/tmp /work/places.geojsonl
  "${tip[@]}" tile-join -q -o "/work/$PLACES_ARCHIVE" --no-tile-size-limit -n vn-places \
    -N "Overture places ${overture}, VN" -A "© Overture Maps Foundation" /work/z13.pmtiles /work/z15.pmtiles

  pmtiles verify "$work/$PLACES_ARCHIVE"
  check_places_header "$work/$PLACES_ARCHIVE"
  mv "$work/$PLACES_ARCHIVE" "$dir/$PLACES_ARCHIVE"
  PLACES_FEATURES="$n"
  rm -rf "$work"
}

build() {
  command -v pmtiles >/dev/null || die "thiếu go-pmtiles (https://github.com/protomaps/go-pmtiles/releases, v1.31.2)"
  command -v curl >/dev/null || die "thiếu curl"
  command -v docker >/dev/null || die "thiếu docker (DuckDB + tippecanoe chạy trong ảnh ghim digest)"
  check_overture
  [[ "${DUCKDB_IMAGE:-}" =~ ^[a-z0-9./-]+@sha256:[0-9a-f]{64}$ ]] \
    || die "DUCKDB_IMAGE phải là <ảnh>@sha256:<64 hex> — lấy bằng 'basemap.py images' (deploy/basemap/basemap-task.sh)"
  [ -n "${TIPPECANOE_IMAGE:-}" ] || die "TIPPECANOE_IMAGE: ảnh dựng từ deploy/basemap/tippecanoe.Dockerfile (basemap-task.sh)"
  [ ! -e "$dir" ] || die "$dir đã có — mỗi lần dựng một thư mục mới, không ghi đè"

  # Before an hour of downloading: Overture keeps only its last few releases on the bucket.
  curl -fsSL --retry 3 "${OVERTURE_HTTPS}/?list-type=2&prefix=release/${overture}/theme=places/type=place/&max-keys=1" \
    | grep -q '<Key>' || die "không thấy bản Overture ${overture} trên ${OVERTURE_BUCKET} — Overture chỉ giữ vài bản gần nhất"

  local source="https://build.protomaps.com/${date}.pmtiles"
  # The build list says which tile schema each daily build has; a major the style does not know
  # draws a half-empty map with no error.
  local version
  version="$(curl -fsSL https://build-metadata.protomaps.dev/builds.json \
    | grep -o "\"key\":\"${date}.pmtiles\"[^}]*" | grep -o '"version":"[^"]*"' | cut -d'"' -f4 || true)"
  [ -n "$version" ] || die "không thấy bản dựng ${date} trong danh sách — Protomaps chỉ giữ các ngày gần đây"
  [ "${version%%.*}" = "$TILE_SCHEMA_MAJOR" ] \
    || die "bản dựng ${date} có lược đồ tile v${version}; style của web-admin đọc v${TILE_SCHEMA_MAJOR}.x"

  mkdir -p "$dir/fonts" "$dir/sprites"
  echo "── 1/4 cắt vùng ${BBOX}, z${MINZOOM}–${MAXZOOM}, từ ${source} (lược đồ v${version})"
  pmtiles extract "$source" "$dir/$ARCHIVE" --bbox="$BBOX" --minzoom="$MINZOOM" --maxzoom="$MAXZOOM"
  pmtiles verify "$dir/$ARCHIVE"

  echo "── 2/4 glyph (${#FONTS[@]} bộ chữ × 256 khoảng) từ basemaps-assets@${ASSETS_COMMIT:0:7}"
  local f start enc
  for f in "${FONTS[@]}"; do
    mkdir -p "$dir/fonts/$f"
    enc="${f// /%20}"
    for ((start = 0; start <= 65280; start += 256)); do
      curl -fsSL --retry 3 -o "$dir/fonts/$f/${start}-$((start + 255)).pbf" \
        "${ASSETS_RAW}/fonts/${enc}/${start}-$((start + 255)).pbf"
    done
  done

  echo "── 3/4 sprite ${SPRITE}"
  local s
  for s in "${SPRITE}.json" "${SPRITE}.png" "${SPRITE}@2x.json" "${SPRITE}@2x.png"; do
    curl -fsSL --retry 3 -o "$dir/sprites/$s" "${ASSETS_RAW}/sprites/v4/$s"
  done

  echo "── 4/4 địa điểm Overture ${overture} → ${PLACES_ARCHIVE} (z${PLACES_MINZOOM}–${PLACES_MAXZOOM})"
  build_places

  {
    echo "source=${source}"
    echo "tile_schema=${version}"
    echo "bbox=${BBOX} minzoom=${MINZOOM} maxzoom=${MAXZOOM}"
    echo "assets_commit=${ASSETS_COMMIT}"
    echo "places_source=overture ${overture}"
    echo "places_filter=confidence>=${PLACES_MIN_CONFIDENCE} country=VN cjk=dropped"
    echo "places_zoom=${PLACES_MINZOOM}-${PLACES_MAXZOOM}"
    echo "places_features=${PLACES_FEATURES}"
    echo "places_tools=${DUCKDB_IMAGE} ${TIPPECANOE_IMAGE}"
    echo "built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  } > "$dir/BUILD-INFO.txt"
  (cd "$dir" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS)

  echo
  echo "XONG: $dir"
  # `|| true`: under pipefail, `head` closing the pipe early (SIGPIPE) must not fail a finished build.
  pmtiles show "$dir/$ARCHIVE" | head -30 || true
  echo
  pmtiles show "$dir/$PLACES_ARCHIVE" | head -30 || true
  echo
  echo "Bước tiếp: upload, rồi KIỂM CHỦ QUYỀN có ký (kb/40-runbooks/basemap-pmtiles.md) TRƯỚC khi đổi BASEMAP_URL."
}

upload() {
  command -v mc >/dev/null || die "thiếu mc (MinIO client)"
  [ -n "$alias" ] && [ -n "$bucket" ] || die "--alias và --bucket là bắt buộc khi upload"
  # Both archives or nothing: the style reads both from the same prefix.
  [ -f "$dir/$ARCHIVE" ] && [ -f "$dir/$PLACES_ARCHIVE" ] && [ -f "$dir/SHA256SUMS" ] \
    || die "$dir chưa dựng xong (cần $ARCHIVE + $PLACES_ARCHIVE + SHA256SUMS) — chạy 'build' trước"
  grep -q " ./${ARCHIVE}\$" "$dir/SHA256SUMS" && grep -q " ./${PLACES_ARCHIVE}\$" "$dir/SHA256SUMS" \
    || die "SHA256SUMS trong $dir không phủ cả hai tệp .pmtiles — dựng lại"
  (cd "$dir" && sha256sum --quiet -c SHA256SUMS) || die "tệp trong $dir không khớp SHA256SUMS"

  local dest="${alias}/${bucket}/config/map/${date}/"
  # Same date again overwrites (owner, 12/10/2026). If the prefix is live, maps open during the copy can
  # read byte ranges of two different archives until reloaded, and the signed checklist of this prefix
  # no longer matches — basemap-task.sh says which case it is; here it is said in one line.
  if [ -n "$(mc ls "$dest" 2>/dev/null | head -1)" ]; then
    echo "⚠ GHI ĐÈ: $dest đã có tệp — thay bằng bản dựng này. Tiền tố đang dùng thì kiểm và ký lại sau đó."
  fi
  mc cp --recursive "$dir/" "$dest"
  echo
  echo "ĐÃ TẢI LÊN: $dest — job không đổi BASEMAP_URL."
  echo "Giá trị BASEMAP_URL tương ứng: https://<MinIO nội bộ>:<cổng>/${bucket}/config/map/${date}"
  echo "Bước tiếp: đặt giá trị ấy ở STAGING, kiểm chủ quyền + ký danh mục, rồi mới đặt ở prod."
}

case "$mode" in
  build) build ;;
  upload) upload ;;
  *) usage ;;
esac
