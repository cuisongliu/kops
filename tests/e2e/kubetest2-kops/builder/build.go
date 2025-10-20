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

package builder

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"k8s.io/klog/v2"
	krel "k8s.io/release/pkg/build"
	"sigs.k8s.io/kubetest2/pkg/exec"
)

type BuildOptions struct {
	KopsRoot        string `flag:"-"`
	KubeRoot        string `flag:"-"`
	StageLocation   string `flag:"-"`
	TargetBuildArch string `flag:"~target-build-arch" desc:"CPU architecture to test against"`
	BuildKubernetes bool   `flag:"~build-kubernetes" desc:"Set this flag to true to build kubernetes"`
}

// BuildResults describes the outcome of a successful build.
type BuildResults struct {
	KopsBaseURL       string
	KubernetesBaseURL string
}

// Build will build the kops artifacts and publish them to the stage location
func (b *BuildOptions) Build() (*BuildResults, error) {
	// We expect to upload to a subdirectory with a version identifier
	gcsLocation := b.StageLocation
	if !strings.HasSuffix(gcsLocation, "/") {
		gcsLocation += "/"
	}

	results := &BuildResults{}

	if b.BuildKubernetes {
		// Build k/k
		re := regexp.MustCompile(`^gs://([\w-]+)/(devel|ci)(/.*)?`)

		// StageLocation is often just the root of the bucket. the leading slash has been stripped
		kubeStageLocation := b.StageLocation + "/ci/kubernetes"
		mat := re.FindStringSubmatch(kubeStageLocation)
		if mat == nil || len(mat) < 4 {
			return nil, fmt.Errorf("invalid stage location: %v. Use gs://<bucket>/<ci|devel>/<optional-suffix>", kubeStageLocation)
		}

		if err := krel.NewInstance(&krel.Options{
			Bucket:             mat[1],
			GCSRoot:            "kubernetes",
			AllowDup:           true,
			CI:                 true,
			NoUpdateLatest:     false,
			RepoRoot:           b.KubeRoot,
			KubeBuildPlatforms: b.TargetBuildArch,
		}).Build(); err != nil {
			return nil, fmt.Errorf("stage via krel push: %w", err)
		}
		kubeBaseURL := "https://storage.googleapis.com/" + mat[1] + "/kubernetes/latest.txt"

		results = &BuildResults{
			KubernetesBaseURL: kubeBaseURL,
		}
		return results, nil
	}

	args := []string{"make", "gcs-publish-ci"}
	cmd := exec.Command(args[0], args[1:]...)
	env := []string{
		fmt.Sprintf("HOME=%v", os.Getenv("HOME")),
		fmt.Sprintf("PATH=%v", os.Getenv("PATH")),
		fmt.Sprintf("GCS_LOCATION=%v", gcsLocation),
		fmt.Sprintf("GOPATH=%v", os.Getenv("GOPATH")),
	}
	// We need to "forward" some variables, in particular the variables like "CI" that change the version we use
	for _, k := range []string{"CI"} {
		v := os.Getenv(k)
		if v != "" {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	cmd.SetEnv(env...)
	cmd.SetDir(b.KopsRoot)
	exec.InheritOutput(cmd)
	klog.Infof("Executing %q (in %v) with env %v", strings.Join(args, " "), b.KopsRoot, env)
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Get the full path (including subdirectory) that we uploaded to
	// It is written by gcs-publish-ci to .build/upload/latest-ci.txt
	latestPath := filepath.Join(b.KopsRoot, ".build", "upload", "latest-ci.txt")
	kopsBaseURL, err := os.ReadFile(latestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", latestPath, err)
	}
	u, err := url.Parse(strings.TrimSpace(string(kopsBaseURL)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse url %q from file %q: %w", string(kopsBaseURL), latestPath, err)
	}
	u.Path = strings.ReplaceAll(u.Path, "//", "/")
	results = &BuildResults{
		KopsBaseURL: u.String(),
	}

	// Write some meta files so that other tooling can know e.g. KOPS_BASE_URL
	metaDir := filepath.Join(b.KopsRoot, ".kubetest2")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to Mkdir(%q): %w", metaDir, err)
	}
	p := filepath.Join(metaDir, "kops-base-url")
	if err := os.WriteFile(p, []byte(results.KopsBaseURL), 0o644); err != nil {
		return nil, fmt.Errorf("failed to WriteFile(%q): %w", p, err)
	}
	klog.Infof("wrote file %q with %q", p, results.KopsBaseURL)

	return results, nil
}
