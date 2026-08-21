-- NodeNetworkConfigurationPolicy (NNCP) is a cluster-scoped resource that defines
-- the desired network configuration for nodes in a Kubernetes cluster.
--
-- Documentation:
--   User guide (configuration & conditions): https://github.com/nmstate/kubernetes-nmstate/blob/main/docs/user-guide/102-configuration.md
--   Troubleshooting (failure states):        https://github.com/nmstate/kubernetes-nmstate/blob/main/docs/user-guide/103-troubleshooting.md
--
-- Condition types and reasons are defined in:
--   https://github.com/nmstate/kubernetes-nmstate/blob/main/api/shared/nodenetworkconfigurationpolicy_types.go
--
-- NNCP exposes three active condition types:
--   Available   (True)  - All matched nodes successfully configured (SuccessfullyConfigured)
--   Degraded    (True)  - One or more nodes failed to configure (FailedToConfigure)
--   Progressing (True)  - Configuration is being applied across nodes (ConfigurationProgressing)
--   Ignored     (True)  - Policy matches no nodes (NoMatchingNode)
--
-- ArgoCD health mapping:
--   Available=True   => Healthy
--   Degraded=True    => Degraded
--   Progressing=True => Progressing
--   Ignored=True     => Suspended (policy intentionally matches no nodes)
--   No status yet    => Progressing
local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.status == "True" then
        local msg = condition.reason
        if condition.message ~= nil and condition.message ~= "" then
          msg = condition.reason .. ": " .. condition.message
        end
        if condition.type == "Available" then
          hs.status = "Healthy"
          hs.message = msg
          return hs
        end
        if condition.type == "Degraded" then
          hs.status = "Degraded"
          hs.message = msg
          return hs
        end
        if condition.type == "Progressing" then
          hs.status = "Progressing"
          hs.message = msg
          return hs
        end
        if condition.type == "Ignored" then
          hs.status = "Suspended"
          hs.message = msg
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for policy to be applied"
return hs
