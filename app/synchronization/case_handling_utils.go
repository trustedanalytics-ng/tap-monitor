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

// NOTE: currently not used but probably later on it will be
// NOTE: can be generalized for other cases (should be easy - change caseToFix to interface)

import (
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/trustedanalytics-ng/tap-catalog/models"
)

type caseToFix struct {
	key, id   string
	nextCheck time.Time
	tryCount  int
}

func (ctf *caseToFix) nextTry(retryLimit int, baseRetryTimeout time.Duration) bool {
	if ctf.tryCount > retryLimit {
		return false
	}
	if time.Now().Before(ctf.nextCheck) {
		return false
	}
	extraTime := float64(ctf.tryCount*ctf.tryCount) * baseRetryTimeout.Seconds()
	ctf.nextCheck = ctf.nextCheck.Add(time.Duration(extraTime) * time.Second)
	ctf.tryCount += 1
	return true
}

func computeKey(id string, ci *models.Instance, ko *K8SObject) string {
	ciState := "none"
	if ci != nil {
		ciState = string(ci.State)
	}
	koState := "none"
	if ko != nil {
		koState = "present"
	}
	return id + "_" + ciState + "_" + koState
}

type caseCache struct {
	cache *cache.Cache

	retryLimit       int
	baseRetryTimeout time.Duration
}

func newCaseCache(retryLimit int, baseRetryTimeout time.Duration, caseExpirationTime time.Duration) *caseCache {
	return &caseCache{
		cache:            cache.New(caseExpirationTime, 30*time.Second),
		retryLimit:       retryLimit,
		baseRetryTimeout: baseRetryTimeout,
	}
}

func (cc *caseCache) newCase(id string, ci *models.Instance, ko *K8SObject) *caseToFix {
	key := computeKey(id, ci, ko)
	return &caseToFix{
		key:       key,
		id:        id,
		nextCheck: time.Now().Add(cc.baseRetryTimeout),
		tryCount:  1,
	}
}

func (cc *caseCache) shouldHandle(id string, ci *models.Instance, ko *K8SObject) bool {
	key := computeKey(id, ci, ko)

	var casee *caseToFix
	caseRaw, found := cc.cache.Get(key)
	if !found {
		casee = cc.newCase(id, ci, ko)
	} else {
		casee = caseRaw.(*caseToFix)
	}
	cc.cache.Add(key, casee, cache.DefaultExpiration)
	return !found || casee.nextTry(cc.retryLimit, cc.baseRetryTimeout)
}
