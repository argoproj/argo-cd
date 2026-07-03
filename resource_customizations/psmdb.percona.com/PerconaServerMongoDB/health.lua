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
  local state_map = {
    initializing = "Progressing",
    ready = "Healthy",
    error = "Degraded",
    stopping = "Progressing",
    paused = "Suspended"
  }

  hs.status = state_map[obj.status.state] or "Unknown"
  hs.message = obj.status.ready .. "/" .. obj.status.size .. " node(s) are ready"
  return hs
end

hs.status = "Unknown"
hs.message = "Cluster status is unknown"
return hs
