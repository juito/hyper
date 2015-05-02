# Pod in VM spec

## General Message

- message will be 

## Start Pod

### Example

    {
        "action": "start"
	    "pod": {
	    	“hostname”: "hostname",
	        "containers": [{
	            "id": 				"uniq-hash-of-container",
	            "rootfs": 			"/rootfs",
	            “fstype”:			"ext4",
	            "images": [
	                "device_name",
	            ], 
	            "volumes": [{
	                "device": 		"device_name",
	                "mount" : 		"/mount/point/path",
	                "fstype":		"ext4",
	                "readOnly": 	false
	            }],
	            "fsmap": [{
	                "source": 		"/path/relative/to/9p/share/dir",
	                "path":   		"/file/path/in/container",
	                "readOnly":	    false
	            }],
	            "tty": 				"",
	            "workdir": 			"/ralative/path/to/rootfs",
	            "cmd": 				"/ralative/path/to/rootfs and parameters",
	            "envs": [{
	                "env": 			"name",
	                "value": 		"value"
	            }],
	            "restartPolicy": 	"never"
	        }],
	        "interfaces": [{
	            "device": 			"bus_address",
	            "ipAddress": 		"1.3.5.7",
	            "netMask": 			"255.255.255.0"
	        }],
	        "routes": [{
	        	"dest":				"0.0.0.0/0",
	        	"gateway":			"ip_addr",
	        	"device":			"bus_address_in_interfaces"
	        }],
	        "socket": 				"chardev.name",
	        "shareDir":     		"mount_tag_9p"
	    }
	}

### Fields

- `hostname`: hostname for containers in POD
- `containers`: spec of containers in pod
  - `id`: hash or name of container for the convenience of reference
  - about filesystem:
    - `rootfs`, the rootfs path relative to the root device, `/rootfs` by default 
    - `fstype`, filesystem type of the rootfs images
    - `images`, the device for root device, if more than one is given, it should be `aufs` layers, from buttom to up.
    - `volumes`, volumes need to be mounted:
      - `device`, the name of device, **no** `9p` fs as we decided only one 9p fs in VM, 
      
        **Note**: the device may be mount in multiple containers.
      - `mount`: mount point of the volume
      - `fstype`:filesystem type of the volume
      - `readOnly`: true or false
    - `fsmap`: files and dirs, which need bind to container, should be put in 
               the dir and map into vm with `9p`
      - `source`: the path of the file in 9p fs
      - `path`: the path should be bind to in container
  - exec configuration:
  - `restartPolicy`: `never`, `onFailure`, or `always`
  - `tty`: tty char device name for debug/attach
- `devices`: block devices for rootfs or volumes
- `interfaces`: configuration of network interfaces
- `routes`: 
  - `dest` may be a net or host description, such as `default`, `128.0.0.0/1`,
          `172.16.0.0`
  - `gateway` and `device` should provide at least one
- `socket`: the socket for connection with dvm daemon. 
- `share`
    
## Terminate Pod

    {
        "action": "terminate"
    }