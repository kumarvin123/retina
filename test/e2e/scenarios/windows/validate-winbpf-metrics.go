package windows

import (
	"fmt"
	"strings"
	"time"

	kubernetes "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

type ValidateWinBpfMetric struct {
	KubeConfigFilePath        string
	EbpfXdpDeamonSetNamespace string
	EbpfXdpDeamonSetName      string
	RetinaDaemonSetNamespace  string
	RetinaDaemonSetName       string
	NonHpcAppNamespace        string
	NonHpcAppName             string
	NonHpcPodName             string
}

func (v *ValidateWinBpfMetric) GetPromMetrics(ebpfLabelSelector string) (string, error) {
	_, err := kubernetes.ExecCommandInWinPod(v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-GetRetinaPromMetrics",
		v.EbpfXdpDeamonSetNamespace, ebpfLabelSelector)
	if err != nil {
		return "", err
	}

	time.Sleep(10 * time.Second)
	promOutput, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-CurlOut",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)

	if err != nil {
		return "", err
	}

	if promOutput == "" {
		return "", fmt.Errorf("prometheus metrics is empty")
	}
	return promOutput, nil
}

func (v *ValidateWinBpfMetric) Run() error {
	ebpfLabelSelector := fmt.Sprintf("name=%s", v.EbpfXdpDeamonSetName)
	promOutput, err := v.GetPromMetrics(ebpfLabelSelector)
	if err != nil {
		return err
	}

	fmt.Println(promOutput)

	fwd_labels := map[string]string{
		"direction": "ingress",
	}
	drp_labels := map[string]string{
		"direction": "ingress",
		"reason":    "130, 0",
	}

	var preTestFwdBytes float64 = 0
	var preTestDrpBytes float64 = 0
	var preTestFwdCount float64 = 0
	var preTestDrpCount float64 = 0
	if promOutput == "" {
		fmt.Println("PreTest - no prometheus metrics found")
	} else {
		preTestFwdBytes, _ = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_bytes", fwd_labels)
		fmt.Printf("networkobservability_forward_bytes value %f, labels: %v\n", preTestFwdBytes, fwd_labels)

		preTestFwdCount, _ = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_count", fwd_labels)
		fmt.Printf("networkobservability_forward_count value %f, labels: %v\n", preTestFwdBytes, fwd_labels)

		preTestDrpBytes, _ = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_bytes", drp_labels)
		fmt.Printf("networkobservability_drop_bytes value %f, labels: %v\n", preTestDrpBytes, drp_labels)

		preTestDrpCount, _ = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_count", drp_labels)
		fmt.Printf("networkobservability_drop_count value %f, labels: %v\n", preTestDrpBytes, drp_labels)
	}

	nonHpcLabelSelector := fmt.Sprintf("app=%s", v.NonHpcAppName)
	_, err = kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-GetPodIpAddress",
		v.NonHpcAppNamespace,
		nonHpcLabelSelector)
	if err != nil {
		return err
	}
	time.Sleep(10 * time.Second)
	nonHpcIpAddr, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-Dump",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}
	if strings.Contains(nonHpcIpAddr, "failed") || strings.Contains(nonHpcIpAddr, "error") {
		return fmt.Errorf("failed to get nonHpcIpAddr")
	}
	fmt.Println("Non HPC IP Addr: ", nonHpcIpAddr)

	_, err = kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-GetPodIfIndex",
		v.NonHpcAppNamespace,
		nonHpcLabelSelector)
	if err != nil {
		return err
	}
	time.Sleep(10 * time.Second)
	nonHpcIfIndex, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-Dump",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}
	if strings.Contains(nonHpcIfIndex, "failed") || strings.Contains(nonHpcIfIndex, "error") {
		return fmt.Errorf("failed to get nonHpcIfIndex")
	}
	fmt.Println("Non HPC Interface Index: ", nonHpcIfIndex)

	//Attach to the non HPC pod
	_, err = kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		fmt.Sprintf("C:\\event-writer-helper.bat EventWriter-Attach %s", nonHpcIfIndex),
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}
	output, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-Dump",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}
	fmt.Println(output)
	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return fmt.Errorf("failed to attach to non HPC pod interface %s", output)
	}

	//TRACE
	fmt.Printf("Produce Trace Events\n")
	//Example.com - 23.192.228.84
	_, err = kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-SetFilter -event 4 -srcIP 23.192.228.84",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	output, err = kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-Dump",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}
	fmt.Println(output)
	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return fmt.Errorf("failed to set filter for event writer")
	}

	numcurls := 10
	for numcurls > 0 {
		_, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			"C:\\event-writer-helper.bat EventWriter-Curl 23.192.228.84",
			v.NonHpcAppNamespace,
			nonHpcLabelSelector)
		if err != nil {
			return err
		}
		numcurls--
	}

	//DROP
	time.Sleep(60 * time.Second)
	fmt.Printf("Produce Drop Events\n")
	_, err = kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-SetFilter -event 1 -srcIP 23.192.228.84",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	output, err = kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-Dump",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}
	fmt.Println(output)
	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return fmt.Errorf("failed to start event writer")
	}

	numcurls = 10
	for numcurls > 0 {
		_, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			"C:\\event-writer-helper.bat EventWriter-Curl 23.192.228.84",
			v.NonHpcAppNamespace,
			nonHpcLabelSelector)
		if err != nil {
			return err
		}
		numcurls--
	}

	fmt.Println("Waiting for basic metrics to be updated as part of next polling cycle")
	time.Sleep(60 * time.Second)
	promOutput, err = v.GetPromMetrics(ebpfLabelSelector)
	if err != nil {
		return err
	}
	fmt.Println(promOutput)
	postTestFwdCount, _ := prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_count", fwd_labels)
	fmt.Printf("networkobservability_forward_count value %f, labels: %v\n", preTestFwdBytes, fwd_labels)

	postTestFwdBytes, err := prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_bytes", fwd_labels)
	if err != nil {
		return fmt.Errorf("failed to get metric: %w", err)
	}
	fmt.Printf("networkobservability_forward_bytes value %f, labels: %v\n", postTestFwdBytes, fwd_labels)

	postTestDrpBytes, err := prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_bytes", drp_labels)
	if err != nil {
		return fmt.Errorf("failed to get metric: %w", err)
	}
	fmt.Printf("networkobservability_drop_bytes value %f, labels: %v\n", postTestDrpBytes, drp_labels)

	postTestDrpCount, _ := prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_count", drp_labels)
	fmt.Printf("networkobservability_drop_count value %f, labels: %v\n", preTestDrpBytes, drp_labels)

	if postTestFwdBytes < preTestFwdBytes {
		return fmt.Errorf("networkobservability_forward_bytes not incremented")
	}

	if postTestDrpBytes < preTestDrpBytes {
		return fmt.Errorf("networkobservability_drop_bytes not incremented")
	}

	if postTestFwdCount < preTestFwdCount {
		return fmt.Errorf("networkobservability_forward_count not incremented")
	}
	if postTestDrpCount < preTestDrpCount {
		return fmt.Errorf("networkobservability_drop_count not incremnted")
	}

	// Advanced Metrics
	adv_fwd_count_labels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_forward_count", adv_fwd_count_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_forward_count")
	}

	tcpFlags := []string{"ACK", "FIN", "PSH"}
	for _, flag := range tcpFlags {
		tcpFlagLabels := map[string]string{
			"flag":          flag,
			"ip":            "23.192.228.84",
			"namespace":     "",
			"podname":       "",
			"workload_kind": "unknown",
			"workload_name": "unknown",
		}

		err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_tcpflags_count", tcpFlagLabels)
		if err != nil {
			return fmt.Errorf("failed to find networkobservability_adv_tcpflags_count for flag %s: %w", flag, err)
		}
		fmt.Printf("Found TCP flag metric for %s\n", flag)
	}

	adv_drop_byte_labels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"reason":        "Drop_NotAccepted",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_bytes", adv_drop_byte_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_bytes")
	}

	adv_drop_count_labels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"reason":        "Drop_NotAccepted",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_count", adv_drop_count_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_count")
	}

	adv_fwd_count_labels = map[string]string{
		"direction":     "ingres",
		"ip":            nonHpcIpAddr,
		"namespace":     v.NonHpcAppNamespace,
		"podname":       v.NonHpcPodName,
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_forward_count", adv_fwd_count_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_forward_count")
	}

	for _, flag := range tcpFlags {
		tcpFlagLabels := map[string]string{
			"flag":          flag,
			"ip":            nonHpcIpAddr,
			"namespace":     v.NonHpcAppNamespace,
			"podname":       v.NonHpcPodName,
			"workload_kind": "unknown",
			"workload_name": "unknown",
		}

		err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_tcpflags_count", tcpFlagLabels)
		if err != nil {
			return fmt.Errorf("failed to find networkobservability_adv_tcpflags_count for flag %s: %w", flag, err)
		}
		fmt.Printf("Found TCP flag metric for %s\n", flag)
	}

	adv_drop_byte_labels = map[string]string{
		"direction":     "ingress",
		"ip":            nonHpcIpAddr,
		"namespace":     v.NonHpcAppNamespace,
		"podname":       v.NonHpcPodName,
		"reason":        "Drop_NotAccepted",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_bytes", adv_drop_byte_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_bytes with ingress label")
	}

	adv_drop_count_labels = map[string]string{
		"direction":     "ingress",
		"ip":            nonHpcIpAddr,
		"namespace":     v.NonHpcAppNamespace,
		"podname":       v.NonHpcPodName,
		"reason":        "Drop_NotAccepted",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_count", adv_drop_count_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_count with ingress label")
	}

	return nil
}

func (v *ValidateWinBpfMetric) Prevalidate() error {
	return nil
}

func (v *ValidateWinBpfMetric) Stop() error {
	return nil
}
