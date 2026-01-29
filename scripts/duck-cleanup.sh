#!/bin/bash
TOKEN="c25b2730-ac68-4177-9d77-e05d0c87af13"
DOMAIN="micromon"
echo "Cleaning TXT record..."
curl -s "https://www.duckdns.org/update?domains=$DOMAIN&token=$TOKEN&txt=&clear=true"
