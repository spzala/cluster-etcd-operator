package render

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/cluster-etcd-operator/pkg/cmd/render/options"
)

var (
	expectedServiceIPv4CIDR        = []string{"172.30.0.0/16"}
	expectedServiceMixedCIDR       = []string{"172.30.0.0/16", "2001:db8::/32"}
	expectedServiceMixedSwapCIDR   = []string{"2001:db8::/32", "172.30.0.0/16"}
	expectedServiceSingleStackCIDR = []string{"2001:db8::/32"}

	clusterAPIConfig = `
apiVersion: machine.openshift.io/v1beta1
kind: Cluster
metadata:
  creationTimestamp: null
  name: cluster
  namespace: openshift-machine-api
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 2001:db8::/32
    serviceDomain: ""
    services:
      cidrBlocks:
        - 172.30.0.0/16
  providerSpec: {}
status: {}
`
	networkConfigIpv4 = `
apiVersion: config.openshift.io/v1
kind: Network
metadata:
  creationTimestamp: null
  name: cluster
spec:
  clusterNetwork:
    - cidr: 10.128.0.0/14
      hostPrefix: 23
  networkType: OpenShiftSDN
  serviceNetwork:
    - 172.30.0.0/16
status: {}
`
	networkConfigMixed = `
apiVersion: config.openshift.io/v1
kind: Network
metadata:
  creationTimestamp: null
  name: cluster
spec:
  clusterNetwork:
    - cidr: 10.128.0.0/14
      hostPrefix: 23
  networkType: OpenShiftSDN
  serviceNetwork:
    - 172.30.0.0/16
    - 2001:db8::/32
status: {}
`
	networkConfigMixedSwap = `
apiVersion: config.openshift.io/v1
kind: Network
metadata:
  creationTimestamp: null
  name: cluster
spec:
  clusterNetwork:
    - cidr: 10.128.10.0/14
      hostPrefix: 23
  networkType: OpenShiftSDN
  serviceNetwork:
    - 2001:db8::/32
    - 172.30.0.0/16
status: {}
`
	networkConfigIPv6SingleStack = `
apiVersion: config.openshift.io/v1
kind: Network
metadata:
  creationTimestamp: null
  name: cluster
spec:
  clusterNetwork:
    - cidr: 10.128.0.0/14
      hostPrefix: 23
  networkType: OpenShiftSDN
  serviceNetwork:
    - 2001:db8::/32
status: {}
`
	infraConfig = `
apiVersion: config.openshift.io/v1
kind: Infrastructure
metadata:
  name: cluster
spec:
  cloudConfig:
    name: ""
status:
  platform: AWS
  platformStatus:
    aws:
      region: us-east-1
    type: AWS
`
)

type testConfig struct {
	t                    *testing.T
	clusterNetworkConfig string
	infraConfig          string
	want                 TemplateData
	bootstrapIP          string
}

func TestRenderIpv4(t *testing.T) {
	want := TemplateData{
		ManifestConfig: options.ManifestConfig{
			EtcdAddress: options.EtcdAddress{
				LocalHost: "127.0.0.1",
			},
		},
		ClusterCIDR:     []string{"10.128.0.0/14"},
		ServiceCIDR:     []string{"172.30.0.0/16"},
		SingleStackIPv6: false,
	}

	config := &testConfig{
		t:                    t,
		clusterNetworkConfig: networkConfigIpv4,
		infraConfig:          infraConfig,
		want:                 want,
	}

	testRender(config)
}

