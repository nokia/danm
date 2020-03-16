#!/bin/sh -e

echo "Copying plugins..."

# Copy files into place and atomically move into final binary name
for plugin in /cni/*
do
  plugin=$(basename "${plugin}")

  echo "  - ${plugin}"
  cp -fa /cni/"${plugin}" /host/cni/_"${plugin}"
  mv -f  /host/cni/_"${plugin}" /host/cni/"${plugin}"
done

echo "Done. Sleeping..."
sleep infinity
