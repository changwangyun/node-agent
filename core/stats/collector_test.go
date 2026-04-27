package stats

import (
	"fmt"
	"testing"
)

type mockCollector struct {
	traffic     *TrafficData
	connections *ConnectionData
	trafficErr  error
	connErr     error
}

func (m *mockCollector) GetTraffic() (*TrafficData, error) {
	if m.trafficErr != nil {
		return nil, m.trafficErr
	}
	return m.traffic, nil
}

func (m *mockCollector) GetConnections() (*ConnectionData, error) {
	if m.connErr != nil {
		return nil, m.connErr
	}
	return m.connections, nil
}

func (m *mockCollector) GetStats() (*StatsResult, error) {
	traffic, terr := m.GetTraffic()
	if terr != nil {
		return nil, terr
	}
	conn, cerr := m.GetConnections()
	if cerr != nil {
		return nil, cerr
	}
	return &StatsResult{Traffic: *traffic, Connections: *conn}, nil
}

func TestFallbackCollector(t *testing.T) {
	collector := NewFallbackCollector()

	traffic, err := collector.GetTraffic()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if traffic.Upload != 0 || traffic.Download != 0 {
		t.Error("expected zero traffic initially")
	}

	collector.AddUpload(1024)
	collector.AddDownload(2048)

	traffic, err = collector.GetTraffic()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if traffic.Upload != 1024 {
		t.Errorf("expected 1024 upload, got %d", traffic.Upload)
	}
	if traffic.Download != 2048 {
		t.Errorf("expected 2048 download, got %d", traffic.Download)
	}

	conn, err := collector.GetConnections()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.ActiveConnections != 0 {
		t.Error("expected 0 connections initially")
	}

	collector.SetConnections(5)
	conn, err = collector.GetConnections()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.ActiveConnections != 5 {
		t.Errorf("expected 5 connections, got %d", conn.ActiveConnections)
	}
}

func TestFallbackCollectorStats(t *testing.T) {
	collector := NewFallbackCollector()
	collector.AddUpload(100)
	collector.AddDownload(200)
	collector.SetConnections(3)

	stats, err := collector.GetStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Traffic.Upload != 100 {
		t.Errorf("expected 100 upload, got %d", stats.Traffic.Upload)
	}
	if stats.Traffic.Download != 200 {
		t.Errorf("expected 200 download, got %d", stats.Traffic.Download)
	}
	if stats.Connections.ActiveConnections != 3 {
		t.Errorf("expected 3 connections, got %d", stats.Connections.ActiveConnections)
	}
}

func TestMultiCollectorPrimarySuccess(t *testing.T) {
	primary := &mockCollector{
		traffic:     &TrafficData{Upload: 100, Download: 200},
		connections: &ConnectionData{ActiveConnections: 5},
	}
	fallback := NewFallbackCollector()
	fallback.AddUpload(500)
	fallback.AddDownload(600)

	multi := NewMultiCollector(primary, fallback)

	stats, err := multi.GetStats()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Traffic.Upload != 100 {
		t.Errorf("expected primary upload 100, got %d", stats.Traffic.Upload)
	}
	if stats.Traffic.Download != 200 {
		t.Errorf("expected primary download 200, got %d", stats.Traffic.Download)
	}
}

func TestMultiCollectorFallbackOnError(t *testing.T) {
	primary := &mockCollector{
		trafficErr: fmt.Errorf("connection refused"),
	}
	fallback := NewFallbackCollector()
	fallback.AddUpload(500)
	fallback.AddDownload(600)
	fallback.SetConnections(2)

	multi := NewMultiCollector(primary, fallback)

	traffic, err := multi.GetTraffic()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if traffic.Upload != 500 {
		t.Errorf("expected fallback upload 500, got %d", traffic.Upload)
	}
	if traffic.Download != 600 {
		t.Errorf("expected fallback download 600, got %d", traffic.Download)
	}
}

func TestFallbackCollectorIncremental(t *testing.T) {
	collector := NewFallbackCollector()

	collector.AddUpload(100)
	collector.AddUpload(50)
	collector.AddDownload(200)
	collector.AddDownload(300)

	traffic, _ := collector.GetTraffic()
	if traffic.Upload != 150 {
		t.Errorf("expected 150 cumulative upload, got %d", traffic.Upload)
	}
	if traffic.Download != 500 {
		t.Errorf("expected 500 cumulative download, got %d", traffic.Download)
	}
}

func TestSingBoxStatsCollectorUnreachable(t *testing.T) {
	collector := NewSingBoxStatsCollector("127.0.0.1:1", "")

	traffic, err := collector.GetTraffic()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if traffic == nil {
		t.Fatal("traffic should not be nil even when unreachable")
	}

	conn, err := collector.GetConnections()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.ActiveConnections != 0 {
		t.Errorf("expected 0 connections when unreachable, got %d", conn.ActiveConnections)
	}
}
