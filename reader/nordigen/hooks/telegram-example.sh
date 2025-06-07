#!/bin/sh

 # This script will send a message to a Telegram channel with the reauthentication link
 #
 # Configuration is done in the /data/telegram_config.env
 # Environment variables expected
 #
 # telegram_bot_token
 # telegram_chat_id
 #

 # Check if the required parameters are provided
 if [ "$#" -ne 2 ]; then
   echo "Usage: $0 <status> <link>"
   exit 1
 fi

 # Load Telegram credentials from the file if available
 if [ -f "/data/telegram_config.env" ]; then
   . "/data/telegram_config.env"
 fi

 # Set the parameters
 status="$1"
 link="$2"

 # Send the message to the Telegram channel
 curl -s -X POST https://api.telegram.org/bot${telegram_bot_token}/sendMessage -d chat_id=${telegram_chat_id} -d text="YNABber needs reauthentication\n\nStatus: $status\nLink: $link"

