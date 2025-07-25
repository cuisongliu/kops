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

package kubeconfig

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
	"k8s.io/kops/pkg/cloudinstances"
	"k8s.io/kops/pkg/testutils"

	"github.com/google/go-cmp/cmp"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/pki"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/util/pkg/vfs"
)

const (
	certData        = "-----BEGIN CERTIFICATE-----\nMIIC2DCCAcCgAwIBAgIRALJXAkVj964tq67wMSI8oJQwDQYJKoZIhvcNAQELBQAw\nFTETMBEGA1UEAxMKa3ViZXJuZXRlczAeFw0xNzEyMjcyMzUyNDBaFw0yNzEyMjcy\nMzUyNDBaMBUxEzARBgNVBAMTCmt1YmVybmV0ZXMwggEiMA0GCSqGSIb3DQEBAQUA\nA4IBDwAwggEKAoIBAQDgnCkSmtnmfxEgS3qNPaUCH5QOBGDH/inHbWCODLBCK9gd\nXEcBl7FVv8T2kFr1DYb0HVDtMI7tixRVFDLgkwNlW34xwWdZXB7GeoFgU1xWOQSY\nOACC8JgYTQ/139HBEvgq4sej67p+/s/SNcw34Kk7HIuFhlk1rRk5kMexKIlJBKP1\nYYUYetsJ/QpUOkqJ5HW4GoetE76YtHnORfYvnybviSMrh2wGGaN6r/s4ChOaIbZC\nAn8/YiPKGIDaZGpj6GXnmXARRX/TIdgSQkLwt0aTDBnPZ4XvtpI8aaL8DYJIqAzA\nNPH2b4/uNylat5jDo0b0G54agMi97+2AUrC9UUXpAgMBAAGjIzAhMA4GA1UdDwEB\n/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBVGR2r\nhzXzRMU5wriPQAJScszNORvoBpXfZoZ09FIupudFxBVU3d4hV9StKnQgPSGA5XQO\nHE97+BxJDuA/rB5oBUsMBjc7y1cde/T6hmi3rLoEYBSnSudCOXJE4G9/0f8byAJe\nrN8+No1r2VgZvZh6p74TEkXv/l3HBPWM7IdUV0HO9JDhSgOVF1fyQKJxRuLJR8jt\nO6mPH2UX0vMwVa4jvwtkddqk2OAdYQvH9rbDjjbzaiW0KnmdueRo92KHAN7BsDZy\nVpXHpqo1Kzg7D3fpaXCf5si7lqqrdJVXH4JC72zxsPehqgi8eIuqOBkiDWmRxAxh\n8yGeRx9AbknHh4Ia\n-----END CERTIFICATE-----\n"
	privatekeyData  = "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA4JwpEprZ5n8RIEt6jT2lAh+UDgRgx/4px21gjgywQivYHVxH\nAZexVb/E9pBa9Q2G9B1Q7TCO7YsUVRQy4JMDZVt+McFnWVwexnqBYFNcVjkEmDgA\ngvCYGE0P9d/RwRL4KuLHo+u6fv7P0jXMN+CpOxyLhYZZNa0ZOZDHsSiJSQSj9WGF\nGHrbCf0KVDpKieR1uBqHrRO+mLR5zkX2L58m74kjK4dsBhmjeq/7OAoTmiG2QgJ/\nP2IjyhiA2mRqY+hl55lwEUV/0yHYEkJC8LdGkwwZz2eF77aSPGmi/A2CSKgMwDTx\n9m+P7jcpWreYw6NG9BueGoDIve/tgFKwvVFF6QIDAQABAoIBAA0ktjaTfyrAxsTI\nBezb7Zr5NBW55dvuII299cd6MJo+rI/TRYhvUv48kY8IFXp/hyUjzgeDLunxmIf9\n/Zgsoic9Ol44/g45mMduhcGYPzAAeCdcJ5OB9rR9VfDCXyjYLlN8H8iU0734tTqM\n0V13tQ9zdSqkGPZOIcq/kR/pylbOZaQMe97BTlsAnOMSMKDgnftY4122Lq3GYy+t\nvpr+bKVaQZwvkLoSU3rECCaKaghgwCyX7jft9aEkhdJv+KlwbsGY6WErvxOaLWHd\ncuMQjGapY1Fa/4UD00mvrA260NyKfzrp6+P46RrVMwEYRJMIQ8YBAk6N6Hh7dc0G\n8Z6i1m0CgYEA9HeCJR0TSwbIQ1bDXUrzpftHuidG5BnSBtax/ND9qIPhR/FBW5nj\n22nwLc48KkyirlfIULd0ae4qVXJn7wfYcuX/cJMLDmSVtlM5Dzmi/91xRiFgIzx1\nAsbBzaFjISP2HpSgL+e9FtSXaaqeZVrflitVhYKUpI/AKV31qGHf04sCgYEA6zTV\n99Sb49Wdlns5IgsfnXl6ToRttB18lfEKcVfjAM4frnkk06JpFAZeR+9GGKUXZHqs\nz2qcplw4d/moCC6p3rYPBMLXsrGNEUFZqBlgz72QA6BBq3X0Cg1Bc2ZbK5VIzwkg\nST2SSux6ccROfgULmN5ZiLOtdUKNEZpFF3i3qtsCgYADT/s7dYFlatobz3kmMnXK\nsfTu2MllHdRys0YGHu7Q8biDuQkhrJwhxPW0KS83g4JQym+0aEfzh36bWcl+u6R7\nKhKj+9oSf9pndgk345gJz35RbPJYh+EuAHNvzdgCAvK6x1jETWeKf6btj5pF1U1i\nQ4QNIw/QiwIXjWZeubTGsQKBgQCbduLu2rLnlyyAaJZM8DlHZyH2gAXbBZpxqU8T\nt9mtkJDUS/KRiEoYGFV9CqS0aXrayVMsDfXY6B/S/UuZjO5u7LtklDzqOf1aKG3Q\ndGXPKibknqqJYH+bnUNjuYYNerETV57lijMGHuSYCf8vwLn3oxBfERRX61M/DU8Z\nworz/QKBgQDCTJI2+jdXg26XuYUmM4XXfnocfzAXhXBULt1nENcogNf1fcptAVtu\nBAiz4/HipQKqoWVUYmxfgbbLRKKLK0s0lOWKbYdVjhEm/m2ZU8wtXTagNwkIGoyq\nY/C1Lox4f1ROJnCjc/hfcOjcxX5M8A8peecHWlVtUPKTJgxQ7oMKcw==\n-----END RSA PRIVATE KEY-----\n"
	nextCertificate = "-----BEGIN CERTIFICATE-----\nMIIBZzCCARGgAwIBAgIBBDANBgkqhkiG9w0BAQsFADAaMRgwFgYDVQQDEw9zZXJ2\naWNlLWFjY291bnQwHhcNMjEwNTAyMjAzMjE3WhcNMzEwNTAyMjAzMjE3WjAaMRgw\nFgYDVQQDEw9zZXJ2aWNlLWFjY291bnQwXDANBgkqhkiG9w0BAQEFAANLADBIAkEA\no4Tridlsf4Yz3UAiup/scSTiG/OqxkUW3Fz7zGKvVcLeYj9GEIKuzoB1VFk1nboD\nq4cCuGLfdzaQdCQKPIsDuwIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0T\nAQH/BAUwAwEB/zAdBgNVHQ4EFgQUhPbxEmUbwVOCa+fZgxreFhf67UEwDQYJKoZI\nhvcNAQELBQADQQALMsyK2Q7C/bk27eCvXyZKUfrLvor10hEjwGhv14zsKWDeTj/J\nA1LPYp7U9VtFfgFOkVbkLE9Rstc0ltNrPqxA\n-----END CERTIFICATE-----\n"
)

