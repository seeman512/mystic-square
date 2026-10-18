package web

import "net/http"

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// GET is intentionally idempotent and does not clear state. The POST
		// action has already expired the cookie before redirecting here.
		h.renderPage(w, r, "logout", h.pageData("Goodbye", r, PageData{}), http.StatusOK)
	case http.MethodPost:
		h.expireSession(w)
		h.redirect(w, r, "/logout")
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}
