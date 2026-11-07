-- AWSEndpointService manages the AWS VPC Endpoint Service and corresponding VPC Endpoint
-- used for private connectivity to a hosted cluster's API server on AWS.
-- One resource is created per NLB (e.g. kube-apiserver, ignition).
--
-- Condition types and reasons are defined in:
--   https://github.com/openshift/hypershift/blob/main/api/hypershift/v1beta1/endpointservice_types.go
--
-- Condition types:
--   AWSEndpointServiceAvailable - the NLB-backed Endpoint Service exists in the management VPC
--   AWSEndpointAvailable        - the VPC Endpoint exists in the guest VPC
--     Reasons (both conditions):
--       AWSSuccess - operation completed successfully
--       AWSError   - AWS API error (quota, permissions, resource not found, etc.)
--
-- ArgoCD health mapping:
--   Any condition with reason=AWSError and status=False => Degraded
--   Both AWSEndpointServiceAvailable=True AND AWSEndpointAvailable=True => Healthy
--   Otherwise => Progressing (creation in progress)
local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for AWSEndpointService status"
  return hs
end

-- Any condition in error state is degraded
for _, condition in ipairs(obj.status.conditions) do
  if condition.status == "False" and condition.reason == "AWSError" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
end

-- Both endpoint conditions must be True
local serviceAvailable = false
local endpointAvailable = false
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "AWSEndpointServiceAvailable" and condition.status == "True" then
    serviceAvailable = true
  end
  if condition.type == "AWSEndpointAvailable" and condition.status == "True" then
    endpointAvailable = true
  end
end

if serviceAvailable and endpointAvailable then
  hs.status = "Healthy"
  hs.message = "AWS endpoint service and endpoint are available"
  return hs
end

hs.status = "Progressing"
hs.message = "Waiting for AWS endpoint service and endpoint to become available"
return hs
