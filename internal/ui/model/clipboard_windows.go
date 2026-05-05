//go:build windows && !arm && !386

package model

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"syscall"
	"unsafe"

	nativeclipboard "github.com/aymanbagabas/go-nativeclipboard"
	"golang.org/x/image/bmp"
)

const (
	winCfDIB    = 8
	winCfDIBV5  = 17
	winCfBitmap = 2
)

var (
	user32Win   = syscall.NewLazyDLL("user32.dll")
	kernel32Win = syscall.NewLazyDLL("kernel32.dll")

	winOpenClipboard               = user32Win.NewProc("OpenClipboard")
	winCloseClipboard              = user32Win.NewProc("CloseClipboard")
	winGetClipboardData            = user32Win.NewProc("GetClipboardData")
	winIsClipboardFormatAvailable  = user32Win.NewProc("IsClipboardFormatAvailable")

	winGlobalLock   = kernel32Win.NewProc("GlobalLock")
	winGlobalUnlock = kernel32Win.NewProc("GlobalUnlock")
)

type winBitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type winBitmapV5Header struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
	RedMask       uint32
	GreenMask     uint32
	BlueMask      uint32
	AlphaMask     uint32
	CSType        uint32
	Endpoints     [36]byte
	GammaRed      uint32
	GammaGreen    uint32
	GammaBlue     uint32
	Intent        uint32
	ProfileData   uint32
	ProfileSize   uint32
	Reserved      uint32
}

func readClipboard(f clipboardFormat) ([]byte, error) {
	switch f {
	case clipboardFormatText:
		return nativeclipboard.Text.Read()
	case clipboardFormatImage:
		data, err := nativeclipboard.Image.Read()
		if err == nil {
			return data, nil
		}
		// Fallback: try reading DIBv5 with any bit depth, then DIB.
		return readClipboardImageFallback()
	}
	return nil, errClipboardUnknownFormat
}

func readClipboardImageFallback() ([]byte, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Open clipboard.
	r, _, _ := winOpenClipboard.Call(0)
	if r == 0 {
		return nil, errClipboardPlatformUnsupported
	}
	defer winCloseClipboard.Call()

	// Try CF_DIBV5 first (handles any bit depth).
	if data, err := readDIBV5Any(); err == nil {
		return data, nil
	}

	// Try CF_DIB.
	return readDIBAny()
}

func readDIBV5Any() ([]byte, error) {
	r, _, _ := winIsClipboardFormatAvailable.Call(winCfDIBV5)
	if r == 0 {
		return nil, errClipboardPlatformUnsupported
	}

	hMem, _, _ := winGetClipboardData.Call(winCfDIBV5)
	if hMem == 0 {
		return nil, errClipboardPlatformUnsupported
	}

	p, _, _ := winGlobalLock.Call(hMem)
	if p == 0 {
		return nil, errClipboardPlatformUnsupported
	}
	defer winGlobalUnlock.Call(hMem)

	header := (*winBitmapV5Header)(unsafe.Pointer(p))
	width := int(header.Width)
	height := int(header.Height)
	if height < 0 {
		height = -height
	}
	bitCount := int(header.BitCount)

	switch bitCount {
	case 32:
		return readDIBV5_32(p, header, width, height)
	case 24:
		return readDIBV5_24(p, header, width, height)
	default:
		return nil, errClipboardPlatformUnsupported
	}
}

func readDIBV5_32(p uintptr, header *winBitmapV5Header, width, height int) ([]byte, error) {
	dataSize := int(header.Size) + 4*width*height
	data := unsafe.Slice((*byte)(unsafe.Pointer(p)), dataSize)

	bottomUp := header.Height > 0
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	offset := int(header.Size)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := offset + 4*(y*width+x)
			var yOut int
			if bottomUp {
				yOut = height - 1 - y
			} else {
				yOut = y
			}
			b := data[idx+0]
			g := data[idx+1]
			r := data[idx+2]
			a := data[idx+3]
			img.SetRGBA(x, yOut, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readDIBV5_24(p uintptr, header *winBitmapV5Header, width, height int) ([]byte, error) {
	stride := (width*3 + 3) &^ 3 // Rows are aligned to 4 bytes.
	dataSize := int(header.Size) + stride*height
	data := unsafe.Slice((*byte)(unsafe.Pointer(p)), dataSize)

	bottomUp := header.Height > 0
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	offset := int(header.Size)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := offset + y*stride + x*3
			var yOut int
			if bottomUp {
				yOut = height - 1 - y
			} else {
				yOut = y
			}
			b := data[idx+0]
			g := data[idx+1]
			r := data[idx+2]
			img.SetRGBA(x, yOut, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readDIBAny() ([]byte, error) {
	r, _, _ := winIsClipboardFormatAvailable.Call(winCfDIB)
	if r == 0 {
		return nil, errClipboardPlatformUnsupported
	}

	hMem, _, _ := winGetClipboardData.Call(winCfDIB)
	if hMem == 0 {
		return nil, errClipboardPlatformUnsupported
	}

	p, _, _ := winGlobalLock.Call(hMem)
	if p == 0 {
		return nil, errClipboardPlatformUnsupported
	}
	defer winGlobalUnlock.Call(hMem)

	header := (*winBitmapInfoHeader)(unsafe.Pointer(p))
	height := int(header.Height)
	if height < 0 {
		height = -height
	}

	const fileHeaderLen = 14
	infoHeaderLen := int(header.Size)

	sizeImage := int(header.SizeImage)
	if sizeImage == 0 && header.Compression == 0 {
		stride := (int(header.Width)*int(header.BitCount)/8 + 3) &^ 3
		sizeImage = stride * height
	}

	totalSize := fileHeaderLen + infoHeaderLen + sizeImage

	buf := new(bytes.Buffer)
	buf.Grow(totalSize)
	binary.Write(buf, binary.LittleEndian, uint16('B')|(uint16('M')<<8))
	binary.Write(buf, binary.LittleEndian, uint32(totalSize))
	binary.Write(buf, binary.LittleEndian, uint32(0))
	binary.Write(buf, binary.LittleEndian, uint32(fileHeaderLen+infoHeaderLen))

	dibData := unsafe.Slice((*byte)(unsafe.Pointer(p)), infoHeaderLen+sizeImage)
	buf.Write(dibData)

	// Decode BMP to PNG.
	img, err := bmp.Decode(buf)
	if err != nil {
		return nil, err
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, err
	}
	return pngBuf.Bytes(), nil
}
