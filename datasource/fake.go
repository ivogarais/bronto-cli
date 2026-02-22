package datasource

import (
	"context"
	"fmt"
	"math"
	"time"
)

type FakeDataSource struct {
	seq int
}

func NewFake() *FakeDataSource {
	return &FakeDataSource{}
}

func (f *FakeDataSource) RunQuery(ctx context.Context, q Query) (Result, error) {
	f.seq++

	switch q.Kind {
	case KindBar:
		// Fake changing values (smooth-ish wave)
		base := float64(f.seq)
		labels := []string{"api", "worker", "web", "db"}
		values := []float64{
			120 + 20*math.Sin(base/2),
			80 + 15*math.Sin(base/3),
			60 + 10*math.Sin(base/4),
			20 + 5*math.Sin(base/5),
		}
		return Result{
			Kind: KindBar,
			Bar: &BarResult{
				Labels: labels,
				Values: values,
			},
		}, nil

	case KindTable:
		cols := []string{"ts", "service", "message"}

		// Fake "new log line" each refresh
		now := time.Now().UTC().Format(time.RFC3339)
		service := []string{"api", "worker", "web", "db"}[f.seq%4]
		msg := fmt.Sprintf("error #%d: something happened", f.seq)

		rows := [][]string{
			{now, service, msg},
		}

		return Result{
			Kind: KindTable,
			Table: &TableResult{
				Columns: cols,
				Rows:    rows,
			},
		}, nil

	default:
		return Result{}, fmt.Errorf("fake datasource: unsupported query kind %q", q.Kind)
	}
}
