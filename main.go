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

package main

import (
	"github.com/trustedanalytics/tapng-monitor/app"
	"github.com/trustedanalytics/tapng-go-common/logger"
)

var logger = logger_wrapper.InitLogger("main")


func main() {

	err := app.InitConnections()
	if err != nil {
		logger.Fatal("ERROR initConnections: ", err.Error())
	}

	err = app.InitQueue()
	if err != nil {
		logger.Fatal("ERROR initQueue: ", err.Error())
	}

	err = app.StartMonitor()
	if err != nil {
		logger.Fatal("ERROR StartMonitor: ", err.Error())
	}
}