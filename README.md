# growthbook-controller - Managing growthbook resources

[![release](https://img.shields.io/github/release/DoodleScheduling/growthbook-controller/all.svg)](https://github.com/DoodleScheduling/growthbook-controller/releases)
[![release](https://github.com/doodlescheduling/growthbook-controller/actions/workflows/release.yaml/badge.svg)](https://github.com/doodlescheduling/growthbook-controller/actions/workflows/release.yaml)
[![report](https://goreportcard.com/badge/github.com/DoodleScheduling/growthbook-controller)](https://goreportcard.com/report/github.com/DoodleScheduling/growthbook-controller)
[![Coverage Status](https://coveralls.io/repos/github/DoodleScheduling/growthbook-controller/badge.svg?branch=master)](https://coveralls.io/github/DoodleScheduling/growthbook-controller?branch=master)
[![license](https://img.shields.io/github/license/DoodleScheduling/growthbook-controller.svg)](https://github.com/DoodleScheduling/growthbook-controller/blob/master/LICENSE)

Kubernetes controller for managing growthbook.

Currently supported are `GrowthbookOrganization`, `GrowthbookUser`, `GrowthbookFeature`, `GrowthbookClient` and `GrowthbookInstance` while the later one is the main resource
referencing all other resources while organizations select further resources including clients, features and users (organization membership).
Basically for deploying features and clients a `GrowthbookInstance` as well as at least one `GrowthbookOrganization` resource needs to be created.

This controller does not deploy growthbook itself. It manages resources for an existing growthbook instance.
Growthbook currently does not support managing features nor clients within the scope of the rest api. This controller 
bypasses their api and manages the resources on MongoDB directly.

## Resource relationship

![graph](https://github.com/DoodleScheduling/growthbook-controller/blob/master/docs/resource-relationship.jpg?raw=true)

## Example Usage

The following manifests configure:
* A reference to an existing growthbook instance (mongodb)
* A growthbook org called `my-org`
* An admin user called `admin` and assigned to `my-org` as admin with the password `password`
* Two features assigned to the org `my-org`
* A Client SDK connection assigned to the org `my-org` with the token `token`

```yaml
apiVersion: growthbook.infra.doodle.com/v1beta1
kind: GrowthbookInstance
metadata:
  name: my-instance
  namespace: growthbook
spec:
  interval: 5m
  timeout: 1m
  suspend: false
  mongodb:
    uri: mongo://mongodb:27017
    rootSecret:
      name: growthbook-mongodb-credentials
  resourceSelector:
    matchLabels:
      growthbook-instance: my-instance
---
apiVersion: v1
kind: Secret
metadata:
  name: growthbook-mongodb-credentials
  namespace: growthbook
data:
  username: dXNlcg==
  password: cGFzc3dvcmQ=
---
apiVersion: growthbook.infra.doodle.com/v1beta1
kind: GrowthbookOrganization
metadata:
  name: my-org
  labels:
    growthbook-instance: my-instance
spec:
  ownerEmail: owner@myorg.com
  users:
  - role: admin
    selector:
      matchLabels: 
        growthbook-org: my-org
        growthbook-admin: "yes"
  resourceSelector:
    matchLabels: 
      growthbook-org: my-org
---
apiVersion: growthbook.infra.doodle.com/v1beta1
kind: GrowthbookUser
metadata:
  name: admin
  labels:
    growthbook-org: my-org
    growthbook-admin: "yes"
    growthbook-instance: my-instance
spec:
  email: admin@myorg.com
  secret:
    name: growthbook-admin
---
apiVersion: v1
kind: Secret
metadata:
  name: growthbook-admin
data:
  username: cm9vdA== #root
  password: cGFzc3dvcmQ= #password
---
apiVersion: growthbook.infra.doodle.com/v1beta1
kind: GrowthbookFeature
metadata:
  name: feature-a
  labels:
    growthbook-org: my-org
    growthbook-instance: my-instance
  namespace: growthbook
spec:
  description: feature A
  defaultValue: "true"
  valueType: boolean
  tags:
  - frontend
  environments:
  - name: "production"
    enabled: true
---
apiVersion: growthbook.infra.doodle.com/v1beta1
kind: GrowthbookFeature
metadata:
  name: feature-b
  labels:
    growthbook-org: my-org
    growthbook-instance: my-instance
  namespace: growthbook
spec:
  description: feature B
  defaultValue: "false"
  valueType: boolean
  tags:
  - frontend
  environments:
  - name: "production"
    enabled: true
---
apiVersion: growthbook.infra.doodle.com/v1beta1
kind: GrowthbookClient
metadata:
  name: client-1
  labels:
    growthbook-org: my-org
    growthbook-instance: my-instance
  namespace: growthbook
spec:
  environment: production
  tokenSecret:
    name: growthbook-client-1-token
---
apiVersion: v1
kind: Secret
metadata:
  name: growthbook-client-1-token
  namespace: growthbook
data:
  token: cGFzc3dvcmQ=
```

## Setup

### Helm chart

Please see [chart/growthbook-controller](https://github.com/DoodleScheduling/growthbook-controller) for the helm chart docs.

### Manifests/kustomize

Alternatively you may get the bundled manifests in each release to deploy it using kustomize or use them directly.

## Configure the controller

The controller can be configured by cmd args:
```
--concurrent int                            The number of concurrent Pod reconciles. (default 4)
--enable-leader-election                    Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
--graceful-shutdown-timeout duration        The duration given to the reconciler to finish before forcibly stopping. (default 10m0s)
--health-addr string                        The address the health endpoint binds to. (default ":9557")
--insecure-kubeconfig-exec                  Allow use of the user.exec section in kubeconfigs provided for remote apply.
--insecure-kubeconfig-tls                   Allow that kubeconfigs provided for remote apply can disable TLS verification.
--kube-api-burst int                        The maximum burst queries-per-second of requests sent to the Kubernetes API. (default 300)
--kube-api-qps float32                      The maximum queries-per-second of requests sent to the Kubernetes API. (default 50)
--leader-election-lease-duration duration   Interval at which non-leader candidates will wait to force acquire leadership (duration string). (default 35s)
--leader-election-release-on-cancel         Defines if the leader should step down voluntarily on controller manager shutdown. (default true)
--leader-election-renew-deadline duration   Duration that the leading controller manager will retry refreshing leadership before giving up (duration string). (default 30s)
--leader-election-retry-period duration     Duration the LeaderElector clients should wait between tries of actions (duration string). (default 5s)
--log-encoding string                       Log encoding format. Can be 'json' or 'console'. (default "json")
--log-level string                          Log verbosity level. Can be one of 'trace', 'debug', 'info', 'error'. (default "info")
--max-retry-delay duration                  The maximum amount of time for which an object being reconciled will have to wait before a retry. (default 15m0s)
--metrics-addr string                       The address the metric endpoint binds to. (default ":9556")
--min-retry-delay duration                  The minimum amount of time for which an object being reconciled will have to wait before a retry. (default 750ms)
--watch-all-namespaces                      Watch for resources in all namespaces, if set to false it will only watch the runtime namespace. (default true)
--watch-label-selector string               Watch for resources with matching labels e.g. 'sharding.fluxcd.io/shard=shard1'.
```