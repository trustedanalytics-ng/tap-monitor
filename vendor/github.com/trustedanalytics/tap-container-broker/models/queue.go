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

package models

const (
	CONTAINER_BROKER_QUEUE_NAME         = "tap-container-broker"
	CONTAINER_BROKER_CREATE_ROUTING_KEY = "create"
	CONTAINER_BROKER_DELETE_ROUTING_KEY = "delete"
	CONTAINER_BROKER_BIND_ROUTING_KEY   = "bind"
	CONTAINER_BROKER_UNBIND_ROUTING_KEY = "unbind"
)