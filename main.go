package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

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
	ProductName string
	Memory      resource.Quantity
	Status      string
}

type DRAClient interface {
	GetResourceSlices(ctx context.Context) ([]resourcev1beta1.ResourceSlice, error)
	GetResourceClaims(ctx context.Context) ([]resourcev1beta1.ResourceClaim, error)
	GetNodes(ctx context.Context) ([]corev1.Node, error)
}

type draClient struct {
	typedClient kubernetes.Interface
}

func NewDRAClient(kubeconfigPath string) (DRAClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	typedClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create typed client: %w", err)
	}

	return &draClient{typedClient: typedClient}, nil
}

func (c *draClient) GetResourceSlices(ctx context.Context) ([]resourcev1beta1.ResourceSlice, error) {
	list, err := c.typedClient.ResourceV1beta1().ResourceSlices().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}
	return list.Items, nil
}

func (c *draClient) GetResourceClaims(ctx context.Context) ([]resourcev1beta1.ResourceClaim, error) {
	list, err := c.typedClient.ResourceV1beta1().ResourceClaims("").List(ctx, metav1.ListOptions{}) // "" for all namespaces
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceClaims: %w", err)
	}
	return list.Items, nil
}

func (c *draClient) GetNodes(ctx context.Context) ([]corev1.Node, error) {
	list, err := c.typedClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	return list.Items, nil
}

func main() {
	kubeconfig := flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "path to the kubeconfig file")
	flag.Parse()

	if *kubeconfig == "" {
		*kubeconfig = clientcmd.RecommendedHomeFile
	}

	client, err := NewDRAClient(*kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating DRA client: %v\n", err)
		os.Exit(1)
	}

	if err := displayTabularInfo(client); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying node info: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n------------------------------")
}

func formatMemoryAsGiB(q resource.Quantity) string {
	val, ok := q.AsInt64()
	if !ok {
		// Fallback for very large values that don't fit in int64
		return q.String()
	}
	gib := float64(val) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.2fGi", gib)
}

func displayTabularInfo(client DRAClient) error {
	ctx := context.Background()
	fmt.Println("Fetching node and resource info...")

	nodes, err := client.GetNodes(ctx)
	if err != nil {
		return err
	}

	resourceSlices, err := client.GetResourceSlices(ctx)
	if err != nil {
		return err
	}

	resourceClaims, err := client.GetResourceClaims(ctx)
	if err != nil {
		return err
	}

	allocatedDevices := make(map[string]map[string]bool)
	for _, rc := range resourceClaims {
		if rc.Status.Allocation != nil && len(rc.Status.Allocation.Devices.Results) > 0 {
			for _, ads := range rc.Status.Allocation.Devices.Results {
				sliceIdentifier := fmt.Sprintf("%s-%s", ads.Driver, ads.Pool)
				if _, ok := allocatedDevices[sliceIdentifier]; !ok {
					allocatedDevices[sliceIdentifier] = make(map[string]bool)
				}
				allocatedDevices[sliceIdentifier][ads.Device] = true
			}
		}
	}

	// Map to hold all info per node
	nodeMap := make(map[string]*NodeInfo)
	for _, node := range nodes {
		role := "<none>"
		for k := range node.Labels {
			if strings.HasPrefix(k, "node-role.kubernetes.io/") {
				role = strings.TrimPrefix(k, "node-role.kubernetes.io/")
				break
			}
		}

		nodeMap[node.Name] = &NodeInfo{
			NodeName: node.Name,
			NodeRole: role,
			NodeCapacity: NodeCapacity{
				TotalCPU:         node.Status.Capacity[corev1.ResourceCPU],
				AvailableCPU:     node.Status.Allocatable[corev1.ResourceCPU],
				TotalMemory:      node.Status.Capacity[corev1.ResourceMemory],
				AvailableMemory:  node.Status.Allocatable[corev1.ResourceMemory],
				TotalStorage:     node.Status.Capacity[corev1.ResourceStorage],
				AvailableStorage: node.Status.Allocatable[corev1.ResourceStorage],
			},
			Devices: []Device{},
		}
	}

	// Populate devices for each node
	for _, rs := range resourceSlices {
		nodeInfo, ok := nodeMap[rs.Spec.NodeName]
		if !ok {
			continue
		}

		sliceIdentifier := fmt.Sprintf("%s-%s", rs.Spec.Driver, rs.Spec.Pool.Name)
		for _, dev := range rs.Spec.Devices {
			status := "Available"
			if allocatedDevices[sliceIdentifier][dev.Name] {
				status = "Allocated"
			}
			var memory resource.Quantity
			if dev.Basic != nil {
				if mem, ok := dev.Basic.Capacity["memory"]; ok {
					memory = mem.Value
				}
			}

			var productName string
			productName = rs.Spec.Driver
			if productName == "gpu.nvidia.com" {
				if dev.Basic != nil {
					if attrProductName, ok := dev.Basic.Attributes["productName"]; ok {
						productName = *attrProductName.StringValue
					}
				}
			}
			nodeInfo.Devices = append(nodeInfo.Devices, Device{
				ProductName: productName,
				Memory:      memory,
				Status:      status,
			})
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Header for the new format
	fmt.Fprintln(w, "NODE\tROLE\tCPU(TOTAL/AVAIL)\tMEMORY(TOTAL/AVAIL GiB)\tSTORAGE(TOTAL/AVAIL)\tDEVICES")

	for _, node := range nodes {
		nodeInfo := nodeMap[node.Name]

		// Aggregate device info
		deviceCounts := make(map[string]map[string]int) // driver -> status -> count
		for _, dev := range nodeInfo.Devices {
			deviceAndMemoryName := dev.ProductName
			if !dev.Memory.IsZero() {
				deviceAndMemoryName += "+" + dev.Memory.String()
			}
			if _, ok := deviceCounts[deviceAndMemoryName]; !ok {
				deviceCounts[deviceAndMemoryName] = make(map[string]int)
			}
			deviceCounts[deviceAndMemoryName]["total"]++
			if dev.Status == "Allocated" {
				deviceCounts[deviceAndMemoryName]["allocated"]++
			}
		}

		// Create a string for the devices column
		var deviceString string
		if len(deviceCounts) == 0 {
			deviceString = "None"
		} else {
			var parts []string
			for driver, counts := range deviceCounts {
				total := counts["total"]
				allocated := counts["allocated"]
				available := total - allocated
				parts = append(parts, fmt.Sprintf("%s: %d total, %d available", driver, total, available))
			}
			deviceString = strings.Join(parts, "; ")
		}

		// Print the main row for the node
		fmt.Fprintf(w, "%s\t%s\t%s/%s\t%s/%s\t%s/%s\t%s\n",
			node.Name,
			nodeInfo.NodeRole,
			nodeInfo.NodeCapacity.TotalCPU.String(), nodeInfo.NodeCapacity.AvailableCPU.String(),
			formatMemoryAsGiB(nodeInfo.NodeCapacity.TotalMemory), formatMemoryAsGiB(nodeInfo.NodeCapacity.AvailableMemory),
			nodeInfo.NodeCapacity.TotalStorage.String(), nodeInfo.NodeCapacity.AvailableStorage.String(),
			deviceString,
		)
	}

	return nil
}
