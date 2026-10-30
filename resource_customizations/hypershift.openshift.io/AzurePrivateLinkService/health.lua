-- AzurePrivateLinkService manages the Azure Private Link Service infrastructure
-- used for private connectivity to a hosted cluster's API server on Azure.
-- It provisions an Internal Load Balancer, a Private Link Service, a Private Endpoint,
-- and the corresponding Private DNS zone and A records.
--
-- Condition types and reasons are defined in:
--   https://github.com/openshift/hypershift/blob/main/api/hypershift/v1beta1/azureprivatelinkservice_types.go
--
-- Condition types:
--   AzurePrivateLinkServiceAvailable    - overall infrastructure availability (primary)
--   AzureInternalLoadBalancerAvailable  - the ILB has a frontend IP
--   AzurePLSCreated                     - the PLS resource has been created
--   AzurePrivateEndpointAvailable       - the Private Endpoint exists in the guest VNet
--   AzurePrivateDNSAvailable            - DNS zone and A records are configured
--     Reasons (all conditions):
--       AzureSuccess - operation completed successfully
--       AzureError   - Azure API error (quota, permissions, resource conflict, etc.)
--
-- ArgoCD health mapping (based on the primary/aggregate condition):
--   AzurePrivateLinkServiceAvailable=True              => Healthy
--   AzurePrivateLinkServiceAvailable=False, reason=AzureError => Degraded
--   AzurePrivateLinkServiceAvailable=False, other      => Progressing
--   No conditions                                      => Progressing
local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for AzurePrivateLinkService status"
  return hs
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "AzurePrivateLinkServiceAvailable" then
    if condition.status == "True" then
      hs.status = "Healthy"
      hs.message = condition.message
      return hs
    end
    if condition.reason == "AzureError" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
    hs.status = "Progressing"
    hs.message = condition.message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Azure Private Link Service to become available"
return hs
