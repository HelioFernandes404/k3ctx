package infrastructure

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/systemframe/k3ctx/internal/domain"
)

type httpDoer func(url string, perReqTimeout time.Duration, tlsConfig *tls.Config) (int, error)

func defaultPollAPIReady(localPort int, kubeconfigText string, timeout, interval time.Duration) error {
	tlsConfig := buildTLSConfig(kubeconfigText)
	return pollAPIReadyWithDoer(localPort, timeout, interval, tlsConfig, defaultHTTPDoer)
}

func pollAPIReadyWithDoer(localPort int, timeout, interval time.Duration, tlsConfig *tls.Config, doer httpDoer) error {
	url := fmt.Sprintf("https://127.0.0.1:%d/version", localPort)
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		perReq := interval
		if perReq < 2*time.Second {
			perReq = 2 * time.Second
		}
		if perReq > remaining {
			perReq = remaining
		}
		if perReq <= 0 {
			break
		}

		code, err := doer(url, perReq, tlsConfig)
		if err == nil && code >= 200 && code < 500 {
			return nil
		}
		lastErr = err

		sleep := interval
		if rem := time.Until(deadline); sleep > rem {
			sleep = rem
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
	}

	detail := ""
	if lastErr != nil {
		detail = lastErr.Error()
	}
	return &domain.OperationError{
		Code:      "kubernetes_api_unreachable",
		Message:   fmt.Sprintf("Kubernetes API did not become ready on %s", url),
		Detail:    detail,
		Retryable: true,
	}
}

func defaultHTTPDoer(url string, timeout time.Duration, tlsConfig *tls.Config) (int, error) {
	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

// buildTLSConfig extracts TLS material from a kubeconfig YAML and creates a tls.Config.
// Falls back to InsecureSkipVerify when the kubeconfig has no usable certs.
func buildTLSConfig(kubeconfigText string) *tls.Config {
	tlsCfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec

	var kube map[string]any
	if err := yaml.Unmarshal([]byte(kubeconfigText), &kube); err != nil {
		return tlsCfg
	}

	clusters, _ := kube["clusters"].([]any)
	if len(clusters) > 0 {
		cluster, _ := clusters[0].(map[string]any)
		clusterData, _ := cluster["cluster"].(map[string]any)
		if caData, ok := clusterData["certificate-authority-data"].(string); ok && caData != "" {
			caDER, err := base64.StdEncoding.DecodeString(caData)
			if err == nil {
				pool := x509.NewCertPool()
				pool.AppendCertsFromPEM(caDER)
				tlsCfg.RootCAs = pool
				tlsCfg.InsecureSkipVerify = false
			}
		}
	}

	users, _ := kube["users"].([]any)
	if len(users) > 0 {
		user, _ := users[0].(map[string]any)
		userData, _ := user["user"].(map[string]any)
		certData, hasCert := userData["client-certificate-data"].(string)
		keyData, hasKey := userData["client-key-data"].(string)
		if hasCert && hasKey {
			certPEM, err1 := base64.StdEncoding.DecodeString(certData)
			keyPEM, err2 := base64.StdEncoding.DecodeString(keyData)
			if err1 == nil && err2 == nil {
				if pair, err := tls.X509KeyPair(certPEM, keyPEM); err == nil {
					tlsCfg.Certificates = []tls.Certificate{pair}
				}
			}
		}
	}

	return tlsCfg
}
