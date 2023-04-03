hs = {}
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
    hs.status = "Degraded"
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
    hs.message = "Cluster is on error: " .. table.concat(obj.status.messages, ", ")
    return hs
  end

end

hs.status = "Progressing"
hs.message = "Waiting for cluster state"
return hs
