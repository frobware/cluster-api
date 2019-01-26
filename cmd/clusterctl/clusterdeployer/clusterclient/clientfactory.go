/*
Copyright 2018 The Kubernetes Authors.

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

package clusterclient

import (
	"k8s.io/client-go/kubernetes"
	"github.com/openshift/cluster-api/cmd/clusterctl/clientcmd"
)

// Factory can create cluster clients
type Factory interface {
	NewClientFromKubeconfig(string) (Client, error)
	NewCoreClientsetFromKubeconfigFile(string) (*kubernetes.Clientset, error)
}

type clientFactory struct {
}

func NewFactory() *clientFactory {
	return &clientFactory{}
}

func (f *clientFactory) NewClientFromKubeconfig(kubeconfig string) (Client, error) {
	return New(kubeconfig)
}

func (f *clientFactory) NewCoreClientsetFromKubeconfigFile(kubeconfigPath string) (*kubernetes.Clientset, error) {
	return clientcmd.NewCoreClientSetForDefaultSearchPath(kubeconfigPath, clientcmd.NewConfigOverrides())
}
