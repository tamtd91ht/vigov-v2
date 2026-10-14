#!/usr/bin/env bash
# Task `nen-ban-do` — build the self-hosted basemap on the BUILD MACHINE and upload it to the
# environment's MinIO. Called by `vigov-deploy` (deploy/Jenkinsfile), which only checks its own
# parameters (XAC_NHAN = namespace, TICKET on prod) and calls this file.
#
# Why and the full procedure: ADR 0072, kb/40-runbooks/basemap-pmtiles.md. Decisions with tests live in
# deploy/basemap/basemap.py (plan · tools · images · key-source · mc-host); the build itself
# is web-admin/scripts/build-basemap.sh.
#
# Environment (set by the calling job, never by a person):
#   NS          vigov-prod | vigov-dev
#   NGAY_NEN    YYYYMMDD of a Protomaps daily build (validated by basemap.py before any use)
#   BAN_OVERTURE  Overture release of the places archive, e.g. 2026-09-23.1 (validated by basemap.py)
#   GHI         1 = do it; anything else = dry run (print the plan, change nothing)
#   KUBECONFIG  the job's ViGov kubeconfig       WORKSPACE  Jenkins workspace (repo checkout = cwd)
#   PYTHON      optional — Python 3.8+; discovered here when unset
#   NGUOI_BAM · TICKET · OPS_SUMMARY   optional — who/ticket for the GHI line, summary file to append to
#
# Guarantees: tools pinned by SHA-256, Docker images by digest, tippecanoe by commit (refuses while
# UNPINNED) · MinIO keys = the EXISTING pair of
# comms-secrets (fallback platform-secrets), read with the job's kubeconfig into 0600 files → MC_HOST_*,
# never argv, never echoed · prefix <bucket>/config/map/<date>/, never overwritten · NEVER writes
# BASEMAP_URL · $WORKSPACE/basemap, .tools, .mc removed on every exit.

# Jenkins runs `sh -xe`: without this, trace would print the keys.
set +x
set -eu
umask 077

: "${NS:?}" "${NGAY_NEN?}" "${BAN_OVERTURE?}" "${WORKSPACE:?}" "${KUBECONFIG:?}"
case "$NS" in vigov-prod|vigov-dev) ;; *) echo "DỪNG: NS '$NS' không hợp lệ."; exit 1 ;; esac

TMP_DIR=$(mktemp -d)
cleanup() {
  rm -rf "$TMP_DIR" "$WORKSPACE/basemap" "$WORKSPACE/.tools" "$WORKSPACE/.mc"
}
trap cleanup EXIT

if [ -z "${PYTHON:-}" ]; then
  # CentOS 7: `python3` is 3.6 — same discovery loop as the other Jenkinsfiles.
  for py in python3.14 python3.13 python3.12 python3.11 python3.10 python3.9 python3.8 python3; do
    command -v "$py" >/dev/null 2>&1 || continue
    if "$py" -c 'import sys; sys.exit(0 if sys.version_info >= (3, 8) else 1)'; then PYTHON=$(command -v "$py"); break; fi
  done
  [ -n "${PYTHON:-}" ] || { echo "DỪNG: máy build không có Python 3.8+ trong PATH."; exit 1; }
fi
OPS() { "$PYTHON" deploy/basemap/basemap.py "$@"; }
note() {
  echo "GHI · ${NGUOI_BAM:-?} · $(date -u +%Y-%m-%dT%H:%M:%SZ) · $NS · ticket ${TICKET:--} · $*"
  [ -z "${OPS_SUMMARY:-}" ] || printf '%s · %s\n' "$(date -u +%H:%M:%SZ)" "$*" >> "$OPS_SUMMARY"
}

# ── Plan (read only) ──────────────────────────────────────────────────────────────────────────
kubectl -n "$NS" get configmap common-config -o json > "$TMP_DIR/cm.json" || {
  echo "DỪNG: không đọc được configmap/common-config trong $NS. Không làm gì."; exit 1; }
OPS plan --current "$TMP_DIR/cm.json" --date "$NGAY_NEN" --overture "$BAN_OVERTURE" --out "$TMP_DIR/bm.env"
BM_BUCKET=$(sed -n 's/^BUCKET=//p' "$TMP_DIR/bm.env")
BM_ENDPOINT=$(sed -n 's/^UPLOAD_ENDPOINT=//p' "$TMP_DIR/bm.env")
BM_URL=$(sed -n 's/^BASEMAP_URL=//p' "$TMP_DIR/bm.env")
BM_OVERTURE=$(sed -n 's/^OVERTURE=//p' "$TMP_DIR/bm.env")
OPS tools --out "$TMP_DIR/tools.tsv"
OPS images --out "$TMP_DIR/images.env"
for c in curl sha256sum tar df find xargs sort base64 cmp od awk docker; do
  command -v "$c" >/dev/null 2>&1 || { echo "DỪNG: máy build thiếu $c (build-basemap.sh cần). Không làm gì."; exit 1; }
