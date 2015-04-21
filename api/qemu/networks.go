package qemu

import (
    "dvm/api/network"
    "dvm/lib/glog"
    "fmt"
    "net"
)

func CreateInterface(index int, pciAddr int, name string, isDefault bool, callback chan QemuEvent) {
    inf, err := network.Allocate("")
    if err != nil {
        glog.Error("interface creating failed", err.Error())
        callback <- &InterfaceCreated{
            Index:      index,
            PCIAddr:    pciAddr,
            DeviceName: name,
            Fd:         0,
            IpAddr:     "",
            NetMask:    "",
            RouteTable: nil,
        }
        return
    }

    interfaceGot(index, pciAddr, name, isDefault, callback, inf)
}

func interfaceGot(index int, pciAddr int, name string, isDefault bool, callback chan QemuEvent, inf *network.Settings) {

    ip,nw,err := net.ParseCIDR(fmt.Sprintf("%s/%d", inf.IPAddress, inf.IPPrefixLen))
    if err != nil {
        glog.Error("can not parse cidr")
        callback <- &InterfaceCreated{
            Index:      index,
            PCIAddr:    pciAddr,
            DeviceName: name,
            Fd:         0,
            IpAddr:     "",
            NetMask:    "",
            RouteTable: nil,
        }
        return
    }
    var tmp []byte = nw.Mask
    var mask net.IP = tmp

    rt:=[]*RouteRule{
        //        &RouteRule{
        //            Destination: fmt.Sprintf("%s/%d", nw.IP.String(), inf.IPPrefixLen),
        //            Gateway:"", ViaThis:true,
        //        },
    }
    if isDefault {
        rt = append(rt, &RouteRule{
            Destination: "0.0.0.0/0",
            Gateway: inf.Gateway, ViaThis: true,
        })
    }

    event := &InterfaceCreated{
        Index:      index,
        PCIAddr:    pciAddr,
        DeviceName: name,
        Fd:         uint64(inf.File.Fd()),
        IpAddr:     ip.String(),
        NetMask:    mask.String(),
        RouteTable: rt,
    }

    callback <- event
}
