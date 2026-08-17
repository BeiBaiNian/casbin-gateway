# Kubernetes Deployment Guide for Casbin Gateway

This guide provides instructions for deploying Casbin Gateway on Kubernetes.

## Prerequisites

- A running Kubernetes cluster (1.19+)
- `kubectl` configured to access your cluster
- A running Casdoor instance (can be in the same cluster or external)
- Basic understanding of Kubernetes resources

## Architecture

The deployment consists of:
- **Casbin Gateway Application**: The main WAF application
- **PersistentVolumeClaim**: Holds the SQLite database at `/data/caswaf.db`. There is no database server to deploy
- **Secrets**: Stores sensitive credentials (Casdoor client ID/secret)
- **ConfigMap**: Contains Casbin Gateway configuration template
- **Service**: Exposes Casbin Gateway within the cluster
- **Ingress** (optional): Exposes Casbin Gateway externally

Because SQLite is a single file written by one process, the Deployment stays at
one replica and uses the `Recreate` strategy. Switch to MySQL or PostgreSQL (see
[Using an external database](#using-an-external-database)) before scaling out.

## Quick Start

### 1. Deploy Casdoor (if not already deployed)

Casbin Gateway requires Casdoor for authentication. If you don't have Casdoor deployed:

```bash
# Follow Casdoor's Kubernetes deployment guide:
# https://casdoor.org/docs/deployment/k8s
```

### 2. Configure Secrets

Edit `k8s/secret.yaml` and update the sensitive credentials:

```yaml
stringData:
  casdoor-client-id: "YOUR_ACTUAL_CLIENT_ID"
  casdoor-client-secret: "YOUR_ACTUAL_CLIENT_SECRET"
```

**Important**: 
- **REQUIRED**: You must replace all placeholder values before deployment
- Get `casdoor-client-id` and `casdoor-client-secret` from your Casdoor application settings
- The deployment will fail with validation errors if placeholders are not replaced

**Security Best Practice**: Never commit actual secrets to version control. Consider using:
- [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets)
- [External Secrets Operator](https://external-secrets.io/)
- Cloud provider secret management (AWS Secrets Manager, Azure Key Vault, GCP Secret Manager)

### 3. Review the data volume

`k8s/pvc.yaml` requests 10Gi of `ReadWriteOnce` storage for the SQLite database.
Adjust the size, or add a `storageClassName`, if your cluster has no default
storage class.

### 4. Configure Casdoor Endpoint (Optional)

If your Casdoor is not at `http://casdoor.casdoor-system.svc.cluster.local:8000`, edit `k8s/configmap.yaml`:

```yaml
casdoorEndpoint: http://your-casdoor-service:port
```

### 5. Deploy to Kubernetes

**Option A: Using the deployment script (Recommended)**

The easiest way to deploy Casbin Gateway:

```bash
cd k8s
chmod +x deploy.sh
./deploy.sh
```

The script will:
- Validate your configuration
- Create the namespace and the data volume
- Deploy secrets and configuration
- Deploy Casbin Gateway application
- Optionally deploy Ingress
- Show deployment status

**Option B: Using individual files**

```bash
# Create the namespace
kubectl apply -f k8s/namespace.yaml

# Create the volume that holds the SQLite database
kubectl apply -f k8s/pvc.yaml

# Deploy Secrets
kubectl apply -f k8s/secret.yaml

# Deploy ConfigMap
kubectl apply -f k8s/configmap.yaml

# Deploy Casbin Gateway
kubectl apply -f k8s/deployment.yaml

# (Optional) Deploy Ingress
kubectl apply -f k8s/ingress.yaml
```

**Option C: Using Kustomize**
```bash
kubectl apply -k k8s/
```

### 6. Verify Deployment

```bash
# Check if pods are running
kubectl get pods -n caswaf

# Check logs
kubectl logs -f deployment/caswaf -n caswaf

# Check services
kubectl get svc -n caswaf
```

### 7. Access Casbin Gateway

If using Ingress:
```bash
# Update your DNS or /etc/hosts to point to your ingress controller IP
# Then access: http://caswaf.example.com
```

If using port-forward for testing:
```bash
kubectl port-forward svc/caswaf 17000:17000 -n caswaf
# Access: http://localhost:17000
```

## Configuration Details

### Secrets (`secret.yaml`)

Stores sensitive credentials:
- `casdoor-client-id`: Casdoor application client ID
- `casdoor-client-secret`: Casdoor application client secret

**Security Note**: Never commit actual secrets to version control. Use sealed-secrets, external secret operators, or other secret management solutions in production.

### ConfigMap (`configmap.yaml`)

Key configuration parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `httpport` | Casbin Gateway HTTP port | `17000` |
| `runmode` | Run mode (dev/prod) | `prod` |
| `driverName` | Database driver | `sqlite` |
| `dataSourceName` | SQLite file, or the connection string of a database server | `/data/caswaf.db` |
| `dbName` | Database name (unused by SQLite) | `caswaf` |
| `casdoorEndpoint` | Casdoor API endpoint | Required |
| `casdoorInsecureSkipVerify` | Skip TLS verification for Casdoor | `true` |
| `clientId` | Casdoor application client ID | Uses secrets substitution |
| `clientSecret` | Casdoor application client secret | Uses secrets substitution |
| `casdoorOrganization` | Casdoor organization name | `built-in` |
| `casdoorApplication` | Casdoor application name | Required |

### Data Volume (`pvc.yaml`)

- 10Gi `ReadWriteOnce` claim mounted at `/data`
- Holds `caswaf.db`, the SQLite database, plus its `-wal` and `-shm` sidecar files
- Deleting the claim deletes all Gateway data

### Casbin Gateway Deployment (`deployment.yaml`)

Features:
- Init container that substitutes secrets into the configuration file
- TCP-based liveness and readiness probes (no authentication required)
- Resource limits and requests
- Configuration mounted from ConfigMap with secret substitution
- One replica with the `Recreate` strategy, because the SQLite file takes a
  single writer

## Troubleshooting

### Common Issues

#### 1. Pod stuck in `Pending`

**Cause**: The `caswaf-data-pvc` claim is unbound, usually because the cluster has no default storage class.

**Solution**:
```bash
# Check the claim and why it is not bound
kubectl get pvc -n caswaf
kubectl describe pvc caswaf-data-pvc -n caswaf

# List the available storage classes, then set storageClassName in k8s/pvc.yaml
kubectl get storageclass
```

#### 2. "casdoorsdk.GetCerts() error: Unauthorized operation"

**Cause**: Incorrect Casdoor configuration or credentials

**Solution**:
1. Verify Casdoor is accessible:
   ```bash
   kubectl run -it --rm debug --image=curlimages/curl --restart=Never -n caswaf -- \
     curl -v http://casdoor.casdoor-system.svc.cluster.local:8000
   ```

2. Verify `clientId` and `clientSecret` in ConfigMap match your Casdoor application

3. Ensure the Casdoor application is configured correctly:
   - Organization name matches `casdoorOrganization`
   - Application name matches `casdoorApplication`
   - Client ID and secret are correct

4. Check Casdoor logs for authentication errors

#### 3. Database Connection Issues

The startup summary in the pod log names the database Gateway actually opened,
and says whether it answered.

**Solution**:
```bash
# Confirm the SQLite file is on the volume
kubectl exec -it deployment/caswaf -n caswaf -- ls -l /data

# For an external database server, test that it is reachable from the pod
kubectl exec -it deployment/caswaf -n caswaf -- sh
# Then inside the pod, e.g.:
# nc -zv mysql.example.com 3306
```

#### 4. Init Container Stuck

If the `setup-config` init container never finishes:
```bash
# Check init container logs
kubectl logs -n caswaf <pod-name> -c setup-config

# Force restart
kubectl rollout restart deployment/caswaf -n caswaf
```

### Viewing Logs

```bash
# Casbin Gateway logs
kubectl logs -f deployment/caswaf -n caswaf

# All logs in namespace
kubectl logs -f -n caswaf --all-containers=true
```

## Production Recommendations

1. **Use an External Database**: The SQLite default is fine for a single replica. For production, consider a managed MySQL or PostgreSQL service (AWS RDS, Google Cloud SQL, etc.) — see [Using an external database](#using-an-external-database)

2. **Configure TLS**: 
   - Set `casdoorInsecureSkipVerify = false`
   - Use proper TLS certificates for Casdoor

3. **Resource Limits**: Adjust resource limits based on your traffic:
   ```yaml
   resources:
     requests:
       memory: "512Mi"
       cpu: "500m"
     limits:
       memory: "2Gi"
       cpu: "2000m"
   ```

4. **High Availability**:
   - Move to an external database first: more than one replica cannot share the SQLite file
   - Then increase replicas for Casbin Gateway
   - Use database replication or a managed service
   - Configure proper health checks

5. **Monitoring**: Set up monitoring and alerting:
   - Prometheus metrics
   - Application logs
   - Resource usage

6. **Backup**: Regular backups of the database. For SQLite that is the `caswaf-data-pvc` volume:
   ```bash
   kubectl exec deployment/caswaf -n caswaf -- tar cf - /data > caswaf-data.tar
   ```

7. **Security**:
   - Use Kubernetes Secrets for sensitive data
   - Enable RBAC
   - Network policies to restrict traffic
   - Regular security updates

## Updating Casbin Gateway

```bash
# Update the image version in deployment.yaml, then:
kubectl set image deployment/caswaf caswaf=casbin/caswaf:NEW_VERSION -n caswaf

# Or apply updated deployment
kubectl apply -f k8s/deployment.yaml

# Check rollout status
kubectl rollout status deployment/caswaf -n caswaf
```

## Uninstall

```bash
# Delete all resources
kubectl delete -f k8s/ingress.yaml
kubectl delete -f k8s/deployment.yaml
kubectl delete -f k8s/configmap.yaml
kubectl delete -f k8s/secret.yaml

# Deleting the claim destroys the SQLite database
kubectl delete -f k8s/pvc.yaml

# Or delete the entire namespace
kubectl delete namespace caswaf
```

## Advanced Configuration

### Using an external database

Edit `configmap.yaml`:
```yaml
driverName = mysql
dataSourceName = root:password@tcp(external-mysql.example.com:3306)/
dbName = caswaf
```

Gateway creates `dbName` on first start if it does not exist. Once the data no
longer lives on the volume you can drop the `data` volume and its mount from
`deployment.yaml`, stop deploying `pvc.yaml`, and raise `replicas`.

### Using Redis for Sessions

Edit `configmap.yaml`:
```yaml
redisEndpoint: redis-host:6379
```

### Custom Domain

Edit `ingress.yaml`:
```yaml
spec:
  rules:
  - host: waf.yourdomain.com
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/apache/casbin-gateway/issues
- Documentation: https://caswaf.org
- Casdoor Documentation: https://casdoor.org

## License

Apache-2.0
