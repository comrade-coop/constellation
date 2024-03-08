package main

import (
	"log"
	"os"
	"os/exec"
	"time"
)

const (
	manifest                  = "/usr/lib/helmfile-template"
	kubeconfigPath            = "/etc/kubernetes/admin.conf"
	kubeConfigEnv             = "KUBECONFIG"
	nginxTimeoutArg           = "--timeout=800s"
	bootstrapperSleepInterval = 100 * time.Second
)

func applyCustomManifest() {
	// Export the KUBECONFIG environment variable
	err := os.Setenv("KUBECONFIG", kubeconfigPath)
	if err != nil {
		log.Printf("Failed to set KUBECONFIG environment variable: %v", err)
	}
	for {
		if !isClusterReady() {
			log.Println("Bootstraper not finished yet. Waiting...")
			time.Sleep(bootstrapperSleepInterval)
			continue
		}
		break
	}
	applyManifest()
	log.Println("manifest applied successfully using kubectl.")
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
		cmd := exec.Command("kubectl", "wait", "--namespace", "keda", "--for=condition=available", "deployment/ingress-nginx-controller", nginxTimeoutArg)
		err := cmd.Run()
		if err != nil {
			log.Printf("Failed waiting for nginx controller to start")
		}
		applyManifest()
	}
	return
}
