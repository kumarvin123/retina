package kubernetes

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type LoadWinBPFMaps struct {
	KubeConfigFilePath               string
	LoadWinBPFMapsDeamonSetNamespace string
	LoadWinBPFMapsDeamonSetName      string
}

type CommandResult struct {
	Output string
}

func (a *LoadWinBPFMaps) ExecCommandInWinPod(cmd string, DeamonSetName string, DaemonSetNamespace string, LabelSelector string) (error, string) {
	config, err := clientcmd.BuildConfigFromFlags("", a.KubeConfigFilePath)
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
		return fmt.Errorf("no Windows Pod found in DaemonSet %s and label %s", DeamonSetName, LabelSelector), ""
	}

	result := &CommandResult{}
	err = defaultRetrier.Do(context.TODO(), func() error {
		outputBytes, err := ExecPod(context.TODO(), clientset, config, windowsPod.Namespace, windowsPod.Name, cmd)
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

func (a *LoadWinBPFMaps) Run() error {
	// Copy Event Writer into Node
	loadWinBPFMapsDLabelSelector := fmt.Sprintf("name=%s", a.LoadWinBPFMapsDeamonSetName)
	err, _ := a.ExecCommandInWinPod("move /Y .\\event-writer-helper.bat C:\\event-writer-helper.bat", a.LoadWinBPFMapsDeamonSetName, a.LoadWinBPFMapsDeamonSetNamespace, loadWinBPFMapsDLabelSelector)
	if err != nil {
		return err
	}

	err, _ = a.ExecCommandInWinPod("C:\\event-writer-helper.bat Setup-EventWriter", a.LoadWinBPFMapsDeamonSetName, a.LoadWinBPFMapsDeamonSetNamespace, loadWinBPFMapsDLabelSelector)
	if err != nil {
		return err
	}

	// pin maps
	err, _ = a.ExecCommandInWinPod("C:\\event-writer-helper.bat Start-EventWriter PinMaps", a.LoadWinBPFMapsDeamonSetName, a.LoadWinBPFMapsDeamonSetNamespace, loadWinBPFMapsDLabelSelector)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	err, output := a.ExecCommandInWinPod("C:\\event-writer-helper.bat DumpEventWriter", a.LoadWinBPFMapsDeamonSetName, a.LoadWinBPFMapsDeamonSetNamespace, loadWinBPFMapsDLabelSelector)
	if err != nil {
		return err
	}

	fmt.Println(output)
	// dump event writer
	return nil
}

func (a *LoadWinBPFMaps) Prevalidate() error {
	return nil
}

func (a *LoadWinBPFMaps) Stop() error {
	return nil
}
