param location string = resourceGroup().location
param appName string = 'camplist'
param registryName string
param image string
param dbUrl string
param googleClientId string
@secure()
param dbKey string
@secure()
param googleClientSecret string
@secure()
param sessionKey string
@secure()
param csrfKey string

resource environment 'Microsoft.App/managedEnvironments@2024-03-01' existing = {
  name: 'env-${appName}-prod'
}
resource registry 'Microsoft.ContainerRegistry/registries@2023-07-01' existing = {
  name: registryName
}
resource identity 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' existing = {
  name: 'id-${appName}-runtime'
}
var callbackUrl = 'https://${appName}.${environment.properties.defaultDomain}/auth/callback'
resource app 'Microsoft.App/containerApps@2024-03-01' = {
  name: appName
  location: location
  tags: { application: appName, environment: 'production', managedBy: 'bicep' }
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: { '${identity.id}': {} }
  }
  properties: {
    managedEnvironmentId: environment.id
    configuration: {
      activeRevisionsMode: 'Single'
      ingress: {
        external: true
        allowInsecure: false
        targetPort: 3000
        transport: 'http'
      }
      registries: [ { server: registry.properties.loginServer, identity: identity.id } ]
      secrets: [
        { name: 'db-key', value: dbKey }
        { name: 'google-client-secret', value: googleClientSecret }
        { name: 'session-key', value: sessionKey }
        { name: 'csrf-key', value: csrfKey }
      ]
    }
    template: {
      containers: [ {
        name: appName
        image: image
        resources: { cpu: json('0.25'), memory: '0.5Gi' }
        env: [
          { name: 'DB_URL', value: dbUrl }
          { name: 'DB_KEY', secretRef: 'db-key' }
          { name: 'GOOGLE_CLIENT_ID', value: googleClientId }
          { name: 'GOOGLE_CLIENT_SECRET', secretRef: 'google-client-secret' }
          { name: 'REDIRECT_URL', value: callbackUrl }
          { name: 'SESSION_KEY', secretRef: 'session-key' }
          { name: 'CSRF_KEY', secretRef: 'csrf-key' }
          { name: 'GOMEMLIMIT', value: '400MiB' }
        ]
        probes: [
          { type: 'Startup', httpGet: { path: '/healthz', port: 3000 }, periodSeconds: 5, failureThreshold: 30 }
          { type: 'Liveness', httpGet: { path: '/healthz', port: 3000 }, periodSeconds: 30, failureThreshold: 3 }
          { type: 'Readiness', httpGet: { path: '/healthz', port: 3000 }, periodSeconds: 10, failureThreshold: 3 }
        ]
      } ]
      scale: {
        minReplicas: 0
        maxReplicas: 1
        rules: [ { name: 'http', http: { metadata: { concurrentRequests: '20' } } } ]
      }
    }
  }
}
output url string = 'https://${app.properties.configuration.ingress.fqdn}'
output callbackUrl string = callbackUrl
