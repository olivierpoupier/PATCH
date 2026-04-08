package main

import (
	"github.com/bluetuith-org/bluetooth-classic/api/bluetooth"
	"github.com/bluetuith-org/bluetooth-classic/api/config"
	"github.com/bluetuith-org/bluetooth-classic/session"
)

// adapterInfo holds the current state of the Bluetooth adapter.
type adapterInfo struct {
	Name        string
	Address     string
	Powered     bool
	Discovering bool
}

// deviceInfo holds information about a known Bluetooth device.
type deviceInfo struct {
	Name      string
	Address   bluetooth.MacAddress
	Type      string
	Connected bool
	Paired    bool
}

// btAdapter wraps the bluetooth-classic session and adapter.
type btAdapter struct {
	session bluetooth.Session
	adapter bluetooth.Adapter
	address bluetooth.MacAddress
}

func newBTAdapter() (*btAdapter, error) {
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

	return &btAdapter{
		session: sess,
		adapter: adapter,
		address: first.Address,
	}, nil
}

func (b *btAdapter) info() (adapterInfo, error) {
	props, err := b.adapter.Properties()
	if err != nil {
		return adapterInfo{}, err
	}

	name := props.Alias
	if name == "" {
		name = props.Name
	}

	return adapterInfo{
		Name:        name,
		Address:     props.Address.String(),
		Powered:     props.Powered,
		Discovering: props.Discovering,
	}, nil
}

func (b *btAdapter) devices() ([]deviceInfo, error) {
	devs, err := b.adapter.Devices()
	if err != nil {
		return nil, err
	}

	var result []deviceInfo
	for _, d := range devs {
		typeName := d.Type
		if typeName == "" {
			typeName = bluetooth.DeviceTypeFromClass(d.Class)
		}

		name := d.Name
		if name == "" {
			name = d.Address.String()
		}

		result = append(result, deviceInfo{
			Name:      name,
			Address:   d.Address,
			Type:      typeName,
			Connected: d.Connected,
			Paired:    d.Paired,
		})
	}

	return result, nil
}

func (b *btAdapter) togglePower() error {
	props, err := b.adapter.Properties()
	if err != nil {
		return err
	}
	return b.adapter.SetPoweredState(!props.Powered)
}

func (b *btAdapter) startDiscovery() error {
	return b.adapter.StartDiscovery()
}

func (b *btAdapter) stopDiscovery() error {
	return b.adapter.StopDiscovery()
}

func (b *btAdapter) connectDevice(addr bluetooth.MacAddress) error {
	return b.session.Device(addr).Connect()
}

func (b *btAdapter) disconnectDevice(addr bluetooth.MacAddress) error {
	return b.session.Device(addr).Disconnect()
}

func (b *btAdapter) close() error {
	return b.session.Stop()
}
