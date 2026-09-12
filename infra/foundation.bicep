param location string = resourceGroup().location
param appName string = 'camplist'
param githubRepository string = 'Baqiwaqi/camplist'
param githubEnvironment string = 'production'
param registryName string = 'camplist${uniqueString(resourceGroup().id)}'

var tags = { application: appName, environment: 'production', managedBy: 'bicep' }

resource registry 'Microsoft.ContainerRegistry/registries@2023-07-01' = {
  name: registryName
  location: location
  tags: tags
  sku: { name: 'Basic' }
  properties: { adminUserEnabled: false }
}
resource environment 'Microsoft.App/managedEnvironments@2024-03-01' = {
  name: 'env-${appName}-prod'
  location: location
  tags: tags
  properties: {
    // Omit log storage: live log streaming works without a billed workspace.
    zoneRedundant: false
  }
}
resource runtimeIdentity 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' = {
  name: 'id-${appName}-runtime'
  location: location
  tags: tags
}
resource deploymentIdentity 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' = {
  name: 'id-${appName}-github'
  location: location
  tags: tags
}
resource federation 'Microsoft.ManagedIdentity/userAssignedIdentities/federatedIdentityCredentials@2023-01-31' = {
  parent: deploymentIdentity
  name: 'github-production'
  properties: {
    issuer: 'https://token.actions.githubusercontent.com'
    subject: 'repo:${githubRepository}:environment:${githubEnvironment}'
    audiences: [ 'api://AzureADTokenExchange' ]
  }
}
resource runtimePull 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(registry.id, runtimeIdentity.id, 'AcrPull')
  scope: registry
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', '7f951dda-4ed3-4680-a7ca-43fe172d538d')
    principalId: runtimeIdentity.properties.principalId
    principalType: 'ServicePrincipal'
  }
}
resource deploymentPush 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(registry.id, deploymentIdentity.id, 'AcrPush')
  scope: registry
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', '8311e382-0749-4cb8-b61a-304f252e45ec')
    principalId: deploymentIdentity.properties.principalId
    principalType: 'ServicePrincipal'
  }
}
resource deploymentAppAccess 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(resourceGroup().id, deploymentIdentity.id, 'Container Apps Contributor')
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', '358470bc-b998-42bd-ab17-a7e34c199c0f')
    principalId: deploymentIdentity.properties.principalId
    principalType: 'ServicePrincipal'
  }
}
resource deploymentIdentityAccess 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(runtimeIdentity.id, deploymentIdentity.id, 'Managed Identity Operator')
  scope: runtimeIdentity
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', 'f1a07417-d97a-45cb-824c-7a7467783830')
    principalId: deploymentIdentity.properties.principalId
    principalType: 'ServicePrincipal'
  }
}
output registryName string = registry.name
output registryServer string = registry.properties.loginServer
output environmentName string = environment.name
output defaultDomain string = environment.properties.defaultDomain
output runtimeIdentityName string = runtimeIdentity.name
output clientId string = deploymentIdentity.properties.clientId
output tenantId string = tenant().tenantId
output subscriptionId string = subscription().subscriptionId
