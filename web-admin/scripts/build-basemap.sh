#!/usr/bin/env bash
# Builds and uploads the self-hosted Vietnam basemap of the PETITION maps (ADR 0072 §Sửa đổi 09/10/2026,
# §Trả lời 09/10/2026). An OPERATOR runs this, on a machine with Internet access and an `mc` alias to
# ViGov's MinIO — never CI, never a pod. The whole procedure, the mandatory sovereignty check and the
# signed checklist: kb/40-runbooks/basemap-pmtiles.md. Read it first.
#
#   build   web-admin/scripts/build-basemap.sh build  --date 20261008 --out /data/basemap
#   upload  web-admin/scripts/build-basemap.sh upload --date 20261008 --out /data/basemap \
#             --alias vigov --bucket vigov-prod-public
#
# Uploading is NOT going live: the files land under a NEW dated prefix that nothing reads until the
# ConfigMap key BASEMAP-URL is changed to point at it — and that change comes only after the signed check.
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

die() { echo "DỪNG: $*" >&2; exit 1; }

usage() {
  sed -n '2,12p' "$0" >&2
  exit 2
}

mode="${1:-}"; shift || true
date="" out="" alias="" bucket=""
while [ $# -gt 0 ]; do
  case "$1" in
    --date)   date="${2:-}"; shift 2 ;;
    --out)    out="${2:-}"; shift 2 ;;
    --alias)  alias="${2:-}"; shift 2 ;;
    --bucket) bucket="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done

[[ "$date" =~ ^20[0-9]{6}$ ]] || die "--date phải là YYYYMMDD của một bản dựng hằng ngày Protomaps"
[ -n "$out" ] || die "--out: thư mục làm việc (cần vài GB trống)"
dir="${out%/}/${date}"

build() {
  command -v pmtiles >/dev/null || die "thiếu go-pmtiles (https://github.com/protomaps/go-pmtiles/releases, v1.31.2)"
  command -v curl >/dev/null || die "thiếu curl"
  [ ! -e "$dir" ] || die "$dir đã có — mỗi lần dựng một thư mục mới, không ghi đè"

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
  echo "── 1/3 cắt vùng ${BBOX}, z${MINZOOM}–${MAXZOOM}, từ ${source} (lược đồ v${version})"
  pmtiles extract "$source" "$dir/$ARCHIVE" --bbox="$BBOX" --minzoom="$MINZOOM" --maxzoom="$MAXZOOM"
  pmtiles verify "$dir/$ARCHIVE"

  echo "── 2/3 glyph (${#FONTS[@]} bộ chữ × 256 khoảng) từ basemaps-assets@${ASSETS_COMMIT:0:7}"
  local f start enc
  for f in "${FONTS[@]}"; do
    mkdir -p "$dir/fonts/$f"
    enc="${f// /%20}"
    for ((start = 0; start <= 65280; start += 256)); do
      curl -fsSL --retry 3 -o "$dir/fonts/$f/${start}-$((start + 255)).pbf" \
        "${ASSETS_RAW}/fonts/${enc}/${start}-$((start + 255)).pbf"
    done
  done

  echo "── 3/3 sprite ${SPRITE}"
  local s
  for s in "${SPRITE}.json" "${SPRITE}.png" "${SPRITE}@2x.json" "${SPRITE}@2x.png"; do
    curl -fsSL --retry 3 -o "$dir/sprites/$s" "${ASSETS_RAW}/sprites/v4/$s"
  done

  {
    echo "source=${source}"
    echo "tile_schema=${version}"
    echo "bbox=${BBOX} minzoom=${MINZOOM} maxzoom=${MAXZOOM}"
    echo "assets_commit=${ASSETS_COMMIT}"
    echo "built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  } > "$dir/BUILD-INFO.txt"
  (cd "$dir" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS)

  echo
  echo "XONG: $dir"
  pmtiles show "$dir/$ARCHIVE" | head -30
  echo
  echo "Bước tiếp: upload, rồi KIỂM CHỦ QUYỀN có ký (kb/40-runbooks/basemap-pmtiles.md) TRƯỚC khi đổi BASEMAP-URL."
}

upload() {
  command -v mc >/dev/null || die "thiếu mc (MinIO client)"
  [ -n "$alias" ] && [ -n "$bucket" ] || die "--alias và --bucket là bắt buộc khi upload"
  [ -f "$dir/$ARCHIVE" ] && [ -f "$dir/SHA256SUMS" ] || die "$dir chưa dựng xong — chạy 'build' trước"
  (cd "$dir" && sha256sum --quiet -c SHA256SUMS) || die "tệp trong $dir không khớp SHA256SUMS"

  local dest="${alias}/${bucket}/basemap/${date}/"
  # Never overwrite: a prefix that is already live is being read in byte ranges right now, and mixing
  # ranges of two archives is a corrupted map. A refresh is always a new date.
  if [ -n "$(mc ls "$dest" 2>/dev/null | head -1)" ]; then
    die "$dest đã có tệp — không ghi đè. Dùng một ngày dựng khác."
  fi
  mc cp --recursive "$dir/" "$dest"
  echo
  echo "ĐÃ TẢI LÊN: $dest — CHƯA ai đọc tiền tố này."
  echo "Giá trị BASEMAP-URL tương ứng: https://<MinIO nội bộ>:<cổng>/${bucket}/basemap/${date}"
  echo "Bước tiếp: đặt giá trị ấy ở STAGING, kiểm chủ quyền + ký danh mục, rồi mới đặt ở prod."
}

case "$mode" in
  build) build ;;
  upload) upload ;;
  *) usage ;;
esac
