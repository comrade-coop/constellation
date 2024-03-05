package main

import (
	"log"
	"os"
	"os/exec"
	"time"
)

const (
	manifest       = "/usr/lib/helmfile-template"
	kubeconfigPath = "/etc/kubernetes/admin.conf"
)

func applyCustomManifest() {
	// Export the KUBECONFIG environment variable
	err := os.Setenv("KUBECONFIG", kubeconfigPath)
	if err != nil {
		log.Printf("Failed to set KUBECONFIG environment variable: %v", err)
	}
	for {
		if isClusterReady() {
			break
		} else {
			log.Println("Bootstraper not finished yet. Waiting...")
			time.Sleep(30 * time.Second)
			continue
		}
	}
	applyManifest()
}

func isClusterReady() bool {
	// Use kubectl command to check the cluster readiness
	cmd := exec.Command("kubectl", "get", "pods", "-A")
	err := cmd.Run()
	if err != nil {
		return false
	}
	return true
}

func applyManifest() {
	// Check if the output contains the desired string
	log.Println("K8s cluster initliazed, applying chart...")
	cmd := exec.Command("kubectl", "apply", "--server-side", "-f", manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to apply Template using kubectl: %v\nOutput: %s", err, string(out))
		return
	}
	log.Println("chart applied successfully using kubectl.")
}
