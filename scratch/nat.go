package main

import (
	"github.com/NebulousLabs/go-upnp"
	"github.com/jackpal/gateway"
	"github.com/jackpal/go-nat-pmp"
	"log"
	"net"
)

func upnpDiscover() (*upnp.IGD, error) {
	client, err := upnp.Discover()
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

func pmpOpen(client *natpmp.Client, port int) error {
	_, err := client.AddPortMapping("udp", port, port, 30 * 24 * 60 * 60)
	if err != nil {
		log.Println(err)
		return err
	}
    return nil
}

func pmpClose(client *natpmp.Client, port int) error {
	_, err := client.AddPortMapping("udp", port, 0, 0)
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

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ip, err := getOutboundIP()
	if err != nil {
		log.Fatal("error getting outbound ip", err)
	}

	var extIP net.IP
	if isPrivateIP(ip) {
		var upnpClient *upnp.IGD
		var pmpClient *natpmp.Client
		log.Println("private ip detected")
		log.Println("trying upnp...")
		
		upnpClient, err := upnpDiscover()
		if err != nil {
			log.Println("error with upnp", err)
			goto pmp
		}

		extIP, err = upnpExternalIP(upnpClient)
		if err != nil {
			log.Println("error with upnp", err)
			goto pmp
		}

		err = upnpOpen(upnpClient, 0xdab)
		if err != nil {
			log.Println("error with upnp", err)
			goto pmp
		}

		defer upnpClose(upnpClient, 0xdab)
		goto natSuccess
pmp:
		log.Println("trying pmp...")
		// untested!!!
		pmpClient, err = pmpDiscover()
		if err != nil {
			log.Println("error with pmp", err)
			goto natFail
		}

		extIP, err = pmpExternalIP(pmpClient)
		if err != nil {
			log.Println("error with pmp", err)
			goto natFail
		}
		
		err = pmpOpen(pmpClient, 0xdab)
		if err != nil {
			log.Println("error with pmp", err)
			goto natFail
		}
		
		defer pmpClose(pmpClient, 0xdab)
		goto natSuccess
natFail:
		log.Println("could not map port with upnp or pmp")
		log.Fatal("goodbye")
natSuccess:
	}

	log.Println(extIP)
}
