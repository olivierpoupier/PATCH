package usb

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// USBDevice represents a single USB device or hub in the tree.
type USBDevice struct {
	Name         string
	VendorName   string
	VendorID     string
	ProductID    string
	LinkSpeed    string
	HardwareType string // "Built-in", "Removable", "Non-removable"
	LocationID   string
	SerialNumber string
	Type         DeviceType
	Children     []*USBDevice
}

// USBBus represents a USB host controller bus.
type USBBus struct {
	Name         string
	HardwareType string
	LocationID   string
	Devices      []*USBDevice
}

// ThunderboltPort represents a physical Thunderbolt/USB4 receptacle.
type ThunderboltPort struct {
	BusName      string // "thunderboltusb4_bus_0"
	ReceptacleID string // "1", "2"
	CurrentSpeed string // "Up to 40 Gb/s"
}

// PortGroup is the top-level display grouping: a physical port and its USB buses.
type PortGroup struct {
	Port  *ThunderboltPort
	Label string
	Buses []*USBBus
}

// TreeNode is a flattened, visible row in the tree used for rendering and navigation.
type TreeNode struct {
	Label       string
	TypeLabel   string
	Depth       int
	IsLast      bool
	Ancestors   []bool // for each depth < Depth, true if ancestor was last child
	HasChildren bool
	Collapsed   bool
	NodeID      string // LocationID, stable across refreshes
	IsHeader    bool
	IsBus       bool
	IsBuiltIn   bool
	Device      *USBDevice
}

// buildPortGroups correlates USB buses with Thunderbolt ports and produces display groups.
func buildPortGroups(buses []USBBus, ports []ThunderboltPort) []PortGroup {
	// Index TB ports by bus suffix number.
	portByIdx := make(map[int]*ThunderboltPort)
	for i := range ports {
		parts := strings.Split(ports[i].BusName, "_")
		if idx, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			portByIdx[idx] = &ports[i]
		}
	}

	// Index USB buses by high byte of LocationID.
	type indexedBus struct {
		idx int
		bus *USBBus
	}
	var indexed []indexedBus
	for i := range buses {
		busIdx := busIndex(buses[i].LocationID)
		indexed = append(indexed, indexedBus{idx: busIdx, bus: &buses[i]})
	}
	sort.Slice(indexed, func(i, j int) bool { return indexed[i].idx < indexed[j].idx })

	// Collect all indices.
	seen := make(map[int]bool)
	for _, ib := range indexed {
		seen[ib.idx] = true
	}
	for idx := range portByIdx {
		seen[idx] = true
	}
	var indices []int
	for idx := range seen {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	var groups []PortGroup
	for _, idx := range indices {
		port := portByIdx[idx]

		label := fmt.Sprintf("USB Bus %d", idx)
		if port != nil {
			label = fmt.Sprintf("Port %s (USB-C)", port.ReceptacleID)
		}

		g := PortGroup{Port: port, Label: label}
		for _, ib := range indexed {
			if ib.idx == idx {
				g.Buses = append(g.Buses, ib.bus)
			}
		}
		groups = append(groups, g)
	}
	return groups
}

// busIndex extracts the bus index from a LocationID like "0x01000000" → 1.
func busIndex(locationID string) int {
	s := strings.TrimPrefix(locationID, "0x")
	if len(s) < 2 {
		return 0
	}
	idx, _ := strconv.ParseInt(s[:2], 16, 32)
	return int(idx)
}

