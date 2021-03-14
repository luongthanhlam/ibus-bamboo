package wl

import (
	"bytes"
	"fmt"
	"syscall"

	"github.com/rajveermalviya/wl/internal/byteorder"
)

type Event struct {
	pid    ProxyID
	Opcode uint32
	data   []byte
	scms   []syscall.SocketControlMessage
	off    int
}

func (ctx *Context) readEvent() (*Event, error) {
	buf := make([]byte, 8)
	control := make([]byte, 24)

	n, oobn, _, _, err := ctx.conn.ReadMsgUnix(buf[:], control)
	if err != nil {
		return nil, err
	}
	if n != 8 {
		return nil, fmt.Errorf("unable to read message header")
	}
	ev := &Event{}
	if oobn > 0 {
		if oobn > len(control) {
			return nil, fmt.Errorf("insufficient control msg buffer")
		}
		scms, err2 := syscall.ParseSocketControlMessage(control)
		if err2 != nil {
			return nil, fmt.Errorf("control message parse error: %s", err)
		}
		ev.scms = scms
	}

	ev.pid = ProxyID(byteorder.NativeEndian.Uint32(buf[0:4]))
	ev.Opcode = uint32(byteorder.NativeEndian.Uint16(buf[4:6]))
	size := uint32(byteorder.NativeEndian.Uint16(buf[6:8]))

	// subtract 8 bytes from header
	data := make([]byte, int(size)-8)
	n, err = ctx.conn.Read(data)
	if err != nil {
		return nil, err
	}
	if n != int(size)-8 {
		return nil, fmt.Errorf("invalid message size")
	}
	ev.data = data

	return ev, nil
}

func (ev *Event) FD() uintptr {
	if ev.scms == nil {
		return 0
	}
	fds, err := syscall.ParseUnixRights(&ev.scms[0])
	if err != nil {
		panic("unable to parse unix rights")
	}
	// TODO is this required
	ev.scms = append(ev.scms, ev.scms[1:]...)
	return uintptr(fds[0])
}

func (ev *Event) Uint32() uint32 {
	buf := ev.next(4)
	if len(buf) != 4 {
		panic("unable to read unsigned int")
	}
	return byteorder.NativeEndian.Uint32(buf)
}

func (ev *Event) Proxy(ctx *Context) Proxy {
	return ctx.lookupProxy(ProxyID(ev.Uint32()))
}

func (ev *Event) String() string {
	l := int(ev.Uint32())
	buf := ev.next(l)
	if len(buf) != l {
		panic("unable to read string")
	}
	ret := string(bytes.TrimRight(buf, "\x00"))
	// padding to 32 bit boundary
	if (l & 0x3) != 0 {
		ev.next(4 - (l & 0x3))
	}
	return ret
}

func (ev *Event) Int32() int32 {
	return int32(ev.Uint32())
}

func (ev *Event) Float32() float32 {
	return float32(fixedToFloat64(ev.Int32()))
}

func (ev *Event) Array() []int32 {
	l := int(ev.Uint32())
	arr := make([]int32, l/4)
	for i := range arr {
		arr[i] = ev.Int32()
	}
	return arr
}

func (ev *Event) next(n int) []byte {
	ret := ev.data[ev.off : ev.off+n]
	ev.off += n
	return ret
}
