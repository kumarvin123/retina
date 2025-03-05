package windows

import (
	"context"
	"fmt"

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

func (v *ValidateWinBpfMetric) ExecCommandInWinPod(cmd string, DeamonSetName string, DaemonSetNamespace string, LabelSelector string) error {
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	pods, err := clientset.CoreV1().Pods(DaemonSetNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: LabelSelector,
	})
	if err != nil {
		panic(err.Error())
	}

	var windowsPod *v1.Pod
	for pod := range pods.Items {
		fmt.Println("Pod Name: ", pods.Items[pod].Name)
		fmt.Println("Pod OS: ", pods.Items[pod].Spec.NodeSelector["kubernetes.io/os"])

		if pods.Items[pod].Spec.NodeSelector["kubernetes.io/os"] == "windows" {
			windowsPod = &pods.Items[pod]
		}
	}

	if windowsPod == nil {
		fmt.Println("No Windows Pod found")
		return ErrorNoWindowsPod
	}

	err = defaultRetrier.Do(context.TODO(), func() error {
		outputBytes, err := k8s.ExecPod(context.TODO(), clientset, config, windowsPod.Namespace, windowsPod.Name, cmd)
		if err != nil {
			return fmt.Errorf("error executing command in windows retina pod: %w", err)
		}
		output := string(outputBytes)
		fmt.Println(output)
		return nil
	})
	if err != nil {
		panic(err.Error())
	}

	return nil
}

func (v *ValidateWinBpfMetric) Run() error {
	// Hardcoding IP addr for aka.ms - 23.213.38.151 - 399845015
	//aksmsIpaddr := 399845015
	// Setup Event Writer
	v.ExecCommandInWinPod("event-writer-helper.bat Setup-EventWriter", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, fmt.Sprintf("name=%s", v.EbpfXdpDeamonSetName))
	v.ExecCommandInWinPod("event-writer-helper.bat Start-EventWriter", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, fmt.Sprintf("name=%s", v.EbpfXdpDeamonSetName))
	v.ExecCommandInWinPod("event-writer-helper.bat GetRetinaPromMetrics", v.RetinaDaemonSetName, v.RetinaDaemonSetNamespace, "k8s-app=retina")
	return nil
}

func (v *ValidateWinBpfMetric) Prevalidate() error {
	return nil
}

func (v *ValidateWinBpfMetric) Stop() error {
	return nil
}
