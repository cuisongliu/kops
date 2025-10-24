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

package config

import (
	"k8s.io/kops/pkg/bootstrap/pkibootstrap"
	"k8s.io/kops/upup/pkg/fi/cloudup/awsup"
	"k8s.io/kops/upup/pkg/fi/cloudup/azure"
	"k8s.io/kops/upup/pkg/fi/cloudup/do"
	gcetpm "k8s.io/kops/upup/pkg/fi/cloudup/gce/tpm"
	"k8s.io/kops/upup/pkg/fi/cloudup/hetzner"
	"k8s.io/kops/upup/pkg/fi/cloudup/openstack"
	"k8s.io/kops/upup/pkg/fi/cloudup/scaleway"
)

type Options struct {
	ClusterName           string         `json:"clusterName,omitempty"`
	Cloud                 string         `json:"cloud,omitempty"`
	ConfigBase            string         `json:"configBase,omitempty"`
	SecretStore           string         `json:"secretStore,omitempty"`
	Server                *ServerOptions `json:"server,omitempty"`
	CacheNodeidentityInfo bool           `json:"cacheNodeidentityInfo,omitempty"`

	// EnableCloudIPAM enables the cloud IPAM controller.
	EnableCloudIPAM bool `json:"enableCloudIPAM,omitempty"`

	// Discovery configures options relating to discovery, particularly for gossip mode.
	Discovery *DiscoveryOptions `json:"discovery,omitempty"`

	// CAPI configures Cluster API (CAPI) support.
	CAPI *CAPIOptions `json:"capi,omitempty"`
}

func (o *Options) PopulateDefaults() {
}

type CAPIOptions struct {
	// Enabled specifies whether CAPI support is enabled.
	Enabled *bool `json:"enabled,omitempty"`
}

// IsEnabled returns true if CAPI support is enabled.
func (o *CAPIOptions) IsEnabled() bool {
	if o == nil || o.Enabled == nil {
		return false
	}
	return *o.Enabled
}

type ServerOptions struct {
	// Listen is the network endpoint (ip and port) we should listen on.
	Listen string

	// Provider is the cloud provider.
	Provider ServerProviderOptions `json:"provider"`

	// PKI configures private/public key node authentication.
	PKI *pkibootstrap.Options `json:"pki,omitempty"`

	// ServerKeyPath is the path to our TLS serving private key.
	ServerKeyPath string `json:"serverKeyPath,omitempty"`
	// ServerCertificatePath is the path to our TLS serving certificate.
	ServerCertificatePath string `json:"serverCertificatePath,omitempty"`

	// CABasePath is a base of the path to CA certificate and key files.
	CABasePath string `json:"caBasePath"`
	// SigningCAs is the list of active signing CAs.
	SigningCAs []string `json:"signingCAs"`
	// CertNames is the list of active certificate names.
	CertNames []string `json:"certNames"`
}

type ServerProviderOptions struct {
	AWS          *awsup.AWSVerifierOptions           `json:"aws,omitempty"`
	GCE          *gcetpm.TPMVerifierOptions          `json:"gce,omitempty"`
	Hetzner      *hetzner.HetznerVerifierOptions     `json:"hetzner,omitempty"`
	OpenStack    *openstack.OpenStackVerifierOptions `json:"openstack,omitempty"`
	DigitalOcean *do.DigitalOceanVerifierOptions     `json:"do,omitempty"`
	Scaleway     *scaleway.ScalewayVerifierOptions   `json:"scaleway,omitempty"`
	Azure        *azure.AzureVerifierOptions         `json:"azure,omitempty"`
}

// DiscoveryOptions configures our support for discovery, particularly gossip DNS (i.e. k8s.local)
type DiscoveryOptions struct {
	// Enabled specifies whether support for discovery population is enabled.
	Enabled bool `json:"enabled"`
}
