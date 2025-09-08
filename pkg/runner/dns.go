package runner

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type CustomResolver struct {
	resolvers    []string
	timeout      time.Duration
	lastResolver string
}

// NewCustomResolver creates DNS resolver
func NewCustomResolver(resolvers []string, timeout time.Duration) *CustomResolver {
	return &CustomResolver{
		resolvers: resolvers,
		timeout:   timeout,
	}
}

// ResolutionResult holds the result of a DNS resolution attempt
type ResolutionResult struct {
	IPs      []net.IP
	Resolver string
	Error    error
}

func (r *CustomResolver) ResolveHost(ctx context.Context, host string) ResolutionResult {
	if len(r.resolvers) == 0 {
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			r.lastResolver = "system"
			return ResolutionResult{Error: err, Resolver: "system"}
		}

		var netIPs []net.IP
		for _, ip := range ips {
			netIPs = append(netIPs, ip.IP)
		}
		r.lastResolver = "system"
		return ResolutionResult{IPs: netIPs, Resolver: "system"}
	}

	for _, resolver := range r.resolvers {
		result := r.tryResolver(ctx, host, resolver)
		if result.Error == nil {
			result.Resolver = resolver
			r.lastResolver = resolver
			return result
		}
		Log.Debugf("DNS resolution failed with resolver %s: %v", resolver, result.Error)
	}

	// fallback
	Log.Debugf("All custom resolvers failed, trying system DNS")
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		r.lastResolver = "system"
		return ResolutionResult{Error: fmt.Errorf("all resolvers failed, last error: %v", err), Resolver: "system"}
	}

	var netIPs []net.IP
	for _, ip := range ips {
		netIPs = append(netIPs, ip.IP)
	}
	r.lastResolver = "system"
	return ResolutionResult{IPs: netIPs, Resolver: "system"}
}

func (r *CustomResolver) tryResolver(ctx context.Context, host, resolver string) ResolutionResult {
	if !strings.Contains(resolver, ":") {
		resolver = resolver + ":53"
	}

	client := &dns.Client{
		Timeout: r.timeout,
	}

	msg := &dns.Msg{}
	msg.SetQuestion(dns.Fqdn(host), dns.TypeA)
	msg.RecursionDesired = true

	response, _, err := client.ExchangeContext(ctx, msg, resolver)
	if err != nil {
		return ResolutionResult{Error: fmt.Errorf("DNS query failed: %v", err)}
	}

	if response.Rcode != dns.RcodeSuccess {
		return ResolutionResult{Error: fmt.Errorf("DNS query returned error code: %d", response.Rcode)}
	}

	var ips []net.IP
	for _, answer := range response.Answer {
		if a, ok := answer.(*dns.A); ok {
			ips = append(ips, a.A)
		}
	}

	if len(ips) == 0 {
		return ResolutionResult{Error: fmt.Errorf("no A records found")}
	}

	return ResolutionResult{IPs: ips}
}

func (r *CustomResolver) CustomDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	if net.ParseIP(host) != nil {
		return (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext(ctx, network, address)
	}

	result := r.ResolveHost(ctx, host)
	if result.Error != nil {
		return nil, result.Error
	}

	for _, ip := range result.IPs {
		addr := net.JoinHostPort(ip.String(), port)
		conn, err := (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext(ctx, network, addr)

		if err == nil {
			Log.Debugf("Successfully connected to %s via resolver %s (resolved to %s)", host, result.Resolver, ip.String())
			r.lastResolver = result.Resolver
			return conn, nil
		}
		Log.Debugf("Failed to connect to %s: %v", addr, err)
	}

	return nil, fmt.Errorf("failed to connect to any resolved IP for %s", host)
}

// GetLastResolver returns last used resolver
func (r *CustomResolver) GetLastResolver() string {
	return r.lastResolver
}
