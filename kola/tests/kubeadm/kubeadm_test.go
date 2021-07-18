// Copyright 2021 Kinvolk GmbH
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
// local package import and it's behavior as a package

package kubeadm

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderTemplate(t *testing.T) {
	t.Run("SuccessWithBase64", func(t *testing.T) {
		res, err := render(
			"Hello, {{ .World }}",
			map[string]interface{}{
				"World": "world !",
			},
			true,
		)
		require.Nil(t, err)
		assert.Equal(t, "SGVsbG8sIHdvcmxkICE=", res.String())

	})
	t.Run("Success", func(t *testing.T) {
		res, err := render(
			"Hello, {{ .World }}",
			map[string]interface{}{
				"World": "world !",
			},
			false,
		)
		require.Nil(t, err)
		assert.Equal(t, "Hello, world !", res.String())

	})
	t.Run("SuccessMasterScript", func(t *testing.T) {
		for _, CNI := range CNIs {
			res, err := render(
				masterScript,
				map[string]interface{}{
					"CNI":            CNI,
					"Endpoints":      []string{"http://1.2.3.4:2379"},
					"Params":         "amd64",
					"CNIVersion":     "v0.8.7",
					"CRIctlVersion":  "v1.17.0",
					"ReleaseVersion": "v0.4.0",
					"Release":        "v1.21.0",
					"DownloadDir":    "/opt/bin",
					"PodSubnet":      "192.168.0.0/17",
					"KubeadmSum":     "0673408403a3474c868ae86109f11f9114bca7ddce204be0d169316fb3ce0edefa4b2a472ba9b8308e423e6b927d4098ac36296405570f444f39551fb1c4bbb4",
					"KubeletSum":     "530689c0cc32ef1830f7ae26ac10995f815043d48a905141e23a34a5e61522c4ee2ff46953648c47c5592d7c2ffa40ce90469a697f36f68475b8da5abd73f9f5",
					"CRIctlSum":      "e258f4607a89b8d44c700036e636dd42cc3e2ed27a3bb13beef736f80f64f10b7974c01259a66131d3f7b44ed0c61b1ca0ea91597c416a9c095c432de5112d44",
					"CNISum":         "8f2cbee3b5f94d59f919054dccfe99a8e3db5473b553d91da8af4763e811138533e05df4dbeab16b3f774852b4184a7994968f5e036a3f531ad1ac4620d10ede",
					"KubectlSum":     "9557d298146ef62ffbcf05b3591bf1ce74f345628370447a4f614b5f64e367b5bfa8e397cc4755da9ea38f1ba04c95c65c313e735550ffc3b03c197e936c3e11",
				},
				false,
			)
			require.Nil(t, err)
			script, err := ioutil.ReadFile(fmt.Sprintf("testdata/master-%s-script.sh", CNI))
			require.Nil(t, err)
			assert.Equal(t, string(script), res.String())
		}
	})
}
