package main

import (
	"fmt"
	"time"
)

// heartbeat is how often a long-running command says it is still working.
var heartbeat = 30 * time.Second

// note shows a line under the running step on the page ("Downloading …").
func (j *Job) note(text string) {
	j.lastNote = text
	fmt.Fprintf(j.Log, "@@NOTE %s\n", text)
}

// busy runs fn, adding "· still working, N min" to the step's note every
// heartbeat, so a long install never looks stuck.
func (j *Job) busy(fn func()) {
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		start := time.Now()
		for {
			select {
			case <-stop:
				return
			case <-time.After(heartbeat):
				label := j.lastNote
				if label == "" {
					label = "Working"
				}
				fmt.Fprintf(j.Log, "@@NOTE %s · still working, %s so far\n", label, time.Since(start).Round(time.Second))
			}
		}
	}()
	fn()
	close(stop)
	<-done // never write to the log alongside the caller
}
