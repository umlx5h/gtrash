#!/bin/bash

set -eu
#
# mkdir -p /tmp/external /tmp/external_alt
#
# # use tmpfs for test
# mount -t tmpfs external /tmp/external
# mount -t tmpfs external_alt /tmp/external_alt
#
# Create .Trash folder beforehand
mkdir -p "/external/.Trash"
mkdir -p "/external_alt/.Trash"

chmod a+rw /external/.Trash /external_alt/.Trash

# sticky bit set only in /external
chmod +t /external/.Trash
