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

package testutil

import "github.com/openshift/cluster-api/pkg/apis/cluster/v1beta1"

// GetVanillaCluster return a bare minimum functional cluster resource object
func GetVanillaCluster() v1beta1.Cluster {
	return v1beta1.Cluster{
		Spec: v1beta1.ClusterSpec{
			ClusterNetwork: v1beta1.ClusterNetworkingConfig{
				Services: v1beta1.NetworkRanges{
					CIDRBlocks: []string{"10.96.0.0/12"},
				},
				Pods: v1beta1.NetworkRanges{
					CIDRBlocks: []string{"192.168.0.0/16"},
				},
				ServiceDomain: "cluster.local",
			},
		},
	}
}
