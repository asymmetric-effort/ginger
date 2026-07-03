package influxdb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/asymmetric-effort/ginger/internal/codec/influxlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

// DependencyStorage stores and queries service dependencies.
type DependencyStorage struct {
	client *Client
	bucket string
}

// NewDependencyStorage creates a new InfluxDB dependency storage.
func NewDependencyStorage(client *Client, bucket string) *DependencyStorage {
	return &DependencyStorage{client: client, bucket: bucket}
}

// WriteLinks writes dependency links to InfluxDB.
func (ds *DependencyStorage) WriteLinks(ctx context.Context, ts time.Time, deps []storage.DependencyLink) error {
	var points []influxlp.Point
	for _, d := range deps {
		points = append(points, influxlp.Point{
			Measurement: "dependencies",
			Tags: []influxlp.Tag{
				{Key: "parent", Value: d.Parent},
				{Key: "child", Value: d.Child},
			},
			Fields: []influxlp.Field{
				influxlp.IntField("call_count", int64(d.CallCount)),
			},
			Timestamp: ts,
		})
	}
	return ds.client.Write(ctx, points)
}

// GetDependencies returns dependencies within the time window.
func (ds *DependencyStorage) GetDependencies(ctx context.Context, endTime time.Time, lookback time.Duration) ([]storage.DependencyLink, error) {
	startTime := endTime.Add(-lookback)
	flux := fmt.Sprintf(`from(bucket: "%s")
  |> range(start: %s, stop: %s)
  |> filter(fn: (r) => r._measurement == "dependencies" and r._field == "call_count")
  |> group(columns: ["parent", "child"])
  |> sum()`,
		ds.bucket,
		startTime.UTC().Format(time.RFC3339),
		endTime.UTC().Format(time.RFC3339))

	records, err := ds.client.Query(ctx, flux)
	if err != nil {
		return nil, err
	}

	var deps []storage.DependencyLink
	for _, r := range records {
		count, _ := strconv.ParseUint(r.Values["_value"], 10, 64)
		deps = append(deps, storage.DependencyLink{
			Parent:    r.Values["parent"],
			Child:     r.Values["child"],
			CallCount: count,
		})
	}
	return deps, nil
}
