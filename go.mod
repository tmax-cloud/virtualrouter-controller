// This is a generated file. Do not edit directly.

module github.com/cho4036/virtualrouter-controller

go 1.15

require (
	github.com/google/uuid v1.2.0
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.3 // indirect
	github.com/pkg/errors v0.9.1
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20200520041808-52d707b772fe
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2 // indirect
	golang.org/x/oauth2 v0.0.0-20201109201403-9fd604954f58 // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	google.golang.org/grpc v1.38.0
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/cri-api v0.0.0
	k8s.io/klog/v2 v2.8.0
	k8s.io/kubernetes v0.0.0-00010101000000-000000000000
)

replace (
	k8s.io/api => k8s.io/api v0.19.15
	// k8s.io/api => k8s.io/api v0.0.0-20210329192645-60680b5087d3
	k8s.io/apiextensions-apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20200804142048-1afc53514032
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.15
	// k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20210329192041-0c7db653e2b6
	k8s.io/apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20201201102839-3321f00ed14e
	k8s.io/cli-runtime => k8s.io/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20201201102839-3321f00ed14e
	// k8s.io/client-go => k8s.io/client-go v0.0.0-20210329193902-8b9f5901612d
	k8s.io/client-go => k8s.io/client-go v0.19.15
	k8s.io/cloud-provider => k8s.io/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20201201102839-3321f00ed14e
	k8s.io/cluster-bootstrap => k8s.io/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20201201102839-3321f00ed14e
	k8s.io/code-generator => k8s.io/code-generator v0.19.15
	k8s.io/component-base => k8s.io/kubernetes/staging/src/k8s.io/component-base v0.0.0-20201201102839-3321f00ed14e
	k8s.io/component-helpers => k8s.io/kubernetes/staging/src/k8s.io/component-helpers v0.0.0-20201201102839-3321f00ed14e
	k8s.io/controller-manager => k8s.io/kubernetes/staging/src/k8s.io/controller-manager v0.0.0-20201201102839-3321f00ed14e
	//
	k8s.io/cri-api => k8s.io/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20201201102839-3321f00ed14e
	k8s.io/csi-translation-lib => k8s.io/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20201201102839-3321f00ed14e
	k8s.io/kube-aggregator => k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20201201102839-3321f00ed14e
	k8s.io/kube-controller-manager => k8s.io/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20201201102839-3321f00ed14e
	k8s.io/kube-proxy => k8s.io/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20201201102839-3321f00ed14e
	k8s.io/kube-scheduler => k8s.io/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20201201102839-3321f00ed14e
	k8s.io/kubectl => k8s.io/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20201201102839-3321f00ed14e
	k8s.io/kubelet => k8s.io/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20201201102839-3321f00ed14e
	k8s.io/kubernetes => k8s.io/kubernetes v1.19.0
	k8s.io/legacy-cloud-providers => k8s.io/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20201201102839-3321f00ed14e
	k8s.io/metrics => k8s.io/kubernetes/staging/src/k8s.io/metrics v0.0.0-20201201102839-3321f00ed14e
	k8s.io/sample-apiserver => k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20201201102839-3321f00ed14e
)
