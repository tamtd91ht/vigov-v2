package http

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
)

// DuongDanWebhookZalo is the path registered in the Zalo Developer Console.
//
// NOT under /api/v1: that prefix is the commune-scoped browser surface, and this endpoint has
// no commune at all (see MountWebhookZalo). A path that looks commune-scoped but is not is a
// path the next reader will mount behind the commune chain.
const DuongDanWebhookZalo = "/webhooks/zalo"

// MountWebhookZalo registers the Zalo Mini App webhook receiver.
//
// ⚠ IT MUST BE MOUNTED ON THE **OUTER** MUX, beside /healthz — never on the inner one.
//
//	The inner mux sits behind TenantMiddleware, which resolves Host -> commune and answers 404
//	for a Host that matches no commune. Zalo calls this URL from its own infrastructure, on a
//	host that belongs to the VENDOR and to no commune, so behind that chain every call Zalo
//	makes would get a 404 — and Zalo disables a webhook that keeps failing. This is the same
//	shape as the /healthz bug already recorded in cmd/server/main.go: an outside prober with no
//	commune context, mounted where a commune is mandatory.
//
// WHY THIS SERVICE OWNS IT: one Zalo App ID serves EVERY commune, and it is operated by the
// vendor, not by any commune (`kb/00-foundation/domain-boundaries.md`: platform = "Nền tảng —
// nhà cung cấp vận hành"). Putting a vendor integration in a commune-scoped service would bind
// it to whichever commune happened to be wired first — rule 2, and rule 1 by consequence.
//
// WHY IT ANSWERS 200 TO EVERYTHING, INCLUDING GET AND A MALFORMED BODY:
//
//	Zalo retries and then disables an endpoint that returns errors. Today this receiver has no
//	work to do — registering the URL is a submission requirement, not a feature — so the only
//	honest answer is "received". Returning 200 while doing nothing is a DELIBERATE no-op, not a
//	stub that forgot its body.
//
// ⚠ THIS ENDPOINT VERIFIES NOTHING. Anyone on the internet can POST to it, and that is
// acceptable for exactly as long as it keeps doing nothing and storing nothing. THE MOMENT IT
// ACTS ON A PAYLOAD it needs Zalo's signature check (a MAC over the body using the app secret,
// rule 8 — the secret belongs in the secret store, never in source). Without that check, acting
// on a payload means acting on anything a stranger sends. Whoever adds the first line of
// behaviour here adds the verification in the same change, or the endpoint becomes an
// unauthenticated write path into a government system.
//
// ⚠ THE BODY IS NEVER READ AND NEVER LOGGED. A Zalo event payload carries user identifiers, so
// one debug line printing the whole payload would put them into centralised logging, backups
// and a third-party monitoring vendor at once, from where they cannot be recalled (rule 3,
// forbidden #1). There is nothing to gain from reading a body we do not use.
func MountWebhookZalo(mux *http.ServeMux) {
	// Declared Public explicitly even though the outer mux is outside the authz chain: rule 5
	// invariant 1 admits no implicit default, and the reason is kept in the binary so nobody
	// six months from now has to guess whether the exemption was deliberate.
	mux.Handle(DuongDanWebhookZalo, authz.Public(
		"Zalo gọi từ hạ tầng của họ, không mang phiên đăng nhập nào; endpoint không đọc và không lưu gì",
	)(http.HandlerFunc(nhanWebhookZalo)))
}

// nhanWebhookZalo answers every request with 200 and an empty body.
//
// No method switch on purpose: Zalo may probe with GET when the URL is saved and POST events
// afterwards, and a 405 on the probe is a URL the Console refuses to accept.
func nhanWebhookZalo(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
