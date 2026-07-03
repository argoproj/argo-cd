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
if obj.status ~= nil then
    if obj.status.state ~= nil then
        if obj.status.state == "Exists" then
            hs.status = "Healthy"
            hs.message = "Component state: Exists."
        end
        if obj.status.state == "NotFound" then
            hs.status = "Missing"
            hs.message = "Component state: NotFound."
        end
        if obj.status.state == "ConfigError" then
            hs.status = "Degraded"
            hs.message = "Component state: ConfigError."
        end
        if obj.status.state == "Unknown" then
            hs.status = "Unknown"
            hs.message = "Component state: Unknown."
        end
    end
    return hs
end
return hs