// flattenTree converts PortGroups into a flat slice of visible TreeNodes,
// respecting collapse state.
func flattenTree(groups []PortGroup, collapsed map[string]bool) []TreeNode {
	var nodes []TreeNode
	for _, g := range groups {
		// Port header.
		nodes = append(nodes, TreeNode{
			Label:    g.Label,
			IsHeader: true,
			Depth:    0,
			NodeID:   "port-" + g.Label,
		})

		for bi, bus := range g.Buses {
			busIsLast := bi == len(g.Buses)-1
			busID := bus.LocationID
			hasDev := len(bus.Devices) > 0
			nodes = append(nodes, TreeNode{
				Label:       bus.Name,
				IsBus:       true,
				Depth:       1,
				IsLast:      busIsLast,
				NodeID:      busID,
				HasChildren: hasDev,
				Collapsed:   collapsed[busID],
				IsBuiltIn:   bus.HardwareType == "Built-in",
			})
			if collapsed[busID] || !hasDev {
				continue
			}
			flattenDevices(&nodes, bus.Devices, 2, []bool{busIsLast}, collapsed)
		}
	}
	return nodes
}

func flattenDevices(nodes *[]TreeNode, devices []*USBDevice, depth int, ancestors []bool, collapsed map[string]bool) {
	for i, dev := range devices {
		isLast := i == len(devices)-1
		id := dev.LocationID
		anc := make([]bool, len(ancestors))
		copy(anc, ancestors)

		node := TreeNode{
			Label:       formatDeviceName(dev),
			TypeLabel:   dev.Type.String(),
			Depth:       depth,
			IsLast:      isLast,
			Ancestors:   anc,
			HasChildren: len(dev.Children) > 0,
			Collapsed:   collapsed[id],
			Device:      dev,
			NodeID:      id,
			IsBuiltIn:   dev.HardwareType == "Built-in" || dev.HardwareType == "Non-removable",
		}
		*nodes = append(*nodes, node)

		if !collapsed[id] && len(dev.Children) > 0 {
			childAnc := append(append([]bool(nil), ancestors...), isLast)
			flattenDevices(nodes, dev.Children, depth+1, childAnc, collapsed)
		}
	}
}

func formatDeviceName(dev *USBDevice) string {
	if dev.VendorName != "" {
		return fmt.Sprintf("%s (%s)", dev.Name, dev.VendorName)
	}
	return dev.Name
}

// countDevices returns the total number of devices (excluding buses) in the tree.
func countDevices(groups []PortGroup) int {
	count := 0
	for _, g := range groups {
		for _, bus := range g.Buses {
			count += countDevicesIn(bus.Devices)
		}
	}
	return count
}

func countDevicesIn(devices []*USBDevice) int {
	n := len(devices)
	for _, d := range devices {
		n += countDevicesIn(d.Children)
	}
	return n
}

// TableRow is a flat row for the peripheral table view.
type TableRow struct {
	Name   string
	Type   string
	Port   string
	Device *USBDevice
}

// collectTableRows extracts leaf, removable peripherals into a flat table.
func collectTableRows(groups []PortGroup) []TableRow {
	var rows []TableRow
	for _, g := range groups {
		for _, bus := range g.Buses {
			collectDeviceRows(&rows, bus.Devices, g.Label)
		}
	}
	return rows
}

func collectDeviceRows(rows *[]TableRow, devices []*USBDevice, port string) {
	for _, dev := range devices {
		// Skip built-in / non-removable devices.
		if dev.HardwareType == "Built-in" || dev.HardwareType == "Non-removable" {
			continue
		}
		// Skip hubs — recurse into their children instead.
		if dev.Type == DeviceTypeHub {
			collectDeviceRows(rows, dev.Children, port)
			continue
		}
		*rows = append(*rows, TableRow{
			Name:   formatDeviceName(dev),
			Type:   dev.Type.String(),
			Port:   port,
			Device: dev,
		})
		// Also collect any children that aren't hubs/built-in.
		collectDeviceRows(rows, dev.Children, port)
	}
}

// truncateText truncates s to maxWidth runes, appending "…" if truncated.
func truncateText(s string, maxWidth int) string {
	if maxWidth <= 1 {
		return "…"
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	return string(runes[:maxWidth-1]) + "…"
}

// padRight pads s with spaces to width. If s is longer, it is returned as-is.
func padRight(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(r))
}
