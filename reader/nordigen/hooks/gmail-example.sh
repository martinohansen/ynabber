#!/bin/sh

# This script will send and e-mail with reauthentication link using gmail
#
# Configuration is done in the /data/gmail_config.env
# Environment variables expected
#
# sender_email
# app_specific_password
# recipient
#

# Check if the required parameters are provided
if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <status> <link>"
  exit 1
fi

# Load Gmail credentials from the file
if [ -f "/data/gmail_config.env" ]; then
  . "/data/gmail_config.env"
else
  echo "Error: gmail_config.env file not found."
  exit 1
fi

# Set the parameters
status="$1"
link="$2"

# Check if msmtp is installed, and install it if not
if ! command -v msmtp > /dev/null; then
  echo "Installing msmtp..."
  apk update
  apk add msmtp
fi

# Create a temporary file for the email message
email_file=$(mktemp)
echo -e "Subject: YNABber needs reauthentication\n\nStatus: $status\nLink: $link" > "$email_file"

# Generate the .msmtprc content
msmtprc_content=$(cat <<EOL
defaults
auth on
tls on
tls_starttls on
tls_trust_file /etc/ssl/certs/ca-certificates.crt

account gmail
host smtp.gmail.com
port 587
from $sender_email
user $sender_email
password $app_specific_password

logfile /data/msmtp.log
EOL
)

# Write the content to ~/.msmtprc
echo "$msmtprc_content" > ~/.msmtprc

# Use msmtp to send the email
msmtp -a gmail -t "$recipient" < "$email_file"

# Clean up the temporary file
rm "$email_file"
