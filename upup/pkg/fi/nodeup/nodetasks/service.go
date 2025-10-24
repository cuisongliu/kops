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

package nodetasks

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"k8s.io/klog/v2"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/nodeup/install"
	"k8s.io/kops/upup/pkg/fi/nodeup/local"
	"k8s.io/kops/util/pkg/distributions"
)

const (
	debianSystemdSystemPath = "/lib/systemd/system"

	// TODO: Generally only repo packages write to /usr/lib/systemd/system on _rhel_family
	// But we use it in two ways: we update the docker manifest, and we install our own
	// package (protokube, kubelet).  Maybe we should have the idea of a "system" package.
	centosSystemdSystemPath      = "/usr/lib/systemd/system"
	flatcarSystemdSystemPath     = "/etc/systemd/system"
	containerosSystemdSystemPath = "/etc/systemd/system"

	containerdService = "containerd.service"
	dockerService     = "docker.service"
	kubeletService    = "kubelet.service"
	protokubeService  = "protokube.service"
)

type Service struct {
	Name       string
	Definition *string `json:"definition,omitempty"`
	Running    *bool   `json:"running,omitempty"`

	// Enabled configures the service to start at boot (or not start at boot)
	Enabled *bool `json:"enabled,omitempty"`

	ManageState  *bool `json:"manageState,omitempty"`
	SmartRestart *bool `json:"smartRestart,omitempty"`
}

type InstallService struct {
	Service
}

var (
	_ fi.InstallHasDependencies = &InstallService{}
	_ fi.NodeupHasDependencies  = &Service{}
	_ fi.HasName                = &InstallService{}
	_ fi.HasName                = &Service{}
)

func (i *InstallService) GetDependencies(tasks map[string]fi.InstallTask) []fi.InstallTask {
	var deps []fi.InstallTask
	for _, v := range tasks {
		if _, ok := v.(*InstallService); !ok {
			deps = append(deps, v)
		}
	}
	return deps
}

func (s *Service) GetDependencies(tasks map[string]fi.NodeupTask) []fi.NodeupTask {
	var deps []fi.NodeupTask
	for _, v := range tasks {
		// We assume that services depend on everything except for
		// LoadImageTask or IssueCert. If there are any LoadImageTasks (e.g. we're
		// launching a custom Kubernetes build), they all depend on
		// the "docker.service" Service task.
		switch v := v.(type) {
		case *Package, *AptSource, *UserTask, *GroupTask, *Chattr, *BindMount, *Archive, *Prefix, *UpdateEtcHostsTask:
			deps = append(deps, v)
		case *Service, *PullImageTask, *IssueCert, *BootstrapClientTask, *KubeConfig:
			// ignore
		case *LoadImageTask:
			if s.Name == kubeletService {
				deps = append(deps, v)
			}
		case *File:
			if len(v.BeforeServices) > 0 {
				for _, b := range v.BeforeServices {
					if s.Name == b {
						deps = append(deps, v)
					}
				}
			} else {
				deps = append(deps, v)
			}
		default:
			klog.Warningf("Unhandled type %T in Service::GetDependencies: %v", v, v)
			deps = append(deps, v)
		}
	}
	return deps
}

func (s *Service) String() string {
	return fmt.Sprintf("Service: %s", s.Name)
}

func (i *InstallService) InitDefaults() *InstallService {
	i.Service.InitDefaults()
	return i
}
func (s *Service) InitDefaults() *Service {
	// Default some values to true: Running, SmartRestart, ManageState
	if s.Running == nil {
		s.Running = fi.PtrTo(true)
	}
	if s.SmartRestart == nil {
		s.SmartRestart = fi.PtrTo(true)
	}
	if s.ManageState == nil {
		s.ManageState = fi.PtrTo(true)
	}

	// Default Enabled to be the same as running
	if s.Enabled == nil {
		s.Enabled = s.Running
	}

	return s
}

func getSystemdStatus(name string) (map[string]string, error) {
	klog.V(2).Infof("querying state of service %q", name)
	cmd := exec.Command("systemctl", "show", "--all", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error doing systemd show %s: %v\nOutput: %s", name, err, output)
	}
	properties := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		tokens := strings.SplitN(line, "=", 2)
		if len(tokens) != 2 {
			klog.Warningf("Ignoring line in systemd show output: %q", line)
			continue
		}
		properties[tokens[0]] = tokens[1]
	}
	return properties, nil
}

