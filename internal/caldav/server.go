package caldav

import (
	"context"
	"fmt"
	"net/http"
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
	ctx, ok := h.buildRequestContext(w, r)
	if !ok {
		return
	}

	if ctx.resourcePath == "" {
		ctx.userHandler.ServeHTTP(w, ctx.rewritten)
		return
	}

	if !h.loadCurrentResource(w, &ctx) {
		return
	}

	if !passesIfMatchPrecondition(ctx.rewritten, ctx.found, ctx.currentResource.Content) {
		http.Error(w, "precondition failed", http.StatusPreconditionFailed)
		return
	}

	if !prepareRequestBodyAndResponseETag(&ctx) {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	statusCode := serveWithStatusRecorder(w, ctx.userHandler, ctx.rewritten, ctx.responseETag)
	if !isSuccessfulStatus(statusCode) {
		return
	}

	h.persistMutation(ctx)
}
