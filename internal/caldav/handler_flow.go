package caldav

import (
	"bytes"
	"io"
	"net/http"
)

type requestContext struct {
	userKey         string
	resourcePath    string
	userHandler     http.Handler
	rewritten       *http.Request
	currentResource Resource
	found           bool
	requestBody     []byte
	responseETag    string
}

func (h *persistingHandler) buildRequestContext(w http.ResponseWriter, r *http.Request) (requestContext, bool) {
	userKey, resourcePath := normalizeResourcePath(r.URL.Path, h.defaultUserKey)

	userHandler, err := h.getUserHandler(userKey)
	if err != nil {
		http.Error(w, "failed to load caldav resources", http.StatusInternalServerError)
		return requestContext{}, false
	}

	ctx := requestContext{
		userKey:      userKey,
		resourcePath: resourcePath,
		userHandler:  userHandler,
		rewritten:    cloneRequestWithPath(r, toHandlerPath(resourcePath)),
	}

	return ctx, true
}

func (h *persistingHandler) loadCurrentResource(w http.ResponseWriter, ctx *requestContext) bool {
	resource, found, err := h.store.GetResource(ctx.rewritten.Context(), ctx.userKey, ctx.resourcePath)
	if err != nil {
		http.Error(w, "failed to lookup caldav resource", http.StatusInternalServerError)
		return false
	}

	ctx.currentResource = resource
	ctx.found = found
	return true
}

func prepareRequestBodyAndResponseETag(ctx *requestContext) bool {
	if ctx.found && (ctx.rewritten.Method == http.MethodGet || ctx.rewritten.Method == http.MethodHead) {
		ctx.responseETag = resourceETag(ctx.currentResource.Content)
	}

	if ctx.rewritten.Method != http.MethodPut {
		return true
	}

	requestBody, err := io.ReadAll(ctx.rewritten.Body)
	if err != nil {
		return false
	}

	ctx.requestBody = requestBody
	ctx.rewritten.Body = io.NopCloser(bytes.NewReader(requestBody))
	ctx.responseETag = resourceETag(requestBody)
	return true
}

func (h *persistingHandler) persistMutation(ctx requestContext) {
	switch ctx.rewritten.Method {
	case http.MethodPut:
		_ = h.store.PutResource(ctx.rewritten.Context(), ctx.userKey, ctx.resourcePath, ctx.requestBody)
	case http.MethodDelete:
		_ = h.store.DeleteResource(ctx.rewritten.Context(), ctx.userKey, ctx.resourcePath)
	}
}