func (_ *Service) systemdSystemPath() (string, error) {
	d, err := distributions.FindDistribution("/")
	if err != nil {
		return "", fmt.Errorf("unknown or unsupported distro: %v", err)
	}

	if d.IsDebianFamily() {
		return debianSystemdSystemPath, nil
	} else if d.IsRHELFamily() {
		return centosSystemdSystemPath, nil
	} else if d == distributions.DistributionFlatcar {
		return flatcarSystemdSystemPath, nil
	} else if d == distributions.DistributionContainerOS {
		return containerosSystemdSystemPath, nil
	} else {
		return "", fmt.Errorf("unsupported systemd system")
	}
}

func (e *InstallService) Find(_ *fi.InstallContext) (*InstallService, error) {
	actual, err := e.Service.Find(nil)
	if actual == nil || err != nil {
		return nil, err
	}
	return &InstallService{*actual}, nil
}
func (e *Service) Find(_ *fi.NodeupContext) (*Service, error) {
	systemdSystemPath, err := e.systemdSystemPath()
	if err != nil {
		return nil, err
	}

	servicePath := path.Join(systemdSystemPath, e.Name)

	d, err := os.ReadFile(servicePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("Error reading systemd file %q: %v", servicePath, err)
		}

		// Not found
		return &Service{
			Name:       e.Name,
			Definition: nil,
			Running:    fi.PtrTo(false),
		}, nil
	}

	actual := &Service{
		Name:       e.Name,
		Definition: fi.PtrTo(string(d)),

		// Avoid spurious changes
		ManageState:  e.ManageState,
		SmartRestart: e.SmartRestart,
	}

	properties, err := getSystemdStatus(e.Name)
	if err != nil {
		return nil, err
	}

	activeState := properties["ActiveState"]
	switch activeState {
	case "active":
		actual.Running = fi.PtrTo(true)

	case "failed", "inactive":
		actual.Running = fi.PtrTo(false)
	default:
		klog.Warningf("Unknown ActiveState=%q; will treat as not running", activeState)
		actual.Running = fi.PtrTo(false)
	}

	wantedBy := properties["WantedBy"]
	switch wantedBy {
	case "":
		actual.Enabled = fi.PtrTo(false)

	// TODO: Can probably do better here!
	case "multi-user.target", "graphical.target multi-user.target":
		actual.Enabled = fi.PtrTo(true)

	default:
		klog.Warningf("Unknown WantedBy=%q; will treat as not enabled", wantedBy)
		actual.Enabled = fi.PtrTo(false)
	}

	return actual, nil
}

// Parse the systemd unit file to extract obvious dependencies
func getSystemdDependencies(serviceName string, definition string) ([]string, error) {
	var dependencies []string
	for _, line := range strings.Split(definition, "\n") {
		line = strings.TrimSpace(line)
		tokens := strings.SplitN(line, "=", 2)
		if len(tokens) != 2 {
			continue
		}
		k := strings.TrimSpace(tokens[0])
		v := strings.TrimSpace(tokens[1])
		switch k {
		case "EnvironmentFile":
			dependencies = append(dependencies, v)
		case "ExecStart":
			// ExecStart=/usr/local/bin/kubelet "$DAEMON_ARGS"
			// We extract the first argument (only)
			tokens := strings.SplitN(v, " ", 2)
			dependencies = append(dependencies, tokens[0])
			klog.V(2).Infof("extracted dependency from %q: %q", line, tokens[0])
		}
	}
	return dependencies, nil
}

func (e *InstallService) Run(c *fi.InstallContext) error {
	return fi.InstallDefaultDeltaRunMethod(e, c)
}

func (e *Service) Run(c *fi.NodeupContext) error {
	return fi.NodeupDefaultDeltaRunMethod(e, c)
}

func (i *InstallService) CheckChanges(a, e, changes *InstallService) error {
	return nil
}

