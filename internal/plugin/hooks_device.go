package plugin

import "context"

// One person, one device: a plugin that lets people sell away from the counter
// (a phone) turns this on, so signing in on one device locks that person's
// other devices — one drawer, one place at a time. Off, nothing changes.
var oneDevice bool

// TakeoverGuard may refuse a sign-in that would take a person away from the
// device they are on now (activeDevice), e.g. one that can't be reached to
// hand its work back. Return the reason the person sees. Must be cheap and
// fail open (block=false on error).
type TakeoverGuard func(ctx context.Context, userID int64, activeDevice string) (block bool, reason string)

var takeoverGuards hook[TakeoverGuard]

func (r *Registry) OneDevicePerPerson()              { oneDevice = true }
func (r *Registry) AddTakeoverGuard(g TakeoverGuard) { takeoverGuards.add(g) }

func OneDevice() bool                 { return oneDevice }
func TakeoverGuards() []TakeoverGuard { return takeoverGuards.all() }

// SetOneDeviceForTest turns the rule on or off with exactly these guards (tests only).
func SetOneDeviceForTest(on bool, guards ...TakeoverGuard) {
	oneDevice, takeoverGuards = on, hook[TakeoverGuard]{items: guards}
}
