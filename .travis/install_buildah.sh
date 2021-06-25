#!/bin/bash
sudo apt-get -y install software-properties-common
sudo add-apt-repository -y ppa:alexlarsson/flatpak
sudo add-apt-repository -y ppa:gophers/archive
sudo apt-add-repository -y ppa:projectatomic/ppa
sudo apt-get -y -qq update
sudo apt-get -y install runc buildah

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
