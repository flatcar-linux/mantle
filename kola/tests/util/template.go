// Copyright 2019 Kinvolk GmbH
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

package util

import (
	"bytes"
	"text/template"
)

func ExecTemplate(tmplStr string, tmplData interface{}) (string, error) {
	var out bytes.Buffer

	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		return out.String(), err
	}

	if err := tmpl.Execute(&out, tmplData); err != nil {
		return out.String(), err
	}
	return out.String(), nil
}
