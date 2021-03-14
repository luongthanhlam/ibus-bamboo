package main

import (
	"flag"
	"image"
	"os"
	"syscall"

	"github.com/nfnt/resize"
	"github.com/rajveermalviya/wl"
	"github.com/rajveermalviya/wl/internal/log"
	"github.com/rajveermalviya/wl/internal/swizzle"
	"github.com/rajveermalviya/wl/internal/tempfile"
	"github.com/rajveermalviya/wl/xdg"
)

func init() {
	flag.Parse()
}

func main() {
	if flag.NArg() == 0 {
		log.Fatalf("usage: %s imagefile", os.Args[0])
	}

	pImage, err := imageFromFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	pImage = resize.Resize(0, 1080, pImage, resize.Lanczos3)

	img := resize.Resize(0, 480, copyImage(pImage), resize.Lanczos3)
	rect := img.Bounds()

	app := &appState{
		pImage:   pImage,
		width:    int32(rect.Dx()),
		height:   int32(rect.Dy()),
		image:    img,
		exitChan: make(chan struct{}),
	}

	display, err := wl.Connect("")
	if err != nil {
		log.Fatalf("unable to connect to wayland server: %v", err)
	}
	app.display = display

	display.AddErrorHandler(app)

	run(app)

	log.Println("closing")

	if app.pointer != nil {
		app.releasePointer()
	}

	if app.keyboard != nil {
		app.releaseKeyboard()
	}

	if app.seat != nil {
		app.seat.RemoveCapabilitiesHandler(app)
		app.seat.RemoveNameHandler(app)

		if err := app.seat.Release(); err != nil {
			log.Println("unable to destroy wl_seat")
		}
		app.seat = nil
	}

	if app.wmBase != nil {
		app.wmBase.RemovePingHandler(app)

		if err := app.wmBase.Destroy(); err != nil {
			log.Println("unable to destroy xdg_wm_base")
		}
		app.wmBase = nil
	}

	app.Context().Close()
}

type appState struct {
	pImage        image.Image
	width, height int32
	image         image.Image
	exitChan      chan struct{}

	display    *wl.Display
	shm        *wl.Shm
	compositor *wl.Compositor
	wmBase     *xdg.WmBase

	surface    *wl.Surface
	xdgSurface *xdg.Surface
	seat       *wl.Seat

	keyboard *wl.Keyboard
	pointer  *wl.Pointer

	pointerEvent pointerEvent

	// offset    float64
	// lastFrame uint32
}

func (app *appState) Dispatch() chan<- struct{} {
	return app.Context().Dispatch()
}

func (app *appState) Context() *wl.Context {
	return app.display.Context()
}

// type surfaceFrameDone struct {
// 	app *appState

// 	cb *wl.Callback
// }

// func (d surfaceFrameDone) HandleCallbackDone(ev wl.CallbackDoneEvent) {
// 	d.cb.RemoveDoneHandler(d)

// 	callback, err := d.app.surface.Frame()
// 	if err != nil {
// 		log.Fatalf("unable to get surface frame callback: %v", err)
// 	}
// 	callback.AddDoneHandler(surfaceFrameDone{app: d.app, cb: callback})

// 	time := ev.CallbackData
// 	if d.app.lastFrame != 0 {
// 		elapsed := time - d.app.lastFrame
// 		d.app.offset += float64(elapsed) / 1000.0 * 24
// 	}

// 	buffer := d.app.drawFrame()

// 	if err := d.app.surface.Attach(buffer, 0, 0); err != nil {
// 		log.Fatalf("unable to attach buffer to surface: %v", err)
// 	}
// 	if err := d.app.surface.DamageBuffer(0, 0, math.MaxInt32, math.MaxInt32); err != nil {
// 		log.Fatalf("unable to damage full buffer: %v", err)
// 	}
// 	if err := d.app.surface.Commit(); err != nil {
// 		log.Fatalf("unable to commit surface state: %v", err)
// 	}

// 	d.app.lastFrame = time
// }

