# Virtual Router Controller

## 역할
* 클러스터 내부에 Virtual Router에 대한 Deployment를 생성, 삭제

## 동작
* 내부적으로 VirtualRouter CR을 watching하며 k8s cluster에 deployment resource를 생성, 삭제함
