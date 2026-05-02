package serialterm

import (
	"io"

	"go.bug.st/serial"
)

// Transport is the byte-stream abstraction a Session reads from and writes to.
// The SerialTransport implementation ships with this package; future transports
// (HID, TCP, USB bulk) can satisfy the same interface without changes to
// Session or device packages.
type Transport interface {
	io.ReadWriteCloser
}

// OpenSerial opens a serial port at the given path and baud rate with the
// standard 8N1 framing used by every USB-CDC device we currently target.
func OpenSerial(path string, baud int) (Transport, error) {
	mode := &serial.Mode{
		BaudRate: baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	return serial.Open(path, mode)
}
