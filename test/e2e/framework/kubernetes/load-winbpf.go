package kubernetes

import (
	"fmt"
	"strings"
	"time"
)

type LoadAndPinWinBPF struct {
	KubeConfigFilePath                 string
	LoadAndPinWinBPFDeamonSetNamespace string
	LoadAndPinWinBPFDeamonSetName      string
}

func (a *LoadAndPinWinBPF) Run() error {
	// Copy Event Writer into Node
	LoadAndPinWinBPFDLabelSelector := fmt.Sprintf("name=%s", a.LoadAndPinWinBPFDeamonSetName)
	_, err := ExecCommandInWinPod(a.KubeConfigFilePath, "move /Y .\\event-writer-helper.bat C:\\event-writer-helper.bat", a.LoadAndPinWinBPFDeamonSetName, a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector)
	if err != nil {
		return err
	}

	_, err = ExecCommandInWinPod(a.KubeConfigFilePath, "C:\\event-writer-helper.bat EventWriter-Setup", a.LoadAndPinWinBPFDeamonSetName, a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector)
	if err != nil {
		return err
	}

	// pin maps
	_, err = ExecCommandInWinPod(a.KubeConfigFilePath, "C:\\event-writer-helper.bat EventWriter-LoadAndPinPrgAndMaps", a.LoadAndPinWinBPFDeamonSetName, a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	output, err := ExecCommandInWinPod(a.KubeConfigFilePath, "C:\\event-writer-helper.bat EventWriter-Dump", a.LoadAndPinWinBPFDeamonSetName, a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector)
	if err != nil {
		return err
	}

	fmt.Println(output)
	if strings.Contains(output, "error") || strings.Contains(output, "failed") {
		return fmt.Errorf("error in loading and pinning BPF maps and program: %s", output)
	}
	return nil
}

func (a *LoadAndPinWinBPF) Prevalidate() error {
	return nil
}

func (a *LoadAndPinWinBPF) Stop() error {
	return nil
}
