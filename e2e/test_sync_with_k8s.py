# Copyright (c) 2017 Intel Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import json
import subprocess
import sys
import time
import uuid

SUPPORTED_KINDS = ('secrets', 'deployments', 'pvc', 'svc', 'ing', 'sa')
TAP = './tap'


def jloads(data):
    try:
        return json.loads(data)
    except Exception as ex:
        print("Exception when loading json from: " + data)
        print(ex)
        raise ex

def tap_cli(*args):
    args = [TAP] + list(args)
    return subprocess.check_output(args, stderr=subprocess.STDOUT)

def extract_id(line, name):
    BODY = "BODY: "
    bi = line.find(BODY)
    j = line[bi + len(BODY):].strip()
    services = jloads(j)
    for service in services:
        if service['name'] == name:
            return service['id']
    raise Exception("No service was found")

def tap_create_instance():
    name = "mt" + str(uuid.uuid4())[:8]
    out = tap_cli("cs", "redis30", "free", name)
    assert 'OK' in out
    out = tap_cli("-v", "DEBUG", "svcs")
    for line in out.split('\n'):
        if name in line:
            return extract_id(line, name), name
    raise Exception("No service was created")

def stop_instance(name):
    out = tap_cli("ds", name)
    print(out)


def kubectl(*args):
    args = ['kubectl'] + list(args)
    return subprocess.check_output(args, stderr=sys.stderr)

def jkubectl(*args):
    out = kubectl(*(list(args) + ['-o', 'json']))
    return jloads(out)

def get_k8s_structs(kind):
    return jkubectl('get', kind, '-l', 'managed_by=TAP', )

def kube_del_all(idd):
    kinds = ",".join(SUPPORTED_KINDS)
    return kubectl("delete", kinds, "-l", 'instance_id=' + idd)

def get_id_kind_name(k8s_struct):
    metadata = k8s_struct['metadata']
    idd = metadata['labels']['instance_id']
    kind = k8s_struct['kind']
    name = metadata['name']
    return idd, kind, name

def get_structs():
    structs = []
    for kind in SUPPORTED_KINDS:
        kind_structs = get_k8s_structs(kind)
        structs.extend(kind_structs['items'])
    return structs

def group_structs(lstructs):
    structs = {}
    for struct in lstructs:
        idd, kind, name = get_id_kind_name(struct)
        val = structs.setdefault(idd, [])
        val.append((kind, name))
    return structs

def get_grouped_structs():
    lstructs = get_structs()
    return group_structs(lstructs)

def get_single_tap_instance():
    idd, name = tap_create_instance()
    print("probably created instance with id: " + idd + " and name: " + name)
    sys.stdout.write("Waiting for it being available")
    i = 0
    while i < 30:
        time.sleep(2)
        sys.stdout.write('.')
        sys.stdout.flush()
        structs = get_grouped_structs()
        if idd in structs:
            print("OK")
            wait_for_state(name, 'RUNNING')
            return idd, name, structs[idd]
        i += 1

    raise Exception("No just created TAP catalog instance on K8S")


def scale_instance(idd, items, replicas):
    for kind, idd in items:
        if kind == "Deployment":
            out = kubectl("scale", "--replicas", str(replicas), "deployment", idd)
            print(out)


def get_tap_state(name):
    lines = tap_cli("svcs").split('\n')
    for line in lines:
        if name in line:
            return line.strip('|').split("|")[3].strip()
    raise Exception("No instance with given name")


def wait_for_state(name, expected_state):
    print("Waiting for state: " + expected_state)
    old_state = None
    while True:
        time.sleep(1)
        state = get_tap_state(name)
        if state == old_state:
            sys.stdout.write('.')
        else:
            sys.stdout.write(" " + state)
        old_state = state
        sys.stdout.flush()
        if state == expected_state:
            print("")
            return


def run_test_on_running():
    print("Starting test on running")
    idd, name, items = get_single_tap_instance()
    print("Working with instance of ID: " + idd)
    print("It has: " + str(items))
    print("donwscalling")
    scale_instance(idd, items, 0)
    wait_for_state(name, 'FAILURE')

def run_test_on_stopped():
    print("Starting test on stopped")
    idd, name, items = get_single_tap_instance()
    print("Working with instance of ID: " + idd)
    print("It has: " + str(items))
    print("stopping")
    stop_instance(name)
    print("upscalling")
    scale_instance(idd, items, 1)
    wait_for_state(name, 'FAILURE')


def main(args):
    global TAP
    if args:
        TAP = args[1]
    run_test_on_running()
    run_test_on_stopped()


if __name__ == '__main__':
    main(sys.argv)
