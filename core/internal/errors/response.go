package errors

import (
	"encoding/json"
	"net/http"
)

func Abort(w http.ResponseWriter, status int, err *ErrorResponse) {
	err.Status = status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(err)
}

func AbortBadRequest(w http.ResponseWriter, err *ErrorResponse) {
	Abort(w, http.StatusBadRequest, err)
}

func AbortUnauthorized(w http.ResponseWriter, err *ErrorResponse) {
	Abort(w, http.StatusUnauthorized, err)
}

func AbortForbidden(w http.ResponseWriter, err *ErrorResponse) {
	Abort(w, http.StatusForbidden, err)
}

func AbortNotFound(w http.ResponseWriter, err *ErrorResponse) {
	Abort(w, http.StatusNotFound, err)
}

func AbortConflict(w http.ResponseWriter, err *ErrorResponse) {
	Abort(w, http.StatusConflict, err)
}

func AbortInternalError(w http.ResponseWriter, err *ErrorResponse) {
	Abort(w, http.StatusInternalServerError, err)
}
