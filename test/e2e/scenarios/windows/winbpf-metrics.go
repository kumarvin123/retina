package windows

import (
	"context"
	"fmt"
	"log"
	"net"
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
	// Setup Event Writer into Node
	ebpfLabelSelector := fmt.Sprintf("name=%s", v.EbpfXdpDeamonSetName)
	err, _ := v.ExecCommandInWinPod("move .\\event-writer-helper.bat C:\\event-writer-helper.bat", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, ebpfLabelSelector)
	if err != nil {
		return err
	}

	err, _ = v.ExecCommandInWinPod("C:\\event-writer-helper.bat Setup-EventWriter", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, ebpfLabelSelector)
	if err != nil {
		return err
	}

	// Resolve the hostname for aka.ms
	// need only 1 IP address
	ips, err := net.LookupIP("http://aka.ms")
	if err != nil {
		log.Fatal(err)
	}
	aksIpaddress := ips[0].String()

	//TRACE
	cmd := fmt.Sprintf("C:\\event-writer-helper.bat Start-EventWriter -event 4 -srcIP %s", aksIpaddress)
	err, output := v.ExecCommandInWinPod(cmd, v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, ebpfLabelSelector)
	if err != nil {
		return err
	}
	fmt.Println(output)

	cmd = fmt.Sprintf("C:\\event-writer-helper.bat Curl %s", aksIpaddress)
	err, output = v.ExecCommandInWinPod(cmd, v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, ebpfLabelSelector)
	if err != nil {
		return err
	}
	fmt.Println(output)

	time.Sleep(20 * time.Second)

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

	// Check for Basic Metrics
	if strings.Contains(output, "networkobservability_forward_bytes") {
		fmt.Println("The output contains networkobservability_forward_bytes")
	}

	// Check for Advanced Metrics
	if strings.Contains(output, "networkobservability_adv_tcpflags_count") {
		fmt.Println("The output contains networkobservability_adv_tcpflags_count")
	}

	fmt.Println(output)
	return nil
}

func (v *ValidateWinBpfMetric) Prevalidate() error {
	return nil
}

func (v *ValidateWinBpfMetric) Stop() error {
	return nil
}
