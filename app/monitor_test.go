/**
 * Copyright (c) 2016 Intel Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package app

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	catalogModels "github.com/trustedanalytics/tap-catalog/models"
)

func TestConvertDependenciesToBindings(t *testing.T) {
	Convey("For set of test cases ConvertDependenciesToBindings should return proper responses", t, func() {
		testCases := []struct {
			input  []catalogModels.InstanceDependency
			output []catalogModels.InstanceBindings
		}{
			{[]catalogModels.InstanceDependency{{"1"}, {"123456987"}, {"3"}}, []catalogModels.InstanceBindings{{"1", nil}, {"123456987", nil}, {"3", nil}}},
			{[]catalogModels.InstanceDependency{{"2"}}, []catalogModels.InstanceBindings{{"2", nil}}},
			{[]catalogModels.InstanceDependency{}, []catalogModels.InstanceBindings{}},
		}

		for _, tc := range testCases {
			Convey(fmt.Sprintf("For input dependencies %v it should return proper InstanceBindings", tc.input), func() {
				output := convertDependenciesToBindings(tc.input)
				So(output, ShouldResemble, tc.output)
			})
		}
	})
}
