package librtmp

// #cgo LDFLAGS: -lrtmp
// #include <librtmp/rtmp.h>
// #include <librtmp/log.h>
// #include <stdlib.h>
import "C"

import (
	"io"
	"unsafe"
)

type RTMP struct {
	rtmp *C.struct_RTMP
	url  *C.char
	io.ReadCloser
}

func Alloc() (*RTMP, error) {
	r := &RTMP{}
	rtmp, err := C.RTMP_Alloc()
	if err != nil {
		return nil, err
	}
	r.rtmp = rtmp

	return r, err
}

func (r *RTMP) Init() {
	C.RTMP_Init(r.rtmp)
}

func (r *RTMP) SetupURL(url string) error {
	r.url = C.CString(url)
	_, err := C.RTMP_SetupURL(r.rtmp, r.url)
	return err
}

func (r *RTMP) IsConnected() bool {
	ret := C.RTMP_IsConnected(r.rtmp)
	return (ret != 0)
}

func (r *RTMP) IsTimedout() bool {
	ret := C.RTMP_IsTimedout(r.rtmp)
	return (ret != 0)
}

func (r *RTMP) Connect() error {
	ret, err := C.RTMP_Connect(r.rtmp, nil)
	if ret != 0 {
		return err
	}
	return nil
}

func (r *RTMP) LogSetLevel(loglevel int) {
	C.RTMP_LogSetLevel(C.RTMP_LogLevel(loglevel))
}

func (r *RTMP) Read(p []byte) (n int, err error) {
	size, err := C.RTMP_Read(r.rtmp, (*C.char)(unsafe.Pointer(&p[0])), C.int(len(p)))
	return int(size), nil
}

func (r *RTMP) Close() error {
	_, err := C.RTMP_Close(r.rtmp)
	if err != nil {
		return err
	}
	return nil
}

func (r *RTMP) Free() error {
	_, err := C.RTMP_Free(r.rtmp)
	if err != nil {
		return err
	}
	C.free(unsafe.Pointer(r.url))
	return nil
}
