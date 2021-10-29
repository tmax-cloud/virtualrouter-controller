# VirtualRouter 

## What is Virtual Router?
* Tenant에게 Layer 2 수준에서의 독립적인 네트워크를 제공하기 위한 가상의 게이트웨이
* Tenant Network의 Default Gateway
* Tenant 마다의 독립적인 NAT, LB, FW 등의 NFV 기능을 제공

## 구성 요소 및 버전
* VirtualRouter/Controller([tmaxcloudck/virtualrouter-controller:0.0.1](https://hub.docker.com/repository/docker/tmaxcloudck/virtualrouter-controller))
* VirtualRouter/Daemon([tmaxcloudck/virtualrouter-daemon:0.0.1](https://hub.docker.com/repository/docker/tmaxcloudck/virtualrouter-daemon))


## 폐쇄망 설치 가이드
설치를 진행하기 전 아래의 과정을 통해 필요한 이미지 및 yaml 파일을 준비한다.
1. **폐쇄망에서 설치하는 경우** 사용하는 image repository에 virtual router 설치 시 필요한 이미지를 push한다. 

    * 작업 디렉토리 생성 및 환경 설정
    ```bash
    $ mkdir -p ~/virtualrouter-install
    $ export VIRTUALROUTER_HOME=~/virtualrouter-install
    $ export VIRTUALROUTER_CONTROLLER_VERSION=0.0.1
    $ export VIRTUALROUTER_DAEMON_VERSION=0.0.1
    $ export REGISTRY=172.22.8.106:5000
    $ cd $VIRTUALROUTER_HOME
    ```

    * 외부 네트워크 통신이 가능한 환경에서 필요한 이미지를 다운받는다.
    ```bash
    $ sudo docker pull tmaxcloudck/virtualrouter-controller:${VIRTUALROUTER_CONTROLLER_VERSION}
    $ sudo docker save tmaxcloudck/virtualrouter-controller:${VIRTUALROUTER_CONTROLLER_VERSION} > virtualrouter-controller_${VIRTUALROUTER_CONTROLLER_VERSION}.tar
    $ sudo docker pull tmaxcloudck/virtualrouter-daemon:${VIRTUALROUTER_DAEMON_VERSION}
    $ sudo docker save tmaxcloudck/virtualrouter-daemon:${VIRTUALROUTER_DAEMON_VERSION} > virtualrouter-daemon_${VIRTUALROUTER_DAEMON_VERSION}.tar
    ```

    * deploy를 위한 virtualrouter controller & daemon yaml을 다운로드한다. 
    ```bash
    $ curl https://github.com/tmax-cloud/virtualrouter-controller/deploy/controller/deploy.yaml > controller_deploy.yaml
    $ curl https://github.com/tmax-cloud/virtualrouter-controller/deploy/daemon/deploy.yaml > daemon_deploy.yaml
    ```

    * deploy를 위한 virtualrouter CRD와 role, namespace에 대한 yaml을 다운로드한다. 
    ```bash
    $ curl https://github.com/tmax-cloud/virtualrouter-controller/deploy/integrated/namespace.yaml > namespace.yaml
    $ curl https://github.com/tmax-cloud/virtualrouter-controller/deploy/integrated/role.yaml > controller_role.yaml
    $ curl https://github.com/tmax-cloud/virtualrouter-controller/deploy/integrated/virtaulrouter-crd.yaml > virtualrouter-crd.yaml
    ```



2. 위의 과정에서 생성한 tar 파일들을 폐쇄망 환경으로 이동시킨 뒤 사용하려는 registry에 이미지를 push한다.
    ```bash
    $ sudo docker load < virtualrouter-controller_${VIRTUALROUTER_CONTROLLER_VERSION}.tar
    $ sudo docker load < virtualrouter-daemon_${VIRTUALROUTER_DAEMON_VERSION}.tar

    $ sudo docker tag virtualrouter-controller_${VIRTUALROUTER_CONTROLLER_VERSION} ${REGISTRY}/virtualrouter-controller:${VIRTUALROUTER_CONTROLLER_VERSION}
    $ sudo docker tag virtualrouter-daemon_${VIRTUALROUTER_DAEMON_VERSION} ${REGISTRY}/virtualrouter-daemon:${VIRTUALROUTER_DAEMON_VERSION}

    $ sudo docker push ${REGISTRY}/virtualrouter-controller:${VIRTUALROUTER_CONTROLLER_VERSION}
    $ sudo docker push ${REGISTRY}/virtualrouter-daemon:${VIRTUALROUTER_DAEMON_VERSION}
    ```

## 설치 가이드
0. [deploy.yaml 수정](#step0 "step0")
1. [VirtualRouter의 네트워크 대역 설정](#step1 "step1")
2. [Virtualrouter Controller & Daemon 설치](#step2 "step2")

<h2 id="step0"> Step0. VirtualRouter Controller & Daemon deploy yaml 수정 </h2>

* 목적 : `deploy yaml에 이미지 registry, 버전 정보 수정`
* 생성 순서 : 
    * 아래의 command를 수정하여 사용하고자 하는 image 버전 정보를 수정한다. (기본 설정 버전은 0.0.1)
	```bash
            sed -i 's/0.0.1/'${VIRTUALROUTER_CONTROLLER_VERSION}'/g' controller_deploy.yaml
            sed -i 's/0.0.1/'${VIRTUALROUTER_DAEMON_VERSION}'/g' daemon_deploy.yaml
	```

* 비고 :
    * `폐쇄망에서 설치를 진행하여 별도의 image registry를 사용하는 경우 registry 정보를 추가로 설정해준다.`
	```bash
            sed -i 's/tmaxcloudck\/virtualrouter-controller/'${REGISTRY}'\/virtualrouter-controller/g' controller_deploy.yaml 
            sed -i 's/tmaxcloudck\/virtualrouter-daemon/'${REGISTRY}'\/virtualrouter-daemon/g' daemon_deploy.yaml 
	```


<h2 id="step1"> Step 1. VirtualRouter의 네트워크 대역 설정 </h2>

* 목적 : `VirtualRouter에서 사용할 내부&외부 대역 설정 (VirtualRouter를 사용할 호스트의 내부&외부 대역 사용)`
* 생성 순서: daemon_deploy.yaml의 env 값에 Virtual Router를 사용할 Host의 내부&외부 대역을 기재. 
* <b>Linux Bridge를 생성하고 연결할 Interface를 찾기 위한 설정</b>
* <b>Pod 대역이 사용할 대역과 무관함! </b>
* <b>VirtualRouter Instance를 사용할 모든 Host는 동일한 내부 외부 대역을 가져야함.</b>
* 예제 :
    Host가 외부망으로 192.168.9.0/24 대역을 사용하고 내부망으로 10.0.0.0/24 대역을 사용하는경우
    ```yaml
    env:
        - name: internalCIDR
          value: "10.0.0.0/24"
        - name: externalCIDR
          value: "192.168.9.0/24"
    ```
    

<h2 id="step2"> Step 2. VirtualRouter Controller & Daemon 설치 </h2>

* 목적 : `VirtualRouter Controller & Daemon 정상 기동`
* 생성 순서: 
1. Namespace, VirtualRouter CRD, role 적용
    ```bash
    kubectl apply -f namespace.yaml
    kubectl apply -f controller_role.yaml
    kubectl apply -f virtualrouter-crd.yaml
    ```
2. VirtualRouter Controller & Daemon.yaml 설치  
    ```bash
    kubectl apply -f controller_deploy.yaml -f daemon_deploy.yaml
    ```


## 삭제 가이드
1. 이전 설치시 VirtualRouter yaml을 설치한 디렉토리로 이동 및 VirtualRouter 삭제
    * 작업 디렉토리 생성 및 환경 설정
    ```bash
    cd ~/virtualrouter-install
    kubectl delete -f controller_deploy.yaml -f daemon_deploy.yaml
    kubectl delete -f controller_role.yaml
    kubectl delete -f virtualrouter-crd.yaml
    kubectl delete -f namespace.yaml
    cd ..
    rm -r virtualrouter-install
    ```