package shout

import (
	"fmt"
	"net/url"
	"unsafe"
)

/*
#cgo LDFLAGS: -lshout
#include <stdlib.h>
#include <shout/shout.h>
*/
import "C"

const (
	SHOUT_BUFFER_SIZE = 4096
)

const (
	// See shout.h
	SHOUTERR_SUCCESS     = 0  
	SHOUTERR_INSANE      = -1 
	SHOUTERR_NOCORRECT   = -2
	SHOUTERR_NOLOGIN     = -3
	SHOUTERR_SOCKET      = -4
	SHOUTERR_MALLOC      = -5
	SHOUTERR_METADATA    = -6
	SHOUTERR_CONNECTED   = -7
	SHOUTERR_UNCONNECTED = -8
	SHOUTERR_UNSUPPORTED = -9
	SHOUTERR_BUSY        = -10
)

const (
	SHOUT_FORMAT_OGG = iota
	SHOUT_FORMAT_MP3
	SHOUT_FORMAT_WEBM
)

const (
	SHOUT_PROTOCOL_HTTP = iota
	SHOUT_PROTOCOL_XAUDIOCAST
	SHOUT_PROTOCOL_ICY
)

type ShoutError struct {
	Message string
	Code    int
}

func (e ShoutError) Error() string {
	return fmt.Sprintf("%s (%d)", e.Message, e.Code)
}

type Shout struct {
	Host     url.URL
	Port     uint16
	User     string
	Password string
	Mount    string
	Format   int
	Protocol int

	// wrap the native C struct
	struc *C.struct_shout

	stream chan []byte
}

func init() {
	C.shout_init()
}

func Shutdown() {
	C.shout_shutdown()
}

func Free(s *Shout) {
	C.shout_free(s.struc)
}

func (s *Shout) lazyInit() {
	if s.struc != nil {
		return
	}

	s.struc = C.shout_new()
	s.updateParameters()

	s.stream = make(chan []byte)
}

func (s *Shout) updateParameters() {
	// set hostname
	p := C.CString(s.Host.String())
	C.shout_set_host(s.struc, p)
	C.free(unsafe.Pointer(p))

	// set port
	C.shout_set_port(s.struc, C.ushort(s.Port))

	// set username
	p = C.CString(s.User)
	C.shout_set_user(s.struc, p)
	C.free(unsafe.Pointer(p))

	// set password
	p = C.CString(s.Password)
	C.shout_set_password(s.struc, p)
	C.free(unsafe.Pointer(p))

	// set mount point
	p = C.CString(s.Mount)
	C.shout_set_mount(s.struc, p)
	C.free(unsafe.Pointer(p))

	// set format
	C.shout_set_format(s.struc, C.uint(s.Format))

	// set protocol
	C.shout_set_protocol(s.struc, C.uint(s.Protocol))
}

func (s *Shout) GetError() string {
	s.lazyInit()
	err := C.shout_get_error(s.struc)
	return C.GoString(err)
}

func (s *Shout) Open() (chan<- []byte, error) {
	s.lazyInit()

	errcode := int(C.shout_open(s.struc))
	if errcode != C.SHOUTERR_SUCCESS {
		return nil, ShoutError{
			Code:    errcode,
			Message: s.GetError(),
		}
	}

	go s.handleStream()

	return s.stream, nil
}

func (s *Shout) Close() error {
	errcode := int(C.shout_close(s.struc))
	if errcode != C.SHOUTERR_SUCCESS {
		return ShoutError{
			Code:    errcode,
			Message: s.GetError(),
		}
	}

	return nil
}

func (s *Shout) send(buffer []byte) error {
	ptr := (*C.uchar)(&buffer[0])
	C.shout_send(s.struc, ptr, C.size_t(len(buffer)))

	errno := int(C.shout_get_errno(s.struc))
	if errno != C.SHOUTERR_SUCCESS {
		fmt.Println("something went wrong: %d", errno)
	}

	C.shout_sync(s.struc)
	return nil
}

func (s *Shout) handleStream() {
	for buf := range s.stream {
		s.send(buf)
	}
	fmt.Println("end handle")
}