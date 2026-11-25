-- External Secrets Operator syncs a changed spec, unless the Secret is only created once: with refreshPolicy
-- CreatedOnce, or since 0.11.0 with a refreshInterval of 0 or less and any refreshPolicy but OnChange. Until it does,
-- Ready still describes the previous spec. status.syncedResourceVersion is
-- "<generation>-<hash of labels and annotations>" as of the last successful sync. Only the generation is compared.
-- Before 0.11.0, immutable targets and Secrets deleted by deletionPolicy Delete didn't update syncedResourceVersion.
-- https://github.com/external-secrets/external-secrets/blob/v2.14.0/pkg/controllers/externalsecret/externalsecret_controller.go
local function waitingForSpecSync(ready)
  if obj.metadata == nil or obj.metadata.generation == nil or obj.spec == nil
      or obj.status.syncedResourceVersion == nil then
    return false
  end
  if obj.spec.refreshPolicy == "CreatedOnce" or ready.reason == "SecretDeleted"
      or (obj.spec.target ~= nil and obj.spec.target.immutable == true) then
    return false
  end
  local interval = obj.spec.refreshInterval
  if obj.spec.refreshPolicy ~= "OnChange" and interval ~= nil
      and (string.find(interval, "[1-9]") == nil or string.sub(interval, 1, 1) == "-") then
    return false
  end
  local syncedGeneration = string.match(obj.status.syncedResourceVersion, "^(%d+)-")
  return syncedGeneration ~= nil and tonumber(syncedGeneration) ~= tonumber(obj.metadata.generation)
end

local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" and condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
      if condition.type == "Ready" and condition.status == "True" then
        if waitingForSpecSync(condition) then
          hs.status = "Progressing"
          hs.message = "Waiting for ExternalSecret to sync generation " .. obj.metadata.generation
          return hs
        end
        hs.status = "Healthy"
        hs.message = condition.message
        return hs
      end
    end
  end
end
hs.status = "Progressing"
hs.message = "Waiting for ExternalSecret"
return hs
