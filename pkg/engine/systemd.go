package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/opencontainers/runtime-spec/specs-go"

	"k8s.io/klog/v2"
)

func systemdPodman(ctx context.Context, mo *FileMountOptions, dest string) error {
	sd := mo.Target.Methods.Systemd
	if sd.RestartAlways {
		return enableSystemdService(mo, "restart", dest, filepath.Base(mo.Path))
	}
	if sd.Enable {
		return enableSystemdService(mo, "enable", dest, filepath.Base(mo.Path))
	}
	return nil
}

func enableSystemdService(mo *FileMountOptions, action, dest, service string) error {
	klog.Infof("Target: %s, running systemctl %s %s", mo.Target.Name, action, service)
	sd := mo.Target.Methods.Systemd
	if err := FetchImage(mo.Conn, systemdImage, true); err != nil {
		return err
	}
	os.Setenv("ROOT", "true")
	if !sd.Root {
		//os.Setenv("ROOT", "false")
		klog.Info("At this time, harpoon non-root user cannot enable systemd service on the host")
		klog.Infof("To enable this non-root service, run 'systemctl --user enable %s' on host machine", service)
		klog.Info("To enable service as root, run with Systemd.Root = true")
		return nil
	}

	s := specgen.NewSpecGenerator(systemdImage, false)
	runMount := "/run/systemd"
	if !sd.Root {
		runMount = "/run/user/1000/systemd"
		s.User = "1000"
	}
	s.Name = "systemd-" + action + "-" + service + "-" + mo.Target.Name
	s.Privileged = true
	s.PidNS = specgen.Namespace{
		NSMode: "host",
		Value:  "",
	}

	envMap := make(map[string]string)
	envMap["ROOT"] = strconv.FormatBool(sd.Root)
	envMap["SERVICE"] = service
	envMap["ACTION"] = action
	s.Env = envMap
	s.Mounts = []specs.Mount{{Source: dest, Destination: dest, Type: "bind", Options: []string{"rw"}}, {Source: runMount, Destination: runMount, Type: "bind", Options: []string{"rw"}}}
	createResponse, err := createAndStartContainer(mo.Conn, s)
	if err != nil {
		return err
	}

	err = waitAndRemoveContainer(mo.Conn, createResponse.ID)
	if err != nil {
		return err
	}
	klog.Infof("Target: %s, systemd %s %s complete", mo.Target.Name, action, service)
	return nil
}
