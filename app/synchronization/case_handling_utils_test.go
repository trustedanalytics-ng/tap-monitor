/**
 * Copyright (c) 2017 Intel Corporation
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

package synchronization

import (
	"testing"
	"time"
)

const (
	someId               = "someId"
	baseTestRetryTimeout = 100 * time.Millisecond
	maxTestValidityTime  = 8 * baseTestRetryTimeout
)

func TestUtilsFullFlow(t *testing.T) {

	cc := newCaseCache(1, baseTestRetryTimeout, maxTestValidityTime)

	if !cc.shouldHandle(someId, nil, nil) {
		t.Error("New cases should be marked for handling")
	}
	if cc.shouldHandle(someId, nil, nil) {
		t.Error("Just handled case shouldn't be marked for handling so soon")
	}

	time.Sleep(baseTestRetryTimeout / 2)
	if cc.shouldHandle(someId, nil, nil) {
		t.Error("Too soon case was marked for handling")
	}

	time.Sleep(baseTestRetryTimeout/2 + baseTestRetryTimeout/5)
	if !cc.shouldHandle(someId, nil, nil) {
		t.Error("Case wasn't marked for handling even after timeout")
	}
	if cc.shouldHandle(someId, nil, nil) {
		t.Error("Just handled case shouldn't be marked for handling so soon")
	}

	time.Sleep(baseTestRetryTimeout * 5)
	if cc.shouldHandle(someId, nil, nil) {
		t.Error("Case mared for handling after retry limit")
	}

	time.Sleep(maxTestValidityTime + baseTestRetryTimeout)
	if !cc.shouldHandle(someId, nil, nil) {
		t.Error("Case wasn't marked for handling after being droped from cache")
	}
}

func TestUtilsDifferentiationOfCases(t *testing.T) {
	cc := newCaseCache(1, baseTestRetryTimeout, maxTestValidityTime)
	if !cc.shouldHandle(someId, nil, nil) {
		t.Error("New case wasn't marked for handling")
	}
	if !cc.shouldHandle(someId, nil, &K8SObject{}) {
		t.Error("New case wasn't marked for handling - probably detected it as previous one?")
	}
	time.Sleep(baseTestRetryTimeout + baseTestRetryTimeout/2)
	if !cc.shouldHandle(someId, nil, nil) {
		t.Error("Case wasn't marked for handling")
	}
	if !cc.shouldHandle(someId, nil, &K8SObject{}) {
		t.Error("Case wasn't marked for handling - probably detected it as previous one?")
	}
}
