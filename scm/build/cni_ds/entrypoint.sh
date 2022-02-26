#!/bin/sh -e
# Copyright 2020 Nokia
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause


echo "Copying plugins..."
for plugin in /cni/*
do
  plugin=$(basename "${plugin}")

  echo "  - ${plugin}"
  # Copy files into place and atomically move into final binary name
  cp -fa "/cni/${plugin}" "/host/cni/_${plugin}"
  mv -f  "/host/cni/_${plugin}" "/host/cni/${plugin}"
done

if [ -e /config ]
then
  echo "Copy configuration files..."
  bootstrap_network=$(cat /config/bootstrap_network)

  if [ -e /config/bootstrap_cni_config_data ]
  then
    # If we were given a bootstrap config, then copy it. This may be useful
    # in scenarios where the user originally had a .conflist file installed,
    # and wants to distribute a flat bootstrap CNI configuration alongside
    # with DANM.
    cat /config/bootstrap_cni_config_data | base64 -d > /host/net.d/_${bootstrap_network}.conf
    mv -f /host/net.d/_${bootstrap_network}.conf /host/net.d/${bootstrap_network}.conf
  fi

  if [ ! -f "/host/net.d/${bootstrap_network}.conf" ]
  then
    echo "Bootstrap network configuration ${bootstrap_network} is expected to be exist,"
    echo "but file ${bootstrap_network}.conf does not exist."
    exit 1
  fi

  # Copy kubeconfig
  cp -f /config/danm-kubeconfig /host/net.d/_danm-kubeconfig
  mv -f /host/net.d/_danm-kubeconfig /host/net.d/danm-kubeconfig
  chmod 0600 /host/net.d/danm-kubeconfig

  # Copy DANM configuration. Do this last, in order to minimize the risk of leaving the
  # host in an inoperable state if any of the previous steps failed.
  cp -f /config/00-danm.conf /host/net.d/_00-danm.conf
  mv -f /host/net.d/_00-danm.conf /host/net.d/00-danm.conf
fi
  
echo "Done. Sleeping..."
sleep infinity
