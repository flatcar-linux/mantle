// Copyright 2017 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package misc

import (
	"encoding/json"

	"github.com/coreos/mantle/kola/cluster"
	"github.com/coreos/mantle/kola/register"
	"github.com/coreos/mantle/kola/tests/util"
	"github.com/coreos/mantle/platform"
	"github.com/coreos/mantle/platform/conf"
	"github.com/coreos/mantle/platform/machine/qemu"
	"github.com/coreos/mantle/platform/machine/unprivqemu"
)

const (
	CLConfigRootRaid = `storage:
  disks:
    - device: "/dev/disk/by-id/virtio-secondary"
      wipe_table: true
      partitions:
       - label: root1
         number: 1
         size: 256MiB
         type_guid: be9067b9-ea49-4f15-b4f6-f36f8c9e1818
       - label: root2
         number: 2
         size: 256MiB
         type_guid: be9067b9-ea49-4f15-b4f6-f36f8c9e1818
  raid:
    - name: "rootarray"
      level: "{{ .RaidLevel }}"
      devices:
        - "/dev/disk/by-partlabel/root1"
        - "/dev/disk/by-partlabel/root2"
  filesystems:
    - name: "ROOT"
      mount:
        device: "/dev/md/rootarray"
        format: "ext4"
        label: ROOT
    - name: "NOT_ROOT"
      mount:
        device: "/dev/disk/by-id/virtio-primary-disk-part9"
        format: "ext4"
        label: wasteland
        wipe_filesystem: true
`

	CLConfigDataRaid = `storage:
  raid:
    - name: "DATA"
      level: "{{ .RaidLevel }}"
      devices:
        - "/dev/disk/by-partlabel/OEM-CONFIG"
        - "/dev/disk/by-partlabel/USR-B"
  filesystems:
    - name: "DATA"
      mount:
        device: "/dev/md/DATA"
        format: "ext4"
        label: DATA
systemd:
  units:
    - name: "var-lib-data.mount"
      enable: true
      contents: |
          [Mount]
          What=/dev/md/DATA
          Where=/var/lib/data
          Type=ext4
          
          [Install]
          WantedBy=local-fs.target
`
)

var (
	raid0RootUserData *conf.UserData
	raid1RootUserData *conf.UserData

	raidTypes = []string{
		"raid0",
		"raid1",
	}
)

type raidConfig struct {
	RaidLevel string
}

func init() {
	// root with raid0
	tmplRootRaid0, _ := util.ExecTemplate(CLConfigRootRaid, raidConfig{
		RaidLevel: "raid0",
	})
	raid0RootUserData = conf.ContainerLinuxConfig(tmplRootRaid0)

	register.Register(&register.Test{
		Run:         RootOnRaid0,
		ClusterSize: 0,
		Platforms:   []string{"qemu"},
		Name:        "cl.disk.raid0.root",
		Distros:     []string{"cl"},
	})

	// root with raid1
	tmplRootRaid1, _ := util.ExecTemplate(CLConfigRootRaid, raidConfig{
		RaidLevel: "raid1",
	})
	raid1RootUserData = conf.ContainerLinuxConfig(tmplRootRaid1)

	register.Register(&register.Test{
		// This test needs additional disks which is only supported on qemu since Ignition
		// does not support deleting partitions without wiping the partition table and the
		// disk doesn't have room for new partitions.
		// TODO(ajeddeloh): change this to delete partition 9 and replace it with 9 and 10
		// once Ignition supports it.
		Run:         RootOnRaid1,
		ClusterSize: 0,
		Platforms:   []string{"qemu"},
		Name:        "cl.disk.raid1.root",
		Distros:     []string{"cl"},
	})

	// data with raid0
	tmplDataRaid0, _ := util.ExecTemplate(CLConfigDataRaid, raidConfig{
		RaidLevel: "raid0",
	})

	register.Register(&register.Test{
		Run:         DataOnRaid,
		ClusterSize: 1,
		Name:        "cl.disk.raid0.data",
		UserData:    conf.ContainerLinuxConfig(tmplDataRaid0),
		Distros:     []string{"cl"},
	})

	// data with raid1
	tmplDataRaid1, _ := util.ExecTemplate(CLConfigDataRaid, raidConfig{
		RaidLevel: "raid1",
	})

	register.Register(&register.Test{
		Run:         DataOnRaid,
		ClusterSize: 1,
		Name:        "cl.disk.raid1.data",
		UserData:    conf.ContainerLinuxConfig(tmplDataRaid1),
		Distros:     []string{"cl"},
	})
}

