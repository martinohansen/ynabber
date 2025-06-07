#!/bin/sh

# A hook to always fail when new requisition is asked.
# This maybe useful on headless environment where an outside requistion renewal is required
echo $(basename $0 .sh) called with parameters $*
exit 1
