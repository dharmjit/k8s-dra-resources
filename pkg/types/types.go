package types

import "k8s.io/apimachinery/pkg/api/resource"

// NodeInfo holds all information about a node, including capacity and devices.
type NodeInfo struct {
	NodeName     string
	NodeRole     string
	NodeCapacity NodeCapacity
	Devices      []Device
}

// NodeCapacity holds the capacity information for a node.
type NodeCapacity struct {
	TotalCPU         resource.Quantity
	AvailableCPU     resource.Quantity
	TotalMemory      resource.Quantity
	AvailableMemory  resource.Quantity
	TotalStorage     resource.Quantity
	AvailableStorage resource.Quantity
}

// Device contains the relevant information for a device.
type Device struct {
	ProductName    string
	TotalCount     int
	AvailableCount int
	Memory         resource.Quantity
}