func RootOnRaid0(c cluster.TestCluster) {
	var m platform.Machine
	var err error
	options := platform.MachineOptions{
		AdditionalDisks: []platform.Disk{
			{Size: "520M", DeviceOpts: []string{"serial=secondary"}},
		},
	}
	switch pc := c.Cluster.(type) {
	// These cases have to be separated because when put together to the same case statement
	// the golang compiler no longer checks that the individual types in the case have the
	// NewMachineWithOptions function, but rather whether platform.Cluster does which fails
	case *qemu.Cluster:
		m, err = pc.NewMachineWithOptions(raid0RootUserData, options)
	case *unprivqemu.Cluster:
		m, err = pc.NewMachineWithOptions(raid0RootUserData, options)
	default:
		c.Fatal("unknown cluster type")
	}
	if err != nil {
		c.Fatal(err)
	}

	checkIfMountpointIsRaid(c, m, "/")

	// reboot it to make sure it comes up again
	err = m.Reboot()
	if err != nil {
		c.Fatalf("could not reboot machine: %v", err)
	}

	checkIfMountpointIsRaid(c, m, "/")
}

func RootOnRaid1(c cluster.TestCluster) {
	var m platform.Machine
	var err error
	options := platform.MachineOptions{
		AdditionalDisks: []platform.Disk{
			{Size: "520M", DeviceOpts: []string{"serial=secondary"}},
		},
	}
	switch pc := c.Cluster.(type) {
	// These cases have to be separated because when put together to the same case statement
	// the golang compiler no longer checks that the individual types in the case have the
	// NewMachineWithOptions function, but rather whether platform.Cluster does which fails
	case *qemu.Cluster:
		m, err = pc.NewMachineWithOptions(raid1RootUserData, options)
	case *unprivqemu.Cluster:
		m, err = pc.NewMachineWithOptions(raid1RootUserData, options)
	default:
		c.Fatal("unknown cluster type")
	}
	if err != nil {
		c.Fatal(err)
	}

	checkIfMountpointIsRaid(c, m, "/")

	// reboot it to make sure it comes up again
	err = m.Reboot()
	if err != nil {
		c.Fatalf("could not reboot machine: %v", err)
	}

	checkIfMountpointIsRaid(c, m, "/")
}

func DataOnRaid(c cluster.TestCluster) {
	m := c.Machines()[0]

	checkIfMountpointIsRaid(c, m, "/var/lib/data")

	// reboot it to make sure it comes up again
	err := m.Reboot()
	if err != nil {
		c.Fatalf("could not reboot machine: %v", err)
	}

	checkIfMountpointIsRaid(c, m, "/var/lib/data")
}

type lsblkOutput struct {
	Blockdevices []blockdevice `json:"blockdevices"`
}

type blockdevice struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Mountpoint *string       `json:"mountpoint"`
	Children   []blockdevice `json:"children"`
}

// checkIfMountpointIsRaid will check if a given machine has a device of type
// raid1 mounted at the given mountpoint. If it does not, the test is failed.
func checkIfMountpointIsRaid(c cluster.TestCluster, m platform.Machine, mountpoint string) {
	output := c.MustSSH(m, "lsblk --json")

	l := lsblkOutput{}
	err := json.Unmarshal(output, &l)
	if err != nil {
		c.Fatalf("couldn't unmarshal lsblk output: %v", err)
	}

	foundRoot := checkIfMountpointIsRaidWalker(c, l.Blockdevices, mountpoint)
	if !foundRoot {
		c.Fatalf("didn't find root mountpoint in lsblk output")
	}
}

// checkIfMountpointIsRaidWalker will iterate over bs and recurse into its
// children, looking for a device mounted at / with type raid1. true is returned
// if such a device is found. The test is failed if a device of a different type
// is found to be mounted at /.
func checkIfMountpointIsRaidWalker(c cluster.TestCluster, bs []blockdevice, mountpoint string) bool {
	for _, b := range bs {
		if b.Mountpoint != nil && *b.Mountpoint == mountpoint {
			if !isValidRaidType(b.Type) {
				c.Fatalf("device %q is mounted at %q with type %q (was expecting raid1)", b.Name, mountpoint, b.Type)
			}
			return true
		}
		foundRoot := checkIfMountpointIsRaidWalker(c, b.Children, mountpoint)
		if foundRoot {
			return true
		}
	}
	return false
}

// isValidRaidType checks if the given type string is one of the possible
// RAID types supported by the testsuite. For example, raid0 or raid1.
func isValidRaidType(rType string) bool {
	for _, t := range raidTypes {
		if t == rType {
			return true
		}
	}
	return false
}
