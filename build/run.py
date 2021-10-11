import subprocess
import os
import getopt, sys
import time

# PKG_NAME = 'github.com/cho4036/virtualrouter-controller'
CONTROLLER_GO_BINARY_NAME = '../build/virtualroutermanager/virtualrouter-controller'
CONTROLLER_PKG_NAME = '../cmd/virtualroutermanager/main.go'
DAEMON_GO_BINARY_NAME = '../build/daemon/daemon'
DAEMON_PKG_NAME = '../cmd/daemon/main.go'

DOCKER_REGISTRY = '10.0.0.4:5000/'
CONTROLLER_DOCKER_IMAGE_NAME = "virtualrouter-controller"
CONTROLLER_DOCKER_IMAGE_TAG = "0.0.1"
DAEMON_DOCKER_IMAGE_NAME = 'daemon'
DAEMON_DOCKER_IMAGE_TAG = "0.0.1"

def subprocess_open(command):
    p = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True)
    (stdoutdata, stderrdata) = p.communicate()
    return stdoutdata, stderrdata

def go_build(package, output):
    # out, err = subprocess_open(['go', 'build', '-a', '-o', output, package])
    out, err = subprocess_open(['go', 'build', '-o', output, package])
    if out != "" or err != "":
        return out, err
    return "", ""

def docker_image_check(name, tag, registry):
    image = registry + name + ":" + tag
    out, err = subprocess_open(['docker', 'images', '-f', 'reference=' + image, '-q'])
    if err != "":
        print("image_check_err")
        return 
    return out

def docker_clean(name, tag, registry):
    checkout = docker_image_check(name,tag,registry)
    # print("checkout Type: " + type(checkout))
    if checkout == "" :
        print("There is no image")
    else :
        print("deleting "+ checkout)
        out, err = subprocess_open(['docker', 'rmi', checkout])
        print(out)
        print(err)
        return 

def docker_build(program, name, tag, registry):
    if program == "controller":
        os.chdir("../build/virtualroutermanager")
    else :
        os.chdir("../build/daemon")
    docker_clean(name,tag,registry)
    # checkout = docker_image_check(name,tag,registry)
    # print("checkout Type: " + type(checkout))
    # if checkout == "" :
    #     print("There is no image")
    # else :
    #     print("deleting "+ checkout)
    #     subprocess_open(['docker', 'rmi', checkout])
    image = registry + name + ":" + tag
    out, err = subprocess_open(['docker', 'build', '-t', image, "." ])
    if out != "" or err != "":
        return out, err
    return "", ""

def docker_push(name, tag, registry):
    image = registry + name + ":" + tag
    out, err = subprocess_open(['docker', 'push', image])
    if out != "" or err != "":
        return out, err
    return "", ""

def main(argv):
    FILE_NAME = argv[0]
    program = ""
    pkg_name = ""
    go_binary_name = ""
    docker_image_name = ""
    docker_image_tag = ""
    # print(FILE_NAME)
    if len(argv) < 2 :
            print(FILE_NAME, "few arguments. Choose at least one option")
            sys.exit(2)
    try :
        opts, etc_args = getopt.getopt(sys.argv[1:], "ap:h", ["help", "gobuild", "dockerbuild", "dockerpush", "all", "program="])

    except getopt.GetoptError:
        print(FILE_NAME, '--gobuild , --dockerbuild or --dockerpush')
        sys.exit(2)
    for opt, args in opts:
        if opt in ("-h", "--help"):
            print(FILE_NAME, '-p daemon or controller, --gobuild  --dockerbuild or --dockerpush')
            sys.exit(0)

        if opt in ("-p", "--program"):
            if not(args == "daemon" or args == "controller"): 
                print("Wrong Program name. Select either \"daemon\" or \"controller\"")
                return
            program = args

    if program == "daemon":
        pkg_name = DAEMON_PKG_NAME
        go_binary_name = DAEMON_GO_BINARY_NAME
        docker_image_name = DAEMON_DOCKER_IMAGE_NAME
        docker_image_tag = DAEMON_DOCKER_IMAGE_TAG
    elif program == "controller":
        pkg_name = CONTROLLER_PKG_NAME
        go_binary_name = CONTROLLER_GO_BINARY_NAME
        docker_image_name = CONTROLLER_DOCKER_IMAGE_NAME
        docker_image_tag = CONTROLLER_DOCKER_IMAGE_TAG
    print(program)
    for opt, args in opts:
        print(opt)
        if opt in ("-a","-all"):
            print("print", pkg_name, go_binary_name)
            out, err = go_build(package=pkg_name, output=go_binary_name)
            if err != "" or out != "":
                print("Error: " + err + ", Out: " + out)
                sys.exit(1)
            print("Go build done")

            out, err = docker_build(program=program ,name=docker_image_name,tag=docker_image_tag, registry=DOCKER_REGISTRY)
            if err != "":
                print("Error: " + err)
                sys.exit(1)
            print(out)
            print("Docker build done")

            out, err = docker_push(name=docker_image_name,tag=docker_image_tag, registry=DOCKER_REGISTRY)
            if err != "":
                print("Error: " + err )
                sys.exit(1)
            print(out)
            print("Docker push done")

            docker_clean(name=docker_image_name,tag=docker_image_tag, registry=DOCKER_REGISTRY)

            break
        elif opt in ("--gobuild"):
            print("print", pkg_name, go_binary_name)
            out, err = go_build(package=pkg_name, output=go_binary_name)
            if err != "" or out != "":
                print("Error: " + err + ", Out: " + out)
                sys.exit(1)
            print("Go build done")
        elif opt in ("--dockerbuild"):
            out, err = docker_build(name=docker_image_name,tag=docker_image_tag, registry=DOCKER_REGISTRY)
            if err != "":
                print("Error: " + err)
                sys.exit(1)
            print(out)
            print("Docker build done")
        elif opt in ("dockerpush"):
            out, err = docker_push(name=docker_image_name,tag=docker_image_tag, registry=DOCKER_REGISTRY)
            if err != "":
                print("Error: " + err )
                sys.exit(1)
            print(out)
            print("Docker push done")
            docker_clean(name=docker_image_name,tag=docker_image_tag, registry=DOCKER_REGISTRY)


    # os.chdir("../")
    # currentPath = os.getcwd()
    # print(currentPath)

if __name__ == "__main__":
    main(sys.argv)