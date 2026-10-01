# syntax=docker/dockerfile:1.7
#
# Ảnh của bàn điều khiển vận hành ViHAT (khu vận hành, ADR 0048). Tệp này thuộc về ứng dụng này.
#
# Dựng — NGỮ CẢNH LÀ CHÍNH THƯ MỤC NÀY, như web-admin (lý do đầy đủ ở đầu `web-admin/Dockerfile`):
#   DOCKER_BUILDKIT=1 docker build -f platform-admin/Dockerfile \
#     --build-arg VERSION=$(git rev-parse --short=12 HEAD) \
#     -t <registry>/<project>/vigov-platform-admin:<sha> platform-admin
#
# Ứng dụng này có `package.json` và lock RIÊNG, không dùng chung gì với web-admin. Loại trừ:
# `.dockerignore` ngay cạnh tệp này.

ARG NODE_VERSION=22

# ---------------------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS deps
WORKDIR /app

# `npm ci` chứ không `npm install`: ci cài ĐÚNG những gì package-lock.json ghi và đổ nếu lock
# lệch package.json.
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

# ---------------------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS build
WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .

# ─────────────────────────────────────────────────────────────────────────────────────────
# RÀO CHẮN THẬT, KHÔNG PHẢI MỘT DÒNG CHÚ THÍCH.
#
# Mọi biến `NEXT_PUBLIC_*` có mặt lúc build đều bị NUNG THẲNG vào bundle trình duyệt. Với bàn
# điều khiển này điều đó nặng hơn ở web-admin: bật khu vận hành hay trỏ nó đi đâu bằng một hằng
# số lúc build là ĐIỀU KIỆN DỪNG #5 của ADR 0048, và hai biến duy nhất ứng dụng đọc
# (PLATFORM_HTTP_ADDR, OPERATOR_HOST) được đọc LÚC CHẠY, phía máy chủ (`src/lib/server/`).
#
# PHÉP KIỂM NẰM CÙNG MỘT `RUN` VỚI `npm run build`, CÓ CHỦ Ý: một `RUN` riêng phía trên chỉ đo
# môi trường tại thời điểm ấy, và một dòng `ENV NEXT_PUBLIC_...` chen vào giữa sẽ lọt (đo được
# bằng đột biến ở web-admin). Cùng một shell với `npm run build` thì không còn khe nào.
ENV NEXT_TELEMETRY_DISABLED=1
RUN if env | grep -q '^NEXT_PUBLIC_'; then \
      echo ""; \
      echo "TỪ CHỐI BUILD: có biến NEXT_PUBLIC_* trong môi trường build."; \
      env | grep '^NEXT_PUBLIC_' | cut -d= -f1; \
      echo ""; \
      echo "Biến NEXT_PUBLIC_* đi thẳng vào bundle trình duyệt. Bàn điều khiển vận hành chỉ đọc"; \
      echo "cấu hình LÚC CHẠY, phía máy chủ — xem src/lib/server/platform-origin.ts."; \
      echo ""; \
      exit 1; \
    fi && npm run build
# ─────────────────────────────────────────────────────────────────────────────────────────

# ---------------------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS runtime
WORKDIR /app
ARG VERSION=dev

# HOSTNAME=0.0.0.0: máy chủ standalone mặc định nghe trên localhost ở một số bản Next — trong
# container là "không ai gọi được", và triệu chứng chỉ là probe thất bại.
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

# `output: "standalone"` (next.config.ts): ảnh không mang mã nguồn lẫn cây phụ thuộc đầy đủ.
#
# KHÔNG chép `public/`: thư mục đó chưa tồn tại, và một dòng COPY trỏ vào chỗ không có làm build
# đổ. Khi nào có tài sản tĩnh thì thêm dòng ấy cùng lúc.
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static

EXPOSE 3000

# Chạy dưới người dùng `node` (UID 1000) có sẵn trong ảnh, không phải root.
USER node

LABEL org.opencontainers.image.title="vigov-platform-admin" \
      org.opencontainers.image.revision="${VERSION}" \
      org.opencontainers.image.source="https://github.com/tamtd91ht/vigov-v2" \
      org.opencontainers.image.vendor="VIHAT"

CMD ["node", "server.js"]
