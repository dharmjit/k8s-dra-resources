package main

import (
	"flag"
	"fmt"
	"os"

	resourceClient "github.com/dharmjit/k8s-dra-resources/pkg/client"
	"github.com/dharmjit/k8s-dra-resources/pkg/display"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	kubeconfig := flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "path to the kubeconfig file")
	flag.Parse()

	if *kubeconfig == "" {
		*kubeconfig = clientcmd.RecommendedHomeFile
	}

	client, err := resourceClient.NewResourceClient(*kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating DRA client: %v\n", err)
		os.Exit(1)
	}

	if err := display.DisplayTabularInfo(client); err != nil {
		fmt.Fprintf(os.Stderr, "Error displaying node info: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n------------------------------")
}