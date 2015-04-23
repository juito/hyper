# INTRODUCE

* This repository includes the command line and daemon of DVM

* The daemon of DVM is used for communicating with docker's daemon to get and set data

# BUILD
### DVM client side
* go build dvmcli.go
* ./dvmcli info

### DVM daemon side
* go build daemon.go
* ./davmon

# RUN
### DVM daemon side
* ./daemon with root

### DVM client side
* ./dvmcli info
* ./dvmcli create tomcat:latest (This command will create a new container of "tomcat:latest")
* ./dvmcli list [pod|vm|container] (currently, the 'container' will be supported later)
* ./dvmcli pod test.pod (create a new POD) 
* ./dvmcli stop $POD_ID (stop a POD)
