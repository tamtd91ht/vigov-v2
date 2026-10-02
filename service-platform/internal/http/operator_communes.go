package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/opauth"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// CommuneReader is the registry read side (*store.OperatorRegistry).
type CommuneReader interface {
	ListCommunes(ctx context.Context, req page.Request) (page.Result[domain.Commune], error)
	Commune(ctx context.Context, id string) (domain.Commune, []domain.CommuneMiniApp, error)
	Provinces(ctx context.Context) ([]domain.Province, error)
}

// CommuneWriter is the registry write side (*store.RegistryWriter). Every method takes the TARGET
// commune from ctx (rule 1 invariant 4) and writes its audit entry in the same transaction.
type CommuneWriter interface {
	CreateCommune(ctx context.Context, in store.NewCommune, by domain.OperatorActor) (domain.Commune, error)
	AddDomain(ctx context.Context, host string, by domain.OperatorActor) error
	SetPrimaryDomain(ctx context.Context, host string, by domain.OperatorActor) (bool, error)
	CorrectName(ctx context.Context, name, reason string, by domain.OperatorActor) (bool, error)
	SetActivation(ctx context.Context, active bool, reason string, by domain.OperatorActor) (bool, []string, error)
	AttachMiniApp(ctx context.Context, appID, note string, by domain.OperatorActor) (domain.CommuneMiniApp, error)
	ReplaceMiniApp(ctx context.Context, oldAppID, newAppID, reason string, by domain.OperatorActor) (domain.CommuneMiniApp, error)
	SetMiniAppActivation(ctx context.Context, appID string, active bool, reason string, by domain.OperatorActor) (bool, error)
}

// --- shapes ---------------------------------------------------------------------------------------

// communeView is registry metadata ONLY — the five fields the user's 30/09 exception covers (ADR 0048
// §30/09 #5). Adding a field takes the unaudited list out of that exception: ask first.
type communeView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Province string   `json:"province"`
	Active   bool     `json:"active"`
	Domains  []string `json:"domains"` // primary first
}

type communePageView struct {
	Items      []communeView `json:"items"`
	NextCursor string        `json:"next_cursor"`
	HasMore    bool          `json:"has_more"`
}

