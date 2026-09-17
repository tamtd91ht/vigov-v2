# syntax=docker/dockerfile:1.7
#
# Ảnh của web quản trị xã. Tệp này thuộc về ứng dụng này.
#
# Dựng — NGỮ CẢNH LÀ CHÍNH THƯ MỤC NÀY, khác với tám dịch vụ Go:
#   DOCKER_BUILDKIT=1 docker build -f web-admin/Dockerfile \
#     --build-arg VERSION=$(git rev-parse --short=12 HEAD) \
#     -t <registry>/<project>/vigov-web-admin:<sha> web-admin
#
# Các dịch vụ Go phải lấy ngữ cảnh ở gốc kho vì go.mod của chúng `replace` module core bằng
# đường dẫn `../core` (ADR 0016) — mỗi dịch vụ có go.mod RIÊNG, thứ dùng chung là `core/` và
# `core/gen/`. Ứng dụng này không dùng chung gì cả — `package.json` và lock riêng của nó —
# nên ngữ cảnh của nó là thư mục của nó. Ít thứ gửi sang trình đóng ảnh cũng là ít thứ có
# thể lọt vào ảnh. Loại trừ: `.dockerignore` ngay cạnh tệp này.

ARG NODE_VERSION=22

# ---------------------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS deps
WORKDIR /app

# `npm ci` chứ không `npm install`: ci cài ĐÚNG những gì package-lock.json ghi và đổ nếu lock
# lệch package.json. `install` thì tự sửa lock — tức ảnh phát hành có thể mang một cây phụ
# thuộc chưa ai từng chạy thử.
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
# Mọi biến `NEXT_PUBLIC_*` có mặt lúc build đều bị NUNG THẲNG vào bundle trình duyệt. Trong
# một hệ thống mà MỘT tiến trình phục vụ 200+ xã phân biệt bằng tên miền, một giá trị của xã
# nằm trong bundle nghĩa là ảnh ấy chỉ còn đúng cho một xã (luật 1 bất biến 10, luật 8 bất
# biến 4).
#
# Nó không hỏng ở xã đầu tiên. Nó hỏng ở xã THỨ HAI, dưới dạng tên xã khác hiện trên màn
# hình một cơ quan nhà nước — và lúc ấy ảnh đã chạy ở mọi nơi. Không bài test nào đỏ.
#
# Nếu về sau thật sự cần một hằng số công khai TOÀN NỀN TẢNG (không thuộc xã nào), hãy gỡ
# rào này một cách tường minh và ghi lý do — đừng lách nó bằng một tên biến khác.
# PHÉP KIỂM NẰM CÙNG MỘT `RUN` VỚI `npm run build`, CÓ CHỦ Ý — và đây là điều dễ vô hiệu hoá
# nhất ở cả tệp. Khi nó đứng thành một `RUN` riêng phía trên, nó chỉ đo môi trường TẠI THỜI
# ĐIỂM ẤY: một dòng `ENV NEXT_PUBLIC_...` thêm vào giữa nó và `npm run build` không bị nó
# nhìn thấy — mà đó đúng là chỗ người ta sẽ thêm, ngay cạnh các ENV khác, ngay cạnh dòng
# build. Đo được bằng một lượt đột biến: rào cũ vẫn xanh trước đúng ca ấy.
#
# Cùng một shell với `npm run build` thì không còn khe nào để chen vào.
ENV NEXT_TELEMETRY_DISABLED=1
RUN if env | grep -q '^NEXT_PUBLIC_'; then \
      echo ""; \
      echo "TỪ CHỐI BUILD: có biến NEXT_PUBLIC_* trong môi trường build."; \
      env | grep '^NEXT_PUBLIC_' | cut -d= -f1; \
      echo ""; \
      echo "Biến NEXT_PUBLIC_* đi thẳng vào bundle trình duyệt. Giá trị riêng của xã phải"; \
      echo "được đọc LÚC CHẠY từ Host — xem src/lib/tenant.server.ts."; \
      echo ""; \
      exit 1; \
    fi && npm run build
# ─────────────────────────────────────────────────────────────────────────────────────────

# ---------------------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS runtime
WORKDIR /app
ARG VERSION=dev

# HOSTNAME=0.0.0.0 là dòng dễ bị bỏ sót nhất ở đây: máy chủ standalone mặc định nghe trên
# localhost ở một số bản Next. Trong container thì đó là "không ai gọi được", và triệu chứng
# là probe của k8s thất bại — không phải một thông báo lỗi nào đọc được.
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

# `output: "standalone"` (next.config.ts) gói sẵn đúng phần node_modules thật sự chạy, nên
# ảnh này KHÔNG mang mã nguồn lẫn cây phụ thuộc đầy đủ. Ít thứ trong ảnh cũng là ít thứ phải
# vá khi có CVE.
#
# KHÔNG chép `public/`: thư mục đó chưa tồn tại, và một dòng COPY trỏ vào chỗ không có sẽ làm
# build đổ. Khi nào có tài sản tĩnh thì thêm dòng ấy cùng lúc.
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static

EXPOSE 3000

# Chạy dưới người dùng `node` (UID 1000) có sẵn trong ảnh, không phải root.
USER node

LABEL org.opencontainers.image.title="vigov-web-admin" \
      org.opencontainers.image.revision="${VERSION}" \
      org.opencontainers.image.source="https://github.com/tamtd91ht/vigov-v2" \
      org.opencontainers.image.vendor="VIHAT"

CMD ["node", "server.js"]
