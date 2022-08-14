package main

import (
	"fmt"
	"log"
	"net"

	"github.com/miekg/dns"
)

var override_dns = map[string]bool{
	"geo.hivebedrock.network.": true,
}

func answerQuery(remote net.Addr, req *dns.Msg) (reply *dns.Msg) {
	reply = new(dns.Msg)

	answered := false
	for _, q := range req.Question {
		switch q.Qtype {
		case dns.TypeA:
			log.Printf("Query for %s\n", q.Name)

			if override_dns[q.Name] {
				host, _, _ := net.SplitHostPort(remote.String())
				remote_ip := net.ParseIP(host)

				addrs, _ := net.InterfaceAddrs()
				var ip string
				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.Contains(remote_ip) {
							ip = ipnet.IP.String()
						}
					}
				}
				if ip == "" {
					log.Panicf("query from outside of own network??")
				}

				rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
				if err == nil {
					reply.Answer = append(reply.Answer, rr)
					answered = true
				}
			}
		}
	}

	// forward to 1.1.1.1 if not intercepted
	if !answered {
		c, err := dns.DialWithTLS("tcp4", "1.1.1.1:853", nil)
		if err != nil {
			panic(err)
		}
		if err = c.WriteMsg(req); err != nil {
			panic(err)
		}
		if reply, err = c.ReadMsg(); err != nil {
			panic(err)
		}
		c.Close()
	}

	return reply
}

func dns_handler(w dns.ResponseWriter, req *dns.Msg) {
	var reply *dns.Msg

	switch req.Opcode {
	case dns.OpcodeQuery:
		reply = answerQuery(w.RemoteAddr(), req)
	default:
		reply = new(dns.Msg)
	}

	reply.SetReply(req)
	w.WriteMsg(reply)
}

func init_dns() {
	dns.HandleFunc(".", dns_handler)

	server := &dns.Server{Addr: ":53", Net: "udp"}
	go func() {
		fmt.Printf("Starting dns at %s:53\n", GetLocalIP())
		err := server.ListenAndServe()
		defer server.Shutdown()
		if err != nil {
			log.Printf("Failed to start dns server: %s\n ", err.Error())
			println("you may have to use bedrockconnect")
		}
	}()
}
