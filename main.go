package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1" // Use v1beta1
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type DRAClient interface {
	GetResourceSlices(ctx context.Context) ([]resourcev1beta1.ResourceSlice, error)
	GetResourceClaims(ctx context.Context) ([]resourcev1beta1.ResourceClaim, error) // New function
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

	return &draClient{
		typedClient: typedClient,
	}, nil
}

func (c *draClient) GetResourceSlices(ctx context.Context) ([]resourcev1beta1.ResourceSlice, error) {
	list, err := c.typedClient.ResourceV1beta1().ResourceSlices().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}

	return list.Items, nil
}

// GetResourceClaims retrieves all ResourceClaim objects using the typed client.
// ResourceClaims are namespaced, but we'll list across all namespaces for simplicity for now.
func (c *draClient) GetResourceClaims(ctx context.Context) ([]resourcev1beta1.ResourceClaim, error) {
	list, err := c.typedClient.ResourceV1beta1().ResourceClaims("").List(ctx, metav1.ListOptions{}) // "" for all namespaces
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceClaims: %w", err)
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

	if err := displayResourceSlices(client); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying ResourceSlices: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n------------------------------\n")
	fmt.Println("Note: This tool displays allocation status based on ResourceClaims.")
	fmt.Println("Total and available capacities are not directly exposed in resource.k8s.io/v1beta1.Device struct.")
}

func displayResourceSlices(client DRAClient) error {
	fmt.Println("Fetching ResourceSlices and ResourceClaims...")
	resourceSlices, err := client.GetResourceSlices(context.Background())
	if err != nil {
		return err
	}

	resourceClaims, err := client.GetResourceClaims(context.Background())
	if err != nil {
		return err
	}

	// Map to store allocated devices: ResourceSliceIdentifier (Driver-PoolName) -> DeviceName -> true (allocated)
	allocatedDevices := make(map[string]map[string]bool)
	for _, rc := range resourceClaims {
		if rc.Status.Allocation != nil && rc.Status.Allocation.Devices.Results != nil {
			for _, ads := range rc.Status.Allocation.Devices.Results {
				// Construct a unique identifier for the ResourceSlice based on Driver and Pool
				sliceIdentifier := fmt.Sprintf("%s-%s", ads.Driver, ads.Pool)
				if _, ok := allocatedDevices[sliceIdentifier]; !ok {
					allocatedDevices[sliceIdentifier] = make(map[string]bool)
				}
				allocatedDevices[sliceIdentifier][ads.Device] = true
			}
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "SLICE_NAME\tNODE\tDRIVER\tDEVICE_NAME\tSTATUS")
	for _, rs := range resourceSlices {
		// Construct the identifier for the current ResourceSlice
		sliceIdentifier := fmt.Sprintf("%s-%s", rs.Spec.Driver, rs.Spec.Pool.Name) // Assuming rs.Spec.Pool.Name exists

		if len(rs.Spec.Devices) == 0 {
			fmt.Fprintf(w, "%s\t%s\t%s\t<no devices>\tN/A\n", rs.Name, rs.Spec.NodeName, rs.Spec.Driver)
			continue
		}
		for _, dev := range rs.Spec.Devices {
			status := "Available"
			if allocatedDevices[sliceIdentifier][dev.Name] {
				status = "Allocated"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				rs.Name, rs.Spec.NodeName, rs.Spec.Driver, dev.Name, status)
		}
	}

	return nil
}
