package main

import "net/http"

func (app *application) errorResponse(w http.ResponseWriter, status int, message string) {
	app.writeJSON(w, status, envelope{"error": message})
}

func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Error("server error", "method", r.Method, "uri", r.URL.RequestURI(), "error", err)
	app.errorResponse(w, http.StatusInternalServerError, "the server encountered a problem")
}
