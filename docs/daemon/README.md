# Virtual Router Daemon

## 역할
* VirtualRouter가 호스트와 동일한 Layer2에 존재함을 보장

## 동작
* Host 내부에 Linux Bridge를 생성
* Virtual Router Pod 생성에 맞추어 Veth 인터페이스를 생성 및 삭제
* Veth를 Linux Bridge에 연결하고 Peer Interface는 Pod Namespace에게 넘겨줌
* Peer Interface에 IP 할당 및 Routing 설정

## 환경변수
* internalCIDR: 내부 망을 위한 Linux Bridge에 연결한 호스트의 내부망 인터페이스 찾는 용도, 호스트의 내부 대역 기입
* externalCIDR: 외부 망을 위한 Linux Bridge에 연결한 호스트의 외부망 인터페이스 찾는 용도, 호스트의 외부 대역 기입