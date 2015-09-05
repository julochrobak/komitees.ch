#!/bin/bash
sudo setcap 'cap_net_bind_service=+ep' bin/committees
nohup ./bin/committees -port 80 > committees.log 2>&1 &
