package caldav

import (
	"bytes"
	"context"
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

	var requestBody []byte
	if rewritten.Method == http.MethodPut {
		requestBody, _ = io.ReadAll(rewritten.Body)
		rewritten.Body = io.NopCloser(bytes.NewReader(requestBody))
	}

	recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
	userHandler.ServeHTTP(recorder, rewritten)

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

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
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
