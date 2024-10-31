local hs = {}
if obj.status ~= nil then

  if obj.status.state == "initializing" then
    hs.status = "Progressing"
    hs.message = "Cluster is initializing..."
    return hs
  end

  if obj.status.state == "ready" then
    hs.status = "Healthy"
    hs.message = "Cluster is healthy"
    return hs
  end

  if obj.status.state == "error" then
    hs.status = "Degraded"
    hs.message = "Cluster has error"
    return hs
  end

  if obj.status.state == "stopping" then
    hs.status = "Progressing"
    hs.message = "Cluster is stopping..."
    return hs
  end

  if obj.status.state == "paused" then
    hs.status = "Suspended"
    hs.message = "Cluster is paused"
    return hs
  end

end

hs.status = "Unknown"
hs.message = "Cluster status is unknown"
return hs
