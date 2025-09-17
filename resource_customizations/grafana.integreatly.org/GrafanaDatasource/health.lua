
-- if UID not yet created, we are progressing
if obj.status == nil or obj.status.uid == "" then
  return {
    status = "Progressing",
    message = "",
  }
end

-- NoMatchingInstances distinguishes if we are healthy or degraded
if obj.status.NoMatchingInstances then
  return {
    status = "Degraded",
    message = "can't find matching grafana instance",
  }
end
return {
  status = "Healthy",
  message = "",
}