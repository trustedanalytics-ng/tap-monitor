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
	"sync"

	commonLogger "github.com/trustedanalytics/tap-go-common/logger"
	"github.com/trustedanalytics/tap-go-common/util"
	"github.com/trustedanalytics/tap-monitor/app"
)

var logger, _ = commonLogger.InitLogger("main")
var waitGroup = &sync.WaitGroup{}

func main() {
	go util.TerminationObserver(waitGroup, "Monitor")

	if err := app.InitConnections(); err != nil {
		logger.Fatal("ERROR initConnections: ", err.Error())
	}

	app.StartMonitor(waitGroup)
}
