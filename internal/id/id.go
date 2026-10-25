package id

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

var entropy = ulid.Monotonic(rand.Reader, 0)

func New() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
