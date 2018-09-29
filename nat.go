package main

import (
	"context"
	"github.com/NebulousLabs/go-upnp"
	"github.com/jackpal/gateway"
	"github.com/jackpal/go-nat-pmp"
	"log"
	"net"
	"time"
)

func upnpDiscover() (*upnp.IGD, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := upnp.DiscoverCtx(ctx)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return client, nil
}

func upnpExternalIP(client *upnp.IGD) (net.IP, error) {
	ip, err := client.ExternalIP()
	if err != nil {
		log.Println(err)
		return net.ParseIP("0.0.0.0"), err
	}

	log.Printf("external ip: %s\n", ip)
	return net.ParseIP(ip), nil
}

func upnpOpen(client *upnp.IGD, port uint16) error {
	err := client.Forward(port, "party-line")
	if err != nil {
		log.Println(err)
		return err
	}

	log.Printf("port (0x%x) open\n", port)
	return nil
}

func upnpClose(client *upnp.IGD, port uint16) error {
	err := client.Clear(port)
	if err != nil {
		log.Println(err)
		return err
	}

	log.Printf("port (0x%x) closed\n", port)
	return nil
}

func pmpDiscover() (*natpmp.Client, error) {
	gatewayIP, err := gateway.DiscoverGateway()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	client := natpmp.NewClient(gatewayIP)
	return client, nil
}

func pmpExternalIP(client *natpmp.Client) (net.IP, error) {
	response, err := client.GetExternalAddress()
	if err != nil {
		log.Println(err)
		return net.ParseIP("0.0.0.0"), err
	}

	extBytes := response.ExternalIPAddress
	return net.IPv4(extBytes[0], extBytes[1], extBytes[2], extBytes[3]), nil
}

func pmpOpen(client *natpmp.Client, port uint16) error {
	_, err := client.AddPortMapping("udp", int(port), int(port), 30*24*60*60)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func pmpClose(client *natpmp.Client, port uint16) error {
	_, err := client.AddPortMapping("udp", int(port), 0, 0)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Println(err)
		return net.ParseIP("0.0.0.0"), err
	}

	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

func isPrivateIP(ip net.IP) bool {
	var privateIPBlocks []*net.IPNet
	privs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
	}

	for _, cidr := range privs {
		_, block, _ := net.ParseCIDR(cidr)
		privateIPBlocks = append(privateIPBlocks, block)
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

var closeUpnp bool = false
var closePmp bool = false
var openedPort uint16 = 0
var upnpClient *upnp.IGD
var pmpClient *natpmp.Client

func natCleanup() {
	if closeUpnp {
		upnpClose(upnpClient, openedPort)
	} else if closePmp {
		pmpClose(pmpClient, openedPort)
	}
}

func tryNatPMP(extIP *net.IP, port uint16, successChan chan bool) {
	log.Println("trying pmp...")
	log.Println("untested!!! let me know if it works")

	var err error
	pmpClient, err = pmpDiscover()
	if err != nil {
		log.Println("error with pmp", err)
		successChan <- false
	}

	*extIP, err = pmpExternalIP(pmpClient)
	if err != nil {
		log.Println("error with pmp", err)
		successChan <- false
	}

	err = pmpOpen(pmpClient, port)
	if err != nil {
		log.Println("error with pmp", err)
		successChan <- false
	}

	successChan <- true
}

func natStuff(port uint16) net.IP {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ip, err := getOutboundIP()
	if err != nil {
		log.Fatal("error getting outbound ip", err)
	}

	openedPort = port

	var extIP net.IP = ip
	var successChan chan bool
	if isPrivateIP(ip) {
		log.Println("private ip detected")
		log.Println("trying upnp...")

		upnpClient, err = upnpDiscover()
		if err != nil {
			log.Println("error with upnp (discover)", err)
			goto pmp
		}

		extIP, err = upnpExternalIP(upnpClient)
		if err != nil {
			log.Println("error with upnp (ext ip)", err)
			goto pmp
		}

		err = upnpOpen(upnpClient, port)
		if err != nil {
			log.Println("error with upnp (open)", err)
			goto pmp
		}

		closeUpnp = true
		goto natSuccess
	pmp:
		successChan = make(chan bool)
		go tryNatPMP(&extIP, port, successChan)
		select {
		case result := <-successChan:
			if result {
				closePmp = true
				goto natSuccess
			} else {
				goto natFail
			}
		case <-time.After(30 * time.Second):
			log.Println("nat pmp timed out :(")
			goto natFail
		}

		goto natSuccess
	natFail:
		log.Println("could not map port with upnp or pmp")
		log.Fatal("goodbye")
	natSuccess:
	}

	log.Println(extIP)
	return extIP
}