func testRender(tc *testConfig) {
	var errOut io.Writer
	dir, err := ioutil.TempDir("/tmp", "assets-")
	if err != nil {
		tc.t.Fatal(err)
	}

	defer os.RemoveAll(dir) // clean up

	clusterConfigFile, err := ioutil.TempFile(dir, "cluster-network-02-config.*.yaml")
	if err != nil {
		tc.t.Fatal(err)
	}

	infraConfigFile, err := ioutil.TempFile(dir, "cluster-infrastructure-02-config.*.yaml")
	if err != nil {
		tc.t.Fatal(err)
	}
	defer infraConfigFile.Close()

	if err := writeFile(tc.clusterNetworkConfig, clusterConfigFile); err != nil {
		tc.t.Fatal(err)
	}

	if err := writeFile(tc.infraConfig, infraConfigFile); err != nil {
		tc.t.Fatal(err)
	}

	generic := options.GenericOptions{
		AssetInputDir:    dir,
		AssetOutputDir:   dir,
		TemplatesDir:     filepath.Join("../../..", "bindata", "bootkube"),
		ConfigOutputFile: filepath.Join(dir, "config"),
	}

	render := renderOpts{
		generic:           generic,
		manifest:          *options.NewManifestOptions("etcd"),
		errOut:            errOut,
		clusterConfigFile: clusterConfigFile.Name(),
		infraConfigFile:   infraConfigFile.Name(),
	}

	if err := render.Run(); err != nil {
		tc.t.Errorf("failed render.Run(): %v", err)
	}
}

func TestTemplateDataIpv4(t *testing.T) {
	want := TemplateData{
		ManifestConfig: options.ManifestConfig{
			EtcdAddress: options.EtcdAddress{
				LocalHost: "127.0.0.1",
			},
		},
		ClusterCIDR:     []string{"10.128.0.0/14"},
		ServiceCIDR:     []string{"172.30.0.0/16"},
		SingleStackIPv6: false,
	}

	config := &testConfig{
		t:                    t,
		clusterNetworkConfig: networkConfigIpv4,
		infraConfig:          infraConfig,
		want:                 want,
	}
	testTemplateData(config)
}

func TestTemplateDataMixed(t *testing.T) {
	want := TemplateData{
		ManifestConfig: options.ManifestConfig{
			EtcdAddress: options.EtcdAddress{
				LocalHost: "127.0.0.1",
			},
		},
		ClusterCIDR:     []string{"10.128.10.0/14"},
		ServiceCIDR:     []string{"2001:db8::/32", "172.30.0.0/16"},
		SingleStackIPv6: false,
	}

	config := &testConfig{
		t:                    t,
		clusterNetworkConfig: networkConfigMixedSwap,
		infraConfig:          infraConfig,
		want:                 want,
	}
	testTemplateData(config)
}

func TestTemplateDataSingleStack(t *testing.T) {
	want := TemplateData{
		ManifestConfig: options.ManifestConfig{
			EtcdAddress: options.EtcdAddress{
				LocalHost: "[::1]",
			},
		},
		ClusterCIDR:     []string{"10.128.0.0/14"},
		ServiceCIDR:     []string{"2001:db8::/32"},
		SingleStackIPv6: true,
		BootstrapIP:     "2001:0DB8:C21A",
	}

	config := &testConfig{
		t:                    t,
		clusterNetworkConfig: networkConfigIPv6SingleStack,
		infraConfig:          infraConfig,
		want:                 want,
		bootstrapIP:          "2001:0DB8:C21A",
	}
	testTemplateData(config)
}

