#!/bin/bash
sudo ip link add name br-1 type bridge
sudo ip link set dev br-1 up
sudo ip link set ens1f3np3 master br-1

sudo ip link add name br-2 type bridge
sudo ip link set dev br-2 up
sudo ip link set ens1f1np1 master br-2

sudo ip link add name br-4 type bridge
sudo ip link set dev br-4 up
# sudo ip link set ens1f0 master br-4