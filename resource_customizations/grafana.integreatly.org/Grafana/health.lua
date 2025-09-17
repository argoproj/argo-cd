
-- if no status info available yet, assume progressing
if obj.status == nil or obj.status.stageStatus == nil then
  return {
    status = "Progressing",
    message = "Waiting for Grafana status info",
  }
end

-- if last stage failed, we are stuck here
if obj.status.stageStatus == "failed" then
  return {
    status = "Degraded",
    message = "Failed at stage " .. obj.status.stage,
  }
end

-- only if "complete" stage was successful, Grafana can be considered healthy
if obj.status.stage == "complete" and obj.status.stageStatus == "success" then
  return {
    status = "Healthy",
    message = "",
  }
end

-- no final status yet, assume progressing
return {
  status = "Progressing",
  message = obj.status.stage,
}