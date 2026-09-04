-- GCPPrivateServiceConnect manages the GCP Private Service Connect infrastructure
-- used for private connectivity to a hosted cluster's API server on GCP.
-- It provisions a Service Attachment on the management cluster's ILB and a PSC Endpoint
-- in the customer VPC, along with DNS configuration.
--
-- Condition types and reasons are defined in:
--   https://github.com/openshift/hypershift/blob/main/api/hypershift/v1beta1/gcpprivateserviceconnect_types.go
--
-- Condition types:
--   GCPPrivateServiceConnectAvailable - overall infrastructure availability (primary/aggregate)
--   GCPServiceAttachmentAvailable     - the Service Attachment exists on the management ILB
--   GCPEndpointAvailable              - the PSC Endpoint exists in the customer VPC
--   GCPDNSAvailable                   - DNS zone and records are configured
--     Reasons (all conditions):
--       GCPSuccess - operation completed successfully
--       GCPError   - GCP API error (permissions, quota, resource conflict, etc.)
--
-- ArgoCD health mapping (based on the primary/aggregate condition):
--   GCPPrivateServiceConnectAvailable=True              => Healthy
--   GCPPrivateServiceConnectAvailable=False, reason=GCPError => Degraded
--   GCPPrivateServiceConnectAvailable=False, other      => Progressing
--   No conditions                                       => Progressing
local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for GCPPrivateServiceConnect status"
  return hs
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "GCPPrivateServiceConnectAvailable" then
    if condition.status == "True" then
      hs.status = "Healthy"
      hs.message = condition.message
      return hs
    end
    if condition.reason == "GCPError" then
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
hs.message = "Waiting for GCP Private Service Connect to become available"
return hs
