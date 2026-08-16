package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/db"
	"github.com/miekg/dns"
)

func encodedDNSLogKey(value byte) string {
	return base64.StdEncoding.EncodeToString(bytesOf(value, dnsLogMinimumKeyBytes))
}

func bytesOf(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func TestDNSLogPseudonymizerCanonicalizesAndSeparatesContexts(t *testing.T) {
	key := bytesOf(0x42, dnsLogMinimumKeyBytes)
	pseudonymizer := &dnsLogPseudonymizer{key: key}

	domain := pseudonymizer.domain(" WWW.Example.COM ")
	if domain != pseudonymizer.domain("www.example.com.") {
		t.Fatal("equivalent domain names produced different pseudonyms")
	}

	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(dnsLogDomainContext))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte("www.example.com."))
	want := dnsLogDomainToken + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if domain != want {
		t.Fatalf("domain pseudonym = %q, want %q", domain, want)
	}

	domainContextToken := pseudonymizer.token(dnsLogDomainContext, dnsLogDomainToken, "1.1.1.1")
	client, err := pseudonymizer.client("1.1.1.1")
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if strings.TrimPrefix(domainContextToken, dnsLogDomainToken) == strings.TrimPrefix(client, dnsLogClientToken) {
		t.Fatal("domain and client contexts produced the same digest")
	}
	if !validDNSLogToken(client, dnsLogClientToken) {
		t.Fatalf("invalid client token: %q", client)
	}
}

func TestLoadDNSLogPseudonymizationKey(t *testing.T) {
	key := encodedDNSLogKey(0x31)

	t.Run("environment", func(t *testing.T) {
		t.Setenv("TDNS_TEST_DNSLOG_KEY", key)
		loaded, err := loadDNSLogPseudonymizationKey(config.DNSLogPseudonymizationConf{
			KeyEnvironment: "TDNS_TEST_DNSLOG_KEY",
			KeyFile:        filepath.Join(t.TempDir(), "missing"),
		})
		if err != nil {
			t.Fatalf("load key: %v", err)
		}
		if len(loaded) != dnsLogMinimumKeyBytes || loaded[0] != 0x31 {
			t.Fatalf("unexpected loaded key")
		}
	})

	t.Run("restricted file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "dnslog.key")
		if err := os.WriteFile(path, []byte(key+"\n"), 0o640); err != nil {
			t.Fatalf("write key: %v", err)
		}
		if _, err := loadDNSLogPseudonymizationKey(config.DNSLogPseudonymizationConf{KeyFile: path}); err != nil {
			t.Fatalf("load key: %v", err)
		}
	})

	t.Run("public file rejected", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "dnslog.key")
		if err := os.WriteFile(path, []byte(key), 0o644); err != nil {
			t.Fatalf("write key: %v", err)
		}
		if _, err := loadDNSLogPseudonymizationKey(config.DNSLogPseudonymizationConf{KeyFile: path}); err == nil {
			t.Fatal("load accepted a publicly readable key")
		}
	})

	t.Run("short key rejected", func(t *testing.T) {
		t.Setenv("TDNS_TEST_SHORT_DNSLOG_KEY", base64.StdEncoding.EncodeToString([]byte("short")))
		if _, err := loadDNSLogPseudonymizationKey(config.DNSLogPseudonymizationConf{
			KeyEnvironment: "TDNS_TEST_SHORT_DNSLOG_KEY",
		}); err == nil {
			t.Fatal("load accepted a short key")
		}
	})
}

func configuredPrivacyDNSLog(t *testing.T, connString, environment, encodedKey string, domains, clients bool) *DNSLog {
	t.Helper()
	t.Setenv(environment, encodedKey)
	cs := &DNSLog{}
	err := cs.Config(config.Config{
		Database: config.DatabaseConf{File: connString},
		DNSLog: config.DNSLogConf{
			Enabled: true,
			Pseudonymization: config.DNSLogPseudonymizationConf{
				Domains:        domains,
				Clients:        clients,
				KeyEnvironment: environment,
			},
		},
	})
	if err != nil {
		t.Fatalf("Config: %v", err)
	}
	t.Cleanup(cs.se.Close)
	return cs
}

