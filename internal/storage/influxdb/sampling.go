package influxdb

import (
	"context"
	"encoding/json"
	"time"

	"github.com/asymmetric-effort/ginger/internal/codec/influxlp"
	"github.com/asymmetric-effort/ginger/internal/storage"
)

// SamplingStorage persists adaptive sampling data.
type SamplingStorage struct {
	client *Client
	bucket string
}

// NewSamplingStorage creates a new InfluxDB sampling storage.
func NewSamplingStorage(client *Client, bucket string) *SamplingStorage {
	return &SamplingStorage{client: client, bucket: bucket}
}

// InsertThroughput stores throughput data.
func (ss *SamplingStorage) InsertThroughput(ctx context.Context, throughput []storage.Throughput) error {
	var points []influxlp.Point
	for _, t := range throughput {
		points = append(points, influxlp.Point{
			Measurement: "sampling_throughput",
			Tags: []influxlp.Tag{
				{Key: "service", Value: t.Service},
				{Key: "operation", Value: t.Operation},
			},
			Fields: []influxlp.Field{
				influxlp.IntField("count", t.Count),
				influxlp.FloatField("probabilities", t.Probabilities),
			},
			Timestamp: time.Now(),
		})
	}
	return ss.client.Write(ctx, points)
}

// InsertProbabilities stores sampling probabilities.
func (ss *SamplingStorage) InsertProbabilities(ctx context.Context, hostname string, probs storage.ServiceOperationProbabilities) error {
	probsJSON, err := json.Marshal(probs)
	if err != nil {
		return err
	}
	return ss.client.Write(ctx, []influxlp.Point{{
		Measurement: "sampling_probabilities",
		Tags: []influxlp.Tag{
			{Key: "hostname", Value: hostname},
		},
		Fields: []influxlp.Field{
			influxlp.StringField("probabilities", string(probsJSON)),
		},
		Timestamp: time.Now(),
	}})
}

// GetThroughput returns throughput data.
func (ss *SamplingStorage) GetThroughput(ctx context.Context, _, _ time.Time) ([]storage.Throughput, error) {
	flux := `from(bucket: "` + ss.bucket + `")
  |> range(start: -1h)
  |> filter(fn: (r) => r._measurement == "sampling_throughput")`

	records, err := ss.client.Query(ctx, flux)
	if err != nil {
		return nil, err
	}

	var result []storage.Throughput
	for _, r := range records {
		result = append(result, storage.Throughput{
			Service:   r.Values["service"],
			Operation: r.Values["operation"],
		})
	}
	return result, nil
}

// GetLatestProbabilities returns the latest sampling probabilities.
func (ss *SamplingStorage) GetLatestProbabilities(ctx context.Context) (storage.ServiceOperationProbabilities, error) {
	flux := `from(bucket: "` + ss.bucket + `")
  |> range(start: -1h)
  |> filter(fn: (r) => r._measurement == "sampling_probabilities")
  |> last()`

	records, err := ss.client.Query(ctx, flux)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return storage.ServiceOperationProbabilities{}, nil
	}

	var probs storage.ServiceOperationProbabilities
	probsStr := records[0].Values["_value"]
	if probsStr != "" {
		json.Unmarshal([]byte(probsStr), &probs)
	}
	return probs, nil
}
