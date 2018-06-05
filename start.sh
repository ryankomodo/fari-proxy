#!/bin/bash

if [ "$1" = "client" ]
then
  echo "Starting client..."
  sudo supervisord -c supervisord.conf
  sudo supervisorctl start fari-client
elif [ "$1" = "server" ]
then
  echo "Starting server..."
  sudo supervisord -c supervisord.conf
  sudo supervisorctl start fari-server
else
  echo "Invalid parameter"
fi
