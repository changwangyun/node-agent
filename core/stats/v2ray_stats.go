package stats

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statsServiceGRPC "github.com/v2fly/v2ray-core/v5/app/stats/command"
)

type UserTraffic struct {
	UserID   string `json:"user_id"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

type V2RayStatsCollector struct {
	mu      sync.RWMutex
	addr    string
	conn    *grpc.ClientConn
	client  statsServiceGRPC.StatsServiceClient
	enabled bool

	userTraffic map[string]*UserTraffic
}

func NewV2RayStatsCollector(addr string) *V2RayStatsCollector {
	c := &V2RayStatsCollector{
		addr:        addr,
		userTraffic: make(map[string]*UserTraffic),
	}
	c.tryConnect()
	return c
}

func (v *V2RayStatsCollector) tryConnect() {
	conn, err := grpc.Dial(v.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		v.enabled = false
		log.Printf("[v2ray-stats] dial %s failed: %v", v.addr, err)
		return
	}
	v.conn = conn
	v.client = statsServiceGRPC.NewStatsServiceClient(conn)
	v.enabled = true
	log.Printf("[v2ray-stats] dial %s succeeded (lazy connect)", v.addr)
}

func (v *V2RayStatsCollector) IsEnabled() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.enabled
}

func (v *V2RayStatsCollector) GetUserTraffic(userID string) (*UserTraffic, error) {
	if err := v.refresh(); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	if ut, ok := v.userTraffic[userID]; ok {
		return ut, nil
	}
	return &UserTraffic{UserID: userID}, nil
}

func (v *V2RayStatsCollector) GetAllUserTraffic() ([]*UserTraffic, error) {
	if err := v.refresh(); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	result := make([]*UserTraffic, 0, len(v.userTraffic))
	for _, ut := range v.userTraffic {
		result = append(result, ut)
	}
	return result, nil
}

func (v *V2RayStatsCollector) refresh() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.enabled {
		if v.conn != nil {
			v.conn.Close()
			v.conn = nil
		}
		v.tryConnect()
		if !v.enabled {
			return fmt.Errorf("v2ray api not available")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := v.client.QueryStats(ctx, &statsServiceGRPC.QueryStatsRequest{
		Reset_: false,
	})
	if err != nil {
		v.enabled = false
		if v.conn != nil {
			v.conn.Close()
			v.conn = nil
		}
		log.Printf("[v2ray-stats] query failed: %v, will retry on next call", err)
		return fmt.Errorf("query v2ray stats: %w", err)
	}

	for _, stat := range resp.GetStat() {
		name := stat.GetName()
		value := stat.GetValue()

		userID, direction := parseStatName(name)
		if userID == "" {
			log.Printf("[v2ray-stats] skipping stat: name=%q value=%d", name, value)
			continue
		}

		ut, ok := v.userTraffic[userID]
		if !ok {
			ut = &UserTraffic{UserID: userID}
			v.userTraffic[userID] = ut
		}

		if direction == "up" {
			ut.Upload = value
		} else if direction == "down" {
			ut.Download = value
		}
	}

	return nil
}

func parseStatName(name string) (userID, direction string) {
	parts := strings.Split(name, ">>>")
	if len(parts) < 4 {
		return "", ""
	}

	if parts[0] == "user" && parts[2] == "traffic" {
		dir := "up"
		if parts[3] == "downlink" {
			dir = "down"
		}
		return parts[1], dir
	}

	if parts[0] == "outbound" && parts[2] == "traffic" {
		dir := "up"
		if parts[3] == "downlink" {
			dir = "down"
		}
		return "outbound:" + parts[1], dir
	}

	return "", ""
}

func (v *V2RayStatsCollector) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.conn != nil {
		v.conn.Close()
	}
}