func TestDNSLogPseudonymizesBeforeStorageAndPreservesClientOperations(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	cs := configuredPrivacyDNSLog(t, connString, "TDNS_TEST_STORAGE_KEY", encodedDNSLogKey(0x51), true, true)

	message := new(dns.Msg)
	message.SetQuestion("WWW.Example.COM.", dns.TypeA)
	event, err := cs.newLogEvent(time.Now().UTC().Add(-time.Minute), config.CtxValue{
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("2001:0db8::1"), Port: 53000},
		Values:     map[string]string{"blocked": "1"},
	}, message)
	if err != nil {
		t.Fatalf("newLogEvent: %v", err)
	}
	if !strings.HasPrefix(event.Domain, dnsLogDomainToken) || !strings.HasPrefix(event.Client, dnsLogClientToken) {
		t.Fatalf("event was not pseudonymized before queueing: %#v", event)
	}

	cs.queue = []LogEvent{event}
	cs.doInsert()
	if err := cs.AddAlias("office", "2001:db8::1"); err != nil {
		t.Fatalf("AddAlias: %v", err)
	}

	conn := cs.se.GetConn()
	var storedDomain, storedClient, aliasClient string
	if err := conn.QueryRow("SELECT domain, client FROM tdnslog").Scan(&storedDomain, &storedClient); err != nil {
		cs.se.FreeConn(conn)
		t.Fatalf("read DNS log: %v", err)
	}
	if err := conn.QueryRow("SELECT ipAddr FROM hosts WHERE host = 'office'").Scan(&aliasClient); err != nil {
		cs.se.FreeConn(conn)
		t.Fatalf("read alias: %v", err)
	}
	cs.se.FreeConn(conn)
	if storedDomain != event.Domain || storedClient != event.Client || aliasClient != event.Client {
		t.Fatalf("unexpected pseudonymized storage: domain=%q client=%q alias=%q", storedDomain, storedClient, aliasClient)
	}
	if strings.Contains(storedDomain, "example.com") || strings.Contains(storedClient, "2001:db8") || strings.Contains(aliasClient, "2001:db8") {
		t.Fatal("raw identifiers reached persistent storage")
	}

	top, err := cs.GetTop(10, "24h", "blocked", "2001:db8::1", "ip")
	if err != nil {
		t.Fatalf("GetTop: %v", err)
	}
	if len(top) != 1 || top[0].Domain != event.Domain || top[0].Host != "office" {
		t.Fatalf("unexpected filtered top result: %#v", top)
	}
	clients, err := cs.SearchClients("2001:db8::1", 10)
	if err != nil {
		t.Fatalf("SearchClients: %v", err)
	}
	if len(clients) != 1 || clients[0].Address != event.Client || clients[0].Host != "office" {
		t.Fatalf("unexpected client search result: %#v", clients)
	}
	clients, err = cs.SearchClients(event.Client, 10)
	if err != nil {
		t.Fatalf("SearchClients token: %v", err)
	}
	if len(clients) != 1 || clients[0].Address != event.Client {
		t.Fatalf("unexpected token search result: %#v", clients)
	}
}

func TestDNSLogPrivacyDetectsIncompatibleExistingData(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	plain := configuredPrivacyDNSLog(t, connString, "TDNS_UNUSED_PLAIN_KEY", encodedDNSLogKey(0x61), false, false)
	if _, err := plain.se.SyncExec(
		"INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, 0)",
		[]any{time.Now().UnixNano(), "1.1.1.1", "example.com."},
	); err != nil {
		t.Fatalf("insert plaintext data: %v", err)
	}

	pseudonymized := configuredPrivacyDNSLog(t, connString, "TDNS_TEST_MIGRATION_KEY", encodedDNSLogKey(0x62), true, true)
	status := pseudonymized.PrivacyStatus()
	if !status.RequiresClear || !strings.Contains(status.Reason, "clear DNS-log data") {
		t.Fatalf("incompatible data status = %#v", status)
	}
}

func TestDNSLogPrivacyDetectsKeyChange(t *testing.T) {
	connString := newTempConnString(t)
	if err := db.RunMigrations(context.Background(), connString, db.TargetDNSLog); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	first := configuredPrivacyDNSLog(t, connString, "TDNS_TEST_FIRST_KEY", encodedDNSLogKey(0x71), true, false)
	if _, err := first.se.SyncExec(
		"INSERT INTO tdnslog (dt, client, domain, blocked) VALUES (?, ?, ?, 0)",
		[]any{time.Now().UnixNano(), "1.1.1.1", first.pseudonymizer.domain("example.com.")},
	); err != nil {
		t.Fatalf("insert pseudonymized data: %v", err)
	}

	second := configuredPrivacyDNSLog(t, connString, "TDNS_TEST_SECOND_KEY", encodedDNSLogKey(0x72), true, false)
	if status := second.PrivacyStatus(); !status.RequiresClear {
		t.Fatalf("changed key was accepted with existing data: %#v", status)
	}
}
