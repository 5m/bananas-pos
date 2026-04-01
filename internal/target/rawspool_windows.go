//go:build windows

package target

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"bananas-pos/internal/job"
)

var (
	winspoolDLL             = windows.NewLazySystemDLL("winspool.drv")
	openPrinterProc         = winspoolDLL.NewProc("OpenPrinterW")
	closePrinterProc        = winspoolDLL.NewProc("ClosePrinter")
	startDocPrinterProc     = winspoolDLL.NewProc("StartDocPrinterW")
	endDocPrinterProc       = winspoolDLL.NewProc("EndDocPrinter")
	startPagePrinterProc    = winspoolDLL.NewProc("StartPagePrinter")
	endPagePrinterProc      = winspoolDLL.NewProc("EndPagePrinter")
	writePrinterProc        = winspoolDLL.NewProc("WritePrinter")
	enumPrintersProc        = winspoolDLL.NewProc("EnumPrintersW")
	getDefaultPrinterProc   = winspoolDLL.NewProc("GetDefaultPrinterW")
	errUnsupportedOperation = errors.New("operation not supported by system print queue")
)

type RawSpool struct {
	printerName string
}

type docInfo1 struct {
	docName    *uint16
	outputFile *uint16
	dataType   *uint16
}

func NewRawSpool(printerName string) *RawSpool {
	return &RawSpool{printerName: printerName}
}

func (r *RawSpool) Name() string {
	return "system-print-queue"
}

func (r *RawSpool) Send(_ context.Context, printJob job.PrintJob) error {
	if len(printJob.Raw) == 0 {
		return errors.New("print job payload is empty")
	}

	printerName, err := r.resolvePrinterName()
	if err != nil {
		return fmt.Errorf("resolve default printer: %w", err)
	}

	handle, err := openPrinter(printerName)
	if err != nil {
		return fmt.Errorf("open printer %q: %w", printerName, err)
	}
	defer closePrinter(handle)

	docName, err := windows.UTF16PtrFromString(spoolTitle(printJob))
	if err != nil {
		return err
	}
	rawDataType, err := windows.UTF16PtrFromString("RAW")
	if err != nil {
		return err
	}

	doc := docInfo1{
		docName:  docName,
		dataType: rawDataType,
	}
	jobID, err := startDocPrinter(handle, &doc)
	if err != nil {
		return fmt.Errorf("start print job: %w", err)
	}
	if jobID == 0 {
		return errUnsupportedOperation
	}
	defer endDocPrinter(handle)

	if err := startPagePrinter(handle); err != nil {
		return fmt.Errorf("start print page: %w", err)
	}
	defer endPagePrinter(handle)

	written, err := writePrinter(handle, printJob.Raw)
	if err != nil {
		return fmt.Errorf("write print job: %w", err)
	}
	if written != len(printJob.Raw) {
		return fmt.Errorf("write print job: short write %d/%d", written, len(printJob.Raw))
	}

	return nil
}

func (r *RawSpool) Health(context.Context) error {
	printerName, err := r.resolvePrinterName()
	if err != nil {
		return fmt.Errorf("resolve default printer: %w", err)
	}

	handle, err := openPrinter(printerName)
	if err != nil {
		return fmt.Errorf("open printer %q: %w", printerName, err)
	}
	return closePrinter(handle)
}

func (r *RawSpool) Description(context.Context) (string, error) {
	printerName, err := r.resolvePrinterName()
	if err != nil {
		return "", fmt.Errorf("resolve default printer: %w", err)
	}
	return printerName, nil
}

func (r *RawSpool) AvailablePrinters(context.Context) ([]string, error) {
	return enumPrinterNames()
}

func spoolTitle(printJob job.PrintJob) string {
	if printJob.ID != "" {
		return printJob.ID
	}
	if printJob.Source != "" {
		return "bananas-pos-" + printJob.Source
	}
	return "bananas-pos"
}

func (r *RawSpool) resolvePrinterName() (string, error) {
	if r.printerName != "" {
		return r.printerName, nil
	}
	return defaultPrinterName()
}

