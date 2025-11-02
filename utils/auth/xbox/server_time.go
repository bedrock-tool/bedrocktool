package xbox

import (
	"net/http"
	"sync/atomic"
	"time"
)

type serverTimeEnt struct {
	Updated time.Time
	Time    time.Time
}

var (
	serverTime atomic.Pointer[serverTimeEnt]
)

func updateServerTimeFromHeaders(headers http.Header) {
	date := headers.Get("Date")
	if date == "" {
		return
	}
	t, err := time.Parse(time.RFC1123, date)
	if err != nil || t.IsZero() {
		return
	}
	serverTime.Store(&serverTimeEnt{
		Updated: time.Now(),
		Time:    t,
	})
}

func getCurrentServerTime() time.Time {
	ent := serverTime.Load()
	if ent == nil {
		return time.Now()
	}
	diff := time.Since(ent.Updated)
	return ent.Time.Add(diff)
}
