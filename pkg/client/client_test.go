package client

import (
	"context"
	"sort"
	"testing"

	"github.com/dharmjit/k8s-dra-resources/pkg/types"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetK8sResources(t *testing.T) {
	testCases := []struct {
		name           string
		nodes          []corev1.Node
		pods           []corev1.Pod
		resourceSlices []resourcev1beta1.ResourceSlice
		resourceClaims []resourcev1beta1.ResourceClaim
		expected       []*types.NodeInfo
		expectErr      bool
	}{
		{
			name: "should correctly process nodes, resource slices, and claims",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node-1",
						Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:     resource.MustParse("4"),
							corev1.ResourceMemory:  resource.MustParse("16Gi"),
							corev1.ResourceStorage: resource.MustParse("100Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:     resource.MustParse("3"),
							corev1.ResourceMemory:  resource.MustParse("14Gi"),
							corev1.ResourceStorage: resource.MustParse("90Gi"),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node-2",
						Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:     resource.MustParse("4"),
							corev1.ResourceMemory:  resource.MustParse("16Gi"),
							corev1.ResourceStorage: resource.MustParse("100Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:     resource.MustParse("3"),
							corev1.ResourceMemory:  resource.MustParse("14Gi"),
							corev1.ResourceStorage: resource.MustParse("90Gi"),
						},
					},
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						NodeName: "node-1",
						Containers: []corev1.Container{
							{
								Name: "container-1",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("2Gi"),
									},
								},
							},
						},
					},
				},
			},
			resourceSlices: []resourcev1beta1.ResourceSlice{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "slice-1"},
					Spec: resourcev1beta1.ResourceSliceSpec{
						NodeName: "node-1",
						Driver:   "gpu.nvidia.com",
						Pool: resourcev1beta1.ResourcePool{
							Name: "pool-a",
						},
						Devices: []resourcev1beta1.Device{
							{
								Name: "gpu-0",
								Basic: &resourcev1beta1.BasicDevice{
									Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
										"productName": {StringValue: new(string)},
									},
									Capacity: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceCapacity{
										"memory": {Value: resource.MustParse("8Gi")},
									},
								},
							},
							{
								Name: "gpu-1",
								Basic: &resourcev1beta1.BasicDevice{
									Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
										"productName": {StringValue: new(string)},
									},
									Capacity: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceCapacity{
										"memory": {Value: resource.MustParse("8Gi")},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "slice-2"},
					Spec: resourcev1beta1.ResourceSliceSpec{
						NodeName: "node-2",
						Driver:   "gpu.nvidia.com",
						Pool: resourcev1beta1.ResourcePool{
							Name: "pool-b",
						},
						Devices: []resourcev1beta1.Device{
							{
								Name: "gpu-0",
								Basic: &resourcev1beta1.BasicDevice{
									Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
										"productName": {StringValue: new(string)},
									},
									Capacity: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceCapacity{
										"memory": {Value: resource.MustParse("8Gi")},
									},
								},
							},
							{
								Name: "gpu-1",
								Basic: &resourcev1beta1.BasicDevice{
									Attributes: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceAttribute{
										"productName": {StringValue: new(string)},
									},
									Capacity: map[resourcev1beta1.QualifiedName]resourcev1beta1.DeviceCapacity{
										"memory": {Value: resource.MustParse("8Gi")},
									},
								},
							},
						},
					},
				},
			},
			resourceClaims: []resourcev1beta1.ResourceClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "claim-1"},
					Status: resourcev1beta1.ResourceClaimStatus{
						Allocation: &resourcev1beta1.AllocationResult{
							Devices: resourcev1beta1.DeviceAllocationResult{
								Results: []resourcev1beta1.DeviceRequestAllocationResult{
									{
										Driver: "gpu.nvidia.com",
										Pool:   "pool-a",
										Device: "gpu-0",
									},
								},
							},
						},
					},
				},
			},
			expected: []*types.NodeInfo{
				{
					NodeName: "node-1",
					NodeRole: "worker",
					NodeCapacity: types.NodeCapacity{
						TotalCPU:         resource.MustParse("4"),
						AvailableCPU:     resource.MustParse("2"),
						TotalMemory:      resource.MustParse("16Gi"),
						AvailableMemory:  resource.MustParse("12Gi"),
						TotalStorage:     resource.MustParse("100Gi"),
						AvailableStorage: resource.MustParse("90Gi"),
					},
					Devices: []types.Device{
						{
							ProductName:    "NVIDIA GeForce RTX 5090",
							TotalCount:     2,
							AvailableCount: 1,
							Memory:         resource.MustParse("8Gi"),
						},
					},
				},
				{
					NodeName: "node-2",
					NodeRole: "worker",
					NodeCapacity: types.NodeCapacity{
						TotalCPU:         resource.MustParse("4"),
						AvailableCPU:     resource.MustParse("3"),
						TotalMemory:      resource.MustParse("16Gi"),
						AvailableMemory:  resource.MustParse("14Gi"),
						TotalStorage:     resource.MustParse("100Gi"),
						AvailableStorage: resource.MustParse("90Gi"),
					},
					Devices: []types.Device{
						{
							ProductName:    "NVIDIA GeForce RTX 5090",
							TotalCount:     2,
							AvailableCount: 2,
							Memory:         resource.MustParse("8Gi"),
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()

			for i := range tc.nodes {
				_, err := client.CoreV1().Nodes().Create(context.Background(), &tc.nodes[i], metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create node: %v", err)
				}
			}
			for i := range tc.pods {
				_, err := client.CoreV1().Pods(tc.pods[i].Namespace).Create(context.Background(), &tc.pods[i], metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create pod: %v", err)
				}
			}
			for i := range tc.resourceSlices {
				// Initialize productName for gpu.nvidia.com driver
				if tc.resourceSlices[i].Spec.Driver == "gpu.nvidia.com" {
					for j := range tc.resourceSlices[i].Spec.Devices {
						if tc.resourceSlices[i].Spec.Devices[j].Basic != nil {
							if _, ok := tc.resourceSlices[i].Spec.Devices[j].Basic.Attributes["productName"]; ok {
								*tc.resourceSlices[i].Spec.Devices[j].Basic.Attributes["productName"].StringValue = "NVIDIA GeForce RTX 5090"
							}
						}
					}
				}
				_, err := client.ResourceV1beta1().ResourceSlices().Create(context.Background(), &tc.resourceSlices[i], metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create resource slice: %v", err)
				}
			}
			for i := range tc.resourceClaims {
				_, err := client.ResourceV1beta1().ResourceClaims("").Create(context.Background(), &tc.resourceClaims[i], metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create resource claim: %v", err)
				}
			}

			rc := &resourceClient{typedClient: client}
			got, err := rc.GetK8sResources(context.Background())
			if (err != nil) != tc.expectErr {
				t.Fatalf("GetK8sResources() error = %v, expectErr %v", err, tc.expectErr)
			}

			sort.Slice(got, func(i, j int) bool {
				return got[i].NodeName < got[j].NodeName
			})
			sort.Slice(tc.expected, func(i, j int) bool {
				return tc.expected[i].NodeName < tc.expected[j].NodeName
			})

			if diff := cmp.Diff(got, tc.expected,
				cmp.Comparer(func(x, y resource.Quantity) bool {
					return x.Equal(y)
				}),
			); diff != "" {
				t.Errorf("mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