done
docker info >/dev/null 2>&1 || { echo "DỪNG: user $(id -un) không gọi được docker daemon (DuckDB + tippecanoe chạy trong ảnh). Không làm gì."; exit 1; }
# Key NAMES only — values are read after the dry-run stop, straight into files.
: > "$TMP_DIR/keysrc.tsv"
for s in comms platform; do
  kubectl -n "$NS" get secret "$s-secrets" -o go-template='{{range $k, $v := .data}}{{$k}} {{end}}' 2>/dev/null \
    | tr ' ' '\n' | sed "s/^/$s-secrets\t/" >> "$TMP_DIR/keysrc.tsv" || true
done
KEY_SECRET=$(OPS key-source --present "$TMP_DIR/keysrc.tsv") || { echo "$KEY_SECRET"; exit 1; }
echo "Khoá MinIO: cặp OBJECT_STORAGE_ACCESS_KEY / OBJECT_STORAGE_SECRET_KEY của secret/$KEY_SECRET (không in giá trị)"
# Proposal, not a measured figure: the cut is a few GB; a full disk breaks every job on this machine.
free_kb=$(df -Pk "$WORKSPACE" | awk 'NR==2 {print $4}')
[ "${free_kb:-0}" -ge 8388608 ] || {
  echo "DỪNG: $WORKSPACE còn $((${free_kb:-0} / 1048576)) GB trống — cần ít nhất 8 GB. Không làm gì."; exit 1; }
echo "Sẽ: tải pmtiles + mc (kiểm SHA-256 đã ghim), kéo DuckDB + dựng tippecanoe (ảnh ghim digest) → dựng vn-mainland +"
echo "    vn-places (Overture $BM_OVERTURE) → tải lên $BM_BUCKET/config/map/$NGAY_NEN/. BASEMAP_URL KHÔNG đổi."
if [ "${GHI:-0}" != "1" ]; then
  echo ""
  echo "CHẠY THỬ — kế hoạch ở trên, KHÔNG làm gì. Làm thật: bỏ tick CHAY_THU, gõ XAC_NHAN=$NS (+ TICKET nếu prod)."
  exit 0
fi

# ── Tools: downloaded into the workspace, checked against the pins, never installed ───────────
TOOLS="$WORKSPACE/.tools"
rm -rf "$TOOLS" "$WORKSPACE/basemap" "$WORKSPACE/.mc"
mkdir -p "$TOOLS/dl" "$WORKSPACE/.mc"
TAB=$(printf '\t')
while IFS="$TAB" read -r name url sha kind; do
  [ -n "$name" ] || continue
  f="$TOOLS/dl/$name.download"
  curl -fsSL --retry 3 --proto '=https' --proto-redir '=https' -o "$f" "$url"
  echo "$sha  $f" | sha256sum -c --quiet - || {
    echo "DỪNG: $name tải từ $url KHÔNG khớp SHA-256 đã ghim trong basemap.py — không chạy tệp ấy."; exit 1; }
  case "$kind" in
    tar-gz:*) member=${kind#tar-gz:}; tar -xzf "$f" -C "$TOOLS/dl" "$member"; mv "$TOOLS/dl/$member" "$TOOLS/$name" ;;
    binary) mv "$f" "$TOOLS/$name" ;;
    *) echo "DỪNG: kiểu tệp '$kind' lạ"; exit 1 ;;
  esac
  chmod 0700 "$TOOLS/$name"
done < "$TMP_DIR/tools.tsv"
rm -rf "$TOOLS/dl"
PATH="$TOOLS:$PATH"; export PATH
export MC_CONFIG_DIR="$WORKSPACE/.mc"
echo "pmtiles: $(pmtiles version 2>&1 | head -1) · mc: $(mc --version 2>&1 | head -1)"

