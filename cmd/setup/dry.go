package main

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// -dry: the real page and plan, but every step only says what it would do.
// KAROTS_SETUP_DRY_FAIL=<step id> makes that step fail (for a look at the
// failure screen; "database" asks for the postgres password).

func dryJob(ch Choice) (*Job, error) {
	j, err := newJob(ch)
	if err == nil {
		j.Run = nil // never used: drySteps don't run anything
	}
	return j, err
}

func drySteps(j *Job) []Step {
	steps := Steps(j)
	for i := range steps {
		s := steps[i]
		steps[i].Do = func(j *Job) error {
			fmt.Fprintf(j.Log, "(dry run) %s\n", s.Detail(j))
			time.Sleep(600 * time.Millisecond)
			if os.Getenv("KAROTS_SETUP_DRY_FAIL") == s.ID {
				if s.ID == "database" && j.SuperPassword == "" {
					return ErrNeedSuperPassword
				}
				if s.ID != "database" {
					return errors.New("port 3000 is used by another program (karots-pos?) — stop it, or change the shop's till port in the Shop Builder")
				}
			}
			return nil
		}
	}
	return steps
}
