#!/bin/bash

set -e

source dev-container-features-test-lib

check "kumo exists" which kumo
check "kumo-init exists" ls /usr/local/share/kumo-init.sh

reportResults
