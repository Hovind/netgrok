package timer

import (
	"time"
)


type Timer struct {
    Timer *time.Timer
    Running bool
}

func New() *Timer {
	t := time.NewTimer(0);
	<-t.C;
	return &Timer{Timer: t, Running: false};
}

func (t *Timer) Start(d time.Duration) {
	t.Timer.Reset(d);
	t.Running = true;
}

func (t *Timer) Stop() {
	t.Timer.Stop();
	t.Running = false;
}
