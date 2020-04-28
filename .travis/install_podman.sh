#!/bin/bash

setup () {
# https://github.com/containers/libpod/blob/master/install.md#ubuntu
echo travis_fold:start:install_podman
sudo apt-get update -qq
sudo apt-get install -qq -y software-properties-common uidmap
sudo add-apt-repository -y ppa:projectatomic/ppa
sudo apt-get update -qq
sudo apt-get -qq -y install podman
echo travis_fold:end:install_podman
}

verify () {
# Show version and info.
echo travis_fold:start:verify_podman
podman --version
podman version
podman info --debug
apt-cache show podman
dpkg-query -L podman
echo travis_fold:end:verify_podman
}

configure () {
echo travis_fold:start:configure_podman
# Hack podman's configuration files.
# /etc/containers/registries.conf does not exist.
# https://clouding.io/kb/en/how-to-install-and-use-podman-on-ubuntu-18-04/
ls -1 /etc/containers/registries.conf || true
sudo mkdir -p /etc/containers

sudo tee -a /etc/containers/registries.conf >/dev/null <<-'EOF'
     [registries.search]
     registries = ['docker.io', 'registry.fedoraproject.org', 'quay.io', 'registry.access.redhat.com', 'registry.centos.org']

     [registries.insecure]
     registries = []

     [registries.block]
     registries = []

EOF
echo travis_fold:end:configure_podman
}

main () {

}


main