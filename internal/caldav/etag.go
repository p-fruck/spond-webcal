package caldav

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

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
