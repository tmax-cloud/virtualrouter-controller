#!/bin/bash

private_registry="192.168.7.191:5000"

function build() {
    go build .
    mv virtual-router ./build
    docker build -t ${private_registry}/virtual-router:0.1 ./build/
    docker push ${private_registry}/virtual-router:0.1
}

function install() {
    kubectl apply -f deploy/namespace.yaml
    kubectl apply -f deploy/crd-validation.yaml
    kubectl apply -f deploy/deploy.yaml
}

function uninstall() {
    kubectl delete -f deploy/crd-validation.yaml
    kubectl delete -f deploy/service.yaml
    kubectl delete -f deploy/deploy.yaml
    kubectl delete -f deploy/namespace.yaml
}

function main() {
    case "${1:-}" in
    build)
        build
        ;;
    install)
        install
        ;;
    uninstall)
        uninstall
        ;;
    *)
        set +x
        echo "service list:" >&2
        echo "  $0 build" >&2
        echo "  $0 install" >&2
        echo "  $0 uninstall" >&2
        ;;
    esac
}

main $1