package windows

import (
	"context"
	"fmt"
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

func (v *ValidateWinBpfMetric) ExecCommandInPod(cmd string, DeamonSetName string, DaemonSetNamespace string, OS string) error {
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	LabelSelector := fmt.Sprintf("name=%s", DeamonSetName)
	pods, err := clientset.CoreV1().Pods(DaemonSetNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: LabelSelector,
	})
	if err != nil {
		panic(err.Error())
	}

	var windowsPod *v1.Pod
	for pod := range pods.Items {
		if pods.Items[pod].Spec.NodeSelector["kubernetes.io/os"] == OS {
			windowsPod = &pods.Items[pod]
		}
	}

	if windowsPod == nil {
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
	v.ExecCommandInPod("dir C:", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, "windows")
	v.ExecCommandInPod("copy .\\event_writer.exe C:\\event_writer.exe", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, "windows")
	v.ExecCommandInPod("copy .\\bpf_event_writer.sys C:\\bpf_event_writer.sys", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, "windows")
	v.ExecCommandInPod("dir C:", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, "windows")
	v.ExecCommandInPod("cd C:\\ && .\\event_writer.exe -event 4", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace, "windows")
	//v.ExecCommandInPod("cd C:\\", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace)
	//v.ExecCommandInPod("powershell -Command \"Start-Process -FilePath '.\\event_writer.exe' -ArgumentList '-event 4'\"", v.EbpfXdpDeamonSetName, v.EbpfXdpDeamonSetNamespace)

	time.Sleep(time.Second * time.Duration(300))
	v.ExecCommandInPod("curl -s \"http://localhost:10093/metrics\"", v.RetinaDaemonSetName, v.RetinaDaemonSetNamespace, "windows")
	//v.ExecCommandInPod("powershell -Command \"Invoke-WebRequest -Uri 'http://localhost:10093/metrics'\"", v.RetinaDaemonSetName, v.RetinaDaemonSetNamespace, "windows")
	return nil
}

func (v *ValidateWinBpfMetric) Prevalidate() error {
	return nil
}

func (v *ValidateWinBpfMetric) Stop() error {
	return nil
}
