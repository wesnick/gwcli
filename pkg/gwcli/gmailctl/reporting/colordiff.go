// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Vendored from github.com/mbrt/gmailctl

package reporting

import (
	"strings"

	"github.com/fatih/color"
)

func ColorizeDiff(diff string) string {
	coloredDiff := &strings.Builder{}
	lines := strings.Split(diff, "\n")
	colorBold := color.New(color.Bold)
	colorBold.EnableColor()
	colorCyan := color.New(color.FgCyan)
	colorCyan.EnableColor()
	colorRed := color.New(color.FgRed)
	colorRed.EnableColor()
	colorGreen := color.New(color.FgGreen)
	colorGreen.EnableColor()

	for i, line := range lines {
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			colorBold.Fprint(coloredDiff, line)
		} else if strings.HasPrefix(line, "@@") {
			colorCyan.Fprint(coloredDiff, line)
		} else if strings.HasPrefix(line, "-") {
			colorRed.Fprint(coloredDiff, line)
		} else if strings.HasPrefix(line, "+") {
			colorGreen.Fprint(coloredDiff, line)
		} else {
			coloredDiff.WriteString(line)
		}
		if i < len(lines)-1 { // Avoid adding an extra newline at the very end
			coloredDiff.WriteString("\n")
		}
	}
	return coloredDiff.String()
}
