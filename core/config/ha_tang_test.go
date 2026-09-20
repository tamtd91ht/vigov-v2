package config

// The infrastructure dependencies read from configuration and nowhere else: PostgreSQL, Redis,
// RabbitMQ, Elasticsearch (ADR 0010).
//
// A separate file rather than an addition to config_test.go, same as grpc_addr_test.go: the two
// do not collide while both are being worked on. The helper datMoiTruong, dsnGia, redisGia and
// khoaGia come from config_test.go — same package.
//
// TWO QUESTIONS THIS FILE EXISTS TO KEEP ANSWERED:
//
//  1. A REQUIRED variable that is missing is refused BY NAME. "Failed to start" sends the
//     operator to the logs of the wrong layer; "thiếu DATABASE_DSN" sends them to the manifest.
//  2. An OPTIONAL variable that is absent does NOT stop a service starting. Nothing here uses
//     RabbitMQ or Elasticsearch yet, and a required variable protecting code that does not
//     exist would stop all eight services on every machine that has not set four more values.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// Fake credentials. The text says so, so a scanner and a reviewer can both tell at a glance
// that this is not a real value (rule 8, forbidden #1).
const (
	amqpGia   = "amqp://vigov:khong-phai-mat-khau-that@rabbit.noi-bo:5672/vigov"
	esKhoaGia = "khoa-api-elastic-GIA-KHONG-PHAI-KHOA-THAT"
)

// nenChay is the smallest environment in which Load succeeds AND CanhBao is empty, so a test
// about an infrastructure variable fails for the reason it is about and not for a missing
// baseline. Two signing keys and a Redis DSN on purpose: one key in prod, or no cache outside
// dev, each produce a warning of their own, and a test asserting "no warning" would then be
// asserting something it is not about.
func nenChay(env string) map[string]string {
	return map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  env,
		"SESSION_SIGNING_KEYS": khoaGia + "-moi," + khoaGia + "-cu",
		"REDIS_DSN":            redisGia,
	}
}

// --- optional: absent must not stop a service starting -----------------------------------

func TestHaTangTuyChonVangMatVanKhoiDongDuoc(t *testing.T) {
	// THE TRAP THIS TEST GUARDS. Making RABBITMQ_DSN or ELASTICSEARCH_ADDRS required at Load
	// would stop all eight services on any machine that has not set them — to protect code that
	// does not exist yet, because nothing in this repository connects to either. Required
	// belongs to what is actually used: DATABASE_DSN, opened in every service's first thirty
	// lines. These are refused BY NAME at connect time instead, the IDENTITY_GRPC_ADDR pattern.
	for _, env := range []string{EnvDev, EnvStaging, EnvProd} {
		t.Run(env, func(t *testing.T) {
			datMoiTruong(t, nenChay(env))
			for _, bien := range []string{
				"RABBITMQ_DSN", "RABBITMQ_EXCHANGE",
				"ELASTICSEARCH_ADDRS", "ELASTICSEARCH_API_KEY", "ELASTICSEARCH_INDEX_PREFIX",
			} {
				t.Setenv(bien, "")
			}

			cfg, err := Load("petitions")
			if err != nil {
				t.Fatalf("thiếu hạ tầng chưa ai dùng mà chặn khởi động: %v", err)
			}
			if cfg.RabbitMQDSN != "" || cfg.RabbitMQExchange != "" {
				t.Errorf("RabbitMQ phải rỗng chứ không đoán: %q %q", cfg.RabbitMQDSN, cfg.RabbitMQExchange)
			}
			if len(cfg.ElasticsearchAddrs) != 0 || !cfg.ElasticsearchAPIKey.Rong() ||
				cfg.ElasticsearchIndexPrefix != "" {
				t.Errorf("Elasticsearch phải rỗng chứ không đoán: %v", cfg.ElasticsearchAddrs)
			}
			// AND IT MUST BE SILENT. A deployment that uses neither is the deployment ADR 0010
			// expects today; warning about it at every startup of eight services teaches
			// operators to scroll past CanhBao, which costs the DANGEROUS_AUTH_BYPASS line its
			// only reader.
			if len(cfg.CanhBao()) != 0 {
				t.Errorf("không dùng RabbitMQ/Elastic thì không được cảnh báo: %v", cfg.CanhBao())
			}
		})
	}
}

