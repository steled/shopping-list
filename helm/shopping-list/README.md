# shopping-list

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

Lightweight shopping list web application

**Homepage:** <https://github.com/steled/shopping-list>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| steled | <34886888+steled@users.noreply.github.com> |  |

## Source Code

* <https://github.com/steled/shopping-list>

## Requirements

Kubernetes: `>=1.29.0-0`

## Install

Before installing, generate the required secrets:

```bash
# bcrypt-hash for auth.password (requires htpasswd from apache2-utils / httpd-tools)
htpasswd -bnBC 12 "" 'your-password' | tr -d ':\n'

# 32-byte random secret for auth.sessionSecret
openssl rand -base64 32
```

```bash
helm install shopping-list oci://ghcr.io/steled/charts/shopping-list \
  --set auth.password='<bcrypt-hash>' \
  --set auth.sessionSecret='<random-base64>'
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity rules for pod scheduling |
| auth.existingSecret | string | `""` | Reference an existing Secret instead of creating one. Must contain keys: `username`, `password`, `session-secret` |
| auth.password | string | `""` | Login password (plain text; compared with bcrypt hash at runtime). Required when existingSecret is empty. |
| auth.sessionSecret | string | `""` | HMAC secret used to sign session cookies (minimum 32 random bytes). Required when existingSecret is empty. |
| auth.username | string | `"admin"` | Login username |
| fullnameOverride | string | `""` | Full name override for all resources |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.repository | string | `"ghcr.io/steled/shopping-list"` | Image repository |
| image.tag | string | `""` | Overrides the image tag whose default is the chart AppVersion |
| imagePullSecrets | list | `[]` | Image pull secrets for private registries (e.g. `[{name: my-pull-secret}]`) |
| livenessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/healthz","port":8080},"initialDelaySeconds":10,"periodSeconds":30}` | Liveness probe configuration |
| nameOverride | string | `""` | Partial name override for all resources |
| networking.gateway.hostname | string | `"shopping-list.tools.example.lan"` | Hostname exposed via the HTTPRoute |
| networking.gateway.parentRefs | list | `[{"name":"api-gateway","namespace":"nginx-gateway","sectionName":""}]` | Gateway parentRefs (which Gateway and section to attach to) |
| networking.ingress.annotations | object | `{}` | Additional annotations for the Ingress resource |
| networking.ingress.className | string | `""` | IngressClass name |
| networking.ingress.hostname | string | `""` | Hostname exposed via Ingress |
| networking.ingress.tls | list | `[]` | TLS configuration for the Ingress |
| networking.type | string | `"gateway"` | Networking type: `gateway` creates a Gateway API HTTPRoute, `ingress` creates a classic Kubernetes Ingress |
| nodeSelector | object | `{}` | Node selector for pod scheduling |
| persistence.accessMode | string | `"ReadWriteOnce"` | PVC access mode |
| persistence.annotations | object | `{}` | Annotations to add to the PVC. |
| persistence.enabled | bool | `true` | Enable persistent storage for the SQLite database. When disabled an emptyDir is used and data is lost on restart. |
| persistence.mountPath | string | `"/data"` | Mount path for the data volume inside the container |
| persistence.size | string | `"1Gi"` | PVC storage size |
| persistence.storageClass | string | `""` | Storage class for the PVC. Leave empty to use the cluster default. |
| podAnnotations | object | `{}` | Annotations to add to the pod |
| podLabels | object | `{}` | Labels to add to the pod |
| podSecurityContext | object | `{"fsGroup":1001}` | Pod-level security context |
| readinessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/healthz","port":8080},"initialDelaySeconds":5,"periodSeconds":10}` | Readiness probe configuration |
| replicaCount | int | `1` | Number of pod replicas |
| resources | object | `{"limits":{"cpu":"100m","memory":"64Mi"},"requests":{"cpu":"20m","memory":"32Mi"}}` | CPU/memory resource requests and limits for the application container |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":false,"runAsGroup":1001,"runAsNonRoot":true,"runAsUser":1001}` | Container-level security context |
| service.port | int | `8080` | Service port (should match the application listen port) |
| service.type | string | `"ClusterIP"` | Kubernetes service type |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | Name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| tolerations | list | `[]` | Tolerations for pod scheduling |
