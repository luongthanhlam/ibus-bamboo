package main

import (
	"github.com/rajveermalviya/wl"
	"github.com/rajveermalviya/wl/internal/log"
)

type pointerEventMask int

const (
	pointerEventEnter        pointerEventMask = 1 << 0
	pointerEventLeave                         = 1 << 1
	pointerEventMotion                        = 1 << 2
	pointerEventButton                        = 1 << 3
	pointerEventAxis                          = 1 << 4
	pointerEventAxisSource                    = 1 << 5
	pointerEventAxisStop                      = 1 << 6
	pointerEventAxisDiscrete                  = 1 << 7
)

type pointerEvent struct {
	eventMask          pointerEventMask
	surfaceX, surfaceY uint32
	button, state      uint32
	time               uint32
	serial             uint32
	axes               [2]struct {
		valid    bool
		value    int32
		discrete int32
	}
	axisSource uint32
}

func (app *appState) HandlePointerEnter(ev wl.PointerEnterEvent) {
	app.pointerEvent.eventMask |= pointerEventEnter
	app.pointerEvent.serial = ev.Serial
	app.pointerEvent.surfaceX = uint32(ev.SurfaceX)
	app.pointerEvent.surfaceY = uint32(ev.SurfaceY)
}

func (app *appState) HandlePointerLeave(ev wl.PointerLeaveEvent) {
	app.pointerEvent.eventMask |= pointerEventLeave
	app.pointerEvent.serial = ev.Serial
}

func (app *appState) HandlePointerMotion(ev wl.PointerMotionEvent) {
	app.pointerEvent.eventMask |= pointerEventMotion
	app.pointerEvent.time = ev.Time
	app.pointerEvent.surfaceX = uint32(ev.SurfaceX)
	app.pointerEvent.surfaceY = uint32(ev.SurfaceY)
}

func (app *appState) HandlePointerButton(ev wl.PointerButtonEvent) {
	app.pointerEvent.eventMask |= pointerEventButton
	app.pointerEvent.serial = ev.Serial
	app.pointerEvent.time = ev.Time
	app.pointerEvent.button = ev.Button
	app.pointerEvent.state = ev.State
}

func (app *appState) HandlePointerAxis(ev wl.PointerAxisEvent) {
	app.pointerEvent.eventMask |= pointerEventAxis
	app.pointerEvent.time = ev.Time
	app.pointerEvent.axes[ev.Axis].valid = true
	app.pointerEvent.axes[ev.Axis].value = int32(ev.Value)
}

func (app *appState) HandlePointerAxisSource(ev wl.PointerAxisSourceEvent) {
	app.pointerEvent.eventMask |= pointerEventAxis
	app.pointerEvent.axisSource = ev.AxisSource
}

func (app *appState) HandlePointerAxisStop(ev wl.PointerAxisStopEvent) {
	app.pointerEvent.eventMask |= pointerEventAxisStop
	app.pointerEvent.time = ev.Time
	app.pointerEvent.axes[ev.Axis].valid = true
}

func (app *appState) HandlePointerAxisDiscrete(ev wl.PointerAxisDiscreteEvent) {
	app.pointerEvent.eventMask |= pointerEventAxisDiscrete
	app.pointerEvent.axes[ev.Axis].valid = true
	app.pointerEvent.axes[ev.Axis].discrete = ev.Discrete
}

var axisName = map[int]string{
	wl.PointerAxisVerticalScroll:   "vertical",
	wl.PointerAxisHorizontalScroll: "horizontal",
}

var axisSource = map[uint32]string{
	wl.PointerAxisSourceWheel:      "wheel",
	wl.PointerAxisSourceFinger:     "finger",
	wl.PointerAxisSourceContinuous: "continuous",
	wl.PointerAxisSourceWheelTilt:  "wheel tilt",
}

func (app *appState) HandlePointerFrame(ev wl.PointerFrameEvent) {
	event := app.pointerEvent

	if (event.eventMask & pointerEventEnter) != 0 {
		log.Printf("entered %v, %v", event.surfaceX, event.surfaceY)
	}

	if (event.eventMask & pointerEventLeave) != 0 {
		log.Print("leave")
	}
	if (event.eventMask & pointerEventMotion) != 0 {
		log.Printf("motion %v, %v", event.surfaceX, event.surfaceY)
	}
	if (event.eventMask & pointerEventButton) != 0 {
		if event.state == wl.PointerButtonStateReleased {
			log.Printf("button %d released", event.button)
		} else {
			log.Printf("button %d pressed", event.button)
		}
	}

	axisEvents := pointerEventMask(pointerEventAxis | pointerEventAxisSource | pointerEventAxisStop | pointerEventAxisDiscrete)

	if (event.eventMask & axisEvents) != 0 {
		for i := 0; i < 2; i++ {
			if !event.axes[i].valid {
				continue
			}
			log.Printf("%s axis ", axisName[i])
			if (event.eventMask & pointerEventAxis) != 0 {
				log.Printf("value %v", event.axes[i].value)
			}
			if (event.eventMask & pointerEventAxisDiscrete) != 0 {
				log.Printf("discrete %d ", event.axes[i].discrete)
			}
			if (event.eventMask & pointerEventAxisSource) != 0 {
				log.Printf("via %s", axisSource[event.axisSource])
			}
			if (event.eventMask & pointerEventAxisStop) != 0 {
				log.Printf("(stopped)")
			}
		}
	}

	app.pointerEvent = pointerEvent{}
}
