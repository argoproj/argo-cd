-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

hs = {
    status = "Progressing",
    message = "Update in progress"
}
if (obj.metadata.labels ~= nil and obj.metadata.labels["shoot.gardener.cloud/status"] ~= nil) then
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "healthy" then
        hs.status = "Healthy"
        hs.message = "Component state: Healthy"
    end
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "progressing" then
        hs.status = "Progressing"
        hs.message = "Component state: Update in progress"
    end
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "unhealthy" then
        hs.status = "Degraded"
        hs.message = "Component state: Unhealthy"
    end
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "unknown" then
        hs.status = "Unknown"
        hs.message = "Component state: Unknown"
    end
    return hs
end
return hs
