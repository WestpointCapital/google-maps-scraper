//go:build !turso

package sqlite

import (
	"fmt"

	"github.com/gosom/google-maps-scraper/web"
)

// NewTurso is only available when built with -tags turso, which links the
// github.com/tursodatabase/go-libsql driver (supported on e.g. darwin/arm64
// and linux; upstream does not ship a darwin/amd64 library).
func NewTurso(_, _ string) (web.Datastore, error) {
	return nil, fmt.Errorf("turso (libsql): rebuild with -tags turso on a supported platform (darwin/arm64 or linux); darwin/amd64 has no bundled libsql library")
}
