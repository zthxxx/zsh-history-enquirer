package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/internal/keys"
	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
	"github.com/zthxxx/zsh-history-enquirer/internal/ui"
)

// runEventLoop drives the model/event/render cycle until either the
// model signals termination (Submit / Cancel) or the parent ctx is
// canceled. Owns its own throttle, trailing-flush timer, and the
// "process probe-leftover events first" prelude.
func runEventLoop(
	ctx context.Context,
	t *tty.TTY,
	model *ui.Model,
	events <-chan keys.Event,
	preEvents []keys.Event,
	debugW io.Writer,
) (*RunResult, error) {
	throttle := ui.NewThrottle(RenderInterval)
	prevSize := 0

	render := func(force bool) {
		if !force && !throttle.Fire(time.Now()) {
			return
		}
		frame := model.Render(ui.RenderOptions{PrevSize: prevSize})
		_, _ = io.WriteString(t.Writer(), frame.Pre+frame.Body+frame.Post)
		prevSize = frame.Size
	}

	render(true)

	// Process any events that came from the probe-leftover bytes
	// before turning on the live event channel.
	for _, ev := range preEvents {
		debugEvent(debugW, "preevent", ev, model)
		if model.Update(ev) {
			render(true)
			return &RunResult{Output: model.Result}, nil
		}
	}
	if len(preEvents) > 0 {
		render(false)
	}

	// trailingFlush fires shortly after the last event so that the
	// final state of a burst (a paste, a fast-typed word) reaches
	// the screen even when the leading-edge throttle blocked the
	// per-event renders.
	trailingFlush := time.NewTimer(time.Hour)
	trailingFlush.Stop()
	armTrailing := func() {
		if !trailingFlush.Stop() {
			select {
			case <-trailingFlush.C:
			default:
			}
		}
		trailingFlush.Reset(RenderInterval)
	}
	defer trailingFlush.Stop()

	for {
		select {
		case <-ctx.Done():
			model.Canceled = true
			model.Result = model.Input
			render(true)
			return &RunResult{Output: model.Result}, ctx.Err()
		case ev, ok := <-events:
			if !ok {
				model.Canceled = true
				model.Result = model.Input
				render(true)
				return &RunResult{Output: model.Result}, errors.New("input closed")
			}
			debugEvent(debugW, "event", ev, model)
			if model.Update(ev) {
				render(true)
				return &RunResult{Output: model.Result}, nil
			}
			render(false)
			armTrailing()
		case <-trailingFlush.C:
			// Throttle window has elapsed since the last event; flush
			// whatever the latest model state is so the user sees the
			// final view of a burst.
			render(true)
		}
	}
}

// debugEvent is a one-line ZHE_DEBUG writer for an event + model
// snapshot. No-op when debugW is nil or io.Discard.
func debugEvent(w io.Writer, label string, ev keys.Event, m *ui.Model) {
	if w == nil || w == io.Discard {
		return
	}
	fmt.Fprintf(w, "[zhe] %s: %+v input=%q\n", label, ev, m.Input)
}
