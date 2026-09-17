# syntax=docker/dockerfile:1.7
#
# MỘT Dockerfile CHO CẢ TÁM DỊCH VỤ GO, chọn bằng `--build-arg SERVICE=<tên>`.
#
# Tám bản sao của một tệp giống hệt nhau là tám bản sẽ lệch nhau, và bản không ai cập nhật
# là bản người sau đi theo (luật 9, bất biến 1). Tám dịch vụ này có CÙNG hình dạng build —
# cùng go.mod, cùng `cmd/server`, cùng cổng — nên khác biệt duy nhất xứng đáng tồn tại là
# một tham số.
#
# Dựng:
#   DOCKER_BUILDKIT=1 docker build -f build/go.Dockerfile \
#     --build-arg SERVICE=identity --build-arg VERSION=$(git rev-parse --short HEAD) \
#     -t <registry>/vigov-identity:<sha> .
#
# Ngữ cảnh build là GỐC KHO, không phải services/<tên>: các dịch vụ dùng chung `pkg/`,
# `go.mod` và `gen/`.
#
# CẦN BuildKit (`RUN --mount=type=cache`). Jenkinsfile đặt DOCKER_BUILDKIT=1 tường minh chứ
# không trông vào mặc định của máy chủ build.

ARG GO_VERSION=1.26
# Ảnh nền cuối: KHÔNG shell, KHÔNG trình quản lý gói, chạy dưới người dùng không phải root.
# Không có shell nghĩa là một lỗ RCE trong mã Go không tìm thấy `sh` để leo tiếp, và cũng
# nghĩa là `kubectl exec` vào pod này sẽ không vào được — đó là đánh đổi có chủ ý, đổi khả
# năng gỡ lỗi tại chỗ lấy một bề mặt tấn công nhỏ hơn hẳn cho một hệ thống nhà nước.
ARG RUNTIME=gcr.io/distroless/static-debian12:nonroot

# ---------------------------------------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS build

ARG SERVICE
ARG VERSION=dev
WORKDIR /src

# Từ chối SỚM và NÓI RÕ, thay vì để `go build` báo một lỗi khó hiểu ở phút thứ ba.
RUN test -n "${SERVICE}" || { \
      echo ""; \
      echo "THIẾU --build-arg SERVICE=<tên dịch vụ>."; \
      echo "Hợp lệ: comms documents dossiers finance identity petitions platform reporting"; \
      echo ""; \
      exit 1; }

# Tải phụ thuộc trước khi chép mã nguồn: tầng này chỉ đổi khi go.mod/go.sum đổi, nên chín
# ảnh trong một lượt build dùng chung đúng một lần tải.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# gen/ NẰM TRONG .gitignore — và mã nguồn IMPORT nó.
#
# `gen/` được sinh từ proto/ bằng `buf generate`, nên một bản checkout sạch KHÔNG có nó và
# `go build ./...` sẽ đổ ở một lỗi "package not found" chẳng nói lên điều gì. buf.gen.yaml
# dùng plugin TỪ XA (buf.build), tức bước sinh cần mạng ra ngoài — thứ mà một build ảnh
# thường không có và cũng không nên có.
#
# Nên bước sinh thuộc về máy chủ build, không thuộc về ảnh: Jenkinsfile chạy `make proto`
# trước khi đóng ảnh. Ở đây chỉ kiểm tra và nói thẳng khi thiếu.
RUN test -d gen || { \
      echo ""; \
      echo "THIẾU gen/ — mã sinh từ proto chưa có trong ngữ cảnh build."; \
      echo "gen/ nằm trong .gitignore và được sinh bằng plugin từ xa, nên nó phải được"; \
      echo "sinh TRƯỚC ở máy chủ build:   make proto"; \
      echo ""; \
      exit 1; }

RUN test -d "services/${SERVICE}/cmd/server" || { \
      echo ""; \
      echo "KHÔNG CÓ dịch vụ '${SERVICE}' — không thấy services/${SERVICE}/cmd/server"; \
      echo ""; \
      exit 1; }

# CGO_ENABLED=0: nhị phân tĩnh, chạy được trên ảnh nền không có libc. Không gói nào trong
# kho này dùng cgo (đã kiểm), và `-race` — thứ DUY NHẤT cần cgo — là việc của cổng kiểm ở
# máy chủ build, không phải của ảnh phát hành.
#
# -trimpath: bỏ đường dẫn tuyệt đối của máy build khỏi nhị phân. Một stack trace in ra
# /home/jenkins/workspace/... là rò rỉ nhỏ nhưng miễn phí để chặn.
ENV CGO_ENABLED=0 GOOS=linux
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
        -o /out/server "./services/${SERVICE}/cmd/server"

# ---------------------------------------------------------------------------------------
FROM ${RUNTIME}

ARG SERVICE
ARG VERSION=dev

# MÚI GIỜ, và đây không phải chi tiết vặt.
#
# distroless/static KHÔNG có /usr/share/zoneinfo. Thiếu nó, `time.LoadLocation("Asia/...")`
# trả về lỗi — và luật 10 bất biến 4 bắt mọi hạn xử lý phải đếm theo GIỜ LÀM VIỆC của từng
# xã. Một hạn tính sai là một cam kết sai với người dân, và con số sai ấy đi thẳng lên báo
# cáo cho lãnh đạo.
#
# Hôm nay chưa có dòng LoadLocation nào trong kho, nên thiếu nó sẽ KHÔNG hỏng gì cả — nó sẽ
# hỏng vào đúng ngày ai đó viết phần tính hạn, ở môi trường thật, và không ai nghĩ tới ảnh
# Docker. Ba megabyte để một cái bẫy không bao giờ được sập.
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=build /out/server /server

# 8080 REST (ra trình duyệt) · 9090 gRPC (giữa các dịch vụ).
#
# HAI CỔNG TÁCH RIÊNG, và 9090 KHÔNG ĐƯỢC PHƠI RA INTERNET: nó là kênh giữa các dịch vụ,
# không có mTLS, và không đi qua lớp kiểm quyền của bề mặt REST. Một Ingress trỏ nhầm vào
# 9090 là mở thẳng nội thất hệ thống. EXPOSE ở đây chỉ là tài liệu — chặn thật là việc của
# NetworkPolicy và Service bên phía k8s.
EXPOSE 8080 9090

# distroless:nonroot chạy dưới UID 65532. Khai lại ở đây để nó là một sự thật đọc được từ
# tệp này. Phía k8s nên đặt runAsNonRoot: true — nếu ảnh bị đổi thành chạy root, pod đổ chứ
# không âm thầm leo quyền.
USER 65532:65532

# KHÔNG CÓ HEALTHCHECK: ảnh không có shell lẫn curl để chạy nó. Mọi dịch vụ đều phục vụ
# `GET /healthz` NGOÀI chuỗi phân giải xã (xem cmd/server/main.go), nên probe phía k8s dùng
# httpGet trên /healthz cổng 8080. Một exec probe sẽ không chạy được trên ảnh này.

LABEL org.opencontainers.image.title="vigov-${SERVICE}" \
      org.opencontainers.image.revision="${VERSION}" \
      org.opencontainers.image.source="https://github.com/tamtd91ht/vigov-v2" \
      org.opencontainers.image.vendor="VIHAT"

ENTRYPOINT ["/server"]
