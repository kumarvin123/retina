package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubectl/pkg/scheme"
)

type CopyPod struct {
	PodNamespace       string
	KubeConfigFilePath string
	PodName            string
	srcPath, destPath  string
}

func (e *CopyPod) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := clientcmd.BuildConfigFromFlags("", e.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	err = retry.OnError(retry.DefaultRetry, func(err error) bool {
		// Retry on every error
		return true
	}, func() error {
		copypodErr := CopyFileToPod(ctx, clientset, config, e.PodNamespace, e.PodName, e.srcPath, e.destPath)
		if copypodErr != nil {
			log.Printf("error copy files: %v", copypodErr)
		}
		return copypodErr
	})
	if err != nil {
		return fmt.Errorf("error executing command, all retries exhausted: %w", err)
	}

	return nil
}

func (e *CopyPod) Prevalidate() error {
	return nil
}

func (e *CopyPod) Stop() error {
	return nil
}

// CopyFileToPod copies a local file (srcPath) to a destination file (destPath)
// inside the target pod identified by namespace and podName, without tar archiving.
func CopyFileToPod(ctx context.Context, clientset *kubernetes.Clientset, config *rest.Config, namespace, podName, srcPath, destPath string) error {
	// Open the source file.
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	// Read the file content (or you could stream it directly).
	var buf bytes.Buffer
	if _, err = io.Copy(&buf, file); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Prepare the command to write the file into the destination in the pod.
	// Using a simple shell command that redirects STDIN to a file:
	// For Linux containers.
	command := []string{"sh", "-c", fmt.Sprintf("cat > %s", destPath)}

	// Build the exec request.
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	option := &v1.PodExecOptions{
		Command: command,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}
	req.VersionedParams(option, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  &buf,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return fmt.Errorf("error executing remote command: %w, stderr: %s", err, stderr.String())
	}

	return nil
}
