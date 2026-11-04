package automation

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// This is an internal control API. The bearer credential authorizes the trusted
// integration/gateway for all workspaces; it is not a browser-user credential.
func NewHTTPHandler(service *Service, token string, extensions ...func(*http.ServeMux)) http.Handler {
	mux := http.NewServeMux()
	respond := func(w http.ResponseWriter, r *http.Request, err error) {
		status := http.StatusInternalServerError
		switch {
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
	mux.HandleFunc("PUT /v1/workspaces/{tenant}/accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Platform string `json:"platform"`
		}
		if !decode(w, r, &body) {
			return
		}
		if err := service.RegisterAccount(r.Context(), r.PathValue("tenant"), r.PathValue("id"), body.Platform); err != nil {
			respond(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	base := "/v1/workspaces/{tenant}/automations/{id}"
	mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) {
		var d Document
		if !decode(w, r, &d) {
			return
		}
		if err := service.Save(r.Context(), r.PathValue("tenant"), r.PathValue("id"), d); err != nil {
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
		action       func(*http.Request) error
	}{
		{"/publish", "POST", func(r *http.Request) error {
			return service.Publish(r.Context(), r.PathValue("tenant"), r.PathValue("id"))
		}},
		{"/disable", "POST", func(r *http.Request) error {
			return service.Disable(r.Context(), r.PathValue("tenant"), r.PathValue("id"))
		}},
		{"", "DELETE", func(r *http.Request) error {
			return service.Delete(r.Context(), r.PathValue("tenant"), r.PathValue("id"))
		}},
	} {
		mux.HandleFunc(operation.method+" "+base+operation.path, func(w http.ResponseWriter, r *http.Request) {
			if err := operation.action(r); err != nil {
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
