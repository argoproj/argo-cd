-- HostedControlPlane is an internal resource created by the HyperShift operator per HostedCluster.
-- It represents the control plane components running as pods on the management cluster.
-- This resource is typically managed by the operator and visible in the control plane namespace.
--
-- Documentation:
--   API reference: https://hypershift-docs.netlify.app/reference/api/
--
-- Condition types and constants are defined in:
--   https://github.com/openshift/hypershift/blob/main/api/hypershift/v1beta1/hostedcluster_conditions.go
--
-- HostedControlPlane uses the standard OpenShift operator condition pattern:
--   Available   (True)  - The control plane API server is ready and functional
--   Progressing (True)  - Control plane components are being deployed or updated
--   Degraded    (True)  - An error has been detected in the control plane
-- In addition, status.ready (bool) is set to true when the API server is ready.
--
-- ArgoCD health mapping:
--   Degraded=True    => Degraded    (checked first)
--   Progressing=True => Progressing (checked before Available: during an upgrade
--                                    both Available=True and Progressing=True are
--                                    set simultaneously)
--   Available=True   => Healthy
--   status.ready=true (no conditions) => Healthy (fallback)
--   No status        => Progressing
local hs = {}

if obj.status == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for HostedControlPlane status"
  return hs
end

if obj.status.conditions ~= nil then
  -- Degraded takes priority
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Degraded" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
  end

  -- Progressing is checked before Available: during an upgrade both
  -- Available=True and Progressing=True are set simultaneously.
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Progressing" and condition.status == "True" then
      hs.status = "Progressing"
      hs.message = condition.message
      return hs
    end
  end

  -- Available=True with no active Progressing means the control plane is healthy
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Available" and condition.status == "True" then
      hs.status = "Healthy"
      hs.message = condition.message
      return hs
    end
  end
end

-- Fall back to the ready boolean when conditions are not yet populated
if obj.status.ready == true then
  hs.status = "Healthy"
  hs.message = "HostedControlPlane is ready"
  return hs
end

hs.status = "Progressing"
hs.message = "Waiting for HostedControlPlane to become available"
return hs
