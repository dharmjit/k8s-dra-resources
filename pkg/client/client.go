package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/dharmjit/k8s-dra-resources/pkg/types"
	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ResourceClient interface {
	getResourceSlices(ctx context.Context) ([]resourcev1beta1.ResourceSlice, error)
	getResourceClaims(ctx context.Context) ([]resourcev1beta1.ResourceClaim, error)
	getNodes(ctx context.Context) ([]corev1.Node, error)
	getPods(ctx context.Context) ([]corev1.Pod, error)
	GetK8sResources(ctx context.Context) ([]*types.NodeInfo, error)
}

type resourceClient struct {
	typedClient kubernetes.Interface
}

func NewResourceClient(kubeconfigPath string) (ResourceClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	typedClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create typed client: %w", err)
	}

	return &resourceClient{typedClient: typedClient}, nil
}

func (c *resourceClient) getResourceSlices(ctx context.Context) ([]resourcev1beta1.ResourceSlice, error) {
	list, err := c.typedClient.ResourceV1beta1().ResourceSlices().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}
	return list.Items, nil
}

func (c *resourceClient) getResourceClaims(ctx context.Context) ([]resourcev1beta1.ResourceClaim, error) {
	list, err := c.typedClient.ResourceV1beta1().ResourceClaims("").List(ctx, metav1.ListOptions{}) // "" for all namespaces
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceClaims: %w", err)
	}
	return list.Items, nil
}

func (c *resourceClient) getNodes(ctx context.Context) ([]corev1.Node, error) {
	list, err := c.typedClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	return list.Items, nil
}

func (c *resourceClient) getPods(ctx context.Context) ([]corev1.Pod, error) {
	list, err := c.typedClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return list.Items, nil
}

func (c *resourceClient) GetK8sResources(ctx context.Context) ([]*types.NodeInfo, error) {
	nodes, err := c.getNodes(ctx)
	if err != nil {
		return nil, err
	}

	resourceSlices, err := c.getResourceSlices(ctx)
	if err != nil {
		return nil, err
	}

	resourceClaims, err := c.getResourceClaims(ctx)
	if err != nil {
		return nil, err
	}

	pods, err := c.getPods(ctx)
	if err != nil {
		return nil, err
	}

	// calculate total requested resources per node
	requestedResources := make(map[string]corev1.ResourceList)
	for _, pod := range pods {
		if pod.Spec.NodeName == "" {
			continue
		}
		if _, ok := requestedResources[pod.Spec.NodeName]; !ok {
			requestedResources[pod.Spec.NodeName] = make(corev1.ResourceList)
		}
		for _, container := range pod.Spec.Containers {
			for resName, resQuant := range container.Resources.Requests {
				if _, ok := requestedResources[pod.Spec.NodeName][resName]; !ok {
					requestedResources[pod.Spec.NodeName][resName] = resQuant.DeepCopy()
				} else {
					existingQuant := requestedResources[pod.Spec.NodeName][resName]
					existingQuant.Add(resQuant)
					requestedResources[pod.Spec.NodeName][resName] = existingQuant
				}
			}
		}
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
	nodeMap := make(map[string]*types.NodeInfo)
	for _, node := range nodes {
		role := "<none>"
		for k := range node.Labels {
			if strings.HasPrefix(k, "node-role.kubernetes.io/") {
				role = strings.TrimPrefix(k, "node-role.kubernetes.io/")
				break
			}
		}

		// Calculate available resources
		availableCPU := node.Status.Allocatable[corev1.ResourceCPU].DeepCopy()
		availableMemory := node.Status.Allocatable[corev1.ResourceMemory].DeepCopy()
		availableStorage := node.Status.Allocatable[corev1.ResourceStorage].DeepCopy()

		if reqs, ok := requestedResources[node.Name]; ok {
			if cpuReq, ok := reqs[corev1.ResourceCPU]; ok {
				availableCPU.Sub(cpuReq)
			}
			if memReq, ok := reqs[corev1.ResourceMemory]; ok {
				availableMemory.Sub(memReq)
			}
			if storageReq, ok := reqs[corev1.ResourceStorage]; ok {
				availableStorage.Sub(storageReq)
			}
		}

		nodeMap[node.Name] = &types.NodeInfo{
			NodeName: node.Name,
			NodeRole: role,
			NodeCapacity: types.NodeCapacity{
				TotalCPU:         node.Status.Capacity[corev1.ResourceCPU],
				AvailableCPU:     availableCPU,
				TotalMemory:      node.Status.Capacity[corev1.ResourceMemory],
				AvailableMemory:  availableMemory,
				TotalStorage:     node.Status.Capacity[corev1.ResourceStorage],
				AvailableStorage: availableStorage,
			},
			Devices: []types.Device{},
		}
	}

	// Populate devices for each node
	for _, rs := range resourceSlices {
		nodeInfo, ok := nodeMap[rs.Spec.NodeName]
		if !ok {
			continue
		}

		deviceMap := make(map[string]types.Device) // key is productName

		sliceIdentifier := fmt.Sprintf("%s-%s", rs.Spec.Driver, rs.Spec.Pool.Name)
		for _, dev := range rs.Spec.Devices {

			var productName string
			productName = rs.Spec.Driver
			if productName == "gpu.nvidia.com" {
				if dev.Basic != nil {
					if attrProductName, ok := dev.Basic.Attributes["productName"]; ok {
						productName = *attrProductName.StringValue
					}
				}
			}

			var memory resource.Quantity
			if dev.Basic != nil {
				if mem, ok := dev.Basic.Capacity["memory"]; ok {
					memory = mem.Value
				}
			}

			// if productName is not in deviceMap, initialize it otherwise increment the TotalCount and AvailableCount by 1
			if _, ok := deviceMap[productName]; !ok {
				deviceMap[productName] = types.Device{
					ProductName:    productName,
					TotalCount:     1,
					AvailableCount: 1,
					Memory:         memory,
				}
			} else {
				dev := deviceMap[productName]
				dev.TotalCount++
				dev.AvailableCount++
				deviceMap[productName] = dev
			}

			// if the device is allocated, reduce the available count by 1
			if allocatedDevices[sliceIdentifier][dev.Name] {
				dev := deviceMap[productName]
				if dev.AvailableCount > 0 {
					dev.AvailableCount--
				}
				deviceMap[productName] = dev
			}
		}
		// Iterate over the deviceMap to populate nodeInfo.Devices
		for _, dev := range deviceMap {
			nodeInfo.Devices = append(nodeInfo.Devices, dev)
		}
	}

	var nodeInfoList []*types.NodeInfo
	for _, nodeInfo := range nodeMap {
		nodeInfoList = append(nodeInfoList, nodeInfo)
	}

	return nodeInfoList, nil
}
