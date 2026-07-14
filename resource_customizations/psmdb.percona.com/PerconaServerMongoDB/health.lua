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
