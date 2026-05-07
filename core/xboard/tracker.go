package xboard

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
	"sync/atomic"
)

type trackerSnapshot struct {
	traffic  map[string][2]int64
	aliveIPs map[string]map[string]bool
	online   map[string]int
}

type Tracker struct {
	mu              sync.Mutex
	lastSeen        map[string][2]int64
	pendingTraffic  map[string][2]int64
	live            atomic.Pointer[trackerSnapshot]
	aliveIPsBuf     map[string][]string
	lastAliveIPsHash string
}

func NewTracker() *Tracker {
	t := &Tracker{
		lastSeen:       make(map[string][2]int64),
		pendingTraffic: make(map[string][2]int64),
		aliveIPsBuf:    make(map[string][]string),
	}
	t.live.Store(&trackerSnapshot{
		traffic:  make(map[string][2]int64),
		aliveIPs: make(map[string]map[string]bool),
		online:   make(map[string]int),
	})
	return t
}

func (t *Tracker) Process(cumTraffic map[string][2]int64, kernelAliveIPs map[string]map[string]bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for uid, cum := range cumTraffic {
		prev := t.lastSeen[uid]
		deltaUp := cum[0] - prev[0]
		deltaDown := cum[1] - prev[1]
		if deltaUp < 0 {
			deltaUp = cum[0]
		}
		if deltaDown < 0 {
			deltaDown = cum[1]
		}
		t.lastSeen[uid] = cum
		if deltaUp > 0 || deltaDown > 0 {
			cur := t.pendingTraffic[uid]
			cur[0] += deltaUp
			cur[1] += deltaDown
			t.pendingTraffic[uid] = cur
		}
	}

	online := make(map[string]int, len(kernelAliveIPs))
	for uid, ips := range kernelAliveIPs {
		online[uid] = len(ips)
	}

	t.live.Store(&trackerSnapshot{
		traffic:  copyTrafficMap(t.pendingTraffic),
		aliveIPs: kernelAliveIPs,
		online:   online,
	})
}

func (t *Tracker) FlushTraffic() map[string][2]int64 {
	t.mu.Lock()
	data := t.pendingTraffic
	t.pendingTraffic = make(map[string][2]int64, len(data))
	t.mu.Unlock()
	return data
}

func (t *Tracker) RestoreTraffic(data map[string][2]int64) {
	t.mu.Lock()
	for uid, d := range data {
		cur := t.pendingTraffic[uid]
		cur[0] += d[0]
		cur[1] += d[1]
		t.pendingTraffic[uid] = cur
	}
	t.mu.Unlock()
}

func (t *Tracker) HasTraffic() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pendingTraffic) > 0
}

func (t *Tracker) FlushAliveIPs() map[string][]string {
	s := t.live.Load()
	t.mu.Lock()
	defer t.mu.Unlock()

	currentHash := calcAliveIPsHash(s.aliveIPs)
	if currentHash == t.lastAliveIPsHash {
		return nil
	}
	t.lastAliveIPsHash = currentHash

	for k := range t.aliveIPsBuf {
		delete(t.aliveIPsBuf, k)
	}
	for uid, ips := range s.aliveIPs {
		buf := t.aliveIPsBuf[uid]
		if buf == nil {
			buf = make([]string, 0, len(ips))
		}
		buf = buf[:0]
		for ip := range ips {
			buf = append(buf, ip)
		}
		t.aliveIPsBuf[uid] = buf
	}
	return t.aliveIPsBuf
}

func (t *Tracker) CurrentOnline() map[string]int {
	s := t.live.Load()
	cp := make(map[string]int, len(s.online))
	for k, v := range s.online {
		cp[k] = v
	}
	return cp
}

func calcAliveIPsHash(aliveIPs map[string]map[string]bool) string {
	if len(aliveIPs) == 0 {
		return ""
	}
	h := sha256.New()
	userIDs := make([]string, 0, len(aliveIPs))
	for uid := range aliveIPs {
		userIDs = append(userIDs, uid)
	}
	sort.Strings(userIDs)
	for _, uid := range userIDs {
		ips := aliveIPs[uid]
		ipList := make([]string, 0, len(ips))
		for ip := range ips {
			ipList = append(ipList, ip)
		}
		sort.Strings(ipList)
		h.Write([]byte(uid))
		for _, ip := range ipList {
			h.Write([]byte(ip))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func copyTrafficMap(src map[string][2]int64) map[string][2]int64 {
	dst := make(map[string][2]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
