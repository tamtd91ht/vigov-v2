package config

import (
	"fmt"
	"strings"
)

// WHY THIS FILE EXISTS (29/09/2026). ADR 0057 made a declared group's variables required in
// staging and prod, and the first production pod it stopped said only
// `thiếu biến môi trường bắt buộc: TRUSTED_PROXY_CIDRS`. The operator reading that line had no
// idea what the variable meant, where its value comes from, or which k8s object to put it in — and
// the only place that knew was a Go comment and a README he had no reason to open. A refusal that
// cannot be acted on is an outage that lasts as long as the search for the answer.
//
// So every variable Load can refuse carries its explanation HERE, and Load prints it at the point
// of failure. hints_test.go derives the refusable set from the r.read calls in config.go and is
// red for any of them without an entry: a variable cannot become required without saying what it
// is.
//
// ONE SOURCE (rule 9): this table is the source of the startup message. deploy/cau-hinh/README.md
// is the operator's map of which object holds which key and links here for the full text; a test
// keeps the two agreeing on the object.
//
// NO VALUE IS EVER PRINTED. The table holds static text only, and the renderer takes names, never
// values — a missing variable has no value, and a present one may be a credential (rule 8).
// Shapes are placeholders; a secret's shape is the command that generates one.

// placement is the k8s object that supplies a variable to the pod.
type placement int

const (
	// inConfigMap: the ConfigMap every Go pod reads through envFrom — not a credential.
	inConfigMap placement = iota + 1
	// inSecret: the service's own Secret, read through envFrom — anything credentialed.
	inSecret
	// inDeploymentEnv: the `env:` block of this service's Deployment — a value only this service
	// reads, which in the shared ConfigMap would be handed to every other pod as well.
	inDeploymentEnv
)

// The object names the RUNNING cluster uses (built by hand in Rancher; deploy/Jenkinsfile:5-17 and
// :32, measured 25/09/2026). A wrong name here is worse than none: the operator creates an object
// no Deployment reads, and the pod stops again with the same message. hints_test.go checks them
// against deploy/base, whose envFrom must read the same objects.
const configMapName = "common-config"

func secretName(service string) string     { return service + "-secrets" }
func deploymentName(service string) string { return "vigov-service-" + service }

// describe renders where the variable goes, for this service.
func (p placement) describe(service string) string {
	switch p {
	case inConfigMap:
		return fmt.Sprintf("ConfigMap `%s` — dùng chung cho mọi pod Go", configMapName)
	case inSecret:
		return fmt.Sprintf("Secret `%s` — KHÔNG đặt vào ConfigMap: giá trị là thông tin xác thực", secretName(service))
	case inDeploymentEnv:
		return fmt.Sprintf("khối `env:` của Deployment `%s` — không đặt vào ConfigMap chung", deploymentName(service))
	}
	return "chưa xác định — xem deploy/cau-hinh/README.md"
}

// envHint explains one refusable variable to the operator who has to set it. Vietnamese: it is
// read by the people running the cluster (rule 12 invariant 2).
type envHint struct {
	meaning string    // what it is, and what breaks without it — one sentence
	source  string    // how to obtain the value: a command, or who holds it
	place   placement // which k8s object carries it
	shape   string    // placeholder shape of the value; for a secret, never a value
}

// newKeyList is the shape of every rotatable key list: first entry signs/encrypts, the rest verify.
const newKeyList = "<khoá mới>,<khoá cũ> — khoá đầu dùng để ký/mã hoá, các khoá sau chỉ để đọc khi đang xoay khoá; lần đầu chỉ cần một khoá"

// grpcAddrHint is the shape shared by every inter-service address.
func grpcAddrHint(target, what string) envHint {
	return envHint{
		meaning: fmt.Sprintf("Địa chỉ gRPC nội bộ của dịch vụ %s — %s.", target, what),
		source: fmt.Sprintf("Tên Service của %s trên cụm cộng cổng 9090: kubectl get svc -A | grep %s. "+
			"Chỉ địa chỉ trong cụm — kênh này không mã hoá (ADR 0025).", target, target),
		place: inDeploymentEnv,
		// The Service name is NOT asserted: deploy/cau-hinh/README.md writes `platform:9090`,
		// deploy/Jenkinsfile:9 says the live Service is vigov-service-<service>. Only the cluster knows.
		shape: fmt.Sprintf("<tên Service của %s đúng như kubectl get svc in ra>:9090", target),
	}
}

