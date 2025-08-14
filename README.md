# k8s-dra-resources

A command-line tool to list and show the status of Kubernetes Dynamic Resource Allocation (DRA) resources.

## Description

This tool lists all `ResourceSlices` and `ResourceClaims` in a Kubernetes cluster and displays the allocation status of each device. It helps administrators and developers to quickly see which DRA-managed devices are available and which are allocated.

## Usage

To run the tool, you need to have a valid kubeconfig file. By default, it uses the `KUBECONFIG` environment variable or the recommended home file (`~/.kube/config`).

```bash
go run main.go
```

You can also specify the path to your kubeconfig file using the `-kubeconfig` flag:

```bash
go run main.go -kubeconfig /path/to/your/kubeconfig
```

### Example Output

The output is a table that lists all the devices in each `ResourceSlice` and their allocation status.

```sh
SLICE_NAME      NODE    DRIVER          DEVICE_NAME     STATUS
slice-1         node-1  example.com/gpu gpu-0           Available
slice-1         node-1  example.com/gpu gpu-1           Allocated
slice-2         node-2  example.com/fpga fpga-0         Available
```

## Aggregated View

You can use the `-aggregated-view` flag to see an aggregated view of devices per node per driver.

```bash
go run main.go -aggregated-view
```

### Example Output

```sh
NODE    DRIVER          ALLOCATED       AVAILABLE
node-1  example.com/gpu 1               1
node-2  example.com/fpga 0              1
```
