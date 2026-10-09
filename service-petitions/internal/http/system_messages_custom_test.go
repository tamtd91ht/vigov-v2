package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ADR 0079 Q2 on the wire (migration 0033): the switch, the commune sentences, and the list fields. The
// rule 5 invariant 7 set for the four new routes is in system_messages_test.go (systemMessageRoutes).

func TestListSystemMessagesCarriesOriginGroupAndSwitch(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	at := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)
	s.svc.list = []domain.SystemMessage{
		{Key: domain.KeyFeedbackNeverPublic, Group: domain.GroupFeedback, Origin: domain.OriginShipped,
			DefaultText: "Mặc định.", CurrentText: "Mặc định.", OverrideText: "Câu của xã.", Overridden: true,
			Active: false, UpdatedAt: &at, UpdatedBy: "CB-1"},
		{Key: customCode, Group: domain.GroupShared, Origin: domain.OriginCommune, CurrentText: "Xin chào.",
			Active: true, UpdatedAt: &at, UpdatedBy: "CB-2"},
	}
	w := s.call(http.MethodGet, hostA, systemMessagesPath, canBoCuaXa(xaA), "")
	doiMa(t, w, http.StatusOK)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 2 {
		t.Fatalf("body %s", w.Body.String())
	}
	off, own := out.Items[0], out.Items[1]
	for k, want := range map[string]any{"group_code": "phan-anh", "origin": "shipped", "is_active": false,
		"overridden": true, "current_text": "Mặc định.", "override_text": "Câu của xã."} {
		if off[k] != want {
			t.Errorf("switched-off %s = %v, want %v", k, off[k], want)
		}
	}
	for k, want := range map[string]any{"group_code": "chung", "origin": "commune", "is_active": true, "code": customCode} {
		if own[k] != want {
			t.Errorf("commune sentence %s = %v, want %v", k, own[k], want)
		}
	}
	if _, has := own["default_text"]; has {
		t.Error("a commune sentence carries a default_text it does not have")
	}
}

func TestSwitchPassesStateAndRequiresIt(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	doiMa(t, s.call(http.MethodPatch, hostA, overridePath(domain.KeyFeedbackNeverPublic), canBoCuaXa(xaA), `{"is_active":true}`), http.StatusOK)
	if s.svc.active == nil || !*s.svc.active || s.svc.op != "switch" {
		t.Errorf("active=%v op=%q", s.svc.active, s.svc.op)
	}
	s.svc.calls = 0
	doiMa(t, s.call(http.MethodPatch, hostA, overridePath(domain.KeyFeedbackNeverPublic), canBoCuaXa(xaA), `{}`), http.StatusBadRequest)
	if s.svc.calls != 0 {
		t.Error("a PATCH with no state reached the use case")
	}
}

func TestCreateCustomPassesFields(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	s.svc.result = domain.SystemMessage{Key: customCode, Group: "chung", Origin: domain.OriginCommune, CurrentText: "Xin chào.", Active: true}
	w := s.call(http.MethodPost, hostA, systemMessagesPath, canBoCuaXa(xaA),
		`{"group_code":"chung","code":"chung.loi-chao","text":"Xin chào.","description":"Lời chào"}`)
	doiMa(t, w, http.StatusCreated)
	c := s.svc.created
	if c.Group != "chung" || c.Key != customCode || c.Text != "Xin chào." || c.Description == nil || *c.Description != "Lời chào" {
		t.Errorf("reached use case: %+v", c)
	}
}

// PATCH `description`: absent = leave (nil change); null = clear; a string = set.
func TestEditCustomDescriptionThreeStates(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	path := systemMessagesPath + "/" + customCode
	s.call(http.MethodPatch, hostA, path, canBoCuaXa(xaA), `{"text":"x"}`)
	if s.svc.edited.Description != nil || s.svc.edited.Text == nil {
		t.Errorf("absent: %+v", s.svc.edited)
	}
	s.call(http.MethodPatch, hostA, path, canBoCuaXa(xaA), `{"description":null}`)
	if d := s.svc.edited.Description; d == nil || d.Value != nil {
		t.Errorf("null: %+v", d)
	}
	s.call(http.MethodPatch, hostA, path, canBoCuaXa(xaA), `{"description":"Mô tả","is_active":false}`)
	if d := s.svc.edited.Description; d == nil || d.Value == nil || *d.Value != "Mô tả" || s.svc.edited.Active == nil || *s.svc.edited.Active {
		t.Errorf("set: %+v", s.svc.edited)
	}
}

func TestDeleteCustomPassesReason(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	doiMa(t, s.call(http.MethodDelete, hostA, systemMessagesPath+"/"+customCode, canBoCuaXa(xaA), `{"reason":"Không dùng"}`), http.StatusNoContent)
	if s.svc.reason != "Không dùng" || s.svc.key != customCode {
		t.Errorf("reason=%q key=%q", s.svc.reason, s.svc.key)
	}
}

func TestCustomMessageRefusalsMapped(t *testing.T) {
	for name, c := range map[string]struct {
		err           error
		status        int
		code, message string
	}{
		"code taken": {domain.ErrMessageCodeTaken, http.StatusConflict, "message_code_taken", "Mã này đã được dùng cho một câu khác."},
		// The unique-key race (store.AddCustom), wrapped as it arrives from the use case.
		"code taken, race": {fmt.Errorf("system_message: thêm câu cho xã X: %w", fmt.Errorf("custom_system_message: chèn: %w", domain.ErrMessageCodeTaken)),
			http.StatusConflict, "message_code_taken", "Mã này đã được dùng cho một câu khác."},
		"shipped delete": {domain.ErrShippedMessageNotDeletable, http.StatusConflict, "system_message", "Câu đi kèm phần mềm chỉ sửa lời được, không xoá được."},
		"shipped edit":   {domain.ErrShippedMessageNotCustom, http.StatusConflict, "system_message", ""},
		"full":           {domain.ErrCustomCatalogueFull, http.StatusConflict, "catalogue_full", ""},
		"not found":      {domain.ErrCustomMessageNotFound, http.StatusNotFound, "not_found", ""},
		"bad key":        {domain.ErrCustomKeyPrefix, http.StatusBadRequest, "invalid_request", ""},
		"no reason":      {domain.ErrCustomDeleteReasonEmpty, http.StatusBadRequest, "invalid_request", ""},
	} {
		t.Run(name, func(t *testing.T) {
			s := newSystemMessagesServer(t)
			s.grant(xaA, "admin.lookup")
			s.svc.err = c.err
			w := s.call(http.MethodDelete, hostA, systemMessagesPath+"/"+customCode, canBoCuaXa(xaA), `{"reason":"x"}`)
			doiMa(t, w, c.status)
			e := loiTra(t, w)
			if e.Code != c.code || (c.message != "" && e.Message != c.message) {
				t.Errorf("error = %+v", e)
			}
		})
	}
}
