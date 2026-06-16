package caldav

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"golang.org/x/net/webdav"
)

type Resource struct {
	Path    string
	Content []byte
}

type ResourceStore interface {
	ListResources(ctx context.Context, userKey string) ([]Resource, error)
	GetResource(ctx context.Context, userKey, resourcePath string) (Resource, bool, error)
	PutResource(ctx context.Context, userKey, resourcePath string, content []byte) error
	DeleteResource(ctx context.Context, userKey, resourcePath string) error
}

func NewHandler(store ResourceStore, userKey string) (http.Handler, error) {
	if store == nil {
		return nil, fmt.Errorf("resource store is required")
	}

	if userKey == "" {
		userKey = "default"
	}

	h := &persistingHandler{
		store:          store,
		defaultUserKey: userKey,
		handlers:       map[string]http.Handler{},
	}

	// Warm up the default user handler so startup fails fast if store hydration fails.
	if _, err := h.getUserHandler(userKey); err != nil {
		return nil, err
	}

	return h, nil
}

type persistingHandler struct {
	store          ResourceStore
	defaultUserKey string

	mu       sync.Mutex
	handlers map[string]http.Handler
}

func (h *persistingHandler) getUserHandler(userKey string) (http.Handler, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.handlers == nil {
		h.handlers = make(map[string]http.Handler)
	}

	if existing, ok := h.handlers[userKey]; ok {
		return existing, nil
	}

	fs := webdav.NewMemFS()
	resources, err := h.store.ListResources(context.Background(), userKey)
	if err != nil {
		return nil, fmt.Errorf("list caldav resources: %w", err)
	}

	for _, resource := range resources {
		if err := writeResourceToFS(fs, resource.Path, resource.Content); err != nil {
			return nil, fmt.Errorf("hydrate resource %q: %w", resource.Path, err)
		}
	}

	handler := &webdav.Handler{
		Prefix:     "/caldav",
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
	}

	h.handlers[userKey] = handler
	return handler, nil
}

func (h *persistingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userKey, resourcePath := normalizeResourcePath(r.URL.Path, h.defaultUserKey)

	userHandler, err := h.getUserHandler(userKey)
	if err != nil {
		http.Error(w, "failed to load caldav resources", http.StatusInternalServerError)
		return
	}

	rewritten := cloneRequestWithPath(r, toHandlerPath(resourcePath))
	if resourcePath == "" {
		userHandler.ServeHTTP(w, rewritten)
		return
	}

	currentResource, found, lookupErr := h.store.GetResource(rewritten.Context(), userKey, resourcePath)
	if lookupErr != nil {
		http.Error(w, "failed to lookup caldav resource", http.StatusInternalServerError)
		return
	}

	if rewritten.Method == http.MethodGet || rewritten.Method == http.MethodHead {
		if found {
			w.Header().Set("ETag", resourceETag(currentResource.Content))
		}
	}

	ifMatch := rewritten.Header.Get("If-Match")
	if ifMatch != "" && (rewritten.Method == http.MethodPut || rewritten.Method == http.MethodDelete) {
		if !ifMatchSatisfied(ifMatch, found, currentResource.Content) {
			http.Error(w, "precondition failed", http.StatusPreconditionFailed)
			return
		}
	}

	var requestBody []byte
	responseETag := ""
	if found && (rewritten.Method == http.MethodGet || rewritten.Method == http.MethodHead) {
		responseETag = resourceETag(currentResource.Content)
	}

	if rewritten.Method == http.MethodPut {
		requestBody, _ = io.ReadAll(rewritten.Body)
		rewritten.Body = io.NopCloser(bytes.NewReader(requestBody))
		responseETag = resourceETag(requestBody)
	}

	recorder := &statusRecorder{ResponseWriter: w, etagOverride: responseETag}
	userHandler.ServeHTTP(recorder, rewritten)
	if recorder.statusCode == 0 {
		recorder.statusCode = http.StatusOK
	}

	if recorder.statusCode < http.StatusOK || recorder.statusCode >= http.StatusMultipleChoices {
		return
	}

	switch rewritten.Method {
	case http.MethodPut:
		_ = h.store.PutResource(rewritten.Context(), userKey, resourcePath, requestBody)
	case http.MethodDelete:
		_ = h.store.DeleteResource(rewritten.Context(), userKey, resourcePath)
	}
}

func resourceETag(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("\"%x\"", sum)
}

func ifMatchSatisfied(ifMatch string, found bool, content []byte) bool {
	if ifMatch == "*" {
		return found
	}

	if !found {
		return false
	}

	current := resourceETag(content)
	for _, part := range strings.Split(ifMatch, ",") {
		candidate := strings.TrimSpace(part)
		if candidate == current {
			return true
		}
	}

	return false
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

func normalizeResourcePath(requestPath, defaultUserKey string) (string, string) {
	cleaned := path.Clean(requestPath)
	prefix := "/caldav"
	if cleaned == prefix || cleaned == prefix+"/" || !strings.HasPrefix(cleaned, prefix) {
		return defaultUserKey, ""
	}

	trimmed := strings.TrimPrefix(cleaned, prefix)
	if trimmed == "" || trimmed == "/" {
		return defaultUserKey, ""
	}

	trimmed = strings.TrimPrefix(trimmed, "/")
	segments := strings.Split(trimmed, "/")

	if len(segments) == 1 {
		// Backward compatibility for /caldav/file.ics style paths.
		if strings.Contains(segments[0], ".") {
			return defaultUserKey, segments[0]
		}
		// /caldav/{user}
		return segments[0], ""
	}

	userKey := segments[0]
	resourcePath := strings.Join(segments[1:], "/")
	return userKey, resourcePath
}

func toHandlerPath(resourcePath string) string {
	if resourcePath == "" {
		return "/caldav"
	}

	return "/caldav/" + strings.TrimPrefix(resourcePath, "/")
}

func cloneRequestWithPath(r *http.Request, newPath string) *http.Request {
	clone := r.Clone(r.Context())
	clone.URL.Path = newPath
	clone.RequestURI = newPath
	return clone
}

func writeResourceToFS(fs webdav.FileSystem, resourcePath string, content []byte) error {
	resourcePath = path.Clean("/" + resourcePath)
	if resourcePath == "/" {
		return nil
	}

	if err := mkdirAll(fs, path.Dir(resourcePath)); err != nil {
		return err
	}

	file, err := fs.OpenFile(context.Background(), resourcePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(content)
	return err
}

func mkdirAll(fs webdav.FileSystem, dirPath string) error {
	if dirPath == "/" || dirPath == "." || dirPath == "" {
		return nil
	}

	parts := strings.Split(strings.TrimPrefix(path.Clean(dirPath), "/"), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}

		current += "/" + part
		err := fs.Mkdir(context.Background(), current, 0o755)
		if err != nil && !os.IsExist(err) {
			return err
		}
	}

	return nil
}