// mock a fake status store.
type fakeStatusCloud struct {
	GetApiIngressStatusFn func(cluster *kops.Cluster) ([]fi.ApiIngressStatus, error)
}

var _ fi.Cloud = &fakeStatusCloud{}

func (f fakeStatusCloud) GetApiIngressStatus(cluster *kops.Cluster) ([]fi.ApiIngressStatus, error) {
	return f.GetApiIngressStatusFn(cluster)
}

func (f fakeStatusCloud) ProviderID() kops.CloudProviderID {
	panic("not implemented")
}

func (f fakeStatusCloud) DNS() (dnsprovider.Interface, error) {
	panic("not implemented")
}

func (f fakeStatusCloud) FindVPCInfo(id string) (*fi.VPCInfo, error) {
	panic("not implemented")
}

func (f fakeStatusCloud) DeleteInstance(instance *cloudinstances.CloudInstance) error {
	panic("not implemented")
}

func (f fakeStatusCloud) DeleteGroup(group *cloudinstances.CloudInstanceGroup) error {
	panic("not implemented")
}

func (f fakeStatusCloud) DetachInstance(instance *cloudinstances.CloudInstance) error {
	panic("not implemented")
}

func (f fakeStatusCloud) DeregisterInstance(instance *cloudinstances.CloudInstance) error {
	panic("not implemented")
}

func (f fakeStatusCloud) GetCloudGroups(cluster *kops.Cluster, instancegroups []*kops.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*cloudinstances.CloudInstanceGroup, error) {
	panic("not implemented")
}

func (f fakeStatusCloud) Region() string {
	panic("not implemented")
}