# ── Images of the places archive: pulled/built from pins only, never a tag from a registry ────
DUCKDB_IMAGE=$(sed -n 's/^DUCKDB_IMAGE=//p' "$TMP_DIR/images.env")
TIPPECANOE_BASE=$(sed -n 's/^TIPPECANOE_BASE=//p' "$TMP_DIR/images.env")
TIPPECANOE_COMMIT=$(sed -n 's/^TIPPECANOE_COMMIT=//p' "$TMP_DIR/images.env")
TIPPECANOE_IMAGE=$(sed -n 's/^TIPPECANOE_TAG=//p' "$TMP_DIR/images.env")
docker pull -q "$DUCKDB_IMAGE" >/dev/null || { echo "DỪNG: không kéo được $DUCKDB_IMAGE (máy build tới được Docker Hub?)."; exit 1; }
docker pull -q "$TIPPECANOE_BASE" >/dev/null || { echo "DỪNG: không kéo được $TIPPECANOE_BASE."; exit 1; }
# No build context (stdin): the Dockerfile fetches exactly TIPPECANOE_COMMIT and checks it before compiling.
# `buildx build --load` when buildx exists: a buildx builder with the docker-container driver builds the
# image INSIDE its own container and never hands it to the local daemon unless told to — the build
# "succeeds" and `docker run` then finds nothing (lượt Jenkins 12/10/2026, 22:07: pull access denied for
# vigov-basemap/tippecanoe, after the whole mainland cut).
TIP_BUILD_ARGS=(--progress=plain -t "$TIPPECANOE_IMAGE" --build-arg BASE_IMAGE="$TIPPECANOE_BASE"
  --build-arg TIPPECANOE_COMMIT="$TIPPECANOE_COMMIT" -)
if docker buildx version >/dev/null 2>&1; then
  docker buildx build --load "${TIP_BUILD_ARGS[@]}" < deploy/basemap/tippecanoe.Dockerfile
else
  DOCKER_BUILDKIT=1 docker build "${TIP_BUILD_ARGS[@]}" < deploy/basemap/tippecanoe.Dockerfile
fi || { echo "DỪNG: dựng ảnh tippecanoe ($TIPPECANOE_COMMIT) hỏng — xem log ở trên."; exit 1; }
# Checked here, before the hour of building, not discovered by `docker run` at step 4/4.
docker image inspect "$TIPPECANOE_IMAGE" >/dev/null 2>&1 || {
  echo "DỪNG: dựng xong nhưng Docker cục bộ không có ảnh $TIPPECANOE_IMAGE — builder buildx đang dùng không nạp ảnh về"
  echo "      (docker buildx ls: builder có dấu * phải dùng driver 'docker', hoặc build cần --load). Chưa dựng nền."; exit 1; }
echo "duckdb: $DUCKDB_IMAGE · tippecanoe: $TIPPECANOE_IMAGE (commit $TIPPECANOE_COMMIT)"
export DUCKDB_IMAGE TIPPECANOE_IMAGE

# ── Keys: Secret → pipe → 0600 file. Never a shell variable, never argv, never echoed ─────────
K="$WORKSPACE/.mc/keys"
mkdir -p "$K"
kubectl -n "$NS" get secret "$KEY_SECRET" -o go-template='{{index .data "OBJECT_STORAGE_ACCESS_KEY"}}' | base64 -d > "$K/ak"
kubectl -n "$NS" get secret "$KEY_SECRET" -o go-template='{{index .data "OBJECT_STORAGE_SECRET_KEY"}}' | base64 -d > "$K/sk"
OPS mc-host --endpoint "$BM_ENDPOINT" --access-key-file "$K/ak" --secret-key-file "$K/sk" --out "$K/mchost"
rm -f "$K/ak" "$K/sk"
MC_HOST_vgbasemap=$(cat "$K/mchost"); export MC_HOST_vgbasemap
rm -rf "$K"

# Before an hour of building: the key works, the policy lists config/map/, the date is new.
# build-basemap.sh's own overwrite guard hides `mc ls` errors — a denied list would pass it.
if ! mc ls "vgbasemap/$BM_BUCKET/config/map/" > "$TMP_DIR/ls.txt" 2> "$TMP_DIR/ls.err"; then
  sed -e 's#//[^/@ ]*@#//***@#g' -e 's/^/   /' "$TMP_DIR/ls.err"
  if grep -qi 'denied\|forbidden\|403' "$TMP_DIR/ls.err"; then
    echo "DỪNG: policy MinIO của khoá trong secret/$KEY_SECRET không phủ $BM_BUCKET/config/map/* — người quản trị MinIO thêm s3:ListBucket (prefix config/map/*) + s3:GetObject + s3:PutObject trên arn:aws:s3:::$BM_BUCKET/config/map/* vào policy của user ấy (mc admin policy create/attach), rồi bấm lại. Chưa dựng gì."
  else
    echo "DỪNG: không liệt kê được $BM_BUCKET/config/map/ qua $BM_ENDPOINT — máy build không tới được MinIO, hoặc khoá trong secret/$KEY_SECRET sai. Chưa dựng gì."
  fi
  exit 1
