# k8s-dra-resources

A command-line tool and library to list and show the status of Kubernetes Dynamic Resource Allocation (DRA) resources.

## Description

This tool provides a summary of nodes, their capacities, and the status of DRA resources. It helps administrators and developers to quickly see node details and which DRA-managed devices are available and which are allocated.

## Command-Line Usage

To run the tool, you need to have a valid kubeconfig file. By default, it uses the `KUBECONFIG` environment variable or the recommended home file (`~/.kube/config`).

```bash
go run cmd/main.go
```

You can also specify the path to your kubeconfig file using the `-kubeconfig` flag:

```bash
go run cmd/main.go -kubeconfig /path/to/your/kubeconfig
```

### Example Output

The output is a table that lists all nodes and their resource information.

```sh
NODE    ROLE    CPU(TOTAL/AVAIL)    MEMORY(TOTAL/AVAIL GiB) STORAGE(TOTAL/AVAIL)    DEVICES
node-1  master  12/11               31.25/30.25             100G/90G                gpu.nvidia.com: 2 total, 1 available
node-2  worker  8/7                 15.63/14.63             100G/90G                None
```

## Library Usage

This project can also be used as a library to fetch information about DRA resources programmatically.

### Example

```go
package main

import (
 "context"
 "fmt"
 "os"

 "github.com/dharmjit/k8s-dra-resources/pkg/client"
)

func main() {
 kubeconfig := os.Getenv("KUBECONFIG")
 if kubeconfig == "" {
  kubeconfig = clientcmd.RecommendedHomeFile
 }

 c, err := client.NewResourceClient(kubeconfig)
 if err != nil {
  fmt.Fprintf(os.Stderr, "Error creating DRA client: %v\n", err)
  os.Exit(1)
 }

 nodeInfo, err := c.GetK8sResources(context.Background())
 if err != nil {
  fmt.Fprintf(os.Stderr, "Error getting resources: %v\n", err)
  os.Exit(1)
 }

 for _, node := range nodeInfo {
  fmt.Printf("Node: %s\n", node.NodeName)
  fmt.Printf("  Role: %s\n", node.NodeRole)
  fmt.Printf("  CPU (Total/Available): %s/%s\n", node.NodeCapacity.TotalCPU.String(), node.NodeCapacity.AvailableCPU.String())
  // ... and so on
 }
}
```
