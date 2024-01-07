#!/bin/sh

# This is an example of how to use Logsnag (https://logsnag.com/) to send a
# notifications.

reqURL=$2

logsnagToken="<your-logsnag-token>"
logsnagProject="<your-project>"
logsnagChannel="<your-channel>"

wget --header 'Content-Type: application/json' \
     --header "Authorization: Bearer ${logsnagToken}" \
     --post-data "{\"project\":\"${logsnagProject}\",\"channel\":\"${logsnagChannel}\",\"event\":\"Confirm requisition URL\",\"description\":\"${reqURL}\",\"icon\":\"❤️\",\"notify\":true}" \
     'https://api.logsnag.com/v1/log'