type miniAppView struct {
	AppID     string    `json:"app_id"`
	Mode      string    `json:"mode"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

type communeDetailView struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Province string        `json:"province"`
	Active   bool          `json:"active"`
	Domains  []string      `json:"domains"`
	MiniApps []miniAppView `json:"mini_apps"`
}

type provinceView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type provinceListView struct {
	Items []provinceView `json:"items"`
}

type createCommuneBody struct {
	Name          string `json:"name"`
	ProvinceID    string `json:"province_id"`
	PrimaryDomain string `json:"primary_domain"`
}

type addDomainBody struct {
	Domain string `json:"domain"`
}

type primaryDomainBody struct {
	Domain string `json:"domain"`
}

type correctNameBody struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type activationBody struct {
	// A pointer so an absent field is a 400, never a silent `false` that deactivates a commune.
	Active *bool  `json:"active"`
	Reason string `json:"reason"`
}

type attachMiniAppBody struct {
	AppID string `json:"app_id"`
	Note  string `json:"note"`
}

// miniAppReplacementBody — the OLD App ID is the path's, never the body's: the operator confirms the
// row they saw on the screen, and the store checks it is this commune's running app.
type miniAppReplacementBody struct {
	NewAppID string `json:"new_app_id"`
	Reason   string `json:"reason"`
}

type miniAppActivationBody struct {
	// A pointer so an absent field is a 400, never a silent `false` that detaches a commune's app.
	Active *bool  `json:"active"`
	Reason string `json:"reason"`
}

func toCommuneView(c domain.Commune) communeView {
	d := c.Domains
	if d == nil {
		d = []string{}
	}
	return communeView{ID: c.ID, Name: c.Name, Province: c.Province, Active: c.Active, Domains: d}
}

func toMiniAppView(a domain.CommuneMiniApp) miniAppView {
	return miniAppView{AppID: a.AppID, Mode: string(a.Mode), Active: a.Active,
		CreatedAt: a.CreatedAt.UTC(), CreatedBy: a.CreatedBy}
}

func toDetailView(c domain.Commune, apps []domain.CommuneMiniApp) communeDetailView {
	v := toCommuneView(c)
	out := communeDetailView{ID: v.ID, Name: v.Name, Province: v.Province, Active: v.Active,
		Domains: v.Domains, MiniApps: []miniAppView{}}
	for _, a := range apps {
		out.MiniApps = append(out.MiniApps, toMiniAppView(a))
	}
	return out
}

// --- error mapping ----------------------------------------------------------------------------------

const (
	msgInternal        = "Đã xảy ra lỗi. Vui lòng thử lại."
	msgCommuneNotFound = "Không tìm thấy xã."
)

// writeRegistryError maps a store/domain error to the one error shape. Unknown errors are 500 and
// are logged by the caller with the commune id only (no body, no actor, rule 3).
func (h *operatorHandlers) writeRegistryError(w http.ResponseWriter, r *http.Request, err error) {
	type m struct {
		status     int
		code, text string
	}
	for target, v := range map[error]m{
		store.ErrCommuneNotFound:    {http.StatusNotFound, "commune_not_found", msgCommuneNotFound},
		store.ErrProvinceNotFound:   {http.StatusUnprocessableEntity, "unknown_province", "Tỉnh không có trong danh mục."},
		store.ErrDomainTaken:        {http.StatusConflict, "domain_taken", "Tên miền đã được gắn cho một xã."},
		store.ErrDuplicateName:      {http.StatusConflict, "duplicate_name", "Đã có xã cùng tên trong tỉnh này."},
		store.ErrCommuneInactive:    {http.StatusConflict, "commune_inactive", "Xã đã ngừng hoạt động."},
		store.ErrDomainNotInCommune: {http.StatusUnprocessableEntity, "domain_not_in_commune", "Tên miền không thuộc xã này."},
		store.ErrMiniAppTaken:       {http.StatusConflict, "mini_app_taken", "App ID đã có trong sổ Mini App."},
		store.ErrMiniAppNotInCommune: {http.StatusNotFound, "mini_app_not_found",
			"Không tìm thấy Mini App riêng này ở xã."},
		store.ErrMiniAppInactive: {http.StatusConflict, "mini_app_inactive", "App ID này đã tắt."},
		store.ErrMiniAppAlreadyRunning: {http.StatusConflict, "mini_app_already_running",
			"Xã đã có một Mini App riêng đang chạy. Dùng thao tác đổi App ID."},
		store.ErrCommuneSucceeded: {http.StatusConflict, "commune_succeeded",
			"Xã đã được sáp nhập hoặc chia tách vào đơn vị khác, không mở lại hoạt động được."},
		domain.ErrCommuneHostInvalid:  {http.StatusUnprocessableEntity, "invalid_domain", "Tên miền không hợp lệ."},
		domain.ErrCommuneHostReserved: {http.StatusUnprocessableEntity, "reserved_domain", "Tên miền dành riêng cho nền tảng, không gắn cho xã."},
		domain.ErrNameInvalid:         {http.StatusUnprocessableEntity, "invalid_name", "Tên xã không hợp lệ."},
		domain.ErrReasonInvalid:       {http.StatusUnprocessableEntity, "invalid_reason", "Cần ghi lý do (tối đa 500 ký tự)."},
		domain.ErrMiniAppIDInvalid:    {http.StatusUnprocessableEntity, "invalid_app_id", "App ID không hợp lệ (chỉ gồm chữ số)."},
	} {
		if errors.Is(err, target) {
			httpx.WriteError(w, v.status, v.code, v.text, "")
			return
		}
	}
	h.d.Log.ErrorContext(r.Context(), "khu vận hành: ghi/đọc sổ xã thất bại", "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", msgInternal, "")
}

// actorOf is "who" for an audited write: the business code from the live principal, and the address
// the edge observed. The guard guarantees a principal; an empty code is refused by the store.
func actorOf(r *http.Request) domain.OperatorActor {
	p, _ := opauth.From(r.Context())
	return domain.OperatorActor{Code: p.OperatorCode, IP: httpx.ClientIP(r)}
}

// targetCommune puts the commune OF THE PATH into the context — the only source of the target
// (rule 1: never a body or a header). A malformed id is answered exactly like an unknown one.
func targetCommune(w http.ResponseWriter, r *http.Request) (context.Context, string, bool) {
	id := r.PathValue("id")
	if !tenant.ID(id).Valid() {
		httpx.WriteError(w, http.StatusNotFound, "commune_not_found", msgCommuneNotFound, "")
		return nil, "", false
	}
	return tenant.Into(r.Context(), tenant.ID(id)), id, true
}

// respondCommune re-reads the commune after a write, so the response is what the registry now holds.
func (h *operatorHandlers) respondCommune(w http.ResponseWriter, r *http.Request, id string, status int) {
	c, apps, err := h.d.Registry.Commune(r.Context(), id)
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	writeJSON(w, status, toDetailView(c, apps))
}

// --- reads ----------------------------------------------------------------------------------------

func (h *operatorHandlers) listCommunes(w http.ResponseWriter, r *http.Request) {
	req, err := page.Parse(r.URL.Query(), store.CommuneOrder)
	if err != nil {
		status, code, msg := page.HTTPError(err)
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	res, err := h.d.Registry.ListCommunes(r.Context(), req)
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	out := communePageView{Items: []communeView{}, NextCursor: res.NextCursor, HasMore: res.HasMore}
	for _, c := range res.Items {
		out.Items = append(out.Items, toCommuneView(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *operatorHandlers) getCommune(w http.ResponseWriter, r *http.Request) {
	_, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	h.respondCommune(w, r, id, http.StatusOK)
}

func (h *operatorHandlers) listProvinces(w http.ResponseWriter, r *http.Request) {
	ps, err := h.d.Registry.Provinces(r.Context())
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	out := provinceListView{Items: []provinceView{}}
	for _, p := range ps {
		out.Items = append(out.Items, provinceView{ID: p.ID, Name: p.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

// --- writes ---------------------------------------------------------------------------------------

func (h *operatorHandlers) createCommune(w http.ResponseWriter, r *http.Request) {
	var b createCommuneBody
	if !decodeBody(w, r, &b) {
		return
	}
	name, err := domain.ValidateCommuneName(norm.NFC.String(b.Name))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	host, err := domain.ParseCommuneHost(b.PrimaryDomain, h.d.OperatorHost)
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	// A new opaque ULID (rule 1 invariant 2), generated HERE — never accepted from the body.
	id, err := h.d.NewID()
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	ctx := tenant.Into(r.Context(), tenant.ID(id))
	if _, err := h.d.Writer.CreateCommune(ctx, store.NewCommune{Name: name, ProvinceID: b.ProvinceID, Host: host},
		actorOf(r)); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	// A negative answer for this host may be cached from before it existed.
	h.d.Forget(host)
	h.respondCommune(w, r, id, http.StatusCreated)
}

func (h *operatorHandlers) addDomain(w http.ResponseWriter, r *http.Request) {
	ctx, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	var b addDomainBody
	if !decodeBody(w, r, &b) {
		return
	}
	host, err := domain.ParseCommuneHost(b.Domain, h.d.OperatorHost)
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	if err := h.d.Writer.AddDomain(ctx, host, actorOf(r)); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	h.d.Forget(host)
	h.respondCommune(w, r, id, http.StatusCreated)
}

func (h *operatorHandlers) setPrimaryDomain(w http.ResponseWriter, r *http.Request) {
	ctx, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	var b primaryDomainBody
	if !decodeBody(w, r, &b) {
		return
	}
	// Normalised like any host input, AND checked against the reserved list BEFORE the write. "It
	// must already be one of this commune's rows" is not that check: the two frozen admin*.vigov.vn
	// rows ARE some commune's rows (rule 7 keeps them, migration 0007), and a commune row could name
	// OPERATOR_HOST if it was written before the variable was set. Promoting either to primary would
	// make GetTenant/ResolveHost hand out a platform host as the commune's address.
	host, err := domain.NormaliseHost(b.Domain)
	if err != nil {
		h.writeRegistryError(w, r, domain.ErrCommuneHostInvalid)
		return
	}
	if domain.IsReservedCommuneHost(host, h.d.OperatorHost) {
		h.writeRegistryError(w, r, domain.ErrCommuneHostReserved)
		return
	}
	if _, err := h.d.Writer.SetPrimaryDomain(ctx, host, actorOf(r)); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	h.respondCommune(w, r, id, http.StatusOK)
}

func (h *operatorHandlers) correctName(w http.ResponseWriter, r *http.Request) {
	ctx, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	var b correctNameBody
	if !decodeBody(w, r, &b) {
		return
	}
	name, err := domain.ValidateCommuneName(norm.NFC.String(b.Name))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	if _, err := h.d.Writer.CorrectName(ctx, name, reason, actorOf(r)); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	h.respondCommune(w, r, id, http.StatusOK)
}

// setActivation switches a commune on or off. Deactivating stops its hosts resolving: at once in
// this process (the hosts are forgotten below), and within one TENANT_CACHE_TTL — 30 s by default —
// in every other service's cached directory. Nothing is deleted (rule 7 invariant 6).
func (h *operatorHandlers) setActivation(w http.ResponseWriter, r *http.Request) {
	ctx, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	var b activationBody
	if !decodeBody(w, r, &b) {
		return
	}
	if b.Active == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	_, hosts, err := h.d.Writer.SetActivation(ctx, *b.Active, reason, actorOf(r))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	for _, host := range hosts {
		h.d.Forget(host)
	}
	h.respondCommune(w, r, id, http.StatusOK)
}

func (h *operatorHandlers) attachMiniApp(w http.ResponseWriter, r *http.Request) {
	ctx, _, ok := targetCommune(w, r)
	if !ok {
		return
	}
	var b attachMiniAppBody
	if !decodeBody(w, r, &b) {
		return
	}
	if err := domain.ValidateMiniAppID(b.AppID); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	note := ""
	if b.Note != "" {
		n, err := domain.ValidateReason(norm.NFC.String(b.Note))
		if err != nil {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_note", "Ghi chú không hợp lệ (tối đa 500 ký tự).", "")
			return
		}
		note = n
	}
	app, err := h.d.Writer.AttachMiniApp(ctx, b.AppID, note, actorOf(r))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toMiniAppView(app))
}

// pathMiniApp is the App ID of the path. A malformed one is answered exactly like one this commune
// does not hold — the store's ErrMiniAppNotInCommune.
func (h *operatorHandlers) pathMiniApp(w http.ResponseWriter, r *http.Request) (string, bool) {
	appID := r.PathValue("app_id")
	if domain.ValidateMiniAppID(appID) != nil {
		h.writeRegistryError(w, r, store.ErrMiniAppNotInCommune)
		return "", false
	}
	return appID, true
}

// replaceMiniApp changes the commune's dedicated App ID (ADR 0070 #1): new row on, old row off, one
// transaction. Answers the commune as it now stands, so the console shows both rows.
func (h *operatorHandlers) replaceMiniApp(w http.ResponseWriter, r *http.Request) {
	ctx, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	oldAppID, ok := h.pathMiniApp(w, r)
	if !ok {
		return
	}
	var b miniAppReplacementBody
	if !decodeBody(w, r, &b) {
		return
	}
	if err := domain.ValidateMiniAppID(b.NewAppID); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	if _, err := h.d.Writer.ReplaceMiniApp(ctx, oldAppID, b.NewAppID, reason, actorOf(r)); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	h.respondCommune(w, r, id, http.StatusCreated)
}

// setMiniAppActivation detaches (active=false) or reactivates (active=true) one of the commune's
// dedicated apps (ADR 0070 #2, #3). Nothing to forget in a cache: Directory.MiniApp has none.
func (h *operatorHandlers) setMiniAppActivation(w http.ResponseWriter, r *http.Request) {
	ctx, id, ok := targetCommune(w, r)
	if !ok {
		return
	}
	appID, ok := h.pathMiniApp(w, r)
	if !ok {
		return
	}
	var b miniAppActivationBody
	if !decodeBody(w, r, &b) {
		return
	}
	if b.Active == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	if _, err := h.d.Writer.SetMiniAppActivation(ctx, appID, *b.Active, reason, actorOf(r)); err != nil {
		h.writeRegistryError(w, r, err)
		return
	}
	h.respondCommune(w, r, id, http.StatusOK)
}
