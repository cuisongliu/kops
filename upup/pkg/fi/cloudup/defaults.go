/*
Copyright 2019 The Kubernetes Authors.

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

package cloudup

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azure"
	"k8s.io/kops/upup/pkg/fi/cloudup/gce"
	"k8s.io/kops/util/pkg/vfs"

	kopsversion "k8s.io/kops"
)

const (
	defaultAWSNetworkCIDR    = "172.20.0.0/16"
	defaultAzureNetworkCIDR  = "10.0.0.0/16"
	defaultNonMasqueradeCIDR = "100.64.0.0/10"
)

// PerformAssignments populates values that are required and immutable
// For example, it assigns stable Keys to InstanceGroups & Masters, and
// it assigns CIDRs to subnets
// We also assign KubernetesVersion, because we want it to be explicit
//
// PerformAssignments is called on create, as well as an update. In fact
// any time Run() is called in apply_cluster.go we will reach this function.
// Please do all after-market logic here.
func PerformAssignments(c *kops.Cluster, vfsContext *vfs.VFSContext, cloud fi.Cloud) error {
	ctx := context.TODO()

	for i := range c.Spec.EtcdClusters {
		etcdCluster := &c.Spec.EtcdClusters[i]
		if etcdCluster.Manager == nil {
			etcdCluster.Manager = &kops.EtcdManagerSpec{}
		}
		if etcdCluster.Manager.BackupRetentionDays == nil {
			etcdCluster.Manager.BackupRetentionDays = fi.PtrTo[uint32](90)
		}
	}

	// Topology support
	// TODO Kris: Unsure if this needs to be here, or if the API conversion code will handle it
	if c.Spec.Networking.Topology == nil {
		c.Spec.Networking.Topology = &kops.TopologySpec{}
	}

	if cloud == nil {
		return fmt.Errorf("cloud cannot be nil")
	}

	if cloud.ProviderID() == kops.CloudProviderGCE {
		if err := gce.PerformNetworkAssignments(ctx, c, cloud); err != nil {
			return err
		}
	}

	if cloud.ProviderID() == kops.CloudProviderAWS && c.Spec.Networking.NetworkCIDR == "" {
		if c.SharedVPC() {
			vpcInfo, err := cloud.FindVPCInfo(c.Spec.Networking.NetworkID)
			if err != nil {
				return err
			}
			if vpcInfo == nil {
				return fmt.Errorf("unable to find Network ID %q", c.Spec.Networking.NetworkID)
			}
			c.Spec.Networking.NetworkCIDR = vpcInfo.CIDR
			if c.Spec.Networking.NetworkCIDR == "" {
				return fmt.Errorf("unable to infer NetworkCIDR from Network ID, please specify --network-cidr")
			}
		} else {
			// TODO: Choose non-overlapping networking CIDRs for VPCs, using vpcInfo
			c.Spec.Networking.NetworkCIDR = defaultAWSNetworkCIDR
		}

		// Amazon VPC CNI uses the same network
		if c.Spec.Networking.AmazonVPC != nil && c.Spec.Networking.NonMasqueradeCIDR != "::/0" {
			c.Spec.Networking.NonMasqueradeCIDR = c.Spec.Networking.NetworkCIDR
		}
	}

	if cloud.ProviderID() == kops.CloudProviderAzure && c.Spec.Networking.NetworkCIDR == "" {
		if c.SharedVPC() {
			if c.Spec.CloudProvider.Azure == nil || c.Spec.CloudProvider.Azure.ResourceGroupName == "" {
				return fmt.Errorf("missing required --azure-resource-group-name when specifying Network ID")
			}
			vpcInfo, err := cloud.(azure.AzureCloud).FindVNetInfo(c.Spec.Networking.NetworkID, c.Spec.CloudProvider.Azure.ResourceGroupName)
			if err != nil {
				return err
			}
			if vpcInfo == nil {
				return fmt.Errorf("unable to find Network ID %q", c.Spec.Networking.NetworkID)
			}
			c.Spec.Networking.NetworkCIDR = vpcInfo.CIDR
			if c.Spec.Networking.NetworkCIDR == "" {
				return fmt.Errorf("unable to infer NetworkCIDR from Network ID, please specify --network-cidr")
			}
		} else {
			c.Spec.Networking.NetworkCIDR = defaultAzureNetworkCIDR
		}
	}

	if c.Spec.Networking.NonMasqueradeCIDR == "" {
		c.Spec.Networking.NonMasqueradeCIDR = defaultNonMasqueradeCIDR
	}

	// TODO: Unclear this should be here - it isn't too hard to change
	if c.UsesPublicDNS() && c.Spec.API.PublicName == "" && c.ObjectMeta.Name != "" {
		c.Spec.API.PublicName = "api." + c.ObjectMeta.Name
	}

	// We only assign subnet CIDRs on AWS, OpenStack, and Azure.
	pd := cloud.ProviderID()
	if pd == kops.CloudProviderAWS || pd == kops.CloudProviderOpenstack || pd == kops.CloudProviderAzure {
		// TODO: Use vpcInfo
		err := assignCIDRsToSubnets(c, cloud)
		if err != nil {
			return err
		}
	}

	proxy, err := assignProxy(c)
	if err != nil {
		return err
	}
	c.Spec.Networking.EgressProxy = proxy

	if c.Spec.CloudProvider.Azure != nil && c.Spec.CloudProvider.Azure.StorageAccountID == "" {
		storageAccountName := os.Getenv("AZURE_STORAGE_ACCOUNT")
		if storageAccountName == "" {
			return fmt.Errorf("AZURE_STORAGE_ACCOUNT must be set")
		}
		sa, err := cloud.(azure.AzureCloud).FindStorageAccountInfo(storageAccountName)
		if err != nil {
			return err
		}
		klog.Infof("Found storage account %q", *sa.ID)
		c.Spec.CloudProvider.Azure.StorageAccountID = *sa.ID
	}

	return ensureKubernetesVersion(vfsContext, c)
}

// ensureKubernetesVersion populates KubernetesVersion, if it is not already set
// It will be populated with the latest stable kubernetes version, or the version from the channel
func ensureKubernetesVersion(vfsContext *vfs.VFSContext, c *kops.Cluster) error {
	if c.Spec.KubernetesVersion == "" {
		if c.Spec.Channel != "" {
			channel, err := kops.LoadChannel(vfsContext, c.Spec.Channel)
			if err != nil {
				return err
			}
			kubernetesVersion := kops.RecommendedKubernetesVersion(channel, kopsversion.Version)
			if kubernetesVersion != nil {
				c.Spec.KubernetesVersion = kubernetesVersion.String()
				klog.Infof("Using KubernetesVersion %q from channel %q", c.Spec.KubernetesVersion, c.Spec.Channel)
			} else {
				klog.Warningf("Cannot determine recommended kubernetes version from channel %q", c.Spec.Channel)
			}
		} else {
			klog.Warningf("Channel is not set; cannot determine KubernetesVersion from channel")
		}
	}

	if c.Spec.KubernetesVersion == "" {
		latestVersion, err := FindLatestKubernetesVersion()
		if err != nil {
			return err
		}
		klog.Infof("Using kubernetes latest stable version: %s", latestVersion)
		c.Spec.KubernetesVersion = latestVersion
	}
	return nil
}

// FindLatestKubernetesVersion returns the latest kubernetes version,
// as stored at https://dl.k8s.io/release/stable.txt
// This shouldn't be used any more; we prefer reading the stable channel
func FindLatestKubernetesVersion() (string, error) {
	stableURL := "https://dl.k8s.io/release/stable.txt"
	klog.Warningf("Loading latest kubernetes version from %q", stableURL)
	b, err := vfs.Context.ReadFile(stableURL)
	if err != nil {
		return "", fmt.Errorf("KubernetesVersion not specified, and unable to download latest version from %q: %v", stableURL, err)
	}
	latestVersion := strings.TrimSpace(string(b))
	return latestVersion, nil
}

func assignProxy(cluster *kops.Cluster) (*kops.EgressProxySpec, error) {
	egressProxy := cluster.Spec.Networking.EgressProxy
	// Add default no_proxy values if we are using a http proxy
	if egressProxy != nil {

		var egressSlice []string
		if egressProxy.ProxyExcludes != "" {
			egressSlice = strings.Split(egressProxy.ProxyExcludes, ",")
		}

		ip, _, err := net.ParseCIDR(cluster.Spec.Networking.NonMasqueradeCIDR)
		if err != nil {
			return nil, fmt.Errorf("unable to parse Non Masquerade CIDR")
		}

		firstIP, err := incrementIP(ip, cluster.Spec.Networking.NonMasqueradeCIDR)
		if err != nil {
			return nil, fmt.Errorf("unable to get first ip address in Non Masquerade CIDR")
		}

		// run through the basic list
		for _, exclude := range []string{
			"127.0.0.1",
			"localhost",
			cluster.Spec.ClusterDNSDomain, // TODO we may want this for public loadbalancers
			cluster.Spec.API.PublicName,
			cluster.ObjectMeta.Name,
			firstIP,
			cluster.Spec.Networking.NonMasqueradeCIDR,
		} {
			if exclude == "" {
				continue
			}
			if !strings.Contains(egressProxy.ProxyExcludes, exclude) {
				egressSlice = append(egressSlice, exclude)
			}
		}

		awsNoProxy := "169.254.169.254"

		if cluster.GetCloudProvider() == kops.CloudProviderAWS && !strings.Contains(cluster.Spec.Networking.EgressProxy.ProxyExcludes, awsNoProxy) {
			egressSlice = append(egressSlice, awsNoProxy)
		}

		// the kube-apiserver will need to talk to kubelets on their node IP addresses port 10250
		// for pod logs to be available via the api
		if cluster.Spec.Networking.NetworkCIDR != "" {
			if !strings.Contains(cluster.Spec.Networking.EgressProxy.ProxyExcludes, cluster.Spec.Networking.NetworkCIDR) {
				egressSlice = append(egressSlice, cluster.Spec.Networking.NetworkCIDR)
			}
		} else {
			klog.Warningf("No NetworkCIDR defined (yet), not adding to egressProxy.excludes")
		}

		for _, cidr := range cluster.Spec.Networking.AdditionalNetworkCIDRs {
			if !strings.Contains(cluster.Spec.Networking.EgressProxy.ProxyExcludes, cidr) {
				egressSlice = append(egressSlice, cidr)
			}
		}

		egressProxy.ProxyExcludes = strings.Join(egressSlice, ",")
		klog.V(8).Infof("Completed setting up Proxy excludes as follows: %q", egressProxy.ProxyExcludes)
	} else {
		klog.V(8).Info("Not setting up Proxy Excludes")
	}

	return egressProxy, nil
}

func incrementIP(ip net.IP, cidr string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
	if !ipNet.Contains(ip) {
		return "", fmt.Errorf("overflowed CIDR while incrementing IP")
	}
	return ip.String(), nil
}
