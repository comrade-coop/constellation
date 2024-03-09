package main

import (
	"bytes"
	"context"
	"fmt"
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
	commandTimeout            = 2 * time.Minute
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
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "--server-side", "-f", manifest)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := startTimedProcess(cmd, commandTimeout); err != nil {
		log.Printf("kubectl apply output: %s\n", outBuf.String())
		log.Printf("kubectl apply error: %s\n", errBuf.String())

		log.Printf("Failed to apply Template using kubectl: %v\n", err)

		waitForNginxController()
	}
}

func startTimedProcess(cmd *exec.Cmd, timeout time.Duration) error {
	go func() {
		<-time.After(timeout)
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			fmt.Println("Failed to send interrupt signal:", err)
		}
	}()
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Failed starting command %v", cmd.Path)
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func waitForNginxController() {
	cmd := exec.Command("kubectl", "wait", "--namespace", "keda", "--for=condition=available", "deployment/ingress-nginx-controller", nginxTimeoutArg)
	// Execute the command
	log.Println("Waiting for nginx controller...")
	if err := cmd.Run(); err != nil {
		log.Printf("Failed waiting for nginx controller to start: %v\n", err)
		// Retry applying the manifest
		applyManifest()
	}
}
