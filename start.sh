#!/bin/bash

if [ "$1" = "client" ]
then
  sudo cp fari-client.service /etc/systemd/system/
  systemctl enable fari-client
  systemctl start fari-client
  systemctl status fari-client -l
elif [ "$1" = "server" ]
then
  sudo cp fari-server.service /etc/systemd/system/
  systemctl enable fari-server
  systemctl start fari-server
  systemctl status fari-server -l
else
  echo "Invalid parameter"
fi