func defaultPrinterName() (string, error) {
	var size uint32
	r1, _, err := getDefaultPrinterProc.Call(0, uintptr(unsafe.Pointer(&size)))
	if r1 == 0 && err != windows.ERROR_INSUFFICIENT_BUFFER {
		if err != nil && err != windows.ERROR_SUCCESS {
			return "", err
		}
		return "", windows.ERROR_FILE_NOT_FOUND
	}
	if size == 0 {
		return "", windows.ERROR_FILE_NOT_FOUND
	}

	buffer := make([]uint16, size)
	r1, _, err = getDefaultPrinterProc.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return "", err
		}
		return "", windows.ERROR_FILE_NOT_FOUND
	}

	return windows.UTF16ToString(buffer), nil
}

type printerInfo4 struct {
	printerName *uint16
	serverName  *uint16
	attributes  uint32
}

func enumPrinterNames() ([]string, error) {
	const (
		printerEnumLocal       = 0x2
		printerEnumConnections = 0x4
		level                  = 4
	)

	var needed uint32
	var returned uint32
	r1, _, err := enumPrintersProc.Call(
		uintptr(printerEnumLocal|printerEnumConnections),
		0,
		uintptr(level),
		0,
		0,
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if r1 == 0 && err != windows.ERROR_INSUFFICIENT_BUFFER {
		if err != nil && err != windows.ERROR_SUCCESS {
			return nil, err
		}
		return nil, errUnsupportedOperation
	}
	if needed == 0 {
		return nil, nil
	}

	buffer := make([]byte, needed)
	r1, _, err = enumPrintersProc.Call(
		uintptr(printerEnumLocal|printerEnumConnections),
		0,
		uintptr(level),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(needed),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if r1 == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return nil, err
		}
		return nil, errUnsupportedOperation
	}

	infos := unsafe.Slice((*printerInfo4)(unsafe.Pointer(&buffer[0])), returned)
	printers := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.printerName == nil {
			continue
		}
		name := windows.UTF16PtrToString(info.printerName)
		if name == "" {
			continue
		}
		printers = append(printers, name)
	}
	return printers, nil
}

func openPrinter(name string) (windows.Handle, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}

	var handle windows.Handle
	r1, _, callErr := openPrinterProc.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(&handle)),
		0,
	)
	if r1 == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return 0, callErr
		}
		return 0, errUnsupportedOperation
	}

	return handle, nil
}

func closePrinter(handle windows.Handle) error {
	if handle == 0 {
		return nil
	}

	r1, _, err := closePrinterProc.Call(uintptr(handle))
	if r1 == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return err
		}
		return errUnsupportedOperation
	}

	return nil
}

func startDocPrinter(handle windows.Handle, doc *docInfo1) (uint32, error) {
	r1, _, err := startDocPrinterProc.Call(
		uintptr(handle),
		uintptr(1),
		uintptr(unsafe.Pointer(doc)),
	)
	if r1 == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return 0, err
		}
		return 0, errUnsupportedOperation
	}

	return uint32(r1), nil
}

func endDocPrinter(handle windows.Handle) error {
	r1, _, err := endDocPrinterProc.Call(uintptr(handle))
	if r1 == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return err
		}
		return errUnsupportedOperation
	}
	return nil
}

func startPagePrinter(handle windows.Handle) error {
	r1, _, err := startPagePrinterProc.Call(uintptr(handle))
	if r1 == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return err
		}
		return errUnsupportedOperation
	}
	return nil
}

func endPagePrinter(handle windows.Handle) error {
	r1, _, err := endPagePrinterProc.Call(uintptr(handle))
	if r1 == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return err
		}
		return errUnsupportedOperation
	}
	return nil
}

func writePrinter(handle windows.Handle, data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	var written uint32
	r1, _, err := writePrinterProc.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(uint32(len(data))),
		uintptr(unsafe.Pointer(&written)),
	)
	if r1 == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return 0, err
		}
		return 0, errUnsupportedOperation
	}

	return int(written), nil
}
