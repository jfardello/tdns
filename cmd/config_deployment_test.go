package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/config"
	"gopkg.in/yaml.v3"
)

func TestGeneratedSystemdUnitIsHardened(t *testing.T) {
	unitPath := filepath.Join(t.TempDir(), "tdns.service")
	createUnit("/usr/local/bin/tdns", "/etc/tdns/tdns.yaml", unitPath)

	contents, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatalf("read generated unit: %v", err)
	}
	unit := string(contents)

	for _, expected := range []string{
		"ExecStart=/usr/local/bin/tdns serve -c /etc/tdns/tdns.yaml",
		"User=tdns",
		"Group=tdns",
		"UMask=0077",
		"StateDirectory=tdns",
		"AmbientCapabilities=CAP_NET_BIND_SERVICE",
		"CapabilityBoundingSet=CAP_NET_BIND_SERVICE",
		"NoNewPrivileges=true",
		"ProtectSystem=strict",
		"ReadOnlyPaths=/etc/tdns",
		"ReadWritePaths=/var/lib/tdns",
	} {
		if !strings.Contains(unit, expected) {
			t.Errorf("generated unit is missing %q", expected)
		}
	}
	if strings.Contains(unit, "User=root") {
		t.Fatal("generated unit runs TDNS as root")
	}

	info, err := os.Stat(unitPath)
	if err != nil {
		t.Fatalf("stat generated unit: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("unit mode = %04o, want 0644", got)
	}
}

func TestGeneratedConfigUsesSelectedDeploymentPaths(t *testing.T) {
	oldBasePath, oldDataPath := basepath, dataPath
	oldDNSListen, oldAPIListen := dnsListen, apiListen
	t.Cleanup(func() {
		basepath, dataPath = oldBasePath, oldDataPath
		dnsListen, apiListen = oldDNSListen, oldAPIListen
	})

	basepath = "/etc/tdns"
	dataPath = "/srv/tdns"
	dnsListen = ":8053"
	apiListen = ":8443"

	generated := newConf()
	if got, want := generated.Database.File, "/srv/tdns/tdns.sqlite"; got != want {
		t.Fatalf("database file = %q, want %q", got, want)
	}
	if got, want := generated.Blacklist.File, "/srv/tdns/bhole_list"; got != want {
		t.Fatalf("blacklist file = %q, want %q", got, want)
	}
	if got, want := generated.Server.ListenAddr, ":8053"; got != want {
		t.Fatalf("DNS listener = %q, want %q", got, want)
	}
	if got, want := generated.Server.APIAddr, ":8443"; got != want {
		t.Fatalf("API listener = %q, want %q", got, want)
	}
	if generated.Server.SwaggerEnabled {
		t.Fatal("generated configuration enables Swagger")
	}
	if generated.Status.ExposeStats || generated.Status.ExposeUptime {
		t.Fatal("generated configuration exposes status details")
	}
	if len(generated.DNSAccess.AllowedClientCIDRs) != 0 {
		t.Fatalf("generated DNS allowlist = %v, want loopback-only default", generated.DNSAccess.AllowedClientCIDRs)
	}
	if generated.DNSAccess.ClientQueriesPerSecond != 100 ||
		generated.DNSAccess.GlobalResponsesPerSecond != 1000 ||
		generated.DNSAccess.MaxConcurrentUpstreams != 128 {
		t.Fatalf("generated DNS access defaults are incomplete: %#v", generated.DNSAccess)
	}
}

func TestWriteSampleConfigProtectsCredentialsAndUsesThirtyDayToken(t *testing.T) {
	oldDestination, oldBasePath, oldDataPath := destination, basepath, dataPath
	oldDNSListen, oldAPIListen := dnsListen, apiListen
	t.Cleanup(func() {
		destination, basepath, dataPath = oldDestination, oldBasePath, oldDataPath
		dnsListen, apiListen = oldDNSListen, oldAPIListen
	})

	destination = t.TempDir()
	basepath = "/etc/tdns"
	dataPath = "/var/lib/tdns"
	dnsListen = "127.0.0.1:53"
	apiListen = "127.0.0.1:8443"

	before := time.Now()
	WriteSampleConfig("tdns.yaml", "/etc/tdns/tdns_cert.pem", "/etc/tdns/tdns_key.pem")

	configPath := filepath.Join(destination, "tdns.yaml")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat generated config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %04o, want 0600", got)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	var generated config.Config
	if err := yaml.Unmarshal(contents, &generated); err != nil {
		t.Fatalf("parse generated config: %v", err)
	}
	if generated.Server.SigningKey == "" {
		t.Fatal("generated config does not contain a persistent signing key")
	}

	token, _, err := new(jwt.Parser).ParseUnverified(generated.Client.Token, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("parse generated bootstrap token: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("token claims type = %T, want jwt.MapClaims", token.Claims)
	}
	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil {
		t.Fatalf("read token expiration: %v", err)
	}
	wantMin := before.Add(30*24*time.Hour - time.Minute)
	wantMax := time.Now().Add(30*24*time.Hour + time.Minute)
	if expiresAt.Before(wantMin) || expiresAt.After(wantMax) {
		t.Fatalf("token expiration = %s, want approximately 30 days", expiresAt)
	}
}

func TestDeploymentPathMapsRelativeOutputDirectory(t *testing.T) {
	got := deploymentPath("tdns-config/tdns_cert.pem", "./tdns-config", "/etc/tdns")
	if want := "/etc/tdns/tdns_cert.pem"; got != want {
		t.Fatalf("deploymentPath() = %q, want %q", got, want)
	}
}
