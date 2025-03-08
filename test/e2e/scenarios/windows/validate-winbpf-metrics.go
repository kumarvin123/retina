package windows

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8s "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ValidateWinBpfMetric struct {
	KubeConfigFilePath        string
	EbpfXdpDeamonSetNamespace string
	EbpfXdpDeamonSetName      string
	RetinaDaemonSetNamespace  string
	RetinaDaemonSetName       string
}

type CommandResult struct {
	Output string
}

func (v *ValidateWinBpfMetric) ExecCommandInWinPod(cmd string, DeamonSetName string, DaemonSetNamespace string, LabelSelector string) (error, string) {
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err), ""
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err), ""
	}

	pods, err := clientset.CoreV1().Pods(DaemonSetNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: LabelSelector,
	})
	if err != nil {
		panic(err.Error())
	}

	var windowsPod *v1.Pod
	for pod := range pods.Items {
		if pods.Items[pod].Spec.NodeSelector["kubernetes.io/os"] == "windows" {
			windowsPod = &pods.Items[pod]
		}
	}

	if windowsPod == nil {
		return fmt.Errorf("No Windows Pod found in DaemonSet %s and label %s", DeamonSetName, LabelSelector), ""
	}

	result := &CommandResult{}
	err = defaultRetrier.Do(context.TODO(), func() error {
		outputBytes, err := k8s.ExecPod(context.TODO(), clientset, config, windowsPod.Namespace, windowsPod.Name, cmd)
		if err != nil {
			fmt.Errorf("error executing command in windows pod: %w", err)
			return fmt.Errorf("error executing command in windows pod: %w", err)
		}

		result.Output = string(outputBytes)
		return nil
	})
	if err != nil {
		return err, ""
	}

	return nil, result.Output
}

func (v *ValidateWinBpfMetric) Run() error {
	ebpfLabelSelector := fmt.Sprintf("name=%s", v.EbpfXdpDeamonSetName)

	//TRACE
	err, _ := v.ExecCommandInWinPod("C:\\event-writer-helper.bat Start-EventWriter -event 4 -srcIP 23.192.228.84",
		v.EbpfXdpDeamonSetName,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Second)
	err, _ = v.ExecCommandInWinPod("C:\\event-writer-helper.bat CurlExampleCOM",
		v.EbpfXdpDeamonSetName,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}

	err, output := v.ExecCommandInWinPod("C:\\event-writer-helper.bat DumpEventWriter",
		v.EbpfXdpDeamonSetName,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}
	fmt.Println(output)
	if strings.Contains(output, "failed") {
		return fmt.Errorf("failed to start event writer")
	}

	//DROP
	time.Sleep(60 * time.Second)
	err, _ = v.ExecCommandInWinPod("C:\\event-writer-helper.bat Start-EventWriter -event 1 -srcIP 23.192.228.84",
		v.EbpfXdpDeamonSetName,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Second)
	err, _ = v.ExecCommandInWinPod("C:\\event-writer-helper.bat CurlExampleCOM",
		v.EbpfXdpDeamonSetName,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}

	err, output = v.ExecCommandInWinPod("C:\\event-writer-helper.bat DumpEventWriter",
		v.EbpfXdpDeamonSetName,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}
	fmt.Println(output)
	if strings.Contains(output, "failed") {
		return fmt.Errorf("failed to start event writer")
	}

	err, output = v.ExecCommandInWinPod("C:\\event-writer-helper.bat DumpCurl", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, ebpfLabelSelector)
	if err != nil {
		return err
	}
	if strings.Contains(output, "failed") {
		return fmt.Errorf("failed to curl to example.com")
	}

	var promOutput string
	numAttempts := 10
	for promOutput == "" && numAttempts > 0 {
		err, promOutput = v.ExecCommandInWinPod("C:\\event-writer-helper.bat GetRetinaPromMetrics", v.RetinaDaemonSetName, v.RetinaDaemonSetNamespace, "k8s-app=retina")

		if err != nil {
			fmt.Println(err.Error())
			return err
		}
		if promOutput != "" {
			break
		}
		numAttempts--
		time.Sleep(5 * time.Second)
	}

	if promOutput == "" {
		return fmt.Errorf("failed to get prometheus metrics from Retina DaemonSet")
	}
	fmt.Println(promOutput)

	// Check for Basic Metrics (Node Level)
	//Forward
	if strings.Contains(promOutput, "networkobservability_forward_bytes{direction=\"ingress\"}") {
		fmt.Println("prom metrics output contains networkobservability_forward_bytes{direction=\"ingress\"}")
	} else {
		return fmt.Errorf("prom metrics output does not contain networkobservability_forward_bytes{direction=\"ingress\"}")
	}

	if strings.Contains(promOutput, "networkobservability_forward_count{direction=\"ingress\"}") {
		fmt.Println("prom metrics output contains networkobservability_forward_count{direction=\"ingress\"}")
	} else {
		return fmt.Errorf("prom metrics output does not contain networkobservability_forward_count{direction=\"ingress\"}")
	}

	//Drop
	if strings.Contains(promOutput, "networkobservability_drop_bytes{direction=\"ingress\"}") {
		fmt.Println("prom metrics output contains networkobservability_drop_bytes{direction=\"ingress\"}")
	} else {
		return fmt.Errorf("prom metrics output does not contain networkobservability_drop_bytes{direction=\"ingress\"}")
	}

	if strings.Contains(promOutput, "networkobservability_drop_count{direction=\"ingress\"}") {
		fmt.Println("prom metrics output contains networkobservability_drop_count{direction=\"ingress\"}")
	} else {
		return fmt.Errorf("prom metrics output does not contain networkobservability_drop_count{direction=\"ingress\"}")
	}

	// Check for Advanced Metrics
	if strings.Contains(promOutput, "networkobservability_adv_tcpflags_count") {
		fmt.Println("prom metrics output contains networkobservability_adv_tcpflags_count")
	}

	return nil
}

func (v *ValidateWinBpfMetric) Prevalidate() error {
	return nil
}

func (v *ValidateWinBpfMetric) Stop() error {
	return nil
}
