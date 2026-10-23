package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
	"github.com/pencelheimer/go-compose-assembler/internal/export"
)

const maxRequestBody = 1 << 20

type handlers struct {
	d   Deps
	hub *broadcaster // nil when live updates are off
}

// listResponse is a best-effort catalog listing: every valid entry plus a
// description of each file that was skipped.
type listResponse[T any] struct {
	Items   []T      `json:"items"`
	Skipped []string `json:"skipped"`
}

func newListResponse[T any](items []T, err error) listResponse[T] {
	resp := listResponse[T]{Items: items, Skipped: []string{}}
	if resp.Items == nil {
		resp.Items = []T{}
	}
	for _, e := range flatten(err) {
		resp.Skipped = append(resp.Skipped, e.Error())
	}
	return resp
}

func (h *handlers) listComponents(w http.ResponseWriter, r *http.Request) {
	items, err := h.d.Components.ListComponents(r.Context())
	writeJSON(w, http.StatusOK, newListResponse(items, err))
}

func (h *handlers) listPresets(w http.ResponseWriter, r *http.Request) {
	items, err := h.d.Presets.ListPresets(r.Context())
	writeJSON(w, http.StatusOK, newListResponse(items, err))
}

func (h *handlers) getComponent(w http.ResponseWriter, r *http.Request) {
	def, err := h.d.Components.GetComponent(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, def)
}

func (h *handlers) getPreset(w http.ResponseWriter, r *http.Request) {
	p, err := h.d.Presets.GetPreset(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *handlers) generate(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	dec.DisallowUnknownFields()
	var req domain.GenerateRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, errs.InvalidRequestError{Reason: fmt.Sprintf("malformed JSON body: %v", err)})
		return
	}

	res, err := h.d.Generator.Generate(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	compose, env, err := h.d.Exporter.Export(res)
	if err != nil {
		writeError(w, err)
		return
	}
	warnings := res.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	writeJSON(w, http.StatusOK, export.Bundle{Compose: string(compose), Env: string(env), Warnings: warnings})
}

func (h *handlers) apiNotFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, apiError{
		Kind:    kindNotFound,
		Message: fmt.Sprintf("no API route for %s %s", r.Method, r.URL.Path),
		Details: []errorDetail{},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // the client went away; nothing left to do
}
