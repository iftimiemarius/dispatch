package models

import (
	"math/rand"
	"sync"
	"time"

	"github.com/oklog/ulid"
)

// idMu guards the entropy source for ULID generation.
var (
	idMu  sync.Mutex
	idRnd = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// NewID returns a new time-sortable ULID string, suitable for primary keys.
// ULIDs sort by creation time, which keeps listings naturally ordered.
func NewID() string {
	idMu.Lock()
	defer idMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulid.Monotonic(idRnd, 0)).String()
}
