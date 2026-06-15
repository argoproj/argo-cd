local hs = {}

if obj.status == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for Druid status"
  return hs
end

if obj.status.druidNodeStatus ~= nil then
  local nodeStatus = obj.status.druidNodeStatus

  if nodeStatus.druidNodeConditionStatus == "False" then
    hs.status = "Degraded"
    hs.message = nodeStatus.reason or "Druid cluster is not ready"
    return hs
  end

  if nodeStatus.druidNodeConditionType == "DruidClusterReady" and nodeStatus.druidNodeConditionStatus == "True" then
    hs.status = "Healthy"
    hs.message = nodeStatus.reason or "Druid cluster is ready"
    return hs
  end

  if nodeStatus.druidNodeConditionType == "DruidNodeErrorState" then
    hs.status = "Degraded"
    local podName = nodeStatus.druidNode or "unknown"
    local reason = nodeStatus.reason or "Pod is not ready"
    hs.message = "Pod " .. podName .. ": " .. reason
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Druid cluster is being provisioned"
return hs
