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

	catalogModels "github.com/trustedanalytics-ng/tap-catalog/models"
	templateModels "github.com/trustedanalytics-ng/tap-template-repository/model"
)

func TestConvertDependenciesToBindings(t *testing.T) {
	Convey("For set of test cases ConvertDependenciesToBindings should return proper responses", t, func() {
		testCases := []struct {
			input  []catalogModels.InstanceDependency
			output []catalogModels.InstanceBindings
		}{
			{[]catalogModels.InstanceDependency{{Id: "1"}, {Id: "123456987"}, {Id: "3"}},
				[]catalogModels.InstanceBindings{{Id: "1", Data: nil}, {Id: "123456987", Data: nil}, {Id: "3", Data: nil}}},
			{[]catalogModels.InstanceDependency{{Id: "2"}}, []catalogModels.InstanceBindings{{Id: "2", Data: nil}}},
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

func TestAdjustTemplateIdAndImage(t *testing.T) {
	Convey("Test adjustTemplateIdAndImage", t, func() {
		templateId := "sameple-tempalte-id"
		image := "sameple-image"
		imageKey := "image"

		rawTemplate := templateModels.RawTemplate{
			imageKey: templateModels.GetPlaceholderWithDollarPrefix(templateModels.PlaceholderImage),
		}

		Convey("For proper input should return proper response", func() {
			adjustedRawTemplate, err := adjustTemplateIdAndImage(templateId, image, rawTemplate)
			So(err, ShouldBeNil)
			So(adjustedRawTemplate[templateModels.RAW_TEMPLATE_ID_FIELD], ShouldEqual, templateId)
			So(adjustedRawTemplate[imageKey], ShouldEqual, image)
		})
	})
}