func testTemplateData(tc *testConfig) {
	var errOut io.Writer
	dir, err := ioutil.TempDir("/tmp", "assets-")
	if err != nil {
		tc.t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	clusterConfigFile, err := ioutil.TempFile(dir, "cluster-network-02-config.*.yaml")
	if err != nil {
		tc.t.Fatal(err)
	}
	defer clusterConfigFile.Close()

	infraConfigFile, err := ioutil.TempFile(dir, "cluster-infrastructures-02-config.*.yaml")
	if err != nil {
		tc.t.Fatal(err)
	}
	defer infraConfigFile.Close()

	if err := writeFile(tc.clusterNetworkConfig, clusterConfigFile); err != nil {
		tc.t.Fatal(err)
	}

	if err := writeFile(tc.infraConfig, infraConfigFile); err != nil {
		tc.t.Fatal(err)
	}

	generic := options.GenericOptions{
		AssetInputDir:    dir,
		AssetOutputDir:   dir,
		TemplatesDir:     filepath.Join("../../..", "bindata", "bootkube"),
		ConfigOutputFile: filepath.Join(dir, "config"),
	}

	render := &renderOpts{
		generic:           generic,
		manifest:          *options.NewManifestOptions("etcd"),
		errOut:            errOut,
		clusterConfigFile: clusterConfigFile.Name(),
		infraConfigFile:   infraConfigFile.Name(),
		bootstrapIP:       tc.bootstrapIP,
	}

	got, err := newTemplateData(render)
	if err != nil {
		tc.t.Fatal(err)
	}

	switch {
	case got.ClusterCIDR[0] != tc.want.ClusterCIDR[0]:
		tc.t.Errorf("ClusterCIDR[0] want: %q got: %q", tc.want.ClusterCIDR[0], got.ClusterCIDR[0])
	case len(got.ClusterCIDR) != len(tc.want.ClusterCIDR):
		tc.t.Errorf("len(ClusterCIDR) want: %d got: %d", len(tc.want.ClusterCIDR), len(got.ClusterCIDR))
	case got.ServiceCIDR[0] != tc.want.ServiceCIDR[0]:
		tc.t.Errorf("ServiceCIDR[0] want: %q got: %q", tc.want.ServiceCIDR[0], got.ServiceCIDR[0])
	case len(got.ServiceCIDR) != len(tc.want.ServiceCIDR):
		tc.t.Errorf("len(ServiceCIDR) want: %d got: %d", len(tc.want.ServiceCIDR), len(got.ServiceCIDR))
	case got.SingleStackIPv6 != tc.want.SingleStackIPv6:
		tc.t.Errorf("SingleStackIPv6 want: %v got: %v", tc.want.SingleStackIPv6, got.SingleStackIPv6)
	case got.ManifestConfig.EtcdAddress.LocalHost != tc.want.ManifestConfig.EtcdAddress.LocalHost:
		tc.t.Errorf("LocalHost want: %q got: %q", tc.want.ManifestConfig.EtcdAddress.LocalHost, got.ManifestConfig.EtcdAddress.LocalHost)
	case got.BootstrapIP != tc.want.BootstrapIP:
		// if we don't say we want a specific IP we dont fail.
		if tc.want.BootstrapIP != "" {
			tc.t.Errorf("BootstrapIP want: %q got: %q", tc.want.BootstrapIP, got.BootstrapIP)
		}
	}
}

func writeFile(input string, w io.Writer) error {
	var buffer bytes.Buffer
	buffer.WriteString(input)
	if _, err := buffer.WriteTo(w); err != nil {
		return err
	}
	return nil
}

func TestTemplateData_setPlatform(t1 *testing.T) {
	tests := []struct {
		name         string
		infraSpec    string
		wantErr      bool
		wantPlatform configv1.PlatformType
	}{
		{
			name: "test infra config file with AWS",
			infraSpec: `apiVersion: config.openshift.io/v1
kind: Infrastructure
metadata:
  name: cluster
spec:
  cloudConfig:
    name: ""
status:
  platform: AWS
  platformStatus:
    aws:
      region: us-east-1
    type: AWS
`,
			wantErr:      false,
			wantPlatform: configv1.AWSPlatformType,
		},
		{
			name: "test infra config file with empty platform",
			infraSpec: `apiVersion: config.openshift.io/v1
kind: Infrastructure
metadata:
  name: cluster
spec:
  cloudConfig:
    name: ""
status:
  platform: ""
  platformStatus:
    type: ""
`,
			wantErr:      false,
			wantPlatform: "",
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			file, err := ioutil.TempFile("/tmp", "infra-config-file")
			if err != nil {
				t1.Fatal(err)
			}

			err = ioutil.WriteFile(file.Name(), []byte(tt.infraSpec), os.ModePerm)
			if err != nil {
				t1.Fatal(err)
			}

			t := &TemplateData{}
			if err := t.setPlatform(file.Name()); (err != nil) != tt.wantErr {
				t1.Errorf("setPlatform() error = %v, wantErr %v", err, tt.wantErr)
			}
			if t.Platform != string(tt.wantPlatform) {
				t1.Errorf("setPlatform() want = %v, got %v", tt.wantPlatform, t.Platform)
			}
			os.Remove(file.Name())
		})
	}
}
