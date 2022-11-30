# VirtualRouter 

## What is Virtual Router?
* Tenant에게 Layer 2 수준에서의 독립적인 네트워크를 제공하기 위한 가상의 게이트웨이
* Tenant Network의 Default Gateway
* Tenant 마다의 독립적인 NAT, LB, FW 등의 NFV 기능을 제공

## 구성 요소 및 버전
* VirtualRouter/Controller([tmaxcloudck/virtualrouter-controller](https://hub.docker.com/repository/docker/tmaxcloudck/virtualrouter-controller))
* VirtualRouter/Daemon([tmaxcloudck/virtualrouter-daemon](https://hub.docker.com/repository/docker/tmaxcloudck/virtualrouter-daemon))
* VirtualRouter([tmaxcloudck/virtualrouter](https://hub.docker.com/repository/docker/tmaxcloudck/virtualrouter))
* </b>위 링크를 통해 현재 제공중인 버전을 반드시 확인하고 사용</b>

## 폐쇄망 설치 가이드
설치를 진행하기 전 아래의 과정을 통해 필요한 이미지 및 yaml 파일을 준비한다.
1. **폐쇄망에서 설치하는 경우** 사용하는 image repository에 필요 버전을 확인하여 virtual router 설치 시 필요한 이미지를 push한다. 

    * 작업 디렉토리 생성 및 환경 설정 (version 확인 후 변경 vx.y.z, registry 주소 변경 a.b.c.d:e)
    ```bash
    $ mkdir -p ~/virtualrouter-install
    $ export VIRTUALROUTER_HOME=~/virtualrouter-install
    $ export VIRTUALROUTER_CONTROLLER_VERSION=vx.y.z
    $ export VIRTUALROUTER_DAEMON_VERSION=vx.y.z
    $ export VIRTUALROUTER_VERSION=vx.y.z
    $ export REGISTRY=a.b.c.d:e
    $ cd $VIRTUALROUTER_HOME
    ```

    * 외부 네트워크 통신이 가능한 환경에서 필요한 이미지를 다운받는다.
    ```bash
    $ sudo docker pull tmaxcloudck/virtualrouter-controller:${VIRTUALROUTER_CONTROLLER_VERSION}
    $ sudo docker save tmaxcloudck/virtualrouter-controller:${VIRTUALROUTER_CONTROLLER_VERSION} > virtualrouter-controller_${VIRTUALROUTER_CONTROLLER_VERSION}.tar
    $ sudo docker pull tmaxcloudck/virtualrouter-daemon:${VIRTUALROUTER_DAEMON_VERSION}
    $ sudo docker save tmaxcloudck/virtualrouter-daemon:${VIRTUALROUTER_DAEMON_VERSION} > virtualrouter-daemon_${VIRTUALROUTER_DAEMON_VERSION}.tar
    $ sudo docker pull tmaxcloudck/virtualrouter:${VIRTUALROUTER_VERSION}
    $ sudo docker save tmaxcloudck/virtualrouter:${VIRTUALROUTER_VERSION} > virtualrouter_${VIRTUALROUTER_VERSION}.tar
    ```

    * deploy를 위한 virtualrouter controller & daemon yaml을 다운로드한다. 
    ```bash
    $ curl https://raw.githubusercontent.com/tmax-cloud/virtualrouter-controller/main/deploy/controller/controller_deploy.yaml > controller_deploy.yaml
    $ curl https://raw.githubusercontent.com/tmax-cloud/virtualrouter-controller/main/deploy/daemon/daemon_deploy.yaml > daemon_deploy.yaml
    ```

    * deploy를 위한 virtualrouter CRD와 role, namespace에 대한 yaml을 다운로드한다. 
    ```bash
    $ curl https://raw.githubusercontent.com/tmax-cloud/virtualrouter-controller/main/deploy/integrated/namespace.yaml > namespace.yaml
    $ curl https://raw.githubusercontent.com/tmax-cloud/virtualrouter-controller/main/deploy/integrated/controller_role.yaml > controller_role.yaml
    $ curl https://raw.githubusercontent.com/tmax-cloud/virtualrouter-controller/main/deploy/integrated/virtualrouter-crd.yaml > virtualrouter-crd.yaml
    ```

    * NFV Function 사용을 위한 NFV CRD와 Virtualrouter role에 대한 yaml을 다운로드한다. 
    ```bash
    $ curl https://raw.githubusercontent.com/tmax-cloud/virtualrouter/main/deploy/natruleCRD.yaml > natruleCRD.yaml
    $ curl https://raw.githubusercontent.com/tmax-cloud/virtualrouter/main/deploy/firewallCRD.yaml > firewallCRD.yaml
    $ curl https://raw.githubusercontent.com/tmax-cloud/virtualrouter/main/deploy/loadbalancerCRD.yaml > loadbalancerCRD.yaml
    ```



2. 위의 과정에서 생성한 tar 파일들을 폐쇄망 환경으로 이동시킨 뒤 사용하려는 registry에 이미지를 push한다.
    ```bash
    $ sudo docker load < virtualrouter-controller_${VIRTUALROUTER_CONTROLLER_VERSION}.tar
    $ sudo docker load < virtualrouter-daemon_${VIRTUALROUTER_DAEMON_VERSION}.tar
    $ sudo docker load < virtualrouter_${VIRTUALROUTER_VERSION}.tar

    $ sudo docker tag virtualrouter-controller_${VIRTUALROUTER_CONTROLLER_VERSION} ${REGISTRY}/virtualrouter-controller:${VIRTUALROUTER_CONTROLLER_VERSION}
    $ sudo docker tag virtualrouter-daemon_${VIRTUALROUTER_DAEMON_VERSION} ${REGISTRY}/virtualrouter-daemon:${VIRTUALROUTER_DAEMON_VERSION}
    $ sudo docker tag virtualrouter_${VIRTUALROUTER_VERSION} ${REGISTRY}/virtualrouter:${VIRTUALROUTER_VERSION}

    $ sudo docker push ${REGISTRY}/virtualrouter-controller:${VIRTUALROUTER_CONTROLLER_VERSION}
    $ sudo docker push ${REGISTRY}/virtualrouter-daemon:${VIRTUALROUTER_DAEMON_VERSION}
    $ sudo docker push ${REGISTRY}/virtualrouter:${VIRTUALROUTER_VERSION}
    ```

## 설치 가이드
0. [deploy.yaml 수정](#step0 "step0")
1. [VirtualRouter의 네트워크 대역 설정](#step1 "step1")
2. [Virtualrouter Controller & Daemon 설치](#step2 "step2")
3. [VirtualRouter Instance 배포 사전작업](#step3 "step3")
4. [VirtualRouter Instance 배포](#step4 "step4")

<h2 id="step0"> Step0. VirtualRouter Controller & Daemon & VirtualRouter deploy yaml 수정 </h2>

* 목적 : `deploy yaml에 이미지 registry, 버전 정보 수정`
* 생성 순서 : 
    * 아래의 command를 수정하여 사용하고자 하는 image 버전 정보를 수정한다. (해당 버전에 맞게 설정 vx.y.z)
	```bash
            sed -i 's/vx.y.z/'${VIRTUALROUTER_CONTROLLER_VERSION}'/g' controller_deploy.yaml
            sed -i 's/vx.y.z/'${VIRTUALROUTER_DAEMON_VERSION}'/g' daemon_deploy.yaml
            sed -i 's/vx.y.z/'${VIRTUALROUTER_VERSION}'/g' example-virtualrouter.yaml
	```

* 비고 :
    * `폐쇄망에서 설치를 진행하여 별도의 image registry를 사용하는 경우 registry 정보를 추가로 설정해준다.`
	```bash
            sed -i 's/tmaxcloudck\/virtualrouter-controller/'${REGISTRY}'\/virtualrouter-controller/g' controller_deploy.yaml 
            sed -i 's/tmaxcloudck\/virtualrouter-daemon/'${REGISTRY}'\/virtualrouter-daemon/g' daemon_deploy.yaml 
            sed -i 's/tmaxcloudck\/virtualrouter/'${REGISTRY}'\/virtualrouter/g' example-virtualrouter.yaml
	```


<h2 id="step1"> Step 1. VirtualRouter Controller&Daemon을 설치하기 위한 초기 설정 </h2>


* 목적 : `VirtualRouter에서 사용할 내부&외부용 NIC에 대한설정`
* 순서 : daemon을 deploy할 노드에 label 추가 및 해당 노드의 NIC 이름을 annotation 으로 설정
* 예제 :
    ```bash
    kubectl label nodes {node 이름} virtualrouter/daemon=deploy
    kubectl annotate nodes {node 이름} externalInterface={external용 Interface Name}
    kubectl annotate nodes {node 이름} internalInterface={internal용 Interface Name}
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

<h2 id="step3"> Step 3. VirtualRouter Instance 배포 사전작업 </h2>

* 목적 : `VirtualRouter Intance 배포를 위한 CRD 적용`
* 생성 순서: 
1. Virtual Router가 사용할 NFV CRD 적용
    ```bash
    kubectl apply -f natruleCRD.yaml.yaml -f firewallCRD.yaml -f loadbalancerCRD.yaml
    ```

<h2 id="step4"> Step 4. VirtualRouter Instance 배포 </h2>

* 목적 : `VirtualRouter Intance 배포를 위한 CR 생성`
* 생성 순서: 
1. Tenant 네트워크와 일치하도록 Virtual Router에 대한 기본 설정 적용

    ex) Tenant Network 설정이 아래와 같을 때, yaml 예제([example-virtualrouter.yaml](../deploy/integrated/example-virtualrouter.yaml))
    - VLAN: 201
    - Tenant의 내부 대역 Network CIDR: 10.10.10.0/24, 
    - Virtual Router 내부 대역의 Interface의 IP 주소: 10.10.10.11
    - Tenant의 외부 대역 Network CIDR: 192.168.8.0/24
    - Virtual Router 외부 대역의 Interface의 IP 주소: 192.168.8.153
    - Virtual Router 외부 대역의 게이트웨이 IP 주소: 192.168.8.1
    ```yaml
    spec:
      vlanNumber: 210
      internalIP: 10.10.10.11
      internalNetmask: 255.255.255.0
      externalIP: 192.168.8.153
      externalNetmask: 255.255.255.0
      gatewayIP: 192.168.8.1
    ```

2. Virtual Router Instance 배포
    ```bash
    kubectl apply -f example-virtualrouter.yaml
    ```

## 사용 가이드
* 정상 생성 된 Virtual Router에 NFV 룰을 적용하여 Tenant Network에 네트워크 기능 제공

1. NAT Rule 적용 시나리오
    1. SNAT 적용을 위한 CR 사용법
        * Match에 Tenant의 네트워크의 CIDR을 기입
        * Action의 srcIP에 0.0.0.0(MASQUERADE)를 입력
        * ex) [masqExample.yaml](https://github.com/tmax-cloud/virtualrouter/blob/main/deploy/masqExample.yaml)

        ```yaml
        apiVersion: virtualrouter.tmax.hypercloud.com/v1
        kind: NATRule
        metadata:
        name: testmasquerade
        namespace: virtualrouter
        spec:
        rules:
        - match:
            srcIP: 10.10.10.0/24
            protocol: all
            action:
            srcIP: 0.0.0.0
        ```

    2. StaticNAT(FloatingIP) 적용을 위한 CR 사용법 
        * privateIP <==> publicIP 에 대한 1대1 대응 rule 생성
        * match에는 /32로 32 Masking, action에는 Masking 표현 없이 기재
        * ex) 10.10.10.4(internalIP) <==> 192.168.9.134(publicIP)
        * ex) [staticNATExample.yaml](https://github.com/tmax-cloud/virtualrouter/blob/main/deploy/staticNATExample.yaml)
        
        ```yaml
        apiVersion: virtualrouter.tmax.hypercloud.com/v1
        kind: NATRule
        metadata:
        name: teststaticnat
        namespace: virtualrouter
        spec:
        rules:
        - match:
            srcIP: 10.10.10.4/32 # private IP
            protocol: all
          action:
            srcIP: 192.168.9.134 # public IP
        - match:
            dstIP: 192.168.9.134/32 # public IP
            protocol: all
          action:
            dstIP: 10.10.10.4 # private IP
        ```
2. FW Rule 적용 시나리오
    1. FW 적용을 위한 CR 사용법
        * Default Policy는 Drop으로 WhiteList로써 관리
        * private CIDR <==> public CIDR 에 대한 "ACCEPT" or "DROP" policy 생성
        * 정상적으로 허용 rule을 만들기 위해선 아래 예제와 같이 양방향에 대한 rule을 생성해야함
        * ex) 10.10.10.0/24 (internalIP) <==> 192.168.9.31/32 (publicIP) 통신 허용 rule 생성
        * ex) [firewallExample.yaml](https://github.com/tmax-cloud/virtualrouter/blob/main/deploy/firewallExample.yaml)
        
        ```yaml
        apiVersion: virtualrouter.tmax.hypercloud.com/v1
        kind: FireWallRule
        metadata:
          name: testfw
          namespace: virtualrouter
        spec:
          rules:
          - match:
              srcIP: 10.10.10.0/24
              dstIP: 192.168.9.31/32
              protocol: all
            action:
              policy: ACCEPT
          - match:
              srcIP: 192.168.9.31/32
              dstIP: 10.10.10.0/24
              protocol: all
            action:
              policy: ACCEPT
        ```
3. LB Rule 적용 시나리오
    1. LB 적용을 위한 CR 사용법
        * VIP에 대한 Target IPs를 기입하며 각 Target에 대한 가중치를 기입
        * 가중치의 범위는 1~100 사이의 자연수 값이며 (가중치 값 / 100) 확률로 해당 Target이 선택됨
        * 마지막 Target의 경우 항상 weight는 100으로 설정
        * ex) 아래 예제의 경우 10.10.10.3과 10.10.10.4 간에 4:6 비율로 부하 분산이 됨
        * ex) [loadbalancerExample.yaml](https://github.com/tmax-cloud/virtualrouter/blob/main/deploy/loadbalancerExample.yaml)
        
        ```yaml
        apiVersion: virtualrouter.tmax.hypercloud.com/v1
        kind: LoadBalancerRule
        metadata:
          name: testloadbalancer
          namespace: virtualrouter
        spec:
          rules:
          - loadBalancerIP: 192.168.9.133  # LB IP
            loadBalancerPort: 10000        # LB port
            backendIPs:
            - backendIP: 10.10.10.3        # target1 IP
              backendPort: 20000           # target1 Port
              weight: 40
            - backendIP: 10.10.10.4        # target2 IP
              backendPort: 20000           # target2 Port
              weight: 100
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
