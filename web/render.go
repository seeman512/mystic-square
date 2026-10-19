package web

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

func (h *Handler) pageData(title string, r *http.Request, data PageData) PageData {
	data.Title = title
	data.Date = h.now().UTC().Format(dateFormat)
	if !data.Authenticated {
		// Login, register, and logout are deliberately guest pages. The only
		// pages that opt into the authenticated header do so explicitly above.
		data.FullName = ""
	}
	return data
}

func (h *Handler) renderFormError(w http.ResponseWriter, r *http.Request, page string, data PageData, status int, message string) {
	data.Errors = []string{message}
	h.renderPage(w, r, page, h.pageData(pageTitle(page), r, data), status)
}

func pageTitle(page string) string {
	if page == "register" {
		return "Register"
	}
	return "Login"
}

func (h *Handler) renderPage(w http.ResponseWriter, r *http.Request, page string, data PageData, status int) {
	name := "layout"
	if isHTMXRequest(r) && (page == "login" || page == "register") && status >= http.StatusUnprocessableEntity {
		name = page + "-form"
	}
	h.renderNamedPage(w, r, page, name, data, status)
}

func (h *Handler) renderNamedPage(w http.ResponseWriter, r *http.Request, page, name string, data PageData, status int) {
	// Both full-page and fragment responses can be returned by these routes, so
	// caches must distinguish them by HX-Request.
	w.Header().Add("Vary", "HX-Request")

	tmpl := h.pageTemplates[page]
	if tmpl == nil {
		h.logger.Error("web template is missing", "page", page)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var body bytes.Buffer
	if err := tmpl.ExecuteTemplate(&body, name, data); err != nil {
		h.logger.Error("execute web template", "page", page, "template", name, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := io.Copy(w, &body); err != nil {
		h.logger.Error("write web response", "page", page, "error", err)
	}
}

func (h *Handler) renderError(w http.ResponseWriter, r *http.Request, title, message string, status int) {
	data := h.pageData(title, r, PageData{Error: message})
	h.renderBasePage(w, r, data, status)
}

func (h *Handler) renderBasePage(w http.ResponseWriter, r *http.Request, data PageData, status int) {
	w.Header().Add("Vary", "HX-Request")
	var body bytes.Buffer
	if err := h.baseTemplates.ExecuteTemplate(&body, "layout", data); err != nil {
		h.logger.Error("execute web error template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := io.Copy(w, &body); err != nil {
		h.logger.Error("write web error response", "error", err)
	}
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, location string) {
	w.Header().Add("Vary", "HX-Request")
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func isHTMXRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func (h *Handler) methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	h.renderError(w, &http.Request{}, "Method Not Allowed", "method not allowed", http.StatusMethodNotAllowed)
}

var _ http.Handler = (*Handler)(nil)