fi
# Same date again OVERWRITES the prefix (owner, 12/10/2026 — reverses the earlier "a refresh is always a
# new date"). Said out loud, never silent: if BASEMAP_URL already reads this prefix, maps being viewed
# during the upload can mix byte ranges of the old and new archives until reloaded, and the signed
# sovereignty checklist of this prefix no longer matches its SHA-256 — it must be checked and signed again.
if grep -q " $NGAY_NEN/\$" "$TMP_DIR/ls.txt"; then
  LIVE_URL=$("$PYTHON" -c 'import json,sys; print(((json.load(open(sys.argv[1])).get("data") or {}).get("BASEMAP_URL") or "").strip().rstrip("/"))' "$TMP_DIR/cm.json")
  if [ -n "$LIVE_URL" ] && [ "$LIVE_URL" = "${BM_URL%/}" ]; then
    echo "⚠ GHI ĐÈ BẢN ĐANG DÙNG: BASEMAP_URL của $NS đang đọc chính $BM_BUCKET/config/map/$NGAY_NEN/."
    echo "  Trong lúc tải lên, bản đồ đang mở có thể vỡ ô tới khi tải lại trang; danh mục kiểm chủ quyền đã ký"
    echo "  của tiền tố này hết khớp SHA-256 — kiểm và ký lại sau lượt này."
  else
    echo "⚠ GHI ĐÈ: $BM_BUCKET/config/map/$NGAY_NEN/ đã có — tệp cùng tên sẽ bị thay bằng bản dựng lượt này."
  fi
  note "MinIO $BM_BUCKET/config/map/$NGAY_NEN/: GHI ĐÈ tiền tố đã có"
fi

# ── Build + upload ────────────────────────────────────────────────────────────────────────────
bash web-admin/scripts/build-basemap.sh build --date "$NGAY_NEN" --overture "$BM_OVERTURE" --out "$WORKSPACE/basemap"
bash web-admin/scripts/build-basemap.sh upload --date "$NGAY_NEN" --out "$WORKSPACE/basemap" \
  --alias vgbasemap --bucket "$BM_BUCKET"
note "MinIO $BM_BUCKET/config/map/$NGAY_NEN/: tải nền bản đồ lên (CHƯA đưa vào dùng — BASEMAP_URL không đổi)"
DIR="$WORKSPACE/basemap/$NGAY_NEN"
if mc cat "vgbasemap/$BM_BUCKET/config/map/$NGAY_NEN/SHA256SUMS" 2>/dev/null | cmp -s - "$DIR/SHA256SUMS"; then
  echo "✓ SHA256SUMS trên MinIO khớp bản vừa dựng"
else
  echo "✗ SHA256SUMS trên MinIO KHÁC bản vừa dựng — KHÔNG đặt BASEMAP_URL; kiểm tay (runbook dòng 7)."; exit 1
fi
echo ""
echo "════ GHI VÀO DANH MỤC KÝ ════════════════════════════════════════════════════"
echo "── SHA-256 vn-mainland.pmtiles"
grep 'vn-mainland[.]pmtiles$' "$DIR/SHA256SUMS"
echo "── SHA-256 vn-places.pmtiles"
grep 'vn-places[.]pmtiles$' "$DIR/SHA256SUMS"
echo "── BUILD-INFO.txt"
cat "$DIR/BUILD-INFO.txt"
echo "── Giá trị BASEMAP_URL (job KHÔNG đặt):"
echo "   $BM_URL"
echo "════ BƯỚC TIẾP ═══════════════════════════════════════════════════════════════"
echo "1. DEV: đặt BASEMAP_URL trong common-config của vigov-dev (runbook, mục 'Đưa vào dùng'), khởi"
echo "   động lại vigov-web-admin và vigov-service-comms. URL trên là của $NS: dev có MinIO/bucket riêng"
echo "   thì chạy lại việc này với MT=dev để tải lên bucket của dev, rồi dùng URL lượt ấy in ra."
echo "2. Kiểm chủ quyền có ký trên dev — kb/40-runbooks/basemap-pmtiles.md. Một dòng không đạt: DỪNG."
echo "3. Chỉ sau khi đã ký: cùng giá trị ở vigov-prod. Không xoá tiền tố cũ (đường quay lui)."
