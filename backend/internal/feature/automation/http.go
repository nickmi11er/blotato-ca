package automation

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// This is an internal control API. The bearer credential authorizes the trusted
// integration/gateway for all workspaces; it is not a browser-user credential.
func NewHTTPHandler(service *Service, token string, extensions ...func(*http.ServeMux)) http.Handler {
	mux := http.NewServeMux()
	respond := func(w http.ResponseWriter, r *http.Request, err error) {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, ErrConflict):
			status = http.StatusConflict
		case errors.Is(err, ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, ErrInvalid):
			status = http.StatusBadRequest
		case errors.Is(err, ErrOwnership):
			status = http.StatusForbidden
		case errors.Is(err, ErrLimit):
			status = http.StatusConflict
		}
		message := err.Error()
		if status == http.StatusInternalServerError {
			slog.ErrorContext(r.Context(), "automation API failed", "error", err)
			message = "internal server error"
		}
		http.Error(w, message, status)
	}
	decode := func(w http.ResponseWriter, r *http.Request, v any) bool {
		r.Body = http.MaxBytesReader(w, r.Body, MaxDocumentBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			http.Error(w, "invalid or oversized JSON body", http.StatusBadRequest)
			return false
		}
		if dec.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "expected one JSON document", http.StatusBadRequest)
			return false
		}
		return true
	}
	expected := func(w http.ResponseWriter, r *http.Request) (*int64, bool) {
		raw := r.Header.Get("If-Match")
		if raw == "" {
			return nil, true
		}
		v, err := strconv.ParseInt(strings.Trim(raw, "\""), 10, 64)
		if err != nil || v < 0 {
			http.Error(w, "invalid If-Match", 400)
			return nil, false
		}
		return &v, true
	}
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("GET /v1/workspaces/{tenant}/automations", func(w http.ResponseWriter, r *http.Request) {
		v, err := service.List(r.Context(), r.PathValue("tenant"))
		if err != nil {
			respond(w, r, err)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /v1/workspaces/{tenant}/catalog", func(w http.ResponseWriter, r *http.Request) {
		v, err := service.Catalog(r.Context(), r.PathValue("tenant"))
		if err != nil {
			respond(w, r, err)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /v1/workspaces/{tenant}/templates", func(w http.ResponseWriter, r *http.Request) {
		v, err := service.Templates(r.Context(), r.PathValue("tenant"))
		if err != nil {
			respond(w, r, err)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("PUT /v1/workspaces/{tenant}/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		var t EmailTemplate
		if !decode(w, r, &t) {
			return
		}
		t.ID = r.PathValue("id")
		if err := service.SaveTemplate(r.Context(), r.PathValue("tenant"), t); err != nil {
			respond(w, r, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("DELETE /v1/workspaces/{tenant}/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		version, ok := expected(w, r)
		if !ok {
			return
		}
		if err := service.DeleteTemplate(r.Context(), r.PathValue("tenant"), r.PathValue("id"), version); err != nil {
			respond(w, r, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("PUT /v1/workspaces/{tenant}/accounts/{account}/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var p PostView
		if !decode(w, r, &p) {
			return
		}
		p.ID = r.PathValue("id")
		p.AccountID = r.PathValue("account")
		if err := service.SavePost(r.Context(), r.PathValue("tenant"), p); err != nil {
			respond(w, r, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("PUT /v1/workspaces/{tenant}/accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Platform  string  `json:"platform"`
			Name      *string `json:"name"`
			Handle    *string `json:"handle"`
			CanSendDM *bool   `json:"canSendDM"`
		}
		if !decode(w, r, &body) {
			return
		}
		if err := service.RegisterAccount(r.Context(), r.PathValue("tenant"), r.PathValue("id"), body.Platform); err != nil {
			respond(w, r, err)
			return
		}
		if body.Name != nil || body.Handle != nil || body.CanSendDM != nil {
			a := AccountView{ID: r.PathValue("id"), Platform: body.Platform, Name: body.Platform, Handle: r.PathValue("id"), CanSendDM: body.Platform != "youtube"}
			if body.Name != nil {
				a.Name = *body.Name
			}
			if body.Handle != nil {
				a.Handle = *body.Handle
			}
			if body.CanSendDM != nil {
				a.CanSendDM = *body.CanSendDM
			}
			if err := service.AccountDetails(r.Context(), r.PathValue("tenant"), a); err != nil {
				respond(w, r, err)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
	base := "/v1/workspaces/{tenant}/automations/{id}"
	mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) {
		var d Document
		if !decode(w, r, &d) {
			return
		}
		version, ok := expected(w, r)
		if !ok {
			return
		}
		if err := service.SaveVersion(r.Context(), r.PathValue("tenant"), r.PathValue("id"), d, version); err != nil {
			respond(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		v, err := service.Get(r.Context(), r.PathValue("tenant"), r.PathValue("id"))
		if err != nil {
			respond(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	})
	for _, operation := range []struct {
		path, method string
		action       string
	}{
		{"/publish", "POST", "publish"},
		{"/disable", "POST", "disable"},
		{"", "DELETE", "delete"},
	} {
		mux.HandleFunc(operation.method+" "+base+operation.path, func(w http.ResponseWriter, r *http.Request) {
			version, ok := expected(w, r)
			if !ok {
				return
			}
			if err := service.MutateVersion(r.Context(), r.PathValue("tenant"), r.PathValue("id"), operation.action, version); err != nil {
				respond(w, r, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
	for _, extension := range extensions {
		extension(mux)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credential := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || subtle.ConstantTimeCompare([]byte(credential), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
