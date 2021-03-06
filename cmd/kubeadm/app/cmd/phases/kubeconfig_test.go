/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package phases

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/certs/pkiutil"
	testutil "k8s.io/kubernetes/cmd/kubeadm/test"
	cmdtestutil "k8s.io/kubernetes/cmd/kubeadm/test/cmd"
	kubeconfigtestutil "k8s.io/kubernetes/cmd/kubeadm/test/kubeconfig"
)

func TestKubeConfigCSubCommandsHasFlags(t *testing.T) {
	cmd := NewCmdUserKubeConfig(nil, "", phaseTestK8sVersion)

	flags := []string{
		"cert-dir",
		"apiserver-advertise-address",
		"apiserver-bind-port",
		"kubeconfig-dir",
		"token",
		"client-name",
	}

	cmdtestutil.AssertSubCommandHasFlags(t, []*cobra.Command{cmd}, "user", flags...)
}

func TestKubeConfigSubCommandsThatWritesToOut(t *testing.T) {

	// Temporary folders for the test case
	tmpdir := testutil.SetupTempDir(t)
	defer os.RemoveAll(tmpdir)

	// Adds a pki folder with a ca cert to the temp folder
	pkidir := testutil.SetupPkiDirWithCertificateAuthorithy(t, tmpdir)

	outputdir := tmpdir

	// Retrieves ca cert for assertions
	caCert, _, err := pkiutil.TryLoadCertAndKeyFromDisk(pkidir, kubeadmconstants.CACertAndKeyBaseName)
	if err != nil {
		t.Fatalf("couldn't retrieve ca cert: %v", err)
	}

	commonFlags := []string{
		"--apiserver-advertise-address=1.2.3.4",
		"--apiserver-bind-port=1234",
		"--client-name=myUser",
		fmt.Sprintf("--cert-dir=%s", pkidir),
		fmt.Sprintf("--kubeconfig-dir=%s", outputdir),
	}

	var tests = []struct {
		command         string
		withClientCert  bool
		withToken       bool
		additionalFlags []string
	}{
		{ // Test user subCommand withClientCert
			command:        "user",
			withClientCert: true,
		},
		{ // Test user subCommand withToken
			withToken:       true,
			command:         "user",
			additionalFlags: []string{"--token=123456"},
		},
	}

	for _, test := range tests {
		buf := new(bytes.Buffer)

		// Get subcommands working in the temporary directory
		cmd := NewCmdUserKubeConfig(buf, tmpdir, phaseTestK8sVersion)

		// Execute the subcommand
		allFlags := append(commonFlags, test.additionalFlags...)
		cmdtestutil.RunSubCommand(t, []*cobra.Command{cmd}, test.command, allFlags...)

		// reads kubeconfig written to stdout
		config, err := clientcmd.Load(buf.Bytes())
		if err != nil {
			t.Errorf("couldn't read kubeconfig file from buffer: %v", err)
			continue
		}

		// checks that CLI flags are properly propagated
		kubeconfigtestutil.AssertKubeConfigCurrentCluster(t, config, "https://1.2.3.4:1234", caCert)

		if test.withClientCert {
			// checks that kubeconfig files have expected client cert
			kubeconfigtestutil.AssertKubeConfigCurrentAuthInfoWithClientCert(t, config, caCert, "myUser")
		}

		if test.withToken {
			// checks that kubeconfig files have expected token
			kubeconfigtestutil.AssertKubeConfigCurrentAuthInfoWithToken(t, config, "myUser", "123456")
		}
	}
}
