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

local hs = {}
if obj.status ~= nil then

  if obj.status.state == "initializing" then
    hs.status = "Progressing"
    hs.message = obj.status.ready .. "/" .. obj.status.size .. " node(s) are ready"
    return hs
  end

  if obj.status.state == "ready" then
    hs.status = "Healthy"
    hs.message = obj.status.ready .. "/" .. obj.status.size .. " node(s) are ready"
    return hs
  end

  if obj.status.state == "paused" then
    hs.status = "Unknown"
    hs.message = "Cluster is paused"
    return hs
  end

  if obj.status.state == "stopping" then
    hs.status = "Degraded"
    hs.message = "Cluster is stopping (" .. obj.status.ready .. "/" .. obj.status.size .. " node(s) are ready)"
    return hs
  end

  if obj.status.state == "error" then
    hs.status = "Degraded"
    hs.message = "Cluster is on error: " .. table.concat(obj.status.message, ", ")
    return hs
  end

end

hs.status = "Unknown"
hs.message = "Cluster status is unknown. Ensure your ArgoCD is current and then check for/file a bug report: https://github.com/argoproj/argo-cd/issues"
return hs
