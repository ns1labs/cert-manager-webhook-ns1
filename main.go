package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"k8s.io/klog"

	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	ctx := context.Background()
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}
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
	klog.V(6).Infof("call function Present: namespace=%s, zone=%s, fqdn=%s", ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	cfg, err := c.clientConfig(c.ctx, ch)
	if err != nil {
		return err
	}
	// Create TXT record
	c.createTxtRecord(cfg, ch)

	return nil
}

func (c *ns1DNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	klog.V(6).Infof("call function CleanUp: namespace=%s, zone=%s, fqdn=%s", ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	cfg, err := c.clientConfig(c.ctx, ch)
	if err != nil {
		return err
	}

	// Delete TXT record
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
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (ns1DNSProviderConfig, error) {
	cfg := ns1DNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
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
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return cfg, err
	}

	sec, err := c.client.CoreV1().Secrets(ch.ResourceNamespace).Get(ctx, cfg.ApiKeySecretRef, metav1.GetOptions{})
	if err != nil {
		return cfg, fmt.Errorf("unable to get secret `%s/%s`; %v", cfg.ApiKeySecretRef, ch.ResourceNamespace, err)
	}
	apiKey, err := stringFromSecretData(&sec.Data, "api-key")
	cfg.ApiKey = apiKey
	if err != nil {
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

	// check if exists
	_, _, err := c.ns1api.Records.Get(config.ZoneName, recName, "TXT")
	if err != nil {
		newRecord := dns.NewRecord(config.ZoneName, recName, "TXT")
		newRecord.AddAnswer(dns.NewTXTAnswer(ch.Key))
		newRecord.TTL = 3600

		//create record
		_, err := c.ns1api.Records.Create(newRecord)
		if err != nil {
			klog.Errorf("unable to create txt record zone=`%s` recordName=`%s`; %v", config.ZoneName, recName, err)
		}

		klog.Infof("Added TXT record result: %s", string(ch.ResolvedFQDN))
	} else {
		// record already created
		klog.Infof("Already Added TXT record result: %s", string(ch.ResolvedFQDN))
	}
}

// Delete record
func (c *ns1DNSProviderSolver) deleteTxtRecord(config ns1DNSProviderConfig, ch *v1alpha1.ChallengeRequest) {
	recName := getRecordName(ch.ResolvedFQDN)

	// delete record
	_, err := c.ns1api.Records.Delete(config.ZoneName, recName, "TXT")
	if err != nil {
		klog.Errorf("unable to delete txt record zone=`%s` recordName=`%s`; %v", config.ZoneName, recName, err)
	}

	klog.Infof("Deleted TXT record result: %s", string(ch.ResolvedFQDN))
}
