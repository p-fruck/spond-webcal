package caldav

import (
	"net/http"
)

func passesIfMatchPrecondition(r *http.Request, found bool, content []byte) bool {
	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		return true
	}

	if r.Method != http.MethodPut && r.Method != http.MethodDelete {
		return true
	}

	return ifMatchSatisfied(ifMatch, found, content)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	wroteHeader  bool
	etagOverride string
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	if r.etagOverride != "" {
		r.ResponseWriter.Header().Set("ETag", r.etagOverride)
	}
	r.statusCode = statusCode
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	return r.ResponseWriter.Write(p)
}

func serveWithStatusRecorder(w http.ResponseWriter, handler http.Handler, req *http.Request, etagOverride string) int {
	recorder := &statusRecorder{ResponseWriter: w, etagOverride: etagOverride}
	handler.ServeHTTP(recorder, req)
	if recorder.statusCode == 0 {
		return http.StatusOK
	}

	return recorder.statusCode
}

func isSuccessfulStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}
