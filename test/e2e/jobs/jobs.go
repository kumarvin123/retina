package retina

import (
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/azure"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/hubble"

	"github.com/microsoft/retina/test/e2e/scenarios/dns"
	"github.com/microsoft/retina/test/e2e/scenarios/drop"
	"github.com/microsoft/retina/test/e2e/scenarios/latency"
	tcp "github.com/microsoft/retina/test/e2e/scenarios/tcp"
	"github.com/microsoft/retina/test/e2e/scenarios/windows"
)

func CreateTestInfra(subID, rg, clusterName, location, kubeConfigFilePath string, createInfra bool) *types.Job {
	job := types.NewJob("Create e2e test infrastructure")
	if createInfra {
		job.AddStep(&azure.CreateResourceGroup{
			SubscriptionID:    subID,
			ResourceGroupName: rg,
			Location:          location,
		}, nil)

		job.AddStep(&azure.CreateVNet{
			VnetName:         "testvnet",
			VnetAddressSpace: "10.0.0.0/9",
		}, nil)

		job.AddStep(&azure.CreateSubnet{
			SubnetName:         "testsubnet",
			SubnetAddressSpace: "10.0.0.0/12",
		}, nil)

		job.AddStep(&azure.CreateNPMCluster{
			ClusterName:  clusterName,
			PodCidr:      "10.128.0.0/9",
			DNSServiceIP: "192.168.0.10",
			ServiceCidr:  "192.168.0.0/28",
		}, nil)

		job.AddStep(&azure.GetAKSKubeConfig{
			KubeConfigFilePath: kubeConfigFilePath,
		}, nil)

	} else {
		job.AddStep(&azure.GetAKSKubeConfig{
			KubeConfigFilePath: kubeConfigFilePath,
			ClusterName:        clusterName,
			SubscriptionID:     subID,
			ResourceGroupName:  rg,
			Location:           location,
		}, nil)
	}

	return job
}

func DeleteTestInfra(subID, rg, location string, deleteInfra bool) *types.Job {
	job := types.NewJob("Delete e2e test infrastructure")

	if deleteInfra {
		job.AddStep(&azure.DeleteResourceGroup{
			SubscriptionID:    subID,
			ResourceGroupName: rg,
			Location:          location,
		}, nil)
	}

	return job
}

func InstallRetina(kubeConfigFilePath, chartPath string, enableHeartBeat bool) *types.Job {
	job := types.NewJob("Install and test Retina with basic metrics")

	job.AddStep(&kubernetes.InstallHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
		EnableHeartbeat:    enableHeartBeat,
	}, nil)

	return job
}

func UninstallRetina(kubeConfigFilePath, chartPath string) *types.Job {
	job := types.NewJob("Uninstall Retina")

	job.AddStep(&kubernetes.UninstallHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
	}, nil)

	return job
}

func InstallEbpfXdp(kubeConfigFilePath string) *types.Job {
	job := types.NewJob("Install EBPF and XDP")
	job.AddStep(&kubernetes.CreateNamespace{
		KubeConfigFilePath: kubeConfigFilePath,
		Namespace:          "install-ebpf-xdp"}, nil)

	job.AddStep(&kubernetes.ApplyYamlConfig{
		YamlFilePath: "yaml/windows/install-ebpf-xdp.yaml",
	}, nil)
	return job
}

func InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath string, testPodNamespace string) *types.Job {
	job := types.NewJob("Install and test Retina with basic metrics")

	job.AddStep(&kubernetes.InstallHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
	}, nil)

	dnsScenarios := []struct {
		name string
		req  *dns.RequestValidationParams
		resp *dns.ResponseValidationParams
	}{
		{
			name: "Validate basic DNS request and response metrics for a valid domain",
			req: &dns.RequestValidationParams{
				NumResponse: "0",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
				Command:     "nslookup kubernetes.default",
				ExpectError: false,
			},
			resp: &dns.ResponseValidationParams{
				NumResponse: "1",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
				ReturnCode:  "No Error",
				Response:    "10.0.0.1",
			},
		},
		{
			name: "Validate basic DNS request and response metrics for a non-existent domain",
			req: &dns.RequestValidationParams{
				NumResponse: "0",
				Query:       "some.non.existent.domain.",
				QueryType:   "A",
				Command:     "nslookup some.non.existent.domain",
				ExpectError: true,
			},
			resp: &dns.ResponseValidationParams{
				NumResponse: "0",
				Query:       "some.non.existent.domain.",
				QueryType:   "A",
				Response:    dns.EmptyResponse, // hacky way to bypass the framework for now
				ReturnCode:  "Non-Existent Domain",
			},
		},
	}

	for _, arch := range common.Architectures {
		job.AddScenario(drop.ValidateDropMetric(testPodNamespace, arch))
		job.AddScenario(tcp.ValidateTCPMetrics(testPodNamespace, arch))

		for _, scenario := range dnsScenarios {
			name := scenario.name + " - Arch: " + arch
			job.AddScenario(dns.ValidateBasicDNSMetrics(name, scenario.req, scenario.resp, testPodNamespace, arch))
		}
	}

	job.AddScenario(windows.ValidateWindowsBasicMetric())

	job.AddStep(&kubernetes.EnsureStableComponent{
		PodNamespace:           common.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		IgnoreContainerRestart: false,
	}, nil)

	return job
}

func UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, valuesFilePath string, testPodNamespace string) *types.Job {
	job := types.NewJob("Upgrade and test Retina with advanced metrics")

	// enable advanced metrics
	job.AddStep(&kubernetes.UpgradeRetinaHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
		ValuesFile:         valuesFilePath,
	}, nil)

	dnsScenarios := []struct {
		name string
		req  *dns.RequestValidationParams
		resp *dns.ResponseValidationParams
	}{
		{
			name: "Validate advanced DNS request and response metrics for a valid domain",
			req: &dns.RequestValidationParams{
				NumResponse: "0",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
				Command:     "nslookup kubernetes.default",
				ExpectError: false,
			},
			resp: &dns.ResponseValidationParams{
				NumResponse: "1",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
				ReturnCode:  "NOERROR",
				Response:    "10.0.0.1",
			},
		},
		{
			name: "Validate advanced DNS request and response metrics for a non-existent domain",
			req: &dns.RequestValidationParams{
				NumResponse: "0",
				Query:       "some.non.existent.domain.",
				QueryType:   "A",
				Command:     "nslookup some.non.existent.domain.",
				ExpectError: true,
			},
			resp: &dns.ResponseValidationParams{
				NumResponse: "0",
				Query:       "some.non.existent.domain.",
				QueryType:   "A",
				Response:    dns.EmptyResponse, // hacky way to bypass the framework for now
				ReturnCode:  "NXDOMAIN",
			},
		},
	}

	// Validate Windows BPF Metrics
	job.AddStep(&kubernetes.ApplyYamlConfig{
		YamlFilePath: "yaml/windows/non-hpc-pod.yaml",
	}, nil)

	for _, arch := range common.Architectures {
		for _, scenario := range dnsScenarios {
			name := scenario.name + " - Arch: " + arch
			job.AddScenario(dns.ValidateAdvancedDNSMetrics(name, scenario.req, scenario.resp, kubeConfigFilePath, testPodNamespace, arch))
		}
	}

	job.AddScenario(windows.ValidateWinBpfMetricScenario())
	job.AddScenario(latency.ValidateLatencyMetric(testPodNamespace))

	job.AddStep(&kubernetes.EnsureStableComponent{
		PodNamespace:           common.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		IgnoreContainerRestart: false,
	}, nil)

	return job
}

func ValidateHubble(kubeConfigFilePath, chartPath string, testPodNamespace string) *types.Job {
	job := types.NewJob("Validate Hubble")

	job.AddStep(&kubernetes.ValidateHubbleStep{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
	}, nil)

	job.AddScenario(hubble.ValidateHubbleRelayService())

	job.AddScenario(hubble.ValidateHubbleUIService(kubeConfigFilePath))

	job.AddStep(&kubernetes.EnsureStableComponent{
		PodNamespace:           common.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		IgnoreContainerRestart: false,
	}, nil)

	return job
}

func LoadGenericFlags() *types.Job {
	job := types.NewJob("Loading Generic Flags to env")

	job.AddStep(&generic.LoadFlags{
		TagEnv:            generic.DefaultTagEnv,
		ImageNamespaceEnv: generic.DefaultImageNamespace,
		ImageRegistryEnv:  generic.DefaultImageRegistry,
	}, nil)

	return job
}

func LoadAndPinWinBPFJob(kubeConfigFilePath string) *types.Job {
	job := types.NewJob("Load Windows BPF Maps")
	job.AddStep(&kubernetes.LoadAndPinWinBPF{
		KubeConfigFilePath:                 kubeConfigFilePath,
		LoadAndPinWinBPFDeamonSetNamespace: "install-ebpf-xdp",
		LoadAndPinWinBPFDeamonSetName:      "install-ebpf-xdp",
	}, nil)

	return job
}

func UnLoadAndPinWinBPFJob(kubeConfigFilePath string) *types.Job {
	job := types.NewJob("Unload Windows BPF Maps")
	job.AddStep(&kubernetes.UnLoadAndPinWinBPF{
		KubeConfigFilePath:                   kubeConfigFilePath,
		UnLoadAndPinWinBPFDeamonSetNamespace: "install-ebpf-xdp",
		UnLoadAndPinWinBPFDeamonSetName:      "install-ebpf-xdp",
	}, nil)

	return job
}
