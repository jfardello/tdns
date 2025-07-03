package sched

import (
	"context"
	"github.com/go-co-op/gocron"
	"github.com/jfardello/tdns/log"
	"math/rand"
	"sync"
	"time"
)

type Task struct {
	Name string
	Expr string
	Fn   func()
}

// TaskRegistry holds tasks created by plugin Init methods.
var TaskRegistry []Task

type Runner func(context.Context)

var randomSource *rand.Rand

func init() {
	randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// RandomInt generates a random integer between min and max
func randomInt(max int64) int64 {
	return randomSource.Int63n(max + 1)
}

func AddCron(s *gocron.Scheduler, expr string, fn func()) (*gocron.Job, error) {
	job, err := s.Cron(expr).Do(fn)
	if err != nil {
		return nil, err
	}
	job.SingletonMode()
	return job, nil
}

func Add(t Task) {
	logger := log.GetLogger("sched", "Add")
	logger.Infof("Adding task '%s', task expr: %s...", t.Name, t.Expr)
	TaskRegistry = append(TaskRegistry, t)
}

func FuzzyTask(name string, ctx context.Context, maxFuzziness int64, fn Runner) func() {
	logger := log.GetLogger("sched", "FuzzyTask")
	return func() {
		var wg sync.WaitGroup
		wg.Add(1)
		ri := randomInt(maxFuzziness)
		logger.Debugf("FuzzyTask (%s) starting, waiting %d seconds", name, time.Duration(ri)/time.Second)
		<-time.After(time.Duration(ri))
		go func() {
			logger.Debug("Calling inner function")
			fn(ctx)
			wg.Done()
		}()
		wg.Wait()
	}
}
