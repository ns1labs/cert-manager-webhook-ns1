# ACME webhook for NS1 DNS API

This solver can be used when you want to use cert-manager with NS1 API. API documentation is [here](https://ns1.com/api)

## Requirements
-   [go](https://golang.org/) >= 1.16.0
-   [helm](https://helm.sh/) >= v3.0.0
-   [kubernetes](https://kubernetes.io/) >= v1.21.0
-   [cert-manager](https://cert-manager.io/) >= v1.6.1

## Installation

#### 1 - Log in on ns1.com and obtain an api secret, so create a k8s secret with index api-key on cert-manager namespace, such as:

```bash
kubectl create secret generic ns1-api-secret --from-literal=api-key='xxxxxxx' -n cert-manager
```
#### 2 - Install cert-manager-webhook-ns1 from local checkout
#### INSTALL:
```bash
helm install --namespace cert-manager cert-manager-webhook-ns1 deploy/ns1-webhook/ --set groupName=acme.mydomain.com
```
#### UNINSTALL:
```bash
helm uninstall --namespace cert-manager cert-manager-webhook-ns1 deploy/ns1-webhook/
```

**Note**: The kubernetes resources used to install the Webhook should be deployed within the same namespace as the cert-manager.
#### From local checkout

#### 3 - Add NS1 ClusterIssuer into k8s cluster
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-ns1
spec:
  acme:
    # The ACME server URL
    server: https://acme-v02.api.letsencrypt.org/directory # production server, change to staging for tests

    # Email address used for ACME registration
    email: myemail@mydomain.com # REPLACE THIS WITH YOUR EMAIL!!!

    # Name of a secret used to store the ACME account private key
    privateKeySecretRef:
      name: letsencrypt-ns1

    solvers:
      - dns01:
          webhook:            
            groupName: acme.mydomain.com
            solverName: ns1
            config:
              apiKeySecretRef: ns1-api-secret
              zoneName: mydomain.com
```

#### 4 - Add wildcard certificate for domains
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: cert-tls
  namespace: orb-live
spec:
  dnsNames:
    - '*.mydomain.com'
    - mydomain.com
  issuerRef:
    name: letsencrypt-ns1
    kind: ClusterIssuer
  secretName: cert-tls
```

#### 5 - If necessary, add redirect to one domain to the base domain
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/permanent-redirect: "https://mydomain.com/"
  name: endpoint-redirect
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - app.mydomain.com
    secretName: cert-tls
  rules:
  - host: app.mydomain.com
```