func TestHaTangDocTuMoiTruong(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvProd,
		"SESSION_SIGNING_KEYS": khoaGia,
		"REDIS_DSN":            redisGia,
		"RABBITMQ_DSN":         "  " + amqpGia + "\n",
		"RABBITMQ_EXCHANGE":    " vigov.tac-vu \n",
		// Trailing comma and stray spaces: what a hand-edited ConfigMap actually contains.
		"ELASTICSEARCH_ADDRS":        " http://es-1.noi-bo:9200 , http://es-2.noi-bo:9200 ,,",
		"ELASTICSEARCH_API_KEY":      "  " + esKhoaGia + "\n",
		"ELASTICSEARCH_INDEX_PREFIX": " vigov \n",
	})

	cfg, err := Load("reporting")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	// Trimmed everywhere. A trailing newline pasted out of a k8s Secret produces a connection
	// error naming the wrong cause — exactly what cost an afternoon on GRPC_CALLER_KEY.
	if cfg.RabbitMQDSN.Lo() != amqpGia {
		t.Errorf("RabbitMQDSN đọc sai: %q", cfg.RabbitMQDSN.Lo())
	}
	if cfg.RabbitMQExchange != "vigov.tac-vu" {
		t.Errorf("RabbitMQExchange = %q", cfg.RabbitMQExchange)
	}
	if string(cfg.ElasticsearchAPIKey.Lo()) != esKhoaGia {
		t.Error("ElasticsearchAPIKey đọc sai — client sẽ bị cụm tìm kiếm từ chối")
	}
	if cfg.ElasticsearchIndexPrefix != "vigov" {
		t.Errorf("ElasticsearchIndexPrefix = %q", cfg.ElasticsearchIndexPrefix)
	}
	muon := []string{"http://es-1.noi-bo:9200", "http://es-2.noi-bo:9200"}
	if len(cfg.ElasticsearchAddrs) != len(muon) {
		t.Fatalf("đọc %d địa chỉ, muốn %d: %q", len(cfg.ElasticsearchAddrs), len(muon), cfg.ElasticsearchAddrs)
	}
	for i := range muon {
		if cfg.ElasticsearchAddrs[i] != muon[i] {
			t.Errorf("địa chỉ %d = %q, muốn %q", i, cfg.ElasticsearchAddrs[i], muon[i])
		}
	}
}

// --- required: missing must be refused BY NAME -------------------------------------------

func TestBienBatBuocThieuThiTuChoiDungTen(t *testing.T) {
	// ErrThieuBienMoiTruong alone is not enough, and the difference is an afternoon: an operator
	// reading "thiếu biến môi trường bắt buộc" goes looking at eight manifests, one reading
	// "thiếu DATABASE_DSN" opens one key. The existing tests assert the name for
	// SESSION_SIGNING_KEYS and GRPC_CALLER_KEY; DATABASE_DSN and ENV had no such test.
	cases := []struct {
		bien string
		moi  map[string]string
	}{
		{"DATABASE_DSN", map[string]string{"ENV": EnvDev, "DATABASE_DSN": ""}},
		{"ENV", map[string]string{"DATABASE_DSN": dsnGia, "ENV": ""}},
		{"GRPC_CALLER_KEY", map[string]string{
			"DATABASE_DSN": dsnGia, "ENV": EnvDev, "GRPC_CALLER_KEY": ""}},
	}
	for _, c := range cases {
		t.Run(c.bien, func(t *testing.T) {
			datMoiTruong(t, c.moi)

			_, err := Load("documents")
			if !errors.Is(err, ErrThieuBienMoiTruong) {
				t.Fatalf("muốn ErrThieuBienMoiTruong, nhận %v", err)
			}
			if !strings.Contains(err.Error(), c.bien) {
				t.Errorf("thông báo phải gọi đúng tên biến %s: %v", c.bien, err)
			}
			// And it must name the service, or an operator watching eight pods restart cannot
			// tell which one is misconfigured.
			if !strings.Contains(err.Error(), "documents") {
				t.Errorf("thông báo phải nói service nào: %v", err)
			}
		})
	}
}