func (f fakeStatusCloud) FindClusterStatus(cluster *kops.Cluster) (*kops.ClusterStatus, error) {
	panic("not implemented")
}

// mock a fake key store
type fakeKeyStore struct {
	FindKeysetFn func(name string) (*fi.Keyset, error)

	// StoreKeysetFn writes the keyset to the store.
	StoreKeysetFn func(name string, keyset *fi.Keyset) error

	// MirrorTo will copy secrets to a vfs.Path, which is often easier for a machine to read
	MirrorToFn func(basedir vfs.Path) error
}

// FindKeyset implements KeystoreReader.
func (f fakeKeyStore) FindKeyset(ctx context.Context, name string) (*fi.Keyset, error) {
	return f.FindKeysetFn(name)
}

func (f fakeKeyStore) StoreKeyset(ctx context.Context, name string, keyset *fi.Keyset) error {
	return f.StoreKeysetFn(name, keyset)
}

func (f fakeKeyStore) MirrorTo(ctx context.Context, basedir vfs.Path) error {
	return f.MirrorToFn(basedir)
}

// build a generic minimal cluster
func buildMinimalCluster(clusterName string, masterPublicName string, lbCert bool, nlb bool) *kops.Cluster {
	cluster := testutils.BuildMinimalCluster(clusterName)
	cluster.Spec.API.PublicName = masterPublicName
	cluster.Spec.KubernetesVersion = "1.30.0"
	if lbCert || nlb {
		cluster.Spec.API.LoadBalancer = &kops.LoadBalancerAccessSpec{}
		if lbCert {
			cluster.Spec.API.LoadBalancer.SSLCertificate = "cert-arn"
		}
		if nlb {
			cluster.Spec.API.LoadBalancer.Class = kops.LoadBalancerClassNetwork
		}
	}
	return cluster
}

// create a fake keyset
func fakeKeyset() *fi.Keyset {
	cert, _ := pki.ParsePEMCertificate([]byte(certData))
	key, _ := pki.ParsePEMPrivateKey([]byte(privatekeyData))
	nextCert, _ := pki.ParsePEMCertificate([]byte(nextCertificate))
	keyset, _ := fi.NewKeyset(cert, key)
	_, _ = keyset.AddItem(nextCert, nil, false)
	return keyset
}

