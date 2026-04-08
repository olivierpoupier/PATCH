package bluetooth

import (
	"log/slog"

	"github.com/bluetuith-org/bluetooth-classic/api/bluetooth"
	"github.com/bluetuith-org/bluetooth-classic/api/config"
	"github.com/bluetuith-org/bluetooth-classic/session"
)

// AdapterInfo holds the current state of the Bluetooth adapter.
type AdapterInfo struct {
	Name        string
	Address     string
	Powered     bool
	Discovering bool
}

// DeviceInfo holds information about a known Bluetooth device.
type DeviceInfo struct {
	Name      string
	Address   bluetooth.MacAddress
	Type      string
	Connected bool
	Paired    bool
}

// Adapter wraps the bluetooth-classic session and adapter.
type Adapter struct {
	session bluetooth.Session
	adapter bluetooth.Adapter
	address bluetooth.MacAddress
}

// NewAdapter initializes a Bluetooth session and returns the first adapter.
func NewAdapter() (*Adapter, error) {
	slog.Debug("starting bluetooth session")
	sess := session.NewSession()
	cfg := config.New()

	_, _, err := sess.Start(&bluetooth.DefaultAuthorizer{}, cfg)
	if err != nil {
		return nil, err
	}

	adapters, err := sess.Adapters()
	if err != nil {
		sess.Stop()
		return nil, err
	}

	if len(adapters) == 0 {
		sess.Stop()
		return nil, err
	}

	first := adapters[0]
	adapter := sess.Adapter(first.Address)
	slog.Debug("adapter selected", "address", first.Address)

	return &Adapter{
		session: sess,
		adapter: adapter,
		address: first.Address,
	}, nil
}

// Info retrieves the current adapter properties.
func (a *Adapter) Info() (AdapterInfo, error) {
	props, err := a.adapter.Properties()
	if err != nil {
		return AdapterInfo{}, err
	}

	name := props.Alias
	if name == "" {
		name = props.Name
	}

	return AdapterInfo{
		Name:        name,
		Address:     props.Address.String(),
		Powered:     props.Powered,
		Discovering: props.Discovering,
	}, nil
}

// Devices lists all known devices with metadata.
func (a *Adapter) Devices() ([]DeviceInfo, error) {
	devs, err := a.adapter.Devices()
	if err != nil {
		return nil, err
	}

	var result []DeviceInfo
	for _, d := range devs {
		typeName := d.Type
		if typeName == "" {
			typeName = bluetooth.DeviceTypeFromClass(d.Class)
		}

		name := d.Name
		if name == "" {
			name = d.Address.String()
		}

		result = append(result, DeviceInfo{
			Name:      name,
			Address:   d.Address,
			Type:      typeName,
			Connected: d.Connected,
			Paired:    d.Paired,
		})
	}

	return result, nil
}

// TogglePower flips the adapter power state.
func (a *Adapter) TogglePower() error {
	props, err := a.adapter.Properties()
	if err != nil {
		return err
	}
	return a.adapter.SetPoweredState(!props.Powered)
}

// StartDiscovery begins device scanning.
func (a *Adapter) StartDiscovery() error {
	slog.Debug("starting discovery")
	return a.adapter.StartDiscovery()
}

// StopDiscovery stops device scanning.
func (a *Adapter) StopDiscovery() error {
	return a.adapter.StopDiscovery()
}

// ConnectDevice connects to a device by address.
func (a *Adapter) ConnectDevice(addr bluetooth.MacAddress) error {
	slog.Debug("connecting to device", "address", addr)
	return a.session.Device(addr).Connect()
}

// DisconnectDevice disconnects from a device by address.
func (a *Adapter) DisconnectDevice(addr bluetooth.MacAddress) error {
	slog.Debug("disconnecting from device", "address", addr)
	return a.session.Device(addr).Disconnect()
}

// Close cleans up the session.
func (a *Adapter) Close() error {
	return a.session.Stop()
}
