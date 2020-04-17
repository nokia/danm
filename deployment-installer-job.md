# Deployment via installer job


## TL;DR;

```
${EDITOR} integration/install/danm-installer-config.yaml
kubectl apply -f integration/install
```

## Installer job

This (currently experimental) method installs DANM using a Kubernetes Job.

Just like the manual deployment, it assumes that a previous CNI (also referred to
as "bootstrap CNI") is already installed. In the setup deployed by this installer,
the bootstrap CNI will both be used by DANM components themselves (ie. netwatcher
and svcwatcher will utilize that bootstrap CNI for their own network connectivity),
as well as being configured as a DanmNet or ClusterNetwork with the name "default",
that will be used by any Kubernetes applications without a `danm.k8s.io` annotation.

Please be aware that the existing (bootstrap) CNI configuration must be a single
CNI, *not* a list of CNIs. This means that in your CNI configuration directory,
`/etc/cni/net.d`, there should be an existing file with a `.conf` extension, often
named something like `10-flannel.conf` or `10-calico.conf`.

If there is a file with a `.conflist` extension (such as `10-calico.conflist`), then
that is a chained list of multiple CNIs. DANM does not currently support using
such a `.conflist` chain as a bootstrap network. Depending on your setup, you may be
able to to extract only the first CNI from the list using a command such as the
following:

```
jq -M '{ name: .name, cniVersion: .cniVersioni} + .plugins[0]' \
  /etc/cni/net.d/${EXISTING_CONFLIST_FILE}.conflist \
  > /etc/cni/net.d/${FIRST_PLUGIN_FROM_LIST_CONFIG_FILE}.conf
```

Either way, please be sure that you have a functional `/etc/cni/net.d/*.conf` CNI
configuration before proceeding, and know the name of that `.conf` file.


### Configuration file (configMap)

This file will need modification to match your setup.

Please review/edit `integration/install/danm-installer-config.yaml`.


### Installer Job resource

This file will need modification only if the installation container needs to be
pulled from an external registry. If this is the case, then please review/edit
`integration/install/danm-installer.yaml`.

If you have built DANM locally and do not need to pull images, this file does not
need updating.


### Deploying the installer

```
kubectl apply -f integration/install
```


### Watching installer progress

After applying the installer CRD, in `kubectl get pods -n kube-system` you should
first see a `danm-installer-*` pod starting, and shortly after, the
`danm-cni` and `netwatcher` daemonsets, `svcwatcher`, and `danm-webhook-deployment`
pods.

The `danm-installer-*` pod should end up in "Completed" status - if not, please check
the pod logs for any errors.


### Cleaning up (optional)

After the installer pod ran to completion, you can remove the installer itself:

```
kubectl delete -f integration/install
```
