package sched

import (
	"context"
	"github.com/go-co-op/gocron"
	"reflect"
	"testing"
	"time"
)

func TestFuzzyTask(t *testing.T) {
	var ctx context.Context
	var cf context.CancelFunc
	var cancelled = map[string]bool{}
	ctx, cf = context.WithCancel(context.Background())
	type args struct {
		ctx          context.Context
		cancel       context.CancelFunc
		maxFuzziness int64
		fn           Runner
	}
	tests := []struct {
		name      string
		args      args
		wait      time.Duration
		cancelled bool
	}{
		{
			name:      "test_0",
			cancelled: true,
			args: args{
				ctx:          ctx,
				cancel:       cf,
				maxFuzziness: int64(2 * time.Second),
				fn: func(ctx context.Context) {
					select {
					case <-ctx.Done():
						cancelled["test_0"] = true
						return
					case <-time.After(500 * time.Second):
						cancelled["test_0"] = false
						return
					}
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyTask(tt.name, tt.args.ctx, tt.args.maxFuzziness, tt.args.fn)
			tt.args.cancel()
			got()
			c, ok := cancelled[tt.name]
			if ok {
				if tt.cancelled && !c {
					t.Errorf("FuzzyTask() should set cancelled to %v, got %v", tt.cancelled, c)
				}
			}
			if !ok && tt.cancelled {
				t.Errorf("FuzzyTask() should set cancelled to %v!", tt.cancelled)
			}
		})
	}
}

func TestAddCron(t *testing.T) {
	type args struct {
		s    *gocron.Scheduler
		expr string
		fn   func()
	}
	tests := []struct {
		name    string
		args    args
		want    *gocron.Job
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				s:    gocron.NewScheduler(time.UTC),
				expr: "* * * * *",
				fn:   func() {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AddCron(tt.args.s, tt.args.expr, tt.args.fn)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddCron() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tp := reflect.TypeOf(got); tp.Name() != "" {
				t.Errorf("AddCron() got = %v, want %v", got, tt.want)
			}
		})
	}
}