func run(app *appState) {
	app.registerGlobalInterfaces()

	log.Print("all interfaces registered")

	surface, err := app.compositor.CreateSurface()
	if err != nil {
		log.Fatalf("unable to create compositor surface: %v", err)
	}
	app.surface = surface
	log.Print("created new wl_surface")

	xdgSurface, err := app.wmBase.GetXdgSurface(surface)
	if err != nil {
		log.Fatalf("unable to get xdg surface: %v", err)
	}
	app.xdgSurface = xdgSurface
	log.Print("got xdg_surface")

	xdgSurface.AddConfigureHandler(app)
	log.Print("added configure handler")

	xdgTopLevel, err := xdgSurface.GetToplevel()
	if err != nil {
		log.Fatalf("unable to get xdg toplevel: %v", err)
	}
	log.Print("get xdg toplevel")

	xdgTopLevel.AddConfigureHandler(app)
	xdgTopLevel.AddCloseHandler(app)
	log.Print("added toplevel close handler")

	if err2 := xdgTopLevel.SetTitle(flag.Arg(0)); err2 != nil {
		log.Fatalf("unable to set toplevel title: %v", err2)
	}
	if err2 := app.surface.Commit(); err2 != nil {
		log.Fatalf("unable to commit surface state: %v", err2)
	}
	log.Printf("title set to: %v", flag.Arg(0))

	// callback, err := app.surface.Frame()
	// if err != nil {
	// 	log.Fatalf("unable to get surface frame callback: %v", err)
	// }
	// callback.AddDoneHandler(surfaceFrameDone{app: app, cb: callback})

	for {
		select {
		case <-app.exitChan:
			return

		case app.Dispatch() <- struct{}{}:
			log.Print("dispatched")
		}
	}
}

func (app *appState) drawFrame() *wl.Buffer {
	log.Print("drawing frame")

	stride := app.width * 4
	size := stride * app.height

	file, err := tempfile.TempFile(int64(size))
	if err != nil {
		log.Fatalf("TempFile failed: %s", err)
	}

	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		log.Fatalf("unable to create mapping: %s", err)
	}

	pool, err := app.shm.CreatePool(file.Fd(), size)
	if err != nil {
		log.Fatalf("unable to create shm pool: %s", err)
	}

	buf, err := pool.CreateBuffer(0, app.width, app.height, stride, wl.ShmFormatArgb8888)
	if err != nil {
		log.Fatalf("unable to create wl.Buffer from shm pool: %v", err)
	}
	if err := pool.Destroy(); err != nil {
		log.Printf("unable to destroy shm pool: %v", err)
	}
	if err := file.Close(); err != nil {
		log.Printf("unable to close file: %v", err)
	}

	// Draw image
	imgData := app.image.(*image.RGBA).Pix
	swizzle.BGRA(imgData)
	copy(data, imgData)

	if err := syscall.Munmap(data); err != nil {
		log.Printf("unable to delete mapping: %v", err)
	}
	buf.AddReleaseHandler(bufferReleaser{buf: buf})

	log.Print("drawing frame complete")
	return buf
}

func (app *appState) HandleSurfaceConfigure(ev xdg.SurfaceConfigureEvent) {
	if err := app.xdgSurface.AckConfigure(ev.Serial); err != nil {
		log.Fatal("unable to ack xdg surface configure")
	}

	buffer := app.drawFrame()

	if err := app.surface.Attach(buffer, 0, 0); err != nil {
		log.Fatalf("unable to attach buffer to surface: %v", err)
	}
	if err := app.surface.Commit(); err != nil {
		log.Fatalf("unable to commit surface state: %v", err)
	}
}

func (app *appState) HandleToplevelConfigure(ev xdg.ToplevelConfigureEvent) {
	width := ev.Width
	height := ev.Height

	if width == 0 || height == 0 {
		// Compositor is deferring to us
		return
	}

	app.image = resize.Resize(uint(width), uint(height), app.pImage, resize.Lanczos3)

	app.width = width
	app.height = height
}

type bufferReleaser struct {
	buf *wl.Buffer
}

func (b bufferReleaser) HandleBufferRelease(ev wl.BufferReleaseEvent) {
	if err := b.buf.Destroy(); err != nil {
		log.Printf("unable to destroy buffer: %v", err)
	}
}

type registrar struct {
	ch chan wl.RegistryGlobalEvent
}

func (r registrar) HandleRegistryGlobal(ev wl.RegistryGlobalEvent) {
	r.ch <- ev
}

type doner struct {
	ch chan wl.CallbackDoneEvent
}

func (d doner) HandleCallbackDone(ev wl.CallbackDoneEvent) {
	d.ch <- ev
}

