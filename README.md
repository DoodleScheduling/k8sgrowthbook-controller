# k8sgrowthbook-controller - Managing growthbook resources

[![release](https://img.shields.io/github/release/DoodleScheduling/k8sgrowthbook-controller/all.svg)](https://github.com/DoodleScheduling/k8sgrowthbook-controller/releases)
[![release](https://github.com/doodlescheduling/k8sgrowthbook-controller/actions/workflows/release.yaml/badge.svg)](https://github.com/doodlescheduling/k8sgrowthbook-controller/actions/workflows/release.yaml)
[![report](https://goreportcard.com/badge/github.com/DoodleScheduling/k8sgrowthbook-controller)](https://goreportcard.com/report/github.com/DoodleScheduling/k8sgrowthbook-controller)
[![Coverage Status](https://coveralls.io/repos/github/DoodleScheduling/k8sgrowthbook-controller/badge.svg?branch=master)](https://coveralls.io/github/DoodleScheduling/k8sgrowthbook-controller?branch=master)
[![license](https://img.shields.io/github/license/DoodleScheduling/k8sgrowthbook-controller.svg)](https://github.com/DoodleScheduling/k8sgrowthbook-controller/blob/master/LICENSE)

Kubernetes controller for managing growthbook.

Currently supported are `GrowthbookOrganization`, `GrowthbookUser`, `GrowthbookFeature`, `GrowthbookClient` and `GrowthbookInstance` while the later one is the main resource
referencing all other resources while organizations select further resources including clients, features and users (organization membership).
Basically for deploying features and clients a `GrowthbookInstance` as well as at least one `GrowthbooKOrganization` resource needs to be created.

This controller does not deploy growthbook itself. It manages resources for an existing growthbook instance.
Growthbook currently does not support managing features nor clients within the scope of the rest api. This controller 
bypasses their api and manages the resources on MongoDB directly.

## Resource relationship

![graph](https://github.com/DoodleScheduling/k8sgrowthbook-controller/blob/master/docs/resource-relationship.jpg.jpg?raw=true)

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
  description: feature B
  environment: production
  tags:
  - frontend
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

Please see [chart/k8sgrowthbook-controller](https://github.com/DoodleScheduling/k8sgrowthbook-controller) for the helm chart docs.

### Manifests/kustomize

Alternatively you may get the bundled manifests in each release to deploy it using kustomize or use them directly.

## Configure the controller

You may change base settings for the controller using env variables (or alternatively command line arguments).
Available env variables:

| Name  | Description | Default |
|-------|-------------| --------|
| `METRICS_ADDR` | The address of the metric endpoint binds to. | `:9556` |
| `PROBE_ADDR` | The address of the probe endpoints binds to. | `:9557` |
| `ENABLE_LEADER_ELECTION` | Enable leader election for controller manager. | `false` |
| `LEADER_ELECTION_NAMESPACE` | Change the leader election namespace. This is by default the same where the controller is deployed. | `` |
| `NAMESPACES` | The controller listens by default for all namespaces. This may be limited to a comma delimted list of dedicated namespaces. | `` |
| `CONCURRENT` | The number of concurrent reconcile workers.  | `2` |