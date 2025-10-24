/*
Copyright 2020 The Kubernetes Authors.

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

package bootstrap

import (
	"context"
	"errors"
	"net/http"

	"k8s.io/kops/pkg/nodeidentity/clusterapi"
)

var ErrAlreadyExists = errors.New("node already exists")

// Authenticator generates authentication credentials for requests.
type Authenticator interface {
	CreateToken(body []byte) (string, error)
}

// VerifyResult is the result of a successfully verified request.
type VerifyResult struct {
	// Nodename is the name that this node is authorized to use.
	NodeName string

	// InstanceGroupName is the name of the kops InstanceGroup this node is a member of.
	InstanceGroupName string

	// CAPIMachine is the Cluster API Machine object corresponding to this node, if available.
	CAPIMachine *clusterapi.Machine

	// CertificateNames is the alternate names the node is authorized to use for certificates.
	CertificateNames []string

	// ChallengeEndpoint is a valid endpoints to which we should issue a challenge request,
	// corresponding to the node the request identified as.
	// This should be sourced from e.g. the cloud, and acts as a cross-check
	// that this is the correct instance.
	ChallengeEndpoint string
}

// Verifier verifies authentication credentials for requests.
type Verifier interface {
	// VerifyToken performs full validation of the provided token, often making cloud API calls to verify the caller.
	// It should return either an error or a validated VerifyResult.
	// If the token looks like it is intended for a different verifier
	// (for example it has the wrong prefix), we should return ErrNotThisVerifier
	VerifyToken(ctx context.Context, rawRequest *http.Request, token string, body []byte) (*VerifyResult, error)
}

// ErrNotThisVerifier is returned when a verifier receives a token that is not intended for it.
var ErrNotThisVerifier = errors.New("token not valid for this verifier")