// --- credentials must not reach a log line ------------------------------------------------

func TestRabbitMQDSNKhongLoMatKhau(t *testing.T) {
	// One `slog.Info("boot", "cfg", cfg)` written while debugging sends the broker password into
	// centralised logging, backups and a third-party monitoring vendor at once, from where it
	// cannot be recalled (rule 8, invariant 1). The type has to refuse, not the call site.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvProd,
		"SESSION_SIGNING_KEYS": khoaGia,
		"RABBITMQ_DSN":         amqpGia,
	})

	cfg, err := Load("comms")
	if err != nil {
		t.Fatal(err)
	}

	tho, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	log.Info("khởi động", "cfg", cfg)
	log.Info("chỉ broker", "dsn", cfg.RabbitMQDSN)
	log.With("cfg", cfg.Redacted()).Info("kèm sẵn")

	for _, ra := range []string{
		fmt.Sprintf("%v", cfg.RabbitMQDSN),
		fmt.Sprintf("%s", cfg.RabbitMQDSN),
		fmt.Sprintf("%q", cfg.RabbitMQDSN),
		fmt.Sprintf("%#v", cfg.RabbitMQDSN),
		fmt.Sprintf("%+v", cfg),
		fmt.Sprintf("%v", cfg.Redacted()),
		string(tho),
		buf.String(),
	} {
		if strings.Contains(ra, "khong-phai-mat-khau-that") {
			t.Errorf("mật khẩu broker lọt ra: %q", ra)
		}
	}
	// AND THE OTHER HALF: a DSN redacted down to *** makes a service pointed at the wrong broker
	// by a bad deploy undiagnosable from its own startup line. The host has to survive.
	if !strings.Contains(buf.String(), "rabbit.noi-bo:5672") {
		t.Errorf("che quá tay — không đọc ra được đang nối tới broker nào:\n%s", buf.String())
	}
	// ...and the material must still arrive intact at whoever dials.
	if cfg.RabbitMQDSN.Lo() != amqpGia {
		t.Error("Lo() không trả về DSN thật — sẽ không nối được broker")
	}
}

