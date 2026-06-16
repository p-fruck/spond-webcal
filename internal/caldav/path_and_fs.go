package caldav

import (
	"context"
	"net/http"
	"os"
	"path"
	"strings"

	"golang.org/x/net/webdav"
)

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