func (app *appState) registerGlobalInterfaces() {
	registry, err := app.display.GetRegistry()
	if err != nil {
		log.Fatalf("unable to get global registry object: %v", err)
	}

	rgeChan := make(chan wl.RegistryGlobalEvent)
	rgeHandler := registrar{rgeChan}
	registry.AddGlobalHandler(rgeHandler)

	callback, err := app.display.Sync()
	if err != nil {
		log.Fatalf("unable to get sync callback: %v", err)
	}
	cdeChan := make(chan wl.CallbackDoneEvent)
	cdeHandler := doner{cdeChan}
	callback.AddDoneHandler(cdeHandler)

loop:
	for {
		select {
		case ev := <-rgeChan:
			log.Printf("we discovered an interface: %q\n", ev.Interface)

			switch ev.Interface {
			case "wl_shm":
				shm := wl.NewShm(app.display.Context())
				err := registry.Bind(ev.Name, ev.Interface, ev.Version, shm)
				if err != nil {
					log.Fatalf("unable to bind wl_shm interface: %v", err)
				}
				app.shm = shm
			case "wl_compositor":
				compositor := wl.NewCompositor(app.display.Context())
				err := registry.Bind(ev.Name, ev.Interface, ev.Version, compositor)
				if err != nil {
					log.Fatalf("unable to bind wl_compositor interface: %v", err)
				}
				app.compositor = compositor
			case "xdg_wm_base":
				wmBase := xdg.NewWmBase(app.display.Context())
				err := registry.Bind(ev.Name, ev.Interface, ev.Version, wmBase)
				if err != nil {
					log.Fatalf("unable to bind xdg_wm_base interface: %s", err)
				}
				app.wmBase = wmBase
				wmBase.AddPingHandler(app)
			case "wl_seat":
				seat := wl.NewSeat(app.display.Context())
				err := registry.Bind(ev.Name, ev.Interface, ev.Version, seat)
				if err != nil {
					log.Fatalf("unable to bind wl_seat interface: %v", err)
				}
				app.seat = seat
				seat.AddCapabilitiesHandler(app)
				seat.AddNameHandler(app)
			}
		case app.Dispatch() <- struct{}{}:
		case <-cdeChan:
			break loop
		}
	}
}

func (app *appState) attachPointer() {
	pointer, err := app.seat.GetPointer()
	if err != nil {
		log.Fatal("unable to register pointer interface")
	}
	app.pointer = pointer
	pointer.AddEnterHandler(app)
	pointer.AddLeaveHandler(app)
	pointer.AddMotionHandler(app)
	pointer.AddButtonHandler(app)
	pointer.AddAxisHandler(app)
	pointer.AddAxisSourceHandler(app)
	pointer.AddAxisStopHandler(app)
	pointer.AddAxisDiscreteHandler(app)
	pointer.AddFrameHandler(app)

	log.Print("pointer interface registered")
}

func (app *appState) releasePointer() {
	app.pointer.RemoveEnterHandler(app)
	app.pointer.RemoveLeaveHandler(app)
	app.pointer.RemoveMotionHandler(app)
	app.pointer.RemoveButtonHandler(app)
	app.pointer.RemoveAxisHandler(app)
	app.pointer.RemoveAxisSourceHandler(app)
	app.pointer.RemoveAxisStopHandler(app)
	app.pointer.RemoveAxisDiscreteHandler(app)
	app.pointer.RemoveFrameHandler(app)

	if err := app.pointer.Release(); err != nil {
		log.Println("unable to release pointer interface")
	}
	app.pointer = nil

	log.Print("pointer interface released")
}

func (app *appState) attachKeyboard() {
	keyboard, err := app.seat.GetKeyboard()
	if err != nil {
		log.Fatal("unable to register keyboard interface")
	}
	app.keyboard = keyboard

	keyboard.AddKeyHandler(app)

	log.Print("keyboard interface registered")
}

func (app *appState) releaseKeyboard() {
	app.keyboard.RemoveKeyHandler(app)

	if err := app.keyboard.Release(); err != nil {
		log.Println("unable to release keyboard interface")
	}
	app.keyboard = nil

	log.Print("keyboard interface released")
}

func (app *appState) HandleSeatCapabilities(ev wl.SeatCapabilitiesEvent) {
	havePointer := (ev.Capabilities * wl.SeatCapabilityPointer) != 0

	if havePointer && app.pointer == nil {
		app.attachPointer()
	} else if !havePointer && app.pointer != nil {
		app.releasePointer()
	}

	haveKeyboard := (ev.Capabilities * wl.SeatCapabilityKeyboard) != 0

	if haveKeyboard && app.keyboard == nil {
		app.attachKeyboard()
	} else if !haveKeyboard && app.keyboard != nil {
		app.releaseKeyboard()
	}
}

func (*appState) HandleSeatName(ev wl.SeatNameEvent) {
	log.Printf("seat name: %v", ev.Name)
}

// HandleDisplayError handles wl.Display errors
func (*appState) HandleDisplayError(ev wl.DisplayErrorEvent) {
	// Just log.Fatal for now
	log.Fatalf("display error event: %v", ev)
}

// HandleWmBasePing handles xdg ping by doing a Pong request
func (app *appState) HandleWmBasePing(ev xdg.WmBasePingEvent) {
	log.Printf("xdg_wmbase ping: serial=%v", ev.Serial)
	app.wmBase.Pong(ev.Serial)
	log.Print("xdg_wmbase pong sent")
}

func (app *appState) HandleToplevelClose(ev xdg.ToplevelCloseEvent) {
	close(app.exitChan)
}
