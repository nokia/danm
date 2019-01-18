#!/bin/bash

if [[ $WITH_BUILDAH == 'Y' ]]; then
    
    sudo systemctl disable docker

    apt-get -y install software-properties-common
    add-apt-repository -y ppa:alexlarsson/flatpak
    add-apt-repository -y ppa:gophers/archive
    apt-add-repository -y ppa:projectatomic/ppa
    apt-get -y -qq update
    apt-get -y install bats btrfs-tools git libapparmor-dev libdevmapper-dev libglib2.0-dev libgpgme11-dev libostree-dev libseccomp-dev libselinux1-dev skopeo-containers go-md2man
    apt-get -y install golang-1.10

    mkdir -p "$GOPATH/src/github.com/containers/buildah"
    git clone https://github.com/containers/buildah "$GOPATH/src/github.com/containers/buildah"
    cd "$GOPATH/src/github.com/containers/buildah" || exit
    PATH=/usr/lib/go-1.10/bin:$PATH make runc all SECURITYTAGS="apparmor seccomp"
    make install install.runc
    
    cat > /etc/containers/registries.conf <<- "EOF"
        [registries.search]
        registries = ['docker.io', 'registry.fedoraproject.org', 'quay.io', 'registry.access.redhat.com', 'registry.centos.org']
        
        [registries.insecure]
        registries = []
        
        [registries.block]
        registries = []
EOF

    apt-get -y install libprotobuf-dev libprotobuf-c0-dev python3-setuptools
    git clone https://github.com/kubernetes-sigs/cri-o "$GOPATH/src/github.com/kubernetes-sigs/cri-o"
    cd "$GOPATH/src/github.com/kubernetes-sigs/cri-o" || exit
    mkdir bin
    make bin/conmon
    install -D -m 755 bin/conmon /usr/libexec/podman/conmon

    git clone https://github.com/containers/libpod/ "$GOPATH/src/github.com/containers/libpod"
    cd "$GOPATH/src/github.com/containers/libpod" || exit
    make
    make install PREFIX=/usr
fi