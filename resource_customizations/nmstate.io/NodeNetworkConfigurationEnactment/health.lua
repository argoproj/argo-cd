-- NodeNetworkConfigurationEnactment (NNCE) is a cluster-scoped, per-node resource
-- that tracks the application of a NodeNetworkConfigurationPolicy on a single node.
-- NNCEs are named <node>.<policy> and have no spec; all state is in status.
--
-- Documentation:
--   User guide (configuration & conditions): https://github.com/nmstate/kubernetes-nmstate/blob/main/docs/user-guide/102-configuration.md
--   Troubleshooting (failure states):        https://github.com/nmstate/kubernetes-nmstate/blob/main/docs/user-guide/103-troubleshooting.md
--
-- Condition types and reasons are defined in:
--   https://github.com/nmstate/kubernetes-nmstate/blob/main/api/shared/nodenetworkconfigurationenactment_types.go
--
-- NNCE exposes five condition types:
--   Progressing (True)                     - Actively applying desired state (ConfigurationProgressing)
--   Failing     (True) + Progressing(True) - Failed at least once, still retrying (Retrying)
--   Failing     (True) + Progressing(False)- Terminal failure, no more retries (FailedToConfigure)
--   Pending     (True)                     - Blocked by maxUnavailable limit (MaxUnavailableLimitReached)
--   Aborted     (True)                     - Skipped because another node in the rollout failed (ConfigurationAborted)
--   Available   (True)                     - Successfully configured (SuccessfullyConfigured)
--
-- ArgoCD health mapping:
--   Progressing=True                           => Progressing
--   Failing=True + Progressing=True (retry)    => Progressing
--   Failing=True + Progressing=False (terminal)=> Degraded
--   Pending=True                               => Progressing
--   Aborted=True                               => Suspended
--   Available=True                             => Healthy
--   No status yet                              => Progressing
local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local failing = false
    local progressing = false

    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Failing" and condition.status == "True" then
        failing = true
      end
      if condition.type == "Progressing" and condition.status == "True" then
        progressing = true
      end
    end

    -- Retrying: Failing=True AND Progressing=True simultaneously
    if failing and progressing then
      hs.status = "Progressing"
      hs.message = "Retrying"
      for i, condition in ipairs(obj.status.conditions) do
        if condition.type == "Failing" and condition.status == "True" then
          local msg = condition.reason
          if condition.message ~= nil and condition.message ~= "" then
            msg = condition.reason .. ": " .. condition.message
          end
          hs.message = msg
        end
      end
      return hs
    end

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
        if condition.type == "Failing" then
          hs.status = "Degraded"
          hs.message = msg
          return hs
        end
        if condition.type == "Progressing" then
          hs.status = "Progressing"
          hs.message = msg
          return hs
        end
        if condition.type == "Pending" then
          hs.status = "Progressing"
          hs.message = msg
          return hs
        end
        if condition.type == "Aborted" then
          hs.status = "Suspended"
          hs.message = msg
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for enactment to be applied"
return hs