func (s *Service) CheckChanges(a, e, changes *Service) error {
	return nil
}

func (i *InstallService) RenderInstall(_ *install.InstallTarget, a, e, changes *InstallService) error {
	var actual *Service
	if a != nil {
		actual = &a.Service
	}

	return i.Service.RenderLocal(nil, actual, &e.Service, &changes.Service)
}
func (s *Service) RenderLocal(_ *local.LocalTarget, a, e, changes *Service) error {
	systemdSystemPath, err := e.systemdSystemPath()
	if err != nil {
		return err
	}

	serviceName := e.Name

	action := ""

	if changes.Running != nil && fi.ValueOf(e.ManageState) {
		if fi.ValueOf(e.Running) {
			action = "restart"
		} else {
			action = "stop"
		}
	}

	if changes.Definition != nil {
		servicePath := path.Join(systemdSystemPath, serviceName)
		err := fi.WriteFile(servicePath, fi.NewStringResource(*e.Definition), 0o644, 0o755, "", "")
		if err != nil {
			return fmt.Errorf("error writing systemd service file: %v", err)
		}

		klog.Infof("Reloading systemd configuration")
		cmd := exec.Command("systemctl", "daemon-reload")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error doing systemd daemon-reload: %v\nOutput: %s", err, output)
		}
	}

	// "SmartRestart" - look at the obvious dependencies in the systemd service, restart if start time older
	if fi.ValueOf(e.ManageState) && fi.ValueOf(e.SmartRestart) {
		definition := fi.ValueOf(e.Definition)
		if definition == "" && a != nil {
			definition = fi.ValueOf(a.Definition)
		}

		if action == "" && fi.ValueOf(e.Running) && definition != "" {
			dependencies, err := getSystemdDependencies(serviceName, definition)
			if err != nil {
				return err
			}

			// Include the systemd unit file itself
			dependencies = append(dependencies, path.Join(systemdSystemPath, serviceName))

			var newest time.Time
			for _, dependency := range dependencies {
				stat, err := os.Stat(dependency)
				if err != nil {
					klog.Infof("Ignoring error checking service dependency %q: %v", dependency, err)
					continue
				}
				modTime := stat.ModTime()
				if newest.IsZero() || newest.Before(modTime) {
					newest = modTime
				}
			}

			if !newest.IsZero() {
				properties, err := getSystemdStatus(e.Name)
				if err != nil {
					return err
				}

				startedAt := properties["ExecMainStartTimestamp"]
				if startedAt == "" {
					klog.Warningf("service was running, but did not have ExecMainStartTimestamp: %q", serviceName)
				} else {
					startedAtTime, err := time.Parse("Mon 2006-01-02 15:04:05 MST", startedAt)
					if err != nil {
						return fmt.Errorf("unable to parse service ExecMainStartTimestamp %q: %v", startedAt, err)
					}
					if startedAtTime.Before(newest) {
						klog.V(2).Infof("will restart service %q because dependency changed after service start", serviceName)
						action = "restart"
					} else {
						klog.V(2).Infof("will not restart service %q - started after dependencies", serviceName)
					}
				}
			}
		}
	}

	if action != "" && fi.ValueOf(e.ManageState) {
		args := []string{"systemctl", action, serviceName}
		// We use --no-block to avoid hanging if the service has issues stopping/starting
		args = append(args, "--no-block")
		cmd := exec.Command(args[0], args[1:]...)
		klog.Infof("Restarting service %q (running %q)", serviceName, strings.Join(cmd.Args, " "))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error doing systemd %s %s: %v\nOutput: %s", action, serviceName, err, output)
		}
	}

	if changes.Enabled != nil && fi.ValueOf(e.ManageState) {
		var args []string
		if fi.ValueOf(e.Enabled) {
			klog.Infof("Enabling service %q", serviceName)
			args = []string{"enable", serviceName}
		} else {
			klog.Infof("Disabling service %q", serviceName)
			args = []string{"disable", serviceName}
		}
		cmd := exec.Command("systemctl", args...)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error doing 'systemctl %v': %v\nOutput: %s", args, err, output)
		}
	}

	return nil
}

func (s *Service) GetName() *string {
	return &s.Name
}
