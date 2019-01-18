#!/bin/bash

if [[ $WITH_BUILDAH == 'Y' ]]; then
    
    sudo rm -f  "$(command -v docker)"
	
    sudo apt-get -y install software-properties-common
    sudo add-apt-repository -y ppa:alexlarsson/flatpak
    sudo add-apt-repository -y ppa:gophers/archive
    sudo apt-add-repository -y ppa:projectatomic/ppa
    sudo apt-get -y -qq update
    sudo apt-get -y install bats btrfs-tools git libapparmor-dev libdevmapper-dev libglib2.0-dev libgpgme11-dev libostree-dev libseccomp-dev libselinux1-dev skopeo-containers go-md2man
    
    mkdir -p "$GOPATH/src/github.com/containers/buildah"
    git clone https://github.com/containers/buildah "$GOPATH/src/github.com/containers/buildah"
    cd "$GOPATH/src/github.com/containers/buildah" || exit
    make runc all SECURITYTAGS="apparmor seccomp"
    sudo make install install.runc
    
    sudo -s 'cat > /etc/containers/registries.conf' <<- "EOF"
        [registries.search]
        registries = ['docker.io', 'registry.fedoraproject.org', 'quay.io', 'registry.access.redhat.com', 'registry.centos.org']
        
        [registries.insecure]
        registries = []
        
        [registries.block]
        registries = []
EOF

    sudo apt-get -y install libprotobuf-dev libprotobuf-c0-dev python3-setuptools
    git clone https://github.com/kubernetes-sigs/cri-o "$GOPATH/src/github.com/kubernetes-sigs/cri-o"
    cd "$GOPATH/src/github.com/kubernetes-sigs/cri-o" || exit
    mkdir bin
    make bin/conmon
    sudo install -D -m 755 bin/conmon /usr/libexec/podman/conmon

    git clone https://github.com/containers/libpod/ "$GOPATH/src/github.com/containers/libpod"
    cd "$GOPATH/src/github.com/containers/libpod" || exit
    make
    sudo make install PREFIX=/usr
fi