# syntax=docker/dockerfile:1.7
#
# Ảnh của dịch vụ `comms`. Tệp này thuộc về DỊCH VỤ NÀY.
#
# Cách đóng gói một dịch vụ — ảnh nền, cổng, entrypoint, thứ nó cần lúc chạy — là một phần
# hợp đồng của nó với nền tảng, nên nó nằm cạnh dịch vụ chứ không ở một tệp dùng chung.
#
# NHỮNG DÒNG ĐÁNH DẤU `#[bat-bien]` LÀ BẤT BIẾN AN TOÀN dùng chung cho mọi dịch vụ.
# `tools/check_build.py` (chạy trong `make check`) đỏ nếu tệp nào đánh rơi chúng — tách tệp ra
# là để dịch vụ tự quyết phần RIÊNG, không phải để lặng lẽ bỏ phần CHUNG.
#
# Dựng — NGỮ CẢNH LÀ GỐC KHO, không phải thư mục này:
#   DOCKER_BUILDKIT=1 docker build -f service-comms/Dockerfile \
#     --build-arg VERSION=$(git rev-parse --short=12 HEAD) \
#     -t <registry>/<project>/vigov-service-comms:<sha> .
#
# Gốc kho vì dịch vụ này `replace` module `core` bằng đường dẫn `../core`.
#
# KHÔNG CHÉP `go.work` VÀO ẢNH, và đó là điều quan trọng nhất ở tệp này. go.work làm mọi
# module thấy nhau qua thư mục, nên nó CHE mất một `require` bị thiếu trong go.mod. Ảnh không
# có nó, nên lượt build này CHỨNG MINH dịch vụ dựng được chỉ với `core/` và chính nó.

ARG GO_VERSION=1.26
ARG RUNTIME=gcr.io/distroless/static-debian12:nonroot   #[bat-bien] khong shell, khong root

# ---------------------------------------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS build
ARG VERSION=dev
WORKDIR /src

# Chép manifest trước mã nguồn: tầng tải phụ thuộc chỉ đổi khi go.mod/go.sum đổi.
COPY core/go.mod core/go.sum ./core/
COPY service-comms/go.mod service-comms/go.sum ./service-comms/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd service-comms && go mod download

COPY core ./core
COPY service-comms ./service-comms

# core/gen nằm trong .gitignore và mã nguồn import nó; buf.gen.yaml dùng plugin TỪ XA nên nó
# không sinh được bên trong ảnh. `make proto` chạy ở máy chủ build. Ở đây chỉ nói thẳng khi
# thiếu, thay vì để `go build` báo "package not found" chẳng chỉ ra điều gì.
RUN test -d core/gen || { \
      echo ""; \
      echo "THIẾU core/gen — mã sinh từ proto chưa có trong ngữ cảnh build."; \
      echo "Sinh trước ở máy chủ build:   make proto"; \
      echo ""; \
      exit 1; }

# CGO_ENABLED=0 #[bat-bien]: nhị phân tĩnh, chạy được trên ảnh nền không libc. `-race` là thứ
# duy nhất cần cgo, và nó thuộc cổng kiểm chứ không thuộc ảnh phát hành.
# -trimpath: bỏ đường dẫn tuyệt đối của máy build khỏi stack trace.
ENV CGO_ENABLED=0 GOOS=linux
WORKDIR /src/service-comms
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/server ./cmd/server

# ---------------------------------------------------------------------------------------
FROM ${RUNTIME}
ARG VERSION=dev

# #[bat-bien] MÚI GIỜ. distroless/static không có /usr/share/zoneinfo, mà luật 10 bất biến 4
# bắt đếm hạn xử lý theo GIỜ LÀM VIỆC của từng xã. Hôm nay kho chưa có dòng LoadLocation nào
# nên thiếu nó chưa hỏng gì — nó hỏng vào đúng ngày ai đó viết phần tính hạn, ở môi trường
# thật, và không ai nghĩ tới ảnh Docker.
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=build /out/server /server

# 8080 REST ra trình duyệt · 9090 gRPC giữa các dịch vụ.
# 9090 KHÔNG ĐƯỢC PHƠI RA INTERNET: không mTLS, không qua lớp kiểm quyền của bề mặt REST.
# EXPOSE chỉ là tài liệu — chặn thật là NetworkPolicy phía k8s.
EXPOSE 8080 9090

# #[bat-bien] distroless:nonroot chạy UID 65532. Khai lại để nó đọc được từ chính tệp này.
USER 65532:65532

# KHÔNG HEALTHCHECK: ảnh không có shell lẫn curl. Dịch vụ phục vụ `GET /healthz` NGOÀI chuỗi
# phân giải xã, nên probe phía k8s dùng httpGet /healthz cổng 8080.

LABEL org.opencontainers.image.title="vigov-service-comms" \
      org.opencontainers.image.revision="${VERSION}" \
      org.opencontainers.image.source="https://github.com/tamtd91ht/vigov-v2" \
      org.opencontainers.image.vendor="VIHAT"

ENTRYPOINT ["/server"]
