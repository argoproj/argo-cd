-- HostedCluster represents an OpenShift cluster whose control plane is hosted on a management cluster.
-- It is the primary API for creating and managing HyperShift-based hosted clusters.
--
-- Documentation:
--   API reference: https://hypershift-docs.netlify.app/reference/api/
--   Architecture:  https://hypershift-docs.netlify.app/reference/concepts-and-personas/
--
-- Condition types and constants are defined in:
--   https://github.com/openshift/hypershift/blob/main/api/hypershift/v1beta1/hostedcluster_conditions.go
--
-- HostedCluster uses the standard OpenShift operator condition pattern:
--   Available   (True)  - The hosted cluster control plane is healthy and functional
--   Progressing (True)  - An initial deployment or upgrade is in progress
--   Degraded    (True)  - An error requiring user intervention has occurred
--
-- ArgoCD health mapping:
--   Degraded=True    => Degraded    (checked first — errors must always surface)
--   Progressing=True => Progressing (checked before Available to surface upgrades:
--                                    during an upgrade both Available=True and
--                                    Progressing=True are set simultaneously)
--   Available=True   => Healthy
--   No conditions    => Progressing (waiting for first reconcile)
local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for HostedCluster status"
  return hs
end

-- Degraded takes priority: a cluster in error state should surface immediately
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Degraded" and condition.status == "True" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
end

-- Progressing is checked before Available: during an upgrade both
-- Available=True and Progressing=True are set simultaneously; returning
-- Progressing ensures ArgoCD surfaces the in-flight upgrade correctly.
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Progressing" and condition.status == "True" then
    hs.status = "Progressing"
    hs.message = condition.message
    return hs
  end
end

-- Available=True with no active Progressing means the cluster is healthy
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Available" and condition.status == "True" then
    hs.status = "Healthy"
    hs.message = condition.message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for HostedCluster to become available"
return hs
