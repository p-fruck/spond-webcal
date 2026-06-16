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

	fs := webdav.NewMemFS()
	resources, err := store.ListResources(context.Background(), userKey)
	if err != nil {
		return nil, fmt.Errorf("list caldav resources: %w", err)
	}

	for _, resource := range resources {
		if err := writeResourceToFS(fs, resource.Path, resource.Content); err != nil {
			return nil, fmt.Errorf("hydrate resource %q: %w", resource.Path, err)
		}
	}

	base := &webdav.Handler{
		Prefix:     "/caldav",
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
	}

	return &persistingHandler{base: base, store: store, userKey: userKey}, nil
}

type persistingHandler struct {
	base    http.Handler
	store   ResourceStore
	userKey string
}

func (h *persistingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resourcePath := normalizeResourcePath(r.URL.Path)
	if resourcePath == "" {
		h.base.ServeHTTP(w, r)
		return
	}

	var requestBody []byte
	if r.Method == http.MethodPut {
		requestBody, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(requestBody))
	}

	recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
	h.base.ServeHTTP(recorder, r)

	if recorder.statusCode < http.StatusOK || recorder.statusCode >= http.StatusMultipleChoices {
		return
	}

	switch r.Method {
	case http.MethodPut:
		_ = h.store.PutResource(r.Context(), h.userKey, resourcePath, requestBody)
	case http.MethodDelete:
		_ = h.store.DeleteResource(r.Context(), h.userKey, resourcePath)
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

func normalizeResourcePath(requestPath string) string {
	cleaned := path.Clean(requestPath)
	prefix := "/caldav"
	if cleaned == prefix || cleaned == prefix+"/" || !strings.HasPrefix(cleaned, prefix) {
		return ""
	}

	trimmed := strings.TrimPrefix(cleaned, prefix)
	if trimmed == "" || trimmed == "/" {
		return ""
	}

	return strings.TrimPrefix(trimmed, "/")
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