func TestElasticsearchAPIKeyKhongHienRaKhiGhiLog(t *testing.T) {
	// An API key is key material, not a URL: there is no host inside worth keeping readable, so
	// every rendering path must collapse it to *** — including the NUMERIC verbs, which fmt
	// never routes through String(). secret.Secret.Format is what closes those.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":          dsnGia,
		"ENV":                   EnvProd,
		"SESSION_SIGNING_KEYS":  khoaGia,
		"ELASTICSEARCH_ADDRS":   "http://es-1.noi-bo:9200",
		"ELASTICSEARCH_API_KEY": esKhoaGia,
	})

	cfg, err := Load("reporting")
	if err != nil {
		t.Fatal(err)
	}

	tho, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	log.Info("khởi động", "cfg", cfg)
	log.Info("chỉ khoá", "k", cfg.ElasticsearchAPIKey)
	log.With("cfg", cfg.Redacted()).Info("kèm sẵn")

	ra := []string{
		fmt.Sprintf("%v", cfg.ElasticsearchAPIKey),
		fmt.Sprintf("%s", cfg.ElasticsearchAPIKey),
		fmt.Sprintf("%q", cfg.ElasticsearchAPIKey),
		fmt.Sprintf("%#v", cfg.ElasticsearchAPIKey),
		fmt.Sprintf("%d", cfg.ElasticsearchAPIKey),
		fmt.Sprintf("%c", cfg.ElasticsearchAPIKey),
		fmt.Sprintf("%+v", cfg),
		fmt.Sprintf("%v", cfg.Redacted()),
		string(tho),
		buf.String(),
	}
	for _, r := range ra {
		if strings.Contains(r, esKhoaGia) {
			t.Errorf("khoá API Elastic lọt ra nguyên văn: %q", r)
		}
		// A key printed as bytes is still the key: a grep of the log pipeline would miss it,
		// a person reading the line would not.
		if strings.Contains(r, fmt.Sprintf("%d %d", esKhoaGia[0], esKhoaGia[1])) {
			t.Errorf("khoá API Elastic lọt ra dưới dạng byte: %q", r)
		}
	}
	// ...but it must still be reachable for whoever builds the client.
	if string(cfg.ElasticsearchAPIKey.Lo()) != esKhoaGia {
		t.Error("Lo() không trả về khoá thật — cụm tìm kiếm sẽ từ chối")
	}
}

// --- half-configured is the state nobody means to be in -----------------------------------

func TestCanhBaoKhiHaTangCauHinhNuaVoi(t *testing.T) {
	// The usual cause is a typo'd key in a ConfigMap or Secret: the intended variable stays
	// empty while a plausible-looking one is present, and nothing fails — the queue simply never
	// receives anything. A named warning at startup is what turns that into a five-minute fix.
	cases := []struct {
		ten   string
		moi   map[string]string
		phaiC string
	}{
		{
			ten:   "exchange có nhưng DSN trống",
			moi:   map[string]string{"RABBITMQ_EXCHANGE": "vigov.tac-vu", "RABBITMQ_DSN": ""},
			phaiC: "RABBITMQ_DSN",
		},
		{
			ten:   "khoá API có nhưng địa chỉ trống",
			moi:   map[string]string{"ELASTICSEARCH_API_KEY": esKhoaGia, "ELASTICSEARCH_ADDRS": ""},
			phaiC: "ELASTICSEARCH_ADDRS",
		},
		{
			// An index holding petition contents is an index holding citizen personal data
			// (rule 3). Not refused — a closed network is a legitimate choice — but said out
			// loud so somebody owns it.
			ten:   "địa chỉ có nhưng không xác thực",
			moi:   map[string]string{"ELASTICSEARCH_ADDRS": "http://es-1.noi-bo:9200", "ELASTICSEARCH_API_KEY": ""},
			phaiC: "ELASTICSEARCH_API_KEY",
		},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			datMoiTruong(t, nenChay(EnvProd))
			datMoiTruong(t, c.moi)

			cfg, err := Load("petitions")
			if err != nil {
				t.Fatalf("cấu hình nửa vời phải CẢNH BÁO chứ không chặn khởi động: %v", err)
			}
			canh := strings.Join(cfg.CanhBao(), " | ")
			if !strings.Contains(canh, c.phaiC) {
				t.Errorf("cảnh báo phải gọi đúng tên biến %s, nhận: %q", c.phaiC, canh)
			}
		})
	}
}

func TestElasticKhongXacThucODevThiKhongCanhBao(t *testing.T) {
	// A local single-node Elastic has no credential to configure, and a warning nobody can act
	// on is a warning that trains people to ignore the list.
	datMoiTruong(t, nenChay(EnvDev))
	datMoiTruong(t, map[string]string{
		"ELASTICSEARCH_ADDRS":   "http://localhost:9200",
		"ELASTICSEARCH_API_KEY": "",
	})

	cfg, err := Load("reporting")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CanhBao()) != 0 {
		t.Errorf("ở dev không cần cảnh báo Elastic mở: %v", cfg.CanhBao())
	}
}
