package config

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/vihat/vigov/core/secret"
)

// MINI_APP_PROFILE_CODE — the code service-platform appends as `pf=<code>` to every commune QR link it
// builds (domain.MiniAppLaunchLink / OwnMiniAppLaunchLink). The citizen Mini App hashes `pf` and, when
// the SHA-256 equals DEV_PROFILE_CODE_SHA256 in citizen-app/src/lib/runtime-profile.ts, talks to the DEV
// hosts for that one launch; no `pf` = prod (ADR 0046 §Sửa đổi 14/10/2026 #9). Owner 14/10/2026: the QR
// shown in platform-admin and web-admin must follow the same structure `npm run qr -- --profile=dev`
// prints, and those screens render exactly the URL platform returns — so the code has to reach platform.
//
// THE NAME SAYS THE ROLE, NOT "DEV" (rule 11 invariant 3, and ADR 0046 #9b: the parameter itself never
// says "dev" either). The value is the SAME code as citizen-app/.env.qr.local VIGOV_DEV_PROFILE_CODE.
//
// k8s SECRET (`platform-secrets` in vigov-dev only), secret.Secret: whoever holds it can open the app
// against the dev cluster. It is not a credential to the data — a dev QR prints it in clear — but it
// stays out of logs, manifests and chat like one (rule 8 invariant 3).
//
// NEVER REQUIRED, IN ANY ENVIRONMENT — the AdminSeed kind of switch. Empty = links carry no `pf`, which
// is the prod link. Requiring it anywhere would make the dev cluster's pod refuse to start over a QR
// parameter.
//
// SET IN STAGING OR PROD = REFUSED TO START (fail closed). In prod every QR an operator or a commune
// prints would send citizens' petitions to the dev cluster, and the link LOOKS normal — nobody reading
// it can tell. Staging is held to prod's rule, as everywhere in this package (reader.strict).
//
// THE DIGEST IS NOT CHECKED HERE. Matching DEV_PROFILE_CODE_SHA256 would mean a second copy of a value
// that lives in citizen-app (rule 9); a code that does not match makes the app open PROD, the safe
// direction. commune-qr.mjs refuses a mismatch because it can read the bundle source; this process
// cannot.

// miniAppProfileCodeShape is commune-qr.mjs DEV_PROFILE_CODE: URL-safe, long enough that the SHA-256 in
// the bundle cannot be brute-forced. The same rule on both sides, so a code one accepts the other does.
var miniAppProfileCodeShape = regexp.MustCompile(`^[A-Za-z0-9_-]{20,128}$`)

var (
	// ErrMiniAppProfileCodeInvalid is a MINI_APP_PROFILE_CODE that does not have the shape above.
	ErrMiniAppProfileCodeInvalid = errors.New("config: MINI_APP_PROFILE_CODE không hợp lệ")
	// ErrMiniAppProfileCodeInStrictEnv is a MINI_APP_PROFILE_CODE set where ENV is staging or prod.
	ErrMiniAppProfileCodeInStrictEnv = errors.New("config: MINI_APP_PROFILE_CODE chỉ được đặt ở cụm dev")
)

// parseMiniAppProfileCode validates the trimmed value. "" is off in every environment. The errors name
// the variable, what it is, where its value comes from and where it goes — NEVER the value (rule 8).
func parseMiniAppProfileCode(raw, env, service string) (secret.Secret, error) {
	if raw == "" {
		return nil, nil
	}
	if env == EnvStaging || env == EnvProd {
		return nil, fmt.Errorf("%w: đang có giá trị trong khi ENV=%s (service %s).\n"+
			"  Là gì:        mã profile dev mà platform gắn vào mọi liên kết QR Mini App (pf=<mã>); Mini App gặp mã khớp "+
			"thì gọi máy chủ DEV (ADR 0046 §Sửa đổi 14/10/2026 #9). Để ở đây, mọi QR in ra sẽ đưa phản ánh của công dân "+
			"vào môi trường dev mà nhìn liên kết không ai nhận ra.\n"+
			"  Cách sửa:     xoá key MINI_APP_PROFILE_CODE khỏi Secret `%s` của namespace này "+
			"(chỉ đặt ở vigov-dev), rồi kubectl -n <namespace> rollout restart deploy/%s.",
			ErrMiniAppProfileCodeInStrictEnv, env, service, secretName(service), deploymentName(service))
	}
	if !miniAppProfileCodeShape.MatchString(raw) {
		return nil, fmt.Errorf("%w: sai khuôn (service %s) — cần 20–128 ký tự A-Z a-z 0-9 - _.\n"+
			"  Là gì:        mã profile dev gắn vào liên kết QR Mini App (pf=<mã>), ADR 0046 §Sửa đổi 14/10/2026 #9.\n"+
			"  Lấy giá trị:  đúng mã VIGOV_DEV_PROFILE_CODE trong citizen-app/.env.qr.local (người giữ profile dev cấp); "+
			"mã khác với digest trong citizen-app/src/lib/runtime-profile.ts thì QR mở PROD.\n"+
			"  Đặt ở:        Secret `%s` của namespace vigov-dev — KHÔNG đặt ở prod. Để trống thì QR không mang pf.",
			ErrMiniAppProfileCodeInvalid, service, secretName(service))
	}
	return secret.Secret(raw), nil
}