// envHints is keyed by variable name. Every variable Load can refuse has an entry
// (TestEveryRefusableVariableHasAHint); an entry for a variable Load never refuses is red too.
var envHints = map[string]envHint{
	"ENV": {
		meaning: "Môi trường chạy — quyết định biến nào bắt buộc và chặn cờ nguy hiểm ở prod.",
		source:  "Đặt theo môi trường của cụm: staging hoặc prod.",
		place:   inConfigMap,
		shape:   "prod",
	},
	"DATABASE_DSN": {
		meaning: "Chuỗi kết nối PostgreSQL tới CSDL riêng của dịch vụ này — mỗi dịch vụ một CSDL, không dùng chung.",
		source:  "Người quản trị PostgreSQL cấp user và mật khẩu cho CSDL vigov_<dịch vụ>. Luôn sslmode=require, không bao giờ tắt mã hoá.",
		place:   inSecret,
		shape:   "postgres://<user>:<mật khẩu>@<host>:5432/vigov_<dịch vụ>?sslmode=require",
	},
	"TRUSTED_PROXY_CIDRS": {
		meaning: "Dải IP Pod (Pod CIDR) của ingress-nginx và web-admin — các trạm chuyển tiếp được tin khi báo IP người dùng qua X-Forwarded-For. " +
			"Thiếu thì vết kiểm toán ghi IP của pod thay vì IP người thao tác.",
		source: "Dải Pod của từng node: kubectl get nodes -o jsonpath='{range .items[*]}{.spec.podCIDR}{\"\\n\"}{end}' ; " +
			"IP pod ingress và web-admin: kubectl get pods -A -o wide | grep -E 'ingress|web-admin' (RKE2 chạy ingress-nginx trong kube-system). " +
			"RKE2/k3s thường là 10.42.0.0/16. Nếu ingress-nginx chạy hostNetwork thì thêm dải IP node (kubectl get nodes -o wide). " +
			"Không bao giờ 0.0.0.0/0 hay ::/0 — tin mọi địa chỉ là tin X-Forwarded-For giả của bất kỳ ai, pod từ chối khởi động.",
		place: inConfigMap,
		shape: "10.42.0.0/16 — nhiều dải cách nhau dấu phẩy",
	},
	"GRPC_CALLER_KEY": {
		meaning: "Khoá chung xác thực lời gọi gRPC giữa các dịch vụ ViGov (ADR 0025) — thiếu thì cổng gRPC trả lời bất kỳ ai, nên mọi môi trường đều từ chối.",
		source:  "Sinh một lần: openssl rand -base64 48. CÙNG MỘT giá trị ở Secret của cả 7 dịch vụ — đã có ở dịch vụ khác thì dùng lại đúng giá trị ấy.",
		place:   inSecret,
		shape:   "<kết quả của openssl rand -base64 48>",
	},
	"PLATFORM_GRPC_ADDR":  grpcAddrHint("platform", "nơi phân giải tên miền ra xã; thiếu thì không phục vụ được xã nào"),
	"IDENTITY_GRPC_ADDR":  grpcAddrHint("identity", "nơi đổi phiên cán bộ thành người dùng; thiếu thì mọi route cán bộ trả 503"),
	"PETITIONS_GRPC_ADDR": grpcAddrHint("petitions", "identity hỏi trước khi xoá mềm một đơn vị; thiếu thì không xoá được đơn vị nào"),
	"DOCUMENTS_GRPC_ADDR": grpcAddrHint("documents", "identity hỏi trước khi xoá mềm một đơn vị, petitions hỏi trước khi chuyển đơn thư thành nhiệm vụ (ADR 0085); "+
		"thiếu ở identity thì không xoá được đơn vị nào, thiếu ở petitions thì POST /api/v1/citizen-letter-tasks luôn trả 503"),
	"COMMS_GRPC_ADDR": grpcAddrHint("comms", "hộp thông báo mà bộ chạy tự động hoá gửi nhắc việc vào (ADR 0058); thiếu thì không xã nào nhận nhắc việc"),
	"REDIS_DSN": {
		meaning: "Redis chống gửi trùng và giới hạn tần suất — thiếu thì một yêu cầu gửi hai lần tạo hai bản ghi vĩnh viễn.",
		source:  "Người vận hành Redis cấp host và mật khẩu; một Redis dùng chung cho mọi xã (khoá đã mang tiền tố xã).",
		place:   inSecret,
		shape:   "redis://:<mật khẩu>@<host>:6379/0",
	},
	"SESSION_SIGNING_KEYS": {
		meaning: "Khoá ký phiên đăng nhập cán bộ — chỉ identity giữ; ai có khoá giả được phiên cán bộ của mọi xã.",
		source:  "Mỗi khoá: openssl rand -base64 48. Chỉ đặt ở Secret của identity, không ở dịch vụ nào khác.",
		place:   inSecret,
		shape:   newKeyList,
	},
	"CITIZEN_CORS_ALLOWED_ORIGINS": {
		meaning: "Các origin https mà Zalo Mini App chạy từ đó được gọi API công dân (CORS) — thiếu thì trình duyệt chặn Mini App.",
		source:  "Danh sách miền của Zalo Mini App, lấy đúng dòng này ở deploy/cau-hinh/README.md mục 2. Không bao giờ * hay http://.",
		place:   inConfigMap,
		shape:   "https://<miền Zalo>,https://*.<miền Zalo> — cách nhau dấu phẩy",
	},
	"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR": {
		meaning: "Cổng gRPC riêng của identity chỉ cho backend vihat-miniapp gọi để đổi phiên công dân (ADR 0045) — thiếu thì không công dân nào đăng nhập được Mini App.",
		source:  "Chọn một cổng khác 9090, trùng với quy tắc NetworkPolicy chỉ cho pod vihat-miniapp vào. Phải đặt cùng lúc với CITIZEN_SESSION_BRIDGE_KEYS.",
		place:   inDeploymentEnv,
		shape:   ":<cổng>",
	},
	"CITIZEN_SESSION_BRIDGE_KEYS": {
		meaning: "Khoá mà backend vihat-miniapp gửi khi gọi cầu phiên công dân — khác GRPC_CALLER_KEY, chỉ mở đúng một RPC.",
		source:  "Mỗi khoá: openssl rand -base64 48. Cùng giá trị đặt vào Secret của vihat-miniapp. Phải đặt cùng lúc với CITIZEN_SESSION_BRIDGE_LISTEN_ADDR.",
		place:   inSecret,
		shape:   newKeyList,
	},
	"OBJECT_STORAGE_ENDPOINT": {
		meaning: "Địa chỉ MinIO/S3 nội bộ mà dịch vụ gọi để lưu tệp đính kèm (ADR 0052) — thiếu thì mọi lần tải tệp bị từ chối.",
		source:  "Người vận hành MinIO; đúng một host, https.",
		place:   inConfigMap,
		shape:   "https://<minio nội bộ>:<cổng>",
	},
	"OBJECT_STORAGE_PUBLIC_ENDPOINT": {
		meaning: "Địa chỉ S3 API của MinIO mà trình duyệt và Mini App thấy — nằm trong đường dẫn tải có chữ ký (presigned URL). Là CỔNG API (mặc định :9000), KHÔNG phải MinIO Console (giao diện web, :9001): Console trả 200 cho lệnh tải lên nên điện thoại tưởng xong mà không có tệp nào (09/10/2026).",
		source:  "Người vận hành MinIO: tên miền công khai trỏ vào cổng S3 API của CÙNG MinIO với OBJECT_STORAGE_ENDPOINT, https. Không có tên miền nội bộ riêng thì đặt bằng OBJECT_STORAGE_ENDPOINT.",
		place:   inConfigMap,
		shape:   "https://<minio trình duyệt thấy>",
	},
	"OBJECT_STORAGE_ACCESS_KEY": {
		meaning: "Access key MinIO riêng của dịch vụ này — mỗi dịch vụ một cặp khoá (ADR 0052 §3).",
		source:  "Tạo user riêng trong MinIO (mc admin user add), gắn policy chỉ cho bucket của môi trường này.",
		place:   inSecret,
		shape:   "<access key của user MinIO của dịch vụ>",
	},
	"OBJECT_STORAGE_SECRET_KEY": {
		meaning: "Secret key đi cặp với OBJECT_STORAGE_ACCESS_KEY.",
		source:  "Đặt lúc tạo user MinIO; tự sinh được bằng openssl rand -base64 32.",
		place:   inSecret,
		shape:   "<secret key của user MinIO đó>",
	},
	"OBJECT_STORAGE_BUCKET_PREFIX": {
		meaning: "Tiền tố tên bucket của môi trường này — bucket thật là <tiền tố>-private, -public, -temp.",
		source:  "Theo môi trường; chữ thường, số và dấu gạch ngang.",
		place:   inConfigMap,
		shape:   "vigov-prod (staging: vigov-stg)",
	},
	"OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL": {
		meaning: "URL gốc công khai của bucket media công khai, cho ảnh và tệp Mini App hiển thị.",
		source:  "Người vận hành MinIO: địa chỉ công khai của bucket <tiền tố>-public.",
		place:   inConfigMap,
		shape:   "https://<host media>/<tiền tố>-public",
	},
	"MALWARE_SCANNER_ADDRESS": {
		meaning: "Địa chỉ clamd quét mã độc tệp tải lên (ADR 0052 §9) — thiếu thì mọi lần tải tệp bị từ chối.",
		source:  "Tên Service của ClamAV trên cụm cộng cổng 3310: kubectl get svc -A | grep -i clam. Không có tiền tố tcp://.",
		place:   inConfigMap,
		shape:   "<Service clamav>:3310 — nhiều địa chỉ cách nhau dấu phẩy",
	},
	"SECRET_ENCRYPTION_KEYS": {
		meaning: "Khoá mã hoá các bí mật riêng của từng xã lưu trong CSDL, vd mật khẩu hộp thư (ADR 0009) — thiếu thì không lưu và không đọc được bí mật nào.",
		source: "Mỗi khoá: openssl rand -base64 32 (đúng 32 byte). SAO LƯU RIÊNG trước khi lưu bí mật đầu tiên — " +
			"mất khoá là mất mọi bí mật đã mã hoá bằng nó.",
		place: inSecret,
		shape: newKeyList,
	},
	"OPERATOR_SESSION_SIGNING_KEYS": {
		meaning: "Khoá ký phiên của khu vực nhà vận hành (ADR 0048) — tách hẳn khỏi SESSION_SIGNING_KEYS. identity ký, platform kiểm chữ ký.",
		source: "Mỗi khoá: openssl rand -base64 48; không trùng khoá nào của SESSION_SIGNING_KEYS (trùng thì pod từ chối). " +
			"CÙNG MỘT giá trị ở Secret của identity và của platform — khác nhau thì platform từ chối mọi phiên vận hành.",
		place: inSecret,
		shape: newKeyList,
	},
	// NOT grpcAddrHint: that one says port 9090, and pointing platform at identity:9090 is exactly
	// the port that no longer serves OperatorService.
	"IDENTITY_OPERATOR_GRPC_ADDR": {
		meaning: "Địa chỉ gRPC nội bộ của OperatorService ở identity — platform tra và mở phiên vận hành qua đây (ADR 0048); thiếu thì platform không khởi động ở staging/prod.",
		source: "Tên Service của identity trên cụm cộng cổng 9093 (cổng grpc-operator), KHÔNG phải 9090: kubectl get svc -A | grep identity. " +
			"Chỉ địa chỉ trong cụm — kênh này mang mật khẩu và mã TOTP, không mã hoá.",
		place: inDeploymentEnv,
		shape: "<tên Service của identity đúng như kubectl get svc in ra>:9093",
	},
	"OPERATOR_TOTP_ENCRYPTION_KEY": {
		meaning: "Khoá AES-256 mã hoá bí mật TOTP của tài khoản nhà vận hành (ADR 0048).",
		source:  "Mỗi khoá: openssl rand -base64 32 (đúng 32 byte).",
		place:   inSecret,
		shape:   newKeyList,
	},
	"ZALO_BOT_WEBHOOK_HOST": {
		meaning: "Tên máy (host) mà Zalo gửi tin nhắn của bot dùng chung tới — comms tự dựng địa chỉ webhook https://<host>/api/v1/zalo-bot-updates (ADR 0074 #5). " +
			"Thiếu thì người vận hành không trỏ được webhook, và không cán bộ nào ghép được Zalo.",
		source: "Tên miền đã trỏ DNS và có chứng chỉ TLS + Ingress vào service-comms: bot.api.vigov.vn ở prod (ADR 0074); môi trường khác hỏi người giữ DNS. " +
			"Chỉ tên máy: chữ thường, không https://, không cổng, không đường dẫn, không bao giờ là tên miền của một xã.",
		place: inDeploymentEnv,
		shape: "bot.api.vigov.vn",
	},
	"RABBITMQ_DSN": {
		meaning: "Kết nối RabbitMQ cho tác vụ nền có hẹn giờ (ADR 0010) — không dùng cho sự kiện giữa các dịch vụ.",
		source:  "Người vận hành RabbitMQ cấp user, mật khẩu và vhost.",
		place:   inSecret,
		shape:   "amqps://<user>:<mật khẩu>@<host>:5671/<vhost>",
	},
	"RABBITMQ_EXCHANGE": {
		meaning: "Tên exchange mà tác vụ nền gửi vào.",
		source:  "CHƯA được quyết (ADR 0010 chưa đặt tên) — hỏi người nối tác vụ nền đầu tiên, đừng tự đặt.",
		place:   inConfigMap,
		shape:   "<tên exchange đã quyết>",
	},
	"ELASTICSEARCH_ADDRS": {
		meaning: "Các nút Elasticsearch cho tìm kiếm toàn hệ thống (ADR 0010) — không bao giờ là nguồn số liệu báo cáo.",
		source:  "Người vận hành Elasticsearch; https.",
		place:   inConfigMap,
		shape:   "https://<nút 1>:9200,https://<nút 2>:9200",
	},
	"ELASTICSEARCH_API_KEY": {
		meaning: "API key xác thực tới Elasticsearch — index chứa nội dung phản ánh là dữ liệu cá nhân.",
		source:  "Tạo trong Elasticsearch (POST /_security/api_key), chỉ quyền trên index có tiền tố của môi trường này.",
		place:   inSecret,
		shape:   "<api key do Elasticsearch cấp>",
	},
	"ELASTICSEARCH_INDEX_PREFIX": {
		meaning: "Tiền tố tên index của môi trường này.",
		source:  "Theo môi trường.",
		place:   inConfigMap,
		shape:   "vigov-prod (staging: vigov-stg)",
	},
}

// missingHints renders, for each missing variable, its name and its hint — the text appended to
// ErrThieuBienMoiTruong. Names only, never values (see the top of this file).
func missingHints(service string, names []string) string {
	var b strings.Builder
	for _, name := range names {
		b.WriteString("\n\n" + name)
		h, ok := envHints[name]
		if !ok {
			// Unreachable while TestEveryRefusableVariableHasAHint is green; said out loud if not.
			b.WriteString("\n  (chưa có hướng dẫn — xem deploy/cau-hinh/README.md)")
			continue
		}
		b.WriteString("\n  Là gì:        " + h.meaning)
		b.WriteString("\n  Lấy giá trị:  " + h.source)
		b.WriteString("\n  Đặt ở:        " + h.place.describe(service))
		b.WriteString("\n  Dạng:         " + h.shape)
	}
	fmt.Fprintf(&b, "\n\nĐặt xong: kubectl -n <namespace> rollout restart deploy/%s. Bảng đầy đủ: deploy/cau-hinh/README.md.",
		deploymentName(service))
	return b.String()
}
