#! /bin/sh

echo "Hi from hook 👋
status: $1
link: $2
at: $(date)" | tee /tmp/nordigen.log

# If you want to only act on certain events, you key off the first argument like
# this:
if [ "$1" == "CR" ]; then
    echo "Requsition created!" | tee -a /tmp/nordigen.log
fi
