package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path"
	"time"

	"github.com/jfardello/tdns/log"
)

func Generate384Cert(basapath, basename string, valid time.Duration, hosts []string) (certname string, keyname string) {
	logger := log.GetLogger("config", "generate384cert")

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		panic(err)
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(valid)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"tDNS self signed"},
			CommonName:   "tdns.kubewire.net",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	if err != nil {
		logger.Fatalf("Failed to generate serial number: %v", err)
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		logger.Fatalf("Failed to create certificate: %v", err)
	}
	certOut, err := os.OpenFile(path.Join(basapath, basename+"cert.pem"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		logger.Fatalf("Failed to open cert.pem for writing: %v", err)
	}

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		logger.Fatalf("Failed to write data to cert.pem: %v", err)
	}
	if err := certOut.Close(); err != nil {
		logger.Fatalf("Error closing cert.pem: %v", err)
	}
	logger.Infof("wrote %s\n", certOut.Name())

	keyOut, err := os.OpenFile(path.Join(basapath, basename+"key.pem"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		logger.Fatalf("Failed to open key.pem for writing: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		logger.Fatalf("Unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		logger.Fatalf("Failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		logger.Fatalf("Error closing key.pem: %v", err)
	}
	logger.Infof("wrote %s \n", keyOut.Name())

	return certOut.Name(), keyOut.Name()
}