func TestBuildKubecfg(t *testing.T) {
	originalPKIDefaultPrivateKeySize := pki.DefaultPrivateKeySize
	pki.DefaultPrivateKeySize = 2048
	defer func() {
		pki.DefaultPrivateKeySize = originalPKIDefaultPrivateKeySize
	}()

	type args struct {
		CreateKubecfgOptions
		cluster     *kops.Cluster
		secretStore fi.SecretStore
		status      fakeStatusCloud
	}

	publicCluster := buildMinimalCluster("testcluster", "testcluster.test.com", false, false)
	emptyMasterPublicNameCluster := buildMinimalCluster("emptyMasterPublicNameCluster", "", false, false)
	gossipCluster := buildMinimalCluster("testgossipcluster.k8s.local", "", false, true)
	certCluster := buildMinimalCluster("testcluster", "testcluster.test.com", true, false)
	certNLBCluster := buildMinimalCluster("testcluster", "testcluster.test.com", true, true)
	certGossipNLBCluster := buildMinimalCluster("testgossipcluster.k8s.local", "", true, true)

	fakeStatus := fakeStatusCloud{
		GetApiIngressStatusFn: func(cluster *kops.Cluster) ([]fi.ApiIngressStatus, error) {
			return []fi.ApiIngressStatus{
				{
					Hostname: "elbHostName",
				},
			}, nil
		},
	}

	tests := []struct {
		name           string
		args           args
		want           *KubeconfigBuilder
		wantErr        bool
		wantClientCert bool
	}{
		{
			name: "Test Kube Config Data For Public DNS with admin",
			args: args{
				cluster: publicCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin: DefaultKubecfgAdminLifetime,
					User:  "",
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://testcluster.test.com",
				TLSServerName: "api.internal.testcluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testcluster",
			},
			wantClientCert: true,
		},
		{
			name: "Test Kube Config Data For Public DNS with admin and secondary NLB port",
			args: args{
				cluster: certNLBCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin: DefaultKubecfgAdminLifetime,
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://elbHostName:8443",
				TLSServerName: "api.internal.testcluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testcluster",
			},
			wantClientCert: true,
		},
		{
			name: "Test Kube Config Data For Public DNS with admin and CLB ACM Certificate",
			args: args{
				cluster: certCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin: DefaultKubecfgAdminLifetime,
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://elbHostName",
				TLSServerName: "api.internal.testcluster",
				CACerts:       nil,
				User:          "testcluster",
			},
			wantClientCert: true,
		},
		{
			name: "Test Kube Config Data For Public DNS without admin and with ACM certificate",
			args: args{
				cluster: certNLBCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin: 0,
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://testcluster.test.com",
				TLSServerName: "api.internal.testcluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testcluster",
			},
			wantClientCert: false,
		},
		{
			name: "Test Kube Config Data For Public DNS without admin",
			args: args{
				cluster: publicCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin: 0,
					User:  "myuser",
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://testcluster.test.com",
				TLSServerName: "api.internal.testcluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "myuser",
			},
			wantClientCert: false,
		},
		{
			name: "Test Kube Config Data For Public DNS with Empty Master Name",
			args: args{
				cluster: emptyMasterPublicNameCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin: 0,
					User:  "",
				},
			},
			want: &KubeconfigBuilder{
				Context:       "emptyMasterPublicNameCluster",
				Server:        "https://api.emptyMasterPublicNameCluster",
				TLSServerName: "api.internal.emptyMasterPublicNameCluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "emptyMasterPublicNameCluster",
			},
			wantClientCert: false,
		},
		{
			name: "Test Kube Config Data For Gossip cluster",
			args: args{
				cluster: gossipCluster,
				status:  fakeStatus,
			},
			want: &KubeconfigBuilder{
				Context:       "testgossipcluster.k8s.local",
				Server:        "https://elbHostName",
				TLSServerName: "api.internal.testgossipcluster.k8s.local",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testgossipcluster.k8s.local",
			},
			wantClientCert: false,
		},
		{
			name: "Public DNS with kops auth plugin",
			args: args{
				cluster: publicCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin:                       0,
					UseKopsAuthenticationPlugin: true,
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://testcluster.test.com",
				TLSServerName: "api.internal.testcluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testcluster",
				AuthenticationExec: []string{
					"kops",
					"helpers",
					"kubectl-auth",
					"--cluster=testcluster",
					"--state=memfs://example-state-store",
				},
			},
			wantClientCert: false,
		},
		{
			name: "Test Kube Config Data For internal DNS name with admin",
			args: args{
				cluster: publicCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin:    DefaultKubecfgAdminLifetime,
					Internal: true,
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://api.internal.testcluster",
				TLSServerName: "api.internal.testcluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testcluster",
			},
			wantClientCert: true,
		},
		{
			name: "Test Kube Config Data For Gossip cluster with admin and secondary NLB port",
			args: args{
				cluster: certGossipNLBCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin: DefaultKubecfgAdminLifetime,
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testgossipcluster.k8s.local",
				Server:        "https://elbHostName:8443",
				TLSServerName: "api.internal.testgossipcluster.k8s.local",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testgossipcluster.k8s.local",
			},
			wantClientCert: true,
		},
		{
			name: "Test Kube Config Data for Public cluster with admin and internal option",
			args: args{
				cluster: publicCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin:    DefaultKubecfgAdminLifetime,
					Internal: true,
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://api.internal.testcluster",
				TLSServerName: "api.internal.testcluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testcluster",
			},
			wantClientCert: true,
		},
		{
			name: "Test Kube Config Data for Public cluster without admin and with internal option",
			args: args{
				cluster: publicCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin:    0,
					Internal: true,
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://api.internal.testcluster",
				TLSServerName: "api.internal.testcluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testcluster",
			},
			wantClientCert: false,
		},
		{
			name: "Test Kube Config Data with APIEndpoint set",
			args: args{
				cluster: publicCluster,
				status:  fakeStatus,
				CreateKubecfgOptions: CreateKubecfgOptions{
					Admin:             DefaultKubecfgAdminLifetime,
					Internal:          true,
					OverrideAPIServer: "https://api.testcluster.example.com",
				},
			},
			want: &KubeconfigBuilder{
				Context:       "testcluster",
				Server:        "https://api.testcluster.example.com",
				TLSServerName: "api.internal.testcluster",
				CACerts:       []byte(nextCertificate + certData),
				User:          "testcluster",
			},
			wantClientCert: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()

			kopsStateStore := "memfs://example-state-store"

			keyStore := fakeKeyStore{
				FindKeysetFn: func(name string) (*fi.Keyset, error) {
					return fakeKeyset(),
						nil
				},
			}

			got, err := BuildKubecfg(ctx, tt.args.cluster, keyStore, tt.args.secretStore, tt.args.status, tt.args.CreateKubecfgOptions, kopsStateStore)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildKubecfg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantClientCert {
				if got.ClientCert == nil {
					t.Errorf("Expected ClientCert, got nil")
				}
				if got.ClientKey == nil {
					t.Errorf("Expected ClientKey, got nil")
				}
				tt.want.ClientCert = got.ClientCert
				tt.want.ClientKey = got.ClientKey
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("BuildKubecfg() diff (+got, -want): %s", diff)
			}
		})
	}
}
