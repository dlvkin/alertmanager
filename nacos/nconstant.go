package nacos

import (
	"fmt"
	"net"
	"os"
	"sort"
)

/*
  @Author:hunterfox
  @Time: 2020/5/22 下午4:49

*/
// HomeDir home dir is work dir
const HomeDir string = "homedir"

// ModuleProductName programm product name
const ModuleProductName string = "altermanager"

//GetMatchLocalIP get local ipaddress by host
func GetMatchLocalIP(host string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return GetLocalIP()
	}
	addlist := make([]string, 0)
	addlist = append(addlist, GetLocalIP())
	for i := 0; i < len(ifaces); i++ {
		if (ifaces[i].Flags & net.FlagUp) != 0 {
			addrs, _ := ifaces[i].Addrs()
			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						findhost := ipnet.IP.String()
						if findhost == host {
							return host
						}
						addlist = append(addlist, findhost)
					}
				}
			}
		}
	}
	sort.Strings(addlist)
	return addlist[0]
}
//GetLocalIP getlocal ip
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				fmt.Println(ipnet.IP.String())
				return ipnet.IP.String()
			}
		}
	}
	return ""
}