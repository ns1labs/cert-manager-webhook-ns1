package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	api "gopkg.in/ns1/ns1-go.v2/rest"
	"gopkg.in/ns1/ns1-go.v2/rest/model/dns"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	ctx := context.Background()
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}
	log.Println("Starting cert-manager webhook NS1")
	cmd.RunWebhookServer(GroupName,
		&ns1DNSProviderSolver{ctx: ctx},
	)
}

type ns1DNSProviderSolver struct {
	client *kubernetes.Clientset
	ctx    context.Context
	ns1api *api.Client
}

type ns1DNSProviderConfig struct {
	ZoneName        string `json:"zoneName"`
	ApiUrl          string `json:"apiUrl"`
	ApiKeySecretRef string `json:"apiKeySecretRef"`
	ApiKey          string `json:"apiKey"`
}

func (c *ns1DNSProviderSolver) Name() string {
	return "ns1"
}

func (c *ns1DNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	log.Println("Call function Present: namespace=", ch.ResourceNamespace, " zone=", ch.ResolvedZone, " fqdn=", ch.ResolvedFQDN)

	cfg, err := c.clientConfig(c.ctx, ch)
	if err != nil {
		return err
	}

	c.createTxtRecord(cfg, ch)

	return nil
}

func (c *ns1DNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	log.Println("Call function CleanUp: namespace=", ch.ResourceNamespace, " zone=", ch.ResolvedZone, " fqdn=", ch.ResolvedFQDN)

	cfg, err := c.clientConfig(c.ctx, ch)
	if err != nil {
		return err
	}

	c.deleteTxtRecord(cfg, ch)

	return nil
}

func (c *ns1DNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	k8sClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.client = k8sClient

	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
func loadConfig(cfgJSON *extapi.JSON) (ns1DNSProviderConfig, error) {
	cfg := ns1DNSProviderConfig{}
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}
	return cfg, nil
}

func stringFromSecretData(secretData *map[string][]byte, key string) (string, error) {
	data, ok := (*secretData)[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret data", key)
	}
	return string(data), nil
}

func getRecordName(fqdn string) string {
	// Both ch.ResolvedZone and ch.ResolvedFQDN end with a dot: '.'
	recordName := strings.TrimSuffix(fqdn, ".")
	return recordName
}

func (c *ns1DNSProviderSolver) clientConfig(ctx context.Context, ch *v1alpha1.ChallengeRequest) (ns1DNSProviderConfig, error) {
	cfg, err := loadConfig((*extapi.JSON)(ch.Config))
	if err != nil {
		return cfg, err
	}

	sec, err := c.client.CoreV1().Secrets(ch.ResourceNamespace).Get(ctx, cfg.ApiKeySecretRef, metav1.GetOptions{})
	if err != nil {
		log.Println("Unable to get secret " + cfg.ApiKeySecretRef + "/" + ch.ResourceNamespace)
		return cfg, fmt.Errorf("unable to get secret `%s/%s`; %v", cfg.ApiKeySecretRef, ch.ResourceNamespace, err)
	}
	apiKey, err := stringFromSecretData(&sec.Data, "api-key")
	cfg.ApiKey = apiKey
	if err != nil {
		log.Println("Unable to get api-key from secret ", cfg.ApiKeySecretRef, "/", ch.ResourceNamespace)
		return cfg, fmt.Errorf("unable to get api-key from secret `%s/%s`; %v", cfg.ApiKeySecretRef, ch.ResourceNamespace, err)
	}
	c.setNS1Client(cfg)
	return cfg, nil
}

func (c *ns1DNSProviderSolver) setNS1Client(config ns1DNSProviderConfig) {
	if c.ns1api == nil {
		httpClient := &http.Client{Timeout: time.Second * 10}
		c.ns1api = api.NewClient(httpClient, api.SetAPIKey(config.ApiKey))
	}
}

// Create record
func (c *ns1DNSProviderSolver) createTxtRecord(config ns1DNSProviderConfig, ch *v1alpha1.ChallengeRequest) {
	recName := getRecordName(ch.ResolvedFQDN)

	_, _, err := c.ns1api.Records.Get(config.ZoneName, recName, "TXT")
	if err != nil {
		newRecord := dns.NewRecord(config.ZoneName, recName, "TXT")
		newRecord.AddAnswer(dns.NewTXTAnswer(ch.Key))
		newRecord.TTL = 3600

		_, err := c.ns1api.Records.Create(newRecord)
		if err != nil {
			log.Println("Unable to create txt record zone=", config.ZoneName, " recordName=", recName, err)
		}

		log.Println("Added TXT record result: ", string(recName))
	} else {
		log.Println("Already Added TXT record result: ", string(recName))
	}
}

// Delete record
func (c *ns1DNSProviderSolver) deleteTxtRecord(config ns1DNSProviderConfig, ch *v1alpha1.ChallengeRequest) {
	recName := getRecordName(ch.ResolvedFQDN)

	_, err := c.ns1api.Records.Delete(config.ZoneName, recName, "TXT")
	if err != nil {
		log.Println("Unable to delete txt record zone=", config.ZoneName, " recordName=", recName, err)
	}

	log.Println("Deleted TXT record result:", string(recName))
}
