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

package do

import (
	"context"
	"strconv"

	"github.com/digitalocean/godo"
	"golang.org/x/crypto/ssh"

	"github.com/coreos/mantle/platform"
)

type machine struct {
	cluster   *cluster
	droplet   *godo.Droplet
	journal   *platform.Journal
	publicIP  string
	privateIP string
}

func (dm *machine) ID() string {
	return strconv.Itoa(dm.droplet.ID)
}

func (dm *machine) IP() string {
	return dm.publicIP
}

func (dm *machine) PrivateIP() string {
	return dm.privateIP
}

func (dm *machine) SSHClient() (*ssh.Client, error) {
	return dm.cluster.SSHClient(dm.IP())
}

func (dm *machine) PasswordSSHClient(user string, password string) (*ssh.Client, error) {
	return dm.cluster.PasswordSSHClient(dm.IP(), user, password)
}

func (dm *machine) SSH(cmd string) ([]byte, []byte, error) {
	return dm.cluster.SSH(dm, cmd)
}

func (dm *machine) Reboot() error {
	return platform.RebootMachine(dm, dm.journal, dm.cluster.RuntimeConf())
}

func (dm *machine) Destroy() error {
	if err := dm.cluster.api.DeleteDroplet(context.TODO(), dm.droplet.ID); err != nil {
		return err
	}

	if dm.journal != nil {
		if err := dm.journal.Destroy(); err != nil {
			return err
		}
	}

	dm.cluster.DelMach(dm)
	return nil
}

func (dm *machine) ConsoleOutput() string {
	// DigitalOcean provides no API for this
	return ""
}