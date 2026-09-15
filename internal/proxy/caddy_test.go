package proxy

import (
	"testing"

	"github.com/N3M1K/xrp/internal/config"
	"github.com/N3M1K/xrp/internal/scanner"
	"github.com/N3M1K/xrp/internal/ssl"
)

func testCfg() *config.Config {
	return &config.Config{
		TLD:         ".localhost",
		ProjectTLDs: map[string]string{},
		HTTPPort:    80,
		HTTPSPort:   443,
		CaddyPort:   2019,
	}
}

func TestGenerateConfigWithCerts(t *testing.T) {
	cfg := testCfg()
	procs := []scanner.Process{
		{Port: 3000, ProjectName: "myapp", Addr: "127.0.0.1"},
		{Port: 8096, ProjectName: "jelly", Addr: "::1"},
	}
	certs := []ssl.CertPair{{Cert: "/tmp/c.pem", Key: "/tmp/k.pem"}}

	c := GenerateConfig(procs, cfg, certs)

	if c.Admin == nil || c.Admin.Listen != "127.0.0.1:2019" {
		t.Fatalf("admin listen = %+v, want 127.0.0.1:2019", c.Admin)
	}

	https, ok := c.Apps.HTTP.Servers["xrp_https"]
	if !ok {
		t.Fatal("missing xrp_https server")
	}
	if len(https.TLSConnectionPolicies) != 1 {
		t.Error("expected one TLS connection policy")
	}
	if len(https.Routes) != 2 {
		t.Fatalf("expected 2 https routes, got %d", len(https.Routes))
	}

	dials := map[string]string{}
	for _, r := range https.Routes {
		dials[r.Match[0].Host[0]] = r.Handle[0].Upstreams[0].Dial
	}
	if dials["myapp.localhost"] != "127.0.0.1:3000" {
		t.Errorf("myapp dial = %q", dials["myapp.localhost"])
	}
	if dials["jelly.localhost"] != "[::1]:8096" {
		t.Errorf("jelly dial = %q", dials["jelly.localhost"])
	}

	httpSrv, ok := c.Apps.HTTP.Servers["xrp_http"]
	if !ok {
		t.Fatal("missing xrp_http server")
	}
	if len(httpSrv.Routes) != 1 {
		t.Fatalf("expected 1 redirect route, got %d", len(httpSrv.Routes))
	}
	if httpSrv.Routes[0].Handle[0].Handler != "static_response" {
		t.Error("http server should be a static_response redirect")
	}

	if c.Apps.TLS == nil || len(c.Apps.TLS.Certificates.LoadFiles) != 1 {
		t.Fatal("expected one loaded certificate")
	}
}

func TestGenerateConfigCustomTLD(t *testing.T) {
	cfg := testCfg()
	cfg.ProjectTLDs["jelly"] = "media"
	procs := []scanner.Process{{Port: 8096, ProjectName: "jelly", Addr: "127.0.0.1"}}

	c := GenerateConfig(procs, cfg, []ssl.CertPair{{Cert: "c", Key: "k"}})
	https := c.Apps.HTTP.Servers["xrp_https"]
	if host := https.Routes[0].Match[0].Host[0]; host != "jelly.media" {
		t.Errorf("host = %q, want jelly.media", host)
	}
}

func TestGenerateConfigDedupesHosts(t *testing.T) {
	cfg := testCfg()
	procs := []scanner.Process{
		{Port: 3000, ProjectName: "myapp", Addr: "127.0.0.1"},
		{Port: 3001, ProjectName: "myapp", Addr: "127.0.0.1"},
	}
	c := GenerateConfig(procs, cfg, []ssl.CertPair{{Cert: "c", Key: "k"}})
	https := c.Apps.HTTP.Servers["xrp_https"]
	if len(https.Routes) != 1 {
		t.Fatalf("expected duplicate hosts to collapse to 1 route, got %d", len(https.Routes))
	}
	if got := https.Routes[0].Handle[0].Upstreams[0].Dial; got != "127.0.0.1:3000" {
		t.Errorf("first route should win: dial = %q", got)
	}
}

func TestGenerateConfigWithoutCertsServesHTTP(t *testing.T) {
	cfg := testCfg()
	procs := []scanner.Process{{Port: 3000, ProjectName: "myapp", Addr: "127.0.0.1"}}

	c := GenerateConfig(procs, cfg, nil)

	if _, ok := c.Apps.HTTP.Servers["xrp_https"]; ok {
		t.Error("https server should not exist without certs")
	}
	httpSrv, ok := c.Apps.HTTP.Servers["xrp_http"]
	if !ok {
		t.Fatal("missing xrp_http server")
	}
	if len(httpSrv.Routes) != 1 || httpSrv.Routes[0].Handle[0].Handler != "reverse_proxy" {
		t.Error("expected plain HTTP reverse_proxy routes without certs")
	}
	if c.Apps.TLS != nil {
		t.Error("tls app should be omitted without certs")
	}
}
