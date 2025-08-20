package display

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	resourceClient "github.com/dharmjit/k8s-dra-resources/pkg/client"
	"k8s.io/apimachinery/pkg/api/resource"
)

func formatMemoryAsGiB(q resource.Quantity) string {
	val, ok := q.AsInt64()
	if !ok {
		// Fallback for very large values that don't fit in int64
		return q.String()
	}
	gib := float64(val) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.2fGi", gib)
}

func DisplayTabularInfo(client resourceClient.ResourceClient) error {
	ctx := context.Background()
	fmt.Println("Fetching node and resource info...")

	nodeInfoList, err := client.GetK8sResources(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Header for the new format
	fmt.Fprintln(w, "NODE\tROLE\tCPU(TOTAL/AVAIL)\tMEMORY(TOTAL/AVAIL GiB)\tSTORAGE(TOTAL/AVAIL)\tDEVICES")

	for _, nodeInfo := range nodeInfoList {
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
			nodeInfo.NodeName,
			nodeInfo.NodeRole,
			nodeInfo.NodeCapacity.TotalCPU.String(), nodeInfo.NodeCapacity.AvailableCPU.String(),
			formatMemoryAsGiB(nodeInfo.NodeCapacity.TotalMemory), formatMemoryAsGiB(nodeInfo.NodeCapacity.AvailableMemory),
			nodeInfo.NodeCapacity.TotalStorage.String(), nodeInfo.NodeCapacity.AvailableStorage.String(),
			deviceString,
		)
	}

	return nil
}
