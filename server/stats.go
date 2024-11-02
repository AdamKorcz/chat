// Logic related to expvar handling: reporting live stats such as
// session and topic counts, memory usage etc.
// The stats updates happen in a separate go routine to avoid
// locking on main logic routines.

package server

import (
	"encoding/json"
	"expvar"
	"net/http"
	"runtime"
	"sort"
	"time"

	"github.com/tinode/chat/server/logs"
	"github.com/tinode/chat/server/store"
)

// A simple implementation of histogram expvar.Var.
// `Bounds` specifies the histogram buckets as follows (length = len(bounds)):
//
//	(-inf, Bounds[i]) for i = 0
//	[Bounds[i-1], Bounds[i]) for 0 < i < length
//	[Bounds[i-1], +inf) for i = length
type histogram struct {
	Count          int64     `json:"count"`
	Sum            float64   `json:"sum"`
	CountPerBucket []int64   `json:"count_per_bucket"`
	Bounds         []float64 `json:"bounds"`
}

func (h *histogram) addSample(v float64) {
	h.Count++
	h.Sum += v
	idx := sort.SearchFloat64s(h.Bounds, v)
	h.CountPerBucket[idx]++
}

func (h *histogram) String() string {
	if r, err := json.Marshal(h); err == nil {
		return string(r)
	}
	return ""
}

func statsRegisterDbStats() {
	if f := store.Store.DbStats(); f != nil {
		expvar.Publish("DbStats", expvar.Func(f))
	}
}

// Register integer variable. Don't check for initialization.
func statsRegisterInt(name string) {
	expvar.Publish(name, new(expvar.Int))
}

// Register histogram variable. `bounds` specifies histogram buckets/bins
// (see comment next to the `histogram` struct definition).
func statsRegisterHistogram(name string, bounds []float64) {
	numBuckets := len(bounds) + 1
	expvar.Publish(name, &histogram{
		CountPerBucket: make([]int64, numBuckets),
		Bounds:         bounds,
	})
}
