# syntax=docker/dockerfile:1.7
#
# Ảnh của dịch vụ `platform`. Tệp này thuộc về DỊCH VỤ NÀY.
#
# Cách đóng gói một dịch vụ — ảnh nền, cổng, entrypoint, thứ nó cần lúc chạy — là một phần
# hợp đồng của nó với nền tảng, nên nó nằm cạnh dịch vụ chứ không nằm ở một tệp dùng chung.
# Hôm nay tám dịch vụ đóng gói giống hệt nhau; ngày chúng khác nhau (reporting cần font
# tiếng Việt để sinh PDF, documents cần thư viện xử lý tệp) thì mỗi bên sửa tệp của mình,
# không ai phải đụng vào một tệp mà bảy dịch vụ khác cũng đang dùng.
#
# NHỮNG DÒNG CÓ ĐÁNH DẤU `#[bat-bien]` LÀ BẤT BIẾN AN TOÀN dùng chung cho mọi dịch vụ.
# `tools/check_images.py` (chạy trong `make check`) đỏ nếu một tệp nào thiếu chúng — tách
# tệp ra là để dịch vụ tự quyết phần RIÊNG của nó, không phải để lặng lẽ đánh rơi phần chung.
#
# Dựng — NGỮ CẢNH LÀ GỐC KHO, không phải thư mục này:
#   DOCKER_BUILDKIT=1 docker build -f services/platform/Dockerfile #     --build-arg VERSION=$(git rev-parse --short=12 HEAD) #     -t <registry>/<project>/vigov-platform:<sha> .
#
# Gốc kho vì tám dịch vụ dùng chung một `go.mod`, `pkg/` và `gen/`.

ARG GO_VERSION=1.26
ARG RUNTIME=gcr.io/distroless/static-debian12:nonroot   #[bat-bien] khong shell, khong root

# ---------------------------------------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS build
ARG VERSION=dev
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# gen/ nằm trong .gitignore nhưng mã nguồn import nó, và buf.gen.yaml dùng plugin TỪ XA nên
# không sinh được bên trong ảnh. `make proto` chạy ở máy chủ build. Ở đây chỉ nói thẳng khi
# thiếu, thay vì để `go build` báo một lỗi "package not found" chẳng chỉ ra điều gì.
RUN test -d gen || {       echo "";       echo "THIẾU gen/ — mã sinh từ proto chưa có trong ngữ cảnh build.";       echo "Sinh trước ở máy chủ build:   make proto";       echo "";       exit 1; }

# CGO_ENABLED=0 #[bat-bien]: nhị phân tĩnh, chạy được trên ảnh nền không libc. `-race` là thứ
# duy nhất cần cgo, và nó thuộc cổng kiểm chứ không thuộc ảnh phát hành.
# -trimpath: bỏ đường dẫn tuyệt đối của máy build khỏi stack trace.
ENV CGO_ENABLED=0 GOOS=linux
RUN --mount=type=cache,target=/go/pkg/mod     --mount=type=cache,target=/root/.cache/go-build     go build -trimpath -ldflags "-s -w -X main.version=${VERSION}"         -o /out/server ./services/platform/cmd/server

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

LABEL org.opencontainers.image.title="vigov-platform"       org.opencontainers.image.revision="${VERSION}"       org.opencontainers.image.source="https://github.com/tamtd91ht/vigov-v2"       org.opencontainers.image.vendor="VIHAT"

ENTRYPOINT ["/server"]
